// Synthetic provider + bus double. No Discord or host-bus connection is made.
import { BusError, KVNotFound, type Bus } from '../../clients/typescript/src/index.ts';
import { Api } from './api.ts';
import { App, type CallerMessage } from './app.ts';
export const caller: CallerMessage={from:'#assistant@fixture',roles:[],message_id:'request-1',topic:'work',tag:'t'};
export function fixture(config:any={guilds:['1'],default:'alerts',users:{'7':'ada@fixture'},channels:{alerts:{id:'10'}}}) {
  const requests:{method:string;path:string;body:any;auth:string|null}[]=[],sent:any[]=[],warnings:string[]=[],values=new Map<string,any>();
  const state={admit:true,refuse:false,unknown:false,failPath:'',failStatus:0,failOnce:false};let next=1000;
  const server=Bun.serve({hostname:'127.0.0.1',port:0,async fetch(r){
    const u=new URL(r.url),path=u.pathname,body=r.method==='GET'||r.method==='DELETE'?undefined:await r.json().catch(()=>undefined);
    requests.push({method:r.method,path:u.pathname+u.search,body,auth:r.headers.get('authorization')});
    if(state.failPath===path&&state.failStatus){const status=state.failStatus;if(state.failOnce)state.failStatus=0;return Response.json({retry_after:.01,message:'fake-secret-do-not-print'},{status});}
    if(path==='/users/@me')return Response.json({id:'9',username:'fixture-bot'});
    if(path==='/users/@me/guilds')return Response.json([{id:'1',name:'allowed'},{id:'2',name:'outside'}]);
    if(/^\/guilds\/[12]\/channels$/.test(path))return Response.json([{id:'10',name:'alerts',type:0,guild_id:'1'},{id:'11',name:'other',type:0,guild_id:'1'}]);
    if(/^\/channels\/(10|11|20|30)$/.test(path)){const id=path.split('/').at(-1);return Response.json({id,name:'alerts',type:id==='30'?1:0,...(id==='30'?{}:{guild_id:id==='20'?'2':'1'}),topic:'fixture'});}
    if(path.endsWith('/messages')&&r.method==='GET')return Response.json([{id:'102',content:'new',author:{id:'7'}},{id:'101',content:'old',author:{id:'7'}}]);
    if(path.endsWith('/messages')&&r.method==='POST')return Response.json({id:String(next++),channel_id:path.split('/')[2]});
    if(/\/messages\/[0-9]+$/.test(path)&&r.method==='GET')return Response.json({id:path.split('/').at(-1),author:{id:path.endsWith('/101')?'7':'9'}});
    if(path.endsWith('/threads')&&r.method==='POST')return Response.json({id:'50',name:body?.name});
    if(r.method==='PATCH')return Response.json({id:path.split('/').at(-1)});
    if(r.method==='DELETE'||r.method==='PUT')return new Response(null,{status:204});
    return Response.json({message:'unknown fixture path'},{status:404});
  }});
  const kv={
    async get(kind:string,key:string){if(!values.has(kind+':'+key))throw new KVNotFound('not found');return{value:structuredClone(values.get(kind+':'+key))};},
    async set(kind:string,key:string,value:any,options:any){if(options.mode==='add'&&values.has(kind+':'+key))throw new BusError(412,'exists');values.set(kind+':'+key,structuredClone(value));},
    async list(){return{json:[...values.keys()].filter(k=>k.startsWith('json:')).map(k=>({name:k.slice(5)}))};},
    async delete(kind:string,key:string){return values.delete(kind+':'+key);},
    async jsonApply(key:string,ops:any[]){const v=values.get('json:'+key);if(!v)throw new KVNotFound('gone');for(const op of ops)v[op.key]=op.value;},
  };
  const bus={record:()=>({kv}),async request(method:string,path:string,body:any){if(path==='/lookup')return{kind:'agent'};if(path==='/send'){sent.push(body);return{message_id:'bus-'+sent.length};}throw Error('unexpected bus operation');},
    async maySend(){return state.admit;},async send(to:string,body:string,options:any){if(state.refuse)throw new BusError(403,'refused');if(state.unknown)throw Error('unknown');sent.push({to,body,...options,options});return'sent';}} as unknown as Bus;
  const api=new Api(async()=>({DISCORD_BOT_TOKEN:'fake-secret-do-not-print',DISCORD_API_URL:`http://127.0.0.1:${server.port}`}));
  const app=new App(bus,'#discord/fixture@fixture',api,async()=>config,s=>warnings.push(s));
  return{app,api,bus,server,requests,sent,warnings,values,state,config,async close(){await app.close();server.stop(true);}};
}
