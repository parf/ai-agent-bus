# opencode review — design-system research, the fork, and the IA

Peer review file, owned by opencode. Scope: design-system research judged
against the no-script rule; the enhancement-layer fork on the record; the
templ question; critique of the [information architecture](../information-architecture.md)
as posted 2026-09-16. Verdicts here are mine; owner questions stay the owner's.

## 1. Research method

One test was applied to every candidate, because "works without JavaScript"
is claimed more often than it is true:

> Do the components stay fully functional with scripting disabled, and can
> the whole thing be served as local static assets from the binary?

Three answers are possible: **survives** (✅), **partial** (⚠️ — usable subset,
rest dead), **fails** (❌ — its value is its script).

**Basis, stated honestly:** verdicts below are assessed from each project's
documented component architecture and primary docs — **no page was loaded with
scripting disabled in a browser by this reviewer**. Sources pinned per row
where fetched (bulma.io, open-props.style, docs.tabler.io inspected 2026-09-16);
rows marked *to verify* rest on documented architecture and should be
confirmed before Q59 is answered. The measured step belongs in visual design:
render each shortlisted candidate's component demo in the audit browser with
scripting disabled, once, and record what breaks. That converts this table
from assessed to tested evidence.

## 2. What was examined

Beyond the four known candidates (Carbon, GOV.UK, Pico, Tailwind CLI), these
were assessed: Bulma, Open Props, Every Layout, Utopia, MVP.css / Water.css /
Simple.css, Halfmoon, Bootstrap (CSS-only usage), Shoelace / Web Awesome,
Material web components, Primer CSS, and the themed admin-template family
(Tabler, CoreUI, AdminLTE and kin).

| Candidate | Kind | No-script verdict | Why |
|---|---|---|---|
| **Bulma** (v1.0.4) | component framework | ⚠️ | Corrected from an earlier ✅: shipping no JavaScript is not the same as every component working without it. Its own modal docs say "Bulma does not include any JavaScript. However, this documentation provides a JS implementation example" — modal, dropdown, navbar burger and similar toggles are inert without the JS you write. The **static subset** (tables, form fields, panels, tags, message banners, cards, buttons, grid, pagination-as-links) is script-free and covers this dashboard's whole component need. Graded symmetrically with Tabler: CSS subset live, interactions dead — Bulma's edges are zero JS to remove, theming via CSS variables, and no demo aesthetic to strip. *Source: bulma.io incl. /documentation/components/modal/, inspected 2026-09-16 (MIT)* |
| **Open Props** (v1.7.23) | design tokens | ✅ | CSS custom properties only: 4 kB core (brotli), Open Color (20 hues × 12 steps), shadows, fluid sizes, easings, light/dark normalize, and local-only modern font stacks — which solves the no-external-font problem outright. Its one JS extra (theme switch) is not needed; the server can emit `data-theme`. *Source: open-props.style, inspected 2026-09-16 (MIT)* |
| **Every Layout** | layout primitives | ✅ | Stack / cluster / sidebar / frame as small reusable CSS. Directly the toolkit for dense tables plus the narrow-screen behaviour W10 asked for. *to verify: current license of the code vs the paid book* |
| **Utopia.fyi** | scale generator | ✅ | Emits a fluid type/space scale as CSS custom properties. The disciplined answer to 390 px |
| **Tabler** (v1.5.1) | admin template | ⚠️ | Corrected from an earlier blanket ❌. Its docs explicitly separate concerns: "The CSS covers the layout and components, and the script adds the interactive parts such as dropdowns and modals", and self-hosted download is supported. So the CSS subset — layout, tables, cards, forms, its skin — survives no-script. What dies: dropdowns, modals, offcanvas, charts, every interactive behaviour, and a demo-site aesthetic that must be stripped. Still dominated by Bulma on this axis (whole component set no-JS by doctrine vs a subset), but it is **partial, not dead**. *Source: docs.tabler.io/ui/getting-started/installation, inspected 2026-09-16 (MIT)* |
| GOV.UK Frontend | design system | ⚠️ | Genuinely survives (progressive-enhancement doctrine; pages remain functional without JS — rare and honest), but it is public-service forms, not dense telemetry. Wrong register: you would fight it on every table and toolbar |
| Tailwind CLI | utilities | ⚠️ | Runtime-safe — zero JS ships — but it answers utilities, not design. It gives you atoms, not a table, a toolbar or a form pattern. Compatible with any choice here without deciding any of them |
| Bootstrap, CSS-only subset | component framework | ⚠️ | Styles apply; tabs, modals, dropdowns, menus die. Dominated by Bulma for this project's case. *Tabler is this pattern with a nicer skin — same verdict, same reason* |
| Halfmoon | component framework | ⚠️ | Dark-first CSS core with optional JS. Viable, smaller ecosystem and less mature theming than Bulma. *to verify* |
| MVP.css / Water.css / Simple.css | classless baselines | ⚠️ | Fine for prototypes; no admin componentry. Not sufficient for this dashboard |
| **Carbon** | design system | ❌ | Components are JS (React/Vue/web components). Only its design tokens transfer. *to verify: no CSS-only distribution currently offered* |
| Shoelace / Web Awesome, Material web components | components | ❌ | Custom elements require runtime JS by specification. *architectural, safe without a browser check* |
| Primer CSS | components | ❌ | Its standalone CSS line has been left behind by GitHub's move to React components. *to verify before citing externally* |
| CoreUI, AdminLTE | admin templates | ⚠️ | Corrected from an earlier ❌: AdminLTE v4 is Bootstrap 5.3 and "drops the jQuery dependency entirely" (adminlte.io FAQ, pinned 2026-09-16, MIT) — the jQuery-era description was stale. Same family and same verdict as Tabler: Bootstrap-derived CSS subset survives no-script; the demonstrated value (18+ plugins — Chart.js, DataTables, SweetAlert2, toasts) is JS. CoreUI follows the same pattern and was not individually verified. **No themed admin template survives whole** — that claim holds; each survives as a subset like everyone else. This row is a caution, not tested fact |

