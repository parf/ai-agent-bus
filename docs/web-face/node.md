# Web face node pages

📌 **TL;DR:** Internal working document for the TypeScript rewrite, not
linked from the docs set. It specifies the node-level pages of
`agent-bus-web` as built in 0.8.31: the landing and sign-in page, Overview,
sign-in and sign-out, Activity, Diagnostics, health, the static assets and the
redirects that are not record pages. Every page follows the
[shell](shell.md#web-face-shell). Where code and older docs disagree the code
wins; each page ends with its differences.

Every page below uses the same eight headings: route, access, daemon calls,
content, states, controls, forms and links. A heading with nothing to say is
omitted. "Frame" means the header and footer in [shell](shell.md#page-frame).
It needs `GET /identity` on every page except `/healthz`, `/ui.js`,
`/signout` and `/agent-bus.jpg`.

## `/` signed out: landing and sign-in

### Route

| Method | Path | Params | Notes |
|---|---|---|---|
| GET | `/` | none | also answers **every unmatched GET path** for a visitor with no cookie, with `200`. The same page is the sign-in response of every other page; see [shell § signed-out requests](shell.md#signed-out-requests) |
| HEAD | `/` | — | answered like GET |

### Access

Anonymous only: the page shown when the request carries no `agent_bus_session` cookie.

### Daemon calls

| Call | Credential | Gives |
|---|---|---|
| `GET /identity` | none | header release, build, host; footer owner and uptime |

### Content

| # | Part | Content |
|---|---|---|
| 1 | Title | `Sign in · agent-bus` |
| 2 | Frame | skip link, logo, release and host; **no** account link or navigation. Footer as on every page |
| 3 | Hero | `<p class=hero><img src=/agent-bus.jpg width=648 height=432 alt="A red double-decker named Agents Bus, carrying AI and non-AI riders: Claude, OpenAI, Slack, Telegram, Email and a shell">` |
| 4 | Heading | `<h1>One bus for agents, bots and services</h1>` |
| 5 | Lede | "Connect AI and NON-AI agents, bots and services so they can find and message each other. One daemon gives you a registry, message queues, an MCP server, dashboard and much more…" |
| 6 | Features | four cards, each a bold title with its mark, then one sentence (below) |
| 7 | Links | `GitHub — docs & updates` → `https://github.com/parf/ai-agent-bus` · by `Serg Parf` → `https://parf.dev/` |
| 8 | Sign-in card | `<h2>🔑 Sign in</h2>`, help button, popover, form |

| Card | Mark | Sentence |
|---|---|---|
| Registry | 🪪 | Who and what is on the bus: users, agents, queues, services and groups. Every record has an owner and a list of who may reach it. |
| Messages | 📮 | Queues hold what was sent until somebody reads it. Pub/sub copies one publication to everyone subscribed. |
| MCP server | 👾 | An agent reaches the bus through MCP, so finding a peer and sending it a message are tools the model already knows how to call. |
| Dashboard | 🏠 | This web face, once you are signed in: what is registered, what is waiting, and what has gone wrong. |

| Help | Value |
|---|---|
| Button | `aria-label="How to get a token"`, tooltip "A token is what every call carries. Run agent-bus-token <name> on the box, or ssh agent-busd@<node> token from anywhere your key reaches." |
| Popover `token-help` | `<h2>Getting a token</h2>`: On this box: `agent-bus-token <name>` · From anywhere your key reaches: `ssh agent-busd@<node> token` · Asking again returns the token you already have. It does not expire on its own. |

### States

| State | Shown |
|---|---|
| Fresh visit | the form, no message, `200` |
| Refused sign-in | `<p class=warn>that credential was not accepted` under the field, `200` |
| From a protected page | message `sign in to open this page`, `401`; or none, `200`, from `/diagnostics` |
| Session ended | message `that session has ended — sign in to carry on`, `401` |
| No identity | header `AgentBus node unavailable`, footer `Node information unavailable` |

### Forms

`<form method=post action=/signin>`

| `name` | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `return` | hidden | no | only present when non-empty; the page asked for (see [shell](shell.md#signed-out-requests)) | where to go after sign-in |
| `token` | `password`, `id=token`, `autofocus` | no (no `required` attribute) | empty | the visitor's token |

Submit: `<button type=submit>sign in</button>`. The label is `<label for=token>token</label>`.

### Links out

`/agent-bus.jpg`, the GitHub and parf.dev links, the form action.

### Differences

| Older doc | Code |
|---|---|
| `Plans/R0.8-MVP/web-handoff/pages.md#landing-and-sign-in-`: acquisition help mentions the SSH onboarding path | the popover states two commands and the non-expiry note only |

## `POST /signin`

### Route

| Method | Path | Body |
|---|---|---|
| POST | `/signin` | form `token`, `return` |

### Access

Anyone. The global origin check applies: a foreign `Origin` gets `403 same-origin form required`. An absent `Origin` passes.

### Daemon calls

| Call | Credential | Gives |
|---|---|---|
| `POST /session` | `X-Agent-Bus-Token: <typed token>` | `{"session": "<id>", "idle": "30m0s"}` |

### States

| Case | Response |
|---|---|
| `token` empty | no daemon call; the sign-in page, `200`, `that credential was not accepted`, `return` kept |
| Daemon refuses (any code, including `403 suspended` for an inactive user) or is unreachable | the sign-in page, `200`, the same message, `return` kept. One message for every failure, so the form is no oracle |
| Success | `303 See Other`, `Location: local(return)` (default `/`), `Set-Cookie: agent_bus_session=<id>; Path=/; HttpOnly; SameSite=Strict` (+ `Secure` under TLS) |

The token is never stored, logged or echoed.

### Differences

| Older doc | Code |
|---|---|
| — | a bus that is down is reported as `that credential was not accepted` (see below) |

## `POST /signout`

### Route

| Method | Path | Body |
|---|---|---|
| POST | `/signout` | none needed |

### Access

Anyone; also without a cookie. Global origin check only. The face makes no `/identity` call for this path.

### Daemon calls

| Call | Credential | When |
|---|---|---|
| `DELETE /session` | the cookie's session id | only if a cookie was sent. The result is ignored |

### States

Always `303 See Other`, `Location: /`, `Set-Cookie: agent_bus_session=; Path=/; Max-Age=0`.

### Forms

The form is part of the header on every signed-in page: `<form method=post action=/signout class=who>` holding the account link and `<button type=submit>sign out</button>`. No fields.

## `/` signed in: Overview

### Route

| Method | Path | Params | Notes |
|---|---|---|---|
| GET | `/` | none | also answers every unmatched GET path for a signed-in visitor, `200` |

Links here: navigation entry 1, the unclean-stop item (`/#node`), sign-in with no `return`.

### Access

| Caller | Difference |
|---|---|
| No cookie | the landing page above |
| Daemon Owner | records it may see (all of them); `owner_inactive` item possible |
| Administrator | records it may see; `owner_inactive` item possible |
| Ordinary user | only records it may see; the daemon sends no `owner_inactive`, so that item never appears |
| Everyone | refusal items and the node strip are node-wide and identical for every caller |

### Daemon calls

| Call | Used for |
|---|---|
| `GET /identity` (no credential) | frame; the three call tiles (`calls.windows[]`, `calls.total`) |
| `GET /status` | `you`; `waiting` → Readers; `queued` → Queued; `kinds` → six kind tiles; `services` → Records (fallback); `up` → Uptime tile; `refused` → refusal items; `unclean`; `owner_inactive` |
| `GET /ls` | active visible records → record items |
| `GET /inactive` | inactive visible records; the face sets their status to `inactive` → record items |

### Content

| # | Section | Content |
|---|---|---|
| 1 | Title | `Overview · agent-bus` |
| 2 | Heading | `<h1>🏠 Overview</h1>` + help (below) |
| 3 | Needs attention | `<section aria-labelledby=attention><h2 id=attention>Needs attention</h2>` and a list of `<article class="attention-item attention-{level}">`. **Omitted entirely** when there are no items |
| 4 | This node | `<h2 id=node>This node</h2>` + help, then the node strip, then `<p class=muted>Node-wide. The lists linked below contain only records visible to you; the two never have to agree.</p>` |
| 5 | Find | `<nav class=overview-links aria-label="Find records"><strong>Find</strong>` and three links |

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
applies: ` · oldest {oldest}` · ` · inactive` · ` · at capacity` ·
` · when full: drop the oldest` (overflow `ring`) or ` · when full: refuse`
(otherwise; shown when at bound) · ` · {dropped} dropped` · ` · {expired} expired`.
The detail path follows the kind: `/agent`, `/queue` (queue and user),
`/pubsub/topic`, `/group`, else `/service`, with `?name=`.

#### Node strip

Each tile is `<div class=node-fact><span>{label}</span><strong>{value}</strong></div>`.
Zero is a muted `—` (`figure`). Two rows split by `<div class=node-break>`.

| Row | Tile | Source |
|---|---|---|
| 1 | Readers | `status.waiting` |
| 1 | Queued | `status.queued` |
| 1 | Agents, Services, Queues, PubSub, Users, Groups | `status.kinds[agent / service / queue / pubsub / user / group]`. If `kinds` is empty, one tile `Records` = `status.services` instead |
| 2 | Uptime | `status.up` (not the identity's `up`) |
| 2 | Calls, minute / Calls, hour | each `identity.calls.windows[]` in order; window `1m` is "minute", any other is "hour". `collecting history` when `available` is false |
| 2 | Calls, total | `identity.calls.total` |
| 2 | Calls `unavailable` | one tile in place of the three when identity or `calls` is missing |

#### Help text

| Button | Tooltip | Popover |
|---|---|---|
| About Overview (`overview-help`) | Only enumerated observations appear. An empty list does not claim the node is healthy. | Overview scope: attention covers conditions over records visible to you, node-wide refusals, the previous-stop marker and, for Administrators, owner-inactive records · a backlog alone is ordinary work · node totals and your lists have different scopes |
| About node totals (`node-help`) | Whole-node values. Caller-visible lists may show a smaller set. | Node totals: six points on scope, dash meaning none, kind counts, Readers, Calls and `collecting history` |

### States

| State | Shown |
|---|---|
| Nothing to report | no attention section at all |
| `/status` fails | problem page from `fail`, with an empty account name |
| `/ls` or `/inactive` fails | problem page; the whole page fails |
| No identity | Calls tile `unavailable`; frame degraded |

### Controls by role

None. The page is read-only for every role.

### Links out

Navigation; attention links; Find: `/agents?sort=queued&work=held` (Agents holding work), `/queues?sort=queued&work=held` (Queues holding work), `/services` (External services).

### Differences

| Older doc | Code |
|---|---|
| `Plans/R0.8-MVP/web-handoff/pages.md#overview-`: heading has observation time and a Refresh | no time or Refresh on the page; the footer states `Generated` |
| same: "Unregistered credentials awaiting review" item, blue | not built; leftovers appear only on Diagnostics |
| `docs/05-discovery.md#overview-and-diagnostics`: Find links the two holding-work views "and nothing else" | a third link, `External services` → `/services` |
| `Plans/R0.8-MVP/web-handoff/glyphs.md#where-a-glyph-is-allowed`: one severity glyph per attention item | no glyph; the level is a coloured left border only |

## `/activity`

### Route

| Method | Path | Param | Values | Default |
|---|---|---|---|---|
| GET | `/activity` | `name` | a record name; empty means every record visible to the caller | empty |

Links here: navigation entry 8; record detail's activity section with `?name=`.

### Access

| Caller | Difference |
|---|---|
| No cookie | sign-in page, `401`, `sign in to open this page`, `return=/activity…` |
| Daemon Owner | unfiltered Refused series is node-wide |
| Other users | unfiltered series sum the live records they may see; Refused likewise |
| Any caller, `name` not visible, absent or inactive | `404 No such name` for the whole page |

### Daemon calls

| Call | Used for |
|---|---|
| `GET /status` | `you`, roles; `up` → the Uptime line |
| `GET /ls` | the select options, sorted by name (active records only) |
| `GET /activity?name={name}` | 144 slots `{at, in, out, dropped, expired, refused}`, oldest first |

### Content

| # | Part | Content |
|---|---|---|
| 1 | Title | `Activity graphs · agent-bus` |
| 2 | Heading | `<h1>{chart SVG} Activity graphs</h1>` + help button `aria-label="About activity history"` (no tooltip text of its own) and popover `activity-help` |
| 3 | Scope form | see Forms below |
| 4 | Scope line | `Scope: <code>{name}</code> · {start} to {end}, ten-minute slots.` or `Scope: visible records · …`. `start` = first slot, `end` = last slot + 10 min, format `Jan 2 15:04` |
| 5 | Window line | `<p class=muted>Window: the last 24 hours of the node's clock · Uptime: {status.up or unavailable}.</p>` |
| 6 | Chart | `<figure>` with an SVG `viewBox="0 0 640 170"`, `role=img`, `aria-label="Activity per ten-minute slot from {start} to {end}; shared maximum {max} over the displayed nonzero series"` |
| 7 | Caption | `Shared scale: 0–{max} per ten-minute slot over the displayed nonzero series; a tick at every hour.` |
| 8 | Legend | one `<li>` per drawn series: swatch + `{Label}: {total} in the last day` |
| 9 | Zero note | `Zero all day: {labels}.` when some but not all series are zero |
| 10 | Slot table | `<details><summary>Slot values</summary>` table `aria-label="Slot values, one row per ten-minute slot"`: Slot (`Jan 2 15:04`) · Accepted · Dequeued · Dropped · Expired · Refused, 144 rows |

| Series | Field | Style |
|---|---|---|
| Accepted | `in` | solid blue `#1d5fa8` |
| Dequeued | `out` | dashed orange `#8a5000` |
| Dropped | `dropped` | solid red `#a8271b` |
| Expired | `expired` | dashed red |
| Refused | `refused` | dotted blue |

<details><summary>Chart geometry (face-computed)</summary>

| Element | Rule |
|---|---|
| Axis | path `M50 25 V125 H610`; labels `{max}` at (10,30) and `0` at (34,130) |
| X of slot i | `50 + 560·i/(n−1)`; 330 for a single slot |
| Y | `125 − 100·value/max`, `max` shared over all five series |
| Ticks | at every slot whose minute is 00: `M{x} 125 v4`; a `HH:MM` label at y=145 when the hour is divisible by 3 |
| Lines | one `<polyline>` per series whose day maximum > 0, points to one decimal |

</details>

Popover `activity-help`: the last 24 hours in ten-minute slots of the node's clock, 00:00 … 23:50, saved across restarts · down time reads as zero · the last slot is still counting · unfiltered Refused is node-wide for the daemon Owner and covers visible records for others · Dequeued is not completion.

### States

| State | Shown |
|---|---|
| All series zero | no chart; `All five series: <strong>0</strong> in the last day.` |
| Daemon returned no slots | `<p class=muted>The daemon answered no activity.</p>` |
| `/ls` or `/activity` refused | whole-page problem page (`404` for an unknown or hidden name) |

### Controls by role

Same for every role.

### Forms

`<form method=get>` (action is the current page)

| `name` | Type | Default | Meaning |
|---|---|---|---|
| `name` | `<select data-submit-on-change>` inside `<label>Service or channel …</label>` | the option equal to the current `name` is `selected` | first option `value=""` **All visible**, then every `/ls` name in name order (users, agents, groups, services, queues, topics) |

`<noscript><button>Apply</button></noscript>`. `/ui.js` submits on change. No daemon call beyond reloading the page.

TypeScript face, 0.8.41: Day · Week · Month tabs (`range=`), an anchor date
`at=yymmdd`, ‹ Prev and Next › and a Today link, on this page and in every
record detail's Activity card; see [activity history](../05-discovery.md#activity-history).
Week draws hourly lines and a 7×144 grid, Month day bars and a 30-day calendar,
each row or cell a link to its day. The dates shown are the page's title, in bold (`Activity · Sep 17 – 23`, `Last 24 hours` for the live day); the page says nothing about slots.

TypeScript face: each option also names that record's hits — everything its
five series counted in the last day, as `name (hits)` — `jobs@dev (14)` —
from one `GET /activity?name=` per visible active record; an inactive one
reads `name (inactive)`, and **All visible** carries the unfiltered total.
A record with no hits is left out — there is nothing to draw — unless it is
the one chosen.

### Links out

Navigation only.

### Differences

| Older doc | Code |
|---|---|
| `Plans/R0.8-MVP/web-handoff/pages.md#activity-activity`: about 145 readings; a restart empties history; `¿` marks unobserved slots | 144 fixed slots, saved across restarts; down time is `0`; no `¿` |
| `docs/web-face/site-map.md`: "reset on restart" | saved across restarts (`docs/05-discovery.md#activity-history` agrees with the code) |
| Select label "Service or channel" | also lists users, agents and groups |

## `/diagnostics`

### Route

| Method | Path | Params | Anchors |
|---|---|---|---|
| GET | `/diagnostics` | none | `#refusals`, `#stuck`, `#exchanges`, `#loss`, `#leftovers`, `#message-{id}` per exchange row |

Links here: navigation entry 9; Overview refusal items (`#refusals`); `/users?kind=other` (`303` → `#leftovers`); user pages' `return=/diagnostics`.

### Access

| Caller | Difference |
|---|---|
| No cookie | sign-in page with `200` and **no** message, `return=/diagnostics` |
| Daemon Owner | exchanges cover the whole node (`/recent` answers the node) |
| Others | exchanges only for messages they were party to |
| Everyone | refusal counts are node-wide; held and loss tables cover visible active records |
| Leftovers | rows only when the daemon's `/users` answer holds non-`user` kinds; the removal link depends on `can_remove` |

### Daemon calls

| Call | Used for | On failure |
|---|---|---|
| `GET /status` | `you`; `refused` → Refusals | whole page fails |
| `GET /ls` | held inboxes, loss (active records only) | whole page fails |
| `GET /recent` | exchanges | section says `Envelope history unavailable: {message}` |
| `GET /users` | leftovers: entries whose `kind` is not `user` | section omitted, no notice |

### Content

| # | Section | Content |
|---|---|---|
| 1 | Title and heading | `Diagnostics · agent-bus`; `<h1>{magnifier SVG} Diagnostics</h1>` + help "Caller-visible queues, loss and retained envelope evidence. Bodies are never shown." |
| 2 | Refresh | `<p><a href=/diagnostics>Refresh</a></p>` |
| 3 | Refusals `#refusals` | `fit-table`, caption `Refusals since this daemon started, by reason`: Reason (`<code>`) · Count |
| 4 | Inboxes holding messages `#stuck` | caption `Inboxes holding messages, longest wait first — visible to you`: Name · Readers · Held now · Oldest held · Capacity |
| 5 | Exchanges in retained history `#exchanges` | table `class=exchanges`, caption `Messages and explicitly referenced receipts`: Observed message · Route and conversation · Envelopes · Evidence |
| 6 | Loss by name `#loss` | Name · Dropped · Expired |
| 7 | Leftover names `#leftovers` | only while one exists: Name · What it is · Next step |

<details><summary>Refusals</summary>

Rows: every reason the daemon defines (`api.Reasons()`: `acl`, `busy`,
`credential`, `enrolment`, `full`, `malformed`, `name-taken`,
`second-reader`, `suspended`, `unknown`), with missing ones as a measured `0`,
plus any reason the daemon reports that the face does not know. Sorted by
count descending, then reason. Help: whole-node handled API refusals since
process start; zero is measured; router misses and internal failures excluded.

</details>

<details><summary>Inboxes holding messages</summary>

Rows: `/ls` records with `queued` > 0, sorted by `oldest` descending (parsed as
a Go duration; unparseable sorts as zero), then `queued` descending, then name.

| Column | Source |
|---|---|
| Name | `<a href="{detail path}?name={name}"><code>{name}</code></a>` |
| Readers | `readers`, or `unavailable` when absent |
| Held now | `queued` |
| Oldest held | `oldest`, or muted `—` |
| Capacity | `<b class=warn>at capacity when observed</b>` when `at_bound`, else muted `—` |

Empty: one row `every queue you can see is empty`.

</details>

<details><summary>Exchanges</summary>

Face-computed from `/recent` envelopes (`exchanges`). Bodies are never read.

| Step | Rule |
|---|---|
| Rows | every non-receipt envelope is a row |
| Folding | a receipt joins its original only when `re` names exactly one retained original, it travels the original's reply route, and it is not earlier than it. Anything else is its own row with a notice |
| Reply route | `reply_to` if set, else back to `from` with the same topic and tag. A response matches when its `from` is the original's `to` and `to`, topic and tag match that route |
| Matches | for a tagged non-receipt row, every earlier non-receipt whose route it answers. `Late` when exactly one matches and this row is after its deadline |
| Order | newest activity (row time or latest folded receipt) first, then ID |

| Column | Content |
|---|---|
| Observed message | `<time>{at, 2006-01-02 15:04:05Z07:00}</time><br><code>{message_id}</code>` |
| Route and conversation | `{from} → {to}`; `Topic: {topic or not supplied} · Tag: {tag or not supplied}`; with reply-to, `Reply route: …` |
| Envelopes | 1 + folded receipts |
| Evidence, receipt row | `<strong>{ack or done} receipt</strong> about <code>{re}</code>` and one notice: `Receipt has no original message reference.` · `Original message not in this visible history.` · `Reference found; route or time differs from the original. Kept separate.` · `Reference is ambiguous in this history. Kept separate.` · `The referenced message is itself a receipt. Kept separate.` |
| Evidence, message row | `Acknowledgement observed.` (if an ack folded); `Completion receipt observed.` or muted `No completion receipt observed in retained history.`; `Possible response — matching earlier routes:` links to `#message-{id}`; warn `After the matching message's deadline.`; `<details><summary>Receipt evidence</summary>` per folded receipt: type, ID, re, route, time, `after the request's deadline` |

Empty: `No envelopes in your retained history. This is not a count of all traffic.`
On a phone the rows become cards with each cell's `data-label`.

</details>

<details><summary>Loss and leftovers</summary>

Loss rows: `/ls` records with `dropped + expired` > 0, by that sum descending,
then name. Name links to detail. Empty: `nothing lost`.

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
| `/recent` refused | `<p class=warn>Envelope history unavailable: {daemon message or "the daemon did not answer"}</p>` |
| No leftovers, or `/users` failed | the section is absent |
| `/status` or `/ls` fails | problem page |

### Controls by role

None. Leftover links go to the people pages, which own the removal.

### Links out

Refresh; record detail per name; `#message-` anchors; `/user?name=…&return=/diagnostics`.

### Differences

| Older doc | Code |
|---|---|
| `Plans/R0.8-MVP/web-handoff/pages.md#diagnostics-diagnostics`: sections are exchanges, refusals and losses | also inboxes holding messages and leftovers; order is refusals, inboxes, exchanges, loss, leftovers |
| same: demote the Envelopes column and the repeated "No completion receipt" line | both still on every row |
| `docs/05-discovery.md#overview-and-diagnostics`: no Refresh link (owner, 0.5.83) | Diagnostics still has `Refresh`; Overview has none |

## `/healthz`

| Heading | Value |
|---|---|
| Route | `GET /healthz`; other methods `405` plain text |
| Access | anyone |
| Daemon calls | none; it does not ask the daemon, so it says only that the web process answers |
| Content | `200`, empty body, the standard security headers |

## Static assets

| Route | Access | Daemon call | Response |
|---|---|---|---|
| `GET /ui.js` | anyone | none | the change-submit script ([shell § assets](shell.md#assets)) |
| `GET /favicon.svg` | anyone | `/identity` (unused) | SVG, `no-store` |
| `GET /favicon.ico` | anyone | `/identity` (unused) | `404`, plain-text body `404 page not found` |
| `GET /agent-bus.jpg` | anyone | none | JPEG 648×432, `max-age=86400` |

## Redirects and catch-alls

Record-page aliases (`/channels`, `/channels/new`, `/channel`, `/channel/edit`)
are specified with the record pages.

| From | To | Status | Rule |
|---|---|---|---|
| `GET /users?kind=other` | `/diagnostics#leftovers` | `303` | before the sign-in check; any other `kind` is ignored |
| Daemon `GET /` on its TCP API port (exact root) | the dashboard URL, default `http://127.0.0.1:6780/` | `301` | served by `agent-busd`, set with `-dashboard` / `AGENT_BUS_DASHBOARD`; empty disables it |
| Any unmatched GET path | — | `200` | rendered as `/`: landing when signed out, Overview when signed in |
| Unmatched non-GET on a known path | — | `405` | plain text |

## Inconsistencies worth fixing in the rewrite

| Where | Behaviour | Evidence |
|---|---|---|
| Diagnostics | **an exchange with `reply_to` stops the page.** The template reads `.Service` on `ReplyTo`, which has `Name`; the render fails at the `Reply route:` cell, so the rest of the table, Loss, Leftovers and the footer are missing, with `200` | reproduced: `render: … can't evaluate field Service in type *protocol.ReplyTo` |
| Diagnostics | held and loss tables use `/ls` only, so an inactive record holding work is on Overview but absent here | `bobq` inactive with 1 held |
| Diagnostics | a `/users` failure silently drops Leftovers, against "a failed section is not an empty one" | code |
| Diagnostics | a receipt sent after consuming a queue is never folded: it comes from the consumer, not the queue, so every such ack/done is "route … differs" | reproduced with `ack`/`done` on a `jobs` message |
| Sign-in | an unreachable daemon is reported as `that credential was not accepted` | reproduced with a web face on a missing socket |
| Sign-in | a refused sign-in answers `200`, while an ended session answers `401` | reproduced |
| Overview, Diagnostics | a failed `/status` renders the problem page with an empty account name | reproduced (suspended user, `502`) |
| Activity | an inactive record's name gives a whole-page `404`; the select also cannot offer inactive records | reproduced |
| Overview | Uptime tile reads `/status`, footer reads `/identity`: two readings of one fact on one page | code |
