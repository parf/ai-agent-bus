import type { Envelope } from "./bus.ts";

export function describe(e: Envelope): string {
  const head = [`from ${e.from}`, e.roles?.length && `roles ${e.roles.join(", ")}`, e.topic && `topic ${e.topic}`, e.tag && `tag ${e.tag}`, `id ${e.message_id}`]
    .filter(Boolean)
    .join(" · ");
  // A receipt carries no body: saying so beats handing over a blank one,
  // which reads as an empty answer (docs/04-messaging.md#receipts).
  if (e.receipt) {
    const what = e.re ?? "your message";
    // The two say opposite things about what happens next, and telling the
    // model "the answer is still to come" after a `done` is a lie that costs
    // it a pointless wait (docs/04-messaging.md#receipts).
    return e.receipt === "done"
      ? `${head}\n\nreceipt: done — ${e.from} finished ${what} and sent no answer. Nothing further is coming; do not wait for it.`
      : `${head}\n\nreceipt: ack — ${e.from} received ${what}. Not an answer; the answer is still to come.`;
  }
  // An ask has no body: it asks this session about itself
  // (docs/constitution.md#agent-self-description).
  if (e.ask === "help") return `${head}\n\nask: help — ${e.from} asks what you can do. Answer it with a description for an AI reader: what you are for, what you can do for this sender, and how to ask you.`;
  if (e.ask === "mcp") return `${head}\n\nask: mcp — ${e.from} asks for your MCP tools. Answer with a JSON tools/list result, {"tools": [...]}, or {"tools": []} if you have none.`;
  return `${head}\n\n${e.body}`;
}

// The launcher and standalone pusher share one rendering and reply route.
//
// A sidecar holds the inbox read, so the session it pushes into never
// consumed the message and has no reply context of its own: the route has to
// be spelled out, or the answer stays in the session
// (docs/06-launchers.md#adapters).
export function sidecarMessage(e: Envelope): string {
  if (e.receipt) return describe(e);
  const route = e.reply_to
    ? { to: e.reply_to.name, topic: e.reply_to.topic, tag: e.reply_to.tag }
    : { to: e.from, topic: e.topic, tag: e.tag };
  return `${describe(e)}\n\nRoute ${JSON.stringify(route)}: ab_send for answers; ab_receipt with ${JSON.stringify({ message_id: e.message_id })} and this route only if needed (answer this turn: no ack; later turn: ack; never done after an answer; done only for a task expecting no answer; nothing for reports, notices or receipts).`;
}

// The name this was born with, kept for the Codex pusher's call sites.
export const codexMessage = sidecarMessage;
