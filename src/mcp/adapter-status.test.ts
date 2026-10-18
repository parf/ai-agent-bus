import { expect, test } from "bun:test";
import { Codex } from "./codex.ts";
import { Opencode } from "./opencode.ts";
import { Opencode1 } from "./opencode1.ts";

// The launcher prints an adapter line red unless the adapter marks it as a
// status line, so "attached" must arrive marked and a failure must not.
type Line = { s: string; ok?: boolean };

test("opencode's attach line is a status line, not a failure", async () => {
  const server = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: (req) =>
    new URL(req.url).pathname === "/api/config" ? Response.json([]) : new Response("gone", { status: 500 }) });
  const lines: Line[] = [];
  const o = new Opencode("/tmp", (s, ok) => lines.push({ s, ok }), `http://127.0.0.1:${server.port}`);
  try {
    expect(await o.probe()).toBe(true);
    await o.start();
    const attached = lines.filter((l) => l.s.startsWith("opencode: attached to"));
    expect(attached).toHaveLength(1);
    expect(attached[0]!.ok).toBe(true);
  } finally {
    o.stop();
    server.stop(true);
  }
});

test("opencode's probe refuses a server that does not accept it", async () => {
  // v1 servers answer /config with the web UI; only /api/config with the
  // password says v2-and-allowed, so a password-less probe must stay false.
  const server = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: (req) => {
    const url = new URL(req.url);
    if (url.pathname !== "/api/config") return new Response("html", { status: 200 });
    return req.headers.get("authorization") ? Response.json([]) : new Response("unauthorized", { status: 401 });
  } });
  const bare = new Opencode("/tmp", () => {}, `http://127.0.0.1:${server.port}`);
  const authed = new Opencode("/tmp", () => {}, `http://127.0.0.1:${server.port}`, "sesame");
  try {
    expect(await bare.probe()).toBe(false);
    expect(await authed.probe()).toBe(true);
  } finally {
    bare.stop();
    authed.stop();
    server.stop(true);
  }
});

test("opencode unwraps v2 pages and keeps only this directory's own sessions", async () => {
  const server = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: (req) => {
    const url = new URL(req.url);
    if (url.pathname !== "/api/session") return new Response("no", { status: 404 });
    // opencode 2 filters children out when asked: a page of 50 full of a
    // subagent's children otherwise left no session to resume.
    if (url.searchParams.get("parentID") !== "null") return Response.json({ data: Array.from({ length: 50 }, (_, i) => ({ id: `child-${i}`, parentID: "own", location: { directory: "/tmp" } })), cursor: { previous: null, next: "more" } });
    return Response.json({ data: [
      { id: "own", location: { directory: "/tmp" } },
      { id: "child", parentID: "own", location: { directory: "/tmp" } },
      { id: "elsewhere", location: { directory: "/elsewhere" } },
      { id: "legacy-no-directory" },
    ], cursor: { previous: null, next: null } });
  } });
  const o = new Opencode("/tmp", () => {}, `http://127.0.0.1:${server.port}`);
  try {
    expect((await o.sessions()).map(s => s.id)).toEqual(["own", "legacy-no-directory"]);
  } finally {
    o.stop();
    server.stop(true);
  }
});

test("opencode delivers one queued text prompt and acknowledges on acceptance", async () => {
  let seen: any;
  const server = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: async (req) => {
    const url = new URL(req.url);
    if (url.pathname !== "/api/session/ses_x/prompt" || req.method !== "POST") return new Response("no", { status: 404 });
    seen = await req.json();
    return Response.json({ data: { id: "msg_1", sessionID: "ses_x" } });
  } });
  const o = new Opencode("/tmp", () => {}, `http://127.0.0.1:${server.port}`);
  try {
    o.bind("ses_x");
    await o.deliver("hello bus", "m-7");
    expect(seen.text).toBe("hello bus");
    expect(seen.metadata).toEqual({ source: "agent-bus", message_id: "m-7" });
    expect(seen.delivery).toBe("queue");
  } finally {
    o.stop();
    server.stop(true);
  }
});

test("opencode1's attach line is a status line, and a dropped event stream is not", async () => {
  const server = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: (req) =>
    new URL(req.url).pathname === "/config" ? Response.json({}) : new Response("gone", { status: 500 }) });
  const lines: Line[] = [];
  const o = new Opencode1("/tmp", (s, ok) => lines.push({ s, ok }), `http://127.0.0.1:${server.port}`);
  try {
    await o.start();
    const attached = lines.filter((l) => l.s.startsWith("opencode: attached to"));
    expect(attached).toHaveLength(1);
    expect(attached[0]!.ok).toBe(true);
    for (let i = 0; i < 100 && !lines.some((l) => l.s.includes("event stream dropped")); i++) await Bun.sleep(10);
    const dropped = lines.find((l) => l.s.includes("event stream dropped"));
    expect(dropped).toBeDefined();
    expect(dropped!.ok).toBeFalsy();
  } finally {
    o.stop();
    server.stop(true);
  }
});

test("codex's attach line is a status line", async () => {
  const server = Bun.serve({
    port: 0, hostname: "127.0.0.1",
    fetch: (req, s) => (s.upgrade(req) ? undefined : new Response("no", { status: 400 })),
    websocket: { message(ws, raw) {
      const m = JSON.parse(String(raw));
      if (m.id !== undefined) ws.send(JSON.stringify({ jsonrpc: "2.0", id: m.id, result: {} }));
    } },
  });
  const lines: Line[] = [];
  const c = new Codex("/tmp", (s, ok) => lines.push({ s, ok }), `ws://127.0.0.1:${server.port}`, "t");
  try {
    await c.start();
    const attached = lines.filter((l) => l.s.startsWith("codex: attached to"));
    expect(attached).toHaveLength(1);
    expect(attached[0]!.ok).toBe(true);
  } finally {
    c.stop();
    server.stop(true);
  }
});
