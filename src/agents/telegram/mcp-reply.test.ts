import { expect, test } from "bun:test";
import { Router } from "./router.ts";
import { startFixture, identity } from "../slack/daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";

test("Telegram MCP-style answers without re reach the correlated chat, not the request handler", async () => {
  const fx=await startFixture(),self=`#mcp-router-${process.pid}@t`,session=`#session-${process.pid}@t`;
  const tok=await identity(fx,self),st=await identity(fx,session),bus=Bus.connect({addr:fx.socket,token:tok}),peer=Bus.connect({addr:fx.socket,token:st});
  const told:any[]=[],stop=new AbortController();let requests=0;
  const router=new Router({self,get:async()=>undefined,set:async()=>{},config:async()=>({users:{42:"parf"},route:session}),send:async(route,body,reply_to)=>(await bus.request("POST","/send",{...route,body,reply_to})).message_id,tell:async(chat,message,text)=>{told.push({chat,message,text});return 99;},fallback:async()=>false,log:()=>{}});
  const serving=bus.serve(()=>{requests++;return "ordinary request";},{signal:stop.signal,matchesReply:m=>router.matchesReply(m),onReply:m=>router.answer(m)});
  try{
    await router.incoming({message_id:7,date:1,from_id:42,text:"question"},"100");
    const request=await peer.consume({wait:1000});expect(request).not.toBeNull();
    await peer.send(self,"from MCP",{topic:request!.reply_to!.topic,tag:request!.reply_to!.tag});
    const end=Date.now()+3000;while(!told.length&&Date.now()<end)await Bun.sleep(10);
    expect(told).toEqual([{chat:"100",message:7,text:expect.stringContaining("from MCP")}]);expect(requests).toBe(0);
  }finally{stop.abort();await serving;await fx.stop();}
},15000);
