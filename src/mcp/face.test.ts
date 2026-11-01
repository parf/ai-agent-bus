import { afterEach, expect, test } from "bun:test";
import {spawnSync} from "node:child_process";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { startTime } from "./face-mark.ts";

// The face's tools against a fake daemon: what it asks the daemon is the
// claim, so every check reads the request the daemon saw, not the face's word.
type Handler = (req: Request, url: URL) => Response | Promise<Response>;
const cleanup: (() => unknown)[] = [];
afterEach(async () => { for (const f of cleanup.splice(0).reverse()) await f(); });

async function face(route: Handler, env: Record<string, string> = {}, more?: (port: number) => Record<string, string>, identity: object = {protocol:1,protocol_min:1}) {
  const daemon = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: (req) => {
    const url = new URL(req.url);
    if (url.pathname === "/identity") return Response.json(identity);
    if (url.pathname === "/ls") return Response.json([]);
    if (url.pathname === "/register") return Response.json({ name: "#face@test", kind: "agent", owner: "o" });
    return route(req, url);
  } });
  cleanup.push(() => daemon.stop(true));
  const stderr: string[] = [];
  const transport = new StdioClientTransport({
    command: process.execPath, args: [`${import.meta.dir}/server.ts`], stderr: "pipe",
    env: { PATH: process.env.PATH ?? "", AGENT_BUS_ADDR: `http://127.0.0.1:${daemon.port}`, AGENT_BUS_TOKEN: "t", AGENT_BUS_NAME: "#face@test", ...env, ...more?.(daemon.port!) },
  });
  transport.stderr?.on("data", (d) => stderr.push(String(d)));
  const client = new Client({ name: "face-test", version: "1" });
  const notifications: any[] = [];
  client.fallbackNotificationHandler = async n => { notifications.push(n); };
  await client.connect(transport);
  cleanup.push(() => client.close());
  const call = async (name: string, args: Record<string, unknown>, signal?: AbortSignal) => {
    const r = await client.callTool({ name, arguments: args }, undefined, signal ? { signal } : undefined) as { content: { text: string }[]; isError?: boolean };
    return { text: r.content.map((c) => c.text).join("\n"), isError: !!r.isError };
  };
  return { call, stderr, notifications, pid: transport.pid };
}

const message = { message_id: "m1", from: "#peer@test", to: "#face@test", body: "queued-body", at: "2026-10-17T00:00:00Z" };

test("ab_consume asks the daemon with wait 0s, and gets what is queued", async () => {
  const waits: (string | null)[] = [];
  const { call } = await face((req, url) => {
    if (url.pathname !== "/consume") return new Response("no", { status: 404 });
    waits.push(url.searchParams.get("wait"));
    return Response.json(message);
  });
  const got = await call("ab_consume", { wait: "0s" });
  expect(waits).toEqual(["0s"]);
  expect(got.text).toContain("queued-body");
});

test("ab_consume passes a wait that is no duration to the daemon, which refuses it", async () => {
  const waits: (string | null)[] = [];
  const { call } = await face((_req, url) => {
    waits.push(url.searchParams.get("wait"));
    return url.searchParams.get("wait") === "2" ? new Response("wait is not a duration", { status: 400 }) : new Response(null, { status: 204 });
  });
  const got = await call("ab_consume", { wait: "2" });
  expect(waits).toEqual(["2"]);
  expect(got.isError).toBe(true);
  expect(got.text).toContain("bus said 400");
});

test("a cancelled ab_lock wait ends the daemon's request", async () => {
  let opened = false, aborted = false;
  const { call } = await face(async (req, url) => {
    if (url.pathname !== "/lock") return new Response("no", { status: 404 });
    opened = true;
    // The daemon parks a lock wait on the request: only the request ending lets it go.
    await new Promise<void>((r) => req.signal.addEventListener("abort", () => { aborted = true; r(); }, { once: true }));
    return new Response(null, { status: 204 });
  });
  const stop = new AbortController();
  const pending = call("ab_lock", { record: "r@test", name: "n", ttl: "30s", wait: "30s" }, stop.signal).catch(() => "cancelled");
  for (let i = 0; i < 100 && !opened; i++) await Bun.sleep(20);
  expect(opened).toBe(true);
  stop.abort();
  await pending;
  for (let i = 0; i < 100 && !aborted; i++) await Bun.sleep(20);
  expect(aborted).toBe(true);
});

test("ab_kv_get answers an int past 2^53 exactly", async () => {
  const { call } = await face((_req, url) => url.pathname === "/kv"
    ? new Response('{"record":"r@test","name":"n","kind":"int","value":9007199254740993}', { headers: { "content-type": "application/json" } })
    : new Response("no", { status: 404 }));
  const got = await call("ab_kv_get", { record: "r@test", name: "n", kind: "int" });
  expect(got.text).toContain('"value":9007199254740993');
});

