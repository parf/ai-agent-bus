import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { call as busCall, identity, startFixture, type Fixture } from "../slack/daemon.ts";
import { Bus, BusError, type CallOutcome } from "../../clients/typescript/src/index.js";
import { configFault, secretFault } from "./config.ts";

// TG.1: the agent registers, serves help, and reads its token from the bus
// secret — never from the process env, which belongs to the runner
// (Plans/R1.0-Release/telegram/TODO.md TG.1). Tests carry their own generous
// timeouts: the fixture daemon's bus takes a few seconds to come up behind
// its socket.
const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let callerToken: string;
let fake: { port: number; saw: string[] };
let fakeServer: ReturnType<typeof Bun.serve>;
const name = `#telegram-${process.pid}@t`;
const bare = `#telegram-empty-${process.pid}@t`;
const bareTokenless = `#telegram-notoken-${process.pid}@t`;
let agentToken = "";
let bareToken = "";
let bareTokenlessToken = "";
const agents: Subprocess[] = [];

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-${process.pid}@t`);
  for (const [agent, kind] of [[name, "main"], [bare, "bare"], [bareTokenless, "bare"]] as const) {
    await fx.call(fx.ownerToken, "POST", "/register", { name: agent, kind: "agent", allow: ["*"] });
    const { token } = await fx.call(fx.ownerToken, "POST", "/token", { name: agent });
    if (kind === "main") agentToken = token;
    else if (agent === bare) bareToken = token;
    else bareTokenlessToken = token;
  }
  fake = { port: 0, saw: [] };
  fakeServer = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    fake.saw.push(new URL(req.url).pathname);
    return Response.json({ ok: true, result: { id: 42, is_bot: true, username: "fixture_bot", first_name: "Fixture Bot" } });
  } });
  fake.port = fakeServer.port;
}, 30000);

afterAll(async () => {
  for (const a of agents) a.kill("SIGKILL");
  await Bun.sleep(50);
  fakeServer?.stop(true);
  await fx?.stop();
}, 15000);

/** Start the agent under the given name and process env beside its credential. */
function startAgent(token: string, agentName: string, env: Record<string, string> = {}): Subprocess {
  const proc = spawn(["bun", "telegram.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: agentName, AGENT_BUS_TOKEN: token, ...env },
    stderr: Bun.file(join(fx.dir, `agent-${agents.length}.log`)),
    stdout: Bun.file(join(fx.dir, `agent-${agents.length}.log`)),
  });
  agents.push(proc);
  return proc;
}

async function stopAgents(): Promise<void> {
  for (const a of agents) a.kill("SIGTERM");
  await Bun.sleep(200);
}

async function call(body: string, token = callerToken, to = name): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token });
  const outcome: CallOutcome = await bus.call(to, body, { timeout: 10000 });
  if (outcome.kind === "answer") return outcome.message.body;
  if (outcome.kind === "done") return "";
  return "(timeout)";
}

async function waitForRegistration(agent = name): Promise<void> {
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await busCall(fx.socket, fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(agent)}`);
      if (String(rec?.descr ?? "").startsWith("telegram:")) return;
    } catch { /* not yet */ }
    await Bun.sleep(50);
  }
  throw new Error("the agent never registered");
}

/** The fixture secret: a token plus the fake Telegram's URL. */
const secret = (extra: Record<string, string | undefined>) =>
  Object.entries({ TELEGRAM_BOT_TOKEN: "111:fixture-bot", TELEGRAM_API_URL: `http://127.0.0.1:${fake.port}`, ...extra })
    .filter(([, v]) => v !== undefined).map(([k, v]) => `${k}=${v}`).join("\n");
const setSecret = (agent: string, value: string) => fx.call(fx.ownerToken, "POST", "/secret", { name: agent, secret: value });

t("the agent registers, serves help and whoami through the fake Telegram", async () => {
  await setSecret(name, secret({}));
  startAgent(agentToken, name);
  await waitForRegistration();
  const help = await call("help");
  expect(help).toContain("post [<chat>]");
  expect(help).toContain("sub list");
  expect(help).toContain("whoami");
  expect(await call("whoami")).toBe("@fixture_bot (id 42)");
  // The token is a URL path segment with Telegram, not a header: the fake
  // saw the secret's token, spelled exactly as the Bot API documents.
  expect(fake.saw.at(-1)).toBe(`/bot111:fixture-bot/getMe`);
});

t("a missing token is named, from the bus secret alone", async () => {
  await setSecret(name, secret({ TELEGRAM_BOT_TOKEN: undefined }));
  expect(await call("whoami")).toBe("error: TELEGRAM_BOT_TOKEN is not set");
});

t("with no bus secret at start it exits 78, not configured, saying why", async () => {
  const proc = startAgent(bareToken, bare);
  const code = await Promise.race([proc.exited, Bun.sleep(15_000).then(() => -1)]);
  expect(code).toBe(78);
  expect(await Bun.file(join(fx.dir, `agent-${agents.length - 1}.log`)).text()).toContain("telegram: the bus secret is not set");
});

