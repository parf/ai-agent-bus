import { expect, test } from "bun:test";
import { spawn } from "bun";
import { join } from "node:path";
import { startFixture, identity } from "../slack/daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";

test("TG.8 corrupt stored cache keys are logged and the agent starts and serves", async () => {
 const fx=await startFixture();
 const fake=Bun.serve({port:0,hostname:"127.0.0.1",fetch(req){
  return Response.json({ok:true,result:new URL(req.url).pathname.endsWith("getUpdates")?[]:{id:42,username:"cache_fixture"}});
 }});
 try {
  for (const key of ["chats","users","seen:old"]) for (const raw of ["not JSON",'{"wrong":true}']) {
   const name=`#tg-cache-${process.pid}-${Math.random().toString(16).slice(2)}@t`;
   const token=await identity(fx,name);
   const b=Bus.connect({addr:fx.socket,token:fx.ownerToken});
   await b.record(name).kv.set("string","chats",'[{"id":"old","type":"supergroup"}]',{mode:"set"});
   await b.record(name).kv.set("string",key,raw,{mode:"set"});
   await fx.call(fx.ownerToken,"POST","/secret",{name,secret:`TELEGRAM_BOT_TOKEN=111:fixture\nTELEGRAM_API_URL=http://127.0.0.1:${fake.port}\n`});
   const log=join(fx.dir,"cache.log");
   const p=spawn(["bun","telegram.ts"],{cwd:import.meta.dir,env:{...process.env,AGENT_BUS_ADDR:fx.socket,AGENT_BUS_NAME:name,AGENT_BUS_TOKEN:token},stdout:Bun.file(log),stderr:Bun.file(log)});
   try {
    const end=Date.now()+6000;let ready=false;
    while(Date.now()<end && p.exitCode===null){
     const record=await fx.call(fx.ownerToken,"GET",`/lookup?name=${encodeURIComponent(name)}`);
     if(record.readers>0){ready=true;break;}await Bun.sleep(25);
    }
    expect(ready).toBe(true);
    const out=await b.call(name,"whoami",{timeout:3000});
    expect(out.kind).toBe("answer");
    if(out.kind==="answer") expect(out.message.body).toContain("@cache_fixture");
    expect(await Bun.file(log).text()).toContain(`stored ${key} is not `);
   } finally {p.kill("SIGKILL");await p.exited;}
  }
 } finally {fake.stop(true);await fx.stop();}
},60000);
