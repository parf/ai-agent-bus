# R1.2 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q14 | Whether `unshare` becomes a second sandbox backend, for the container where there is no systemd user manager | owner, with the image | [runner § sandboxing](../../docs/06-dedicated-runner.md#sandboxing) |
| Q25 | Whether `kv`'s hash of locks is the daemon's [shared locks](../../docs/01-identity-and-authority.md#shared-locks) under a name, or a second authority | owner | [bundled agents § data](services.md#data) |
| Q26 | Whether the bundled `kv` is still wanted, and optional, now that every record has the daemon's own [key-value store](../../docs/03-records-kv.md#key-value-store) (0.8.77): it would narrow to what that store lacks — a ttl per key, blocking lists and the atomic pull-push, hashes | owner | [bundled agents § data](services.md#data) |
| Q31 | Who may read contact routes: everyone, administrators, or a narrower agent audience | owner | [context](people.md#how-to-reach-a-person) |
| Q32 | Whether record expiry and agent credentials are allowed to change the daemon despite the former whole-stage no-change criterion | owner | [context](README.md#scope) |
| Q73 | How a service secret is stored and rotated, and whether a read is recorded | owner | [external services](records.md#external-services-and-their-secrets) |

## Agents context

Identity linkage is deferred to [R1.3 questions](../R1.3/QUESTIONS.md#open-questions).

❓ **Whether `kv` is optional.** It is not in the required minimum
([overview § principles](../../docs/00-overview.md#how-it-works)) and the bus runs without
it — but a store that holds agents' **configuration** is a hard thing to call
optional, because then the agents that keep their config there are optional
too. *Settled by:* owner.

❓ **A hash of locks.** Asked for, and the one item here that would be a
**second lock authority**: the daemon grants named locks as of R1
([messaging § shared locks](../R1.0-Release/done/locks.md#shared-locks)), and two things
granting locks is exactly what that section argues against — more so now that
the store is `kvrocks`, where such a lock would be that server's rather than
the bus's. What a set of locks is *for* is written down there now ([a set of
locks](../R2.0-Future/lock-sets.md#a-set-of-locks)), so the question left is narrower:
whether `kv` shows them at all, or callers ask the daemon. `setNX` with a ttl
is already a lock in everything but name, which is why this is worth settling
rather than leaving to whatever each caller invents. Either these *are*
the daemon's locks under a name, or the kv holds them itself and then a `kv`
that is a **pool** cannot be correct. *Settled by:* owner.

## People context

❓ **Who may read somebody else's.** Writing is settled — a maintainer, and
nobody else ([who may write a record](../../docs/01-identity-and-authority.md#users-and-profiles)). Reading is
not: a phone number is not an avatar, and the alerter needs everybody's.
*Settled by:* owner, with the ACL.

## Messaging agents

- ❓ Q244 Discord: who creates and owns the application and bot, for which server
  - Context: needed before any live test; no sign-up was made. Moved from R1 to R1.1, then to R1.2 with the Discord agent ([plan](discord.md#open-for-the-owner))
  - Proposal: the owner names the account and the server
  - Impact: none of the Discord agent can be tried live before this
  - Settled by: the owner

Google Chat ([B.27b](TODO.md#objective), [plan](google-chat.md#proposed-methods-for-approval)) moved here from R1 (owner, 2026-11-01: a corporate account is needed). No sign-up or live call was made.

- ✅ Q247 → [Google Chat needs a corporate Workspace account](DECISIONS.md#recorded-decisions)

- ✅ Q248 → [Google Chat needs a corporate Workspace account](DECISIONS.md#recorded-decisions)

## Runner hardening

Moved from R1.1 with M.14 (owner, 2026-11-06): 5 open questions in [runner hardening](runner-hardening.md#questions) (№Q276–Q280).
