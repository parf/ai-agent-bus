## PoC

Prove that two agent sessions can find each other and talk. Nothing else has
to be true.

**Build**

| | |
|---|---|
| daemon | one process, listening on a **unix socket and HTTP** |
| language | daemon and CLI in **Go**; the MCP face and adapters in **TypeScript on bun** ([modules § languages](../../../docs/10-modules.md#languages)) |
| identity | **one master token**, reaching every service — no per-service anything ([identity § acl](../../../docs/02-access.md#acl)). It authenticates *the host's owner*, who may use any name: PoC is single-tenant, and per-user tokens arrive with the per-user sockets at MVP |
| tokens | issued **over SSH** — `ssh agent-busd@<node> token` ([access § getting a token](../../../docs/02-access.md#getting-a-token)). Kept even in PoC because it **costs us nothing**: sshd does the authentication against a key the user already has, and our side is a forced command |
| encryption | **none — no sessions at all.** Bodies travel plaintext, the `encryption: off` path the design already has for development ([access § encrypted sessions](../../../docs/02-access.md#trust-boundary)) |
| services | **basic request/reply** ([messaging § request and reply](../../../docs/04-messaging.md#request-and-reply)): a service consumes its inbox, `ack`s it (got it), does the work and replies; a caller sends and waits for that reply. The reply stands in for `done`, which comes at MVP |
| services from scripts | `agent-bus start <name> --algo=json\|args <script> [-N]` — a shell script becomes a service, `-N` of them running at once, no bus code inside it ([runner § script services](../../../docs/08-runner-role.md#script-services)) |
| mcp | **basic MCP face** — list what is registered, send, consume. Unfiltered: the master token sees everything ([discovery § faces](../../../docs/05-discovery.md#faces)) |
| install | run the built binary. **No npm** |
| setup | one thing only: the user's pubkey in the `agent-busd` account's `authorized_keys` behind that forced command |
| storage | memory. A restart loses everything, and that is fine here |
| exposure | **loopback or an SSH tunnel only.** Plaintext bodies and a master token are acceptable in a PoC; putting them on a public interface is not |

**CLI — the basic set**

| Verb | Does |
|---|---|
| `agent-bus register <name> [--addr …] [--protocol …]` | put a service or agent in the registry; `--protocol` says how to call it when that is not through the bus ([services § how to call it](../../../docs/03-services-and-topics.md#how-to-call-it)) |
| `agent-bus ls [<name>] [--kind …]` | what is registered, or one service — and whether anything is actually serving it ([discovery § what a listing answers](../../../docs/05-discovery.md#what-a-listing-answers)) |
| `agent-bus send <name> [--topic] [--tag]` | one message to one receiver |
| `agent-bus call <name> [--topic] [--tag]` | send and wait for the reply |
| `agent-bus publish --topic <t>` | one message to a topic |
| `agent-bus consume [--follow]` | read my own queue |
| `agent-bus ack <message-id>` | receipt: got it ([messaging § receipts](../../../docs/04-messaging.md#receipts)) |
| `agent-bus reply <message-id>` | answer something this client consumed — routed back by topic + tag ([messaging § reply routing](../../../docs/04-messaging.md#reply-routing)) |
| `agent-bus topic create <t> --kind queue\|pubsub` | a topic to publish into |
| `agent-bus start <name> --algo=json\|args <script> [-N]` | publish a shell script as a service, `-N` at a time ([runner § script services](../../../docs/08-runner-role.md#script-services)) |
| `agent-bus status` | is the daemon up, who is connected |
| `agent-bus service-template <template/instance@realm> [-]` | configure a template into a service, or print that configuration ([services § configuring a template](../../../docs/03-services-and-topics.md#configuring-a-template)) |

Everything else (`keygen`, `auth *`) waits — `start` is here without
supervision or restart.

**A queue topic is an inbox with a name**, so `consume --topic <t>` reads it —
an option, not a verb of its own.

**Pub/sub is MVP.** Not because fan-out is hard, but because *subscriber* is
undefined without an ACL: the design subscribes through the `consume:<glob>`
capability, and PoC has no capabilities. PoC stores `--kind pubsub` on the
record and answers *MVP* on publish, rather than inventing a second meaning of
subscription that would have to be unpicked later. MVP settles it — a
subscriber is a registered name and the copy lands in its own inbox
([messaging § subscribers](../../../docs/04-messaging.md#subscribers)).

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
([runner § adapters](../../../docs/08-runner-role.md#adapters)) — are what let a live session
*receive* instead of poll. Legacy-V1 has both, and they are **prior art, not code we
inherit**: they carry the signed wires, journals and delivery observations
JetStream needed and V2 does not. PoC writes its own, small, then measures
itself against Legacy-V1 — which ran in production and hit the cases a fresh
implementation has not thought of — and takes Legacy-V1's solution wherever that is
the better one.

**Deliberately absent**: encryption, AUTH, per-service ACLs, persistence,
sandboxing, restart policy, the dashboard, generated docs and catalog
filtering, npm.

What plaintext costs: in PoC the daemon, its logs and anyone on the host
can read message bodies, so *"the bus never reads payloads"* is not yet true.
It is not true at MVP either — that stage does not claim it ([access §
encrypted sessions](../../../docs/02-access.md#trust-boundary)); the keys that make it
true are R1.
