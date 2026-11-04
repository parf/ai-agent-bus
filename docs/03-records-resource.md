# Resources

📌 **TL;DR:**
- **Problem:** an AI client wants to read data some source holds, and the bus should point to it without copying or serving it.
- **Outcome:** a 📚 Resource is a card for data an MCP client may read: its URI (or a URI template, 🧩), what it is, and who answers a read: an Agent, an MCP Service, or the web for a plain `https://` card. The card holds no content, and the daemon serves none.
- **Start:** list resource cards: `agent-bus ls --kind=resource` ([Try this first](#try-this-first)).

## Why you'd want it

- Let an AI discover readable data without copying that data into the bus.
- Describe a family of related resources with one URI template.

## What you get

- Searchable cards with a URI, description and source.
- MCP reads forwarded to an Agent, MCP Service or HTTPS endpoint under the relevant access rules.

## Try this first

List visible resource cards:

```sh
agent-bus ls --kind=resource
```

A fresh bus may have none. When a card exists, inspect its returned name with `agent-bus ls <name>`; listing the card does not fetch its source.

## What a resource is

- A 📚 Resource describes data through the [MCP resource interface](constitution.md#external-protocols). The card carries no content.
- Listing a card does not check that its source is available or accessible to the reader.

| | |
|---|---|
| The card | the common fields as on every record, and a `resource` descriptor: `uri` (an RFC 6570 template when `template` is set, 🧩), the MCP `name` (the record's name when empty), and the optional `title`, `mimeType`, `size` (not on a template), `icons` and `annotations`. `description` is the record's own |
| Source | who answers a read: a 👾 Agent, or a 📡 Service of protocol `mcp`. Only a plain `https://` card may name none. Nothing is queued on the card, so queue settings, an address and private values are refused |
| Listing | `resources/list` and `resources/templates/list` answer the cards the caller's [ACL](02-access.md#acl) admits |
| Reading | `resources/read` finds the card (an exact URI first, then the first listed matching template — not the most specific, so overlapping templates are the registrant's to avoid) and asks its source. An Agent gets a message whose body is the URI, on topic `resources/read`, and its answer is the contents: plain text, or JSON `{contents, ttlMs, cacheScope}`. A Service is forwarded the read over MCP, with `MCP_AUTHORIZATION=…` from its [secret](03-records-service.md#secrets) as the Authorization header; the secret is its Owner's and Maintainers' to read, so a reader who may not read it is refused rather than sent on without it, and a Service with no secret is reached as it is. An `https://` card is fetched by the face, up to 10 MiB |
| Rules | the card stores no read content; `cacheScope` is `public` only when the card admits `*`; an unknown URI, a card the caller may not see, or a source that does not answer is `-32602`, never an empty `contents`; `ab_ls` returns a `resource_link` for each concrete card, so a model reaches them through a tool |

The daemon serves no content: the reading and forwarding happen in the face,
which runs as the caller. An Agent source is reached under its own ACL, so it
must admit whoever its cards admit.

<details>
<summary>Trust: a card's source is fetched from the reader's host</summary>

- The face follows an `https://` card wherever it points, including loopback and private addresses.
- That is a server-side request made from the reader's machine on the card author's word.
- Register and read cards from trusted authors: the face does not block private network destinations. The answer goes to the reader; plain `http://` cards without a source are refused.
- No choice here stops what a card's content says to a model that reads it; content from any source is untrusted.

</details>

## Registering the card

This example requires an existing `#notes-reader@team` Agent that implements `resources/read` and admits the intended readers, plus the `@team` group. Replace them with your source and audience:

```sh
agent-bus register notes@team --uri 'md://notes/{+path}' --template \
  --source '#notes-reader@team' --mime text/markdown --allow '@team' --descr "team notes"
```

The resource flags map to fields in its `resource` descriptor:

| Flag | Field |
|---|---|
| `--uri` | `uri`, required for a resource |
| `--template` | `template`, an RFC 6570 URI template |
| `--source` | `source`, the Agent or MCP Service that reads it |
| `--mime` | `mimeType` |
| `--mcp-name` | `name`, the MCP name; otherwise the record name is used |
| `--title` | `title`, a display title |
| `--size` | `size`, a nonnegative byte count on a concrete resource, not a template |
| `--icons` | `icons`, a JSON array of icon descriptors |
| `--annotations` | `annotations`, a JSON object of MCP annotations |

`--descr` remains the record’s description, rather than a second descriptor
field. These flags describe data; they do not upload its contents.
