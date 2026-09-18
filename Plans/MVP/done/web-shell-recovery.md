# Shared shell and recovery — F.13.2

📌 **TL;DR:** Nine of the twelve acceptance clauses were already pinned; the
audit found one real defect, two unpinned clauses and one title that two pages
of the same kind shared.

Completed requirement: F.13.2, whose acceptance row moved to
[DONE](../DONE.md#done--mvp); the contract now lives in
[shell and recovery](../../../docs/05-discovery.md#shell-and-recovery).
Earlier pieces of the same shell:
[header, footer and the public node call](node-identity.md#what-this-is-not),
[title marks](web-title-help.md#checks),
[form recovery and the keyboard skip](web-form-recovery.md#checks).

## Result

**Audit first.** Every clause was traced to a named check before anything was
written. Nine were already independently pinned and were left alone: current
navigation with exactly one marked entry; the shell on every page with one
`<main>` and one skip target; deep-link return after sign-in; foreign return
addresses refused in six spellings; return and filter state kept and local;
the retained-form allowlist and the second boundary in the template; missing
and hidden answering byte-identically below `<main>`; the permission refusal;
the unreachable bus.

**One real defect.** Two degraded sections rendered `err.Error()` straight into
the page. On a transport failure that string is the address this child talks to,
so the record page answered `Activity unavailable: Get
"http://127.0.0.1:PORT/activity?name=svc%40h": EOF` and Account replaced its
credential table with `Get "http://127.0.0.1:PORT/names": EOF`. It is the same
defect [the envelope feed had](web-overview-diagnostics.md#result), in the two
places that fix did not reach. `feedProblem` became `sectionProblem(what, err)`
and all three sections go through it: a refusal the daemon worded is shown,
unwrapped from its JSON body; anything else is logged and replaced with *the
daemon did not answer*, which says the section failed rather than leaving it
reading as an answer of nothing.

**The check that covered the first one passed over it.** It looked for the words
*Activity unavailable*, which the leak also satisfied — the hollow shape this
slice's predecessor found four times.

**Titles name their resource.** Two Service tabs, two identities, two groups,
two Danger Zones and the two confirmations each shared one generic title, which
is the thing the clause exists to prevent: the page's own rationale is that a
tab and a history entry can be told apart. Detail, confirmation and problem
pages now name the record, identity, group, action or refusal they are about.
Two not-found pages deliberately keep one title, and that sameness is asserted,
because telling a hidden record from an absent one is what the single answer
prevents.

Escaping was checked before the change, not argued: `html/template` escapes an
action in title context, so `</title><script>` in a record name comes back
inert. The comment on `shellTitle` now states that rule in place of the older
claim that caller text never reaches the title.

**Two clauses were true but unnamed.** The expired-session recovery and the
breadth of the shell had no check. Both are now asserted; neither needed a
product change.

**Two hollows in this slice's own first matrix**, found in review and closed.
It walked only Service instances of Danger Zone and the confirmations, although
all three take their section from the record's kind, so a Channel could regress
to Services unseen. And its two confirmations were different actions, so
dropping the record name from the title would still have left them distinct.
Both were reproduced with mutations before the matrix was widened.

## Corrections

| Found | Corrected to |
|---|---|
| Record activity printed the transport error | `sectionProblem("record activity", err)` |
| Account's credential section printed the transport error | `sectionProblem("held credentials", err)` |
| `feedProblem` named one section | `sectionProblem(what, err)`, used by all three |
| A failed section could read as an answer of nothing | It says the daemon did not answer |
| Service, Channel and Agent details shared one title per kind | The title names the record |
| Two identities shared *Identity details* | The title names the identity |
| Two groups shared *Group* | The title names the group |
| Two Danger Zones shared *Danger Zone* | The title names its record |
| Both confirmations shared *Confirm action* | The title names the action and its record |
| Every refusal shared *Problem* | The title is the refusal's own |
| *Add user* shared the identity page's title | Its own title |
| `shellTitle`'s comment forbade what its own callers now do | It states the escaping rule instead |

Three assertions in `journeys_test.go` and one in `truthful_test.go` carried the
old generic titles and were updated to the new ones. No other test changed.

## Checks

Four named checks, in `shell_recovery_test.go`:

| Check | Clause |
|---|---|
| `TestADegradedSectionSaysSoWithoutNamingTheBackend` | a failed section never names the backend, never reads as empty, and passes a daemon refusal through unwrapped |
| `TestExpiredPermissionAndUnavailableRecoveriesAreFourDistinctAnswers` | never signed in, session ended, refused for permission and bus unreachable are four different answers |
| `TestEverySignedInPageIsTitledUniquelyAndCarriesItsShell` | unique title, landmarks, skip target, account controls and the marked section, on every route |
| `TestAFailedRecordReadIsARefusalNotAnEmptyRegistry` | a failed registry read is a refusal, not a page saying nothing is registered |

The route matrix walks 32 GET routes and three POST confirmations, each with an
explicit expected navigation entry. Each route asserts one `<header>`, one
`<footer>`, one `<main>` pair, one skip link and one skip target, the sign-out
form *and its button*, the account link, and exactly one `aria-current=page` —
zero on a problem page, which belongs under no section. Public sign-in is the
positive control: it has none of the account controls and its own title.

**Two instances of every page that names a resource**, because one instance
cannot show a shared title, and **two of the same action** wherever the action
word would otherwise carry the difference by itself:

| Page | Instances |
|---|---|
| Record detail | two generic Services, two Channels, an Agent |
| Identity detail | two |
| Group detail | two |
| Danger Zone | two Services and a Channel |
| Ban confirmation | two |
| Credential removal | two |
| Removal confirmation | a Service and a Channel, both *remove* |
| Ownership transfer | one, beside the two removes |

Danger Zone, the confirmations and the record page take their section from the
record's own kind in `loadRecord`, so a Channel reaches all three and can
regress to Services on its own; each has a Channel instance and a mutation of
its own. The retained legacy topic URL, `GET /service?name=<channel>`, is the
same page at its old address: it is checked for the shell and for marking
Channels, and its title is asserted against the canonical page's rather than
counted as a collision.

Targeted mutations caught **32/32**:

| Break | Check that failed |
|---|---|
| Record activity prints the transport error | degraded section |
| Held credentials print the transport error | degraded section |
| The envelope feed prints the transport error | envelope section (F.13.5) |
| A failed section says nothing | degraded section |
| A daemon refusal arrives as its raw JSON body | degraded section |
| A daemon refusal is replaced by the generic text | degraded section |
| A record does not name itself in the tab | shell matrix |
| An identity does not name itself in the tab | shell matrix |
| A group does not name itself in the tab | shell matrix |
| A Danger Zone does not name its record | shell matrix |
| A confirmation does not name its action and record | shell matrix |
| A confirmation names its action but not its record | shell matrix |
| A Danger Zone names the zone but not its record | shell matrix |
| A ban confirmation does not name its subject | shell matrix |
| A credential removal does not name its subject | shell matrix |
| A Channel Danger Zone regresses to Services | shell matrix |
| A Channel confirmation regresses to Services | shell matrix |
| A record page stops following the record's kind | shell matrix |
| A refusal does not name the refusal | shell matrix |
| *Add user* takes the identity page's title | shell matrix |
| A page carries no title of its own | shell matrix |
| The account link is dropped | shell matrix |
| Sign-out keeps its form action but loses its button | shell matrix |
| The header landmark is dropped | shell matrix |
| The footer landmark is dropped | shell matrix |
| An ended session reads as a stranger | recovery |
| A stranger reads as an ended session | recovery |
| A permission refusal offers a credential | recovery |
| An unreachable bus reads as a refusal | recovery |
| Return state is dropped | `TestServiceDetailReturnIsLocalAndStateful` |
| A refused secret is echoed into its field | `TestDangerZoneNeverRendersRetainedConfiguration` |
| A failed registry read yields an empty page | failed record read |

Four hollow proofs caught **4/4**: the degraded check stops failing when no
section fails; the shell matrix stops passing when the account link is removed
from a fetched page and when the footer is removed from a confirmation; the
recovery check stops passing when the ended-session wording is removed from the
body it reads.

One mutation receives no credit. Adding `token` to the retained-form allowlist
at the `/user` call site survived, because the check on that helper is a direct
unit test with its own field list, nothing posts a token to that form, and no
template reads the key — the inert shape
[already recorded](web-form-recovery.md#checks). It was replaced by a mutation
at the boundary that can reach HTML: echoing the retained configuration back
into its textarea. A second mutation was replaced because it failed only by not
compiling.

`go vet` clean; the full Go suite green across 14 packages, and green again
under `-race`.

**One frozen run takes no credit.** The first frozen `--slow` was **red, 610/1**.
The failure was mine and it was real: the degraded-section fixture switched how
the backend failed by assigning a plain `string` that the `httptest` handler
goroutine read concurrently, so `-race` reported a write at the switch against a
read in the handler. It is now an `atomic.Bool`, which is the same fixture
without the shared write. Reproduced before fixing, and the mutation set was
rerun afterwards to confirm the atomic did not hollow the two checks that depend
on the mode actually changing: still **32/32** and **4/4**.

## Frozen run

The corrected byte-frozen `src/smoke.sh --slow` run passes **611/0**. Its log is
`tmp/f132-final/slow-final.log`; `go vet` and `go race` are green on lines 10
and 11. All **411** entries in
`tmp/f132-final/source-frozen-final.sha256` matched afterwards, and
`tmp/f132-final/manifest-final.log` contains only `OK` lines.

| Input | SHA-256 |
|---|---|
| `src/smoke.sh` | `3ca97eebf8522665203d9787d6fdd1dbcc4a065ae9d8c02ee079e8852ee26f52` |
| `tmp/scripts/f132-mutations.py` | `ce06d05f118a955447a90688089f35ce77e16aea1d068457875bbff82165ef9d` |

Both hashes were recomputed after the run. The evidence file itself and the
unrelated `.gitignore` edit were deliberately outside the manifest. The final
documentation sweep checked **180 Markdown files** and **2,916 local paths and
anchors**, with zero errors.

## Browser evidence

Real Chromium, headless, at 1280 px and 375 px, against a fixture whose section
reads and whose whole daemon can be taken away on demand.

| State | Observed at both widths |
|---|---|
| Healthy, 13 routes | one header, one main, one footer each; the skip link is the first keyboard stop; sign-out and the account link present; exactly one marked entry, and none on the not-found page; distinct titles including `Service orders@h` beside `Service billing@h`, and `Danger Zone · orders@h` beside `Danger Zone · news@h` |
| Channel routes | `/channel?name=news@h`, the legacy `/service?name=news@h` and `/service-danger?name=news@h` each render the Channel page and mark **Channels**, not Services |
| Never signed in | 401, `Sign in · agent-bus`, *sign in to open this page*, no account controls |
| Session ended | 401, the same form, *that session has ended — sign in to carry on* |
| Refused for permission | 403 on two routes, `Not yours to see · agent-bus`, the daemon's own reason, the shell intact, no sign-in offered |
| Daemon stopped under a live session | 502, `The bus is not answering · agent-bus`, the shell intact, no sign-in offered |
| Section failed | `Activity unavailable: the daemon did not answer` on the record page and *the daemon did not answer* under Credentials, with the rest of both pages still true |

No page named the backend address in any of these states; the degraded pages
were searched for `127.0.0.1:PORT` in the whole document, not only in the
visible text. No page-level horizontal scrolling at 375 px on any route.

Two runs take no credit. The first unavailable pass signed in after the daemon
had already been stopped, so it observed an anonymous visitor rather than a
session meeting a stopped bus; it was rerun with the session established first.
The first forbidden pass asked for a confirmation about a user the caller
cannot see, which is correctly a not-found rather than a refusal, so it did not
exercise the 403 at all; two routes that do were used instead. Neither attempt
caused a product change.

The overflow scan lists elements extending past the viewport unless the element
itself scrolls, which reports the entries inside the horizontally scrollable
navigation and footer. Those containers scroll, the document does not, and the
same false positive was [measured and withdrawn
before](web-overview-diagnostics.md#browser-evidence).

## Not implemented

Per-record titles stop at the resource name. A record whose name is long is not
truncated for the tab, and no page states its own filter state in the title.
Neither is required by the clause.
