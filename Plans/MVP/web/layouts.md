# Layouts

Draft for peer review. Representative arrangements, wide and narrow, for the
pages that carry the design's weight — codex's
[S13](review/codex.md#specification-review-round-one): a design is reviewable
when somebody can disagree with it before it is built.

Field decisions are in [pages](pages.md#how-to-read-this); values are in
[tokens](tokens.md#the-budget). This file is arrangement only.

Wide is 1280×800, the lower of the two target desktop sizes. Narrow is 320 CSS
px, the [reflow](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html) floor.

## The frame every page shares

```
┌──────────────────────────────────────────────────────────────┐
│ agent-bus · node observed 14:22          parf@parf  Sign out │  surface-3
│ Overview  Services  Channels  Activity  Users  Groups  Diag  │  accent underline on current
├──────────────────────────────────────────────────────────────┤
│                                                              │  surface-1
│  ◇ Page title  ⓘ                                  [Refresh] │  text-xl
│  One short factual subtitle, only where useful               │  text-sm / text-2
│                                                              │
│  … page body, max 76rem, centred, 16px gutters …             │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

The shell owns the whole document structure, `<main>` included, so a page
cannot forget to close it — seven of nine templates do today
([components](components.md#accessibility-which-is-the-real-cost-of-building-our-own)).

Narrow: identity and sign-out stack; navigation collapses into a
`details`/`summary` disclosure whose summary names the current page, so the
answer to *where am I* survives the collapse.

## Overview — the only page allowed to be short

```
  Overview                                             [Refresh]
  What needs attention on this node, as of 14:22

  ┌──────────────────────────────────────────────────────────┐
  │ ▌ Queue at capacity when observed                          │   red bar, surface-2
  │   billing/invoices@parf.us · 512 of 512 held · overflow:   │
  │   refuse · observed 14:22                    → the queue   │
  ├──────────────────────────────────────────────────────────┤
  │ ▌ Delivery is off and work is held                         │   orange bar
  │   ocr/intake@parf.us · 48 held · oldest 3h 12m             │
  │   Nothing is arriving while it stays off.    → the record   │
  └──────────────────────────────────────────────────────────┘

  This node                                    ← labelled node-wide, always
  ┌──────────┬──────────┬──────────┬──────────┐
  │ Uptime   │ Records  │ Queued   │ Readers  │
  │ 6d 4h    │ 231      │ 604      │ 12       │   text-2xl, tabular
  └──────────┴──────────┴──────────┴──────────┘
  Node-wide. A list below shows only what you may see; the two
  never have to agree.

  Find ▸ Services   Channels   Users
```

**Refusals are deliberately not in that strip.** The draft put them there,
against this specification's own *never counted twice* rule — refusals and
losses are attention items and Diagnostics figures, and a third copy on the
same page is how a page comes to disagree with itself (codex's
[R2-7](review/codex.md#review-of-the-round-one-response)).

Each attention item is: **what was observed**, the thing it was observed on,
the values it rests on, when, and one link. The coloured bar is 3px of
`--red`/`--orange` on the leading edge and never the only carrier — the heading
text states the condition in words ([glyphs](glyphs.md#the-rule-that-matters-most)).

Empty:

```
  ┌────────────────────────────────────────────────────────────┐
  │ No observed attention conditions in this view, as of 14:22.│
  │ This covers the conditions the daemon reports, over the    │
  │ records you may see. It is not a statement that everything │
  │ is working.                                                │
  └────────────────────────────────────────────────────────────┘
```

One bordered line, `--text-2`, no illustration, no green. It does not get a
third of the screen for saying nothing.

## Services — the page the density is tuned against

Built first, at ~200 records, with the pathological content named in
[acceptance](visual-design.md#acceptance). Every other table inherits what is
decided here, which is why it is not built second.

```
  ⚙️ Services  ⓘ                                      [Refresh]
  Caller-visible services

  All (231)  [My (12)]  Personal (3)  Register service

  ┌──────────────────────────────────────────────────────────┐
  │ Search ▢─────────────────────  State [Any] Enabled Disabled│ surface-3
  │ Kind All [Agent] Service                              │
  │ Sort[Queued ↓▾]                                  [Filter]│
  │ 27 of 231 · kind: agent                        [Clear all]│
  └──────────────────────────────────────────────────────────┘

   │  SERVICE                 OWNER         READER     QUEUED │  text-xs 600
   ├──────────────────────────────────────────────────────────┤
   │▌ Invoice intake  [Yours]  parf@parf.us attached      512 │  red bar
   │  billing/invoices@parf.us              at capacity       │  mono, text-2
   ├──────────────────────────────────────────────────────────┤
   │▌ Document intake         ops@parf.us   no reader      48 │  orange bar
   │  ocr/intake@parf.us                       delivery is off│
   ├──────────────────────────────────────────────────────────┤
   │  OCR pipeline            parf@parf.us  no reader       0 │
   │  ocr/pipeline@parf.us                                    │
   └──────────────────────────────────────────────────────────┘
                                          ‹ Previous   Next ›
```

Two lines per row: **description then address**. The description is how a
session is recognised and is absent from every list today; the address
disambiguates and is demoted, not dropped. The judgment column is the leading
bar plus a word under the description, so it costs no column width at all. Rows
follow the query — here depth descending — which is why the ordinary zero-depth
row sits last rather than in the middle of a list filtered on nothing of the
sort.

**Kind is absent here because this list is filtered to agents**, and the toolbar
says so. On an unfiltered or mixed list it is a column, because nothing else
tells a reader what a row is.

Brackets mark the current link choice in this text mockup. All, My and Personal
are second-level links with caller-visible category totals; state and kind
expose their two or three values directly and preserve them in the URL. The
larger sort list remains a select. Register service is section navigation, not a
form appended below the table.

**Yours is never colour alone.** The label and row shape remain in All, My,
Personal and filtered results. Personal has its own visible word and orange
treatment; a row may carry both. The service name is the single route to
read-first detail and any controls authorized there.

**No cell truncates a value that has no other route to it.** The draft ellipsed
owners to `parf@…` and offered no recovery, which without script means the value
is simply gone: a `title` attribute is not keyboard-reachable and may not be the
only path. Owners wrap to a second line instead. The recovery may not be *"open
the owner's user page"* either — the caller may have no permission to read it,
and a value already present in this answer must not need a second, refusable
request to see (codex's [R2-7](review/codex.md#review-of-the-round-one-response)).

Narrow (320px): description, then address, then a status line. Owner, reader
and queued move into that line as `ops@… · no reader · 48 held`. The bar and the
description survive at every width; nothing scrolls sideways.

```
  ┌──────────────────────────────┐
  │▌Document intake              │
  │ ocr/intake@parf.us           │
  │ delivery is off · 48 held    │
  ├──────────────────────────────┤
```

## Service detail — read first, then one edit at a time

```
  👾 Document intake  ⓘ                               [Refresh]
  ocr/intake@parf.us

  Yours                                                   [Edit]

  Identity                                              [Edit]
    Description   Document intake
    Address       ocr/intake@parf.us
    Protocol      —
    Kind          agent
    Owner         [photo] ops@parf.us
    Maintainers   @ocr-team
    State         Disabled · delivery is off
    Updated       2026-09-14 09:31

  Queue                                                 [Edit]
    Held               48          ← "held", not "waiting"
    Oldest held        3h 12m
    Accepted           1 902       ← cumulative across restarts
    Dequeued             1 854
    Dropped                  0
    Expired                  0
    At capacity        no
    Capacity           512
    Retention          no queue-imposed expiry — a message may set its own
    Overflow           refuse

    ▌ Delivery is off and this work is held. Nothing is
      arriving while it stays off.

  Access                                                [Edit]
  Configuration
    Digest        sha256:…
  Activity
    Accepted / dequeued                          [inline graph]
    Dropped / expired / refused                  2 / 0 / 1
                                      [View all activity →]
  Danger Zone                                    [open dangerous actions →]
```

Every section is read-first with its own `[Edit]`, shown only where authority
allows and collapsed until asked for. There is **no trailing Manage block**: the
eight concerns that share one page of always-open editors today become eight
disclosures, each beside what it changes. Opening one:

```
  Queue                                               [Cancel]
    Held               48
    …
    ┌──────────────────────────────────────────────────────┐
    │ Retention   ▢─────────────  blank = no queue TTL      │
    │ Capacity    ▢─────────────  blank = daemon default    │
    │ Overflow    (•) Refuse   ( ) Drop oldest              │
    │                                  [Save queue policy]  │
    └──────────────────────────────────────────────────────┘
```

The read values stay visible above the editor. Cancel returns without a
round-trip having changed anything, and the section's filters and scroll
position are preserved on return.

Access and Maintainers use the same line-list textarea: one named user, group,
agent or service per line, with submitted lines retained beside line-specific
errors. Access additionally admits `*`. The Maintainers list depends on its
accepted daemon-model change; it is not the current single-group value stretched
into a textarea.

The owner photo appears only for a caller-visible User profile and uses the
local thumbnail; it is decorative beside the linked owner name. Otherwise the
ordinary entity label remains.

The red **Danger Zone** link opens a separate server-rendered resource view.
Only Replace configuration, Transfer ownership and Remove registration live
there; none of their forms appears on the ordinary detail page. Transfer and
removal continue to a fresh confirmation page before submission.

## A consequential confirmation

Server-rendered, its own page, because the consequence is computed when it is
shown ([forms](forms.md#consequential-actions)).

```
  Remove billing/archive@parf.us?

  ┌────────────────────────────────────────────────────────────┐
  │ This is what happens                                       │
  │                                                            │
  │  · The address goes, and its credential goes with it.      │
  │  · Nothing answers to this name afterwards.                │
  │  · ops@parf.us keeps their own credential.                 │
  │                                                            │
  │ Observed just now: nothing queued, no unfiltered read      │
  │ waiting. Filtered reads are not reported. The daemon       │
  │ settles eligibility when you confirm.                      │
  └────────────────────────────────────────────────────────────┘

  [Remove this service]   Cancel
```

The destructive action is a filled `--red` button with `--text-on-accent`
(7.07:1). Cancel is a plain link, not a second button —
one primary action per form, and a cancel shaped like a button is how people
click the wrong one.

**The draft drew this on the 48-held record and promised "48 held messages go
with it".** That is a destructive purge the daemon does not offer, on a record
that could not have been removed at all: `UnregisterAnd` prunes and then
**refuses** with `ErrBusy` if any message or reader remains
(unregister.go:76–79). codex's find, and the worse half is that a confirmation
page exists to state the consequence truthfully — inventing one there is the
defect class this whole specification is against.

Removal is never a purge, so the unavailable case is its own state rather than a
scarier confirmation:

```
  ┌────────────────────────────────────────────────────────────┐
  │ Removal is not available while work is held                │
  │                                                            │
  │ 48 messages are held and 1 reader is waiting. Drain the    │
  │ queue and stop its readers first — nothing is discarded    │
  │ on your behalf.                                            │
  └────────────────────────────────────────────────────────────┘
```

This is [only offer transitions that apply](forms.md#rules) doing real work: the
control is absent, and the reason stands where the control would have been.

**The confirmation shows observations; it does not certify eligibility**, and
the draft's *"which is what makes this available at all"* claimed it did.
`withLiveness` sets `Reading` only for an **unfiltered** waiter (bus.go:425–429),
while `UnregisterAnd` refuses on **any** waiter (unregister.go:78). codex
reproduced it. Note the wording that survives: **not** "no reader attached",
which the counterexample directly contradicts — there *is* a reader attached, it
is filtered, and the field does not report it. The line says what the field
actually answers and names what it omits.

**And a refusal here is not the conditions-changed state.** That was the draft's
next sentence and it is wrong in exactly this case: the filtered waiter existed
before the confirmation was drawn, was never observable through that field, and
still exists at submission. *Nothing changed.* It is the ordinary current-state
refusal — 409, with the daemon's message
([Problem](pages.md#problem--recovery-by-what-the-face-actually-knows)).

The final daemon check covers **stale** facts and **incomplete** facts alike, but
they are different things to be told, and only one of them is anybody's fault
for waiting:

| | The person is told |
|---|---|
| Stale fact — it was true when shown and stopped being true | the world moved between the question and the answer; here is what is true now |
| Incomplete fact — the face never had it | the daemon's own reason, plainly. No suggestion that anything changed, because nothing did |

Making eligibility visible would need the daemon to report filtered waiters,
which it does not: [owed](pages.md#owed-by-this-specification).

## The five states, per component

Each component is proved against all five on one static page before it is used
anywhere — home-parf's fifth check, and the reason it gates rather than
concludes: *empty* and *denied* become afterthoughts exactly when they are
validated last.

| State | Table | Definition list | Form |
|---|---|---|---|
| Populated | rows | values | values, valid |
| Empty | what this page would hold, and how to create one | the label goes with the value ([glyphs](glyphs.md#absence-which-is-four-different-facts)) | — |
| Denied | *not visible to you*, in words, never an empty array as zero | the same, per field | the control is absent, not disabled-looking |
| Unavailable | the daemon did not answer; retry. **Never an empty healthy table** | `¿` | the form, with what was typed preserved |
| Long name | 74-character address wraps in the second line; the row does not widen | wraps; the label column does not move | `field-sizing`, no overflow |

## What is deliberately not drawn here

Diagnostics, Activity, Users, Groups, Register, Account, Sign in and Problem
follow from these five arrangements and the field decisions in
[pages](pages.md#how-to-read-this). Drawing all fifteen before any is built
would be specifying rather than designing — the review that matters is of the
frame, the table, the detail-with-edits, the confirmation and the states, and
those are here.
