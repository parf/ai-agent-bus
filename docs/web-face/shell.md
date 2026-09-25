# Web face shell

📌 **TL;DR:** Internal working document for the TypeScript rewrite, not
linked from the docs set. It states the rules every page of `agent-bus-web`
follows, as built in 0.8.31: the process and its one daemon connection, the
session cookie, sign-in in place of a refused page, origin checks, the page
frame, the problem pages, form recovery, headers, assets, glyphs, layout,
paging and `return` addresses. Derived from `src/cmd/agent-bus-web/*.go` and
checked against a disposable daemon; where older docs disagree the code wins,
and the [differences](#differences-from-older-docs) are listed at the end.

## Process model

The web face is a stateless HTTP renderer. It keeps no credential, no session
map and no cache. Every page is built from daemon answers fetched for that
request.

| Fact | Value |
|---|---|
| Binary | `agent-bus-web`; supervised as a child when `agent-busd -web` is given, or run standalone |
| Listen | `-addr` / `AGENT_BUS_WEB_ADDR`, default `127.0.0.1:6780` |
| TLS | `-cert` + `-key` (`AGENT_BUS_WEB_CERT`, `AGENT_BUS_WEB_KEY`). Both files must exist. One named but missing makes the process exit rather than serve plain HTTP |
| Daemon address | `AGENT_BUS_ADDR`: a Unix socket path, or `http://host:port`. Empty means the default socket (`$XDG_RUNTIME_DIR/agent-bus/bus.sock`, else `/tmp/agent-bus-<uid>.sock`). The supervised child is given the shared socket only |
| Client | HTTP/1.1 over the socket, host `unix`, 2-minute timeout |
| Credential sent | header `X-Agent-Bus-Token: <value>`. The value is the typed token once, on `POST /session`, and the session id from the cookie on every other call. No call is made with an empty credential. The face refuses locally with `sign in required` instead |
| Anonymous call | `GET /identity`, sent with no header on every request except `/healthz`, `/ui.js`, `/avatar`, `/signout` and `/agent-bus.jpg`. 3-second timeout. Failure leaves the node facts empty; it is never an error |
| Registration header | `POST /register` with a body also sends `If-None-Match: *` (record pages) |
| Body limit | each POST body is capped at 1 MiB |
| Mutations | every write is a daemon call made with the visitor's session. The face never retries as anybody else |

<details><summary>Confinement</summary>

Owned by [processes § the web face](../11-processes.md#the-web-face) since
0.8.50: its own `agent-bus-web` account and locked-down systemd unit, the
shared socket only, no credential of its own, no writable path, and a unit
that may execute bun and nothing else. The Go face's supervised bubblewrap
child is gone.

</details>

## Session

| Rule | Value |
|---|---|
| Cookie name | `agent_bus_session` |
| Value | the session id from `POST /session` (`{"session": "<hex>", "idle": "30m0s"}`), never the token |
| Attributes set at sign-in | `Path=/; HttpOnly; SameSite=Strict`, plus `Secure` only when serving TLS. No `Max-Age` and no `Expires`: a browser-session cookie |
| Lifetime | the daemon's idle timeout, 30 minutes since last use (`auth.IdleLife`). A daemon restart, removing the credential, unregistering, or the ownerless sweep ends it. A web-child restart does not |
| Sign-out cookie | `agent_bus_session=; Path=/; Max-Age=0`, with no other attribute |
| Validity check | none in the face. The cookie is checked by being used: the first daemon call answers `401` if it has ended |

## Signed-out requests

There is **no redirect**. A signed-out request gets the sign-in page at the
address that was asked for. The form carries the address back in `return`.

| Case | Status | Message under the form | `return` value |
|---|---|---|---|
| No cookie, `/` or `/diagnostics` (the `loadView` path) | `200` | none | the request URI; nothing for `/` |
| No cookie, any other page (the `signedIn` path) | `401` | `sign in to open this page` | the request URI |
| Cookie sent, daemon answers `401` | `401` | `that session has ended — sign in to carry on` | GET: the request URI. POST: the Referer if it is this host |
| Any unknown GET path, signed out | `200` | none | that path |

<details><summary>How <code>return</code> is chosen on the sign-in page</summary>

| Request | `return` |
|---|---|
| GET, path not `/` | `RequestURI()` (path and query) |
| GET `/` | empty, so the hidden field is omitted |
| POST | the Referer's path and query when its host equals the request host, else `/` (then omitted) |

</details>

## Origin checks

Two layers. Both answer a plain-text `403 same-origin form required` with no
shell.

| Layer | Applies to | Rule |
|---|---|---|
| Global | every POST | if an `Origin` header is present, it must be same-origin. An absent `Origin` passes |
| Mutation | every form action except `/signin` and `/signout` (`POST /service`, `/service-confirm`, `/groups`, `/user`) | `Origin` must be present and same-origin |

**Same-origin** means: `Origin` parses as `<scheme>://<Host header>` with
the scheme `https` under TLS and `http` otherwise, with no user, path, query or
fragment; and `Sec-Fetch-Site` is absent or `same-origin`. No CSRF token
exists. `SameSite=Strict` and the CSP `form-action 'self'` complete the
defence.

## Security headers

Set on every response, before routing:

| Header | Value |
|---|---|
| `Cache-Control` | `no-store`, except `/agent-bus.jpg`, which overrides it with `max-age=86400` |
| `X-Content-Type-Options` | `nosniff` |
| `Content-Security-Policy` | `default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; img-src 'self' data:; form-action 'self'; frame-ancestors 'none'; base-uri 'none'` |
| `Content-Type` | `text/html; charset=utf-8` on pages; per asset below |

The CSP means: one same-origin script, inline `<style>` only, images from this
face or `data:` URLs (user photos are `data:image/png`), forms post only here,
no framing.

## Page frame

Every HTML page is one document: the shared head, the header, `<main>`, the
footer. The whole stylesheet is inline in every page.

<details><summary>Head</summary>

| Element | Value |
|---|---|
| Doctype, lang | `<!doctype html><html lang=en>` |
| Meta | `charset=utf-8`; `viewport` `width=device-width,initial-scale=1` |
| Icon | `<link rel=icon href=/favicon.svg type="image/svg+xml">` |
| Script | `<script defer src=/ui.js></script>` |
| Style | one inline `<style>` block; design tokens on `:root`: `--surface-1 #fbfbf9`, `--surface-2 #f2f2ee`, `--surface-3 #e8e7e2`, `--border #d2d0c9`, `--border-strong #87847b`, `--text-1 #1a1a17`, `--text-2 #56544c`, `--accent #1d5fa8`, `--red #a8271b`, `--orange #8a5000`, `--green #2d6a3f`. Light theme only |
| Title | `<title>{page title} · agent-bus</title>` |

</details>

### Header

`<a class=skip-link href=#main>Skip to main content</a>` comes first, then
`<header class=site-header aria-label="Site header">`:

| Part | Content | Source |
|---|---|---|
| Logo | inline SVG bus, `class=node-logo`, `aria-hidden`, 80×58 (shown 64 px, 48 px on a phone), mirrored red double-decker | fixed markup |
| Release | `<strong>AgentBus <span>v{version}</span></strong>`. The span has `class=build-tip tabindex=0 title="Build daemon {build_info}"` when a build string exists. `v unavailable` when version is empty | `/identity` `version`, `build_info` |
| Host | `<span>@ {host}</span>`, or `@ host unavailable` | `/identity` `host` |
| No identity | `<strong>AgentBus</strong> <span>node unavailable</span>` in place of both | `/identity` failed |
| Account (signed in only) | `<form method=post action=/signout class=who>` holding `<a class=account-link href=/account><code>{you}</code></a>` and `<button type=submit>sign out</button>`. The link has `aria-current=page` on `/account` | `/status` `you` |
| Navigation (signed in only) | `<nav aria-label="sections">`, entries below | fixed |

The sign-in page carries the skip link, logo, release and host, and no account
or navigation.

| # | Entry | Path | Mark | Current on |
|---|---|---|---|---|
| 1 | Overview | `/` | 🏠 | Overview |
| 2 | Agents | `/agents` | 👾 | agent list, agent pages, **Personal** pages |
| 3 | Services | `/services` | 📡 | service pages |
| 4 | Queues | `/queues` | 📮 | queue pages, user inbox records |
| 5 | PubSub | `/pubsub` | 📣 | pub/sub pages |
| 6 | Users | `/users` | 👤 | user pages |
| 7 | Groups | `/groups` | 👥 | group pages |
| 8 | Activity | `/activity` | inline SVG line chart | Activity |
| 9 | Diagnostics | `/diagnostics` | inline SVG magnifier | Diagnostics |

The current entry gets `aria-current=page`. A problem page marks none. Each
mark is `<span class=page-title-mark aria-hidden=true>` (or an SVG of that
class) directly before the label. The entries wrap onto more rows; they never
scroll sideways.

### Footer

`</main>` then `<footer class=site-footer aria-label="Node and build information">`:

| Case | Content |
|---|---|
| Identity answered | `Owner <code>{owner}</code>` (`unavailable` if empty) · `Uptime {up}` (`unavailable` if empty) · `Generated {time}` |
| Identity failed | `Node information unavailable` · `Generated {time}` |

`Generated` is the web process's local time when the request began, formatted
`2006-01-02 15:04:05`, with no zone. No page refreshes itself.

## Titles and help

| Rule | Detail |
|---|---|
| Document title | `{title} · agent-bus`. Fixed per page, or built from daemon data (record, user, group names) so two tabs differ. Problem pages use the problem title. Both not-found pages share `No such name` |
| Page heading | `<div class=page-title><h1>{mark} {title}</h1>{help button}</div>`. The mark is decorative (`aria-hidden`) |
| Help button | `<button type=button class=help-button popovertarget={id} aria-label="{short}" data-tooltip="{sentence}">ⓘ</button>` |
| Tooltip | pure CSS: hover or keyboard focus shows `data-tooltip`, or `aria-label` when there is none, beside the button |
| Popover | `<div popover id={id} class=context-help><h2>{heading}</h2><ul><li>…</li></ul></div>`: the native popover opens on click |
| Section headings | `<h2>` inside the same `page-title` wrapper when they carry help |

## Problem page

One template for every whole-page failure: the signed-in shell, then an `<h1>`
with the warning-triangle SVG and the title, the daemon's message in
`<p class=warn>` when there is one, the advice in `<p>`, and a way back.

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
| Face-side rejection (`localProblem`) | as given, mostly `400`; `502` for "saved but secret not stored" | That request was not understood | the face's own sentence | Nothing was sent to the daemon. Return to the page and use the action shown there. |
| Danger Zone re-check failed (`conditionsChanged`) | `409` | The conditions changed | the face's sentence | The daemon was re-read before the action. Review the current record and confirm again if the action still applies. Back link: `Review the current Danger Zone` |

| Way back | Rule |
|---|---|
| GET | `<a href="{same URI}">Try again</a>` |
| POST | `<a href="{Referer path}">Back to the page</a>` when the Referer is this host and not `/`; else none |

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
| `fail(you, err)` | a whole page cannot be built | the problem page table above |
| `sectionProblem(what, err)` | one section failed and the rest is true | returns the daemon's message, or `the daemon did not answer` (logged) for a transport error. The page shows it in place of that section |
| `formRefusal(err)` | a form submission was refused | returns `(code, message, preserve)`. `preserve` is true for `400`, `404`, `409`, `412` and `429`: the form is re-rendered with that status. Other codes (`403`, `500`, `503`, transport) go to `fail` |
| `localProblem(code, detail)` | input the face rejected before any call | the "not understood" page. It never claims the daemon refused |
| Plain `http.Error` | foreign-origin POST; unknown `/avatar` name; `/favicon.ico`; wrong method on a known path | `text/plain` body, no shell |

A record or group form also returns to the form, whatever the code, when the
daemon's message names one submitted line of a list field (`lineRefusal`).
The records and people specifications own that list.

## Form recovery

A refused submission whose target can still be rendered comes back as the same
form, with the refusal's status code.

| Part | Rule |
|---|---|
| Retained values | an explicit allowlist per form. Tokens, secrets and private configuration are never retained; a replacement textarea is always empty |
| Summary | `<section class=form-error role=alert aria-labelledby=form-error-title><h2 id=form-error-title>Check this form</h2><p>{message}</p><p><a href="#form-{action}">Review the submitted fields</a></p></section>` |
| Field | when the face can tell which field without guessing from prose: `aria-invalid="true"` and `aria-describedby` pointing at the message. Every other field has `aria-invalid="false"` |
| Field inference | `412` on create marks `name`; a message naming a line of a list field marks that field and quotes the line |
| Target gone | a missing or newly hidden target falls back to `404 No such name` |

## Assets

| Path | Type | Behaviour |
|---|---|---|
| `/ui.js` | `text/javascript; charset=utf-8` | the only script. On any `change` event, the nearest `[data-submit-on-change]` control's form is submitted with `requestSubmit()`. It reads nothing and requests nothing. Every such form also has `<noscript><button>Apply</button></noscript>` |
| `/favicon.svg` | `image/svg+xml; charset=utf-8` | a 32×32 bus on a dark rounded square, drawn separately from the logo. `no-store` |
| `/favicon.ico` | `404 text/plain` | exists only so the catch-all does not answer it with a page |
| `/agent-bus.jpg` | `image/jpeg`, `Cache-Control: max-age=86400` | the 648×432 landing picture (121,675 bytes), embedded in the binary. Served without calling the daemon |
| `/avatar?name=` | PNG or SVG | a signed-in user's photo (people pages) |

No external asset, font or CDN. Page-title marks, the logo and graphs are
inline SVG or emoji.

## Glyphs

Owned by `src/internal/display`, the only source of entity and authority
glyphs, shared with the CLI.

| Kind or authority | Glyph | Label (`display.Entity`) |
|---|---|---|
| user (also `person`) | 👤 | 👤 User |
| agent | 👾 | 👾 Agent |
| queue | 📮 | 📮 Queue |
| pubsub | 📣 | 📣 PubSub |
| service | 📡 | 📡 Service |
| group | 👥 | 👥 Group |
| daemon Owner | 🔱 | 🔱 Daemon owner. It replaces the entity label on the owner's own row (`display.Identity`) |
| Maintainer | 👮 | the `👮 Maintainers:` label on record and group detail. The templates write it as a literal, not through `display` |
| Authority words | — | `🔱 Daemon owner`, `Daemon administrator`, `User` (`display.Authority`) |
| Unknown kind | none | the kind word passes through unmarked |

| Title mark (`titleMark`) | Mark |
|---|---|
| overview | 🏠 |
| credentials (sign-in) | 🔑 |
| identity | 🪪 |
| agents, services, queues, pubsub, users, groups | the entity glyph |
| activity, diagnostics, problem | fixed inline SVG |
| anything else | ⚙️ |

| Rule | |
|---|---|
| Default | no glyph. One glyph per cell, always beside a word |
| Source | a glyph follows a daemon-stated kind or flag, never a name |
| Machine values | URLs, JSON, ACL text and form values stay plain |
| Inactive | the word badge `INACTIVE` (`state-badge`), not a glyph |
| Numbers | `number()` groups thousands with commas. The Overview strip uses `figure()`: zero is a muted `—`. Tables keep `0` |

## Narrow screens and zoom

Breakpoints are in the inline stylesheet. A page never scrolls sideways at
desktop width, at 200% zoom or on a 420 px phone.

| Width | Changes |
|---|---|
| > 70rem | main column up to 104rem, centred; 1rem side padding |
| ≤ 70rem | record search toolbar goes to two rows; person and account grids go to one column; service fact cards go to two columns |
| ≤ 40rem | 48 px logo; navigation drops to its own block and wraps with 1rem gaps; account form on its own line; inputs 100% wide; `.record-table` rows become stacked cards with a `data-label:` prefix per cell and a visually hidden `<thead>`; node-strip tiles take half the width; form grids go to one column; any other table that is not `.fit-table` scrolls on its own; exchange rows become cards |

## Pagination

| Rule | Value |
|---|---|
| Page size | 25 rows (record lists and the user directory) |
| Parameter | `page`, 1-based. Missing, invalid or out of range is clamped to 1…last |
| Links | `<nav aria-label="Record pages">` (or `"Directory pages"`) with `Previous page` / `Next page` links, each shown only when it exists. Links keep every filter |
| Summary | `Showing {start}–{end} of {matched} matching records, …` plus `Clear filters` when a filter is set |
| Counts | section tab counts are totals before filters, not page counts |
| Source | the face pages one caller-visible `/ls` + `/inactive` answer. The daemon has no paging |

## Return addresses

Every `return` value passes `local()` first: it must start with `/`, not
`//`, and parse with no scheme, host or user. Anything else becomes `/`.

| Where | Allowed | Fallback |
|---|---|---|
| `POST /signin` `return` | any local path and query | `/` |
| Record detail, edit and deactivate (`recordReturn`) | only the record's own listing path (`/agents`, `/services`, `/queues`, `/pubsub`, `/groups`, or `/personal` for a Personal record), with its query; fragment dropped | that listing path |
| User pages (`directoryReturn`) | `/users` or `/diagnostics`, with query; fragment dropped | `/users` |
| Problem-page back link | the Referer when it is this host | none |

## Differences from older docs

| Older doc says | Code does |
|---|---|
| `docs/05-discovery.md#signing-in`: cookie is `Secure` | `Secure` only when serving TLS; plain-HTTP loopback cookies are not `Secure` |
| Brief for this spec: signed-out request redirects to `/` with `return=` | no redirect; the sign-in form is served at the requested address with a hidden `return` |
| `docs/05-discovery.md#what-a-node-says-about-itself`: header shows the realm | header shows `@ {host}` from `/identity`; there is no realm in the frame |
| `Plans/R0.8-MVP/web-handoff/glyphs.md#where-a-glyph-is-allowed`: absence marks `¿` and `∅` | never rendered; `—` and `unavailable` are the only absence marks |

<details><summary>Inconsistencies worth fixing in the rewrite</summary>

| Behaviour | Why it matters |
|---|---|
| `/` and `/diagnostics` answer a signed-out visitor `200` with no message; every other page answers `401` with `sign in to open this page` | two different sign-in answers for one situation |
| A status failure on `/`, `/diagnostics` and every `signedIn` page renders the problem page with an empty name: `<a class=account-link href=/account><code></code></a>` | the suspended page, and the 502 page, show a blank account link |
| Sign-out clears the cookie without `HttpOnly`, `SameSite` or `Secure` | harmless, but not the attributes it was set with |
| The global check lets a POST with no `Origin` through; only mutation handlers require one | `/signin` and `/signout` accept Origin-less posts, and rely on `SameSite=Strict` |
| `GET /identity` is fetched for `/favicon.svg`, `/favicon.ico` and every unknown path | an asset request costs a daemon call |
| An unknown GET path renders Overview (`200`) or the sign-in page, never `404` | typos look like the front page |
| Wrong-method requests (`POST /diagnostics`) get a plain-text `405` without the shell | the only unframed answers besides the origin refusal |
| `404 No such name` offers `Try again` to the same URL | retrying the same name cannot help |

</details>
