# Components

Draft for peer review. The set is small on purpose: nine page types, and the
hard part is information design rather than widget count.

## The set

| Component | What it is | Notes |
|---|---|---|
| Shell | Header with node identity and page title, navigation with the current entry marked, signed-in principal linking to Account, sign out | Exists as `shell()` and already carries sign-out everywhere ([inventory](review/current-state.md#routes-and-templates)). What changes is the destination set and correct page identity |
| Navigation | Seven destinations | Narrow screens use `details`/`summary`, not script |
| Task toolbar | Search, scope, filters, sort, result count, active filters, clear | GET only, state in the URL |
| Data table | Caption, scoped headers, stable order, one judgment column, chosen narrow-screen columns | Hand-written because ours is URL-driven rather than client-side, and we found nothing supplying that. **Not** "the component nothing off the shelf supplies" — a universal we did not survey and do not need ([S15](review/codex.md#specification-review-round-one)) |
| Detail sections | Heading, definition list, and — where authority allows — the form for that one concern, collapsed until asked for | Read sections never depend on edit permission. There is **no trailing Manage block**: an edit lives in the section it changes, which is the whole point of splitting them |
| Form | Labels above controls, help beside the control, error summary and field errors, one primary action | |
| Confirmation | Server-rendered page naming target and consequence | [forms](forms.md#consequential-actions) |
| Status | Text, with colour and shape reinforcing it | [glyphs](glyphs.md#attention-levels) |
| States | Populated, empty, denied, unavailable | below |
| Graph | Inline SVG, labelled axes, shared range, value table beside it | No script, no canvas, no external chart library |
| Pagination | Previous and next as links carrying every filter | |

## States

Four, and the fourth is the one that is usually forgotten.

| State | Reads as | Never |
|---|---|---|
| Populated | the data | |
| Empty | what this page would contain, and how to create one | a bare table with a blank row |
| Denied | you may not see or change this, and who can | distinguishable from not-found, for a record you may not see |
| Unavailable | the daemon did not answer; retry | **an empty healthy list** ([W12](../done/web-review.md#findings)); a promise that nothing changed |

Empty is designed per page, not once: an empty Channels page must explain what a
channel is, or the Services/Channels split reads as a bug.

## Interaction without script

The owner has settled that there is no enhancement layer
([README](README.md#settled-direction)). That is not the 2023 version of the
constraint — the declarative pieces landed since then, and the plan uses them by
name rather than reinventing them:

| Native | Used for |
|---|---|
| `details` / `summary`, with `name=` for exclusive groups | narrow-screen navigation, filter disclosure, receipt evidence, help text |
| `popover` with `popovertarget` | a menu, where one genuinely earns itself |
| Invoker commands (`command` / `commandfor`) | disclosure and dialog invocation without script. Cross-browser since Safari 26.2, December 2025 |
| `:has()` | parent-conditional styling, replacing marker classes |
| container queries | table and card density by available width, not viewport width |
| `field-sizing` | inputs that fit their content |
| `:user-valid` / `:user-invalid` | validation styling that does not fire before the person has typed |
| `datalist` | suggesting owners, groups and record names without a combobox widget |

**`dialog` is not used for consequential confirmation**, which stays a
server-rendered page ([forms](forms.md#consequential-actions)) — because a
confirmation carried in the page describes conditions as they were when that
page was rendered, and a fetched one re-reads them. Not because it would remove
the daemon's authorization check, which it would not; that was my error and is
corrected where the rule lives.

What is genuinely given up, stated plainly: sort and filter are round-trips; no
instant search; no copy-to-clipboard; graph detail is limited to what SVG
`title` carries. Bounded data makes the round-trips cheap.

## Patterns copied, packages not

The owner chose a house layer over a framework
([README](README.md#settled-direction)). Patterns still come from people who
have solved this before — read, transcribe, credit; never depend on:

| Source | Taken |
|---|---|
| [Cloudscape](https://cloudscape.design/) (AWS) | The closest published prior art: built since 2016 for cloud operations consoles. Table density and collection preferences, property filter, empty/loading/error state guidance for resource tables. React, so nothing ships |
| [GOV.UK Design System](https://design-system.service.gov.uk/) | Error summary, check-answers before a consequential change. Progressive enhancement is doctrine there and tested, not claimed. Wrong density register for us — it is one question per page |
| [Shopify Polaris](https://polaris.shopify.com/) | Index tables and resource lists; the best published empty-state and admin content guidance |
| [Carbon](https://carbondesignsystem.com/) | Data-table usage guidance only. Its components are JavaScript; only tokens and guidance transfer |
| [WAI](https://www.w3.org/WAI/tutorials/) and WCAG 2.2 | Page structure, table semantics, reflow, contrast, pause-stop-hide |

## Accessibility, which is the real cost of building our own

Adopting nothing means adopting nobody's accessibility work. It is the strongest
argument against the direction the owner chose, and the mitigation is specific
rather than hopeful: the hard part is forms and errors, and GOV.UK's markup is
copyable pattern by pattern. Acceptance carries keyboard operation, landmarks,
programmatically associated table headings, measured contrast against **the one palette**
([settled](visual-design.md#theme-and-density-controls): there is no second
scheme to measure), and reflow at narrow and zoomed sizes.

The `<main>` landmark is currently opened by the shell and closed by two of nine
templates. Whatever else changes, the shell owns the whole document structure so
that a page cannot forget to close it.
