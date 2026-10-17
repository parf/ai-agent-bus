#!/usr/bin/env bun
// The Slack agent (Plans/R1.2/slack/README.md#slack-agent): one process that
// serves itself over the bus, reads its tokens from the record's bus secret,
// and puts a Slack workspace behind #slack@<realm>.
//
// S.1: register, serve, help, whoami, and the bus secret. Later waves add
// parsing (S.2), Slack calls (S.3), subscriptions and forwarding (S.4-S.6).
import { Bus, type Message } from "../../clients/typescript/src/index.js";
import { parseEnvFile } from "./envfile.js";
import { slackWeb } from "./slackapi.js";
import { parseCall } from "./parse.js";

const name = (process.env.AGENT_BUS_NAME ?? "").trim().toLowerCase();
if (!name) {
  console.error("slack: AGENT_BUS_NAME is not set; the agent cannot know whose secret to read");
  process.exit(1);
}

const bus = Bus.connect({ expectName: name });
const help = `slack: a Slack workspace on the bus
  post <#chan|CID|email> <text…>        post; answers the permalink
  read <#chan|CID> [N]                  the last N messages, oldest first
  channels [filter]                     the channels the bot is in
  users [filter]                        the workspace's people
  user <id|email|@name>                 one person
  channel <#chan|CID>                   one channel
  thread <#chan|CID> <ts> [N]           one thread's messages
  react <#chan|CID> <ts> <emoji>        add a reaction
  edit <#chan|CID> <ts> <text…>         edit the bot's own message
  delete <#chan|CID> <ts>               delete the bot's own message
  join <#chan|CID>                      join a public channel
  sub add <#chan|CID> <target> [filter] forward a channel's messages
  sub list                              the subscriptions
  sub edit <id> [channel=…] [target=…] [filter=…]
  sub delete <id>
  whoami                                the bot's own identity
help lists this; a call may also be JSON: {"method":"post","channel":…}`;

// The record's secret, dotenv form: SLACK_BOT_TOKEN, SLACK_APP_TOKEN and an
// optional SLACK_API_URL. Never logged, never taken from the process env —
// the process env is the runner's, and a token arriving by accident there is
// one nobody set (Plans/R1.2/slack/README.md#how-it-runs). Read per call, so
// a changed secret reaches the next message without a restart.
async function secrets(): Promise<Record<string, string>> {
  const secret = await bus.record(name).secret();
  return parseEnvFile("the bus secret", secret.reveal());
}

/** The first thing the agent cannot work without, as an answer to its caller. */
export async function missingToken(): Promise<string | undefined> {
  let env: Record<string, string>;
  try {
    env = await secrets();
  } catch {
    return "error: the bus secret is not set";
  }
  if (!env.SLACK_BOT_TOKEN) return "error: SLACK_BOT_TOKEN is not set";
  if (!env.SLACK_APP_TOKEN) return "error: SLACK_APP_TOKEN is not set";
  return undefined;
}

/** Every Slack call goes through here: bearer bot token, JSON body. */
async function web(kind: "write" | "read", method: string, params: Record<string, unknown> = {}) {
  const env = await secrets();
  if (!env.SLACK_BOT_TOKEN) throw new Error("SLACK_BOT_TOKEN is not set");
  return slackWeb(env.SLACK_BOT_TOKEN, env.SLACK_API_URL, kind, method, params);
}

/** A channel as Slack wants it: a CID as it is, an email to its user id, a
 *  #name found in the channel list (refreshed once on a miss from S.3b's
 *  cache; a plain list lookup here). */
async function resolveChannel(channel: string): Promise<string> {
  if (channel.includes("@")) {
    const found = await web("read", "users.lookupByEmail", { email: channel });
    if (found.ok === true && typeof found.user?.id === "string") return found.user.id;
    return `error: ${channel}: ${found.error ?? "no such user"}`;
  }
  if (channel.startsWith("#")) {
    const list = await web("read", "conversations.list", { types: "public_channel,private_channel", limit: 200 });
    if (list.ok !== true) return `error: ${list.error ?? "conversations.list failed"}`;
    const hit = (list.channels ?? []).find((c: any) => c.name === channel.slice(1));
    if (!hit) return `error: no channel ${channel}`;
    return hit.id;
  }
  return channel;
}

async function post(channel: string, text: string, extra: { thread_ts?: string; username?: string; icon?: string }): Promise<string> {
  const where = await resolveChannel(channel);
  if (where.startsWith("error:")) return where;
  const params: Record<string, unknown> = { channel: where, text };
  if (extra.thread_ts !== undefined) params.thread_ts = extra.thread_ts;
  if (extra.username !== undefined) params.username = extra.username;
  if (extra.icon !== undefined) {
    if (extra.icon.startsWith("http://") || extra.icon.startsWith("https://")) params.icon_url = extra.icon;
    else params.icon_emoji = extra.icon;
  }
  const sent = await web("write", "chat.postMessage", params);
  if (sent.ok !== true) return `error: ${sent.error ?? "chat.postMessage failed"}`;
  const link = await web("read", "chat.getPermalink", { channel: where, message_ts: sent.ts });
  if (link.ok !== true || typeof link.permalink !== "string") return `${channel} ts=${sent.ts}`;
  return link.permalink;
}

async function read(channel: string, limit: number): Promise<string> {
  const where = await resolveChannel(channel);
  if (where.startsWith("error:")) return where;
  const cap = Math.min(limit, 200);
  const messages: { ts: string; user?: string; text?: string }[] = [];
  let cursor: string | undefined;
  do {
    const page: any = await web("read", "conversations.history", { channel: where, limit: Math.min(cap - messages.length, 200) || 200, ...(cursor ? { cursor } : {}) });
    if (page.ok !== true) return `error: ${page.error ?? "conversations.history failed"}`;
    for (const m of page.messages ?? []) if (typeof m.ts === "string") messages.push({ ts: m.ts, user: typeof m.user === "string" ? m.user : undefined, text: typeof m.text === "string" ? m.text : undefined });
    cursor = typeof page.response_metadata?.next_cursor === "string" && page.response_metadata.next_cursor !== "" ? page.response_metadata.next_cursor : undefined;
  } while (messages.length < cap && cursor);
  return messages.slice(0, cap).reverse().map(m => `${m.ts} ${m.user ?? "unknown"} ${m.text ?? ""}`).join("\n");
}

// The client's serve loop acks each message before the handler runs and
// refuses receipts there, so they are consumed and never answered here.
// Both call forms arrive through one parser; methods later waves build fall
// through to help, which is also the answer for anything unknown (S.2).
async function handle(e: Message): Promise<string | null> {
  const call = parseCall(e.body);
  const missing = call.method === "help" ? undefined : await missingToken();
  if (missing) return missing;
  try {
    switch (call.method) {
      case "help":
        return help;
      case "whoami": {
        const r = await web("read", "auth.test");
        if (r.ok !== true) return `error: ${r.error ?? "auth.test failed"}`;
        return `${r.user} on ${r.team} (${r.url})`;
      }
      case "post":
        return post(call.channel, call.text, call);
      case "read":
        return read(call.channel, call.limit);
      default:
        return help; // later waves build the rest
    }
  } catch (e) {
    return `error: ${e instanceof Error ? e.message : "the call failed"}`;
  }
}

await bus.register({ name, kind: "agent", descr: "slack: a workspace on the bus — post, read, subscriptions" }, { createOnly: false });
const stop = new AbortController();
process.on("SIGINT", () => stop.abort());
process.on("SIGTERM", () => stop.abort());
await bus.serve(handle, { signal: stop.signal, drainTimeout: 5000 });
