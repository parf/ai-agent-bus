#!/usr/bin/env bun
// The Telegram agent (Plans/R1.0-Release/telegram/README.md#telegram-agent):
// one process that serves itself over the bus, reads its token from the
// record's bus secret, and puts a Telegram bot behind #telegram/<account>@<realm>.
//
// Each call runs in one context: one snapshot of the bus secret, so a call
// never mixes two chats, and one absolute deadline, the caller's, which
// every Telegram request and retry stays inside. Every method answers a
// result; the one output boundary renders it as text or, for a JSON call,
// as Telegram's own {"ok": …} shape, and nothing of the agent's
// credentials or of a stack escapes.
import { Bus, type Message } from "../../clients/typescript/src/index.js";
import { parseEnvFile } from "../slack/envfile.js";
import { configFault, parseConfig, resolveDestination, secretFault, type Config } from "./config.js";
import { parseCall } from "./parse.js";
import { help as helpText } from "./help.js";
import { DeadlinePassed, telegramWeb } from "./telegramapi.ts";
import { fail, ok, render, type Result } from "./render.ts";
import { Seen } from "./seen.ts";
import { Receiver } from "./receive.ts";
import { Forwarder } from "./forward.ts";
import { mayChangeSubscriptions, subAdd, subDelete, subEdit, subList as subListStore, type Subscription } from "../slack/subs.ts";

const name = (process.env.AGENT_BUS_NAME ?? "").trim().toLowerCase();
if (!name) {
  console.error("telegram: AGENT_BUS_NAME is not set; the agent cannot know whose secret to read");
  process.exit(1);
}

const bus = Bus.connect({ expectName: name });
const help = helpText;

/** The bus did not answer a read this call needed: the wrapper answers
 *  "the bus did not answer", never "not configured". */
class BusDidNotAnswer extends Error {
  constructor() {
    super("the bus did not answer");
    this.name = "BusDidNotAnswer";
  }
}

/** A call with no deadline of its caller's still ends. */
const defaultBudgetMs = 60_000;

/** The caller's absolute deadline, or the default budget when none was
 *  given (slack.ts deadlineOf, mirrored): Go's zero time is no deadline. */
function deadlineOf(e?: Message): number {
  const raw = e?.deadline;
  const at = typeof raw === "string" ? Date.parse(raw) : NaN;
  if (!Number.isFinite(at) || new Date(at).getUTCFullYear() <= 1) return Date.now() + defaultBudgetMs;
  return at;
}

// The record's secret, dotenv form: TELEGRAM_BOT_TOKEN and an optional
// TELEGRAM_API_URL (tests point it at a fake). Never logged, never taken
// from the process env — the process env is the runner's. Read per call,
// so a changed secret reaches the next message without a restart.
async function secrets(): Promise<Record<string, string>> {
  const secret = await bus.record(name).secret();
  return parseEnvFile("the bus secret", secret.reveal());
}

/** Every Telegram call goes through here: the secret's token and endpoint,
 *  under the pinned policy for the kind — a write is one attempt plus the
 *  429 retry, a read retries once on 5xx. */
async function thisCall(kind: "read" | "write", method: string, params: Record<string, unknown> = {}, deadline = Date.now() + defaultBudgetMs) {
  const env = await secrets();
  if (!env.TELEGRAM_BOT_TOKEN) throw new Error("TELEGRAM_BOT_TOKEN is not set");
  return telegramWeb(env.TELEGRAM_BOT_TOKEN, env.TELEGRAM_API_URL, kind, method, params, deadline);
}

/** The first thing the agent cannot work without, as an answer to its caller. */
async function missingToken(): Promise<{ answer: string; unconfigured: boolean } | undefined> {
  let env: Record<string, string>;
  try {
    env = await secrets();
  } catch (e) {
    const f = secretFault(e);
    return { answer: f.message, unconfigured: f.kind !== "bus" };
  }
  if (!env.TELEGRAM_BOT_TOKEN) return { answer: "TELEGRAM_BOT_TOKEN is not set", unconfigured: true };
  return undefined;
}

