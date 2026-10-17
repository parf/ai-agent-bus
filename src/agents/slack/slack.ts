#!/usr/bin/env bun
// The Slack agent (Plans/R1.2/slack/README.md#slack-agent): one process that
// serves itself over the bus, reads its tokens from the record's bus secret,
// and puts a Slack workspace behind #slack@<realm>.
//
// Each call runs in one context: one snapshot of the bus secret, so a call
// never mixes two workspaces, and one absolute deadline, the caller's, which
// every Slack request and retry stays inside. Every method answers a result;
// the one output boundary renders it as text or, for a JSON call, as Slack's
// own `{"ok": …}` shape (README § JSON answers), and scrubs the agent's
// credentials from whatever leaves.
import { createHash } from "node:crypto";
import { hostname } from "node:os";
import { Bus, type Message } from "../../clients/typescript/src/index.js";
import { parseEnvFile } from "./envfile.js";
import { DeadlinePassed, endpoint, scrub, slackWeb } from "./slackapi.js";
import { parseCall, type Call } from "./parse.js";
import { KVNotFound } from "../../clients/typescript/src/index.js";
import { mayChangeSubscriptions, subAdd, subDelete, subDisable, subEdit, subList, withSub, type Store, type Subscription, type SubResult } from "./subs.js";
import { Forwarder, Posted, type Identity } from "./forward.js";
import { SocketMode } from "./socket.js";
import { postAs } from "./postas.js";
import { describe } from "./describe.js";
import { stopFilters } from "./filters.js";

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
help lists this, agent-bus call <this agent> --help describes it in full; a call may also be JSON: {"method":"post","channel":…}`;
// Subscriptions are changed by whoever manages their destination (S.8), so
// every caller is shown how; only this agent's own roles change any.
const helpFor = (e: Message) => help + (mayChangeSubscriptions(e, name) ? "\nyou may change any subscription" : "\nsub add, edit and delete: for a destination you manage");

/** The tokens as last read, for scrubbing logs. */
const known: { bot?: string; app?: string } = {};

/** A call with no deadline of its caller's still ends. */
const defaultBudgetMs = 60_000;

/** One call's context: the secret as it was when the call began, and the deadline. */
type Ctx = { env: Record<string, string>; deadline: number; key: string };

/** What a method answers: its text form and its JSON fields, or a failure. */
type Result = { ok: true; text: string; fields: Record<string, unknown> } | { ok: false; error: string };
const fail = (error: string): Result => ({ ok: false, error });
const ok = (text: string, fields: Record<string, unknown> = {}): Result => ({ ok: true, text, fields });

// The record's secret, dotenv form: SLACK_BOT_TOKEN, SLACK_APP_TOKEN and an
// optional SLACK_API_URL. Never logged, never taken from the process env —
// the process env is the runner's, and a token arriving by accident there is
// one nobody set (Plans/R1.2/slack/README.md#how-it-runs). Read per call, so
// a changed secret reaches the next message without a restart.
async function secrets(): Promise<Record<string, string>> {
  const secret = await bus.record(name).secret();
  return parseEnvFile("the bus secret", secret.reveal());
}

/** The first thing the agent cannot work without, or the call's context. */
async function context(e?: Message): Promise<Ctx | string> {
  let env: Record<string, string>;
  try {
    env = await secrets();
  } catch {
    return "the bus secret is not set";
  }
  known.bot = env.SLACK_BOT_TOKEN;
  known.app = env.SLACK_APP_TOKEN;
  if (!env.SLACK_BOT_TOKEN) return "SLACK_BOT_TOKEN is not set";
  if (!env.SLACK_APP_TOKEN) return "SLACK_APP_TOKEN is not set";
  // The directories are this workspace's: keyed by the endpoint and the
  // token's digest, so a changed secret starts empty (slack-review-S.3b).
  const key = endpoint(env.SLACK_API_URL) + "|" + createHash("sha256").update(env.SLACK_BOT_TOKEN).digest("hex");
  return { env, deadline: deadlineOf(e), key };
}

function deadlineOf(e?: Message): number {
  const raw = e?.deadline;
  const at = typeof raw === "string" ? Date.parse(raw) : NaN;
  // Go's zero time, or none, is no deadline of the caller's.
  if (!Number.isFinite(at) || new Date(at).getUTCFullYear() <= 1) return Date.now() + defaultBudgetMs;
  return at;
}

/** Every Slack call goes through here, inside the call's deadline. */
function web(ctx: Ctx, kind: "write" | "read", method: string, params: Record<string, unknown> = {}) {
  return slackWeb(ctx.env.SLACK_BOT_TOKEN!, ctx.env.SLACK_API_URL, kind, method, params, ctx.deadline);
}

type Channel = { id: string; name: string; num_members?: number; is_member?: boolean; topic?: string };
type Person = { id: string; name: string; real_name?: string; email?: string; is_bot?: boolean; deleted?: boolean };

/** Cached directories, one per workspace (the context's key): #name and
 *  @name resolve through them, refreshed once when a lookup misses (S.3b). */
const directories = new Map<string, { channels?: Channel[]; users?: Person[]; strangers?: Set<string> }>();
const dir = (ctx: Ctx) => {
  let d = directories.get(ctx.key);
  if (!d) directories.set(ctx.key, (d = {}));
  return d;
};

const nextCursor = (page: any): string | undefined =>
  typeof page.response_metadata?.next_cursor === "string" && page.response_metadata.next_cursor !== "" ? page.response_metadata.next_cursor : undefined;

async function listChannels(ctx: Ctx): Promise<Channel[]> {
  const d = dir(ctx);
  if (d.channels) return d.channels;
  const all: any[] = [];
  let cursor: string | undefined;
  do {
    const page: any = await web(ctx, "read", "conversations.list", { types: "public_channel,private_channel", limit: 200, ...(cursor ? { cursor } : {}) });
    if (page.ok !== true) throw new Error(page.error);
    all.push(...(Array.isArray(page.channels) ? page.channels : []));
    cursor = nextCursor(page);
  } while (cursor);
  d.channels = all.map((c: any) => ({ id: String(c.id), name: String(c.name), num_members: typeof c.num_members === "number" ? c.num_members : undefined, is_member: c.is_member === true, topic: typeof c.topic?.value === "string" ? c.topic.value : undefined }));
  return d.channels;
}

async function listUsers(ctx: Ctx): Promise<Person[]> {
  const d = dir(ctx);
  if (d.users) return d.users;
  const all: any[] = [];
  let cursor: string | undefined;
  do {
    const page: any = await web(ctx, "read", "users.list", { limit: 200, ...(cursor ? { cursor } : {}) });
    if (page.ok !== true) throw new Error(page.error);
    all.push(...(Array.isArray(page.members) ? page.members : []));
    cursor = nextCursor(page);
  } while (cursor);
  d.users = all.map(personOf);
  return d.users;
}

const personOf = (m: any): Person => ({ id: String(m.id), name: String(m.name), real_name: typeof m.real_name === "string" ? m.real_name : undefined, email: typeof m.profile?.email === "string" ? m.profile.email : undefined, is_bot: m.is_bot === true, deleted: m.deleted === true });

/** A user id's person: from the directory, loaded at start and kept in
 *  memory; one who is not in it — joined since — is asked for once and kept,
 *  and one Slack does not know is not asked for again. */
async function whoIs(ctx: Ctx, id: string): Promise<Person | undefined> {
  if (!/^[UW][A-Z0-9]+$/.test(id)) return undefined;
  let all: Person[];
  try {
    all = await listUsers(ctx);
  } catch {
    return undefined;
  }
  const hit = all.find(u => u.id === id);
  if (hit) return hit;
  const d = dir(ctx);
  if (d.strangers?.has(id)) return undefined;
  const one: any = await web(ctx, "read", "users.info", { user: id }).catch(() => undefined);
  if (one?.ok === true && one.user) {
    const p = personOf(one.user);
    all.push(p);
    return p;
  }
  (d.strangers ??= new Set()).add(id);
  return undefined;
}

type Resolved = { id: string } | { error: string };

/** A channel as Slack wants it: a CID as it is, an email to its user id, a
 *  #name from the cached channel list, refreshed once if it misses. */
async function resolveChannel(ctx: Ctx, channel: string): Promise<Resolved> {
  if (channel.includes("@")) {
    const found = await web(ctx, "read", "users.lookupByEmail", { email: channel });
    if (found.ok === true && typeof found.user?.id === "string") return { id: found.user.id };
    return { error: `${channel}: ${found.error ?? "no such user"}` };
  }
  if (channel.startsWith("#")) {
    for (let attempt = 0; attempt < 2; attempt++) {
      const hit = (await listChannels(ctx)).find(c => c.name === channel.slice(1));
      if (hit) return { id: hit.id };
      dir(ctx).channels = undefined; // one refresh, then it really is not there
    }
    return { error: `no channel ${channel}` };
  }
  return { id: channel };
}

/** A person: an email by lookup, an id as it is, an @name from the cached
 *  user list, refreshed once if it misses (slack-review-S.3b). */
async function resolveUser(ctx: Ctx, who: string): Promise<Resolved> {
  if (who.includes("@") && !who.startsWith("@")) {
    const found = await web(ctx, "read", "users.lookupByEmail", { email: who });
    if (found.ok === true && typeof found.user?.id === "string") return { id: found.user.id };
    return { error: `${who}: ${found.error ?? "no such user"}` };
  }
  if (!who.startsWith("@") && /^[UW][A-Z0-9]+$/.test(who)) return { id: who };
  const bare = who.replace(/^@/, "");
  for (let attempt = 0; attempt < 2; attempt++) {
    const hit = (await listUsers(ctx)).find(u => u.name === bare);
    if (hit) return { id: hit.id };
    dir(ctx).users = undefined;
  }
  return { error: `no user ${who}` };
}

type Line = { ts: string; user?: string; user_name?: string; text?: string; thread_ts?: string };
// A bot's post has no user: it is shown by the name it posted under, else its bot id.
const authorOf = (m: any): string | undefined => [m.user, m.username, m.bot_id].find(v => typeof v === "string" && v !== "");
const lineOf = (m: any): Line | undefined =>
  typeof m?.ts === "string" ? { ts: m.ts, user: authorOf(m), text: typeof m.text === "string" ? m.text : undefined, ...(typeof m.thread_ts === "string" ? { thread_ts: m.thread_ts } : {}) } : undefined;
const mention = /<@([UW][A-Z0-9]+)(?:\|[^>]*)?>/g;

/** Messages answered with their authors' names: JSON keeps the ids and adds
 *  user_name; the text form shows @name, mentions in the text included. */
async function named(ctx: Ctx, lines: Line[], empty: string): Promise<Result> {
  const names = new Map<string, string | undefined>();
  const nameOf = async (id: string) => {
    if (!names.has(id)) names.set(id, (await whoIs(ctx, id))?.name);
    return names.get(id);
  };
  const out: string[] = [];
  for (const m of lines) {
    const who = m.user ? await nameOf(m.user) : undefined;
    if (who) m.user_name = who;
    let text = m.text ?? "";
    for (const [, id] of [...text.matchAll(mention)]) await nameOf(id!);
    text = text.replace(mention, (all, id) => (names.get(id) ? `@${names.get(id)}` : all));
    out.push(`${m.ts} ${who ? `@${who}` : (m.user ?? "unknown")} ${text}`);
  }
  return ok(out.join("\n") || empty, { messages: lines });
}

/** Pages of messages until N are had. `newestFirst` is history's order, which
 *  is reversed to oldest first; replies come oldest first already. */
async function messages(ctx: Ctx, method: string, params: Record<string, unknown>, limit: number, newestFirst: boolean): Promise<Line[] | { error: string }> {
  const cap = Math.min(limit, 200);
  const got: Line[] = [];
  let cursor: string | undefined;
  do {
    const page: any = await web(ctx, "read", method, { ...params, limit: Math.min(cap - got.length, 200) || 200, ...(cursor ? { cursor } : {}) });
    if (page.ok !== true) return { error: page.error };
    for (const m of Array.isArray(page.messages) ? page.messages : []) {
      const line = lineOf(m);
      if (line) got.push(line);
    }
    cursor = nextCursor(page);
  } while (got.length < cap && cursor);
  const kept = got.slice(0, cap);
  return newestFirst ? kept.reverse() : kept;
}

async function post(ctx: Ctx, call: Extract<Call, { method: "post" }>): Promise<Result> {
  const where = await resolveChannel(ctx, call.channel);
  if ("error" in where) return fail(where.error);
  const params: Record<string, unknown> = { channel: where.id, text: call.text };
  if (call.thread_ts !== undefined) params.thread_ts = call.thread_ts;
  // Its name and icon: the call's, the configuration's, the secret's (postas.ts).
  Object.assign(params, postAs(call, process.env, ctx.env, hostname()));
  posted.begin();
  let sent: any;
  try {
    sent = await web(ctx, "write", "chat.postMessage", params);
  } finally {
    posted.end(sent?.ok === true && typeof sent.channel === "string" ? sent.channel : where.id, sent?.ok === true ? String(sent.ts) : undefined);
  }
  if (sent.ok !== true) return fail(sent.error);
  // Where it landed is the conversation Slack answered with — a DM's own id
  // when posted to a person — and a post that landed is never reported as
  // failed, nor posted again, because its permalink could not be had.
  const channel = typeof sent.channel === "string" ? sent.channel : where.id;
  const ts = String(sent.ts);
  try {
    const link = await web(ctx, "read", "chat.getPermalink", { channel, message_ts: ts });
    if (link.ok === true && typeof link.permalink === "string") return ok(link.permalink, { channel, ts, permalink: link.permalink });
  } catch {
    /* the message is posted; only its link is missing */
  }
  return ok(`${channel} ts=${ts}`, { channel, ts });
}

async function run(ctx: Ctx, e: Message, call: Call): Promise<Result> {
  switch (call.method) {
    case "whoami": {
      const r = await web(ctx, "read", "auth.test");
      if (r.ok !== true) return fail(r.error);
      return ok(`${r.user} on ${r.team} (${r.url})`, { user: r.user, user_id: r.user_id, team: r.team, team_id: r.team_id, url: r.url });
    }
    case "post":
      return post(ctx, call);
    case "read": {
      const where = await resolveChannel(ctx, call.channel);
      if ("error" in where) return fail(where.error);
      const lines = await messages(ctx, "conversations.history", { channel: where.id }, call.limit, true);
      if ("error" in lines) return fail(lines.error);
      return named(ctx, lines, "");
    }
    case "thread": {
      const where = await resolveChannel(ctx, call.channel);
      if ("error" in where) return fail(where.error);
      const lines = await messages(ctx, "conversations.replies", { channel: where.id, ts: call.ts }, call.limit, false);
      if ("error" in lines) return fail(lines.error);
      return named(ctx, lines, "(no replies)");
    }
    case "channels": {
      const all = await listChannels(ctx);
      const rows = (call.filter ? all.filter(c => c.name.toLowerCase().includes(call.filter!.toLowerCase())) : all).filter(c => !call.member_only || c.is_member);
      const text = rows.map(c => `${c.id} #${c.name}${c.num_members !== undefined ? ` members=${c.num_members}` : ""}${c.is_member ? " member" : ""}${c.topic ? ` topic=${c.topic}` : ""}`).join("\n") || "(no channels)";
      return ok(text, { channels: rows.map(c => ({ id: c.id, name: c.name, members: c.num_members, is_member: c.is_member === true, topic: c.topic })) });
    }
    case "users": {
      const all = (await listUsers(ctx)).filter(u => !u.deleted && !u.is_bot);
      const rows = call.filter ? all.filter(u => (u.name + " " + (u.real_name ?? "") + " " + (u.email ?? "")).toLowerCase().includes(call.filter!.toLowerCase())) : all;
      const text = rows.map(u => `${u.id} ${u.name}${u.real_name ? ` (${u.real_name})` : ""}${u.email ? ` <${u.email}>` : ""}`).join("\n") || "(no users)";
      return ok(text, { users: rows.map(u => ({ id: u.id, name: u.name, real_name: u.real_name, email: u.email })) });
    }
    case "user": {
      const id = await resolveUser(ctx, call.who);
      if ("error" in id) return fail(id.error);
      const one: any = await web(ctx, "read", "users.info", { user: id.id });
      if (one.ok !== true) return fail(one.error);
      const u = one.user ?? {};
      const user = { id: u.id, name: u.name, real_name: u.real_name, email: u.profile?.email, tz: u.tz, title: u.profile?.title, is_bot: u.is_bot === true };
      return ok(`${u.id} ${u.name}${u.real_name ? ` (${u.real_name})` : ""}${u.profile?.email ? ` <${u.profile.email}>` : ""} tz=${u.tz ?? "?"}${u.profile?.title ? ` title=${u.profile.title}` : ""}${u.is_bot ? " bot" : ""}`, { user });
    }
    case "channel": {
      const where = await resolveChannel(ctx, call.channel);
      if ("error" in where) return fail(where.error);
      // Slack leaves the member count out unless asked for it (S.3b).
      const one: any = await web(ctx, "read", "conversations.info", { channel: where.id, include_num_members: true });
      if (one.ok !== true) return fail(one.error);
      const c = one.channel ?? {};
      const members = typeof c.num_members === "number" ? c.num_members : undefined;
      const channel = { id: c.id, name: c.name, topic: c.topic?.value, purpose: c.purpose?.value, members, created: c.created };
      return ok(`${c.id} #${c.name}${members !== undefined ? ` members=${members}` : ""}${c.topic?.value ? ` topic=${c.topic.value}` : ""}${c.purpose?.value ? ` purpose=${c.purpose.value}` : ""} created=${c.created ?? "?"}`, { channel });
    }
    case "react":
    case "edit":
    case "delete":
    case "join": {
      const where = await resolveChannel(ctx, call.channel);
      if ("error" in where) return fail(where.error);
      const [method, params, word]: [string, Record<string, unknown>, string] =
        call.method === "react" ? ["reactions.add", { channel: where.id, timestamp: call.ts, name: call.emoji.replace(/^:|:$/g, "") }, "reacted"]
        : call.method === "edit" ? ["chat.update", { channel: where.id, ts: call.ts, text: call.text }, "edited"]
        : call.method === "delete" ? ["chat.delete", { channel: where.id, ts: call.ts }, "deleted"]
        : ["conversations.join", { channel: where.id }, `joined ${call.channel}`];
      const done: any = await web(ctx, "write", method, params);
      if (done.ok !== true) return fail(done.error);
      return ok(word, { channel: where.id, ...("ts" in call ? { ts: call.ts } : {}) });
    }
    case "sub.list":
      return subList(bus, name, id => forwarder.pending(id));
    case "sub.add":
    case "sub.edit":
    case "sub.delete": {
      // Whoever manages a subscription's destination may change it, and this
      // agent's own Owner and Maintainers may change any (S.8). Control is
      // asked of the daemon for the sender it delivered, never the body's.
      const any = mayChangeSubscriptions(e, name);
      const guard = async (sub: Subscription) => {
        if (any) return undefined;
        const to = call.method === "sub.edit" ? call.edits.target : undefined;
        for (const target of to !== undefined ? [sub.target, to] : [sub.target]) {
          const why = await uncontrolled(e.from, target);
          if (why) return why;
        }
        return undefined;
      };
      if (call.method === "sub.add") {
        const why = any ? undefined : await uncontrolled(e.from, call.target);
        return why ? fail(why) : subAdd(bus, name, e, call.channel, call.target, call.filter);
      }
      if (call.method === "sub.delete") return subDelete(bus, name, call.id, guard);
      return subEdit(bus, name, call.id, call.edits, guard);
    }
    default:
      return ok(help, { help });
  }
}

