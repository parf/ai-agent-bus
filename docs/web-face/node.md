# Web face node pages

📌 **TL;DR:** The node-level pages of the web face (`agent-bus-web`,
`src/web`): the landing and sign-in page, sign-in and sign-out, Overview,
Activity, Diagnostics, liveness, the static assets and the redirects that are
not record pages. Every page follows the [shell](shell.md#web-face-shell).

Every page below uses the same headings: route, access, daemon calls,
content, states, controls, forms and links. A heading with nothing to say is
omitted. "Frame" means the [page frame](shell.md#page-frame); every rendered
page makes the anonymous `GET /identity` for it.

## `/` signed out: landing and sign-in

### Route

| Method | Path | Params | Notes |
|---|---|---|---|
| GET, HEAD | `/` | none | the homepage without a session. The same page is the sign-in answer of every other page; see [shell § signed-out requests](shell.md#signed-out-requests) |

### Access

Anonymous only: the page shown when the request carries no valid `agent_bus_session` cookie.

### Daemon calls

| Call | Credential | Gives |
|---|---|---|
| `GET /identity` | none | brand release, build and host; footer owner and uptime |

### Content

| # | Part | Content |
|---|---|---|
| 1 | Title | `Sign in · agent-bus` |
| 2 | Frame | bare top bar: skip link, brand, theme toggle; **no** sidebar or account link. Footer as on every page |
| 3 | Backdrop | the landing picture, blurred behind the page (`alt=""`) |
| 4 | Hero | eyebrow `agent-bus`; `<h1>One bus for agents, bots and services</h1>`; lede "Connect AI and NON-AI agents, bots and services so they can find and message each other. One daemon gives you a registry, message queues, an MCP server, dashboard and much more…"; `GitHub — docs & updates` → `https://github.com/parf/ai-agent-bus` · by `Serg Parf` → `https://parf.dev/` |
| 5 | Sign-in card | `<h2>Sign in</h2>` with a key mark, help button, popover, form |
| 6 | Picture | `<img src=/agent-bus.<hash>.webp width=1536 height=1024 alt="A red double-decker named Agents Bus, carrying AI and non-AI riders: Claude, OpenAI, Slack, Telegram, Email and a shell">` |
| 7 | Features | four cards, each a mark, a title and one sentence (below) |

| Card | Mark | Sentence |
|---|---|---|
| Registry | Lucide `id-card` | Who and what is on the bus: users, agents, queues, services and groups. Every record has an owner and a list of who may reach it. |
| Messages | 📮 | Queues hold what was sent until somebody reads it. Pub/sub copies one publication to everyone subscribed. |
| MCP server | 👾 | An agent reaches the bus through MCP, so finding a peer and sending it a message are tools the model already knows how to call. |
| Dashboard | Lucide `layout-dashboard` | This web face, once you are signed in: what is registered, what is waiting, and what has gone wrong. |

| Help | Value |
|---|---|
| Button | `aria-label="How to get a token"`, tooltip "A token is what every call carries. Run agent-bus-token <name> on the box, or ssh agent-busd@<node> token from anywhere your key reaches." |
| Popover `token-help` | heading `Getting a token`: On this box: `agent-bus-token <name>` · From anywhere your key reaches: `ssh agent-busd@<node> token` · Asking again returns the token you already have. It does not expire on its own. |

### States

| State | Shown |
|---|---|
| Fresh visit | the form, no message, `200` |
| Refused sign-in | `that credential was not accepted` under the field, `401` |
| Daemon unreachable at sign-in | `the bus is not answering, so nothing was checked — try again when it is back`, `502` |
| From a protected page | `sign in to open this page`, `401` |
| Session ended | `that session has ended — sign in to carry on`, `401` |
| No identity | brand `AgentBus node unavailable`, footer `Node information unavailable` |

### Forms

`<form method=post action=/signin class=signin-form>`

| `name` | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `return` | hidden | no | only present when non-empty; the page asked for (see [shell](shell.md#signed-out-requests)) | where to go after sign-in |
| `token` | `password`, `id=token`, `autofocus`, `autocomplete=current-password`, placeholder `Paste your token` | no (no `required` attribute) | empty | the visitor's token |

Label `<label for=token>Token</label>`; submit `Sign in`. A message sets
`aria-invalid="true"` on the field and shows as `<p class=warn id=signin-error role=alert>`.

### Links out

The GitHub and parf.dev links, the form action.

## `POST /signin`

### Route

| Method | Path | Body |
|---|---|---|
| POST | `/signin` | form `token`, `return` |

### Access

Anyone, subject to the [origin check](shell.md#origin-checks).

### Daemon calls

| Call | Credential | Gives |
|---|---|---|
| `POST /session` | `X-Agent-Bus-Token: <typed token>` | `{"session": "<id>", "idle": "30m0s"}` |

### States

| Case | Response |
|---|---|
| `token` empty, or not printable ASCII | no daemon call; the sign-in page, `401`, `that credential was not accepted`, `return` kept |
| Daemon refuses (any code, including `403 suspended` for an inactive user) | the sign-in page, `401`, the same message, `return` kept. One message for every refusal, so the form is no oracle |
| Daemon unreachable | the sign-in page, `502`, `the bus is not answering, so nothing was checked — try again when it is back` |
| Success | `303 See Other`, `Location: local(return)` (default `/`), the [session cookie](shell.md#session) |

The token is never stored, logged or echoed.

## `POST /signout`

### Route

| Method | Path | Body |
|---|---|---|
| POST | `/signout` | none needed |

### Access

Anyone, also without a cookie, subject to the [origin check](shell.md#origin-checks).

### Daemon calls

| Call | Credential | When |
|---|---|---|
| `DELETE /session` | the cookie's session id | only if a session was sent. The result is ignored |

### States

Always `303 See Other`, `Location: /`, the cookie cleared ([shell § session](shell.md#session)).

### Forms

The form is in the top bar of every signed-in page: `<form method=post action=/signout class=who>` holding the account link and a sign-out button. No fields.

## `/` signed in: Overview

### Route

| Method | Path | Params |
|---|---|---|
| GET | `/` | none |

Links here: navigation entry 1, the brand, the unclean-stop item (`/#node`), sign-in with no `return`.

### Access

| Caller | Difference |
|---|---|
| No session | the landing page above |
| Daemon Owner | records it may see (all of them); `owner_inactive` item possible; `Daemon owner` pill |
| Administrator | records it may see; `owner_inactive` item possible; `Daemon administrator` pill |
| Ordinary user | only records it may see; the daemon sends no `owner_inactive`, so that item never appears |
| Everyone | refusal items and the node strip are node-wide and identical for every caller |

### Daemon calls

| Call | Used for | On failure |
|---|---|---|
| `GET /identity` (no credential) | frame; Uptime; the call tiles (`calls.windows[]`, `calls.total`) | tiles `unavailable` |
| `GET /status` | `you`, roles; `waiting` → Readers; `queued` → Queued; `kinds` → six kind tiles; `services` → Records (fallback); `up` → Uptime when identity has none; `refused`, `unclean`, `owner_inactive` → items | problem page |
| `GET /ls`, `GET /inactive` | visible records, inactive ones marked → record items | problem page |
| `GET /activity` | today's slots for visible records → Queued sparkline and the Today card | both omitted |

### Content

| # | Section | Content |
|---|---|---|
| 1 | Title | `Overview · agent-bus` |
| 2 | Heading | `Overview` + help (below); sub line `Signed in as <code>{you}</code>` and the authority pill |
| 3 | Needs attention | `<section aria-labelledby=attention>`, heading with the item count, and one `<article class="attention-item attention-{level}">` per item: mark, `<h3>` title, paragraph, link. With no items, a `Nothing to report` card: "No enumerated condition holds right now. That is not a claim the node is healthy." |
| 4 | This node | `<h2 id=node>This node</h2>` + help, the node strip, then "Node-wide. The lists linked below contain only records visible to you; the two never have to agree." |
| 5 | Today | card `Today, visible records`: the 144-cell day ribbon, its first and last slot times, and each series' total; `Open Activity` → `/activity` |
| 6 | Find | `<nav class=overview-links aria-label="Find records">` and three links |

#### Attention items

Computed by the face (`attentionItems`) from daemon facts. One item per record,
at most. Sort order: level (red, orange, blue), then title, then name or reason.

| Kind | Condition | Level | `<h3>` title | Paragraph | Link |
|---|---|---|---|---|---|
| unclean | `status.unclean` true | red | The previous stop was not clean | Memory from the previous run may not have reached the snapshot. | `/#node` · View node totals |
| refusal | each reason in `status.refused` with count > 0 | blue | Requests were refused | `<code>{reason}</code> · {count} since this daemon started` | `/diagnostics#refusals` · View refusal reasons |
| owner-inactive | `status.owner_inactive.records` > 0 | orange | Records inactive because their owner is | `{records} record(s) · {messages} message(s) held · node-wide; each returns when its owner is reactivated` | `/users?state=inactive` · View inactive users |
| record | inactive and `queued` > 0 | orange | Inactive and work is held | record line | record detail · View record |
| record | `at_bound` (and not the case above) | red | Queue at capacity when observed | record line | record detail · View record |
| record | `dropped + expired` > 0 (and neither above) | orange | Messages were lost from this inbox | record line | record detail · View record |

**Record line:** `<code>{name}</code> · {queued} held now`, then each that
applies: ` · oldest {oldest}` (when something is held) · ` · inactive` ·
` · at capacity` with ` · when full: drop the oldest` (overflow `ring`) or
` · when full: refuse` · ` · {dropped} dropped` · ` · {expired} expired`.
The detail path follows the kind: `/agent`, `/queue` (queue and user),
`/pubsub/topic`, `/group`, else `/service`, with `?name=`.

#### Node strip

Each tile is a label with its mark and a value. Zero is a muted `—`. Two rows.

| Row | Tile | Source |
|---|---|---|
| 1 | Readers | `status.waiting` |
| 1 | Queued | `status.queued`, with a sparkline of Accepted per ten minutes today |
| 1 | Agents, Services, Queues, PubSub, Users, Groups | `status.kinds[agent / service / queue / pubsub / user / group]`, each linked to its list. If `kinds` is empty, one tile `Records` = `status.services` instead |
| 2 | Uptime | `identity.up`, else `status.up`, else `unavailable` |
| 2 | Calls, minute / Calls, hour | each `identity.calls.windows[]` in order; window `1m` is "minute", any other is "hour". `collecting history` when `available` is false |
| 2 | Calls, total | `identity.calls.total` |
| 2 | Calls `unavailable` | one tile in place of the three when identity or `calls` is missing |

#### Help text

| Button | Tooltip | Popover |
|---|---|---|
| About Overview (`overview-help`) | Only enumerated observations appear. An empty list does not claim the node is healthy. | Overview scope: attention covers conditions over records visible to you, node-wide refusals, the previous-stop marker and, for Administrators, owner-inactive records · a backlog alone is ordinary work · node totals and your lists have different scopes |
| About node totals (`node-help`) | Whole-node values. Caller-visible lists may show a smaller set. | Node totals: six points on scope, dash meaning none, kind counts, Readers, Calls and `collecting history`, Uptime |

### Controls by role

None. The page is read-only for every role.

### Links out

Navigation; attention links; kind tiles; `Open Activity`; Find:
`/agents?sort=queued&work=held` (Agents holding work),
`/queues?sort=queued&work=held` (Queues holding work), `/services` (External services).

## `/activity`

### Route

| Method | Path | Param | Values | Default |
|---|---|---|---|---|
| GET | `/activity` | `name` | a record name; empty means every record visible to the caller | empty |
| | | `range` | `day`, `week`, `month` (1, 7 or 30 days) | `day` |
| | | `at` | the range's last day, `yymmdd`, clamped to the days the daemon keeps (`/status` `today`, `activity_days_kept`) | today |

Links here: navigation entry 8; Overview's `Open Activity`; record detail's
`Open in Activity` with `?name=` and its range.

### Access

| Caller | Difference |
|---|---|
| No session | sign-in page, `401`, `return=/activity…` |
| Daemon Owner | unfiltered Refused series is node-wide |
| Other users | unfiltered series sum the live records they may see; Refused likewise |
| `name` not visible or absent | `404 No such name` for the whole page |
| `name` inactive | an `Inactive record` card: its history is kept and returns when it is reactivated |

### Daemon calls

| Call | Used for |
|---|---|
| `GET /status` | `you`; `today` and `activity_days_kept` for the range; `up` when identity has none |
| `GET /ls`, `GET /inactive` | the chooser's names, inactive ones marked |
| `GET /activity?name=` | the live day, when the range is today's Day |
| `GET /activity/days?name=&from=&to=` | any other range, from the stored days |
| `GET /activity/days?totals=1&from=&to=` | each visible record's hits over the range, for the chooser; on failure the chooser offers only All visible and the chosen name |

The rules behind these answers are [activity history](../05-discovery.md#activity-history).

### Content

| # | Part | Content |
|---|---|---|
| 1 | Title | `Activity · {range title} · agent-bus` |
| 2 | Heading | `Activity` and the range title in bold: `Last 24 hours` for the live day, `Mon, Sep 22` for a past day, `Sep 17 – 23` for a week or month; help button `aria-label="About activity history"` and popover `activity-help` |
| 3 | Sub line | `Scope: <code>{name}</code>` or `Scope: visible records` · `Uptime {up}` |
| 4 | Chooser | see Forms below |
| 5 | Range nav | Day · Week · Month; `Prev` and `Next` step one range while the days are kept (disabled at the ends); `Today` when away from it |
| 6 | Chart | uPlot over the non-zero series (Accepted, Dequeued, Dropped, Expired, Refused) on one scale: ten-minute slots for a day, hourly lines plus a 7×144 grid of the days for a week, day bars plus a 30-cell calendar for a month; each grid row or calendar cell links to its day |
| 7 | Zero note | `Zero all day: {labels}.` (`in this range` for a past range) when some but not all series are zero |
| 8 | Values | `<details>` `Slot values` · `Hour values` · `Day values`: one row per slot, hour or day with the five series |

Popover `activity-help`: Day shows the day on the node's clock, Week sums it per
hour, Month per day · every day is kept for 400 days, and Prev and Next step
through them · down time reads as zero, and today's last slot is still counting
· unfiltered Refused is node-wide for the daemon Owner and covers visible
records for others · Dequeued is not completion · the chooser lists records with
hits in the chosen range.

### States

| State | Shown |
|---|---|
| All series zero | no chart; `All five series: 0 in the last day.` (`in this range`) |
| Daemon returned no slots | `The daemon answered no activity.` |
| A daemon call refused | whole-page problem page (`404` for an unknown or hidden name) |

### Controls by role

Same for every role.

### Forms

`<form method=get class=scope-form>` (action is the current page), carrying `range` and `at` hidden when not the defaults.

| `name` | Type | Default | Meaning |
|---|---|---|---|
| `name` | `<select id=scope-name data-submit-on-change>`, visually hidden label `Record` | the current `name` | first option `value=""` `All visible ({hits})`, then every visible name with hits in the range, in name order, as `name (hits)`; an inactive one reads `name (inactive)`. The chosen name is always listed |

### Links out

Navigation; range links; the grid and calendar day links.

## `/diagnostics`

### Route

| Method | Path | Params | Anchors |
|---|---|---|---|
| GET | `/diagnostics` | none | `#refusals`, `#stuck`, `#exchanges`, `#loss`, `#leftovers`, `#message-{id}` per exchange row |

Links here: navigation entry 9; Overview refusal items (`#refusals`); `/users?kind=other` (`303` → `#leftovers`); user pages' `return=/diagnostics`.

### Access

| Caller | Difference |
|---|---|
| No session | sign-in page, `401`, `sign in to open this page`, `return=/diagnostics` |
| Daemon Owner | exchanges cover the whole node (`/recent` answers the node) |
| Others | exchanges only for messages they were party to |
| Everyone | refusal counts are node-wide; held and loss tables cover visible records, inactive ones included |
| Leftovers | rows only when the daemon's `/users` answer holds non-`user` kinds; the removal link depends on `can_remove` |

### Daemon calls

| Call | Used for | On failure |
|---|---|---|
| `GET /status` | `you`; `refused` → Refusals | whole page fails |
| `GET /ls`, `GET /inactive` | held inboxes, loss | whole page fails |
| `GET /recent` | exchanges | section says `Envelope history unavailable: {message}` |
| `GET /users` | leftovers: entries whose `kind` is not `user` | section says `Leftover names unavailable: {message}` |

### Content

| # | Section | Content |
|---|---|---|
| 1 | Title and heading | `Diagnostics · agent-bus`; `Diagnostics` + help "Caller-visible queues, loss and retained envelope evidence. Bodies are never shown."; a `Refresh` button → `/diagnostics` |
| 2 | Jump links | `On this page`: Refusals · Held inboxes · Exchanges · Loss · Leftover names (when any) |
| 3 | Refusals `#refusals` | Reason (`<code>`) · Count; caption `Refusals since this daemon started, by reason` |
| 4 | Loss by name `#loss` | Name · Dropped · Expired |
| 5 | Inboxes holding messages `#stuck` | caption `Inboxes holding messages, longest wait first — visible to you`: Name · Readers · Held now · Oldest held · Capacity |
| 6 | Exchanges in retained history `#exchanges` | caption `Messages and explicitly referenced receipts`: Observed message · Route and conversation · Envelopes · Evidence |
| 7 | Leftover names `#leftovers` | only while one exists: Name · What it is · Next step |

A name cell links to the record's detail and carries `INACTIVE` for an inactive record.

<details><summary>Refusals</summary>

Rows: every reason the daemon defines (`acl`, `busy`, `credential`,
`enrolment`, `full`, `malformed`, `name-taken`, `second-reader`, `suspended`,
`unknown`), missing ones as a measured `0`, plus any reason the daemon reports
that the face does not know. Sorted by count descending, then reason; a
non-zero row is highlighted. Help: whole-node handled API refusals since
process start; zero is measured; router misses and internal failures excluded.

</details>

<details><summary>Inboxes holding messages</summary>

Rows: visible records with `queued` > 0, sorted by `oldest` descending (parsed
as a Go duration; unparseable sorts as zero), then `queued` descending, then name.

| Column | Source |
|---|---|
| Name | kind mark and `<code>{name}</code>`, linked to its detail |
| Readers | `readers`, or `unavailable` when absent |
| Held now | `queued` |
| Oldest held | `oldest`, or muted `—` |
| Capacity | `at capacity when observed` when `at_bound`, else muted `—` |

Empty: one row `every queue you can see is empty`.

</details>

<details><summary>Exchanges</summary>

Face-computed from `/recent` envelopes (`exchanges`). Bodies are never read.

| Step | Rule |
|---|---|
| Rows | every non-receipt envelope is a row |
| Folding | a receipt joins its original only when `re` names exactly one retained original that is not itself a receipt, the receipt travels the original's reply route, and it is not earlier than it. Its sender is not checked, so a receipt from a queue's consumer folds. Anything else is its own row with a notice |
| Reply route | `reply_to` if set, else back to `from` with the same topic and tag |
| Matches | for a tagged non-receipt row, every earlier non-receipt it answers: sent from the original's `to` (or its original address) along its reply route. `Late` when exactly one matches and this row is after its deadline |
| Order | newest activity (row time or latest folded receipt) first, then ID |

| Column | Content |
|---|---|
| Observed message | `<time>{at, 2006-01-02 15:04:05}</time>` and `<code>{message_id}</code>` |
| Route and conversation | `{from} → {to}`; `Topic: {topic or not supplied} · Tag: {tag or not supplied}`; with `reply_to`, `Reply route: {name} · {topic} · {tag}` |
| Envelopes | 1 + folded receipts |
| Evidence, receipt row | `{ack or done} receipt` about `<code>{re}</code>` and one notice: `Receipt has no original message reference.` · `Original message not in this visible history.` · `Reference found; route or time differs from the original. Kept separate.` · `Reference is ambiguous in this history. Kept separate.` · `The referenced message is itself a receipt. Kept separate.` |
| Evidence, message row | `Acknowledgement observed.` (if an ack folded); `Completion receipt observed.` or muted `No completion receipt observed in retained history.`; `Possible response — matching earlier routes:` links to `#message-{id}`; warn `After the matching message's deadline.`; `<details><summary>Receipt evidence</summary>` per folded receipt: type, ID, re, route, time, `after the request's deadline` |

Empty: `No envelopes in your retained history. This is not a count of all traffic.`

</details>

<details><summary>Loss and leftovers</summary>

Loss rows: visible records with `dropped + expired` > 0, by that sum
descending, then name. Empty: `nothing lost`.

| Leftover `kind` | What it is | Next step |
|---|---|---|
| `record` | Self-owned record, no User profile | `Inspect before deciding` → `/user?name={name}&return=/diagnostics` |
| `credential`, `can_remove` | Credential with no record | `Review credential removal` → same URL |
| `credential`, not removable | Credential with no record | text: An authorized administrator can review removal. |

</details>

### States

| State | Shown |
|---|---|
| No refusals | every reason listed at `0` |
| `/recent` refused | `Envelope history unavailable: {daemon message or "the daemon did not answer"}` |
| `/users` refused | `Leftover names unavailable: {…}` |
| No leftovers | the section is absent |
| `/status`, `/ls` or `/inactive` fails | problem page |

### Controls by role

None. Leftover links go to the people pages, which own the removal.

### Links out

Refresh; jump links; record detail per name; `#message-` anchors; `/user?name=…&return=/diagnostics`.

## `/healthz`

| Heading | Value |
|---|---|
| Route | `GET` or `HEAD /healthz`; any other method gets the framed `405` |
| Access | anyone |
| Daemon calls | none; it says only that the web process answers |
| Content | `200`, empty body, the standard security headers |

## Static assets

| Route | Access | Daemon call | Response |
|---|---|---|---|
| `GET /app.<hash>.css`, `/ui.<hash>.js`, `/agent-bus.<hash>.webp` | anyone | none | the file, `immutable` ([shell § assets](shell.md#assets)) |
| `GET /favicon.svg` | anyone | none | SVG, `no-store` |
| `GET /favicon.ico` | anyone | none | `404`, plain-text body `404 page not found` |

Another method on any of them gets the framed `405`.

## Redirects and catch-alls

Record-page aliases (`/channels`, `/channels/new`, `/channel`, `/channel/edit`)
are specified with the [record pages](records.md#lists).

| From | To | Status | Rule |
|---|---|---|---|
| `GET /users?kind=other`, signed in | `/diagnostics#leftovers` | `303` | any other `kind` is ignored |
| Daemon `GET /` on its TCP API port (exact root) | the dashboard URL, default `http://127.0.0.1:6780/` | `301` | served by `agent-busd` ([where it listens](../05-discovery.md#where-it-listens)) |
| Any unknown path | — | `404` | framed `No such page` ([shell § problem page](shell.md#problem-page)) |
| Known path, wrong method | — | `405` | framed `Not answered this way` |

## Inconsistencies worth fixing in the rewrite

History: the Go face's list, and each page's differences from older docs, are in
[web-go-face-differences § node](../../Plans/R0.8-MVP/done/web-go-face-differences.md#nodemd).
