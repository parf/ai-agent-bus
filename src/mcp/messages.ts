import type { Envelope } from "./bus.ts";

export function describe(e: Envelope): string {
  const head = [`from ${e.from}`, e.topic && `topic ${e.topic}`, e.tag && `tag ${e.tag}`, `id ${e.message_id}`]
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
  return `${head}\n\n${e.body}`;
}

// The launcher and standalone pusher share one rendering and reply route.
//
// A sidecar holds the inbox read, so the session it pushes into never
// consumed the message and has no reply context of its own: the route has to
// be spelled out, or the answer stays in the session
// (docs/08-runner-role.md#adapters).
export function sidecarMessage(e: Envelope): string {
  if (e.receipt) return describe(e);
  const route = e.reply_to
    ? { to: e.reply_to.name, topic: e.reply_to.topic, tag: e.reply_to.tag }
    : { to: e.from, topic: e.topic, tag: e.tag };
  return `${describe(e)}\n\nReply using ab_send with ${JSON.stringify(route)} and text containing your answer.`;
}

// The name this was born with, kept for the Codex pusher's call sites.
export const codexMessage = sidecarMessage;
