import { expect, test } from "bun:test";
import { agentHelp, AgentDescription, SecretHelp, ConfigurationHelp, clearMetadataCache, methodExamples } from "../agent-metadata.tsx";
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
        expect(paths).toEqual(stalled === "/send" ? ["/send"] : ["/send", "/consume"]);
      } finally { release(); server.stop(true); }
    }
  } finally { metadataWait.ms = original; }
}, 3000);

test("a silent help-json ask consumes the budget without a second ask", async () => {
 clearMetadataCache(); const asks:string[]=[];
 const ctx={session:"bare-silent",call:async(_name:string,_body:string,_ms:number,ask:string)=>{asks.push(ask);return ask==="help-json"?{kind:"timeout"}:{kind:"answer",body:"Legacy help"};}} as unknown as Ctx;
 const help=await agentHelp(ctx,{kind:"agent",name:"#silent-json@h",readers:1} as Rec);
 expect(help.unavailable).toContain("timed out"); expect(asks).toEqual(["help-json"]);
});

const methods = {
  help: { help: "Help is not an application action", args: [] },
  status: { help: "Show a service", args: [{ name: "unit", help: "Exact unit", type: "string" }, { name: "format", help: "Output style", type: ["short", "long"], optional: true }] },
  restart: { help: "Restart a service", role: ["operate", "admin"], returns: "Job state", args: [{ name: "unit", help: "Exact unit", type: "string" }] },
  destroy: { help: "Remove it forever", destructive: true, args: [] },
  post: { help: "Send text", args: [{ name: "text", help: "Message", type: "string", rest: true }, { name: "reply_to", help: "Thread", type: "string", optional: true, json_only: true }] },
};
test("methods are native disclosure rows with meaningful groups, signatures and copyable examples", () => {
  const html = render(AgentDescription({ name: "#fixture@h", help: { data: { description: "Fixture", methods, body_formats: ["text", "json"] } } }));
  expect(html).toContain("Methods (4)");
  expect(html).not.toContain('id="method-help"');
  expect(html).not.toContain("Help is not an application action");
  expect(html).not.toContain("<table");
  expect(html.match(/<details class="method-row"/g)?.length).toBe(4);
  expect(html.match(/data-copy=/g)?.length).toBe(8);
  expect(html).toContain('&lt;unit&gt; [format]');
  expect(html).toContain('&lt;text…&gt;');
  expect(html).toContain('<code class="pill">short</code>');
  expect(html).toContain('<code class="pill">long</code>');
  expect(html).toContain('operate or admin');
  expect(html).toContain('⚠ destructive');
  expect(html).toContain('Returns:</strong> Job state');
  for (const label of ['Read', 'Act', 'Destructive']) expect(html).toContain(`<h3>${label} `);
  expect(html).toContain('method="get"');
  expect(html).toContain('name="name" value="#fixture@h"');
  expect(html).toContain('name="m"');
  expect(html).toContain('<noscript>');
  const summary = /id="method-post"><summary>(.*?)<\/summary>/s.exec(html)?.[1];
  expect(summary).not.toContain('reply_to');
  expect(html).toContain('JSON only');
});
test("examples respect supported forms, JSON-only arguments, actual text grammar and authored override", () => {
  const example = methodExamples('post', methods.post, '#fixture@h', ['text','json']);
  expect(example.cli).toBe("agent-bus call '#fixture@h' 'post <text>'");
  expect(example.cli).not.toContain('reply_to');
  expect(JSON.parse(example.json!)).toEqual({ method: 'post', text: '<text>', reply_to: '<reply_to>' });
  expect(methodExamples('hello', { args: [{name:'name',type:'string',optional:true}] }, '#fixture@h', ['text']).cli).toBe("agent-bus call '#fixture@h' 'hello <name>'");
  const overridden = methodExamples('sub.add', { args: [], text: 'sub add <channel> <target>', example: "agent-bus call '#slack@h' 'sub add #alerts #me@h'" }, '#fixture@h', ['text']);
  expect(overridden.cli).toContain('sub add #alerts #me@h');
  expect(overridden.json).toBeUndefined();
  expect(methodExamples('sub.add', { text: 'sub add <channel> <target>' }, '#fixture@h', ['text']).cli).toContain('sub add <channel> <target>');
  const jsonOnly = methodExamples('take', { args: [{ name: 'n', type: 'int', default: 5 }, { name: 'mode', type: ['fast','slow'] }, { name: 'verbose', type: 'bool' }] }, '#fixture@h', ['json']);
  expect(jsonOnly.cli).toBeUndefined();
  expect(JSON.parse(jsonOnly.json!)).toEqual({ method: 'take', n: 5, mode: 'fast', verbose: true });
  const legacy = render(AgentDescription({ help: { data: {description:'Legacy',methods:{ping:{args:[]}}} } }));
  expect(legacy).not.toContain('JSON body');
  expect(legacy).toContain('data-copy=');
});
test("method filter is server-rendered, preserves the record and escapes untrusted declarations", () => {
  const html = render(AgentDescription({ name: '#fixture@h', filter: 'RESTART', help: { data: {description:'Fixture',methods} } }));
  expect(html).toContain('Methods (4)');
  expect(html).toContain('1 of 4 methods match.');
  expect(html).toContain('id="method-restart"');
  expect(html).not.toContain('id="method-status"');
  expect(html).toContain('/agent?name=%23fixture%40h#methods');
  const unsafe = render(AgentDescription({name:"#o'h@h",filter:'"><script>alert(1)</script>',help:{data:{description:'<script>bad</script>',methods:{'<img>':{help:'<script>bad</script>',args:[]}}}}}));
  expect(unsafe).not.toContain('<script>');
  expect(unsafe).toContain('&lt;script&gt;');
  expect(unsafe).toContain('No methods match this filter.');
});


