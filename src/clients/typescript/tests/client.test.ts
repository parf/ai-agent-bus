import { test, expect, beforeEach, afterEach } from 'bun:test';
import { spawn, spawnSync, type ChildProcessWithoutNullStreams } from 'node:child_process';
import { mkdtempSync, rmSync, readFileSync } from 'node:fs';
import { resolve, join } from 'node:path';
import { createServer } from 'node:http';
import { createServer as createTLS } from 'node:https';
import { X509Certificate, createHash } from 'node:crypto';
import { inspect } from 'node:util';
const library = await import(resolve(process.env.AB_TYPESCRIPT_CLIENT_PATH ?? join(import.meta.dir,'..'),'src/index.ts')) as typeof import('../src/index.js');
const {Bus,UnexpectedAnswer,BusError,BusClosed,IdentityMismatch,ReaderInUse,UncertainOutcome,BusUnreachable,DrainTimeout,KVNotFound} = library;
const root = resolve(import.meta.dir,'../../../..');
const fixture = process.env.AB_CLIENT_FIXTURE ?? join(root,'tmp/client-fixture');
const sleep = (ms:number) => new Promise<void>(resolve => setTimeout(resolve,ms));
function barrier() { let resolve!:()=>void; const promise=new Promise<void>(r=>resolve=r); return {promise,resolve}; }
async function bounded(promise:Promise<void>,label:string) {
 let timer:ReturnType<typeof setTimeout>;
 try { await Promise.race([promise,new Promise<never>((_,reject)=>{timer=setTimeout(()=>reject(new Error(`timed out: ${label}`)),2000);})]); }
 finally {clearTimeout(timer!);}
}
// Observe the real automatic reply completing, including its response read,
// before teardown can close the bus. The implementation still does the work.
function replyCompleted(b:InstanceType<typeof Bus>) {
 const done=barrier(),reply=b.reply.bind(b);
 b.reply=async(...args:Parameters<typeof reply>)=>{const id=await reply(...args);done.resolve();return id;};
 return ()=>bounded(done.promise,'automatic reply completion');
}
let scratch:string, processFixture:ChildProcessWithoutNullStreams, info:{addr:string;tokens:{[key:string]:string}}, buses:InstanceType<typeof Bus>[], serving:Promise<void>[];
let servers:Array<ReturnType<typeof createServer>>;
const bus = (name:string) => { const b = Bus.connect({addr:info.addr,token:info.tokens[name],expectName:name,timeout:2000}); buses.push(b); return b; };
beforeEach(async () => {
  scratch = mkdtempSync(join(root,'tmp/ts-client-')); buses=[]; serving=[]; servers=[];
  processFixture = spawn(fixture,[join(scratch,'owner.sock')],{stdio:'pipe'});
  info = await new Promise((resolve,reject) => { let text=''; const timer=setTimeout(()=>reject(new Error('fixture startup timed out')),3000); processFixture.stdout.on('data',chunk=>{text+=chunk; if(text.includes('\n')) {clearTimeout(timer);resolve(JSON.parse(text.split('\n')[0]!));}}); processFixture.on('error',reject); });
});
afterEach(async () => {
  for(const b of buses) { try {b.stop();} catch {} }
  await Promise.allSettled(serving); await Promise.all(buses.map(b=>b.close()));
  for(const server of servers) { server.closeAllConnections(); await new Promise<void>(resolve=>server.close(()=>resolve())); }
  processFixture.stdin.end(); await new Promise<void>(resolve=>processFixture.once('exit',()=>resolve())); rmSync(scratch,{recursive:true,force:true});
});
async function serve(b:InstanceType<typeof Bus>,handler:(m:InstanceType<typeof library.Message>)=>string|null|Promise<string|null>,options:Parameters<InstanceType<typeof Bus>['serve']>[1]={}) {
  const promise=b.serve(handler,{drainTimeout:1000,...options}); serving.push(promise); promise.catch(()=>{}); return {promise};
}
async function fake(handler:import('node:http').RequestListener) { const server=createServer(handler);servers.push(server);await new Promise<void>(resolve=>server.listen(0,'127.0.0.1',resolve)); return `http://127.0.0.1:${(server.address() as import('node:net').AddressInfo).port}`; }

