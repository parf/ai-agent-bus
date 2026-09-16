# home-parf review — design-system research, field-level truth, density acceptance

Peer review for [specs round 1](../README.md). Two asks were put to me directly:
whether [glyphs](../glyphs.md) and [data-dictionary](../data-dictionary.md)
flattened my analysis, and what acceptance would catch the typography-and-density
risk early. The research is written up below them, negative results included.

## Method and limits

| | |
|---|---|
| Read in full | glyphs, data-dictionary, visual-design, README, components, technology, forms |
| Skimmed | pages, information-architecture — my findings are concentrated where I was asked to look |
| Verified against source | every field named in data-dictionary and every "observed basis" in glyphs, read against `internal/protocol/envelope.go`, `internal/core/bus.go` and `internal/core/status` |
| Not done | no browser capture, no live data beyond one `agent-bus ls`, no accessibility audit |
| Peer overlap | codex's C01–C16 are presentation findings and opencode's are IA findings; mine are field-level and do not restate either |

The verification is the part worth having. Both documents promise that no
severity is invented and that every threshold names a real observation. Six of
those promises do not survive a read of the daemon.

## Answer 1: what got flattened

Nothing was flattened in the sense of lost. The glyph analysis came back
sharper than I sent it — codex's correction that *backlog with no reader is not
severity* is right and my original suggestion was wrong. What follows is not
recovery of lost material; it is six places where the specs now claim more than
the code says, and three residual ambiguities.

### F1. Red rests on an observation the send path treats as provisional

**Severity: highest. This is the one glyph allowed to mean "act now".**