// The last configuration read, for the forwarder's resolution: a changed
// alias mapping reroutes subscriptions at the next message.
let lastConfig: Config | undefined;

/** The configuration as it is now. A bus failure throws (the wrapper
 *  answers it as the bus); a malformed one comes back as the reason. */
async function configuration(): Promise<Config | string> {
  let raw: unknown;
  try {
    raw = await bus.record(name).config();
  } catch (e) {
    const f = configFault(e);
    if (f.kind === "bus") throw new BusDidNotAnswer();
    return { aliases: {} };
  }
  const parsed = parseConfig(raw);
  if (typeof parsed === "object") lastConfig = parsed; // the forwarder resolves through the live mapping
  return parsed;
}

async function whoami(json: boolean, deadline: number): Promise<Result> {
  const missing = await missingToken();
  if (missing) return fail(missing.answer);
  let r;
  try {
    r = await thisCall("read", "getMe", {}, deadline);
  } catch (e) {
    if (e instanceof DeadlinePassed) throw e;
    return fail("getMe failed");
  }
  if (r.ok !== true) return fail(r.description ?? r.error ?? "getMe failed");
  const me = r.result ?? {};
  return ok(`@${me.username} (id ${me.id})`, { username: me.username, id: me.id });
}

async function post(json: boolean, call: Extract<Call, { method: "post" }>, deadline: number): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer);
  const r = await thisCall("write", "sendMessage", {
    chat_id: dest.chat_id,
    text: call.text,
    ...(call.reply_to !== undefined ? { reply_to_message_id: Number(call.reply_to) } : {}),
  }, deadline);
  if (r.ok !== true) return fail(r.description ?? r.error ?? "sendMessage failed");
  const mid = r.result?.message_id;
  return mid !== undefined
    ? ok(`${dest.named} message ${mid}`, { channel: dest.named, chat_id: dest.chat_id, message_id: mid })
    : fail("sendMessage answered no message id");
}

async function channel(json: boolean, call: Extract<Call, { method: "channel" }>, deadline: number): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer);
  const r = await thisCall("read", "getChat", { chat_id: dest.chat_id }, deadline);
  if (r.ok !== true) return fail(r.description ?? r.error ?? "getChat failed");
  const chat = r.result ?? {};
  return ok(`${chat.title ?? chat.username ?? dest.named} (${chat.type ?? "chat"})`, {
    id: chat.id, title: chat.title, type: chat.type, ...(chat.description ? { description: chat.description } : {}),
  });
}

async function user(json: boolean, call: Extract<Call, { method: "user" }>, deadline: number): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  // A user is seen through a chat: the declared default is the window.
  const dest = resolveDestination(undefined, cfg);
  if (!dest.ok) return fail(dest.error);
  let user_id = call.who;
  if (call.who.startsWith("#") || call.who.startsWith("@")) {
    const alias = resolveDestination(call.who, cfg);
    if (!alias.ok) return fail(alias.error);
    user_id = alias.chat_id;
  }
  const missing = await missingToken();
  if (missing) return fail(missing.answer);
  const r = await thisCall("read", "getChatMember", { chat_id: dest.chat_id, user_id }, deadline);
  if (r.ok !== true) return fail(r.description ?? r.error ?? "getChatMember failed");
  const member = r.result ?? {};
  return ok(`@${member.user?.username ?? user_id} (${member.status ?? "member"})`, {
    id: member.user?.id ?? user_id,
    ...(member.user?.username ? { username: member.user.username } : {}),
    ...(member.user?.first_name ? { first_name: member.user.first_name } : {}),
    status: member.status ?? "member",
  });
}

async function react(json: boolean, call: Extract<Call, { method: "react" }>, deadline: number): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer);
  const r = await thisCall("write", "setMessageReaction", {
    chat_id: dest.chat_id,
    message_id: Number(call.id),
    reaction: [{ type: "emoji", emoji: call.emoji }],
  }, deadline);
  if (r.ok !== true) return fail(r.description ?? r.error ?? "setMessageReaction failed");
  return ok(`reacted ${call.emoji} on message ${call.id}`);
}