// A Codex push loop whose app-server went away must not take another message:
// it is consumed and then lost when the delivery fails.
test("codex push stops reading when the app-server connection drops", async () => {
  const ws = Bun.serve({
    port: 0, hostname: "127.0.0.1",
    fetch: (req, s) => (s.upgrade(req) ? undefined : new Response("no", { status: 400 })),
    websocket: { message(sock, raw) {
      const m = JSON.parse(String(raw));
      if (m.id === undefined) return;
      sock.send(JSON.stringify({ jsonrpc: "2.0", id: m.id, result: {} }));
      if (m.method === "initialize") setTimeout(() => sock.close(), 100);
    } },
  });
  cleanup.push(() => ws.stop(true));
  let consumes = 0, handed = 0;
  const { stderr } = await face(async (req, url) => {
    if (url.pathname !== "/consume") return new Response("no", { status: 404 });
    consumes++;
    await Bun.sleep(400);
    if (req.signal.aborted) return new Response(null, { status: 204 });
    handed++;
    return Response.json({ ...message, message_id: `m${handed}` });
  }, { AGENT_BUS_PUSH: "codex", AGENT_BUS_CODEX_WS: `ws://127.0.0.1:${ws.port}` });
  await Bun.sleep(1500);
  expect(consumes).toBeGreaterThan(0);
  expect(stderr.join("")).toContain("codex: the shared app-server closed the connection");
  expect(handed).toBe(0);
});

// A plain `claude` in a directory an ab-claude session once used starts the
// local-scope channel entry with no session and no token: the face leaves at
// once, exit 0 with one line, never dying with "set AGENT_BUS_TOKEN" (J2).
test("with no launcher session and no token the face exits quietly", async () => {
  const env = Object.fromEntries(Object.entries(process.env).filter(([k]) => !k.startsWith("AGENT_BUS_")));
  const p = Bun.spawn(["bun", "server.ts"], { cwd: import.meta.dir, env, stdin: "pipe", stdout: "pipe", stderr: "pipe" });
  const code = await Promise.race([p.exited, Bun.sleep(10_000).then(() => { p.kill(); return -1; })]);
  expect(code).toBe(0);
  expect(await new Response(p.stderr).text()).toContain("no launcher session and no token here; nothing to serve");
});

test("MCP registration keeps the record purpose instead of stale inherited metadata", async () => {
  const registered: Record<string, unknown>[] = [];
  const daemon = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: async (req) => {
    const path = new URL(req.url).pathname;
    if (path === "/identity") return Response.json({protocol:1,protocol_min:1});
    if (path === "/ls") return Response.json([{ name: "#face@test", kind: "agent", descr: "Web-edited purpose", tags: ["review", "billing"], allow: ["@owner"] }]);
    if (path === "/register") { const body = await req.json() as Record<string, unknown>; registered.push(body); return Response.json(body); }
    return new Response("no", { status: 404 });
  } });
  cleanup.push(() => daemon.stop(true));
  const transport = new StdioClientTransport({ command: process.execPath, args: [`${import.meta.dir}/server.ts`], stderr: "ignore",
    env: { PATH: process.env.PATH ?? "", AGENT_BUS_ADDR: `http://127.0.0.1:${daemon.port}`, AGENT_BUS_TOKEN: "t", AGENT_BUS_NAME: "#face@test", AGENT_BUS_DESCR: "Old inherited purpose", AGENT_BUS_RUNTIME: "codex" },
  });
  const client = new Client({ name: "purpose-test", version: "1" });
  cleanup.push(() => client.close());
  await client.connect(transport);
  expect(registered).toHaveLength(1);
  expect(registered[0]?.name).toBe("#face@test");
  expect(registered[0]?.descr).toBe("Web-edited purpose");
  expect(registered[0]?.tags).toEqual(["ai-cli", "billing", "codex", "review"]);
});


