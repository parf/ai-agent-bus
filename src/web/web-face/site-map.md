# Web face site map

📌 **TL;DR:** The page contract of the web face (`agent-bus-web`, TypeScript
in `src/web`, port 6780). This index lists every address it answers and links
the page that specifies it: route and parameters, access by position, daemon calls,
content, states, controls, form fields by name and links out. The process, its
account and its unit belong to [processes § the web face](../../../docs/11-processes.md#the-web-face).

## The specification

| File | Owns |
|---|---|
| [shell](shell.md) | What every page shares: process model, session and cookie, origin checks, headers, frame, problem page, refusal routing, form recovery, assets, glyphs, layout, paging, `return` |
| [node](node.md) | Landing and sign-in, Overview, Activity, Diagnostics, liveness, assets, redirects |
| [records](records.md) | Agents, Services, Queues, PubSub and Resources, each with its Personal filter: lists, register, detail, settings, deactivate, Danger Zone |
| [people](people.md) | Users, Groups (with their locks) and Account |

Every page section has the same parts, in order: Route, Access, Daemon calls,
Content, States, Controls by position, Forms (every field's `name`, type, required,
prefill, allowed values; hidden fields; the daemon call; the redirect; how a
refusal shows) and Links out. A part with nothing to say is omitted.

## Every address

| Address | What it is | Specified in |
|---|---|---|
| `/` — homepage for an unregistered visitor | Landing page: picture, what agent-bus is, four feature cards, project links and sign-in by token | [/` signed out: landing and sign-in](node.md#-signed-out-landing-and-sign-in) |
| `POST /signin`, `POST /signout` | Start and end a browser session | [POST /signin](node.md#post-signin), [POST /signout](node.md#post-signout) |
| `/` — homepage for a signed-in user | Overview: attention items, the node strip, today's traffic, the Find row | [/` signed in: Overview](node.md#-signed-in-overview) |
| `/activity` | Day, Week or Month of activity for the visible records or one record | [/activity](node.md#activity) |
| `/diagnostics` | Refusals, held inboxes, retained exchanges, loss, leftover names | [/diagnostics](node.md#diagnostics) |
| `/agents` `/services` `/queues` `/pubsub` `/resources` | One kind's records; `?personal=1` shows its Personal ones | [Lists](records.md#lists) |
| `/agents/new` `/services/new` `/queues/new` `/pubsub/new` `/resources/new` | Register a record of that kind | [Register](records.md#register) |
| `/agent` `/service` `/queue` `/pubsub/topic` `/resource` `?name=` | One record; an inactive one is read-only with Reactivate | [Record detail](records.md#record-detail), [Inactive record view](records.md#inactive-record-view) |
| `/agent/edit` `/service/edit` `/queue/edit` `/pubsub/topic/edit` `/resource/edit` | Settings for the Owner or a Maintainer | [Settings](records.md#settings) |
| `/service-deactivate` | Confirm deactivating a record | [Deactivate](records.md#deactivate) |
| `/service-danger` | Danger Zone: deactivation, configuration, transfer, removal | [Danger Zone](records.md#danger-zone) |
| `POST /service`, `POST /service-confirm` | Every record change, and confirmed transfer and removal | [POST /service](records.md#post-service), [POST /service-confirm](records.md#post-service-confirm) |
| `/locks` | Every lock on the records the visitor may use, with a kind filter and search; `?kind=` and `?q=` | [Locks](records.md#locks) |
| `POST /release-lock` | Release a lock on a record's page; `force=1` releases another's | [Locks](records.md#locks) |
| `/kv` | Every key-value store the visitor may use, with a kind filter and search | [Key-value stores](records.md#key-value-stores) |
| `/kv/record?name=` | One record's store: strings, integers and JSON, with Add value | [Key-value stores](records.md#key-value-stores) |
| `/kv/value?record=&kind=&name=` | One value, to edit, count or delete | [Key-value stores](records.md#key-value-stores) |
| `POST /kv-set`, `/kv-inc`, `/kv-delete` | Write, count and delete a value | [Key-value stores](records.md#key-value-stores) |
| `/users` | The Users directory | [Users `/users](people.md#users-users) |
| `/users/new` | Register a user | [Register user `/users/new](people.md#register-user-usersnew) |
| `/user?name=` | One user | [User `/user?name=](people.md#user-username) |
| `/user/edit?name=` | Profile editor | [Edit user `/user/edit?name=](people.md#edit-user-usereditname) |
| `POST /user` | Create or save a profile, own email, GitHub refresh, user state, credential removal | [POST /user](people.md#post-user) |
| `/user-danger`, `/user-deactivate`, `/credential-remove` | User Danger Zone and deactivation confirmation, or credential removal | [User](people.md#a-user-kind-user), [Confirm deactivation `/user-deactivate?name=](people.md#confirm-deactivation-user-deactivatename), [Confirm credential removal `/credential-remove?name=](people.md#confirm-credential-removal-credential-removename) |
| `/groups`, `/groups/new` | Groups, with `?personal=1`, and registering one | [Groups `/groups](people.md#groups-groups), [Register group `/groups/new](people.md#register-group-groupsnew) |
| `/group?name=`, `/group/edit?name=` | One group, and its editor | [Group `/group?name=](people.md#group-groupname), [Edit group `/group/edit?name=](people.md#edit-group-groupeditname) |
| `POST /groups` | Create or save a group | [POST /groups](people.md#post-groups) |
| `/service-danger?name=<group>` | A group's Danger Zone | [Group Danger Zone](people.md#group-danger-zone) |
| `/account` | Your identity, owned records and credentials | [Account `/account](people.md#account-account) |
| `/palette.json` | The `⌘K` palette's names, the visitor's own view; JSON `401` signed out, `403` cross-site | [interactive features](../../../Plans/R0.8-MVP/web/README.md#interactive-features) |
| `/healthz`, `/favicon.svg`, `/favicon.ico`, `/app.<hash>.css`, `/ui.<hash>.js`, `/agent-bus.<hash>.webp` | Liveness and assets | [/healthz](node.md#healthz), [Static assets](node.md#static-assets) |
| `/channels`, `/channel`, other old addresses | Redirects kept for bookmarks | [Redirects and catch-alls](node.md#redirects-and-catch-alls) |
| `/_styleguide` | With `AGENT_BUS_WEB_DEV=1`: every component; `404` otherwise | [W.2](../../../Plans/R0.8-MVP/web/TODO.md#steps) |
| Any other path | Framed `404 No such page`; a known path asked with the wrong method is a framed `405` | [shell § problem page](shell.md#problem-page) |

## Shared rules

[Process model](shell.md#process-model) · [Session](shell.md#session) · [Signed-out requests](shell.md#signed-out-requests) · [Origin checks](shell.md#origin-checks) · [Security headers](shell.md#security-headers) · [Page frame](shell.md#page-frame) · [Titles and help](shell.md#titles-and-help) · [Problem page](shell.md#problem-page) · [Refusal routing](shell.md#refusal-routing) · [Form recovery](shell.md#form-recovery) · [Assets](shell.md#assets) · [Glyphs](shell.md#glyphs) · [Narrow screens and zoom](shell.md#narrow-screens-and-zoom) · [Pagination](shell.md#pagination) · [Return addresses](shell.md#return-addresses)

What the Go face and older specs got differently is
[history](../../../Plans/R0.8-MVP/done/web-go-face-differences.md#site-mapmd).
