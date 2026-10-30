import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { call as busCall, identity, startFixture, type Fixture } from "./daemon.ts";
import { Bus, type CallOutcome } from "../../clients/typescript/src/index.js";

// S.1: the agent registers, serves help, and reads its tokens from the bus
// secret — never from the process env, which belongs to the runner
// (Plans/R1.2/done/slack/TODO.md S.1). Tests carry their own generous timeouts:
// the fixture daemon's bus takes a few seconds to come up behind its socket.
const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let callerToken: string;
let fake: { port: number; saw: { method: string; auth: string }[] };
let fakeServer: ReturnType<typeof Bun.serve>;
const name = `#slack-${process.pid}@t`;
const bare = `#slack-empty-${process.pid}@t`;
let agentToken = "";
let bareToken = "";
const agents: Subprocess[] = [];

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  await fx.call(fx.ownerToken, "POST", "/register", { name: bare, kind: "agent", allow: ["*"] });
  bareToken = (await fx.call(fx.ownerToken, "POST", "/token", { name: bare })).token;
  fake = { port: 0, saw: [] };
  fakeServer = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    fake.saw.push({ method: new URL(req.url).pathname, auth: req.headers.get("authorization") ?? "" });
    return Response.json({ ok: true, user: "agentbot", team: "Fixture Team", url: "https://fixture-team.slack.com" });
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
  const proc = spawn(["bun", "slack.ts"], {
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

async function waitForRegistration(): Promise<void> {
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await busCall(fx.socket, fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
      if (String(rec?.descr ?? "").length > 0) return;
    } catch { /* not yet */ }
    await Bun.sleep(50);
  }
  throw new Error("the agent never registered");
}

/** The fixture secret: real tokens plus the fake Slack's URL. */
const secret = (extra: Record<string, string>) => Object.entries({ SLACK_BOT_TOKEN: "xoxb-fixture-bot", SLACK_APP_TOKEN: "xapp-fixture-app", SLACK_API_URL: `http://127.0.0.1:${fake.port}/api/`, ...extra }).map(([k, v]) => `${k}=${v}`).join("\n");
const setSecret = (value: string) => fx.call(fx.ownerToken, "POST", "/secret", { name, secret: value });

t("the agent registers, serves help and whoami through the fake Slack", async () => {
  await setSecret(secret({}));
  startAgent(agentToken, name);
  await waitForRegistration();
  const help = await call("help");
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  for (const ask of ["help-config", "help-secret", "help-json"] as const) {
    const result = await bus.call(name, "", { ask, timeout: 10000 });
    expect(result.kind).toBe("answer");
    if (result.kind !== "answer") throw Error("metadata ask did not answer");
    const data = ask === "help-json" ? JSON.parse(result.message.body) : undefined;
    if (data) {
      expect(data).toHaveProperty("methods");
      expect(data).toHaveProperty("config");
      expect(data).toHaveProperty("secrets");
    }
    expect(result.message.body).not.toContain("fixture-bot");
    expect(result.message.body).not.toContain("fixture-app");
    if (ask === "help-json") {
      expect(data.secrets.map((s: {name:string}) => s.name)).toContain("SLACK_BOT_TOKEN");
      for (const key of data.secrets) expect(key.help.length).toBeGreaterThan(0);
    }
  }
  expect(help).toContain("post <channel>");
  expect(help).toContain("sub list");
  expect(help).toContain("whoami");
  expect(await call("whoami")).toBe("agentbot on Fixture Team (https://fixture-team.slack.com)");
  expect(fake.saw.at(-1)).toEqual({ method: "/api/auth.test", auth: "Bearer xoxb-fixture-bot" });
});

t("a JSON call with a key its method does not take is refused, naming the key", async () => {
  const answer = JSON.parse(await call('{"method":"whoami","channel":"C1"}'));
  expect(answer.ok).toBe(false);
  expect(answer.error).toBe('whoami takes no "channel"');
});

t("the missing tokens are named, from the bus secret alone", async () => {
  await setSecret(secret({ SLACK_BOT_TOKEN: undefined as unknown as string }).replaceAll("SLACK_BOT_TOKEN=undefined\n", ""));
  expect(await call("whoami")).toBe("error: SLACK_BOT_TOKEN is not set");
  await setSecret(secret({ SLACK_APP_TOKEN: undefined as unknown as string }).replaceAll("SLACK_APP_TOKEN=undefined\n", ""));
  expect(await call("whoami")).toBe("error: SLACK_APP_TOKEN is not set");
});

t("with no bus secret at start it exits 78, not configured, saying why", async () => {
  const proc = startAgent(bareToken, bare);
  const code = await Promise.race([proc.exited, Bun.sleep(15_000).then(() => -1)]);
  expect(code).toBe(78);
  expect(await Bun.file(join(fx.dir, `agent-${agents.length - 1}.log`)).text()).toContain("slack: the bus secret is not set");
});

/** A socket in front of the daemon whose secret reads fail with 503 while `failing` is on. */
function flakySecretProxy() {
  const path = join(fx.dir, `proxy-${agents.length}.sock`);
  const state = { failing: true, path };
  const server = Bun.serve({ unix: path, async fetch(req) {
    const url = new URL(req.url);
    if (state.failing && url.pathname === "/secret") return new Response("the store is busy", { status: 503 });
    return fetch(`http://unix${url.pathname}${url.search}`, { method: req.method, headers: req.headers, body: req.body, unix: fx.socket } as RequestInit);
  } });
  return { state, stop: () => server.stop(true) };
}

t("a bus that does not answer the secret is not 'not configured': no exit 78, and a call says the bus", async () => {
  await stopAgents();
  await setSecret(secret({}));
  const p = flakySecretProxy();
  try {
    const failed = startAgent(agentToken, name, { AGENT_BUS_ADDR: p.state.path });
    const code = await Promise.race([failed.exited, Bun.sleep(15_000).then(() => -1)]);
    expect(code).not.toBe(78);
    expect(code).not.toBe(-1);
    expect(await Bun.file(join(fx.dir, `agent-${agents.length - 1}.log`)).text()).not.toContain("is not set");
    p.state.failing = false;
    startAgent(agentToken, name, { AGENT_BUS_ADDR: p.state.path });
    await waitForRegistration();
    expect(await call("whoami")).toBe("agentbot on Fixture Team (https://fixture-team.slack.com)");
    p.state.failing = true;
    expect(await call("whoami")).toBe("error: the bus did not answer");
  } finally {
    p.stop();
  }
});

t("the fake Slack sees only the bus secret's token, and no sentinel is logged or stored", async () => {
  await stopAgents();
  await setSecret(secret({ SLACK_BOT_TOKEN: "xoxb-secret-sentinel" }));
  startAgent(agentToken, name, { SLACK_BOT_TOKEN: "xoxb-env-sentinel", SLACK_APP_TOKEN: "xapp-env-sentinel" });
  await waitForRegistration();
  expect(await call("whoami")).toBe("agentbot on Fixture Team (https://fixture-team.slack.com)");
  expect(fake.saw.at(-1)!.auth).toBe("Bearer xoxb-secret-sentinel");
  await Bun.sleep(150);
  for (let i = 0; i < agents.length; i++) {
    const log = await Bun.file(join(fx.dir, `agent-${i}.log`)).text();
    expect(log).not.toContain("sentinel");
    expect(log).not.toContain("xoxb-");
    expect(log).not.toContain("xapp-");
  }
  // The store, previews included: the agent writes nothing here, and a token
  // planted as a VALUE must be caught too, not only as a name.
  const listing = await fx.call(fx.ownerToken, "GET", `/kv/list?record=${encodeURIComponent(name)}`);
  expect(JSON.stringify(listing)).not.toContain("sentinel");
  expect(JSON.stringify(listing)).not.toContain("xoxb-");
  for (const entries of Object.values(listing.values ?? {})) expect(entries).toEqual([]); // no writes at all
});

t("an environment endpoint receives nothing: the endpoint is the secret's alone", async () => {
  await stopAgents();
  // The secret names one endpoint, the environment another; the call is
  // awaited, so a zero count is a request that never came, not one not yet
  // sent. No network: the secret's endpoint is a second fake. Slack's own
  // default, when the secret names none, is pinned by the endpoint() unit test.
  let secretSaw = 0;
  const own = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: () => { secretSaw++; return Response.json({ ok: true, user: "bot", team: "T", url: "https://t" }); } });
  const before = fake.saw.length;
  try {
    await setSecret(`SLACK_BOT_TOKEN=xoxb-fixture-bot\nSLACK_APP_TOKEN=xapp-fixture-app\nSLACK_API_URL=http://127.0.0.1:${own.port}/api/\n`);
    startAgent(agentToken, name, { SLACK_API_URL: `http://127.0.0.1:${fake.port}/api/` });
    await waitForRegistration();
    const answer = await call("whoami");
    expect(answer).toBe("bot on T (https://t)");
    expect(secretSaw).toBeGreaterThanOrEqual(1);
    expect(fake.saw.length).toBe(before); // the env endpoint answered nothing
  } finally {
    own.stop(true);
  }
});

