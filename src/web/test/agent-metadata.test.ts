import { expect, test } from "bun:test";
import { agentHelp, AgentDescription, SecretHelp, ConfigurationHelp, clearMetadataCache } from "../agent-metadata.tsx";
import type { Ctx, Rec } from "../ctx.ts";
import { render } from "../jsx.ts";

test("bare agent falls back to help text without invented tables", async () => {
  const asks: (string | undefined)[] = [];
  const ctx = { call: async (_name: string, body: string, _wait: number, ask?: string) => { expect(body).toBe(""); asks.push(ask); return { kind: "answer", body: ask === "help" ? "Bare agent instructions" : "unsupported ask" }; } } as Ctx;
  const help = await agentHelp(ctx, { kind: "agent", name: "#bare@h", owner: "owner", readers: 1 } as Rec);
  const html = render(AgentDescription({ help }));
  expect(asks).toEqual(["help-json", "help"]);
  expect(html).toContain("Bare agent instructions");
  expect(html).not.toContain("<table");
  expect(render(SecretHelp({ help }))).toBe("");
  expect(render(ConfigurationHelp({ help }))).toBe("");
});
test("offline agents receive no queued help requests", async () => {
  const ctx = { call: async () => { throw new Error("must not send"); } } as unknown as Ctx;
  expect((await agentHelp(ctx, { kind: "agent", readers: 0 } as Rec)).unavailable).toContain("offline");
});

test("metadata cache is scoped by visitor and invalidated by record timestamp", async () => {
 clearMetadataCache(); let calls=0;
 const context=(session:string)=>({session,call:async()=>{calls++;return {kind:"answer",body:JSON.stringify({description:"fixture",methods:{}})}}}) as unknown as Ctx;
 const rec={kind:"agent",name:"#cached@h",readers:1,at:"one"} as Rec;
 await agentHelp(context("a"),rec); await agentHelp(context("a"),rec); expect(calls).toBe(1);
 await agentHelp(context("b"),rec); expect(calls).toBe(2);
 await agentHelp(context("a"),{...rec,at:"two"}); expect(calls).toBe(3);
});

test("stalled metadata send or answer has one shared deadline and a visible timeout", async () => {
  const { Ctx } = await import("../ctx.ts");
  const { Daemon } = await import("../daemon.ts");
  const { metadataWait } = await import("../agent-metadata.tsx");
  const original = metadataWait.ms;
  metadataWait.ms = 80;
  try {
    for (const stalled of ["/send", "/consume"]) {
      clearMetadataCache();
      let release!: () => void;
      const paths: string[] = [];
      const pause = new Promise<void>(resolve => { release = resolve; });
      const server = Bun.serve({ hostname: "127.0.0.1", port: 0, async fetch(req) {
        const path = new URL(req.url).pathname; paths.push(path);
        if (path === stalled) await pause;
        return new Response("{}");
      } });
      try {
        const ctx = new Ctx(new Request("http://localhost/agent", { headers: {cookie:"agent_bus_session=deadbeef"} }), new Daemon(server.url.toString()), false);
        const start = Date.now();
        const help = await agentHelp(ctx, { kind: "agent", name: "#slow@h", readers: 1 } as Rec);
        expect(render(AgentDescription({ help }))).toContain("description unavailable (timed out)");
        expect(Date.now() - start).toBeLessThan(1000);
        expect(paths).toEqual(stalled === "/send" ? ["/send", "/send"] : ["/send", "/consume", "/send", "/consume"]);
      } finally { release(); server.stop(true); }
    }
  } finally { metadataWait.ms = original; }
}, 3000);

test("a bare agent that ignores help-json still gets a text-help fallback", async () => {
 clearMetadataCache(); const asks:string[]=[];
 const ctx={session:"bare-silent",call:async(_name:string,_body:string,_ms:number,ask:string)=>{asks.push(ask);return ask==="help-json"?{kind:"timeout"}:{kind:"answer",body:"Legacy help"};}} as unknown as Ctx;
 const help=await agentHelp(ctx,{kind:"agent",name:"#silent-json@h",readers:1} as Rec);
 expect(help.text).toBe("Legacy help"); expect(asks).toEqual(["help-json","help"]);
});
