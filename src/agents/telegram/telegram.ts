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
import { configFault, parseConfig, secretFault, type Config } from "./config.js";
import { parseCall } from "./parse.js";
import { help as helpText } from "./help.js";
import { telegramWeb } from "./telegramapi.js";

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

/** Every Telegram call goes through here: the secret's token and endpoint. */
export async function call(method: string, params: Record<string, unknown> = {}) {
  const env = await secrets();
  if (!env.TELEGRAM_BOT_TOKEN) throw new Error("TELEGRAM_BOT_TOKEN is not set");
  return telegramWeb(env.TELEGRAM_BOT_TOKEN, env.TELEGRAM_API_URL, method, params);
}

/** The instance's configuration as it is now, or the reason it is refused
 *  (config.ts): read as this record, so a changed configuration reaches the
 *  next call without a restart. No configuration at all is none, not an
 *  error — the calls that need a destination are refused then. A read that
 *  failed for any other reason throws: the caller answers the bus's name
 *  (tg1-review P2-2), never a silent "no default" for one that was
 *  declared. */
export async function configuration(): Promise<Config | string> {
  let raw: unknown;
  try {
    raw = await bus.record(name).config();
  } catch (e) {
    const f = configFault(e);
    if (f.kind === "bus") throw new Error(f.message);
    return { aliases: {} };
  }
  return parseConfig(raw);
}

async function whoami(): Promise<string> {
  // A transport failure is an answer too: the method's name, never the URL,
  // whose path segment is the token (telegramapi.ts).
  let r;
  try {
    r = await call("getMe");
  } catch (e) {
    return `error: ${e instanceof Error ? e.message : "getMe failed"}`;
  }
  if (r.ok !== true) return `error: ${r.description ?? r.error ?? "getMe failed"}`;
  const me = r.result ?? {};
  return `@${me.username} (id ${me.id})`;
}

// The client's serve loop acks each message before the handler runs and
// refuses receipts there, so they are consumed and never answered here.
async function handle(e: Message): Promise<string | null> {
  // Both call forms arrive through one parser; methods later waves build
  // fall through to help, which is also the answer for anything unknown
  // (TG.2). At TG.1's scope the parser accepts them, help answers them.
  switch (parseCall(e.body).method) {
    case "whoami": {
      const missing = await missingToken();
      if (missing) return missing.answer;
      return whoami();
    }
    default:
      return help;
  }
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
