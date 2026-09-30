# MCP Resources

Status: proposed, not built. It follows only the latest MCP specification,
[2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/server/resources)
([constitution § external protocols](../../docs/constitution.md#external-protocols)).

## Resource records

**A Resource is information, not a service and not an Agent** (owner,
2026-09-30). It is a card, like a 📡 Service: it describes data some MCP server
has, and our MCP face passes it on one to one. The card carries no content.
Listing it is the promise that a read of its URI can be answered.

| | |
|---|---|
| Kind | one kind, `resource`. A **template** flag makes it a Resource Template: 📄 Resource, 📑 Resource Template |
| Access | the common fields, as on every record: owner, name, `description`, `personal`, `maintainers`, `allow`, `status`, timestamps. The [ACL](../../docs/02-access.md#acl) decides who sees it and who may read it |
| MCP fields | every descriptor field the spec defines, its required ones mandatory: `uri` (or `uriTemplate` for a template), `name`, and the optional `title`, `mimeType`, `size` (not on a template), `icons` and `annotations`. `description` is our existing description field |
| Owning server | the 📡 Service, protocol `mcp`, whose server holds the data and resolves the URI at read time. An `https://` card may name none |
| No queue | like a Service, nothing is queued here, so `ttl`, `bound`, `overflow` and `deliver_to` are refused rather than ignored |

## What the MCP face does

| MCP method | Answer |
|---|---|
| `resources/list` | the non-template cards the caller's ACL admits, one to one. The spec lets a list vary by the request's authorization, never by connection |
| `resources/templates/list` | the same, for template cards |
| `resources/read` | forwarded to the owning server, with that Service's [secret](../../docs/06-services.md#secrets), which the caller's ACL already lets it read; the answer is returned as it came. An `https://` card with no owning server is fetched by the face itself: the spec lets a client fetch it directly, and the face still answers the read |

| Rule | |
|---|---|
| Read-only | a read changes nothing. The face passes it through; the owning server decides what the bytes are |
| Caching | the owning server's `ttlMs` passes through. `cacheScope` is at most what the ACL allows: `public` only when the card admits `*`, otherwise `private`, whatever the server said |
| Errors | an unknown URI, a card the caller may not see, or an owning server that does not answer is `-32602`, the last with a reason. `contents` is never empty for a Resource that is not there |
| The model's path | `ab_ls` returns `resource_link`s for the cards the caller may see, so a model reaches them through a tool and the client reads them like any other resource |
| The daemon | serves no content: no file, HTTP or git fetching in `agent-busd`. The fetching and forwarding happen in the face, which runs as the caller, so the [process boundary](../../docs/11-processes.md#the-rule) and the [trust boundary](../../docs/02-access.md#trust-boundary) do not change |

The face becomes an MCP client of each owning server, on the same latest
specification. Subscriptions (`subscriptions/listen`,
`notifications/resources/updated`) are not part of this.
