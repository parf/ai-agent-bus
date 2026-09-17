// The MCP face. Six tools, each one call to the daemon and nothing else: no
// routing, no retry, no domain logic — see docs/10-modules.md. ab_rename is the
// one exception to "one call": an address change is register-then-unregister,
// because there is no rename on the daemon and a session must never be left
// with no address at all.
//
// Tool names are ab_*: a client exposes them as mcp__<server>__<tool> and a
// model's tool names may not contain a colon (docs/glossary.md).

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { CallToolRequestSchema, ListToolsRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { Bus, BusError, defaultName, type Envelope, type Record_ } from "./bus.ts";
import { startPush, type Push } from "./push.ts";
import { Codex } from "./codex.ts";
import { version } from "./version.ts";
import { describe, codexMessage } from "./messages.ts";
import { readFileSync } from "node:fs";

if (process.argv.length === 3 && ["--version", "-version"].includes(process.argv[2]!)) {
  console.log(version);
  process.exit(0);
}

// The launcher's private environment file keeps session credentials out of
// command lines and the user's persistent runtime configuration.
function loadSessionEnv(): void {
  if (!process.env.AGENT_BUS_SESSION_FILE) return;
  const env = JSON.parse(readFileSync(process.env.AGENT_BUS_SESSION_FILE, "utf8"));
  for (const key of ["AGENT_BUS_TOKEN", "AGENT_BUS_NAME", "AGENT_BUS_ADDR", "AGENT_BUS_DESCR", "AGENT_BUS_RUNTIME", "AGENT_BUS_PUSH", "AGENT_BUS_CONTROL_ADDR", "AGENT_BUS_CONTROL_TOKEN"]) {
    if (typeof env[key] === "string") process.env[key] = env[key];
  }
}
loadSessionEnv();

let bus = new Bus();

// Push mode is known before anything is served: ab_consume must not become
// the reader in the window where a push adapter is still starting.
const mode = (process.env.AGENT_BUS_PUSH ?? "off").trim().toLowerCase();
const pushing = mode === "claude" || mode === "codex";
const log = (s: string) => process.stderr.write(`agent-bus: ${s}\n`);

// How to answer what this process consumed. The daemon keeps no reply state
// (docs/04-messaging.md#reply-routing), so the face keeps the routing — and
// only the routing: bodies are unbounded, and a reply needs none of one.
type ReplyTo = { from: string; topic?: string; tag?: string };
const consumed = new Map<string, ReplyTo>();
const REMEMBER = 200;
// Where an answer to this message belongs: the sender, unless the request
// named a third party (docs/04-messaging.md#reply-routing).
function remember(e: Envelope) {
  const r = e.reply_to;
  consumed.set(e.message_id, r
    ? { from: r.service, topic: r.topic, tag: r.tag }
    : { from: e.from, topic: e.topic, tag: e.tag });
  if (consumed.size > REMEMBER) consumed.delete(consumed.keys().next().value!);
}

// Both ab_reply and ab_receipt need the envelope this session consumed, and
// both fail the same way without it.
const noContext = (id: string) =>
  `no reply context for ${id}: this session did not consume it, or it aged out of the last ${REMEMBER}. ` +
  `Use ab_send with the original sender, topic and tag — a reply is matched on all three.`;

const tools = [
  {
    name: "ab_ls",
    description:
      "List what is registered on the agent bus: agents, services and topics, with their descriptions. Use it to find who or what to talk to.",
    inputSchema: {
      type: "object",
      properties: { kind: { type: "string", description: "only this kind: agent, generic or topic" } },
    },
  },
  {
    name: "ab_send",
    description:
      "Send a message to a registered participant by name (user@realm). This means the bus accepted it, not that the peer read it — only a reply proves that. " +
      "Pass a topic and a tag you have not used before if you intend to wait for the answer: a reply is matched on both.",
    inputSchema: {
      type: "object",
      properties: {
        to: { type: "string", description: "receiver, as name@realm" },
        text: { type: "string", description: "the message body" },
        topic: { type: "string", description: "conversation id, e.g. one exchange" },
        tag: { type: "string", description: "your label for this message; a reply carries it back" },
      },
      required: ["to", "text"],
    },
  },
  {
    name: "ab_consume",
    description:
      "Take the next message from my own inbox, waiting up to a few seconds. Taking it removes it from the inbox: nobody else will see it, and there is no second chance to read it. " +
      "Finding nothing is a normal result, not a failure. With a topic and tag, wait for that one message instead — that is how you collect a reply to something you sent.",
    inputSchema: {
      type: "object",
      properties: {
        topic: { type: "string" },
        tag: { type: "string" },
        wait: { type: "string", description: "how long to wait, e.g. 5s (default 5s, max 60s)" },
      },
    },
  },
  {
    name: "ab_reply",
    description:
      "Answer a message this session consumed, by its id. Writing the reply in your own output leaves it in this session and the asker never sees it. " +
      "A pushed message says which tool answers it: a Codex push arrives from a sidecar whose reply context this process does not hold, and spells out an ab_send instead. " +
      "Routing comes from the original — sender, topic and tag — so the asker can match it.",
    inputSchema: {
      type: "object",
      properties: {
        message_id: { type: "string" },
        text: { type: "string" },
      },
      required: ["message_id", "text"],
    },
  },
  {
    name: "ab_receipt",
    description:
      "Tell the sender of a message this session consumed what became of it: 'ack' that you have it, 'done' that you finished. " +
      "Neither is an answer — send the answer with ab_reply. Use 'done' when the work produced no answer to send, so the asker stops waiting instead of timing out.",
    inputSchema: {
      type: "object",
      properties: {
        message_id: { type: "string" },
        kind: { type: "string", enum: ["ack", "done"], description: "ack = got it, done = finished it" },
      },
      required: ["message_id", "kind"],
    },
  },
  {
    name: "ab_rename",
    description:
      "Change the address this session is registered under, so peers find it by what it is working on rather than by when it started. " +
      "With an ab-* launcher, a supplied name renames the runtime session and bus address together; with no argument it reads the current session title. " +
      "The old address is released only when nothing is queued or waiting there; a busy one is kept and reported, so no message is lost.",
    inputSchema: {
      type: "object",
      properties: {
        name: { type: "string", description: "the title to register under; omit to use the session's own title" },
      },
    },
  },
] as const;

const server = new Server(
  { name: "agent-bus", version },
  {
    capabilities: {
      tools: {},
      // Claude Code only accepts notifications/claude/channel from a server
      // that declared it here — without this the session refuses the
      // connection outright, not just the notification.
      ...(mode === "claude" ? { experimental: { "claude/channel": {} } } : {}),
    },
    instructions:
      `You are ${bus.name} on the agent bus. ab_ls finds other participants, ab_send messages one, ` +
      `ab_consume takes the next message from your inbox, ab_reply answers one you received. ` +
      `Your own output never reaches the peer: answer with the tool the message says to use — ab_reply for one you took with ab_consume, and a pushed delivery spells out its own call. ` +
      `A reply is matched by topic and tag. The face registers this name at start and stops being reachable if the daemon restarts; restart it with the daemon.`,
  },
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({ tools }));

server.setRequestHandler(CallToolRequestSchema, async (req, extra) => {
  const args = (req.params.arguments ?? {}) as Record<string, unknown>;
  try {
    await refreshSession();
    switch (req.params.name) {
      case "ab_ls": {
        const records = await bus.ls(maybe(args, "kind"));
        return text(
          records.length === 0
            ? "nothing is registered"
            : records.map(catalogue).join("\n"),
        );
      }
      case "ab_send": {
        const topic = maybe(args, "topic"), tag = maybe(args, "tag");
        const e = await bus.send({ to: need(args, "to"), body: need(args, "text"), topic, tag });
        // The topic and tag come back because they are what a reply is
        // matched on (docs/04-messaging.md#request-and-reply).
        const how = [topic && `topic ${topic}`, tag && `tag ${tag}`].filter(Boolean).join(", ");
        return text(`the bus accepted ${e.message_id} for ${e.to}${how ? ` (${how})` : ""}`);
      }
      case "ab_consume": {
        // Push holds this inbox's one unfiltered read, so an unfiltered
        // consume would only collide with it. A filtered one is still served,
        // ahead of the loop (docs/04-messaging.md#one-reader-per-inbox).
        // While a push adapter is actually reading, yes. Once it has given
        // up — a 409 from a second reader, or a delivery the runtime
        // refused — saying "messages arrive on their own" is false, and it
        // would leave the inbox unread until a restart. `push` is still
        // undefined during the startup window the comment above guards.
        if (pushing && (push?.running() ?? true) && !args.topic && !args.tag) {
          return text(`push is on for ${bus.name}: messages arrive on their own. Pass topic and tag to wait for one reply.`, true);
        }
        const topic = maybe(args, "topic"), tag = maybe(args, "tag");
        const wait = maybe(args, "wait") ?? "5s";
        // A receipt is a message, so it ends a wait — but it is not the
        // answer the caller asked for. A filtered read asks again with what
        // is left of its deadline, exactly as `call` does
        // (docs/04-messaging.md#receipts).
        const deadline = Date.now() + seconds(wait) * 1000;
        for (;;) {
          // What is LEFT of the deadline, not the whole of it again: asking
          // for the full wait after a receipt made the worst case twice what
          // the caller asked for. Sent in ms so the daemon's ParseDuration
          // reads exactly what this computed, whatever the caller spelled.
          const left = deadline - Date.now();
          if (left <= 0) return text("nothing waiting");
          const e = await bus.consume({ topic, tag, wait: `${left}ms` }, extra.signal);
          if (!e) return text("nothing waiting");
          remember(e);
          // `done` says no answer is coming, so a filtered wait ends on it
          // rather than spending the rest of the deadline
          // (docs/04-messaging.md#receipts).
          if (!e.receipt || e.receipt === "done" || !(topic || tag)) return text(describe(e));
        }
      }
      case "ab_reply": {
        const id = need(args, "message_id"), body = need(args, "text");
        const original = consumed.get(id);
        if (!original) return text(noContext(id), true);
        const e = await bus.send({
          to: original.from,
          body,
          topic: original.topic,
          tag: original.tag,
        });
        return text(`replied to ${original.from} as ${e.message_id}`);
      }
      case "ab_receipt": {
        // The closed set has one home, and it is the daemon
        // (docs/04-messaging.md#receipts). Checking it again here would be a
        // second place to edit, and the refusal it sends back is the better
        // answer anyway — what matters is that the face does not swallow it.
        const id = need(args, "message_id"), kind = need(args, "kind");
        const original = consumed.get(id);
        if (!original) return text(noContext(id), true);
        await bus.send({
          to: original.from,
          body: "",
          topic: original.topic,
          tag: original.tag,
          receipt: kind,
          re: id,
        });
        return text(`told ${original.from} ${kind} for ${id}`);
      }
      case "ab_rename": {
        if (process.env.AGENT_BUS_SESSION_FILE) {
          const address = process.env.AGENT_BUS_CONTROL_ADDR;
          const token = process.env.AGENT_BUS_CONTROL_TOKEN;
          if (!address || !token) return text("Restart the ab-* launcher to enable coordinated renaming; this older launcher cannot move its inbox reader. No address was changed.", true);
          push?.stop();
          await push?.done;
          push = undefined;
          try {
            const response = await fetch(address + "/rename", {
              method: "POST", headers: { authorization: `Bearer ${token}`, "content-type": "application/json" },
              body: JSON.stringify({ name: maybe(args, "name") }), signal: AbortSignal.timeout(60_000),
            });
            return text(await response.text(), !response.ok);
          } finally { await refreshSession(); readInbox(); }
        }
        const title = maybe(args, "name") ?? sessionTitle();
        // Nothing to rename from: the session has no title of its own and the
        // caller named none. Say how to get one rather than inventing it.
        if (!title) return text(renameHelp(), true);
        const next = derive(title);
        if (next === bus.name) return text(`already registered as ${bus.name}. ${renameHelp()}`, true);

        const previous = bus;
        // A reader of its own inbox is a waiter, and the daemon refuses to
        // unregister an address somebody is waiting on. Stop reading first.
        push?.stop();
        push = undefined;
        let moved: Bus;
        try {
          // New address first: a failure here leaves the session exactly where
          // it was, which is the only safe direction to fail in.
          await previous.register({ name: next, kind: "agent", descr: title }, true);
          moved = await previous.as(next);
        } catch (err) {
          readInbox();
          throw err;
        }
        bus = moved;
        readInbox();

        // Releasing the old address is the part that is allowed to fail: it is
        // busy exactly when dropping it would lose something.
        let old = `released ${previous.name}`;
        try {
          await previous.unregister(previous.name);
        } catch (err) {
          old = err instanceof BusError
            ? `kept ${previous.name}: ${err.message}`
            : `kept ${previous.name}: ${err}`;
        }
        return text(`registered as ${bus.name} (${title}); ${old}. Peers that knew the old address must look it up again with ab_ls.`);
      }
      default:
        return text(`unknown tool ${req.params.name}`, true);
    }
  } catch (err) {
    if (err instanceof BadArgs) return text(String(err.message), true);
    const msg = err instanceof BusError ? `bus said ${err.status}: ${err.message}` : String(err);
    return text(msg, true);
  }
});

// The low-level Server does not enforce the schema it advertises, so a
// missing required argument would otherwise become an empty body on the wire.
function need(args: Record<string, unknown>, key: string): string {
  const v = args[key];
  if (typeof v !== "string" || v.trim() === "") throw new BadArgs(`${key} is required and must be a non-empty string`);
  return v;
}
function maybe(args: Record<string, unknown>, key: string): string | undefined {
  const v = args[key];
  if (v === undefined || v === null || v === "") return undefined;
  if (typeof v !== "string") throw new BadArgs(`${key} must be a string`);
  return v;
}
class BadArgs extends Error {}

// The runtime's own name for this session. The launcher refreshes the session
// file whenever the title changes (docs/08-runner-role.md#session-names), so
// re-reading it here is how the face learns a rename it was never told about.
// Without a launcher there is no file, and there is nothing to rename from.
function sessionTitle(): string | undefined {
  const file = process.env.AGENT_BUS_SESSION_FILE;
  if (!file) return undefined;
  let descr: unknown;
  try {
    descr = JSON.parse(readFileSync(file, "utf8")).AGENT_BUS_DESCR;
  } catch {
    return undefined; // the launcher owns this file; a partial write is its business, not an error here
  }
  if (typeof descr !== "string" || descr.trim() === "") return undefined;
  // The launcher appends " #2" to keep labels distinct; the title is what the
  // person typed, and the same disambiguation happens again on the new name.
  const title = descr.replace(/ #[2-9][0-9]*$/, "").trim();
  // Its fallback label when a session has no title of its own is
  // `runtime(dir)` — a placeholder to rename away from, not a title.
  const runtime = process.env.AGENT_BUS_RUNTIME ?? "agent";
  if (title === "" || title.startsWith(`${runtime}(`)) return undefined;
  return title;
}

// The same derivation the launcher uses for a session address
// (docs/01-identity-and-roles.md#names), so a rename here and a restart there agree on
// the name. The separator is whatever this session already registered with.
function derive(title: string): string {
  return defaultName(
    { ...process.env, AGENT_BUS_RUNTIME: process.env.AGENT_BUS_RUNTIME ?? "agent", AGENT_BUS_CWD: title },
    bus.name.includes("/") ? "/" : ".",
  );
}

function renameHelp(): string {
  return "Rename the session itself first — /rename in the session, or whatever its runtime calls it — then call ab_rename again with no argument. " +
    "Or call ab_rename with a name to register under that instead.";
}

function text(body: string, isError = false) {
  return { content: [{ type: "text" as const, text: body }], ...(isError ? { isError } : {}) };
}

// What a caller needs to decide whether to call it: what it is, how to reach
// it, and whether anyone is there. A record with no protocol is an ordinary
// bus service — send to the name; anything else the caller speaks itself, at
// the address given (docs/05-discovery.md#what-a-listing-answers).
function catalogue(r: Record_): string {
  const notes = [
    r.protocol ? `speaks ${r.protocol}${r.addr ? ` at ${r.addr}` : ""} — call it yourself, not through the bus` : undefined,
    // Whether anyone reads its inbox says nothing about a record the bus
    // does not serve, so it is left out rather than reported as absent.
    //
    // `reading` counts a read that accepts ANY message: a read restricted to
    // a topic or tag is attached and is not in it (core/bus.go withLiveness),
    // and `deliver` serves a matching one of those ahead of an unfiltered
    // reader. So the false case cannot say nobody is reading — it did, and the
    // dashboard said the same thing until F.13.1. Q70's accepted all-reader
    // web count is pending (docs/05-discovery.md#readers); this remains the
    // current MCP rendering of the existing bit.
    r.protocol ? undefined : r.reading ? "a reader is attached" : "no unfiltered reader — a read restricted to a topic or tag is not counted, and does take what matches it",
    r.queued ? `${r.queued} queued` : undefined,
    r.config_sha ? `configured (${r.config_sha.slice(0, 12)})` : undefined,
  ].filter(Boolean);
  return `${r.name}  [${r.kind}]  ${r.descr ?? ""}`.trimEnd() + `\n    ${notes.join(" · ")}`;
}

// "30s" / "500ms" / "2m" as seconds. A bare number is NOT a duration to Go's
// ParseDuration, and the daemon silently falls back to its own default when
// it cannot parse one — so anything this cannot read is left to the daemon
// to decide rather than guessed at differently here.
const MAX_WAIT = 60; // the daemon's cap; asking for more is not an error there
function seconds(s: string): number {
  const m = /^(\d+(?:\.\d+)?)(ms|s|m)$/.exec(s.trim());
  if (!m) return MAX_WAIT;
  const n = Number(m[1]);
  return Math.min(m[2] === "ms" ? n / 1000 : m[2] === "m" ? n * 60 : n, MAX_WAIT);
}

// Push: the session receives instead of polling. Two modes, one loop
// (push.ts); off is the default and everything above still works. Declared
// before the server serves anything, because ab_rename restarts the reader on
// the new inbox and a request may arrive as soon as the transport is up.
let push: Push | undefined;
const alsoStop: (() => void)[] = [];
const stopWith = (f: () => void) => alsoStop.push(f);
// What to do with a delivered message, once per mode. Reading is started again
// after a rename, against the new address; how to deliver never changes.
let deliver: ((e: Envelope) => Promise<void>) | undefined;
function readInbox(): void {
  if (deliver && !push?.running()) push = startPush(bus, deliver, log);
}

// Every tool uses the launcher's current credential, including MCP clients
// that were started before another client renamed the session.
async function refreshSession(): Promise<void> {
  if (!process.env.AGENT_BUS_SESSION_FILE) return;
  loadSessionEnv();
  if (process.env.AGENT_BUS_NAME === bus.name) return;
  const next = new Bus();
  push?.stop();
  await push?.done;
  push = undefined;
  bus = next;
  readInbox();
}

// Registering on start is what makes "find each other by name" possible: the
// name is this session's address for as long as it runs.
await bus.register({
  name: bus.name,
  kind: "agent",
  descr: process.env.AGENT_BUS_DESCR ?? `${process.env.AGENT_BUS_RUNTIME ?? "agent"} session`,
});

await server.connect(new StdioServerTransport());

// When the client goes away the face has no reader to deliver to. A poll
// left running would keep taking messages nobody will see.
function shutDown(why: string): void {
  if (!push?.running() && alsoStop.length === 0) return;
  log(`stopping: ${why}`);
  push?.stop();
  for (const f of alsoStop.splice(0)) {
    try { f(); } catch { /* going away anyway */ }
  }
}
server.onclose = () => shutDown("the client closed the connection");
// The SDK's stdio transport does not translate EOF into onclose. A killed
// runtime closes this pipe; its inbox reader must not outlive that runtime.
process.stdin.on("end", () => { shutDown("the client exited"); process.exit(0); });
for (const sig of ["SIGINT", "SIGTERM"] as const) process.on(sig, () => { shutDown(sig); process.exit(0); });
process.on("exit", () => shutDown("exit"));

if (mode === "claude") {
  // The Claude Code channel contract: one notification, content plus string
  // metadata. The session needs --dangerously-load-development-channels.
  deliver = async (e) => {
    remember(e);
    await server.notification({
      method: "notifications/claude/channel",
      params: {
        content: `Message from agent-bus peer ${e.from}:\n\n${e.body}`,
        meta: {
          message_id: e.message_id,
          from: e.from,
          ...(e.topic ? { topic: e.topic } : {}),
          ...(e.tag ? { tag: e.tag } : {}),
          at: e.at,
        },
      },
    });
  };
  readInbox();
} else if (mode === "codex") {
  const codex = new Codex(process.env.AGENT_BUS_CWD ?? process.cwd(), log);
  await codex.start();
  if (!codex.shared) {
    log("codex: AGENT_BUS_CODEX_WS is not set, so this drives its own app-server — it will not reach a session someone is typing in");
  }
  deliver = async (e) => {
    remember(e);
    const how = await codex.deliver(codexMessage(e), e.message_id);
    log(`delivered ${e.message_id} by ${how}`);
  };
  readInbox();
  stopWith(() => codex.stop());
} else if (mode !== "off" && mode !== "") {
  log(`AGENT_BUS_PUSH=${mode} is not a mode; use claude, codex or off`);
}
if (push) log(`push mode ${mode} is reading ${bus.name}`);
