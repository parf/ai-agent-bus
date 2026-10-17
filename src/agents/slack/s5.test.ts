import { afterAll, afterEach, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { startFixture, type Fixture } from "./daemon.ts";
import { Bus, type CallOutcome } from "../../clients/typescript/src/index.js";
import { SocketMode } from "./socket.ts";

// S.5 forwarding and S.6 reconnect (Plans/R1.2/slack/TODO.md): a fake Slack
// with a Socket Mode websocket feeds events to the real agent over a real
// daemon, and what reaches each target's inbox is counted there.
const t = (name: string, fn: () => Promise<void>) => test(name, fn, 40000);

let fx: Fixture;
let agent: Subprocess;
let ownerCaller: string;
const pid = process.pid;
const name = `#slack-fwd-${pid}@t`;
const target = `#target-${pid}@t`;
const closed = `#closed-${pid}@t`;
const news = `news-${pid}@t`;
const r1 = `#r1-${pid}@t`;
const r2 = `#r2-${pid}@t`;
let tokens: Record<string, string> = {};
let fake: ReturnType<typeof Bun.serve>;
let sockets: any[] = [];
let upgrades = 0;
let acks: string[] = [];
let envelope = 0;

function push(event: any, eventId = `Ev${++envelope}`): string {
  const id = `env-${envelope}-${Math.random()}`;
  for (const ws of sockets) ws.send(JSON.stringify({ envelope_id: id, type: "events_api", payload: { event_id: eventId, event } }));
  return id;
}

beforeAll(async () => {
  fx = await startFixture();
  ownerCaller = fx.ownerToken; // the Owner itself: it holds the generated owner role
  for (const n of [name]) await fx.call(fx.ownerToken, "POST", "/register", { name: n, kind: "agent", allow: ["*"] });
  for (const n of [target, r1, r2]) await fx.call(fx.ownerToken, "POST", "/register", { name: n, kind: "agent", allow: [name, news] });
  await fx.call(fx.ownerToken, "POST", "/register", { name: closed, kind: "agent" }); // admits nobody but its Owner
  await fx.call(fx.ownerToken, "POST", "/register", { name: news, kind: "pubsub", allow: [name] });
  await fx.call(fx.ownerToken, "POST", "/manage", { name: news, subs: [r1, r2] });
  for (const n of [name, target, r1, r2, closed]) tokens[n] = (await fx.call(fx.ownerToken, "POST", "/token", { name: n })).token;
  fake = Bun.serve({
    port: 0, hostname: "127.0.0.1",
    async fetch(req, server) {
      const path = new URL(req.url).pathname;
      if (path === "/socket") return server.upgrade(req) ? undefined : new Response("no", { status: 400 });
      const method = path.slice("/api/".length);
      if (method === "apps.connections.open") return Response.json({ ok: true, url: `ws://127.0.0.1:${fake.port}/socket` });
      if (method === "auth.test") return Response.json({ ok: true, user: "slackbot", user_id: "UBOT", bot_id: "BBOT", team: "T", url: "https://t" });
      if (method === "conversations.list") return Response.json({ ok: true, channels: [{ id: "C1", name: "dev" }, { id: "C2", name: "ops" }] });
      return Response.json({ ok: false, error: "no route" });
    },
    websocket: {
      open(ws) { upgrades++; sockets.push(ws); },
      message(_ws, data) { try { const m = JSON.parse(String(data)); if (m.envelope_id) acks.push(m.envelope_id); } catch { /* not an ack */ } },
      close(ws) { sockets = sockets.filter(s => s !== ws); },
    },
  });
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=xoxb-fwd\nSLACK_APP_TOKEN=xapp-fwd\nSLACK_API_URL=http://127.0.0.1:${fake.port}/api/\n` });
  agent = spawn(["bun", "slack.ts"], {
    cwd: import.meta.dir,
    // The agent's timing is this test's alone, nothing inherited.
    env: { ...Object.fromEntries(Object.entries(process.env).filter(([k]) => !k.startsWith("SLACK_AGENT_"))), AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: tokens[name]!, SLACK_AGENT_FLUSH_MS: "1500", SLACK_AGENT_BACKOFF_MS: "100" },
    stderr: Bun.file(join(fx.dir, "agent.log")), stdout: Bun.file(join(fx.dir, "agent.log")),
  });
  for (let i = 0; i < 200 && sockets.length === 0; i++) await Bun.sleep(50);
}, 40000);

afterAll(async () => {
  agent?.kill("SIGKILL");
  await Bun.sleep(50);
  fake?.stop(true);
  await fx?.stop();
}, 15000);

async function call(body: string): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token: ownerCaller });
  const outcome: CallOutcome = await bus.call(name, body, { timeout: 12000 });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}

