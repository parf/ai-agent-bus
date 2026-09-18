# Components

Draft for peer review. The set is small on purpose: nine page types, and the
hard part is information design rather than widget count.

## The set

| Component | What it is | Notes |
|---|---|---|
| Shell | Header with node identity and page title, navigation with the current entry marked, signed-in principal linking to Account, sign out | Exists as `shell()` and already carries sign-out everywhere ([inventory](review/current-state.md#routes-and-templates)). What changes is the destination set and correct page identity |
| Page title | One small image or glyph, the visible title, optional Refresh, and at most one short factual subtitle | [Title vocabulary](glyphs.md#page-title-images-and-glyphs); instructional prose belongs in Context help |
| Navigation | Seven destinations | Narrow screens use `details`/`summary`, not script |
| Section navigation | List categories with counts and register destinations for Services, Channels, Users and Groups, with the current entry marked | The register link is conditional on caller authority; [page map](information-architecture.md#navigation) |
| Task toolbar | Search, scope, filters, sort, result count, active filters, clear | GET only, state in the URL |
| Data table | Caption, scoped headers, stable order, one judgment column, owned-item marker, chosen narrow-screen columns; every numeric column is right-aligned with tabular figures | Hand-written because ours is URL-driven rather than client-side, and we found nothing supplying that. **Not** "the component nothing off the shelf supplies" — a universal we did not survey and do not need ([S15](review/codex.md#specification-review-round-one)) |
| Detail sections | Heading, definition list, and — where authority allows — the form for that one concern, collapsed until asked for | Read sections never depend on edit permission. There is **no trailing Manage block**: an edit lives in the section it changes, which is the whole point of splitting them |
| Form | Labels above controls, help beside the control, error summary and field errors, one primary action | |
| Line-list textarea | One plain identity, group member or ACL term per line; submitted lines survive validation and each refused line receives its own error | The same component edits group members, ACLs and the accepted Maintainers list; ACL additionally admits `*`. Glyphs never enter editable syntax |
| Context help | A visible `ⓘ` button opening a compact popover with a heading and short bulleted list | Keyboard, touch and pointer accessible; never a hover-only `title` attribute |
| Confirmation | Server-rendered page naming target and consequence | [forms](forms.md#consequential-actions) |
| Danger Zone | A red text link to a server-rendered resource subpage; not an always-visible panel | Contains Replace configuration, Transfer ownership and Remove registration only. Authority remains daemon-enforced; transfer and removal still continue to Confirmation |
| Status | Text, with colour and shape reinforcing it | [glyphs](glyphs.md#attention-levels) |
| States | Populated, empty, denied, unavailable | below |
| Graph | Inline SVG, labelled axes, shared range, value table beside it | No script, no canvas, no external chart library |
| Pagination | Previous and next as links carrying every filter | |

## Compact help, not prose walls

Pages do not lead with instructional paragraphs. The title may carry one short
factual subtitle; definitions, limits and usage guidance move behind a visible
`ⓘ` help control beside the title or the section it explains.

The shared native control is built in 0.5.65 on Services, Channels, Personal
and Users, replacing their leading definition walls. Other planned placements
remain with their owning page journeys.

The control is a real button using the native popover mechanism. It has an
accessible name such as *About service views*, works by keyboard and touch, and
opens a panel containing a heading and a short bulleted list. Bullets are one
idea each; if the explanation needs a page, the final bullet links to that page
instead of putting the page inside the popover.

Help never hides a current condition, refusal reason, field constraint needed
to complete a form, or the consequence of a destructive action. Those remain
in the page at the point of decision. The popover carries explanation, not
evidence the person must discover before acting.

## Owned items

Every caller-owned record is marked wherever it appears in a human list,
including All, My, Personal, search results and related-record lists. The mark
uses a blue leading rule and blue semibold name; the **My** category link uses
the same blue. It does not repeat **Yours**. A Personal record separately
carries the visible **Personal** word with orange bold emphasis, matched by the
**Personal** category link. Personal overrides the owned treatment when both
facts apply. None of these marks changes ordering, access or kind.

The record name is the one route to its read-first detail page. Controls on
that page follow the daemon's returned authority; the list does not duplicate
the same destination with an Edit link or infer edit authority from the visual
ownership mark. This compact treatment is built in 0.5.66.

## Small choice controls

A select hides choices that are few enough to show. **Two or three stable
choices do not use a select.**

| Choice | Control |
|---|---|
| Destination or URL-backed list state, such as All / Mine | links or submit buttons; the active value is visibly and programmatically marked, and GET state remains in the URL |
| A value inside a form | radio buttons in a labelled `fieldset` |
| Four or more choices, or a dynamic list | select, autocomplete or the appropriate list control |

This rule applies to scope, state, kind and mode wherever only two or three
values are offered. It does not turn a form choice into navigation or a URL
filter into form-only state. If a stable set grows from three choices to four,
the renderer changes it to a select while keeping the same field name and URL
values.

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
| `popover` with `popovertarget` | compact context help and a menu, where one genuinely earns itself |
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