test('identity is verified before a write; roles are immutable metadata and receipts cannot loop',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture'),owner=bus('owner@fixture');
 const wrong=Bus.connect({addr:info.addr,token:info.tokens['#caller@fixture'],expectName:'#worker@fixture'});buses.push(wrong);
 await expect(wrong.send('#worker@fixture','bad')).rejects.toBeInstanceOf(IdentityMismatch);
 expect(await worker.consume({wait:0})).toBeNull();
 await expect(owner.send('#worker@fixture','bad',{roles:['owner']} as any)).rejects.toThrow('unsupported');
 const id=await owner.send('#worker@fixture','hi',{topic:'t',tag:'g'});const msg=(await worker.consume({wait:0}))!;
 expect(msg.message_id).toBe(id);expect(msg.roles).toEqual(['owner']);expect(Object.isFrozen(msg.envelope)).toBe(true);
 expect(()=>{(msg.roles as string[]).push('evil');}).toThrow(); await msg.ack();const ack=(await owner.consume({wait:0}))!;
 expect([ack.receipt,ack.re,ack.topic,ack.tag]).toEqual(['ack',id,'t','g']);await expect(ack.ack()).rejects.toThrow('non-receipt');
 expect((await caller.whoami()).you).toBe('#caller@fixture');expect((await owner.identity()).version).toBeTruthy();
});
test('a zero wait is a send without one',async()=>{
 const owner=bus('owner@fixture'),worker=bus('#worker@fixture');
 for(const wait of [0,'0s','0','0ms']) {
  await expect(owner.send('#worker@fixture','zero',{wait})).resolves.toEqual(expect.any(String));
  expect((await worker.consume({wait:0}))?.body).toBe('zero');
 }
 expect(await worker.consume({wait:0})).toBeNull();
});
test('private values obey authority and absence, secrets stay redacted, closed handles refuse reads',async()=>{
 const worker=bus('#worker@fixture'),caller=bus('#caller@fixture'); const rec=worker.record('#worker@fixture');
 expect(await rec.config()).toEqual({workers:1,label:'fixture'}); const secret=await rec.secret();expect(new TextDecoder().decode(secret.reveal())).toContain('dummy-fixture');
 expect(String(secret)).toBe('Secret[redacted]');expect(inspect(secret)).not.toContain('dummy');expect(JSON.stringify(secret)).not.toContain('dummy');
 const bytes=secret.reveal();bytes.fill(0);expect(new TextDecoder().decode(secret.reveal())).toContain('dummy');
 await expect(caller.record('#worker@fixture').config()).rejects.toMatchObject({status:403});expect(await caller.record('#caller@fixture').config()).toBeNull();await expect(caller.record('#caller@fixture').secret()).rejects.toMatchObject({status:404});
 await worker.close();await expect(rec.config()).rejects.toBeInstanceOf(BusClosed);expect(()=>worker.record('jobs@fixture')).toThrow(BusClosed);
});
test('KV preserves signed int64, binary strings, modes and atomic JSON operations',async()=>{
 const owner=bus('owner@fixture'),kv=owner.record('jobs@fixture').kv;
 await kv.set('int','big',9223372036854775806n,{mode:'set'});await expect(kv.get('int','big')).resolves.toEqual({kind:'int',value:9223372036854775806n});expect(await kv.increment('big')).toBe(9223372036854775807n);
 await expect(kv.increment('big')).rejects.toBeInstanceOf(BusError);await expect(kv.set('int','oops',9223372036854775808n,{mode:'set'})).rejects.toThrow('int64');
 await kv.set('string','bytes',new Uint8Array([0,255,1]),{mode:'set'});expect((await kv.get('string','bytes')).value).toEqual(new Uint8Array([0,255,1]));
 await kv.set('string','empty',new Uint8Array(),{mode:'set'});expect((await kv.get('string','empty')).value).toBe('');
 await expect(kv.set('string','empty','x',{mode:'add'})).rejects.toBeInstanceOf(BusError);await expect(kv.set('string','missing','x',{mode:'replace'})).rejects.toBeInstanceOf(BusError);
 await kv.set('json','state',{done:1,items:[]},{mode:'set'});await kv.jsonApply('state',[{op:'inc',key:'done',value:1},{op:'push',key:'items',value:'widget'}]);
 expect((await kv.get('json','state')).value).toEqual({done:2,items:['widget']});
 await expect(kv.jsonApply('state',[{op:'inc',key:'done',value:1},{op:'bad',key:'items'}])).rejects.toBeInstanceOf(BusError);expect((await kv.get('json','state')).value).toEqual({done:2,items:['widget']});
 expect((await kv.list()).int.map((item:{name:string})=>item.name)).toContain('big');expect(await kv.delete('int','big')).toBe(true);await expect(kv.get('int','big')).rejects.toBeInstanceOf(KVNotFound);
});
test('registry target handles and lock authority, waiting, force release and cancellation',async()=>{
 const owner=bus('owner@fixture'),worker=bus('#worker@fixture');await owner.register({name:'test@fixture',kind:'queue',allow:['*']});const rec=owner.record('test@fixture');expect((await rec.lookup()).name).toBe('test@fixture');
 await rec.manage({maintainers:['#worker@fixture']});await rec.locks.acquire('build',{ttl:'1s'});expect((await rec.locks.holders())[0]).toMatchObject({name:'build',holder:'owner@fixture'});
 await expect(rec.locks.tryAcquire('build',{ttl:'1s'})).rejects.toMatchObject({status:409});expect(await worker.record('test@fixture').locks.forceRelease('build')).toBe('owner@fixture');
 await rec.locks.acquire('build',{ttl:'1s'});const take=worker.record('test@fixture').locks.acquire('build',{ttl:'1s',wait:'1s'});await rec.locks.release('build');await take;await worker.record('test@fixture').locks.extend('build','2s');
 await worker.record('test@fixture').locks.release('build');await rec.locks.acquire('build',{ttl:'100ms'});const requested=barrier();
 const addr=await fake((req,res)=>{if(req.url==='/status')res.end('{"you":"#worker@fixture"}');else if(req.url?.startsWith('/lock'))requested.resolve();});
 const held=Bus.connect({addr,timeout:1000});buses.push(held);
 const controller=new AbortController();const abort=held.record('test@fixture').locks.acquire('build',{ttl:'100ms',wait:'1s',signal:controller.signal});await bounded(requested.promise,'held lock request');controller.abort();await expect(abort).rejects.toMatchObject({name:'AbortError'});await sleep(110);/* Real lease expiry, not asynchronous completion. */expect(await rec.locks.holders()).toEqual([]);await rec.unregister();await expect(rec.lookup()).rejects.toMatchObject({status:404});
});
test('parallel calls, capacity-bound nested calls and alternate reply inbox',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture'),helper=bus('#helper@fixture');let active=0,max=0;
 await serve(helper,async msg=>`helper:${msg.body}`);
 await serve(worker,async msg=>{active++;max=Math.max(max,active);try{expect(await worker.record('#worker@fixture').config()).toMatchObject({workers:1}); const result=await worker.call('#helper@fixture',msg.body,{timeout:1000});expect(result.kind).toBe('answer');return result.kind==='answer'?result.message.body:'';}finally{active--;}},{workers:1});
 const results=await Promise.all([1,2,3,4].map(n=>caller.call('#worker@fixture',String(n),{timeout:2500})));expect(results.map(r=>r.kind==='answer'?r.message.body:r.kind)).toEqual(['helper:1','helper:2','helper:3','helper:4']);expect(max).toBe(1);
 const outcome=await caller.call('#worker@fixture','alternate',{timeout:1500,reply_to:{name:'jobs@fixture',topic:'alternate'}});expect(outcome.kind).toBe('answer');if(outcome.kind==='answer')expect(outcome.message.body).toBe('helper:alternate');
 await expect(worker.consume({wait:0})).rejects.toBeInstanceOf(ReaderInUse);
});
test('empty answer differs from done, manual answer stays single, exceptions produce ack only',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');let errors=0;
 await serve(worker,async msg=>{if(msg.body==='done')return null;if(msg.body==='fail')throw new Error('SECRET-do-not-log');if(msg.body==='manual'){await msg.reply('manual');return 'duplicate';}return '';},{onError:(_id,type)=>{expect(type).toBe('Error');errors++;}});
 expect((await caller.call('#worker@fixture','empty',{timeout:500})).kind).toBe('answer');expect((await caller.call('#worker@fixture','done',{timeout:500})).kind).toBe('done');const manual=await caller.call('#worker@fixture','manual',{timeout:500});expect(manual.kind==='answer'&&manual.message.body).toBe('manual');
 const result=await caller.call('#worker@fixture','fail',{timeout:160});expect(result).toEqual({kind:'timeout',acked:true});expect(errors).toBe(1);await expect(caller.consume({wait:0})).resolves.toBeNull();
});
test('no prefetch at capacity; stopping drains the taken message and pending handler call',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture'),helper=bus('#helper@fixture');let entered!:()=>void,release!:()=>void;const began=new Promise<void>(r=>entered=r),gate=new Promise<void>(r=>release=r);let count=0;
 await serve(helper,async()=>{await gate;return 'nested';});const running=await serve(worker,async msg=>{count++;entered();const result=await worker.call('#helper@fixture',msg.body,{timeout:700});return result.kind==='answer'?result.message.body:null;},{workers:1});
 const first=caller.call('#worker@fixture','one',{timeout:1000});await began;const second=await caller.send('#worker@fixture','two');await sleep(70); // Deliberate observation window: no prefetch while capacity stays full.
 worker.stop();release();expect((await first).kind).toBe('answer');await running.promise;expect(count).toBe(1);expect((await worker.consume({wait:0}))?.message_id).toBe(second);
});
test('drain is bounded and abort cancels a call without retry',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture'),helper=bus('#helper@fixture');const replied=replyCompleted(worker);let release!:()=>void;const gate=new Promise<void>(r=>release=r);let entered!:()=>void;const began=new Promise<void>(r=>entered=r);
 const running=await serve(worker,async()=>{entered();await gate;return 'late';},{drainTimeout:30});await caller.send('#worker@fixture','hang');await began;worker.stop();await expect(running.promise).rejects.toBeInstanceOf(DrainTimeout);release();await replied();
 const controller=new AbortController();const call=caller.call('#helper@fixture','cancel',{timeout:1000,signal:controller.signal});expect(await helper.consume({wait:1000})).not.toBeNull();controller.abort();await expect(call).rejects.toMatchObject({name:'AbortError'});await expect(caller.consume({wait:0,topic:'unused'})).resolves.toBeNull();
});
test('wrong and absent receipt re cannot finish a call; late replies are not served as work',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');let handled=0;const reached=barrier();const servingCaller=await serve(caller,msg=>{if(msg.body==='ordering-barrier')reached.resolve();else handled++;return null;});
 // Complete an observed call before late duplicates; a cold deadline does not prove a send.
 const call=caller.call('#worker@fixture','x',{timeout:2000});const msg=await worker.consume({wait:1000});expect(msg).not.toBeNull();if(!msg)throw new Error('call request missing');
 await worker.request('POST','/send',{to:msg.from,topic:msg.topic,tag:msg.tag,receipt:'done',re:'wrong'});await worker.request('POST','/send',{to:msg.from,topic:msg.topic,tag:msg.tag,receipt:'done'});
 await worker.reply(msg,'real answer');const result=await call;expect(result.kind).toBe('answer');if(result.kind==='answer')expect(result.message.body).toBe('real answer');await worker.request('POST','/send',{to:msg.from,topic:msg.topic,tag:msg.tag,body:'late',re:msg.message_id});await worker.request('POST','/send',{to:msg.from,topic:msg.topic,tag:msg.tag,body:'legacy-late'});await worker.send('#caller@fixture','ordering-barrier');await bounded(reached.promise,'inbox ordering barrier');caller.stop();await servingCaller.promise;expect(handled).toBe(0);
});
test('Unix account identity and token override reach the real daemon',async()=>{
 const owner=Bus.connect({addr:join(scratch,'owner.sock'),token:'',expectName:'owner@fixture'}),worker=Bus.connect({addr:join(scratch,'owner.sock'),token:info.tokens['#worker@fixture'],expectName:'#worker@fixture'});buses.push(owner,worker);
 expect((await owner.whoami()).you).toBe('owner@fixture');expect((await worker.whoami()).you).toBe('#worker@fixture');await owner.send('#worker@fixture','unix');expect((await worker.consume({wait:0}))?.body).toBe('unix');
});
test('uncertain POST and destructive consume are surfaced, never replayed, budgets include send',async()=>{
 let posts=0,consumes=0;
 const addr=await fake((req,res)=>{if(req.url==='/status'){res.end('{"you":"#caller@fixture"}');return;}if(req.url==='/send'){posts++;req.socket.destroy();return;}if(req.url?.startsWith('/consume')){consumes++;req.socket.destroy();}});
 const b=Bus.connect({addr,token:'dummy',timeout:500});buses.push(b);await expect(b.send('#worker','x')).rejects.toBeInstanceOf(UncertainOutcome);expect(posts).toBe(1);await expect(b.consume({wait:0})).rejects.toBeInstanceOf(UncertainOutcome);expect(consumes).toBe(1);
 // Hold the POST open until the client ends it. A delayed success races the
 // timeout when a loaded event loop resumes with both timers overdue.
 const held=await fake((req,res)=>{if(req.url==='/status')res.end('{"you":"#caller@fixture"}');});
 const c=Bus.connect({addr:held,timeout:1000});buses.push(c);
 await c.whoami(); // This check exercises the send budget, not identity discovery.
 const started=performance.now();
 await expect(c.call('#worker','x',{timeout:50})).rejects.toBeInstanceOf(UncertainOutcome);
 expect(performance.now()-started).toBeLessThan(500);
 // An uncertain write does not promise the server received it by the deadline.
 // The dropped-connection checks above prove that writes are not replayed.
});
test('certificate pins are verified before any credentials; correct pin permits HTTPS',async()=>{
 const key=join(scratch,'key.pem'),cert=join(scratch,'cert.pem');const created=spawnSync('openssl',['req','-x509','-newkey','rsa:2048','-nodes','-days','1','-subj','/CN=localhost','-keyout',key,'-out',cert],{stdio:'ignore'});expect(created.status).toBe(0);
 let credentials=0;const server=createTLS({key:readFileSync(key),cert:readFileSync(cert)},(req,res)=>{if(req.headers['x-agent-bus-token'])credentials++;res.end('{"you":"#caller@fixture"}');});servers.push(server);await new Promise<void>(r=>server.listen(0,'127.0.0.1',r));const addr=`https://127.0.0.1:${(server.address() as import('node:net').AddressInfo).port}`;
 const wrong=Bus.connect({addr,token:'must-not-leak',fingerprint:'00'.repeat(32)});buses.push(wrong);await expect(wrong.whoami()).rejects.toBeInstanceOf(BusUnreachable);expect(credentials).toBe(0);
 const pin=createHash('sha256').update(new X509Certificate(readFileSync(cert)).raw).digest('hex');const right=Bus.connect({addr,token:'dummy',fingerprint:pin});buses.push(right);expect((await right.whoami()).you).toBe('#caller@fixture');expect(credentials).toBe(1);
});

