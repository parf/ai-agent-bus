# Stages

What gets built when, and what counts as done. Each stage ends with something
that **works end to end** — not a layer that waits for the next one.

PoC is the owner's. MVP and Release 1 are proposed and want a cut.

| | PoC | MVP | Release 1 |
|---|---|---|---|
| Identity | one master token, issued over SSH | `user@realm` + token, per-user sockets | groups, roles, delegation |
| Identity enrolment | none — the token is everything | manual + GitHub | AUTH bundle, LDAP/AD if wanted |
| Queues | bounded, and a record says whether a full one refuses or rings ([messaging § overflow](04-messaging.md#overflow)) | TTL, `reply-to` | — |
| Encryption | **none** | AEAD sessions, bodies end to end | — |
| Access control | master token reaches everything | service ACL + master ACL | expressions over groups |
| Storage | memory only | SQLite + Parquet dumps | git snapshots, peer sync |
| Processes | one | supervisor + children | AUTH child |
| Services | request/reply with `ack`; queue topics; scripts as services; a template configured into a service; a record says how to call it and whether anyone is serving it | pub/sub; deadlines, `done`, `reply-to`, several workers behind one name | calls across chained buses; pools across hosts; many names addressed together, scatter-gather |
| Faces | CLI + basic MCP | MCP with generated docs, filtered; dashboard | — |
| Install | built Go binary; bun runs the faces | `npm install` + `sudo agent-bus-setup` | packaged, zero-downtime reload |

## PoC

Prove that two agent sessions can find each other and talk. Nothing else has
to be true.

**Build**

| | |
|---|---|
| daemon | one process, listening on a **unix socket and HTTP** |
| language | daemon and CLI in **Go**; the MCP face and adapters in **TypeScript on bun** ([modules § languages](10-modules.md#languages)) |
| identity | **one master token**, reaching every service — no per-service anything ([identity § acl](01-identity.md#acl)). It authenticates *the host's owner*, who may use any name: PoC is single-tenant, and per-user tokens arrive with the per-user sockets at MVP |
| tokens | issued **over SSH** — `ssh agent-busd@<node> static-token` ([access § getting a token](02-access.md#getting-a-token)). Kept even in PoC because it **costs us nothing**: sshd does the authentication against a key the user already has, and our side is a forced command |
| encryption | **none — no sessions at all.** Bodies travel plaintext, the `encryption: off` path the design already has for development ([access § encrypted sessions](02-access.md#encrypted-sessions)) |
| services | **basic request/reply** ([messaging § request and reply](04-messaging.md#request-and-reply)): a service consumes its inbox, `ack`s it (got it), does the work and replies; a caller sends and waits for that reply. The reply stands in for `done`, which comes at MVP |
| services from scripts | `agent-bus start <name> --algo=json\|args <script> [-N]` — a shell script becomes a service, `-N` of them running at once, no bus code inside it ([runner § script services](08-runner-role.md#script-services)) |
| mcp | **basic MCP face** — list what is registered, send, consume. Unfiltered: the master token sees everything ([discovery § faces](05-discovery.md#faces)) |
| install | run the built binary. **No npm** |
| setup | one thing only: the user's pubkey in the `agent-busd` account's `authorized_keys` behind that forced command |
| storage | memory. A restart loses everything, and that is fine here |
| exposure | **loopback or an SSH tunnel only.** Plaintext bodies and a master token are acceptable in a PoC; putting them on a public interface is not |

**CLI — the basic set**

| Verb | Does |
|---|---|
| `agent-bus register <name> [--addr …] [--protocol …]` | put a service or agent in the registry; `--protocol` says how to call it when that is not through the bus ([services § how to call it](03-services-and-topics.md#how-to-call-it)) |
| `agent-bus ls [<name>] [--kind …]` | what is registered, or one service — and whether anything is actually serving it ([discovery § what a listing answers](05-discovery.md#what-a-listing-answers)) |
| `agent-bus send <name> [--topic] [--tag]` | one message to one receiver |
| `agent-bus call <name> [--topic] [--tag]` | send and wait for the reply |
| `agent-bus publish --topic <t>` | one message to a topic |
| `agent-bus consume [--follow]` | read my own queue |
| `agent-bus ack <message-id>` | receipt: got it ([messaging § receipts](04-messaging.md#receipts)) |
| `agent-bus reply <message-id>` | answer something this client consumed — routed back by topic + tag ([messaging § reply routing](04-messaging.md#reply-routing)) |
| `agent-bus topic create <t> --kind queue\|pubsub` | a topic to publish into |
| `agent-bus start <name> --algo=json\|args <script> [-N]` | publish a shell script as a service, `-N` at a time ([runner § script services](08-runner-role.md#script-services)) |
| `agent-bus status` | is the daemon up, who is connected |
| `agent-bus service-template <template/instance@host> [-]` | configure a template into a service, or print that configuration ([services § configuring a template](03-services-and-topics.md#configuring-a-template)) |

Everything else (`keygen`, `auth *`, `stop`/`logs`) waits —
`start` is here without supervision, sandboxing or restart.

**A queue topic is an inbox with a name**, so `consume --topic <t>` reads it —
an option, not a verb of its own.

**Pub/sub is MVP.** Not because fan-out is hard, but because *subscriber* is
undefined without an ACL: the design subscribes through the `consume:<glob>`
capability, and PoC has no capabilities. PoC stores `--kind pubsub` on the
record and answers *MVP* on publish, rather than inventing a second meaning of
subscription that would have to be unpicked later. MVP settles it — a
subscriber is a registered name and the copy lands in its own inbox
([messaging § subscribers](04-messaging.md#subscribers)).

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
- **`echo "Hello $1"` in a file is a service**: started with one command,
  found by another party through `ls` or the MCP catalog, and it answers.

The **push adapters** — Claude Code Channels and the Codex App Server
([runner § adapters](08-runner-role.md#adapters)) — are what let a live session
*receive* instead of poll. V1 has both, and they are **prior art, not code we
inherit**: they carry the signed wires, journals and delivery observations
JetStream needed and V2 does not. PoC writes its own, small, then measures
itself against V1 — which ran in production and hit the cases a fresh
implementation has not thought of — and takes V1's solution wherever that is
the better one.

**Deliberately absent**: encryption, AUTH, per-service ACLs, persistence,
sandboxing, restart policy, the dashboard, generated docs and catalog
filtering, npm.

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
| encryption | 🚫 *struck* — the daemon issues the token a session key derives from, so end to end against it is not reachable in this stage's key mode; the bus is trusted on its own host ([access § encrypted sessions](02-access.md#encrypted-sessions)) |
| access | service ACL, then master ACL; a service may refuse master ([identity § acl](01-identity.md#acl)) |
| messaging | TTL, `reply-to`, and **pub/sub topics** — a subscription is a `consume:<glob>` capability, which exists once there is an ACL ([messaging](04-messaging.md)) |
| services | calls grow up: `done` (finished processing) as well as `ack` (got it), caller deadlines, `reply-to` a third party, several workers behind one name, per-service call stats ([messaging](04-messaging.md)) |
| storage | SQLite store, Parquet dump and reload ([setup § storage](09-setup.md#storage)) |
| faces | the PoC MCP face grown up: generated docs, catalog filtered per caller; a dashboard people sign in to, showing the registry, stuck inboxes, exchanges, losses and refusals ([discovery § what it shows](05-discovery.md#what-it-shows)) |
| starting services | `agent-bus start <name> … <command>` — one command line publishes a service in the foreground, confined if it asks to be. The install lays out **both accounts and the directory tree**, because that is the arrangement and it is cheap; what waits is the runner program that would use the second one ([runner role](08-runner-role.md)) |
| processes | the supervisor/children split ([processes](11-processes.md)) |
| install | `npm install -g` + `sudo agent-bus-setup`: the programs it brings ([setup § the programs](09-setup.md#the-programs)), the two accounts and the tree they own ([setup § the two accounts](09-setup.md#the-two-accounts)) |

**Works at the end of MVP**

- Several users share one host, each seeing only the services they may.
- The bus restarts without losing queued messages.
- An agent asks the MCP face "what can I use?" and gets a filtered catalog.
- A service started with one command line is registered and reachable, and is
  confined when it asks to be.
- A person signs in to the dashboard with the credential they already have and
  sees the bus as they may see it — and a stranger sees only how to get one.

**Deliberately absent**: AUTH role, groups, chaining, peer sync, billing,
client libraries in other languages. The dashboard's groups, health, load
graphs, child liveness and record origin go with them — each waits on the
thing that would make it true rather than on the page
([discovery § what it shows](05-discovery.md#what-it-shows)).

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
| **the runner** | `agent-bus-runner` as its own account and its own program: installed instances under `runner/`, configuration it holds and never hands back, autostart and a restart policy, on-demand start — a wrapper over the MVP's `agent-bus start` ([runner role](08-runner-role.md)) |
| script forms | **`--algo=std`**, the body as bytes on stdin, so an image scaler is a service; **`--algo=jsonl`** and **`--algo=msgpack`**, a child kept alive across messages with a deadline and `reload`, the second carrying the envelope and a binary body in one frame ([runner § long-lived services](08-runner-role.md#long-lived-services)) |
| service pools | **`start --share`**: one name served by any number of processes on any number of hosts, passing the word `consume` already takes. The members are given a **complete name** — a realm the daemon holds, `image-scaler@pool1`, rather than each host's own ([runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts)) |
| encryption | AEAD sessions and bodies end to end, on the pairwise or derived keys that make the claim true ([access § encrypted sessions](02-access.md#encrypted-sessions)) |
| credentials | **one token per principal per service**, so a service you call cannot replay your credential at another one as you — the MVP's master token is what this replaces ([access § token scope](02-access.md#token-scope)) |
| clients | Go, PHP, Rust, JS, Python — gated on how `protocol` is specified ([modules](10-modules.md)) |
| operations | zero-downtime reload, packaging |
| fan-out | **many names addressed together**: several services sharing a template, asked at once, and the scatter-gather that needs — plus the how-to. Not the same as a **pool**, which is many processes behind *one* name and answers once ([runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts)). Deferred here on the owner's word; the PoC and MVP address one service at a time ([services § service and template](03-services-and-topics.md#service-and-template)) |

**Works at the end of Release 1**

- A company bus with central identities and groups, two AUTH replicas, and no
  service holding its own user list.
- A laptop bus chains to it: local first, upstream for the rest.
- A stranger with a GitHub key enrols in a public service.
- A host runs services through the runner with no daemon on it at all, against
  a bus somewhere else.

## Release 1.1

*Proposed.* The stage that ships **tools**, not mechanism: by here the bus, the
runner and the ACL are built, and what is missing is the set of services a
company would otherwise write itself.

The catalogue is [bundled services](13-bundled-services.md), which owns it.
What belongs in *this* stage rather than that document:

| | |
|---|---|
| why it is after Release 1 | every entry is kept by the runner, granted by a service ACL, and configured by env layers — all of which Release 1 is what builds |
| what counts as done | **no entry in the catalogue needed a change to `agent-busd`.** That is the acceptance criterion, and a failure of it is a finding about the design rather than about the tool |
| what it is not | a plugin system. Each one is a service written the ordinary way, and nothing here gives a bundled service an ability an outside one lacks |

Billing is **not in any stage**: it is designed and deferred
([future/billing.md](future/billing.md)).
