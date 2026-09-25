# Gate-to-mutation window — H.5.8

Closed 2026-09-15, shipped in 0.5.31. The rule is in
[what a call carries](../../../docs/02-access.md#what-a-call-carries); this file
is evidence, not a contract.

## Scope

Authority was established at the gate and acted on later with no hold between.
The gate takes the registry lock, answers, and lets go; core takes it again when
the operation runs. Everything decided in the first half described a moment that
had passed by the second.

| Shape | What it cost | Closed by |
|---|---|---|
| caller's record removed mid-request | it registered itself back into existence, because a name owned by itself was allowed to be created by a name that was nobody | the clause is gone; `register` takes an `enrolled` argument, so the one caller that legitimately creates a newcomer says so |
| caller paused or banned after the gate | served anyway | `acting` asked inside `may`, `manages`, `mayEditUser` and at each verb |
| `/token` settled ownership before minting | a transfer in between handed the **former** owner the **current** owner's credential — not a stale read, a credential the caller was never entitled to | `IssueFor` takes the caller and decides entitlement under the hold it mints in |
| `api.unregister` dropped the lock before `tokens.Forget` | the name could be claimed in between and *their* credential dropped; a failed store write left the record gone and its credential live | `UnregisterAnd`: forget first, abandon the whole removal if it fails, delete after |
| a transfer to a suspended name | the record ends up owned by somebody who cannot answer for it | `Manage` checks the new owner can act |
| a removed principal's blocked read | still being served on standing nobody has | `recheckInbox` asks `acting`; `Unregister` rechecks readers |
| inactive maintainer's group listing | refused by the API before core would have to | `Groups`, `Owned`, `Recent`, `Users` answer nothing to a caller that may not act |

## Checks

- `TestEveryVerbAsksWhoTheCallerIsWhereItActs` — 15 verbs by 2 callers: a name
  the daemon holds nothing for, and a paused **maintainer**, so what it is
  refused cannot be mistaken for never having had the authority. Asserts the
  refusal *reason*, not merely that it was refused: `401 credential` and
  `403 suspended` are different answers a caller acts on
  ([refusals](../../../docs/05-discovery.md#refusals)).
- `TestListingsShowNothingToACallerThatMayNotAct` — the reads, which have no
  error to return, so the refusal is the empty answer.
- `TestIssuingDoesNotHandOverACredentialOwnershipHasMovedOn`,
  `TestIssuingHoldsTheRegistryWhileItDecidesAndMints`.
- `TestRemovingAnAddressAndItsCredentialIsOneOperation`,
  `TestRemovingHoldsTheRegistryWhileItDropsTheCredential`.
- `TestARemovedCallerCannotRegisterItselfBack`,
  `TestATransferCannotHandARecordToSomebodyWhoCannotAct`,
  `TestARemovedPrincipalsBlockedReadIsReleased`.

**The concurrent cases are pinned by holding the callback open**, not by racing
and hoping to land in the window: the mint or the credential write blocks, the
conflicting operation is known to have been attempted, and the check is that it
*cannot proceed* meanwhile. The acceptance asked for both interleavings driven
under `-race`; this is the stronger form of the same assertion, because a race
that happens not to land proves nothing and passes anyway. The suite runs under
`-race` regardless.

## Mutations

Twenty-five, each caught by a named check, except one:

| Mutation | Result |
|---|---|
| `manages` asks only whether the caller is active, not whether it is anybody | **nothing caught it.** Every caller of `manages` already asks first, `may` included, so the strictness is defence in depth and no path distinguishes it. Kept, because the predicate should mean what its name says; recorded here rather than claimed as covered |

Two mutations were badly formed on the first attempt and are not counted: one
did not apply, and one added a second `forget` rather than removing the abort,
so it changed nothing. Both were rewritten, and both are caught.

## Fixtures

No ordinary call brings a self-owned name into existence any more, so fixtures
that wanted a principal state it instead of registering it — `known` and
`provision` in `core`, `known` in `api` and in the web command. That is how a
store loaded at start supplies one, and it is the shape enrolment writes.

## Not in this slice

[H.5.9](../TODO.md#objective) — `agent-bus-admin user add` still does not create
the user it adds, so a fresh install cannot onboard anybody under this rule.
A mapped-socket confinement hazard reported during this work belongs to G.1.3.
