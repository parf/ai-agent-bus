import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { identity, startFixture, type Fixture } from "./daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";

// RT25(b): while Slack paginates the workspace's users and channels it answers
// `status: starting: <what>` and the help asks from an asks-only read, and an
// ordinary message stays queued until it serves.
let fx: Fixture;
let agent: Subprocess;
let provider: ReturnType<typeof Bun.serve>;
let open!: () => void;
const gate = new Promise<void>(done => (open = done));
const name = `#slack-${process.pid}@t`;
let callerToken = "";

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  const agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  provider = Bun.serve({
    port: 0, hostname: "127.0.0.1",
    async fetch(request) {
      const path = new URL(request.url).pathname;
      if (path.endsWith("/users.list")) { await gate; return Response.json({ ok: true, members: [] }); }
      if (path.endsWith("/conversations.list")) return Response.json({ ok: true, channels: [] });
      return Response.json({ ok: true, user: "b", user_id: "U1", team: "T", url: "u" });
    },
  });
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=xoxb-fixture\nSLACK_APP_TOKEN=xapp-fixture\nSLACK_API_URL=http://127.0.0.1:${provider.port}/api/\n` });
  agent = spawn(["bun", "slack.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: agentToken },
    stderr: Bun.file(join(fx.dir, "agent.log")), stdout: Bun.file(join(fx.dir, "agent.log")),
  });
}, 30000);

afterAll(async () => {
  open();
  agent?.kill("SIGKILL");
  await Bun.sleep(50);
  provider?.stop(true);
  await fx?.stop();
}, 15000);

const ask = async (bus: Bus, kind: "status" | "help"): Promise<string> => {
  const outcome = await bus.call(name, "", { timeout: 4000, ask: kind });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
};

test("Slack answers starting while it loads the directory, keeps ordinary messages queued, then serves them", async () => {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  let status = "";
  for (let i = 0; i < 100 && !status.includes("loading the workspace"); i++) {
    status = await ask(bus, "status");
    if (!status.includes("loading the workspace")) await Bun.sleep(100);
  }
  expect(status).toBe("starting: loading the workspace's users and channels");
  expect(await ask(bus, "help")).toContain("Slack");
  // An ordinary message is not taken by the asks-only read: it waits, and a status ask beside it is still answered.
  const pending = bus.call(name, "help", { timeout: 15000 });
  let settled = false;
  void pending.then(() => (settled = true));
  await Bun.sleep(1500);
  expect(settled).toBe(false);
  const record = await fx.call(fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
  expect(record.queued).toBe(1);
  expect(await ask(bus, "status")).toStartWith("starting: ");
  open();
  const outcome = await pending;
  expect(outcome.kind).toBe("answer");
  const after = await fx.call(fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
  expect(after.queued ?? 0).toBe(0);
}, 40000);