test("session asks are answered by the face without entering the model push channel", async () => {
  const asks=["help","help-json","help-config","help-secret","mcp","status","not-a-known-ask"];
  const incoming=asks.map((ask,i)=>({...message,message_id:`ask-${i}`,body:"",ask,topic:"description",tag:`q${i}`,
    ...(i===1?{reply_to:{name:"#viewer@test",topic:"web",tag:"metadata"}}:{})}));
  incoming.push({...message,message_id:"ordinary"} as any);
  const replies:any[]=[];
  let done!:()=>void; const delivered=new Promise<void>(r=>done=r);
  const {notifications}=await face(async(req,url)=>{
    if(url.pathname==="/lookup") return Response.json({name:"#face@test",kind:"agent",descr:"Review the bus",tags:["claude","ai-cli","review"]});
    if(url.pathname==="/send") {const reply=await req.json() as Record<string,unknown>;replies.push(reply);return Response.json({...message,...reply});}
    if(url.pathname==="/consume") {
      const e=incoming.shift();if(e)return Response.json(e);
      done();await new Promise<void>(resolve=>req.signal.addEventListener("abort",()=>resolve(),{once:true}));
      return new Response(null,{status:204});
    }
    return new Response("no",{status:404});
  },{AGENT_BUS_PUSH:"claude",AGENT_BUS_RUNTIME:"claude",AGENT_BUS_CWD:"/work/review"});
  expect(await Promise.race([delivered.then(()=>true),Bun.sleep(1000).then(()=>false)])).toBe(true);
  expect(replies).toHaveLength(7);
  for(let i=0;i<asks.length;i++) expect(replies[i].re).toBe(`ask-${i}`);
  expect(replies[0]).toMatchObject({to:"#peer@test",topic:"description",tag:"q0"});
  expect(replies[0].body).toContain("AI CLI session; talk to it in plain text");
  const data=JSON.parse(replies[1].body);
  expect(data).toMatchObject({description:expect.stringContaining("Review the bus"),runtime:"claude",cwd:"/work/review",tags:["claude","ai-cli","review"],methods:{},config:{},secrets:[]});
  expect(replies[1]).toMatchObject({to:"#viewer@test",topic:"web",tag:"metadata"});
  expect(replies[2].body).toBe("none");expect(replies[3].body).toBe("none");
  expect(JSON.parse(replies[4].body)).toEqual({tools:[]});
  expect(replies[5].body).toBe("unsupported: status");
  expect(replies[6].body).toBe("unsupported: not-a-known-ask");
  for(let i=0;i<100&&!notifications.length;i++)await Bun.sleep(2);
  expect(notifications).toHaveLength(1);
  expect(notifications[0].params.meta.message_id).toBe("ordinary");
});


test("pull mode answers its own asks while a foreign inbox remains untouched", async () => {
  let own=0;const replies:any[]=[];
  const {call}=await face(async(req,url)=>{
    if(url.pathname==="/lookup")return Response.json({descr:"Pull session",tags:[]});
    if(url.pathname==="/send"){const reply=await req.json() as Record<string,unknown>;replies.push(reply);return Response.json({...message,...reply});}
    if(url.pathname==="/consume")return Response.json(url.searchParams.has("inbox")
      ? {...message,to:"#other@test",ask:"help",body:""}
      : ++own===1?{...message,message_id:"own-help",ask:"help-json",body:""}:message);
    return new Response("no",{status:404});
  });
  const got=await call("ab_consume",{wait:"1s"});
  expect(got.text).toContain("queued-body");expect(replies).toHaveLength(1);
  expect(replies[0].re).toBe("own-help");
  const other=await call("ab_consume",{inbox:"#other@test",wait:"0s"});
  expect(other.text).toContain("ask: help");expect(replies).toHaveLength(1);
});

// J14: a second session under a live name used to start, log the refusal to
// stderr, and then answer every tool as if its replies would arrive.
test("a face refused as a second reader says so in every tool result", async () => {
  const { call } = await face((_req, url) =>
    url.pathname === "/consume" ? new Response("inbox already has a reader", { status: 409 }) : new Response("no", { status: 404 }), { AGENT_BUS_PUSH: "claude" });
  await Bun.sleep(400); // the push loop's first consume
  for (const tool of ["ab_ls", "ab_send"]) {
    const got = await call(tool, tool === "ab_send" ? { to: "#peer@test", text: "x" } : {});
    expect(got.isError).toBe(true);
    expect(got.text).toContain("#face@test is already read by another session");
    expect(got.text).toContain("--take-over");
  }
});

test("a face that holds its name serves its tools", async () => {
  const { call } = await face((_req, url) =>
    url.pathname === "/consume" ? new Response(null, { status: 204 }) : new Response("no", { status: 404 }), { AGENT_BUS_PUSH: "claude" });
  await Bun.sleep(400);
  expect((await call("ab_ls", {})).isError).toBe(false);
});

// A launcher's run directories, as the face sees them: <runs>/<run>/bus-env.json.
function runs(name: string, ...extra: { run: string; name: string }[]) {
  const root = mkdtempSync(join(resolve(import.meta.dir, "../../tmp"), "face-runs-"));
  cleanup.push(() => rmSync(root, { recursive: true, force: true }));
  const make = (run: string, who: string, env: Record<string, string> = {}) => {
    mkdirSync(join(root, run), { recursive: true });
    const file = join(root, run, "bus-env.json");
    writeFileSync(file, JSON.stringify({ AGENT_BUS_NAME: who, AGENT_BUS_TOKEN: "t", ...env }));
    return file;
  };
  return { root, make, mine: (env: Record<string, string> = {}) => make("claude-mine", name, env) };
}

