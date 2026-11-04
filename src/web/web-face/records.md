# Web face record pages

📌 **TL;DR:** Every record page of the web face (`agent-bus-web`, `src/web`):
the five kind lists and their Personal filter, five registration forms, the
detail, settings, deactivation and Danger Zone pages, and the two POST handlers. Each page gives its parameters, access by
position, daemon calls, content, states, controls, form fields and links. The
frame, problem pages, form recovery, paging and `return` rules are in
[shell](shell.md#problem-page).

## Shared facts

### Positions

The face never decides authority. It reads the daemon's per-caller flags on
each record, and the daemon rechecks every write.

| Position | How the face knows | What the daemon grants on records |
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

| Fact | 👾 agent | 📡 service | 📮 queue | 📣 pubsub | 📚 resource | 👤 user inbox |
|---|---|---|---|---|---|---|
| List | `/agents` | `/services` | `/queues` | `/pubsub` | `/resources` | none (Users) |
| Detail | `/agent` | `/service` | `/queue` | `/pubsub/channel` | `/resource` | `/queue` |
| Settings | `/agent/edit` | `/service/edit` | `/queue/edit` | `/pubsub/channel/edit` | `/resource/edit` | `/queue/edit` |
| Register | `/agents/new` | `/services/new` | `/queues/new` | `/pubsub/new` | `/resources/new` | none |
| Noun | Agent | Service | Queue | PubSub | Resource (🧩 a template) | User |
| Name | `#name[@realm]`; the daemon refuses one without `#` | `name[@realm]` | `name[@realm]` | `name[@realm]` | `name[@realm]` | user name |
| `addr`, `protocol` | – | required | – | – | – | – |
| `uri`, `template`, `source`, `mime`, `title` | – | – | – | – | register only; a card changes by registering it again ([Resource records](../../../docs/03-records-resource.md#what-a-resource-is)) | – |
| `secret` field | yes | yes | – | – | – | – |
| `ttl`, `bound`, `overflow` | yes | – | yes | – (each copy lives by its recipient's TTL) | – | yes (settings only) |
| `subs` | one-slot Deliver-To route | – | one-slot route | Deliver-To list | – | – |
| `allow` | yes | yes | yes (who may send) | yes (who may **publish**) | yes (who may see and read) | – |
| Personal | `personal` tag | `personal` tag | `personal` tag | `personal` tag | `personal` tag | always; no control |
| `maintainers` | settings only | settings only | settings only | settings only | settings only | – |
| Replace configuration | yes | yes | – | – | – | – |
| Deactivate / Reactivate | yes | yes | yes | yes | yes | – (follows its user) |
| Transfer | yes | yes | yes | yes | yes | – |
| Remove | yes | yes | yes | yes | yes | – |

A 📚 resource's list shows URI, Source and MIME type columns and no reader or queue filters; its detail page shows the card, never the data behind it.

A group (👥) is a record too, but lives on [Groups](people.md#groups-groups).
It can reach the normal edit form for configuration; no transfer for
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
| `tags[]` | classification badges; `personal` selects the Personal view |
| `status` | `inactive` or anything else (active). The face also stamps `inactive` on every `/inactive` row |
| `at` | Updated. Listing and detail: relative (`now`, `5m ago`, `3h ago`, `12d ago`, `Jan 2`, `Jan 2, 2006`), exact time in the title; zero or absent → blank |
| `last_used` | Listing and detail: relative last use, exact timestamp in the title; absent → blank in the listing and no detail field |
| `addr`, `protocol` | Address, Protocol |
| `ttl`, `bound`, `overflow` | Retention, Queue bound, When full (`ring` → drop the oldest, else refuse) |
| `allow[]` | editable in Settings; non-empty grants appear in Status & Policy |
| `subs[]` | route destination (first entry) or Deliver-To list; list count on PubSub |
| `secret_sha`, `config_sha` | authorized digests; Configuration is fetched separately for managers, Secret bytes never appear |
| `can_manage`, `can_transfer` | which controls render |
| `route_allowed` | route state: `true` allowed, `false` refused, absent unreported |
| `readers` | number; absent → `unavailable` |
| `queued`, `in`, `out`, `dropped`, `expired`, `oldest`, `at_bound` | counters. `oldest` is a Go duration string (`1m3s`) |

Numbers use comma grouping; tables keep `0`.

## Lists

`/agents` · `/services` · `/queues` · `/pubsub` · `/resources`, each with `?tag=personal`

One handler per kind. The page filters, sorts and pages one `/ls` +
`/inactive` answer; the daemon has no paging. **Personal is a filter, not a
page**: `tag=personal` shows the kind's Personal records instead of its shared
ones. The sidebar is the kind axis, so there is no Kind filter.

### Route

| Parameter | Values | Meaning | Default |
|---|---|---|---|
| `tag` | a normalized label | records carrying the tag; `personal` shows your own Personal records, or every visible one for the daemon Owner | shared records |
| `q` | text, trimmed | case-insensitive substring of `name`, `descr` or `owner` | none |
| `scope` | `my` | only records whose `owner` is you. Agents and Services only, and not with `tag=personal` | all |
| `state` | `active`, `inactive` | status filter | active |
| `readers` | `present` (readers > 0), `none` (= 0), `unavailable` (absent) | reader filter. Ignored on `/services` and `/resources`; **honoured but not offered** on `/pubsub` | any |
| `work` | `held` | `queued > 0`. Ignored on `/services`, `/resources` and `/pubsub` | any |
| `sort` | `updated` (newest `at` first), `last_used` (Agents only, most recently used first; never-used last), `queued` (most work first; on `/pubsub` the 📣's `in`) | order; ties and default by `name` ascending, except Agents put those with outstanding reads first. `queued` is dropped on `/services` and `/resources` | name |
| `page` | integer | 1-based, clamped to 1…last | 1 |
| `owner` | a name | with `tag=personal`, daemon Owner only: one owner's records. From anyone else: `303` to the same URL without `owner` | all visible owners |

Unsupported tag syntax is refused. Other filter values outside their listed choices use their defaults.

### Access

| Position | Gets |
|---|---|
| Any signed-in caller | the page, with the records the daemon lets them see |
| Daemon Owner | every record; with `tag=personal` the owner chooser and every owner's Personal records |
| Everyone else | with `tag=personal` only their own Personal records (`Owned by <you>.`) |
| Signed out | `401` sign-in form |

### Daemon calls

| Call | Gives |
|---|---|
| `GET /status` | `you`, `daemon_owner` |
| `GET /ls` | active visible records |
| `GET /inactive` | inactive visible records |
| `#bus` call `list` (Agents only) | private event summaries, as the signed-in visitor; no entry bodies |

Either registry list call failing fails the page ([shell](shell.md#problem-page)).

- Agents load the event summaries alongside the other page reads, with one `#bus` call. Visible summaries add a recent-alert mark and the retained-warning count beside the agent name.
- A `forbidden` answer or HTTP `403` from the summary call leaves the listing visible with no event marks or unavailable notice.
- HTTP `503`, a transport failure or an invalid summary answer leaves the listing visible and shows `Event summaries unavailable.` No private entry text is loaded here.

### Content

| Order | Element |
|---|---|
| Title | `Agents` · `Services` · `Queues` · `PubSub`, with ` · Personal` under the filter, then ` · agent-bus`. Nav: the matching section, kind links keeping the filter |
| 1 | Page head: the kind mark (a lock under the filter), the title, an `ⓘ` popover `record-views-help` (the kind, Personal, tab counts, Status, Readers, Messages on Agents), and the one Register action: `Register {kind}` → `{kind}/new`, or `Register Personal {kind}` → `{kind}/new?tag=personal` |
| 2 | Tabs: All · My (Agents and Services), then classification tags with counts, including Personal |
| 3 | With `tag=personal` and the daemon Owner: the owner chooser |
| 4 | Toolbar: search and sort; then the Status, Readers and Queue filters |
| 5 | Empty-state card, **or** the Personal note, the results line and the table |
| 6 | `<nav aria-label="Record pages">` with `Previous page` / `Next page` |

**Tabs.** Counts are computed before pagination, after search, state, reader
and work filters. All counts shared records, My the shared ones you own; tag
counts also respect the selected ownership scope. Personal records are limited
to yours, or all visible owners for the daemon Owner. Tabs keep
`state`, `q`, `readers`, `work` and `sort`; Personal also keeps the daemon
Owner's `owner`. My has class `my-view`, Personal `personal-view` with a lock.

**Toolbar.** `<form class=record-search method=get action={this path}>`.

| Control | Detail |
|---|---|
| Search | `<input id=record-query type=search name=q>`, visually hidden label `Search records`, placeholder `Search by name, owner, or description` (`Search by name or description` when the Owner column is hidden) |
| Hidden | `tag`, `scope`, `state`, `readers`, `work`, `owner`, each only when set |
| Sort | `<select id=record-sort name=sort data-submit-on-change>`: `""` Name (A–Z), `updated` Recently updated, `last_used` Recently used (Agents only), `queued` Queued (high–low) — `Accepted (high–low)` on PubSub, absent on Services |
| Status filter | Active · Inactive |
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
| Agents | Agent · Owner · Readers · Messages · Last used · Updated |
| Services | Service · Owner · Address · Protocol · Updated |
| Queues | Queue · Owner · Readers · Held · Accepted · Dequeued · Updated |
| PubSub | PubSub · Owner · Accepted · Copies out · Deliver-To · Updated |

| Cell | Source |
|---|---|
| Name | kind mark and link `{detail path}?name={name}&return={this list URL with page}`; `descr` in bold over `name`, or `name` alone. Row class `owned-record` when `owner == you`, `inactive-record` when inactive |
| Owner | `owner` |
| Status | pill `Active` or `Inactive` |
| Readers | `readers` or `unavailable`; zero is blank on Agents |
| Messages (Agents) | accepted `in`; non-zero `queued` gets a bold orange `N waiting` chip; `dropped + expired` gets a bold red `N lost` chip with a `Dropped X, expired Y` title. No Dequeued or duplicate Queued column. Zero numeric cells stay empty, with columns retained. |
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
| Shared list empty, Personal records of this kind exist | card `No shared {agents|services|queues|pub/sub channels}`, `{n} Personal {kind}{ is|s are} under the Personal tab, which this list omits.`, the kind blurb, and `Open the Personal tab` |
| Category empty | card `No {agents|…} yet` (`No Personal {…} yet`) and the kind blurb (under the filter: `Personal records belong to their Owner's own view instead of the shared lists.`) |
| Filters match nothing | card `No records match these filters` / `Change the active filters above or clear filters.` and `Clear filters` |
| Personal list with rows | daemon Owner: `Only Personal records visible through your normal access; not a node-wide inventory.`; others: `Owned by <code>you</code>.` |
| Inactive records | listed with `Inactive` under the Inactive state filter |
| Pagination | 100 rows. Links keep every parameter |
| Bus unavailable, refusal | problem page ([shell](shell.md#problem-page)) |

"Category" is the page's records after `tag`, `scope` and `owner`, before
the toolbar filters.

### Controls by position

| Control | Who |
|---|---|
| Everything on the page | every signed-in caller; nothing is disabled |
| Owner chooser | daemon Owner with `tag=personal` only |
| Register action | everyone; the daemon decides at submission |

### Forms

| Form | Action | Fields | Submit |
|---|---|---|---|
| Toolbar | `GET {this path}` | `q`, hidden filters, `sort` | reloads the list |
| Owner chooser | `GET` (current path) | `<select id=owner-select name=owner data-submit-on-change>`: `""` All visible owners, then each owner of a visible Personal record, sorted; hidden `tag=personal`, `state`, `readers`, `work`, `q`, `sort` | reloads the list |

### Links out

The page also carries its [GitHub documentation links](#documentation-targets).

Rows → detail with `return`; tabs; filter links; Clear filters (keeps
`tag`, `scope` and the daemon Owner's `owner`); Register; paging.

## Register

`/agents/new` · `/services/new` · `/queues/new` · `/pubsub/new` · `/resources/new`

### Route

`tag=personal` starts the form Personal, and Back and Cancel return to the
Personal list. The Tags field starts with `personal`.

### Access

Any signed-in caller gets the form; the caller becomes the Owner. Signed out:
`401` sign-in form.

### Daemon calls

`GET /status` only.

### Content

| Order | Element |
|---|---|
| Title | `Register agent` · `Register service` · `Register queue` · `Register pub/sub channel` |
| 1 | `Back to {Agents|Services|Queues|PubSub}` (`Back to Personal …`) → the list |
| 2 | `<h1>` kind mark + title; sub line: the kind blurb, then `You become its Owner.` |
| 3 | form error summary (after a refusal) |
| 4 | `<form id=form-create method=post action=/record>`, button = the title, `Cancel` |
| 5 | help popovers for the fields shown (below) |

### Forms

Registration and editing use `RecordFields`; record and group forms share the
Maintainers and Tags controls. Tags follows Description; its help explains
Personal classification and the access restrictions it carries. Template sits beside URI; its help explains URI
expansion, file, schema and log examples, and links RFC 6570. Registration links
the latest official MCP Resources specification. Agent register/edit role help
explains the informational supported list, ACL role assignments and reading
message `roles` or `AB_ROLE_*`. ACL help shows a parenthesized multi-role example
and whole-save refusal for generated names. Both hover/focus tooltips show `@dev(deploy, read_only)` directly; clicking
opens the fuller explanation.

Hidden: `action=create`, `kind={agent|service|queue|pubsub|resource}`,
`edit_sharing=1`. The submitted `tags` preserve the Personal classification.

| `name` | Control | Kinds | Required | Prefill | Placeholder / values | Meaning |
|---|---|---|---|---|---|---|
| `name` | `<input id=create-name>` | all | yes | `#@<configured realm>` for agents, `@<configured realm>` for the other kinds; empty if no realm is available | `#name@realm` (agent, hint `An agent's name starts with #.`), else `name@realm` | Complete the name before `@`; change or remove the suggested realm for an explicit identity. A fresh form reads the current status realm; refused forms retain their submitted name, including an empty one. POST never expands or rewrites names. The saved identity is unchangeable. |
| `descr` | input | all | no | empty | `What this {Noun} is for` | `Shown first in the registry.` |
| `addr` | input | service | yes | empty | `host:port, a path, or a URL` | where a caller reaches it |
| `protocol` | input | service | yes | empty | `https, postgresql, smtp` | a hint; the daemon checks nothing |
| `secret` | `<textarea rows=1 autocomplete=off spellcheck=false>` | agent, service | no | **always empty** | `PGPASSWORD=...` | opaque bytes; `Optional. Leave empty to register without one.` ⓘ |
| `ttl` | input | agent, queue | no | empty | `default` | message retention, e.g. `1h` or `7d` |
| `bound` | `<input type=number min=0>` | agent, queue | no | `0` | whole number; `0 selects the default.` | capacity |
| `overflow` | `<select>` | agent, queue | – | `strict` | `strict` Refuse new messages · `ring` Drop the oldest | full-inbox policy |
| `subs` | `<input>` | agent, queue | no | empty | `#agent@realm, queue@realm or pubsub@realm` | one-slot Deliver-To route ⓘ |
| `subs` | numbered line-list textarea | pubsub | no | empty | `#agent@realm⏎queue@realm⏎@group` | Deliver-To list, one per line; empty reaches nobody ⓘ |
| `allow` | numbered line-list textarea; label `Allow list`, `Who may send` (queue), `Who may publish` (pubsub) | all | no | empty | `#agent@realm⏎user@realm⏎@group⏎@owner⏎*` | one term per line ⓘ |
| `tags` | text with help | all | no | `personal` from a Personal list, otherwise empty | comma-separated labels | normalized classification tags |
| `maintainers` | numbered line-list textarea | all | no | empty | `user@realm⏎@group⏎#agent@realm` | initial management grant, shared with the edit form |

**Submit** (`POST /record`, see [below](#post-record)): `POST /register`
(header `If-None-Match: *`) with body `name`, `kind`, `descr`, `allow`
(split outside role parentheses), `maintainers` (split outside role parentheses), `tags`, `subs` (split outside role parentheses); `addr`, `protocol`
for a service; `ttl`, `overflow`, `bound` for an agent or queue. Then, with a
non-empty secret, `POST /secret {name, secret}` with CRLF turned into LF.

| Outcome | Answer |
|---|---|
| Success | `303` → the record's detail, flash `Registered.` |
| `bound` not a whole number | `400`, form back, field `bound`: `Queue capacity must be a whole number.` (no call made) |
| `kind` not a valid kind | `400` "not understood" page: `Choose a valid record kind.` |
| `412` (name taken) | form back, field `name`, daemon message |
| `400`/`404`/`409`/`429` | form back with the daemon message |
| A message naming a submitted line of `subs`, `allow` or `maintainers` (any code but `412`) | form back, that field marked, the message prefixed `Line N: ` |
| Other `403`, `500`, `503`, transport | problem page |
| Record made, secret refused | `502` saved-in-part page: `The {Noun} was registered and its secret was not stored: {reason} Set it with: agent-bus secret {name} '...'` |

Kept on refusal: `name`, `descr`, `kind`, `addr`, `protocol`, `tags`,
`allow`, `maintainers`, `subs`, `ttl`, `bound`, `overflow`. Never kept:
`secret`. The refused field gets `aria-invalid="true"` and
`aria-describedby=create-error`.

<details><summary>Kind-dependent help text</summary>

- Allow list hint: PubSub `Who may publish here. Who receives is the Deliver-To list above.`; when the record carries the `personal` tag `While Personal: the Owner, the Owner’s own agents, @owner` (`or @agent` for an agent) `, one per line.`; else `One identity, group, @owner, or * per line.`
- Tags help: the Owner’s Personal view, delivery unchanged, and the restriction to the Owner and the Owner’s agents.
- Route: `Empty keeps messages here. One destination: a message sent to this {Noun} moves there instead, and the destination’s allow list must list this {Noun} itself.`
- Secret tooltip: `Its Owner, Maintainers and the agent itself` (agent) or `Its Owner and Maintainers` (service) `read them back, and no page ever shows them again.`
- Popovers restate the allow, route, Deliver-To and secret rules as bullets.

</details>

### Links out

Back to the list; Cancel.

## Record detail

`/agent` · `/service` · `/queue` · `/pubsub/channel` · `/resource`

### Route

| Parameter | Meaning |
|---|---|
| `name` | the record, exact |
| `return` | local list URL for Back; used only when its path is the record's own list, `/runner` or `/runner-settings`, else the list ([shell](shell.md#return-addresses)) |
| `range`, `at` | the Activity card's range, as on [/activity](node.md#activity) |
| `events` | `all` expands the agent’s full retained log; any other value shows the recent preview |

| Address | Answer |
|---|---|
| a detail path not the kind's own | `302` to the kind's detail path, query kept; a group → `/group?name=` |

### Access

| Position | Gets |
|---|---|
| Owner, Maintainer, record itself, daemon Owner (`can_manage`) | full page with Edit settings, the route settings link and the Danger Zone link |
| Other visible caller | read-only page and `You can view this record; its owner and assigned maintainers can manage it.` |
| Stranger, missing name | `404 No such name` |
| Inactive record | [inactive view](#inactive-record-view) instead |

The Events card has its own authority check: the agent's Owner and Maintainers,
and the daemon Owner, can read the [private log](../../../docs/agents/bus.md#agent-logs).
Being able to view the record, or being the agent itself, does not grant log access.

### Daemon calls

| Call | Gives | On failure |
|---|---|---|
| `GET /status` | you, positions, the range | problem page |
| `GET /inactive` | if `name` is among them, the inactive view | treated as none |
| `GET /lookup?name=` | the record and its flags | problem page |
| `GET /users` | the Owner's profile, for its photo and link | Owner shown as plain `<code>` |
| `#bus` call `read <agent>` (agents only) | the selected agent’s retained log, as the signed-in visitor; called anew for preview and expand | the Events card shows its own state below |
| `GET /activity?name=` or `/activity/days?name=&from=&to=` | the Activity card's range | the card says `Activity unavailable: {reason}` |

### Content

| Order | Element | Kinds |
|---|---|---|
| Title | `{Noun} {name}` | all |
| 1 | `Back to records` → `return` | all |
| 2 | `<h1>` kind mark, name with a copy button, and `· Personal` (muted) when Personal; `Edit settings` → settings with `return` for managers | all |
| 3 | meta row: kind pill; `Owner` chip with photo, linked `/user?name=` when the profile is visible, else plain; `👮 Maintainers: …` when any; `Delivery: one at a time` or `a copy to each recipient` (pubsub) | all |
| 4 | the description, when set; the read-only sentence for non-managers | all |
| Left, 1 | **Activity** card `id=activity`: Day · Week · Month, Prev, Next and Today (links keep `name` and `return`, `#activity`), the range title, day ribbon and chart; `Open in Activity` keeps the range. A resource shows its Resource card instead. | all |
| Left, 2 | **Events** `id=events`: recent preview or full retained log ([below](#events-card)) | agent |
| Left, 3 | **Queue & counters** card `id=counters`: one Messages summary as on Agents; Readers only when non-zero, Oldest held only while a message is held and the age is known. Empty cards are omitted; no duplicate in/out, held or lost rows | all but service and resource |
| Left, 4 | **Configuration** `id=configuration`: stored JSON only, pretty-printed read-only for managers, with Edit and an Expand control; long JSON scrolls within a capped block. No card when unconfigured | agent, service |
| Left, 5 | One-line subscription notice with **Take my inbox off this list**: only for a non-manager on this pubsub's list | pubsub |
| Right, 1 | **Status & Policy** card `id=status`: state badge beside its title in the header. First row: Updated and Last used side by side, each label inline with its timestamp, using the listing’s relative-time formatter with the exact timestamp in the title; absent times are omitted. Then non-empty Allow and Maintainers as compact name rows, non-default policy rows for agent, queue and user inbox: external reach, a configured non-zero bound or TTL, and ring overflow. Defaults and other kinds omit policy rows | all |
| Right, 3 | **Package settings**: declared Configuration and Secret keys, when any; names and help only for Secret | agent |
| Right, 4 | **Locks** `id=locks` only when held, then **Key-value store** `id=kv` with **Open store** | permitted viewers only |
| Right, 5 | Script, Roles, Where it is; each where applicable. Roles is absent without application-role declarations or assignments. It contains only application roles; ACL and Maintainers stay in Status & Policy. | applicable kinds |
| Right, 6 | managers (not a user inbox): red **Danger Zone** link, naming deactivation, transfer (with `can_transfer`) and removal | all |

Deliver-To cards are absent from the record page. Managers edit the route or
recipient list in Settings. The route check appears under that field for agents
and queues with exactly one `subs`: `true` → `Route allowed now.`; `false` →
`Configured, but {dest} does not allow {name} now — or {dest} is inactive or no
longer registered. Add {name} to its allow list, or clear this route.`; absent →
`Whether this route is usable now was not reported.`

The face formats daemon data, selects the recent event preview, matches the
Owner against `/users` and checks whether your name is on `subs`. Log authority
comes from the daemon, not those page calculations.

### Events card

- The preview shows the three entries with greatest ids across the levels. Each line shows its level, text, last occurrence time and repeat count when repeated. Alerts use the error tone, warnings the warning tone, and notices the plain tone.
- `Expand log` opens `/agent?name=…&events=all#events`. It makes a fresh read, shows every retained entry grouped by level, and offers `Show recent entries`. There is no paging or destructive `pull`/`clean` call.
- Both views say `Showing {shown} of {retained} retained entries.` The expanded view explains that expired or evicted entries are no longer retained; it does not claim to show an all-time history.
- The loader rejects a different agent’s log, inconsistent counts or an incomplete answer as unavailable. The daemon owns [retention, levels and authority](../../../docs/agents/bus.md#agent-logs).

| Read outcome | Events card |
|---|---|
| `forbidden` or HTTP `403` | `Not allowed to read this agent’s events.`; no entries or empty-log claim |
| `not_found` | `No events yet for this agent.` |
| A valid active log with no entries | `No retained events.` with the shown/retained count |
| HTTP `503`, another failure or an invalid answer | `Events unavailable. Try again.` |

The same card and fresh read are used on the inactive agent view. An inactive
agent’s log answers `not_found` to the daemon Owner and `forbidden` to everyone
else, so this view shows only “No events yet for this agent.” or “Not allowed to
read this agent’s events.”, never entries. Other record kinds have no Events card.

### States

| State | Shown |
|---|---|
| Owner has no visible profile | plain `<code>owner</code>` |
| No maintainers | the Maintainers item is omitted |
| Activity refused or unreachable | `Activity unavailable: {daemon message or "the daemon did not answer"}` |
| Activity empty | `The daemon answered no activity.` |
| Bus unavailable | `502` problem page |

### Controls by position

| Control | Manager | Viewer |
|---|---|---|
| Edit settings, route settings link, Danger Zone | shown (Danger Zone not for a user inbox) | omitted |
| Deactivate → `GET /record-deactivate` | in Danger Zone, not for a user inbox or group | omitted |
| Take my inbox off | anyone whose name is on `subs` | same |

### Forms

| Form | Action | Fields | Daemon call | Result |
|---|---|---|---|---|
| Deactivate | `GET /record-deactivate` | `name` in URL, from Danger Zone | – | confirmation page |
| Take my inbox off | `POST /record` | hidden `name`; button `action=unsubscribe` | `POST /subscribe {channel, off: true}` | `303` → detail |

Refusals of both go to the problem page.

### Links out

Back; Owner → `/user?name=`; Activity range links and `/activity?name=`;
`{detail}/edit?name=…&return=…`; `/record-danger?name=`.

## Inactive record view

Served by the detail addresses when `name` is in `/inactive`.

| Part | Detail |
|---|---|
| Visibility | daemon Owner: every inactive record; others: those their ACL, ownership or maintenance admits |
| Calls | `/status`, `/inactive`; agents also request their Events card |
| Title | `{Noun} {name}` |
| Content | `Back to records`; `<h1>` kind mark, name, badge `INACTIVE`; kind pill; `Owner` chip, never linked; Status card with the `Inactive` pill and three bullets, the second `Its queued work is kept: {queued} held when observed.` (service: without the count) |
| Manager (not a user inbox) | `<form method=post action=/record>` hidden `name`, button `name=action value=reactivate` → `POST /manage {name, status: "active"}` → `303` detail |
| Others | `Its owner and assigned maintainers can reactivate it.` |
| Absent | settings, Danger Zone, activity, counters |

Every other page for an inactive name (settings, deactivate, Danger Zone)
answers `404 No such name`.

## Settings

`/agent/edit` · `/service/edit` · `/queue/edit` · `/pubsub/channel/edit` · `/resource/edit`

### Route

| Parameter | Meaning |
|---|---|
| `name` | the record; a group → `302` to `/group/edit?name=` |
| `return` | as on detail; carried into the form |

An inactive record is `404`.

### Access

| Position | Gets |
|---|---|
| Owner, daemon Owner | every field editable |
| Maintainer, record itself | Maintainers **disabled**; Tags editable, with Personal changes restricted by the daemon to its Owner or daemon Owner |
| Other visible caller | `403 Not yours to see`, message `only the owner or an assigned Maintainer can change this record's settings` |
| Stranger, inactive | `404` |

### Daemon calls

`/status`, `/inactive`, `/lookup`.

### Content

| Order | Element |
|---|---|
| Title | `Edit {name}` |
| 1 | `Back to {name}` → detail with `return` |
| 2 | `<h1>` kind mark + `Edit {Noun} {name}`, with **Locks** → `/locks?q=<name>` and **Store** → the record’s KV page; both always shown |
| 3 | error summary |
| 4 | `<form id=form-save method=post action=/record>`, button `Save settings`, `Cancel` |
| 5 | help popovers; red `Danger Zone` link |

### Private configuration

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

Owner and Maintainers see stored **Configuration (JSON)** on the record page
when configured, and can edit it in the editor, pretty-printed. Agent and service editors put it after
Secret; groups use the same field. Only authorized callers fetch it through
`GET /config`; general management rights alone do not grant this read.
Secret (ENV) always starts empty and is never displayed.

All textareas use the shared `rowsFor(value, focused)` helper: empty is one row, content uses its line count capped at 20; focus grows to at least five rows. Blur restores the content height, and the no-JS form keeps its server-rendered content height. Sizing never changes a value, and Secret remains empty. Blank keeps the stored value; nonempty configuration
is validated and sent compact through `POST /configure {name, config}`. Formatting
preserves exact numbers, escaped strings and key order, so the digest matches
CLI input. An open editor saves its displayed configuration, including an older
value. Invalid JSON returns 400 before settings writes; invalid Configuration edits stay in the field for correction. Settings, Secret and configuration remain separate writes with
partial-failure reporting. Configuration is absent from the Danger Zone. The dedicated `action=configure` form is only for `#bus` settings; other records save Configuration through the normal editor.

### Forms

Hidden: `action=save`, `return` (when set), `name`. The field set is the
registration set with these changes.

| `name` | Change from registration |
|---|---|
| `name` | hidden, not editable |
| `descr`, `addr`, `protocol`, `ttl`, `bound`, `overflow`, `subs`, `allow` | prefilled from the record (`bound` as plain digits; lists one per line) |
| `secret` | always empty; the hint says whether a secret is stored and that blank keeps it |
| `edit_subs=1` | hidden, with `subs` (agent, queue, pubsub): says the form carried `subs` |
| `edit_allow=1` | hidden, with `allow` (not a user inbox) |
| `tags` | after Description; comma-separated normalized labels, including Personal classification |
| `maintainers` | line-list textarea, one per line, prefilled; `disabled` unless `can_transfer`. Hint `One user, group, agent or service per line; @owner is ACL-only.` |
| `edit_sharing=1` | hidden, only when `can_transfer` |
| user inbox | only `descr`, `ttl`, `bound`, `overflow` |

**Submit** makes one `POST /manage` whose body carries `name`, `descr`
always, and only what the form showed:

| Sent when | Body fields |
|---|---|
| `addr` or `protocol` present | `addr`, `protocol` |
| any of `bound`, `ttl`, `overflow` present | `bound` (must be a whole number, else `400` field `bound`, no call), `ttl`, `overflow` |
| `edit_allow` present | `allow` (split outside role parentheses; empty clears) |
| `edit_subs` present | `subs` (split outside role parentheses; empty clears) |
| `edit_sharing` present | `maintainers` |
| `tags` present | normalized `tags` array |

Then, if `secret` is non-empty, `POST /secret {name, secret}` (CRLF → LF); non-empty Configuration follows through `POST /configure`, as described in [Private configuration](#private-configuration).

| Outcome | Answer |
|---|---|
| Success | `303` → detail of the fresh `/lookup`, `return` kept, flash `Settings saved.` |
| Line-attributable refusal (any code) | form back, field `subs`, `allow` or `maintainers`, `Line N: ` prefix; when a term is typed in two lists both are named (`Allow list line 2 and Maintainers line 1: …`) |
| `400`/`404`/`409`/`412`/`429` mentioning `maintainer` | form back, field `maintainers` |
| Other preserved codes | form back, no field |
| `403`, `500`, `503`, transport | problem page |
| Settings saved, secret refused | `502`: `The settings were saved and its secret was not stored: {reason} Set it with: agent-bus secret {name} '...'` |

Kept on refusal: `descr`, `addr`, `protocol`, `ttl`, `overflow`, `bound`,
`allow`, `edit_allow`, `subs`, `edit_subs`, `tags`, `roles`, `config`,
`maintainers`, `edit_sharing`, `return`. Never: `secret`. Error element id
`save-error`; summary anchor `#form-save`. The record is re-read, so a record
that vanished answers `404`.

### Links out

Back to detail; Cancel; Danger Zone.

## Deactivate

`GET /record-deactivate?name=`

| Part | Detail |
|---|---|
| Access | `can_manage` and not a user inbox; else `403` `only the owner or an assigned Maintainer can deactivate this record`. Stranger or inactive: `404` |
| Calls | `/status`, `/inactive`, `/lookup` |
| Title | `Confirm deactivation · {name}` |
| Content | `Back to the Danger Zone`; `<h1>` warning mark `Confirm deactivation`; card `Deactivate <code>name</code>?` and bullets: it disappears from listings and refuses use; `Queued work is kept ({queued} held now), and running processes are not stopped.`; reactivation from its Inactive view |
| Form | `POST /record`, hidden `name`, button `name=action value=deactivate` `Deactivate {name}` (red), `Cancel` → Danger Zone |
| Submit | the record is re-read, then `POST /manage {name, status: "inactive"}` → `303` detail (now the inactive view) |

## Danger Zone

`GET /record-danger?name=`

### Access

| Position | Gets |
|---|---|
| Owner, daemon Owner | deactivation (not group), transfer, removal (not group) |
| Maintainer, record itself | deactivation (not group) and removal; **transfer omitted** |
| Other visible | `403` `only the owner or an assigned Maintainer can manage this record` |
| User inbox | `403` `a user's inbox has no configuration, transfer or removal; it follows its user` |
| Stranger, inactive | `404` |

### Content and forms

Calls: `/status`, `/inactive`, `/lookup`. Title `Danger Zone · {name}`.
`Back to {name}`, `<h1>` `Danger Zone · {name}`, the sub line `Deactivation,
transfer and removal ask once more before they happen.`, error summary, then:

| Section | Shown when | Form |
|---|---|---|
| Deactivate | kind is agent, service, queue or pubsub | link to `GET /record-deactivate?name=`, which confirms the action |
| Transfer ownership | `can_transfer`, name ≠ `@administrators`, name ≠ owner, not a `@<user>/…` group | `id=form-transfer`, `POST /record-confirm`, hidden `name`, `action=transfer`; `<input name=owner required>` (label `New owner`, refilled after a refusal); note on credentials; button `Continue to confirmation` |
| Transfer note | a `@<user>/…` group | `A group named for its owner (@<user>/…) is never transferred. Its new owner creates their own instead.` |
| Remove registration | kind is not group | `POST /record-confirm`, hidden `name`, `action=delete`; `No registration, no access: …`; button `Continue to confirmation` |
| Group note | kind = group | `A group is retired by emptying its members, never removed: …` |

## POST /record-confirm

Renders a confirmation for `action` = `transfer` or `delete`, after
`/status`, `/inactive` and `/lookup` of `name` (`404` when hidden or inactive).

| Action | Check | Page |
|---|---|---|
| `transfer` | `can_transfer` and name ≠ owner, else `403` `only this record's owner or the daemon owner can transfer it`. Empty `owner` → Danger Zone `400`, field `owner`, `New owner is required.` | title `Confirm ownership transfer · {name}`; `Transfer <code>name</code> from <code>owner</code> to <code>new</code>?`; form `POST /record` hidden `action=transfer`, `name`, `owner`, `expected_owner`, `confirmed=1`; button `Transfer ownership`, `Cancel` |
| `delete` | `can_manage`, not a group or user inbox, else `403` | title `Confirm removal · {name}`; `Remove <code>name</code>?`, `It currently holds {queued} messages and has {readers} outstanding reads.`; form hidden `action=delete`, `name`, `expected_owner`, `expected_queued`, `expected_readers` (`unavailable` when absent), `confirmed=1`; button `Remove registration`, `Cancel` |
| other | – | `400` "not understood": `That confirmation action is not available.` |

Both pages link `Back to the Danger Zone`. Status `200`.

## POST /record

Every record write. [Origin](shell.md#origin-checks) and session are checked
first, then `/status`. Each success is a `303` with a flash message.

| `action` | Fields | Daemon calls | Success `303` |
|---|---|---|---|
| `create` | see [Register](#register) | `POST /register`, then `POST /secret` | detail |
| `save` | see [Settings](#settings) | `POST /manage`, then submitted Secret and Configuration through `/secret` and `/configure` | detail |
| `configure` | `name=#bus`, `config` | `POST /configure {name, config}` | `/agent/edit?name=%23bus` |
| `deactivate` | `name` | re-read, `POST /manage {name, status:"inactive"}` | detail |
| `reactivate` | `name` | `POST /manage {name, status:"active"}`, `GET /lookup` | detail |
| `transfer` | `name`, `owner`, `expected_owner`, `confirmed` | re-read, `POST /manage {name, owner}`, `GET /lookup` | detail if still visible, else the record's list |
| `delete` | `name`, `expected_owner`, `expected_queued`, `expected_readers`, `confirmed` | re-read, `POST /unregister {name}` | the record's list (`?tag=personal` when Personal) |
| `unsubscribe` | `name` | `POST /subscribe {channel, off}` | detail |
| other | – | – | `400` `That record action is not available.` |

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
`configure` is restricted to `#bus` settings; `transfer` returns the Danger Zone; a refused transfer keeps and
marks `owner`. `delete`, `deactivate`, `reactivate` and the Deliver-To actions
go to the problem page.

## Locks

**`/locks`** lists every lock on the records the visitor may use (`GET
/holders` with no record): a search over record, lock and holder (`q`), a
**Kind** segmented filter over the kinds present (`kind`), and a table Record
(kind glyph, linked to the record's page at `#locks`) · Lock · Holder (a `you`
pill) · Time left · the controls below, whose release comes back to `/locks`.
Nothing held: *No locks held*; nothing matching: *No locks match these
filters* with **Clear filters**.

A record's page carries a **Locks** card `id=locks` only while locks are held; a Group's page also shows the empty state. Only its Owner, Maintainers and own Agent see it; `GET /holders` refuses anyone else, and the card is then absent. Name · Holder (a `you` pill for the viewer's own) · Time left · a control: **Release** for one's own; for another's, **Force release**, a `<details>` that opens a line naming the holder and the audit, then **Confirm force release of <name>**. None: muted “No locks held.”

`POST /release-lock` with `record`, `name` and, for another's lock,
`force=1`: it calls `POST /release` or `POST /release-force`
([shared locks](../../../docs/01-identity-and-authority.md#shared-locks)) and
redirects to the record's page at `#locks` with *Lock released.* or *Lock released by
force; the audit log records it.* A refusal, such as releasing another's lock
without `force`, is the daemon's, on the problem page.

## Key-value stores

Only a record's Owner, Maintainers and own Agent see or change its store
([key-value store](../../../docs/03-records-kv.md#key-value-store)):
`GET /kv/list` refuses anyone else, and then there is no card and no row.

| Where | What |
|---|---|
| **Card** `id=kv` | on every record's page, a Group's, and a User's own for that User: non-zero type counts with singular/plural labels, separated by dots; keys and values are shown only on the store page; **Open store** → `/kv/record?name=`. An empty store keeps only the **Open store** link. |
| **`/kv`** | every store the visitor may use (`GET /kv/list` with no record): search over the record (`q`), a **Kind** filter over the kinds present (`kind`), and a table Record (kind glyph, linked to its page) · Strings · Integers · JSON · **Open**. An **Open a record's store** form takes any record name, for a store still empty. None: *No values stored*; nothing matching: *No stores match these filters* |
| **`/kv/record?name=`** | a card per kind that holds values, Strings, Integers and JSON (`id=kv-string`, `kv-int`, `kv-json`), a kind with none left out and an empty store saying *No values yet*: Name · Size (not for integers) · the preview · **Edit** → `/kv/value`, and **Delete** behind a confirmation. A string that is not text shows as `base64:…`, set through the CLI or API only. **Add value** `id=form-kv-add`: Kind, Name, Mode (set, add if absent, replace if present) and Value. A **Bigfiles** card `id=kv-bigfile` appears only when the record holds bigfiles (`GET /bigfile/list`; a refusal leaves it out): Name · Size · the first 12 hex of the SHA-256 (full in the tooltip) · Held · Time left · **Delete** behind a confirmation (`POST /bigfile-delete`); contents are never shown, and upload and download stay with the CLI and API |
| **`/kv/value?record=&kind=&name=`** | the whole value: an integer as a field, text or JSON (indented) in a textarea; **Save** writes it with `how=replace`. An integer also has **Add 1** and **Subtract 1**. **Delete** behind a confirmation |

| Post | Daemon call | Success | Refusal |
|---|---|---|---|
| `POST /kv-set` | `POST /kv/set`; an integer is sent as its digits, exact past 2^53; JSON is parsed first, each number kept as written; a string's CRLF becomes LF | **303** to the store, *Value saved.* | 400, 404, 409, 412, 413, 429: the same page again with the typed value kept and marked; else the problem page |
| `POST /kv-inc` | `POST /kv/inc` with `n` 1 or −1 | **303** to the value | the problem page |
| `POST /kv-delete` | `POST /kv/delete` | **303** to the store, *Value deleted.* | the problem page |


## Record labels

The top browse row contains All, My and tag(count) filters. My is a
hardcoded ownership filter, not a tag. The [tag contract](../../../docs/03-records.md#tags)
owns normalization, count scope, sorting and the expandable remainder. Tags
follow Description in every record editor.

## Documentation targets

| Pages | Main documentation |
|---|---|
| Agents, including add/edit/detail | [Agents](../../../docs/03-records-agent.md#what-an-agent-is) |
| Services, including add/edit/detail | [Services](../../../docs/03-records-service.md#what-a-service-is) |
| Queues and PubSub, including add/edit/detail | [Channels](../../../docs/03-records-channel.md#the-two-channel-kinds); queue [overflow](../../../docs/04-messaging.md#overflow) or [Deliver-To](../../../docs/04-messaging.md#deliver-to) |
| Resources, including add/edit/detail | [Resources](../../../docs/03-records-resource.md#what-a-resource-is) |
| Danger Zone, deactivation and confirmations | [Record fields](../../../docs/03-records.md#fields), [ownership](../../../docs/01-identity-and-authority.md#ownership), [unregistering](../../../docs/01-identity-and-authority.md#unregistering) |
| Locks | [Shared locks](../../../docs/01-identity-and-authority.md#shared-locks) |
| KV listing, record store and value forms | [KV rules](../../../docs/03-records-kv.md#rules), [JSON operations](../../../docs/03-records-kv.md#json-operations) |

Registry pages also link [tags](../../../docs/03-records.md#tags); record editors
link [fields](../../../docs/03-records.md#fields) and [ACL](../../../docs/02-access.md#acl).

The [shared documentation control](shell.md#github-documentation) owns appearance,
external-link behavior and field-help targets.

### Agent roles display

The [detail content table](#content-2) separates application Roles from Allow and Maintainers in Status & Policy. Group grants remain group names; the face does not invent membership or effective authority.

## Agent metadata

The record page and editor use the visitor's `help-json` ask to show
[agent descriptions and settings declarations](../../../docs/03-records-agent.md#about-an-agent).
Unavailable metadata leaves the rest of the page usable; bare agents fall back
to text help. Secret declaration rendering selects name, help and required only,
never arbitrary fields from the answer. The page does not request stored Secret.

### Method rows

The Methods card uses native disclosure rows: name, argument signature and a
short description stay visible; expanding shows typed arguments, enum chips,
meaningful returns and copyable CLI/JSON examples. Required arguments use
`<name>`, optional ones `[name]`, and rest arguments `…`; `json_only` arguments
appear only in JSON. The authored CLI example wins, otherwise the card builds
a template from the method's args or text usage. Values in angle brackets
must be replaced. JSON examples use `{"method": "name", ...args}` only when the
agent declares the `json` call form; absent `body_formats` means text only.

`help` is omitted. The count excludes it; rows keep author order within Read
(no declared role), Act (a role) and Destructive (explicit destructive flag).
Role and destructive badges appear only where declared. A native GET filter
`?m=` matches name or description, preserving the agent name. Empty matches
say so; clearing restores the full list. No call is sent by this card.
Configuration and Secret declarations use compact key/description rows.
Native details and filtering work without JavaScript; copying then means
selecting the example text. Narrow screens stack key rows and wrap signatures;
long example lines scroll inside their own block.

The footer reads `hidden_methods` from machine help: it shows the role-hidden
count and required role names, never hidden method details. The filter only
searches methods returned to this visitor; it cannot search hidden names.

Settings field hints stay short; detailed explanations belong in their help popovers. Roles uses the full width of the form, like Tags. Queue settings share one heading and help popover, with Retention, Bound and When full on one wrapping row. An unset bound is empty with a default placeholder; submitted names and empty/default semantics are unchanged.

### Configuration and JSON display

Stored Configuration is visible only to its managers. Its expandable preview header
shows an eight-character SHA with the full digest in its title; Status & Policy
does not repeat it. An absent Configuration has no preview or digest. Read-only
JSON uses one escaped token highlighter; selection and copy preserve plain JSON.
JSON editors remain native text inputs.

### Record labels and agent status

Tags use the same linked badges on listings, record and runner pages; `personal`
has a distinct style. Record pages put them once in the header metadata line,
after kind and owner; every badge links to the agents tag filter. The synthetic
empty-tag filter is never a record label.
The My listing omits Owner; other listings mark the signed-in owner's records
with the you badge.

While an agent page is open, one asynchronous request asks its status as the
visitor, with a 1.8-second budget. Its escaped answer appears next to the record
state in Status & Policy. Unsupported, offline and timed-out status answers
show nothing. Methods is a closed disclosure; its anchors and copy buttons
remain available when expanded.

KV listing count cells are empty for zero, with numeric data and ordering unchanged.

Activity strips share the chart component for every range. Their bounds come from the rendered uPlot plot area and refresh whenever the plot is rebuilt for width or theme.
