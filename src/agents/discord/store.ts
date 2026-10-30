// Only transport/KV adaptation; the shared engine owns subscription policy.
import { Bus, BusError, KVNotFound } from '../../clients/typescript/src/index.ts';
import { type Port, type Row, SubscriptionError } from '../shared/subscriptions.ts';
export class Store implements Port {
  deadline = Infinity;
  constructor(readonly bus: Bus, readonly source: string) {}
  op() { const timeout = Math.min(1000,this.deadline-Date.now()); if(timeout<=0)throw new SubscriptionError('admission_unavailable');return {timeout}; }
  async admits(who:string,target:string) {try{return await this.bus.maySend(who,target,this.op())?'yes' as const:'no' as const;}catch(e){return e instanceof BusError&&e.status===404?'no' as const:'unavailable' as const;}}
  async kind(target:string) {try{return(await this.bus.request('GET','/lookup',undefined,{...this.op(),query:{name:target}})).kind;}catch{return undefined;}}
  async load(){const kv=this.bus.record(this.source).kv, listing=await kv.list(this.op()),rows=new Map<string,unknown>();for(const item of listing.json??[]){if(!item.name.startsWith('sub.'))continue;try{rows.set(item.name,(await kv.get('json',item.name,this.op())).value);}catch(e){if(!(e instanceof KVNotFound))throw e;}}return rows;}
  async create(key:string,row:Row){try{await this.bus.record(this.source).kv.set('json',key,row as any,{...this.op(),mode:'add'});return true;}catch(e){if(e instanceof BusError&&e.status===412)return false;throw e;}}
  remove(key:string){return this.bus.record(this.source).kv.delete('json',key,this.op());}
  async counts(key:string,sent:number,dropped:number){const kv=this.bus.record(this.source).kv;await kv.get('json',key,this.op());await kv.jsonApply(key,[{op:'set',key:'sent',value:sent},{op:'set',key:'dropped',value:dropped}],this.op());}
  async send(target:string,body:string,options:{topic?:string;tag?:string;deadline?:number}){try{await this.bus.send(target,body,{...this.op(),topic:options.topic,tag:options.tag});return 'ok' as const;}catch(e){return e instanceof BusError&&e.status>=400&&e.status<500?'refused' as const:'unknown' as const;}}
}
