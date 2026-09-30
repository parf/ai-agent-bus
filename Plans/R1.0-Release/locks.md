# Shared locks

Status: **built** — in 0.8.66, and tied to records in 0.8.74. The contract is
[shared locks](../../docs/01-identity-and-roles.md#shared-locks). **A set of
locks is still pending**; this page holds that design
([questions](QUESTIONS.md#open-questions)).

## Shared locks

A lock belongs to a **record**: the record is its namespace, and its Owner,
its Maintainers and its own Agent may use its locks — the same authority as
its [private values](../../docs/constitution.md#-private-values) and its
[key-value store](kv.md#per-record-storage) (owner, 2026-09-30). Its allow
list grants use of the record, not of its locks.

| | |
|---|---|
| Verbs | `lock`, `try-lock`, `extend`, `release`, `release --force`, `holders`, each taking a record and a lock name ([contract](../../docs/01-identity-and-roles.md#shared-locks)) |
| Record | must exist and be active; any deactivation ends every lock |
| TTL | every lock has one; a crashed holder cannot wedge the rest |
| Memory only | a map in the daemon's process, never stored; a restart releases every lock |
| Holder | the calling principal, from its token |

### A set of locks — pending

A record's Owner or Maintainers may declare a **set** of interchangeable locks
on it — four GPUs, eight browser sessions. One who may use the record's locks
takes a named one (`gpu2`) or *any free one*, and the answer says which it
got. An empty set waits like a held lock, and it reports how many are free.
Not built; the [TODO](TODO.md#dependencies) owns the acceptance.

<details>
<summary>Why it is built this way</summary>

| | |
|---|---|
| Why the daemon | a pool already shares one thing, the bus ([runner § one name on many hosts](runner.md#one-name-on-many-hosts)). A lock service would be a second authority, and could not itself be a pool without consensus |
| Why a record | the Owner, Maintainers, own Agent and inactive rule already exist on every record, so locks add no new access model. Two records may each have a `deploy` lock and never block each other |
| Why a set, not a counter | a counter says *proceed* and lets two holders pick the same GPU; a set says *you have `gpu2`* |
| What is left out | fair queuing, reentrancy, fencing numbers, locks that outlive their holder. Whoever needs one is describing a service of their own |
| What it holds | nothing. A value the holders agree on belongs in the record's [key-value store](kv.md#per-record-storage), or in the blind store [R1.3 is exploring](../R1.3/exploration.md#shared-secrets-and-a-kv-with-locks) |

</details>