/** An inbox's messages: until `want` have come (then 300 ms more, so an
 *  extra one is seen), or, with no `want`, until it has been quiet for 300 ms
 *  after the first wait of `ms`. Bounded either way. */
async function drain(who: string, ms = 1500, want?: number): Promise<{ body: string; topic?: string; tag?: string }[]> {
  const bus = Bus.connect({ addr: fx.socket, token: tokens[who]! });
  const out: { body: string; topic?: string; tag?: string }[] = [];
  const until = Date.now() + 10000;
  for (let wait = ms; Date.now() < until; ) {
    const m = await bus.consume({ wait }).catch(() => null);
    if (!m) {
      if (want === undefined || out.length >= want) break;
      continue; // still short of what was asked for: keep waiting, within the bound
    }
    if (m.receipt) continue;
    out.push({ body: m.body, topic: m.topic as string | undefined, tag: m.tag as string | undefined });
    wait = 300;
  }
  return out;
}

const live = new Set<string>();
async function addSub(args: string): Promise<string> {
  const id = JSON.parse(await call(`sub add ${args}`)).id;
  expect(typeof id).toBe("string");
  live.add(id);
  return id;
}

// A test that fails leaves no subscription behind to double the next one's deliveries.
afterEach(async () => {
  for (const id of live) await call(`sub delete ${id}`);
  live.clear();
  for (const who of [target, r1, r2, closed]) await drain(who, 100);
});

const msg = (over: Record<string, unknown>) => ({ type: "message", channel: "C1", user: "U1", text: "hello", ts: `${Date.now() / 1000}`, ...over });

t("a matching event reaches its target as JSON, topic slack, tag the channel; a filtered-out one does not", async () => {
  const id = await addSub(`#dev ${target} ^agent: `);
  push(msg({ text: "agent: deploy now" }));
  push(msg({ text: "chatter" }));
  push(msg({ channel: "C2", text: "agent: other channel" }));
  const got = await drain(target);
  expect(got).toHaveLength(1);
  expect(got[0]!.topic).toBe("slack");
  expect(got[0]!.tag).toBe("C1");
  expect(JSON.parse(got[0]!.body)).toMatchObject({ channel: "C1", channel_name: "#dev", user: "U1", text: "agent: deploy now" });
  await call(`sub delete ${id}`);
});

t("its own posts are dropped; edits and deletes are dropped; another bot's message is forwarded", async () => {
  const id = await addSub(`C1 ${target}`);
  push(msg({ user: "UBOT", text: "mine by user" }));
  push(msg({ user: undefined, bot_id: "BBOT", subtype: "bot_message", text: "mine by bot" }));
  push(msg({ subtype: "message_changed", text: "edited" }));
  push(msg({ subtype: "message_deleted" }));
  push(msg({ user: undefined, bot_id: "BALERT", subtype: "bot_message", text: "alert" }));
  push({ type: "reaction_added", item: { channel: "C1" } });
  const got = await drain(target);
  expect(got.map(m => JSON.parse(m.body).text)).toEqual(["alert"]);
  await call(`sub delete ${id}`);
});

t("a pubsub target fans out to both its recipients", async () => {
  const id = await addSub(`C1 ${news}`);
  push(msg({ text: "to everyone" }));
  const [a, b] = [await drain(r1), await drain(r2)];
  expect(a.map(m => JSON.parse(m.body).text)).toEqual(["to everyone"]);
  expect(b.map(m => JSON.parse(m.body).text)).toEqual(["to everyone"]);
  await call(`sub delete ${id}`);
});