t("with a secret that has no token it exits 78, naming the key", async () => {
  await setSecret(bareTokenless, "TELEGRAM_API_URL=http://127.0.0.1:1\n");
  const proc = startAgent(bareTokenlessToken, bareTokenless);
  const code = await Promise.race([proc.exited, Bun.sleep(15_000).then(() => -1)]);
  expect(code).toBe(78);
  expect(await Bun.file(join(fx.dir, `agent-${agents.length - 1}.log`)).text()).toContain("telegram: TELEGRAM_BOT_TOKEN is not set");
});

t("the fake sees only the bus secret's token, and no sentinel is logged or stored", async () => {
  await stopAgents(); // only the agent under test is serving: no earlier one answers in its place
  await setSecret(name, secret({ TELEGRAM_BOT_TOKEN: "111:secret-sentinel" }));
  startAgent(agentToken, name, { TELEGRAM_BOT_TOKEN: "111:env-sentinel", TELEGRAM_API_URL: "http://127.0.0.1:1" });
  await waitForRegistration();
  expect(await call("whoami")).toBe("@fixture_bot (id 42)");
  expect(fake.saw.at(-1)).toBe(`/bot111:secret-sentinel/getMe`);
  await Bun.sleep(150);
  for (let i = 0; i < agents.length; i++) {
    const log = await Bun.file(join(fx.dir, `agent-${i}.log`)).text();
    expect(log).not.toContain("sentinel");
    expect(log).not.toContain("111:");
  }
  // The store, previews included: the agent writes nothing here, and a token
  // planted as a VALUE must be caught too, not only as a name.
  const listing = await fx.call(fx.ownerToken, "GET", `/kv/list?record=${encodeURIComponent(name)}`);
  expect(JSON.stringify(listing)).not.toContain("sentinel");
  expect(JSON.stringify(listing)).not.toContain("111:");
  for (const entries of Object.values(listing.values ?? {})) expect(entries).toEqual([]); // no writes at all
});

t("an environment endpoint receives nothing: the endpoint is the secret's alone", async () => {
  await stopAgents(); // only the agent under test is serving
  // The secret names one endpoint, the environment another; the call is
  // awaited, so a zero count is a request that never came, not one not yet
  // sent. No network: the secret's endpoint is a second fake.
  let secretSaw = 0;
  const own = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: () => { secretSaw++; return Response.json({ ok: true, result: { id: 7, is_bot: true, username: "own_bot" } }); } });
  const before = fake.saw.length;
  try {
    await setSecret(name, secret({ TELEGRAM_API_URL: `http://127.0.0.1:${own.port}` }));
    startAgent(agentToken, name, { TELEGRAM_API_URL: `http://127.0.0.1:${fake.port}` });
    await waitForRegistration();
    expect(await call("whoami")).toBe("@own_bot (id 7)");
    expect(secretSaw).toBeGreaterThanOrEqual(1);
    expect(fake.saw.length).toBe(before); // the env endpoint answered nothing
  } finally {
    own.stop(true);
  }
});

t("a transport failure names the method, never the URL that carries the token", async () => {
  // Nothing listens there; the error must not carry the URL, whose path is
  // the token. The call is awaited: the answer is already in.
  await setSecret(name, secret({ TELEGRAM_API_URL: "http://127.0.0.1:1" }));
  const answer = await call("whoami");
  expect(answer).toContain("error:");
  expect(answer).not.toContain("111:fixture-bot");
  expect(answer).not.toContain("/bot");
});

