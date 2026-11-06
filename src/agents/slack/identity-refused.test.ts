import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { identity, startFixture, type Fixture } from "./daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";

// Slack refuses the agent for good (a bad token, not a "try again"): holding the start for
// the usual wait would only keep the agent from answering, so it serves at once, says
// "not ready" in status and keeps asking in the background.
let fx: Fixture;
let agent: Subprocess;
let provider: ReturnType<typeof Bun.serve>;
const name = `#slack-refused-${process.pid}@t`;
let callerToken = "";
let authCalls = 0;
let sockets: any[] = [];

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  const agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  provider = Bun.serve({
    port: 0, hostname: "127.0.0.1",
    fetch(request, server) {
      const path = new URL(request.url).pathname;
      if (path === "/socket") return server.upgrade(request) ? undefined : new Response("no", { status: 400 });
      if (path.endsWith("/apps.connections.open")) return Response.json({ ok: true, url: `ws://127.0.0.1:${provider.port}/socket` });
      if (path.endsWith("/auth.test")) { authCalls++; return Response.json({ ok: false, error: "invalid_auth" }); }
      if (path.endsWith("/users.list")) return Response.json({ ok: true, members: [] });
      if (path.endsWith("/conversations.list")) return Response.json({ ok: true, channels: [] });
      return Response.json({ ok: false, error: "no route" });
    },
    websocket: { open(ws) { sockets.push(ws); }, message() {}, close(ws) { sockets = sockets.filter(s => s !== ws); } },
  });
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=xoxb-fixture\nSLACK_APP_TOKEN=xapp-fixture\nSLACK_API_URL=http://127.0.0.1:${provider.port}/api/\n` });
  agent = spawn(["bun", "slack.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: agentToken },
    stderr: Bun.file(join(fx.dir, "agent.log")), stdout: Bun.file(join(fx.dir, "agent.log")),
  });
}, 30000);

afterAll(async () => {
  agent?.kill("SIGKILL");
  await Bun.sleep(50);
  provider?.stop(true);
  await fx?.stop();
}, 15000);

test("a refusal that will not pass does not hold the start: it serves and says not ready", async () => {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  let s = "";
  const began = Date.now();
  for (let i = 0; i < 100 && !s.startsWith("not ready"); i++) {
    const o = await bus.call(name, "", { timeout: 4000, ask: "status" });
    s = o.kind === "answer" ? o.message.body : "(timeout)";
    if (!s.startsWith("not ready")) await Bun.sleep(100);
  }
  expect(s).toBe("not ready: Slack has not said who the agent is yet (invalid_auth)");
  expect(Date.now() - began).toBeLessThan(15000); // well inside the 20 s a "try again" would be given
  const help = await bus.call(name, "help", { timeout: 5000 });
  expect(help.kind).toBe("answer"); // an ordinary call is answered, not left queued
  // It keeps asking in the background on a growing wait (1 s, doubling to 30 s), not in a burst:
  // after another 6 s there are the start-up ask plus the asks at about +1, +3 and +7 s.
  await Bun.sleep(6000);
  expect(authCalls).toBeGreaterThanOrEqual(2);
  expect(authCalls).toBeLessThanOrEqual(5);
}, 40000);

// An event that arrives while Slack refuses the token costs one ask, not the four of a try-again answer.
test("an event under a definitive refusal is not retried: one auth.test call, then the warning", async () => {
  for (let i = 0; i < 100 && sockets.length === 0; i++) await Bun.sleep(50);
  expect(sockets.length).toBeGreaterThan(0);
  const before = authCalls;
  for (const ws of sockets) ws.send(JSON.stringify({ envelope_id: "e1", type: "events_api", payload: { event_id: "Ev1", event: { type: "message", channel: "C1", user: "U1", text: "hello", ts: "1.1" } } }));
  let log = "";
  for (let i = 0; i < 100 && !log.includes("identity is still unknown"); i++) { await Bun.sleep(50); log = await Bun.file(join(fx.dir, "agent.log")).text(); }
  expect(log).toContain("identity is still unknown");
  // The event's own ask, plus at most one tick of the background asker; a retried event would make four or more.
  expect(authCalls - before).toBeGreaterThanOrEqual(1);
  expect(authCalls - before).toBeLessThanOrEqual(2);
}, 30000);