Of the known four: Carbon fails as above; GOV.UK survives with the wrong
register; Pico survives and is too thin; Tailwind is compatible but silent on
every question this redesign actually faces.

## 3. Recommendation

**A lean, not a conclusion** — the comparative certainty this section once
carried did not survive symmetric grading, and the honest procedure is
Codex's: enumerate the components this dashboard actually needs (from the IA
and the inventory: dense tables, forms with help/error slots, status badges,
definition lists, pagination links, disclosures), then compare the chosen
subsets of Bulma, Tabler and a hand-built token system on those, keyboard
behaviour and narrow layouts, with the disabled-script browser check, before
Q59 goes to the owner.

What survives as my lean toward a **house system** (Open Props tokens + Every
Layout primitives + a hand-authored component layer): the token layer serves
Q61 dark mode directly; nothing needs stripping; every asset stays auditable
in a security-relevant binary; and the needed-component list contains nothing
that requires a **JS widget runtime** — the dashboard is full of interaction
(forms, filters, disclosures), and every bit of it is native elements: form
POSTs, GET queries, `details`. The JS-dependent half of every framework is
dead weight we would import to not use. Bulma's static subset is the
strongest pre-built alternative and the fallback if the owner weights
delivery speed; Tabler and AdminLTE v4 are the same shape with a demo
aesthetic to strip. With symmetric grading the decision rests on subset fit,
theming weight and the measured check — not on a doctrine badge.

### The no-script interactivity kit

What "modern" looks like with script off — these belong in the visual-design
and components docs by name: `details`/`summary` (+ `name=` for exclusive
disclosures), `popover` with `popovertarget` (zero-JS menus and light-dismiss
top-layer content — **not** dialog semantics: no focus trap, `role=dialog` must
be authored, light dismiss is not a modal guarantee), `dialog` (open-by-default
is **non-modal**; a true modal needs `showModal`, which needs script — the
honest no-JS pattern for a consequential confirm is an interstitial step or an
inline expanded confirm form, not a fake modal), `:has()`, container queries,
`field-sizing`, `:user-valid` / `:user-invalid`, `light-dark()`, `datalist`,
native SVG `<title>` tooltips on graphs.

## 4. Glyphs: adopt the semantics, not the emoji

The attention levels (🔴🟠🟢⚪🔵), the ordered bands, and above all the absence
vocabulary — `0` measured zero, `∅` rounds away, `—` not applicable, `¿` not
observed — are exactly the state language an operations console needs, and
zero-vs-`¿` is literally W09 ("zero differs from absent"). But render them as
CSS shape + colour + text, never as emoji: vendor emoji rendering varies by OS
and projector, and a security console cannot have severity reinterpreted by
the viewer's platform. The vocabulary fixes *meaning*; the medium is ours.

## 5. The fork, on the record

**No enhancement layer. Advanced means information design in plain HTML.**

1. **Security is structural, not incidental.** The rule exists because script
   inherits master-page access. htmx is well-behaved and self-hosted, but
   `hx-on:` attributes are inline script under another name, and the real
   exposure is that any future template slip becomes script execution on a
   master's page instead of broken markup.
