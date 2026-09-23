# Web face site map

📌 **TL;DR:** An internal working document, not part of the linked docs set:
the reference for rewriting the web face from Go to TypeScript. It lists every
address the admin web face (`agent-bus-web`, port 6780) answers, what each page
is for and where it links, as built in 0.8.10. No other document links here,
and it owns no contract: what a page shows is owned by
[pages](../../Plans/MVP/web/pages.md#pages), its fields by
[forms](../../Plans/MVP/web/forms.md#the-set), and the contract by
[discovery § dashboard](../05-discovery.md#dashboard).

## Every page

Signed out, every address leads to the sign-in page. Signed in, every page has:

- **Header:** logo, the release and the node's realm, then the navigation
  🏠 Overview · 👾 Agents · 📡 Services · 📮 Queues · 📣 PubSub · 👤 Users ·
  👥 Groups · Activity · Diagnostics, which wraps onto several rows on a phone.
  On the right, your name (→ `/account`) and **sign out** (`POST /signout`).
- **Footer:** the daemon Owner, uptime and when the page was generated.
- A skip link to `#main`. `/ui.js` adds progressive enhancement: a sort
  select submits on change, for example.

## Pages

| Address | Page | Links to |
|---|---|---|
| `/` | Signed out: a landing page (the project's description, links and picture) with sign-in by token at its foot. Signed in: [Overview](../../Plans/MVP/web/pages.md#overview-): attention items, the node strip (eight tiles, one per record kind, readers and calls) and a Find row | Find: agents and queues holding work (`?sort=queued&work=held`), `/services` |
| `/agents` · `/services` · `/queues` · `/pubsub` | [One kind's records](../../Plans/MVP/web/pages.md#agents-agents--services-services--queues-queues--pubsub-pubsub): search, Status, Readers and Queue filters, Sort. All and My omit Personal records; an empty shared list says how many Personal ones the Personal tab holds | Tabs All · My (`?scope=my`) · Personal (`/personal?kind=<this kind>`, counting that kind) · Register; each row → its record page |
| `/personal` | Personal records of every kind, or one with `?kind=agent`, `service`, `queue` or `pubsub`, for each owner you may act for, with the same toolbar and a Kind filter | Owner chooser; rows → record pages |
| `/agents/new` · `/services/new` · `/queues/new` · `/pubsub/new` | [Register a record](../../Plans/MVP/web/pages.md#register-agentsnew--servicesnew--queuesnew--pubsubnew) of that kind. An agent's name is `#name[@realm]` | Back to its list |
| `/agent?name=` · `/service?name=` · `/queue?name=` · `/pubsub/topic?name=` | [One record](../../Plans/MVP/web/pages.md#agent-agentname--service-servicename--queue-queuename--topic-pubsubtopicname): status, policy, queue and counters, activity, route, settings. An inactive record shows read-only with Reactivate | Owner → `/user`; Activity → `/activity?name=`; Settings; Deactivate…; Danger Zone |
| `/agent/edit` · `/service/edit` · `/queue/edit` · `/pubsub/topic/edit` | Settings: the registration form filled in, plus Maintainers, for the Owner or a Maintainer | Back to the record |
| `/service-deactivate?name=` | Confirm deactivating a record | Cancel → the record |
| `/service-danger?name=` | [Danger Zone](../05-discovery.md#resource-danger-zone): replace configuration, transfer ownership, remove. A group offers configuration and transfer only; a prefixed group and `@administrators` offer no transfer | Back to the record |
| `/groups` | Groups you may see, with owner, Maintainers and member counts | Tabs All · Register group; rows → `/group` |
| `/groups/new` | [Register a group](../../Plans/MVP/web/pages.md#register-group-groupsnew); any User may and becomes its Owner. A Personal group must be named `@<you>/<name>` | Back to Groups |
| `/group?name=` | [One group](../../Plans/MVP/web/pages.md#groups-groups--group-groupname): members, Owner, Maintainers, records using it | Edit group; members and records → their pages; Danger Zone |
| `/group/edit?name=` | Members, description, Personal, Maintainers and secret, as the viewer may change them | Back to the group |
| `/users` | [Users only](../../Plans/MVP/web/pages.md#users-users): User · Authority · Contact · Agents · Last used; an inactive User is struck and marked beside the name. Search and Status (Active · Inactive · All states) | Tabs All · Register user; rows → `/user`. `?kind=other` → `/diagnostics#leftovers` |
| `/users/new` | [Register a user](../../Plans/MVP/web/pages.md#register-user-usersnew): a profile; SSH keys are added on the host | Back to Users |
| `/user?name=` | [One user](../../Plans/MVP/web/pages.md#user-username): profile, identity, groups, access, owned records | Edit profile; groups and records → their pages; Deactivate (Administrators, for another user) |
| `/user/edit?name=` | Profile editor | Back to the user |
| `/account` | [Your account](../../Plans/MVP/web/pages.md#account-account): identity, owned records, the credentials you hold with fingerprint, issued and last used | Records → their pages; credential removal |
| `/activity` | [Activity charts](../05-discovery.md#activity-history) per visible record, about a day at ten-minute samples, reset on restart | Back to a record from `?name=` |
| `/diagnostics` | [Diagnostics](../05-discovery.md#overview-and-diagnostics): refusals, inboxes holding messages, retained exchanges (metadata, never bodies), message loss, and Leftover names (`#leftovers`) only while one exists | Leftover names → removal review |

## Other routes

| Route | Does |
|---|---|
| `POST /signin`, `POST /signout` | Start and end a session |
| `POST /service` | Every record change: `create`, `save`, `configure`, `deactivate`, `reactivate` |
| `POST /service-confirm` | Confirmed changes: `transfer`, `delete` |
| `POST /groups` | Create or save a group |
| `POST /user` | Create or save a user profile |
| `GET /credential-remove` | Confirm removing a credential |
| `GET /user-deactivate` | Confirm deactivating a user |
| `GET /avatar?name=` | A profile photo |
| `GET /healthz`, `/ui.js`, `/favicon.svg`, `/favicon.ico` | Liveness and assets |
| `GET /channels`, `/channels/new` | Redirect to `/queues` or `/pubsub` (`kind=pubsub`), keeping the query |
| `GET /channel?name=`, `/channel/edit?name=` | Redirect a visible queue or topic to its `/queue` or `/pubsub/topic` address; anything else is answered as before |
