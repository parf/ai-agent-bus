#!/usr/bin/env bun
// The Telegram agent (Plans/R1.0-Release/telegram/README.md#telegram-agent):
// one process that serves itself over the bus, reads its token from the
// record's bus secret, and puts a Telegram bot behind #telegram/<account>@<realm>.
//
// TG.1: register, serve, help, whoami, and the bus secret. Later waves add
// parsing (TG.2), Telegram calls (TG.3-TG.4), receiving (TG.5) and
// subscriptions (TG.6). Shared code is imported from src/agents/slack/ —
// never copied (README § what carries over).
import { Bus, type Message } from "../../clients/typescript/src/index.js";
import { parseEnvFile } from "../slack/envfile.js";
import { configFault, parseConfig, resolveDestination, secretFault, type Config } from "./config.js";
import { parseCall } from "./parse.js";
import { help as helpText } from "./help.js";
import { DeadlinePassed, telegramWeb } from "./telegramapi.js";
import { fail, ok, render, type Result } from "./render.ts";

const name = (process.env.AGENT_BUS_NAME ?? "").trim().toLowerCase();
if (!name) {
  console.error("telegram: AGENT_BUS_NAME is not set; the agent cannot know whose secret to read");
  process.exit(1);
}

const bus = Bus.connect({ expectName: name });
// Both call forms arrive through one parser; methods later waves build fall
// through to help, which is also the answer for anything unknown (TG.2).
// The help text lives in help.ts so the parser and help cannot drift apart.
// Both call forms arrive through one parser; methods later waves build fall
// through to help, which is also the answer for anything unknown (TG.2).
// The help text lives in parse.ts so the parser and help cannot drift apart.
const help = helpText;

// The record's secret, dotenv form: TELEGRAM_BOT_TOKEN and an optional
// TELEGRAM_API_URL (tests point it at a fake). Never logged, never taken
// from the process env — the process env is the runner's, and a token
// arriving by accident there is one nobody set
// (Plans/R1.0-Release/telegram/README.md § where things live). Read per
// call, so a changed secret reaches the next message without a restart.
// Read failures propagate: what they mean is secretFault's to say.
async function secrets(): Promise<Record<string, string>> {
  const secret = await bus.record(name).secret();
  return parseEnvFile("the bus secret", secret.reveal());
}

/** What a call is answered, and whether the agent may even start: absent or
 *  malformed is unconfigured (exit 78), the bus failing is not — the runner
 *  restarts it instead of marking it unconfigured until a reload
 *  (tg1-review P2-1). */
export type Missing = { answer: string; unconfigured: boolean };

/** The first thing the agent cannot work without, as an answer to its caller. */
export async function missingToken(): Promise<Missing | undefined> {
  let env: Record<string, string>;
  try {
    env = await secrets();
  } catch (e) {
    const f = secretFault(e);
    return { answer: `error: ${f.message}`, unconfigured: f.kind !== "bus" };
  }
  if (!env.TELEGRAM_BOT_TOKEN) return { answer: "error: TELEGRAM_BOT_TOKEN is not set", unconfigured: true };
  return undefined;
}

/** Every Telegram call goes through here: the secret's token and endpoint,
 *  under the pinned policy for the kind — a write is one attempt plus the
 *  429 retry, a read retries once on 5xx. */
async function thisCall(kind: "read" | "write", method: string, params: Record<string, unknown> = {}) {
  const env = await secrets();
  if (!env.TELEGRAM_BOT_TOKEN) throw new Error("TELEGRAM_BOT_TOKEN is not set");
  return telegramWeb(env.TELEGRAM_BOT_TOKEN, env.TELEGRAM_API_URL, kind, method, params);
}

/** The instance's configuration as it is now, or the reason it is refused
 *  (config.ts): read as this record, so a changed configuration reaches the
 *  next call without a restart. No configuration at all is none, not an
 *  error — the calls that need a destination are refused then. A read that
 *  failed for any other reason throws: the caller answers the bus's name
 *  (tg1-review P2-2), never a silent "no default" for one that was
 *  declared. */
