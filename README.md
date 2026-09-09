# ai-agent-bus

Design notes for **agent-bus** — a minimal, optional set of core services
(AUTH/Config, Service Discovery, Health, Stats) plus the `agent-busd` runner,
with Ed25519 identity, Kerberos-style access keys and point-to-point
encrypted sessions. No broker.

Status: design draft, main ideas only.

Start with [HANDOFF.md](HANDOFF.md) — full discussion record: decisions, open
questions, superseded ideas. `docs/` is the terse distilled version.

## Docs

| File | Covers |
|---|---|
| [docs/00-overview.md](docs/00-overview.md) | goal, principles, core services, chaining, storage, trade-offs, open questions |
| [docs/01-identity-and-auth.md](docs/01-identity-and-auth.md) | principals, GitHub/LDAP key directories, groups/ACL/roles, ownership, private config |
| [docs/02-keys-sessions-replication.md](docs/02-keys-sessions-replication.md) | access-key modes, encrypted sessions, signed generations, SSH admin |
| [docs/03-services-and-discovery.md](docs/03-services-and-discovery.md) | service kinds, personal/shared, instances, discovery, health, stats |
| [docs/04-runner.md](docs/04-runner.md) | `agent-busd` supervisor: adapters, identity injection, sandboxing, in-process queue |
| [docs/v1-original.md](docs/v1-original.md) | historical: PRF-36 brainstorm + comments, existing V1 code per Linear, V1→V2 mapping |

## Names

- `agent-busd` — runner daemon · `agent-bus` — CLI · `ab_` — MCP tool prefix only