test('a work poll completing after drain expiry is still handled exactly once',async()=>{
 let resolvePoll!:()=>void,polled!:()=>void;const begun=new Promise<void>(r=>polled=r),release=new Promise<void>(r=>resolvePoll=r);let polls=0,handled=0,replies=0;
 const addr=await fake((req,res)=>{if(req.url==='/status'){res.end('{"you":"#worker@fixture"}');return;}if(req.url?.startsWith('/consume')){polls++;polled();void release.then(()=>res.end('{"message_id":"taken","from":"#caller@fixture","to":"#worker@fixture","body":"work"}'));return;}if(req.url==='/send'){replies++;res.end('{"message_id":"response"}');}});
 const b=Bus.connect({addr,timeout:1000});buses.push(b);const replied=replyCompleted(b);const running=await serve(b,()=>{handled++;return 'answer';},{drainTimeout:10});await begun;b.stop();await expect(running.promise).rejects.toMatchObject({unfinished:1});resolvePoll();await replied();expect(handled).toBe(1);expect(replies).toBe(2);expect(polls).toBe(1);
});

test('call identity timeout is a timeout outcome, and closing wakes a pending call',async()=>{
 let sends=0;const addr=await fake((req,res)=>{if(req.url==='/status')return;else {sends++;res.end('{"message_id":"no"}');}});const b=Bus.connect({addr,timeout:500});buses.push(b);await expect(b.call('#worker','x',{timeout:25})).resolves.toEqual({kind:'timeout',acked:false});expect(sends).toBe(0);
 const caller=bus('#caller@fixture'),helper=bus('#helper@fixture');const waiting=caller.call('#helper@fixture','close',{timeout:1000});expect(await helper.consume({wait:1000})).not.toBeNull();await caller.close();await expect(waiting).rejects.toBeInstanceOf(BusClosed);
});