2. **The interaction budget is near zero.** Every verb here is form-POST or
   GET navigation; W11 deliberately removed auto-refresh; and not one of
   W01–W17 is fixed by partial DOM updates — they are all information
   architecture, forms and data shape.
3. **It is a one-way door.** Templates accrete `hx-` attributes until they are
   meaningless without the layer. Staying plain preserves a per-page option
   later, under a written threat model, never on feed/master pages — the only
   adoption shape I would ever endorse.
4. **Honest costs, stated.** Sort and filter are round-trips (bounded data —
   a 100-envelope ring, a paged directory — makes them cheap); no instant
   search; no copy-to-clipboard buttons; chart tooltips limited to native
   SVG `<title>`. Acceptable at this scale; revisit only if data volumes grow.

And the question is badly posed as a technology referendum: htmx buys partial
updates, not "advanced UI". For an operations console, advanced is density,
scannability and honest states — and no library delivers those. What should go
to the owner is *what "advanced" is being asked for*, not which library.

## 6. templ vs html/template — design angle

**templ, if the owner accepts build-time codegen** (one more line in
`build.sh`; runtime-identical output; zero bearing on the no-script rule).
The design-relevant gain is enforcement of **structure**: typed components
make the house patterns non-optional — a `DataTable` that requires caption
and `scope`'d headers, a form-field partial with label/help/error slots —
and field-name and type drift become compile errors. **What it does not
catch, stated so nobody buys it for the wrong reason:** wrong-but-valid
prose and semantics. W05 (help promising credentials survive when the
daemon forgets them) was a true-for-the-type, false-for-the-world string;
W06 (credential identities labelled "User") was a valid type carrying the
wrong meaning. Neither class compiles wrong in any template language — the
guard for those is the truthful-pages test discipline the repo already has
(`TestPagesDoNotPromiseWhatTheDaemonRefuses`), and it stays necessary under
templ too. Component files double as the design-system inventory. If codegen
is vetoed, `html/template` plus a strict partial contract is workable;
consistency becomes discipline instead of a guarantee. Either way the design
decisions are identical — this choice must not block any design decision,
and the owner question should say so.

## 7. IA critique

### The three decisions you asked about

**Channels as its own destination — right, keep it, and cap the cost.** The
split is justified by the data, not taste: a channel's primary column is
delivery mode + subscribers; a service's is reader/queued state. Different
tables answering different questions — exactly what W-type sharing buried, and
a filter on a shared list would make mode a sticky URL parameter, which is
W13's lost-state problem wearing a costume. The doubling is the part to
manage, so say explicitly in components.md: two destinations, **one detail
template family** — Service and Channel detail share queue/access/configuration
sections and differ in the mode/subscribers block (channels) and owner/liveness
block. The empty-state obligation comes with the split: a deploy with zero
channels gets a page that must explain what a channel is and how to create
one, or the split reads as a bug. And Overview attention links must route by
kind (a stuck channel → `/channels/{name}`) so the split never fragments an
incident.

**Path segments over `?name=` — the decision as stated is wrong, because the
cost it names is not the real one.** `@` costs nothing: RFC 3986 permits `@`
unencoded in a path segment, and `/services/svc@realm` works as-is in
browsers and Go's ServeMux. The real cost is the **slash**: launcher-registered
agent names contain `/` (`codex/run-42@realm`), and the audit counted 206 of
232 directory names with a slash — majority case, not edge. In a path segment
that must be `%2F`, and everything then depends on escaped-segment matching
and unescaping behaving end to end (ServeMux, hand-built hrefs in templates,
redirect targets, `Return` round-trips). That is verifiable but must be
verified *before* the URL shape is baked in — one round-trip acceptance check
with a slashed name, slash-encoded name and `@`-name against every route.
Recommendation: keep the path-segment ambition only if that check passes and
lands in the technology doc; otherwise `?name=` everywhere is the robust
default — and one rule for all resources, never segments for some and queries
for others.

**Killing the registry table from the homepage — right kill, two guards.** The
table answered no question; it was on `/` because the data existed — the exact
anti-principle in the README. Guard 1: Overview's attention items need an
enumerated source list in pages.md or "needs attention" becomes vibes —
proposed: unclean stop, non-zero refusals, backlog entries, at-bound queues,
per-name losses; each with a glyph-band severity and a link to the thing.
Guard 2: the benign empty case must be **scoped, not absolute** — the Overview
only observes what the caller may see and what retained history holds, so the
honest sentence is "no observed issues in this view", with the view's scope
and an as-of timestamp (no auto-refresh, W11). An unqualified "nothing needs
attention" is W15's scope-mixing one level up — corrected from this review's
own first draft, which made exactly that mistake.

