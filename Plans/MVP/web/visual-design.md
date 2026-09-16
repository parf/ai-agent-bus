# Visual design

Draft for peer review. The owner settled the direction
([README](README.md#settled-direction)): a house layer of tokens plus layout
primitives plus hand-authored components, with dark mode shipped.

## What ships

Three vendored files, no build step beyond what `templ` already adds, every byte
auditable inside a security-relevant binary.

| Layer | Content | Origin |
|---|---|---|
| Tokens | Colour, space, type scale, radii, shadows, easings | [Open Props](https://open-props.style/) or a copied subset. MIT |
| Layout | Stack, cluster, sidebar, frame, switcher | [Every Layout](https://every-layout.dev/) primitives, transcribed |
| Components | The nine in [components](components.md#the-set) | Ours |

No icon font, no webfont, no CDN, no external asset of any kind
([rules](../../../docs/05-discovery.md#rules-it-is-built-to)). Fonts are a local
system stack. Icons, where any are needed, are inline SVG.

For scale: the dashboard currently ships about thirty-nine lines of inline CSS
for the entire interface. This is not a migration — it is the first deliberate
design the project has had.

## Tokens

**The values live in [tokens](tokens.md#the-budget)**, which is their one home.
Nothing here restates them; a page that introduces a size or a colour rather
than using a token is a review failure.

What that file settles, and what changed from this document's first draft:

| | Decision | Changed from the draft |
|---|---|---|
| Colour | Neutral surfaces, one accent, three status colours carrying meaning only | — |
| Scheme | Both, as two whole palettes rather than one inverted | — |
| Type | System stack; six sizes, two weights, fixed steps | **fluid scale dropped.** It fights the 200% zoom requirement, and density should follow the container rather than the viewport |
| Space | One geometric scale with deliberate gaps | — |
| Depth | Surface and border only | **shadows dropped.** Elevation this interface does not have, and the first thing to look wrong in dark mode |
| Numerics | Tabular figures in every compared column | — |

Density is one variable rather than a thousand edits, which is what makes the
narrow-screen and reflow requirements tractable at all.

## Theme and density controls

**Neither ships in MVP, and the draft should not have promised them** — codex's
[S12](review/codex.md#specification-review-round-one) was right that they
appeared in this document and in no form inventory, which is how an unapproved
control becomes an assumed requirement.

| | MVP | If it is later wanted |
|---|---|---|
| Colour scheme | **follows the operating system**, via `light-dark()` over the token set. No control | a form in the shell posting to a preferences route, which sets a cookie the shell reads to emit `data-theme`. A server round-trip, no script, and the token set already supports the override |
| Table density | **one comfortable default**, with a container query tightening a table in a narrow column. No control | the same mechanism |

Both are deliberately cheap to add later and neither is free now: a preference
needs a route, a cookie, a persistence scope and a decision about whether it
follows the person or the browser. That is [Q62](../QUESTIONS.md#open-questions), and it
is the owner's, not ours.

## Contrast

Measured, in **both** schemes, and in acceptance rather than by eye. The audited
build shipped muted text at 3.54:1 — below the AA minimum for ordinary text —
and the current `#6b6b6b` is a fix that was never checked against a second
palette because there was not one. Two palettes double this obligation; opencode
and home-parf both raised it independently.

**The obligation has already paid.** Computing the ratios while choosing the
palette caught two failures that looked entirely reasonable on screen: the input
outline in both schemes measured 2.36 and 2.34 against the darkest surface,
against the 3:1 a UI component owes. Every pair is tabulated in
[tokens](tokens.md#measured-contrast) and the acceptance test recomputes them
from the hex values.

Colour is never the only carrier of meaning. Every status glyph sits beside a
word ([glyphs](glyphs.md#the-rule-that-matters-most)).

## Hierarchy

| | |
|---|---|
| Page title and purpose | what this page answers |
| Task toolbar | search and filters, where a list |
| Primary data | the table, or the identity and its operational summary |
| Secondary detail | inheritance, digests, evidence |
| Actions | grouped by concern, primary action visually singular |

Operational problems come before implementation digests. The audited service
page led with a configuration digest and the current homepage leads with node
counters; neither is what somebody arriving is asking.

## Graphs

Inline SVG or nothing. Shared time range across series, labelled axes in real
timestamps, explicit units, stated interval, partial buckets differentiated, and
the value table beside the graph as the accessible path to the numbers. A zero
series is summarised rather than given the same height as real traffic.

## Quiet states

A brief factual statement and a next action. An empty table and a flat chart do
not get most of the screen. Emptiness must never read as breakage — which is the
zero-versus-absent distinction one level up
([glyphs](glyphs.md#absence-which-is-four-different-facts)).

## Refresh

A visible observation time and an explicit Refresh. No automatic reload: it
replaces what somebody is reading and it moves focus. This was already removed
once ([W11](../done/web-review.md#findings)) and should not return.

## Acceptance

home-parf's framing, taken: the mitigation recorded under
[risk](#risk-recorded) is *a reason to expect a good outcome, not a check*. The
falsifiable version is **swap the token file for a deliberately bad one — wrong
scale ratio, tight leading, two extra weights, a fourth text colour — and
something must go red.** If only a reviewer's opinion catches that, the risk is
unmitigated whatever this document says.

Five checks, in the order they are worth adding:

| | Check | Catches |
|---|---|---|
| 1 | **Shipped-surface budget, enforced by a test over the stylesheet**: the [caps](tokens.md#the-budget) — 6 sizes, 2 weights, 3 text colours, 1 accent, one space scale. Build fails over the cap | accretion, which is the actual mechanism. Nobody chooses eleven font sizes; they arrive one page at a time and each is defensible alone. **Adopt this one if only one is adopted** |
| 2 | **Contrast as a unit test over token pairs, in both schemes**, recomputed from the hex values | [W11](../done/web-review.md#findings) recurring. It already caught two failures during design |
| 3 | **Numeric density targets**: a minimum row count visible at 1280×800 and 1366×768, *and* the same table passing 1.4.4 at 200% zoom and 1.4.10 reflow at 320 CSS px with no horizontal page scroll | "comfortable density" being unfalsifiable. Rows-above-the-fold catches too airy; zoom and reflow catch too tight. Together they pin it from both sides |
| 4 | **The worst page first.** Services at ~200 records with pathological content — 74-character address, absent description, description duplicating the address, one at capacity, one disabled, one external, one with backlog and no reader, CJK, a very long realm — built and reviewed **before page two exists** | nine pages built on a scale tuned against the Overview and all wrong together. **This is the check that expires**: it is free today and unavailable the moment a second page exists |
| 5 | **A five-state proof sheet per component** — populated, empty, denied, unavailable, long-name — as one static page, gating that component before it is used anywhere ([layouts](layouts.md#the-five-states-per-component)) | empty and denied becoming afterthoughts, which is what happens when they are validated last rather than first |

**Which check rejects which mutation.** codex's
[R2-6](review/codex.md#review-of-the-round-one-response) is right that the caps
alone cannot reject the mutations the framing promises: six arbitrarily spaced
sizes still satisfy a six-size cap, and tightening every line height leaves it
untouched. Each named mutation needs a bound that bites:

| Mutation | Rejected by |
|---|---|
| A seventh size, a third weight, a fourth text colour | check 1, the counts |
| **Wrong scale ratio** | a **ratio bound**, not a count: adjacent steps must stay within 1.12–1.30, computed over the sorted size list ([tokens](tokens.md#type)) |
| **Tight leading** | a **minimum line height per size band**: 1.4 at and below `--text-base`, 1.2 above it |
| A fourth accent, a decorative status colour | check 1, plus the rule that status colour appears only beside a word |
| Too airy | check 3's minimum row count |
| Too tight | check 3's zoom and reflow bounds |

**Check 3's numbers are derived, in the value home**: **9 service rows** at
1280×800 and **8** at 1366×768, comfortable density, two-line identity cells,
with the shell, title block and toolbar present. The derivation and every input
are in [tokens](tokens.md#density-and-the-row-capacity-it-implies).

The first draft asserted 18 and 16 instead. Both are impossible — 18 comfortable
rows need 956px against an 800px viewport — and codex caught it by arithmetic.
That is this check's own failure mode appearing inside the document that defines
it: a density target chosen beside the tokens rather than from them is
decorative, however precise it looks. The number now moves when a token moves.

**Contrast over declared pairs is not contrast in the built page.** Check 2
proves the token table is sound; it cannot prove a component used those tokens.
Rendered verification of the built pages, in both schemes, stays a separate
requirement and is not discharged by the unit test.

Check 4 also fixes the fixture problem: the installation's live data is too
quiet to validate incident presentation, so fixtures supply the exceptional
cases — and a fixture capture is not evidence of what operators actually do.
A **restart-shaped fixture** is required among them, so that a counter label
claiming "since start" cannot pass review (codex's
[S01](review/codex.md#specification-review-round-one)).

## Risk, recorded

Building our own means nobody arrives knowing it, and it will look amateur if
nobody owns typography and density. This is the strongest argument against the
chosen direction and it was raised by the peer who nonetheless recommended it.
The mitigation is that the component surface is genuinely small and the patterns
are copied from people who solved this already
([components](components.md#patterns-copied-packages-not)); the residual risk is
real and is accepted knowingly.

**The risk is conditional, and the condition is ownership.** It reads *amateur
if nobody owns typography and density*. The five checks above make ownership
visible when it lapses; they do not create it. One named person, every token
change through them, and a page that introduces a size instead of using a token
failing review — that is what the checks are checking for. Naming that person is
a standing commitment rather than a technical decision, so it is the owner's:
[Q62](../QUESTIONS.md#open-questions). Accepting a risk is not the same as being able to
see it arrive.