test('KV string bytes preserve a leading UTF8 BOM exactly',async()=>{
 const kv=bus('owner@fixture').record('jobs@fixture').kv;
 const original=new Uint8Array([239,187,191,65]);
 await kv.set('string','bom',original,{mode:'set'});
 const stored=await kv.get('string','bom');
 expect(stored.kind).toBe('string');
 const bytes=typeof stored.value==='string'?new TextEncoder().encode(stored.value):stored.value;
 expect(bytes).toEqual(original);
});

test('serve skips an expired caller deadline before ack and handler, and accepts an unset deadline',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');
 const expired=await caller.send('#worker@fixture','expired',{wait:'1ms',topic:'deadlines'});
 await sleep(15); // Intentionally cross the message deadline before starting service.
 const seen:string[]=[];let finish!:()=>void;const received=new Promise<void>(r=>finish=r);
 const running=await serve(worker,msg=>{seen.push(msg.body);finish();return 'answer';});
 await caller.send('#worker@fixture','unset',{topic:'deadlines'});
 await received;worker.stop();await running.promise;
 expect(seen).toEqual(['unset']);
 const responses:InstanceType<typeof library.Message>[]=[];for(let message; (message=await caller.consume({wait:0}));)responses.push(message);
 expect(responses.some(message=>message.re===expired)).toBe(false);
 expect(responses.filter(message=>message.receipt==='ack')).toHaveLength(1);
});
test('serve topic filtering leaves unrelated requests in the daemon inbox',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');
 const unrelated=await caller.send('#worker@fixture','other',{topic:'other'});
 const seen:string[]=[];let finish!:()=>void;const received=new Promise<void>(r=>finish=r);
 const running=await serve(worker,msg=>{seen.push(msg.body);finish();return 'answer';},{topic:'selected'});
 await caller.send('#worker@fixture','selected',{topic:'selected'});
 await received;worker.stop();await running.promise;
 expect(seen).toEqual(['selected']);
 expect((await worker.consume({wait:0}))?.message_id).toBe(unrelated);
});
test('an alternate reply inbox carries the call topic, which a replier would otherwise read as none',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');
 const pending=caller.call('#worker@fixture','q',{timeout:2000,topic:'build',reply_to:{name:'jobs@fixture'}});
 const request=await worker.consume({wait:1000});expect(request?.reply_to?.topic).toBe('build');
 const route=request!.reply_to!;await worker.send(route.name!,'a',{topic:route.topic ?? '',tag:route.tag!});
 const outcome=await pending;expect(outcome.kind==='answer'&&[outcome.message.body,outcome.message.topic]).toEqual(['a','build']);
});
test('a consume or lock wait is added to the request budget, so an empty inbox is not an uncertain outcome',async()=>{
 const short=(name:string)=>{const b=Bus.connect({addr:info.addr,token:info.tokens[name],expectName:name,timeout:500});buses.push(b);return b;};
 const owner=bus('owner@fixture'),reader=short('owner@fixture'),waiter=short('#worker@fixture');
 const started=performance.now();expect(await reader.consume({wait:1000})).toBeNull();expect(performance.now()-started).toBeGreaterThanOrEqual(900);
 await owner.register({name:'budget@fixture',kind:'queue',allow:['*']});const rec=owner.record('budget@fixture');await rec.manage({maintainers:['#worker@fixture']});
 await rec.locks.acquire('budget',{ttl:'5s'});setTimeout(()=>{rec.locks.release('budget').catch(()=>{});},800);
 await waiter.record('budget@fixture').locks.acquire('budget',{ttl:'1s',wait:'2s'});await waiter.record('budget@fixture').locks.release('budget');
});
test('an ask travels as the field, with no body, and a call can make one',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');
 await serve(worker,async msg=>`ask=${msg.ask ?? ''} body=${msg.body}`);
 const asked=await caller.call('#worker@fixture','',{timeout:1000,ask:'help'});expect(asked.kind==='answer'&&asked.message.body).toBe('ask=help body=');
 const plain=await caller.call('#worker@fixture','work',{timeout:1000});expect(plain.kind==='answer'&&plain.message.body).toBe('ask= body=work');
 await expect(caller.send('#worker@fixture','text',{ask:'help'})).rejects.toMatchObject({status:400});
});

