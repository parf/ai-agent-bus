# Web redesign

Design phase. **No web code is written while this is open**, by anyone.

The owner opened this on 2026-09-16 asking for: modern Go libraries, modern web
design, every page reviewed for an advanced UI carrying every required function
and field, forms that make sense, UX that makes sense, best practice, junk and
non-essential data hidden, the best available admin design, the
[glyph vocabulary](glyphs.md) applied, review from every peer, and design
documents at all levels before implementation.

## What each document owns

| Document | Owns |
|---|---|
| This file | Scope, principles, settled direction, review status |
| [Information architecture](information-architecture.md#pages) | Pages, navigation, URL map, what moves where |
| [Pages](pages.md#overview) | Page-by-page purpose, content, actions and states |
| [Components](components.md#the-set) | Shell, tables, toolbars, forms, status, empty and error states |
| [Data dictionary](data-dictionary.md#fields) | Every field: source, meaning, label, and when it is not shown |
| [Glyphs](glyphs.md#the-rule-that-matters-most) | The state vocabulary, and where it is allowed to appear |
| [Visual design](visual-design.md#tokens) | Principles, contrast obligation, acceptance, and the shipped asset layer |
| [Tokens](tokens.md#the-budget) | **The one home for every value**: the surface budget, both palettes with measured contrast, type, space and shape |
| [Layouts](layouts.md#the-frame-every-page-shares) | Representative arrangements, wide and narrow, and the five states per component |
| [Technology](technology.md#rendering) | Go rendering and asset decisions, with their reasoning |

Current facts about the built dashboard are **not restated here**. They live in
codex's [inventory](review/current-state.md#routes-and-templates) — fifteen
route patterns, nine templates, every field, form input and actual effect — and
its [findings](review/codex.md#junk-and-misleading-content) C01–C16, alongside
the earlier [audit](../done/web-review.md#findings) W01–W17. Specifications here
link to those; where a spec and an inventory disagree, the inventory is evidence
and the spec is wrong.

[web-interfaces.md](../web-interfaces.md#proposal) remains the earlier proposal
and the plan's topic file. It is not a second specification: where the two
disagree, this directory is newer, and the disagreement is a decision to record.

## Settled direction

Owner-answered 2026-09-16. Question IDs Q58–Q61 are **spent** and are not to be
reused ([question namespace](../../../CLAUDE.md#working-rules)).

| | Answer | Consequence |
|---|---|---|
| Q58 What "advanced" means | **Density and honest states, no script.** | The [no-script rule](../../../docs/05-discovery.md#rules-it-is-built-to) stands unchanged. Advanced is information design, not partial DOM updates. Sort, filter and paging are page round-trips |
| Q59 Design system | **A house layer: tokens plus layout primitives plus hand-authored ops components.** | No framework adopted. See [visual design](visual-design.md#tokens) for what ships and codex's [dissent](#dissent) |
| Q60 Go rendering | **Adopt `templ`.** | Build-time codegen, server-rendered, no bearing on the script rule. See [technology](technology.md#rendering) |
| Q61 Dark mode | **Build for it and ship it.** | Tokens carry both schemes from the start; contrast is checked in both |

## Principles

| | |
|---|---|
| A page answers a question somebody has | Not "here is what we have". The current homepage is seven sections of everything at once |
| Show what is wrong, not what exists | Most rows are ordinary. The design's job is to make the two that are not findable |
| Absence has four meanings and they are different | Measured zero, below display precision, not applicable, not observed ([glyphs](glyphs.md#absence-which-is-four-different-facts)) |
| Declared state and observed state are never merged | A record says it is enabled; a reader is either attached or not. Neither is health |
| One concern per form | Queue policy, access, configuration, ownership and removal are five decisions, not one page |
| Edit authority does not decide read authority | The daemon's visibility rules decide what a caller may read, always and only. Within what it has already returned, lacking permission to *change* a value must not additionally hide it — which is what happens today ([C04](review/codex.md#junk-and-misleading-content)) |
| Demote by task, do not discard | A full routing name disambiguates a session; a message ID serves diagnostics. The question is which page needs it, not whether anyone does |
| The face renders; it decides nothing | No authorization, no I/O and no derived truth in a template |
| A consequential action is confirmed on a page the server rendered | Because the consequence it describes is computed when it is shown. A dialog carried in the page states conditions as of that page's render; a server-rendered confirmation re-reads them. **Not** because a client dialog removes the daemon's recheck — it does not, the daemon rechecks at submission either way. That was my error, corrected where the rule lives ([forms](forms.md#consequential-actions)) |

## Review status

| Peer | Slice | State |
|---|---|---|
| codex | [Inventory](review/current-state.md) and [findings](review/codex.md) | round one delivered `5f01f90`; specification review S01–S15 delivered `6824bcf`. Verdict: **not implementation-ready** |
| opencode | [Research and UX critique](review/opencode.md) | delivered `4892883`, corrected `f41973e` |
| claude/home-parf | [Design research and field-level verification](review/home-parf.md) | delivered `aaac271`, with negative results and a density acceptance proposal |
| claude/ab-dvp | Architecture, page specs, documents, owner questions | drafted; correcting against round one |

**Round one: thirteen of fifteen closed.** S01–S11 and S14–S15 are answered in
the specifications, each against source rather than against the finding text.
What they changed is mostly truthfulness — a severity vocabulary the daemon
cannot support, an Account page that assumed a person profile, an error page
that lost distinctions the current build already makes, and promised values with
no dependency, which now have [named fallbacks](pages.md#owed-by-this-specification).

**Round one is answered in full.** S12 and S13 close with two new documents:
[tokens](tokens.md#the-budget) supplies the chosen values — both palettes with
every contrast pair computed rather than judged, a capped type and space scale,
and the reasoning for each cap — and [layouts](layouts.md#the-frame-every-page-shares)
supplies the arrangements at both widths. Theme and density controls are
**removed from MVP** rather than specified: the draft promised them in this
document and in no form inventory, which is how an unapproved control becomes an
assumed requirement.

home-parf's five density checks are now the [acceptance](visual-design.md#acceptance),
replacing a recorded mitigation that was a reason to expect a good outcome
rather than a check.

**Owner input needed** — [Q62](../QUESTIONS.md#open-questions): who owns the token file,
and whether a scheme or density control belongs in MVP at all. The recorded
density risk is conditional on nobody owning typography, so the checks make that
lapse visible but cannot substitute for the name.

The specifications have not yet been read by the owner; that remains
[F.13.0](../TODO.md#objective).

### Dissent

Recorded rather than resolved, because the owner has since decided Q59:

**codex disputes the evidence for excluding template families.** opencode's
"every themed admin template fails no-script" is overstated in both directions:
Tabler documents CSS with optional JS, and Bulma's own modal needs caller
script — so "all components work without JS" is not true of the recommended
fallback either. codex's position is that a like-for-like comparison of the
components we would actually use was owed before any winner, bespoke included.
That comparison was not run. The owner chose the house layer with
[home-parf's independent analysis](#review-status) reaching the same
recommendation by a different route, so the decision does not rest on the
disputed claim — but the claim should not be repeated as though it were
established.
