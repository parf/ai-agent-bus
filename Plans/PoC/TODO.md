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

**Next step**: C.1 — `call` and `ack`.

## Blockers

None. Code is in `src/`; `consume` is at-most-once; the daemon keeps no reply
state and a client replies from the envelope it consumed
([decisions](../../docs/decisions.md)).

## Waves

One wave, one deliverable; review and commit at the end of each.

### A — two shells talk

_Done → [DONE.md](DONE.md): A_

### B — the live slice

The risky wave, second on purpose: runtime behaviour and inbox ownership are
what can sink this PoC, and a shell cannot show either.

| ID | Task | Notes |
|---|---|---|
| B.0 | ✅ _Done → [DONE.md](DONE.md): B.0_ — bun drives `codex app-server` over stdio NDJSON; no WebSocket, no `ws`, one runtime |
| B.1 | ✅ _Done → [DONE.md](DONE.md): B_ — `src/mcp/` on bun, four tools, each one `fetch` |
| B.2 | ✅ _Done → [DONE.md](DONE.md): B_ — registers `AGENT_BUS_NAME` at start |
| B.3 | ⚠️ _Done → [DONE.md](DONE.md): B_ — both modes built and covered by smokes of their own, the Codex one against a fake App Server. **Neither push proven into a live interactive session**: channels did not surface in a headless Claude, and steering a real TUI was not attempted. Both are the by-hand criterion |
| B.4 | ✅ _Done → [DONE.md](DONE.md): B.4_ — plugin manifest, `.mcp.json` fallback, Codex config; the name is derived when unset |
| B.5 | ⚠️ _Done → [DONE.md](DONE.md): B.5_ — `/ab:ls` and `/ab:send`. The SessionStart hook was dropped: the face already registers itself |
| B.4 | loaded into Claude as a **plugin** `ab` (`claude plugin init ab` scaffolds locally and auto-loads; its `plugin.json` declares the MCP server) and into Codex via `~/.codex/config.toml`, with the App Server started beside the Codex session and its socket path passed in | Codex without the MCP server can only answer from a shell; without the socket, nothing can push into it. `--mcp-config` is the fallback if the plugin path costs anything |
| B.5 | if the plugin carries its weight: `/ab:send`, `/ab:ls` as commands, and a SessionStart hook that registers `<name>@<host>` — which is B.2 done by the runtime | plugin commands and skills use the colon namespace; **MCP tools cannot** ([glossary](../../docs/glossary.md)) |

**Done when**: a Claude session and a Codex session hold a two-way exchange
live. The proof is the **correlated reply** — a transport ack says nothing
about whether the model acted
([runner § adapters](../../docs/08-runner-role.md#adapters)).

⚠️ **Not yet met.** Everything below it is: the tools, self-registration, both
push paths, the plugin. What is missing is one run with two *interactive*
sessions — the recipe is in [`../../src/mcp/README.md`](../../src/mcp/README.md).

### C — the rest of the verbs

| ID | Task | Notes |
|---|---|---|
| C.1 | `call`, `ack` | `call` = send, then the A.3 filter. The daemon serves it, so no client dispatcher ([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)) |
| C.2 | `topic create`, `publish`, and `consume --topic <t>` | queue kind only: a queue topic is an inbox with a name. `--kind pubsub` is stored and publish to it answers *MVP* ([stages § PoC](../../docs/12-stages.md#poc)) |

| C.3 | `start <name> --algo=std\|args <script> [-N]` and the JSON form on stdin | spawn per message up to N at once, envelope in the environment, stdout is the reply, exit non-zero means none. The `start` process is the inbox's one reader; the scripts never see the bus. No sandbox, no restart ([runner § script services](../../docs/08-runner-role.md#script-services)) |

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
Running it needs Go, bun and **both agent CLIs logged in** — not a bare host.

Driving two interactive sessions headless is a test harness, not a PoC task:
the live criterion is checked by hand, once, and the script covers the rest.

## Out of PoC

What is absent is listed once, in
[stages § PoC](../../docs/12-stages.md#poc). Anything learned about it goes to
the doc that owns it, not here.