t("the same event_id on a new connection is forwarded once, and every envelope is acked", async () => {
  const id = await addSub(`C1 ${target}`);
  const before = acks.length;
  push(msg({ text: "once" }), "EvSame");
  for (let i = 0; i < 40 && acks.length < before + 1; i++) await Bun.sleep(25); // acked before the drop
  const was = upgrades;
  for (const ws of sockets) ws.close(); // the agent reconnects
  for (let i = 0; i < 100 && upgrades === was; i++) await Bun.sleep(50);
  push(msg({ text: "once" }), "EvSame");
  const got = await drain(target);
  expect(got.map(m => JSON.parse(m.body).text)).toEqual(["once"]);
  for (let i = 0; i < 40 && acks.length < before + 2; i++) await Bun.sleep(25);
  expect(acks.length).toBe(before + 2);
  await call(`sub delete ${id}`);
});

t("a refused target records last_error, without a retry", async () => {
  const id = await addSub(`C1 ${closed}`);
  push(msg({ text: "nobody may" }));
  await Bun.sleep(800);
  expect(await drain(closed, 300)).toHaveLength(0);
  const listed = JSON.parse(await call('{"method":"sub.list"}')).subs.find((s: any) => s.id === id);
  expect(listed.last_error).toContain("may not send");
  expect(listed.forwarded).toBe(0);
  await call(`sub delete ${id}`);
});

t("counters are written once per flush, not per event, and a flush never brings back a deleted subscription", async () => {
  const id = await addSub(`C1 ${target}`);
  for (let i = 0; i < 5; i++) push(msg({ text: `n${i}` }));
  expect(await drain(target, 1500, 5)).toHaveLength(5);
  const kv = Bus.connect({ addr: fx.socket, token: fx.ownerToken }).record(name).kv as any;
  await expect(kv.get("int", `stat.${id}.forwarded`)).rejects.toThrow(); // not yet: no write per event
  expect(JSON.parse(await call('{"method":"sub.list"}')).subs.find((s: any) => s.id === id).forwarded).toBe(5);
  await Bun.sleep(2200); // past one flush
  expect(Number((await kv.get("int", `stat.${id}.forwarded`)).value)).toBe(5);
  // A deleted subscription stays deleted through a flush that had its counts.
  push(msg({ text: "late" }));
  expect(await drain(target, 1500, 1)).toHaveLength(1);
  await call(`sub delete ${id}`);
  await Bun.sleep(2200);
  await expect(kv.get("json", `sub.${id}`)).rejects.toThrow();
  await expect(kv.get("int", `stat.${id}.forwarded`)).rejects.toThrow();
});

t("a filter that takes too long is disabled, and says why", async () => {
  const id = await addSub(`C1 ${target} ^(a+)+$`);
  push(msg({ text: "a".repeat(32) + "!" }));
  for (let i = 0; i < 60; i++) {
    const s = JSON.parse(await call('{"method":"sub.list"}')).subs.find((x: any) => x.id === id);
    if (s?.disabled) {
      expect(s.last_error).toContain("took longer");
      break;
    }
    expect(i).toBeLessThan(59); // never disabled within 6 s fails here
    await Bun.sleep(100);
  }
  push(msg({ text: "a".repeat(5) }));
  expect(await drain(target, 600)).toHaveLength(0); // disabled: it forwards nothing more
  await call(`sub delete ${id}`);
});

t("a dropped socket reconnects (S.6)", async () => {
  const before = upgrades;
  for (const ws of sockets) ws.close();
  for (let i = 0; i < 100 && upgrades === before; i++) await Bun.sleep(50);
  expect(upgrades).toBe(before + 1);
});

t("a silent connection is refreshed by the watchdog (S.6)", async () => {
  let opened = 0;
  const quiet = Bun.serve({ port: 0, hostname: "127.0.0.1",
    fetch(req, server) {
      if (new URL(req.url).pathname === "/socket") return server.upgrade(req) ? undefined : new Response("no", { status: 400 });
      return Response.json({ ok: true, url: `ws://127.0.0.1:${quiet.port}/socket` });
    },
    websocket: { open() { opened++; }, message() {}, close() {} },
  });
  const s = new SocketMode(async () => ({ appToken: "xapp-x", api: `http://127.0.0.1:${quiet.port}/api/` }), () => {}, () => {}, { backoffMs: 50, watchdogMs: 300 });
  try {
    s.start();
    for (let i = 0; i < 100 && opened === 0; i++) await Bun.sleep(20);
    await Bun.sleep(900); // three watchdog periods with no traffic at all
    expect(opened).toBeGreaterThanOrEqual(2);
  } finally {
    s.stop();
    quiet.stop(true);
  }
});

