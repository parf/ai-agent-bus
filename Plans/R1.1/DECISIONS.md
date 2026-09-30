# R1.1 decisions

## Recorded decisions

Migrated 2026-09-13 and split from the old R1 on 2026-09-30; the R1 rows are in [R1 decisions](../R1.0-Release/DECISIONS.md#recorded-decisions). Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

| Decision topic | Substance | Earlier rows |
|---|---|---|
| Additional storage backends assigned to R1.1 | [storage](storage.md#backends) | 2026-09-20 owner instruction; implement SQLite first in 0.7 |
| Sigils | [definition](identity.md#sigils) | D6, D7, D27, D28, D29, D30 |
| Enrolment policy | [definition](access.md#enrolment-policy) | D18 |
| Delegation | [definition](identity.md#delegation) | D37 |
| Ownership | [definition](identity.md#ownership) | D38, D40, D133, D134, D135 |
| Sealed private config | [definition](identity.md#sealed-private-config) | D41 |
| Token scope | [definition](access.md#token-scope) | D46 |
| Key modes | [definition](access.md#key-modes) | D58; lifecycle revised 2026-09-13 |
| Encrypted sessions | [definition](access.md#encrypted-sessions) | D59, D60 |
| Key confirmation | [definition](access.md#key-confirmation) | D63 |
| Scope | [definition](README.md#scope) | D69 |
| Registry sync | [definition](registry.md#registry-sync) | D79 |
| Where it runs | [definition](auth.md#where-it-runs) | D103, D105 |
| Topology | [definition](auth.md#topology) | D104 |
| Ssh admin | [definition](auth.md#ssh-admin) | D106 |
| Dashboard extensions | [definition](discovery.md#dashboard-extensions) | D243 |

## Open

Unresolved choices live in [questions](QUESTIONS.md#open-questions).

## Credential lifecycle revision

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Credential lifecycle across releases | Preserve the material needed for unprocessed encrypted messages; owner instruction | [token lifetime](../../docs/02-access.md#token-lifetime), [R1.1 key modes](access.md#key-modes) |

## Dashboard scope revision

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Optional dashboard additions in R1.1 | Owner confirms required/optional split | [dashboard extensions](discovery.md#dashboard-extensions) |

## Superseded

| Earlier design | Replacement |
|---|---|
| Basic groups, maintainers and activity graphs deferred to R1 | [MVP groups](../../docs/01-identity-and-roles.md#groups), [required dashboard](../../docs/05-discovery.md#required-tabs) |
| Clock-based derived-key lifecycle and its overlap window | [Credential lifetime policy](../../docs/02-access.md#token-lifetime); key sources remain in [key modes](access.md#key-modes) |
| Epoch-bound authorization freshness | [AUTH consistency](auth.md#consistency-window); replacement propagation rules remain open |

## History

Original wording and superseded choices are preserved in [decision history](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../R0.8-MVP/done/document-migration.md#decision-mapping).