async function edit(json: boolean, call: Extract<Call, { method: "edit" }>, deadline: number): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer);
  const r = await thisCall("write", "editMessageText", {
    chat_id: dest.chat_id,
    message_id: Number(call.id),
    text: call.text,
  }, deadline);
  if (r.ok !== true) return fail(r.description ?? r.error ?? "editMessageText failed");
  const mid = r.result?.message_id ?? Number(call.id);
  return ok(`message ${mid} edited`, { message_id: mid, text: call.text });
}

async function del(json: boolean, call: Extract<Call, { method: "delete" }>, deadline: number): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer);
  const r = await thisCall("write", "deleteMessage", { chat_id: dest.chat_id, message_id: Number(call.id) }, deadline);
  if (r.ok !== true) return fail(r.description ?? r.error ?? "deleteMessage failed");
  return ok(`message ${call.id} deleted`, { message_id: Number(call.id) });
}

// What the bot has seen, flushed to its own KV after each received batch
// (README § receiving). The seen store lives beside the agent; `read`,
// `channels` and `users` answer from it.
const seen = new Seen();

/** The agent's own KV, for the offset and the seen state. */
async function kvSet(key: string, value: unknown): Promise<void> {
  await bus.record(name).kv.set("string", key, typeof value === "string" ? value : JSON.stringify(value), { mode: "set" });
}

async function kvGet(key: string): Promise<string | undefined> {
  const v = (await bus.record(name).kv.get("string", key)).value;
  return typeof v === "string" ? v : v === undefined ? undefined : new TextDecoder().decode(v);
}

// The forwarder: every stored message goes to the matching subscriptions
// (TG.6) — the subscription's channel resolves through the live mapping
// and the message is sent to the target with topic telegram, tag the chat
// id (README § receiving).
const forwarder = new Forwarder({
  async subs() {
    const r = await subListStore(bus, name);
    return r.ok ? (r.fields.subs as Subscription[]) : [];
  },
  resolve: (channel) => resolveDestination(channel, lastConfig ?? { aliases: {} }),
  send: async (to, body, topic, tag) => { await bus.send(to, body, { topic, tag }); },
  persistCount: async (id, forwarded) => { await bus.record(name).kv.increment(`stat.${id}.forwarded`, { by: forwarded }); },
  persistError: async (id, lastError) => { await bus.record(name).kv.set("string", `stat.${id}.last_error`, lastError, { mode: "set" }); },
  log: (line) => console.error(`telegram: ${line}`),
});

async function read(json: boolean, call: Extract<Call, { method: "read" }>, deadline: number): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const messages = seen.read(dest.chat_id, call.limit);
  const lines = messages.map(m => `${m.message_id} ${m.from ?? "?"}: ${m.text ?? ""}`);
  return ok(lines.length > 0 ? lines.join("\n") : `no messages from ${dest.named} yet`, { channel: dest.named, count: lines.length });
}

async function channels(json: boolean, call: Extract<Call, { method: "channels" }>): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const list = seen.chatList();
  const f = call.filter?.toLowerCase();
  const hit = f ? list.filter(c => (c.title ?? "").toLowerCase().includes(f) || c.id.includes(f)) : list;
  const aliasEntries = Object.entries(cfg.aliases);
  const aliasHit = f ? aliasEntries.filter(([k, id]) => k.toLowerCase().includes(f) || id.includes(f)) : aliasEntries;
  const lines = hit.map(c => `${c.id} ${c.title ?? ""} (${c.type})`.trim());
  const aliasLines = aliasHit.map(([k, id]) => `${k} → ${id}`);
  const all = [...aliasLines, ...lines];
  return ok(all.length > 0 ? all.join("\n") : "no chats seen yet", { count: hit.length });
}