test('opt-in onReply receives a real reply without invoking or answering a request handler',async()=>{
 const worker=bus('#worker@fixture'),caller=bus('#caller@fixture');
 const id=await worker.send('#caller@fixture','routed',{topic:'telegram-router',tag:'fixed-route',reply_to:{name:'#worker@fixture',topic:'telegram-router',tag:'fixed-route'}});
 const request=(await caller.consume({wait:1000}))!;
 const received=barrier();let requests=0;let answer:InstanceType<typeof library.Message>|undefined;
 await serve(worker,()=>{requests++;return 'wrong handler';},{onReply:message=>{answer=message;received.resolve();}});
 await caller.reply(request,'session answer');
 await Promise.race([received.promise,sleep(2000)]);
 expect(answer?.re).toBe(id);expect(answer?.from).toBe('#caller@fixture');expect(answer?.body).toBe('session answer');expect(answer?.topic).toBe('telegram-router');expect(answer?.tag).toBe('fixed-route');
 expect(requests).toBe(0);expect(await caller.consume({wait:50})).toBeNull();
});

test('opt-in matchesReply selects a non-re correlation and leaves ordinary requests unchanged',async()=>{
 const worker=bus('#worker@fixture'),caller=bus('#caller@fixture');
 const received=barrier(),handled=barrier();let replies=0,requests=0;
 await serve(worker,()=>{requests++;handled.resolve();return 'request answer';},{matchesReply:async m=>m.topic==='router'&&m.tag==='fixed',onReply:m=>{replies++;expect(m.re).toBeUndefined();received.resolve();}});
 await caller.send('#worker@fixture','MCP answer',{topic:'router',tag:'fixed'});
 await Promise.race([received.promise,sleep(2000)]);expect(replies).toBe(1);expect(requests).toBe(0);
 expect(await caller.consume({wait:50})).toBeNull();
 const done=replyCompleted(worker);
 await caller.send('#worker@fixture','ordinary',{topic:'other',tag:'tag'});
 await bounded(handled.promise,'ordinary request');await done();expect(requests).toBe(1);expect(replies).toBe(1);
});

