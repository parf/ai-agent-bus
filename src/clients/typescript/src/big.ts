import {createHash} from 'node:crypto';
import type {RecordHandle} from './record.js';
import type {Operation} from './transport.js';
export const MAX_BIG=32*1024*1024;
/** The daemon's bigfile body allowance: milliseconds of stall, and the bytes a second it promises to finish at. */
export const bigLimits={idle:30_000,rate:64*1024};
/** Whole time a bigfile put or get may take, in milliseconds: the stall allowance plus the slowest promised rate for size bytes. */
export const bigBudget=(size:number)=>bigLimits.idle+Math.min(Math.max(size,0),MAX_BIG)*1000/bigLimits.rate;
export class BigfileHashMismatch extends Error {constructor(message='bigfile size or SHA-256 mismatch; discard the download'){super(message);this.name='BigfileHashMismatch';}}
export interface BigfileKey {name:string;size:number;sha256:string;held:'file'|'memory';expires:string}
export interface BigfileInfo extends BigfileKey {record:string;replaced?:boolean}
export interface BigfilePutOptions extends Operation {size?:number;ttl?:string|number;memory?:boolean;sha256?:string}
/** Numeric durations, as elsewhere in the TS client, are milliseconds. */
export function bigTTL(value:string|number='24h'):number {
 let seconds=0;
 if(typeof value==='number')seconds=value/1000;
 else {let rest=value;while(rest){const m=/^(\d+)([dhms])/.exec(rest);if(!m)throw new TypeError('ttl must use whole s/m/h/d');seconds+=Number(m[1])*({d:86400,h:3600,m:60,s:1}[m[2]!]!);rest=rest.slice(m[0].length);}}
 if(!Number.isSafeInteger(seconds)||seconds<1||seconds>2592000)throw new TypeError('ttl must be whole seconds in 1s..30d');return seconds;
}
function key(name:string){if(typeof name!=='string'||Buffer.byteLength(name)<1||Buffer.byteLength(name)>256)throw new TypeError('bigfile name must be 1..256 UTF-8 bytes');}
export class RecordBigfile {
 constructor(private record:RecordHandle){}
 async put(name:string,source:Uint8Array|AsyncIterable<Uint8Array>,options:BigfilePutOptions={}):Promise<BigfileInfo>{
  key(name);const ttl=bigTTL(options.ttl);const memory=options.memory??false;
  if(typeof memory!=='boolean')throw new TypeError('memory must be boolean');
  if(options.sha256!==undefined&&!/^[a-fA-F0-9]{64}$/.test(options.sha256))throw new TypeError('sha256 must be 64 hex characters');
  const size=options.size??(source instanceof Uint8Array?source.length:undefined);
  if(!Number.isSafeInteger(size)||size!<1||size!>MAX_BIG)throw new TypeError('bigfile input size must be 1..33554432 bytes');
  if(source instanceof Uint8Array&&size!==source.length)throw new TypeError('size differs from input bytes');
  const iterator=(source instanceof Uint8Array?(async function*(){yield source;})():source)[Symbol.asyncIterator]();
  let first=await iterator.next();while(!first.done&&first.value.length===0)first=await iterator.next();
  if(!first.done&&(!(first.value instanceof Uint8Array)||first.value.length>size!))throw new TypeError('size differs from input bytes');
  if(first.done)throw new TypeError('bigfile input must not be empty');
  const digest=createHash('sha256');
  const chunks=async function*(){let step:IteratorResult<Uint8Array>=first,total=0;try{while(!step.done){const part=step.value;if(!(part instanceof Uint8Array)||total+part.length>size!)throw new TypeError('size differs from input bytes');total+=part.length;digest.update(part);yield part;step=await iterator.next();}if(total!==size)throw new TypeError('size differs from input bytes');}finally{await iterator.return?.();}};
  const headers:Record<string,string>={};if(options.sha256)headers['X-Bigfile-Sha256']=options.sha256.toLowerCase();
  const result=await this.record.bus.request('PUT','/bigfile',undefined,{...options,timeout:options.timeout??bigBudget(size!),query:{record:this.record.name,name,ttl,memory},headers,upload:{source:chunks(),size:size!}});
  if(result.sha256!==digest.digest('hex')||result.size!==size)throw new BigfileHashMismatch('bigfile put size or SHA-256 mismatch; upload may have completed');return result;
 }
 async get(name:string,options:Operation={}):Promise<{data:Buffer;info:BigfileInfo}>{
  key(name);const {data,headers}=await this.record.bus.request('GET','/bigfile',undefined,{...options,timeout:options.timeout??bigBudget(MAX_BIG),query:{record:this.record.name,name},bigResponse:true});
  const sha256=createHash('sha256').update(data).digest('hex');
  if(!data.length||data.length>MAX_BIG||headers['content-length']!==String(data.length)||headers['x-bigfile-sha256']!==sha256)throw new BigfileHashMismatch();
  return {data,info:{record:this.record.name,name,size:data.length,sha256,held:headers['x-bigfile-held'],expires:headers['x-bigfile-expires']}};
 }
 async list(options:Operation={}):Promise<BigfileKey[]>{const answer=await this.record.bus.request('GET','/bigfile/list',undefined,{...options,query:{record:this.record.name}});return answer.keys.sort((a:BigfileKey,b:BigfileKey)=>a.name<b.name?-1:a.name>b.name?1:0);}
 async delete(name:string,options:Operation={}):Promise<boolean>{key(name);return(await this.record.bus.request('POST','/bigfile/delete',{record:this.record.name,name},options)).deleted;}
}
