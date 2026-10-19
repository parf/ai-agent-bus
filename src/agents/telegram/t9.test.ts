import { expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { startFixture, identity } from "../slack/daemon.ts";
import { Bus, type Message } from "../../clients/typescript/src/index.js";

async function until(check: () => Promise<boolean> | boolean) {
 const end=Date.now()+8000;
 while(Date.now()<end){if(await check())return;await Bun.sleep(25);}
 throw Error("routing fixture did not reach its barrier");
}

test("TG.9 reply routing uses the mapped sender and correlation, listed users only",async()=>{
 const fx=await startFixture();
 const self=`#telegram-router-${process.pid}@t`,a=`#alpha-${process.pid}@t`,b=`#beta-${process.pid}@t`;
 const token=await identity(fx,self),at=await identity(fx,a),bt=await identity(fx,b);
 const owner=Bus.connect({addr:fx.socket,token:fx.ownerToken});
 const A=Bus.connect({addr:fx.socket,token:at}),B=Bus.connect({addr:fx.socket,token:bt});
 const updates:any[]=[];const sent:any[]=[];let mid=100,seq=1;
 const api=Bun.serve({hostname:"127.0.0.1",port:0,async fetch(req){
  const method=new URL(req.url).pathname.split('/').pop();
  const body=await req.json() as any;
  if(method==='getUpdates') {const result=updates.filter(u=>u.update_id>=Number(body.offset));if(!result.length)await Bun.sleep(100);return Response.json({ok:true,result});}
  if(method==='sendMessage'){sent.push({...body,message_id:++mid});return Response.json({ok:true,result:{message_id:mid}});}
  return Response.json({ok:true,result:{id:99,username:"router_fixture"}});
 }});
 const config:any={default:"@parf",aliases:{"@parf":"100"},tags:{a,b},users:{42:"parf"}};
 await fx.call(fx.ownerToken,'POST','/configure',{name:self,config});
 await fx.call(fx.ownerToken,'POST','/secret',{name:self,secret:`TELEGRAM_BOT_TOKEN=fixture\nTELEGRAM_API_URL=http://127.0.0.1:${api.port}\n`});
 const log=join(fx.dir,'router.log');let agent:Subprocess|undefined;
 async function start(){
  agent=spawn(['bun','telegram.ts'],{cwd:import.meta.dir,env:{...process.env,AGENT_BUS_ADDR:fx.socket,AGENT_BUS_TOKEN:token,AGENT_BUS_NAME:self},stdout:Bun.file(log),stderr:Bun.file(log)});
  await until(async()=>Number((await fx.call(fx.ownerToken,'GET','/lookup?name='+encodeURIComponent(self))).readers)>0);
 }
 async function drain(bus:Bus){const messages:Message[]=[];for(;;){const m=await bus.consume({wait:50});if(!m)break;if(!m.receipt)messages.push(m);}return messages;}
 async function post(bus:Bus,topic:string,tag:string){
  const id=await bus.send(self,'post hello',{topic,tag});
  await until(()=>sent.length>0&&sent.at(-1).text.startsWith('hello'));
  const telegram=sent.at(-1).message_id;
  await until(async()=>{const state=(await owner.record(self).kv.get('string','reply_routes')).value;return String(state).includes('"'+id+'"');});
  await drain(bus);return {id,telegram};
 }
 async function incoming(text:string,reply?:number,user=42){
  const id=++seq;
  updates.push({update_id:id,message:{message_id:id,date:1,text,from:{id:user,username:'employee',first_name:'Employee',last_name:'Person'},chat:{id:100,title:'Fixture chat',type:'private'},...(reply===undefined?{}:{reply_to_message:{message_id:reply}})}});
  await until(async()=>{try{return Number((await owner.record(self).kv.get('string','offset')).value)>id;}catch(e){if((e as any).status===404)return false;throw e;}});
 }
 try{
  await start();
  const first=await post(A,'original-topic','first-tag');expect(sent.at(-1).text).toBe('hello\n\n#a');
  await post(B,'other-topic','second-tag');expect(sent.at(-1).text).toBe('hello\n\n#b');
  await incoming('reply to alpha',first.telegram);
  const replies=await drain(A);const wrong=await drain(B);
  expect(replies.length).toBe(1);expect(wrong.length).toBe(0);
  expect(replies[0]!.from).toBe(self);expect(replies[0]!.topic).toBe('original-topic');expect(replies[0]!.tag).toBe('first-tag');expect(replies[0]!.re).toBe(first.id);
  const replyNotice=sent.length;
  await B.request('POST','/send',{to:self,re:replies[0]!.message_id,topic:replies[0]!.reply_to!.topic,tag:replies[0]!.reply_to!.tag,body:'spoofed session answer'});
  await until(async()=>(await Bun.file(log).text()).includes('dropped unmatched session reply'));
  expect(sent.length).toBe(replyNotice);
  await A.reply(replies[0]!, 'session answer');
  await until(()=>sent.length>replyNotice);
  expect(sent.at(-1).text).toBe('session answer\n\n#a');
  expect(sent.at(-1).reply_to_message_id).toBe(2);
  expect(replies[0]!.body).toBe('[parf (TG)] reply to alpha');expect(replies[0]!.roles).not.toContain('owner');expect(replies[0]!.roles).not.toContain('maintainer');
  await incoming('#b tagged');expect((await drain(B)).map(m=>m.body)).toEqual(['[parf (TG)] tagged']);expect(await drain(A)).toEqual([]);
  await incoming(a+' full name');expect((await drain(A)).map(m=>m.body)).toEqual(['[parf (TG)] full name']);
  await incoming('private '+ 'z'.repeat(100),first.telegram,43);
  expect(await drain(A)).toEqual([]);expect(await drain(B)).toEqual([]);
  expect(sent.at(-1).text).toBe('we do not know you');
  const refused=await owner.call(self,'{"method":"refused"}',{timeout:3000});expect(refused.kind).toBe('answer');
  if(refused.kind==='answer')expect(JSON.parse(refused.message.body).senders).toMatchObject([{id:'43',username:'employee',name:'Employee Person',chat:'100',count:1}]);
  const forbidden=await A.call(self,'refused',{timeout:3000});expect(forbidden.kind).toBe('answer');if(forbidden.kind==='answer')expect(forbidden.message.body).toContain('requires owner or maintainer');
  const logs=await Bun.file(log).text();expect(logs).toContain('"user":"43"');expect(logs).toContain('@employee');expect(logs).toContain('Employee Person');expect(logs).toContain('Fixture chat');expect(logs).toContain('not a listed user');expect(logs).not.toContain('z'.repeat(41));
  const before=sent.length;await incoming('where does this go?');expect(sent.length).toBe(before+1);expect(sent.at(-1).text).toContain('did not understand');expect(sent.at(-1).text).toContain('#a');expect(sent.at(-1).text).toContain('#b');
  config.route=b;await fx.call(fx.ownerToken,'POST','/configure',{name:self,config});await incoming('fallback');expect((await drain(B)).map(m=>m.body)).toEqual(['[parf (TG)] fallback']);
  delete config.route;await fx.call(fx.ownerToken,'POST','/configure',{name:self,config});
  agent!.kill('SIGKILL');await agent!.exited;
  const state=JSON.parse(String((await owner.record(self).kv.get('string','reply_routes')).value));state.messages['100:'+first.telegram].until=Date.now()-1;
  await owner.record(self).kv.set('string','reply_routes',JSON.stringify(state),{mode:'set'});
  await start();
  await incoming('too late',first.telegram);expect(sent.at(-1).text).toBe('that thread expired; use #tag');expect(await drain(A)).toEqual([]);expect(await drain(B)).toEqual([]);
  await incoming('#a still the same session');expect((await drain(A)).map(m=>m.body)).toEqual(['[parf (TG)] still the same session']);
 }finally{agent?.kill('SIGKILL');if(agent)await agent.exited;api.stop(true);await fx.stop();}
},60000);
