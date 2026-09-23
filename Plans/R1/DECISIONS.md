# R1 decisions

## Recorded decisions

Migrated 2026-09-13. Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

| Decision topic | Substance | Earlier rows |
|---|---|---|
| Record-defined roles move to R1 as its first topic | [groups and roles](identity.md#groups-and-roles) | 2026-09-22 owner decision; moved out of MVP, no representation adopted |
| Additional storage backends assigned to R1 | [storage](storage.md#backends) | 2026-09-20 owner instruction; implement SQLite first in 0.7 |
| Locks are held in memory and never stored | [shared locks](locks.md#shared-locks) | 2026-09-22 owner decision; no SQLite table and no dump, a restart releases every grant |
| A per-name key-value store assigned to R1 | [key-value store](kv.md#per-name-storage) | 2026-09-22 owner instruction; SQLite-backed, per User and per registry record, with atomic operations |
| Sigils | [definition](identity.md#sigils) | D6, D7, D27, D28, D29, D30 |
| Enrolment policy | [definition](access.md#enrolment-policy) | D18 |
| On demand | [definition](runner.md#on-demand) | |
| Message routing | [definition](runner.md#many-names-into-one-inbox) | |
| Delegation | [definition](identity.md#delegation) | D37 |
| Ownership | [definition](identity.md#ownership) | D38, D40, D133, D134, D135 |
| Sealed private config | [definition](identity.md#sealed-private-config) | D41 |
| Token scope | [definition](access.md#token-scope) | D46 |
| Key modes | [definition](access.md#key-modes) | D58; lifecycle revised 2026-09-13 |
| Encrypted sessions | [definition](access.md#encrypted-sessions) | D59, D60 |
| Additional script forms | [definition](runner.md#additional-script-forms) | D61, D118, D119, D121 |
| Key confirmation | [definition](access.md#key-confirmation) | D63 |
| Shared locks | [definition](locks.md#shared-locks) | D64 |
| A set of locks | [definition](locks.md#a-set-of-locks) | D65, D66 |
| Scope | [definition](README.md#scope) | D69 |
| Registry sync | [definition](registry.md#registry-sync) | D79 |
| Chaining | [definition](federation.md#chaining) | D80 |
| Where it runs | [definition](auth.md#where-it-runs) | D103, D105 |
| Topology | [definition](auth.md#topology) | D104 |
| Ssh admin | [definition](auth.md#ssh-admin) | D106 |
| Long lived services | [definition](runner.md#long-lived-services) | D120, D122 |
| One name on many hosts | [definition](runner.md#one-name-on-many-hosts) | D123, D124, D125, D128 |
| What an instance is | [definition](runner.md#what-an-instance-is) | D129, D202, D207, D210 |
| Backing it up | [definition](runner.md#backing-it-up) | D136, D137, D144 |
| The list of what is installed | [definition](runner.md#the-list-of-what-is-installed) | D138, D139, D140 |
| What it comes after | [definition](runner.md#what-it-comes-after) | D141, D142, D143 |
| Modules | [definition](modules.md#modules) | D178 |
| Reaching the runner | [definition](runner.md#reaching-the-runner) | D197, D198, D208 |
| Runner unit | [definition](operations.md#runner-unit) | D199, D200, D201 |
| The three env layers | [definition](runner.md#the-three-env-layers) | D203, D204 |
| What the child is told | [definition](runner.md#what-the-child-is-told) | D205, D206 |
| Where it runs | [definition](runner.md#where-it-runs) | D209 |
| What the runner does | [definition](runner.md#what-the-runner-does) | D211, D212 |
| Who it runs as | [definition](runner.md#who-it-runs-as) | D213 |
| Dashboard extensions | [definition](discovery.md#dashboard-extensions) | D243 |
| Groups and roles | [definition](identity.md#groups-and-roles) | D249 |

## Backup encryption choice

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | User-key backup encryption | Reuse the user's existing key and an established tool; owner instruction | [runner § backing it up](runner.md#backing-it-up) |

## Distribution choice

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Published release distributions | Owner requests installation and container startup without a source build | [Release artifacts](distribution.md#release-artifacts), [container runtime](distribution.md#container-runtime) |

## Open

Unresolved choices live in [questions](QUESTIONS.md#open-questions).

## Credential lifecycle revision

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Credential lifecycle across releases | Preserve the material needed for unprocessed encrypted messages; owner instruction | [token lifetime](../../docs/02-access.md#token-lifetime), [R1 key modes](access.md#key-modes) |

## Dashboard scope revision

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Optional dashboard additions in R1 | Owner confirms required/optional split | [dashboard extensions](discovery.md#dashboard-extensions) |

## Superseded

| Earlier design | Replacement |
|---|---|
| Basic groups, maintainers and activity graphs deferred to R1 | [MVP groups](../../docs/01-identity-and-roles.md#groups), [required dashboard](../../docs/05-discovery.md#required-tabs) |
| Clock-based derived-key lifecycle and its overlap window | [Credential lifetime policy](../../docs/02-access.md#token-lifetime); key sources remain in [key modes](access.md#key-modes) |
| Epoch-bound authorization freshness | [AUTH consistency](auth.md#consistency-window); replacement propagation rules remain open |

## History

Original wording and superseded choices are preserved in [decision history](../MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../MVP/done/document-migration.md#decision-mapping).
