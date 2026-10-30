import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { call as busCall, identity, startFixture, type Fixture } from "../slack/daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";
import { FilterWorker, compileFilter } from "../shared/subscriptions.ts";

// TG.6: subscriptions. A match reaches its target with topic `telegram`,
// tag the chat id, and the README's body; a non-match reaches nothing; the
// bot's own posts and edits are not forwarded; and the authority is the
// destination's (S.8) — its maintainers and the agent's owner change
// subscriptions, an ACL-only caller does not
// (Plans/R1.0-Release/done/telegram/TODO.md TG.6).

const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let ownerToken = "";
let plainToken = "";
let bobToken = "";
const name = `#telegram-t6-${process.pid}@t`;
const target = `#t6-target-${process.pid}@t`;
let targetToken = "";
let agentToken = "";
let agent: Subprocess | undefined;

// The receiving agent's Telegram endpoint is a local fake feeding
// getUpdates; the test pushes batches and the receiver polls them.
const t6Updates: any[] = [];
const t6config = { users: { "900": "fixture" }, default: "#dev", aliases: { "#dev": "-100123", "#ops": "-100999" } };
const t6name = `#telegram-t6r-${process.pid}@t`;
let t6agent: Subprocess | undefined;

beforeAll(async () => {
  fx = await startFixture();
  ownerToken = fx.ownerToken;
  plainToken = await identity(fx, `#plain-t6-${process.pid}@t`);
  bobToken = await identity(fx, `#bob-t6-${process.pid}@t`);
  // The Telegram agent: everyone may call; its own owner may change subs.
  await fx.call(ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  // The target, maintained by bob: bob changes subscriptions pointing at
  // it; the plain caller does not manage it (S.8).
  await fx.call(ownerToken, "POST", "/register", { name: target, kind: "agent", allow: ["*"] });
  targetToken = (await fx.call(fx.ownerToken, "POST", "/token", { name: target })).token;
  await fx.call(ownerToken, "POST", "/manage", { name: target, maintainers: [`#bob-t6-${process.pid}@t`] });
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `TELEGRAM_BOT_TOKEN=111:t6-bot\nTELEGRAM_API_URL=http://127.0.0.1:1\n` });
  await fx.call(fx.ownerToken, "POST", "/configure", { name, config: t6config });
  agent = spawn(["bun", "telegram.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: agentToken },
    stderr: Bun.file(join(fx.dir, "t6-agent.log")),
    stdout: Bun.file(join(fx.dir, "t6-agent.log")),
  });
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await busCall(fx.socket, fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
      if (String(rec?.descr ?? "").length > 0) return;
    } catch { /* not yet */ }
    await Bun.sleep(50);
  }
  throw new Error("the agent never registered");
}, 45000);

// The receiving agent with a live fake: messages on -100123 (subscribed)
// and -100999 (not), the bot's own posts (from 42), and an edit.
const t6fake: { port: number } = { port: 0 };
let t6fakeServer: ReturnType<typeof Bun.serve>;
const t6target = `#t6r-target-${process.pid}@t`;
let t6targetToken = "";
let t6agentToken = "";

beforeAll(async () => {
  await fx.call(ownerToken, "POST", "/register", { name: t6name, kind: "agent", allow: ["*"] });
  t6agentToken = (await fx.call(ownerToken, "POST", "/token", { name: t6name })).token;
  await fx.call(ownerToken, "POST", "/register", { name: t6target, kind: "agent", allow: ["*"] });
  t6targetToken = (await fx.call(ownerToken, "POST", "/token", { name: t6target })).token;
  t6fakeServer = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    const url = new URL(req.url);
    if (url.pathname.endsWith("/getUpdates")) {
      const offset = Number((await req.json().catch(() => ({}))).offset ?? 0);
      const batch = t6Updates.filter(u => u.update_id >= offset).slice(0, 5);
      if (batch.length === 0) await Bun.sleep(1500); // hold like the real long poll
      return Response.json({ ok: true, result: batch });
    }
    const method = url.pathname.split("/").pop() ?? "";
    if (method === "getMe") return Response.json({ ok: true, result: { id: 42, is_bot: true, username: "t6r_bot" } });
    if (method === "sendMessage") return Response.json({ ok: true, result: { message_id: 999 } });
    return Response.json({ ok: true, result: {} });
  } });
  t6fake.port = t6fakeServer.port;
  await fx.call(ownerToken, "POST", "/secret", { name: t6name, secret: `TELEGRAM_BOT_TOKEN=111:t6r\nTELEGRAM_API_URL=http://127.0.0.1:${t6fake.port}\n` });
  await fx.call(ownerToken, "POST", "/configure", { name: t6name, config: t6config });
  t6agent = spawn(["bun", "telegram.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: t6name, AGENT_BUS_TOKEN: t6agentToken },
    stderr: Bun.file(join(fx.dir, "t6r-agent.log")),
    stdout: Bun.file(join(fx.dir, "t6r-agent.log")),
  });
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await busCall(fx.socket, ownerToken, "GET", `/lookup?name=${encodeURIComponent(t6name)}`);
      if (String(rec?.descr ?? "").length > 0) return;
    } catch { /* not yet */ }
    await Bun.sleep(50);
  }
  throw new Error("the receiving agent never registered");
}, 45000);

