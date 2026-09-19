# R1.1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q14 | Whether `unshare` becomes a second sandbox backend, for the container where there is no systemd user manager | owner, with the image | [runner § sandboxing](../../docs/08-runner-role.md#sandboxing) |
| Q25 | Whether `kv`'s hash of locks is the daemon's locks under a name, or a second authority | owner | [bundled services § data](services.md#data) |
| Q26 | Whether `kv` is optional, given that it is where service configuration would live | owner | [bundled services § data](services.md#data) |
| Q31 | Who may read contact routes: everyone, administrators, or a narrower service audience | owner | [context](people.md#how-to-reach-a-person) |
| Q32 | Whether record expiry and service credentials are allowed to change the daemon despite the former whole-stage no-change criterion | owner | [context](README.md#scope) |
| Q73 | How an `x-service` secret is stored, rotated and granted separately from seeing the record, and whether the MVP protocol-bearing record migrates into this kind or stays beside it | owner | [external services](records.md#external-services-and-their-secrets) |

## Services context

Identity linkage is deferred to [R1.2 questions](../R1.2/QUESTIONS.md#open-questions).

❓ **Whether `kv` is optional.** It is not in the required minimum
([overview § principles](../../docs/00-overview.md#principles)) and the bus runs without
it — but a store that holds services' **configuration** is a hard thing to call
optional, because then the services that keep their config there are optional
too. *Settled by:* owner.

❓ **A hash of locks.** Asked for, and the one item here that would be a
**second lock authority**: the daemon grants named locks as of R1
([messaging § shared locks](../R1/locks.md#shared-locks)), and two things
granting locks is exactly what that section argues against — more so now that
the store is `kvrocks`, where such a lock would be that server's rather than
the bus's. What a set of locks is *for* is written down there now ([messaging § a set of
locks](../R1/locks.md#a-set-of-locks)), so the question left is narrower:
whether `kv` shows them at all, or callers ask the daemon. `setNX` with a ttl
is already a lock in everything but name, which is why this is worth settling
rather than leaving to whatever each caller invents. Either these *are*
the daemon's locks under a name, or the kv holds them itself and then a `kv`
that is a **pool** cannot be correct. *Settled by:* owner.

## People context

❓ **Who may read somebody else's.** Writing is settled — a maintainer, and
nobody else ([who may write a record](../../docs/01-identity-and-roles.md#users-and-profiles)). Reading is
not: a phone number is not an avatar, and the alerter needs everybody's.
*Settled by:* owner, with the ACL.
