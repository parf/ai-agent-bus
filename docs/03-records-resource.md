# Resources

📌 **TL;DR:** A 📚 Resource is a card for data an MCP client may read: its URI
(or a URI template, 🧩), what it is, and who answers a read — an Agent, an MCP
Service, or the web for a plain `https://` card. The card holds no content, and
the daemon serves none.

## What a resource is

A 📚 Resource is information, not a service and not an
Agent: a card for data some source has, which the MCP face passes to MCP one to
one under the [latest specification](constitution.md#external-protocols). The
card carries no content; listing it promises that a read of its URI is answered.

| | |
|---|---|
| The card | the common fields as on every record, and a `resource` descriptor: `uri` (an RFC 6570 template when `template` is set, 🧩), the MCP `name` (the record's name when empty), and the optional `title`, `mimeType`, `size` (not on a template), `icons` and `annotations`. `description` is the record's own |
| Source | who answers a read: a 👾 Agent, or a 📡 Service of protocol `mcp`. Only a plain `https://` card may name none. Nothing is queued on the card, so queue settings, an address and private values are refused |
| Listing | `resources/list` and `resources/templates/list` answer the cards the caller's [ACL](02-access.md#acl) admits |
| Reading | `resources/read` finds the card (an exact URI first, then the first listed matching template — not the most specific, so overlapping templates are the registrant's to avoid) and asks its source. An Agent gets a message whose body is the URI, on topic `resources/read`, and its answer is the contents: plain text, or JSON `{contents, ttlMs, cacheScope}`. A Service is forwarded the read over MCP, with `MCP_AUTHORIZATION=…` from its [secret](03-records-service.md#secrets) as the Authorization header; the secret is its Owner's and Maintainers' to read, so a reader who may not read it is refused rather than sent on without it, and a Service with no secret is reached as it is. An `https://` card is fetched by the face, up to 10 MiB |
| Rules | a read changes nothing; `cacheScope` is `public` only when the card admits `*`; an unknown URI, a card the caller may not see, or a source that does not answer is `-32602`, never an empty `contents`; `ab_ls` returns a `resource_link` for each concrete card, so a model reaches them through a tool |

The daemon serves no content: the reading and forwarding happen in the face,
which runs as the caller. An Agent source is reached under its own ACL, so it
must admit whoever its cards admit.

<details>
<summary>Trust: a card's source is fetched from the reader's host</summary>

The face follows an `https://` card wherever it points, loopback and private
addresses included (Q141, [decision](decisions.md#settled)). That is a
server-side request made from the reader's machine on the card author's word.
It is accepted on a trusted bus: the answer goes only to the reader, who could
fetch it anyway, and plain `http://` is refused without a source. When users
other than the Owner register cards, the face should refuse loopback, private
and link-local addresses — checked after name resolution and after every
redirect, with an allowlist for intranet sources. No choice here stops what a
card's content says to a model that reads it; content from any source is
untrusted.

</details>

```sh
agent-bus register notes@team --uri 'md://notes/{+path}' --template \
  --source '#notes-reader@team' --mime text/markdown --allow '@team' --descr "team notes"
```
