# Messaging

## Status

| MVP | Scope |
|---|---|
| Built | Inbox delivery, shared readers, filtered waits, receipts, deadlines, TTL, subscriptions, overflow and JSON restart snapshots. |
| Pending | Whether inbox selection and message filtering should use separate options; [MVP questions](../Plans/MVP/QUESTIONS.md#open-questions). |


How principals on the bus talk. The bus delivers securely and says who sent it;
everything else is the receiver's business.

## Inbox queues

**Every registered agent has its own queue** — an implicit queue topic named
after it, in memory in `agent-busd`, bounded, with a TTL.

**An inbox belongs to a registered name.** Consuming as a name the registry
does not know is refused rather than answered with an empty wait: nothing
could ever arrive in it — `send` refuses an unknown receiver — and creating
one on demand would let any caller name leave an inbox behind for good.

**The address outlives the process**: anyone may `send` to a registered agent
that is down — or one that has never run yet — and the message waits until
something with that name reads it, unless the TTL expires or the queue fills
first. Choose both sensibly. A queue belongs to a **name**, never to a
connection or a session, so nothing is lost because a reader died: that is
the whole difference from an ephemeral channel.

## Verbs

| Verb | Target | Lands in | Allowed if |
|---|---|---|---|
| **`send`** | a known receiver, `service@realm` | exactly that queue | you may talk to that principal |
| **`publish`** | a topic | **as the topic's kind says** — a queue topic to one consumer and kept until taken; a pub/sub topic to every current subscriber, kept for none ([services § topics](03-services-and-topics.md#topics)) | you hold `publish:<glob>` |

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
| deadline | the caller's, and it **travels with the request** so the service can give up early — see below. The message's TTL is a different bound, and is what stops a late answer arriving |

The bus adds nothing here — matching is the topic + tag it already carries, the
wait is an ordinary `consume`, and a caller that does not want to wait simply
does not. A service may answer twice (`ack` now, result later) or hand the
answer to a third party with `reply-to`.

**The caller's deadline travels.** A request states how long its caller will
wait, and the bus turns that into a moment on the envelope the service
consumes. A service that sees the moment already past **does not do the
work**: nobody is waiting for it. Without the field the two parties time out
independently and the work is done for a caller who has gone.

It is a second pair beside the TTL, not a spelling of it, and the difference
is what each one belongs to:

| | Belongs to | Bounded by the queue | The bus acts on it |
|---|---|---|---|
| **TTL** ([message ttl](#message-ttl)) | the message | yes — the receiver owns its retention | yes: it stops delivering |
| **deadline** | the caller | no — it is not the receiver's to shorten | no: it only carries it |

So a message whose caller has gone is **still delivered**. Only the service
knows whether the work is worth doing for somebody else — a third party on
`reply-to`, a cache, an audit — and the bus does not guess. What the bus does
guarantee is that the moment is its own: the caller states a **duration** and
never an instant, so the service is not reading the caller's clock.

## One reader per inbox

**An inbox has exactly one reader.** A session runs a push adapter, an MCP
face and possibly a CLI, and all three would otherwise `consume` the same
queue and take each other's messages — a waiting `call` losing its reply to
the notifier, or the reverse.

| Rule | Why |
|---|---|
| **one outstanding unfiltered read** at a time — a second is **refused**, not queued behind the first, unless both asked to share ([several readers](#several-readers-may-wait-when-they-say-so)) | a silent second reader looks exactly like message loss |
| by convention, one *designated* process does that reading for a principal | the bus enforces the outstanding read, not process ownership; claiming otherwise would need a lease nobody wants in a PoC |
| a waiter passes a **topic + tag filter** to `consume`, and the daemon hands it a match ahead of the unfiltered reader | the match happens where the message already is: no dispatcher in a client, and no local protocol between a Go CLI and a TypeScript session process |

**Reading a topic is not filtering.** A queue topic is an inbox with a name
([services § topics](03-services-and-topics.md#topics)), so
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

### Several readers may wait when they say so

The rule above exists for an *accidental* second reader, and a worker pool is
not one — so **the pool says so**. A reader that asks to **share** the inbox
is one of several, and any number of those may block on one empty inbox at
once; the first message wakes one of them.

| | |
|---|---|
| how the daemon tells the two apart | it is asked. A pool passes one word; an accident cannot pass it by accident |
| a reader that does not ask | keeps the whole old guarantee, **in both directions**: it is refused beside a pool, and a pool member is refused beside it. Wanting the inbox to yourself is still something you get |
| what this does not change | competing consumers, which already worked — while there is a backlog, N readers take turns and no message goes to two of them ([services § topics](03-services-and-topics.md#topics)). What sharing adds is the **empty** inbox, which is a pool's steady state |
| what it deliberately is not | a lease, a group or a registration. Nothing is remembered between reads, so a worker that dies leaves nothing behind to clean up |

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

**`done` ends a caller's wait.** A service that replies has plainly finished,
so it sends no `done`; therefore a `done` that does arrive says *finished, and
no answer is coming*. A caller blocked on the answer stops there rather than
sitting out its deadline for something that will never be sent — and says
which of the two happened, because "finished with nothing to return" and "no
answer in time" are different outcomes. `ack` ends nothing: it says the message
arrived, not that the work stopped.

Both are ordinary messages to the sender's queue (or its `reply-to`), carrying
the original `message_id`, topic and tag. The field is a **closed set** — one
of those two words — because a caller tells an answer from a receipt by
reading it, and a third value would read as an answer. Neither is required; a sender that
wants them asks. A **reply carries the answer** and says nothing by itself
about either.

## Message TTL

Optional, and shorter than the topic's — asking for longer is not an error,
it simply is not kept that long, because the receiver owns its retention the
same way it owns its [overflow](#overflow). When it expires undelivered the
message is dropped from the queue and **counted apart from overflow**, never
handed to a consumer. A question nobody should answer late sets one.

Two counters, because they are two problems: a queue too small for its
traffic, and a message nobody wanted by the time it arrived. One number
cannot tell an operator which they have.

## Reply routing

To the sender's queue with the same topic + tag — unless the request sets
`reply-to: {service, topic, tag}`.

**A request that expects a reply needs a reply address that exists.** The
route is the requester's own name plus the exchange's topic and tag, or an
explicit `reply-to` naming another; either way the destination is a
**registered name**, checked when the request is accepted rather than
discovered when the answer bounces. Registered is not the same as *live* —
the name owns a queue and its reader may be offline, which is the point of
name-owned queues ([inbox queues](#inbox-queues)). A plain `send` or
`publish` stays open to unregistered senders: fire-and-forget asks for
nothing back.

**The daemon keeps no reply state.** A reply is an ordinary `send` carrying
the routing fields, and `reply <message-id>` is sugar: the client that
consumed the message still has its envelope, so it fills in receiver, topic and
tag itself. The daemon stays simple — nothing to bound, expire or reconcile —
and the rule that a consumed message is gone stays true.

The consequence: **you can only `reply` from a client that
holds the routing context of what was consumed** — the receiver, topic and
tag. Whether that is one process is up to the client: the MCP face keeps it in
memory, and the CLI writes it where its next invocation finds it, so `consume`
and `reply` work as two commands. Since an inbox has exactly one reader, the
context has one owner either way.

## Push and pull

**Consumers pull** through the consume API. The current daemon does not push
to a registered callback address. Agent sessions receive pushes from their
client-side adapters — see
[runner § adapters](08-runner-role.md#adapters).

### Subscribers

**A subscriber is a registered name, and a copy lands in its own inbox.** So
publishing to a pub/sub topic is one copy per subscriber, and the topic itself
keeps nothing — the copies are kept, by the inboxes they are in.

Subscribing is a **record on the topic**, not an outstanding read: it survives
a restart with the rest of the registry ([durability](#durability)), and a
subscriber that is down keeps its backlog exactly like any other name
([inbox queues](#inbox-queues)). That is the whole reason to put the copy in
an inbox rather than hand it to whoever is connected.

| | |
|---|---|
| who may subscribe | anyone the topic's ACL lets see it ([identity § acl](01-identity.md#acl)) — and you subscribe **yourself**, because it is your inbox the copies land in |
| and must be registered | the copy needs somewhere to go, and an inbox belongs to a registered name |
| asked again at **every publish** | access taken away stops the copies. Checking only at subscribe would make a subscription a way to go on reading a topic that stopped allowing you |
| whose bound, TTL and overflow apply | the **subscriber's**, because the copy is in the subscriber's inbox |
| a subscriber that will not read | loses its own copies and **stops nothing**: the publish still succeeds for everyone else, and the copy that would not fit is counted as a drop ([overflow](#overflow)). A publisher one stopped reader can block is a queue topic, which is the other mode and is what that caller wanted |

## Overflow

Declared per topic at creation, inboxes included:

| Mode | Full queue | For |
|---|---|---|
| **strict** (default) | **reject the send/publish** with an error to the producer | anything that is work: losing one silently is worse than failing loudly, so this is what you get unless you ask otherwise |
| **ring** | drop the **oldest**, and count it in stats | alerts, telemetry, progress — the newest matters most and a gap is not a bug |

Declared on the record, so it is a property of the **receiver**, not of the
sender or the message: whoever owns the queue decides what its fullness
means. The count of what a ring has dropped is in `status`, because a queue
that forgets silently looks exactly like one nobody sent to.

## Durability

Queues, registry records and counters are snapshotted together through the
`dump` port. The MVP adapter writes **JSON**, on startup, on graceful shutdown,
and optionally periodically. Delivery checks message expiry after reload.
A drained inbox remains drained across subsequent snapshots.

The snapshot records whether shutdown was clean. A start after an unclean stop
reports the potential gap since the last snapshot. Traffic after that snapshot
may be lost; a consumer being offline is different, because its queue remains
in the running daemon.

Credentials use their own [store](09-setup.md#storage). Uptime and browser
sessions describe the current process lifetime, not recovered state.

## Envelope

Bodies travel as plaintext strings in the current JSON envelope. The daemon
carries them without interpreting their application meaning. It stamps the
sender from the credential and rejects a caller's attempt to choose another.
There is no built delegation field or negotiated binary encoding.

The [protocol source](../src/internal/protocol/envelope.go) owns the implemented
fields. The dashboard receives a separate feed with bodies removed in the bus;
this does not encrypt queued bodies or snapshots.
