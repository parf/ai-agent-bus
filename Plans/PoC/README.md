# PoC — agent-bus V2

What a developer must know to work on the PoC correctly. The active plan is
[TODO.md](TODO.md); the design of record is
[docs/](../../docs/00-overview.md) and it wins on every question of substance.

## Purpose

Prove that **two agent sessions find each other and talk** over a V2 daemon.
Nothing else has to be true. The PoC is allowed to be thrown away; it is not
allowed to lie about what it demonstrated.

Scope is fixed in [stages § PoC](../../docs/12-stages.md#poc) — read it before
adding anything here.

## Target shape

```
  claude session ──┐                                  ┌── notifier-claude (bun)
                   ├─ unix socket ─→ agent-busd (Go) ─┤
  codex session  ──┘        HTTP                       └── notifier-codex   (bun)
                                   ↑
                              mcp face (bun)
```

| Part | Language | Source |
|---|---|---|
| `protocol`, `core`, `api` face, `cli` | Go | new; cribs V1 `libs/go/protocol` and `libs/go/app` |
| `mcp` face | bun / TypeScript | ported from V1 `mcp/src` |
| push adapters | bun / TypeScript | ported from V1 `notifier-claude`, `notifier-codex` |

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
| **No external broker.** The daemon is the broker | [overview](../../docs/00-overview.md) |

## What PoC deliberately does not have

Encryption (bodies are plaintext, so the daemon *can* read them — that stops
being true at MVP), AUTH, per-service ACL, persistence, sandboxing, the
dashboard, generated docs, catalog filtering, npm.

One master token per user reaches every service, and it is issued over SSH
because sshd does the authentication for free
([access § getting a token](../../docs/02-access.md#getting-a-token)).

## V1 sources

V1 is the code being ported from, not a spec: code at `/rd/service/agent-bus/`,
normative design at `/rd/vhosts/realty/Plans/PRF-25/`.

| Need | Look at |
|---|---|
| envelope, routing, wire, signing | `libs/go/protocol` |
| send, consume, discovery, dead letters | `libs/go/app` |
| CLI shape | `cli/commands.go` |
| MCP tools, inbox, registry, stats | `mcp/src` |
| Channels push | `notifier-claude` |
| App Server push | `notifier-codex` |
| the layer rules V2 inherited | `PRF-25/README.md` § Mandatory code layers |

V1 runs on NATS JetStream. **The transport does not come across** — every port
of a V1 component drops NATS and speaks the V2 socket protocol instead.

## Working rules

- Task IDs are stable and never renumbered; use them in commits.
- One wave, one deliverable; commit at the end of each, push only when asked.
- A decision taken here lands in [decisions](../../docs/decisions.md) and in
  the doc that owns it — never only in the plan.
- What the PoC teaches that contradicts the design is a **doc edit**, not a
  note in this file.
