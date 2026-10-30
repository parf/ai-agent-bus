import { expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { startFixture, identity } from "./daemon.ts";
import { Bus, type Message } from "../../clients/typescript/src/index.js";

async function waitFor(check: () => Promise<boolean> | boolean) {
  const end = Date.now() + 8000;
  while (Date.now() < end) { if (await check()) return; await Bun.sleep(20); }
  expect(await check()).toBe(true);
}

test("S.10 standalone bundles route real bus replies to Slack threads and keep Telegram serving", async () => {
  const fx = await startFixture(), self = `#slack-router-${process.pid}@t`, tg = `#tg-router-${process.pid}@t`, a = `#alpha-${process.pid}@t`, b = `#beta-${process.pid}@t`;
  const token = await identity(fx, self), tgToken = await identity(fx, tg), at = await identity(fx, a), bt = await identity(fx, b);
  const A = Bus.connect({addr:fx.socket,token:at}), B = Bus.connect({addr:fx.socket,token:bt}), owner = Bus.connect({addr:fx.socket,token:fx.ownerToken});
  const posts: any[] = [], sockets: any[] = []; let counter = 0, eventId = 0;
  const api = Bun.serve({port:0,hostname:"127.0.0.1",async fetch(req, server) {
    const method = new URL(req.url).pathname.split("/").pop();
    if (method === "socket") return server.upgrade(req) ? undefined : new Response("bad",{status:400});
    if (method === "apps.connections.open") return Response.json({ok:true,url:`ws://127.0.0.1:${api.port}/socket`});
    if (method === "auth.test") return Response.json({ok:true,user:"fixture",user_id:"UBOT",bot_id:"BBOT",app_id:"ABOT",team:"T",url:"https://t"});
    if (method === "users.list") return Response.json({ok:true,members:[{id:"U1",name:"parf"}]});
    if (method === "users.info") return Response.json({ok:true,user:{id:"U1",name:"parf"}});
    if (method === "conversations.list") return Response.json({ok:true,channels:[{id:"C1",name:"dev"},{id:"C2",name:"friends"}]});
    if (method === "chat.postMessage") { const params = Object.fromEntries(new URLSearchParams(await req.text())); const ts=`1740000000.${String(++counter).padStart(6,"0")}`; posts.push({...params,ts});
      const identity = counter % 3 === 1 ? {user:"UBOT"} : counter % 3 === 2 ? {bot_id:"BBOT"} : {app_id:"ABOT"};
      for(const ws of sockets) ws.send(JSON.stringify({envelope_id:`self-${counter}`,type:"events_api",payload:{event_id:`self-${counter}`,event:{type:"message",subtype:"bot_message",channel:params.channel,text:params.text,ts,...identity,...(params.thread_ts?{thread_ts:params.thread_ts}:{})}}}));
      return Response.json({ok:true,channel:params.channel,ts}); }
    if (method === "chat.getPermalink") return Response.json({ok:true,permalink:"https://t/message"});
    if (method === "getUpdates") { await Bun.sleep(100); return Response.json({ok:true,result:[]}); }
    if (method === "getMe") return Response.json({ok:true,result:{id:99,username:"fixture"}});
    return Response.json({ok:false,error:"unexpected "+method});
  }, websocket:{open(ws){sockets.push(ws);},message(){},close(ws){const i=sockets.indexOf(ws);if(i>=0)sockets.splice(i,1);}}});
  const config: any = {users:{U1:"parf"},tags:{a,b},route:b,channels:{C2:{route:a,anyone:true}}};
  await fx.call(fx.ownerToken,"POST","/configure",{name:self,config});
  await fx.call(fx.ownerToken,"POST","/secret",{name:self,secret:`SLACK_BOT_TOKEN=fixture\nSLACK_APP_TOKEN=fixture\nSLACK_API_URL=http://127.0.0.1:${api.port}/api/\n`});
  await fx.call(fx.ownerToken,"POST","/secret",{name:tg,secret:`TELEGRAM_BOT_TOKEN=fixture\nTELEGRAM_API_URL=http://127.0.0.1:${api.port}\n`});
  const children: Subprocess[] = [];
  async function call(bus:Bus, target:string, body:string) { const r=await bus.call(target,body,{timeout:4000});expect(r.kind).toBe("answer");return r.kind==="answer"?r.message.body:""; }
  async function drain(bus:Bus) {const messages:Message[]=[];for(;;){const m=await bus.consume({wait:50});if(!m)break;if(!m.receipt)messages.push(m);}return messages;}
  function sendEvent(over:any) { const id=++eventId; for(const ws of sockets)ws.send(JSON.stringify({envelope_id:`env-${id}`,type:"events_api",payload:{event_id:`event-${id}`,event:{type:"message",channel:"C1",user:"U1",ts:`1750000000.${id}`,...over}}})); }
  // Slack ACKs before routing: wait for this event's observable effect, not its ACK.
  async function event(over:any, done:()=>Promise<boolean>|boolean) { sendEvent(over); await waitFor(done); }
  async function routed(over:any, bus:Bus) {
    const messages:Message[]=[];
    await event(over,async()=>{const m=await bus.consume({wait:100});if(m&&!m.receipt)messages.push(m);return messages.length>0;});
    messages.push(...await drain(bus));
    return messages;
  }
  try {
    // Both executables run from packaged directories outside the source tree.
    for (const [kind,n,tok] of [["slack",self,token],["telegram",tg,tgToken]]) {
      const dir=join(fx.dir,kind!);const pack=spawn([join(import.meta.dir,"..",kind!,"package.sh"),dir],{stdout:"pipe",stderr:"pipe"});expect(await pack.exited).toBe(0);
      children.push(spawn(["bun",join(dir,kind+".js")],{cwd:dir,env:{...process.env,AGENT_BUS_ADDR:fx.socket,AGENT_BUS_NAME:n,AGENT_BUS_TOKEN:tok},stdout:Bun.file(join(fx.dir,kind+".log")),stderr:Bun.file(join(fx.dir,kind+".log"))}));
      await waitFor(async()=>Number((await fx.call(fx.ownerToken,"GET","/lookup?name="+encodeURIComponent(n!))).readers)>0);
    }
    expect(await call(owner,tg,"whoami")).toContain("fixture");
    await waitFor(()=>sockets.length>0);
    await call(A,self,"post C1 first");const first=posts.at(-1);expect(first.text).toBe("first\n\n#a");
    await call(B,self,"post C1 second");expect(posts.at(-1).text).toBe("second\n\n#b");await drain(A);await drain(B);
    const received=await routed({thread_ts:first.ts,text:"reply to first"},A);expect(received.length).toBe(1);expect(await drain(B)).toEqual([]);expect(received[0]!.re).toBeDefined();expect(received[0]!.body).toBe("[parf (Slack)] reply to first");
    const count=posts.length;
    await B.request("POST","/send",{to:self,re:received[0]!.message_id,topic:received[0]!.reply_to!.topic,tag:received[0]!.reply_to!.tag,body:"spoofed"});await call(owner,self,"whoami");expect(posts.length).toBe(count);
    await A.reply(received[0]!,"answer in thread");await waitFor(()=>posts.length>count);expect(posts.at(-1)).toMatchObject({channel:"C1",thread_ts:first.ts,text:"answer in thread\n\n#a"});
    // The next routed event is a FIFO barrier for self echoes and silent chatter.
    sendEvent({text:"ordinary chatter",user:"U9"});
    const tagged = await routed({text:"<@UBOT> #b tag"},B);
    expect(await drain(A)).toEqual([]);expect(posts.length).toBe(count+1);expect(tagged.map(m=>m.body)).toEqual(["[parf (Slack)] tag"]);
    const beforeMCP = posts.length;
    await B.send(self,"MCP answer without re",{topic:tagged[0]!.reply_to!.topic,tag:tagged[0]!.reply_to!.tag});
    await waitFor(()=>posts.length>beforeMCP);expect(posts.at(-1)).toMatchObject({channel:"C1",thread_ts:"1750000000.3",text:"MCP answer without re\n\n#b"});
    expect((await routed({text:"DM fallback",channel:"D1"},B)).map(m=>m.body)).toEqual(["[parf (Slack)] DM fallback"]);
    expect((await routed({text:"<@UBOT> #b constrained",channel:"C2",user:"U9"},A)).map(m=>m.body)).toEqual(["[Slack user U9 (Slack)] #b constrained"]);expect(await drain(B)).toEqual([]);
    const beforeStranger=posts.length;
    await event({text:"<@UBOT> secret stranger",user:"U9"},()=>posts.length>beforeStranger);expect(posts.at(-1).text).toBe("we do not know you");expect(await drain(A)).toEqual([]);expect(await drain(B)).toEqual([]);
    const refused=JSON.parse(await call(owner,self,'{"method":"refused"}'));expect(refused.refused).toMatchObject([{id:"U9",count:1}]);
    expect(await call(A,self,"refused")).toContain("requires this agent's Owner or Maintainer");
    // Subscriptions remain independent even for an unknown routing user.
    await call(owner,self,`sub add C1 ${a} /subscription/`);
    expect((await routed({text:"subscription",user:"U9"},A)).map(m=>JSON.parse(m.body).text)).toEqual(["subscription"]);
    expect(await call(owner,tg,"whoami")).toContain("fixture");
  } finally {for(const child of children){child.kill("SIGKILL");await child.exited;}api.stop(true);await fx.stop();}
},60000);
