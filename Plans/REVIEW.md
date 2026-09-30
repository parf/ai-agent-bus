# Locks review

Review of `ca701cb` (record-scoped locks) and `42e03f6` (Locks page),
2026-09-30. Current behavior is defined by
[shared locks](../docs/01-identity-and-roles.md#shared-locks).

## Findings

| ID | Severity | Finding | Resolution |
|---|---|---|---|
| P1 | High | A waiting `/lock` request passed the registry gate before sleeping. Deactivation removed the hold and woke the waiter, which acquired the lock and returned 200 while its record was inactive. | Fixed: registry changes and cleanup share the acquisition mutex; ending a record's life invalidates its pending takes. Every retry rechecks liveness and management authority. |
| P2 | Medium | `/unregister` removed the record but left its locks keyed by name. Another owner registering that name inherited the previous holder's locks and received 409 on acquisition. | Fixed: successful unregister deletes all holds and invalidates pending takes on the canonical record name before replying. Failed unregister and unrelated records retain their locks. |

## Evidence

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
