# Agent Bus — Overview

Status: draft · Scope: main ideas only, no data models yet (deferred by owner)

Documents:
1. `00-overview.md` — goal, principles, core services, chaining, storage, trade-offs, open questions
2. `01-identity-and-auth.md` — principals, key directories, groups/ACL/roles, ownership, private config
3. `02-keys-sessions-replication.md` — access keys, encrypted sessions, signed generations, SSH admin
4. `03-services-and-discovery.md` — service kinds, personal/shared, instances, discovery, health, stats
5. `04-runner.md` — `agent-busd` supervisor: adapters, identity injection, sandboxing, in-process queue

`../HANDOFF.md` holds the full discussion record, incl. superseded ideas.
`v1-original.md` records the V1 brainstorm (Linear PRF-36) and the existing V1 code.

## Goal

Replace V1 (external broker, per-service `user → token` maps; see
`v1-original.md`) with our **own daemon** and a small set of **optional** core
services. **`agent-busd` is the broker** — registry + per-agent queues in one
process — so there is **no external broker** to run. Direct calls between
services that already know each other stay point-to-point, encrypted, with
the core needed only on first contact. Inside a service, local work uses a
bounded in-process queue (see `04-runner.md`).

Model: **Kerberos-style** — AUTH hands both parties a shared secret, then
gets out of the way.

## Principles

- **Minimum to run: `agent-busd`** — Service Discovery (registrations +
  **in-memory queues**), API, MCP server, WEB dashboards. **AUTH role off.**
  Nothing else is required.
- **AUTH is an optional role of the same daemon** — `auth: on` in config and
  a restart. It runs as its **own child process**; health-checker and stats
  are optional too. A service must work without any of them.
- **Authentication is always required** — every participant presents a
  token; there is no anonymous access. What is optional is the AUTH
  *service*. **Minimal mode**: a token set manually, at minimum
  `ENV AGENT_BUS_USER_TOKEN`, matched by the service's local mapping file →
  **zero AUTH calls**. The AUTH role is turned on only when an org wants
  central identities and derived keys.
- Ed25519 keys wherever there is a key; no passwords, no client secrets.
  In minimal mode the token *is* the identity and there is no key.
- Core is on the hot path **only once** per (user, service, epoch).
- **AUTH data** (principals, groups, ACL/roles, admin SSH keys) changes rarely
  (few/week) → signed generations, master/slave, 2+ replicas, kept in **git
  over SSH**. **Service definitions are live records in `agent-busd`**
  (token to publish, owner to change) — not in the signed bundle. Live state
  (health, stats, instances, queues) is separate and never signed.
- Policy lives in AUTH; **services only interpret**, never decide.
- Two independent controls: SSH decides *who may administer*; a signature
  decides *which config is real*.
- Maximal simplicity: one daemon, no external broker; once two parties know
  each other they may talk directly.

## Core services

