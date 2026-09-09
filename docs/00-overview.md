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

Replace V1 (broker-based, per-service `user → token` maps; see `v1-original.md`)
with our **own daemon** and a small set of **optional** core services. No
broker: everything talks point-to-point, encrypted, with the core needed only
on first contact — or not at all. Inside a service, queuing is a bounded
in-process queue (see `04-runner.md`).

Model: **Kerberos-style** — AUTH hands both parties a shared secret, then
gets out of the way.

## Principles

- **Minimum to run: `agent-busd`** — Service Discovery (registrations +
  **in-memory queues**), API, MCP server, WEB dashboards. **No AUTH in it.**
  Nothing else is required.
- **AUTH is a separate, optional service**; health-checker and stats are
  optional too. A service must work without any of them.
- **Authentication is always required** — every participant presents a
  token; there is no anonymous access. What is optional is the AUTH
  *service*. **Minimal mode**: a token set manually, at minimum
  `ENV AGENT_BUS_USER_TOKEN`, matched by the service's local mapping file →
  **zero AUTH calls**. The AUTH service is added only when an org wants
  central identities and derived keys.
- Ed25519 keys everywhere; no passwords, no client secrets.
- Core is on the hot path **only once** per (user, service, epoch).
- **AUTH data** (principals, groups, ACL/roles, admin keys) changes rarely
  (few/week) → signed generations, master/slave, 2+ replicas, kept in **git
  over SSH**. **Service definitions are live records in `agent-busd`**
  (token to publish, owner to change) — not in the signed bundle. Live state
  (health, stats, instances, queues) is separate and never signed.
- Policy lives in AUTH; **services only interpret**, never decide.
- Two independent controls: SSH decides *who may administer*; a signature
  decides *which config is real*.
- Maximal simplicity: once discovered, a broker is useless.

## Core services

**The main service is `agent-busd`** (one daemon; also the runner, see
`04-runner.md`). Its parts:
- **Service Discovery** — registrations (anyone can push a description:
  "MySQL `xxx` on host:port") + in-memory queues
- **API** — the wire face for agents and services
- **MCP server** — exposes the bus to agents; **generates docs/tool
  descriptions for all known services available to the calling client**
- **WEB** — fancy dashboards: registry, health, stats graphs

| Service | Role | Optional |
|---|---|---|
| **`agent-busd`** (main service) | discovery (registrations, in-memory queues) · API · MCP server with generated docs · WEB dashboards · runner | **no — the minimum** |
| **AUTH / Config** | identities, keys, groups, ACLs, roles, encrypted private configs | yes |
| **Health-checker** | module of discovery; probes generic services per their hints | yes |
| **Stats** | module of discovery; in-memory ring buffers, **own dashboard** (graphs per service / server / user / …), exporters | yes |

All are replicated the same way (signed generations, master/slave).
AUTH may be co-hosted in the same process as Discovery as an optional role
(leaning yes, open) — but the minimal daemon runs without it.

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
| Publishing services | Needs a **token** (min `ENV AGENT_BUS_USER_TOKEN`; from AUTH when on). Anyone who can reach `agent-busd` may publish a **new** service; changing an **existing** one requires being **owner or in the owner group**. Definitions are live in `agent-busd`, not in the signed bundle. |
| One binary | `agent-busd` = discovery + API + MCP server + WEB + runner. AUTH separate, optional, may be co-hosted. |
| Delegation | A calls B for user U as **A + on-behalf-of U** claim; B checks A's delegation role. U's key never leaves U. |
| Forward secrecy | **No** ephemeral exchange. Accepted trade-off. |
| Consumers | **Both**: pull (long-poll/stream) by default; a consumer may register a push address. |
| Overflow | **Drop oldest**, count in stats. |
| Queues & addressing | Every agent gets its **own queue on start**. Messages carry **topic + tag**: *topic* = conversation identifier (A→B), *tag* = message id. Reply goes to sender's queue with the same topic+tag, **unless** the sender sets `reply-to: {service, topic, tag}`. |
| Instance / address | **`unique-name@host`** — instance id and address are the same string. |
| `master_secret` | Out-of-band file on each replica. |
| Wire format | **JSON**, with **msgpack** as an optional negotiated binary encoding. |
| MCP tool info | Store raw `tools` JSON, check shape only; docs generated from it. |
| Bundle gaps | **Newer generation wins**. Bundle repo in **git over SSH**; replicas pull on start; **master/slave is the default config**. |
| AUTH deployment option | **Local AUTH server + private GitHub repo as backup** of the signed bundles: push on every generation, pull to bootstrap/restore. GitHub is the off-site copy, never a runtime dependency. |
| Language | **Go** first; **bun/NPM** version later. Client libs: **Go, PHP, Rust, JS, Python**. |
| V1 leftovers | RAG, KV/DB gateways, writers: **deferred, non-core** — later as ordinary bus services. |

## Still open

1. Capability globs (`publish:<glob>` / `consume:<glob>`) vs. the topic+tag
   scheme — are topics ACL'd, and how do they map to per-agent queues?
2. Audience filtering in minimal mode (no AUTH): per-token only?
3. Handshake key confirmation (detect a wrong key before data flows).
4. GitHub `/users/<login>/keys` `last_used` field — re-verify against the live API.
