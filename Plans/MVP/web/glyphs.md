# Glyphs and state language

Draft. Corrected against codex's and home-parf's review; not yet settled.

Source: the owner's [shared symbol vocabulary](https://parf.dev/ai-skills/Glyphs.md),
which already governs documents in this repository
([writing conventions](../../../CLAUDE.md#writing-conventions)). This file says
what it means for the dashboard, which is a different medium with a different
renderer.

## The rule that matters most

> The default is no glyph. A document where every row is marked has marked
> nothing.

An operations console is the case the rule is for: most rows are ordinary. A
glyph on every row has become a column heading and should be one.

| | |
|---|---|
| One glyph per cell | Most cells get none |
| One category axis per table | Severity bands and category markers never share a table |
| If most rows carry it, none should | Move it to the heading, or to a filter control |
| Every glyph sits beside a word | Resolves the apparent conflict between the vocabulary's colour-primary rule and our contrast requirement: the judgment column is a **status column containing text**, not a bare dot. Colour and shape reinforce the word; neither replaces it |

## Identity and access symbols

The owner selected these human-facing symbols. Keep the word beside the glyph;
URLs, JSON, ACL expressions and other editable or machine-readable values stay
plain text.

| Glyph | Unicode | Visible label | Meaning |
|---|---|---|---|
| 👤 | `U+1F464` | User | one registered person |
| 👥 | `U+1F465` | Group | a group or team |
| 👾 | `U+1F47E` | Agent | an agent identity |
| ⚙️ | `U+2699 U+FE0F` | Service | a service identity |
| 🪪 | `U+1FAAA` | Identity | an identity as such, without asserting its entity type or credential |
| 🔑 | `U+1F511` | Credentials | credentials used to prove an identity; never the secret value itself |

`👾` is owner-selected for Agent. Its common uses also include games and bugs;
inside AgentBus human output it means Agent only. The visible word remains on
full labels, while compact directory rows carry the same word in their
accessible label.

User, Agent and Service are the implemented shared web/CLI entity labels. WEB
directory rows put their glyph directly before the identity name, and WEB group
headings put Group directly before the group name; the surrounding page states
the meanings in words. Identity and Credentials record the selected vocabulary;
their exact placements remain part of the unsettled web proposal below.

## Page-title images and glyphs

Owner-selected exception to the quiet-glyph rule: every page title starts with
one small image or glyph. It names the page category and always sits beside the
visible title, so it is neither a status signal nor a replacement for text.
The image is decorative to assistive technology because the adjacent `h1`
already supplies its name.

| Page | Title image or glyph |
|---|---|
| Overview | 🏠 |
| Services and generic-service detail | ⚙️ Service |
| Agent detail | 👾 Agent |
| Channels and channel detail | a small inline channel SVG |
| Activity | a small inline graph SVG |
| Users | 👤 User |
| User detail | the locally imported profile photo; 👤 User when no photo exists |
| Groups and group detail | 👥 Group |
| Account | 🪪 Identity |
| Sign in and credential pages | 🔑 Credentials |
| Diagnostics | a small inline inspection SVG |
| Problem | a small inline warning SVG |

Register pages inherit their section image. The inline SVGs are repository-owned
and self-contained: no external asset, icon font, hotlink or extra public route.
Detail pages choose only from daemon-stated kind; an absent kind gets the
section image rather than a guessed entity glyph.

**Built in 0.5.65 for every current page route.** The local photo planned for
User detail remains a separate data-backed feature; its current title uses the
User fallback.

Overview's mark was the inline AgentBus mark until 0.5.82, when the owner
replaced it with 🏠: the header carries that logo on every page, so the title
was repeating the product rather than naming the page.

### The same marks in the navigation

Second owner-selected exception, 0.5.82: **every top-level navigation entry
starts with its own section's title mark.** It is the same mark, not a second
symbol for one thing, and it is decorative to assistive technology for the same
reason — the link text beside it already names the section.

This does not reopen the quiet-glyph rule. That rule is about **data rows**,
where marking every row turns a signal into a column heading. The navigation is
a fixed seven-item wayfinding row that carries no status and no measurement, so
there is no attention budget to spend and nothing a reader could mistake for a
judgment. The rule that still applies is the one below it: every glyph sits
beside a word, and here every one does.

Nothing else in the shell is marked. The header keeps the single bus mark and
the footer stays plain.

## Rendering: a proposal, not a settled decision

**Proposed: adopt the semantics, render them as CSS shape plus colour plus
text, and keep the emoji for Markdown.**

The reasoning, from opencode and home-parf independently: emoji render per
operating system and per font, several symbols in the vocabulary need `U+FE0F`
to get colour at all, and [no external asset](../../../docs/05-discovery.md#rules-it-is-built-to)
means no icon font and no webfont to normalise them. A console where severity is
reinterpreted by the viewer's platform is worse than one with no severity.

codex's objection is recorded and is fair: the owner asked for *the glyph
vocabulary*, and a CSS equivalent is our substitution, not an instruction we
were given. It is proposed here so the owner can refuse it.

## Attention levels

Every threshold below names the observation it rests on. A severity we cannot
derive from something the daemon actually reports is not ours to invent.

The judgment column is **one ordered status enum whose values carry colour**,
not a severity axis and a category axis sharing a cell. That is what keeps it
inside the one-axis rule; home-parf caught that the first draft broke it.

| Level | Means | Admitted use here | Observed basis |
|---|---|---|---|
| red | demands attention | a queue at capacity **when observed** | `AtBound`. A condition, never a prediction — see [what an observation is worth](#what-an-observation-is-worth) |
| orange | notable, not urgent | a disabled record still holding queued work | `Disabled` with `Queued` > 0. `Send` refuses a disabled record and `recheckInbox` releases its waiters, so the work can be neither delivered nor read |
| white | no signal; not observed | a value the daemon does not report for this row | the loud form of `¿`. Same fact, two notations: white in a status column, `¿` in a data cell |
| blue | informational | a record declaring an external protocol | `Proto` is a **caller-supplied hint**, not proof the thing is served elsewhere. It means "do not expect a local reader here", nothing more. Not-applicable is `—`, not blue |
| green | nothing to worry about | **not used** | see below |

**Precedence is defined, because it is not arbitrary.** A disabled record at its
bound is not urgent: nothing is being accepted, so capacity is not the thing
wrong with it. Disabled outranks at-bound. Where two would light, the one that
explains the other wins.

## What an observation is worth

Verified in source after home-parf and codex arrived at it independently, from
different directions, and both were right.

`withLiveness` ([bus.go:407](../../../src/internal/core/bus.go)) computes
`AtBound`, `Queued` and `Oldest` **without pruning**. `prune` runs in exactly
three places — send-when-full, consume, and unregister — and **none of them is on
the observation path**. Meanwhile `deliver` prunes expiry *before* the overflow
decision, hands a matching waiter its message straight through without touching
the queue at all, and a concurrent consume can free space underneath both.

Three consequences, and the vocabulary has to carry all three:

| Reported | What it actually says |
|---|---|
| `AtBound` | at capacity **at the moment of the read**, counting messages that may already have outlived their TTL unswept. The next send may be accepted by a waiter, by a prune, or by a concurrent consume. Stating a consequence — even "refused or drops oldest" — overstates it |
| `Queued` | messages **held** now, not messages **waiting** now |
| `Oldest` | the age of the head, which may be a message the daemon already considers dead |

So red says *at capacity when observed*, names the configured overflow policy
beside it as a setting, and predicts nothing.

Corrections applied from codex and home-parf, each of which I had wrong:

- *"unclean stop not yet acknowledged"* invented an acknowledgement state. The
  daemon has `Status.Unclean` and nothing else. It is a fact to state, with no
  lifecycle.
- *"refusals climbing"* requires a window and a delta. `Status.Refused` is a
  lifetime total, so the word goes from the Overview. **One correction back to
  both peers**: it is not that we have no window anywhere. `activity.go` samples
  and computes per-interval deltas including `Refused`, so a windowed refusal
  signal is buildable on Activity. It is simply not something the Overview's
  basis can say, and sampled history does not survive a restart. codex's scope
  qualifier, accepted: that series carries **node-wide** refusals only on the
  unfiltered daemon-Owner view (activity.go:98–99) and is a sum over
  visible records otherwise, so the two must never be labelled alike.
- *"backlog with no reader"* is **not** severity. A queue worker between pulls
  is exactly that shape and nothing is wrong.
- *"backlog older than the record's own TTL"* was orange in two drafts and is
  **cut**. home-parf's find. The record's `TTL` is simply the wrong right-hand
  side: where it is unset there is nothing to compare against, and where it is
  set it is still not the deadline the head was accepted under. `life()`
  ([bus.go:724](../../../src/internal/core/bus.go)) fixes each envelope's
  expiry once, at accept, from whichever of the sender's and the queue's TTLs
  was shorter, so a perfectly live head accepted under a longer setting exceeds
  today's shortened one with nothing wrong.

  **My first version of this cut over-argued it**, and codex caught both halves.
  “An unset record TTL means no expiry at all” is a one-sided read of the same
  function: `life()` returns zero only when *neither* TTL is set, and
  `life("", "1m")` returns the sender's minute (bus.go:738). And the claim that
  `Oldest > TTL` indicates “nobody has read the inbox and it is not full” does
  not follow either way — `prune` is lazy, so a queue can fill and then age with
  no operation touching it at all. The cut stands on the wrong-right-hand-side
  argument alone; replacing an invalid signal with a second inference would have
  been the same mistake again.

  Orange survives on the disabled-record-holding-work item, which rests on two
  values the daemon reports directly. The level is not retired; its TTL basis
  is.

Green is not used at all, and this is deliberate. The only green anyone would
reach for is "a reader is attached", and reader presence is not health
([W03](../done/web-review.md#findings)). Since ordinary is the majority state,
marking it marks nothing — and it would spend the attention budget on the rows
that do not need it.

Purple (off-scale), black (filled) and brown (faded, for deprecated things) are
in the vocabulary and have no use here yet. **`∅` joins them**: it is for a
measured non-zero value below display precision, every figure on this dashboard
is an integer count, and integer counts do not round away. Retired honestly
rather than left in the table looking available — home-parf's point, and the
same standard the colours are held to.

## Absence, which is four different facts

The part of the vocabulary the dashboard needs most, because
[W09](../done/web-review.md#findings) and
[C07](review/codex.md#junk-and-misleading-content) are exactly it: *zero differs
from absent*, and today both print blank or `0`. home-parf's judgement is that
these quiet markers will fix more audited problems than any severity colour, and
I agree.

| | Means | Example here |
|---|---|---|
| `0` | measured zero | the queue is empty and we looked |
| `∅` | a measured non-zero value below display precision | **no MVP use**: every figure here is an integer count and integer counts never round. Kept in the vocabulary, unused on this dashboard — see [not used](#attention-levels) |
| `—` | not applicable; the thing does not exist here | subscribers on a queue-mode channel; oldest-message age on a queue that is empty **now** — the daemon does not say whether it ever held one |
| `¿` | not measured, or not observable | traffic before the last restart |

`¿` reads quietly on purpose; in a dense cell a `❓` would shout louder than the
findings around it.

**Hidden membership is not `¿`.** Group membership withheld from an ordinary
caller is a known, explainable fact and gets words — *not visible to you* — not
an ambiguous mark ([C05](review/codex.md#junk-and-misleading-content)).

**Four markers are not a licence to mark every blank.** The noise rule applies
here too: where an absence does not change a decision, the label goes with the
value rather than being replaced by a symbol.

## Where a glyph is allowed

| Screen | Allowed | Not allowed |
|---|---|---|
| Page title | one page-category image or glyph beside the title text | status, inferred entity type, or multiple decorative marks |
| Section navigation | each entry's own [section mark](#the-same-marks-in-the-navigation), decorative, beside the label | status, counts, or a mark that differs from that section's title mark |
| Overview attention items | severity, one per item — that page is nothing but exceptions | — |
| Services and Channels lists | **one** judgment column, lit only on exceptional rows | the kind column (a category: use a filter); the enabled column while most rows are enabled; anything green |
| Service and Channel detail | queue condition; refusal reason | section headings |
| Activity | the absence vocabulary in the value table | the graph itself — label the axes |
| Diagnostics | refusal reason; the absence vocabulary | per-envelope decoration |
| Users and Groups | paused and banned states | classification of an identity — that is a word, not a colour |
| Forms and prose | nothing | everything |

Disabled is a **decision, not a failure**, and takes the cancelled mark rather
than the failure one. The distinction earns its place: the audit shows people
already read "disabled" as "broken".

## What this replaces

| Today | Becomes |
|---|---|
| `<b class=warn>nobody</b>` in the reader column | the numeric **Readers** count, unmarked. Explicit zero is not, by itself, a problem or a claim that a process is dead ([Readers](../../../docs/05-discovery.md#readers)) |
| `<b class=warn>full</b>` | red status, text *at bound* |
| `Maximum: 0` repeated down the page | removed; `0` and `¿` in the value table carry it |
| blank cell for an unset value | one of the four markers, or the label goes too |
| `Serving` / `Offline` | numeric **Readers**; keep the independent external hint in its own field |

## Open

- Whether `∅`, `—` and `¿` ship as those characters or as words. Characters are
  compact and are the vocabulary's own; words are unambiguous on first meeting.
  A legend is the usual compromise and is itself a cost.
- Whether the owner accepts CSS rendering in place of the emoji.
