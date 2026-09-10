# Stages

What gets built when, and what counts as done. Each stage ends with something
that **works end to end** — not a layer that waits for the next one.

PoC is the owner's. MVP and Release 1 are proposed and want a cut.

| | PoC | MVP | Release 1 |
|---|---|---|---|
| Identity | one master token, issued over SSH | `user@realm` + token, per-user sockets | groups, roles, delegation |
| Registration | none — the token is everything | manual + GitHub | AUTH bundle, LDAP/AD if wanted |
| Encryption | **none** | AEAD sessions, bodies end to end | — |
| Access control | master token reaches everything | service ACL + master ACL | expressions over groups |
| Storage | memory only | SQLite + Parquet dumps | git snapshots, peer sync |
| Processes | one | supervisor + children | AUTH child; billing only if it ships |
| Services | request/reply with `ack` | deadlines, `done`, `reply-to`, many instances | calls across chained buses |
| Faces | CLI + basic MCP | MCP with generated docs, filtered; dashboard | — |
| Install | built Go binary; Node runs the faces | `npm install` + `agent-bus setup` | packaged, zero-downtime reload |

## PoC

Prove that two agent sessions can find each other and talk. Nothing else has
to be true.

**Build**

| | |
|---|---|
| daemon | one process, listening on a **unix socket and HTTP** |
| language | daemon and CLI in **Go**; the MCP face and adapters in **TypeScript**, built and tested by bun, **run on Node** ([modules § languages](10-modules.md#languages)) |
| identity | **one master token per user**, reaching every service — no per-service anything ([identity § acl](01-identity.md#acl)) |
| tokens | issued **over SSH** — `ssh agent-bus@<node> static-token` ([access § getting a token](02-access.md#getting-a-token)). Kept even in PoC because it **costs us nothing**: sshd does the authentication against a key the user already has, and our side is a forced command |
| encryption | **none — no sessions at all.** Bodies travel plaintext, the `encryption: off` path the design already has for development ([access § encrypted sessions](02-access.md#encrypted-sessions)) |
| services | **basic request/reply** ([messaging § request and reply](04-messaging.md#request-and-reply)): a service consumes its inbox, `ack`s it (got it), does the work and replies; a caller sends and waits for that reply. The reply stands in for `done`, which comes at MVP |
| mcp | **basic MCP face** — list what is registered, send, consume. Unfiltered: the master token sees everything ([discovery § faces](05-discovery.md#faces)) |
| install | run the built binary. **No npm** |
| setup | one thing only: the user's pubkey in the `agent-bus` account's `authorized_keys` behind that forced command |
| storage | memory. A restart loses everything, and that is fine here |
| exposure | **loopback or an SSH tunnel only.** Plaintext bodies and a master token are acceptable in a PoC; putting them on a public interface is not |

**CLI — the basic set**

| Verb | Does |
|---|---|
| `agent-bus register <name> [--addr …]` | put a service or agent in the registry |
| `agent-bus ls [--kind …]` | what is registered — discovery, human-readable |
| `agent-bus send <name> [--topic] [--tag]` | one message to one receiver |
| `agent-bus call <name> [--topic] [--tag]` | send and wait for the reply |
| `agent-bus publish --topic <t>` | one message to a topic |
| `agent-bus consume [--follow]` | read my own queue |
| `agent-bus ack <message-id>` | receipt: got it ([messaging § receipts](04-messaging.md#receipts)) |
| `agent-bus reply <message-id>` | answer it — routed back by topic + tag |
| `agent-bus topic create <t> --kind queue\|pubsub` | a topic to publish into |
| `agent-bus status` | is the daemon up, who is connected |

Ten verbs. Everything else (`keygen`, `auth *`, `start`/`stop`/`logs`) waits.

❓ **How a consumer names a topic** — `consume` reads *your own queue*, and a
pub/sub topic reaches "every current subscriber", but with AUTH off nothing
says who is subscribed and no verb subscribes. PoC default: **a queue topic is
an inbox with a name**, so `consume --topic <t>` reads it — an option, not an
eleventh verb — and publish to a pub/sub topic answers *not in PoC*.
*Settled by:* owner.

**Works at the end of PoC**

- A **Claude Code session talks to a Codex session** and back, by name.
- A **publisher** emits to a topic without being a registered service.
- A **consumer** reads that topic, including messages sent while it was down.
- **Service publishing and discovery**: register something, another party
  finds it by name and calls it.
- An agent asks the **MCP face** "what can I use?" and can send and consume
  through it.
- A **service** takes a request, acknowledges it, and answers; the caller
  blocks on the answer and gets the right one back, matched by topic + tag.

The **push adapters** — Claude Code Channels and the Codex App Server
([runner § adapters](08-runner-role.md#adapters)) — are what let a live session
*receive* instead of poll. V1 has both, and they are **prior art, not code we
inherit**: they carry the signed wires, journals and delivery observations
JetStream needed and V2 does not. PoC writes small ones fresh and copies a
single file, the App Server's JSON-RPC client.

**Deliberately absent**: encryption, AUTH, per-service ACLs, persistence,
sandboxing, the dashboard, generated docs and catalog filtering, npm.

Note what plaintext costs: in PoC the daemon, its logs and anyone on the host
can read message bodies, so *"the bus never reads payloads"* is not yet true.
It becomes true at MVP, with AEAD sessions.

## MVP

*Proposed.* Somebody other than the author can install it and use it safely on
a shared host.

**Build**

| | |
|---|---|
| identity | `user@realm` + token, per-user sockets ([access](02-access.md)) |
| registration | manual record + GitHub ([identity § registration](01-identity.md#registration)) |
| tokens | persisted, previous kept, local never expires ([access § token lifetime](02-access.md#token-lifetime)) |
| encryption | AEAD sessions; bodies end to end ([access § encrypted sessions](02-access.md#encrypted-sessions)) |
| access | service ACL, then master ACL; a service may refuse master ([identity § acl](01-identity.md#acl)) |
| messaging | TTL, bound, `ring`/`strict`, receipts ([messaging](04-messaging.md)) |
| services | calls grow up: `done` (finished processing) as well as `ack` (got it), caller deadlines, `reply-to` a third party, several instances behind one name, per-service call stats ([messaging](04-messaging.md)) |
| storage | SQLite store, Parquet dump and reload ([setup § storage](09-setup.md#storage)) |
| faces | the PoC MCP face grown up: generated docs, catalog filtered per caller; a basic dashboard ([discovery § faces](05-discovery.md#faces)) |
| runner | supervise and sandbox children ([runner role](08-runner-role.md)) |
| processes | the supervisor/children split ([processes](11-processes.md)) |
| install | `npm install -g` + `agent-bus setup` ([setup](09-setup.md)) |

**Works at the end of MVP**

- Several users share one host, each seeing only the services they may.
- The bus restarts without losing queued messages, and the backlog still
  decrypts.
- An agent asks the MCP face "what can I use?" and gets a filtered catalog.
- A child registered under the runner is supervised, sandboxed and reachable.

**Deliberately absent**: AUTH role, groups, chaining, peer sync, billing,
client libraries in other languages.

## Release 1

*Proposed.* A team or a company can run it, and it can be exposed.

| | |
|---|---|
| AUTH role | bundle, signed generations, git over SSH, 2+ replicas ([AUTH role](06-auth-role.md)) |
| policy | groups with `& \| !`, service-defined roles, delegation ([identity](01-identity.md)) |
| admin | SSH forced commands, audit log ([AUTH role § SSH admin](06-auth-role.md#ssh-admin)) |
| federation | chaining to an upstream; peer registry sync through git ([overview § chaining](00-overview.md#chaining)) |
| calls | a call reaches a service on the **upstream** bus the same way it reaches a local one, carrying on-behalf-of; long answers stream ([overview § chaining](00-overview.md#chaining)) |
| observability | health-checker, stats, Prometheus export ([discovery](05-discovery.md)) |
| secrets | sealed private config ([identity § sealed private config](01-identity.md#sealed-private-config)) |
| billing | **last in the stage, and the first thing to drop.** Nothing else waits on it — the design is written ([billing role](07-billing-role.md)), so it can ship in Release 1 or move to `future/` without touching anything |
| clients | Go, PHP, Rust, JS, Python — gated on how `protocol` is specified ([modules](10-modules.md)) |
| operations | zero-downtime reload, packaging |

**Works at the end of Release 1**

- A company bus with central identities and groups, two AUTH replicas, and no
  service holding its own user list.
- A laptop bus chains to it: local first, upstream for the rest.
- A stranger with a GitHub key enrols in a public service — and is charged for
  it, if billing shipped.

❓ **Billing in Release 1, or deferred** — nothing in the bus depends on it and
it is the one role that needs a RADIUS server and a payment provider to be
worth anything. Release 1 is the earliest it could ship; `future/` is the
honest place for it if Release 1 is already heavy. *Settled by:* owner.
