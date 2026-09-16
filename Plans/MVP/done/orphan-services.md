# H.5.5 and H.5.4 — a start clears out the records whose owner it does not know

Shipped in 0.5.35. Evidence for the acceptance in [TODO](../TODO.md#remaining-work)
as those two rows stood before removal. They land together because they are one
decision made twice — about the record, and then about the credential that
answered for it.

## Scope

The contract was written and marked pending
([identity § when the owner is gone](../../../docs/01-identity.md#when-the-owner-is-gone)):
a record whose owner the daemon knows nothing about is wreckage, and goes at
once with everything that hung on it. Nothing implemented it. And because it
did not exist, H.5.4's credential sweep carried an **interim guard** — it spared
a credential whose name still owned records, so as not to strand them before
there was anything to delete them with. The guard was the only thing standing
between the two sweeps and one answer.

## What changed

| | |
|---|---|
| [`core.Orphans`](../../../src/internal/core/orphans.go) | deletes every record that `wreckage` names, to a fixed point, returns what it took, and collects the waits those names left on inboxes that survived |
| `core.wreckage` | the owner has neither a profile nor a record — **and the record is not a registered user's own** |
| `core.orphan` | releases the readers blocked on a name before it goes |
| `core.forgetName` | one place for what a name leaves behind: record, inbox, subscriptions, **group membership** |
| [`core.UnregisterAnd`](../../../src/internal/core/unregister.go) | a non-person now goes through `forgetName`; a registered user keeps their standing, so only their record and inbox go |
| [`core.ownerless`](../../../src/internal/core/users.go) | the interim guard is gone |
| [`agent-busd`](../../../src/cmd/agent-busd/bus.go) | the sweep runs at start, **after `api.New` and before the credential sweep**, and the snapshot is written again after both |
| [`agent-bus-web`](../../../src/cmd/agent-bus-web/users.go), [`core.RemoveOwnerless`](../../../src/internal/core/users.go), [pages](../web/pages.md#user-username) | the copy describing the interim guard, in three places, for a state no longer reachable |

**A registered user's own record is never wreckage, whoever the store says owns
it.** The rule rests on there being no principal behind the name, and a person
is one: their profile is their standing, `manages` lets the name itself manage
its own record, and messages addressed to them reach somebody. This is the
exemption the credential sweep already makes, for the one reason both share — a
user is never deleted. codex found it, with a store holding `person@h`'s profile
and a `person@h` record owned by a missing name; I reproduced it before fixing
it. The sweep took the person's inbox and their group membership, and handed
the name to be forgotten, while `Ownerless` spared the profile — so the two
sweeps disagreed about one name, which is the thing the ordering exists to
prevent.

**The credentials are dropped by the sweep below and nowhere else.** The purge
had its own `Forget` loop; every name it takes has no record and no profile by
the time `Ownerless` asks, so the loop was a second place deciding one thing.
Mutation says so directly — removing it changed nothing, and removing *both* is
what the smoke's reuse check catches.

**Known is a profile or a record, not a profile.** A self-owned record with no
user behind it is a principal this daemon supports: it authenticates, it may be
handed a record by transfer, and it may register records of its own. An earlier
draft of mine asked instead whether a chain of owners reached a *user*. codex
blocked it as a contract change and I reproduced its counterexample exactly — a
self-owned profileless `legacy@h` authenticates, may legally be handed a record
by alice, and may register `fresh@h`; reachability deletes all three. Ownership
is asked about one step.

**A cycle survives, by the same rule rather than as an exception**: every
member's owner has a record. Probed rather than assumed — `Register` builds
chains freely, but a cycle needs a transfer, transfer demands a self-owned
principal, and re-registration keeps the owner it had. A cycle in a store came
from outside the daemon, and inventing a rule for it here would be inventing the
rule too.

**It runs to a fixed point because deleting makes orphans.** A owns B and B owns
C, all records: taking A is what leaves B unknown. One pass leaves C live,
holding a queue nobody may read, until some later restart happens to notice.

**Waiters are collected twice over, and the two are different problems.** On the
inbox being deleted, each is released by hand before it goes: after the delete
there is no inbox left for `recheckReaders` to find it through, and even before
it a deleted record reads back as the zero record, whose empty allow list makes
`may` true. And on inboxes that **survive**, the purged name's reads elsewhere
are collected by the `recheckReaders` that `UnregisterAnd` ends with and that
this did not carry over. codex found the second and I reproduced it: the waiter
stays attached, `deliver` trusts the waiters it finds without asking the ACL
again, and a name the daemon no longer knows was handed a message sent after it
disappeared.

## The defect this uncovered

`UnregisterAnd` never stripped group membership. A freed name is reclaimable by
anybody, so the membership was not a dangling row — it was **inherited**.
Probed on the live build: mallory registers the freed `svc@h`, arrives already
in `@ops`, and reaches `secret@h`, whose allow list is `@ops` and nothing else.
Fixed here because `forgetName` is shared, and pinned from both sides
(`TestAnUnregisteredNameKeepsNoGroupMembership` walks the whole probe).
Flagged to the owner separately: it was live, and it is not what this task was
for.

## The ordering, and what is actually load-bearing

| Before | Because |
|---|---|
| the credential sweep | deleting a record is what makes its name answer for nothing, so the two cannot disagree about one name |
| after `api.New` | that call is what makes the daemon owner a registered user |
| a `save` after both | the snapshot written at line one of the run predates them |

The second of those was nearly unfalsifiable. A restart over any store this
daemon wrote hands the owner back before `api.New` runs, because `Restore`
rebuilds a profile from the `@maintainers` group — so moving the sweep earlier
changed nothing and the mutation survived. The smoke fixture now takes the
owner out of **both** the user list and that group, which is a hand-edited store
— the same premise the whole feature rests on. With that, moving the sweep
earlier eats `owner-svc@srv1` and `ghost@srv1`.

## Checks

The end-to-end clause is *start the daemon over a store*, which no Go test can
present: smoke § **a start clears out the records whose owner it does not know**
builds a store with its own daemon, stops it, edits it by hand, and starts
again. The hand edit is the only way in — a running daemon refuses every call
that would make an orphan, which is the point.

| Check | What it pins |
|---|---|
| smoke: the record is gone, the chain with it to its end | the fixed point, three deep, one check per link |
| smoke: the credential no longer authenticates | H.5.4's clause, in the same run; and again for a name that **owned services** when the start took it |
| smoke: four controls keep their queues | an active owner, a paused one, a banned one, and the daemon owner — a stopped owner is not a missing one |
| smoke: the freed name registers to somebody else, and is handed none of the purged work | |
| smoke: the store on disk no longer holds it | the save after the sweeps, read off disk rather than through a second restart — a graceful stop would write a clean dump either way and prove nothing |
| `TestAStoppedOwnerIsNotAMissingOne` | the same three controls, in core, with the queues measured |
| `TestDeletionRunsToAFixedPoint` | |
| `TestAPurgeReleasesItsBlockedReaderWithNothingFromTheQueue` | a waiter cannot exist at startup, so the release is exercised directly with one attached. The reader is **filtered** and the queued work does not match it: an unfiltered one would take the message rather than block, and there would be nothing to release |
| `TestAPurgeAsksNeitherGuardThatUnregisterAsks` | both bypassed guards, each with a by-hand control of the same shape refusing beside it |
| `TestAPurgedNameKeepsNoGroupMembership`, `TestAnUnregisteredNameKeepsNoGroupMembership` | the defect above, from both sides |
| `TestRemovingAPersonsRecordLeavesTheirMemberships` | and the exception: a person keeps their standing |
| `TestTheTwoSweepsAgreeAboutOneName` | the interim guard's removal |
| `TestARegisteredUsersOwnRecordIsNotWreckage` | the record, the queue, the membership and the credential all survive, with a service of the same missing owner going beside it as the control; and they can still remove it themselves |
| `TestAPurgeReleasesTheWaitsItLeftOnOtherInboxes` | the count, and the consequence: a message sent afterwards |
| `TestAnOwnershipCycleSurvives` | and what hangs under one |
| `TestAMaintainerInTheStoreIsAUser` | see below |
| smoke: the snapshot is a snapshot, with the survivors and their queued bodies in it | the absence checks beside it pass against an empty file |

## Mutation

Each break made, the named check watched to fail, the file restored.

| Mutation | Failed |
|---|---|
| the sweep stops after one pass | `TestDeletionRunsToAFixedPoint` — *purged [a@h]*; and four smoke checks, one per link plus the count in the log |
| `!active(owner)` in place of *known at all* | `TestAStoppedOwnerIsNotAMissingOne` — the paused and banned controls purged; eleven smoke checks |
| the waiters are not released | `TestAPurgeReleasesItsBlockedReaderWithNothingFromTheQueue` — *never released; its inbox went out from under it* |
| the released reader hears `ErrNotAllow` | the same — *released with not on that service's allow list* |
| membership is left behind | both membership checks |
| the sweep runs before `api.New` | smoke — `owner-svc@srv1` and `ghost@srv1` taken (**only after** the fixture stopped handing the owner back; see above) |
| the `save` after the sweeps is dropped | smoke — the store on disk still holds the chain and its queued work |
| the interim guard restored | `TestTheTwoSweepsAgreeAboutOneName`, `TestDirectoryClassifiesFactsAndPreservesCallerScope` |
| a registered user's record is wreckage after all | `TestARegisteredUsersOwnRecordIsNotWreckage` — *purged [active@h svc@h]* |
| the surviving inboxes are not rechecked | `TestAPurgeReleasesTheWaitsItLeftOnOtherInboxes` — *handed "post-purge" from an inbox that survived* |
| ownership walked to a person instead of one step | `TestAnOwnershipCycleSurvives` — the cycle and what hangs under it, all five taken |
| **both** credential drops removed | smoke — the reclaimed name answers its predecessor's credential |

**One survivor, and it is the ordering working.** The interim guard, restored,
does not save a credential: by the time `Ownerless` asks, `Orphans` has taken
the records, so the guard has nothing to fire on. The purge's own `Forget` loop
was the other survivor, and it is gone rather than excused — a line no mutation
can reach is a line certifying nothing.

## Three checks of mine were hollow

Each passed against every mutant, and each was found by reading output rather
than a summary line.

| | |
|---|---|
| *the purged work is not in the store* | grepped for the body followed by the addressee; a stored envelope writes `to` before `body`, so it could never match |
| *the credential no longer authenticates* | a name with no record is refused at the gate whether or not its bytes were ever dropped, so it passed with both `Forget` paths gone. It now replays the old bytes **after** somebody else has taken the name, which is the question it was for — does the previous holder answer as the new owner. codex found this, the same shape it found in H.5.7 |
| *the store no longer holds it* | `lacks` against an unreadable or empty file passes for every absence there is, so a save that wrote nothing satisfied it. A positive half now comes first: the snapshot parses, the survivors are in it, and so are their queued bodies. codex found this, and the two `sed` extractions of the edited store had the same shape — a match that failed handed back an empty string and read as proof |

## The one the sweep does not see

A name the store lists in `@maintainers` comes back as a **user**, because
`Restore` rebuilds a profile for every member — the levels are nested, and a
maintainer who was not a user would have authority the user administration
could not see. So its records are not wreckage. opencode found it and asked for
a rule rather than a side effect.

It is a rule, and the same one: the owner is known because it has a profile.
`SetGroup` fabricates that profile at the moment of the call, so no live path
puts a nameless name in the group; a store that has one was edited by hand, and
a hand that can write `@maintainers` can write `Users`. This is also the
mechanism by which the **daemon owner** gets their standing back on a restart,
which is why the `api.New` ordering above is so hard to falsify. Pinned by
`TestAMaintainerInTheStoreIsAUser`.

`src/smoke.sh --slow`: green.

## Left open

A `Forget` that cannot write its store leaves the old bytes authenticating, and
both sweeps log it and carry on. The name is then free, so whoever registers it
next is answered by their predecessor's credential. codex reproduced it and
asked for the start to fail instead. It is not introduced here — it is the
credential sweep's existing policy, and deleting records only makes names free
sooner — and the trade is availability against a narrow window under a failed
disk write, which is the owner's to make. Recorded as
[Q69](../QUESTIONS.md#open-questions); the claim in the code and in this
document is now *best effort*, not *guaranteed*.

## Peer review

**codex requested changes four times and was right each time**: the surviving
inboxes, the registered-user record, the credential check that proved nothing,
and the store check that passed against an empty file. It also raised Q69, and
confirmed the group-inheritance defect by running the original `UnregisterAnd`
and reaching a restricted record through a reclaimed name.

**opencode mapped the reachability** rather than the diff, and its finding was
the `@maintainers` resurrection above. It enumerated every write of `Owner` —
`Register`, `Configure`, `Enrol`, transfer, and the refresh path that preserves
it — and agreed no live call produces an unknown owner, which is what makes the
hand-edited store the only fixture. It also checked the buffered `w.stopped`
send under `b.mu` for a lost wakeup and found none, and asked for the cycle
stance to be pinned rather than only commented.

One thing I did not take: opencode read a status change from `ErrProfile` to
`ErrBusy` in `RemoveOwnerless` as part of this. It is not — that wrap predates
the change; only its wording moved, because *owned service* stopped being one of
the things a credential can be backed by.

## Not changed

`Orphans` returns names and the caller drops the credentials, the same shape as
`Ownerless`/`RemoveOwnerless`: core does not reach into the token store. Nothing
was added to `Unregister` for the deletion case — the two share `forgetName` and
differ only in the guards, which is the whole of the difference.