### Additional findings

- **The Problem page is one page for four causes** (refusal, expired session,
  unavailable bus, not found). W12's whole finding was that these need
  *distinct* recoveries; one shell is right but it must be four bodies — pages.md
  should spec four states, each with its own corrective step, or the conflation
  returns wearing a nicer template.
- **Two journeys are missing, and they are different questions.** "Is work
  flowing?" is aggregate — Service → Queue counters (accepted / dequeued /
  queued / oldest) answer it, and it is absent from the journeys table. "Did
  **my** message arrive, and was it receipted?" is per-message — aggregate
  counters cannot answer it; only the retained envelope evidence and receipts
  can, party-scoped, in Diagnostics, with the bounded-history caveat the
  truthful-exchanges work already established. Split correction from this
  review's first draft, which wrongly offered queue counters as the answer to
  the per-message question.
- **Account documents credential rotation — the consequence text is
  load-bearing wherever it appears** (W05-class promise hygiene): rotation
  keeps the previous generation working and drops the one before it. As
  drafted, Account carries command help, not a control — the requirement then
  is that the help copy state the consequence exactly; if a rotation control
  is ever proposed, the same text belongs beside the button, not three
  paragraphs away. (No such control exists or is approved today.)
- **Groups detail page: downgraded from my first verdict.** I argued "keep it"
  because W08 needed somewhere to show why a group cannot be deleted — that
  rationale is superseded by the owner's settlement that groups are not
  deleted at all, which makes the refusal a global fact worth one sentence,
  not a page. What remains is members plus what uses the group (ACL and
  record references) — genuinely useful, and exactly the IA's own open
  question of whether it fits on the list. I take no side; I withdraw the
  deleted-based justification.
- Positives worth keeping verbatim: the "what moves" resolution table,
  accepted/dequeued over in/out, reader-attached over Offline, enabled/disabled
  over Active/Inactive — each replaces an inference with an observation, which
  is the same discipline the truthful-exchanges work established.

## 8. Owner questions

- **Q58 (the fork):** pose it as "what is *advanced* being asked for" per §5.
  My answer on the record: information design in plain HTML.
- **Q59 (design system):** decided by procedure, not by this file's lean —
  list the components the dashboard needs, compare Bulma's, Tabler's and a
  house system's chosen subsets on exactly those (keyboard, narrow, assets,
  licence, adaptation effort), run the disabled-script browser check on the
  shortlist, and take that comparison to the owner. Every framework's
  interactive half is dead under the no-script rule; the comparison is about
  the static halves and the cost of carrying them.
- **Q60 (templ):** ask it, with a stated default — "templ unless vetoed;
  changes nothing rendered". Carry the independence finding (no coupling to
  the script question) into the question text so the owner cannot accidentally
  buy script by saying yes to modern Go, and cannot think refusing it protects
  them from script either.
- **Q61 (dark mode):** agreed with the IA owner — build the token layer
  `light-dark()`-capable from day one (retrofit cost is real: every hardcoded
  surface colour becomes a migration) and let the owner decide whether it
  ships. Addition: contrast acceptance checks should run in **both** schemes
  regardless of the answer — the #888 lesson (W11) must not be relearned in a
  second palette.

  *Outcome note, 2026-09-16: superseded. The owner settled one light palette,
  no scheme control (visual-design.md, tokens.md, README Q61) — "one good
  design rather than two that both need maintaining". The dual-scheme
  recommendation above is advice-history; the contrast obligation stands,
  over the one palette.*

## 9. Specs round 1 — critique of pages/forms/components (2026-09-16)