// RD129 takeover: an explicit --take-over face asks the live holder to let go,
// then reads. The holder here is a real process with a verified mark, so the
// signal's effect (SIGUSR2 ends it) is what the check watches.
test("a --take-over face evicts the verified holder and then reads the name", async () => {
  const r = runs("#face@test");
  const holder = Bun.spawn(["sleep", "30"]);
  cleanup.push(() => holder.kill());
  r.make("claude-theirs", "#face@test");
  writeFileSync(join(r.root, "claude-theirs", `face-${holder.pid}`), startTime(holder.pid));
  let held = true, consumes = 0;
  const got = await face((_req, url) => {
    if (url.pathname !== "/consume") return new Response("no", { status: 404 });
    consumes++;
    if (held && holder.exitCode === null && holder.signalCode === null) return new Response("held", { status: 409 });
    held = false;
    return new Response(null, { status: 204 });
  }, { AGENT_BUS_PUSH: "claude", AGENT_BUS_TAKEOVER: "1" }, () => ({ AGENT_BUS_SESSION_FILE: r.mine({ AGENT_BUS_PUSH: "claude", AGENT_BUS_TAKEOVER: "1" }) }));
  expect(await Promise.race([holder.exited, Bun.sleep(4000).then(() => "alive")])).not.toBe("alive");
  await Bun.sleep(500);
  expect(consumes).toBeGreaterThan(1);
  expect((await got.call("ab_ls", {})).isError).toBe(false);
  expect(got.stderr.join("")).toContain(`asked face pid ${holder.pid} to let go`);
});

test("a face without --take-over leaves the holder alone and names it", async () => {
  const r = runs("#face@test");
  const holder = Bun.spawn(["sleep", "30"]);
  cleanup.push(() => holder.kill());
  r.make("claude-theirs", "#face@test");
  writeFileSync(join(r.root, "claude-theirs", `face-${holder.pid}`), startTime(holder.pid));
  const got = await face((_req, url) => url.pathname === "/consume" ? new Response("held", { status: 409 }) : new Response("no", { status: 404 }),
    { AGENT_BUS_PUSH: "claude" }, () => ({ AGENT_BUS_SESSION_FILE: r.mine({ AGENT_BUS_PUSH: "claude" }) }));
  await Bun.sleep(600);
  const said = await got.call("ab_ls", {});
  expect(said.isError).toBe(true);
  expect(said.text).toContain(`face pid ${holder.pid}`); // the holder is named, not just "refused"
  expect(holder.exitCode).toBeNull();
});

// The holder side: told to let go, it says so in every tool result and, being
// idle, in one notice; it does not read again.
test("a holder told to let go says so and stops reading", async () => {
  let consumes = 0;
  const got = await face((_req, url) => {
    if (url.pathname !== "/consume") return new Response("no", { status: 404 });
    consumes++;
    return new Promise<Response>(r => setTimeout(() => r(new Response(null, { status: 204 })), 150));
  }, { AGENT_BUS_PUSH: "claude" });
  await Bun.sleep(400);
  expect((await got.call("ab_ls", {})).isError).toBe(false);
  process.kill(got.pid!, "SIGUSR2");
  await Bun.sleep(600);
  const after = consumes;
  const said = await got.call("ab_ls", {});
  expect(said.isError).toBe(true);
  expect(said.text).toContain("taken over by another session");
  await Bun.sleep(500);
  expect(consumes).toBe(after); // no reads after the let-go
  expect(JSON.stringify(got.notifications)).toContain("taken over");
});

// A fork: the runtime's session id differs from the one the launcher bound AND
// the name is held. Only then does the face ask the launcher for a name.
function control(answer: () => Response) {
  const asked: string[] = [];
  const srv = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: async req => {
    asked.push(`${new URL(req.url).pathname} ${req.headers.get("authorization")} ${await req.text()}`);
    return answer();
  } });
  cleanup.push(() => srv.stop(true));
  return { asked, addr: `http://127.0.0.1:${srv.port}` };
}
const FORK = "11111111-1111-1111-1111-111111111111", BOUND = "22222222-2222-2222-2222-222222222222";

