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

import { spawn, type ChildProcess } from "node:child_process";
import { startOwnedServer } from "./runtime-listener.ts";

type ServerChild = { proc: ChildProcess };
function runtimeChild(port: number, listen: boolean): ServerChild {
  return { proc: spawn(process.execPath, ["-e", listen
    ? `Bun.serve({hostname:"127.0.0.1",port:${port},fetch:()=>new Response("owned")});`
    : "setInterval(()=>{},1000)"], { stdio: "ignore" }) };
}
async function stopRuntime(child: ServerChild) {
  if (child.proc.exitCode !== null || child.proc.signalCode !== null) return;
  await new Promise<void>(resolve => { child.proc.once("exit", () => resolve()); child.proc.kill("SIGKILL"); });
}

test("foreign listeners never receive credentials, including on the retry", async () => {
  const foreign: ReturnType<typeof Bun.serve>[] = [];
  const children: ServerChild[] = [];
  const headers: (string | null)[] = [];
  let error = "";
  try {
    try {
      const result = await startOwnedServer(port => {
        foreign.push(Bun.serve({ hostname: "127.0.0.1", port, fetch: request => {
          headers.push(request.headers.get("authorization")); return new Response("foreign");
        } }));
        const child = runtimeChild(port, false); children.push(child); return child;
      }, stopRuntime);
      // Exactly what an adapter would do once startup declares the port safe.
      await fetch(`http://127.0.0.1:${result.port}/config`, { headers: { authorization: "Bearer fixture-secret" } });
    } catch (e) { error = String(e); }
    expect(headers).toEqual([]);
    expect(error).toContain("is held by another process; not sending credentials");
    expect(foreign).toHaveLength(2);
  } finally {
    for (const child of children) await stopRuntime(child);
    for (const server of foreign) server.stop(true);
  }
});

test("a port collision retries once and authenticates only to our runtime", async () => {
  let foreign: ReturnType<typeof Bun.serve> | undefined;
  const children: ServerChild[] = [];
  const headers: (string | null)[] = [];
  try {
    const result = await startOwnedServer(port => {
      const collision = children.length === 0;
      if (collision) foreign = Bun.serve({ hostname: "127.0.0.1", port, fetch: request => {
        headers.push(request.headers.get("authorization")); return new Response("foreign");
      } });
      const child = runtimeChild(port, !collision); children.push(child); return child;
    }, stopRuntime);
    const response = await fetch(`http://127.0.0.1:${result.port}/config`, { headers: { authorization: "Bearer fixture-secret" } });
    expect(await response.text()).toBe("owned");
    expect(headers).toEqual([]);
    expect(children).toHaveLength(2);
  } finally {
    for (const child of children) await stopRuntime(child);
    foreign?.stop(true);
  }
});
