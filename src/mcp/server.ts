// The MCP face. Four tools, each one call to the daemon and nothing else: no
// routing, no retry, no domain logic — see docs/10-modules.md.
//
// Tool names are ab_*: a client exposes them as mcp__<server>__<tool> and a
// model's tool names may not contain a colon (docs/glossary.md).

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { CallToolRequestSchema, ListToolsRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { Bus, BusError, type Envelope } from "./bus.ts";

const bus = new Bus();

// What this process consumed, so ab_reply can answer by id. The daemon keeps
// no reply state (docs/04-messaging.md#reply-routing); a long-lived face keeps
// it in memory, which is smaller than a file and dies with the session.
const consumed = new Map<string, Envelope>();
const REMEMBER = 200;
function remember(e: Envelope) {
  consumed.set(e.message_id, e);
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
      "Send a message to a registered participant by name (user@realm). Returns the message id. Use topic and tag to correlate a reply.",
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
      "Read the next message from my own inbox, waiting up to a few seconds. With topic and tag, wait for that one message instead — that is how you collect a reply to something you sent.",
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
      "Answer a message this session consumed, by its id. Routing comes from the original — sender, topic and tag — so the asker can match it.",
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
    capabilities: { tools: {} },
    instructions:
      `You are ${bus.name} on the agent bus. ab_ls finds other participants, ab_send messages one, ` +
      `ab_consume reads your inbox, ab_reply answers something you consumed. A reply is matched by topic and tag.`,
  },
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({ tools }));

server.setRequestHandler(CallToolRequestSchema, async (req) => {
  const args = (req.params.arguments ?? {}) as Record<string, string>;
  try {
    switch (req.params.name) {
      case "ab_ls": {
        const records = await bus.ls(args.kind);
        return text(
          records.length === 0
            ? "nothing is registered"
            : records.map((r) => `${r.name}  [${r.kind}]  ${r.descr ?? ""}`.trimEnd()).join("\n"),
        );
      }
      case "ab_send": {
        const e = await bus.send({ to: args.to, body: args.text, topic: args.topic, tag: args.tag });
        return text(`sent ${e.message_id} to ${e.to}`);
      }
      case "ab_consume": {
        const e = await bus.consume({ topic: args.topic, tag: args.tag, wait: args.wait ?? "5s" });
        if (!e) return text("nothing waiting");
        remember(e);
        return text(describe(e));
      }
      case "ab_reply": {
        const original = consumed.get(args.message_id);
        if (!original) {
          return text(
            `message ${args.message_id} is not one this session consumed — use ab_send with the sender's name`,
            true,
          );
        }
        const e = await bus.send({
          to: original.from,
          body: args.text,
          topic: original.topic,
          tag: original.tag,
        });
        return text(`replied to ${original.from} as ${e.message_id}`);
      }
      default:
        return text(`unknown tool ${req.params.name}`, true);
    }
  } catch (err) {
    const msg = err instanceof BusError ? `bus said ${err.status}: ${err.message}` : String(err);
    return text(msg, true);
  }
});

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
