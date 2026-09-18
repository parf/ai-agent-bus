# Overview and Diagnostics — 0.5.80

📌 **TL;DR:** Root became a short evidence-based Overview; Diagnostics kept the
detail and stopped repeating the registry.

Contract: [what it shows](../../../docs/05-discovery.md#what-it-shows).
Page spec: [Overview](../web/pages.md#overview-),
[attention levels](../web/glyphs.md#attention-levels).

codex wrote the slice. This file records the audit, the corrections and the
evidence; the mutation and browser work below is mine.

## Result

Root renders Overview: observation time and an explicit Refresh, a **Needs
attention** list, the node strip labelled node-wide, and a Find row. Diagnostics
keeps refusals, inboxes holding messages, retained exchanges and loss, and no
longer carries the registry catalogue — Services and Channels now show Accepted
and Dequeued, so nothing that table uniquely exposed was lost.

**Attention items are enumerated, not judged.** An ordinary backlog is not an
item; a queue worker between pulls is exactly that shape. One record yields at
most one item, and the item keeps every other fact that applied, so a loss is
never hidden behind a current queue condition. When nothing is observed the page
says so and says explicitly that this is not a statement that everything is
working.

Levels follow the accepted vocabulary: an unclean previous stop is red, a queue
at capacity when observed is red and names its configured overflow policy as a
setting, cumulative loss and a disabled record holding work are orange, and a
cumulative refusal total is informational. `Disabled` outranks `AtBound`,
because nothing is being accepted and capacity is not what is wrong with the
record; the capacity and the policy stay on the item as supporting facts.

Two filters back the Find row: `work=held` on Services and Channels, carried
through the toolbar, the owner selector and the clear-filters link.

Two error paths stopped printing transport text. The unavailable-bus page
dropped `Detail: err.Error()`; a failed `/recent` call, which leaves the rest of
Diagnostics standing, now shows a refusal the daemon worded or one stable
sentence, and logs the rest. Both messages carried the backend socket or TCP
address to any signed-in reader.

## Corrections made during the audit

| Found | Fix |
|---|---|
| The unclean-stop card linked to `/diagnostics#node`; Diagnostics has no such anchor and deliberately has no node section | link to `/#node`, Overview's own node strip, labelled "View node totals" |
| `/recent` failure assigned `err.Error()` straight to the page, and rendered the daemon's raw `{"error":…}` body on a refusal | `feedProblem`, with `busMessage` factored out of `formRefusal` |
| Unclean was orange, cumulative loss red, cumulative refusals red | red, orange and informational, per the page spec |
| At capacity did not name the configured overflow policy | `whenFull`, reusing the record page's wording |
| `AtBound` was chosen before `Disabled` | precedence reversed, with capacity and policy kept as supporting facts |
| Thirteen smoke checks read moved content at root | pointed at `/diagnostics` and `/services`; the uptime check follows the labelled node strip |
| A fourteenth, HTTPS-only and therefore invisible to the fast subset, still read the exchanges section at root | the TLS fetch reads `/diagnostics`; the certificate, cookie and absent-body assertions are unchanged |
| "the page carries no token anywhere on it" still inspected Overview and the account page alone, so Diagnostics and Services lost their guard when the wall split | it inspects all four, and is renamed for what it now covers |
| "no records or registry totals" asked root, which carries no records for anybody now and so could no longer fail | it asks an anonymous `/services`, pinning the catalogue path itself |
| `identity_journeys_test.go:80` asked root whether credentials had come back, while its own failure said "duplicated on Diagnostics" — true when root **was** the wall, hollow after the split | it asks `/diagnostics`, the page a duplicate would return to |
| One smoke check asserted no refusal reason may show a zero | inverted: every supported reason appears, including measured zero ([refusals](../../../docs/05-discovery.md#refusals)) |

## Checks

`go test ./cmd/agent-bus-web/` passes. Five named tests in `overview_test.go`
cover the split, the admitted set and its levels, the holding-work filter, every
attention link's destination, and the sanitised feed failure.

**Seventeen mutations, each run from a clean baseline, each caught by a named
check.** The runner is `tmp/scripts/f135-mutations.py`.

| Mutation | Check that failed |
|---|---|
| root renders Diagnostics | `TestOverviewIsShortAndDiagnosticsKeepsTheEvidence` |
| an ordinary backlog becomes an item | `TestAttentionItemsAreEnumeratedAndOnePerRecord` |
| unclean stated orange | same |
| cumulative refusals stated red | same |
| cumulative loss stated red | same |
| at-bound chosen before disabled | same |
| at capacity stops naming its overflow policy | same |
| one record emits a second item | same |
| the winning condition drops `Dropped`/`Expired` | same |
| Diagnostics regains a Registry section | `TestOverviewIsShortAndDiagnosticsKeepsTheEvidence` |
| Diagnostics loses the loss section | same |
| the `work=held` filter stops filtering | `TestHoldingWorkLinksAreRealFilters` |
| the feed failure prints `err.Error()` again | `TestAFailedEnvelopeSectionDoesNotExposeTheBackendAddress` |
| the unavailable-bus page restores `Detail` | `TestRefusalsRecoverInsteadOfDeadEnding` |
| the Services table drops Accepted and Dequeued | `TestQueueCountersSayTheirScopeAndNeverSayCompleted` |
| the Channels table drops Dequeued | `TestChannelJourneyNamesModesAndWorkWithoutServiceLanguage` |
| the unclean link points at `/diagnostics#node` | `TestEveryAttentionLinkPointsAtASectionThatExists` |

The runner fails a mutation whose `-run` pattern matches no test, because a
pattern matching nothing exits zero and reads as "survived".

Fast smoke passes **490/0** after the smoke corrections above.

### The first frozen slow run does not count

A frozen `--slow` run finished **610 passed / 1 failed**, retained at
`tmp/f135-final/slow.log`. The failure was "the dashboard answers https when it
is given a certificate", the one moved-registry check the fast subset cannot
reach: the HTTPS half is behind `if slow`, so the sweep that repointed the other
thirteen never executed it. That run is superseded and takes no credit.

The first diagnosis of it — that the assertion should move to `/services` — was
wrong, read from the missing service name alone; the assertion calls
`sect exchanges` and also proves the body stays absent, so `/diagnostics` is the
route. codex caught this on rereading the source before any edit was made, so
nothing was built on it. Recorded because a diagnosis nobody writes down is one
somebody repeats.

Both strengthened checks were then mutated to grep for a string every page
carries — `name=token` and `AgentBus` — and both failed, which is what proves
they inspect four live responses rather than passing on an empty variable.

Injecting `<h2>My names</h2>` into `diagnosticsPage` fails
`TestAccountAndUserJourneysFollowDaemonFacts` at `identity_journeys_test.go:84`,
which is the same proof for the credentials-duplication check: against root it
would have survived.

Every remaining root fetch in the Go tests was read. Two remain —
`overview_test.go:25` and `meanings_test.go:511` — and both assert Overview's
own content: the page markers, and the node strip saying it is node-wide and
need not agree with any list.

A focused reproduction, `tmp/scripts/f135-tls.sh`, builds the binaries, starts
the daemon and a TLS web child, signs in over HTTPS and passes **5/0**: the
secure cookie, the exchanges section over TLS, the absent body, the logged
scheme, and a control proving root over HTTPS carries no exchanges section at
all. It reproduces one block and is not a substitute for the frozen run.

## Browser evidence

Headless Chromium against in-process fixtures on current source, after the
level and overflow corrections. Desktop is 1280×800, narrow is 375×900. Every
capture is under `tmp/f135-audit/`.

| State | Sizes | Observed |
|---|---|---|
| Populated | desktop, narrow | four items in level order: at capacity red with `when full: refuse`, delivery off and loss orange, refusals informational; node strip; Find row. Diagnostics carried refusals, two held inboxes, six retained exchanges and one loss row, with `/channel?…` and `/service?…` links correct per kind |
| Empty | desktop, narrow | "No observed attention conditions in this view", with the sentence that this is not a health claim. Diagnostics showed every supported refusal reason at measured zero, "every queue you can see is empty", "nothing lost", and "No envelopes in your retained history. This is not a count of all traffic." |
| Partial failure — only `/recent` fails | desktop, narrow | 200, the rest of Diagnostics intact, "Envelope history unavailable: the daemon did not answer", no address and no raw JSON |
| Daemon gone under a signed-in session | desktop | 502, "The bus is not answering", no backend address, footer "Node information unavailable" rather than a fabricated zero |

`document.scrollWidth` equalled `clientWidth` on every capture: no horizontal
page scroll at 375 px. Two containers extend past the viewport there and both
are `overflow-x:auto`, so nothing is unreachable — the header navigation
measured 279 px visible against 516 px of content, the footer 375 against 397.
The two elements this slice adds, `.attention-list` and `.node-strip`, measured
343 against 343 inside the 16 px gutter. No UX defect was found to fix.

## Not implemented

[The Overview page spec](../web/pages.md#overview-) admits two further items that this
slice does not emit, and does not supply:

- **Services of a suspended owner** needs the owner's user state, which the
  Overview's two answers (`status` and `/ls`) do not carry. A second question
  would break the rule this file's views are built on.
- **Unregistered credentials awaiting review** is owner-decidable and has no
  accepted transport.

The suspended-owner item is an accepted requirement, not a dropped one: it is
tracked as [F.13.7](../TODO.md#web-redesign), which owes a decision about where
that state comes from before any page can show it. The credential cohort stays
owner-decidable and does not gate anything. F.13.5 ships the derivable
Overview/Diagnostics journey, and nothing on the page claims the fuller set.

## Frozen run

`src/smoke.sh --slow` passes **611/0** over the corrected tree, logged at
`tmp/f135-final/slow-final-2.log`, with `ok go vet` and `ok go race` on lines
10 and 11 and no failing line anywhere. All **409** files in
`tmp/f135-final/source-frozen-final.sha256` matched afterwards:
`final-manifest-check.log` is 409 `OK` lines and nothing else.

| | SHA-256 |
|---|---|
| `src/smoke.sh` | `3ca97eebf8522665203d9787d6fdd1dbcc4a065ae9d8c02ee079e8852ee26f52` |
| `tmp/scripts/f135-mutations.py` | `cb55883eeb39ac9225f889c4c0eb36fdb5c618be5c6ef78e54fc93e18041d97e` |

Both recomputed here rather than copied, and the counts, the vet and race lines
and the manifest result read out of the logs directly. The freeze's entry for
`identity_journeys_test.go` is `696582f0…3eb695`, which is the corrected file on
disk — the check that distinguishes this run from the one before it.

`.gitignore`, which belongs to another worker, and this file, which records the
run, are deliberately outside the freeze.

Documentation validation reports **179 Markdown files** and **2,917 local
links** with zero errors. That is codex's whole-tree measurement from the
integration run, including the untracked evidence files; the anchor checker used
while drafting covered only the documents this slice touched.

### Two earlier runs take no credit

Both were green over a tree that has since been corrected, and both are kept
because a suite passing over a hollow assertion is the thing worth knowing
about.

| Run | Result | Why it does not count |
|---|---|---|
| `slow.log` | 610/1 | the HTTPS dashboard check still read the exchanges section at root; the HTTPS half sits behind `if slow`, so the sweep that repointed its thirteen siblings never executed it |
| `slow-final.log` | 611/0 | green, but `identity_journeys_test.go` was inside its 409-file manifest and still asked root whether credentials had reappeared on Diagnostics |

Twenty-one mutations were caught in all: seventeen against the slice's source,
two against the strengthened smoke checks, one reintroducing the credentials
duplication on Diagnostics, and one confirming the TLS assertion's route. The
focused TLS reproduction passed 5/0.

## Live postflight

Commit `6d34dde` "Complete web overview and diagnostics" deployed as **0.5.80**.
The public identity reports `v0.5.80` with build
`parf@parf.us 2026-09-18 17:35:17`. Polled every two seconds across the restart,
the dashboard answered 0.5.79 at t+0, t+2 and t+4 and 0.5.80 by t+10, so it was
unavailable or still old for under about ten seconds and returned on its own.
The AgentBus face reconnected in the same session without being restarted.

Signed in against the running node, read-only, with no live registry mutation:
Overview is short and carries no Refusals section; Diagnostics carries Refusals
and Exchanges and no Registry; Services shows Accepted and Dequeued; the
`work=held` filter renders as selected; and no backend address appears on
Overview, Diagnostics or Services.

The web child runs with `CapEff` all zero, and its environment is exactly
`AGENT_BUS_ADDR=/bus.sock` and `PWD=/`.

**Who measured what.** The version string, the build stamp, the restart window
and the reconnect were observed here against the anonymous page. `CapEff` was
read here from the supervised child's `/proc` entry. The signed-in page contents
and the environment listing are codex's measurements: the pages need a
credential for the live node, which is not mine to handle, and that process's
`environ` is not readable by this account.
