import { test, expect, beforeEach, afterEach } from 'bun:test';
import { spawn, spawnSync, type ChildProcessWithoutNullStreams } from 'node:child_process';
import { mkdtempSync, rmSync, readFileSync } from 'node:fs';
import { resolve, join } from 'node:path';
import { createServer } from 'node:http';
import { createServer as createTLS } from 'node:https';
import { X509Certificate, createHash } from 'node:crypto';
import { inspect } from 'node:util';
const library = await import(resolve(process.env.AB_TYPESCRIPT_CLIENT_PATH ?? join(import.meta.dir,'..'),'src/index.ts')) as typeof import('../src/index.js');
const {Bus,BusError,BusClosed,IdentityMismatch,ReaderInUse,UncertainOutcome,BusUnreachable,DrainTimeout,KVNotFound} = library;
const root = resolve(import.meta.dir,'../../../..');
const fixture = process.env.AB_CLIENT_FIXTURE ?? join(root,'tmp/client-fixture');
const sleep = (ms:number) => new Promise<void>(resolve => setTimeout(resolve,ms));
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
  const promise=b.serve(handler,{drainTimeout:1000,...options}); serving.push(promise); promise.catch(()=>{}); await sleep(25); return {promise};
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
 await kv.set('int','big',9223372036854775806n,{mode:'set'});expect(await kv.get('int','big')).toEqual({kind:'int',value:9223372036854775806n});expect(await kv.increment('big')).toBe(9223372036854775807n);
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
 await rec.locks.acquire('build',{ttl:'1s'});const take=worker.record('test@fixture').locks.acquire('build',{ttl:'1s',wait:'1s'});await sleep(20);await rec.locks.release('build');await take;await worker.record('test@fixture').locks.extend('build','2s');
 await worker.record('test@fixture').locks.release('build');await rec.locks.acquire('build',{ttl:'100ms'});const controller=new AbortController();const abort=worker.record('test@fixture').locks.acquire('build',{ttl:'100ms',wait:'1s',signal:controller.signal});await sleep(10);controller.abort();await expect(abort).rejects.toMatchObject({name:'AbortError'});await sleep(110);expect(await rec.locks.holders()).toEqual([]);await rec.unregister();await expect(rec.lookup()).rejects.toMatchObject({status:404});
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
 const result=await caller.call('#worker@fixture','fail',{timeout:160});expect(result).toEqual({kind:'timeout',acked:true});expect(errors).toBe(1);expect(await caller.consume({wait:0})).toBeNull();
});
test('no prefetch at capacity; stopping drains the taken message and pending handler call',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture'),helper=bus('#helper@fixture');let entered!:()=>void,release!:()=>void;const began=new Promise<void>(r=>entered=r),gate=new Promise<void>(r=>release=r);let count=0;
 await serve(helper,async()=>{await gate;return 'nested';});const running=await serve(worker,async msg=>{count++;entered();const result=await worker.call('#helper@fixture',msg.body,{timeout:700});return result.kind==='answer'?result.message.body:null;},{workers:1});
 const first=caller.call('#worker@fixture','one',{timeout:1000});await began;const second=await caller.send('#worker@fixture','two');await sleep(70);worker.stop();release();expect((await first).kind).toBe('answer');await running.promise;expect(count).toBe(1);expect((await worker.consume({wait:0}))?.message_id).toBe(second);
});
test('drain is bounded and abort cancels a call without retry',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');let release!:()=>void;const gate=new Promise<void>(r=>release=r);let entered!:()=>void;const began=new Promise<void>(r=>entered=r);
 const running=await serve(worker,async()=>{entered();await gate;return 'late';},{drainTimeout:30});await caller.send('#worker@fixture','hang');await began;worker.stop();await expect(running.promise).rejects.toBeInstanceOf(DrainTimeout);release();await sleep(20);
 const controller=new AbortController();const call=caller.call('#helper@fixture','cancel',{timeout:1000,signal:controller.signal});await sleep(20);controller.abort();await expect(call).rejects.toMatchObject({name:'AbortError'});await expect(caller.consume({wait:0,topic:'unused'})).resolves.toBeNull();
});
test('wrong and absent receipt re cannot finish a call; late replies are not served as work',async()=>{
 const caller=bus('#caller@fixture'),worker=bus('#worker@fixture');let handled=0;await serve(caller,()=>{handled++;return null;});
 const call=caller.call('#worker@fixture','x',{timeout:170});const msg=(await worker.consume({wait:'100ms'}))!;
 await worker.request('POST','/send',{to:msg.from,topic:msg.topic,tag:msg.tag,receipt:'done',re:'wrong'});await worker.request('POST','/send',{to:msg.from,topic:msg.topic,tag:msg.tag,receipt:'done'});
 expect(await call).toEqual({kind:'timeout',acked:false});await worker.reply(msg,'late');await worker.request('POST','/send',{to:msg.from,topic:msg.topic,tag:msg.tag,body:'legacy-late'});await sleep(80);expect(handled).toBe(0);
});
test('Unix account identity and token override reach the real daemon',async()=>{
 const owner=Bus.connect({addr:join(scratch,'owner.sock'),token:'',expectName:'owner@fixture'}),worker=Bus.connect({addr:join(scratch,'owner.sock'),token:info.tokens['#worker@fixture'],expectName:'#worker@fixture'});buses.push(owner,worker);
 expect((await owner.whoami()).you).toBe('owner@fixture');expect((await worker.whoami()).you).toBe('#worker@fixture');await owner.send('#worker@fixture','unix');expect((await worker.consume({wait:0}))?.body).toBe('unix');
});
test('uncertain POST and destructive consume are surfaced, never replayed, budgets include send',async()=>{
 let posts=0,consumes=0;
 const addr=await fake((req,res)=>{if(req.url==='/status'){res.end('{"you":"#caller@fixture"}');return;}if(req.url==='/send'){posts++;req.socket.destroy();return;}if(req.url?.startsWith('/consume')){consumes++;req.socket.destroy();}});
 const b=Bus.connect({addr,token:'dummy',timeout:500});buses.push(b);await expect(b.send('#worker','x')).rejects.toBeInstanceOf(UncertainOutcome);expect(posts).toBe(1);await expect(b.consume({wait:0})).rejects.toBeInstanceOf(UncertainOutcome);expect(consumes).toBe(1);
 let sends=0;const slow=await fake((req,res)=>{if(req.url==='/status')res.end('{"you":"#caller@fixture"}');else if(req.url==='/send'){sends++;setTimeout(()=>res.end('{"message_id":"slow"}'),200);}});const c=Bus.connect({addr:slow,timeout:1000});buses.push(c);const started=performance.now();await expect(c.call('#worker','x',{timeout:50})).rejects.toBeInstanceOf(UncertainOutcome);expect(performance.now()-started).toBeLessThan(160);expect(sends).toBe(1);
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
 const b=Bus.connect({addr,timeout:1000});buses.push(b);const running=await serve(b,()=>{handled++;return 'answer';},{drainTimeout:10});await begun;b.stop();await expect(running.promise).rejects.toMatchObject({unfinished:1});resolvePoll();await sleep(30);expect(handled).toBe(1);expect(replies).toBe(2);expect(polls).toBe(1);
});

test('call identity timeout is a timeout outcome, and closing wakes a pending call',async()=>{
 let sends=0;const addr=await fake((req,res)=>{if(req.url==='/status')setTimeout(()=>res.end('{"you":"#caller@fixture"}'),100);else {sends++;res.end('{"message_id":"no"}');}});const b=Bus.connect({addr,timeout:500});buses.push(b);expect(await b.call('#worker','x',{timeout:25})).toEqual({kind:'timeout',acked:false});expect(sends).toBe(0);
 const caller=bus('#caller@fixture');const waiting=caller.call('#helper@fixture','close',{timeout:1000});await sleep(25);await caller.close();await expect(waiting).rejects.toBeInstanceOf(BusClosed);
});
