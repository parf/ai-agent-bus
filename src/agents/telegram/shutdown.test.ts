import { expect, test } from "bun:test";
import { spawn } from "bun";
import { startFixture } from "../slack/daemon.ts";

test("SIGTERM exits promptly during held poll, conflict backoff and rate-limit retry", async () => {
  const fx = await startFixture();
  const name = `#telegram-stop-${process.pid}@t`;
  let polled = false, mode = "held";
  const fake = Bun.serve({hostname:"127.0.0.1",port:0,fetch(req) {
    if (new URL(req.url).pathname.endsWith("/getUpdates")) {
      polled = true;
      if (mode === "held") return new Promise<Response>(() => {}); // a real open HTTP request
      return Response.json({ok:false,description:`fixture ${mode}`,parameters:{retry_after:30}}, {status:mode==="429" ? 429 : 200});
    }
    return Response.json({ok:true,result:{id:42,is_bot:true,username:"stop_bot"}});
  }});
  let agent: ReturnType<typeof spawn> | undefined;
  try {
    await fx.call(fx.ownerToken,"POST","/register",{name,kind:"agent",allow:["*"]});
    const token=(await fx.call(fx.ownerToken,"POST","/token",{name})).token;
    await fx.call(fx.ownerToken,"POST","/secret",{name,secret:`TELEGRAM_BOT_TOKEN=fixture\nTELEGRAM_API_URL=http://127.0.0.1:${fake.port}\n`});
    for (mode of ["held","409","429"]) {
      polled = false;
    agent=spawn(["bun","telegram.ts"],{cwd:import.meta.dir,env:{...process.env,AGENT_BUS_ADDR:fx.socket,AGENT_BUS_NAME:name,AGENT_BUS_TOKEN:token},stdout:"ignore",stderr:"pipe"});
    const end=Date.now()+3000;
    while (!polled && Date.now()<end) await Bun.sleep(5);
    expect(polled).toBe(true);
    agent.kill("SIGTERM");
    const result=await Promise.race([agent.exited,Bun.sleep(1200).then(()=>"still running")]);
    expect(result).toBe(0);
    }
  } finally {
    if (agent?.exitCode === null) agent.kill("SIGKILL");
    await agent?.exited;
    fake.stop(true); await fx.stop();
  }
},10000);