afterAll(async () => {
  agent?.kill("SIGKILL");
  t6agent?.kill("SIGKILL");
  await Bun.sleep(50);
  t6fakeServer?.stop(true);
  await fx?.stop();
}, 15000);

async function t6call(body: string, token = ownerToken): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token });
  const outcome = await bus.call(t6name, body, { timeout: 10000 });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}

/** Drains the target's inbox for up to ms, returning the forwarded
 *  messages (the client's consume, as Slack's s5 drain does). */
async function forwarded(token: string, want?: number): Promise<{ topic?: string; tag?: string; body: any }[]> {
  // Drains until the expected count and a quiet interval, or 5 s pass: the forward's
  // timing moved with the receiver's batching, so a fixed short window
  // stopped at its first empty drain (tg5 review's hint).
  const bus = Bus.connect({ addr: fx.socket, token });
  const out: { topic?: string; tag?: string; body: any }[] = [];
  const until = Date.now() + 5000;
  for (let wait = 500; Date.now() < until;) {
    const m = await bus.consume({ wait }).catch(() => null);
    if (!m) { if (want !== undefined && out.length >= want) break; continue; }
    if (m.receipt) { wait = 200; continue; }
    out.push({ topic: m.topic as string | undefined, tag: m.tag as string | undefined, body: JSON.parse(m.body) });
    wait = 200; // drain a quiet interval after the expected copies too
  }
  return out;
}

const msg = (update_id: number, text: string, chat_id: number, from_id = 900) => ({
  update_id,
  message: { message_id: update_id, date: 1700000000 + update_id, text, from: { id: from_id, username: `u${update_id}` }, chat: { id: chat_id, title: "Dev", type: "supergroup" } },
});

t("the filter worker is warm before the subscriptions evaluate", async () => {
  const worker = new FilterWorker();
  expect(await worker.test(compileFilter("/warm/")!, "warm", Date.now()+1000)).toBe("hit");
  worker.close();
});

t("admitted authors may subscribe; listing is author-only unless source staff", async () => {
  const made = JSON.parse(await t6call(`sub add #dev ${target}`, bobToken));
  expect(made.id).toMatch(/^[0-9a-f]{12}$/);
  expect(JSON.parse(await t6call('{"method":"sub.list"}', plainToken)).subs).toEqual([]);
  expect(await t6call(`sub delete ${made.id}`, plainToken)).toContain("not_author");
  expect(await t6call(`sub add #ops ${target}`, ownerToken)).not.toContain("error:");
  expect(await t6call("sub list", ownerToken)).toContain(target);
});

t("a match reaches its target: topic telegram, tag the chat id, the README's body", async () => {
  t6Updates.push(msg(300, "hello from the group", -100123));
  const fs = await forwarded(targetToken, 1);
  expect(fs).toHaveLength(1);
  const f = fs.find(x => x.body.text === "hello from the group");
  expect(f).toBeDefined();
  expect(f!.topic).toBe("telegram");
  expect(f!.tag).toBe("-100123");
  expect(f!.body).toEqual({ chat: "-100123", chat_name: "Dev", user: 900, user_name: "@u300", text: "hello from the group", message_id: 300 });
});

t("a non-matching chat reaches nothing", async () => {
  t6Updates.push(msg(301, "unsubscribed noise", -100777));
  expect(await forwarded(targetToken, 0)).toEqual([]);
});