t("stopping ends the socket and every timer; nothing reconnects after (S.6)", async () => {
  let opened = 0;
  const local = Bun.serve({ port: 0, hostname: "127.0.0.1",
    fetch(req, server) {
      if (new URL(req.url).pathname === "/socket") return server.upgrade(req) ? undefined : new Response("no", { status: 400 });
      return Response.json({ ok: true, url: `ws://127.0.0.1:${local.port}/socket` });
    },
    websocket: { open() { opened++; }, message() {}, close() {} },
  });
  const s = new SocketMode(async () => ({ appToken: "xapp-x", api: `http://127.0.0.1:${local.port}/api/` }), () => {}, () => {});
  s.start();
  for (let i = 0; i < 100 && opened === 0; i++) await Bun.sleep(20);
  expect(opened).toBe(1);
  s.stop();
  expect(s.armed).toBe(false);
  await Bun.sleep(400);
  expect(opened).toBe(1);
  local.stop(true);
});

t("what Slack says on a failed open is logged without the token (slack-review-S.5)", async () => {
  const lines: string[] = [];
  const reflect = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: req => Response.json({ ok: false, error: `bad ${req.headers.get("authorization")}` }) });
  const s = new SocketMode(async () => ({ appToken: "xapp-LOGGED-SECRET-1", api: `http://127.0.0.1:${reflect.port}/api/` }), () => {}, l => lines.push(l), { backoffMs: 50 });
  try {
    s.start();
    for (let i = 0; i < 100 && lines.length === 0; i++) await Bun.sleep(20);
    expect(lines.length).toBeGreaterThan(0);
    expect(lines.join("\n")).not.toContain("xapp-LOGGED-SECRET-1");
    expect(lines.join("\n")).toContain("[redacted]");
  } finally {
    s.stop();
    reflect.stop(true);
  }
});

t("a socket URL that cannot be opened is reconnected from, not a crash, and is not logged (slack-review-S.5)", async () => {
  let asked = 0;
  const lines: string[] = [];
  const bad = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: () => { asked++; return Response.json({ ok: true, url: "not a websocket URL ticket=SECRET-TICKET" }); } });
  const s = new SocketMode(async () => ({ appToken: "xapp-x", api: `http://127.0.0.1:${bad.port}/api/` }), () => {}, l => lines.push(l), { backoffMs: 30 });
  const escaped: unknown[] = [];
  const caught = (e: unknown) => escaped.push(e);
  process.on("unhandledRejection", caught);
  try {
    await expect(s.start()).resolves.toBeUndefined(); // the first attempt did not throw
    for (let i = 0; i < 100 && asked < 3; i++) await Bun.sleep(20);
    expect(escaped).toEqual([]); // nothing escaped the reconnect loop
    expect(asked).toBeGreaterThanOrEqual(3); // it kept trying
    expect(lines.join("\n")).not.toContain("SECRET-TICKET");
  } finally {
    process.off("unhandledRejection", caught);
    s.stop();
    bad.stop(true);
  }
});

t("stopping while a reconnect is waiting cancels it (S.6)", async () => {
  let asked = 0;
  const refusing = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: () => { asked++; return Response.json({ ok: false, error: "not_now" }); } });
  const s = new SocketMode(async () => ({ appToken: "xapp-x", api: `http://127.0.0.1:${refusing.port}/api/` }), () => {}, () => {});
  s.start();
  for (let i = 0; i < 100 && asked === 0; i++) await Bun.sleep(20);
  await Bun.sleep(20); // the refusal is in; a reconnect is waiting
  expect(s.armed).toBe(true);
  s.stop();
  expect(s.armed).toBe(false);
  const was = asked;
  await Bun.sleep(400); // past the 100 ms backoff
  expect(asked).toBe(was);
  refusing.stop(true);
});