test('client bearer credentials never appear in diagnostic printing',()=>{
 const client=Bus.connect({addr:info.addr,token:'client-token-sentinel'});buses.push(client);
 for(const output of [inspect(client,{depth:10}),JSON.stringify(client),String(client)])expect(output).not.toContain('client-token-sentinel');
});

test('a stalled reply classifier does not block a pending call answer',async()=>{
 const worker=bus('#worker@fixture'),caller=bus('#caller@fixture');
 const started=barrier(),release=barrier();
 await serve(worker,()=>null,{matchesReply:async()=>{started.resolve();await release.promise;return true;},onReply:()=>{}});
 await caller.send('#worker@fixture','classify');await bounded(started.promise,'reply classifier start');
 try {
  const pending=worker.call('#caller@fixture','request',{timeout:300});
  const request=await caller.consume({wait:1000});expect(request).not.toBeNull();await caller.reply(request!,'answer');
  expect((await pending).kind).toBe('answer');
 } finally {release.resolve();}
});

test('a throwing reply classifier reports onError and leaves serving active',async()=>{
 const worker=bus('#worker@fixture'),caller=bus('#caller@fixture');const errored=barrier();let errors=0;
 await serve(worker,()=> 'healthy',{matchesReply:async message=>{if(message.body==='bad')throw TypeError('classifier failed');return false;},onReply:()=>{},onError:(_id,type)=>{expect(type).toBe('TypeError');errors++;errored.resolve();}});
 await caller.send('#worker@fixture','bad');await bounded(errored.promise,'classifier onError');
 expect(errors).toBe(1);const result=await caller.call('#worker@fixture','good',{timeout:1000});expect(result.kind).toBe('answer');
});

