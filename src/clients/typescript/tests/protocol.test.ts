import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {join} from 'node:path';
import {createServer} from 'node:http';
import {Bus} from '../src/bus.js';
import {KVNotFound} from '../src/record.js';
import {BusError} from '../src/transport.js';
async function fixture(identity:unknown,run:(b:Bus,calls:string[])=>Promise<void>,refusal?:[number,string|undefined]) {
 const calls:string[]=[];
 const server=createServer((req,res)=>{calls.push(req.url!);let answer:unknown={you:'owner@test'};
 if(req.url==='/identity')answer=identity;
 if((req.url==='/ls'||req.url?.startsWith('/kv?'))&&refusal){res.statusCode=refusal[0];answer={error:'cause',kind:refusal[1]};}
 res.end(JSON.stringify(answer));});
 await new Promise<void>(r=>server.listen(0,'127.0.0.1',r));
 const b=new Bus({addr:`http://127.0.0.1:${(server.address() as any).port}`,token:'fixture',timeout:2000});
 try{await run(b,calls);}finally{await b.close();server.closeAllConnections();await new Promise<void>(r=>server.close(()=>r()));}
}
test('protocol handshake rejects too-new and malformed before any application request',async()=>{
 for(const [identity,error] of [[{protocol:2,protocol_min:2},'daemon too new'],[{protocol:1.5},'protocol'],[{protocol:-1},'protocol'],[{protocol:null},'protocol'],[{protocol:true},'protocol'],[{protocol:1,protocol_min:2},'protocol_min']] as const){
  await fixture(identity,async(b,calls)=>{await expect(b.status()).rejects.toThrow(error);expect(calls).toEqual(['/identity']);});
 }
});
test('protocol current and compatible are silent, missing and old warn once',async()=>{
 const original=console.warn,warnings:string[]=[];console.warn=(...a)=>{warnings.push(a.join(' '));};
 try{for(const [identity,old] of [[{protocol:1,protocol_min:1},false],[{protocol:2,protocol_min:1},false],[{},true],[{protocol:0,protocol_min:0},true]] as const){warnings.length=0;await fixture(identity,async(b,calls)=>{await b.status();await b.status();expect(calls[0]).toBe('/identity');expect(warnings.length).toBe(old?1:0);});}}finally{console.warn=original;}
});
test('protocol refusals expose kind, keep unknown separate, and name unavailable operation on old daemon',async()=>{
 for(const identity of [{},{protocol:1,protocol_min:1}])for(const [code,kind] of [[404,'unknown'],[404,'no_route'],[503,'unavailable'],[500,undefined]] as const){await fixture(identity,async b=>{
  try{await b.records();throw new Error('refusal accepted');}catch(e){expect(e).toBeInstanceOf(BusError);const err=e as BusError;expect(err.status).toBe(code);expect(err.kind).toBe(kind);expect(err.message).toBe(code===500?'internal daemon error':!('protocol'in identity)&&kind==='no_route'?'daemon too old for GET /ls':'cause');}
 },[code,kind]);}
});

test('protocol no_route is never converted to a missing KV value and unknown keeps its kind',async()=>{
 for(const kind of ['unknown','no_route'])await fixture({protocol:1,protocol_min:1},async b=>{
  try{await b.record('owner@test').kv.get('string','k');throw new Error('refused KV accepted');}catch(error){expect(error).toBeInstanceOf(BusError);expect((error as BusError).kind).toBe(kind);expect(error instanceof KVNotFound).toBe(kind==='unknown');}
 },[404,kind]);
});

// The shared identity and refusal vectors: a fake daemon answers the vector's raw bodies.
const vectors=(name:string):any[]=>{
 const directory=process.env.AB_CLIENT_CONFORMANCE_CASES??join(import.meta.dir,'../../../testdata/client-conformance');
 const rows=JSON.parse(readFileSync(join(directory,`${name}.json`),'utf8'));expect(rows.length).toBeGreaterThan(0);return rows;
};
async function rawDaemon(identity:string,run:(b:Bus,calls:string[])=>Promise<void>,refuse?:{status:number;body:string;allow?:string}) {
 const calls:string[]=[];
 const server=createServer((req,res)=>{calls.push(req.url!);
  if(req.url==='/identity'){res.end(identity);return;}
  if(refuse&&req.url!.split('?')[0]==='/ls'){res.statusCode=refuse.status;if(refuse.allow)res.setHeader('Allow',refuse.allow);res.end(refuse.body);return;}
  res.end('{"you":"owner@test"}');});
 await new Promise<void>(r=>server.listen(0,'127.0.0.1',r));
 const b=new Bus({addr:`http://127.0.0.1:${(server.address() as any).port}`,token:'fixture',timeout:2000});
 try{await run(b,calls);}finally{await b.close();server.closeAllConnections();await new Promise<void>(r=>server.close(()=>r()));}
}
test('shared identity vectors',async()=>{
 const original=console.warn;
 try{for(const row of vectors('identity')){
  const warnings:string[]=[];console.warn=(...a)=>{warnings.push(a.join(' '));};
  await rawDaemon(row.identity,async(b,calls)=>{
   if(row.expect.fatal){await expect(b.status(),row.id).rejects.toThrow(row.expect.fatal);expect(calls,row.id).toEqual(['/identity']);return;}
   await b.status();expect(calls[0],row.id).toBe('/identity');
   if(row.expect.warns)expect(warnings.join(' '),row.id).toContain(row.expect.warns);else expect(warnings,row.id).toEqual([]);
  });
 }}finally{console.warn=original;}
});
test('shared refusal vectors',async()=>{
 const original=console.warn;console.warn=()=>{};
 try{for(const row of vectors('refusals')){
  await rawDaemon(row.identity==='current'?'{"protocol":1,"protocol_min":1}':'{}',async(b,calls)=>{
   if(row.call){ // the client refuses the call itself: no daemon refusal, and the daemon never saw the path
    const locks=b.record('jobs@fixture').locks,ttl=row.call.ttl_s*1000;let failure:unknown;
    try{if(row.call.op==='acquire')await locks.acquire(row.call.name,{ttl,wait:row.call.wait_s*1000});else if(row.call.op==='try')await locks.tryAcquire(row.call.name,{ttl});else await locks.extend(row.call.name,ttl);}catch(e){failure=e;}
    expect(failure,row.id).toBeDefined();expect(failure,row.id).not.toBeInstanceOf(BusError);
    expect(calls.map(c=>c.split('?')[0]),row.id).not.toContain(row.expect.not_requested);return;
   }
   let error:unknown;try{await b.records();}catch(e){error=e;}
   expect(error,row.id).toBeInstanceOf(BusError);const refusal=error as BusError;
   expect([refusal.status,refusal.kind??'',refusal.message],row.id).toEqual([row.expect.status,row.expect.kind,row.expect.message]);
  },row.daemon);
 }}finally{console.warn=original;}
});