t("a hostile endpoint's reflection does not leak the token into the answer", async () => {
  await stopAgents();
  const reflect = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: req =>
    new Response(`diagnostic ${req.headers.get("authorization") ?? ""} garbage`, { status: 502, headers: { "content-type": "text/html" } }) });
  await setSecret(`SLACK_BOT_TOKEN=xoxb-review-secret\nSLACK_APP_TOKEN=xapp-fixture-app\nSLACK_API_URL=http://127.0.0.1:${reflect.port}/api/\n`);
  startAgent(agentToken, name);
  await waitForRegistration();
  const answer = await call("whoami");
  expect(answer).toContain("error:");
  expect(answer).not.toContain("xoxb-review-secret");
  expect(answer).not.toContain("Bearer");
  reflect.stop(true);
});

t("registering does not touch the record's ACL", async () => {
  await stopAgents();
  const guarded = `#slack-guarded-${process.pid}@t`;
  await fx.call(fx.ownerToken, "POST", "/register", { name: guarded, kind: "agent", allow: ["#one@t"], descr: "before" });
  const guardedToken = (await fx.call(fx.ownerToken, "POST", "/token", { name: guarded })).token;
  await fx.call(fx.ownerToken, "POST", "/secret", { name: guarded, secret: secret({}) });
  startAgent(guardedToken, guarded);
  for (let i = 0; i < 200; i++) {
    const rec = await fx.call(fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(guarded)}`);
    if (String(rec?.descr ?? "").length > 0) {
      expect(rec.allow).toEqual(["#one@t"]); // the update changed the description, not the grants
      return;
    }
    await Bun.sleep(50);
  }
  throw new Error("the guarded agent never registered");
});

t("every ask is answered before normal provider work", async () => {
  await stopAgents();
  await setSecret(secret({}));
  startAgent(agentToken,name);
  await waitForRegistration();
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  try {
    for (const ask of ["help","help-config","help-secret","help-json","mcp","status"] as const) {
      const result=await bus.call(name,"",{ask,timeout:5000});
      expect(result.kind).toBe("answer");
      if(result.kind!=="answer") throw Error("no ask answer");
      if(ask==="status") expect(result.message.body).toBe("unsupported: status");
      else expect(result.message.body.length).toBeGreaterThan(0);
    }
  } finally {bus.close();}
});
