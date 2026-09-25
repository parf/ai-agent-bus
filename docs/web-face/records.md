# Web face record pages

📌 **TL;DR:** Internal working document for the TypeScript rewrite, not
linked from the docs set. It specifies every record page of `agent-bus-web`
as built in 0.8.31: the five lists, four registration forms, the detail,
settings, deactivation and Danger Zone pages, the legacy `/channel` addresses
and the two POST handlers. It gives each page's parameters, access by role,
daemon calls, content, states, controls, form fields and links. Derived from
`src/cmd/agent-bus-web/{admin,recordform,views,activity,main}.go` and checked
against a disposable daemon; where older specs disagree the code wins
([differences](#differences-from-the-older-specs)). The frame, problem
pages, form recovery, paging and `return` rules are in
[shell](shell.md#problem-page).

## Shared facts

### Roles

The face never decides authority. It reads the daemon's per-caller flags on
each record, and the daemon rechecks every write.

| Role | How the face knows | What the daemon grants on records |
|---|---|---|
| Daemon Owner | `GET /status` → `daemon_owner` | sees every record, active or inactive; `can_manage` and `can_transfer` on all of them |
| Administrator | `/status` → `administrator` | **nothing extra on records.** Visibility and flags as an ordinary user. Its authority is users and groups |
| Record Owner | record `owner == you` | `can_manage`, `can_transfer` |
| Record itself | record `name == you` (an agent signed in with its own token) | `can_manage`, not `can_transfer` |
| Maintainer | caller matches a `maintainers` term (direct, group, `@owner`, `@agent`) | `can_manage`, not `can_transfer` |
| Other signed-in user | the record's `allow` admits them | the record is visible, both flags false |
| Stranger | not admitted | the record is absent from `/ls` and `/inactive`; `/lookup` answers `404`. Every page shows `404 No such name`, the same as a name nobody holds |
| Signed out | no `agent_bus_session` cookie | every GET here answers `401` with the sign-in form, message `sign in to open this page`, `return` = the requested URI |

`can_manage` = Owner, record itself, Maintainer or daemon Owner.
`can_transfer` = Owner or daemon Owner. A suspended caller gets neither.

### Kinds

| Fact | 👾 agent | 📡 service | 📮 queue | 📣 pubsub | 👤 user inbox |
|---|---|---|---|---|---|
| List | `/agents` | `/services` | `/queues` | `/pubsub` | none (Users) |
| Detail | `/agent` | `/service` | `/queue` | `/pubsub/topic` | `/queue` |
| Settings | `/agent/edit` | `/service/edit` | `/queue/edit` | `/pubsub/topic/edit` | `/queue/edit` |
| Register | `/agents/new` | `/services/new` | `/queues/new` | `/pubsub/new` | none |
| Noun (`recordNoun`) | Agent | Service | Queue | PubSub | User |
| Name form | `#name[@realm]`, daemon refuses without `#` | `name[@realm]` | `name[@realm]` | `name[@realm]` | user name |
| `addr`, `protocol` | – | required | – | – | – |
| `secret` field | yes | yes | – | – | – |
| `ttl`, `bound`, `overflow` | yes | – | yes | – (detail still shows them) | yes (settings only) |
| `subs` | one-slot Deliver-To route (`<input>`) | – | one-slot route | Deliver-To list (`<textarea>`) | – |
| `allow` | yes | yes | yes (who may send) | yes (who may **publish**) | – |
| Personal | checkbox | checkbox | checkbox | checkbox | always; no control |
| `maintainers` | settings only | settings only | settings only | settings only | – |
| Replace configuration | yes | yes | – | – | – |
| Deactivate / Reactivate | yes | yes | yes | yes | – |
| Transfer | yes | yes | yes | yes | – (`name == owner`) |
| Remove | yes | yes | yes | yes | offered; daemon refuses `409` |

A group (👥) is a record too, but lives on Groups (people spec). It can reach
`/service-danger`: configuration only, no transfer for `@administrators` or a
self-owned name, and no removal.

### Record fields used

Every page reads records as JSON from `/ls`, `/inactive` or `/lookup`.

| JSON field | Shown as |
|---|---|
| `name` | routing name, in `<code>`; the row and page key |
| `kind` | label via `display.Entity`, e.g. `👾 Agent`; the daemon Owner's own user row reads `🔱 Daemon owner` |
| `descr` | description; the list's bold first line |
| `owner` | Owner, `<code>` |
| `maintainers[]` | `👮 Maintainers: a, b` (joined `, `), omitted when empty |
| `personal` | `Personal` marker, orange row rule |
| `status` | `inactive` or anything else (active). The face also stamps `inactive` on every `/inactive` row |
| `at` | Updated. List: relative (`now`, `5m ago`, `3h ago`, `12d ago`, `Jan 2`, `Jan 2, 2006`); detail: `2006-01-02 15:04:05` in the offset the daemon sent; zero → `¿` |
| `addr`, `protocol` | Address, Protocol |
| `ttl`, `bound`, `overflow` | Retention, Queue bound, When full (`ring` → drop the oldest, else refuse) |
| `allow[]` | only in the settings form. **No page displays it** |
| `subs[]` | route destination (first entry) or Deliver-To list; list count on PubSub |
| `secret_sha`, `config_sha` | digests; the bytes never |
| `can_manage`, `can_transfer` | which controls render |
| `route_allowed` | route state: `true` allowed, `false` refused, absent unreported |
| `readers` | number; absent → `unavailable` |
| `queued`, `in`, `out`, `dropped`, `expired`, `oldest`, `at_bound` | counters. `oldest` is a Go duration string (`1m3s`) |

Numbers use comma grouping (`number()`); tables keep `0`.

## Lists

`/agents` · `/services` · `/queues` · `/pubsub` · `/personal`

One handler. The page filters, sorts and pages one `/ls` + `/inactive`
answer; the daemon has no paging.

### Route

| Parameter | Values | Meaning | Default |
|---|---|---|---|
| `q` | text, trimmed | case-insensitive substring of `name`, `descr` or `owner` | none |
| `scope` | `my` | only records whose `owner` is you. Accepted on every list; only Agents and Services have a My tab | all |
| `state` | `active`, `inactive` | status filter | both |
| `readers` | `present` (readers > 0), `none` (= 0), `unavailable` (absent) | reader filter. Ignored on `/services`; **honoured but not offered** on `/pubsub` | any |
| `work` | `held` | `queued > 0`. Ignored on `/services` and `/pubsub` | any |
| `sort` | `updated` (newest `at` first), `queued` (most work first; on `/pubsub` the topic's `in`) | order; ties and default by `name` ascending. `queued` is dropped on `/services` | name |
| `page` | integer | 1-based, clamped to 1…last | 1 |
| `kind` | `agent`, `service`, `queue`, `pubsub` | `/personal` only: narrow to one kind. Ignored elsewhere and for other values | all kinds |
| `owner` | a name | `/personal`, daemon Owner only: one owner's records. From anyone else: `303` to the same URL without `owner` | all visible owners |

Any other value of a parameter reads as its default.

| Old address | Answer |
|---|---|
| `/channels?…` | `301` to `/pubsub` when `kind=pubsub`, else `/queues`; `kind` removed, rest of the query kept. No sign-in needed for the redirect |

### Access

| Role | Gets |
|---|---|
| Any signed-in caller | the page, with the records the daemon lets them see |
| Daemon Owner | every record; on `/personal` the owner chooser and every owner's Personal records |
| Everyone else | on `/personal` only their own Personal records (`Owned by <you>`) |
| Signed out | `401` sign-in form |

### Daemon calls

| Call | Gives |
|---|---|
| `GET /identity` (no credential) | header and footer node facts; `owner`, used for the `🔱` Type cell |
| `GET /status` | `you`, `daemon_owner`, `administrator` |
| `GET /ls` | active visible records |
| `GET /inactive` | inactive visible records |

Either list call failing fails the page ([shell](shell.md#problem-page)).

### Content

| Order | Element |
|---|---|
| Title | `Agents` · `Services` · `Queues` · `PubSub` · `Personal`, then ` · agent-bus`. Nav: the matching section; `/personal` marks Agents |
| 1 | `<h1>` title mark + section name, and an `ⓘ` popover `service-views-help` explaining the section (per-kind bullets, category counts, Status, Readers, Accepted/Dequeued) |
| 2 | Section tabs `<nav class=section-nav>` (below) |
| 3 | `/personal` only: owner chooser form (daemon Owner) or `<p>Owned by <code>you</code></p>` |
| 4 | Toolbar form (search, filters, sort) |
| 5 | Empty-state card, **or** `/personal` note, then no-match card or table |
| 6 | `<nav aria-label="Record pages">` with `Previous page` / `Next page` |

**Section tabs.** Counts are computed by the face over the visible records,
groups and users excluded, before toolbar filters.

| Page | Tabs |
|---|---|
| `/agents` | All (non-Personal agents) · My (those you own) · Personal (Personal agents: yours, or all for the daemon Owner) → `/personal?kind=agent` · Register agent |
| `/services` | All · My · Personal → `/personal?kind=service` · Register service |
| `/queues` | All · Personal → `/personal?kind=queue` · Register queue |
| `/pubsub` | All · Personal → `/personal?kind=pubsub` · Register pub/sub topic |
| `/personal` | the Agents tabs; Personal is current and counts the selected kind, or all Personal records |

**TypeScript face, 0.8.43: Personal is a filter, not a page.** `/personal` is
gone, with no redirect. Each kind's list takes `personal=1` (`/agents?personal=1`,
also `/groups?personal=1`) and its tabs are All · My (Agents and Services) ·
Personal. The sidebar is the kind axis: there is no Kind filter, and while the
filter is on the sidebar's Agents, Services, Queues, PubSub and Groups links
keep `personal=1`, marked with a lock; a Personal record's own pages keep it
too. Filters, paging, Clear filters and the one Register action
(`{kind}/new?personal=1`, which starts the form Personal and returns to the
Personal list) carry the flag, and the daemon Owner's owner chooser appears
with it. A list shows only columns that tell its rows apart: never Type, and
on a Personal list no Owner unless the daemon Owner sees several. There is no
Register tab: the page head holds the one Register action.

Tabs keep `state`, `q`, `readers`, `work`, `sort` (and `owner` for the daemon
Owner on Personal); Queues, PubSub and Services Personal tabs and Register
links are plain. The current tab has `aria-current=true`; My has class
`my-view`, Personal `personal-view`.

**Toolbar.** `<form class=record-search method=get action={this path}>`.

| Control | Detail |
|---|---|
| Search | `<input id=record-query type=search name=q>`, visually hidden label `Search records`, placeholder `Search by name, owner, or description` |
| Hidden | `scope`, `state`, `readers`, `work`, `kind`, `owner` (each only when set; `owner` is also set to *you* for a non-owner on `/personal`, which costs a `303` on every search) |
| Kind filter | `/personal` only: links All · 👾 Agent · 📡 Service · 📮 Queue · 📣 PubSub |
| Status filter | links All · Active · Inactive |
| Readers filter | not on Services or PubSub: All · Reading now · No reader now · Unavailable |
| Queue filter | not on Services or PubSub: All · Holding work |
| Sort | `<select id=record-sort name=sort data-submit-on-change>`: `""` Name (A–Z), `updated` Recently updated, `queued` Queued (high–low) — `Accepted (high–low)` on PubSub, absent on Services. `<noscript><button>Apply</button></noscript>` |

Filter links drop `page`, keep the other filters and toggle their own
parameter; the current one has `aria-current=true`.

**Table.** `<table class=record-table>`, caption `Showing {start}–{end} of
{matched} matching records, caller-visible on this page and not a count of
this node.` plus ` <a>Clear filters</a>` when `q`, `state`, `readers`, `work`
or `sort` is set.

| Page | Columns |
|---|---|
| Agents, Personal | Agent (Record on Personal) · Type · Owner · Status · Readers · Reached · Queued · Accepted · Dequeued · Updated |
| Services | Service · Type · Owner · Status · Address · Protocol · Updated |
| Queues | Queue · Type · Owner · Status · Readers · Held · Accepted · Dequeued · Updated |
| PubSub | PubSub · Type · Owner · Status · Accepted · Copies out · Deliver-To · Updated |

| Cell | Source |
|---|---|
| Name | link `{detail path}?name={name}&return={this list URL with page}`; `descr` in bold over `name`, or `name` alone. Class `owned-record` when `owner == you`, `personal-record` and a `Personal` marker when `personal` |
| Type | `display.Identity(kind, name == node owner)` |
| Owner | `owner` |
| Status | `🔛` (label Active) or `🚫` (Inactive), `role=img` |
| Readers | `readers` or `unavailable` |
| Reached | `external` for a service, else `—`. On Agents and Personal-agent rows it is therefore always `—` |
| Queued / Held | `queued`, plus `at capacity when observed` (`warn`) when `at_bound` |
| Accepted / Dequeued | `in` / `out` |
| Copies out | `out` |
| Deliver-To | count of `subs` |
| Address / Protocol | `addr` / `protocol` |
| Updated | relative `at` |

Numeric columns are right-aligned (`class=num`); every cell carries
`data-label` for the phone layout.

### States

| State | Shown |
|---|---|
| Category empty, Personal records of this kind exist (not on `/personal`) | card `personal-elsewhere`: `No shared {agents|services|queues|pub/sub topics}`, `{n} Personal {noun}{ is| s are} under the Personal tab, which this list omits.` with the link, then the kind blurb |
| Category empty | card `No {agents|services|queues|pub/sub topics|Personal records} yet`, a one-sentence kind blurb and `Register a …` (`/personal` offers `/agents/new`) |
| Filters match nothing | card `No records match these filters` / `Change the active filters above or clear filters.` |
| `/personal` with rows | `<p class=muted>`: daemon Owner `This per-owner view contains only Personal records visible through your normal access; it is not a node-wide inventory.`; others `Your Personal records.` |
| Inactive records | listed with `🚫` under All and Inactive |
| Pagination | 25 rows. Links keep `state`, `q`, `readers`, `work`, `sort`, `scope`, `owner` — **not `kind`**, so page 2 of `/personal?kind=queue` shows every kind |
| Bus unavailable, refusal | problem page ([shell](shell.md#problem-page)) |

"Category" is the page's records after `scope` and `kind`, before the toolbar
filters. `scope=my` with nothing owned therefore reads as `No agents yet`.

### Controls by role

| Control | Who |
|---|---|
| Everything on the page | every signed-in caller; nothing is disabled |
| Owner chooser | daemon Owner on `/personal` only |
| Register links | everyone; the daemon decides at submission |

### Forms

| Form | Action | Fields | Submit |
|---|---|---|---|
| Toolbar | `GET {this path}` | `q`, hidden filters, `sort` | reloads the list |
| Owner chooser | `GET` (current path) | `<select name=owner>`: `""` All visible owners, then each owner of a visible Personal record, sorted; hidden `state`, `readers`, `work`, `q`, `kind`, `sort`; button `Choose owner` | reloads `/personal` |

### Links out

Rows → detail with `return`; tabs; filter links; Clear filters (keeps only
`scope=my` and the daemon Owner's `owner`, **drops `kind`**); Register; paging.

## Register

`/agents/new` · `/services/new` · `/queues/new` · `/pubsub/new`

### Route

No parameters. `/channels/new` → `301` to `/pubsub/new` when `kind=pubsub`,
else `/queues/new`.

### Access

Any signed-in caller gets the form; the caller becomes the Owner. Signed out:
`401` sign-in form.

### Daemon calls

`GET /status` only (plus `/identity`).

### Content

| Order | Element |
|---|---|
| Title | `Register agent` · `Register service` · `Register queue` · `Register pub/sub topic` |
| 1 | `Back to {Agents|Services|Queues|PubSub}` → the list |
| 2 | `<h1>` kind glyph + title |
| 3 | form error summary (after a refusal) |
| 4 | `<form id=form-create class="editor-card task-card" method=post action=/service>`, button = the title |
| 5 | help popovers for the fields shown (below) |

### Forms

Hidden: `action=create`, `kind={agent|service|queue|pubsub}`.

| `name` | Control | Kinds | Required | Prefill | Placeholder / values | Meaning |
|---|---|---|---|---|---|---|
| `name` | `<input id=create-name>` | all | yes | empty | `name@realm` | routing identity; unchangeable. Help: `The routing identity callers use. It cannot be changed afterwards.` |
| `descr` | input | all | no | empty | `What this {Noun} is for` | `Shown first in the registry.` |
| `addr` | input | service | yes | empty | `host:port, a path, or a URL` | where the caller reaches it |
| `protocol` | input | service | yes | empty | `https, postgresql, smtp` | a hint; the daemon checks nothing |
| `secret` | `<textarea rows=4 autocomplete=off spellcheck=false>` | agent, service | no | **always empty** | `PGPASSWORD=...` | opaque bytes; `Optional. Leave empty to register without one.` ⓘ `form-secret-help` |
| `ttl` | input | agent, queue | no | empty | `default` | message retention |
| `bound` | `<input type=number min=0>` | agent, queue | no | `0` | whole number; `0 selects the default.` | capacity |
| `overflow` | `<select>` | agent, queue | – | `strict` | `strict` Refuse · `ring` Drop oldest | full-inbox policy |
| `subs` | `<input>` | agent, queue | no | empty | `#agent@realm, queue@realm or topic@realm` | one-slot Deliver-To route. ⓘ `form-route-help` |
| `subs` | `<textarea rows=5>` | pubsub | no | empty | `#agent@realm⏎queue@realm⏎@group` | Deliver-To list, one per line; empty reaches nobody. ⓘ `form-deliver-help` |
| `allow` | `<textarea rows=5>` | all | no | empty | `#agent@realm⏎user@realm⏎@group⏎@owner⏎*` | allow list, one term per line; on PubSub who may publish. ⓘ `form-access-help` |
| `personal` | checkbox in fieldset `Classification` | all | no | off | `on` | Personal classification |
| `edit_personal` | hidden `1` | all | – | – | – | emitted, ignored by create |

**Submit** (`POST /service`, see [below](#post-service)): `POST /register`
(header `If-None-Match: *`) with body `name`, `kind`, `addr`, `protocol`,
`descr`, `allow` (whitespace-split), `personal`, `subs` (whitespace-split),
`ttl`, `overflow`, `bound`. Then, for agent and service with a non-empty
secret, `POST /secret {name, secret}` with CRLF turned into LF.

| Outcome | Answer |
|---|---|
| Success | `303` → `{detail path}?name={name}` |
| `bound` not an integer | `400`, form back, field `bound`: `Queue capacity must be a whole number.` (no call made) |
| `kind` not a valid kind | `400`, form back (`⚙️ Register channel`), `Choose a valid record kind.` |
| `412` (name taken) | form back, field `name`, daemon message `that name is already registered` |
| `400`/`404`/`409`/`429` | form back with the daemon message; a message naming a submitted line of `subs` or `allow` marks that field and is prefixed `Line N: ` |
| `403` naming a line | form back as above |
| Other `403`, `500`, `503`, transport | problem page |
| Record made, secret refused | `502` "not understood" page: `The {noun} was registered and its secret was not stored: {reason} Set it with: agent-bus secret {name} '...'` |

Kept on refusal: `name`, `descr`, `kind`, `addr`, `protocol`, `personal`,
`allow`, `subs`, `ttl`, `bound`, `overflow`. Never kept: `secret`. The one
refused field gets `aria-invalid="true"` and `aria-describedby=create-error`,
pointing at `<p class=warn id=create-error>` under the grid; the rest carry
`aria-invalid="false"`. Missing service address or an agent name without `#`
is shown without marking a field.

<details><summary>Kind-dependent help text</summary>

- Allow list hint: PubSub `Who may publish here. Who receives is the Deliver-To list above.`; while Personal is ticked `While Personal: the Owner, the Owner’s own agents, @owner` (`or @agent` for an agent) `, one per line.`; else `One identity, group, @owner, or * per line.`
- Classification: `Puts this {Noun} in its Owner’s Personal view instead of the shared pages; delivery is unchanged.` and the Personal-list restriction sentence.
- Route: `Empty keeps messages here. One destination: a message sent to this {Noun} moves there instead, and the destination’s allow list must list this {Noun} itself.`
- Secret tooltip: `Only the agent itself` (agent) or `Only the allow list` (service) `reads them back, and no page ever shows them again.`
- Popovers `form-access-help`, `form-route-help`, `form-deliver-help`, `form-secret-help` restate the allow, route, Deliver-To and secret rules as bullets.

</details>

### Links out

Back to the list.

## Record detail

`/agent` · `/service` · `/queue` · `/pubsub/topic`

### Route

| Parameter | Meaning |
|---|---|
| `name` | the record, exact. Any of the four paths renders any kind: the path is not checked against the kind and never redirects |
| `return` | local list URL for Back; used only when its path is the record's own list (`/personal` for a Personal record), else that list ([shell](shell.md#return-addresses)) |

| Old address | Answer |
|---|---|
| `/channel?name=` | `301` to `/queue?…` or `/pubsub/topic?…` (query kept) when `/lookup` finds a queue or topic; otherwise this page served at `/channel` (inactive record, other kind, or `401`/`404`) |

### Access

| Role | Gets |
|---|---|
| Owner, Maintainer, record itself, daemon Owner (`can_manage`) | full page with Deactivate…, route/settings links, Deliver-To Remove buttons, Settings card and Danger Zone |
| Other visible caller | read-only page, description and `You can view this record; its owner and assigned maintainers can manage it.` |
| Stranger, missing name | `404 No such name` |
| Inactive record | [inactive view](#inactive-record-view) instead |

### Daemon calls

| Call | Gives | On failure |
|---|---|---|
| `GET /status` | you, roles, `up` | problem page |
| `GET /inactive` | if `name` is among them, the inactive view | treated as not found |
| `GET /lookup?name=` | the record and its flags | problem page |
| `GET /groups` | loaded, not displayed | problem page |
| `GET /users` | the Owner's profile (`kind` = directory user, `name == owner`) for its photo and link | Owner shown as plain `<code>` |
| `GET /activity?name=` | 144 ten-minute slots | the Activity card says `Activity unavailable: {reason}` |

### Content

| Order | Element | Kinds |
|---|---|---|
| Title | `{Noun} {name}` | all |
| 1 | `Back to records` → `return` | all |
| 2 | `<h1>` kind glyph, name, and `· Personal` (muted) when Personal | all |
| 3 | form error summary (only after a refused inline POST) | all |
| 4 | meta row: kind pill; `Owner:` photo or initial + link `/user?name=` (when the profile is visible) or `<code>owner</code>`; `👮 Maintainers: …` when any; `Delivery: one at a time` (agent, queue, user) or `a copy to each subscriber` (pubsub) | all |
| 5 | **Status** card: `🔛 Active`, `Deactivate…` for managers | all |
| 6a | **Where it is** card: Address, Protocol, Secret (`secret_sha` or `none`); `Updated {at} · Config {config_sha or —}` | service |
| 6b | **Policy** card: Reached (`external` if `protocol` set, else `this bus`), Queue bound (`bound` or `default`), Retention (`ttl` or `none`), When full | all but service, **pubsub included** |
| 6c | **Queue & counters** card: Readers, Held now (+ at-capacity warning), Oldest held (`—` when empty), Accepted, Dequeued, Dropped / expired; `Updated · Config` line | all but service, **pubsub included** |
| 7 | **Activity** card: `Scope: <code>name</code> · {start} to {end}, ten-minute slots.`; an SVG chart of the non-zero series (Accepted, Dequeued, Dropped, Expired, Refused) on one scale with totals, or `All five series: 0 in the last day.`; `Zero all day: …`; link `View all activity and slot values` → `/activity?name=` | all |
| 8 | **Deliver-To route** section `id=route`: `Forwards to {dest}` with route state, or `No route. A message sent here stays in this {Noun}’s own queue.`; managers get `Set a route` / `Replace or clear the route` `in the settings` → edit page | agent, queue (user: no) |
| 9 | `<details><summary>Deliver-To</summary>`: each `subs` entry as `<code>`; managers get a Remove button per entry; `Nobody. A publication here reaches no inbox.` when empty; `Take my inbox off this list` when your own name is literally on it | pubsub |
| 10 | managers: **Settings** card, `Edit settings` → edit page with `return`; then red `Danger Zone` → `/service-danger?name=` (no `return`) | all |
| 10' | others: `<p>{descr}</p>` and the read-only sentence | all |

Route state (`route_allowed`, only for agent and queue with exactly one
`subs`): `true` → `Route allowed now.`; `false` → warning `Configured, but
{dest} does not allow {name} now — or {dest} is inactive or no longer
registered. …`; absent → `Whether this route is usable now was not reported.`
Then a fixed sentence that the destination checks the record itself, not the
sender or Owner.

The face computes nothing but formatting, the owner match against `/users`,
`onList(subs, you)` and the activity chart geometry.

### States

| State | Shown |
|---|---|
| Owner has no visible profile | plain `<code>owner</code>` |
| No maintainers | the Maintainers item is omitted |
| Activity refused or unreachable | `Activity unavailable: {daemon message or "the daemon did not answer"}` |
| Activity empty | `The daemon answered no activity.` |
| Bus unavailable | `502` problem page |

### Controls by role

| Control | Manager | Viewer |
|---|---|---|
| `Deactivate…` (`GET /service-deactivate`, hidden `name`) | shown, not for a user inbox | omitted |
| Route settings link, Settings card, Danger Zone | shown | omitted |
| Deliver-To Remove | shown | omitted |
| Take my inbox off | anyone whose name is on `subs` | same |
| Description paragraph | **not shown** | shown |

### Forms

| Form | Action | Fields | Daemon call | Result |
|---|---|---|---|---|
| Deactivate… | `GET /service-deactivate` | hidden `name` | – | confirmation page |
| Remove recipient | `POST /service` | hidden `name`, `subscriber`; button `name=action value=remove-subscriber` | `POST /subscriber/remove {channel, subscriber}` | `303` → detail |
| Take my inbox off | `POST /service` | hidden `name`; button `action=unsubscribe` | `POST /subscribe {Channel, Off: true}` | `303` → detail |

Refusals of both go to the problem page.

### Links out

Back; Owner → `/user?name=`; `/activity?name=`; `{detail}/edit?name=…&return=…`;
`/service-danger?name=`.

## Inactive record view

Served by the detail addresses when `name` is in `/inactive`.

| Part | Detail |
|---|---|
| Visibility | daemon Owner: every inactive record; others: those their ACL, ownership or maintenance admits |
| Calls | `/status`, `/inactive` only |
| Title | `{Noun} {name}` |
| Content | `Back to records`; `<h1>` glyph, name, badge `INACTIVE`; kind pill; `Owner: <code>` (never linked); Status card `🚫 Inactive` with three bullets, the second `Its queued work is kept: {queued} held when observed.` (service: without the count) |
| Manager (not a user inbox) | `<form method=post action=/service>` hidden `name`, button `name=action value=reactivate` → `POST /manage {name, status: "active"}` → `303` detail |
| Others | `Its owner and assigned maintainers can reactivate it.` |
| Absent | settings, Danger Zone, activity, counters |

Every other page for an inactive name (settings, deactivate, Danger Zone)
answers `404 No such name`.

## Settings

`/agent/edit` · `/service/edit` · `/queue/edit` · `/pubsub/topic/edit`

### Route

| Parameter | Meaning |
|---|---|
| `name` | the record; path not checked against kind |
| `return` | as on detail; carried into the form |

`/channel/edit?name=` → `301` to `/queue/edit` or `/pubsub/topic/edit` for a
visible queue or topic, else this page served in place.

### Access

| Role | Gets |
|---|---|
| Owner, daemon Owner | every field editable |
| Maintainer, record itself | Personal and Maintainers **disabled** (`Shown for reference: …`); everything else editable |
| Other visible caller | `403 Not yours to see`, message `only the owner or an assigned Maintainer can change this record's settings` |
| Stranger, inactive | `404` |

### Daemon calls

The same as detail: `/status`, `/lookup`, `/groups`, `/users`, `/activity`
(the last three unused on this page).

### Content

| Order | Element |
|---|---|
| Title | `Edit {name}` |
| 1 | `Back to {name}` → detail with `return` |
| 2 | `<h1>` glyph + `Edit {Noun} {name}` |
| 3 | error summary |
| 4 | `<form id=form-save class="editor-card task-card" method=post action=/service>`, button `Save settings` |
| 5 | help popovers; red `Danger Zone` link |

### Forms

Hidden: `action=save`, `return` (when set), `name`. The field set is the
registration set with these changes.

| `name` | Change from registration |
|---|---|
| `name` | hidden, not editable |
| `descr`, `addr`, `protocol`, `ttl`, `bound`, `overflow`, `subs`, `allow` | prefilled from the record (`bound` as plain digits; lists one per line) |
| `secret` | empty; `Leave empty to keep the stored credential. Anything here replaces it.` |
| `edit_subs=1` | hidden, with `subs` (agent, queue, pubsub): says the form carried `subs` |
| `edit_allow=1` | hidden, with `allow` (not user) |
| `personal` | checked from the record; `disabled` unless `can_transfer` |
| `edit_personal=1` | hidden, only when `can_transfer` |
| `maintainers` | `<textarea rows=5>`, one per line, prefilled; `disabled` unless `can_transfer`. Hint `One user, group, agent or service per line; @owner is ACL-only.` |
| `edit_sharing=1` | hidden, only when `can_transfer` |
| user inbox | only `descr`, `ttl`, `bound`, `overflow` |

**Submit** makes one `POST /manage` whose body carries `name`, `descr`
always, and only what the form showed:

| Sent when | Body fields |
|---|---|
| `addr` or `protocol` present | `addr`, `protocol` |
| any of `bound`, `ttl`, `overflow` present | `bound` (must parse, else `400` field `bound`, no call), `ttl`, `overflow` |
| `edit_allow` present | `allow` (whitespace-split; empty clears) |
| `edit_subs` present | `subs` (whitespace-split; empty clears) |
| `edit_sharing` present | `maintainers` |
| `edit_personal` present | `personal` (`on` → true, absent → false) |

Then, if `secret` is non-empty, `POST /secret {name, secret}` (CRLF → LF).

| Outcome | Answer |
|---|---|
| Success | `303` → `{detail path}?name=` of the fresh `/lookup` (the `return` is dropped) |
| Line-attributable refusal (any code) | form back, field `subs`, `allow` or `maintainers`, `Line N: ` prefix; when a term is typed in two lists both are named (`Allow list line 2 and Maintainers line 1: …`) |
| `400`/`404`/`409`/`412`/`429` mentioning `maintainer` | form back, field `maintainers` |
| Other preserved codes | form back, no field |
| `403`, `500`, `503`, transport | problem page |
| Settings saved, secret refused | `502`: `The settings were saved and the secret was not stored: {reason} Set it with: agent-bus secret {name} '...'` |

Kept on refusal: `descr`, `addr`, `protocol`, `ttl`, `overflow`, `bound`,
`allow`, `edit_allow`, `subs`, `edit_subs`, `personal`, `maintainers`,
`edit_sharing`, `edit_personal`. Never: `secret`. Error element id
`save-error`; summary anchor `#form-save`. The record itself is re-read, so a
record that vanished answers `404`.

### Links out

Back to detail; Danger Zone.

## Deactivate

`GET /service-deactivate?name=`

| Part | Detail |
|---|---|
| Access | `can_manage` and not a user inbox; else `403` `only the owner or an assigned Maintainer can deactivate this record`. Stranger or inactive: `404` |
| Calls | `/status`, `/lookup` |
| Title | `Confirm deactivation · {name}` |
| Content | `Back to {name}`; `<h1>` warning mark `Confirm deactivation`; card `Deactivate <code>name</code>?` and bullets: hidden from listings; `Queued work is kept ({queued} held now), and running processes are not stopped.`; reactivation from the Inactive view |
| Form | `POST /service`, hidden `name`, button `name=action value=deactivate` (red), `Cancel` → detail |
| Submit | `/lookup` first (to keep the section), then `POST /manage {name, status: "inactive"}` → `303` `{detail}?name=` (now the inactive view) |
| `return` | read and ignored |

## Danger Zone

`GET /service-danger?name=`

### Access

| Role | Gets |
|---|---|
| Owner, daemon Owner | configuration (agent, service, group), transfer, removal (not group) |
| Maintainer, record itself | configuration and removal; **transfer omitted** |
| Other visible | `403` `only the owner or an assigned Maintainer can manage this record` |
| Stranger, inactive | `404` |

### Content and forms

Calls: `/status`, `/lookup`. Title `Danger Zone · {name}`. `Back to {name}`,
`<h1>` warning mark `Danger Zone · {name}`, error summary, then:

| Section | Shown when | Form |
|---|---|---|
| Replace configuration | kind is agent, service or group | `id=form-configure`, `POST /service`, hidden `name`, `action=configure`; `<textarea name=config rows=6 cols=60 required autocomplete=off>` always empty; `Existing private configuration and a refused replacement are never displayed.`; button `Replace configuration` |
| Transfer ownership | `can_transfer`, name ≠ `@administrators`, name ≠ owner | `id=form-transfer`, `POST /service-confirm`, hidden `name`, `action=transfer`; `<input name=owner required>` (label `New owner`, refilled after a transfer refusal); note on credentials; button `Continue to confirmation` |
| Remove registration | kind ≠ group | `POST /service-confirm`, hidden `name`, `action=delete`; `No registration, no access: …`; button `Continue to confirmation` |
| Group note | kind = group | `A group is retired by emptying its members, never removed: …` |

**Configure submit:** `config` must be valid JSON, else `400` form back, field
`config` (`configure-error`): `Configuration must be valid JSON. The
submitted configuration is not shown again.` Then `POST /configure {Name,
Config}` → `303` detail. A preserved daemon refusal re-renders this page with
the message and no field; `403` etc. go to the problem page.

## POST /service-confirm

Renders a confirmation for `action` = `transfer` or `delete`. Requires the
exact Origin ([shell](shell.md#origin-checks)); `/status`, then `/lookup` of
`name` (`404` when hidden).

| Action | Check | Page |
|---|---|---|
| `transfer` | `can_transfer` and name ≠ owner, else `403` `only this record's owner or the daemon owner can transfer it`. Empty `owner` → Danger Zone `400`, field `owner`, `New owner is required.` | title `Confirm ownership transfer · {name}`; `Transfer <code>name</code> from <code>owner</code> to <code>new</code>?`; form `POST /service` hidden `action=transfer`, `name`, `owner`, `expected_owner`, `confirmed=1`; button `Transfer ownership` |
| `delete` | `can_manage`, else `403` | title `Confirm removal · {name}`; `Remove <code>name</code>? It currently holds {queued} messages and has {readers} outstanding reads.`; form hidden `action=delete`, `name`, `expected_owner`, `expected_queued`, `expected_readers` (`unavailable` when absent), `confirmed=1`; button `Remove registration` |
| other | – | `400` "not understood": `That confirmation action is not available.` |

Both pages link `Back to the Danger Zone`. Status `200`.

## POST /service

Every record write. Requires the exact Origin (else `403 text/plain
same-origin form required`, also when Origin is absent), a session (else the
`401` sign-in form, `return` = Referer), and a body the face can parse (else
`400` "not understood"). Then `/status`.

| `action` | Fields | Daemon calls | Success `303` |
|---|---|---|---|
| `create` | see [Register](#register) | `POST /register`, then `POST /secret` | detail |
| `save` | see [Settings](#settings) | `POST /manage`, then `POST /secret` | detail |
| `configure` | `name`, `config` | `POST /configure {Name, Config}` | detail |
| `deactivate` | `name` | `GET /lookup`, `POST /manage {name, status:"inactive"}` | detail |
| `reactivate` | `name` | `POST /manage {name, status:"active"}`, `GET /lookup` | detail |
| `transfer` | `name`, `owner`, `expected_owner`, `confirmed` | `GET /lookup`, `POST /manage {name, owner}`, `GET /lookup` | detail if still visible, else the record's list |
| `delete` | `name`, `expected_owner`, `expected_queued`, `expected_readers`, `confirmed` | `GET /lookup`, `POST /unregister {name}` | the record's list (`/personal` when Personal) |
| `unsubscribe` | `name` | `POST /subscribe {Channel, Off}` | detail |
| `remove-subscriber` | `name`, `subscriber` | `POST /subscriber/remove {channel, subscriber}` | detail |
| other | – | – | `400` `That service action is not available.` |

"Detail" is `{detail path of the kind}?name={name}`, the kind taken from the
daemon's answer, not a hidden field. When no lookup succeeds the path falls
back to `/service`.

**Stale-confirmation guard.** Only when `confirmed=1` on `transfer` or
`delete`: the record is re-read; a different owner than `expected_owner`,
lost `can_transfer` (transfer), or a different `queued`, reader snapshot or
lost `can_manage` (delete) answers `409 The conditions changed` with the link
`Review the current Danger Zone`. Without `confirmed=1` the action runs
directly.

**Refusal routing.** `create` and `save` return their form as above.
`configure` and `transfer` return the Danger Zone (transfer keeps `owner`,
marks no field). `delete`, `deactivate`, `reactivate` and the Deliver-To
actions go to the problem page (a user inbox removal: `409 That was refused`).

## Differences from the older specs

| Older spec | Code |
|---|---|
| forms.md, pages.md: Personal and Maintainers editable by the Owner or a **daemon administrator** | `can_transfer`: Owner or daemon Owner. The disabled Maintainers hint also says "daemon administrator"; the Personal hint says "daemon Owner" |
| forms.md: Replace configuration on Service, Queue, PubSub topic; "Configuration stays on channels" | agent, service and group only |
| forms.md: Enable / Disable; Transfer and Remove on Service, Queue, PubSub | Deactivate / Reactivate and Transfer / Remove on every kind, agents included; Remove is offered even for a user inbox |
| forms.md, 05-discovery: two-value choices are radio buttons | Overflow is a `<select>` |
| forms.md: Remove a recipient returns to the subscribers section | returns to the top of the detail page |
| forms.md remaining work: add and remove both offered on Deliver-To | only per-entry Remove for managers and a self-removal when on the list |
| pages.md: detail has Identity, Queue, Access, Subscribers, Configuration sections with section-local editors and an Edit heading action | cards Status, Where it is / Policy + Queue & counters, Activity, route or Deliver-To, then one Settings page. **The allow list is shown nowhere but the settings form**; subscribers are unlinked |
| pages.md: drop empty ConfigSHA; drop reader state on pub/sub | `Config —` is always shown; PubSub detail shows Readers, Held, bound, retention and overflow |
| pages.md: Enabled / Disabled labels | Active / Inactive with 🔛 / 🚫 |
| pages.md: search over description and name | also owner |
| pages.md: Kind column only on mixed lists; In/Out move to detail | Type column on every list; Accepted and Dequeued are list columns |
| pages.md: help beside the name states `user@realm` | placeholder `name@realm` on every kind, including agents (`#` required) |
| 05-discovery: `/channel` and `/channel/edit` serve any channel | they `301` a visible queue or topic to its own address |
| 05-discovery: Services, Queues and PubSub show only totals | Services also has My |
| 05-discovery: PubSub filters no readers | the `readers` parameter still filters `/pubsub`, with no control shown |
| 05-discovery: the final submission rechecks the confirmed facts | only when `confirmed=1`; a direct POST skips the guard |
| shell paging: links keep every filter | on `/personal`, paging, row `return` and Clear filters drop `kind` |

<details><summary>Behaviour worth revisiting in the rewrite</summary>

- `kind` lost on `/personal` paging, Back and Clear filters.
- A manager never sees the record's description on its detail page.
- The Agents `Reached` column is always `—`.
- Detail addresses render any kind without redirecting to the canonical one.
- A user inbox is reachable at `/queue?name=`, offers Remove and links Back to `/queues`, where it is never listed.
- Settings loads `/groups`, `/users` and `/activity` it does not use; a `/groups` failure fails detail pages that do not show groups.
- A save's `return` is kept in the form and dropped on success.
- A non-owner's Personal search always costs a `303`, from the hidden `owner`.
- A daemon-refused transfer marks no field.
- `<form>` inside `<p>` on the Deliver-To list is invalid HTML.
- Refusal text can name what a hidden record is: a route to an invisible service says `a service takes no delivered copy`.

</details>