[glyphs](../glyphs.md#attention-levels) defines red as `AtBound` — "messages
are being refused or dropped now". But `withLiveness`
([bus.go:407](../../../../src/internal/core/bus.go#L407)) computes `AtBound`,
`Queued` and `Oldest` **without pruning**, and `prune` runs in only three places
([bus.go:622](../../../../src/internal/core/bus.go#L622) on send-when-full,
[bus.go:808](../../../../src/internal/core/bus.go#L808) on consume,
[unregister.go:77](../../../../src/internal/core/unregister.go#L77)).

So a queue can report `AtBound` on the strength of messages that have already
outlived their TTL and not yet been swept — and the send path prunes *first*,
then re-checks the bound. The listing says at bound; the next send is accepted.
Red would be lit on a queue that is refusing nothing.

Cheapest honest fixes, in order of preference:

| Fix | Cost |
|---|---|
| Word red as *at its bound when last observed*, and say on detail that the sweep is lazy | A sentence. Keeps the glyph, drops the false claim |
| Count expired-but-unswept in `withLiveness` and subtract before comparing | A read path that needs the expiry test but not the write; no mutation |
| Prune in the observation path | Turns a read into a write; needs the write lock on every listing. Not worth it |

Whichever is chosen, `Queued` is likewise "held now", not "waiting now", and
`Oldest` may be the age of a message the daemon already considers dead. Both
labels in the dictionary are currently stronger than the field.

### F2. Orange cannot fire where it matters, and where it fires it means the rejected signal

Orange is "oldest message older than the record's own TTL", basis `Oldest`
against `TTL`, "both are reported". Two problems.

`life()` ([bus.go:724](../../../../src/internal/core/bus.go#L724)) takes the
record TTL and the message TTL. With both unset it returns `0`, the caller
leaves `Expires` as the zero time ([bus.go:560](../../../../src/internal/core/bus.go#L560)),
and `prune` skips anything with a zero `Expires`. **An unset record TTL is not a
daemon default — it is no expiry at all.** For every record that declares no
TTL, and that is most of them, there is no value to compare `Oldest` against,
so orange is uncomputable rather than merely unlit.

Where TTL *is* declared, `Oldest > TTL` is reachable only in the window before a
lazy prune — and what it then indicates is that nobody has read the inbox and it
is not full. That is precisely the *backlog with no reader* signal codex
rejected as not-severity, arriving in a TTL costume.

My reading is that orange has no defensible basis in MVP data and should be
cut with the same sentence that cut "climbing": the window does not exist. That
leaves red and the absence markers, which is a thinner severity vocabulary than
the doc wants but an honest one.

### F3. "Uses the daemon default" is true for `Bound`, false for `TTL`

[data-dictionary](../data-dictionary.md#queue) groups `TTL`, `Bound` and `Full`
in one row labelled *uses the daemon default where unset*. They behave
differently:

| Field | Unset means | Source |
|---|---|---|
| `Bound` | the daemon's `maxQueue` — a real resolved default | `boundOf` ([bus.go:436](../../../../src/internal/core/bus.go#L436)) |
| `TTL` | **no expiry**; messages are kept until dequeued | `life` ([bus.go:724](../../../../src/internal/core/bus.go#L724)) |
| `Full` | `strict`, which is the protocol's documented meaning of empty, not a daemon resolution | [envelope.go](../../../../src/internal/protocol/envelope.go) |

Rendering "uses the daemon default" beside an unset TTL invents a default that
does not exist. That is this document's own defect class — a true value
presented as something it is not — inside the document written to prevent it.
Three rows, three labels.

### F4. `Status.Refusals` does not exist; it is `Status.Refused`

A `map[string]int`, `omitempty`, and by deliberate design it carries **only
reasons that have occurred** — "a reason with a zero beside it is noise on every
other node". Two consequences: the name in the dictionary is wrong, and an
absent reason is not a measured zero. The four markers already answer it and
were not applied here.

### F5. `Status.Unclean` has three states, not two

It is `omitempty` because "a first start has no previous stop to have been clean
or otherwise". So: true is an unclean stop, false is a clean one, and **absent
is no previous stop at all**. The dictionary treats it as one boolean fact. The
third state is exactly what `—` is for, and this is the clearest case in the
whole document for the absence vocabulary being applied to a node field rather
than only to queue cells.

### F6. The directory row mixes daemon truth with face-computed truth

`PeopleCount` and `OtherCount` are fields of `peopleView` in
[cmd/agent-bus-web/users.go:21](../../../../src/cmd/agent-bus-web/users.go#L21) —
the web child counts them while walking the directory. They sit in the
dictionary's Directory table beside `Services` and `Groups`, which are daemon
answers, with nothing to say which is which.

The document's organising rule is that two kinds of truth are never merged.
Provenance — daemon-supplied or face-derived — is the same class of distinction,
and a reader currently cannot tell which numbers would survive a daemon change.
A fourth column, or a marked block, closes it.

### Residual ambiguities, cheaper than the above

**White severity and `¿` are one fact in two notations.** White is "no signal;
absent or unset / not observed / no sample covers the window"; `¿` is "not
measured or not observable / traffic before the last restart". Which appears in
an Activity value cell with no sample? The vocabulary already settles this shape
for `❓` versus `¿` — loud in a status line, quiet in a data cell. Say the same
of white: white in a status or attention line, `¿` in a data cell. One sentence.

**Blue and `—` both claim "not applicable".** Blue is "informational, or not
applicable" and is used for `Proto`; `—` is "not applicable here". Same
collision. Either blue is strictly a record-level declaration and `—` owns
cell-level inapplicability, or "not applicable" leaves blue's definition.

**Precedence in the judgment column is undefined.** One glyph per cell, but a
record can be at bound *and* disabled at once, and both live in that column. No
rank is stated, so whoever writes the template invents one. It is not an
arbitrary choice either: a disabled queue at its bound is not urgent, because
nothing is being accepted anyway. State the order, and that pair explains why
the order carries meaning.

**One axis or two?** The rule is that severity bands and category markers never
share a table, and the judgment column holds severity (red) beside a decision
marker (disabled-as-cancelled). Declare the column a single ordered status enum
whose values happen to carry colour, and the rule holds. Leave it as written and
the design violates its own restraint rule on the first page.

**`∅` has no cell in MVP.** It is excluded from integer counts, and every queue,
refusal and traffic figure is an integer count. Purple, black and brown were
honestly retired as "no use here yet"; `∅` deserves the same line or a named
cell. A marker that never appears teaches people to expect a meaning that never
comes.

## Answer 2: acceptance for the typography-and-density risk

The risk as recorded is right, and the mitigation as recorded — small component
surface, copied patterns — is not a check. It is a reason to expect a good
outcome. What follows is falsifiable, and it is deliberately written in this
repo's own idiom, because the doctrine here is already mutation-first: break the
thing and watch a named check fail.

**The framing test: if vandalising the type scale breaks no check, there is no
check.** Swap the token file for a deliberately bad one — wrong scale ratio,
tight leading, two extra weights, a fourth text colour — and something must go
red. If the only thing that catches it is somebody's opinion in review, the risk
is unmitigated no matter what the document says.

Five checks, in the order I would add them:

| # | Check | Why it catches "amateur" |
|---|---|---|
| 1 | **A shipped-surface budget, enforced by a test over the stylesheet**: at most N font sizes, 2 weights, 3 text colours, 1 accent, and one space scale. Fail the build over the cap | This is the actual mechanism of amateur design. Nobody chooses eleven font sizes; they arrive one page at a time, each defensible alone. A cap is the only thing that stops accretion, and it is a grep |
| 2 | **Contrast asserted as a unit test over the token pairs, in both schemes** — not measured in review | The tokens are known values and the maths is trivial. W11 happened because `#888` was chosen by eye; `#6b6b6b` was never checked against a second palette. A test means it cannot recur, and two schemes double the obligation |
| 3 | **Numeric density targets**: a stated minimum of data rows visible at 1280×800 and 1366×768, and the same table passing 1.4.4 at 200% zoom and 1.4.10 reflow at 320 CSS px with no horizontal page scroll | "Comfortable default density" is unmeasurable and therefore unfalsifiable. Rows-above-the-fold catches too airy; zoom and reflow catch too tight. The two together pin density from both sides |
| 4 | **The worst page first, not the easiest.** The Services list at ~200 records with pathological content — a 74-character address, an absent description, a description duplicating the address, one at bound, one disabled, one external, one with backlog and no reader, plus CJK and a very long realm — built and reviewed *before page two exists* | Density is a property of the hardest table, and nine pages built on a scale tuned against the Overview will all be wrong together. This is the single highest-value ordering decision available |
| 5 | **A five-state proof sheet per component** — populated, empty, denied, unavailable, long-name — as one static page, reviewed before that component is used anywhere | visual-design already requires these examples; what is missing is that they are a gate on one component rather than a review of the finished set. Empty and denied states become afterthoughts precisely when they are validated last |

And the thing no check substitutes for: **name the owner of the token file.**
The risk I raised was conditional — it looks amateur *if nobody owns typography
and density*. One named owner, through whom every token change passes, with a
page that introduces a size or colour instead of using a token failing review,
is the mitigation. Checks 1–5 make that ownership visible when it lapses; they
do not create it.

Check 1 is the one I would bet on if only one were adopted.

## The research, with negative results

Recorded so nobody re-proposes these in six months.

### Split the question first

"Which admin design system do we adopt" is two questions with no overlap: where
the *patterns* come from, and what *CSS* ships. The best pattern sources are
React and unshippable here; everything shippable under no-script is a styling
baseline with no operations patterns in it. Judged as one list, the answer is
mediocre on both axes. This is now the structure of
[components](../components.md).

Two repo-specific facts sharpen it. A design system's product is components —
markup plus behaviour plus styling as a unit — and with server-rendered
templates the markup is hand-authored regardless, so the import is paid and
mostly discarded. And `cmd/agent-bus-web` ships about **39 lines of inline CSS**
for the whole dashboard: adoption is not migration, it is importing two orders
of magnitude more CSS than exists to style nine page types.

### Pattern sources: read, credit, never depend on

| Source | Why it matters | Why it cannot ship |
|---|---|---|
| **Cloudscape** (AWS) | The real omission from the original list. Apache-2.0, built since 2016 *for cloud operations consoles*: 66 components, 36 pattern guidelines. Property filter, table density and collection preferences, split panel, and resource-table empty/loading/error states. Closest published prior art to what agent-bus is | React |
| **Polaris** (Shopify) | Admin-specific: index tables, resource lists, and the best published empty-state and admin content guidelines anywhere | React |
| **GOV.UK** | Progressive enhancement as doctrine, tested rather than claimed. Error summary and check-answers are the patterns we want | Ships as packages; density is wrong for us — built for one-question-per-page citizen services |
| **Carbon** | Data-table usage guidance, task toolbar, dense-data spacing | Sorting, filtering, paging, overflow menus, modals and tabs are all JS; the CSS is Sass and needs a build |
| **USWDS** | GOV.UK's sibling with meaningfully denser typography | Same |

### Implementation layers, judged on no-script

| Candidate | Verdict |
|---|---|
| **Open Props** (+ Open Props UI) | Recommended. Token layer, MIT, copy-paste components owned outright, no runtime, no build |
| **Pico v2** | The defensible fallback if a baseline is wanted. No JS anywhere, honestly; dropdowns are `details/summary`. Its modal docs use JS, now replaceable by `command="show-modal"`. Generous default spacing needs retuning before a dense table reads |
| **Tailwind CLI** | Survives trivially — it is generated CSS — but supplies zero patterns, adds a build step, and utility-soup templates are a real cost when templates are the review artifact |
| **Primer CSS** | Genuine density and a `<details>`-based no-JS disclosure heritage that is the right idea. Drifting React-ward; maintenance risk |
| **Bulma, Halfmoon** | Pure CSS, no JS, generic look, no operations patterns |
| **daisyUI** | *Negative.* Components are pure CSS, but the interactive ones are checkbox and `:focus-within` hacks with keyboard and AT costs now strictly worse than native popover and dialog. Also needs a Tailwind build |
| **Basecoat** | *Negative.* "shadcn without React", but needs Tailwind plus vanilla JS for the interactive components. Fails the rule |
| **Franken UI** | *Negative.* HTML-first in presentation only: UIkit 3 extended with LitElement. It is JS |
| **SLDS** | *Negative, and the one most likely to be re-proposed.* It used to be the genuinely CSS-only enterprise system with real data tables. SLDS 2 (Spring '25) has coupled itself to the Salesforce platform, the Cosmos theme and LWC base components, with its own linters. It is no longer the free-standing CSS framework people remember |
| **Shoelace / Web Awesome, htmx, Datastar** | Out by the rule, not by quality |

### The fact that re-priced the constraint

Native declarative HTML acquired the missing pieces. The Popover API reached
Baseline widely-available in **April 2025**. Invoker Commands — `command` and
`commandfor`, with `show-modal`, `close`, `request-close`, `toggle-popover` —
shipped Chrome/Edge 135, **Firefox 144 in October 2025** and **Safari 26.2 in
December 2025**. Menus, disclosures and modal dialogs are authorable with zero
script.

In 2023 "no JavaScript" cost exactly those components, which is *why* design
systems ship JS for them. In 2026 it does not. That changes the cost of the
constraint more than any library choice, which is why it belongs in the record
with dates attached.

**One boundary on it.** Not for consequential confirmation. A confirmation must
re-check authorization *and current conditions at submission*, which a
client-side dialog quietly removes, and 0.5.31 has just made that re-check the
whole point of the gate ordering. Server-rendered confirmation pages stay. Worth
writing as a rule precisely because the new API makes the wrong choice easy and
attractive.

### What we give up

| | |
|---|---|
| Free accessibility work | The real cost. GOV.UK's forms-and-errors markup is copyable pattern-by-pattern, and that is where the hard accessibility lives |
| Familiarity | Nobody arrives knowing our CSS |
| Typography and density | Covered above; the strongest argument against the direction, and the reason Answer 2 exists |
| Theming for free | Open Props supplies most of it |

## Currency

The correction is accepted: my W05 note described `d2f4c55` / 0.5.17, and
0.5.29 and 0.5.31 have since made credential removal atomic with unregistration
under one registry hold and left an unregistered name able to do nothing at all.
The specs encode the current contract.

One consequence follows for the dictionary. `Kind: unregistered` in the
credentials table is now a **legacy-only** category: per 0.5.29 such a name is
refused `401` on every call and is not issued a credential in the first place,
so the daemon should no longer create these rows. The label still describes the
231 that predate it. Said plainly, or a reader will take it for a live category
the daemon still populates — and it is my own code from 0.5.16 that prints it.

## Sources

[Cloudscape](https://cloudscape.design/about/) ·
[Popover API](https://developer.mozilla.org/en-US/docs/Web/API/Popover_API) ·
[Invoker Commands API](https://developer.mozilla.org/en-US/docs/Web/API/Invoker_Commands_API) ·
[GOV.UK progressive enhancement](https://www.gov.uk/service-manual/technology/using-progressive-enhancement) ·
[Pico](https://picocss.com/docs) ·
[Open Props](https://open-props.style/) ·
[SLDS 2](https://www.lightningdesignsystem.com/) ·
[Basecoat](https://basecoatui.com/) ·
[Franken UI](https://franken-ui.dev/) ·
[Every Layout](https://every-layout.dev/) ·
WCAG [1.4.4](https://www.w3.org/WAI/WCAG22/Understanding/resize-text.html),
[1.4.10](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html),
[1.4.3](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html)
