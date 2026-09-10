# TODO — PoC

The active plan. Stable knowledge for this project is in
[README.md](README.md), the design is [docs/](../../docs/00-overview.md), and
open questions and settled decisions live in
[decisions](../../docs/decisions.md). This file holds only what is being built
now.

**Objective**: the PoC as scoped in [stages § PoC](../../docs/12-stages.md#poc) —
a Claude Code session and a Codex session find each other and talk, through a
Go daemon with a TypeScript MCP face.

**Next step**: A.1 — decide where the code lives, then the envelope struct.

## We do not port V1

V1's code is JetStream-era: signed wires, `event_hash`, KV journals, delivery
observations, dead letters. PoC has none of that, so reading V1 to strip it
costs more than writing fresh against a daemon that speaks JSON.

| V1 | Size | What we take |
|---|---|---|
| `notifier-codex` | 2.6k LOC + 2.0k tests | `app-server.ts` only — the JSON-RPC client. It **runs on Node**: bun's WebSocket does not work against the App Server |
| `mcp/src` | 4.2k LOC | nothing — it is a services.d control plane that shells out to the V1 CLI |
| `libs/ts/src` | 730 LOC | nothing — our client is a `fetch` wrapper |
| `libs/go/protocol` | 1.8k LOC | the identifier rules; not the envelope, which carries keys and trust we do not have |
| `notifier-claude` | — | the shape: it is *already* an MCP server that also pushes `notifications/claude/channel` |

**Every row below that says "new" means new.** A task that says "port" is a
task nobody has estimated.

## Blockers

| | Blocks | Proposed | Settled by |
|---|---|---|---|
| ❓ Where V2 code lives | everything | this repo, one `src/` tree beside `docs/` | owner |
| ❓ What the listeners speak | A.3 | **HTTP + JSON on both** — unix socket and TCP, `consume` a long-poll GET, `--follow` repeated long-polls. Bun's `fetch` speaks `unix:`, so no client library is needed either side | owner — it is a transport choice, not a data model |
| ❓ What `consume` does to a message | A.2 | at-most-once for PoC, loss documented ([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)) | owner |

## Waves

One wave, one deliverable; review and commit at the end of each.

### A — two shells talk

| ID | Task | Notes |
|---|---|---|
| A.1 | envelope struct + `user@realm` / `name@host` parsing | new; take only the identifier rules from V1 |
| A.2 | registry of services and topics, in-memory inbox per principal | [services and topics](../../docs/03-services-and-topics.md) |
| A.3 | one process, both listeners, and the `agent-bus` binary that talks to them | token and address from env; no framing invented — see the blocker |
| A.4 | master token per user: read at start, checked on every call | [access § two parameters](../../docs/02-access.md#two-parameters) |
| A.5 | `register`, `ls`, `send`, `consume`, `status` | five of the ten verbs |

**Done when**: shell 1 registers and consumes; shell 2 sends and it arrives.
Kill the consumer, send again, restart it — the backlog arrives. A wrong token
is refused.

### B — the live slice

The risky wave, and it comes second on purpose: runtime incompatibility and
inbox ownership are what can sink this PoC, and neither shows up in a shell.

| ID | Task | Notes |
|---|---|---|
| B.1 | **one inbox reader** per session: it dispatches replies to waiters and pushes the rest | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| B.2 | **new** bun MCP server: `ab_ls`, `ab_send`, `ab_consume`, `ab_reply` — each one `fetch` to the daemon | `ab_` is the MCP prefix and nothing else ([glossary](../../docs/glossary.md)) |
| B.3 | same package, channel mode: push `notifications/claude/channel`; `ab_reply` keeps the reply context of the message it answers | a receiving agent that is merely *prompted* answers in its own UI — it needs the tool to answer the bus |
| B.4 | it registers as `<name>@<host>` on start, name from an env var, and is the inbox reader for that name | without this, "find each other by name" has no name |
| B.5 | Codex push: **new** script, long-poll inbox → `thread/list` newest by cwd → `turn/steer` if busy, else `turn/start`. **Runs on Node** | copy `app-server.ts`, nothing else |
| B.6 | loaded into Claude (`--mcp-config` + the channels flag) and Codex (`~/.codex/config.toml`) | Codex without it can only answer from a shell |

**Done when**: a Claude session and a Codex session hold a two-way exchange
live — send, push, answer, read — and the proof is the **correlated reply**,
not a transport ack, which says nothing about whether the model acted
([runner § adapters](../../docs/08-runner-role.md#adapters)).

### C — the rest of the verbs

| ID | Task | Notes |
|---|---|---|
| C.1 | `call`, `ack` | `call` is **client-side**: send, then wait on the inbox reader for topic + tag. No call machinery in the daemon ([messaging § request and reply](../../docs/04-messaging.md#request-and-reply)) |
| C.2 | `topic create`, `publish` — queue and pub/sub | pub/sub keeps nothing, so it is the smaller of the two; the backlog criterion is a queue topic |

**Done when**: one shell calls a service registered by another and gets the
reply matched by topic + tag; a publisher with no service record emits to a
queue topic and a consumer that was down reads it; a pub/sub topic delivers to
current subscribers and keeps nothing.

### D — close the stage

| ID | Task |
|---|---|
| D.1 | `static-token` over SSH as a forced command — a script that prints the token file |
| D.2 | smoke recipe as one script: start the daemon, write both MCP configs, launch both sessions, assert the correlated answer |
| D.3 | run every criterion in [stages § PoC](../../docs/12-stages.md#poc) from a fresh clone |
| D.4 | doc review: what the PoC taught that the design says wrongly |

**Done when**: the script exits 0 and `stages § PoC` is true as written.
"Fresh clone" means Go and bun installed and **both agent CLIs logged in** —
not a bare host.

## Out of PoC

Encryption, AUTH, per-service ACL, persistence, sandboxing, the dashboard,
generated docs, catalog filtering, npm. Anything learned about them goes to the
doc that owns it, not here.