/** The bus did not answer a read this call needed: the wrapper answers
 *  "the bus did not answer", never "not configured". */
class BusDidNotAnswer extends Error {
  constructor() {
    super("the bus did not answer");
    this.name = "BusDidNotAnswer";
  }
}

export async function configuration(): Promise<Config | string> {
  let raw: unknown;
  try {
    raw = await bus.record(name).config();
  } catch (e) {
    const f = configFault(e);
    if (f.kind === "bus") throw new BusDidNotAnswer();
    return { aliases: {} };
  }
  return parseConfig(raw);
}

async function whoami(json: boolean): Promise<Result> {
  // A transport failure is an answer too: the method's name, never the URL,
  // whose path segment is the token (telegramapi.ts).
  let r;
  try {
    r = await thisCall("read", "getMe");
  } catch (e) {
    if (e instanceof DeadlinePassed) throw e;
    return fail("getMe failed");
  }
  if (r.ok !== true) return fail(r.description ?? r.error ?? "getMe failed");
  const me = r.result ?? {};
  return ok(`@${me.username} (id ${me.id})`, { username: me.username, id: me.id });
}

// The client's serve loop acks each message before the handler runs and
// refuses receipts there, so they are consumed and never answered here.
async function handle(e: Message): Promise<string | null> {
  // Asked for help, it says what it is for and how to call it. A JSON call
  // is answered in JSON (Plans/R1.0-Release/telegram/README.md § Forms,
  // carried over from Slack): {"ok":true, …fields} or
  // {"ok":false,"error":…}, and a call that is no call carries the help to
  // say why. Methods later waves build fall through to help, which is also
  // the answer for anything unknown (TG.2).
  if (e.ask === "help") return help;
  const json = e.body.trim().startsWith("{");
  const call = parseCall(e.body);
  if (call.method === "help") {
    if (json && !isHelpCall(e.body)) return JSON.stringify({ ok: false, error: "not a call this agent knows", help });
    return render(json, ok(help, { help }));
  }
  try {
    let r: Result;
    switch (call.method) {
      case "whoami": {
        const missing = await missingToken();
        if (missing) return render(json, fail(missing.answer.replace(/^error: /, "")));
        r = await whoami(json);
        break;
      }
      case "post":
        r = await post(json, call);
        break;
      case "channel":
        r = await channel(json, call);
        break;
      case "user":
        r = await user(json, call);
        break;
      case "react":
        r = await react(json, call);
        break;
      case "edit":
        r = await edit(json, call);
        break;
      case "delete":
        r = await del(json, call);
        break;
      case "read":
      case "channels":
      case "users":
      case "sub.list":
      case "sub.add":
      case "sub.edit":
      case "sub.delete":
        // Known, built in later waves: help answers them for now (TG.2).
        return render(json, ok(help, { help }));
      default:
        return render(json, fail("not a call this agent knows", { help }));
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

/** The configuration for one call, or the reason the call is refused. */
async function configOr(): Promise<Config | string> {
  const cfg = await configuration();
  return cfg;
}

async function post(json: boolean, call: Extract<Call, { method: "post" }>): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer.replace(/^error: /, ""));
  const r = await thisCall("write", "sendMessage", {
    chat_id: dest.chat_id,
    text: call.text,
    ...(call.reply_to !== undefined ? { reply_to_message_id: Number(call.reply_to) } : {}),
  });
  if (r.ok !== true) return fail(r.description ?? r.error ?? "sendMessage failed");
  const mid = r.result?.message_id;
  return mid !== undefined
    ? ok(`${dest.named} message ${mid}`, { channel: dest.named, chat_id: dest.chat_id, message_id: mid })
    : fail("sendMessage answered no message id");
}

async function channel(json: boolean, call: Extract<Call, { method: "channel" }>): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer.replace(/^error: /, ""));
  const r = await thisCall("read", "getChat", { chat_id: dest.chat_id });
  if (r.ok !== true) return fail(r.description ?? r.error ?? "getChat failed");
  const chat = r.result ?? {};
  return ok(`${chat.title ?? chat.username ?? dest.named} (${chat.type ?? "chat"})`, {
    id: chat.id, title: chat.title, type: chat.type, ...(chat.description ? { description: chat.description } : {}),
  });
}