test("a fork refused its parent's name asks the launcher for its own and reads under it", async () => {
  const r = runs("#face@test");
  const ctl = control(() => new Response("#face.job@test"));
  const seen: string[] = [];
  const got = await face((req, url) => {
    if (url.pathname !== "/consume") return new Response("no", { status: 404 });
    const who = req.headers.get("x-agent-bus-token") ?? "";
    seen.push(who);
    return who.endsWith("fork-token") ? new Response(null, { status: 204 }) : new Response("held", { status: 409 });
  }, { AGENT_BUS_PUSH: "claude", CLAUDE_CODE_SESSION_ID: FORK }, () => {
    const file = r.mine({ AGENT_BUS_PUSH: "claude", AGENT_BUS_SESSION_ID: BOUND, AGENT_BUS_CONTROL_ADDR: ctl.addr, AGENT_BUS_CONTROL_TOKEN: "ctl" });
    writeFileSync(join(r.root, "claude-mine", `fork-${FORK}.json`), JSON.stringify({ AGENT_BUS_NAME: "#face.job@test", AGENT_BUS_TOKEN: "fork-token", AGENT_BUS_PUSH: "claude" }));
    return { AGENT_BUS_SESSION_FILE: file, AGENT_BUS_SESSION_ID: BOUND, AGENT_BUS_CONTROL_ADDR: ctl.addr, AGENT_BUS_CONTROL_TOKEN: "ctl" };
  });
  await Bun.sleep(800);
  expect(ctl.asked).toEqual([`/fork Bearer ctl {"id":"${FORK}"}`]);
  expect(seen.some(a => a.endsWith("fork-token"))).toBe(true); // reads under the derived credential
  expect((await got.call("ab_ls", {})).isError).toBe(false);
  expect(got.stderr.join("")).toContain("this fork is #face.job@test");
});

test("a changed session id without a held name is not a fork", async () => { // /clear, then an /mcp reconnect
  const r = runs("#face@test");
  const ctl = control(() => new Response("#face.job@test"));
  await face((_req, url) => url.pathname === "/consume" ? new Response(null, { status: 204 }) : new Response("no", { status: 404 }),
    { AGENT_BUS_PUSH: "claude", CLAUDE_CODE_SESSION_ID: FORK }, () => ({ AGENT_BUS_SESSION_FILE: r.mine({ AGENT_BUS_PUSH: "claude" }), AGENT_BUS_SESSION_ID: BOUND, AGENT_BUS_CONTROL_ADDR: ctl.addr, AGENT_BUS_CONTROL_TOKEN: "ctl" }));
  await Bun.sleep(600);
  expect(ctl.asked).toEqual([]);
});

test("the bound session itself, refused its name, does not ask for a fork name", async () => {
  const r = runs("#face@test");
  const ctl = control(() => new Response("#face.job@test"));
  const got = await face((_req, url) => url.pathname === "/consume" ? new Response("held", { status: 409 }) : new Response("no", { status: 404 }),
    { AGENT_BUS_PUSH: "claude", CLAUDE_CODE_SESSION_ID: BOUND }, () => ({ AGENT_BUS_SESSION_FILE: r.mine({ AGENT_BUS_PUSH: "claude" }), AGENT_BUS_SESSION_ID: BOUND, AGENT_BUS_CONTROL_ADDR: ctl.addr, AGENT_BUS_CONTROL_TOKEN: "ctl" }));
  await Bun.sleep(600);
  expect(ctl.asked).toEqual([]);
  expect((await got.call("ab_ls", {})).isError).toBe(true);
});

// Two --take-over faces started together must not evict each other: the taker
// ignores the let-go signal.
test("a face started with --take-over ignores the let-go signal", async () => {
  const got = await face((_req, url) => url.pathname === "/consume"
    ? new Promise<Response>(r => setTimeout(() => r(new Response(null, { status: 204 })), 150))
    : new Response("no", { status: 404 }), { AGENT_BUS_PUSH: "claude", AGENT_BUS_TAKEOVER: "1" });
  await Bun.sleep(300);
  process.kill(got.pid!, "SIGUSR2");
  await Bun.sleep(400);
  expect((await got.call("ab_ls", {})).isError).toBe(false);
});

