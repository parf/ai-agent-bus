import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { call as busCall, identity, startFixture, type Fixture } from "../slack/daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";
import { help } from "./help.ts";

// TG.2's wiring, end to end: the parser dispatch answers through a real
// agent, and the start path's fault classification is proved against a
// proxy that fails the secret read — the classifier unit test pins the
// mapping, this pins that telegram.ts wires it in (tg2).

const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let callerToken: string;
let fake: { port: number };
let fakeServer: ReturnType<typeof Bun.serve>;
const name = `#telegram-t2-${process.pid}@t`;
let agentToken = "";
const agents: Subprocess[] = [];

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-t2-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  fake = { port: 0 };
  fakeServer = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch() {
    return Response.json({ ok: true, result: { id: 42, is_bot: true, username: "fixture_bot" } });
  } });
  fake.port = fakeServer.port;
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `TELEGRAM_BOT_TOKEN=111:t2-bot\nTELEGRAM_API_URL=http://127.0.0.1:${fake.port}\n` });
}, 30000);

afterAll(async () => {
  for (const a of agents) a.kill("SIGKILL");
  await Bun.sleep(50);
  fakeServer?.stop(true);
  await fx?.stop();
}, 15000);

function startAgent(token: string, agentName: string, env: Record<string, string> = {}): Subprocess {
  const proc = spawn(["bun", "telegram.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: agentName, AGENT_BUS_TOKEN: token, ...env },
    stderr: Bun.file(join(fx.dir, `t2-agent-${agents.length}.log`)),
    stdout: Bun.file(join(fx.dir, `t2-agent-${agents.length}.log`)),
  });
  agents.push(proc);
  return proc;
}

async function call(body: string): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  const outcome = await bus.call(name, body, { timeout: 10000 });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}

async function waitForRegistration(): Promise<void> {
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await busCall(fx.socket, fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
      if (String(rec?.descr ?? "").startsWith("telegram:")) return;
    } catch { /* not yet */ }
    await Bun.sleep(50);
  }
  throw new Error("the agent never registered");
}

/** A socket in front of the daemon whose secret reads fail with 503 while
 *  `failing` is on; everything else passes through. */
function flakySecretProxy(path: string) {
  let failing = true;
  const server = Bun.serve({ unix: path, async fetch(req) {
    const url = new URL(req.url);
    if (failing && url.pathname === "/secret") return new Response("the store is busy", { status: 503 });
    return fetch(`http://unix${url.pathname}${url.search}`, { method: req.method, headers: req.headers, body: req.body, unix: fx.socket } as RequestInit);
  } });
  return { set failing(v: boolean) { failing = v; }, stop: () => server.stop(true) };
}

t("a bus that does not answer the secret is not 'not configured': no exit 78, and a call says the bus", async () => {
  const proxy = flakySecretProxy(join(fx.dir, `proxy-${process.pid}.sock`));
  try {
    // Fail mode: the start path is an ordinary failure, never exit 78.
    const failed = startAgent(agentToken, name, { AGENT_BUS_ADDR: proxySock(proxy) });
    const code = await Promise.race([failed.exited, Bun.sleep(15_000).then(() => -1)]);
    expect(code).not.toBe(78);
    expect(code).not.toBe(0);
    expect(code).not.toBe(-1); // it did exit
    const log = await Bun.file(join(fx.dir, `t2-agent-${agents.length - 1}.log`)).text();
    expect(log).toContain("telegram: the bus did not answer");
    expect(log).not.toContain("is not set");
    // Pass-through: the agent serves, and the parser dispatch answers.
    proxy.failing = false;
    startAgent(agentToken, name, { AGENT_BUS_ADDR: proxySock(proxy) });
    await waitForRegistration();
    expect(await call("whoami")).toBe("@fixture_bot (id 42)"); // getMe through the proxy
    expect(await call("help")).toBe(help);                     // help parses to itself
    expect(await call("--help")).toBe(help);                   // the ask form
    expect(await call("nonsense")).toBe(help);                 // unknown parses to help
    expect(await call("sub list")).toBe(help);                 // built in TG.6: falls through for now
    // Failing again: the running agent answers the bus, never "not set".
    proxy.failing = true;
    expect(await call("whoami")).toBe("error: the bus did not answer");
  } finally {
    proxy.stop();
  }
}, 40000);

const proxySock = (p: { stop: () => void }) => p && join(fx.dir, `proxy-${process.pid}.sock`);
