# Shared locks

Status: **built** — in 0.8.66, and tied to records in 0.8.74. The contract is
[shared locks](../../../docs/01-identity-and-authority.md#shared-locks); this page
holds the reasoning.

## Shared locks

A lock belongs to a **record**: the record is its namespace, and its Owner,
its Maintainers and its own Agent may use its locks — the same authority as
its [private values](../../../docs/constitution.md#-private-values) and its
[key-value store](kv.md#per-record-storage) (owner, 2026-09-30). Its allow
list grants use of the record, not of its locks.

| | |
|---|---|
| Verbs | `lock`, `try-lock`, `extend`, `release`, `release --force`, `holders`, each taking a record and a lock name ([contract](../../../docs/01-identity-and-authority.md#shared-locks)) |
| Record | must exist and be active; any deactivation ends every lock |
| TTL | every lock has one; a crashed holder cannot wedge the rest |
| Memory only | a map in the daemon's process, never stored; a restart releases every lock |
| Holder | the calling principal, from its token |

Sets of interchangeable locks moved to [R2.0](../../R2.0-Future/lock-sets.md#a-set-of-locks) on 2026-09-30.

<details>
<summary>Why it is built this way</summary>

| | |
|---|---|
| Why the daemon | a pool already shares one thing, the bus ([runner § one name on many hosts](../runner-implementation.md#one-name-on-many-hosts)). A lock service would be a second authority, and could not itself be a pool without consensus |
| Why a record | the Owner, Maintainers, own Agent and inactive rule already exist on every record, so locks add no new access model. Two records may each have a `deploy` lock and never block each other |
| What is left out | fair queuing, reentrancy, fencing numbers, locks that outlive their holder. Whoever needs one is describing a service of their own |
| What it holds | nothing. A value the holders agree on belongs in the record's [key-value store](kv.md#per-record-storage), or in the blind store [R1.3 is exploring](../../R1.3/exploration.md#shared-secrets-and-a-kv-with-locks) |

</details>

## Review, 2026-09-30

<details>
<summary>History: the review of the record-scoped locks, both findings fixed</summary>

Review of `ca701cb` (record-scoped locks) and `42e03f6` (Locks page),
2026-09-30. Current behavior is defined by
[shared locks](../../../docs/01-identity-and-authority.md#shared-locks).

#### Findings

| ID | Severity | Finding | Resolution |
|---|---|---|---|
| P1 | High | A waiting `/lock` request passed the registry gate before sleeping. Deactivation removed the hold and woke the waiter, which acquired the lock and returned 200 while its record was inactive. | Fixed: registry changes and cleanup share the acquisition mutex; ending a record's life invalidates its pending takes. Every retry rechecks liveness and management authority. |
| P2 | Medium | `/unregister` removed the record but left its locks keyed by name. Another owner registering that name inherited the previous holder's locks and received 409 on acquisition. | Fixed: successful unregister deletes all holds and invalidates pending takes on the canonical record name before replying. Failed unregister and unrelated records retain their locks. |

#### Evidence

Both original findings reproduced three times with the race detector. Existing
Go lock/API tests and the 16 relevant web contract tests passed before the fix;
they did not cover these lifecycle cases.

| Check | Covers |
|---|---|
| `TestUnregisterDeletesEveryRecordLock` | All named holds deleted, canonical spelling, refused removal preserves holds, unrelated holds preserved, another owner's reused name starts free |
| `TestRecordLifecycleRefusesWaitingLocks` | Queued takes return 404 promptly after `/manage` deactivation, owner deactivation, or unregister; no hold is recreated |
| `TestDropRecordCancelsQueuedTakesAcrossReuse` | A confirmed queued take is invalidated across name reuse; unrelated holds survive |
| `TestTakeRechecksAuthorityAfterWaiting` | Revoked authority refuses a retry after the holder releases |
| `TestFailedChangeKeepsLocks` | Failed durable changes do not clear holds |

The regression checks fail on assertions when lifecycle cleanup is disabled;
that mutant also fails `src/smoke.sh --slow`. Restoring the fix passes the API
and lock package tests with `-race` and the isolated full `--slow` gate
(895 passed, zero failed).

The first shared-workspace run encountered a concurrent schema migration edit
and a launcher terminal-title failure. An isolated checkout with only this fix
passed the complete gate. The final shared-workspace `--slow` gate also passed (895 passed, zero failed).

</details>
