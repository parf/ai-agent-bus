# R1.1 decisions

## Recorded decisions

Migrated 2026-09-13. Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

| Decision topic | Substance | Earlier rows |
|---|---|---|
| How to reach a person | [definition](people.md#how-to-reach-a-person) | D23, D159 |
| Service to service | [definition](access.md#service-to-service) | D62 |
| How long a record lives | [definition](records.md#how-long-a-record-lives) | D130, D131, D132 |
| Rules they all obey | [definition](services.md#rules-they-all-obey) | D145, D146, D147, D154 |
| For the agents themselves | [definition](services.md#for-the-agents-themselves) | D148, D149, D150 |
| Agent runtimes | [definition](services.md#agent-runtimes) | |
| One contract for the set | [definition](services.md#one-contract-for-the-set) | D151, D152, D153 |
| The bus watching itself | [definition](services.md#the-bus-watching-itself) | D155, D156 |
| People and the world outside | [definition](services.md#people-and-the-world-outside) | D157, D158, D167 |
| Data | [definition](services.md#data) | D160, D161, D162 |
| Scope | [definition](README.md#scope) | D163, D164 |
| Reading the box | [definition](services.md#reading-the-box) | D165 |
| Other buses | [definition](services.md#other-buses) | D166 |

## Declared record state

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-15 | Two declared states: down and retired | *Not today* and *not ever* are different instructions to a caller | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | Briefly unavailable is not settable; a service returns it over the protocol | Only the thing restarting knows it is, and a record marked so is stale the moment nobody updates it | [coming back in a moment](records.md#coming-back-in-a-moment-is-not-one-of-them) |
| 2026-09-15 | `down` is the record already called disabled, not a state beside it | One behaviour with two names would be two truths about one thing | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | No declared state answers `500` | Down is deliberate and a 500 is the daemon saying it broke; the two are opposite claims, and a 500 is uncounted as a refusal | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | Down answers `409` and retired `410`; briefly unavailable is the service's own `503` | A code a caller acts on without parsing a header; `409` is what a disabled record already answers, and *gone* says a name was real, is not coming back, and is worth caching | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | `retired` is terminal and holds the name | *Not now* and *not ever* are different answers, and a caller can act on the difference | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | Declared state is what the health checker ignores | A service turned off on purpose is not a service that failed | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | A declared state keeps the backlog and refuses what is new | Work somebody already accepted is not thrown away because a name was given up; TTL empties what is left | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | A retired name keeps no credential | A credential per given-up name is the cost MVP measured and dropped; the record holds the name without one | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | Down and retired are declared by the owner or the assigned maintainers | The authority that already disables and deletes a record; no new authority and no new group | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | Retirement is undone by the service owner or the daemon owner | The service has nothing left to ask with, so it is the owner's act, made with their own credential | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | Each declared state answers with its own code and words, and never *no such name* | A caller cannot tell *no such name* from a typo, and a refusal it cannot act on is not an answer | [down and retired](records.md#down-and-retired) |
| 2026-09-15 | Retirement is where a name is protected, and R1.2's removed-names topic is retired with it | A name deliberately kept costs one record; reserving every removed one is what MVP paid for and dropped | [down and retired](records.md#down-and-retired) |

## Runtime integration scope revision

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Runtime integrations promoted to MVP | Owner requires usable integrations in the current stage | [MVP delivery](../../docs/08-runner-role.md#runtime-integration-delivery) |

## Open

Unresolved choices live in [questions](QUESTIONS.md#open-questions).

## History

Original wording and superseded choices are preserved in [decision history](../MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../MVP/done/document-migration.md#decision-mapping).
