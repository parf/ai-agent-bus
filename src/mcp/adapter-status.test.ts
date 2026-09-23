import { expect, test } from "bun:test";
import { Codex } from "./codex.ts";
import { Opencode } from "./opencode.ts";

// The launcher prints an adapter line red unless the adapter marks it as a
// status line, so "attached" must arrive marked and a failure must not.
type Line = { s: string; ok?: boolean };

test("opencode's attach line is a status line, not a failure", async () => {
  const server = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: (req) =>
    new URL(req.url).pathname === "/config" ? Response.json({}) : new Response("gone", { status: 500 }) });
  const lines: Line[] = [];
  const o = new Opencode("/tmp", (s, ok) => lines.push({ s, ok }), `http://127.0.0.1:${server.port}`);
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
