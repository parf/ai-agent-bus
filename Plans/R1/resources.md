# MCP Resources

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Resource records

**A Resource and a Resource Template become registry record kinds, carrying the
same [common fields](../../docs/constitution.md#common-record-fields) and the
same [ACL](../../docs/02-access.md#acl) as every other kind.** MCP lets a server
publish context a model can read — a file, a schema, a report — each identified
by a URI, and publish parameterized ones as URI templates. A node that wants to
offer those has nowhere to register them today: 📡 Service describes something
to *call*, and a Resource is something to *read*.

**The bus is a connector, so a Resource is an Agent in disguise** (owner,
2026-09-23). Behind the name is a party that answers when asked, and reading a
Resource is a message the daemon switches — which is the thing it already does.
It is not a content store, and a record here is not a copy of what it names.

| | |
|---|---|
| What it is | one registered card for one MCP Resource, and one for one Resource Template |
| Fields | the common ones unchanged: owner, name, description, `personal`, `maintainers`, `allow`, `status`, timestamps |
| ACL | the same `allow` list, the same typed actor terms, the same [authority rules](../../docs/constitution.md#authority-rules). Whoever the list admits sees the card; everyone else gets no such entity |
| What answers | whatever stands behind the name, reached the way everything on this bus is reached; how a read reaches it without a queue here is [Q114](QUESTIONS.md#open-questions) |
| Not a channel | as with [📡 Service](../../docs/06-services.md#it-has-no-queue-here): nothing is queued here, so `ttl`, `bound`, `overflow` and `deliver_to` are refused rather than ignored |
| Inactive | the [no such entity](../../docs/constitution.md#common-record-fields) rule, unchanged |

Registration, ownership, Maintainers, Personal, status, the audit entry on every
edit and the web forms are the ones every kind already has. What is new is one
closed-set value — today the [record kind](../../docs/constitution.md#-record-kind)
enum is six — and whatever the record must carry to name the thing it points at.

## Basic protocols in the daemon

**The owner has put one thing up for discussion: `agent-busd` implementing a few
basic protocols itself**, so that a Resource whose content is a file, or one
plain fetch away, needs no agent standing behind it to serve it.

Which protocols, and whether any at all, is [Q113](QUESTIONS.md#open-questions).
It is worth deciding on its own because it is the one part that changes the
daemon rather than the registry: a daemon that goes and gets content on a
caller's behalf is doing something this one does not do today, and the
[process boundary](../../docs/11-processes.md#the-rule) and the
[trust boundary](../../docs/02-access.md#trust-boundary) are where that is
settled.

## What is not decided

The spec ([2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/server/resources))
identifies a Resource by an RFC 3986 URI and a Resource Template by an RFC 6570
URI template, and gives both an optional MIME type, size, icons and annotations.
Which of that a record stores, and whether the two are one kind or two, are
[Q110 and Q111](QUESTIONS.md#open-questions).

Subscriptions are not part of this. The spec's `notifications/resources/updated`
is a fan-out of a change, which is what a 📣 PubSub record already is; wiring one
to the other is a separate ask, not a property of the record.
