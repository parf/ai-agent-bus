# Visual design

Draft for peer review. The owner settled the direction
([README](README.md#settled-direction)): a house layer of tokens plus layout
primitives plus hand-authored components — **one design, no themes**.

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
| Scheme | **One palette, light.** No second scheme and no control | **dark mode dropped.** Owner-settled 2026-09-16: one good design rather than two that both need maintaining |
| Type | System stack; six sizes, two weights, fixed steps | **fluid scale dropped.** It fights the 200% zoom requirement, and density should follow the container rather than the viewport |
| Space | One geometric scale with deliberate gaps | — |
| Depth | Surface and border only | **shadows dropped.** Elevation this interface does not have, and a shadow is how a flat layout starts pretending to be a stack of cards |
| Numerics | Tabular figures in every compared column | — |

Density is one variable rather than a thousand edits, which is what makes the
narrow-screen and reflow requirements tractable at all.

## Theme and density controls

**Neither ships in MVP, and the draft should not have promised them** — codex's
[S12](review/codex.md#specification-review-round-one) was right that they
appeared in this document and in no form inventory, which is how an unapproved
control becomes an assumed requirement.

**Owner-settled, 2026-09-16: there is nothing to control.** One palette and one
density, so there is no scheme to switch and no preference to store — no
preferences route, no cookie, and no page whose appearance depends on state the
daemon does not hold.

| | MVP |
|---|---|
| Colour scheme | **one palette** ([colour](tokens.md#colour)). Not a default with an override; the only one there is |
| Table density | **one comfortable default**, with a container query tightening a table in a narrow column |

The container query is the one thing that still adapts, and it is not a theme:
it responds to the width a table actually has, which is a fact about the layout
rather than a preference about the person.

## Contrast

Measured in acceptance rather than by eye. The audited build shipped muted text
at 3.54:1 — below the AA minimum for ordinary text — because `#888` was picked
by eye; opencode and home-parf both raised the obligation independently.

**The obligation has already paid.** Computing the ratios while choosing the
palette caught a failure that looked entirely reasonable on screen: the input
outline measured 2.36 against the darkest surface, against the 3:1 a UI
component owes. Every pair is tabulated in
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
| 2 | **Contrast as a unit test over token pairs**, recomputed from the hex values | [W11](../done/web-review.md#findings) recurring. It already caught two failures during design |
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

**But not while mutation-testing.** The expected counts are pinned to the
approved token set, and the check compares a rendered page against *those*
numbers. Recomputing the requirement from whatever stylesheet is loaded would
let an inflated one derive its own lower target and pass — the too-airy mutation
excusing itself. codex's caveat, and it is the difference between a check and a
tautology.

**Contrast over declared pairs is not contrast in the built page.** Check 2
proves the token table is sound; it cannot prove a component used those tokens.
Rendered verification of the built pages stays a separate requirement and is not
discharged by the unit test.

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
failing review — that is what the checks are checking for. Accepting a risk is
not the same as being able to see it arrive.

**Owner-settled, 2026-09-16: the daemon owner owns
[the token file](tokens.md#the-budget).** Every change to a token goes through
them, and a page that introduces a value instead of using one is theirs to
refuse.

It is an existing role rather than a new appointment
([groups and maintainers](../../../docs/01-identity-and-roles.md#groups)),
which is what makes it durable: there is no post to leave vacant, and whoever
holds the daemon holds this. **One design makes the post affordable** — there is
a single palette, six sizes and one space scale to keep, not two schemes to keep
in step, and the review this asks for is small enough that a person with another
job can actually do it.