Owner decisions taken as given (no script, house layer, templ, dark mode
shipped). The [dissent](../README.md#dissent) is noted and already
incorporated: my §2 grades Bulma/Tabler/AdminLTE symmetrically ⚠️ — the
contested claim was my pre-correction position, and no live disagreement with
codex remains. The dialog rule in forms.md is endorsed without reservation:
server-rendered confirmation because authorization and conditions are
rechecked at submission is the face-side application of the same
operation-time discipline the daemon spent 0.5.31 on.

**Eight forms — right count of concerns, wrong shape if "Manage" stays a
block.** components.md's own detail-section definition already carries the
answer: "the form for that one concern" lives *inside its section*, not in a
separate stacked Manage area. Resolve the pages.md/components.md divergence
in favour of that: identity carries metadata + maintainers + enable/disable;
queue carries queue policy; access carries the ACL editor; configuration
carries replace-config; subscribers carries subscribe/remove-subscriber.
Transfer becomes a link ("Transfer ownership…") opening its confirm flow, not
an always-rendered form; remove sits at the page end with its confirm. Then
no page ever renders eight forms — it renders sections with at most one
collapsed edit each, and the count stops being a usability variable.

**Attention set — two additions with daemon-derivable basis, one
discoverability candidate, two structural rules — and two basis corrections
I originally passed (caught by codex's round one).** Additions: (1) a
disabled record with queued work — those messages can be neither delivered
nor read (waiters are released on disable), so the work is trapped; orange.
(2) services of a suspended owner — under H.5.7 semantics they authenticate
403, so registered-looking services are effectively dead; orange, links to
Users. Candidate for the owner: a blue count of unregistered credentials
awaiting review (the cleanup cohort exists and C10 promised discoverability —
Overview is where it is findable). Structural: (a) **dedup rule** — one record yields one attention item, at
its worst admitted level; the rule needs no assumption about which
observations co-occur. (b) **split from the Node strip** — losses and
refusals appear in attention (and Diagnostics) only, not again as node
totals, or the strip and the items disagree with themselves.
Corrections, conceded from my first draft and refined again against source:
(c) pages.md's "backlog older than its own TTL" cannot date existing queued
messages — a message's expiry moment was fixed at accept from the TTL then
in effect (envelope.go), so the current record TTL is the wrong ruler twice
over; the honest basis is a daemon-provided per-message fact (oldest
message's Expires, or an expired-not-yet-pruned count), or the item does not
ship. (d) AtBound proves capacity and nothing more — and even a future-tense
sentence overclaims: enqueue prunes expired entries before testing fullness,
a matching waiter can take a message straight through without queueing, and a
reader can drain the queue after observation. The honest form predicts no
event: "at capacity when observed; if enqueueing still finds it full after
expiry pruning, the configured overflow policy applies" — or in UI length,
"At capacity" plus the policy. Severity is a call on capacity risk, not on
any loss event.

**Services table — one column refined, one is mode-blind.** Kind is
retained on the default mixed list — there it disambiguates generic services
from agent sessions and carries the registry-removal precondition (C08:
kind, description and accepted/dequeued on the lists before the homepage
registry goes) — and omitted only where an active kind filter makes the
column uniform, which is when it stops earning its width. The channel list's
Queued column is blind to delivery mode: a pub/sub topic keeps no queue of
its own (the daemon counts publications and fans out; nothing waits on the
topic itself — copies land and may wait in each subscriber's own inbox), so
its Queued cell is structurally meaningless — either make the column
mode-aware (pub/sub rows show accepted; queue rows show queued) or
leave `—` with the not-applicable semantics from glyphs.md. Stating this in
pages.md now prevents a shipped always-zero column.

**Showing data because we have it, remaining instances — one finding
retracted.** I proposed dropping Account's credential-row Owner column as
"always the reader"; codex's round one shows that is false for service and
session principals: /names fills Owner from the record, a session's record
owner is its launcher (who differs from the reader), and the unregistered
case carries the caller as owner by definition — while owned records without
credentials are absent entirely. The column stays; whether to render it only
when it differs from the reader is a visual-design decision. Everything else
audited in pages.md earns its place; the closure discipline (every inventoried
field assigned a home/demotion/removal) is the strongest thing in the
document set and should survive into the acceptance checks verbatim.

**One missing spec: the post-confirm recheck failure.** forms.md says the
daemon rechecks at submission — right — but not what the person sees when the
recheck refuses after they confirmed (the thing they confirmed has changed:
remove-credential's "now backed by a user… refresh" refusal is the canonical
case). That needs its own problem presentation — "conditions changed, here is
what is true now" — or it lands in the generic Refused state reading as a
bug. One row in the Problem four-state table, or a fifth, narrowly scoped.

**Round-one-response corrections, independently verified in source.** Two of
codex's findings against the response commits, confirmed here so the record
carries two verifiers: (1) an attempted unregister prunes expired messages
*before* the busy check (unregister.go:77-81) — so a confirmation promising
held messages "go with" the record invents a purge: with work waiting the
daemon refuses ErrBusy, and the refused attempt has still expired everything
already expired (Expired+1). Confirmation copy for removal must say the
daemon refuses while work waits — drain first — and must not count the queue
as leaving with the record. (2) Activity history is this-run's and is never
restored (activity_test.go:49-53 asserts a fresh process holds zero
samples), so no cross-restart negative delta exists to clamp — any spec
sentence describing such clamping describes a system that does not exist;
the restart boundary is stated, not compensated. Also endorsed from that
review: a fixed density cap alone cannot catch tight leading or wrong
ratios — the spec needs a minimum or a ratio basis, not only a maximum.
