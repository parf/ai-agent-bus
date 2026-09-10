# PoC — agent-bus V2

What a developer must know to work on the PoC correctly. The active plan is
[TODO.md](TODO.md).

**[stages § PoC](../../docs/12-stages.md#poc) fixes the scope**; the rest of
[docs/](../../docs/00-overview.md) is the design of record and governs *how*
anything in that scope is built. Read it that way round, or persistence,
crypto and the runner arrive through the back door.

## Purpose

Prove that **two agent sessions find each other and talk** over a V2 daemon.
Nothing else has to be true. The PoC is allowed to be thrown away; it is not
allowed to lie about what it demonstrated.

Scope is fixed in [stages § PoC](../../docs/12-stages.md#poc) — read it before
adding anything here.

## Target shape

```
  claude session ──[ agent-bus mcp, channel push ]──┐
                                                    ├─→ agent-busd (Go)
  codex session  ──[ agent-bus mcp, app-server push ]┘    socket + HTTP
```

**One TypeScript package, two push modes, one process per session.** V1's
`notifier-claude` is already an MCP server that also pushes, so this is its
shape, not a new idea; process placement is a runtime choice, not a layer
boundary ([modules § languages](../../docs/10-modules.md#languages)). Whether
Codex's MCP process can reach the App Server socket is the first spike — if it
cannot, the push mode becomes its own process and nothing else changes.

| Part | Language | Source |
|---|---|---|
| `protocol`, `core`, `api` face, `cli` | Go | **new**; V1's protocol carries keys, trust and `event_hash` we do not have — take the identifier rules only |
| `mcp` face + both push modes | TypeScript, built and tested by bun, **run on Node** | **new**, all of it. V1 runs Claude's notifier on bun and Codex's on Node; we do not inherit that split either |

Why the split: [modules § languages](../../docs/10-modules.md#languages).

## Contracts that hold even in PoC

These are the ones a shortcut would quietly break.

| Invariant | Where it is stated |
|---|---|
| A call carries **exactly two parameters** — `user@realm` and a token; the socket supplies them, it does not remove them, and a client off the socket supplies both itself | [access § two parameters](../../docs/02-access.md#two-parameters) |
| **The name is the identity.** No provider numeric id is ever a principal id | [identity § names](../../docs/01-identity.md#names) |
| A **reply matches on topic + tag**; the bus adds no call machinery | [messaging § request and reply](../../docs/04-messaging.md#request-and-reply) |
| **`ack` = got it, `done` = finished**, both emitted by the receiver | [messaging § receipts](../../docs/04-messaging.md#receipts) |
| **Dependencies point inward**; only adapters touch the outside world, and no verb exists only in a face | [modules § the rule](../../docs/10-modules.md#the-rule) |
| **An inbox has one reader**; a waiter filters by topic + tag and the daemon serves the match | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| **A transport ack is not "the model acted"** — only a correlated reply is | [runner § adapters](../../docs/08-runner-role.md#adapters) |
| **No external broker.** The daemon is the broker | [overview](../../docs/00-overview.md) |

## What PoC deliberately does not have

The list lives in [stages § PoC](../../docs/12-stages.md#poc). The one
consequence worth repeating: bodies are plaintext, so the daemon *can* read
them, and "the bus never reads payloads" only becomes true at MVP.

One master token per user reaches every service, and it is issued over SSH
because sshd does the authentication for free
([access § getting a token](../../docs/02-access.md#getting-a-token)).

## V1 is the bar

**Write the small version first.** V1's components are entangled with
JetStream's signed wires, `event_hash`, KV journals, `ADAPTER_PENDING`
recovery, delivery observations and a PHP handoff socket — none of which V2
has — so starting from them costs more than starting fresh.

**Then compare, and take V1's solution where it is better.** It ran in
production; every awkward branch in it is a case somebody actually hit. If
V1's approach is better, use it — the code included, when it comes free of the
JetStream machinery.

**Simplicity breaks the tie.** Complexity has to be paid for by a real case:
*"V1 does X"* is not a reason, *"V1 does X because Y happened"* is. Skipping a
V1 behaviour is fine; skipping it without noticing is not.

| Ours | Compare against | Look for |
|---|---|---|
| envelope, names | `libs/go/protocol` | which identifier rules turned out to matter |
| core verbs | `libs/go/app` | what send, consume and discovery had to handle in production |
| CLI | `cli/commands.go` | the flags a real operator needed |
| MCP face | `mcp/src` | the tool surface agents actually used |
| Claude push | `notifier-claude/server.ts` | MCP-server-plus-push in one process; reply correlation |
| Codex push | `notifier-codex/` | the App Server call sequence; idle vs busy, thread changes, disconnects, runtime rejection |
| the layer rules V2 inherited | `PRF-25/README.md` § Mandatory code layers | |

Code at `/rd/service/agent-bus/`, normative design at
`/rd/vhosts/realty/Plans/PRF-25/`. The PoC may be smaller than V1 — it may not
be worse at what it does.

## Working rules

- Task IDs are stable and never renumbered; use them in commits.
- One wave, one deliverable; commit at the end of each, push only when asked.
- A decision taken here lands in [decisions](../../docs/decisions.md) and in
  the doc that owns it — never only in the plan.
- **Before a task with a V1 counterpart closes**, compare the two: take V1's
  solution where it is better, and record what we skip and why. Simplicity
  wins a tie.
- What the PoC teaches that contradicts the design is a **doc edit**, not a
  note in this file.