/** The one output boundary: text or JSON by the request's form, and nothing
 *  of the agent's credentials in either (slack-review-S.1). */
function render(json: boolean, r: Result, ctx?: Ctx): string {
  const out = json
    ? JSON.stringify(r.ok ? { ok: true, ...r.fields } : { ok: false, error: r.error })
    : r.ok ? r.text : `error: ${r.error}`;
  return ctx ? scrub(out, ctx.env.SLACK_BOT_TOKEN, ctx.env.SLACK_APP_TOKEN) : out;
}

// The client's serve loop acks each message before the handler runs and
// refuses receipts there, so they are consumed and never answered here.
/** Why `who` may not point a subscription at `target`, or undefined when it
 *  controls it: manages it, by the daemon's answer. A destination this agent
 *  may not see is one nothing could be forwarded to either. */
async function uncontrolled(who: string, target: string): Promise<string | undefined> {
  try {
    if (await bus.manages(who, target)) return undefined;
  } catch (err) {
    if ((err as any)?.status === 404) return `${target} is not a destination this agent can see; its ACL must admit ${name}`;
    throw err;
  }
  return `${who} does not control ${target}: a subscription is changed by whoever manages its destination`;
}

async function handle(e: Message): Promise<string | null> {
  // Asked for help, it says what it is for and how to call it, as this
  // caller may (docs/constitution.md#agent-self-description). Any other ask
  // carries no input, and is answered as no input is.
  if (e.ask === "help") return describe(name, mayChangeSubscriptions(e, name));
  const json = e.body.trim().startsWith("{");
  const call = parseCall(e.body);
  if (call.method === "help") {
    const help = helpFor(e);
    // A JSON call that is no call is refused, with the help to say why.
    if (json && !isHelpCall(e.body)) return JSON.stringify({ ok: false, error: "not a call this agent knows", help });
    return render(json, ok(help, { help }));
  }
  const ctx = await context(e);
  if (typeof ctx === "string") return render(json, fail(ctx));
  try {
    return render(json, await run(ctx, e, call), ctx);
  } catch (err) {
    const why = err instanceof DeadlinePassed ? "the caller's deadline passed" : err instanceof Error ? err.message : "the call failed";
    return render(json, fail(why), ctx);
  }
}

