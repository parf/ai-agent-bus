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
  claude session ──[ agent-bus mcp + channel push (bun) ]──┐
                                                           ├─→ agent-busd (Go)
  codex session  ──[ agent-bus mcp (bun) ]─────────────────┤    socket + HTTP
                   [ codex push (Node)   ]─────────────────┘
```

One package per session, not one per job: V1's `notifier-claude` is already an
MCP server that also pushes, and process placement is a runtime choice, not a
layer boundary ([modules § languages](../../docs/10-modules.md#languages)).

| Part | Language | Source |
|---|---|---|
| `protocol`, `core`, `api` face, `cli` | Go | **new**; V1's protocol carries keys, trust and `event_hash` we do not have — take the identifier rules only |
| `mcp` face + Claude push | TypeScript, built by bun | **new**; V1 `notifier-claude` shows the shape — an MCP server that also pushes |
| Codex push | TypeScript, built by bun, **run on Node** | **new**; takes `app-server.ts` from V1 and nothing else |

Why the split: [modules § languages](../../docs/10-modules.md#languages).

## Contracts that hold even in PoC

These are the ones a shortcut would quietly break.

| Invariant | Where it is stated |
|---|---|
| A call carries **exactly two parameters** — `user@realm` and a token; the socket supplies them, it does not remove them | [access § two parameters](../../docs/02-access.md#two-parameters) |
| **The name is the identity.** No provider numeric id is ever a principal id | [identity § names](../../docs/01-identity.md#names) |
| A **reply matches on topic + tag**; the bus adds no call machinery | [messaging § request and reply](../../docs/04-messaging.md#request-and-reply) |
| **`ack` = got it, `done` = finished**, both emitted by the receiver | [messaging § receipts](../../docs/04-messaging.md#receipts) |
| **Dependencies point inward**; only adapters touch the outside world, and no verb exists only in a face | [modules § the rule](../../docs/10-modules.md#the-rule) |
| **An inbox has one reader**, which dispatches to waiters and pushes the rest | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| **A transport ack is not "the model acted"** — only a correlated reply is | [runner § adapters](../../docs/08-runner-role.md#adapters) |
| **No external broker.** The daemon is the broker | [overview](../../docs/00-overview.md) |

## What PoC deliberately does not have

Encryption (bodies are plaintext, so the daemon *can* read them — that stops
being true at MVP), AUTH, per-service ACL, persistence, sandboxing, the
dashboard, generated docs, catalog filtering, npm.

One master token per user reaches every service, and it is issued over SSH
because sshd does the authentication for free
([access § getting a token](../../docs/02-access.md#getting-a-token)).

## V1 sources

V1 is **prior art to read, not a codebase to inherit**: code at
`/rd/service/agent-bus/`, normative design at
`/rd/vhosts/realty/Plans/PRF-25/`.

| To learn | Read |
|---|---|
| identifier rules | `libs/go/protocol` |
| what the core verbs had to handle in production | `libs/go/app` |
| CLI shape | `cli/commands.go` |
| what an MCP tool surface looks like | `mcp/src` — for shape only; it is a control plane that shells out to the V1 CLI |
| Channels push, and MCP-server-plus-push in one process | `notifier-claude/server.ts` |
| App Server JSON-RPC client — the one file we copy | `notifier-codex/app-server.ts` |
| the layer rules V2 inherited | `PRF-25/README.md` § Mandatory code layers |

V1 runs on NATS JetStream, and its components carry the machinery that came
with it: signed wires, `event_hash`, KV journals, `ADAPTER_PENDING` recovery,
delivery observations, a PHP handoff socket. **None of that exists in V2.**
Reading a V1 component to strip it costs more than writing the small thing
fresh — so nothing here is a "port" except `app-server.ts`.

## Working rules

- Task IDs are stable and never renumbered; use them in commits.
- One wave, one deliverable; commit at the end of each, push only when asked.
- A decision taken here lands in [decisions](../../docs/decisions.md) and in
  the doc that owns it — never only in the plan.
- What the PoC teaches that contradicts the design is a **doc edit**, not a
  note in this file.
