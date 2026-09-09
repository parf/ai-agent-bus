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
- Definitions change rarely (few/week) → signed generations, master/slave,
  2+ replicas. Live state (health, stats, instances) is separate and never signed.
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

## Open questions (decide with owner before modeling)

1. **Event delivery** — events land in the discovery daemon's **in-memory
   queues** (decided). Must support, among other patterns: a named queue
   owned by one consumer with in-order delivery (V1 fixer, `v1-original.md`
   §3). Still open:
   queue naming/topic namespace for publish/consume, pull vs push to
   consumers, bounds and overflow policy.
2. **AUTH + Discovery**: one daemon with two roles, or two daemons? Leaning one.
3. **Instance identity** — `host+pid` vs persisted UUID (restart semantics).
4. **MCP method info** — store raw `tools` JSON and pass through, or validate
   at registration.
5. **Delegation** — A calls B for user U: U's key for B (per-service `aud`,
   leaning) or A's own identity.
6. **Master secret distribution** — out-of-band file (start) vs sealed per
   replica in the bundle (later).
7. **Language** — assume **Go** (owner's daemons are Go), unconfirmed.
8. Bundle gaps (`prev_gen != current`): reject or log.
9. ~~One binary or two~~ — **decided: one.** `agent-busd` is the main
   service (discovery, API, MCP, WEB) and the runner.
