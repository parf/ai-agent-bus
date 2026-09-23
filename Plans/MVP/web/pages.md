# Pages

Draft for peer review. Every field codex
[inventoried](review/current-state.md#visible-fields-by-page) is given a home,
a demotion or a removal here; nothing is left unassigned, because "hide the
junk" is only checkable against a named list.

## How to read this

| | |
|---|---|
| **keep** | shown on this page, as it is |
| **move** | belongs on another page; named |
| **demote** | stays, but below the fold, inside a disclosure, or only in detail |
| **relabel** | the value is right and the word is wrong |
| **drop** | not shown anywhere |

Every page has: one `h1` with its [page image or glyph](glyphs.md#page-title-images-and-glyphs),
a stated observation time where it shows observations, a toolbar where it lists
things, and the four states
([components](components.md#states)) — populated, empty, denied, unavailable.
None of those is optional, and "unavailable" never renders as "empty".

The title gets at most one short factual subtitle. Long definitions and usage
instructions use the shared [ⓘ help pattern](components.md#compact-help-not-prose-walls)
with a short bulleted list. Current conditions, form constraints, errors and
consequences remain visible where the person acts.

---

## Overview `/`

**Answers:** is anything wrong right now?

Today `/` is a seven-section diagnostics wall
([inventory](review/current-state.md#diagnostics-fields)). It becomes the only
page that is allowed to be short.

| Section | Content | From |
|---|---|---|
| Heading | Node name, observation time, explicit Refresh. No auto-refresh | keep `At`, keep Refresh |
| Needs attention | Zero or more items, each: what was observed, when, and a link to the thing | new |
| Node | `Up`, and the node-wide totals **labelled as node-wide** | move from Diagnostics; [C16](review/codex.md#junk-and-misleading-content) |
| Find | Prominent entry to agents and queues holding work, and to Services | new |

**Attention items are enumerated, not judged.** The complete admitted set, each
with the observation it rests on:

| Item | Basis | Level |
|---|---|---|
| The last stop was not clean | `Status.Unclean` | red, stated as a fact with no lifecycle. Shown **only when true**: the field is `omitempty`, so absent means a clean stop *or* no previous stop and the face cannot tell them apart. Never print "clean shutdown" as an observation |
| Queue at capacity when observed | `AtBound` | red. **A condition, not a prediction.** Even "the next send is refused or drops the oldest" overstates it: the observation path does not prune, while the send path prunes first, can hand a waiting reader the message directly, and races a concurrent consume ([glyphs](glyphs.md#what-an-observation-is-worth)). Name the configured overflow policy beside it as a setting |
| ~~Backlog older than its own TTL~~ | — | **cut.** The record's TTL is the wrong right-hand side: the deadline is per envelope, fixed at accept from whichever of the sender's and the queue's TTLs was shorter, so a live head accepted under a longer setting can exceed today's without anything being wrong ([glyphs](glyphs.md#attention-levels)). home-parf's find |
| Losses, cumulative across restarts | `Dropped`, `Expired` non-zero | orange, links to Diagnostics. **Not** "since start": `Restore` puts these counters back from the snapshot ([dictionary](data-dictionary.md#state)), so a non-zero total may predate this run entirely |
| Refusals, cumulative | `Status.Refused`, a `map[string]int` by reason | informational; **not** "climbing" — this value is a lifetime total. A supported reason absent from it is a measured `0`, not an unknown ([dictionary](data-dictionary.md#node-and-scope)). A windowed signal exists on Activity but is scoped: node-wide refusals only on the unfiltered daemon-Owner view, a sum over visible records otherwise |
| **Disabled record holding queued work** | the **stored** `Disabled` bit and `Queued` > 0 — see the caveat below | orange. opencode's find, and stronger than first stated. `Send` refuses at bus.go:532, `recheckInbox` releases existing waiters with `ErrDisabled` (manage.go:292), and `ConsumeAs` refuses a new reader at bus.go:801 — so the backlog can be neither delivered nor drained. home-parf's extension, corrected twice: **the ordinary paths do not drain it**, because the send-path and consume-path prunes sit behind those guards. That is not the same as nothing being able to expire it. `UnregisterAnd` has **no disabled guard** — an attempted removal prunes first (unregister.go:77), so `Expired` can move on a disabled record, and then refuses with `ErrBusy` while any live message or reader remains. codex reproduced it: one expired and one live message, disable, unregister → `Expired` increments, the live message and the record survive, removal refused. So the page says only what is observed — **delivery is off and work is held** — and promises nothing about drainage, expiry or removability |
| **Services of a suspended owner** | the owner's `State` is paused or banned | orange, linking to the person. [H.5.7](../done/suspended-owner.md#checks) shipped in 0.5.34, so the rule is in force and the item describes it: arrivals are refused and the queue cannot be drained by anybody |
| Unregistered credentials awaiting review | the cleanup cohort is non-empty | blue, a count and a link. **Owner-decidable**: it is discoverability rather than attention, and [C10](review/codex.md#junk-and-misleading-content) left it homeless |

A backlog with no reader is **not** an item. A queue worker between pulls is
exactly that shape and nothing is wrong.

**The `Disabled` we are given is not the `Disabled` that freezes a queue**, and
this item is where that matters. `visible` (manage.go:305) returns
`r.Disabled || !b.active(r.Name)` — one bit merging two causes — so the face
cannot tell an owner's decision from a name that has stopped being active. codex
raised this as a truth defect ([S04](review/codex.md#specification-review-round-one));
it is also an operational one, because **the three causes do not behave alike**.
The third arrived after this table was first written: a suspended owner is
deliberately kept out of the `Disabled` bit so a face can answer it separately
(core/users.go `suspension`), which means a page reading only that bit cannot
see it at all.

| Cause | Arrivals | Drainage |
|---|---|---|
| Stored `Disabled` | refused | refused — `ConsumeAs` checks this bit, so the queue is frozen |
| Name not active | refused with `ErrInactive` | **still possible** — `ConsumeAs` guards the caller's standing and the stored bit, never `b.active(name)` |
| Owner suspended ([H.5.7](../done/suspended-owner.md#checks)) | refused with `ErrInactive` — `Send` asks `suspension(to)` | refused for everybody — `ConsumeAs` asks `ownerSuspension(name)` before it reaches the stored bit |

The divergence is narrower than that table alone suggests, and home-parf's
sharpening earns its half-sentence: `Consume` calls `ConsumeAs(name, name)`, so
a session reading **its own** inbox is refused either way — `acting(caller)`
catches the inactive case. **The two states differ only where some other
principal may drain that inbox**, and through the face that is narrower still.
Through the HTTP face an active third party can drain a permitted topic inbox
using `topic=<registered topic name>` with no tag; an arbitrary service inbox
cannot be selected with `name=`. Core `ConsumeAs` does accept an explicit inbox,
so the delegated read exists in the daemon; what a face can reach is the topic
case. Without saying so, the first operator to test the distinction on their own
session finds it absent and concludes the page is lying.

So an item that reads "disabled and holding work" is urgent in the first case
and merely blocked-at-the-front-door in the second, and today's answer cannot
say which. **Until the daemon separates them the item is written to the weaker
claim** — *delivery is off and work is held* — and does not promise the backlog
is unreachable. Separating them is a daemon change and is named as
[owed](#owed-by-this-specification), not assumed. The owner settled Q63 in the
[user-state access rule](../../../docs/01-identity-and-roles.md#user-states);
the dashboard must preserve that distinction.

Two structural rules, both opencode's:

| | |
|---|---|
| **One queue, one item** | Each record yields a single item, at its worst level. No claim about which conditions co-occur is needed or made: two drafts attached one to this rule and both were unsupported, and the rule never required either — codex |
| **Never counted twice** | Losses and refusals appear as attention items and in Diagnostics. They do **not** also appear in the node strip, or the strip and the items disagree with each other on the same page |

**Empty state: none.** At 0.5.83 the owner removed the section entirely when
nothing was observed — no heading, no explanation, nothing to read. Two earlier
rounds cut the observation time from this sentence and then the sentence itself.

The reasoning the empty block carried is retained and is not contradicted: it
existed so that absence of an item could not be read as *nothing is wrong*
(codex's [S03](review/codex.md#specification-review-round-one)), and a page that
says nothing makes no such claim. What the owner rejected is a heading and a
paragraph spending attention to report that there is nothing to report. The
admitted set is still a handful of conditions the daemon reports, and absence is
still absence of *these* observations — that is now [stated in the
documentation](../../../docs/05-discovery.md#overview-and-diagnostics) rather
than on the page. **Not** *"nothing needs attention"*, which was
the draft's wording and is unbounded — codex's
[S03](review/codex.md#specification-review-round-one). The admitted set is a
handful of conditions the daemon reports; a reader whose process died leaves a
queue that trips none of them. Absence of an item is absence of *these*
observations, not assurance. Never "healthy", for the same reason one size
larger.

**Queued work keeps a direct route**, which the draft lost when the backlog
table became an exception list — codex's
[S03](review/codex.md#specification-review-round-one). Agents and Queues
sort by queue depth and filter to *holding work*, so every caller-visible queue
is reachable in one step with its depth, its head age and its reader state
shown as observations. The exception list is what the Overview *promotes*; it is
not the only way to see a queue. A journey that can only reach a backlog when
severity fires cannot answer "why is work not arriving", because the answer is
usually a queue that trips nothing.

**Node totals and the lists never have to agree**, and the page says so: one is
node-wide, the other is what this caller may see
([dashboard audience](../../../docs/05-discovery.md#dashboard)).

| Leaves this page | To |
|---|---|
| stuck-inbox table | attention items, one per exceptional queue, each linking to its service |
| retained exchanges | Diagnostics |
| the whole registry table | the Agents, Services, Queues and PubSub lists — **only once those carry kind, description and accepted/dequeued**, which they do not today ([C08](review/codex.md#junk-and-misleading-content)) |
| loss by name | Diagnostics, and the record's own Queue section |
| my names, fingerprints, rotation help | Account |
| three trailing explanatory paragraphs | beside the control each explains |

---

## Agents `/agents` · Services `/services` · Queues `/queues` · PubSub `/pubsub`

**Answers:** what can I use, who owns it, and is it in trouble?

One list per kind, **one component family**. A channel's first question is how it
delivers and who subscribes; a service's is whether anything is reading and what
is queued. One table answering both is what buried delivery mode
([C03](review/codex.md#junk-and-misleading-content)). Since 0.5.78 each page has
its own document title, heading, navigation state, columns and detail route.

**0.6.3: one listing per thing a record is**, decided by the [five
kinds](../../../docs/03-records.md#record-kinds) and superseding
the 0.5.84 arrangement below. Agents is first in the menu and holds 👾 alone,
because agents are what the bus exists to carry messages between; Services holds
📡 alone, which is the external case; Channels holds 📮 📣 👤 and keeps the Kind
column and filter. **0.8.4 splits Channels** into Queues (📮) and PubSub (📣),
each a section with its own list, registration and settings, and no Kind
filter ([web face](../../../docs/05-discovery.md#agent-service-and-channel-journeys)).
Personal is a view across every kind, since any record
may be Personal. Delivery mode is no longer a field, so the mode filter is gone: a
queue and a pub/sub topic are kinds of their own.

*History, superseded by the paragraph above.* **0.5.84: an agent's record is an
inbox and belongs to the channels page.** It carried no address and no protocol,
so nothing was served from it; the [messaging
rule](../../../docs/04-messaging.md#inbox-queues) already called it an implicit
queue topic named after the agent, and listing it under Services contradicted
that. Services then held `generic` records alone and dropped its Kind filter.

| Column | Decision |
|---|---|
| Description, then full routing name beneath | **new + keep.** Identification is by address only today; the description exists on records and is how a session is recognised ([C01](review/codex.md#junk-and-misleading-content)). The full name stays because it disambiguates sessions — demote by task, never drop |
| Kind | **A filter wherever the list is already narrowed to one kind**, because a per-row category is then redundant with the control that got you there. **A column on any list that is not** — an unfiltered or mixed-kind list must still say what each row is, and no filter is supplying that. This governs the [mockups](layouts.md#services--the-page-the-density-is-tuned-against) as much as this table; the draft's flat "filter only" was wrong and the two documents disagreed — codex |
| Delivery mode | **superseded in 0.8.4.** The page is the mode: Queues or PubSub |
| Owner | keep |
| Owned by the caller | **always mark.** A blue leading rule and blue semibold name survive All, My, Personal and filtered results without repeating *Yours*; the My category link uses the same blue. Personal independently keeps its visible *Personal* word with stronger orange bold emphasis and category link; orange overrides blue on combined rows |
| Enabled / Disabled | **relabel** from Active/Inactive. Administrative state, not liveness. Disabled is a decision, not a failure |
| Readers | **numeric observation.** Count every outstanding filtered and unfiltered read; keep measured zero separate from unavailable. `Proto` remains an independent caller-supplied external hint |
| Queued | keep on Agents and Queues (Queues calls it Held). **Absent from PubSub**: a pub/sub topic keeps no queue of its own — `Send` hands it to `fanout` and nothing waits on the topic — so a Queued cell there is structurally zero. PubSub rows show accepted and copies out instead |
| Deliver-To (PubSub list) | **new** |
| One judgment column | **new.** Lit only on exceptional rows ([glyphs](glyphs.md#where-a-glyph-is-allowed)) |
| Updated `At` | **compact on the list, full on detail.** Under 30 days use whole-unit age; older values use `Jan 1` in the current year or `Jan 12, 2025` across years |
| `Controls: Manage / View` | **drop.** Inert text shaped like a control ([C01](review/codex.md#junk-and-misleading-content)) |
| `ConfigSHA` | **move** to the configuration section of the detail page |
| `In` / `Out` | **move** to detail, **relabelled** accepted / dequeued. Dequeued is not completed |

**Section navigation:** All (`#`) · My (`#`) · Personal (`#`) · Register
agent or service; Queues and PubSub have no My: All (`#`) · Personal (`#`) ·
Register queue or Register pub/sub topic. It is the second row beneath the
global navigation and follows the shared
[section-navigation rule](information-architecture.md#navigation). All and My
exclude Personal agents, preserving the accepted dedicated Personal grouping;
My is the caller-owned subset of All. Counts are caller-visible category totals
before the search, state and kind filters, so changing a filter does not make a
navigation count describe a different category.

**Built in 0.5.64:** these category links and counts, dedicated Service and
Channel registration routes, Delivery links, create-time Kind/Delivery radios,
and the separate ownership and daemon-authorized detail controls. 0.5.66 adds
owned/Personal emphasis, one name-to-detail route and compact update time; the
2026-09-17 refinement removes the repeated Yours word and matches the My and
Personal link colors without a version bump.
0.5.67 embeds the record-scoped Activity graph and its link to the complete
filtered view.
0.5.72 builds search and sort. 0.5.77 adds the independent Readers filter,
25-row paging, matching counts, clear-filters action and exact list-state
return from detail. 0.5.78 completes the populated owner/visitor Service and
Channel journeys: delivery-mode filtering, mode-aware Work, subscriber facts,
distinct empty states and resource-local registration/edit returns.

The title's `ⓘ` help contains the category definitions as bullets — All is
caller-visible non-Personal agents, My is the caller-owned subset, Personal
is the separately grouped owner view — instead of placing those paragraphs
above the table.

**Toolbar:** search over description and name; Status; Readers; Queue (holding
work); sort. The
service view choice moved to the section-navigation links above. Two- and
three-value filters expose their choices as stateful links or buttons rather
than selects. All state is in the URL as GET parameters, retained through paging
and through a visit to a detail page and back. Result count and active filters
are shown, with one action to clear them.

**Rows have one destination.** The name opens the read-first record
view, where daemon-returned authority decides whether controls appear. A
caller-owned row is always visually marked, but the face never derives
permission from that marker.

**Ordering is stable and named.** Core's `List` iterates a map, so order out of
the daemon is not stable — but **the record lists already sort by
name** (admin.go:189 when reviewed), and the draft was wrong to call it an unstable load.
[W16](../done/web-review.md#findings) survives where nothing re-sorts: the
diagnostics registry. codex's [S14](review/codex.md#specification-review-round-one).
What this specification adds is a *chosen* order with the sort in the URL,
rather than an incidental one.

**Empty states are three different pages**, and this is the split's price:

| | |
|---|---|
| No services at all | what a service is, and the link to register one |
| No queues or topics at all | what a queue or pub/sub topic is and how one is created — without this the split by kind reads as a bug |
| No matches | the active filters, and one action to clear them |

---

## Agent `/agent?name=` · Service `/service?name=` · Queue `/queue?name=` · Topic `/pubsub/topic?name=`

The legacy `/channel?name=` still serves any queue or topic, for old bookmarks.

**Answers:** what is this, is it working, who may use it — and then, separately,
what may I change?

Today this is an operational summary followed immediately by a configuration
digest and an always-open wall of six editors
([C02](review/codex.md#junk-and-misleading-content)). It becomes: read first,
then focused edits.

| Section | Content | Visible to |
|---|---|---|
| Identity | Description, full name, **address**, **protocol**, kind, owner, the owner's caller-visible local photo when the owner is a User, maintainers, enabled state, **updated time** | anyone who may see the record |
| Queue | Queued, oldest, accepted, dequeued, dropped, expired, at-bound, **overflow policy**, TTL and capacity with inheritance stated per field | same |
| Access | Allow list, including the runtime `@owner` term, and what that means in a sentence | same |
| Subscribers (pub/sub only) | Each subscriber, linked where the caller may inspect it | same |
| Configuration | Whether one is set, and its digest as evidence. **Never its contents** | same |
| Activity | Compact record-scoped graph for accepted and dequeued traffic; exceptional dropped, expired and refused series when non-zero; a link to `/activity?name=` for the complete graph and sample table | same |

**The read sections do not depend on edit permission.** Address, protocol, ACL
and queue policy are already in the daemon's answer to this caller and today are
rendered only inside the editor
([C04](review/codex.md#junk-and-misleading-content)). Controls are conditional;
returned metadata is not.

**Activity is embedded only where the daemon can scope it honestly.** Record
detail requests the same bounded history as
`/activity?name=<record>`. The compact section states the observed window and
restart boundary, summarises zero-only series instead of giving each a full
graph, and links to **View all activity** for the complete graph and accessible
sample table. Users and groups get no inferred roll-up: the activity API does
not provide a user- or group-scoped series.

The section's `ⓘ` help carries the sampled-window limits, restart behavior and
the meaning of *dequeued* as bullets. The observed graph, time range and current
absence state remain visible; help does not hide the evidence it qualifies.

Four fields were required in prose and missing from the table above, so the
table is where they now live — codex's
[S07](review/codex.md#specification-review-round-one). **Address** and
**protocol** are read-only in Identity; **updated time** is the detail home the
list demotes it to, and had nowhere to land; **overflow policy** belongs in the
read-only Queue summary, not only in its editor. A requirement stated in a
paragraph and absent from the section map is a requirement an implementation
will miss.

| Field | Decision |
|---|---|
| `Maintainers` when empty | **drop the label with the value.** An empty "Maintainers:" is an unfinished sentence ([C11](review/codex.md#junk-and-misleading-content)) |
| `Oldest` when empty | `—`. The queue is empty now; the daemon does not say whether it ever held anything |
| `ConfigSHA` when empty | **drop.** It leads the page today and is often blank |
| Direct-reader state on a pub/sub channel | **drop.** A pub/sub topic showing "Offline" is meaningless ([C03](review/codex.md#junk-and-misleading-content)) |
| Both Subscribe and Unsubscribe buttons, always | **relabel to one**, reflecting current state |

**Each edit lives inside the section it changes**, not in a Manage block after
them. My first draft drew both and contradicted
[components](components.md#the-set); opencode caught it and components wins,
because a trailing block of eight forms is the wall we are removing.

| Section | Its one edit |
|---|---|
| Identity | metadata, maintainers, enable/disable |
| Queue | queue policy |
| Access | the allow list |
| Subscribers | subscribe/unsubscribe, remove a subscriber |

Each is collapsed until asked for, so no page ever renders eight open editors
and the count stops being a usability variable. Replace configuration,
ownership transfer and registration removal appear only after following the
red **Danger Zone** link to its server-rendered subpage. They never render on
the ordinary detail page. Transfer and removal then open their own fresh
confirmation pages.

The Access and Maintainers editors use the same line-list textarea. Each line
names one user, group, agent or service; Access may also contain `*`. Blank
lines are ignored; invalid lines stay in place with line-specific errors.
Neither editor accepts display glyphs. Maintainers is the daemon's real
[authority list](../../../docs/01-identity-and-roles.md#record-authority), never a
single group rendered in a larger control.

The owner photo is decorative beside the linked owner name and comes from the
same authorized local-thumbnail path as the Users page. If the owner has no
caller-visible User profile or photo, the page renders the existing entity
label without guessing or making a provider request.

For a caller-owned record, the detail heading also carries **Edit**. It opens
the first applicable editor while retaining the section-specific editors below;
the heading action is an entry point, not a second form or a wider grant.

Each returns **to the section it changed** with a specific result — not to
`/services?scope=my`, which is where every service and channel action landed
before 0.5.78, so a channel subscription answers by leaving the channel
([C06](review/codex.md#junk-and-misleading-content)).

Transfer and maintainers assignment are **owner-only**, and that is the
authority contract rather than an editor defect: core restricts both to
`r.Owner` deliberately. A maintainer who may manage a record still may not give
it away or change who maintains it.

### Danger Zone

**Built in 0.5.63.**

The red **Danger Zone** link on a record detail page opens a server-rendered
subpage titled for that resource. It contains exactly three authorized entries:

- **Replace configuration** opens an always-empty textarea; private
  configuration is never prefilled.
- **Transfer ownership** opens the fresh consequential confirmation flow.
- **Remove registration** opens the fresh consequential confirmation flow.

The ordinary detail page shows the configuration digest but none of these
forms. The Danger Zone includes a plain return link to that detail page. Red
identifies this one navigation entry; it is not used to decorate the whole page
or to replace the consequence text.

---

## Register `/agents/new` · `/services/new` · `/queues/new` · `/pubsub/new`

**Answers:** how do I create one?

A dedicated page reached from the section's second-level navigation, not a form
stapled beneath a list. Fields: name, description, initial allow list, and what its kind has
([forms](forms.md#the-set)). **From 0.5.84 each page registers only what it lists**:
`/services/new` registers a service and offers no kind choice, and
`/channels/new` chooses between a queue and a pub/sub topic. A form that
registered a record the page could not then show was the reclassification's
loose end. **From 0.6.3** there are three forms, one per listing: `/agents/new`
registers 👾 and offers the Personal checkbox, `/services/new` registers 📡 and
**requires an address and a protocol**, because the daemon refuses a service
without both. **From 0.8.4** there are four: `/queues/new` registers 📮 and
`/pubsub/new` registers 📣, and `/channels/new` redirects to the one its kind
names. A choice with only two or three values uses radio buttons.
Help beside the name field states the
`user@realm` shape; help beside allow follows the
[ACL contract](../../../docs/02-access.md#acl). The form explains the built restricted default and runtime
`@owner` term without presenting it as an editable group.

Invalid input returns **this form**, with the values preserved and an error
summary. Today an invalid name renders raw JSON and the description the person
typed is gone ([C13](review/codex.md#junk-and-misleading-content)).

---

## Activity `/activity`

**Answers:** what traffic was observed, over what window?

**Built in 0.5.67.** The complete page and compact record detail now share one
presentation: actual sample-time positions, one maximum over the displayed
nonzero series, explicit measured-zero and collecting states, restart/uptime
context, and a partial-final-sample label. The complete page keeps the sample
table; record detail links back with its filter retained.

| Decision | |
|---|---|
| Shared time range across all series | **built in 0.5.67**, replacing the five independently scaled graphs ([C07](review/codex.md#junk-and-misleading-content)) |
| Labelled axes, in real timestamps | **built in 0.5.67**, replacing evenly spaced sample indices ([W09](../done/web-review.md#findings)) |
| Stated units, interval, restart boundary and partial bucket | **built in 0.5.67** |
| Zero series summarised, not given equal height | **built in 0.5.67** |
| `Maximum: 0` repeated down the page | **removed in 0.5.67** |
| Value table beside the graphs | **keep** — it is the accessible path to the numbers |
| Absence vocabulary in the table | `0` measured zero, `¿` not observed. Before the last restart is `¿`, not `0` |

Arriving from a record keeps that filter. The scope control says
what it is scoped to. Every record detail carries the reciprocal link back to
this complete view.

**Four constraints the series carries**, found by home-parf and verified. They
are not presentation choices; three of them change what the graph may be
labelled, and the third is a defect rather than a limit:

| | |
|---|---|
| **The window is about 24 hours** | 145 readings at a ten-minute cadence: one baseline plus up to 144 intervals. The current partial interval may be shorter. No sub-minute trend is claimed |
| **A restart empties the history** | `Restore` puts records and queue counters back and **never restores `b.activity`** — activity_test.go:50–53 asserts exactly that. So a restart leaves the series *absent*, not zero: there is no earlier sample to take a delta against, and everything before the boundary is `¿`. The face still needs `Status.Up` to say where the boundary was, because a short series and a quiet hour look alike. **The draft said a restart clamps to a zero bucket; it does not** — codex's correction of a claim I took from review without checking it against the one already in our own dictionary |
| **A removed record vanishes retroactively** | `total()` iterates the **current** `b.records` for every sample, so a removed record leaves the whole series at once — past buckets included. Two records contributing 2 and 1: remove the first and the historical aggregate reads 1 where it read 3, with nothing marking the change. Stated within one process, which is the only place retained history exists — a restart illustration would contradict itself, since a restart clears the history entirely. **It does not produce a negative delta or a clamped zero**: `prev` and `next` are computed over the same population, so the aggregate simply steps down. I attributed the `max(0, …)` clamp first to restarts and then to removals; neither holds, and the clamp needs no explanation here — codex |
| **The scope switches inside one series** | `total()` sums `Refused` over records the caller may see, but for an unfiltered daemon-Owner view node-wide `s.refused` **replaces** that sum (activity.go:98–99). One series, two meanings, chosen by who is looking. A single static label is wrong for one of the two audiences — and it is invisible when an Owner tests it. The label is computed from the view, never fixed in the template |
| **History covers the records visible now** | the consequence of the row above, stated on the page rather than implied: the window is not a fixed population, and a series is not a claim about records that have since gone |

---

## Diagnostics `/diagnostics`

**Answers:** what happened to this exchange, and what is being refused?

No longer the homepage. Sections: retained exchanges, refusals by reason, losses
by name. The registry table does not come with it.

| Field | Decision |
|---|---|
| Visibility and retention disclaimer | **keep.** It is what makes the rest honest |
| Exchange row: `At`, ID, From, To, Topic/Tag, ReplyTo, folded receipt count | keep; IDs support diagnostics and stay |
| Repeated *"No completion receipt observed in retained history"* | **demote** to the rows where it changes something; the sentence on every row is scaffolding ([C09](review/codex.md#junk-and-misleading-content)) |
| Envelopes column that is almost always `1` | **demote** into the row, shown when greater than one |
| Receipt evidence, match candidates, late qualification | **keep** unchanged. Correlation semantics are settled and are not a design question |
| Loss by name | **keep**, with each name linked to its record |
| Refusals by reason | **keep** |
| Bodies | never |

---

## Users `/users`

**Answers:** who is here, and what may they administer?

**0.8.8: Users only, laid out like the other list pages.** Every name is a
User or an Agent, so the "other identities" section and its **Other** filter
are gone; a leftover name is listed on Diagnostics while one exists, and
`/users?kind=other` redirects there ([overview and diagnostics](../../../docs/05-discovery.md#overview-and-diagnostics)).

**Section navigation:** All (`#`) · Register user. The count covers the
caller-visible Users before search. The register entry appears only for
callers who may create one and opens `/users/new`; it does not sit as a
separate primary action beside the directory heading.

| Field | Decision |
|---|---|
| `PeopleCount` | **keep**, with scope labelled: caller-visible Users, before the search |
| Search, Status filter, paging, `Matched` | keep, in one `record-toolbar`; a search with no match gets an empty-state card with a way to clear it |
| Person name, email, GitHub login, authority | keep; email and GitHub login share a Contact column |
| Agents | count of caller-visible agents each User owns, computed by the face and said so in the help |
| Last used | when the User's own credential last made a call, from the listing; `never` when unused, a dash while inactive, since the inactive listing does not report it |
| User state | **0.5.83: not a column**, kept in 0.8.8. Nearly every row reads active, so a column would spend width on the answer nobody is looking for. An inactive person is struck through and carries `INACTIVE` beside the name, and the directory opens on **Active**, with counted **Active**, **Inactive** and **All states** filters so the hidden rows are declared |
| Search over GitHub metadata | include company, location and Twitter/X handle from the daemon; the existing Email search covers an imported public email. No browser-side provider lookup |
| Photo | **use.** Show the locally imported GitHub photo, then Gravatar fallback, then generated initials. The list uses a small thumbnail beside the name; no remote browser request and no per-row daemon lookup ([W07](../done/web-review.md#findings)) |
| **Register user** | **entry point, which the draft lost** — codex's [S07](review/codex.md#specification-review-round-one). The form exists in [forms](forms.md#the-set) with no page offering it. Its conditional entry now lives in the second-level navigation and opens `/users/new`. Invalid input returns that form with its values and a field-level error, never a problem page. Success lands on the new user's page. Until 0.5.32 the SSH verb could not onboard anybody either ([H.5.9](../done/user-add-provisions.md#scope), now shipped), so this page was the only remaining route and had no entry point — which is what made its absence a gap rather than an omission |

## Register user `/users/new`

A dedicated registration form reached from the Users section navigation.
Fields and results are owned by the [forms inventory](forms.md#the-set).
The route, conditional entry and new-user detail redirect are built in 0.5.64;
field-level invalid-input preservation remains part of the larger journey.
SSH public keys are host onboarding state rather than profile fields. The form
links that fact to `agent-bus-admin user add <user@realm> <key.pub>` in compact
help and never invents a web-writable key field.

## User `/user?name=`

| Section | Content |
|---|---|
| Identity | Locally served profile photo or initials, name, person name, state, daemon authority |
| Memberships | Groups, or an explicit no-memberships statement |
| Owned records | Linked, or an explicit none |
| Profile | AgentBus person name, email, GitHub login, company, location and Twitter/X handle; editable only under the existing field authorities. Setting/changing login can fill public values; **Refresh fields from GitHub** replaces them |
| Lifecycle | **Built in 0.5.79.** Current state is always visible as a word and colour. Applicable daemon-authorized actions sit behind the red **Change** disclosure: active offers Pause/Ban, paused offers Activate/Ban and banned offers Activate. Ban is consequential and confirmed |

The page presents one AgentBus Profile, not a second GitHub profile. Provider
source and fetch-time bookkeeping are not profile fields and are not rendered.
The visible photo is a locally imported, normalized thumbnail; no browser
request goes to GitHub or Gravatar. `name` and public `email` retain their
fill-blank rules, which the form no longer explains to the person filling it in
— the owner cut that hint at 0.5.83. Company, Location and Twitter/X are edited
locally.

The Users list receives all visible thumbnails in the directory answer or one
bounded batch read and embeds/serves them locally. It must not call the daemon
once per row. User detail reads one photo with the profile. If optional image
import failed, both views render the same generated initials rather than a
broken image. A thumbnail or initials immediately beside the visible user name
is decorative (`alt=""`); the adjacent text remains the accessible name. The
same rule applies to the detail-title photo because its `h1` already names the
user.

GitHub refresh is never implicit page I/O. It is attempted when the login field
is created or changed, and from 0.5.83 that is the only way a page causes it:
the **Refresh fields from GitHub** control was removed by owner instruction, and
`POST /user/github-refresh` remains for an authorized caller. Provider
availability never blocks a valid unique login, and an explicit refresh reports
failure without mutation. Photo import remains
optional and may fall back without refusing that otherwise valid update.

There is no delete-user control and there will not be one: a user is never
deleted, only made inactive
([user lifecycle](../../../docs/01-identity-and-roles.md#user-states)).

For a non-user identity: the kind, why it is retained, and the removal control
only when `CanRemove`, with consequences stated before the form. There are two
kinds and no third: a self-owned record, and a credential with neither profile
nor record. *It owns services, so removal is held* was a third, and it went with
the interim guard it described — a name that owns records without being one is
[deleted at start](../../../docs/01-identity-and-roles.md#orphaned-records), so the
page cannot be shown one.

---

## Groups `/groups` · Group `/group?name=`

**Answers:** what groups exist, who is in them, and what uses them?

**Section navigation:** All (`#`) · Register group. `/groups/new` owns the
registration form; Groups remains a list rather than a list followed by an
inline editor. The register entry is conditional on caller authority.

| Field | Decision |
|---|---|
| Group name | keep |
| Members | **Built.** The list keeps comma-separated members beside each linked group; a hidden membership says *not visible to you*. The linked detail shows the line editor only when daemon-returned authority permits it |
| Records referencing the group | **Built in 0.5.79.** Caller-visible records show direct ACL/Maintainers references and nested paths such as `ACL via @outer`; hidden records never enter the list. Editing members changes who may use or manage every listed record |
| Protected state of `@administrators` | **keep**, stated as protection with purpose help and no delete control offered |
| Delete | **no control.** The handler accepts it and nothing renders it; removing the verb is [H.5.6](../TODO.md#objective), not a UI feature to restore |

The detail page earns its place on the references alone — they are what makes a
membership edit a reviewable decision rather than a blind one.

## Register group `/groups/new`

A dedicated registration form reached from the Groups section navigation.
Success lands on the new group detail page; invalid input returns the same form
with its values and field errors.

The conditional route and form are built. Success lands on Group detail;
invalid input returns the registration form with its safe values and errors.

**0.5.83:** the form uses the shared editor-card and `form-grid` layout that
every other registration form uses, instead of bare stacked fields. The note
that `@owner` is runtime ACL syntax and cannot be registered appears only
against a submitted name of `@owner`, not as standing advice on an empty form.

---

## Account `/account`

**Built in 0.5.79.** The signed-in name opens Account; credential facts moved
off Diagnostics. One caller-visible join supplies the optional profile, own
record and owned records, while `/names` supplies only credentials the caller
holds. A service principal without a directory row still has an Account page.

**Answers:** who am I here, what do I hold, and how do I rotate it?

New page. Everything here is moving off the bottom of the diagnostics wall.

**Account is designed for two principals, not one.** codex's
[S08](review/codex.md#specification-review-round-one), verified: `Users`
(users.go:442–454) admits a name that has a person profile, or that owns itself,
or that holds a credential and is unregistered or self-owned. A **service
principal owned by somebody else has none of those**, so it has no directory row
at all — and our current agent identities are exactly that shape. The page may
not assume a profile, and an empty `/users` answer for yourself is not an
absence of identity.

| Field | From |
|---|---|
| Own identity, person name, authority, memberships | Users detail **when a profile exists**; otherwise the record's own identity, with the profile fields absent rather than blank-labelled |
| Own records, separated from own identity | **the authorized record listing**, not `/names`. `Holds` (tokens.go:196–207) skips names with no credential, so `/names` answers *what you hold*, which is not *what you own*. The draft conflated them |
| Each credential: name, kind, fingerprint, issued, last used | my names. **Owner is kept where it differs from you, and only there.** It is not always you: `Owned` starts with the caller's own name ([bus.go](../../../src/internal/core/bus.go)), so a signed-in service holds its own credential while the record's `Owner` is somebody else — which is the shape of our current agent identities. Dropping the column universally would hide exactly the row it matters on. codex's correction of my error |
| An `unregistered` credential, marked, with what it means | my names |
| Rotation | **the command, as help — there is no button.** MVP scope is command help, and the draft's "button" promised an operation nothing implements. Its consequence sits beside the instruction: the replaced credential keeps working until the next rotation. That sentence is load-bearing |
| *"Envelopes only — bodies are never shown"* | **move** to Diagnostics, which is where it is true |

No page ever renders a credential — a fingerprint, when it was issued, when it
was last used, and the command
([rules](../../../docs/05-discovery.md#rules-it-is-built-to)).

---

## Landing and sign in `/`

Signed out, `/` is a landing page — the project's description, links and
picture — with signing in at its foot; the form posts to `POST /signin`.

Token, and how to get one. The form takes a token and nothing else — there is no
name to type, so there is no second failure message to read as an oracle for
which names exist.

Acquisition help is concise and in the supported order, and describes only what
the daemon can complete. **The limit the draft recorded here is gone**:
`agent-bus-admin user add` creates the user it adds as of 0.5.32
([evidence](../done/user-add-provisions.md#scope)), so the SSH path is a real
onboarding journey and is described as one. What remains worth saying is that it
needs a running daemon, because the verb refuses rather than leaving a key that
works before the name exists ([C14](review/codex.md#junk-and-misleading-content)).

A refusal says the token was not accepted and nothing about which names exist.
The return destination is validated as one of this dashboard's own pages before
it reaches the form.

---

## Problem — recovery by what the face actually knows

Today one template carries every failure ([W12](../done/web-review.md#findings)).
**The draft's replacement was worse than the build in one respect**, which
codex's [S10](review/codex.md#specification-review-round-one) caught: `fail`
(admin.go:56–93) already separates 401, 403, 404, 500 and 503, and the four
states I listed collapsed the last two back together. The defect that remains is
its `default` branch, where 400, 409, 412 and 429 arrive as one *"That was
refused"* — four different situations with four different next actions.

**The face does not receive a reason.** codex's
[R2-4](review/codex.md#review-of-the-round-one-response), verified: `reply`
picks a counter kind at server.go:637 and uses it for `Refuse(c.kind)`, then
calls `fail(w, c.code, err.Error())`, which serialises `{"error": text}` only.
The web child stores the status and the raw payload (main.go:303) and has no
stable discriminator. The `kind` vocabulary is also deliberately coarse — its
own comment says everything a caller got wrong is one kind — so it is the wrong
axis for recovery even if it were sent.

So the table below is keyed on **what the face actually has**: the status code,
and the daemon's own message shown verbatim. Reason-specific bodies within a
code are a *dependency*, named here and listed as
[owed](#owed-by-this-specification), not something a template may infer.

| Code | What the face may say | Fallback it uses today |
|---|---|---|
| 401 | the session ended | sign in, returning here. Already the build's behaviour, and correct |
| 403 | you are refused, and the daemon's message says why | **one body, the message verbatim.** It covers permission, suspension and failed enrolment proof, and the face cannot tell them apart. It must therefore not say "ask the owner for permission", which is right for one of the three |
| 404 | nothing under that name, or nothing you may see | identical for both, deliberately |
| 400 | what was wrong with the input | **the form, with values preserved and the message beside the field where it can be attributed** — never a problem page. This is the one case the code alone is enough to route |
| 409 | the request conflicts with current state | **the message verbatim, and no retry offered by default.** The draft called this transient and recommended waiting; it is not. A credential refused because it is now backed by a person or record will be refused identically next time |
| 412 | the name is taken | the form, with values preserved |
| 429 | the receiver is at capacity | its queue, and the overflow policy in force |
| 500 | a fault, not a rule | repeating is unlikely to help; the node's log |
| 503 | the daemon is there and briefly cannot answer | retry. **Never** an empty healthy list |
| transport failed | the request did not complete | **inspect current state before offering anything**, and never "nothing was changed" — which is not knowable, and a blind retry on an uncertain mutation is worse than a look |
| **conditions changed** | what you confirmed **stopped being true** before it ran | show what is true now, and offer the action again if it still applies ([forms](forms.md#when-the-recheck-refuses-after-you-confirmed)). **Reserved for an established change.** A refusal that happens because the face never had the fact — a filtered waiter blocking removal, say — is the ordinary 409 above, and must not be dressed as a race: telling somebody the world moved when it did not is its own false statement — codex |

Three corrections inside that table were mine, all in the same direction —
promising a recovery more specific than the evidence supports. Enrolment means a
proof failed, not that a name is unenrolled. Busy is not a synonym for
transient. And an uncertain mutation outcome is a reason to read current state,
not to repeat the write.

One reusable template is not the defect; one reusable *reason* is. Hidden and
missing stay deliberately indistinguishable.

---

## Owed by this specification

Nothing here is a presentation problem, and none of it may be solved by a
template inventing a value — codex's
[S11](review/codex.md#specification-review-round-one). Each row names what MVP
ships **instead**, so no page is blocked waiting on a daemon change.

| Wanted | Status | MVP fallback, chosen |
|---|---|---|
| Node label and running build | needs a narrow daemon read | **omit the label.** The heading shows observation time and uptime, which are answered today. No placeholder, no hostname guessed by the face. The web executable's version is not the bus version and is never shown as one |
| Effective TTL and capacity | needs a daemon read to resolve inheritance | **show the declared setting with its actual semantics**, per field: `Bound` unset reads *uses the daemon default*, `TTL` unset reads *no queue-imposed expiry — a message may still set its own*, `Full` is written at registration and always concrete ([dictionary](data-dictionary.md#queue)). Never a resolved number the record does not carry |
| Owner's decision separated from name-inactive | needs the daemon to stop merging them in `visible` | **write the attention item to the weaker claim** (delivery is off and work is held) and do not promise the backlog is unreachable (the Overview attention set, above) |
| Whether a record is removable | `Reading` counts only unfiltered waiters (bus.go:425–429); `UnregisterAnd` refuses on any waiter (unregister.go:78). A filtered waiter is invisible to the face | **show the observations and let the daemon decide at submission.** Never present an observed-empty queue as proof that removal will succeed |
| A machine-readable refusal reason | `fail` serialises only `{"error": text}`; the counter `kind` is not sent and is too coarse anyway | **one body per status code**, with the daemon's message verbatim, and no recovery claimed that the code alone cannot support ([Problem](#problem--recovery-by-what-the-face-actually-knows)) |
| Per-message delivery evidence | retained exchanges, scoped | **route the question there**, and where an individual dequeue is not recorded, say it cannot be established ([S09](review/codex.md#specification-review-round-one)) |
| Restart boundary on Activity | needs `Status.Up` to say **when** the process started; the history itself is simply absent across a restart | **state the available sampled interval** and mark everything before it `¿`. There is no cross-restart delta to repair and no window spanning two runs: `Activity` differences samples from the current process only |

Where a row is later taken up as daemon work it moves to the TODO with an ID;
until then the fallback is what ships, and it is honest rather than empty.

---

## Narrow screens

Columns are chosen deliberately per list rather than letting a desktop table
scroll sideways ([C15](review/codex.md#junk-and-misleading-content)). Identity
and the judgment column survive at every width; queue observations may collapse
into the row. Codex's capture showed no whole-page horizontal overflow, so that
is not claimed as a defect — the finding is that the narrow layout was not
designed, not that it is broken.