async function user(json: boolean, call: Extract<Call, { method: "user" }>): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  // A user is seen through a chat: the declared default is the window.
  const dest = resolveDestination(undefined, cfg);
  if (!dest.ok) return fail(dest.error);
  const who = call.who.startsWith("#") || call.who.startsWith("@") ? resolveDestination(call.who, cfg) : undefined;
  const user_id = who && who.ok ? who.chat_id : call.who;
  if (who && !who.ok) return fail(who.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer.replace(/^error: /, ""));
  const r = await thisCall("read", "getChatMember", { chat_id: dest.chat_id, user_id });
  if (r.ok !== true) return fail(r.description ?? r.error ?? "getChatMember failed");
  const member = r.result ?? {};
  return ok(`@${member.user?.username ?? user_id} (${member.status ?? "member"})`, {
    id: member.user?.id ?? user_id,
    ...(member.user?.username ? { username: member.user.username } : {}),
    ...(member.user?.first_name ? { first_name: member.user.first_name } : {}),
    status: member.status ?? "member",
  });
}

async function react(json: boolean, call: Extract<Call, { method: "react" }>): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer.replace(/^error: /, ""));
  const r = await thisCall("write", "setMessageReaction", {
    chat_id: dest.chat_id,
    message_id: Number(call.id),
    reaction: [{ type: "emoji", emoji: call.emoji }],
  });
  if (r.ok !== true) return fail(r.description ?? r.error ?? "setMessageReaction failed");
  return ok(`reacted ${call.emoji} on message ${call.id}`);
}

async function edit(json: boolean, call: Extract<Call, { method: "edit" }>): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer.replace(/^error: /, ""));
  const r = await thisCall("write", "editMessageText", {
    chat_id: dest.chat_id,
    message_id: Number(call.id),
    text: call.text,
  });
  if (r.ok !== true) return fail(r.description ?? r.error ?? "editMessageText failed");
  const mid = r.result?.message_id ?? Number(call.id);
  return ok(`message ${mid} edited`, { message_id: mid, text: call.text });
}

async function del(json: boolean, call: Extract<Call, { method: "delete" }>): Promise<Result> {
  const cfg = await configuration();
  if (typeof cfg === "string") return fail(cfg);
  const dest = resolveDestination(call.channel, cfg);
  if (!dest.ok) return fail(dest.error);
  const missing = await missingToken();
  if (missing) return fail(missing.answer.replace(/^error: /, ""));
  const r = await thisCall("write", "deleteMessage", { chat_id: dest.chat_id, message_id: Number(call.id) });
  if (r.ok !== true) return fail(r.description ?? r.error ?? "deleteMessage failed");
  return ok(`message ${call.id} deleted`, { message_id: Number(call.id) });
}
await bus.register({ name, kind: "agent", descr: "telegram: a bot on the bus — post, read, subscriptions" }, { createOnly: false });
// Not configured at start: say why on stderr and exit 78 (EX_CONFIG), which
// the runner reads as "unconfigured" and does not restart until a reload
// (slack.ts, Plans/R1.0-Release/TODO.md M.13). A secret or configuration
// that cannot be read because the BUS failed is not unconfigured: exit 1,
// which the runner restarts with backoff (tg1-review P2-1/P2-2) — a
// transient failure must not become a permanent outage. A secret that
// changes while it runs is still answered per call.
{
  const why = await missingToken();
  if (why) {
    console.error(`telegram: ${why.answer.replace(/^error: /, "")}`);
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
await bus.serve(handle, { signal: stop.signal, drainTimeout: 5000 });
