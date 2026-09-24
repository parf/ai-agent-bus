# Layers and modules

📌 **TL;DR:** Core decides; ports isolate dependencies; faces translate
requests. Protocol and ports sit inward, storage, directory, signature and
sandbox implementations sit behind their ports, and command entry points
assemble them. Core never imports an adapter.

## Status

The table below describes built code. Future packages and client-library plans
are not dependencies of the current system.

## The rule

Protocol and ports sit inward; core owns registry, queues and access decisions.
Storage, directory, signature and sandbox implementations sit behind their
ports. Command entry points assemble them. HTTP faces translate requests and
clients may compose operations without deciding registry authority.

Core never imports an adapter. External I/O needed by core goes through a port.
Process startup and listener management belong to the command that assembles
the runtime, not to the domain core.

## 0.7 implementation requirements

Requirements the [0.7 work](../Plans/MVP/0.7.0-TODO.md#verification) was built to, and later work keeps:

- Keep authority, validation and lifecycle rules in core, shared by every face
  and backend. Share list mechanics without merging distinct permission rules.
- Evolve small, consumer-driven persistence ports for actual operations.
  Adapters own SQL, driver errors and database locking; commands select them.
  Add abstractions for shared behavior or replaceable dependencies, not one
  interface per type or a generic CRUD framework.
- Management writes persist affected state rather than rewriting the registry
  and queue backlog. Ownership transfer is one ordinary committed record update,
  followed by atomic publication of the new complete in-memory view; it neither
  reloads storage nor rewrites unrelated durable records. Agent/record removal
  is the explicit multi-record exception because it clears every stored
  reference to the removed name in one transaction.
- Serve registry and authority reads from the loaded memory view. Maintain
  indexes where measured access patterns justify them; keep invalidation with
  the corresponding state change.
- Measure latency, allocations, write volume and lock contention before and
  after storage changes. Use evidence to choose optimizations while preserving
  authorization and commit-before-publication guarantees.

### Statistics persistence

Counters and last-use timestamps update in memory, without a database write
per request, message or increment, and are coalesced into one batch every
minute, skipping unchanged values and empty batches. Built for queue counters
in 0.7.1, credential last use in 0.7.6 and the PubSub routing counters in
0.7.12. Runtime-only metrics
stay in memory. Persisted statistics flush on graceful shutdown; a crash may
lose updates since the last successful flush. Retain dirty updates after failed
flushes and preserve increments arriving during a flush. Retry on the next
scheduled flush without double-counting already committed values.

Data changes persist immediately through write-through persistence. Queue
contents and their `in`, `out`, `dropped`, and `expired` counters form queue
state: they flush together every minute and on graceful shutdown, preserving
the [durability boundary](04-messaging.md#durability) and the distinction
between a drained queue and a never-used one without a write per message.
Other statistics batches persist [PubSub router counters](constitution.md#pubsub-routing)
and may update last-use timestamps; routing counters do not create a PubSub queue. Neither batch may
rewrite policy or credential material.

## Modules

| Built area | Responsibility |
|---|---|
| `internal/protocol` | Names, records, envelopes and JSON representation |
| `internal/ports` | Credential storage, snapshot, directory, signature and sandbox interfaces |
| `internal/core` | Registry, queues, ownership, ACL, enrolment and counters |
| `internal/auth` | Tokens and browser sessions |
| `internal/store`, `internal/dump` | Text-file credentials, memory test store and JSON snapshots |
| `internal/directory`, `internal/signature` | Public-key lookup and system signature verification |
| `internal/sandbox` | Current confinement backend |
| `internal/api` | HTTP routes, credentials and errors |
| `internal/version`, `internal/proctitle` | Shared program version, build stamp and process titles |
| `cmd/`, `mcp/` | Program entry points and client faces |

## Languages

Go implements the daemon, CLI and administrative programs. TypeScript on bun
implements the MCP face and push adapters. The process-title helper needs cgo;
[setup § build information](09-setup.md#build-information) owns build requirements.
This is not a claim that the executable has no native dependencies.

## External tools

Prefer the language's built-in, then an existing system tool for occasional
work, then an established library for work that must stay in-process. Never
write custom cryptographic primitives or invent a new protocol for convenience.

### Our own small module

Small shared plumbing belongs in an internal module when repeated code has
established a common responsibility. A dependency is not needed for a line
reader or a pending-request map. This exception does not extend to crypto or
wire-format invention.

### HTTP is built in

Use the built-in HTTP client for API and public-key requests. A shell script
behind an agent may use `curl`, because that is its ordinary client.

### What we do shell out to

| Built operation | System tool |
|---|---|
| Sign or verify possession | `ssh-keygen` |
| Optional script confinement | `systemd-run --user` |
| Supervised web confinement | `bwrap`; [runtime requirements](09-setup.md#install) |
| Install accounts and daemon unit | Host account tools and systemd |
| Switch to the admin account | `sudo` |

## What this buys

A storage or directory implementation can be replaced behind its port. No
future backend is required to make the current boundary real; smoke checks
both the prohibited inward imports and actual adapter assembly.
