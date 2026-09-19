# Changelog 0.6

📌 **TL;DR:** Shipped changes on the 0.6 line, newest first. The 0.5 line is in [changelog 0.5](CHANGELOG.0.5.md#changelog-05).

## 0.6.4 — 2026-09-19

Pages stop saying things about records that are not true of them. Only a
service is reported as reached externally, and only a service's settings offer
an address and a protocol, so an agent can no longer be edited into reading as
external. A Back link and the redirect after a removal name the listing the
record is actually on, which for a Personal agent is Personal. Channels accepts
only a kind it lists, so an old `?kind=agent` no longer empties it, and its
count explanation names user queues, which it counts. The Personal editor asks
for agents, which is what the daemon accepts. The Services page says which of
the two things its Readers and queue numbers are about.

## 0.6.3 — 2026-09-18

A record states which of five kinds it is — `user`, `agent`, `queue`, `pubsub`
or `service` — and the separate delivery mode is gone, because a queue and a
pub/sub topic are kinds of their own. A service is the external case and needs
an address and a protocol; anything running behind a bus name is an agent. One
predicate now answers whether a record could have been registered, and
registration, a settings edit and a restore all ask it.

The dashboard has an Agents section of its own, first in the menu: `/agents`,
`/agent?name=` and `/agents/new`. Services lists external services only,
Channels lists queues and pub/sub topics, and Personal is an agent-only
classification. The service form asks for the address and protocol the daemon
requires.

## 0.6.2 — 2026-09-18

The daemon owner is marked `🔱` in the user directory, on a person's page and
on the signed-in account, and a record's Maintainers are marked `👮` on its
detail. No other role is marked.

## 0.6.1 — 2026-09-18

A pub/sub copy a disabled subscriber cannot take is counted as that
subscriber's drop rather than skipped in silence; a subscriber the topic
stopped allowing is still skipped without a count. An agent's record is
labelled `👾 Agent` again, replacing the `📥 Inbox` of 0.5.84 now that `agent`
is becoming a named kind; those records stay with the channels.

## 0.6.0 — 2026-09-18

Opens the 0.6 line for the record-kind enum: `kind` becomes a closed set of
user, agent, queue, pubsub and service, `mode` retires into it, and a service
is an external one that is not on the bus. No behaviour has changed yet.
