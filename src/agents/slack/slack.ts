#!/usr/bin/env bun
// The Slack agent (Plans/R1.2/slack/README.md#slack-agent): one process that
// serves itself over the bus, reads its tokens from the record's bus secret,
// and puts a Slack workspace behind #slack@<realm>.
//
// S.1: register, serve, help, whoami, and the bus secret. Later waves add
// parsing (S.2), Slack calls (S.3), subscriptions and forwarding (S.4-S.6).
import { Bus, type Message } from "../../clients/typescript/src/index.js";
import { parseEnvFile } from "./envfile.js";
import { slackCall } from "./slackapi.js";
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
export async function call(method: string, params: Record<string, unknown> = {}) {
  const env = await secrets();
  if (!env.SLACK_BOT_TOKEN) throw new Error("SLACK_BOT_TOKEN is not set");
  return slackCall(env.SLACK_BOT_TOKEN, env.SLACK_API_URL, method, params);
}

async function whoami(): Promise<string> {
  let r: any;
  try {
    r = await call("auth.test");
  } catch {
    return "error: Slack did not answer";
  }
  if (r.ok !== true) return `error: ${r.error ?? "auth.test failed"}`;
  return `${r.user} on ${r.team} (${r.url})`;
}

// The client's serve loop acks each message before the handler runs and
// refuses receipts there, so they are consumed and never answered here.
// Both call forms arrive through one parser; methods later waves build fall
// through to help, which is also the answer for anything unknown (S.2).
async function handle(e: Message): Promise<string | null> {
  switch (parseCall(e.body).method) {
    case "help":
      return help;
    case "whoami": {
      const missing = await missingToken();
      if (missing) return missing;
      return whoami();
    }
    default:
      return help;
  }
}

await bus.register({ name, kind: "agent", descr: "slack: a workspace on the bus — post, read, subscriptions" }, { createOnly: false });
const stop = new AbortController();
process.on("SIGINT", () => stop.abort());
process.on("SIGTERM", () => stop.abort());
await bus.serve(handle, { signal: stop.signal, drainTimeout: 5000 });