async function users(json: boolean, call: Extract<Call, { method: "users" }>): Promise<Result> {
  const list = seen.userList();
  const f = call.filter?.toLowerCase();
  const hit = f ? list.filter(u => (u.username ?? "").toLowerCase().includes(f) || (u.first_name ?? "").toLowerCase().includes(f)) : list;
  const lines = hit.map(u => `${u.id} ${u.username ? "@" + u.username : u.first_name ?? ""}`.trim());
  return ok(lines.length > 0 ? lines.join("\n") : "no senders seen yet", { count: hit.length });
}

/** The subscriptions, from the shared store. */
async function subList(json: boolean): Promise<Result> {
  const r = await subListStore(bus, name);
  if (!r.ok) return fail(r.error);
  return ok(r.text, r.fields);
}

async function handle(e: Message): Promise<string | null> {
  // Asked for help, it says what it is for and how to call it. A JSON call
  // is answered in JSON (Plans/R1.0-Release/telegram/README.md § Forms,
  // carried over from Slack): {"ok":true, …fields} or
  // {"ok":false,"error":…}, and a call that is no call carries the help to
  // say why. Methods later waves build fall through to help, which is also
  // the answer for anything unknown (TG.2).
  if (e.ask === "help") return help;
  try {
  const answer = await _handle(e);
  if (typeof answer !== "string") console.error(`telegram: NON-TEXT ANSWER from ${e.body.slice(0, 30)}: ${typeof answer} ${JSON.stringify(answer)?.slice(0, 100)}`); // DEBUG
  return answer;
  } catch (err) { console.error(`telegram: HANDLE-DETAIL ${err instanceof Error ? `${err.name}: ${err.message}` : String(err)}\n${err instanceof Error ? (err.stack ?? "").split("\n").slice(0, 4).join("\n") : ""}`); return "error: the call failed"; }
}

async function _handle(e: Message): Promise<string | null> {
  const json = e.body.trim().startsWith("{");
  const call = parseCall(e.body);
  if (call.method === "help") {
    if (json && !isHelpCall(e.body)) return JSON.stringify({ ok: false, error: call.why ?? "not a call this agent knows", help });
    return render(json, ok(help, { help }));
  }
  const deadline = deadlineOf(e);
  try {
    let r: Result;
    switch (call.method) {
      case "whoami": {
        const missing = await missingToken();
        if (missing) return render(json, fail(missing.answer));
        r = await whoami(json, deadline);
        break;
      }
      case "post":
        r = await post(json, call, deadline);
        break;
      case "channel":
        r = await channel(json, call, deadline);
        break;
      case "user":
        r = await user(json, call, deadline);
        break;
      case "react":
        r = await react(json, call, deadline);
        break;
      case "edit":
        r = await edit(json, call, deadline);
        break;
      case "delete":
        r = await del(json, call, deadline);
        break;
      case "read":
        r = await read(json, call, deadline);
        break;
      case "channels":
        r = await channels(json, call, deadline);
        break;
      case "users":
        r = await users(json, call, deadline);
        break;
      case "sub.add":
      case "sub.edit":
      case "sub.delete": {
        // Whoever manages a subscription's destination may change it, and
        // this agent's own Owner and Maintainers may change any (S.8) —
        // the same rule Slack's dispatch runs, through the shared subs
        // module.
        try {
        const any = mayChangeSubscriptions(e, name);
        const guard = async (sub: Subscription) => {
          if (any) return undefined;
          const to = call.method === "sub.edit" ? call.edits.channel : undefined;
          for (const target of to !== undefined ? [sub.target, to] : [sub.target]) {
            const why = await uncontrolled(e.from, target);
            if (why) return why;
          }
          return undefined;
        };
        if (call.method === "sub.add") {
          const why = any ? undefined : await uncontrolled(e.from, call.target);
          const r = why ? fail(why) : await subAdd(bus, name, e, call.channel, call.target, call.filter);
          return render(json, r);
        }
        if (call.method === "sub.delete") return render(json, await subDelete(bus, name, call.id, guard));
        return render(json, await subEdit(bus, name, call.id, call.edits, guard));
        } catch (err) { console.error(`telegram: SUB-CASE ${call.method}: ${err instanceof Error ? `${err.name}: ${err.message}` : "?"}`); return fail("sub case failed"); }
      }
      case "sub.list":
        r = await subList(json);
        break;
      default:
        return fail("not a call this agent knows");
    }
    return render(json, r);
  } catch (err) {
    // Nothing escapes as a stack or a URL: a bus failure is answered as the
    // bus, and any other unexpected throw is the method failing.
    if (err instanceof BusDidNotAnswer) return render(json, fail("the bus did not answer"));
    if (err instanceof DeadlinePassed) return render(json, fail("the caller's deadline passed"));
    return render(json, fail(`${call.method} failed`));
  }
}

