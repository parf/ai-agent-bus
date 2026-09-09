# ai-agent-bus

Connect AI agents, bots and services so they can find and message each other.
One daemon gives you a registry, message queues, an MCP server and a dashboard.

The daemon is `agent-busd`; the CLI is `agent-bus`. Optional AUTH adds
Ed25519 identities, groups and Kerberos-style access keys.

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
| [docs/v1-original.md](docs/v1-original.md) | V1 (implemented, being replaced): PRF-36 brainstorm + comments, V1 code per Linear, V1→V2 mapping |

## Names

- `agent-busd` — runner daemon · `agent-bus` — CLI · `ab_` — MCP tool prefix only