function isHelpCall(body: string): boolean {
  try {
    return JSON.parse(body)?.method === "help";
  } catch {
    return false;
  }
}

// Forwarding (S.5) and Socket Mode (S.6). Each event gets a context of its
// own: the secret as it is then, and a 15 s budget for its Slack lookups.
const store = (): Store => bus.record(name) as unknown as Store;
let identity: Identity | undefined;
const posted = new Posted();
let identityFor = ""; // the workspace key it was learned for
async function eventCtx(): Promise<Ctx> {
  const ctx = await context();
  if (typeof ctx === "string") throw new Error(ctx);
  ctx.deadline = Date.now() + 15_000;
  return ctx;
}
const forwarder = new Forwarder({
  async subs() {
    const r = await subList(store(), name);
    return r.ok ? (r.fields.subs as Subscription[]) : [];
  },
  async channelId(channel) {
    if (!channel.startsWith("#")) return channel;
    const ctx = await eventCtx();
    const where = await resolveChannel(ctx, channel);
    return "id" in where ? where.id : undefined;
  },
  async userName(id) {
    try {
      return (await whoIs(await eventCtx(), id))?.name ?? "";
    } catch {
      return "";
    }
  },
  async channelName(id) {
    try {
      const hit = (await listChannels(await eventCtx())).find(c => c.id === id);
      return hit ? `#${hit.name}` : "";
    } catch {
      return "";
    }
  },
  async send(to, body, topic, tag) {
    await bus.send(to, body, { topic, tag });
  },
  identity: () => ensureIdentity(),
  posted: (channel, ts) => posted.mine(channel, ts),
  async persistCount(id, forwarded) {
    await (bus.record(name) as any).kv.increment(`stat.${id}.forwarded`, { by: forwarded });
  },
  async persistError(id, lastError) {
    await (bus.record(name) as any).kv.set("string", `stat.${id}.last_error`, lastError, { mode: "set" });
  },
  async exists(id) {
    try {
      await store().kv.get("json", `sub.${id}`);
      return true;
    } catch (err) {
      if (err instanceof KVNotFound) return false;
      throw err;
    }
  },
  async disable(id) {
    await subDisable(store(), name, id);
  },
  serialize: (id, fn) => withSub(id, fn),
  async forget(id) {
    const kv = store().kv;
    for (const [kind, key] of [["int", `stat.${id}.forwarded`], ["string", `stat.${id}.last_error`]] as const) {
      await kv.delete(kind, key).catch(() => {});
    }
  },
  log: line => console.error(`slack: ${scrubKnown(line)}`),
  scrub: text => scrubKnown(text),
});
/** The tokens the agent last read, scrubbed from anything it logs. */
function scrubKnown(text: string): string {
  return scrub(text, known.bot, known.app);
}
const socket = new SocketMode(
  async () => {
    const env = await secrets();
    return { appToken: env.SLACK_APP_TOKEN, api: env.SLACK_API_URL };
  },
  ({ event, eventId }) => forwarder.admit(event, eventId),
  line => console.error(`slack: ${scrubKnown(line)}`),
);