import { Bus, BusError } from './bus.ts';
import {mkdir,mkdtemp,writeFile,readFile,rm} from 'node:fs/promises';
import {createHash} from 'node:crypto';
test('MCP big files stay out of context and corrupt downloads never replace output',async()=>{
 expect(typeof (Bus.prototype as any).bigPut).toBe('function');
 await mkdir(join(import.meta.dir,'../../tmp'),{recursive:true});
 const scratch=await mkdtemp(join(import.meta.dir,'../../tmp/mcp-big-'));
 let data=Buffer.from([0,255,1]),bad=false,calls=0;
 const sha=()=>createHash('sha256').update(data).digest('hex');
 const server=Bun.serve({port:0,hostname:'127.0.0.1',async fetch(req){const url=new URL(req.url);if(url.pathname==='/identity')return Response.json({protocol:1,protocol_min:1});calls++;
  expect(req.headers.get('x-agent-bus-token')).toBe('fixture-only');
  if(req.method==='PUT'){data=Buffer.from(await req.arrayBuffer());expect(req.headers.get('x-bigfile-sha256')).toBe(sha());return Response.json({size:data.length,sha256:sha(),replaced:false});}
  if(url.pathname==='/bigfile/list')return Response.json({record:'owner@fixture',keys:[]});
  if(req.method==='POST')return Response.json({deleted:true});
  return new Response(data,{headers:{'Content-Length':String(data.length),'X-Bigfile-Sha256':bad?'0'.repeat(64):sha(),'X-Bigfile-Held':'memory','X-Bigfile-Expires':'2026-11-02T00:00:00Z'}});
 }});
 const bus:any=new Bus({AGENT_BUS_NAME:'#mcp@fixture',AGENT_BUS_ADDR:`http://127.0.0.1:${server.port}`,AGENT_BUS_TOKEN:'fixture-only',AGENT_BUS_BIGFILE_DIR:scratch});
 const input=join(scratch,'in'),output=join(scratch,'out');
 try{
  await expect(bus.bigPut('r','n',join(scratch,'missing'))).rejects.toThrow();await writeFile(input,'');await expect(bus.bigPut('r','n',input)).rejects.toThrow();await expect(bus.bigGet('r','n',undefined)).rejects.toThrow('output_path');expect(calls).toBe(0);
  const unset:any=new Bus({AGENT_BUS_NAME:'#mcp@fixture',AGENT_BUS_ADDR:`http://127.0.0.1:${server.port}`,AGENT_BUS_TOKEN:'fixture-only'});
  await expect(unset.bigPut('r','n',input)).rejects.toThrow('bigfile is not configured');await expect(unset.bigGet('r','n',output)).rejects.toThrow('bigfile is not configured');
  await expect(bus.bigPut('r','n','/etc/passwd')).rejects.toThrow('outside');await expect(bus.bigGet('r','n','/tmp/agent-bus-escape')).rejects.toThrow('outside');expect(calls).toBe(0);
  data=Buffer.alloc(1048577,255);await writeFile(input,data);const up=await bus.bigPut('r','n',input,{ttl:'30d'});expect(up.size).toBe(data.length);
  const down=await bus.bigGet('r','n',output);expect(down.sha256).toBe(up.sha256);expect(await readFile(output)).toEqual(data);expect(JSON.stringify(down)).not.toContain('data');
  bad=true;await writeFile(output,'keep old');await expect(bus.bigGet('r','n',output)).rejects.toThrow('SHA-256');expect(await readFile(output,'utf8')).toBe('keep old');
 }finally{server.stop(true);await rm(scratch,{recursive:true,force:true});}
});

test('MCP big tools transfer files, require output_path and preserve unavailable causes',async()=>{
 const root=mkdtempSync(join(resolve(import.meta.dir,'../../tmp'),'face-big-'));cleanup.push(()=>rmSync(root,{recursive:true,force:true}));
 const input=join(root,'in'),output=join(root,'out');const payload=Buffer.from([0,255,1,2]);writeFileSync(input,payload);
 const sum=createHash('sha256').update(payload).digest('hex');let unavailable=false;const seen:string[]=[];
 const {call}=await face(async(req,url)=>{
  seen.push(req.method+' '+url.pathname);
  if(unavailable)return Response.json({error:'disk path blocked'},{status:503});
  if(req.method==='PUT'){expect(Buffer.from(await req.arrayBuffer())).toEqual(payload);return Response.json({record:'r',name:'n',size:payload.length,sha256:sum,held:'memory',expires:'2026-11-02T00:00:00Z',replaced:false,content:'SHOULD_NOT_REACH_MODEL'});}
  if(url.pathname==='/bigfile/list')return Response.json({record:'r',keys:[]});
  if(req.method==='POST')return Response.json({deleted:true});
  return new Response(payload,{headers:{'Content-Length':String(payload.length),'X-Bigfile-Sha256':sum,'X-Bigfile-Held':'memory','X-Bigfile-Expires':'2026-11-02T00:00:00Z'}});
 },{AGENT_BUS_BIGFILE_DIR:root});
 const missing=await call('ab_bigfile_get',{record:'r',name:'n'});expect(missing.isError).toBe(true);expect(missing.text).toContain('output_path');expect(seen).toEqual([]);
 const uploaded=await call('ab_bigfile_put',{record:'r',name:'n',input_path:input,ttl:'24h'});expect(uploaded.isError).toBe(false);expect(uploaded.text).toContain(sum);expect(uploaded.text).not.toContain('SHOULD_NOT_REACH_MODEL');
 const downloaded=await call('ab_bigfile_get',{record:'r',name:'n',output_path:output});expect(downloaded.isError).toBe(false);expect(await readFile(output)).toEqual(payload);
 expect((await call('ab_bigfile_ls',{record:'r'})).isError).toBe(false);expect((await call('ab_bigfile_rm',{record:'r',name:'n'})).isError).toBe(false);
 unavailable=true;
 for(const [name,args] of [['ab_bigfile_put',{record:'r',name:'n',input_path:input}],['ab_bigfile_get',{record:'r',name:'n',output_path:output}],['ab_bigfile_ls',{record:'r'}],['ab_bigfile_rm',{record:'r',name:'n'}]] as const){const refusal=await call(name,args);expect(refusal.isError).toBe(true);expect(refusal.text).toContain('503');expect(refusal.text).toContain('disk path blocked');}
});

