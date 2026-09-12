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
  [access § what a call carries](02-access.md#what-a-call-carries). Only generic
  records have none.
- Publisher/consumer name **capabilities on a principal** (`publish:<glob>`,
  `consume:<glob>`), not separate objects. They appear in the table above
  because a record says which a principal behaves as, not because they are a
  third kind of thing.
- generic vs agent differ only in record owner and health mode → same record
  type with a `kind` (when models are designed).

## How to call it

A registration says **where** something is (`addr`); it also says **how** to
talk to it, and the interesting case is that almost nothing needs to.

| `protocol` | Means |
|---|---|
| **unset** | an ordinary agent-bus service: send to its name and the daemon delivers to its inbox ([messaging § inbox queues](04-messaging.md#inbox-queues)). This is the default because it is the common case |
| anything else | **the caller speaks it directly**, at `addr`. `mysql`, `https`, `amqp` — the bus passes the word along and does nothing with it |

**`/etc/services` is the suggested vocabulary, and only a suggestion.** Use
the name from it where there is one, so two people registering the same kind
of thing write the same word. It is not a checked set and cannot become one:
the file is outdated and incomplete — half of what anyone registers here
(`mcp`, `grpc`, an in-house protocol) is not in it, and refusing those would
make the field useless to the people who need it most.

The value is **stored raw and never interpreted** — the same rule the MCP
method info and a configuration follow. The daemon does not implement a
second protocol, does not proxy, and does not refuse a send to a record that
names one: a runner or gateway may well be reading that inbox on the thing's
behalf ([runner § adapters](08-runner-role.md#adapters)). It is a fact in the
registry for whoever is choosing what to call
([discovery § what a listing answers](05-discovery.md#what-a-listing-answers)).

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
| `service@realm` | a standalone configured service: there is no separate template to name |
| `template/instance-name@realm` | configured from a template: `imap-mail-reader/billing@rdvp`, `code-review/claude-2@rdvp` |

The name is stable across restarts, and is also the address and the inbox
([identity § names](01-identity.md#names)). Two services from one template are
**two services**: `code-review/claude@rdvp` and `code-review/claude-2@rdvp`
share a prefix and nothing else — two records, two inboxes, two configs.

**Config is arbitrary and separate from identity.** The name says which
template and which instance; it never *declares* what the service was pointed
at. Naming an instance after the thing it reads is fine and often clearest —
`mail-sender/parf+alerts@comfi.com@realm` is a legal name
([identity § names](01-identity.md#names)) — but that is a human convention,
not a field: nothing parses a name for server, user, mailbox or credentials,
and there is no fixed config schema. `imap-mail-reader/parf@rdvp` with the
mailbox in its config is equally correct. Private config stays with the service by default, optionally
sealed in `agent-busd`
([identity § sealed private config](01-identity.md#sealed-private-config)).

Services register, heartbeat, vanish. Definitions are **live records** changed
by owners under [identity § ownership](01-identity.md#ownership), signed by the
writer when it has a key.

**Addressing many services as one is deferred.** A template may be configured
on several hosts — `code-review/claude@rdvp` and `code-review/claude@srv2` —
but asking them at once and gathering their answers (scatter-gather) is
Release 1, with its how-to ([stages § release 1](12-stages.md#release-1)).
Until then each is addressed on its own, and a template prefix is a shared
name, not a group. One *name* served from several hosts is the other thing and
has its own design — a pool ([runner § one name on many
hosts](08-runner-role.md#one-name-on-many-hosts)).

## Configuring a template

Configuring a service template **is** what produces a configured service. One
verb does it, and reads it back:

| | |
|---|---|
| `cat cfg.json \| agent-bus service-template <template/instance@realm> -` | configure it, JSON on stdin |
| `agent-bus service-template <template/instance@realm> '{"k":"v"}'` | the same, inline |
| `agent-bus service-template <template/instance@realm>` | print that configuration |

The direction is decided by whether a configuration was handed to it, and
setting one answers with its digest rather than with what was set. The verb
is **one hyphenated word** so that it stays a single verb in every face,
including as an MCP tool name
([glossary § names that are enforced](glossary.md#names-that-are-enforced)).

The service is created if it does not exist, with the defaults a bare
registration gets — configuring is not a second way to describe a service,
only the way to give it one. A standalone `service@realm` takes a configuration
the same way; the template prefix is not what makes one configurable.

| | |
|---|---|
| the configuration | **arbitrary JSON, stored opaque.** The only check is that it *is* JSON — the same "stored raw, shape-checked only" rule the MCP method info follows. Nothing looks for a server, a user, a mailbox or a credential |
| who may write it | its **owner, or the service itself**. Unlike a registration, a configuration is not something any caller may overwrite — and registering does not overwrite one either, so a service restarting keeps what it was configured with. A registration carries **neither half**: not the bytes, and not the digest, which is derived from them and would otherwise let anyone claim any setup |
| who may read it | **the service, and nobody else — its owner included.** Setup data goes in and is used; it does not come back out to be looked at |
| what a query gets | **`config_sha`**, a SHA-256 of the stored bytes, on every answer that carries a record — the whole listing, a query for one service (`agent-bus ls <name>`), and the answer to setting one ([why a digest at all](#why-a-digest-at-all)) |
| where the bytes are **not** | anywhere else. No listing carries them, and the one read is the service's own |
| nothing to store | refused: no configuration at all, something that is not JSON, and `null` — which would read back exactly like never having been configured |
| an empty *value* | kept. `{}`, `[]`, `""`, `0` and `false` are configurations; the bus does not judge what is inside |

### Why a digest at all

**So that anything watching can tell whether a service's setup has been
changed by someone, without ever being shown it.** A configuration cannot be
read back — not even by its owner — so the only other way to answer "is this
still what I set?" would be to hand out the secrets to compare. The digest
answers it without them:

| Asking | How |
|---|---|
| is this service configured? | a `config_sha` is there, or it is not |
| did my write land? | setting one answers with its digest; compare it to the next query |
| has someone changed it since? | the digest moved |
| do these two services hold the same setup? | the digests match |
| is this host's copy the one I shipped? | compare digests across hosts |

A
monitor, a peer, a deploy check or the owner can all hold the digest they
expect and notice the day it differs.

The bus stores **one spelling**: the bytes are compacted, so reformatting a
configuration file is not a change and does not move the digest. That also
means `sha256sum cfg.json` matches only if the file is already compact.

A digest of a **short, guessable** configuration can be recovered by trying
candidates. That is inherent in publishing a digest at all; sealing is the
answer, not a longer hash.

⚠️ **Plaintext in the registry, and only as private as the daemon.** Sealing is
designed but not built
([identity § sealed private config](01-identity.md#sealed-private-config)), and
its stage is [stages § release 1](12-stages.md#release-1). Until then: a
configuration is readable by anyone who can read the daemon's state — the
snapshot it reloads from included ([messaging §
durability](04-messaging.md#durability)). The owner check itself is real now — the
caller *is* their credential
([access § what a call carries](02-access.md#what-a-call-carries)) and a record
is only its owner's to change ([identity § ownership](01-identity.md#ownership))
— but *claiming* a name nobody holds is still open to anyone.

**A service fetches its own configuration; nothing injects it** — and it is
the only one that can, so this runs as the service, not as its owner. This is
the *registry's* configuration, not the runner's environment, which is the
other thing that word names ([glossary § terms](glossary.md#terms)):

```sh
cfg=$(agent-bus service-template "$AGENT_BUS_NAME")
```

❓ **What happens to a running service when its configuration changes is
unspecified** — it is not reloaded, restarted or notified, so it sees a change
only if it reads again. *Settled by:* the owner, when the runner supervises
services ([runner](08-runner-role.md)).

❓ **Nothing on a record carries method information**, so there is nothing for
a generated catalog to generate from ([discovery § faces](05-discovery.md#faces)).
What shape that takes is a data model. *Settled by:* the owner, when the MVP
faces are built.

## Topics

A topic is registered like a service and is **first-class** in the same
registry. Two kinds — the Redis model:

| Kind | Delivery | Retention | No subscribers at publish time | Redis analogue |
|---|---|---|---|---|
| **queue** | each message to **one** consumer (competing consumers take turns) | until consumed or **TTL**; bounded; overflow per mode | fine — it waits for its TTL | list + `BRPOP` + `EXPIRE` |
| **pub/sub** | a **copy** to every subscriber, into that subscriber's own inbox ([messaging § subscribers](04-messaging.md#subscribers)) | none of its own — each copy is kept by the inbox it is in | dropped, a no-op | `PUBLISH` / `SUBSCRIBE` |

A topic **declares its kind, TTL, bound and overflow mode at creation**:

| Aspect | Rule |
|---|---|
| Create | any authenticated principal |
| Change / delete | owner or maintainer ([identity § ownership](01-identity.md#ownership)) |
| Record | name, kind, TTL, bound, overflow, owner, description, audience |
| Visibility | registry and MCP catalog, audience-filtered ([discovery § audience](05-discovery.md#audience)) |
| Access | `publish:<glob>` / `consume:<glob>` on principals. Today that is the record's `allow`, asked of a publisher when it publishes and of a subscriber both when it subscribes and at every publish ([messaging § subscribers](04-messaging.md#subscribers)) |
| Signature | signed by the writing principal **when it has a key**; a static-token write is unsigned — the token authenticated it. Nodes verify signatures where present before accepting or syncing |
| Storage | live record in `agent-busd`, snapshotted to git; messages are in memory, see [messaging § durability](04-messaging.md#durability) |
| Stats | depth, in/out rate, drops, subscriber count — on the dashboard like a service |

Inboxes are implicit queue topics created on an agent's first start and owned
by it ([messaging § inbox queues](04-messaging.md#inbox-queues)). Namespacing
follows services; local shadows upstream — and which separator a namespace uses
is open ([overview § chaining](00-overview.md#chaining)).

## How long a record lives

**A record is `kept` or `ephemeral`, and that is a different axis from its
kind.** Kind says what the thing is ([service kinds](#service-kinds));
this says whether the registry is meant to hold it after nobody is using it.

| | Registered by | Expires |
|---|---|---|
| **`kept`** | the runner, and anything that asks for it | never on its own |
| **`ephemeral`** | `agent-bus start` and the dashboard, by default | after long inactivity — weeks, not hours |

The default falls where the registrations do: the runner keeps a list of what
is installed and means every entry to persist
([runner § the list of what is installed](08-runner-role.md#the-list-of-what-is-installed)),
while a name that appeared because somebody ran a command is incidental until
somebody says otherwise. A hand-started service that is meant to stay says so;
nothing is derived from who registered it, because a derived answer and the
runner's list would be two truths about one thing.

**Nothing that is being served ever expires.** `reading` already says whether a
read is outstanding on an inbox right now
([discovery § what a listing answers](05-discovery.md#what-a-listing-answers)),
so the clock only ever considers records nobody is serving, and a rarely-called
tool that is sitting there connected is safe. Inactivity is measured from the
last thing that happened on the name — registered, read, or delivered to — all
of which the daemon already counts.

**A person may always delete, served or not.** The clock is restrained;
authority is not. But deleting a served record is narrower than it looks:

- The daemon **cannot stop the process**. No process the daemon starts may exec
  at all, and the runner is a separate program under its own account, not a
  child ([processes](11-processes.md)). Deleting forgets the record; the
  process keeps running, blind, and sends to it refuse as *no such name*.
- **It comes back if that process restarts**, because a service re-registers on
  every start ([identity § ownership](01-identity.md#ownership)).

So for something running, the honest order is **stop it, then delete it**, and
delete-while-served is the escape hatch rather than the path.

⚠️ **Expiry is single-node until peer sync has a clock.** Deletion has no
representation in *newer record wins per entry* ([registry sync](#registry-sync)):
a deleted record returns from whichever peer still holds it, and a wrong clock
stops meaning *a stale record won* and starts meaning *a live service was
deleted somewhere else*. The open question there gates this one.

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
