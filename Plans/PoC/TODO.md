# TODO — PoC

The active plan. Stable knowledge for this project is in
[README.md](README.md), the design is [docs/](../../docs/00-overview.md), and
open questions and settled decisions live in
[decisions](../../docs/decisions.md). This file holds only what is being built
now.

**Objective**: the PoC as scoped in [stages § PoC](../../docs/12-stages.md#poc) — a
Claude Code session and a Codex session find each other and talk, through a Go
daemon, with the bun MCP face and both notifiers ported from V1.

**Next step**: A.1 — decide where the code lives, then the `protocol` package.

## Blockers

| | Blocks | Settled by |
|---|---|---|
| ❓ Where V2 code lives — this repo, or a new one next to V1 at `/rd/service/agent-bus/` | everything | owner |
| ❓ Whether the bun faces vendor V1's `libs/ts/src` or import it | C, D | first port |

## Waves

One wave, one deliverable; review and commit at the end of each.

### A — daemon answers

| ID | Task | Notes |
|---|---|---|
| A.1 | `protocol`: envelope, `user@realm` and `name@host` parsing, JSON encoding | crib V1 `libs/go/protocol` (envelope, route, wire + tests) |
| A.2 | `core`: registry of services and topics, in-memory inbox per principal | [services and topics](../../docs/03-services-and-topics.md) |
| A.3 | `api` face: unix socket and HTTP, one process | [stages § PoC](../../docs/12-stages.md#poc) |
| A.4 | tokens: one master token per user, checked on every call | [access § two parameters](../../docs/02-access.md#two-parameters) |
| A.5 | `static-token` over SSH as a forced command | [access § getting a token](../../docs/02-access.md#getting-a-token) |

**Done when**: `agent-bus status` answers over both the socket and HTTP, and a
token issued by `ssh agent-bus@<node> static-token` is accepted while a wrong
one is refused.

### B — the ten verbs

| ID | Task | Notes |
|---|---|---|
| B.1 | `register`, `ls` | records only; no health, no filtering |
| B.2 | `send`, `consume` | including a backlog read after the receiver restarts |
| B.3 | `topic create`, `publish` | both kinds — queue and pub/sub |
| B.4 | `call`, `ack`, `reply` | matched on topic + tag ([messaging § request and reply](../../docs/04-messaging.md#request-and-reply)) |

**Done when**: two shells hold a conversation — one registers a service, the
other calls it and gets the reply; a publisher emits with no service record and
a consumer reads it after being down.

### C — MCP face

| ID | Task | Notes |
|---|---|---|
| C.1 | port V1 `mcp/src` off NATS onto the socket protocol | drop the transport, keep the tool surface |
| C.2 | tools: list, send, consume, call — unfiltered | filtering is MVP ([stages § MVP](../../docs/12-stages.md#mvp)) |

**Done when**: an agent asks the MCP face "what can I use?" and can send and
consume through it.

### D — push adapters

| ID | Task | Notes |
|---|---|---|
| D.1 | port `notifier-claude` (Channels) | [runner § adapters](../../docs/08-runner-role.md#adapters) |
| D.2 | port `notifier-codex` (App Server) | same |

**Done when**: a Claude Code session and a Codex session talk to each other by
name, both directions, live.

### E — close the stage

| ID | Task |
|---|---|
| E.1 | run all four PoC criteria end to end, from a clean host |
| E.2 | doc review: what PoC taught that the design says wrongly |

**Done when**: [stages § PoC](../../docs/12-stages.md#poc) is true as written, and
anything it got wrong is fixed in the doc that owns it.

## Out of PoC

Encryption, AUTH, per-service ACL, persistence, sandboxing, the dashboard,
generated docs, catalog filtering, npm. Anything learned about them goes to the
doc that owns it, not here.
