import { Bus } from './bus.js';
import { Secret, type Json } from './message.js';
import { type Operation, duration, BusError } from './transport.js';
export type KVKind = 'string'|'int'|'json';
export type KVValue = {kind:'string';value:string|Uint8Array} | {kind:'int';value:bigint} | {kind:'json';value:{[key:string]:Json}};
export class KVNotFound extends BusError { constructor(message:string) { super(404,message); this.name = 'KVNotFound'; } }
const int64 = (n:bigint|number): bigint => { if (typeof n === 'number' && !Number.isSafeInteger(n)) throw new TypeError('use bigint for exact integers'); const value = BigInt(n); if (value < -(1n<<63n) || value >= (1n<<63n)) throw new TypeError('integer must fit int64'); return value; };
const kindCheck = (kind:KVKind) => { if (!['string','int','json'].includes(kind)) throw new TypeError('kind is string, int or json'); };
export class RecordHandle {
  readonly kv: RecordKV; readonly locks: RecordLocks;
  constructor(readonly bus:Bus,readonly name:string) { this.kv = new RecordKV(this); this.locks = new RecordLocks(this); }
  lookup(options:Operation = {}): Promise<any> { return this.bus.request('GET','/lookup',undefined,{ ...options,query:{name:this.name} }); }
  manage(changes:{[key:string]:unknown},options:Operation = {}): Promise<any> { if ('name' in changes) throw new TypeError('management cannot change target name'); return this.bus.request('POST','/manage',{name:this.name,...changes},options); }
  unregister(options:Operation = {}): Promise<any> { return this.bus.request('POST','/unregister',{name:this.name},options); }
  config(options:Operation = {}): Promise<Json> { return this.bus.request('GET','/config',undefined,{...options,query:{name:this.name}}); }
  async secret(options:Operation = {}): Promise<Secret> { return new Secret(await this.bus.request('GET','/secret',undefined,{...options,query:{name:this.name},raw:true})); }
}
export class RecordKV {
  constructor(private record:RecordHandle) {}
  private body(kind:KVKind,name:string): {[key:string]:unknown} { kindCheck(kind); return {record:this.record.name,name,kind}; }
  async get(kind:KVKind,name:string,options:Operation = {}): Promise<KVValue> {
    let answer:any; try { answer = await this.record.bus.request('GET','/kv',undefined,{...options,query:this.body(kind,name)}); } catch(e) { if (e instanceof BusError && e.status === 404) throw new KVNotFound(e.message); throw e; }
    if (kind === 'int') return {kind,value:int64(answer.value)};
    if (kind === 'string') return {kind,value:answer.value_base64 === undefined ? answer.value : Uint8Array.from(Buffer.from(answer.value_base64,'base64'))};
    return {kind,value:answer.value};
  }
  async set(kind:KVKind,name:string,value:string|Uint8Array|bigint|number|{[key:string]:Json},options:Operation & {mode:'set'|'add'|'replace'}): Promise<any> {
    if (!['set','add','replace'].includes(options.mode)) throw new TypeError('mode is set, add or replace');
    const body = {...this.body(kind,name),how:options.mode} as {[key:string]:unknown};
    if (kind === 'int') body.value = int64(value as bigint|number);
    else if (kind === 'string') { if (typeof value === 'string') body.value = value; else if (value instanceof Uint8Array) { try { body.value = new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(value); } catch { body.value_base64 = Buffer.from(value).toString('base64'); } } else throw new TypeError('string value must be text or bytes'); }
    else { if (!value || typeof value !== 'object' || Array.isArray(value) || value instanceof Uint8Array) throw new TypeError('JSON storage requires an object'); body.value = value; }
    return this.record.bus.request('POST','/kv/set',body,options);
  }
  async delete(kind:KVKind,name:string,options:Operation = {}): Promise<boolean> { return (await this.record.bus.request('POST','/kv/delete',this.body(kind,name),options)).deleted; }
  async increment(name:string,options:Operation & {by?:bigint|number} = {}): Promise<bigint> { return int64((await this.record.bus.request('POST','/kv/inc',{record:this.record.name,name,n:int64(options.by ?? 1)},options)).value); }
  jsonApply(name:string,ops:ReadonlyArray<{op:string;key:string;value?:Json}>,options:Operation = {}): Promise<any> { if (ops.length < 1 || ops.length > 100) throw new TypeError('1 to 100 JSON ops required'); return this.record.bus.request('POST','/kv/json',{record:this.record.name,name,ops},options); }
  async list(options:Operation = {}): Promise<any> { return (await this.record.bus.request('GET','/kv/list',undefined,{...options,query:{record:this.record.name}})).values; }
}
export class RecordLocks {
  constructor(private record:RecordHandle) {}
  private body(name:string): {[key:string]:string} { return {record:this.record.name,name}; }
  acquire(name:string,options:Operation & {ttl:string|number;wait?:string|number}): Promise<any> { return this.record.bus.request('POST','/lock',{...this.body(name),ttl:duration(options.ttl)},{...options,query:{wait:duration(options.wait ?? 0)}}); }
  tryAcquire(name:string,options:Operation & {ttl:string|number}): Promise<any> { return this.record.bus.request('POST','/try-lock',{...this.body(name),ttl:duration(options.ttl)},options); }
  extend(name:string,ttl:string|number,options:Operation = {}): Promise<any> { return this.record.bus.request('POST','/extend',{...this.body(name),ttl:duration(ttl)},options); }
  release(name:string,options:Operation = {}): Promise<any> { return this.record.bus.request('POST','/release',this.body(name),options); }
  async forceRelease(name:string,options:Operation = {}): Promise<string|undefined> { return (await this.record.bus.request('POST','/release-force',this.body(name),options)).holder; }
  async holders(options:Operation = {}): Promise<Array<{name:string;holder:string;expires:string}>> { const result = await this.record.bus.request('GET','/holders',undefined,{...options,query:{record:this.record.name}}); return Object.entries(result.locks).map(([name,value]) => ({name,...value as {holder:string;expires:string}})); }
}
