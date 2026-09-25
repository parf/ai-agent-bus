# Web face shell

📌 **TL;DR:** The rules every page of the web face (`agent-bus-web`, `src/web`)
follows: the process and its one daemon connection, the session cookie,
sign-in in place of a refused page, origin checks, headers, the page frame,
problem pages, form recovery, assets, glyphs, layout, paging and `return`
addresses. The account, unit and confinement belong to
[processes § the web face](../11-processes.md#the-web-face).

## Process model

The web face is a stateless HTTP renderer. It keeps no credential, no session
map and no cache. Every page is built from daemon answers fetched for that
request with the visitor's session.

| Fact | Value |
|---|---|
| Program | `bun run server.ts`; `--version` prints the version. Configured from its environment only, no flags |
| Daemon client | `fetch` over the Unix socket (host `unix`), or `http://host:port`; 2-minute timeout |
| Credential sent | header `X-Agent-Bus-Token: <value>`: the typed token once, on `POST /session`; the session id from the cookie on every other call. No call is made without a session; the face answers the sign-in page instead |
| Anonymous call | `GET /identity`, with no header, once per rendered page; 3-second timeout. Failure leaves the node facts empty and is never an error. Liveness, assets and redirects make no daemon call |
| Registration header | `POST /register` also sends `If-None-Match: *` |
| Body limit | a POST body over 1 MiB gets plain-text `413` |
| `HEAD` | answered as `GET`, without the body |
| Mutations | every write is a daemon call made with the visitor's session. The face never retries as anybody else |

| Variable | Meaning | Default |
|---|---|---|
| `AGENT_BUS_WEB_ADDR` | listen `host:port` | `127.0.0.1:6780` |
| `AGENT_BUS_WEB_CERT`, `AGENT_BUS_WEB_KEY` | serve HTTPS; either without the other, or a missing file, stops the start rather than serving plain HTTP | none: plain HTTP |
| `AGENT_BUS_ADDR` | the daemon: a socket path or `http://host:port` | `$XDG_RUNTIME_DIR/agent-bus/bus.sock`, else `/tmp/agent-bus-<uid>.sock`; the unit gives the shared socket |
| `AGENT_BUS_WEB_DEV` | `1` serves `/_styleguide` | off |

## Session

| Rule | Value |
|---|---|
| Cookie name | `agent_bus_session` |
| Value | the hex session id from `POST /session`, never the token. Any other value reads as no session |
| Attributes | `Path=/; HttpOnly; SameSite=Strict`, plus `Secure` when serving TLS. No `Max-Age` or `Expires`: a browser-session cookie |
| Lifetime | the daemon's idle timeout, 30 minutes since last use. A daemon restart, removing the credential, unregistering or the ownerless sweep ends it. A web-face restart does not |
| Sign-out | `DELETE /session`, then the cookie cleared with the same attributes and `Max-Age=0` |
| Validity check | none in the face: the first daemon call answers `401` if the session has ended |

<details><summary>Other cookies</summary>

None carries authority.

| Cookie | Set by | Holds |
|---|---|---|
| `ab_theme` | the browser script | `dark` or `light`; anything else follows the system |
| `ab_sidebar` | the browser script | `collapsed` or `open` |
| `ab_flash` | a `303` after a successful change (`HttpOnly; SameSite=Strict; Max-Age=60`) | one allowlisted message key, shown once as a toast by the next page, which clears it |

</details>

## Signed-out requests

There is **no redirect**. A signed-out request gets the sign-in page at the
address that was asked for, and the form carries that address back in `return`.

| Case | Status | Message under the form |
|---|---|---|
| No session, `/` | `200` | none: the landing page |
| No session, any other page | `401` | `sign in to open this page` |
| Session sent, daemon answers `401` | `401` | `that session has ended — sign in to carry on` |
| Unknown path | `404` | not the sign-in page: the framed [not-found page](#problem-page) |

<details><summary>How <code>return</code> is chosen on the sign-in page</summary>

| Request | `return` |
|---|---|
| GET, path not `/` | the path and query |
| GET `/` | empty, so the hidden field is omitted |
| POST | the submitted `return`, else the Referer's path and query when its host is this host; `/` is omitted |

</details>

## Origin checks

Every POST, `/signin` and `/signout` included, must come from a page of this
face. Anything else gets a plain-text `403 same-origin form required` with no
frame, before routing.

| Header | Rule |
|---|---|
| `Origin` | required; `<scheme>://<Host header>` with `https` under TLS and `http` otherwise, and no user, path or query |
| `Sec-Fetch-Site` | absent or `same-origin` |

No CSRF token exists. `SameSite=Strict` and the CSP `form-action 'self'`
complete the defence.

## Security headers

Set on every response:

| Header | Value |
|---|---|
| `Cache-Control` | `public, max-age=31536000, immutable` on hashed assets; `no-store` on everything else, `/favicon.svg` included |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `same-origin` |
| `Content-Security-Policy` | generated from `src/web/assets.ts`: `default-src 'none'`; `script-src` and `style-src` `'self'` plus each pinned CDN file's exact URL; `font-src` each font package's `files/` path; `img-src 'self' data:`; `connect-src 'self'`; `form-action 'self'`; `frame-ancestors 'none'`; `base-uri 'none'` |
| `Content-Type` | `text/html; charset=utf-8` on pages; per asset below |

No inline style or script is allowed: the page's CSS and script are
same-origin files. The design and its reasons are in
[Web § Content-Security-Policy](../../Plans/R0.8-MVP/web/README.md#content-security-policy).

## Page frame

Every HTML page is one document: head, then for a signed-in visitor a sidebar
and a top bar, `<main id=main>`, and the footer. The sign-in page has a bare top
bar with the brand and the theme toggle, and no sidebar.

<details><summary>Head</summary>

| Element | Value |
|---|---|
| Doctype, root | `<!doctype html><html lang=en>`, with `data-theme` from `ab_theme` and `data-sidebar=collapsed` from `ab_sidebar` when set |
| Meta | `charset=utf-8`; `viewport` `width=device-width,initial-scale=1`; `color-scheme` `dark light` |
| Title | `<title>{page title} · agent-bus</title>` |
| Icon | `/favicon.svg?v=<hash>` |
| Stylesheets | the pinned CDN fonts (Geist, JetBrains Mono), uPlot's stylesheet on chart pages only, each with `integrity` and `crossorigin=anonymous`; then `/app.<hash>.css` |
| Scripts | `defer`: Lucide from the CDN, uPlot on chart pages only, each with `integrity`; then `/ui.<hash>.js` |
| Theme | dark first, light its equal; the system preference picks unless `ab_theme` says otherwise, and the server writes `data-theme` so the first paint is right |

</details>

### Header

`<a class=skip-link href=#main>Skip to main content</a>` comes first.

| Part | Content | Source |
|---|---|---|
| Brand (sidebar; sign-in top bar) | logo, `AgentBus`, `v{version}` with `title="Build daemon {build_info}"` when a build string exists (`v unavailable` without a version), `@ {host}` (`@ host unavailable`); `node unavailable` in place of both when the call failed | `/identity` |
| Sidebar | the nine entries below; a collapse button keeps it to icons | fixed |
| Top bar | drawer button (narrow screens), the palette button `Jump to… ⌘K`, theme toggle, the account link (avatar and `{you}`, `aria-current=page` on `/account`) → `/account`, and a sign-out button in `<form method=post action=/signout class=who>` | `/status` `you` |

| # | Entry | Path | Mark | Keys |
|---|---|---|---|---|
| 1 | Overview | `/` | Lucide `layout-dashboard` | `g o` |
| 2 | Agents | `/agents` | 👾 | `g a` |
| 3 | Services | `/services` | 📡 | `g s` |
| 4 | Queues | `/queues` | 📮 | `g q` |
| 5 | PubSub | `/pubsub` | 📣 | `g p` |
| 6 | Users | `/users` | 👤 | `g u` |
| 7 | Groups | `/groups` | 👥 | `g g` |
| 8 | Activity | `/activity` | Lucide `activity` | `g t` |
| 9 | Diagnostics | `/diagnostics` | Lucide `scan-search` | `g d` |

The current section gets `aria-current=page`; a user inbox marks Queues, and a
problem page marks none. While a page has the Personal filter on, the Agents,
Services, Queues, PubSub and Groups entries keep `?personal=1` and show a lock.
`⌘K` / `Ctrl-K` opens the command palette, `/` focuses the page's search and
`?` lists the shortcuts ([interactive features](../../Plans/R0.8-MVP/web/README.md#interactive-features)).

### Footer

`<footer class=site-footer aria-label="Node and build information">`:

| Case | Content |
|---|---|
| Identity answered | `Owner <code>{owner}</code>` (`unavailable` if empty) · `Uptime {up}` (`unavailable` if empty) · `Generated {time}` |
| Identity failed | `Node information unavailable` · `Generated {time}` |

`Generated` is the web process's local time when the request began, formatted
`2006-01-02 15:04:05`, with no zone. No page refreshes itself.

## Titles and help

| Rule | Detail |
|---|---|
| Document title | `{title} · agent-bus`. Fixed per page, or built from daemon data (record, user, group names) so two tabs differ. Problem pages use the problem title |
| Page heading | `<header class=page-head>`: optional back link, then `<h1>` with a decorative mark (`aria-hidden`) and the title, the help button, page actions; an optional sub line |
| Help button | `<button type=button class=help-button popovertarget={id} aria-label="{short}" data-tooltip="{sentence}">` with a Lucide `circle-help` icon |
| Tooltip | CSS: hover or keyboard focus shows `data-tooltip`, or `aria-label` when there is none |
| Popover | `<div popover id={id} class=context-help>` with a heading and a list; the native popover opens on click |

## Problem page

One template for every whole-page failure, in the frame (signed in or not):
a mark, the status code, an `<h1>` title, the daemon's message in `<p class=warn>`
when there is one, the advice, and a way back.

| Trigger | Status | Title | Daemon message shown | Advice (verbatim) |
|---|---|---|---|---|
| Daemon unreachable (transport error) | `502` | The bus is not answering | no; logged | The dashboard is running; the daemon behind it is not reachable, so there is nothing to show and nothing was changed. It comes back on its own when the daemon does. |
| `401` | `401` | — | — | not a problem page: the sign-in form, message `that session has ended — sign in to carry on` |
| `403`, message starts `user access is suspended` | `403` | Your access is suspended | yes | You are signed in, and every call is refused until your user is active again — signing in again would change nothing. An administrator of this node is who can lift it. |
| `403`, other | `403` | Not yours to see | yes | You are signed in, and refused for lack of permission rather than for want of a credential — signing in again would change nothing. Its owner, or a maintainer of it, is who can grant this. |
| `404` | `404` | No such name | **no** | Either nothing is registered under that name or it is not one you may see. Those are deliberately the same answer, so this does not tell you which. |
| `503` | `503` | The bus is busy | yes | The daemon is there and briefly cannot answer. Nothing was changed; the same request is worth making again. |
| `500` | `500` | Something went wrong in the daemon | yes | This is a fault, not a rule: repeating it is unlikely to help, and the daemon's log on this node is where it is recorded. |
| Any other (`400`, `409`, `412`, `429`, …) | same | That was refused | yes | Nothing was changed. The reason above is the daemon's own. |
| Face-side rejection (`LocalProblem`) | as given, mostly `400` | That request was not understood | the face's own sentence | Nothing was sent to the daemon. Return to the page and use the action shown there. |
| Saved in part (`LocalProblem`, saved) | `502` | That request was not understood | the face's sentence, saying what was saved | Part of it was saved, as the sentence above says. Return to the page to finish it. |
| Danger Zone re-check failed (`ConditionsChanged`) | `409` | The conditions changed | the face's sentence | The daemon was re-read before the action. Review the current record and confirm again if the action still applies. |
| Unknown address | `404` | No such page | — | Nothing on this dashboard answers at that address. The sections are in the sidebar; a record is reached from its list. |
| Known address, wrong method | `405` | Not answered this way | — | This address exists, but not for that kind of request. Open it as a page, or use the form that sends it. |
| A fault in the face | `500` | Something went wrong in the dashboard | no; logged | This is a fault in the web face, not a daemon refusal; its log on this node records it. |

| Way back | Rule |
|---|---|
| `409` conditions changed | `Review the current Danger Zone` |
| `404` | `Back to Overview` → `/` (`Back to the start` on the unknown-address page when signed out) |
| other GET | `Try again` → the same URI |
| other POST | `Back to the page` → the Referer, when it is this host and not `/`; else none |

**The daemon's message** is the `error` field of its JSON body
(`{"error":"…"}`), else the trimmed raw body. Transport errors never reach a
page, because their text names the socket or address.

<details><summary>Face-built refusals that reuse the daemon table</summary>

Some pages decide a permission from a daemon field and render the matching
problem page themselves, without a daemon call failing. These are the rows
`403 Not yours to see` and `404 No such name` above. The record, user and
group pages list their own cases.

</details>

## Refusal routing

| Helper | Used for | Behaviour |
|---|---|---|
| `problemPage` | a whole page cannot be built | the problem page table above |
| `sectionProblem` | one section failed and the rest is true | the daemon's message, or `the daemon did not answer` (logged) for a transport error, shown in place of that section |
| `formRefusal` | a form submission was refused | `preserve` for `400`, `404`, `409`, `412` and `429`: the form comes back with that status. Other codes and transport errors go to the problem page |
| `LocalProblem` | input the face rejected before any call | the "not understood" page. It never claims the daemon refused |
| Plain text | foreign-origin POST (`403`), body too large (`413`), `/favicon.ico` (`404`) | `text/plain` body, no frame |

A record or group form also returns to the form, whatever the code, when the
daemon's message names one submitted line of a list field (`lineRefusal`): the
daemon's own `Line N:` wins, otherwise a line counts only where the message
names it as a term. The records and people specifications own the fields.

## Form recovery

A refused submission whose target can still be rendered comes back as the same
form, with the refusal's status code.

| Part | Rule |
|---|---|
| Retained values | an explicit allowlist per form. Tokens, secrets and private configuration are never retained; a replacement textarea is always empty |
| Summary | `<section class=form-error role=alert aria-labelledby=form-error-title>` with `<h2 id=form-error-title>Check this form</h2>`, the message, and `<a href="#form-{action}">Review the submitted fields</a>` |
| Field | when the face can tell which field without guessing from prose: `aria-invalid="true"` and `aria-describedby` pointing at the message. Every other field has `aria-invalid="false"` |
| List fields | numbered line-list textareas; a `Line N:` refusal marks that line |
| Target gone | a missing or newly hidden target falls back to `404 No such name` |

## Assets

| Path | Type | Behaviour |
|---|---|---|
| `/app.<hash>.css` | `text/css; charset=utf-8` | the stylesheet, `src/web/style/*.css` joined; immutable |
| `/ui.<hash>.js` | `text/javascript; charset=utf-8` | the browser script from `src/web/client/ui.ts`: icons, theme, sidebar, palette, shortcuts, copy buttons, toasts, charts, and submit on change of a `[data-submit-on-change]` control. It requests only `/palette.json`; immutable |
| `/agent-bus.<hash>.webp` | `image/webp` | the landing picture; immutable |
| `/favicon.svg` | `image/svg+xml; charset=utf-8` | a bus on a dark rounded square; `no-store`, linked with `?v=<hash>` |
| `/favicon.ico` | `404 text/plain` | so the catch-all does not answer it with a page |
| CDN | — | Geist and JetBrains Mono fonts, Lucide icons and uPlot, each pinned to an exact version and hashed ([external assets](../../Plans/R0.8-MVP/web/README.md#external-assets)); the visitor's browser needs internet access for them |

Assets answer `GET` and `HEAD` without a daemon call; another method gets the
framed `405`. User photos are inline `data:` URLs.

## Glyphs

The canonical entity and authority glyphs are `src/internal/display`, shared
with the CLI; `src/web/glyphs.ts` carries the same table, and a test fails when
the two drift.

| Kind or authority | Glyph | Label |
|---|---|---|
| user (also `person`) | 👤 | 👤 User |
| agent | 👾 | 👾 Agent |
| queue | 📮 | 📮 Queue |
| pubsub | 📣 | 📣 PubSub |
| service | 📡 | 📡 Service |
| group | 👥 | 👥 Group |
| daemon Owner | 🔱 | 🔱 Daemon owner. It replaces the entity mark on the owner's own row |
| Maintainer | 👮 | the `👮 Maintainers:` label on record and group detail |
| Authority words | — | `🔱 Daemon owner`, `Daemon administrator`, `User` |
| Unknown kind | none | the kind word passes through unmarked |

| Rule | |
|---|---|
| Other marks | page and section marks that are not a kind or an authority are Lucide icons |
| Default | no glyph beyond these; one per cell, always beside a word |
| Source | a glyph follows a daemon-stated kind or flag, never a name |
| Machine values | URLs, JSON, ACL text and form values stay plain |
| States | status is a pill with a dot, `Active` or `Inactive`; an inactive record's heading and its name in Users, Diagnostics and owned-record lists also carry the word badge `INACTIVE` |
| Numbers | grouped with commas. The Overview strip writes zero as a muted `—`; tables keep `0` |

## Narrow screens and zoom

A page never scrolls sideways at 1440, 768 or 390 px.

| Width | Changes |
|---|---|
| ≤ 1100 px | detail pages go to one column |
| ≤ 860 px | the sidebar becomes a drawer opened from the top bar |
| ≤ 760 px | stacked tables become cards with a `data-label` prefix per cell and a visually hidden header |
| ≤ 640 px | node-strip tiles take half the width |

## Pagination

| Rule | Value |
|---|---|
| Page size | 25 rows (record lists and the user directory) |
| Parameter | `page`, 1-based. Missing, invalid or out of range is clamped to 1…last |
| Links | `<nav aria-label="Record pages">` (or `"Directory pages"`): `Previous page`, `Page N of M`, `Next page`, each link only when it exists. Links keep every filter |
| Summary | `Showing {start}–{end} of {matched} matching records, …`, plus `Clear filters` when a filter is set |
| Counts | tab counts are totals before filters, not page counts |
| Source | the face pages one caller-visible `/ls` + `/inactive` answer. The daemon has no paging |

## Return addresses

Every `return` value is made local first: it must start with `/`, not `//`,
and parse with no scheme, host or user. Anything else becomes `/`.

| Where | Allowed | Fallback |
|---|---|---|
| `POST /signin` | any local path and query | `/` |
| Record detail and settings | only the record's own list path (`/agents`, `/services`, `/queues`, `/pubsub`), with its query, `?personal=1` included; fragment dropped | the record's list, with `?personal=1` for a Personal record |
| User pages | `/users` or `/diagnostics`, with query; fragment dropped. After a state change, the user's own page | `/users` |
| Problem-page back link | the Referer when it is this host | none |

## Differences from older docs

History: what the Go face and older docs said differently is in
[web-go-face-differences § shell](../../Plans/R0.8-MVP/done/web-go-face-differences.md#shellmd).
