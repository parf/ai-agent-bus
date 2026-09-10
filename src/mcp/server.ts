// The MCP face. Four tools, each one call to the daemon and nothing else: no
// routing, no retry, no domain logic — see docs/10-modules.md.
//
// Tool names are ab_*: a client exposes them as mcp__<server>__<tool> and a
// model's tool names may not contain a colon (docs/glossary.md).

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { CallToolRequestSchema, ListToolsRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { Bus, BusError, type Envelope } from "./bus.ts";
import { startPush, type Push } from "./push.ts";
import { Codex } from "./codex.ts";

const bus = new Bus();

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
function remember(e: Envelope) {
  consumed.set(e.message_id, { from: e.from, topic: e.topic, tag: e.tag });
  if (consumed.size > REMEMBER) consumed.delete(consumed.keys().next().value!);
}

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
      "Answer a message this session consumed or was pushed, by its id. This is the only way to answer: writing the reply in your own output leaves it in this session and the asker never sees it. " +
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
] as const;

const server = new Server(
  { name: "agent-bus", version: "0.1.0" },
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
      `A message you answer must be answered with ab_reply — your own output never reaches the peer. ` +
      `A reply is matched by topic and tag. The face registers this name at start and stops being reachable if the daemon restarts; restart it with the daemon.`,
  },
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({ tools }));

server.setRequestHandler(CallToolRequestSchema, async (req, extra) => {
  const args = (req.params.arguments ?? {}) as Record<string, unknown>;
  try {
    switch (req.params.name) {
      case "ab_ls": {
        const records = await bus.ls(maybe(args, "kind"));
        return text(
          records.length === 0
            ? "nothing is registered"
            : records.map((r) => `${r.name}  [${r.kind}]  ${r.descr ?? ""}`.trimEnd()).join("\n"),
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
        if (pushing && !args.topic && !args.tag) {
          return text(`push is on for ${bus.name}: messages arrive on their own. Pass topic and tag to wait for one reply.`, true);
        }
        const e = await bus.consume(
          { topic: maybe(args, "topic"), tag: maybe(args, "tag"), wait: maybe(args, "wait") ?? "5s" },
          extra.signal,
        );
        if (!e) return text("nothing waiting");
        remember(e);
        return text(describe(e));
      }
      case "ab_reply": {
        const id = need(args, "message_id"), body = need(args, "text");
        const original = consumed.get(id);
        if (!original) {
          return text(
            `no reply context for ${id}: this session did not consume it, or it aged out of the last ${REMEMBER}. ` +
              `Use ab_send with the original sender, topic and tag — a reply is matched on all three.`,
            true,
          );
        }
        const e = await bus.send({
          to: original.from,
          body,
          topic: original.topic,
          tag: original.tag,
        });
        return text(`replied to ${original.from} as ${e.message_id}`);
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

function text(body: string, isError = false) {
  return { content: [{ type: "text" as const, text: body }], ...(isError ? { isError } : {}) };
}

function describe(e: Envelope): string {
  const head = [`from ${e.from}`, e.topic && `topic ${e.topic}`, e.tag && `tag ${e.tag}`, `id ${e.message_id}`]
    .filter(Boolean)
    .join(" · ");
  return `${head}\n\n${e.body}`;
}

// Registering on start is what makes "find each other by name" possible: the
// name is this session's address for as long as it runs.
await bus.register({
  name: bus.name,
  kind: "agent",
  descr: process.env.AGENT_BUS_DESCR ?? `${process.env.AGENT_BUS_RUNTIME ?? "agent"} session`,
});

await server.connect(new StdioServerTransport());

// Push: the session receives instead of polling. Two modes, one loop
// (push.ts); off is the default and everything above still works.
let push: Push | undefined;
const alsoStop: (() => void)[] = [];
const stopWith = (f: () => void) => alsoStop.push(f);

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
for (const sig of ["SIGINT", "SIGTERM"] as const) process.on(sig, () => { shutDown(sig); process.exit(0); });
process.on("exit", () => shutDown("exit"));


if (mode === "claude") {
  // The Claude Code channel contract: one notification, content plus string
  // metadata. The session needs --dangerously-load-development-channels.
  push = startPush(bus, async (e) => {
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
  }, log);
} else if (mode === "codex") {
  const codex = new Codex(process.env.AGENT_BUS_CWD ?? process.cwd(), log);
  await codex.start();
  if (!codex.shared) {
    log("codex: AGENT_BUS_CODEX_WS is not set, so this drives its own app-server — it will not reach a session someone is typing in");
  }
  push = startPush(bus, async (e) => {
    remember(e);
    const how = await codex.deliver(`${describe(e)}\n\nReply with the ab_reply tool.`, e.message_id);
    log(`delivered ${e.message_id} by ${how}`);
  }, log);
  stopWith(() => codex.stop());
} else if (mode !== "off" && mode !== "") {
  log(`AGENT_BUS_PUSH=${mode} is not a mode; use claude, codex or off`);
}
if (push) log(`push mode ${mode} is reading ${bus.name}`);
