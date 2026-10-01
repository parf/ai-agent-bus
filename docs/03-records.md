# Records

📌 **TL;DR:** Every registered name is one of seven kinds: 👤 `user`, 👾 `agent`,
📮 `queue`, 📣 `pubsub`, 📡 `service`, 👥 `group` and 📚 `resource`. Four have a
queue here, a service says where something external is, a group is a named
list of actors, and a resource is a card for data an MCP client may read. `kind` is a closed set the daemon answers for, never inferred from
which fields are filled in.

## Status

| MVP | Scope |
|---|---|
| Built | [Seven record kinds](#record-kinds) closing `kind`, [Resource records](#resource-records) in 0.8.70, registry records, [agent templates](03-records-agent.md#agent-templates), private registry configuration and [Personal classification and web grouping](#personal-and-shared). A record's method information is its [description](03-records-agent.md#agent-templates). |

## Record kinds

`kind` is a closed set. The daemon answers what a record is, rather than a page
inferring it from which fields happen to be filled in.

| | Kind | What it is | Registered by | Someone acts as this name |
|---|---|---|---|---|
| 👤 | `user` | the queue a person reads | the daemon, when the person is registered | the person |
| 👾 | `agent` | the queue an [agent](03-records-agent.md#what-an-agent-is) reads | its launcher, or the agent itself at startup | the agent |
| 📮 | `queue` | a [topic](03-records-channel.md#the-two-channel-kinds) created to be shared, named for its own sake rather than for a principal | a user or an agent | nobody |
| 📣 | `pubsub` | a [pub/sub topic](03-records-channel.md#the-two-channel-kinds): it keeps nothing and copies each publication to everyone on its [Deliver-To list](04-messaging.md#subscribers) | a user or an agent | nobody |
| 📡 | `service` | a description of something [**external**](03-records-service.md#what-a-service-is), not on this bus | a user or an agent | nobody here |
| 👥 | `group` | a named list of actors: its `allow` is its [membership](01-identity-and-authority.md#groups), its name begins with `@`, and it has no queue | a user or an agent | nobody |
| 📚 | `resource` | a [card](#resource-records) for data an MCP client may read; 🧩 when it is a template. Its source answers a read; it has no queue | a user or an agent | nobody |

Built in 0.7.10: a Group is an ordinary registry record in the shared ID
space rather than a thing beside the registry
([constitution § Group](constitution.md#-group)).

Registering with no kind stores `service`, because describing something outside
is the case a bare `register` is usually for. `--personal` names an `agent`.

**The bus carries work between people and agents.** Those two act: they hold a
[credential](02-access.md#what-a-call-carries), send under their own name and
answer. The two channel kinds are **passive** — they hold or they copy,
and a person or an agent does the work at each end. A service does not even do
that: it is a card saying where something outside is and how to reach it.

The last column says what a name is *for*, not what the daemon refuses: a
credential can be minted for any registered name its owner asks for, and no
check consults the kind.

**The kind does not change who may read a record.** One allow list still governs
a record in both directions ([ACL](02-access.md#acl)), so a kind naming a single
principal describes what the record is *for*, not a reader the daemon enforces.
Splitting that list into a write side and a delivery side is a separate,
undecided question ([directional access](../Plans/R2.0-Future/acl-direction.md#where-direction-is-needed)).

**A service is the external case, and everything about it lives on its own
page.** It has no queue here, so nothing is sent to it, nothing subscribes it
and nothing consumes from it; it carries an address and a protocol instead, and
the daemon reaches neither. See [services](03-records-service.md#what-a-service-is).

Before R1.2 there is no compatibility obligation, so no migration is written:
existing records are registered again under the kind they should carry.

### Restoring a record

Restore asks the same shape question registration does. A record this version
could not have registered is [ignored and reported](constitution.md#persistence-and-loading),
never converted: an unknown kind, a kind that disagrees with the name's `#`, a
[service](03-records-service.md#it-has-no-queue-here) without an address or a
protocol, a service carrying queue settings or a queue, a
[secret](03-records-service.md#secrets) on a kind that holds none, an owner that is
not a User, or a [Personal](#personal-and-shared) record whose lists reach
outside its cohort. The report names the record and the reason, and the rest
of the node starts.

## Resource records

**Built in 0.8.70.** A 📚 Resource is information, not a service and not an
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

## How to call it

The kind says how a name is reached, and there are only two answers.

| Kind | How a caller reaches it |
|---|---|
| 👤 👾 📮 📣 | **send to the name.** The daemon puts the message in the [queue](04-messaging.md#inbox-queues) belonging to it, and whoever reads that queue takes it |
| 📡 | **call it directly**, at its own address — [services § how to call it](03-records-service.md#how-to-call-it) owns that half |

## Agents

👾 Agents — what they are, their fields, templates and configuration, and how
one is started — have their own page: [agents](03-records-agent.md#what-an-agent-is).

## Personal and shared

A record is **Personal** or **shared**. Personal states an intended audience —
the Owner and the agents that Owner owns — and keeps the web interface
readable: a few company-wide shared agents stay on the main pages while the
hundreds of per-user agents sit apart. Built in 0.7.5.

| Rule | Requirement |
|---|---|
| Carried by | every kind; the tag is not a kind ([record kinds](#record-kinds)) |
| 👤 user record | always Personal and cannot be made shared |
| `allow` and `maintainers` | only the Owner, the Owner's own agents by name, and the runtime [`@owner` and `@agent` terms](02-access.md#acl) |
| Refused | another user's agent, an ordinary group, a user entry other than the Owner, and the [wildcard grant](02-access.md#acl) — each an error on every save, at creation and after it |
| Everything else | unaffected: delivery, `deliver_to` and forwarding behave as on a shared record |
| Main web pages | exclude Personal records; each kind's list shows them under its `?personal=1` filter |
| Launchers | the [`ab-*` launchers](08-runner-role.md#session-names) register their session agents Personal, with `@owner` in the ACL |

Hiding a record from the main web pages does not revoke authorized access or
remove it from the registry.

<details>
<summary>How the stored classification changes</summary>

Only the owner changes Personal. A new registration may state it; an agent
refreshing its metadata preserves the owner's stored choice. A stored record
whose lists reach outside its cohort is
[ignored and reported](constitution.md#persistence-and-loading) at load. A request that
changes Personal and its assignments is checked as one final record, so an
owner can remove sharing while enabling Personal, or disable Personal while
adding sharing, in one operation.

Assignment validity is checked when a record is written, and kept true after
it: removing an allowed agent takes every reference to it in the removal's
commit ([unregistering](01-identity-and-authority.md#unregistering)), and
transferring one to another User takes it off its old Owner's Personal
records in the transfer's commit, since it has left that cohort.

</details>
