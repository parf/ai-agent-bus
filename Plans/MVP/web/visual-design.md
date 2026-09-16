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

Density is one variable rather than a thousand edits. That is what makes the
narrow-screen and reflow requirements tractable at all.

| Token group | Decisions |
|---|---|
| Colour | Neutral surfaces; one restrained accent for navigation and primary actions; state colours reserved for meaning and never decoration |
| Scheme | Both, from day one. `light-dark()` over the token set, with an explicit `data-theme` override so a person can choose rather than only inherit |
| Type | System UI stack for prose and data; monospace for addresses, fingerprints and commands. Fluid scale so narrow screens are designed rather than shrunk |
| Space | One scale. Comfortable table density by default, with a denser setting available per table |
| Numerics | Tabular figures and consistent alignment in numeric columns, so a column can be scanned rather than read |

## Contrast

Measured, in **both** schemes, and in acceptance rather than by eye. The audited
build shipped muted text at 3.54:1 — below the AA minimum for ordinary text —
and the current `#6b6b6b` is a fix that was never checked against a second
palette because there was not one. Two palettes double this obligation; opencode
and home-parf both raised it independently.

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

## Validation before implementation

The design is reviewed against populated, empty, denied, unavailable and
long-name examples before any of it is built. The installation's live data is
too quiet to validate incident presentation, so fixtures supply the exceptional
cases — and a fixture capture is not evidence of what operators actually do.

## Risk, recorded

Building our own means nobody arrives knowing it, and it will look amateur if
nobody owns typography and density. This is the strongest argument against the
chosen direction and it was raised by the peer who nonetheless recommended it.
The mitigation is that the component surface is genuinely small and the patterns
are copied from people who solved this already
([components](components.md#patterns-copied-packages-not)); the residual risk is
real and is accepted knowingly.
