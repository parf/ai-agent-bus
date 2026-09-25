# Tokens

Draft for peer review. **The single home for every value.** A page that
introduces a size, a colour or a spacing rather than using a token here is a
review failure, not a local decision — codex's
[S13](review/codex.md#specification-review-round-one) asked for chosen values
rather than adjectives, and this is them.

Principles and rationale live in [visual design](visual-design.md#what-ships);
this file is the numbers.

## The budget

The surface is capped, and the cap is enforced by a test over the built
stylesheet rather than by review opinion. This is home-parf's first acceptance
check and the reasoning behind it is the reason it is first: **nobody chooses
eleven font sizes.** They arrive one page at a time and each is defensible
alone, which is precisely why a per-page judgement cannot stop them.

| | Cap | Why this number |
|---|---|---|
| Font sizes | **6** | Four would force either a cramped table or an undersized heading; seven starts expressing hierarchy through size instead of weight and space |
| Font weights | **2** | Regular and semibold. A third weight is how a system starts looking designed-by-committee |
| Text colours | **3** | Primary, secondary, and the inverse used on a filled control. Status colours are not text colours — they carry meaning and are counted separately |
| Accent | **1** | Navigation, links and the primary action. A second accent means two things are both the most important |
| Space scale | **1** | Every margin, pad and gap is a step on it |
| Status colours | **3** | Red, orange, blue. Green is [not used](glyphs.md#attention-levels) |

## Colour

**One palette.** Owner-settled, 2026-09-16: one design, no themes and no second
scheme ([what ships](visual-design.md#what-ships)). It is light, because an
operations console is read beside other light tooling and every published
system this plan borrows from defaults that way
([components](components.md#patterns-copied-packages-not)).

Every value below is **measured, not judged** — the ratios are computed from the
hex values, and they are what the acceptance test recomputes. The audited build
shipped muted text at 3.54:1 because `#888` was picked by eye
([W11](../done/web-review.md#findings)); nothing here is picked by eye.

| Token | Value | Role |
|---|---|---|
| `--surface-1` | `#fbfbf9` | page |
| `--surface-2` | `#f2f2ee` | table stripe, card |
| `--surface-3` | `#e8e7e2` | header, toolbar, table head |
| `--border` | `#d2d0c9` | dividers and table rules (decorative, exempt from 3:1) |
| `--border-strong` | `#87847b` | input and control outlines — **a UI component, so 3:1 applies** |
| `--text-1` | `#1a1a17` | primary |
| `--text-2` | `#56544c` | secondary |
| `--text-on-accent` | `#ffffff` | text on a filled control |
| `--accent` | `#1d5fa8` | links, navigation, primary action, focus ring |
| `--red` | `#a8271b` | attention |
| `--orange` | `#8a5000` | notable |
| `--blue` | `#1d5fa8` | informational (the accent, deliberately — informational is not a fourth hue) |

### Measured contrast

Worst case per token across all three surfaces, which is the number that has to
pass. Ordinary text needs 4.5:1; a UI component outline needs 3:1.

| Pair | Ratio | Needs |
|---|---|---|
| `text-1` on any surface | 14.09 | 4.5 |
| `text-2` on any surface | 6.13 | 4.5 |
| `accent` / `blue` on any surface | 5.21 | 4.5 |
| `red` on any surface | 5.71 | 4.5 |
| `orange` on any surface | 5.25 | 4.5 |
| `border-strong` on any surface | 3.02 | 3.0 (non-text) |
| `text-on-accent` on `accent` | 6.45 | 4.5 |
| `text-on-accent` on `red` | 7.07 | 4.5 |
| focus ring on page | 6.23 | 3.0 (non-text) |

**One of these was a failure when first drawn.** `--border-strong` began at
`#9a978e`, which measures 2.36 against `--surface-3` — below the 3:1 an input
outline owes. It looks entirely reasonable. The computation is what caught it,
which is the case for the test existing.

`--border` is deliberately *not* held to 3:1: a table rule is decoration, and a
divider forced to 3:1 draws a grid louder than the data in it. Where a boundary
carries meaning it is `--border-strong` or it is not a boundary.

## Type

System stack, no webfont, no CDN
([rules](../../../docs/05-discovery.md#rules-it-is-built-to)).

```
--font-ui:   system-ui, -apple-system, "Segoe UI", Roboto, sans-serif
--font-mono: ui-monospace, SFMono-Regular, "Cascadia Mono", Menlo, monospace
```

Monospace carries addresses, fingerprints, digests and commands — everything
where a character has to be read individually rather than a word recognised.

| Token | Size | Line height | Used for |
|---|---|---|---|
| `--text-xs` | `0.75rem` | 1.4 | table metadata, the second line of an identity cell |
| `--text-sm` | `0.875rem` | 1.45 | table body, form help, toolbar |
| `--text-base` | `1rem` | 1.55 | prose, definition lists |
| `--text-lg` | `1.125rem` | 1.4 | section headings |
| `--text-xl` | `1.375rem` | 1.3 | page title |
| `--text-2xl` | `1.75rem` | 1.2 | the single figure on a node strip |

Six sizes, at the cap. The ratio is roughly 1.15–1.25 per step — deliberately
tight, because an operations console expresses hierarchy through weight, space
and position rather than through size. A 1.5 scale would make the page title
shout across a dense table.

**Weights: 400 and 600.** Nothing else. Table headers are 600 at `--text-xs`
with slight letter-spacing rather than a third weight or a fourth colour.

**No fluid type.** The draft promised a fluid scale; it is dropped. Fluid sizing
interacts badly with the 200% zoom requirement — the viewport-relative term
resists the zoom it is supposed to respond to — and a table's density should
follow its container, which is what container queries are for
([components](components.md#native-interaction-and-the-one-local-behavior)). Fixed steps, container
queries for density.

**Tabular figures everywhere numbers are compared**: `font-variant-numeric:
tabular-nums` on every numeric column, so a column of counts can be scanned
down rather than read across.

## Space

One scale, geometric, `0.25rem` base.

| Token | Value |
|---|---|
| `--space-1` | `0.25rem` |
| `--space-2` | `0.5rem` |
| `--space-3` | `0.75rem` |
| `--space-4` | `1rem` |
| `--space-6` | `1.5rem` |
| `--space-8` | `2rem` |
| `--space-12` | `3rem` |

Missing steps (5, 7, 9…) are missing on purpose: the gaps are what stop the
scale becoming a continuum.

| Measure | Value |
|---|---|
| Table cell padding, comfortable | `--space-2` vertical, `--space-3` horizontal |
| Table cell padding, dense | `--space-1` vertical, `--space-2` horizontal |
| Section gap | `--space-8` |
| Label-to-control | `--space-1` |
| Control-to-help | `--space-1` |
| Page gutter, narrow | `--space-4` (16px, which is the reflow requirement's floor) |
| Content maximum | `76rem` |

## Shape and depth

| Token | Value | Note |
|---|---|---|
| `--radius-1` | `3px` | inputs, buttons, chips |
| `--radius-2` | `6px` | cards and panels |
| `--focus-ring` | `2px solid var(--accent)`, `2px` offset | **never** `outline: none`; measured above |

**No shadows.** The draft's token list carried them; they are dropped. Elevation
implies layering this interface does not have, and surface plus border already
separates everything that needs separating. A shadow is how a flat layout starts
pretending to be a stack of cards.

## Density, and the row capacity it implies

**Derived from the values above, not chosen beside them.** The first draft named
18 rows at 1280×800 and 16 at 1366×768; codex did the arithmetic and both are
impossible — 18 comfortable rows need 956px, more than the entire viewport,
before any shell. Picking a density target independently of the tokens that
produce it is the failure the acceptance check exists to catch, committed in the
file that defines the check. So the number is computed here, where the inputs
live, and it moves when they do.

At a 16px root, the line boxes are `--text-sm` 20.3px, `--text-xs` 16.8px and
`--text-xl` 28.6px.

| Consumer | Height | From |
|---|---|---|
| Shell header | 73.6 | two `--text-sm` rows at `--space-2` vertical, plus a 1px border |
| Page title block | 76.9 | `--text-xl` + `--space-1` + `--text-sm` + `--space-6` |
| Task toolbar | 102.9 | three `--text-sm` lines, `--space-3` vertical, borders, `--space-4` gap |
| Table head | 33.8 | `--text-xs` at `--space-2` vertical, plus a rule |
| **Chrome total** | **287.2** | |

A two-line identity row is `--text-sm` + `--text-xs` + `--space-2` twice + a 1px
separator: **54.1px comfortable, 46.1px dense**.

| Viewport | Height for rows | Comfortable | Dense |
|---|---|---|---|
| 1280×800 | 512.8 | **9 rows** | 11 |
| 1366×768 | 480.8 | **8 rows** | 10 |

Those are the [acceptance](visual-design.md#acceptance) minima. **Nine is fewer
than it feels like it should be, and that is information rather than a problem
to argue with**: a two-line row costs what it costs, and the levers are in this
file — the vertical padding, or the decision that identity takes two lines at
all. Raising the target without moving a token is how a density requirement
becomes decorative.

## Container queries, not breakpoints

One default, one alternative, applied per table by a container query on
available width rather than by a viewport breakpoint. A narrow column in a wide
window gets the same treatment as a narrow window, which is the behaviour a
viewport breakpoint gets wrong.

There is **no user-facing density control in MVP** — see
[visual design](visual-design.md#theme-and-density-controls).
