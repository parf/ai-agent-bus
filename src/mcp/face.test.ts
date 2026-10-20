import { afterEach, expect, test } from "bun:test";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

// The face's tools against a fake daemon: what it asks the daemon is the
// claim, so every check reads the request the daemon saw, not the face's word.
type Handler = (req: Request, url: URL) => Response | Promise<Response>;
const cleanup: (() => unknown)[] = [];
afterEach(async () => { for (const f of cleanup.splice(0).reverse()) await f(); });

async function face(route: Handler, env: Record<string, string> = {}) {
  const daemon = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: (req) => {
    const url = new URL(req.url);
    if (url.pathname === "/ls") return Response.json([]);
    if (url.pathname === "/register") return Response.json({ name: "#face@test", kind: "agent", owner: "o" });
    return route(req, url);
  } });
  cleanup.push(() => daemon.stop(true));
  const stderr: string[] = [];
  const transport = new StdioClientTransport({
    command: process.execPath, args: [`${import.meta.dir}/server.ts`], stderr: "pipe",
    env: { PATH: process.env.PATH ?? "", AGENT_BUS_ADDR: `http://127.0.0.1:${daemon.port}`, AGENT_BUS_TOKEN: "t", AGENT_BUS_NAME: "#face@test", ...env },
  });
  transport.stderr?.on("data", (d) => stderr.push(String(d)));
  const client = new Client({ name: "face-test", version: "1" });
  await client.connect(transport);
  cleanup.push(() => client.close());
  const call = async (name: string, args: Record<string, unknown>, signal?: AbortSignal) => {
    const r = await client.callTool({ name, arguments: args }, undefined, signal ? { signal } : undefined) as { content: { text: string }[]; isError?: boolean };
    return { text: r.content.map((c) => c.text).join("\n"), isError: !!r.isError };
  };
  return { call, stderr };
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
  expect(registered[0]?.tags).toEqual(["billing", "codex", "review"]);
});
