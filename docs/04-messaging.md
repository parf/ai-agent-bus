# Messaging

How principals on the bus talk. The bus delivers securely and says who sent it;
everything else is the receiver's business.

## Inbox queues

**Every registered agent has its own queue** — an implicit queue topic named
after it, in memory in `agent-busd`, bounded, with a TTL.

**The address outlives the process**: anyone may `send` to a registered agent
that is down — or one that has never run yet — and the message waits until
something with that name reads it, unless the TTL expires or the queue fills
first. Choose both sensibly. A queue belongs to a **name**, never to a
connection or a session, so nothing is lost because a reader died: that is
the whole difference from an ephemeral channel.

## Verbs

| Verb | Target | Lands in | Allowed if |
|---|---|---|---|
| **`send`** | a known receiver, `unique-name@host` | exactly that queue | you may talk to that principal |
| **`publish`** | a topic | **as the topic's kind says** — a queue topic to one consumer and kept until taken; a pub/sub topic to every current subscriber, kept for none ([services and topics § topics](03-services-and-topics.md#topics)) | you hold `publish:<glob>` |

**A message is addressed to a name, and a name that is registered nowhere is
refused at `send`** — there is no label to send to and nothing accepts on
behalf of an address that does not exist. Accepting something undeliverable
and reporting the truth later is the failure mode this rules out: what comes
back from `send` is the whole delivery story, and whether anyone *acted* is
the receiver's own `ack` or reply, never the transport's
([receipts](#receipts)).

`consume` reads your own queue in both cases. Delivery does not report who
received a publish, but the API answers "are there subscribers on this topic,
and who?" to anyone whose access allows the lookup.

## Request and reply

A service call is not a third verb. It is a **`send` whose reply comes back on
the same topic + tag**, and the caller waits for it:

| Side | Does |
|---|---|
| caller | **states its own record** if it has none — an answer needs an address to arrive at ([identity § registration](01-identity.md#registration)) — then `send` and wait on its own queue for a message with that topic + tag. A plain `send` does **not** require this: a sender's name is checked for shape, not for registration, so fire-and-forget works from anyone and an answer to an unregistered sender is refused as *no such name*. Wanting a reply is what makes the record necessary |
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

**Reading a topic is not filtering.** A queue topic is an inbox with a name
([services and topics § topics](03-services-and-topics.md#topics)), so
`consume` reads it as an inbox — one reader, like any other. The same option
names both, and the daemon decides once, for every face: a **registered
topic named on its own** is an inbox to read; anything else, or anything with
a tag beside it, filters the caller's own inbox. A reply always carries a
tag, which is what keeps the two apart. A **name-shaped topic that is
registered nowhere is refused** — `jobs@srv1` when the topic is `jobs@srv-1`
is a typo, and reading it as a filter would answer with a silent timeout.

The filter is how a wait coexists with a live session's reader — it is an
ordinary `consume`, so every client language gets it for free, and the daemon
hands it its match first. It is a **priority, not a lease**: it wins only
while the wait is outstanding, and a wait ends at one message. A receipt is a
message, so between the `ack` and the wait that follows it the unfiltered
reader can take the reply.

❓ **Should reading an inbox and filtering one be different options?**
`--topic` means both, told apart by a tag and by what is registered, so a
mistyped topic name depends on registry state to be caught at all. An
explicit selector would remove the ambiguity at the cost of one more option.
*Settled by:* the owner, with the MVP CLI.

So the guarantee is stated precisely: **filtered and unfiltered readers on
one inbox are allowed, and a synchronous `call` on such an inbox is not
guaranteed its own answer.** A caller that needs the answer — a CLI beside a
pushing session, say — uses **a name of its own**, which costs one record and
no machinery. Making the wait exclusive for the length of an exchange would
mean a reservation in the daemon, and the daemon keeps no exchange state
([reply routing](#reply-routing)).

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
matches them. **A tag is unique to its exchange** — that is what makes the
match sound, and why `call` generates one rather than reusing a label.

## Receipts

Optional, and **both emitted by the receiver** — the service, not the bus:

| Receipt | Means |
|---|---|
| **`ack`** | the service **got** the message |
| **`done`** | the service **finished processing** it |

Together they are what a sender actually wants to know — **`ack`: received.
`done`: the work finished** — and both can only come from the receiver,
because nothing else knows. That is why the bus keeps no delivery journal of
its own: an observation made anywhere but the receiver is about the
transport, and a correlated message from the receiver is both simpler and
worth more.

Read them for exactly what they say. `ack` means the message reached the
receiver, **not** that the work started: a script service acks before it runs
the script ([runner § script services](08-runner-role.md#script-services)).
And **no receipt means unknown** — the request, the work or the receipt may
be late or lost — never proof of loss. Receipts remove a layer of guessing,
not the uncertainty itself.

Both are ordinary messages to the sender's queue (or its `reply-to`), carrying
the original `message_id`, topic and tag. The field is a **closed set** — one
of those two words — because a caller tells an answer from a receipt by
reading it, and a third value would read as an answer. Neither is required; a sender that
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
