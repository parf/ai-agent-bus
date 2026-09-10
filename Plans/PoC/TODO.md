# TODO — PoC

The active plan. Stable knowledge for this project is in
[README.md](README.md), the design is [docs/](../../docs/00-overview.md), and
open questions and settled decisions live in
[decisions](../../docs/decisions.md). This file holds only what is being built
now.

**Objective**: the PoC as scoped in [stages § PoC](../../docs/12-stages.md#poc) —
a Claude Code session and a Codex session find each other and talk, through a
Go daemon with a TypeScript MCP face.

**Write the small version first, then compare with V1 and take its solution
where it is better** — simplicity breaks the tie, and complexity is paid for
by a case V1 actually hit, not by V1 having it:
[README § V1 is the bar](README.md#v1-is-the-bar).

## Blockers

None. Code lives in this repo, in `src/` beside `docs/`; `consume` is
at-most-once; the daemon keeps no reply state and a client replies from the
envelope it consumed ([decisions](../../docs/decisions.md)).

## Waves

One wave, one deliverable; review and commit at the end of each.

### A — two shells talk

| ID | Task | Notes |
|---|---|---|
| A.1 | `src/` tree, then registry and inboxes in memory, envelope and `user@realm` / `name@host` parsing | layout from [modules § modules](../../docs/10-modules.md#modules); records from [services and topics](../../docs/03-services-and-topics.md) |
| A.2 | one process, both listeners, the `agent-bus` binary. **Every client carries a name and a token**: the daemon reads its token from a file at start, the CLI reads name, token and address from env | the name is the inbox it owns and the sender a reply returns to ([access § two parameters](../../docs/02-access.md#two-parameters)) — a client with only a token can send and never be answered |
| A.3 | `status`, `register`, `ls`, `send`, `consume` (with the **topic + tag filter**), `reply` | `reply` is a `send` to the sender with topic + tag copied — it belongs here, not in a face ([modules § languages](../../docs/10-modules.md#languages)). `status` first: it is the first thing that runs |

**Done when**: shell 1 registers and consumes; shell 2 sends; shell 1 replies
and shell 2 reads the reply, matched by topic + tag. Kill the consumer, send
again, restart it — the backlog arrives. A wrong token is refused.

### B — the live slice

The risky wave, second on purpose: runtime behaviour and inbox ownership are
what can sink this PoC, and a shell cannot show either.

| ID | Task | Notes |
|---|---|---|
| B.0 | **spike, one hour**: start `codex app-server --listen unix://…` beside the session, then check whether the MCP server Codex spawned from `config.toml` can reach that socket | nothing creates the socket on its own — V1's launcher does it. If the MCP process can reach it, B.3 is a mode of B.1; if not, it is a second process |
| B.1 | **new** TypeScript package, run on Node: MCP tools `ab_ls`, `ab_send`, `ab_consume`, `ab_reply` — each one `fetch` to the daemon | `ab_` is the MCP prefix and nothing else ([glossary](../../docs/glossary.md)) |
| B.2 | it registers as `<name>@<host>` on start, name from an env var, and is **the** reader of that inbox | without this, "find each other by name" has no name |
| B.3 | push modes in the same package: Claude Channels (`notifications/claude/channel`) and Codex App Server (`thread/list` newest by cwd → `turn/steer` if busy, else `turn/start`) | start with our own small JSON-RPC client — no journal, no delivery events, no recovery. Compare against V1 on idle vs busy, thread changes, disconnect and runtime rejection, and take what earns its keep |
| B.4 | loaded into Claude (`--mcp-config` + the channels flag) and Codex (`~/.codex/config.toml`), with the App Server started beside the Codex session and its socket path passed in | Codex without the MCP server can only answer from a shell; without the socket, nothing can push into it |

**Done when**: a Claude session and a Codex session hold a two-way exchange
live. The proof is the **correlated reply** — a transport ack says nothing
about whether the model acted
([runner § adapters](../../docs/08-runner-role.md#adapters)).

### C — the rest of the verbs

| ID | Task | Notes |
|---|---|---|
| C.1 | `call`, `ack` | `call` = send, then the A.3 filter. The daemon serves it, so no client dispatcher ([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)) |
| C.2 | `topic create`, `publish`, and `consume --topic <t>` | queue kind only: a queue topic is an inbox with a name. `--kind pubsub` is stored and publish to it answers *MVP* ([stages § PoC](../../docs/12-stages.md#poc)) |

| C.3 | `start <name> --algo=std\|args <script>` and the JSON form on stdin | spawn per message, envelope in the environment, stdout is the reply, exit non-zero means none. No sandbox, no restart ([runner § script services](../../docs/08-runner-role.md#script-services)) |

**Done when**: one shell calls a service another registered and gets the reply;
a publisher with no service record emits to a queue topic, and a consumer that
was down reads it afterwards with `consume --topic`; and `echo "Hello $1"` in a
file, started with one command, answers a `call` from another shell.

### D — close the stage

| ID | Task |
|---|---|
| D.1 | `static-token` over SSH as a forced command: a script that prints the token file the daemon already reads at start |
| D.2 | the recipe — three terminals, commands to type — plus a scripted smoke over two `agent-bus` CLIs and the hello-world script service |
| D.3 | run every criterion in [stages § PoC](../../docs/12-stages.md#poc) |

**Done when**: the smoke script exits 0, `stages § PoC` is true as written, and
each component with a V1 counterpart has its comparison written down — what we
took, what we skipped, why.
Running it needs Go, bun and Node, and **both agent CLIs logged in** — not a
bare host.

Driving two interactive sessions headless is a test harness, not a PoC task:
the live criterion is checked by hand, once, and the script covers the rest.

## Out of PoC

What is absent is listed once, in
[stages § PoC](../../docs/12-stages.md#poc). Anything learned about it goes to
the doc that owns it, not here.
