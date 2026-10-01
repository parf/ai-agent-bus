# Sets of interchangeable locks

Status: unassigned, not built; moved from R1 on 2026-09-30 (owner). It builds
on [shared locks](../../docs/01-identity-and-authority.md#shared-locks), which are
built on records. Open choices are [Q142](QUESTIONS.md#open-questions).

## A set of locks

A record's Owner or Maintainers may declare a **set** of interchangeable locks
on it — four GPUs, eight browser sessions, the seats on a licence. One who may
use the record's locks takes a **named** one (`gpu2`) or **any free one**, and
the answer says which it got. An empty set waits like a held lock, and it
reports how many are free. A TTL, memory-only holds, `release --force` and the
audit are as for any lock.

<details>
<summary>Why a set, not a counter</summary>

A counting semaphore says *proceed* and lets two holders pick the same GPU; a
set says *you have `gpu2`*. To the daemon a set is not a second mechanism: a
lock on a record, plus the one claim a plain lock cannot make — its members
are interchangeable, which is what *any free one* means.

</details>
