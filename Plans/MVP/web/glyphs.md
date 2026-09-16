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

| Level | Means | Admitted use here | Observed basis |
|---|---|---|---|
| red | demands attention | a queue at its bound | `AtBound` — messages are being refused or dropped now |
| orange | notable, not urgent | a backlog whose oldest message is older than the record's own TTL | `Oldest` against `TTL`; both are reported |
| white | no signal; absent or unset | not observed | no sample covers the window |
| blue | informational, or not applicable | a record declaring an external protocol | `Proto` is a **caller-supplied hint**, not proof the thing is served elsewhere. It means "do not expect a local reader here", nothing more |
| green | nothing to worry about | **not used** | see below |

Corrections applied from codex, each of which I had wrong:

- *"unclean stop not yet acknowledged"* invented an acknowledgement state. The
  daemon has `Status.Unclean` and nothing else. It is a fact to state, with no
  lifecycle.
- *"refusals climbing"* requires a window and a delta. The overview has lifetime
  totals. Either the face gets a window or the word "climbing" goes; it goes.
- *"backlog with no reader"* is **not** severity. A queue worker between pulls
  is exactly that shape and nothing is wrong. Depth alone is not an incident;
  age against the record's own TTL is the first defensible signal.

Green is not used at all, and this is deliberate. The only green anyone would
reach for is "a reader is attached", and reader presence is not health
([W03](../done/web-review.md#findings)). Since ordinary is the majority state,
marking it marks nothing — and it would spend the attention budget on the rows
that do not need it.

Purple (off-scale), black (filled) and brown (faded, for deprecated things) are
in the vocabulary and have no use here yet.

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
| `∅` | a measured non-zero value below display precision | a rate that rounds away at the shown precision. **Not** for integer counts, which never round |
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
| `<b class=warn>nobody</b>` in the reader column | *no reader waiting*, unmarked. It is not, by itself, a problem |
| `<b class=warn>full</b>` | red status, text *at bound* |
| `Maximum: 0` repeated down the page | removed; `0` and `¿` in the value table carry it |
| blank cell for an unset value | one of the four markers, or the label goes too |
| `Serving` / `Offline` | *reader attached* / *no reader waiting* / *external* |

## Open

- Whether `∅`, `—` and `¿` ship as those characters or as words. Characters are
  compact and are the vocabulary's own; words are unambiguous on first meeting.
  A legend is the usual compromise and is itself a cost.
- Whether the owner accepts CSS rendering in place of the emoji.
