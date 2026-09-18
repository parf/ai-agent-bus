# Discovery

📌 **TL;DR:** Discover accessible records and inspect what the daemon actually knows.

## Status

| MVP | Scope |
|---|---|
| Built | Filtered listings and catalog, all [required dashboard tabs](#required-tabs), administration, envelope-only diagnostics, [reader counts](#readers), [web authority isolation](11-processes.md#web-authority-boundary) and resource limits. |
| Pending | Post-redesign [browser acceptance](#browser-acceptance); the installed session/restart foundation and current five-role authority matrix are built. |

## What a listing answers

**Being in the registry and being callable are different facts.** "There is a
MySQL on `db1:3306`" is a complete registration ([services § service
kinds](03-services-and-topics.md#service-kinds)) and nothing on this bus
answers for it; a service template is registered and deliberately does not run
([services § service and template](03-services-and-topics.md#service-and-template)).
So a caller reading a listing needs more than a name:

| Field | Says | Absent means |
|---|---|---|
| **`protocol`** | how to call it, when that is not through the bus ([services § how to call it](03-services-and-topics.md#how-to-call-it)) | an ordinary bus service: send to the name |
| **`readers`** | how many consume requests are outstanding now, filtered and unfiltered together | unavailable; a current daemon publishes measured zero explicitly |
| **`reading`** | compatibility-only flag: an unfiltered read is outstanding now. Human faces render `readers`; new consumers should use it | no unfiltered read observed; this does not mean nobody is attached |
| **`queued`** | how many messages are waiting in it | none are |
| **`in`** · **`out`** | how many messages have arrived for it, and how many a reader has taken, since the daemon started | none have |
| **`dropped`** · **`expired`** | what its queue lost to overflow, and what outlived its TTL in it, since then ([messaging § overflow](04-messaging.md#overflow)) | it has lost nothing |
| **`oldest`** | how long the message at the head of its queue has been waiting | its queue is empty |
| **`at_bound`** | its queue held the limit it is allowed **when the question was asked**. Not a prediction about the next message: a waiting reader is handed one without it ever queueing, and enqueueing prunes what has expired before it tests fullness, so if the queue is still full then the record's [overflow policy](04-messaging.md#overflow) applies — refuse, or forget the oldest | there is room. The daemon answers it because a record that declares no bound takes the daemon's, and a reader cannot know what that is |

These are **observations attached to the listing**, not values a registrant
may state. Traffic and loss counters survive through snapshots; `readers` and
`reading` do not ([overview § principles](00-overview.md#principles)). They
describe the observed inbox, not service health or a guarantee that a
particular request will be handled.

An inbox that was drained and one nobody ever wrote to both read as empty.
`in` and `out` are what tell them apart, and they are per name: a busy bus
does not make a quiet service look busy.

## Readers

**Built in 0.5.53 (Q70).** Every human face shows one **Readers** count per
visible inbox: all currently outstanding consume requests, filtered and
unfiltered together. No separate counters or breakdown by filter type.

This helps an operator see whether anything is waiting to read. Zero means no
read is outstanding at that instant, not that the service is dead; a reader may
be processing a message between reads. A positive count does not promise that
a particular message matches, or that any work has finished. Count waiting
requests, not processes, sessions or completed reads.

The compatibility `reading` flag cannot provide this count and is not rendered
by WEB, CLI or MCP. Normal service behavior follows the [full-inbox reading
rule](04-messaging.md#one-reader-per-inbox).

## CLI listing

**Built:** `agent-bus ls -h` renders a table; plain `ls` retains JSON.
Both accept a single name or `--kind` filtering through the same API calls.
The table shows name, kind, owner, Readers count, queued count and description.
A current daemon reports a numeric Readers value for every row, including a
record with an external protocol; a missing older answer renders as
`unavailable`. Protocol and inbox observation are separate facts. It is the
[listing observation](#what-a-listing-answers), not a health check.
An empty result says `No matching records.`; lookup errors remain errors.

## Faces

| Face | Built | Pending MVP |
|---|---|---|
| API | Registry, messaging, credentials, sessions and dashboard administration | — |
| MCP | Bus tools and a catalog filtered by the daemon | — |
| WEB | [Required tabs and controls](#required-tabs), filtered through the caller's API access; confined with resource limits | Installed browser acceptance |

## MCP minimum

**Required for MVP:** the [runtime integrations](08-runner-role.md#runtime-integration-delivery)
and every shipped [launcher](08-runner-role.md#smart-launchers) come with the bus MCP
tools configured and callable in the session.

| Capability | Required outcome | Existing implementation |
|---|---|---|
| List the bus | Discover registered agents, services and topics through the caller's [catalogue view](#audience) | `ab_ls` in the [MCP face](../src/mcp/server.ts) |
| Call a service on the bus | Send to a named service and receive its correlated answer or explicit completion under the [request/reply contract](04-messaging.md#request-and-reply); bus acceptance alone is not completion | `ab_send` plus a filtered `ab_consume`, or delivery through the active push adapter; [MCP face](../src/mcp/server.ts) |

These are minimum capabilities, not a restriction on the remaining tools.
The tools and launcher wiring are built; full live-runtime and fresh-host
acceptance remains pending. Both capabilities use the caller's existing
[ACL](02-access.md#acl); loading the tools grants no additional authority.

## Audience

The daemon filters, because it holds the records and enforces the
[ACL](02-access.md#acl). A face requests the caller's view and adds no
independent access policy. There are no chained catalogs or group expressions
in the MVP.

## Dashboard

The WEB child presents the bus and forwards administration **as the person
looking**. The daemon owns state and authorization: every page and form uses
the visitor's authority, and the face has no independent privileges
([audience](#audience)). The [required tabs](#required-tabs) use the same daemon authorization as direct API calls.

It shows the envelope and nothing else
([messaging § envelope](04-messaging.md#envelope)): no message bodies, here or
anywhere.

Bodies are struck out **in the bus**, where the feed is written — so no reader
has to be trusted to leave them alone. The feed and the records beside it are
both filtered per caller: you see the exchanges you were **party to**, sent or
addressed to you, and the daemon Owner sees the node's through node-wide
authority.

**The node's own counters are the node's, not the caller's.** Uptime, the
registry's totals and the [refusal counts](#refusals) describe the daemon, and
anybody who may ask sees the same numbers — there is no narrower audience for
them. Somebody with no access sees nothing at all, which is the whole of the
restriction. A page showing them beside a caller-filtered list must say which
is which: *the node refused 40 calls* and *you were refused 2* are both true,
and neither is the other.

The diagnostics page carries the built rows of [what it shows](#what-it-shows);
service/channel, user and group administration and activity graphs have separate pages. The ordering, the grouping and the late mark
are the page's; everything else on it is an answer the bus gave that caller,
so a view cannot show more than the caller may ask for.

### Required tabs

**Required MVP, built.** Owner confirmation of scope, 2026-09-13.
The daemon owner has the administrative view; other visitors see and change only
what the daemon permits. “All” means all visible to that visitor.

| Tab | Required functionality |
|---|---|
| Registered services | My / all; active / inactive filters; details and [owner controls](01-identity-and-roles.md#services), with owner, Maintainers list, access, Readers count and queue statistics. Excludes Personal services, which have their own tab. Administrative availability and reader observation are distinct facts |
| Personal services | Owner-tagged services grouped separately without changing access. Ordinary visitors see their own; the daemon owner may filter by owner across the node-wide management view |
| Users | List and details; add, edit, activate, pause and ban; show owned services, group membership and administrative authority |
| Groups | List and details; create, edit and manage direct entries, including nested ordinary groups; basic service and channel access. Include the daemon Administrator group and groups named by records' Maintainers lists under the [authority rules](01-identity-and-roles.md#groups); retire groups by emptying them, with no delete control |
| Activity graphs | Recent traffic, messages dequeued, drops, expirations and refusals; per-service and per-channel filtering. Dequeued messages are not proof of successful execution. Use bounded history and inline SVG; [sampling and retention](#activity-history) are bounded |
| Registered pub/sub channels | List and details for pub/sub and queue topics; create, edit and remove; subscriptions, owner, Maintainers list, permissions, TTL, capacity and overflow policy |

The [Personal Services view](03-services-and-topics.md#personal-and-shared) is built.

### Section navigation and registration

**Built in 0.5.64.** Services shows caller-visible **All**, **My** and
**Personal** category counts before the delivery filter; Channels shows its
caller-visible total. Users and Groups show their visible directory totals.
These are counts computed from the page's existing daemon answers, not
node-wide metrics and not additional reads.

### Registry filters and paging

**Built in 0.5.77.** Services, Personal and Channels filter delivery and live
reader observations independently. Reader choices distinguish a positive
count, measured zero and an unavailable observation; none is a health claim.
Search, kind, owner and sort remain URL state beside those filters.

The web face filters and sorts one caller-visible `/ls` answer, then shows at
most 25 rows. It reports the matching count, bounds invalid page numbers and
retains the exact page and filters through record detail and back. Section
counts remain category totals before toolbar filters, rather than page counts.

Every numeric column in a web table aligns its header and values to the right
and uses tabular figures. Prose-embedded counts remain part of their sentence.

### Service and channel journeys

**Built in 0.5.78.** Services and Channels share the compact registry frame but
answer different questions. Services identify a daemon-stated Service or Agent
and show queued inbox work. Channels have their own document title, heading,
canonical `/channel` detail link and delivery-mode filter.

Channel rows state **Queue · one at a time** or **Pub/sub · copy to each**. The
single **Work** column is mode-aware: a queue reports messages held for a
reader; pub/sub reports messages accepted for fan-out and never suggests a
topic backlog. Pub/sub also reports its subscriber count; queue subscriber
count is not applicable. Readers remains the live count of outstanding consume
requests and is independent of mode.

Search, delivery, mode, Readers, sort and page remain plain URL state through
channel detail and back. A successful channel registration or ordinary edit
returns to that channel. Caller-visible operational facts remain readable while
edit controls appear only when the daemon grants management authority. An empty
Channels category explains channels and offers registration; a filtered empty
result instead keeps its filters and offers to clear them.

Registration opens dedicated `/services/new`, `/channels/new`, `/users/new`
and `/groups/new` pages from the matching section navigation. User and Group
entries appear only when the daemon says the visitor is an Administrator;
the routes repeat that authority check. The old empty `/user` registration URL
continues to work.

Two- and three-value URL filters are visible links whose active state and plain
values remain in the URL. Two-value creation choices are labelled radio
buttons. An owned Service or Channel row uses a blue leading rule and blue
semibold linked name; the **My** category link uses the same blue. The row does
not repeat a **Yours** label. A Personal row keeps the visible **Personal** word
and uses stronger orange, bold emphasis shared by the **Personal** category
link. Orange overrides blue when both facts apply. Ownership comes only from
the returned Owner, and Personal only from the returned tag; the protocol hint
changes neither. The compact row treatment began in 0.5.66 and its labels were
refined without a version bump on 2026-09-17.

The linked name is the single route to read-first detail and any controls the
daemon authorizes there; the former duplicate Edit column is gone. List update
times read `now`, whole minutes, hours or days while under 30 days old, then
`Jan 1` within the current year or `Jan 12, 2025` across years. Detail retains
the full registration-update timestamp. Missing time remains unavailable.

### Page titles and compact help

**Built in 0.5.65.** Every page title starts with one decorative image or
glyph and keeps its visible text. Static pages use their section category;
record detail uses the daemon-stated kind, with the Service section mark as the
fallback. The marks are fixed inline markup, never an external asset, caller
text or a machine-readable value.

The Services, Channels, Personal and Users collections keep their definitions
behind a visible `ⓘ` button using the browser's native popover. Service detail
uses the same pattern for Delivery, Policy, Queue & counters and Activity: hovering or
focusing the adjacent button shows the explanation immediately, while clicking
opens the structured list. The button has an accessible name and the panel uses
a heading and short list. Current scope, counts, filters, form constraints,
refusals and dangerous consequences remain visible where they affect a decision.

[Administrative and record authority](01-identity-and-roles.md#groups) applies
to every control and to direct API calls. Membership and policy changes must
survive restart. User lifecycle effects are [daemon policy](01-identity-and-roles.md#user-states),
not merely labels on the Users page.

Keep the [built views](#what-it-shows), including inboxes holding messages, exchanges,
credential fingerprints, losses, refusals and node status, accessible in the
new navigation. Their existing functionality is not deferred by this split.
[Optional additions](../Plans/R1/discovery.md#dashboard-extensions) belong to R1.

### Compact administration pages

**Built in 0.5.73.** Service and Channel registration, User detail, Groups,
Diagnostics and record detail share the same cards, responsive field grids and
line-list textareas. Current facts, form labels, errors and actions stay visible.
Definitions and caveats that do not change the immediate decision use the
adjacent `ⓘ` control: hover or keyboard focus shows them immediately and click
opens the structured native popover.

Record detail presents Delivery, Policy and Queue & counters as one compact
fact row, followed by Activity. Authorized settings and Maintainers stay closed
until chosen or until a refused submission must reopen them; configuration,
transfer and removal remain in the red Danger Zone. User detail separates the
profile editor from identity, authority, groups, lifecycle and owned resources.
Groups use a compact Group/Members table. Selecting a name opens one group;
the full-width membership textarea appears only when the caller may edit it.
Diagnostics retains every existing table and scope boundary without the former
visible paragraph walls.

Human-facing integer counts use grouped decimal figures, including the compact
footer, registry, diagnostics and Activity totals. JSON, URLs, form values and
editable syntax remain unchanged plain values.

### Activity history

**Built in 0.5.67.** Service and channel detail embeds one compact,
record-scoped graph and links to the same filtered `/activity` view with its
sample table. Both views use actual sample timestamps and one scale across the
displayed nonzero series. Zero-only series are summarized; no retained sample
is described as collecting after restart rather than as zero. The displayed
window, current uptime and partial final sample are stated beside the graph.

The four delivery series on the unfiltered view cover records currently visible
to the caller. Refused is node-wide for the daemon Owner and covers visible
records for other callers. A named view is record-scoped for
all five series. Dequeued means handed to a reader, never completed work.

The bus samples record counters every ten minutes, independently of dashboard
visits. It keeps about 24 hours plus the baseline for differences; the dashboard
also includes the current partial interval, which may be shorter than ten minutes.
History is in memory and starts fresh after restart. Export remains
[R1](../Plans/R1/discovery.md#dashboard-extensions).

Graphs and their accessible value table report accepted, dequeued, dropped,
expired and refused counts. Aggregates include only currently visible records;
pub/sub copies count in the subscriber inboxes that accept them. Per-record
refusals count failed sends and reads; the daemon Owner also sees node refusal
totals, on the terms [refusals](#refusals) sets — every
endpoint refusal, including authentication failures and malformed requests, and
not the router's rejections or our own failures. Bodies never
enter this history.

Adding a subscription remains the subscriber's opt-in. Channel owners and
maintainers can remove a subscription; they cannot force another inbox to subscribe.

### Rules it is built to

| Rule | Why |
|---|---|
| **The anonymous page shows what the bus would answer a caller it cannot name — except for the facts the owner named.** A title, the sign-in form, how to get a token, and [what a node says about itself](#what-a-node-says-about-itself) | The default is still nothing, and the reasons hold: uptime is a restart oracle, a service count that moves is a covert channel anyone who can register writes to, and a traffic total is traffic analysis. The owner weighed each of those against a stranger being unable to tell what this node is or whose it is, and published a **closed list** anyway. Everything not on that list stays behind the gate, and the list grows only by an owner decision |
| **No page ever renders a credential** — a fingerprint of it, when it was issued, when it was last used, and the command that rotates it | A token on a page is in the browser cache, the scrollback and every screenshot, and leaves no trace that it was read, so "was this leaked?" stops being answerable. A fingerprint is enough to match the one in your environment |
| **One repository-owned script; no CDN or external asset** | The local script only submits marked selectors on change. It reads no page data, stores nothing and makes no request of its own. Pages remain ordinary URL-backed forms, with a `noscript` Apply control. Graphs stay inline SVG; avatars are served from this node, never hotlinked |
| **The web child writes nothing of its own.** A form posts *as the person*, never as the child | It is the least trusted process and the design gives it no write path ([processes § the processes](11-processes.md#the-processes)). Built administration forms forward the visitor's session to daemon-enforced operations and require an exact matching Origin. Responses are not cached; credentials and existing private configuration are never populated into forms |
| **The sign-in form takes a token and nothing else** | A call carries no name to get wrong ([access § what a call carries](02-access.md#what-a-call-carries)), so there is no second failure message for an anonymous visitor to read as an oracle for which names exist |

### What a node says about itself

**Owner-settled, 2026-09-16.** A short closed list of facts about the node is
published to **anybody who can reach the dashboard, signed in or not**: what it
is, where it runs, whose it is, how long it has been up and how many calls it
has served. They appear in the shell every page shares and on the **sign-in page**,
which is the whole point — somebody who arrives at a bus they do not have a
credential for should be able to tell what it is and whose it is without asking
anybody.

| | |
|---|---|
| what is published | release, host name, daemon owner name, uptime, **calls served** — and the build. **Nothing else**: no record names, no principals, no refusal counts, nothing about who is using it |
| where each one shows | **the header has a two-row bus mark** derived from the [project artwork](img/agent-bus.png) at the far left. Beside it, `AgentBus v<release> @ <host>` carries build detail in the Version tooltip; navigation sits below. The compact footer carries owner, uptime and calls served. No separate build line or web-build value appears |
| to whom | any caller that reaches the face, with no credential and no session |
| host name | the machine's hostname as the OS reports it, `srv1`. The daemon has no node name of its own, so this is a new field rather than a restatement of one; it is not the realm, which the owner's name already carries |
| uptime | a plain figure behind an explicit label, `uptime: 1h23m`, as the owner asked. It is what was true when the page rendered, and the page does not refresh itself |
| calls served | the count of **HTTP requests the bus process has served**, over the **last minute** and **the last hour**, plus the **total since the daemon started**. Every request on every listener, gated or refused or served — it is counted before the handler runs, so it is traffic reaching the daemon rather than work it agreed to do. A node-wide figure, not this caller's. **No host reading is published**: the owner asked for the daemon's own calls only, and an OS load average is a fact about the machine rather than about this node |
| what it costs | an unauthenticated visitor learns the host's name, who runs this node, how long it has been running and how much traffic it carries. Each was put to the owner and accepted. The call counts are the most revealing of these and were accepted explicitly: a total that moves is traffic analysis, and the answer is that it is a total — it names no record, no principal, no endpoint and no direction of business |
| how it is read | **`GET /identity`**, a public daemon call answering these fields and nothing else to a caller with no credential. There was no such call: every route but enrolment and a root redirect sits behind the token gate, and `GET /status` is authenticated and answers refusals and the caller's own standing besides. So this is a new endpoint rather than a relaxation of `/status`, which keeps its gate and its contents |
| the face's part | it asks as anybody does. The face still holds no credential and still acts as the visitor for everything else ([web authority boundary](11-processes.md#web-authority-boundary)): this is a fact the daemon publishes, not a privileged call the face makes |

The total needs no sampler at all: it is the counter itself. The minute and
hour windows come from a **separate 61-sample history of that counter** — plain
readings, no records and no names in it — driven by the **existing minute
ticker**, so there is no additional timer. It is not the per-record activity
the dashboard graphs; those samples stay what they were and are counted
per record.

**Why a total rather than a day.** A day window was asked for and withdrawn.
The counter is process-local and resets with the daemon, so on any node
restarted within 24 hours `day:` would report everything since start while
calling itself a day. `total:` reports the same number and is honest about it —
and it needs no span of its own, because `uptime:` is on the same line and
scopes it.

**This figure is nobody's listing.** It is a process-local counter, incremented
where every HTTP request enters the bus process, with no caller, no record and
no ACL anywhere in it. That is what lets it be published to a stranger at all:
there is no per-caller view to get wrong, because there is no per-caller view.
The dashboard's activity series is the other thing entirely — filtered to what
one named caller may see, and unable to answer this question for a caller it
cannot name, since for a stranger it would report a confident **zero** on a
busy node.

**What the counter can and cannot answer.** These are properties of what is
being counted, not defects to be fixed here. They are documented here; the
compact dashboard does not explain them:

| | |
|---|---|
| it counts requests **admitted**, not work completed | the counter increments before the handler runs, so a refusal, a router 404, a bad token and a served call all count the same. A node being hammered with rejected requests reads as busy, which is correct — it is traffic reaching the daemon, not work the daemon agreed to do |
| a long poll counts when it **starts** | a waiting `consume` holds one request open for as long as it waits, and it was counted on arrival. On a bus whose faces sit in long polls, a quiet minute in which several readers attach still shows calls. Nothing is wrong with the figure; it is answering a different question than "how much happened" |
| the dashboard counts itself | `GET /identity` is a request like any other, so loading a page adds to the figures that page then shows. No page refreshes itself ([what it shows](#what-it-shows)), so an open page left alone generates nothing; it is a **person** reloading who moves the number they are watching |
| the windows are **sampled**, and the header does not say by how much | a window begins at the newest reading at or before its cutoff, so its span is **whatever the readings allow**, not what its name says. With the usual minute cadence and enough history, `minute:` covers roughly one to two minutes and `hour:` a little over the hour; a delayed tick widens it, a young node shortens it below the nominal span entirely, and it matches exactly when the cutoff falls on a retained reading. None of those is the guaranteed case, which is why the daemon measures each span rather than reasoning about it. **The header prints the label alone**, by owner decision at 0.5.40, taken with this stated: `uptime:` beside it shows how young the node is, and that was judged enough for a glance. The span is still measured and still published — `observed` on each window of `GET /identity` — but **no page says so**: the answer lives on the API, not in the dashboard |
| history is **shorter than the window** after a restart | the counter starts at zero with the process, and an hour of readings takes an hour to accumulate. **A window's span is its own, not the node's age**: on a node up three minutes, `minute:` still finds a baseline near its own cutoff and covers about a minute, while `hour:` covers the three minutes it has. `observed` carries this on the wire; the header does not. **Unobserved history is never shown as zero** — that is the distinction this whole layer exists for |
| `total:` is **exact**; the windows are not | the total is the counter read directly, with no sampling in it. Only `minute:` and `hour:` are differences between a reading and now, and only they carry an `observed` span on the wire |
| the three may legitimately be **equal** | during the first minute, and whenever every call the node has served falls inside the shortest observed window. Nothing is wrong and nothing should be built assuming they differ — but nor do they collapse merely because the daemon is young: at ten minutes, `minute:` covers the last one while `hour:` and `total:` cover all ten |
| all three reset on restart | the counter lives with the process. A restart is not a quiet node, and `uptime:` on the same line is what tells them apart |

### Signing in

A person signs in with **the token they already hold**
([access § what a call carries](02-access.md#what-a-call-carries)) — there is no
name to type, no other kind of credential and no password anywhere.

The part worth stating is where the session lives: **in the bus**, which is
the process that holds state ([processes § what is shared](11-processes.md#what-is-shared)).
The child forwards it once, the bus answers with an expiring session id,
and from then on the browser carries that id and nothing else.

| | |
|---|---|
| the cookie | the session id alone — `HttpOnly`, `Secure`, `SameSite=Strict`, idle timeout. Never the token, never in a URL |
| a web-child restart | logs nobody out, because the child was holding nothing. A session map inside it would be a second store, of the worst possible contents: every signed-in person's live credential in the one child that is restarted with backoff |
| what the child holds | nothing. It stops reaching the bus over the owner's socket the moment people sign in — a web child with the owner's authority is a credential mint ([access § getting a token](02-access.md#getting-a-token)) |
| enrolling | not here. The proof is a signature made by the host's own `ssh-keygen` ([identity § proving possession](02-access.md#proving-possession)); the local selector script neither reads keys nor signs. The dashboard prints the host onboarding command |


A bus restart invalidates browser sessions: their map is not persisted.

**So does removing the credential they came from.** A session is a credential
without being a token, so one that outlived its token would leave a name
answering for up to the idle timeout after the daemon decided it answers for
nothing — [no registration, no access](01-identity-and-roles.md#unregistering) not
holding, quietly. Unregistering, deleting a service and the
[ownerless sweep](02-access.md#ownerless-credentials) all end that name's
sessions with its token.

### What it shows

| View | MVP status | Source or remaining dependency |
|---|---|---|
| the sign-in page and the token help | Built | — |
| **registry**, as this caller may see it: kind, owner, protocol, description, `readers`/`queued`/`in`/`out`, when the record was last written, the configuration's digest | Built | — it is `/ls` |
| **inboxes holding messages** — a backlog, oldest first, marked when the queue is at its bound and accompanied by the Readers count. Holding is not being stuck; filtered readers may coexist with unmatched queued work, and the count is observation rather than health. The one view an incident actually needs | Built | — `oldest` and `readers` on the record ([what a listing answers](#what-a-listing-answers)) |
| **exchanges** — retained messages and referenced receipt evidence | Built | [correlation and limits](#retained-exchanges) |
| **my names** — what I hold a credential for, whose it is and what it is for, its fingerprint, when it was issued and last used, and how to rotate it | Built | — the caller asks for its own, and gets a fingerprint rather than the token ([token lifetime](02-access.md#token-lifetime)). A person's own identity is distinguished from the services they registered. A credential [goes with its address](01-identity-and-roles.md#unregistering), so the list stays names something answers on |
| **loss by name** — what each inbox dropped to overflow and what expired in it | Built | — `dropped` and `expired` on the record ([what a listing answers](#what-a-listing-answers)) |
| **refusals** — how many calls were refused and why: bad credential, ACL, unknown receiver, second reader, full queue | Built | The built page shows only refusal reasons that occurred; counters are on `status` ([refusals](#refusals)) |
| **node** — its name, uptime, the registry's totals, and whether the last stop was clean | Built | — `status` carries the unclean-restart fact |
| **people** — identities, profiles, local avatars, authority, state, group membership and owned services | Built | [person records](01-identity-and-roles.md#users-and-profiles) and [user lifecycle](01-identity-and-roles.md#user-states) |

### Retained exchanges

**Built.** Diagnostics preserves each message's identity, routing labels and
receipt references within the caller's [visible feed](#dashboard). A bounded
history is an observation window: absent evidence does not establish failure
or unfinished work, and a failed load is shown separately from an empty feed.

| Evidence | Presentation |
|---|---|
| Receipt references one retained ordinary message, matches its receiver and full return route, and was observed no earlier | Group with that message; expose the receipt's identity, reference, sender and time |
| Missing, ambiguous or inconsistent reference; reference to another receipt | Keep the receipt separate and explain what prevents correlation |
| Ordinary message with a tag matches an earlier message's return route | Keep a separate row and link possible responses; a route match proves neither a reply nor completion |
| Acknowledgement or completion receipt is grouped with the original | Report that receipt as observed; do not infer execution success from a dequeue or ordinary message |
| Grouped receipt arrives after the original's deadline | Mark it after that request's deadline; equality is not late |
| Ordinary message has exactly one possible response match with a deadline | Qualify the late mark by that match; ambiguous matches supply no single deadline |

Return-route overrides replace the original destination and labels for these
comparisons. Receipts from topic subscribers or queue workers stay separate
when their identity differs from the addressed topic; untagged ordinary
messages get no inferred response links. Rows sort by newest observed activity,
with message identity breaking ties; displayed times include their UTC offset.
Bodies never enter this view. [MVP verification](../Plans/MVP/done/exchange-evidence.md#checks)
records the implemented checks; installed browser acceptance remains separate.

### Refusals

A bus that is quiet and one that is refusing every call look identical from
outside. `status` carries **how many calls were turned away and for what**,
counted where a refusal becomes a status code so that the reason and the code
cannot drift apart. Every refusing path calls one shared counter, which is what
keeps the figures whole — a fact about the paths there are, not a guarantee that
a future one cannot answer around it.

**An addressed endpoint's refusals are counted whatever the caller's standing**
— a bad token and a malformed request alike. Two things are not counted, and
neither is a caller being turned away: a route the router rejects before any
handler runs, and a failure of ours, which is a `500`.

| Reason | | |
|---|---|---|
| `credential` | `401` | the token is not one, none came at all, or it backs a name the daemon knows nothing about — all three are *who are you*, and none of them is a state anybody can lift ([access § what a call carries](02-access.md#what-a-call-carries)) |
| `acl` | `403` | the service, the record's owner, or a private configuration said no ([identity § acl](02-access.md#acl)) |
| `suspended` | `403` | a user state is in the way: the caller's own, or that of the owner of the name being called ([user lifecycle](01-identity-and-roles.md#user-states), [services of a paused or banned user](01-identity-and-roles.md#user-states)) |
| `enrolment` | `403` | a challenge that did not hold ([identity § proving possession](02-access.md#proving-possession)) |
| `unknown` | `404` | no such name ([messaging § verbs](04-messaging.md#verbs)) |
| `disabled` | `409` | the receiver's record is turned off by its owner ([owner control](01-identity-and-roles.md#services)) |
| `busy` | `409` | removal conflicts with current state: an inbox has queued messages or a waiting reader ([unregistering](01-identity-and-roles.md#unregistering)), or a credential is backed by a user, record or retained service ([cleanup](02-access.md#ownerless-credentials)) |
| `second-reader` | `409` | an inbox has an incompatible outstanding reader; sharing requires both readers to ask ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox)) |
| `name-taken` | `412` | a registration that asked for an unheld name found it held ([registration](01-identity-and-roles.md#registration)) |
| `full` | `429` | the receiver's queue is at its bound and refuses rather than loses ([messaging § overflow](04-messaging.md#overflow)) |
| `malformed` | `400` | the caller got the request wrong. One reason, not eight: *"you sent nonsense"* is a single answer however many ways there are to send it |

**Two answers a caller must never confuse.** `401` is *who are you* — the
credential is missing or is not one. `403` is *I know who you are and you may
not* — authenticated, and refused for lack of permission. Retrying a `403` with
the same credential is a caller asking the same question twice; a `401` is
worth presenting a credential for.

**One suspension, one reason.** A call refused because the caller is banned and
one refused because the *owner of the name they called* is banned are the same
fact seen from two sides, and get the same code and the same reason rather than
a second of each. The caller is told what they can act on either way, which is
nothing: no credential they could present and no permission anybody could grant
makes a suspended name answer, and the code already says do not retry this.

A record's own state is a reason of its own, and deliberately not `unknown`:
*turned off* and *no such name* send a caller to different places, so they are
never the same answer. Declared states beyond `disabled` are
[R1.1 work](../Plans/R1.1/records.md#down-and-retired) and would each bring
their own reason rather than borrow one.

**Only reasons that have happened appear** — a reason with a zero beside it is
noise on every other node. And a fault of the daemon's own is a 500 and is
**not** in here: refusing a caller and failing one are different things to be
told about, and folding them together would answer *"how often am I refusing
callers?"* with a number that includes our bugs.

### Where it listens

The dashboard is **`http://127.0.0.1:6780`**, and that is the whole of it: an
address it binds, not a name anybody has to make resolve.

| It wants | Default |
|---|---|
| where to listen | `127.0.0.1:6780`, with `-addr` or `AGENT_BUS_WEB_ADDR` |
| a certificate | **none.** Supply `-cert` *and* `-key` and it serves HTTPS on the address it was given. Ask for one and miss it and it **refuses to start**: either flag is the ask, either without the other is the same refusal, and a log line nobody reads is not an answer when the page they open is unencrypted |
| a port it may not bind | an error. No port is a default any more, so every one was asked for on purpose and none is silently traded for another |

**The bus does not listen off this machine**, so the page is for the person at
it. A borrowed public hostname bought a certificate a browser would accept and
nothing else; the one this project used had expired, which left the trust
decision without even that to show for it. Anybody who wants HTTPS here has a
certificate of their own and says where it is.

**The API's own root sends a browser here.** `http://127.0.0.1:6767/` is the
daemon, which has no page: a person who typed it in wanted the dashboard, and
gets `301` to it. Only the exact root — a mistyped route stays the 404 it is.
The daemon is told where to point, with `-dashboard` or `AGENT_BUS_DASHBOARD`,
defaulting to the address above; empty serves no root at all. The redirect is
permanent because the root will never grow a page of its own, so a browser may
stop asking: moving the dashboard afterwards is a thing to clear from a cache.

## Form recovery and keyboard entry

**Built in 0.5.70.** Public and signed-in pages start with a keyboard skip link
to the main content. It becomes visible when focused; the ordinary shell still
supplies the page title, landmarks, current navigation and signed-in identity.

A refused Service, Channel, User or Group submission returns its form when the
face can still render that target. The page keeps only an explicit allowlist of
nonsensitive values, including line-preserving ACL, Maintainer and Group
textareas. Tokens, unknown submitted fields and private configuration never
enter presentation state; a configuration replacement is empty even after a
refusal. The page shows one alert summary and associates an input only when the
face can identify that field without guessing from daemon prose. A missing or
newly hidden edited target still uses the ordinary indistinguishable not-found
page.

The daemon remains the validator. Its JSON error envelope is rendered as a
human message, not raw JSON. A malformed or retired browser action that never
reaches the daemon says so on the shared shell rather than attributing the
refusal to the daemon. This is the recovery portion of
[F.13.2](../Plans/MVP/TODO.md#web-redesign); the remaining page redesign and
typed-component migration stay pending.

## Browser acceptance

**Required MVP; current installed foundation and authority matrix built,
post-redesign rerun pending.** Real
Chromium on the package-only host signs in and out through visible controls,
checks the cookie boundary, visits the current required tabs, retains its
session across a real web-child restart and loses it across a real bus-child
restart ([evidence](../Plans/MVP/done/installed-browser-foundation.md#checks)).
Five independent browser sessions exercise the current service/channel,
user/group, denial, foreign-origin and activity paths as daemon Owner,
Administrator, resource Owner/Maintainer and ordinary user
([evidence](../Plans/MVP/done/installed-browser-role-matrix.md#checks)). Retain
and rerun both sets against the approved redesign. Web and bus restarts
continue to follow the [session contract](#signing-in).
HTTP handler tests and command-line cookie jars remain useful evidence but
do not establish this browser workflow. [F.12](../Plans/MVP/TODO.md#remaining-work)
owns the installed exercise and mutation checks.

## Identity labels in web and CLI

**Built in 0.5.54.** In the web interface and human-readable CLI output, use:

| Label | Entity |
|---|---|
| 👤 User | Registered person |
| 👾 Agent | Agent identity |
| ⚙️ Service | Service identity |
| 👥 Group | Group or team |

These glyphs label entity types, not authority or health. Directory rows put the
glyph directly before the identity name: `👤 chief@srv1`. The directory's
headings, authority column and introductory key carry the words, so the row does
not repeat `👤 User` underneath the same name. Group headings use the same compact
form: `👥 @group`. Other contexts keep the visible type word beside the glyph.
Owner, Administrator, Maintainer and Member remain separate
[role labels](01-identity-and-roles.md#role-names-and-scopes). Use the identity
and record facts returned by the daemon rather than guessing type from a name.
This vocabulary is for displayed labels; it does not rename API kinds, alter
JSON output or prescribe MCP output.

WEB applies the same daemon-kind mapping to directory, service, Personal,
detail and diagnostics views. A directory row with no caller-visible record
kind stays unlabeled; the face does not infer a glyph from its name or
credential. Channel remains the web term for a topic. Human `agent-bus ls -h`
uses the same identity labels and keeps the stated `topic` kind; raw `ls` keeps
the daemon's JSON unchanged. Filter and form values remain plain vocabulary
even when their visible option label carries a glyph.

### ACL editing

ACL textareas use the project's plain-text ACL syntax, not the display glyphs.
Each ACL term occupies one line: a user, ordinary group, agent, service,
runtime `@owner` term or `*`. `@owner` means the record's direct Owner and the
Services and Agents directly owned by that Owner; it is not an editable group.
Users
must not need to type Unicode to identify one. Blank lines are ignored; a
refused line is reported against that line and the submitted text is preserved.
Keep glyphs in surrounding labels or read-only views. Saving an ACL preserves
its terms and does not add display symbols to it. The same rule applies to CLI
command arguments and copyable ACL examples.

Maintainers and Group membership use the same line-list textarea. Each
Maintainer term occupies one line; each direct Group member, including a nested
group, occupies one line. Display glyphs never enter these editable values.

The [ACL contract](02-access.md#acl) defines access terms and their implementation status.
The [proposed role syntax](../Plans/R1/identity.md#sigils) remains separately
identified as proposed; this display rule does not introduce new parser syntax.

### Resource Danger Zone

**Built in 0.5.63.** Ordinary Service and Channel detail pages do not render
configuration replacement, ownership transfer or registration removal forms.
An authorized manager follows the red **Danger Zone** link to a separate page;
the face repeats the caller-visible record lookup and the daemon remains the
authority for every submitted action.

Configuration replacement accepts a new JSON value there and never displays
the stored private value. Transfer and removal first post to a server-rendered
confirmation that re-reads the record. The final submission rechecks the facts
shown by that confirmation and the daemon rechecks the operation itself. A
changed visible fact returns a distinct stale-confirmation page; an unchanged
current-state refusal keeps the daemon's own reason. Successful ordinary edits
and configuration replacement return to the affected detail; successful
removal returns to the matching Services or Channels list.
