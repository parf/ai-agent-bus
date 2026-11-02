import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { identity, startFixture, type Fixture } from "./daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";

// The agent learns who it is in Slack (auth.test) before it takes events. While
// Slack does not answer it says so in `status`; once it does, `status` is ok and
// the agent serves, without a restart and without the failure having cost it
// the start.
let fx: Fixture;
let agent: Subprocess;
let provider: ReturnType<typeof Bun.serve>;
const name = `#slack-id-${process.pid}@t`;
let callerToken = "";
const failMs = Number(process.env.SLACK_TEST_IDENTITY_FAIL_MS ?? "3000");
let failUntil = 0;
let authCalls = 0;

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
      if (path.endsWith("/auth.test")) {
        authCalls++;
        return Date.now() < failUntil ? new Response("<html><h1>502 Bad Gateway</h1></html>", { status: 502, headers: { "content-type": "text/html" } }) : Response.json({ ok: true, user: "b", user_id: "UBOT", team: "T", url: "u" });
      }
      if (path.endsWith("/users.list")) return Response.json({ ok: true, members: [] });
      if (path.endsWith("/conversations.list")) return Response.json({ ok: true, channels: [] });
      return Response.json({ ok: false, error: "no route" });
    },
    websocket: { open() {}, message() {}, close() {} },
  });
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=xoxb-fixture\nSLACK_APP_TOKEN=xapp-fixture\nSLACK_API_URL=http://127.0.0.1:${provider.port}/api/\n` });
  failUntil = Date.now() + failMs;
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

const status = async (bus: Bus): Promise<string> => {
  const outcome = await bus.call(name, "", { timeout: 4000, ask: "status" });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
};

test("status says what it waits for while Slack does not name the agent, then ok, and the failure never stopped the start", async () => {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  let waiting = "";
  for (let i = 0; i < 100 && !waiting.includes("waiting for Slack"); i++) {
    const s = await status(bus);
    if (s.includes("waiting for Slack")) waiting = s;
    else await Bun.sleep(50);
  }
  expect(waiting).toStartWith("starting: waiting for Slack to say who the agent is");
  let last = "";
  for (let i = 0; i < 300 && last !== "ok"; i++) {
    last = await status(bus);
    if (last !== "ok") await Bun.sleep(100);
  }
  expect(last).toBe("ok");
  expect(authCalls).toBeGreaterThan(1); // it asked again, rather than starting without the answer
  expect(await Bun.file(join(fx.dir, "agent.log")).text()).not.toContain("identity unknown");
}, 60000);
