# MCP Resources

Status: proposed, not built. It follows only the latest MCP specification,
[2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/server/resources)
([constitution § external protocols](../../docs/constitution.md#external-protocols)).

## Resource records

**A Resource is a registry record kind that works like an Agent, with the extra
fields the MCP specification defines** (owner, 2026-09-30). MCP lets a server
publish context a model can read — a file, a schema, a report — each named by a
URI, and publish parameterized ones as URI templates. 📡 Service describes
something to *call*; a Resource is something to *read*.

| | |
|---|---|
| Kind | one kind, `resource`. A **template** flag makes it a Resource Template: 📄 Resource, 📑 Resource Template |
| Access | the common fields, as on every record: owner, name, `description`, `personal`, `maintainers`, `allow`, `status`, timestamps. The [ACL](../../docs/02-access.md#acl) decides who sees it and who may read it; everyone else gets [no such entity](../../docs/constitution.md#common-record-fields) |
| MCP fields | every field the spec defines, its required ones mandatory: `uri` (or `uriTemplate` for a template), `name`, and the optional `title`, `mimeType`, `size` (not on a template), `icons` and `annotations`. `description` is our existing description field |
| How a read is answered | like an Agent: a `resources/read` is a message to the Resource's inbox, and whatever reads that inbox — a runner, a session — answers with the contents. A read that needs more input uses the spec's multi round-trip requests over the same topic and tag |
| `https://` resources | need no reader. The spec has the client fetch them directly, so the record is only the card |
| The daemon | serves no content itself: no file, HTTP or git fetching in `agent-busd`, so the [process boundary](../../docs/11-processes.md#the-rule) and the [trust boundary](../../docs/02-access.md#trust-boundary) do not change |

## What the MCP face does

| MCP method | Answered from |
|---|---|
| `resources/list` | the Resource records the caller's ACL admits, without the template flag. The spec lets a list vary by the request's authorization, never by connection |
| `resources/templates/list` | the same, with the template flag |
| `resources/read` | the request/reply above |

Subscriptions (`subscriptions/listen`, `notifications/resources/updated`) are
not part of this. A change fan-out is what a 📣 PubSub record already is;
wiring one to the other is a separate ask.
