#!/usr/bin/env bun
import { Bus } from '../../clients/typescript/src/index.ts';
import { parseEnvFile } from '../slack/envfile.ts';
import { dispatchAsk, helpAnswer } from '../shared/metadata.ts';
import manifest from './agent.json';
import { readFileSync } from 'node:fs';
import { Api } from './api.ts';
import { App } from './app.ts';
import { Gateway } from './gateway.ts';
const name=process.env.AGENT_BUS_NAME?.trim().toLowerCase();
if(!name){console.error('discord: AGENT_BUS_NAME is required');process.exit(1);}
const bus=Bus.connect({expectName:name});
const envDist=readFileSync(new URL('./env.dist',import.meta.url),'utf8');
const api=new Api(async()=>parseEnvFile('record Secret',(await bus.record(name).secret()).reveal()));
const log=(line:string)=>console.error('discord: '+line);
const app=new App(bus,name,api,()=>bus.record(name).config(),log);
const gateway=new Gateway(api,(kind,data)=>{void app.event(kind,data).catch(()=>log('gateway event failed; it is not replayed'));},log);
try{await app.initialize();}catch(e){log(e instanceof Error?e.message:'Configuration or Secret unavailable');await bus.close();process.exit(78);}
const stop=new AbortController();
for(const signal of ['SIGTERM','SIGINT'] as const)process.on(signal,()=>{gateway.stop();api.close();stop.abort();});
await gateway.start();
await bus.serve(async e=>{
  const help=(ask:any)=>helpAnswer(manifest,ask,envDist,[...e.roles],false,name);
  const ask=dispatchAsk(e.ask,help,{status:()=>`Discord gateway ${gateway.armed?'connected or reconnecting':'stopped'}`});
  if(ask!==undefined)return ask;
  if(e.body.trim()==='help')return help('help');
  try{return JSON.stringify({ok:true,result:await app.call(e.body,{from:e.from,roles:e.roles,original_to:typeof e.original_to==='string'?e.original_to:undefined,message_id:e.message_id,topic:e.topic,tag:e.tag})});}
  catch(e){return 'error: '+(e instanceof Error?e.message:'request failed');}
},{signal:stop.signal,drainTimeout:5000,matchesReply:m=>app.router.matchesReply(m),onReply:m=>app.answer(m)});
gateway.stop();await app.close();await bus.close();
