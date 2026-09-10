# TODO — PoC

The active plan. Stable knowledge for this project is in
[README.md](README.md), the design is [docs/](../../docs/00-overview.md), and
open questions and settled decisions live in
[decisions](../../docs/decisions.md). This file holds only what is being built
now.

**Objective**: the PoC as scoped in [stages § PoC](../../docs/12-stages.md#poc) —
a Claude Code session and a Codex session find each other and talk, through a
Go daemon with a TypeScript MCP face.

Nothing here is a port. V1 is prior art we read, and one file we copy —
[README § V1 sources](README.md#v1-sources).

## Blockers

| | Blocks | Proposed | Settled by |
|---|---|---|---|
| ❓ Where V2 code lives | A.1 | this repo, one `src/` tree beside `docs/` | owner |

Three design questions are open in [decisions](../../docs/decisions.md) and
each has a PoC default, so none of them stops work: what `consume` does to a
message (at-most-once), what `reply <id>` resolves against (a bounded map of
recently consumed envelopes), and how a consumer names a topic (pub/sub
answers *not in PoC*).

## Waves

One wave, one deliverable; review and commit at the end of each.

### A — two shells talk

| ID | Task | Notes |
|---|---|---|
| A.1 | registry and inboxes in memory, envelope and `user@realm` / `name@host` parsing | [services and topics](../../docs/03-services-and-topics.md) |
| A.2 | one process, both listeners, the `agent-bus` binary, one token compared as a string | HTTP + JSON on socket and TCP; address and token from env |
| A.3 | `register`, `ls`, `send`, `consume`, `reply`, `status` | `reply` is a `send` to the sender with topic + tag copied — it belongs here, not in a face ([modules § languages](../../docs/10-modules.md#languages)) |

**Done when**: shell 1 registers and consumes; shell 2 sends; shell 1 replies
and shell 2 reads the reply, matched by topic + tag. Kill the consumer, send
again, restart it — the backlog arrives. A wrong token is refused.

### B — the live slice

The risky wave, second on purpose: runtime behaviour and inbox ownership are
what can sink this PoC, and a shell cannot show either.

| ID | Task | Notes |
|---|---|---|
| B.0 | **spike, one hour**: can the MCP server Codex spawns from `config.toml` reach the App Server socket? | if yes, B.3 is a mode of B.1; if no, it is a second process |
| B.1 | **new** TypeScript package, run on Node: MCP tools `ab_ls`, `ab_send`, `ab_consume`, `ab_reply` — each one `fetch` to the daemon | `ab_` is the MCP prefix and nothing else ([glossary](../../docs/glossary.md)) |
| B.2 | it registers as `<name>@<host>` on start, name from an env var, and is **the** reader of that inbox | without this, "find each other by name" has no name |
| B.3 | push modes in the same package: Claude Channels (`notifications/claude/channel`) and Codex App Server (`thread/list` newest by cwd → `turn/steer` if busy, else `turn/start`) | copy `app-server.ts`; no journal, no delivery events, no recovery |
| B.4 | loaded into Claude (`--mcp-config` + the channels flag) and Codex (`~/.codex/config.toml`) | Codex without it can only answer from a shell |

**Done when**: a Claude session and a Codex session hold a two-way exchange
live. The proof is the **correlated reply** — a transport ack says nothing
about whether the model acted
([runner § adapters](../../docs/08-runner-role.md#adapters)).

### C — the rest of the verbs

| ID | Task | Notes |
|---|---|---|
| C.1 | `call`, `ack` | `call` = send, then `consume` filtered by topic + tag. The filter is served by the daemon, so no client dispatcher ([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)) |
| C.2 | `topic create`, `publish` | queue topics deliver; `--kind pubsub` is stored and answers *not in PoC* until the owner settles who subscribes |

**Done when**: one shell calls a service another registered and gets the reply;
a publisher with no service record emits to a queue topic and a consumer that
was down reads it.

### D — close the stage

| ID | Task |
|---|---|
| D.1 | `static-token` over SSH as a forced command: a script that prints the token file the daemon already reads at start |
| D.2 | the recipe — three terminals, commands to type — plus a scripted smoke over two `agent-bus` CLIs |
| D.3 | run every criterion in [stages § PoC](../../docs/12-stages.md#poc) |

**Done when**: the smoke script exits 0 and `stages § PoC` is true as written.
Running it needs Go, bun and **both agent CLIs logged in** — not a bare host.

Driving two interactive sessions headless is a test harness, not a PoC task:
the live criterion is checked by hand, once, and the script covers the rest.

## Out of PoC

What is absent is listed once, in
[stages § PoC](../../docs/12-stages.md#poc). Anything learned about it goes to
the doc that owns it, not here.