t("the filter holds: only matching text is forwarded", async () => {
  expect(await t6call(`sub add #dev ${target} /alarm/`, bobToken)).not.toContain("error:");
  t6Updates.push(msg(302, "hello again", -100123));
  t6Updates.push(msg(303, "alarm: cpu high", -100123));
  const fs = await forwarded(targetToken, 3);
  const alarm = fs.find(x => x.body.text === "alarm: cpu high");
  expect(alarm).toBeDefined();
  // Two subscriptions cover #dev (the unfiltered one and the /alarm one),
  // so the alarm arrives once per subscription — and the non-matching
  // text only once, from the unfiltered one: the filter holds.
  expect(fs.filter(x => x.body.text === "alarm: cpu high").length).toBe(2);
  expect(fs.filter(x => x.body.text === "hello again").length).toBe(1);
});

t("the bot's own posts and edits are not forwarded", async () => {
  // The t6 fake's getMe says the bot is id 42: its own post is dropped, and
  // an edited_message update is not a message.
  t6Updates.push(msg(304, "my own post", -100123, 42));
  t6Updates.push({ update_id: 305, edited_message: { message_id: 300, text: "edited", chat: { id: -100123, title: "Dev", type: "supergroup" } } });
  const fs = await forwarded(targetToken, 0);
  expect(fs).toEqual([]);
  expect(fs.find(x => x.body.text === "my own post")).toBeUndefined();
  expect(fs.find(x => x.body.text === "edited")).toBeUndefined();
});

t("shared delivery counters are persisted in the subscription row", async () => {
  const answer = JSON.parse(await t6call(`{"method":"sub.list"}`));
  const withFilter = (answer.subs as { id: string; filter?: string; sent: number }[]).find(s => s.filter === "/alarm/");
  expect(withFilter).toBeDefined();
  // In memory at once: the listing carries the count before any flush.
  expect(withFilter!.sent).toBeGreaterThanOrEqual(1);
  const stored = await fx.call(ownerToken, "GET", `/kv?record=${encodeURIComponent(t6name)}&kind=json&name=sub.${withFilter!.id}`);
  expect(stored.value.sent).toBe(withFilter!.sent);
});

t("removed editing cannot retarget a subscription", async () => {
 const foreign=`#foreign-${process.pid}@t`;
 await fx.call(ownerToken,"POST","/register",{name:foreign,kind:"agent",allow:["*"]});
 const created=JSON.parse(await t6call(JSON.stringify({method:"sub.add",channel:"#dev",target}),bobToken));
 expect(created.id).toBeDefined();
 const edit=JSON.parse(await t6call(JSON.stringify({method:"sub.edit",id:created.id,target:foreign}),bobToken));
 expect(edit.ok).toBe(false);expect(edit.error).toContain("sub delete");
 const list=JSON.parse(await t6call('{"method":"sub.list"}',ownerToken));
 expect(list.subs.find((s:any)=>s.id===created.id).target).toBe(target);
});


t("RT33 a caller refused by the target cannot subscribe through the Telegram handler", async () => {
  const blocked = `#rt33-telegram-target-${process.pid}@t`, caller = `#plain-t6-${process.pid}@t`;
  await fx.call(ownerToken, "POST", "/register", { name: blocked, kind: "agent", allow: [t6name] });
  const rows = async () => ((await fx.call(ownerToken, "GET", `/kv/list?record=${encodeURIComponent(t6name)}`)).json ?? [])
    .filter((row: { name: string }) => row.name.startsWith("sub."));
  const before = await rows();
  expect(await t6call(`sub add #dev ${blocked}`, plainToken)).toContain("admission_refused");
  expect(await rows()).toEqual(before);
  expect(await t6call("sub list", plainToken)).not.toContain(blocked);
  await fx.call(ownerToken, "POST", "/manage", { name: blocked, allow: [t6name, caller] });
  const added = JSON.parse(await t6call(`sub add #dev ${blocked}`, plainToken));
  expect(added.id).toMatch(/^[0-9a-f]{12}$/);
  const stored = await fx.call(ownerToken, "GET", `/kv?record=${encodeURIComponent(t6name)}&kind=json&name=sub.${added.id}`);
  expect(stored.value.created_by).toBe(caller);
  expect(await t6call(`sub delete ${added.id}`, plainToken)).toContain("deleted");
});