test("hidden-method footer reports only the agent's count and required roles",()=>{
 const html=render(AgentDescription({help:{data:{description:"Fixture",methods:{read:{help:"Read"}},hidden_methods:{count:2,roles:["admin","operate"],description:"must-not-appear"}}},filter:"read"}));
 expect(html).toContain("Methods (1)");
 expect(html).toContain("2 more methods are hidden by your roles.");
 expect(html).toContain("Required roles include: <strong>admin, operate</strong>");
 expect(html).not.toContain("must-not-appear");
 const all=render(AgentDescription({help:{data:{description:"Fixture",methods:{read:{help:"Read"}},hidden_methods:{count:0,roles:[]}}}}));
 expect(all).not.toContain("hidden by your roles");
});

test("help-json gets the full metadata deadline before any fallback",async()=>{
 const {metadataWait}=await import("../agent-metadata.tsx");const original=metadataWait.ms;metadataWait.ms=80;clearMetadataCache();
 const asks:string[]=[];
 try {
  const ctx={session:"full-budget",call:async(_name:string,_body:string,wait:number,ask:string)=>{
   asks.push(ask);if(ask!=="help-json")return {kind:"answer",body:"Legacy help"};
   await Bun.sleep(Math.min(wait,60));return wait<60?{kind:"timeout"}:{kind:"answer",body:JSON.stringify({description:"Structured",methods:{read:{help:"Read"}}})};
  }} as unknown as Ctx;
  const help=await agentHelp(ctx,{kind:"agent",name:"#budget@h",readers:1} as Rec);
  expect(help.data?.description).toBe("Structured");expect(asks).toEqual(["help-json"]);
 }finally{metadataWait.ms=original;}
});

test("method filtering keeps the settings return address",()=>{
 const html=render(AgentDescription({name:"#test@h",filter:"read",returnTo:"/runner-settings?name=%23runner%40h",help:{data:{description:"Test",methods:{read:{help:"Read"}}}}} as Parameters<typeof AgentDescription>[0]));
 expect(html).toContain('type="hidden" name="return" value="/runner-settings?name=%23runner%40h"');
 expect(html).toContain("&amp;return=%2Frunner-settings%3Fname%3D%2523runner%2540h#methods");
});

test("Configuration mentions script environment only for an agent",async()=>{
 const {ConfigurationField}=await import("../ui/forms.tsx");
 for(const kind of ["agent","service","group"]){const html=render(ConfigurationField({st:{values:{}},errId:"err",kind}));expect(html.includes("Scripts receive AGENT_BUS_CONFIG")).toBe(kind==="agent");}
});

test("Methods is a closed disclosure with anchored method copy buttons", () => {
 const html=render(AgentDescription({help:{data:{description:"fixture",methods:{ping:{help:"Ping"}}}}}));
 expect(html).toMatch(/<details class="card agent-methods" id="methods"><summary[^>]*>.*Methods \(1\)/);
 expect(html).not.toMatch(/<details[^>]*id="methods"[^>]*open/);
 expect(html).toContain('id="method-ping"'); expect(html).toContain('data-copy=');
});

test("About shows the agent's docs page only when its help gives an https link", async () => {
  const ctx = (docs?: string) => ({ call: async () => ({ kind: "answer", body: JSON.stringify({ description: "fixture", methods: {}, ...(docs ? { docs } : {}) }) }) }) as unknown as Ctx;
  clearMetadataCache();
  const html = render(AgentDescription({ help: await agentHelp(ctx("https://github.com/o/r/blob/main/docs/agents/f.md"), { kind: "agent", name: "#f@h", readers: 1, at: "1" } as Rec) }));
  expect(html).toContain('<dt>Docs</dt><dd class="no-wrap"><a href="https://github.com/o/r/blob/main/docs/agents/f.md" rel="noreferrer">docs/agents/f.md</a>');
  clearMetadataCache();
  for (const docs of [undefined, "javascript:alert(1)"]) {
    clearMetadataCache();
    expect(render(AgentDescription({ help: await agentHelp(ctx(docs), { kind: "agent", name: "#g@h", readers: 1, at: "2" } as Rec) }))).not.toContain("<dt>Docs</dt>");
  }
});

test("a key name breaks only after an underscore, in the secret and configuration tables", async () => {
  clearMetadataCache();
  const body = JSON.stringify({
    description: "fixture", methods: {},
    secrets: [{ name: "IMAP_CLIENT_SECRET", required: true, help: "the client secret" }, { name: "TOKEN", help: "one word" }],
    config: { allow_plaintext: { type: "boolean", help: "plain" } },
  });
  const ctx = { session: "keys", call: async () => ({ kind: "answer", body }) } as unknown as Ctx;
  const help = await agentHelp(ctx, { kind: "agent", name: "#imap/gmail@h", owner: "owner", readers: 1, at: "keys" } as Rec);
  const secrets = render(SecretHelp({ help }));
  expect(secrets).toContain("<dt><code>IMAP_<wbr>CLIENT_<wbr>SECRET</code>");
  expect(secrets).toContain("<dt><code>TOKEN</code>");
  expect(render(ConfigurationHelp({ help }))).toContain("<dt><code>allow_<wbr>plaintext</code>");
});