test('maySend asks for another name and never reads an error or a foreign answer as no',async()=>{
 const owner=bus('owner@fixture'),worker=bus('#worker@fixture');
 await owner.request('POST','/register',{name:'#closed@fixture',kind:'agent',allow:['#caller@fixture']});
 expect(await owner.maySend('#caller@fixture','#closed@fixture')).toBe(true);
 expect(await owner.maySend('#worker@fixture','#closed@fixture')).toBe(false);
 expect(await owner.maySend('#nobody@fixture','#closed@fixture')).toBe(false);
 await expect(worker.maySend('#caller@fixture','#closed@fixture')).rejects.toMatchObject({status:404});
 await expect(owner.maySend('#caller@fixture','#absent@fixture')).rejects.toMatchObject({status:404});
 await expect(owner.maySend('','#closed@fixture')).rejects.toThrow('maySend');
 expect(typeof await owner.maySend('#caller','#closed@fixture')).toBe('boolean');
 expect(await owner.maySend('#CALLER@Fixture','#closed@fixture')).toBe(true);
 const answers=['{"may_send":true}','{"may_send":true,"who":"#other@fixture"}','{"who":"#worker@fixture"}','{"may_send":"yes","who":"#worker@fixture"}','not json'];
 const addr=await fake((req,res)=>{if(req.url==='/status')res.end('{"you":"#worker@fixture"}');else res.end(answers.shift());});
 const stub=Bus.connect({addr,token:'dummy',expectName:'#worker@fixture',timeout:1000});buses.push(stub);
 for(let i=0;i<4;i++) await expect(stub.maySend('#worker@fixture','#t@fixture')).rejects.toBeInstanceOf(UnexpectedAnswer);
 await expect(stub.maySend('#worker@fixture','#t@fixture')).rejects.toBeDefined();
});

// RT8 uses the same concrete protocol examples as the other native adapters.
// The override supports a dependency branch before the shared cases land.
type SharedCase = {
 id:string;
 request:{to:string;body:string;topic?:string;tag?:string;ask?:import('../src/bus.js').SendOptions['ask']};
 answer?:string;
 expected:{[key:string]:string};
};
function conformanceCases(file:string): SharedCase[] {
 const directory=process.env.AB_CLIENT_CONFORMANCE_CASES ?? join(root,'src/testdata/client-conformance');
 const rows=JSON.parse(readFileSync(join(directory,`${file}.json`),'utf8')) as SharedCase[];
 expect(rows.length).toBeGreaterThan(0);
 return rows;
}
function expectEnvelope(message:InstanceType<typeof library.Message>,expected:SharedCase['expected']) {
 const fields:{[key:string]:unknown}={...message.envelope,from:message.from,body:message.body,topic:message.topic ?? '',tag:message.tag ?? '',ask:message.ask ?? ''};
 for(const [key,value] of Object.entries(expected)) expect(fields[key]).toBe(value);
}

test('RT8 shared send cases',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');
 for(const row of conformanceCases('send')) {
  const {to,body,...options}=row.request;
  const sent=await caller.send(to,body,options);
  const message=await worker.consume({wait:0});
  expect(message).not.toBeNull();
  expectEnvelope(message!,row.expected);
  expect(message!.message_id).toBe(sent);
 }
});

test('RT8 shared call cases',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');
 const rows=conformanceCases('call'),handled:string[]=[],ids=new Map<string,string>();
 await serve(worker,message=>{
  const row=rows.find(row=>row.request.body===message.body);
  expect(row).toBeDefined();
  handled.push(message.body);ids.set(message.body,message.message_id);
  return row!.answer!;
 });
 for(const row of rows) {
  const {to,body,...options}=row.request;
  const outcome=await caller.call(to,body,{...options,timeout:2000});
  expect(outcome.kind).toBe('answer');
  if(outcome.kind==='answer') {
   expectEnvelope(outcome.message,row.expected);
   expect(outcome.message.re).toBe(ids.get(body));
  }
 }
 expect(handled).toEqual(rows.map(row=>row.request.body));
});