/** Who the agent is in Slack, so its own posts are never forwarded back:
 *  learned for the workspace in use, again whenever the token changes or the
 *  last attempt failed (slack-review-S.5). */
async function ensureIdentity(): Promise<Identity | undefined> {
  try {
    const ctx = await eventCtx();
    if (identity && identityFor === ctx.key) return identity;
    const me = await web(ctx, "read", "auth.test");
    if (me.ok !== true || typeof me.user_id !== "string") return (identity = undefined);
    identity = { botUser: me.user_id, botId: me.bot_id, appId: me.app_id };
    identityFor = ctx.key;
    return identity;
  } catch {
    return (identity = undefined);
  }
}

await bus.register({ name, kind: "agent", descr: "slack: a workspace on the bus — post, read, subscriptions" }, { createOnly: false });
await ensureIdentity();
// The directories, loaded once at start and kept in memory: names resolve
// without a Slack call each time. A failure leaves them to the first use.
try {
  const ctx = await eventCtx();
  await Promise.allSettled([listUsers(ctx), listChannels(ctx)]);
} catch {
  /* loaded on first use */
}
forwarder.start();
void socket.start();
const stop = new AbortController();
const shutdown = () => stop.abort();
process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
await bus.serve(handle, { signal: stop.signal, drainTimeout: 5000 });
// Stopped: the socket and its timers first, then the queue drained, counters flushed.
socket.stop();
await forwarder.stop(5000);
stopFilters();
process.exit(0);
