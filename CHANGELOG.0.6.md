# Changelog 0.6

📌 **TL;DR:** Shipped changes on the 0.6 line, newest first. The 0.5 line is in [changelog 0.5](CHANGELOG.0.5.md#changelog-05).

## 0.6.15 — 2026-09-19

A pub/sub topic carries two lists instead of one doing both jobs: its allow
list is who may publish, and its new Deliver-To list is who receives a copy.
Whoever manages the channel writes that list, at registration or afterwards;
it takes users, agents and `@group`, expanded at the publish; a recipient may
take itself off and can no longer put itself on. Delivery no longer consults
the topic's ACL.

## 0.6.14 — 2026-09-19

The Overview node strip divides by time rather than by subject: Readers,
Queued and the four counts are how the node stands now, and Uptime and the
call windows follow on their own row as what has happened since it started. A
strip figure of none is a dash instead of a zero.

## 0.6.13 — 2026-09-19

Registration is a page per kind. The Channels section offers a queue form and a
pub/sub form instead of one with a kind control: a queue declares its TTL,
capacity and overflow at registration, and a topic declares none of it. The
service form carries an optional secret, stored by a second call to its own
verb, normalised to the bytes that were typed and never filled in again.

## 0.6.12 — 2026-09-19

The Overview node strip leads with what the node holds and breaks to a second
row for how it is running, so Uptime no longer stands in front of the counts.

## 0.6.11 — 2026-09-19

Record detail has one editor. Classification and Maintainers are fields of Edit
settings rather than a section of their own, and a caller who may not change
them sees them disabled rather than missing; the form states separately that it
carried them, so a Maintainer's save cannot clear what it was not offered.

## 0.6.10 — 2026-09-19

The dashboard has a tab icon, served as `/favicon.svg`; a browser asking for
`/favicon.ico` is told there is none rather than handed a page. The Overview
node strip counts Agents, Services, Channels and Users separately instead of
summing them into one cell, from per-kind totals the daemon now states. A row
naming the node's daemon owner carries `🔱` and the authority rather than
`👤 User`.

## 0.6.9 — 2026-09-19

The daemon's own words follow the [glossary](docs/glossary.md#terms): a
configuration is private to the **record** it belongs to, an allow list is a
**record's**, a personal **agent** names **agent** identities, delivery to a
name is **turned off** rather than a service being disabled, and the start
sweep reports the **records** it deleted.

## 0.6.8 — 2026-09-19

An external 📡 service holds a **secret**: the credential for reaching it,
opaque bytes the daemon never parses, read back by whoever the record's own
allow list already admits. `agent-bus secret <name>` reads it and
`agent-bus secret <name> -` sets it; every other answer carries `secret_sha`
in its place, and a registration carries neither half.

## 0.6.7 — 2026-09-19

**Breaking:** the 📮 and 📣 record is a **channel**, and `topic` is a label on
one message. One word was doing both jobs, in the CLI and on the wire:
`publish --topic <name>` meant a record while `send --topic t` meant a label.

| Was | Is |
|---|---|
| `agent-bus topic create` | `agent-bus channel create` |
| `publish --topic <name>` | `publish --channel <name>` |
| `/subscribe {"topic":...}` | `/subscribe {"channel":...}` |
| `/subscriber/remove {"topic":...}` | `{"channel":...}` |
| `reply_to.service` | `reply_to.name` |

`send`, `consume` and `reply` keep `--topic`, which is the label. `publish`
still stamps the channel's own name as that label, so a subscriber can pick out
copies from it with `consume --topic`; that is now documented rather than
implied. No old spelling is kept as an alias.

`reply_to.service` named a kind that by the 0.6.3 decision cannot be replied to
at all; the field is `name`.

Channels get their own page, `docs/07-channels.md`, out of records.

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
