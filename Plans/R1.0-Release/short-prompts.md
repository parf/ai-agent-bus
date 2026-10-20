# Shorter AI-to-AI prompts

📌 **TL;DR:** P3 receipt rules are built; the remaining steps are a plan for discussion. Every pushed message now repeats
the protocol (how to ack, reply and finish) and peers write long prose, so
most tokens re-say what the session already knows. The idea, from the owner:
teach the protocol once, then send only the data a message adds. Open choices
are [№Q183–№Q184](QUESTIONS.md#short-prompts).

## Goal

Owner, 2026-10-19: "Review prompts AI's send to each other, we can make them
MUCH shorter; 1st - ask to read proto, then just send minimal data".

A session reads the protocol once, at start. After that a message carries
only what is new: who sent it, its route, and its content.

## Where the tokens go today

| Source | Cost | Repeats |
|---|---|---|
| [MCP instructions](../../src/mcp/instructions.yaml) | about 1.1 KB | once per session |
| [Pushed message](../../src/mcp/messages.ts) footer: "Confirm receipt using ab_receipt with {…} … reply using ab_send with {…} … confirm done with {…}" | about 400 B, the route written three times | every message |
| Receipt notices: "receipt: ack — … Not an answer; the answer is still to come." | about 150 B | every receipt |
| Peer-written bodies: restated context, recaps, politeness | often 1–2 KB | every message |
| The ack round trip itself | one tool call and one message | every message |

<details>
<summary>Observed in today's coordination</summary>

Most messages between Claude B and the Codexes were status the receiver
already had: re-quoted task text, the full rules list repeated in each
assignment, and the three-route footer on each push.

</details>

## Proposal

| Step | What changes | Measure |
|---|---|---|
| P1 Protocol card | One short canonical text, about 10 lines: receipts, replies, routes, ask help, and the message style. The MCP instructions carry it, so it is read once. The current instructions are its starting point | its size, against today's 1.1 KB plus the footers |
| P2 Minimal push | A pushed message is its header line and body only. The footer goes, because the header already has from, topic, tag and id, and the card says how to use them. A `reply_to` route is shown only when it differs from the sender | bytes per pushed message, before and after |
| P3 Fewer receipts | [Receipt rules](#receipt-rules): no ack when the answer comes in this turn; no done after an answer or for a report; the runner acks only a slow script | receipts per exchange on the fixed scenario |
| P4 Message style | The card asks peers to send only what is new: IDs, commits, `file:line`, the decision or result. No recap of context the receiver has, and no restated rules; one link to the rule instead | body bytes on a fixed scenario |
| P5 Check | A fixed scenario (assign, ack, report, review) runs before and after. The face's tests assert the push has no footer and the card is in the instructions | a smoke check per face that fails when the footer comes back |

## Receipt rules

Settled by the owner on 2026-10-19 ([№RD103](DECISIONS.md#recorded-decisions)), which also settles №Q182: "do not send ack when can reply right away (no junk on a bus); do not send done when result sent (only when no results expected)".

Built; current [receipt rules](../../docs/04-messaging.md#receipts) own when to acknowledge or finish work.

## Not in scope

- The bus wire format and envelope fields do not change.
- Agents that are not AI sessions (Slack, Telegram, git) are unaffected.
- Receipt semantics stay as [receipts](../../docs/04-messaging.md#receipts) define them; only when they are sent changes.
