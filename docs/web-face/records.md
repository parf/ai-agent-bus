# Web face record pages

📌 **TL;DR:** Every record page of the web face (`agent-bus-web`, `src/web`):
the four kind lists and their Personal filter, four registration forms, the
detail, settings, deactivation and Danger Zone pages, the legacy `/channel`
addresses and the two POST handlers. Each page gives its parameters, access by
role, daemon calls, content, states, controls, form fields and links. The
frame, problem pages, form recovery, paging and `return` rules are in
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
| Signed out | no session | every page here answers `401` with the sign-in form, `return` = the requested URI |

`can_manage` = Owner, record itself, Maintainer or daemon Owner.
`can_transfer` = Owner or daemon Owner. A suspended caller gets neither.

### Kinds

| Fact | 👾 agent | 📡 service | 📮 queue | 📣 pubsub | 👤 user inbox |
|---|---|---|---|---|---|
| List | `/agents` | `/services` | `/queues` | `/pubsub` | none (Users) |
| Detail | `/agent` | `/service` | `/queue` | `/pubsub/topic` | `/queue` |
| Settings | `/agent/edit` | `/service/edit` | `/queue/edit` | `/pubsub/topic/edit` | `/queue/edit` |
| Register | `/agents/new` | `/services/new` | `/queues/new` | `/pubsub/new` | none |
| Noun | Agent | Service | Queue | PubSub | User |
| Name | `#name[@realm]`; the daemon refuses one without `#` | `name[@realm]` | `name[@realm]` | `name[@realm]` | user name |
| `addr`, `protocol` | – | required | – | – | – |
| `secret` field | yes | yes | – | – | – |
| `ttl`, `bound`, `overflow` | yes | – | yes | – (each copy lives by its recipient's TTL) | yes (settings only) |
| `subs` | one-slot Deliver-To route | – | one-slot route | Deliver-To list | – |
| `allow` | yes | yes | yes (who may send) | yes (who may **publish**) | – |
| Personal | checkbox | checkbox | checkbox | checkbox | always; no control |
| `maintainers` | settings only | settings only | settings only | settings only | – |
| Replace configuration | yes | yes | – | – | – |
| Deactivate / Reactivate | yes | yes | yes | yes | – (follows its user) |
| Transfer | yes | yes | yes | yes | – |
| Remove | yes | yes | yes | yes | – |

A group (👥) is a record too, but lives on [Groups](people.md#groups-groups).
It can reach `/service-danger`: configuration only, no transfer for
`@administrators`, a self-owned name or a `@<user>/…` name, and no removal.

### Record fields used

Every page reads records as JSON from `/ls`, `/inactive` or `/lookup`.

| JSON field | Shown as |
|---|---|
| `name` | routing name, in `<code>`; the row and page key |
| `kind` | kind mark and pill, e.g. `👾 Agent` |
| `descr` | description; the list's bold first line |
| `owner` | Owner, `<code>` |
| `maintainers[]` | `👮 Maintainers: a, b` (joined `, `), omitted when empty |
| `personal` | `· Personal` beside the name; the record lives on its list's Personal filter |
| `status` | `inactive` or anything else (active). The face also stamps `inactive` on every `/inactive` row |
| `at` | Updated. List: relative (`now`, `5m ago`, `3h ago`, `12d ago`, `Jan 2`, `Jan 2, 2006`), the full time as its title; detail: `2006-01-02 15:04:05` in the offset the daemon sent; zero or absent → `—` |
| `addr`, `protocol` | Address, Protocol |
| `ttl`, `bound`, `overflow` | Retention, Queue bound, When full (`ring` → drop the oldest, else refuse) |
| `allow[]` | only in the settings form. **No page displays it** |
| `subs[]` | route destination (first entry) or Deliver-To list; list count on PubSub |
| `secret_sha`, `config_sha` | digests; the bytes never |
| `can_manage`, `can_transfer` | which controls render |
| `route_allowed` | route state: `true` allowed, `false` refused, absent unreported |
| `readers` | number; absent → `unavailable` |
| `queued`, `in`, `out`, `dropped`, `expired`, `oldest`, `at_bound` | counters. `oldest` is a Go duration string (`1m3s`) |

Numbers use comma grouping; tables keep `0`.

## Lists

`/agents` · `/services` · `/queues` · `/pubsub`, each with `?personal=1`

One handler per kind. The page filters, sorts and pages one `/ls` +
`/inactive` answer; the daemon has no paging. **Personal is a filter, not a
page**: `personal=1` shows the kind's Personal records instead of its shared
ones. The sidebar is the kind axis, so there is no Kind filter.

### Route

| Parameter | Values | Meaning | Default |
|---|---|---|---|
| `personal` | `1` | the Personal records of this kind: your own, or every visible one for the daemon Owner | shared records |
| `q` | text, trimmed | case-insensitive substring of `name`, `descr` or `owner` | none |
| `scope` | `my` | only records whose `owner` is you. Agents and Services only, and not with `personal` | all |
| `state` | `active`, `inactive` | status filter | both |
| `readers` | `present` (readers > 0), `none` (= 0), `unavailable` (absent) | reader filter. Ignored on `/services`; **honoured but not offered** on `/pubsub` | any |
| `work` | `held` | `queued > 0`. Ignored on `/services` and `/pubsub` | any |
| `sort` | `updated` (newest `at` first), `queued` (most work first; on `/pubsub` the topic's `in`) | order; ties and default by `name` ascending. `queued` is dropped on `/services` | name |
| `page` | integer | 1-based, clamped to 1…last | 1 |
| `owner` | a name | with `personal=1`, daemon Owner only: one owner's records. From anyone else: `303` to the same URL without `owner` | all visible owners |

Any other value of a parameter reads as its default.

| Old address | Answer |
|---|---|
| `/channels?…` | `301` to `/pubsub` when `kind=pubsub`, else `/queues`; `kind` removed, rest of the query kept. No sign-in needed for the redirect |

### Access

| Role | Gets |
|---|---|
| Any signed-in caller | the page, with the records the daemon lets them see |
| Daemon Owner | every record; with `personal=1` the owner chooser and every owner's Personal records |
| Everyone else | with `personal=1` only their own Personal records (`Owned by <you>.`) |
| Signed out | `401` sign-in form |

### Daemon calls

| Call | Gives |
|---|---|
| `GET /status` | `you`, `daemon_owner` |
| `GET /ls` | active visible records |
| `GET /inactive` | inactive visible records |

Either list call failing fails the page ([shell](shell.md#problem-page)).

### Content

| Order | Element |
|---|---|
| Title | `Agents` · `Services` · `Queues` · `PubSub`, with ` · Personal` under the filter, then ` · agent-bus`. Nav: the matching section, kind links keeping the filter |
| 1 | Page head: the kind mark (a lock under the filter), the title, an `ⓘ` popover `service-views-help` (the kind, Personal, tab counts, Status, Readers, Accepted/Dequeued), and the one Register action: `Register {kind}` → `{kind}/new`, or `Register Personal {kind}` → `{kind}/new?personal=1` |
| 2 | Tabs: All · My (Agents and Services) · Personal, with counts |
| 3 | With `personal=1` and the daemon Owner: the owner chooser |
| 4 | Toolbar: search and sort; then the Status, Readers and Queue filters |
| 5 | Empty-state card, **or** the Personal note, the results line and the table |
| 6 | `<nav aria-label="Record pages">` with `Previous page` / `Next page` |

**Tabs.** Counts are computed by the face over the visible records of this
kind, before toolbar filters: All counts shared records, My the shared ones you
own, Personal the Personal ones (yours, or all for the daemon Owner). Tabs keep
`state`, `q`, `readers`, `work` and `sort`; Personal also keeps the daemon
Owner's `owner`. My has class `my-view`, Personal `personal-view` with a lock.

**Toolbar.** `<form class=record-search method=get action={this path}>`.

| Control | Detail |
|---|---|
| Search | `<input id=record-query type=search name=q>`, visually hidden label `Search records`, placeholder `Search by name, owner, or description` (`Search by name or description` when the Owner column is hidden) |
| Hidden | `personal`, `scope`, `state`, `readers`, `work`, `owner`, each only when set |
| Sort | `<select id=record-sort name=sort data-submit-on-change>`: `""` Name (A–Z), `updated` Recently updated, `queued` Queued (high–low) — `Accepted (high–low)` on PubSub, absent on Services |
| Status filter | All · Active · Inactive |
| Readers filter | not on Services or PubSub: All · Reading now · No reader now · Unavailable |
| Queue filter | not on Services or PubSub: All · Holding work |

Filter links drop `page` and keep the other filters.

**Table.** Results line `Showing {start}–{end} of {matched} matching records,
caller-visible on this page and not a count of this node.` plus `Clear filters`
when `q`, `state`, `readers`, `work` or `sort` is set. A list shows only columns
that tell its rows apart: never a kind column, and on a Personal list no Owner
unless the daemon Owner sees several owners.

| Page | Columns |
|---|---|
| Agents | Agent · Owner · Status · Readers · Queued · Accepted · Dequeued · Updated |
| Services | Service · Owner · Status · Address · Protocol · Updated |
| Queues | Queue · Owner · Status · Readers · Held · Accepted · Dequeued · Updated |
| PubSub | PubSub · Owner · Status · Accepted · Copies out · Deliver-To · Updated |

| Cell | Source |
|---|---|
| Name | kind mark and link `{detail path}?name={name}&return={this list URL with page}`; `descr` in bold over `name`, or `name` alone. Row class `owned-record` when `owner == you`, `inactive-record` when inactive |
| Owner | `owner` |
| Status | pill `Active` or `Inactive` |
| Readers | `readers` or `unavailable` |
| Queued / Held | `queued`, plus the badge `at capacity when observed` when `at_bound` |
| Accepted / Dequeued | `in` / `out` |
| Copies out | `out` |
| Deliver-To | count of `subs` |
| Address / Protocol | `addr` / `protocol` pill, or `—` |
| Updated | relative `at` |

Numeric columns are right-aligned; every cell carries `data-label` for the
phone layout.

### States

| State | Shown |
|---|---|
| Shared list empty, Personal records of this kind exist | card `No shared {agents|services|queues|pub/sub topics}`, `{n} Personal {kind}{ is|s are} under the Personal tab, which this list omits.`, the kind blurb, and `Open the Personal tab` |
| Category empty | card `No {agents|…} yet` (`No Personal {…} yet`) and the kind blurb (under the filter: `Personal records belong to their Owner's own view instead of the shared lists.`) |
| Filters match nothing | card `No records match these filters` / `Change the active filters above or clear filters.` and `Clear filters` |
| Personal list with rows | daemon Owner: `Only Personal records visible through your normal access; not a node-wide inventory.`; others: `Owned by <code>you</code>.` |
| Inactive records | listed with `Inactive` under All and Inactive |
| Pagination | 25 rows. Links keep every parameter |
| Bus unavailable, refusal | problem page ([shell](shell.md#problem-page)) |

"Category" is the page's records after `personal`, `scope` and `owner`, before
the toolbar filters.

### Controls by role

| Control | Who |
|---|---|
| Everything on the page | every signed-in caller; nothing is disabled |
| Owner chooser | daemon Owner with `personal=1` only |
| Register action | everyone; the daemon decides at submission |

### Forms

| Form | Action | Fields | Submit |
|---|---|---|---|
| Toolbar | `GET {this path}` | `q`, hidden filters, `sort` | reloads the list |
| Owner chooser | `GET` (current path) | `<select id=owner-select name=owner data-submit-on-change>`: `""` All visible owners, then each owner of a visible Personal record, sorted; hidden `personal=1`, `state`, `readers`, `work`, `q`, `sort` | reloads the list |

### Links out

Rows → detail with `return`; tabs; filter links; Clear filters (keeps
`personal`, `scope` and the daemon Owner's `owner`); Register; paging.

## Register

`/agents/new` · `/services/new` · `/queues/new` · `/pubsub/new`

### Route

`personal=1` starts the form Personal, and Back and Cancel return to the
Personal list. `/channels/new` → `301` to `/pubsub/new` when `kind=pubsub`,
else `/queues/new`.

### Access

Any signed-in caller gets the form; the caller becomes the Owner. Signed out:
`401` sign-in form.

### Daemon calls

`GET /status` only.

### Content

| Order | Element |
|---|---|
| Title | `Register agent` · `Register service` · `Register queue` · `Register pub/sub topic` |
| 1 | `Back to {Agents|Services|Queues|PubSub}` (`Back to Personal …`) → the list |
| 2 | `<h1>` kind mark + title; sub line `You become its Owner.` and the kind blurb |
| 3 | form error summary (after a refusal) |
| 4 | `<form id=form-create method=post action=/service>`, button = the title, `Cancel` |
| 5 | help popovers for the fields shown (below) |

### Forms

Hidden: `action=create`, `kind={agent|service|queue|pubsub}`,
`from_personal=1` when started from a Personal list, `edit_personal=1`.

| `name` | Control | Kinds | Required | Prefill | Placeholder / values | Meaning |
|---|---|---|---|---|---|---|
| `name` | `<input id=create-name>` | all | yes | empty | `#name@realm` (agent, hint `An agent's name starts with #.`), else `name@realm` | routing identity; unchangeable. Help: `The routing identity callers use. It cannot be changed afterwards.` |
| `descr` | input | all | no | empty | `What this {Noun} is for` | `Shown first in the registry.` |
| `addr` | input | service | yes | empty | `host:port, a path, or a URL` | where a caller reaches it |
| `protocol` | input | service | yes | empty | `https, postgresql, smtp` | a hint; the daemon checks nothing |
| `secret` | `<textarea rows=4 autocomplete=off spellcheck=false>` | agent, service | no | **always empty** | `PGPASSWORD=...` | opaque bytes; `Optional. Leave empty to register without one.` ⓘ |
| `ttl` | input | agent, queue | no | empty | `default` | message retention, e.g. `1h` or `7d` |
| `bound` | `<input type=number min=0>` | agent, queue | no | `0` | whole number; `0 selects the default.` | capacity |
| `overflow` | `<select>` | agent, queue | – | `strict` | `strict` Refuse new messages · `ring` Drop the oldest | full-inbox policy |
| `subs` | `<input>` | agent, queue | no | empty | `#agent@realm, queue@realm or topic@realm` | one-slot Deliver-To route ⓘ |
| `subs` | numbered line-list textarea | pubsub | no | empty | `#agent@realm⏎queue@realm⏎@group` | Deliver-To list, one per line; empty reaches nobody ⓘ |
| `allow` | numbered line-list textarea; label `Allow list`, `Who may send` (queue), `Who may publish` (pubsub) | all | no | empty | `#agent@realm⏎user@realm⏎@group⏎@owner⏎*` | one term per line ⓘ |
| `personal` | checkbox in fieldset `Classification` | all | no | off (on from a Personal list) | `on` | Personal classification |

**Submit** (`POST /service`, see [below](#post-service)): `POST /register`
(header `If-None-Match: *`) with body `name`, `kind`, `descr`, `allow`
(whitespace-split), `personal`, `subs` (whitespace-split); `addr`, `protocol`
for a service; `ttl`, `overflow`, `bound` for an agent or queue. Then, with a
non-empty secret, `POST /secret {name, secret}` with CRLF turned into LF.

| Outcome | Answer |
|---|---|
| Success | `303` → the record's detail, flash `Registered.` |
| `bound` not a whole number | `400`, form back, field `bound`: `Queue capacity must be a whole number.` (no call made) |
| `kind` not a valid kind | `400` "not understood" page: `Choose a valid record kind.` |
| `412` (name taken) | form back, field `name`, daemon message |
| `400`/`404`/`409`/`429` | form back with the daemon message |
| A message naming a submitted line of `subs` or `allow` (any code but `412`) | form back, that field marked, the message prefixed `Line N: ` |
| Other `403`, `500`, `503`, transport | problem page |
| Record made, secret refused | `502` saved-in-part page: `The {Noun} was registered and its secret was not stored: {reason} Set it with: agent-bus secret {name} '...'` |

Kept on refusal: `name`, `descr`, `kind`, `addr`, `protocol`, `personal`,
`allow`, `subs`, `ttl`, `bound`, `overflow`, `from_personal`. Never kept:
`secret`. The refused field gets `aria-invalid="true"` and
`aria-describedby=create-error`.

<details><summary>Kind-dependent help text</summary>

- Allow list hint: PubSub `Who may publish here. Who receives is the Deliver-To list above.`; while Personal is ticked `While Personal: the Owner, the Owner’s own agents, @owner` (`or @agent` for an agent) `, one per line.`; else `One identity, group, @owner, or * per line.`
- Classification: `Puts this {Noun} in its Owner’s Personal view instead of the shared pages; delivery is unchanged.` and the Personal-list restriction sentence.
- Route: `Empty keeps messages here. One destination: a message sent to this {Noun} moves there instead, and the destination’s allow list must list this {Noun} itself.`
- Secret tooltip: `Only the agent itself` (agent) or `Only the allow list` (service) `reads them back, and no page ever shows them again.`
- Popovers restate the allow, route, Deliver-To and secret rules as bullets.

</details>

### Links out

Back to the list; Cancel.

## Record detail

`/agent` · `/service` · `/queue` · `/pubsub/topic`

### Route

| Parameter | Meaning |
|---|---|
| `name` | the record, exact |
| `return` | local list URL for Back; used only when its path is the record's own list, else that list ([shell](shell.md#return-addresses)) |
| `range`, `at` | the Activity card's range, as on [/activity](node.md#activity) |

| Address | Answer |
|---|---|
| a detail path not the kind's own | `302` to the kind's detail path, query kept; a group → `/group?name=` |
| `/channel?name=` | `301` to the record's own detail path (query kept); an inactive record's view is served at `/channel` |

### Access

| Role | Gets |
|---|---|
| Owner, Maintainer, record itself, daemon Owner (`can_manage`) | full page with Edit settings, Deactivate…, the route settings link, Deliver-To Remove buttons and the Danger Zone link |
| Other visible caller | read-only page and `You can view this record; its owner and assigned maintainers can manage it.` |
| Stranger, missing name | `404 No such name` |
| Inactive record | [inactive view](#inactive-record-view) instead |

### Daemon calls

| Call | Gives | On failure |
|---|---|---|
| `GET /status` | you, roles, the range | problem page |
| `GET /inactive` | if `name` is among them, the inactive view | treated as none |
| `GET /lookup?name=` | the record and its flags | problem page |
| `GET /users` | the Owner's profile, for its photo and link | Owner shown as plain `<code>` |
| `GET /activity?name=` or `/activity/days?name=&from=&to=` | the Activity card's range | the card says `Activity unavailable: {reason}` |

### Content

| Order | Element | Kinds |
|---|---|---|
| Title | `{Noun} {name}` | all |
| 1 | `Back to records` → `return` | all |
| 2 | `<h1>` kind mark, name with a copy button, and `· Personal` (muted) when Personal; `Edit settings` → settings with `return` for managers | all |
| 3 | meta row: kind pill; `Owner` chip with photo, linked `/user?name=` when the profile is visible, else plain; `👮 Maintainers: …` when any; `Delivery: one at a time` or `a copy to each subscriber` (pubsub) | all |
| 4 | the description, when set; the read-only sentence for non-managers | all |
| 5 | **Activity** card `id=activity`: Day · Week · Month, Prev, Next and Today (links keep `name` and `return`, `#activity`), the range title, the day ribbon on a Day, the chart; `Open in Activity` → `/activity?name=` with the range | all |
| 6 | **Deliver-To route** card `id=route`: `{name} → {dest}` with route state and the sentence that the destination checks this record itself, not the sender or Owner; or `No route. A message sent here stays in this {Noun}’s own queue.`; managers get `Set a route` / `Replace or clear the route` `in the settings` | agent, queue |
| 7 | **Deliver-To** card `id=subscribers`, with the count: each `subs` entry as `<code>`, a Remove button per entry for managers; `Nobody. A publication here reaches no inbox.` when empty; `Take my inbox off this list` when your own name is on it | pubsub |
| 8 | **Status** card: `Active` pill, `Deactivate…` for managers (not a user inbox); `Updated {at} · Config {config_sha or —}` | all |
| 9 | **Where it is** card: Address, Protocol, Secret (`secret_sha` or `none`) | service |
| 10 | **Policy** card: Reached (`external` if `protocol` set, else `this bus`), Queue bound (`bound` or `default`), Retention (`ttl` or `none`), When full | agent, queue, user inbox |
| 11 | **Queue & counters** card: Readers, Held now (+ at-capacity badge), Oldest held (`—` when empty), Accepted, Dequeued, Dropped / expired | all but service |
| 12 | managers (not a user inbox): red `Danger Zone` link → `/service-danger?name=`, naming configuration, transfer (with `can_transfer`) and removal | all |

Route state (`route_allowed`, only for agent and queue with exactly one
`subs`): `true` → `Route allowed now.`; `false` → warning `Configured, but
{dest} does not allow {name} now — or {dest} is inactive or no longer
registered. Add {name} to its allow list, or clear this route.`; absent →
`Whether this route is usable now was not reported.`

The face computes nothing but formatting, the owner match against `/users` and
whether your name is on `subs`.

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
| Edit settings, route settings link, Danger Zone | shown (Danger Zone not for a user inbox) | omitted |
| `Deactivate…` (`GET /service-deactivate`, hidden `name`) | shown, not for a user inbox | omitted |
| Deliver-To Remove | shown | omitted |
| Take my inbox off | anyone whose name is on `subs` | same |

### Forms

| Form | Action | Fields | Daemon call | Result |
|---|---|---|---|---|
| Deactivate… | `GET /service-deactivate` | hidden `name` | – | confirmation page |
| Remove recipient | `POST /service` | hidden `name`, `subscriber`; button `name=action value=remove-subscriber` | `POST /subscriber/remove {channel, subscriber}` | `303` → detail `#subscribers` |
| Take my inbox off | `POST /service` | hidden `name`; button `action=unsubscribe` | `POST /subscribe {channel, off: true}` | `303` → detail |

Refusals of both go to the problem page.

### Links out

Back; Owner → `/user?name=`; Activity range links and `/activity?name=`;
`{detail}/edit?name=…&return=…`; `/service-danger?name=`.

## Inactive record view

Served by the detail addresses when `name` is in `/inactive`.

| Part | Detail |
|---|---|
| Visibility | daemon Owner: every inactive record; others: those their ACL, ownership or maintenance admits |
| Calls | `/status`, `/inactive` only |
| Title | `{Noun} {name}` |
| Content | `Back to records`; `<h1>` kind mark, name, badge `INACTIVE`; kind pill; `Owner` chip, never linked; Status card with the `Inactive` pill and three bullets, the second `Its queued work is kept: {queued} held when observed.` (service: without the count) |
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
| `name` | the record; a group → `302` to `/group/edit?name=` |
| `return` | as on detail; carried into the form |

`/channel/edit?name=` → `301` to the record's own settings path; an inactive
record is `404`.

### Access

| Role | Gets |
|---|---|
| Owner, daemon Owner | every field editable |
| Maintainer, record itself | Personal and Maintainers **disabled** (`Shown for reference: …`); everything else editable |
| Other visible caller | `403 Not yours to see`, message `only the owner or an assigned Maintainer can change this record's settings` |
| Stranger, inactive | `404` |

### Daemon calls

`/status`, `/inactive`, `/lookup`.

### Content

| Order | Element |
|---|---|
| Title | `Edit {name}` |
| 1 | `Back to {name}` → detail with `return` |
| 2 | `<h1>` kind mark + `Edit {Noun} {name}` |
| 3 | error summary |
| 4 | `<form id=form-save method=post action=/service>`, button `Save settings`, `Cancel` |
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
| `edit_allow=1` | hidden, with `allow` (not a user inbox) |
| `personal` | checked from the record; `disabled` unless `can_transfer` |
| `edit_personal=1` | hidden, only when `can_transfer` |
| `maintainers` | line-list textarea, one per line, prefilled; `disabled` unless `can_transfer`. Hint `One user, group, agent or service per line; @owner is ACL-only.` |
| `edit_sharing=1` | hidden, only when `can_transfer` |
| user inbox | only `descr`, `ttl`, `bound`, `overflow` |

**Submit** makes one `POST /manage` whose body carries `name`, `descr`
always, and only what the form showed:

| Sent when | Body fields |
|---|---|
| `addr` or `protocol` present | `addr`, `protocol` |
| any of `bound`, `ttl`, `overflow` present | `bound` (must be a whole number, else `400` field `bound`, no call), `ttl`, `overflow` |
| `edit_allow` present | `allow` (whitespace-split; empty clears) |
| `edit_subs` present | `subs` (whitespace-split; empty clears) |
| `edit_sharing` present | `maintainers` |
| `edit_personal` present | `personal` (`on` → true, absent → false) |

Then, if `secret` is non-empty, `POST /secret {name, secret}` (CRLF → LF).

| Outcome | Answer |
|---|---|
| Success | `303` → detail of the fresh `/lookup`, `return` kept, flash `Settings saved.` |
| Line-attributable refusal (any code) | form back, field `subs`, `allow` or `maintainers`, `Line N: ` prefix; when a term is typed in two lists both are named (`Allow list line 2 and Maintainers line 1: …`) |
| `400`/`404`/`409`/`412`/`429` mentioning `maintainer` | form back, field `maintainers` |
| Other preserved codes | form back, no field |
| `403`, `500`, `503`, transport | problem page |
| Settings saved, secret refused | `502`: `The settings were saved and its secret was not stored: {reason} Set it with: agent-bus secret {name} '...'` |

Kept on refusal: `descr`, `addr`, `protocol`, `ttl`, `overflow`, `bound`,
`allow`, `edit_allow`, `subs`, `edit_subs`, `personal`, `edit_personal`,
`maintainers`, `edit_sharing`, `return`. Never: `secret`. Error element id
`save-error`; summary anchor `#form-save`. The record is re-read, so a record
that vanished answers `404`.

### Links out

Back to detail; Cancel; Danger Zone.

## Deactivate

`GET /service-deactivate?name=`

| Part | Detail |
|---|---|
| Access | `can_manage` and not a user inbox; else `403` `only the owner or an assigned Maintainer can deactivate this record`. Stranger or inactive: `404` |
| Calls | `/status`, `/inactive`, `/lookup` |
| Title | `Confirm deactivation · {name}` |
| Content | `Back to {name}`; `<h1>` warning mark `Confirm deactivation`; card `Deactivate <code>name</code>?` and bullets: it disappears from listings and refuses use; `Queued work is kept ({queued} held now), and running processes are not stopped.`; reactivation from its Inactive view |
| Form | `POST /service`, hidden `name`, button `name=action value=deactivate` `Deactivate {name}` (red), `Cancel` → detail |
| Submit | the record is re-read, then `POST /manage {name, status: "inactive"}` → `303` detail (now the inactive view) |

## Danger Zone

`GET /service-danger?name=`

### Access

| Role | Gets |
|---|---|
| Owner, daemon Owner | configuration (agent, service, group), transfer, removal (not group) |
| Maintainer, record itself | configuration and removal; **transfer omitted** |
| Other visible | `403` `only the owner or an assigned Maintainer can manage this record` |
| User inbox | `403` `a user's inbox has no configuration, transfer or removal; it follows its user` |
| Stranger, inactive | `404` |

### Content and forms

Calls: `/status`, `/inactive`, `/lookup`. Title `Danger Zone · {name}`.
`Back to {name}`, `<h1>` `Danger Zone · {name}`, the sub line `Each action here
changes who controls this record or what it holds. Transfer and removal ask once
more before they happen.`, error summary, then:

| Section | Shown when | Form |
|---|---|---|
| Replace configuration | kind is agent, service or group | `id=form-configure`, `POST /service`, hidden `name`, `action=configure`; `<textarea name=config rows=6 required autocomplete=off>` always empty; `Existing private configuration and a refused replacement are never displayed.`; button `Replace configuration` |
| Transfer ownership | `can_transfer`, name ≠ `@administrators`, name ≠ owner, not a `@<user>/…` group | `id=form-transfer`, `POST /service-confirm`, hidden `name`, `action=transfer`; `<input name=owner required>` (label `New owner`, refilled after a refusal); note on credentials; button `Continue to confirmation` |
| Transfer note | a `@<user>/…` group | `A group named for its owner (@<user>/…) is never transferred. Its new owner creates their own instead.` |
| Remove registration | kind is not group | `POST /service-confirm`, hidden `name`, `action=delete`; `No registration, no access: …`; button `Continue to confirmation` |
| Group note | kind = group | `A group is retired by emptying its members, never removed: …` |

**Configure submit:** `config` must be valid JSON, else `400` form back, field
`config`: `Configuration must be valid JSON. The submitted configuration is not
shown again.` Then `POST /configure {Name, Config}` → `303` detail (a group's
page for a group). A preserved daemon refusal re-renders this page with the
message and no field; `403` and the rest go to the problem page.

## POST /service-confirm

Renders a confirmation for `action` = `transfer` or `delete`, after
`/status`, `/inactive` and `/lookup` of `name` (`404` when hidden or inactive).

| Action | Check | Page |
|---|---|---|
| `transfer` | `can_transfer` and name ≠ owner, else `403` `only this record's owner or the daemon owner can transfer it`. Empty `owner` → Danger Zone `400`, field `owner`, `New owner is required.` | title `Confirm ownership transfer · {name}`; `Transfer <code>name</code> from <code>owner</code> to <code>new</code>?`; form `POST /service` hidden `action=transfer`, `name`, `owner`, `expected_owner`, `confirmed=1`; button `Transfer ownership`, `Cancel` |
| `delete` | `can_manage`, not a group or user inbox, else `403` | title `Confirm removal · {name}`; `Remove <code>name</code>?`, `It currently holds {queued} messages and has {readers} outstanding reads.`; form hidden `action=delete`, `name`, `expected_owner`, `expected_queued`, `expected_readers` (`unavailable` when absent), `confirmed=1`; button `Remove registration`, `Cancel` |
| other | – | `400` "not understood": `That confirmation action is not available.` |

Both pages link `Back to the Danger Zone`. Status `200`.

## POST /service

Every record write. [Origin](shell.md#origin-checks) and session are checked
first, then `/status`. Each success is a `303` with a flash message.

| `action` | Fields | Daemon calls | Success `303` |
|---|---|---|---|
| `create` | see [Register](#register) | `POST /register`, then `POST /secret` | detail |
| `save` | see [Settings](#settings) | `POST /manage`, then `POST /secret` | detail |
| `configure` | `name`, `config` | `POST /configure {Name, Config}` | detail |
| `deactivate` | `name` | re-read, `POST /manage {name, status:"inactive"}` | detail |
| `reactivate` | `name` | `POST /manage {name, status:"active"}`, `GET /lookup` | detail |
| `transfer` | `name`, `owner`, `expected_owner`, `confirmed` | re-read, `POST /manage {name, owner}`, `GET /lookup` | detail if still visible, else the record's list |
| `delete` | `name`, `expected_owner`, `expected_queued`, `expected_readers`, `confirmed` | re-read, `POST /unregister {name}` | the record's list (`?personal=1` when Personal) |
| `unsubscribe` | `name` | `POST /subscribe {channel, off}` | detail |
| `remove-subscriber` | `name`, `subscriber` | `POST /subscriber/remove {channel, subscriber}` | detail `#subscribers` |
| other | – | – | `400` `That service action is not available.` |

"Detail" is the kind's detail path with `?name=`, the kind taken from the
daemon's answer, not a hidden field.

**Stale-confirmation guard, always.** A `transfer` or `delete` without
`confirmed=1` is refused `400` before any call (`A transfer is confirmed on its
confirmation page first. Start it from the Danger Zone.`, and the same for a
removal). With it, the record is re-read: a different owner than
`expected_owner`, lost `can_transfer` or a self-owned name (transfer), or a
different `queued`, reader count or lost `can_manage` (delete) answers
`409 The conditions changed` with the link `Review the current Danger Zone`.

**Refusal routing.** `create` and `save` return their form as above.
`configure` and `transfer` return the Danger Zone; a refused transfer keeps and
marks `owner`. `delete`, `deactivate`, `reactivate` and the Deliver-To actions
go to the problem page.

## Differences from the older specs

History: the Go face's differences from older specs, and what was worth
revisiting in the rewrite, are in
[web-go-face-differences § records](../../Plans/R0.8-MVP/done/web-go-face-differences.md#recordsmd).
