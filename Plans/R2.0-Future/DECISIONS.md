# Future decisions

## Recorded decisions

Migrated 2026-09-13. Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

| Decision topic | Substance | Earlier rows |
|---|---|---|
| Constitution | [0.7 work](../done/R0.8-MVP/0.7.0-TODO.md#objective) | Owner assigned implementation to 0.7; current decision is indexed in [MVP decisions](../../docs/decisions.md#settled) |
| New auth work moved from R1.1 | [Topics](README.md#topics); [dependencies](FUTURE.md#auth-dependencies); [questions](QUESTIONS.md#access-context) | owner, 2026-11-05 |
| Sigils | [definition](identity.md#sigils) | D6, D7, D27, D28, D29, D30 |
| Enrolment policy | [definition](access.md#enrolment-policy) | D18 |
| Delegation | [definition](identity.md#delegation) | D37 |
| Ownership | [definition](identity.md#ownership) | D38, D40, D133, D134, D135 |
| Sealed private config | [definition](identity.md#sealed-private-config) | D41 |
| Token scope | [definition](access.md#token-scope) | D46 |
| Key modes | [definition](access.md#key-modes) | D58; lifecycle revised 2026-09-13 |
| Encrypted sessions | [definition](access.md#encrypted-sessions) | D59, D60 |
| Key confirmation | [definition](access.md#key-confirmation) | D63 |
| Where it runs | [definition](auth.md#where-it-runs) | D103, D105 |
| Topology | [definition](auth.md#topology) | D104 |
| Ssh admin | [definition](auth.md#ssh-admin) | D106 |
| Candidates | [definition](FUTURE.md#candidates) | D24, D219 |
| Storage | [definition](storage.md#storage) | D88 |
| Debug mode | [definition](debug.md#debug-mode) | D100 |
| The RAG service moved to R1 | [R1 RAG](../R1.0-Release/rag/README.md#rag-service) | 2026-10-27 owner decision |
| Sets of interchangeable locks wait in R2.0, moved from R1 | [lock sets](lock-sets.md#a-set-of-locks) | 2026-09-30 owner decision |
| Billing | [definition](billing.md#billing-role--future) | D107, D108, D171 |
| Directional access | [definition](acl-direction.md#where-direction-is-needed) | 2026-09-18 owner decision: only registered queues and pub/sub channels get separate read and write lists; every other record has one fixed side |
| Teams SDK and first scope (Q249, Q251; #ab-dir, 2026-10-30) | [teams](teams.md#decided) | |
| WhatsApp answers inside the 24-hour window first, opt-in is a per-contact flag (Q254, Q256; #ab-dir, 2026-10-30) | [whatsapp](whatsapp.md#decided) | |
| Messenger serves an own Page only (Q258; #ab-dir, 2026-10-30) | [messenger](messenger.md#decided) | |

## Open

Unresolved choices live in [questions](QUESTIONS.md#open-questions).

## History

Original wording and superseded choices are preserved in [decision history](../done/R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../done/R0.8-MVP/done/document-migration.md#decision-mapping).

## Credential lifecycle revision

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Credential lifecycle across releases | Preserve the material needed for unprocessed encrypted messages; owner instruction | [token lifetime](../../docs/02-access.md#token-lifetime), [R1.1 key modes](access.md#key-modes) |

## Superseded auth designs

| Earlier design | Replacement |
|---|---|
| Clock-based derived-key lifecycle and its overlap window | [Credential lifetime policy](../../docs/02-access.md#token-lifetime); key sources remain in [key modes](access.md#key-modes) |
| Epoch-bound authorization freshness | [AUTH consistency](auth.md#consistency-window); replacement propagation rules remain open |
