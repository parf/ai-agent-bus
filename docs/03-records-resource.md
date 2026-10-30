# Resources

📌 **TL;DR:**
- **Problem:** an AI client wants to read data some source holds, and the bus should point to it without copying or serving it.
- **Outcome:** a 📚 Resource is a card for data an MCP client may read: its URI (or a URI template, 🧩), what it is, and who answers a read: an Agent, an MCP Service, or the web for a plain `https://` card. The card holds no content, and the daemon serves none.
- **Start:** look at a resource card: `agent-bus ls <name>` ([Try this first](#try-this-first)).

## Why you'd want it

- 📚 **An AI should find readable data.** Now a card lists it where the AI already looks.
- 🧭 **The bus is not a file server.** Now the card points to the source, and the source answers the read.
- 🧩 **Data comes in families.** Now one card can carry a URI template.

## What you get

- ✅ **A card:** a URI, what it is and who answers a read.
- ✅ **Listing and reading through MCP:** passed one to one to the MCP face.
- 🚫 **Not for:** storing content; the daemon serves none.

## Try this first

- Look at a resource card (read-only):

```sh
agent-bus ls smoke-gate@parf.us | jq -c 'if type=="array" then .[0] else . end | {name,kind,owner}'
```

<details>
<summary>See it on a real node (ran today)</summary>

```json
{"name":"smoke-gate@parf.us","kind":"resource","owner":"parf"}
```

- It is a 📚 resource owned by `parf`.
- The output is from the development node on 2026-10-30.

</details>

## Technical details

The reference below keeps every section as it was; other pages link to these sections.

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

- The face follows an `https://` card wherever it points, loopback and private addresses included (Q141, [decision](decisions.md#settled)).
- That is a server-side request made from the reader's machine on the card author's word.
- It is accepted on a trusted bus: the answer goes only to the reader, who could fetch it anyway, and plain `http://` is refused without a source.
- When users other than the Owner register cards, the face should refuse loopback, private and link-local addresses — checked after name resolution and after every redirect, with an allowlist for intranet sources.
- No choice here stops what a card's content says to a model that reads it; content from any source is untrusted.

</details>

```sh
agent-bus register notes@team --uri 'md://notes/{+path}' --template \
  --source '#notes-reader@team' --mime text/markdown --allow '@team' --descr "team notes"
```

## Registering the card

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
