import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { identity, startFixture, type Fixture } from "./daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";

// Slack stays silent past the start-up wait (here 1.5 s instead of 20 s): the
// agent starts anyway, warns once, says "not ready" in status, and its
// background asking makes status ok again when Slack answers. When the identity
// is lost again later (a changed token) it is asked for again without waiting
// for an event to do it.
let fx: Fixture;
let agent: Subprocess;
let provider: ReturnType<typeof Bun.serve>;
const name = `#slack-late-${process.pid}@t`;
let callerToken = "";
let failUntil = 0;
let sockets: any[] = [];
const log = () => Bun.file(join(fx.dir, "agent.log")).text();
const secret = (bot: string) => fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=${bot}\nSLACK_APP_TOKEN=xapp-fixture\nSLACK_API_URL=http://127.0.0.1:${provider.port}/api/\n` });

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
      if (path.endsWith("/auth.test")) return Date.now() < failUntil ? Response.json({ ok: false, error: "ratelimited" }) : Response.json({ ok: true, user: "b", user_id: "UBOT", team: "T", url: "u" });
      if (path.endsWith("/users.list")) return Response.json({ ok: true, members: [] });
      if (path.endsWith("/conversations.list")) return Response.json({ ok: true, channels: [] });
      return Response.json({ ok: false, error: "no route" });
    },
    websocket: { open(ws) { sockets.push(ws); }, message() {}, close(ws) { sockets = sockets.filter(s => s !== ws); } },
  });
  await secret("xoxb-first");
  failUntil = Date.now() + 6000;
  agent = spawn(["bun", "slack.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: agentToken, SLACK_AGENT_IDENTITY_GIVE_MS: "1500" },
    stderr: Bun.file(join(fx.dir, "agent.log")), stdout: Bun.file(join(fx.dir, "agent.log")),
  });
}, 30000);

afterAll(async () => {
  agent?.kill("SIGKILL");
  await Bun.sleep(50);
  provider?.stop(true);
  await fx?.stop();
}, 15000);

const status = async (bus: Bus): Promise<string> => {
  const outcome = await bus.call(name, "", { timeout: 4000, ask: "status" });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
};
const until = async (bus: Bus, want: (s: string) => boolean, ms: number): Promise<string> => {
  let s = "";
  for (const end = Date.now() + ms; Date.now() < end; await Bun.sleep(100)) { s = await status(bus); if (want(s)) return s; }
  return s;
};

test("past the start-up wait it warns once, says not ready, and the background asking makes it ok", async () => {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  expect(await until(bus, s => s.startsWith("not ready"), 20000)).toBe("not ready: Slack has not said who the agent is yet");
  expect((await log()).split("identity unknown, retrying").length - 1).toBe(1);
  expect(await until(bus, s => s === "ok", 30000)).toBe("ok");
}, 60000);

test("an identity lost again is asked for again with no event to prompt it", async () => {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  expect(await until(bus, s => s === "ok", 5000)).toBe("ok");
  for (let i = 0; i < 100 && sockets.length === 0; i++) await Bun.sleep(50);
  failUntil = Date.now() + 2500;
  await secret("xoxb-second"); // a new token: the identity learned for the old one no longer counts
  for (const ws of sockets) ws.send(JSON.stringify({ envelope_id: "e1", type: "events_api", payload: { event_id: "Ev1", event: { type: "message", channel: "C1", user: "U1", text: "hello", ts: "1.1" } } }));
  expect(await until(bus, s => s.startsWith("not ready"), 10000)).toStartWith("not ready");
  expect(await until(bus, s => s === "ok", 30000)).toBe("ok"); // and with no second event
}, 90000);
