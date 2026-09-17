import { expect, test } from "bun:test";
import { randomUUID } from "node:crypto";
import { Codex } from "../mcp/codex.ts";

test("the pusher authenticates before reading its shared Codex server", async () => {
  const token = randomUUID();
  let reads = 0;
  const server = Bun.serve({
    hostname: "127.0.0.1", port: 0,
    fetch(request, server) {
      if (request.headers.get("authorization") !== `Bearer ${token}`) return new Response("unauthorized", { status: 401 });
      return server.upgrade(request) ? undefined : new Response("not a websocket", { status: 400 });
    },
    websocket: {
      message(ws, raw) {
        const m = JSON.parse(String(raw));
        if (m.method === "thread/list") reads++;
        if (m.id !== undefined) ws.send(JSON.stringify({ id: m.id, result: m.method === "thread/list" ? { data: [] } : {} }));
      },
    },
  });
  const client = new Codex(process.cwd(), () => {}, `ws://127.0.0.1:${server.port}`, token);
  try {
    await client.start();
    await client.threads();
    expect(reads).toBe(1);
  } finally { client.stop(); server.stop(true); }
});
