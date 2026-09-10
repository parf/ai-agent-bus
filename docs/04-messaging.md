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
| **`publish`** | a topic | **as the topic's kind says** — a queue topic to one consumer and kept until taken; a pub/sub topic to every current subscriber, kept for none ([services and topics § topics](03-services-and-topics.md#topics)) | you hold `publish:<glob>` |

`consume` reads your own queue in both cases. Delivery does not report who
received a publish, but the API answers "are there subscribers on this topic,
and who?" to anyone whose access allows the lookup.

## Request and reply

A service call is not a third verb. It is a **`send` whose reply comes back on
the same topic + tag**, and the caller waits for it:

| Side | Does |
|---|---|
| caller | `send`, then wait on its own queue for a message with that topic + tag |
| service | `consume` its inbox, optionally `ack` (got it), do the work, `reply` — and `done` if the sender asked for it |
| deadline | the caller's, and the message's TTL is what stops a late answer arriving |

The bus adds nothing here — matching is the topic + tag it already carries, the
wait is an ordinary `consume`, and a caller that does not want to wait simply
does not. A service may answer twice (`ack` now, result later) or hand the
answer to a third party with `reply-to`.

## One reader per inbox

**An inbox has exactly one reader.** A session runs a push adapter, an MCP
face and possibly a CLI, and all three would otherwise `consume` the same
queue and take each other's messages — a waiting `call` losing its reply to
the notifier, or the reverse.

| Rule | Why |
|---|---|
| **one outstanding unfiltered read** at a time — a second is **refused**, not queued behind the first | a silent second reader looks exactly like message loss |
| by convention, one *designated* process does that reading for a principal | the bus enforces the outstanding read, not process ownership; claiming otherwise would need a lease nobody wants in a PoC |
| a waiter passes a **topic + tag filter** to `consume`, and the daemon hands it a match ahead of the unfiltered reader | the match happens where the message already is: no dispatcher in a client, and no local protocol between a Go CLI and a TypeScript session process |

The filter is what keeps [request and reply](#request-and-reply) honest next to
a live session — the wait is still an ordinary `consume`, and every client
language gets it for free.

**`consume` is at-most-once**: the message is handed over and gone. A reader
that dies between taking a message and acting on it loses that message, and
that is the accepted cost — it keeps `ack` an optional business receipt rather
than a dequeue contract, and keeps the daemon from tracking in-flight state
for every reader.

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

Optional, and **both emitted by the receiver** — the service, not the bus:

| Receipt | Means |
|---|---|
| **`ack`** | the service **got** the message |
| **`done`** | the service **finished processing** it |

Both are ordinary messages to the sender's queue (or its `reply-to`), carrying
the original `message_id`, topic and tag. Neither is required; a sender that
wants them asks. A **reply carries the answer** and says nothing by itself
about either — though a service that replies has plainly finished, which is
why a request/reply exchange can skip `done`.

## Message TTL

Optional, and shorter than the topic's. When it expires undelivered the message
is dropped from the queue and counted, never handed to a consumer. A question
nobody should answer late sets one.

## Reply routing

To the sender's queue with the same topic + tag — unless the request sets
`reply-to: {service, topic, tag}`.

**The daemon keeps no reply state.** A reply is an ordinary `send` carrying
the routing fields, and `reply <message-id>` is sugar: the client that
consumed the message still has its envelope, so it fills in receiver, topic and
tag itself. The daemon stays simple — nothing to bound, expire or reconcile —
and the rule that a consumed message is gone stays true.

The consequence is worth stating: **you can only `reply` to something you
consumed in that process**. Since an inbox has exactly one reader, that is the
same process anyway.

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

Bodies in a reloaded queue are still encrypted, so the credential they were
encrypted under has to outlive the restart as well — which is why tokens are
persisted and the previous one is kept
([access § token lifetime](02-access.md#token-lifetime)).

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
