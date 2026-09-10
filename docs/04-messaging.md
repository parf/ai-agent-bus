# Messaging

How principals on the bus talk. The bus delivers securely and says who sent it;
everything else is the receiver's business.

## Inbox queues

**Every registered agent has its own queue** — an implicit queue topic named
after it, in memory in `agent-busd`, bounded, with a TTL.

**The address outlives the process**: anyone may `send` to a registered agent
that is down; the message waits and is picked up when the agent comes back —
unless the TTL expires or the queue fills first. Choose both sensibly.

## Verbs

| Verb | Target | Lands in | Allowed if |
|---|---|---|---|
| **`send`** | a known receiver, `unique-name@host` | exactly that queue | you may talk to that principal |
| **`publish`** | a topic | every queue holding a matching `consume:<glob>` | you hold `publish:<glob>` |

`consume` reads your own queue in both cases. Delivery does not report who
received a publish, but the API answers "are there subscribers on this topic,
and who?" to anyone whose access allows the lookup.

## Message fields

Every message carries:

| Field | Meaning |
|---|---|
| **`message_id`** | unique within its channel, assigned by `agent-busd`; what dedup, "did you get it?" and the dashboard refer to |
| **`topic`** | the conversation id, e.g. one A→B exchange |
| **`tag`** | the sender's label for this message |

A asks B three questions with three tags; B's answers carry the same tags, so A
matches them.

## Receipts

Optional. A receiver may answer with **`ack`** (received) and later **`done`**
(processed). Both are ordinary messages to the sender's queue (or its
`reply-to`), carrying the original `message_id`, topic and tag. Neither is
required; a sender that wants them asks.

## Message TTL

Optional, and shorter than the topic's. When it expires undelivered the message
is dropped from the queue and counted, never handed to a consumer. A question
nobody should answer late sets one.

## Reply routing

To the sender's queue with the same topic + tag — unless the request sets
`reply-to: {service, topic, tag}`.

## Push and pull

**Consumers pull** by default (long-poll / stream); a consumer may register a
**push** address and `agent-busd` delivers to it. Agent sessions are pushed
through a per-runtime adapter — see
[runner § adapters](08-runner-role.md#adapters).

## Overflow

Declared per topic at creation, inboxes included:

| Mode | Full queue | For |
|---|---|---|
| **ring** (default) | drop the **oldest**, count it in stats | alerts, telemetry — newest matters most |
| **strict** | **reject the send/publish** with an error to the producer | jobs, commands — losing one silently is worse than failing loudly |

## Durability

Queues and stats live in memory and are **dumped to a Parquet file on graceful
shutdown or restart**, then reloaded on start. An optional **periodic dumper**
(about once a minute) bounds the loss from an untimely death to one interval;
without it a crash loses everything since start.

A *consumer* being down is fine — its queue holds messages until TTL or bound.

## Envelope

**Bodies are opaque to the bus.** A message body is encrypted for its receiver
([access § encrypted sessions](02-access.md#encrypted-sessions)); `agent-busd`
queues and forwards ciphertext. What it reads, counts and shows is the
envelope, and only the envelope:

> sender · receiver · on-behalf-of · `message_id` · topic · tag · size ·
> timestamps · receipts

Once a message is consumed it is gone — the bus keeps counts, not content.

- **Encoding**: JSON; msgpack as an optional negotiated binary form.
- **Who sent it, and the receiver decides the rest.** Every delivered message
  carries the **sender principal**, verified by the bus, and the
  **on-behalf-of** principal when there is one
  ([identity § delegation](01-identity.md#delegation)). That is all the bus
  adds: it does not classify messages as orders or content and enforces no
  policy on the receiver.