test('RT21 shared self-call gets its handler answer',async()=>{
 const caller=bus('#caller@fixture');
 const rows=conformanceCases('self-call'),handled:string[]=[],ids=new Map<string,string>();
 await serve(caller,message=>{
  const row=rows.find(row=>row.request.body===message.body);
  expect(row).toBeDefined();
  handled.push(message.body);ids.set(message.body,message.message_id);
  return row!.answer!;
 });
 for(const row of rows) {
  const {to,body,...options}=row.request;
  const outcome=await caller.call(to,body,{...options,timeout:2000});
  expect(outcome.kind).toBe('answer');
  if(outcome.kind==='answer') {
   expectEnvelope(outcome.message,row.expected);
   expect(outcome.message.re).toBe(ids.get(body));
  }
 }
 expect(handled).toEqual(rows.map(row=>row.request.body));
});

function pendingCalls(b:InstanceType<typeof Bus>): Array<{request?:InstanceType<typeof library.Message>;early:unknown[]}> {
 return [...(b as any).pending.values()];
}

test('RT21 early self request before the send response is still work',async()=>{
 const caller=bus('#caller@fixture'),row=conformanceCases('self-call')[0]!;
 const original=(caller as any).request.bind(caller);
 (caller as any).request=async(method:string,path:string,body:any,options:any)=>{
  const result=await original(method,path,body,options);
  if(path==='/send'&&body?.body===row.request.body) {
   const until=performance.now()+1000;
   while(!pendingCalls(caller).some(entry=>entry.early.length)&&performance.now()<until) await sleep(5);
   expect(pendingCalls(caller).some(entry=>entry.early.length)).toBe(true);
  }
  return result;
 };
 const ids:string[]=[];
 await serve(caller,message=>{ids.push(message.message_id);return row.answer!;});
 const {to,body,...options}=row.request;
 const outcome=await caller.call(to,body,{...options,timeout:2000});
 expect(outcome.kind).toBe('answer');
 if(outcome.kind==='answer') {
  expectEnvelope(outcome.message,row.expected);
  expect(outcome.message.re).toBe(ids[0]);
 }
 expect(ids).toHaveLength(1);
});

test('RT21 self call supports an alternate reply inbox',async()=>{
 const caller=bus('#caller@fixture'),row=conformanceCases('self-call')[0]!,ids:string[]=[];
 await serve(caller,message=>{ids.push(message.message_id);return row.answer!;});
 const {to,body,...options}=row.request;
 const outcome=await caller.call(to,body,{...options,timeout:2000,reply_to:{name:'jobs@fixture',topic:'self-return'}});
 expect(outcome.kind).toBe('answer');
 if(outcome.kind==='answer') {
  expectEnvelope(outcome.message,{...row.expected,to:'jobs@fixture',topic:'self-return'});
  expect(outcome.message.re).toBe(ids[0]);
 }
 expect(ids).toHaveLength(1);
});

test('RT21 retained self work respects capacity and drains after stop',async()=>{
 const caller=bus('#caller@fixture'),owner=bus('owner@fixture'),row=conformanceCases('self-call')[0]!;
 const entered=barrier(),release=barrier(),progressed=barrier(),readRelease=barrier();
 const seen:string[]=[];
 const running=await serve(caller,async message=>{
  seen.push(message.body);
  if(message.body==='hold slot'){entered.resolve();await release.promise;return 'released';}
  return row.answer!;
 },{workers:1,drainTimeout:4000});
 await owner.send('#caller@fixture','hold slot');await bounded(entered.promise,'occupied worker');
 const original=(caller as any).read.bind(caller);
 let activeAtProgress=0,retainedAtProgress=false;
 (caller as any).read=async(options:any)=>{
  const active=(caller as any).serving?.active.size ?? 0;
  const retained=pendingCalls(caller).some(entry=>entry.request);
  if(retained||active>1){activeAtProgress=active;retainedAtProgress=retained;progressed.resolve();await readRelease.promise;}
  return original(options);
 };
 const {to,body,...options}=row.request;
 const result=caller.call(to,body,{...options,timeout:3000});result.catch(()=>{});
 try {
  await bounded(progressed.promise,'self request routed while full');
  expect(activeAtProgress).toBe(1);
  expect(retainedAtProgress).toBe(true);
  expect(seen).toEqual(['hold slot']);
  const server=(caller as any).serving;
  caller.stop();release.resolve();
  const until=performance.now()+1000;
  while(server.active.size&&performance.now()<until)await sleep(1);
  expect(server.active.size).toBe(0);
  // Keep the dispatch read blocked through a drain-loop turn. Only the
  // already-taken self request can keep the serving generation alive now.
  await sleep(20);
  expect((caller as any).serving).toBe(server);
 } finally {release.resolve();readRelease.resolve();}
 const outcome=await result;
 expect(outcome.kind).toBe('answer');
 if(outcome.kind==='answer')expectEnvelope(outcome.message,row.expected);
 await running.promise;
 expect(seen).toEqual(['hold slot',row.request.body]);
});
