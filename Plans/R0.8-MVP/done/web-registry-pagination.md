# Web registry reader filter and pagination

📌 **TL;DR:** 0.5.77 adds one independent Readers filter and bounded,
URL-retaining registry pages.

## Result

Services, Personal and Channels filter the daemon's live numeric `Readers`
observation independently from the stored Delivery setting. The choices keep a
positive count, an explicit measured zero and an omitted/unavailable value
separate; their labels make no availability or health claim.

The face filters and sorts its one caller-visible `/ls` answer before selecting
a 25-row page. It bounds invalid page numbers, states the matching range and
retains search, category, owner, delivery, kind, Readers and sort state through
paging and a detail visit. Category counts remain totals before toolbar filters.

## Checks

Package tests exercise 30-record Service, Personal and Channel listings, page
two and an out-of-range page, one `/ls` call, the exact detail return URL, independent
Delivery and Readers filters, live positive and measured-zero counts, and a
unit matrix that refuses to collapse unavailable into zero.

Seven corrected targeted mutations fail independently: skipping the Readers
predicate, collapsing nil into zero, dropping pager state, paging before
filtering, replacing a category total with a page count, accepting a foreign
return URL and collapsing the caption at the narrow breakpoint. Two earlier
passes receive no credit: the first pager assertion found query values elsewhere
on the page, and the first category assertion matched the unchanged My count.
Both survivors and their corrected assertions are retained in
`tmp/web-registry-pagination/initial-survivor.log`.

Real Chrome against the current-source web child and live read-only daemon
rendered all four Readers choices and the selected measured-zero state at 1440
and 375 CSS pixels. Both widths had zero page overflow. The first narrow image
exposed a one-word-wide table caption; after the CSS and assertion correction,
the rerun measured the caption at 343 pixels across the 375-pixel viewport.

Fast smoke passes **489/0**. Final frozen slow smoke passes **610/0**, including
vet and race. All 400 frozen tracked files match after the run; the foreign
`.gitignore` edit and this measured evidence file were deliberately outside the
manifest. The tracked smoke script retains SHA-256
`a21ad6a6a022acecc28ec87162c2a009d2b9c9fe6c94362a0079915742bec1e5`.
Documentation validation checks **175 files** and **2,912 local links** with
zero errors.

## Scope

This advances F.13.3 without claiming the remaining populated role journeys,
the Overview/diagnostics work, installed browser acceptance or any daemon and
wire change.

## Live postflight

Commit `98bf992` was pushed before deployment. The live supervisor, bus and
public identity report **0.5.77**, stamped `parf@parf.us 2026-09-18 14:28:34`.
A signed-in production request to Services with measured-zero Readers and
Enabled delivery filters renders all four reader choices, the selected state,
matching range and Clear filters. No registry record was changed.

The restarted web child retains zero effective capabilities,
`NoNewPrivileges`, 256 MiB memory, zero swap, 64 tasks and one CPU. The
post-restart AgentBus accepted the OpenCode review/reconnection probe; delivery
alone is not credited as a peer reply.
