# TODO — PoC

The active plan. Stable knowledge for this project is in
[README.md](../README.md#poc--agent-bus-v2), the design is [docs/](../../../../docs/00-overview.md#overview), and
open questions and settled decisions live in
[decisions](../../../../docs/decisions.md#mvp-decisions). This file holds only what is being built
now.

**Objective**: the PoC as scoped in [stages § PoC](../scope.md#poc) —
a Claude Code session and a Codex session find each other and talk, through a
Go daemon with a TypeScript MCP face.

**Write the small version first, then compare with Legacy-V1 and take its solution
where it is better** — simplicity breaks the tie, and complexity is paid for
by a case Legacy-V1 actually hit, not by Legacy-V1 having it:
[README § Legacy-V1 is the bar](../README.md#legacy-v1-is-the-bar).

**Next step**: none here — the PoC is met. The active plan is [Plans/MVP](../../../MVP/TODO.md#todo-mvp).

## Blockers

None. Code is in `src/`; `consume` is at-most-once; the daemon keeps no reply
state and a client replies from the envelope it consumed
([decisions](../../../../docs/decisions.md#mvp-decisions)).

## Waves

One wave, one deliverable; review and commit at the end of each.

### A — two shells talk

_Done → [DONE.md](../DONE.md#done--poc): A_

### B — the live slice

The risky wave, second on purpose: runtime behaviour and inbox ownership are
what can sink this PoC, and a shell cannot show either.

| ID | Task | Notes |
|---|---|---|
| B.0 | ✅ _Done → [DONE.md](../DONE.md#done--poc): B.0_ — bun drives `codex app-server` over stdio NDJSON; no WebSocket, no `ws`, one runtime |
| B.1 | ✅ _Done → [DONE.md](../DONE.md#done--poc): B_ — `src/mcp/` on bun, four tools, each one `fetch` |
| B.2 | ✅ _Done → [DONE.md](../DONE.md#done--poc): B_ — registers `AGENT_BUS_NAME` at start |
| B.3 | ✅ _Done → [DONE.md](../DONE.md#done--poc): B_ — both modes built, covered by smokes, and **both proven into live interactive sessions** |
| B.4 | ⚠️ _Done → [DONE.md](../DONE.md#done--poc): B.4_ — plugin manifest, `.mcp.json` fallback, Codex config; the name is derived when unset. **A plugin-provided server cannot be a channel source**, so Claude push needs the `--mcp-config` form |
| B.5 | ⚠️ _Done → [DONE.md](../DONE.md#done--poc): B.5_ — `/ab:ls` and `/ab:send`. The SessionStart hook was dropped: the face already registers itself |

**Done when**: a Claude session and a Codex session hold a two-way exchange
live. The proof is the **correlated reply** — a transport ack says nothing
about whether the model acted
([runner § adapters](../../../../docs/08-runner-role.md#adapters)).

✅ **Met.** A live interactive Claude session asked a live interactive Codex
session a question over the bus and got the answer back, matched on topic and
tag ([DONE.md](../DONE.md#done--poc): the live run). Recipe:
[`../../../../src/mcp/README.md`](../../../../src/mcp/README.md#the-mcp-face).

### C — the rest of the verbs

| ID | Task | Notes |
|---|---|---|
| C.1 | ✅ _Done → [DONE.md](../DONE.md#done--poc): C_ — `call` is send plus the filtered wait; `ack` is an ordinary message on the same topic and tag |
| C.2 | ✅ _Done → [DONE.md](../DONE.md#done--poc): C_ — queue topics read as inboxes; a pub/sub topic is stored and publishing to one answers *MVP* |

| C.3 | ✅ _Done → [DONE.md](../DONE.md#done--poc): C_ — `start` is the inbox's one reader, spawns per message up to `-N`, both algos |

**Done when**: one shell calls a service another registered and gets the reply;
a publisher with no service record emits to a queue topic, and a consumer that
was down reads it afterwards with `consume --topic`; and `echo "Hello $1"` in a
file, started with one command, answers a `call` from another shell.

✅ **Met** — all three are checks in `src/smoke.sh`.

### D — close the stage

| ID | Task |
|---|---|
| D.1 | ✅ _Done → [DONE.md](../DONE.md#done--poc): D_ — `src/static-token`, the forced command; the daemon needed no change |
| D.2 | ✅ _Done → [DONE.md](../DONE.md#done--poc): D_ — [`src/README.md`](../../../../src/README.md#src); running it as written found the `start` inbox bug |
| D.3 | ✅ _Done → [DONE.md](../DONE.md#done--poc): D_ — every criterion walked, each with a check |

**Done when**: the smoke script exits 0, `stages § PoC` is true as written, and
each component with a Legacy-V1 counterpart has its comparison written down — what we
took, what we skipped, why.

✅ **Met.** `src/smoke.sh` is green, and every criterion is walked in
[DONE.md](../DONE.md#done--poc): D.
Running it needs Go, bun and **both agent CLIs logged in** — not a bare host.

Driving two interactive sessions headless is a test harness, not a PoC task:
the live criterion is checked by hand, once, and the script covers the rest.

## After the stage closed

One bug was worth reopening for, on the owner's word: a full queue lost a
message silently. ✅ _Done → [DONE.md](../DONE.md#done--poc): after the stage closed_ — the
receiver declares `--overflow`, `strict` refuses loudly by default, `ring`
counts what it drops. Sent to Codex for review.

Then two more pieces on the owner's word, each reviewed the same way:

| | |
|---|---|
| names and configuration | a service is a **configured** thing, so `template/instance@host` joins `service@host`, and `service-template` configures one. The configuration is private — only the service reads it, and every other answer carries `config_sha` instead ([services § configuring a template](../../../../docs/03-services-and-topics.md#configuring-a-template)) |
| review rounds | ✅ _Done → [DONE.md](../DONE.md#done--poc): after the stage closed_ — docs then code, three passes, Codex and Fable each time; every finding reproduced before it was accepted, and round 3 found nothing from Codex |

## Out of PoC

What is absent is listed once, in
[stages § PoC](../scope.md#poc). Anything learned about it goes to
the doc that owns it, not here.
