# Unassigned follow-up

## Candidates

| Candidate | What makes it actionable |
|---|---|
| OpenCode push adapter | A runtime transport spike and an owner-selected target release |
| Additional identity providers | A specific provider, enrolment use case and owner-selected scope |
| Second host sandbox backend | A host that cannot use the current backend and a verified confinement requirement |
| Optional snapshot formats and database adapters | Measured persistence need; see [storage](storage.md#storage) |

The stable proposals are indexed in [topics](README.md#topics). Assign a release only when the owner does; generic ideas remain here.

## Plugin namespace

If a Claude Code plugin named `ab` is shipped, its commands, skills and agents
use the `ab:` namespace. That namespace is not an MCP tool name. No plugin release
is committed by reserving the name.
