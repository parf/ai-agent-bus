# Overview node strip and section marks — 0.5.82

📌 **TL;DR:** Three owner instructions on the rendered Overview, and the two
hollow assertions found while pinning them — one shipped in 0.5.81, one written
the same afternoon.

Owner-steered refinement of [F.13.5](web-overview-diagnostics.md#checks); the
contracts stay in [overview and
diagnostics](../../../docs/05-discovery.md#overview-and-diagnostics), [what a
node says about itself](../../../docs/05-discovery.md#what-a-node-says-about-itself)
and [glyphs](../web/glyphs.md#the-same-marks-in-the-navigation). No requirement
row closed here.

## What the owner asked

| Asked | Read as | Built |
|---|---|---|
| *"now when we have statistics in the middle of home page / move Calls minute/hour/total to the middle too"* | the counters leave the shared footer for the Overview node strip | moved, not copied: the footer keeps Owner and Uptime |
| *"Records there is Services + Channels? right? change title then"*, then *"or Agents + Services + Channels?"* | the strip's record count must name the kinds it sums | `Services + Agents + Channels`, which is what `len(b.records)` is |
| *"duplicate refresh date"* | the observation time is stated once | only beside Refresh; cut from the empty state and from every attention item |
| *"We have glyphs for every top level node - add them to top titles too"*, corrected to *"im talkiung about menu not H1s; check image"* with a screenshot of the nav row captioned *"<< no glyphs"* | the section navigation carries the marks | each entry starts with its own section's title mark, decorative |
| *"change glyph for overview (atm it duplicates agent bus logo)"* | Overview needs a mark of its own | 🏠; `titleMark("overview")` literally returned the header's bus logo |

The nav marks are the owner's corrected reading, recorded as a decision and as a
spec section rather than left as layout prose. They are a **second
owner-selected exception** to the quiet-glyph rule, and the reason is that the
rule is about data rows: a fixed seven-item wayfinding row carries no status and
spends no attention budget.

## Result

**Where the counts are published changed, not what.** The 2026-09-16 closed list
is unchanged and `GET /identity` still answers the counts to a caller with no
credential; the dashboard simply no longer prints them before sign-in. Both
halves are stated in the owning section and in a dated
[decision](../../../docs/decisions.md#settled). Owner and uptime keep their
public placement.

**One CSS change beyond the five items.** A three-line
`Services + Agents + Channels` label pushed its own figure below the other six,
so `.node-fact` became a column with the figure at `margin-top:auto`. Alignment
of stated values, not decoration.

**The strip was a four-column grid.** Seven facts in it left an empty cell
rendering as a solid block of the grid's own border colour, so it is a wrapping
flex row: one row at 1280, and 2/2/2/1 at 375 with the last fact spanning.

## Checks

Four named checks in `src/cmd/agent-bus-web/overview_strip_test.go`, plus one
pre-existing public-page check widened in `frame_test.go`.

| Check | What it pins |
|---|---|
| `TestTheNodeStripCarriesTheCallCountersAndNamesWhatItCounts` | the counters are in the strip and not also in the footer; the sum names its three kinds and counts all of them; the help beside it describes the facts instead of counting them, names Agents, and explains the counters |
| `TestTheObservationTimeIsStatedOnce` | exactly one occurrence, on an empty Overview and on one raising two items |
| `TestEveryMenuEntryCarriesItsSectionMark` | every `navItem` carries its own `titleMark`, each decorative **per entry**; Overview's mark is not a drawn bus mark and the menu does not repeat the node logo |
| `TestTheStripReportsCallsInTheThreeStatesTheDaemonCanBeIn` | measured / window-not-sampled / counter-never-bound, against a bound counter; an unobserved window never reads as a measured zero |
| `TestLoginUsesOnlyPublicDaemonFactsAndSeparatesBuilds` | an unauthenticated visitor gets no counters, no strip element and no fabricated legacy values |

### Mutations

`tmp/scripts/strip-mutations.py`, final run: **19 mutations, 5 hollow proofs, 0
unaccounted.** Every one is credited by a named check failing on an assertion,
not by a compile error.

| Instruction | Mutants |
|---|---|
| counters moved | never reach the strip; in the footer as well; an anonymous visitor reads them again |
| the sum names its kinds | called `Records` again; names two of its three kinds; help still counts the facts; help still leaves Agents out; help sends a reader to an Agents page; help does not explain the counters |
| time stated once | repeated per attention item; repeated in the empty state; not stated at all |
| menu marks | no entry carries a mark; one entry loses its mark; a mark is read out beside the word it repeats; Overview takes the header's bus logo back |
| the three counter states | an unobserved window reads as zero; an unbound counter is fabricated as zero; a measured count dressed as a note |

**Two mutants took no credit on their first run and were rewritten.** Both were
the same defect in the mutation itself: a change that fails to compile proves
only that the symbol was wrong.

| First form | Why it took no credit | Rewritten as |
|---|---|---|
| Overview's mark `return busLogo` | no such symbol; `FAIL [build failed]` | `return template.HTML(nodeLogo)`, caught by *the menu repeats the node logo* |
| states hollow proof `_ = stats` | unused variable; `FAIL [build failed]` | binds an empty `CallStats`, caught by the measured assertion |

### Three runs that took no credit

Kept rather than dropped, because a run that cannot be trusted is not a run that
did not happen.

| Run | Why no credit |
|---|---|
| two mutants, first form | credited by a compile error rather than by a named assertion; rewritten as above |
| the first frozen `--slow` | it was red on two new smoke checks, and I edited `smoke.sh` at ~line 2021 while the run was already past that point. `bash` reads a script incrementally by byte offset, so **every** later result in that run was read from a shifted file. The two failures were real; the rest of the run is void either way, so it was killed and rerun on the corrected script |
| the second frozen `--slow`, 622 passed / 0 failed | green, on bytes that were not the shipped ones: the Agents-page copy error was found after it finished. Superseded by the run below rather than deleted |

**The two smoke failures had two separate causes**, and only one was the obvious
one. `grep 'nav aria-label=."sections."'` is a basic regular expression, so the
`.` before `"sections"` ate the opening quote and the literal `"` had nothing
left to match. The second was mine alone: I asserted
`</svg></span>Diagnostics</a>`, but only the emoji marks are wrapped in a span —
`titleMark` returns a bare `<svg class=page-title-mark …>` for channels,
activity, diagnostics and problem. Fixing the quotes would have left that check
failing against correct HTML. Both marks are now asserted in their two distinct
shapes, with a third line failing if any one entry is left unmarked.

### Two hollow assertions found

Both are the family this work keeps turning up: a check that passes for a reason
other than the one it claims.

| Where | Passed because | Now |
|---|---|---|
| my own menu test, written this slice | the decorative check scanned the whole nav, so stripping `aria-hidden` from **one** mark left the attribute present elsewhere | asserted per `navItem`, on the mark itself |
| my own help assertion, first form | it banned *three/four/five/six values*, so *These seven values* would pass — the same brittleness it was removing | asserts the sentence positively; the fixture fact-count stays as the anti-empty fatal |

The first was exposed by a mutation that removed the mark from the Groups entry
alone; the second by codex's review. Neither was visible in a green suite.

### Stale ripple swept

Every document that stated the old placement or the old strip was found and
corrected, and the sweep is listed so a later reader can check it was complete
rather than convenient.

| Document | Was | Now |
|---|---|---|
| [what a node says about itself](../../../docs/05-discovery.md#what-a-node-says-about-itself) | the public closed list included calls served, shown in the footer and on the sign-in page | the placement is revised and the audience change is stated in three places, including what it costs |
| [overview and diagnostics](../../../docs/05-discovery.md#overview-and-diagnostics) | the strip was four node-wide facts | seven, with the label and the once-only time named |
| [glyphs](../web/glyphs.md#page-title-images-and-glyphs) | Overview's mark was *the inline AgentBus mark* | 🏠, plus the navigation exception as its own section and a row in the where-allowed table |
| [information architecture](../web/information-architecture.md#navigation) | a plain nav row | the row notes each entry's mark |
| [pages](../web/pages.md#overview-) | the empty state carried `as of 14:22` | the time is stated once, and why |
| [layouts](../web/layouts.md#overview--the-only-page-allowed-to-be-short) | the Overview mock showed a four-cell `Records` strip, a heading time, a per-item `observed 14:22` and an empty state time | the mock is the seven-fact strip and the once-only time; this is current layout guidance, not history, so it was corrected rather than annotated |
| `src/smoke.sh` | three `has` checks for the counters in the anonymous footer | three absence checks there and nine signed-in Overview checks |
| `Plans/MVP/done/design-qa.md` | recorded a `footer Owner/Uptime/Calls` observation | a dated correction line; the measurement itself is not rewritten, because it is what was observed then |

**One rendered-copy error the sweep found late.** The rewritten help said *the
Services, Agents, Channels and Users pages*, which names a page that does not
exist — Agents are rows on Services, and `navItems` has seven entries, none of
them Agents. It now says *Agents are listed with Services*, and the check pins
the relationship rather than the word: it fails if the help names an Agents
page, and fails loudly if `navItems` ever gains an Agents entry, because then
both the copy and the check need rewriting. Caught by codex after a green
`--slow`, which is why that run is recorded as superseded below rather than as
the shipped evidence.

## Frozen run

`src/smoke.sh --slow` on the final bytes passes **622/0**, log
`tmp/smoke-082-final.log`. `go vet` and `go race` are its lines 10 and 11, both
green. `go test ./... -count=1` is green across all 14 packages with tests, and
the web package is green under `-race` on its own.

| Fact | Value |
|---|---|
| run started | 19:02:18 (log birth time) |
| run ended | 19:03:57 |
| last write to any source or specification file | 19:02:14, four seconds before it started |
| files written during the run | this evidence document alone, at 19:02:49 |

`src/smoke.sh` contains no reference to `docs/` or `Plans/`, so editing this
document while the run was in flight could not reach it. That is the same trap
that voided the first run from the other direction: **`bash` reads a script by
byte offset as it goes**, so editing `smoke.sh` itself mid-run corrupts every
later line it reads.

All **406** entries in `tmp/f1382-final/source-frozen.sha256` — every tracked
file under `src/`, `docs/` and `Plans/`, plus the changelogs, the design QA record and
the new test this slice adds — matched afterwards. This post-run evidence file
is deliberately excluded: including a document that reports its own manifest
would make its recorded hash stale as soon as the result was written.
`tmp/f1382-final/manifest.log` has no line that is not `OK`.

| Input | SHA-256 |
|---|---|
| `src/smoke.sh` | `dee9b79d7ed7f51c17fe779faab7b06c7c4594fc954e674a466633574c29601a` |
| `src/cmd/agent-bus-web/main.go` | `91c706e0c1126901da5a70013a240f24a9db3349acd908e4c7c96bfef9a6f13d` |
| `src/cmd/agent-bus-web/overview_strip_test.go` | `0162fbb7fbee6ccadff07f9abab289c8160ebfa2c572af722ba6fade07da66fe` |
| `tmp/scripts/strip-mutations.py` | `1e1420d07be151d77c0b277317370d7fbe90cccd2ffd6524079e656e6111e1e6` |

The documentation sweep resolves every local path and anchor across the
repository's Markdown with zero errors.

## Browser evidence

Chromium (`/usr/bin/google-chrome`, headless, Playwright) against a disposable
0.5.82 daemon: four records of three kinds, one queue at its bound.

| Measured | 1280 | 375 |
|---|---|---|
| strip facts | 7, one row | 7, rows of 2/2/2/1 |
| figures on one baseline | yes | yes |
| document horizontal scroll | none | none |
| `as of` occurrences in body text | 1 | 1 |
| nav entries carrying a mark | 7/7, every one `aria-hidden="true"` | 7/7 |
| footer | `Owner admin@srv1 Uptime 2m46s` | same |
| anonymous page | no counters, no `<div class=node-strip>`, sign-in form present | — |
| `GET /identity` with no credential | answers `calls` with both windows | — |

Screenshots: `tmp/overview-082-desktop.png`, `tmp/overview-082-narrow.png`,
`tmp/overview-082-anonymous.png` (scratch, not tracked).

**Measured and carried forward to [F.13.6](../TODO.md#web-redesign), not
hidden:** the section navigation already overflowed and scrolled horizontally at
768 and 375 **before** these marks — 564 against 465 CSS px, and 516 against 279.
With the marks it is 743 and 695. The marks lengthen an existing horizontal
scroll rather than creating one, and the document itself still does not scroll
sideways at either width. Diagnostics sits further out of the initial view at
375 than it did.

## Live postflight

Measured on the deployed node after `67816d9`, not on a fixture. Daemon
`agent-busd 0.5.82`, build `parf@parf.us 2026-09-18 19:14:48`, systemd main PID
2226725, active since 19:15:47. Signed in as `claude/ab-dvp@parf.us` with that
identity's own credential — the owner's browser session was neither used nor
needed, because the strip is node-wide and any signed-in principal sees it.

| Postflight item | 1280 | 375 |
|---|---|---|
| node facts in the strip | 7 | 7 |
| `Services + Agents + Channels` | 11 | 11 |
| `Calls, minute` / `hour` / `total` | 49 / 49 / 49 | present |
| strip rows, and figure baselines | 1 row, 1 baseline | 4 rows, 4 baselines |
| `🏠` on the Overview title and menu entry | yes | yes |
| menu entries carrying a mark | 7/7, all `aria-hidden="true"` | 7/7 |
| occurrences of `as of` in the page text | 1 | 1 |
| footer | `Owner parf@parf Uptime 3m56s` | same |
| `Calls` anywhere in the footer | no | no |
| document scrolls horizontally | no | no |

One baseline per row is the alignment fix holding: at 375 there are four rows
and four baselines, not seven.

**Anonymous, on the live node:** title `Sign in · agent-bus`, no `.node-strip`,
the word `Calls` nowhere in the page, the sign-in form present, and the footer
still `Owner parf@parf Uptime`. `GET /identity` over the daemon socket with no
credential still answers `calls` with both windows available. Both halves of the
placement change hold in production.

**Every element overflowing its viewport is inside the scrolling navigation**,
and nothing outside it, at either narrow width:

| Width | nav scrollWidth / clientWidth | overflowing elements | all inside nav | document scrolls |
|---|---|---|---|---|
| 1280 | 743 / 743 | 0 | — | no |
| 768 | 743 / 379 | 1 | yes | no |
| 375 | 695 / 279 | 12 | yes | no |

The live figures match the disposable-fixture measurement exactly, which is the
point of recording both.

### Confinement, rechecked after the restart

Read from the live cgroup and the sandboxed child's own `/proc` entry.

| | Required | Measured |
|---|---|---|
| `memory.max` | 256 MiB | `268435456` |
| `memory.swap.max` | 0 | `0` |
| `pids.max` | 64 | `64` |
| `cpu.max` | 1 CPU | `100000 100000` |
| `CapEff` / `CapPrm` / `CapBnd` | empty | `0000000000000000` for all three |
| `NoNewPrivs` | set | `1` |
| PID namespace | private | `pid:[4026534004]` against init's `pid:[4026531836]` |
| user namespace | private | `user:[4026534005]` against init's `user:[4026531837]` |
| what the child's root contains | the binary and the socket only | `agent-bus-web bus.sock dev etc proc tmp` |

## Not implemented

- Whether the nav should wrap or collapse at narrow widths. It scrolls today and
  scrolled before; deciding it belongs to F.13.6's viewport work.
- The `∅`/`—`/`¿` rendering question and the CSS-versus-emoji proposal stay
  [open](../web/glyphs.md#open); this slice added no new glyph semantics.
