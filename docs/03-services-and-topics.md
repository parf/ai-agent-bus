# Services and topics

What lives on the bus. Both are **records** in `agent-busd`'s registry, created
and changed under the same rules.

## Service kinds

Two axes: reachable or not, and who owns the record.

| Kind | Reachable | Registered by | Health | Signs events |
|---|---|---|---|---|
| **generic** | yes (host/port, unix socket, HTTP) | a user, on behalf of something that exists — description, address, optional MCP method info | health-checker probes per hints | no (no identity) |
| **agent** | yes | itself, own identity | self-reports health/stats | if key-holding |
| **consumer** | no (pulls) | itself, own identity | none | no |
| **publisher** | not a service — an identity that emits events (`agent-bus` CLI or `curl`) | — | none | if key-holding |

- **A shell script is a service too** — one command registers it and answers
  from its stdout ([runner § script services](08-runner-role.md#script-services)).
- **Registering is just pushing a description.** "There is a MySQL service
  named `xxx` on `host:port`" is a complete registration; the thing itself need
  not know the bus exists. Same for an HTTP API, a unix socket, a cron host.
  Health hints are optional extras.
- **Identity is required** for agents, consumers and publishers — see
  [access § two parameters](02-access.md#two-parameters). Only generic records
  have none.
- Publisher/consumer name **capabilities on a principal** (`publish:<glob>`,
  `consume:<glob>`), not separate objects. They appear in the table above
  because a record says which a principal behaves as, not because they are a
  third kind of thing.
- generic vs agent differ only in record owner and health mode → same record
  type with a `kind` (when models are designed).

## Personal and shared

Default is **shared**.

- **personal** — runs *as* a specific user with their account (mail-reader, a
  Claude session joined via `claude --channel`). Owner = the user; default
  audience = the user; lifecycle follows the user; usually one of them;
  typically on the user's own machine. Key: the user's (ephemeral sessions) or
  its own (recommended for anything long-running — narrower, individually
  revocable). When it calls another service, *it is the user calling* — no
  delegation ([identity § delegation](01-identity.md#delegation)).
- **shared** — runs as a service principal for many; explicit ACL; long-lived;
  health-checked.

## Service and template

A **service template** is the *unconfigured* capability — description, declared
roles, health hints, optional MCP method info (stored **raw**, shape-checked
only; docs generated from it). The description may mark individual methods as
**destructive**; the MCP face passes the mark through to the calling agent and
does nothing else with it — a hint from the service, enforced by nobody but the
receiver. A template does not run, holds no config and has **no address**.

A **service is always configured**: a template, its private config, and a place
it runs. Never call an unconfigured template a service. "Configured" is about
*existing as an instantiated thing* — a place to run and an identity; a
`service-template` configuration blob is optional on top
([configuring a template](#configuring-a-template)).

| | |
|---|---|
| `service@host` | a standalone configured service: there is no separate template to name |
| `template/instance-name@host` | configured from a template: `imap-mail-reader/billing@rdvp`, `code-review/claude-2@rdvp` |

The name is stable across restarts, and is also the address and the inbox
([identity § names](01-identity.md#names)). Two services from one template are
**two services**: `code-review/claude@rdvp` and `code-review/claude-2@rdvp`
share a prefix and nothing else — two records, two inboxes, two configs.

**Config is arbitrary and separate from identity.** The name says which
template and which instance; it never *declares* what the service was pointed
at. Naming an instance after the thing it reads is fine and often clearest —
`mail-sender/parf+alerts@comfi.com@host` is a legal name
([identity § names](01-identity.md#names)) — but that is a human convention,
not a field: nothing parses a name for server, user, mailbox or credentials,
and there is no fixed config schema. `imap-mail-reader/parf@rdvp` with the
mailbox in its config is equally correct. Private config stays with the service by default, optionally
sealed in `agent-busd`
([identity § sealed private config](01-identity.md#sealed-private-config)).

Services register, heartbeat, vanish. Definitions are **live records** changed
by owners under [identity § ownership](01-identity.md#ownership), signed by the
writer when it has a key.

**One service on many hosts is deferred.** A template may be configured on
several hosts — `code-review/claude@rdvp` and `code-review/claude@srv2` — but
addressing them *as one* and gathering their answers (scatter-gather) is
Release 1, with its how-to
([stages § release 1](12-stages.md#release-1)). Until then each is addressed on
its own, and a template prefix is a shared name, not a group.

## Configuring a template

Configuring a service template **is** what produces a configured service. One
verb does it, and reads it back:

| | |
|---|---|
| `cat cfg.json \| agent-bus service-template <template/instance@host> -` | configure it, JSON on stdin |
| `agent-bus service-template <template/instance@host> '{"k":"v"}'` | the same, inline |
| `agent-bus service-template <template/instance@host>` | print that configuration |

The direction is decided by whether a configuration was handed to it. The verb
is **one hyphenated word** so that it stays a single verb in every face,
including as an MCP tool name
([glossary § names that are enforced](glossary.md#names-that-are-enforced)).

The service is created if it does not exist, with the defaults a bare
registration gets — configuring is not a second way to describe a service,
only the way to give it one. A standalone `service@host` takes a configuration
the same way; the template prefix is not what makes one configurable.

| | |
|---|---|
| the configuration | **arbitrary JSON, stored opaque.** The only check is that it *is* JSON — the same "stored raw, shape-checked only" rule the MCP method info follows. Nothing looks for a server, a user, a mailbox or a credential |
| who may write it | its **owner, or the service itself**. Unlike a registration, a configuration is not something any caller may overwrite — and registering does not overwrite one either, so a service restarting keeps what it was configured with |
| who may read it | the same two |
| where it is **not** | a listing. `ls` never carries a configuration, so this verb is the only route to one |
| nothing to store | refused: no configuration at all, something that is not JSON, and `null` — which would read back exactly like never having been configured |
| an empty *value* | kept. `{}`, `[]`, `""`, `0` and `false` are configurations; the bus does not judge what is inside |

⚠️ **Plaintext in the registry, and only as private as the daemon.** Sealing is
designed but not built
([identity § sealed private config](01-identity.md#sealed-private-config)), and
its stage is [stages § release 1](12-stages.md#release-1). Until then: a
configuration is readable by anyone who can read the daemon's state, it is lost
when the daemon restarts, and the owner check is a caller-name guard, not
security — in PoC one master token reaches everything and any holder may claim
any name ([stages § PoC](12-stages.md#poc)).

**A service fetches its own configuration; nothing injects it.** There is no
automatic delivery — a service reads it the same way anything else does, with
the same verb under its own name:

```sh
cfg=$(agent-bus service-template "$AGENT_BUS_NAME")
```

❓ **What happens to a running service when its configuration changes is
unspecified** — it is not reloaded, restarted or notified, so it sees a change
only if it reads again. *Settled by:* the owner, when the runner supervises
services ([runner](08-runner-role.md)).

## Topics

A topic is registered like a service and is **first-class** in the same
registry. Two kinds — the Redis model:

| Kind | Delivery | Retention | No subscribers at publish time | Redis analogue |
|---|---|---|---|---|
| **queue** | each message to **one** consumer (competing consumers take turns) | until consumed or **TTL**; bounded; overflow per mode | fine — it waits for its TTL | list + `BRPOP` + `EXPIRE` |
| **pub/sub** | a **copy** to every current subscriber | none | dropped, a no-op | `PUBLISH` / `SUBSCRIBE` |

A topic **declares its kind, TTL, bound and overflow mode at creation**:

| Aspect | Rule |
|---|---|
| Create | any authenticated principal |
| Change / delete | owner or owner group ([identity § ownership](01-identity.md#ownership)) |
| Record | name, kind, TTL, bound, overflow, owner, description, audience |
| Visibility | registry and MCP catalog, audience-filtered ([discovery § audience](05-discovery.md#audience)) |
| Access | `publish:<glob>` / `consume:<glob>` on principals |
| Signature | signed by the writing principal **when it has a key**; a static-token write is unsigned — the token authenticated it. Nodes verify signatures where present before accepting or syncing |
| Storage | live record in `agent-busd`, snapshotted to git; messages are in memory, see [messaging § durability](04-messaging.md#durability) |
| Stats | depth, in/out rate, drops, subscriber count — on the dashboard like a service |

Inboxes are implicit queue topics created on an agent's first start and owned
by it ([messaging § inbox queues](04-messaging.md#inbox-queues)). Namespacing
follows services; local shadows upstream — and which separator a namespace uses
is open ([overview § chaining](00-overview.md#chaining)).

## Registry sync

Between **peer nodes at the same level**, the registry syncs through git — no
live replication protocol.

- Each node holds its registry live and writes snapshots to the shared git
  repo. **On start a node pushes its snapshot and pulls from the other known
  nodes.**
- **Newer record wins per entry, provided its writer had access.** A
  key-holding writer's record carries its signature, which a peer checks before
  taking it; a static-token record is trusted on the strength of the node that
  accepted it.
- **Upstreams are not replicated.** An upstream `agent-busd` has its own
  registry and we usually lack full access to it; chaining queries it and
  caches answers, nothing more
  ([overview § chaining](00-overview.md#chaining)).

❓ **Peer sync trusts unsigned records and has no clock authority** — a peer can
push an unsigned record for any name, and "newer wins" compares clocks that are
not synchronised. *Settled by:* owner.
