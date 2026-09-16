# Pages

Draft for peer review. Fifteen screens. Every field codex
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

Every page has: one `h1` that names it, a stated observation time where it shows
observations, a toolbar where it lists things, and the four states
([components](components.md#states)) — populated, empty, denied, unavailable.
None of those is optional, and "unavailable" never renders as "empty".

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
| Find | Prominent entry to Services and Channels | new |

**Attention items are enumerated, not judged.** The complete admitted set, each
with the observation it rests on:

| Item | Basis | Level |
|---|---|---|
| The last stop was not clean | `Status.Unclean` | red, stated as a fact with no lifecycle. Shown **only when true**: the field is `omitempty`, so absent means a clean stop *or* no previous stop and the face cannot tell them apart. Never print "clean shutdown" as an observation |
| Queue at capacity when observed | `AtBound` | red. **A condition, not a prediction.** Even "the next send is refused or drops the oldest" overstates it: the observation path does not prune, while the send path prunes first, can hand a waiting reader the message directly, and races a concurrent consume ([glyphs](glyphs.md#what-an-observation-is-worth)). Name the configured overflow policy beside it as a setting |
| ~~Backlog older than its own TTL~~ | — | **cut.** The record's TTL is the wrong right-hand side: the deadline is per envelope, fixed at accept from whichever of the sender's and the queue's TTLs was shorter, so a live head accepted under a longer setting can exceed today's without anything being wrong ([glyphs](glyphs.md#attention-levels)). home-parf's find |
| Losses, cumulative across restarts | `Dropped`, `Expired` non-zero | orange, links to Diagnostics. **Not** "since start": `Restore` puts these counters back from the snapshot ([dictionary](data-dictionary.md#state)), so a non-zero total may predate this run entirely |
| Refusals, cumulative | `Status.Refused`, a `map[string]int` by reason | informational; **not** "climbing" — this value is a lifetime total. A supported reason absent from it is a measured `0`, not an unknown ([dictionary](data-dictionary.md#node-and-scope)). A windowed signal exists on Activity but is scoped: node-wide refusals only on the unfiltered admin or master view, a sum over visible records otherwise |
| **Disabled record holding queued work** | the **stored** `Disabled` bit and `Queued` > 0 — see the caveat below | orange. opencode's find, and stronger than first stated. `Send` refuses at bus.go:532, `recheckInbox` releases existing waiters with `ErrDisabled` (manage.go:292), and `ConsumeAs` refuses a new reader at bus.go:801 — so the backlog can be neither delivered nor drained. home-parf's extension, verified: both prune sites sit *behind* those guards, so **the queue is frozen**. Nothing enters, nothing leaves, and nothing expires out of it: `Expired` will not grow even when every message is long past its TTL, and `Oldest` grows without bound. The only exits are re-enable and unregister. The page says the backlog cannot drain by any route |
| **Services of a suspended owner** | the owner's `State` is paused or banned | orange, linking to the person. **Conditional on [H.5.7](../TODO.md#objective)**, which is pending: today such services do not refuse, so the item would describe a rule that is not in force. It goes in when H.5.7 does |
| Unregistered credentials awaiting review | the cleanup cohort is non-empty | blue, a count and a link. **Owner-decidable**: it is discoverability rather than attention, and [C10](review/codex.md#junk-and-misleading-content) left it homeless |

A backlog with no reader is **not** an item. A queue worker between pulls is
exactly that shape and nothing is wrong.

**The `Disabled` we are given is not the `Disabled` that freezes a queue**, and
this item is where that matters. `visible` (manage.go:305) returns
`r.Disabled || !b.active(r.Name)` — one bit merging two causes — so the face
cannot tell an owner's decision from a name that has stopped being active. codex
raised this as a truth defect ([S04](review/codex.md#specification-review-round-one));
it is also an operational one, because **the two causes do not behave alike**:

| Cause | Arrivals | Drainage |
|---|---|---|
| Stored `Disabled` | refused | refused — `ConsumeAs` checks this bit, so the queue is frozen |
| Name not active | refused with `ErrInactive` | **still possible** — `ConsumeAs` guards the caller's standing and the stored bit, never `b.active(name)` |

So an item that reads "disabled and holding work" is urgent in the first case
and merely blocked-at-the-front-door in the second, and today's answer cannot
say which. **Until the daemon separates them the item is written to the weaker
claim** — *delivery is off and work is held* — and does not promise the backlog
is unreachable. Separating them is a daemon change and is named as
[owed](#owed-by-this-specification), not assumed.

Two structural rules, both opencode's:

| | |
|---|---|
| **One queue, one item** | A disabled record holding work is usually also at its bound, and a lossy queue is often both. Each record yields a single item at its worst level, or the page repeats itself. (The original example was at-bound implying backlog-past-TTL; that signal is cut, and dedup never needed it — codex) |
| **Never counted twice** | Losses and refusals appear as attention items and in Diagnostics. They do **not** also appear in the node strip, or the strip and the items disagree with each other on the same page |

**Empty state:** *"No observed attention conditions in this view, as of
14:22"*, with the scope stated. **Not** *"nothing needs attention"*, which was
the draft's wording and is unbounded — codex's
[S03](review/codex.md#specification-review-round-one). The admitted set is a
handful of conditions the daemon reports; a reader whose process died leaves a
queue that trips none of them. Absence of an item is absence of *these*
observations, not assurance. Never "healthy", for the same reason one size
larger.

**Queued work keeps a direct route**, which the draft lost when the backlog
table became an exception list — codex's
[S03](review/codex.md#specification-review-round-one). Services and Channels
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
| the whole registry table | Services and Channels — **only once those carry kind, description and accepted/dequeued**, which they do not today ([C08](review/codex.md#junk-and-misleading-content)) |
| loss by name | Diagnostics, and the record's own Queue section |
| my names, fingerprints, rotation help | Account |
| three trailing explanatory paragraphs | beside the control each explains |

---

## Services `/services` · Channels `/channels`

**Answers:** what can I use, who owns it, and is it in trouble?

Two destinations, **one component family**. A channel's first question is how it
delivers and who subscribes; a service's is whether anything is reading and what
is queued. One table answering both is what buried delivery mode
([C03](review/codex.md#junk-and-misleading-content)). Each page renders its own
heading and marks its own navigation entry. **Precisely:** the `h1` is already
correct — it switches on `.Channels` — but both routes share
`shell("services", "Registered services")`, so Channels carries the Services
document title and highlights the Services nav entry (admin.go:324). The
draft's "renders under the Services heading" overstated it; the title and nav
defect is real. codex's [S14](review/codex.md#specification-review-round-one).

| Column | Decision |
|---|---|
| Description, then full routing name beneath | **new + keep.** Identification is by address only today; the description exists on records and is how a session is recognised ([C01](review/codex.md#junk-and-misleading-content)). The full name stays because it disambiguates sessions — demote by task, never drop |
| Kind | **filter only, not a column.** The session-recognition journey uses it as a filter — Services, then agents — and a per-row category beside the description and full name is redundant with the control that got you there |
| Delivery mode (channel list) | **new.** The channel question |
| Owner | keep |
| Enabled / Disabled | **relabel** from Active/Inactive. Administrative state, not liveness. Disabled is a decision, not a failure |
| Reader: *attached* / *no reader waiting* / *external* | **relabel** from Serving/Offline/External. `Proto` is a caller-supplied hint meaning "expect no local reader", not proof of anything |
| Queued | keep on the service list. **Mode-aware on the channel list**: a pub/sub topic keeps no queue of its own — `Send` hands it to `fanout` and nothing waits on the topic — so a Queued cell there is structurally zero. Pub/sub rows show accepted; queue rows show queued; or the cell reads `—` |
| Subscribers (channel list, pub/sub) | **new** |
| One judgment column | **new.** Lit only on exceptional rows ([glyphs](glyphs.md#where-a-glyph-is-allowed)) |
| Updated `At` | **demote** to detail. It consumes a column and answers a question nobody on a list is asking |
| `Controls: Manage / View` | **drop.** Inert text shaped like a control ([C01](review/codex.md#junk-and-misleading-content)) |
| `ConfigSHA` | **move** to the configuration section of the detail page |
| `In` / `Out` | **move** to detail, **relabelled** accepted / dequeued. Dequeued is not completed |

**Toolbar:** search over description and name; scope (all / mine); state; kind
or mode; sort. All in the URL as GET parameters, all retained through paging and
through a visit to a detail page and back. Result count and active filters are
shown, with one action to clear them.

**Ordering is stable and named.** Core's `List` iterates a map, so order out of
the daemon is not stable — but **the services and channels list already sorts by
name** (admin.go:189), and the draft was wrong to call it an unstable load.
[W16](../done/web-review.md#findings) survives where nothing re-sorts: the
diagnostics registry. codex's [S14](review/codex.md#specification-review-round-one).
What this specification adds is a *chosen* order with the sort in the URL,
rather than an incidental one.

**Empty states are three different pages**, and this is the split's price:

| | |
|---|---|
| No records at all | what a service is, and the link to register one |
| No channels at all | **what a channel is and how one is created** — without this the Services/Channels split reads as a bug |
| No matches | the active filters, and one action to clear them |

---

## Service `/service?name=` · Channel `/channel?name=`

**Answers:** what is this, is it working, who may use it — and then, separately,
what may I change?

Today this is an operational summary followed immediately by a configuration
digest and an always-open wall of six editors
([C02](review/codex.md#junk-and-misleading-content)). It becomes: read first,
then focused edits.

| Section | Content | Visible to |
|---|---|---|
| Identity | Description, full name, **address**, **protocol**, kind or delivery mode, owner, maintainers, enabled state, **updated time** | anyone who may see the record |
| Queue | Queued, oldest, accepted, dequeued, dropped, expired, at-bound, **overflow policy**, TTL and capacity with inheritance stated per field | same |
| Access | Allow list, master-refusal, and what that means in a sentence | same |
| Subscribers (pub/sub only) | Each subscriber, linked where the caller may inspect it | same |
| Configuration | Whether one is set, and its digest as evidence. **Never its contents** | same |

**The read sections do not depend on edit permission.** Address, protocol, ACL
and queue policy are already in the daemon's answer to this caller and today are
rendered only inside the editor
([C04](review/codex.md#junk-and-misleading-content)). Controls are conditional;
returned metadata is not.

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
| Configuration | replace configuration |
| Subscribers | subscribe/unsubscribe, remove a subscriber |

Each is collapsed until asked for, so no page ever renders eight open editors
and the count stops being a usability variable. **Transfer is a link** opening
its confirmation flow rather than a permanently rendered form; **remove** sits
at the end of the page with its own confirmation.

Each returns **to the section it changed** with a specific result — not to
`/services?scope=my`, which is where every service and channel action lands
today, so a channel subscription answers by leaving the channel
([C06](review/codex.md#junk-and-misleading-content)).

Transfer and maintainers assignment are **owner-only**, and that is the
authority contract rather than an editor defect: core restricts both to
`r.Owner` deliberately. A maintainer who may manage a record still may not give
it away or change who maintains it.

---

## Register `/services/new` · `/channels/new`

**Answers:** how do I create one?

A dedicated page, not a form stapled beneath a list. Fields: name, description,
kind or delivery mode, initial allow list. Help beside the name field states the
`user@realm` shape; help beside allow states that empty means every
authenticated caller and that owners and maintainers keep access regardless.

Invalid input returns **this form**, with the values preserved and an error
summary. Today an invalid name renders raw JSON and the description the person
typed is gone ([C13](review/codex.md#junk-and-misleading-content)).

---

## Activity `/activity`

**Answers:** what traffic was observed, over what window?

| Decision | |
|---|---|
| Shared time range across all series | five independently scaled graphs today ([C07](review/codex.md#junk-and-misleading-content)) |
| Labelled axes, in real timestamps | evenly spaced sample indices today ([W09](../done/web-review.md#findings)) |
| Stated units, interval, restart boundary and partial bucket | |
| Zero series summarised, not given equal height | a zero series uses as much space as real traffic today |
| `Maximum: 0` repeated down the page | **drop** |
| Value table beside the graphs | **keep** — it is the accessible path to the numbers |
| Absence vocabulary in the table | `0` measured zero, `¿` not observed. Before the last restart is `¿`, not `0` |

Arriving from a service or channel keeps that filter. The scope selector says
what it is scoped to.

**Four constraints the series carries**, found by home-parf and verified. They
are not presentation choices; three of them change what the graph may be
labelled, and the third is a defect rather than a limit:

| | |
|---|---|
| **The window is one hour, hard** | 61 minute-samples. Any claim about a trend is a claim about at most sixty minutes and must say so. Nothing longer is answerable from this data |
| **A restart renders as a quiet minute** | counters are in-memory and reset on restart, and `max(0, …)` clamps the resulting negative delta to `0` — so the discontinuity becomes a zero bucket rather than a gap. That is the zero-versus-absent defect one layer *below* presentation. The face can only locate the boundary by cross-referencing `Status.Up`, and without that cross-reference **the restart boundary this page promises cannot be drawn at all** |
| **The scope switches inside one series** | `total()` sums `Refused` over records the caller may see, but for an unfiltered admin or master view node-wide `s.refused` **replaces** that sum (activity.go:98–99). One series, two meanings, chosen by who is looking. A single static label is wrong for one of the two audiences — and it is invisible when an admin tests it. The label is computed from the view, never fixed in the template |
| **History is computed against the current record set** | `total()` iterates `b.records`, so a record removed mid-window leaves both `prev` and `next` and its traffic vanishes from the series retroactively. The page says the series covers records visible **now** |

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

| Field | Decision |
|---|---|
| `PeopleCount` / `OtherCount` | **keep**, with scope labelled: caller-visible directory, before the search |
| Search, kind filter, paging, `Matched`, clear-filters | keep |
| Person name, full name, authority, state | keep |
| Empty cleanup table plus its explanation | **collapse** to a count and a link while empty; expand only when candidates exist ([C10](review/codex.md#junk-and-misleading-content)) |
| Classification of an unclassified identity | **keep as a word.** Never a colour, never inferred from a slash or a runtime prefix in a name |
| Avatar | `/avatar` is reachable and authenticated but no template references it ([inventory](review/current-state.md#routes-and-templates)). **Decide deliberately**: use it here, or remove the endpoint. It must not fetch the whole directory per image ([W07](../done/web-review.md#findings)) |
| **Create user** | **entry point, which the draft lost** — codex's [S07](review/codex.md#specification-review-round-one). The form exists in [forms](forms.md#the-set) with no page offering it. A primary action beside the directory heading, **shown only where the caller may create**, opening `/users/new`. Invalid input returns that form with its values and a field-level error, never a problem page. Success lands on the new user's page. This is the only onboarding path for a fresh install, which is what makes its absence a gap rather than an omission ([H.5.9](../TODO.md#objective)) |

## User `/user?name=`

| Section | Content |
|---|---|
| Identity | Name, person name, state, daemon authority |
| Memberships | Groups, or an explicit no-memberships statement |
| Owned records | Linked, or an explicit none |
| Profile | Fields when `CanEdit`; otherwise the values, labelled, plus who may change them |
| Lifecycle | Separate from the summary. **Only the transitions that apply** — an already-active user is offered Activate beside Pause and Ban today, all looking alike ([C12](review/codex.md#junk-and-misleading-content)). Ban is consequential and confirmed |

There is no delete-user control and there will not be one: a user is never
deleted, only made inactive
([user lifecycle](../../../docs/01-identity.md#user-lifecycle)).

For a non-user identity: the kind, why it is retained, and the removal control
only when `CanRemove`, with consequences stated before the form.

---

## Groups `/groups` · Group `/group?name=`

**Answers:** what groups exist, who is in them, and what uses them?

| Field | Decision |
|---|---|
| Group name | keep |
| Members | **fix.** Membership renders only inside the editor today, so an ordinary caller sees a heading with no members and no explanation ([C05](review/codex.md#junk-and-misleading-content)). Say *not visible to you* — never an empty array shown as a zero count |
| Records referencing the group | **new.** This is [W08](../done/web-review.md#findings): there is nowhere today to see which records a membership change will affect. The draft motivated it as *why removal is refused*, two rows above prohibiting deletion — codex's [S14](review/codex.md#specification-review-round-one). The real use is live: editing members changes who may manage every record listed here, and that blast radius is otherwise invisible |
| Protected state of `@maintainers` | **keep**, stated as protection, with no delete control offered |
| Delete | **no control.** The handler accepts it and nothing renders it; removing the verb is [H.5.6](../TODO.md#objective), not a UI feature to restore |

The detail page earns its place on the references alone — they are what makes a
membership edit a reviewable decision rather than a blind one.

---

## Account `/account`

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

## Sign in `/signin`

Token, and how to get one. The form takes a token and nothing else — there is no
name to type, so there is no second failure message to read as an oracle for
which names exist.

Acquisition help is concise and in the supported order. It must not offer an
onboarding journey the daemon cannot complete: `agent-bus-admin user add` does
not currently create the user it adds
([H.5.9](../TODO.md#objective), [C14](review/codex.md#junk-and-misleading-content)),
so the SSH path is described with that limit rather than promised.

A refusal says the token was not accepted and nothing about which names exist.
The return destination is validated as one of this dashboard's own pages before
it reaches the form.

---

## Problem — recovery by reason, not one page

Today one template carries every failure ([W12](../done/web-review.md#findings)).
**The draft's replacement was worse than the build in one respect**, which
codex's [S10](review/codex.md#specification-review-round-one) caught: `fail`
(admin.go:56–93) already separates 401, 403, 404, 500 and 503, and the four
states I listed collapsed the last two back together. The defect that remains is
its `default` branch, where 400, 409, 412 and 429 arrive as one *"That was
refused"* — four different situations with four different next actions.

The rule: **a state exists where the recovery differs**, and it is keyed to the
daemon's reason, not to a code alone. The [refusal
vocabulary](../../../docs/05-discovery.md#refusals) is a closed counted set, so
this table can be complete rather than open-ended.

| Reason | Code | Body | Next action |
|---|---|---|---|
| Not found, or hidden from you | 404 | identical presentation for both — hiding and refusing must not be distinguishable | back to the list |
| Credential | 401 | the session ended | sign in, returning here. Already the build's behaviour, and correct |
| ACL, suspended, enrolment | 403 | **three different bodies.** Lacking permission is not being suspended, and neither is not being enrolled — giving a service permission cures exactly one of them | who can grant it / who can lift the state / how to enrol |
| Name taken | 412 | the name exists | choose another, with the form and its values preserved |
| Malformed | 400 | what was wrong with the input | **the form, with values preserved** — not a problem page at all. A problem page here is the defect ([C13](review/codex.md#junk-and-misleading-content)) |
| Disabled, busy, second-reader | 409 | **three different bodies.** A disabled record is a decision, a busy one is transient, a second reader is a contract | enable it / retry / who holds the read |
| Full | 429 | the receiver is at capacity | its queue, and the overflow policy in force |
| Daemon fault | 500 | a fault, not a rule | repeating is unlikely to help; the node's log. Keep the build's wording |
| Daemon unavailable | 503 | it is there and briefly cannot answer | retry. **Never** an empty healthy list |
| Transport failed | — | the request did not complete | retry, and **never** "nothing was changed", which is not knowable |
| **Conditions changed** | any | what you confirmed stopped being true before it ran | show what is true now, and offer the action again if it still applies ([forms](forms.md#when-the-recheck-refuses-after-you-confirmed)). Not the generic refusal, where it reads as a bug |

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
| Per-message delivery evidence | retained exchanges, scoped | **route the question there**, and where an individual dequeue is not recorded, say it cannot be established ([S09](review/codex.md#specification-review-round-one)) |
| Restart boundary on Activity | needs `Status.Up` cross-referenced, since a restart clamps to a zero bucket | **cross-reference it**, or the boundary is not drawn and the graph says its window may span one |

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
