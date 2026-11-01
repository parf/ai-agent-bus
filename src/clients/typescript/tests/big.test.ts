import {test,expect} from 'bun:test';
import {createServer} from 'node:http';
import {createHash} from 'node:crypto';
import {resolve,join} from 'node:path';
const {Bus}=await import(resolve(process.env.AB_TYPESCRIPT_CLIENT_PATH??join(import.meta.dir,'..'),'src/index.ts')) as typeof import('../src/index.js');
const sha=(b:Uint8Array)=>createHash('sha256').update(b).digest('hex');
async function fixture(run:(big:any,state:any)=>Promise<void>){
 const state={data:Buffer.from([0,255,1]),calls:[] as string[],bad:false,unavailable:false};
 const server=createServer(async(req,res)=>{
  state.calls.push(req.method+' '+req.url);
  expect(req.headers['x-agent-bus-token']).toBe('fixture-only');
  if(req.url==='/status'){res.end('{"you":"owner@fixture"}');return;}
  if(state.unavailable){res.statusCode=503;res.end('{"error":"disk path blocked"}');return;}
  if(req.url?.startsWith('/bigfile/list')){res.end('{"keys":[{"name":"z"},{"name":"a"}]}');return;}
  if(req.method==='POST'){res.end('{"deleted":true}');return;}
  if(req.method==='PUT'){
   expect(req.headers['content-type']).toBe('application/octet-stream');
   const parts:Buffer[]=[];for await(const part of req)parts.push(Buffer.from(part));state.data=Buffer.concat(parts);
   res.end(JSON.stringify({size:state.data.length,sha256:state.bad?'0'.repeat(64):sha(state.data),replaced:true}));return;
  }
  res.setHeader('Content-Length',state.data.length);res.setHeader('X-Bigfile-Sha256',state.bad?'0'.repeat(64):sha(state.data));res.setHeader('X-Bigfile-Held','memory');res.setHeader('X-Bigfile-Expires','2026-11-02T00:00:00Z');res.end(state.data);
 });
 await new Promise<void>(r=>server.listen(0,'127.0.0.1',r));
 const bus=Bus.connect({addr:`http://127.0.0.1:${(server.address() as any).port}`,token:'fixture-only'});
 try{const rec=bus.record('owner@fixture');expect('bigfile' in rec).toBe(true);await run((rec as any).bigfile,state);}
 finally{await bus.close();server.closeAllConnections();await new Promise<void>(r=>server.close(()=>r()));}
}
test('big streamed binary roundtrip verifies metadata, names and ttl',()=>fixture(async(big,s)=>{
 const data=Buffer.alloc(1048577,255);const stream=async function*(){yield data.subarray(0,100);yield data.subarray(100);};
 const info=await big.put('a/b',stream(),{size:data.length,ttl:'30d',memory:true,sha256:sha(data)});
 expect(info.replaced).toBe(true);const got=await big.get('a/b');expect(got.data).toEqual(data);expect(got.info.sha256).toBe(sha(data));expect(got.info.size).toBe(data.length);
 expect(s.calls[1]).toContain('ttl=2592000');expect(s.calls[1]).toContain('name=a%2Fb');expect(s.calls[1]).toContain('memory=true');
 expect((await big.list()).map((k:any)=>k.name)).toEqual(['a','z']);expect(await big.delete('a/b')).toBe(true);
}));
test('big invalid or empty input refuses before even the identity request',()=>fixture(async(big,s)=>{
 for(const [input,opts] of [[Buffer.alloc(0),{}],[Buffer.from('x'),{ttl:'31d'}],[Buffer.from('x'),{ttl:'0s'}],[Buffer.from('x'),{size:33554433}],[ (async function*(){})(),{size:1}]])await expect(big.put('n',input,opts)).rejects.toThrow();
 expect(s.calls).toEqual([]);
}));
test('big hash mismatches are errors and an upload is never retried',()=>fixture(async(big,s)=>{
 s.bad=true;await expect(big.get('n')).rejects.toThrow('SHA-256');await expect(big.put('n',Buffer.from('bytes'))).rejects.toThrow('SHA-256');expect(s.calls.filter((c:string)=>c.startsWith('PUT')).length).toBe(1);
}));
test('big accepts and verifies a complete 32 MiB download',()=>fixture(async(big,s)=>{
 s.data=Buffer.alloc(32*1024*1024,7);const got=await big.get('limit');expect(got.data.length).toBe(s.data.length);expect(got.info.sha256).toBe(sha(s.data));
}));

test('big unavailable responses retain status503 and the daemon cause',()=>fixture(async(big,s)=>{
 s.unavailable=true;
 for(const operation of [()=>big.put('n',Buffer.from('bytes')),()=>big.get('n'),()=>big.list(),()=>big.delete('n')]){
  try{await operation();throw new Error('503 accepted');}catch(error:any){expect(error.status).toBe(503);expect(error.message).toContain('disk path blocked');}
 }
}));

test('big received refusals win over concurrent large-upload write errors',async()=>{
 const http=(await import('node:http')).default;
 const {EventEmitter}=await import('node:events');const original=http.request;
 try{
  for(const status of [403,413,503]){
   let calls=0;
   http.request=((url:any,options:any,callback:any)=>{
    if(url.pathname!=='/bigfile')return original(url,options,callback);
    calls++;const request:any=new EventEmitter();request.destroy=()=>request;request.end=()=>{};request.flushHeaders=()=>queueMicrotask(()=>request.emit('continue'));
    request.write=()=>{queueMicrotask(()=>{
     const response:any=new EventEmitter();response.statusCode=status;response.headers={};callback(response);
     request.emit('error',Object.assign(new Error('peer stopped the large upload'),{code:'ECONNRESET'}));
     queueMicrotask(()=>{response.emit('data',Buffer.from('{"error":"early refusal cause"}'));response.emit('end');});
    });return true;};
    return request;
   }) as any;
   await fixture(async(big)=>{
    try{await big.put('n',Buffer.alloc(3*1024*1024));throw new Error('refused upload accepted');}
    catch(error:any){expect(error.status).toBe(status);expect(error.message).toContain('early refusal cause');}
   });
   expect(calls).toBe(1);
  }
 }finally{http.request=original;}
});

test('big large puts wait for Continue and retain early 403 413 503 causes',async()=>{
 for(const status of [403,413,503]){
  let expected=false,received=0,puts=0;
  const refuse=(req:import('node:http').IncomingMessage,res:import('node:http').ServerResponse)=>{
   puts++;expected=req.headers.expect==='100-continue';req.on('data',chunk=>{received+=chunk.length;});
   res.writeHead(status,{'Content-Type':'application/json','Connection':'close'});res.end('{"error":"early real refusal"}');
  };
  const server=createServer((req,res)=>{if(req.url==='/status'){res.end('{"you":"owner@fixture"}');return;}refuse(req,res);});
  server.on('checkContinue',refuse);
  await new Promise<void>(resolve=>server.listen(0,'127.0.0.1',resolve));
  const bus=Bus.connect({addr:`http://127.0.0.1:${(server.address() as any).port}`,token:'fixture-only'});
  try{
   try{await bus.record('owner@fixture').bigfile.put('n',Buffer.alloc(24*1024*1024));throw new Error('refusal accepted');}
   catch(error:any){expect(error.status).toBe(status);expect(error.message).toContain('early real refusal');}
   expect(expected).toBe(true);expect(received).toBe(0);expect(puts).toBe(1);
  }finally{await bus.close();server.closeAllConnections();await new Promise<void>(resolve=>server.close(()=>resolve()));}
 }
});