test('MCP big tools roundtrip local files through the real daemon without returning content',async()=>{
 const scratch=await mkdtemp(resolve(import.meta.dir,'../../tmp/mcp-real-big-'));
 let client:Client|undefined,daemon:ReturnType<typeof Bun.spawn<"pipe","pipe","pipe">>|undefined;
 try{
  const fixture=process.env.AB_CLIENT_FIXTURE??join(scratch,'client-fixture');
  if(!process.env.AB_CLIENT_FIXTURE){
   const build=spawnSync('go',['build','-o',fixture,'./clients/fixture'],{cwd:resolve(import.meta.dir,'..'),encoding:'utf8',timeout:20000});
   expect(build.status,build.stderr||String(build.error??'fixture build failed')).toBe(0);
  }
  daemon=Bun.spawn([fixture],{stdin:'pipe',stdout:'pipe',stderr:'pipe'});
  const reader=daemon.stdout.getReader();let text='';
  const ready=(async()=>{while(!text.includes('\n')){const chunk=await reader.read();if(chunk.done)throw new Error('real fixture closed before readiness');text+=new TextDecoder().decode(chunk.value);}return JSON.parse(text.split('\n')[0]!);})();
  let timer:ReturnType<typeof setTimeout>|undefined;
  const info=await Promise.race([ready,new Promise<never>((_,reject)=>{timer=setTimeout(()=>reject(new Error('real fixture readiness timed out')),3000);})]).finally(()=>clearTimeout(timer));
  const name='#helper@fixture';
  const transport=new StdioClientTransport({command:process.execPath,args:[join(import.meta.dir,'server.ts')],stderr:'pipe',env:{PATH:process.env.PATH??'',AGENT_BUS_NAME:name,AGENT_BUS_ADDR:info.addr,AGENT_BUS_TOKEN:info.tokens[name],AGENT_BUS_BIGFILE_DIR:scratch}});
  client=new Client({name:'big-real-test',version:'1'});await client.connect(transport);
  const call=async(tool:string,args:Record<string,unknown>)=>{
   const reply:any=await client!.callTool({name:tool,arguments:args});
   expect(reply.isError??false).toBe(false);return JSON.parse(reply.content[0].text);
  };
  const input=join(scratch,'input'),output=join(scratch,'output'),bytes=Buffer.alloc(1048577,255);
  await writeFile(input,bytes);const sha256=createHash('sha256').update(bytes).digest('hex');
  const up=await call('ab_bigfile_put',{record:name,name:'raw',input_path:input,ttl:'24h'});
  expect(up.size).toBe(bytes.length);expect(up.sha256).toBe(sha256);expect(Object.keys(up).sort()).toEqual(['expires','held','name','record','replaced','sha256','size']);
  const down=await call('ab_bigfile_get',{record:name,name:'raw',output_path:output});
  expect(down.sha256).toBe(sha256);expect(down.size).toBe(bytes.length);expect(await readFile(output)).toEqual(bytes);
  expect((await call('ab_bigfile_ls',{record:name})).keys.map((key:any)=>key.name)).toEqual(['raw']);
  expect((await call('ab_bigfile_rm',{record:name,name:'raw'})).deleted).toBe(true);
 }finally{
  await client?.close();daemon?.stdin.end();
  let timer:ReturnType<typeof setTimeout>|undefined;
  if(daemon)await Promise.race([daemon.exited,new Promise<never>((_,reject)=>{timer=setTimeout(()=>{daemon!.kill();reject(new Error('real fixture did not stop'));},3000);})]).finally(()=>clearTimeout(timer));
  await rm(scratch,{recursive:true,force:true});
 }
},30000);

test('MCP large file puts preserve early refusal status and cause',async()=>{
 const scratch=await mkdtemp(resolve(import.meta.dir,'../../tmp/mcp-early-big-'));
 const input=join(scratch,'input');await writeFile(input,Buffer.alloc(24*1024*1024,255));
 let status=403,calls=0;
 const daemon=Bun.serve({port:0,hostname:'127.0.0.1',fetch:(req)=>{if(new URL(req.url).pathname==='/identity')return Response.json({protocol:1,protocol_min:1});calls++;return Response.json({error:'early file refusal cause'},{status});}});
 const bus=new Bus({AGENT_BUS_NAME:'#mcp@fixture',AGENT_BUS_ADDR:`http://127.0.0.1:${daemon.port}`,AGENT_BUS_TOKEN:'fixture-only',AGENT_BUS_BIGFILE_DIR:scratch});
 try{
  for(status of [403,413,503]){
   try{await bus.bigPut('r','n',input);throw new Error('refused file upload accepted');}
   catch(error:any){expect(error.status).toBe(status);expect(error.message).toContain('early file refusal cause');}
  }
  expect(calls).toBe(3);
 }finally{daemon.stop(true);await rm(scratch,{recursive:true,force:true});}
});

