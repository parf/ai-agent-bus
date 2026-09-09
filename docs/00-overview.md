# Agent Bus — Overview

Status: draft · Scope: main ideas only, no data models yet

Documents:
1. `00-overview.md` — goal, principles, core services, storage, trade-offs
2. `01-identity-and-auth.md` — principals, key directories, groups/ACL/roles, ownership, private config
3. `02-keys-sessions-replication.md` — access keys, encrypted sessions, signed generations, SSH admin
4. `03-services-and-discovery.md` — service kinds, personal/shared, instances, discovery, health, stats
5. `04-runner.md` — `agent-busd` supervisor: adapters, identity injection, sandboxing profiles

## Goal

Replace per-service `user → token` maps and the NATS bus with a small set of
**optional** core services. Everything else talks point-to-point, encrypted,
with the core needed only on first contact — or not at all.

Model: **Kerberos-style** — AUTH hands both parties a shared secret, then
gets out of the way.

## Principles

- Every core service is optional; a service must work with none of them.
- Ed25519 keys everywhere; no passwords, no client secrets.
- Core is on the hot path **only once** per (user, service, epoch).
- Definitions change rarely (few/week) → signed generations, master/slave.
  Live state (health, stats, instances) is separate and never signed.
- Policy lives in AUTH; **services only interpret**, never decide.
- Two independent controls: SSH decides *who may administer*; a signature
  decides *which config is real*.

## Core services

| Service | Role | Optional |
|---|---|---|
| **AUTH / Config** | identities, keys, groups, ACLs, roles, encrypted private configs | yes |
| **Service Discovery** | what exists, where, how to check it, who may see it; web + MCP faces | yes |
| **Health-checker** | module of discovery; probes generic services per their hints | yes |
| **Stats** | module of discovery; in-memory metrics, dashboards, exporters | yes |

All are replicated the same way (signed generations, master/slave).
AUTH and Discovery may be one daemon with two roles.

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

- Propagation lag: replica poll interval + one epoch for cached pairs.
- GitHub keys are pinned at enrollment; a key deleted on GitHub stays valid
  until someone refreshes.
- Pairwise/static and local files have no central revocation or audit.
- Stats are in-memory: restart = empty window.
- `master_secret` and the offline signing key are the roots of trust.
