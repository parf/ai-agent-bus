# H.5.7 — a suspended user's services refuse calls

Shipped in 0.5.34. Evidence for the acceptance in
[TODO](../TODO.md#remaining-work) as it stood before removal.

## Scope

The contract was written and marked pending
([identity § services of a user who is paused or banned](../../../docs/01-identity.md#services-of-a-user-who-is-paused-or-banned)):
a paused or banned user keeps everything they own, and **every service they own
refuses calls while that lasts**. The daemon enforced none of it. `Send` refused
a receiver whose *own* name was inactive, which for a service is vacuously true
— a record has no user profile, so it has no state — so a banned person's
services answered normally and the dashboard could only have put a label on it.

## What changed

One predicate, in two halves, in [users](../../../src/internal/core/users.go):

| | |
|---|---|
| `suspension(name)` | the name's own user state, then the half below |
| `ownerSuspension(name)` | the record's **owner's** state — a separate person, so it returns nil for a self-owned record |

Applied on the **caller** side through `acting`, which every gate and every
authority predicate already asks under the hold it writes under; and on the
**called-name** side at `Send`, `ConsumeAs`, `Subscribe` and the blocked-reader
release in `recheckInbox`.

Both sides are the contract rather than an extension: *kept is not accepted —
while the state lasts those credentials grant no access, theirs or their
services'*. The refusal is `403 suspended`, the same code and reason a
suspended caller gets, because it is one suspension seen from either side.

**Owner suspension is not another input to `active`.** `active` asks about a
name's own state and is asked about people at a dozen sites where a record
lookup means nothing; this asks about somebody else's and only of a record.
They also must not merge in `visible`, which reports `Disabled` as *the owner
turned delivery off* — a service refused because its owner is suspended is a
third fact, and presenting it as the second is the conflation
[decisions](../../../docs/decisions.md#settled) forbids.

**The check is ordered before `Disabled` everywhere**, so a record that is both
answers *suspended* on all three edges, and the released waiter hears the
suspension rather than the ACL it still satisfies.

## Two boundaries, stated because they were nearly crossed

**Q63 is untouched.** [Q63](../QUESTIONS.md#open-questions) asks whether
`ConsumeAs` should refuse a read of an inactive name's *own* inbox, the way
`Send` refuses delivery to one. The first draft put `suspension` on that path —
whose leading branch is `!active(name)` — which answered the owner's open
question as a side effect of a different task. codex caught it and reproduced
it. `ConsumeAs`, `Subscribe` and `recheckInbox` now call `ownerSuspension`,
which returns nil for a self-owned record, so the asymmetry Q63 is about is
left exactly as it was found.

**Suspension is not transitive.** A service may own a service, and suspending
the person at the top refuses the services they own but not the services those
own in turn. Both peers raised this independently. The contract says *every
service they own*, which is the direct relation the record states; reaching
further would be a larger rule and would need an answer for a cycle. Now said
in the doc and checked.

## Checks

| Check | What it pins |
|---|---|
| `TestASuspendedOwnersServiceRefusesEveryCaller` | paused and banned; a stranger on the ACL, a daemon maintainer, the daemon owner and the service's own credential all `403`; the answer is not `404`; a positive control owned by an active user answers throughout; reactivation restores service |
| `TestAThirdPartyCannotReadASuspendedOwnersInbox` | the called-name check on the **read** path — the service's own credential is refused at the gate and never reaches `ConsumeAs`, so only an active, authorised third party can certify that branch. The held work survives the pause |
| `TestASuspendedOwnersTopicRefusesNewSubscribersButLetsThemLeave` | joining is a call to the topic; leaving is not |
| `TestSuspensionDestroysNothing` | record, queue, owner, user record and **both credential byte strings** survive a ban; only the daemon owner lifts one |
| `TestSuspensionIsNotTransitive` | one hop down, the person at the top is not the owner any more |
| `TestSuspensionDidNotSettleTheDrainQuestion` (core) | Q63's behaviour is what it was |
| `TestASuspendedOwnersRefusalIsCounted` | exactly one `suspended` per refusal, on the handler path and on the gate path |
| `TestPolicyChangesCancelBlockedReaders/owner` | a reader that is itself active and still on the ACL is released, because the service stopped answering under it |

## Mutation

Each break made, the named check watched to fail, the file restored.

| Mutation | Failed |
|---|---|
| called-name checks removed, `acting` kept | `TestASuspendedOwnersServiceRefusesEveryCaller` — a bystander sent successfully |
| the suspension outlives reactivation | the same — still `403` after lifting |
| pausing reaps the owner's queues | `TestSuspensionDestroysNothing` — *the ban discarded the queue: 0 held* |
| a suspended owner's service answers `404` | `TestASuspendedOwnersServiceRefusesEveryCaller` |
| **only** the `ConsumeAs` check removed | `TestAThirdPartyCannotReadASuspendedOwnersInbox` |
| **only** the `Subscribe` check removed | `TestASuspendedOwnersTopicRefusesNewSubscribersButLetsThemLeave` |
| `ownerSuspension` reaches a self-owned record | `TestSuspensionDidNotSettleTheDrainQuestion` |
| the credentials are forgotten on suspension (fixture perturbation) | `TestSuspensionDestroysNothing` — `401 bad token` |

The last three exist because codex ran them first and they **survived**: the
two single-check removals left both suites green, and the credential claim was
being made by a fixture that re-mints a token on every call rather than by the
daemon keeping one. Those were real holes in the test set, not in the code.

`src/smoke.sh --slow`: green.

## Peer review

opencode mapped the entry points before implementation and approved the diff,
conceding one of its own recommendations: it had asked for `IssueFor` to refuse
minting under suspension, and withdrew it after looking for a case where such a
credential would be usable and finding none — every route gates through
`Authenticate` → `acting`, so the mint is inert until the state is lifted, and
that is what *kept rather than revoked* already means.

codex reviewed independently and requested changes: the Q63 blocker above, the
transitivity boundary, and the three surviving mutations. It also corrected a
claim of mine about rescue paths — there is no daemon-owner override in
`manages`, and transfer is owner-only, so a suspended user's record is
recovered by lifting the state rather than by rank. No bypass was added.

## Not changed

`IssueFor`, `Manage`, `Unregister`, `Configure`, `Lookup` and `Activity` carry
no owner-suspension refusal. Administration of a record is not a call to the
service, and discovery hides rather than punishes: a record stays visible while
refusing calls, which is what makes `403` distinguishable from `404`.
