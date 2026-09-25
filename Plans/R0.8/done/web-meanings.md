# F.13.1 — correct data meanings and narrowly missing read facts

**Later decision (2026-09-17):** Q70 is settled by the [single Readers count](../../../docs/05-discovery.md#readers). The open-question wording below records the state when this work shipped.

Shipped in 0.5.36. Evidence for the acceptance in [TODO](../TODO.md#web-redesign)
as that row stood before removal. The [data dictionary](../web-handoff/data-dictionary.md#the-rule)
owns the vocabulary; this records what the pages now say and what proves it.

## Scope

The audit found that almost every misleading value was a true fact presented as
a different one ([W01–W17](web-review.md#findings),
[C01–C16](../web-handoff/review/codex.md#junk-and-misleading-content)). The rule is that
**declared state, observed state and health are three things**, and the pages had
one word for all three. This task fixes the words and adds the read facts whose
absence forced a reader to guess. It changes no daemon behaviour: the one export
it adds, `api.Reasons`, reports a table the daemon already had.

## What changed

| | |
|---|---|
| [services listing](../../../src/cmd/agent-bus-web/admin.go) | one Status column became **Delivery** (declared), **Reader** (observed) and **Reached** (a caller's hint); the caption says the rows are what the caller may see, not a count of the node |
| [service detail](../../../src/cmd/agent-bus-web/admin.go) | split under *What the record declares* and *What the daemon observed*; an unset bound or TTL says what it inherits instead of showing a guessed number; `Full` says drop-the-oldest or refuse |
| [diagnostics](../../../src/cmd/agent-bus-web/main.go) | node totals are labelled node-wide and every list below them says it is the caller's; the refusals table draws every reason; the backlog and registry tables use the same three words as the listings |
| [refusals](../../../src/cmd/agent-bus-web/views.go), [`api.Reasons`](../../../src/internal/api/server.go) | a supported reason absent from a status answer is a **measured zero** and is drawn as one; a reason the face does not recognise is still shown |
| [directory](../../../src/cmd/agent-bus-web/users.go) | three identity kinds named rather than guessed, and the two counts on the page say the face computed them over what the caller may see |
| [smoke](../../../src/smoke.sh) | two checks matched wording this task replaced |

## What proves it

[`meanings_test.go`](../../../src/cmd/agent-bus-web/meanings_test.go). Every test
puts the shapes it distinguishes on **one page at once**, because a label is only
wrong beside the thing it should have said instead.

A check against the whole body is worth nothing here, and six of these were that
before codex pulled them apart: the page carries the vocabulary it is being asked
about in its legend, its filter options, its headings and its management form, so
each of those matched with the value removed. Claims about a value are now read
out of the row (`row`) or the table (`section`) that states it, and where a
number carries the claim the fixture makes the numbers unequal.

| Distinguished | Test |
|---|---|
| enabled / disabled, never "inactive" | `TestDeliveryIsEnabledOrDisabledAndNeverInactive` |
| a reader observed, never "serving" or "offline" | `TestAReaderIsObservedAndIsNeverCalledOfflineOrServing` |
| reached another way ≠ a reader | `TestExternalDoesNotStandInForTheReaderObservation` |
| pub/sub, queue, and a record that declares neither | `TestPubSubAndQueueDeliveryAreNamedAndNeitherIsGuessed` |
| which reads the reader column counts, and which it leaves out | `TestTheReaderColumnSaysWhichReadsItCounts` |
| the delivery setting is not an answer about the next send | `TestTheDeliverySettingDoesNotClaimASendWouldBeAccepted` |
| counters' scope, and dequeued is never "completed" | `TestQueueCountersSayTheirScopeAndNeverSayCompleted` |
| an unset setting inherits rather than being guessed | `TestUnsetQueueSettingsAreStatedAsInheritanceRatherThanGuessed` |
| a measured zero refusal / a reason never observed | `TestEverySupportedRefusalReasonIsDrawnIncludingItsZero` |
| node-wide totals / caller-visible lists | `TestNodeTotalsSayTheyAreNodeWideAndListsSayTheyAreYours` |
| when a queue observation was true | `TestQueueObservationsSayWhenTheyWereTrue` |
| user, registered name, credential with neither | `TestTheDirectoryNamesThreeKindsOfIdentityAndGuessesNone` |
| the empty unclassified category reads as the populated one | `TestTheUnclassifiedCategoryReadsTheSameWayEmptyAsPopulated` |
| the registry's own column names, and a queue observed at its bound | `TestQueueCountersSayTheirScopeAndNeverSayCompleted`, `TestQueueObservationsSayWhenTheyWereTrue` |
| a face-computed count / a daemon-reported one | `TestFaceComputedCountsAreMarkedAsTheFacesOwn` |
| ordinary, maintainer and owner on one fixture | `TestOneFixtureReadsDifferentlyForOrdinaryMaintainerAndOwner` |

Twenty-six mutations, each caught by its named check. On the directory: merge
the two other identity kinds into one word; label them *Unclassified*; drop *by
this page* from the counts; drop the empty section instead of saying it is
empty. On authority: offer every caller Manage; offer none. On delivery: merge
the two modes; state a mode for a record that declares none; label every row in
the listing Enabled; present the setting as availability on the detail page;
claim availability in the listing's legend. On the reader bit: blank the backlog
table's reader column; count a filtered waiter as a reader; call a filtered
reader nobody; drop the note saying which reads the column leaves out; say a
filtered read will not take a message; remove the send from `deliver` while
still dropping the waiter. On queues and counters: rename the registry's four
columns; swap accepted and dequeued; show neither declared bound nor declared
TTL; render an absent oldest as `0`; render a queue at its bound as *full*; drop
*at capacity* from the listing and the detail page only. On refusals: zero every
count; drop the warning that the counts are partial; stop naming which refusals
the warning is about.

Nine of them were found by codex and opencode, running mutations and sweeps
against this tree that my own set had missed, and five more came out of
correcting what those fixes then claimed — twice over, on the reader bit, where
the first correction replaced one false sentence with another.

The registry pair was found by a mutation that **survived**. `reader` and `held
now` also head columns of the backlog table directly above the registry, so a
page-wide match for them was satisfied by a registry with no headings at all —
the same hollow shape as the legend above. Claims about one of several stacked
tables are now read out of that table.

## What it turned up

| | |
|---|---|
| `Register` clears `Disabled` and normalises `Full` | so "unset — refuse" was unreachable copy, and the fixture turns delivery off through `Manage` as an owner does |
| a daemon maintainer does not manage another's record | `manages` is owner, the name itself, or the record's own maintainers group (core/manage.go). The listing is right and the first draft of the test was wrong |
| nothing found creates a self-owned record with no profile | registration refuses a name that answers for nobody, a handover refuses an owner that is not already one, re-registration keeps its owner, and enrolment writes a profile as well — so the directory fixture restores the shape into the store. The directory has to render it either way, because a snapshot can carry it |
| a 404 from `lookup` is counted nowhere | refusals are counted in `reply`; a handler calling `fail` directly bypasses the counter. Reported, not changed: what the refusals table can claim is bounded by that, and the fixture's control drives a refusal through `reply` |
| delivery is not a declaration | `visible` answers the stored bit OR the name having stopped being active (core/manage.go), so the detail page states it under its own heading rather than under what the record declares |
| the reader bit excludes a read restricted to a topic or tag | the label said *no reader waiting*, which is false while such a reader is attached — and the first correction said it "will not take the next message", which `deliver` disproves by serving a matching one of those first. The cell now says **no unfiltered reader** on every face, each stating which reads it leaves out. Whether to report them as a fact of their own stays [Q70](../QUESTIONS.md) |
| Enabled is not availability | `visible` merges the stored bit with the name being inactive, but a suspended *owner* is checked at `Send` — so a record reads Enabled while every send to it is refused. The page calls it the delivery *setting* and says it does not establish that a send would be accepted |
| not every refusal is counted | a refusal decided before the shared path that maps an error to a code is answered and counted nowhere — an unparseable body, an invalid name in a token request, a lookup of a name the daemon does not hold, a consume on a missing topic. The page said these were recorded counts rather than all refusals, and the contract with it. **Closed by [H.5.10](refusal-counting.md#scope) in 0.5.37**: every refusing path now calls one counter, and both said so again |
| a waiter leaving the list is not a message arriving | the first version of the filtered-read proof asserted only that the waiting count fell, which a daemon that dropped the message would satisfy too — codex overlaid exactly that and it passed. The reader helper now returns what `ConsumeAs` received, and the check reads the body |
| the listing's legend kept the withdrawn claim | the detail page stopped saying a record *takes delivery now* and its sibling three hundred lines away did not, with no check banning it there. Found by opencode, and it is the same shape as the CLI row below: one surface corrected, its sibling left behind |
| the CLI contradicted the dashboard about the same bit | `ab ls` rendered a protocol hint *ahead* of the reader observation, so an external record with a reader attached showed `-`. The observation now comes first, and the table carries a legend saying what the column counts |