async function protocolFixture(identity:unknown,run:(bus:Bus,calls:string[])=>Promise<void>,refusal?:[number,string|undefined]){
 const calls:string[]=[];
 const server=Bun.serve({port:0,hostname:'127.0.0.1',fetch(req){const path=new URL(req.url).pathname;calls.push(path);if(path==='/identity')return Response.json(identity);if(refusal)return Response.json({error:'cause',kind:refusal[1]},{status:refusal[0]});return Response.json({you:'owner@test'});}});
 try{await run(new Bus({AGENT_BUS_ADDR:`http://127.0.0.1:${server.port}`,AGENT_BUS_TOKEN:'fixture',AGENT_BUS_NAME:'owner@test'}),calls);}finally{server.stop(true);}
}
test('MCP protocol handshake refuses too-new and malformed before application requests',async()=>{
 for(const [identity,error] of [[{protocol:2,protocol_min:2},'daemon too new'],[{protocol:1.5},'protocol'],[{protocol:-1},'protocol'],[{protocol:null},'protocol'],[{protocol:true},'protocol'],[{protocol:1,protocol_min:2},'protocol_min']] as const){await protocolFixture(identity,async(bus,calls)=>{await expect(bus.status()).rejects.toThrow(error);expect(calls).toEqual(['/identity']);});}
});
test('MCP protocol compatibility warns once only for old or absent',async()=>{
 const original=console.warn,warnings:string[]=[];console.warn=(...a)=>{warnings.push(a.join(' '));};
 try{for(const [identity,old] of [[{protocol:1,protocol_min:1},false],[{protocol:2,protocol_min:1},false],[{},true],[{protocol:0,protocol_min:0},true]] as const){warnings.length=0;await protocolFixture(identity,async(bus,calls)=>{await bus.status();await bus.status();expect(calls[0]).toBe('/identity');expect(warnings.length).toBe(old?1:0);});}}finally{console.warn=original;}
});
test('MCP protocol refusal kinds and 500 do not lose the daemon cause',async()=>{
 for(const identity of [{},{protocol:1,protocol_min:1}])for(const [code,kind] of [[404,'unknown'],[404,'no_route'],[503,'unavailable'],[500,undefined]] as const){await protocolFixture(identity,async bus=>{try{await bus.status();throw new Error('refusal accepted');}catch(e){expect(e).toBeInstanceOf(BusError);const err=e as BusError;expect(err.status).toBe(code);expect(err.kind).toBe(kind);expect(err.message).toBe(code===500?'internal daemon error':!('protocol'in identity)&&kind==='no_route'?'daemon too old for GET /status':'cause');}},[code,kind]);}
});

test('MCP tool errors surface refusal kind and cause',async()=>{
 const {call}=await face((_req,url)=>url.pathname==='/send'?Response.json({error:'store unavailable',kind:'unavailable'},{status:503}):new Response(null,{status:204}));
 const got=await call('ab_send',{to:'#peer@test',text:'fixture'});
 expect(got.isError).toBe(true);expect(got.text).toContain('unavailable');expect(got.text).toContain('store unavailable');expect(got.text).toContain('503');
});

// A daemon older than this face (C1's version check) says so where the model reads it: once, in a tool
// result; stderr keeps the warning, and a compatible daemon adds nothing.
const tooOld = "bus daemon is older than this tool";
const warned = "daemon too old: protocol is older than this client";
const queued: Handler = () => Response.json(message);

test("a too-old daemon's warning reaches the model once, in a tool result, and stderr keeps it", async () => {
  const { call, stderr } = await face(queued, {}, undefined, {});
  const first = await call("ab_consume", { wait: "0s" });
  expect(first.text).toContain("queued-body");
  expect(first.text).toContain(tooOld);
  const second = await call("ab_consume", { wait: "0s" });
  expect(second.text).not.toContain(tooOld);
  expect(stderr.join("")).toContain(warned);
});

test("a compatible daemon adds nothing to a tool result", async () => {
  const { call, stderr } = await face(queued);
  const got = await call("ab_consume", { wait: "0s" });
  expect(got.text).not.toContain(tooOld);
  expect(stderr.join("")).not.toContain(warned);
});
