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

| | |
|---|---|
| What it is | one registered card for one MCP Resource, and one for one Resource Template |
| Fields | the common ones unchanged: owner, name, description, `personal`, `maintainers`, `allow`, `status`, timestamps |
| ACL | the same `allow` list, the same typed actor terms, the same [authority rules](../../docs/constitution.md#authority-rules). Whoever the list admits sees the card; everyone else gets no such entity |
| Not a channel | as with [📡 Service](../../docs/06-services.md#it-has-no-queue-here): nothing is queued, delivered or consumed here, so `ttl`, `bound`, `overflow` and `deliver_to` are refused rather than ignored |
| Inactive | the [no such entity](../../docs/constitution.md#common-record-fields) rule, unchanged |

**Same fields and same ACL is the whole of the ask.** Registration, ownership,
Maintainers, Personal, status, the audit entry on every edit and the web forms
are the ones every kind already has. What is new is one closed-set value —
today the [record kind](../../docs/constitution.md#-record-kind) enum is six —
and whatever the record must carry to name the thing it points at.

## What is not decided

The spec ([2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/server/resources))
identifies a Resource by an RFC 3986 URI and a Resource Template by an RFC 6570
URI template, and gives both an optional MIME type, size, icons and annotations.
Which of that a record stores, whether the two are one kind or two, and whether
this node ever answers `resources/read` or only hands over the URI, are the
owner's to settle: [Q110, Q111 and Q112](QUESTIONS.md#open-questions).

The read question is the one that decides the rest. Publishing the card keeps
the daemon what it is — a registry, with the
[trust boundary](../../docs/02-access.md#trust-boundary) where it is now.
Answering reads makes it fetch and return content on a caller's behalf, which is
a different program.

Subscriptions are not part of this. The spec's `notifications/resources/updated`
is a fan-out of a change, which is what a 📣 PubSub record already is; wiring one
to the other is a separate ask, not a property of the record.