**The main service is `agent-busd`** (one daemon; also the runner, see
`04-runner.md`). It supervises its own roles as **child processes** — the
runner design applied to itself. Its parts:
- **Service Discovery** — registrations of **services and topics** (anyone can
  push a description: "MySQL `xxx` on host:port"; "topic `alerts.prod`,
  pub/sub") + the in-memory queues behind them
- **API** — the wire face for agents and services
- **MCP server** — exposes the bus to agents; **generates docs/tool
  descriptions for all known services available to the calling client**
- **WEB** — fancy dashboards: registry, health, stats graphs; **child
  process under cgroup limits** (CPU/memory/pids), so a heavy dashboard
  cannot starve the bus
- **AUTH / Config** — **optional role, off by default**; **child process**
  that alone holds `master_secret` and the signed bundle, talking to the core
  over a unix socket (sshd/Postfix-style privilege separation)

| Process | Role | Optional |
|---|---|---|
| **`agent-busd` core** | discovery (services, topics, in-memory queues) · API · MCP server with generated docs · runner · supervises the children below | **no — the minimum** |
| **`agent-busd` WEB child** | dashboards; cgroup-limited | on by default, may be off |
| **`agent-busd` AUTH child** | identities, keys, groups, ACLs, roles, encrypted private configs; owns `master_secret` | **off by default — `auth: on`** |
| **Health-checker** | module of discovery; probes generic services per their hints | yes |
| **Stats** | module of discovery; in-memory ring buffers feeding the WEB child, exporters | yes |

One binary, one unit, one config dir, one CLI, one git repo. Nodes with the
AUTH role on are the AUTH replicas (2+); a laptop node never holds
`master_secret`.

## Chaining — local first, upstream for the rest

AUTH and Discovery accept an **upstream** (which may have its own). Resolution
is local file → local service → upstream → …; first hit wins, unresolved falls
through. The same rule applies to identities, ACL/roles and service lookups.

- Scopes stay where they belong: personal on the laptop, team on the team node,
  company/public upstream. Nothing is pushed up; definitions never leak upward
  (queries do).
- Each hop pins its upstream's signing key; answers arrive as signed
  generations, so a middle hop can fail to forward but cannot forge.
- Local shadows upstream by design; writes warn when they shadow.
- Namespaced ids (`team/ci`, `company/mail`) avoid most collisions.
- Upstream answers are cached with the usual epoch/gen rules; an unreachable
  upstream means "cached or unresolved", never a wrong answer.
- Derived keys need a shared `master_secret`, which does not span levels →
  cross-level access uses pairwise keys (or keys issued by the upstream itself).
- Discovery's MCP face merges levels into one view, tagged by origin.

## Storage

SQLite by default (single file, zero ops); MySQL/PostgreSQL optional behind
one thin store layer. Only instance health/stats is high-churn.

## Known trade-offs

- Consistency window: revoked access may be honored for replica poll interval
  (lagging slave) + one epoch (service cache). Optional `revoked_users` list in
  the bundle closes it for *new* sessions.
- GitHub keys are pinned at enrollment; a key deleted on GitHub stays valid
  until someone refreshes.
- Pairwise/static and local files have no central revocation or audit.
- Stats are in-memory: restart = empty window.
- `master_secret` and the offline signing key are the roots of trust.
- **No forward secrecy** (decided): session keys derive from the access key +
  nonces; a leaked long-term key exposes recorded sessions.
- Service definitions are not signed: anyone holding a valid token can
  publish a new service; only ownership guards changes.

## Decisions 2026-09-09 (owner Q&A)

| Topic | Decision |
|---|---|
| Minimal identity | In static mode **the token is the whole identity** — no Ed25519 key. Keys appear with pairwise mode or AUTH. |
| Publishing services | Needs a **token** (min `ENV AGENT_BUS_USER_TOKEN`; from AUTH when on). Anyone who can reach `agent-busd` may publish a **new** service; changing an **existing** one requires being **owner or in the owner group**. Definitions are live in `agent-busd`, not in the signed bundle. |
| One binary | `agent-busd` = discovery + API + MCP server + WEB + runner **+ AUTH as an optional role**. AUTH and WEB run as **child processes** of the core: AUTH alone holds `master_secret` (privilege separation); WEB is cgroup-limited. Turn AUTH on with `auth: on`. *(revised: was "AUTH separate, may be co-hosted")* |
| Delegation | A calls B for user U as **A + on-behalf-of U** claim; B checks A's delegation role. U's key never leaves U. |
| Forward secrecy | **No** ephemeral exchange. Accepted trade-off. |
| Consumers | **Both**: pull (long-poll/stream) by default; a consumer may register a push address. |
| Overflow | **Drop oldest**, count in stats. |
| Queues & addressing | Every agent gets its **own queue on start**. Messages carry **topic + tag**: *topic* = conversation identifier (A→B), *tag* = message id. Reply goes to sender's queue with the same topic+tag, **unless** the sender sets `reply-to: {service, topic, tag}`. |
| Delivery verbs | **`send`** = to a known receiver (`name@host`), one queue. **`publish`** = to a topic, every matching consumer. `consume` reads your own queue. |
| Topic kinds | **queue** (one consumer per message, retained until consumed or TTL) and **pub/sub** (copy to every current subscriber, no retention). A topic **declares its kind, TTL and bound at creation**. Publishing to an empty queue topic with a TTL is fine; to an empty pub/sub topic it is a no-op. |
| Topics as records | A topic is **registered like a service**: token to create, owner/owner group to change, audience-filtered in registry and MCP catalog, stats on the dashboard. Inboxes are implicit queue topics owned by their agent. |
| Instance / address | **`unique-name@host`** — instance id and address are the same string. |
| `master_secret` | Out-of-band file on each replica. |
| Wire format | **JSON**, with **msgpack** as an optional negotiated binary encoding. |
| MCP tool info | Store raw `tools` JSON, check shape only; docs generated from it. |
| Bundle gaps | **Newer generation wins**. Bundle repo in **git over SSH**; replicas pull on start; **master/slave is the default config**. |
| Shared git repo | The bundle repo is **the daemon's repo**: signed AUTH bundles in one directory (authority), unsigned registry snapshots (services, topics) in another (backup only, never authority). |
| AUTH deployment option | **Local `agent-busd` with `auth: on` + a git-over-SSH remote as backup** of the signed bundles (GitHub repo, private suggested; or the user's SSH account on another server): push on every generation, pull to bootstrap/restore. The remote is the off-site copy, never a runtime dependency. Bundle holds no secrets (pubkeys only). |
| Language | **Go** first; **bun/NPM** version later. Client libs: **Go, PHP, Rust, JS, Python**. |
| V1 leftovers | RAG, KV/DB gateways, writers: **deferred, non-core** — later as ordinary bus services. |

## Still open

1. Capability globs vs. topic+tag — **settled**: `publish:<glob>` guards
   `publish` to a topic, `consume:<glob>` decides whose queues receive it;
   `send` to a known receiver needs only permission to talk to that principal.
   `topic` on a sent message is just the conversation id.
2. ❓ Audience filtering in minimal mode (AUTH role off): per-token only?
   *Settled by:* owner decision.
3. ❓ Handshake key confirmation (detect a wrong key before data flows).
   *Settled by:* owner decision at protocol-design time.
4. ❓ GitHub `/users/<login>/keys` `last_used` field.
   *Settled by:* one `curl` against the live API from a network that can reach it.
