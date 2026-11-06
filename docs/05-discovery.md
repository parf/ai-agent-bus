# Discovery

📌 **TL;DR:**
- **Problem:** being registered and being callable are different facts, and a plain list of names cannot say which names you may reach or whether anything answers for them.
- **Outcome:** you discover the records you may reach and inspect what the daemon actually knows: a listing says what a name is and whether anything answers for it.
- **Start:** list the records visible to you: `agent-bus ls --all` ([Try this first](#try-this-first)).

## Try this first

List the records visible to your identity:

```sh
agent-bus ls --all
```

Use a returned name with `agent-bus ls <name>` to inspect that record. An empty list means no records match your visibility and filters.

## What a listing answers

**Being in the registry and being callable are different facts.** "There is a MySQL on `db1:3306`" is a complete 📡 registration ([records § record kinds](03-records.md#record-kinds)) and nothing on this bus answers for it; an agent template is registered and deliberately does not run ([records § agent templates](03-records-agent.md#agent-templates)).
- So a caller reading a listing needs more than a name:

| Field | Says | Absent means |
|---|---|---|
| **`protocol`** | how to call a 📡, which is not through the bus ([services § how to call it](03-records-service.md#how-to-call-it)) | use the [record kind](03-records.md#record-kinds) to choose how to use it |
| **`readers`** | how many consume requests are outstanding now, filtered and unfiltered together | unavailable; a current daemon publishes measured zero explicitly |
| **`queued`** | how many messages are waiting in it | none are |
| **`in`** · **`out`** | how many messages have arrived for it, and how many a reader has taken, in total | none have |
| **`dropped`** · **`expired`** | what its queue lost to overflow, and what outlived its TTL in it, in total ([messaging § overflow](04-messaging.md#overflow)) | it has lost nothing |
| **`oldest`** | how long the message at the head of its queue has been waiting | its queue is empty |
| **`at_bound`** | its queue held the limit it is allowed **when the question was asked**. Not a prediction about the next message: a waiting reader is handed one without it ever queueing, and enqueueing prunes what has expired before it tests fullness, so if the queue is still full then the record's [overflow policy](04-messaging.md#overflow) applies — refuse, or forget the oldest | there is room. The daemon answers it because a record that declares no bound takes the daemon's, and a reader cannot know what that is |
| **`last_used`** | when the name's credential last authenticated a call, live; the listing puts the most recently used first, then the never used by name | the name holds no credential, or it was never used |

- These are **observations attached to the listing**, not values a registrant may state.
- Traffic and loss counters are durable totals, kept with the queue in the database and checkpointed every minute; `readers` is not ([queue durability](04-messaging.md#durability)).
- They describe the observed inbox, not health or a guarantee that a particular request will be handled.

**A 📡 answers none of them.** It has no queue here, so the daemon states no
reader count and no queued total for one rather than reporting a zero it never
measured ([records § record kinds](03-records.md#record-kinds)).

An inbox that was drained and one nobody ever wrote to both read as empty.
`in` and `out` are what tell them apart, and they are per name: a busy bus
does not make a quiet name look busy.

## Readers

Every human face shows one **Readers** count per
visible inbox: all currently outstanding consume requests, filtered and
unfiltered together. No separate counters or breakdown by filter type.

- This helps an operator see whether anything is waiting to read.
- Zero means no read is outstanding at that instant, not that whatever reads the queue is gone; a reader may be processing a message between reads.
- A positive count does not promise that a particular message matches, or that any work has finished.
- Count waiting requests, not processes, sessions or completed reads.

See the [full-inbox reading rule](04-messaging.md#one-reader-per-inbox) for how waiting readers share work.

## Where a pool member says it is

A pool is one name read by several machines, so the name no longer says which one serves it. A read can state its own host, and the listing then answers *which machines are reading this name*.

- **Stated, never observed:** a read carries `host=<name>`, 1 to 64 characters of letters, digits, `.` and `-`; anything else is refused with a 400 that names it.
- **Who states it:** the runner and `agent-bus run` on every read, from `AGENT_BUS_READER_HOST` when the unit sets it, else the machine's hostname (a value the daemon would refuse is left out, with a note, and the read goes on), and `agent-bus consume --host=name` by hand, which takes the same 1 to 64 characters of letters, digits, `.` and `-`.
- **A label only:** it is not an identity, a route or an ACL input, and it is never written to the database or a backup.
- **Short-lived:** a host stays listed for 2 minutes after its last read began or ended, so a member whose places are all busy keeps its entry.
- **Bounded:** at most 32 hosts per name; beyond that the oldest is forgotten and the listing says it is cut.
- **Shown:** the lookup lists each host with its name and last-read time (only when there is one), to whoever may see the record's `readers`; `agent-bus ls --table <name>` adds a `Hosts` line such as `srv1 (now), srv2 (40s)`, and the web record page lists them under its counters. The `ls` table is unchanged.

## CLI listing

`agent-bus ls --table` renders a table; plain `ls` retains JSON.
**With no option, both list only the agents being read now** — agents with at least one reader, the ones that can take a message.
- `--kind k` lists every record of that kind, readers or not; `--all` lists everything the caller may see; a single name looks that record up.
- `ab_ls` follows the same rule, with `kind` and `all: true`.
- The web face's lists are not filtered this way.
- The table shows name, kind, owner, Readers count, queued count and description.
- A current daemon reports a numeric Readers and queued value for every row that has a queue, and `-` for a 📡, which has none; a missing older answer renders as `unavailable`, which is a different fact from not applying, and the table's legend says which `-` is which.
- It is the [listing observation](#what-a-listing-answers), not a health check.
- An empty result says `No matching records.`; lookup errors remain errors.

Record [tags](03-records.md#tags) narrow the CLI listing, and the readers rule still applies:

- `ls --tag t` keeps the records carrying that tag, and still only the agents being read now: a bus record does not mean it is running. Add `--all`, or `--kind`, to see the rest.
- `ls --tag a,b`, or `ls --tag a --tag b`, keeps the records carrying both.
- An empty `--tag=` is refused, never read as no filter.
- [Calling many agents](user/cli.md#calling-many-agents-at-once) uses the same caller-visible filter to select agents for one question.

## Faces

| Face | Capabilities |
|---|---|
| API | Registry, messaging, credentials, sessions and dashboard administration |
| MCP | Bus tools and a catalog filtered by the daemon |
| WEB | [Required tabs and controls](#required-tabs), filtered through the caller's API access; its own account and locked-down unit ([processes § the web face](08-processes.md#the-web-face)); page contract in [the site map](../src/web/web-face/site-map.md#every-address) |

## MCP minimum

The [session launchers](06-launchers.md#smart-launchers) configure the bus MCP tools in the session.

| Capability | Outcome | Tool |
|---|---|---|
| List the bus | Find the agents being read now; `kind` or `all` for queues, pubsub channels, services and idle agents, through the caller's [catalogue view](#audience) | `ab_ls` in the [MCP face](../src/mcp/server.ts) |
| Call a name on the bus | Send to an agent, user or queue and receive its correlated answer or explicit completion under the [request/reply contract](04-messaging.md#request-and-reply); bus acceptance alone is not completion. A 📡 is not sent to: the catalogue gives its address and the caller speaks to it itself | `ab_send` plus a filtered `ab_consume`, or delivery through the active push adapter; [MCP face](../src/mcp/server.ts) |

- These are minimum capabilities, not a restriction on the remaining tools.
- Both capabilities use the caller's existing [ACL](02-access.md#acl); loading the tools grants no additional authority.

## Audience

The daemon filters, because it holds the records and enforces the
[ACL](02-access.md#acl). A face requests the caller's view and adds no
independent access policy.

## Dashboard

- The [web face](08-processes.md#the-web-face) ([page contract](../src/web/web-face/site-map.md#every-address)) presents the bus and forwards administration **as the person looking**.
- The daemon owns state and authorization: every page and form uses the visitor's authority, and the face has no independent privileges ([audience](#audience)).
- The [required tabs](#required-tabs) use the same daemon authorization as direct API calls.

The diagnostics feed shows [envelopes](04-messaging.md#envelope), never message bodies.

- Bodies are struck out **in the bus**, where the feed is written — so no reader has to be trusted to leave them alone.
- The feed and the records beside it are both filtered per caller: you see the exchanges you were **party to**, sent or addressed to you, and the daemon Owner sees the node's through node-wide authority.

**The node's own counters are the node's, not the caller's.** Uptime, the registry's totals and the [refusal counts](#refusals) describe the daemon, and anybody who may ask sees the same numbers — there is no narrower audience for them.
- Somebody with no access sees nothing at all, which is the whole of the restriction.
- A page showing them beside a caller-filtered list must say which is which: *the node refused 40 calls* and *you were refused 2* are both true, and neither is the other.

- The diagnostics page carries the views in [what it shows](#what-it-shows); record, user and group administration and activity graphs have separate pages.
- The ordering, the grouping and the late mark are the page's; everything else on it is an answer the bus gave that caller, so a view cannot show more than the caller may ask for.

### Required tabs

The daemon owner has the administrative view; other visitors see and change only
what the daemon permits. “All” means all visible to that visitor.

| Tab | Required functionality |
|---|---|
| Agents | 👾 records: my / all; active / inactive filters; details and [owner controls](01-identity-and-authority.md#record-authority), with owner, Maintainers list, access, Readers count and queue statistics. Excludes Personal agents, which its Personal filter shows. Administrative availability and reader observation are distinct facts |
| Services | 📡 records alone — something [external](03-records.md#record-kinds), with its address, protocol, owner, access and description. No Readers count, no queue statistics and no delivery switch, because a service has none |
| Personal | Not a tab: the `?personal=1` filter on each kind's list (Agents, Services, Queues, PubSub, Groups) shows owner-tagged records without changing access. Ordinary visitors see their own; the daemon owner sees every visible owner's and may narrow to one |
| Resources | 📚 cards, 🧩 for templates: list with URI, Source and MIME type; register; detail showing the card, never the data; the ACL and Maintainers as on every record ([Resource records](03-records-resource.md#what-a-resource-is)) |
| Users | List and details; add, edit, deactivate and reactivate; show caller-visible owned records, linked group membership and administrative authority. The directory lists 👤 Users only, laid out like the other list pages: one search and **Status** toolbar (**Active**, **Inactive**, **All states**, counted; opens on Active), then User, Authority, Contact, Agents owned and Last used columns, or an empty-state card when nothing matches. An inactive User is struck and marked beside the name rather than in a column of its own. It has no Other section: a name that is neither User nor Agent is [a Diagnostics leftover](#overview-and-diagnostics). **Edit profile** where allowed; deactivation starts from the user's Danger Zone and keeps its consequence confirmation, as does unused-credential removal |
| Locks | The **Locks** page lists every lock on the records the visitor may use, with a kind filter and search; and on every record's page, for its Owner, Maintainers and own Agent: name, holder, time left, **Release** for one's own and **Force release**, behind a confirmation, for another's ([shared locks](01-identity-and-authority.md#shared-locks)) |
| KV | The **KV** page lists every key-value store on the records the visitor may use, with a kind filter and search; a record's page carries an overview card, and each store and value its own page to add, edit, count and delete, for the record's Owner, Maintainers and own Agent ([key-value store](03-records-kv.md#key-value-store)) |
| Groups | Compact linked table with inline members; create, edit and manage direct entries, including nested ordinary groups; show caller-visible records affected directly or through a nested group. Explain the protected daemon Administrator group and include groups named by records' Maintainers lists under the [authority rules](01-identity-and-authority.md#groups); retire groups by emptying them, with no delete control. **The `@administrators` page alone states the authority its membership carries and its limits**, following the [Administrator rule](01-identity-and-authority.md#daemon-administrators); ordinary groups confer only the access assigned to them |
| Activity graphs | Recent traffic, messages dequeued, drops, expirations and refusals; per-record filtering. Dequeued messages are not proof of successful execution. Day, Week and Month charts over [the stored days](#activity-history), in ten-minute slots of the node clock |
| Queues | 📮 queue records, and the detail of a 👤 user's own inbox; create, edit and remove; owner, Maintainers list, permissions, Deliver-To route, TTL, capacity and overflow policy, Readers and held work |
| PubSub | 📣 pub/sub records; create, edit and remove; owner, Maintainers list, permissions (who may publish) and the Deliver-To list. Accepted and copies-out counters; no held work and no reader filter, because a 📣 keeps nothing |

The [Personal view](03-records.md#personal-and-shared) is that filter. Any kind
may be Personal and a user record always is, so the main collections show
shared records only.

### Section navigation and registration

- Agents and Services show caller-visible **All**, **My** and **Personal** counts; Queues, PubSub, Resources and Groups **All** and **Personal**; Users its visible directory total.
- These are counts computed from the page's existing daemon answers, not node-wide metrics and not additional reads.
- A shared list that is empty only because its records are Personal says how many are under the Personal filter and links there ([records § lists](../src/web/web-face/records.md#lists)).

- The signed-in identity in the top bar links to Account.
- Account shows its optional user profile or own record, caller-visible owned records, held credential fingerprints and command-line rotation help.
- A non-user principal does not need a user-directory row.

### Registry filters and paging

- Agents and Queues filter work held and live reader observations independently; PubSub filters neither, a 📣 holding no queue.
- Reader choices distinguish a positive count, measured zero and an unavailable observation; none is a health claim.
- Search, owner and sort remain URL state beside those filters.
- Services offers none of the three: they are observations of a queue, and a [service has none](03-records.md#record-kinds).

- The web face filters and sorts one caller-visible `/ls` answer, then shows at most 100 rows.
- It reports the matching count, bounds invalid page numbers and retains the exact page and filters through record detail and back.
- Section counts remain category totals before toolbar filters, rather than page counts.

Every numeric column in a web table aligns its header and values to the right
and uses tabular figures. Prose-embedded counts remain part of their sentence.

### Agent, service and channel journeys

Each kind has its own list, detail, registration and settings page, and no
section has a Kind filter: each lists one kind.

| Kind | List | Detail | A row shows |
|---|---|---|---|
| 👾 Agents | `/agents` | `/agent` | queued inbox work beside its Readers count |
| 📡 Services | `/services` | `/service` | address and protocol; nothing about a queue |
| 📮 Queues | `/queues` | `/queue` | messages held for a reader beside its Readers count |
| 📣 PubSub | `/pubsub` | `/pubsub/channel` | messages accepted, copies out and its Deliver-To count; never a backlog |

- The old `/channels`, `/channels/new`, `/channel` and `/channel/edit` redirect permanently to the kind's own address, so bookmarks keep working.
- Search, filters, sort and page are plain URL state kept through detail and back; a successful registration or edit returns to that record.
- Caller-visible facts stay readable while edit controls appear only when the daemon grants management authority.
- Ownership and Personal marks come only from the returned Owner and tag.

- Registration opens `/agents/new`, `/services/new`, `/queues/new`, `/pubsub/new`, `/users/new` and `/groups/new`.
- Register user appears only when the daemon says the visitor is an Administrator, and the route repeats that check; anyone may register a group, and the daemon decides.
- The old empty `/user` URL still opens user registration.
- The pages are specified in [records](../src/web/web-face/records.md#lists) and [people](../src/web/web-face/people.md#users-users).

### Page titles and compact help

- Every page title starts with one decorative mark and keeps its visible text: the kind glyph on kind pages, record detail using the daemon-stated kind, and a Lucide icon elsewhere; never caller text or a machine-readable value.
- Definitions sit behind an `ⓘ` button: hover or focus shows the explanation, a click opens the native popover ([shell § titles and help](../src/web/web-face/shell.md#titles-and-help)).
- Current scope, counts, filters, form constraints, refusals and dangerous consequences stay visible where they affect a decision.

- [Administrative and record authority](01-identity-and-authority.md#groups) applies to every control and to direct API calls.
- Membership and policy changes survive restart.
- User lifecycle effects are [daemon policy](01-identity-and-authority.md#user-states), not merely labels on the Users page.
- The navigation links to the [available views](#what-it-shows).

### Compact administration pages

**Adding an entity and editing one are the same form**, one page per kind and
one field set rendered by both, for a 👤 user and a 👥 group as much as for a
record ([records § register](../src/web/web-face/records.md#register)).

| Rule | |
|---|---|
| Fields by kind | a 👾 and a 📮 declare the TTL, capacity and overflow of the inbox they hold; a 📣 declares a [Deliver-To list](04-messaging.md#deliver-to) and no queue policy; a 📡, a 👾 and a 👥 offer a [secret](03-records-service.md#secrets), written by a second call and never filled in again |
| Settings | every field the daemon lets that kind's manager change ([record fields](constitution.md#common-record-fields)): a 👤 user's own inbox its description and queue policy; a 👥 group its description, members, Personal, Maintainers and secret |
| Not yours to change | a field the caller may not change is shown disabled, not hidden, and the form says which owner-only fields it carried, so a Maintainer saving a description cannot clear what it was not offered |
| Danger Zone | deactivation, transfer and removal sit on a separate red page where applicable. The group form offers no deactivation or removal control; empty it to remove its membership grants. Users have their own deactivation Danger Zone |
| Numbers | human-facing counts use grouped figures; JSON, URLs, form values and editable syntax stay plain. The Overview strip writes none as a dash; a table column keeps `0` |

### Overview and diagnostics

- Overview is the short signed-in landing page.
- It enumerates only supported attention conditions: an unclean prior stop, nonzero refusal reasons, queue capacity, loss, and inactive records still holding work.
- Ordinary backlog is work rather than an alarm.
- One record produces one item while the item retains every supporting fact.
- With nothing to report, the attention section gives way to one short `Nothing to report` card that makes no health claim.

**Records inactive because their owner is.** `GET /status` carries `owner_inactive`: how many records are inactive only because their owning User is, and how many messages they hold.
- It counts records whose own status is active, owned by an inactive User, other than that User's own record — what reactivating the User would bring back.
- The Overview shows one orange, node-wide item from those two numbers when the first is non-zero, linking to the inactive Users; the face joins nothing per record.

| | |
|---|---|
| who is answered | the daemon Owner and active Administrators, who can reactivate a User. Anybody else gets no field: the count says that somebody was made inactive, which is not an ordinary caller's business |
| absent field | unobserved — an older daemon or a caller not answered — and raises no item. It never reads as zero |
| beside the per-record items | an inactive record the caller may see still gets its own item from the [read-only inactive view](constitution.md#common-record-fields); the node-wide count may include records the page does not list |

- The node strip is node-wide.
- It carries the call counters and one tile per kind — Agents, Services, Queues, PubSub, Resources, Users and Groups ([status](#what-a-node-says-about-itself)).
- **When the page was generated is stated once, in the shared footer**, and Overview has no Refresh link; the footer is on every page, so every page is dated by one line.
- Diagnostics has a Refresh button.
- Attention and record links contain only facts the caller may see, so their scopes need not agree.
- The Find row links the two holding-work views, real URL filters, and External services; Users and Diagnostics are navigation entries.

- Diagnostics retains the detailed refusal, held-work, bounded envelope and loss evidence.
- Record names link to their visible record detail.
- **It lists leftover names** — a credential with no record, or a self-owned record with no User — **only while one exists**, each with its removal review; the retired `/users?kind=other` link redirects there.
- Use the registry sections to search records.
- A daemon failure renders a recovery page without exposing the socket or other backend address, which is the general rule in [shell and recovery](#shell-and-recovery).

### Activity history

- Each record keeps its last day of traffic live: a ring of 144 ten-minute slots of the node's local clock, 00:00, 00:10 … 23:50.
- Every calendar day it counted anything is also kept, for 400 days, as one row per name and date, so a week or a month can be read back.
- `/activity` and every record's detail page show Day, Week and Month with ‹ Prev and Next ›; the unfiltered view sums what the caller may see.

<details><summary>Rules</summary>

| | |
|---|---|
| Slot | Ten minutes of node local time holding that interval's Accepted, Dequeued, Dropped, Expired and Refused. At each boundary the record's cumulative counters are differenced once; nothing is added to the send or consume path |
| Day | The 144 slots, oldest first. The last is the current slot, read live and still counting |
| Zero | A slot carries its counts and nothing else. Time the daemon was down, or an hour daylight saving skipped, reads zero like a quiet interval |
| Clock | Slots close at :00, :10 … :50 local time whenever the daemon started; the bus ticks at every minute of the clock, and a read ticks the rings it sums first. A clock that steps back moves nothing, and its counts stay in the open slot. A daylight-saving fall repeats 01:00–01:50, and the second pass overwrites the first |
| Durable days | Table `activity_days`: the local date as yymmdd (`260924`), the name, and that day's non-empty slots only — each as the gap since the previous one and its five counts — zstd-compressed. A silent day has no row; the name `""` holds the node-wide refusals. Kept 400 days, today included, pruned by the first boundary of each day; `/status` states `today` and `activity_days_kept`, which is what a face clamps a range to |
| Durability | Each name keeps a ledger of today and yesterday, seeded from their rows at start: every slot that closes with counts is set into it, and a row is always the ledger's whole day, never the ring's 24-hour window, so midnight, a stop the next day or a gap of a day never shortens or empties a stored day. A row is written when a slot that counted anything closes, right after the boundary — once per ten minutes at most, never on the send or consume path and never on the one-minute queue flush — and the graceful stop writes every held day with the open slot's counts so far. A start rebuilds each ring from yesterday's and today's rows; the time the daemon was down reads zero, and a restart inside the open slot keeps what it counted. An unclean stop loses at most the open ten minutes. A database from before durable days starts from the ring its queue carried, written as rows at once. An unreadable stored day is reported and reads as nothing |
| Range | `GET /activity/days?name=&from=yymmdd&to=yymmdd`, at most 62 days, answers each day's 144 slots from 00:00; the days the rings hold come from them, older days from the store. The same scope as the day, and an unreal date, a reversed or a longer range is `400`. With `totals=1` it answers each visible record's counts summed over the range instead, in one answer, as a chooser needs |
| Lifecycle | Removal deletes the day in the removal's commit. A transfer keeps it: activity belongs to the name. An inactive record's day is kept and served to nobody |
| Scope | A named view is record-scoped for all five series. The unfiltered view sums the live records the caller may see; Refused is node-wide for the daemon Owner, on the terms [refusals](#refusals) sets — every endpoint refusal, including authentication failures and malformed requests, and not the router's rejections or our own failures. Pub/sub copies count in the recipients' inboxes that accept them. Dequeued means handed to a reader, never completed work. Bodies never enter this history |
| Web | Day: a fixed 24-hour axis, one continuous line per nonzero series on one scale, and a table of each slot; a past day reads its stored row. Week: hourly lines and seven rows of 144 cells, one row per day. Month: day bars with the other series over them, and a calendar of 30 cells. ‹ Prev and Next › step one range while the days are kept; a record's detail page carries the same ranges |

</details>

### Rules it is built to

- Public pages expose only the [node identity](#what-a-node-says-about-itself), project information and sign-in help.
- Pages never show credential bytes or Secret values. Configuration appears only to its authorized editors.
- Forms forward the visitor's session and require an exact matching Origin. Responses are not cached.
- Fonts, icons, charts, styles and scripts ship with the face at content-hashed same-origin URLs. The CSP permits no external asset host; photos are inline and the landing artwork is served locally ([security headers](../src/web/web-face/shell.md#security-headers)).
- Sign-in takes a token, without a separate name or password.

### What a node says about itself

`GET /identity` needs no credential. It publishes version, build information,
hostname, daemon Owner name, uptime, call counts and the `protocol` and
`protocol_min` [protocol fields](09-daemon-api.md#protocol-version).

It exposes no registry rows, credentials, refusal counts or per-caller traffic.

- The dashboard header shows `AgentBus v<release> @ <host>`, with build detail in the version tooltip and a mark based on the [project artwork](img/agent-bus.png).
- Its footer shows Owner, uptime and page-generation time. Call counts appear on the signed-in Overview, but remain public through the API.
- The hostname comes from the OS and is not the bus realm.
- Anyone who can reach the API can learn who runs the node, how long it has been up and how much traffic it carries. Authentication does not hide these facts.

**Call counts measure requests, not completed work.** Every HTTP request counts
on arrival, including refusals, router errors and long polls. Loading a page
also makes requests; leaving it open does not refresh it automatically.

| Count | Meaning |
|---|---|
| Total | Exact cumulative count for this bus process; resets on restart |
| Minute and hour | Differences from a 61-sample history, sampled at clock-minute boundaries. The baseline is the newest sample at or before the cutoff |
| `observed` | Actual span of each sampled window, published by the API. The dashboard shows the window label without this span |
| Short or delayed history | A new process has less than an hour of history; delayed samples can widen a window. A nominal minute can cover roughly one to two minutes, and an hour slightly more. Missing history is not measured zero |

The counts can be equal when all traffic falls within the shortest window.
They are node-wide and process-local, separate from caller-filtered
[activity history](#activity-history).

### Signing in

A person signs in with **the token they already hold**
([access § what a call carries](02-access.md#what-a-call-carries)) — there is no
name to type, no other kind of credential and no password anywhere.

- The session lives in the bus ([shared state](08-processes.md#what-is-shared)).
- The face forwards it once, the bus answers with an expiring session id, and from then on the browser carries that id and nothing else.

| | |
|---|---|
| the cookie | the session id alone — `HttpOnly`, `SameSite=Strict`, and `Secure` when the face serves TLS. No `Max-Age`: the bus's idle timeout ends the session, and a fixed browser lifetime would sign out someone still working. Never the token, never in a URL |
| a web-face restart | keeps browser sessions valid; the bus holds them |
| what the face holds | nothing. It reaches the bus only over the shared socket, which supplies no identity — a web face with the owner's authority would be a credential mint ([access § getting a token](02-access.md#getting-a-token)) |
| enrolling | not here. The proof is a signature made by the host's own `ssh-keygen` ([identity § proving possession](02-access.md#proving-possession)); the browser script neither reads keys nor signs. The dashboard prints the host onboarding command |


A bus restart invalidates browser sessions: their map is not persisted.

Removing a credential also ends its browser sessions, including through unregistration, record deletion or the [ownerless sweep](02-access.md#ownerless-credentials).

### What it shows

| View | Source |
|---|---|
| the sign-in page and the token help | — |
| **registry**, as this caller may see it: kind, owner, protocol, description, `readers`/`queued`/`in`/`out`, when the record was last written, the configuration's digest | `/ls` |
| **inboxes holding messages** — a backlog, oldest first, marked when the queue is at its bound and accompanied by the Readers count. Holding is not being stuck; filtered readers may coexist with unmatched queued work, and the count is observation rather than health. The one view an incident actually needs | `oldest` and `readers` on the record ([what a listing answers](#what-a-listing-answers)) |
| **exchanges** — retained messages and referenced receipt evidence | [correlation and limits](#retained-exchanges) |
| **my names** — what I hold a credential for, whose it is and what it is for, its fingerprint, when it was issued and last used, and how to rotate it | the caller asks for its own, and gets a fingerprint rather than the token ([token lifetime](02-access.md#token-lifetime)). A person's own identity is distinguished from the services they registered. A credential [goes with its address](01-identity-and-authority.md#unregistering), so the list stays names something answers on |
| **loss by name** — what each inbox dropped to overflow and what expired in it | `dropped` and `expired` on the record ([what a listing answers](#what-a-listing-answers)) |
| **refusals** — how many calls were refused and why: bad credential, ACL, unknown receiver, second reader, full queue | Diagnostics shows every supported reason, including measured zero; counters are on `status` ([refusals](#refusals)) |
| **node** — its name, uptime, the registry's totals, and whether the last stop was clean | `status` carries the unclean-restart fact |
| **people** — identities, profiles, photos, authority, state, group membership and owned records | [person records](01-identity-and-authority.md#users-and-profiles) and [user lifecycle](01-identity-and-authority.md#user-states) |

### Retained exchanges

Diagnostics preserves each message's identity, routing labels and
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

- Return-route overrides replace the original destination and labels for these comparisons.
- Receipts from Deliver-To recipients or queue workers stay separate when their identity differs from the addressed channel; untagged ordinary messages get no inferred response links.
- Rows sort by newest observed activity, with message identity breaking ties; displayed times include their UTC offset.
- Bodies never enter this view.

### Refusals

- A bus that is quiet and one that is refusing every call look identical from outside.
- Every refusal body is `{"error": "<text>", "kind": "<reason>"}`: the `kind` is the reason below, so a client acts on it and shows the text. A `500` is ours, carries no `kind`, and is not counted; clients say "internal daemon error".
- `status` carries **how many calls were turned away and for what**, counted where a refusal becomes a status code so that the reason and the code cannot drift apart.

**An addressed endpoint's refusals are counted whatever the caller's standing**
— a bad token and a malformed request alike. Two things are not counted, and
neither is a caller being turned away: a route the router rejects before any
handler runs, and a failure of ours, which is a `500`.

| Reason | | |
|---|---|---|
| `credential` | `401` | the token is not one, none came at all, or it backs a name the daemon knows nothing about — all three are *who are you*, and none of them is a state anybody can lift ([access § what a call carries](02-access.md#what-a-call-carries)) |
| `acl` | `403` | the record's ACL, its owner, or configuration access said no ([identity § acl](02-access.md#acl)) |
| `suspended` | `403` | the caller is inactive: an inactive User, or an agent inactive itself or through its User ([user states](01-identity-and-authority.md#user-states)) |
| `enrolment` | `403` | a challenge that did not hold ([identity § proving possession](02-access.md#proving-possession)) |
| `not_found` | `404` | no such name — absent, or inactive and so [no such entity](constitution.md#common-record-fields) ([messaging § verbs](04-messaging.md#verbs)) |
| `busy` | `409` | removal conflicts with current state: an inbox has queued messages or a waiting reader ([unregistering](01-identity-and-authority.md#unregistering)), or a credential is backed by a user or a retained record ([cleanup](02-access.md#ownerless-credentials)) |
| `authority` | `403` | the call needs a standing the caller's route cannot give: a record import must come over the caller's own login socket ([setup § encrypted record backups](administration.md#encrypted-record-backups)) |
| `second-reader` | `409` | an inbox has an incompatible outstanding reader; sharing requires both readers to ask ([messaging § one reader per inbox](04-messaging.md#one-reader-per-inbox)) |
| `name-taken` | `412` | a registration that asked for an unheld name found it held ([registration](01-identity-and-authority.md#registration)) |
| `full` | `429` | the receiver's queue is at its bound and refuses rather than loses ([messaging § overflow](04-messaging.md#overflow)) |
| `malformed` | `400`; `408` when a request body does not arrive in 30 s (a bigfile upload: 30 s of silence, or past its [budget](03-records-kv.md#bigfiles)); `413` for a key-value value, a bigfile or a lock count past its [limit](03-records-kv.md#rules); `501` for SSH keys on a node with none configured | the caller got the request wrong. One reason, not eight: *"you sent nonsense"* is a single answer however many ways there are to send it |
| `unavailable` | `503` | an optional part of the daemon cannot serve now and says why in `error`: the bigfile store when its directory cannot be opened ([bigfiles](03-records-kv.md#bigfiles)); a client may retry later |
| `no_route` | `404`; `405` with an `Allow` header | no such route, or the route does not take that method; not counted, and a client reports the unsupported route ([stability](09-daemon-api.md#protocol-version)) |

**Two answers a caller must never confuse.** `401` is *who are you* — the
credential is missing or is not one. `403` is *I know who you are and you may
not* — authenticated, and refused for lack of permission. Retrying a `403` with
the same credential is a caller asking the same question twice; a `401` is
worth presenting a credential for.

**An inactive caller and an inactive target are different answers.** A caller that is inactive is told it is the one refused, `403 suspended`: it is somebody, and nothing it presents makes it active.
- A target that is inactive, itself or through its User, is no such name, `404 not_found`, whoever asks — the daemon Owner included — because an inactive record is no entity at all.
- The dashboard follows it: a `suspended` refusal is a page saying the caller's access is suspended, not the permission page's "Not yours to see", and every problem page shows the daemon's sentence, never the envelope it arrived in.

**Every supported reason appears**, including measured zero.
- This keeps the reason vocabulary inspectable without making zero a health claim.
- A fault of the daemon's own is a 500 and is **not** in here: refusing a caller and failing one are different things to be told about, and folding them together would answer *"how often am I refusing callers?"* with a number that includes our bugs.

### Where it listens

The dashboard is **`http://127.0.0.1:6780`** (`https://` once setup turns TLS on), and that is the whole of it: an
address it binds, not a name anybody has to make resolve.

| It wants | Default |
|---|---|
| where to listen | `127.0.0.1:6780`, with `AGENT_BUS_WEB_ADDR` in the unit's environment; the face takes no flags |
| a certificate | None unless TLS is configured. `AGENT_BUS_WEB_TLS_DIR` names a directory containing `cert.pem`, private `key.pem` and optional `chain.pem`; the port serves HTTPS and redirects plain HTTP. Missing or unreadable certificate files stop startup ([TLS](administration.md#tls)). |
| a port it may not bind | a startup error; the face does not silently choose another port |

**The web face listens on loopback**, so the page is for the person at the machine; agents on other hosts use the daemon's own TCP address instead.
- Setup's `--tls` gives it the node's certificate; by hand, a certificate of your own.

**The API's own root sends a browser here.** `http://127.0.0.1:6767/` is the daemon, which has no page: a person who typed it in wanted the dashboard, and gets `301` to it.
- Only the exact root — a mistyped route stays the 404 it is.
- The daemon is told where to point, with `--dashboard` or `AGENT_BUS_DASHBOARD`, defaulting to the address above; empty serves no root at all.
- The redirect is permanent because the root will never grow a page of its own, so a browser may stop asking: moving the dashboard afterwards is a thing to clear from a cache.

## Shell and recovery

- Every signed-in page carries the same shell: header and footer landmarks, one main landmark, the keyboard skip link and its target, the signed-in name linking to Account, sign out, and exactly one marked navigation entry.
- A refusal page belongs under no section and marks none.
- Public sign-in carries the landmarks and the skip link, and none of the account controls.

- Each page names itself in one document title, and no two pages share one.
- A detail, confirmation or refusal page names the record, identity, group, action or refusal it is about, so two open tabs of the same kind are two titles.
- The two not-found answers are the deliberate exception: they share one title because [hidden and absent are one answer](#refusals).

Four failures read as four different things to do next: never signed in,
a session that ended, a refusal for want of permission, and a daemon that is
not answering. A permission refusal never offers a credential, because one
cannot help.

**A failed section is not an empty one.** When part of a page cannot be read and the rest is still true, that part says so.
- A refusal the daemon worded is shown as its message rather than its JSON envelope; anything else is logged and replaced, because the transport text names the socket or address this face talks to.
- No page renders that address.
- A failed whole-page read is a refusal, never a page reporting that nothing is registered.

### Form recovery and keyboard entry

- A refused record, user or group submission returns its form when the face can still render that target.
- The page keeps only an explicit allowlist of nonsensitive values, including line-preserving ACL, Maintainer and Group textareas.
- Tokens, secrets and unknown submitted fields never enter presentation state.
- Authorized configuration editors load pretty-printed stored JSON; invalid configuration submissions are not echoed.
- The page shows one alert summary and associates an input only when the face can identify that field without guessing from daemon prose.
- A missing or newly hidden edited target still uses the ordinary indistinguishable not-found page.

- The daemon remains the validator.
- Its JSON error envelope is rendered as a human message, not raw JSON.
- A malformed or retired browser action that never reaches the daemon says so on the shared shell rather than attributing the refusal to the daemon ([shell § form recovery](../src/web/web-face/shell.md#form-recovery)).

## Identity labels in web and CLI

One label per stored kind. In the web interface
and human-readable CLI output, use:

| Label | Entity |
|---|---|
| 👤 User | Registered person |
| 👾 Agent | An agent, and the queue named after it |
| 📮 Queue | A queue registered for its own sake |
| 📣 PubSub | A pub/sub channel |
| 📡 Service | Something [external](03-records.md#record-kinds), not on this bus |
| 👥 Group | Group or team |

Every row above names a [stored kind](03-records.md#record-kinds),
so a label states what the daemon said rather than what a page inferred. A kind
the daemon did not state stays unlabeled.

- These glyphs label entity types, not health.
- Directory rows put the glyph directly before the identity name: `👤 chief@srv1`.
- The directory's headings, authority column and introductory key carry the words, so the row does not repeat `👤 User` underneath the same name.
- Group headings use the same compact form: `👥 @group`.
- Other contexts keep the visible type word beside the glyph.

**Two authorities carry a mark of their own**: `🔱` beside the daemon owner and `👮` beside a record's Maintainers.
- They are the exception that the no-glyph default allows rather than a second vocabulary: a node has exactly one daemon owner and a record states its maintainers once, so neither mark can spread across rows and become a column heading.
- A daemon administrator, a record Owner and a Member stay words, because those can be many.
- All of them remain [position labels](01-identity-and-authority.md#positions-and-scopes) rather than entity types.
- Use the identity and record facts returned by the daemon rather than guessing type from a name.
- This vocabulary is for displayed labels; it does not rename API kinds, alter JSON output or prescribe MCP output.

- WEB applies the same daemon-kind mapping to directory, agent, service, Personal, detail and diagnostics views.
- A directory row with no caller-visible record kind stays unlabeled; the face does not infer a glyph from its name or credential.
- Human `agent-bus ls --table` uses the same identity labels; raw `ls` keeps the daemon's JSON unchanged.
- Filter and form values remain plain vocabulary even when their visible option label carries a glyph.

### ACL editing

- ACL textareas use the project's plain-text ACL syntax, not the display glyphs.
- Each ACL term occupies one line: a user, ordinary group, agent, service, runtime `@owner` term or `*`.
- `@owner` means the record's direct Owner and that Owner's directly owned Agents; it is not an editable group.
- Users must not need to type Unicode to identify one.
- Blank lines are ignored; a refused line is reported against that line and the submitted text is preserved.
- Keep glyphs in surrounding labels or read-only views.
- Saving an ACL preserves its terms and does not add display symbols to it.
- The same rule applies to CLI command arguments and copyable ACL examples.

Maintainers and Group membership use the same line-list textarea. Each
Maintainer term occupies one line; each direct Group member, including a nested
group, occupies one line. Display glyphs never enter these editable values.

An agent term carries a leading `#`, the way a group term
carries `@`, and the stored line keeps it. A line without a marker names a
user or a channel. A refused save names the offending line by number,
never retyping or guessing a kind. See
[typed actor terms](constitution.md#-registry-record).

The [ACL contract](02-access.md#acl) defines access terms.
The [role syntax](03-records-agent.md#roles) allows labels after an ACL term;
commas and spaces inside its parentheses remain part of that one term.

### Resource Danger Zone

- Ordinary record detail pages do not render deactivation, configuration replacement, ownership transfer or registration removal controls.
- An authorized manager follows the red **Danger Zone** link to a separate page; the face repeats the caller-visible record lookup and the daemon remains the authority for every submitted action.

- For active agents, services, queues, pubsub channels and users, deactivation starts here and keeps its consequence confirmation.
- The browser offers no Group deactivation control; this is a UI limit, not an absence of daemon record state.
- Configuration replacement is in the normal edit form after Secret (ENV); Configuration is shown to Owner and Maintainers; Secret is never displayed.
- Keep secrets in Secret and config in Configuration ([rule](03-records-agent.md#configuring-a-template)).
- Transfer and removal first post to a server-rendered confirmation that re-reads the record.
- The final submission rechecks the facts shown by that confirmation and the daemon rechecks the operation itself.
- A changed visible fact returns a distinct stale-confirmation page; an unchanged current-state refusal keeps the daemon's own reason.
- Successful ordinary edits and configuration replacement return to the affected detail; successful removal returns to the matching section list.
