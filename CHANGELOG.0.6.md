# Changelog 0.6

📌 **TL;DR:** Shipped changes on the 0.6 line, newest first. The 0.5 line is in [changelog 0.5](CHANGELOG.0.5.md#changelog-05).

## 0.6.6 — 2026-09-19

The external service gets a page of its own. `docs/03-records.md` keeps the
five kinds, agent templates, configuration and topics and says the minimum
about 📡; `docs/06-services.md` owns the external case — what the record is,
how to call it, what having no queue here refuses, and its secrets.

**Breaking:** the CLI verb `service-template` is now `agent-template`. What it
configures is an agent, so the old spelling named the wrong kind. The old verb
is refused as an unknown one rather than kept as an alias.

## 0.6.5 — 2026-09-19

An external service has no queue here. Nothing is sent to one, nothing
subscribes it and nothing consumes from it; it takes no TTL, capacity, overflow
policy or delivery switch, a snapshot may not restore a queue under its name,
and no face reports a reader count or backlog it never measured. A 📡 record is
an information card — address, protocol, description, configuration and its
secret — read by whoever its allow list admits. The Overview no longer offers
to find services holding work, and the listing legend spells the character its
cells hold.

The documents use the approved vocabulary throughout: service means the
external case, what runs behind a bus name is an agent, Personal tags an agent,
and `generic`, `topic` as a kind and delivery *mode* name nothing.

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