t("registering does not touch the record's ACL", async () => {
  const guarded = `#telegram-guarded-${process.pid}@t`;
  await fx.call(fx.ownerToken, "POST", "/register", { name: guarded, kind: "agent", allow: ["#one@t"], descr: "before" });
  const guardedToken = (await fx.call(fx.ownerToken, "POST", "/token", { name: guarded })).token;
  await fx.call(fx.ownerToken, "POST", "/secret", { name: guarded, secret: secret({}) });
  startAgent(guardedToken, guarded);
  for (let i = 0; i < 200; i++) {
    const rec = await fx.call(fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(guarded)}`);
    if (String(rec?.descr ?? "").startsWith("telegram:")) {
      expect(rec.allow).toEqual(["#one@t"]); // the update changed the description, not the grants
      return;
    }
    await Bun.sleep(50);
  }
  throw new Error("the guarded agent never registered");
});

// The configuration is validated at start, naming the bad key, so a mistake
// is found before the first call that needs it (telegram-build, 2026-10-18).
// Each case runs its own record: the gate is a start-time one.
const startConfigured = async (label: string, config: unknown): Promise<Subprocess> => {
  const agent = `#telegram-cfg-${label}-${process.pid}@t`;
  await fx.call(fx.ownerToken, "POST", "/register", { name: agent, kind: "agent", allow: ["*"] });
  const token = (await fx.call(fx.ownerToken, "POST", "/token", { name: agent })).token;
  await fx.call(fx.ownerToken, "POST", "/secret", { name: agent, secret: "TELEGRAM_BOT_TOKEN=111:fixture-bot\n" });
  await fx.call(fx.ownerToken, "POST", "/configure", { name: agent, config });
  return startAgent(token, agent);
};

// The fault classification: only a 404 means absence; the env-file fault is
// named as malformed; everything else is the bus failing, which must never
// read as unconfigured (tg1-review P2-1, P2-2).
test("secret and configuration faults classify by cause, not by catching everything", () => {
  expect(secretFault(new BusError(404, "no secret"))).toEqual({ kind: "absent", message: "the bus secret is not set" });
  const parse = secretFault(new Error("the bus secret: line 3: a reference ($NAME) is never expanded"));
  expect(parse).toEqual({ kind: "malformed", message: "the bus secret is not an env file: line 3: a reference ($NAME) is never expanded" });
  expect(secretFault(new BusError(403, "not yours"))).toEqual({ kind: "bus", message: "the bus did not answer" });
  expect(secretFault(new BusError(503, "down")).kind).toBe("bus");
  expect(secretFault(new TypeError("surprise")).kind).toBe("bus");
  expect(configFault(new BusError(404, "no configuration"))).toEqual({ kind: "none" });
  expect(configFault(new BusError(500, "down"))).toEqual({ kind: "bus", message: "the bus did not answer" });
  expect(configFault(new TypeError("surprise")).kind).toBe("bus");
});

// The malformed-secret branch cannot be produced through the API: POST
// /secret validates the env file on write, with the same grammar
// (internal/envfile) the agent's parseEnvFile implements. It stays for
// secrets that reach the store without that check — restore paths — and is
// pinned by the classifier test above.

t("no bus at all at start is also not unconfigured: nonzero, never 78", async () => {
  const agent = `#telegram-deadaddr-${process.pid}@t`;
  await fx.call(fx.ownerToken, "POST", "/register", { name: agent, kind: "agent", allow: ["*"] });
  const token = (await fx.call(fx.ownerToken, "POST", "/token", { name: agent })).token;
  await fx.call(fx.ownerToken, "POST", "/secret", { name: agent, secret: "TELEGRAM_BOT_TOKEN=111:fixture-bot\n" });
  const proc = spawn(["bun", "telegram.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: "127.0.0.1:1", AGENT_BUS_NAME: agent, AGENT_BUS_TOKEN: token },
    stderr: Bun.file(join(fx.dir, `agent-${agents.length}.log`)),
    stdout: Bun.file(join(fx.dir, `agent-${agents.length}.log`)),
  });
  agents.push(proc);
  const code = await Promise.race([proc.exited, Bun.sleep(15_000).then(() => -1)]);
  expect(code).not.toBe(78);
  expect(code).not.toBe(0);
  expect(code).not.toBe(-1); // it did exit
});

t("a well-formed configuration starts and serves", async () => {
  const proc = await startConfigured("ok", { default: "#dev", aliases: { "#dev": "-1001234567890", "@alice": "12345678" } });
  await waitForRegistration(`#telegram-cfg-ok-${process.pid}@t`);
  expect(proc.exitCode).toBeNull(); // serving, not exited
});

t("an alias key without its sigil exits 78 naming the key", async () => {
  const proc = await startConfigured("sigil", { aliases: { dev: "123" } });
  const code = await Promise.race([proc.exited, Bun.sleep(15_000).then(() => -1)]);
  expect(code).toBe(78);
  expect(await Bun.file(join(fx.dir, `agent-${agents.length - 1}.log`)).text()).toContain("aliases: dev has neither # nor @");
});

t("a non-decimal alias value exits 78 naming the key", async () => {
  const proc = await startConfigured("value", { aliases: { "#dev": "12x" } });
  const code = await Promise.race([proc.exited, Bun.sleep(15_000).then(() => -1)]);
  expect(code).toBe(78);
  expect(await Bun.file(join(fx.dir, `agent-${agents.length - 1}.log`)).text()).toContain("aliases: #dev is not a decimal chat id");
});

t("a default naming a missing alias exits 78 naming it", async () => {
  const proc = await startConfigured("default", { default: "#missing", aliases: { "#dev": "123" } });
  const code = await Promise.race([proc.exited, Bun.sleep(15_000).then(() => -1)]);
  expect(code).toBe(78);
  expect(await Bun.file(join(fx.dir, `agent-${agents.length - 1}.log`)).text()).toContain("default names #missing, which is not in aliases");
});

t("a numeric default starts undeclared, used as is", async () => {
  const proc = await startConfigured("numeric", { default: "123456" });
  await waitForRegistration(`#telegram-cfg-numeric-${process.pid}@t`);
  expect(proc.exitCode).toBeNull();
});
// A malformed stored secret cannot be produced through the API: POST
// /secret validates the env file on write, with the same grammar
// (internal/envfile) the agent's parseEnvFile implements. The branch stays
// for secrets that reach the store without that check — restore paths — and
// is pinned by the classifier test above.
