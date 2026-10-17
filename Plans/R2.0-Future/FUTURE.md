# Unassigned follow-up

## Candidates

| Candidate | What makes it actionable |
|---|---|
| OpenCode push adapter | A runtime transport spike and an owner-selected target release |
| Additional identity providers | A specific provider, enrolment use case and owner-selected scope |
| Second host sandbox backend | A host that cannot use the current backend and a verified confinement requirement |
| Optional snapshot formats and database adapters | Measured persistence need; see [storage](storage.md#storage) |
| Per-call authority cache | A measured need: 0.7's liveness and ownership checks cost lookup and list ~0.2 µs per record ([0.7 compared](../R0.8-MVP/0.7-baseline.md#07-compared-k17)); any cache must be invalidated by every commit and never let a revoked or deactivated principal act on a stale answer |
| Lock-free reads by copy-on-write | Measured lock contention between readers and commits: a write builds a new complete view and publishes it by one pointer swap, needing persistent maps to stay cheap at 100k records; not required by the publication rule (Q106) |

The stable proposals are indexed in [topics](README.md#topics). Assign a release only when the owner does; generic ideas remain here.

## Plugin namespace

If a Claude Code plugin named `ab` is shipped, its commands, skills and agents
use the `ab:` namespace. That namespace is not an MCP tool name. No plugin release
is committed by reserving the name.
