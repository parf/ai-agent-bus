import {test,expect} from 'bun:test';
import {createServer} from 'node:http';
import {Bus} from '../src/bus.js';
import {BusError} from '../src/transport.js';
async function fixture(identity:unknown,run:(b:Bus,calls:string[])=>Promise<void>,refusal?:[number,string|undefined]) {
 const calls:string[]=[];
 const server=createServer((req,res)=>{calls.push(req.url!);let answer:unknown={you:'owner@test'};
 if(req.url==='/identity')answer=identity;
 if(req.url==='/ls'&&refusal){res.statusCode=refusal[0];answer={error:'cause',kind:refusal[1]};}
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
