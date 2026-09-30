# Shared locks

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Shared locks

**The daemon hands out named locks, each in a Group; one holder has one at a
time.** The Group is the lock's namespace and its ACL: every effective member
may use its locks (owner, 2026-09-30).

| Verb | |
|---|---|
| `lock(group, name, ttl)` | take the lock; waits until granted or the caller's wait runs out |
| `try-lock(group, name, ttl)` | the same, granted or refused now |
| `release(group, name)` | the holder gives it back before the ttl |
| `release(group, name) --force` | any member releases a lock someone else holds, so a later pipeline stage can let go of what an earlier one took; audited |
| `holders(group)` | who holds which lock, for any member |

| Rule | |
|---|---|
| Group | must exist and be active. An inactive Group has no locks: taking one is refused and the ones it held are gone ([no such entity](../../docs/constitution.md#common-record-fields)) |
| Members | the [Group's membership](../../docs/01-identity-and-roles.md#groups), nested groups included, checked when a lock is taken. `*` is allowed, for a lock every user may take ([Q140](QUESTIONS.md#open-questions)). A Personal Group gives one person's agents their own locks |
| TTL | every lock has one; a crashed holder cannot wedge the rest |
| Memory only | a map in the daemon's process, never stored. A restart releases every lock |
| Holder | the calling principal, from its token |

### A set of locks

A Group's Owner or Maintainers may declare a **set** of interchangeable locks —
four GPUs, eight browser sessions. A member takes a named one (`gpu2`) or *any
free one*, and the answer says which it got. An empty set waits like a held
lock, and it reports how many are free.

<details>
<summary>Why it is built this way</summary>

| | |
|---|---|
| Why the daemon | a pool already shares one thing, the bus ([runner § one name on many hosts](runner.md#one-name-on-many-hosts)). A lock service would be a second authority, and could not itself be a pool without consensus |
| Why a Group | membership, Owner, Maintainers and the inactive rule already exist, so locks add no new access model. Two teams may each have a `deploy` lock and never block each other |
| Why a set, not a counter | a counter says *proceed* and lets two holders pick the same GPU; a set says *you have `gpu2`* |
| What is left out | fair queuing, reentrancy, fencing numbers, locks that outlive their holder. Whoever needs one is describing a service of their own |
| What it holds | nothing. A value the holders agree on belongs in the [key-value store](kv.md#per-name-storage), or in the blind store [R1.3 is exploring](../R1.3/exploration.md#shared-secrets-and-a-kv-with-locks) |

</details>
