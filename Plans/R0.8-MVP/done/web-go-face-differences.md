# Web face: Go face and older-spec differences

📌 **TL;DR:** History, not a current contract. When the web face was specified
for its rewrite from Go to TypeScript, each spec file recorded where the Go
face disagreed with older docs and what was worth fixing. The TypeScript face
fixed those items ([behaviour changes](../web/README.md#behaviour-changes)) and
the Go face was removed in 0.8.50. The page contract is now
[the web face site map](../../../docs/web-face/site-map.md#every-address); this
file keeps the removed lists and the Go-era dashboard prose of
[discovery](../../../docs/05-discovery.md#dashboard), one section per source file.

## site-map.md

### Worth fixing in the rewrite

Each spec file ended with what its author found wrong or inconsistent in the Go
version; those lists are the sections below.

## shell.md

### Differences from older docs

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

## node.md

### Landing and sign-in

| Older doc | Code |
|---|---|
| `Plans/R0.8-MVP/web-handoff/pages.md#landing-and-sign-in-`: acquisition help mentions the SSH onboarding path | the popover states two commands and the non-expiry note only |

### POST /signin

| Older doc | Code |
|---|---|
| — | a bus that is down is reported as `that credential was not accepted` (see below) |

### Overview

| Older doc | Code |
|---|---|
| `Plans/R0.8-MVP/web-handoff/pages.md#overview-`: heading has observation time and a Refresh | no time or Refresh on the page; the footer states `Generated` |
| same: "Unregistered credentials awaiting review" item, blue | not built; leftovers appear only on Diagnostics |
| `docs/05-discovery.md#overview-and-diagnostics`: Find links the two holding-work views "and nothing else" | a third link, `External services` → `/services` |
| `Plans/R0.8-MVP/web-handoff/glyphs.md#where-a-glyph-is-allowed`: one severity glyph per attention item | no glyph; the level is a coloured left border only |

### Activity

| Older doc | Code |
|---|---|
| `Plans/R0.8-MVP/web-handoff/pages.md#activity-activity`: about 145 readings; a restart empties history; `¿` marks unobserved slots | 144 fixed slots, saved across restarts; down time is `0`; no `¿` |
| `docs/web-face/site-map.md`: "reset on restart" | saved across restarts (`docs/05-discovery.md#activity-history` agrees with the code) |
| Select label "Service or channel" | also lists users, agents and groups |

### Diagnostics

| Older doc | Code |
|---|---|
| `Plans/R0.8-MVP/web-handoff/pages.md#diagnostics-diagnostics`: sections are exchanges, refusals and losses | also inboxes holding messages and leftovers; order is refusals, inboxes, exchanges, loss, leftovers |
| same: demote the Envelopes column and the repeated "No completion receipt" line | both still on every row |
| `docs/05-discovery.md#overview-and-diagnostics`: no Refresh link (owner, 0.5.83) | Diagnostics still has `Refresh`; Overview has none |

### Inconsistencies worth fixing in the rewrite

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

## records.md

### Differences from the older specs

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

## people.md

### Groups: hidden owner

Intended: Owner and Maintainers read muted “Not visible to you” when the record
is not in `/ls`. **Built:** that branch never fires (Go `with` over a zero
struct is true), so a hidden record shows an empty owner and None.

### Avatar

Signed-in only (401 sign-in HTML otherwise). Runs `load`; name not in the
caller's `/users` → **404 text/plain** `no such user`. Photo: `image/png`
bytes. No photo: `image/svg+xml`, 32×32 rounded square `#e5eaf4` with the
initial in `#253c66`. **No page references it**: pages inline the photo as a
data URI.

### Older docs that disagree

| Doc | Says | Code |
|---|---|---|
| [pages](../web-handoff/pages.md#user-username), [forms](../web-handoff/forms.md#the-set) | Lifecycle Pause/Ban/Activate; confirm for ban | Active/Inactive only: Deactivate… (confirmed), Reactivate (direct) |
| pages § Groups, forms | group delete: “the handler accepts it” | `action=delete` → 400 local refusal |
| [discovery](../../../docs/05-discovery.md#dashboard), pages § Groups | Group registration entry and route only for Administrators / conditional | offered to every signed-in user; the daemon decides |
| forms § remaining work | Activate offered to an already-active user | only the applicable transition is offered |
| pages § User | state in the identity section | separate Access section |

### Worth fixing in the rewrite

| Defect | Where |
|---|---|
| Owner/Maintainers “Not visible to you” never renders; a hidden group shows a blank owner and None | `/groups` |
| Registering an existing name you may edit silently replaces its members | `POST /groups` |
| Secret or description failure after a create says “That request was not understood … Nothing was sent to the daemon” although it was saved | `POST /groups` |
| Transfer offered for `@<user>/…` groups the daemon never transfers | Danger Zone |
| Redirects use the typed name; mixed case lands on 404 | create user, save group |
| Only `name` on 412 is ever marked invalid; `email` wiring is dead; Personal-naming refusal marks `members` | forms |
| `refresh-github` has no control; its summary links to a missing `#form-refresh-github` | `POST /user` |
| Users cannot edit their own email though the daemon offers `can_set_email` and `POST /profile` | user page |
| Filter counts ignore the search; All (N) counts every state | `/users` |
| Ordinary users get two pills, “👤 User” and “User”; Account prints raw `active` and a fixed “👤 User” | user page, Account |
| Account omits inactive owned records that the user page lists | Account |
| Group-in-group membership is labelled `ACL` | Used by visible records |
| `/avatar` is unreferenced and costs three daemon reads | `/avatar` |
| `/users?kind=other` redirects before sign-in | `/users` |

## 05-discovery.md

### Personal tab (0.8.9–0.8.10)

From 0.8.9 a shared list that is empty only because its records of that kind are
Personal says how many are under the **Personal** tab and links there, rather
than claiming there are none; which records each tab holds is unchanged.
From 0.8.10 each section's **Personal** tab counts that section's kind and
opens the Personal page narrowed to it (`/personal?kind=queue`); the Personal
page itself holds every kind and has a Kind filter.

### Agent, service and channel journeys

Two- and three-value URL filters are visible links whose active state and plain
values remain in the URL. Two-value creation choices are labelled radio
buttons. An owned row on any registry page uses a blue leading rule and blue
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

The Services, Queues, PubSub, Personal and Users collections keep their definitions
behind a visible `ⓘ` button using the browser's native popover. Service detail
uses the same pattern for Delivery, Policy, Queue & counters and Activity: hovering or
focusing the adjacent button shows the explanation immediately, while clicking
opens the structured list. The button has an accessible name and the panel uses
a heading and short list. Current scope, counts, filters, form constraints,
refusals and dangerous consequences remain visible where they affect a decision.

### Compact administration pages

**Built in 0.5.73.** Service and Channel registration, User detail, Groups,
Diagnostics and record detail share the same cards, responsive field grids and
line-list textareas. Current facts, form labels, errors and actions stay visible.
Definitions and caveats that do not change the immediate decision use the
adjacent `ⓘ` control: hover or keyboard focus shows them immediately and click
opens the structured native popover.

Record detail presents Delivery, Policy and Queue & counters as one compact
fact row, followed by Activity, and links to the settings form rather than
carrying it; configuration, transfer and removal remain in the red Danger Zone.

**Adding an entity and editing one are the same form**, one page per kind
and one field set rendered by both — for a 👤 user and a 👥 group as much as
for a record
([forms](../web-handoff/forms.md#rules)) — so a 👾 and a 📮 declare the TTL,
capacity and overflow of the inbox they hold, a 📣 declares a
[Deliver-To list](../../../docs/04-messaging.md#subscribers) and no queue policy, and a 📡, a
👾 and a 👥 offer a field for their [secret](../../../docs/06-services.md#secrets), written by a
second call to that verb and never filled in again. **From 0.8.5 every settings
form offers every field the daemon lets that kind's manager change**
([record fields](../../../docs/constitution.md#common-record-fields)): a 👤 user's own inbox
its description and queue policy and no allow list or Maintainers, which the
daemon refuses on it; a 👥 group its description, members, Personal,
Maintainers and secret, saved in one daemon change, with owner transfer and
configuration in its Danger Zone and no removal, a group being retired by
emptying it. Status stays a separate action beside each record, and a group has
none: no view shows an inactive group to reactivate it from. What differs between the
two is what is already in the form: a field the caller may not change is shown
disabled rather than hidden, and the form states separately that it carried the
owner-only fields, so a Maintainer saving a description cannot clear what it
was not offered. User detail states the
profile beside identity, authority, groups, lifecycle and owned resources, and
links to the profile form. Groups use a compact Group/Members table; selecting
a name opens one group, its description and a Danger Zone for its managers, and the way to its settings form appears only when
the caller may change it — as does the form itself, which refuses whoever the
link was withheld from.
Diagnostics keeps refusal, held-work, retained-envelope and loss evidence
without duplicating the full registry catalogue or its former paragraph walls.
The Services, Queues and PubSub tables carry their own accepted/dequeued counters.

Human-facing integer counts use grouped decimal figures, including the compact
footer, registry, diagnostics and Activity totals. JSON, URLs, form values and
editable syntax remain unchanged plain values. The Overview node strip is
divided in two: how the node stands right now, then what has happened since it
started. A strip figure of none is a dash rather than a zero — the same answer,
written so that a quiet node does not read as a page of readings to check. A
table column keeps the plain number, where a dash would break the alignment
that makes the column scannable.

### Rules it is built to: no CDN

| Rule | Why |
|---|---|
| **One repository-owned script; no CDN or external asset** | The local script only submits marked selectors on change. It reads no page data, stores nothing and makes no request of its own. Pages remain ordinary URL-backed forms, with a `noscript` Apply control. Graphs stay inline SVG; avatars and the anonymous page's project picture are served from this node, never hotlinked. The page's own `img-src 'self'` enforces it: a picture named anywhere else is refused by the browser, and the page renders as though it had none |

### Form recovery

[F.13.2](web-shell-recovery.md#checks) completed the shared shell and recovery;
the page redesign and typed-component migration then still stood pending. The
TypeScript face replaced both.