function isHelpCall(body: string): boolean {
  try {
    return JSON.parse(body)?.method === "help";
  } catch {
    return false;
  }
}

/** Why `who` may not point a subscription at `target`, or undefined when
 *  it controls it: manages it, by the daemon's answer (S.8). */
async function uncontrolled(who: string, target: string): Promise<string | undefined> {
  try {
    if (await bus.manages(who, target)) return undefined;
  } catch (err) {
    if ((err as any)?.status === 404) return `${target} is not a destination this agent can see; its ACL must admit ${name}`;
    throw err;
  }
  return `${who} does not control ${target}: a subscription is changed by whoever manages its destination`;
}

await bus.register({ name, kind: "agent", descr: "telegram: a bot on the bus — post, read, subscriptions" }, { createOnly: false });
// Not configured at start: say why on stderr and exit 78 (EX_CONFIG),
// which the runner reads as "unconfigured" and does not restart until a
// reload (slack.ts, Plans/R1.0-Release/TODO.md M.13). The bus failing is
// NOT unconfigured: exit 1 and let the runner restart with backoff
// (tg1-review P2-1/P2-2). A secret or configuration that changes while it
// runs is still answered per call.
{
  const why = await missingToken();
  if (why) {
    console.error(`telegram: ${why.answer}`);
    process.exit(why.unconfigured ? 78 : 1);
  }
  let cfg: Config | string;
  try {
    cfg = await configuration();
  } catch (e) {
    console.error(`telegram: ${e instanceof Error ? e.message : "the bus did not answer"}`);
    process.exit(1);
  }
  // A malformed configuration is refused at start, not at the first call
  // that needs it (telegram-build, 2026-10-18): the refusal names the bad
  // key. No configuration is none, and none runs.
  if (typeof cfg === "string") {
    console.error(`telegram: ${cfg}`);
    process.exit(78);
  }
}
const stop = new AbortController();
process.on("SIGINT", () => stop.abort());
process.on("SIGTERM", () => stop.abort());

// Receiving: getUpdates long polling beside the serve loop, the offset and
// the seen state in the agent's own KV (README § receiving). The secret is
// read fresh per poll, exactly as the methods read theirs. The bot's own
// id, learned from the first getMe, keeps the bot's own posts out of the
// forwarding; until it is known, Telegram does not echo the bot's own
// posts by default. The receiver polls and forwards in its own background
// loop: a long poll or a failing endpoint must not delay the answers.
const receiver = new Receiver(seen, async (kind, method, params) => {
  const env = await secrets();
  if (!env.TELEGRAM_BOT_TOKEN) throw new Error("TELEGRAM_BOT_TOKEN is not set");
  return telegramWeb(env.TELEGRAM_BOT_TOKEN, env.TELEGRAM_API_URL, kind, method, params);
}, kvGet, kvSet, (line) => console.error(`telegram: ${line}`));
receiver.onMessage = (m, chatId, chatTitle) => void forwarder.message(m, chatId, chatTitle);
void (async () => {
  try {
    const me = await thisCall("read", "getMe");
    if (me.ok === true && me.result?.id !== undefined) receiver.botId = String(me.result.id);
  } catch { /* the bot id stays unknown until the endpoint answers */ }
})();
receiver.start();
await bus.serve(handle, { signal: stop.signal, drainTimeout: 5000 });
receiver.stop();
