# Stages

What gets built when, and what counts as done. Each stage ends with something
that **works end to end** — not a layer that waits for the next one.

PoC is the owner's. MVP and Release 1 are proposed and want a cut.

| | PoC | MVP | Release 1 |
|---|---|---|---|
| Identity | one master token | `user@realm` + token, per-user sockets | groups, roles, delegation |
| Registration | none — the token is everything | manual + GitHub | AUTH bundle, LDAP/AD if wanted |
| Encryption | **off** | AEAD sessions, bodies end to end | — |
| Access control | master token reaches everything | service ACL + master ACL | expressions over groups |
| Storage | memory only | SQLite + Parquet dumps | git snapshots, peer sync |
| Processes | one | supervisor + children | AUTH and billing children |
| Install | built binary | `npm install` + `agent-bus setup` | packaged, zero-downtime reload |

## PoC

Prove that two agent sessions can find each other and talk. Nothing else has
to be true.

**Build**

| | |
|---|---|
| daemon | one process, listening on a **unix socket and HTTP** |
| identity | **one master token per user**, reaching every service — no per-service anything ([identity § acl](01-identity.md#acl)) |
| encryption | **none.** Bodies travel plaintext, the `encryption: off` path the design already has for development ([access § encrypted sessions](02-access.md#encrypted-sessions)) |
| install | run the built binary. **No npm** |
| storage | memory. A restart loses everything, and that is fine here |

**CLI — the basic set**

| Verb | Does |
|---|---|
| `agent-bus register <name> [--addr …]` | put a service or agent in the registry |
| `agent-bus ls [--kind …]` | what is registered — discovery, human-readable |
| `agent-bus send <name> [--topic] [--tag]` | one message to one receiver |
| `agent-bus publish --topic <t>` | one message to a topic |
| `agent-bus consume [--follow]` | read my own queue |
| `agent-bus topic create <t> --kind queue\|pubsub` | a topic to publish into |
| `agent-bus status` | is the daemon up, who is connected |

Seven verbs. Everything else (`keygen`, `auth *`, `start`/`stop`/`logs`) waits.

**Works at the end of PoC**

- A **Claude Code session talks to a Codex session** and back, by name.
- A **publisher** emits to a topic without being a registered service.
- A **consumer** reads that topic, including messages sent while it was down.
- **Service publishing and discovery**: register something, another party
  finds it by name and calls it.

The largest single piece is the pair of **push adapters** — Claude Code
Channels and the Codex App Server
([runner § adapters](08-runner-role.md#adapters)) — because a live session has
to *receive*, not poll. Everything else in PoC is small next to this.

**Deliberately absent**: encryption, AUTH, per-service ACLs, persistence,
sandboxing, the dashboard, the MCP face, npm.

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
| storage | SQLite store, Parquet dump and reload ([setup § storage](09-setup.md#storage)) |
| faces | MCP with generated docs, audience-filtered; a basic dashboard ([discovery § faces](05-discovery.md#faces)) |
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
| observability | health-checker, stats, Prometheus export ([discovery](05-discovery.md)) |
| secrets | sealed private config ([identity § sealed private config](01-identity.md#sealed-private-config)) |
| billing | optional role, RADIUS, paid public API ([billing role](07-billing-role.md)) |
| clients | Go, PHP, Rust, JS, Python — gated on how `protocol` is specified ([modules](10-modules.md)) |
| operations | zero-downtime reload, packaging |

**Works at the end of Release 1**

- A company bus with central identities and groups, two AUTH replicas, and no
  service holding its own user list.
- A laptop bus chains to it: local first, upstream for the rest.
- A stranger with a GitHub key enrols in a public service, and with billing on
  is charged for it.

❓ **Encryption in PoC** — read here as *no encrypted sessions at all*, bodies
plaintext. If it meant only that tokens are not encrypted at rest, PoC grows
an AEAD session. *Settled by:* owner.

❓ **Push adapters in PoC** — "claude-cli talks to codex-cli" needs a live
session to receive, which is the Channels and App Server adapters. Pull-only
through an MCP inbox would be a smaller PoC and a weaker demonstration.
*Settled by:* owner.
