# People pages: users, groups, account

📌 **TL;DR:** Internal working spec for the TypeScript rewrite of the web face;
no other document links here and it owns no contract. It describes every
user, group and account page as the Go face (`src/cmd/agent-bus-web`:
`users.go`, `account.go`, `admin.go`) builds them in 0.8.x, checked against
HTML rendered by a disposable daemon. Where older docs disagree the code wins;
the differences, and the defects worth fixing in the rewrite, close the file.

## Shared rules

Every page here starts the same way; the per-page sections state only what
differs.

| Rule | Behaviour |
|---|---|
| Signed out | No `agent_bus_session` cookie: the sign-in page at the requested URL, **401**, with `return` = that URL (POST: the Referer if ours). The problem-page and sign-in markup belong to `shell.md` |
| Caller facts | `GET /status` on every page: `you`, `administrator`, `daemon_owner`, and the node status for the frame |
| Bus refusal → page | `401` sign-in form (“that session has ended — sign in to carry on”); `403` + message starting with the inactive error: *Your access is suspended*; other `403`: *Not yours to see*; `404`: *No such name* (daemon detail dropped: hidden and absent read the same); `503`: *The bus is busy*; `500`: *Something went wrong in the daemon*; `400/409/412/429`: *That was refused*. Transport failure: **502** *The bus is not answering*. GET problem pages carry **Try again** → same URL; POST ones **Back to the page** → Referer |
| Local refusal | Input the face rejects before calling: *That request was not understood*, the given detail, “Nothing was sent to the daemon…”, status as stated per case |
| POST guard | `Origin` must equal the page origin (scheme, host, port; no path) and `Sec-Fetch-Site` empty or `same-origin`, else **403 text/plain** `same-origin form required`. Body cap 1 MiB; unparseable form: local refusal 400 “The submitted form could not be read.” |
| Preserved refusal | A daemon refusal with code `400, 404, 409, 412, 429` re-renders the form **with that status**: alert summary *Check this form* + daemon message + link *Review the submitted fields* → `#form-<action>`, and the same message in `p.warn` under the fields. Other codes → problem page |
| Field error | Only the attributed field gets `aria-invalid="true"` and `aria-describedby` → the form's error id; every other field carries `aria-invalid="false"` |
| Secrets | A secret or configuration is never prefilled and never returned after a refusal. CRLF in a secret is stored as LF |
| `return` (user pages) | Kept only when it is a local URL whose path is `/users` or `/diagnostics` (query kept, fragment dropped); anything else becomes `/users` |
| Directory load (`load`) | User pages (not `/users/new`) call `GET /users`, then `GET /ls` and `GET /inactive` (every caller-visible record, inactive ones marked). Any failure → problem page |

### Roles

| Role | Where the face learns it |
|---|---|
| daemon Owner | `/status.daemon_owner`; per directory row `daemon_owner` |
| Administrator | `/status.administrator` (member of `@administrators`; the Owner always is); per row `administrator` |
| group Owner / Maintainer | the group record from `/ls`: `owner`, `maintainers`, `can_manage` (Owner, Maintainer, daemon Owner), `can_transfer` (record Owner or daemon Owner) |
| the user themself | row `name` = `you` |
| per-row authority | daemon-derived on each `/users` row: `can_edit` (daemon Owner edits anyone; an Administrator edits non-Administrators other than self), `can_activate` (= `can_edit`), `can_remove` (Administrator, and the name has neither profile nor other-owned record), `can_set_email` (self; unused by the face) |

`GET /users` answers **only the caller's own row** unless the caller is an
Administrator, who gets every name: Users (`kind: user`), self-owned records
without a profile (`kind: record`) and leftover credentials (`kind: credential`).

## Users `/users`

| Route | |
|---|---|
| Method, path | `GET /users` |
| `q` | Search text, trimmed; case-insensitive substring of name, person name, email, GitHub login, company, location, Twitter/X joined by spaces. Default empty |
| `state` | `active` (default; any other value), `inactive`, `all` |
| `page` | 1-based integer, 25 rows per page, clamped to `1…last`; junk reads as 1 |
| `kind` | `kind=other` → **303** `/diagnostics#leftovers`, before sign-in is checked. `kind=users` and anything else ignored |

| Access | |
|---|---|
| stranger | sign-in page, 401 |
| Administrator, daemon Owner | every User row; **Register user** tab |
| any other signed-in user | only their own row (daemon rule); no Register tab |

**Daemon calls:** `/status`, `/users` (rows and profile facts), `/ls` +
`/inactive` (agent counts, last-used times). The face filters to `kind: user`
rows and computes every count.

**Content, in order**

| Part | Content and source |
|---|---|
| Title | `Users · agent-bus`; h1 👤 Users + ⓘ help popover (four bullets: what a User is, Active/Inactive and struck names, Last used, face-computed counts) |
| Section nav | **All (N)** → `/users`, always current; N = all User rows in every state, before search. **Register user** → `/users/new` (Administrators only) |
| Search form | `GET /users`; `input type=search name=q id=record-query` value = `q`, placeholder “Search by name, identity, email or GitHub login”, visually hidden label “Search users”; `hidden name=state` only when state ≠ active; `noscript` **Search** button |
| Status filter | “Status”: **Active (a)** · **Inactive (i)** · **All states (n)**; current one `aria-current=true`. Counts ignore `q`; links keep `q`, drop `page` |
| Table | caption “Showing s–e of m matching users.”; columns below |
| Pager | `nav aria-label="Directory pages"`: **Previous page** / **Next page** links when they exist (URL: `q`, `state` if not active, `page`); the nav is always rendered, empty on one page |

| Column | Cell |
|---|---|
| User | photo (`data:image/png;base64` of `photo_png`, `alt=""`) or initial (first letter of person name, else name, upper-cased); person name in bold if set; identity glyph `span role=img aria-label` (“👤 User”, or 🔱 “Daemon owner”); name in `code` linked `/user?name=<name>&return=<this page URL>`; inactive: person name and name struck (`<s>`) + badge `INACTIVE`; company as a muted line |
| Authority | “🔱 Daemon owner” · “Daemon administrator” · “User” |
| Contact | email line; “GitHub `<login>`” line; neither: muted — |
| Agents (numeric) | agents in `/ls`+`/inactive` whose owner is this user; 0 muted |
| Last used | `last_used` of the record named like the user, as `time` (`datetime` ISO, `title` `YYYY-MM-DD HH:MM`) with a relative label: now · Nm ago · Nh ago · Nd ago (< 30 d) · `Jan 2` · `Jan 2, 2006`. None: inactive → muted — (`title` “Not reported while inactive”); active → muted `never` |

| State | Shows |
|---|---|
| no match with `q` | section “No user matches this search”, sentence “A User is a person on this node; every record belongs to one.”, **Clear the search** → `/users` |
| `state=inactive`, none | “No inactive users” + the sentence |
| otherwise empty | “No users yet” + the sentence + **Register a user** → `/users/new` (Administrators) |
| bus down | 502 problem page |

Controls are omitted, never disabled. Links out: tabs, filter links, row
links, pager, Clear the search, Register a user.

## Register user `/users/new`

| Route | |
|---|---|
| `GET /users/new` | no parameters |
| `GET /user` with empty or no `name` | the same page for an Administrator; anybody else gets 404 *No such name* |

| Access | |
|---|---|
| Administrator, daemon Owner | the form |
| other signed-in user | **403** *Not yours to see*, detail “only a daemon Administrator can register a user” |

**Daemon calls:** `/status` only (`/user` also loads the directory first).

**Content:** title `Add user · agent-bus`; **Back to directory** → `/users`;
h1 👤 Add user; error summary (on refusal); section **Profile** holding the
form; aside **🔑 SSH access**: “Public keys are added on the host after the
profile is saved.” + ⓘ popover naming `agent-bus-admin user add <user@realm> <key.pub>`.

**Form** `id=form-create`, `POST /user`. Hidden: `action=create`,
`return=/users`. Fields: [profile fields](#profile-fields). Button **Save
profile**. Submit: [POST /user](#post-user).

## User `/user?name=`

| Route | |
|---|---|
| `name` | Directory name, matched exactly against `/users` rows (the daemon lower-cases names, so a mixed-case URL is 404) |
| `return` | Back target, see [shared rules](#shared-rules); default `/users`. Diagnostics links here with `return=/diagnostics` |

| Access | |
|---|---|
| Administrator | any row |
| other user | own row only; any other name 404 |
| Profile editable | `can_edit` → Edit profile link; else read-only card |
| Access change | `can_activate` and target not daemon Owner |

**Daemon calls:** `load`. Shown facts come from the `/users` row, except
record kinds and paths (`/ls`+`/inactive`) and owned records (row `services`
filtered to caller-visible records, plus visible non-group records whose owner
is this user — so inactive ones appear; sorted).

### A User (`kind: user`)

Title `<name> · agent-bus`; **Back to directory** → `return`; h1 large photo
or initial + name. Two columns:

| Section | Content |
|---|---|
| Profile | `dl`, each pair only when set: Person name · Email · GitHub login (`@login` → `https://github.com/<login>`) · Company · Location · Twitter/X (`@handle` → `https://x.com/<handle>`). `can_edit`: link **Edit profile** `id=profile-edit` → `/user/edit?name=&return=`. Else ⓘ popover on profile fields + muted “Trusted profile fields are edited by a daemon administrator.” |
| Identity | pills: identity label (“👤 User” or “🔱 Daemon owner”), then the authority label when it differs (so an ordinary user shows “👤 User” and “User”) |
| Groups | chips → `/group?name=`, from row `groups` (direct and nested membership, daemon-computed); none: muted “No memberships” |
| Access | ⓘ help; “Current: ● Active” or “● Inactive”; **Change** disclosure (see access rule): inactive → POST form **Reactivate**; active → GET form **Deactivate…** → `/user-deactivate` |
| Owned records | links by kind (`/agent`, `/service`, `/queue`, `/pubsub/topic`, `/group`); none: muted “None” |

### A non-user identity (`kind: record` or `credential`, Administrators only)

h1 🪪 mark + name; meta: identity pill (record kind label) and “No user
lifecycle state”; ⓘ popover on credential removal. Section heading
**Registered name** (record: “A self-owned record exists. *Inspect its
registration and queues*” → record page, then each owned record) or
**Credential only** (“No registered record remains for this credential.”).
With `can_remove`: GET form → `/credential-remove`, hidden `name`, `return`,
button **Review credential removal…**.

| State | Shows |
|---|---|
| inactive user | Current Inactive; Reactivate; owned records still listed (from `/inactive`) |
| unknown or hidden name | 404 *No such name* |
| refused GitHub refresh | this page with summary, status of the refusal ([POST /user](#post-user)) |

## Edit user `/user/edit?name=`

| Route | `name` (exact), `return` as above |
|---|---|
| Access | row `kind: user` and `can_edit`: daemon Owner for anyone; Administrator for non-Administrators other than self. Else **403** “that profile cannot be edited by you”; name absent from `/users` → 404 |
| Daemon calls | `load` |

Title `Edit <name> · agent-bus`; **Back to `<name>`** → `/user?name=&return=`;
h1 photo/initial + “Edit `<name>`”; error summary. **Form** `id=form-save`,
`POST /user`, hidden `action=save`, `return`, `name`. Fields:
[profile fields](#profile-fields), prefilled from the row. Button **Save
profile**.

### Profile fields

One template for both forms.

| `name` | Label | Input | Required | Prefill | Placeholder, help | Who |
|---|---|---|---|---|---|---|
| `name` | Identity | text | yes | typed value on refusal | `user@realm`; “The principal name used by AgentBus. It cannot be changed afterwards.” | register only; on edit a hidden field with the row's name |
| `person_name` | Person name | text | no | row | “The name shown to people.” | form holder |
| `email` | Email | `type=email` | no | row | — | form holder |
| `github_user` | GitHub login | text | no | row | “Setting or changing it attempts to import public values.” | form holder |
| `company` | Company | text | no | row `github_company` | — | form holder |
| `location` | Location | text | no | row `github_location` | — | form holder |
| `twitter` | Twitter/X | text | no | row `github_twitter_username` | `handle` | form holder |

The error paragraph is `p.warn id=profile-error`. Only `name` can be marked
invalid (register, 412); `email` carries the wiring but nothing sets it.

## POST /user

| `action` | Daemon call (JSON body) | Success | Refusal |
|---|---|---|---|
| `create` | `POST /user` with the [profile body](#profile-body), `create: true` | **303** `/user?name=<typed name>` | preserved codes: Add user page again (Administrators; else problem page), status = code; `412` marks `name` |
| `save` | `POST /user`, profile body without `create` | **303** `return` | preserved codes: edit page, the row overlaid with typed values; no field marked |
| `inactive`, `active` | `POST /user/state` `{"name","status"}` | **303** `return` | always a problem page |
| `remove-credential` | `POST /identity/remove` `{"name"}` | **303** `return` | always a problem page; e.g. **409** “…credential is now backed by a user or a record; refresh the directory” |
| `refresh-github` | `POST /user/github-refresh` `{"name"}` | **303** `return` | preserved codes: the user page with summary. **No page renders this action** |
| anything else | none | — | local refusal **400** “That user action is not available.” |

### Profile body

`name`, `person_name`, `email`, `github_user`, `github_company`,
`github_location`, `github_twitter_username` from the form, plus
`profile_details_set: true` so empty company/location/Twitter clear them. The
Go struct also serialises `status: ""` and `kind: ""`; the daemon keeps the
stored status.

Retained on refusal: `name`, `person_name`, `email`, `github_user`, `company`,
`location`, `twitter`, `return` (no secrets exist here).

<details><summary>Daemon refusals seen on this form</summary>

| Input | Code, message |
|---|---|
| existing name | 412 `that name is already registered` |
| bad email | 400 `invalid user profile: invalid email` |
| `#x` | 400 `invalid user profile: #x is an agent's name, not a user's` |
| empty name | 400 `bad name: bad name "": a-z 0-9 . _ - + @ only, starting alphanumeric` |
| a user editing themself (not Owner) | 403 `that record belongs to someone else` (problem page) |
| deactivating the Owner | 403, same message |
| other | identifying-field clash, vouched realm (enrol only), GitHub login changed mid-fetch (409 busy) |

</details>

## Confirm deactivation `/user-deactivate?name=`

| Route | `name`, `return` |
|---|---|
| Access | row `kind: user`, not daemon Owner, `can_activate`, currently active. Else **403** “that user cannot be deactivated by you in their current state” (also for self, the Owner, an inactive user); absent → 404 |
| Daemon calls | `load` |

Title `Confirm deactivation · <name> · agent-bus`; **Back to user** →
`/user?name=&return=`; h1 ⚠ Confirm deactivation; “Deactivate `<name>`?”;
bullets: bus access stops and owned records become inactive; queued work and
tokens kept, processes not stopped. Then “Only the daemon Owner can reactivate
this Administrator later.” (target is an Administrator) or “An authorized
Administrator or the daemon Owner can reactivate this user later.”

Form `POST /user`: hidden `name`, `return`; button `name=action value=inactive`
**Deactivate user**; **Cancel** → `/user?name=&return=`. Reactivation has no
confirmation: the Access form posts `action=active` directly.

## Confirm credential removal `/credential-remove?name=`

| Route | `name`, `return` (Diagnostics passes `/diagnostics`) |
|---|---|
| Access | row not `kind: user` and `can_remove` (Administrators). Else **403** “that credential cannot be removed by you”; absent → 404 |
| Daemon calls | `load` |

Title `Confirm credential removal · <name>`; **Back to identity** → user page;
h1 ⚠; “Remove the credential for `<name>`?”; bullets: current and previous
credentials stop authenticating; every browser session ends; no user or record
is removed. Form `POST /user`: hidden `name`, `return`; button
`name=action value=remove-credential` **Remove credential**; **Cancel**.
Success → `return`.

## Avatar `/avatar?name=`

Signed-in only (401 sign-in HTML otherwise). Runs `load`; name not in the
caller's `/users` → **404 text/plain** `no such user`. Photo: `image/png`
bytes. No photo: `image/svg+xml`, 32×32 rounded square `#e5eaf4` with the
initial in `#253c66`. **No page references it**: pages inline the photo as a
data URI.

## Groups `/groups`

| Route | `GET /groups`, no parameters |
|---|---|
| Access | any signed-in principal; stranger 401 sign-in |
| Daemon calls | `/status`; `GET /groups` (every live group name, members only where readable); `GET /ls` (group records the caller may see: member or manager) |

| Part | Content |
|---|---|
| Title | `Groups · agent-bus`; h1 👥 Groups + ⓘ (tooltip and five bullets: `@owner` is ACL syntax; anyone may register; who edits; who sees what) |
| Section nav | **All (N)** (all group names) · **Register group** → `/groups/new`, **for everybody** |
| Table | caption “N groups”; rows by name |

| Column | Cell |
|---|---|
| Group | 👥 (`role=img aria-label=Group`), name in `code` → `/group?name=`; `@administrators` adds pill `protected` |
| Owner | record `owner` in `code` |
| Maintainers | comma list in `code`, or muted None |
| Members | caller is Administrator or record visible: comma list, or muted “No members”; else muted “Not visible to you” |

Intended: Owner and Maintainers read muted “Not visible to you” when the record
is not in `/ls`. **Built:** that branch never fires (Go `with` over a zero
struct is true), so a hidden record shows an empty owner and None.

Empty table row: “No groups registered.” (unreachable: `@administrators`
always exists).

## Register group `/groups/new`

| Route | `GET /groups/new` |
|---|---|
| Access | any signed-in principal; the daemon decides at submit (a User, or an Agent acting for one, becomes Owner) |
| Daemon calls | `/status` |

Title `Register group`; **Back to Groups**; h1 👥 Register group; error
summary; form `id=form-save`, `POST /groups`, hidden `action=save`, `new=1`,
[group fields](#group-fields) in their registration state. Button **Register
group**. Success → **303** `/group?name=<typed name>`.

## Group `/group?name=`

| Route | `name`, matched exactly against `/groups` keys (lower case) |
|---|---|
| Access | any signed-in principal for any live group name; absent → 404 *No such name* |
| Daemon calls | as `/groups` |

| Part | Content |
|---|---|
| Top | Title `Group <name>`; **Back to Groups**; h1 👥 + name in `code` |
| `@administrators` only | pill `protected` + ⓘ (direct users only; never empty; Owner always a member; adding one creates a profile; nesting it elsewhere grants no authority); muted intro; section **What membership grants** (Ordinary users · Ordinary groups · Membership lists) and **What it does not grant** (The Owner, or each other · Services, Queues and PubSub · This group) — static text |
| Meta | record visible: “Owner: `<owner>`”, “👮 Maintainers: a, b” or “none” (`@administrators`: “none — this group has none, and its Owner follows daemon ownership”), “Personal” when set. Hidden: muted “Owner and Maintainers are not visible to you.” |
| Description | `p.group-description` when visible and set |
| Members | may edit: `code` lines or muted “No members”, link **Edit group** `id=members-edit` → `/group/edit?name=`. Else lines, or muted “Membership is not visible to you.” (not Administrator, record hidden), then muted “Only the daemon owner changes this protected group.” or “Its Owner, its Maintainers and the daemon Administrators change this group's membership.” |
| Used by visible records | table Record (→ its page by kind) · Kind (entity label) · Uses this group (`ACL`, `Maintainers`, `ACL via @outer`, joined ` · `). Face-computed from `/ls` `allow` and `maintainers`, following nested groups through `/groups` memberships; group records count their members as `ACL`. None: muted “No caller-visible record refers to this group.” |
| Danger Zone | link → `/service-danger?name=` when the record is visible and `can_manage` |

**May edit** (face rule, the daemon decides again): `@administrators` → daemon
Owner only; any other group → Administrator or record `can_manage`.

| Viewer of `@ops` (Owner bob, Maintainer carol) | Sees |
|---|---|
| bob (Owner) | Owner, Maintainers, members, Edit group, Danger Zone (with Transfer) |
| carol (Maintainer) | same; Danger Zone without Transfer |
| Administrator, not member | “not visible” meta, members, Edit group, no Danger Zone |
| other user | “not visible” meta and membership, no controls |

## Edit group `/group/edit?name=`

| Route | `name` exact |
|---|---|
| Access | the may-edit rule above; else **403** “that group's membership cannot be changed by you”; absent → 404 |
| Daemon calls | as `/groups` |

Title `Edit <name>`; **Back to `<name>`** → `/group?name=`; h1 👥 Edit
`<name>`; error summary; form `id=form-save`, `POST /groups`, hidden
`action=save`, [group fields](#group-fields). Button **Save group**.

### Group fields

*Assign* = registering, or editing a group other than `@administrators` whose
record is visible with `can_transfer` (its Owner or the daemon Owner).

| `name` | Input | Prefill | Placeholder, help | Enabled for |
|---|---|---|---|---|
| `name` | text, required (register); hidden (edit) | typed value on refusal | `@operators`; “One name for the set. It cannot be changed afterwards. You become its Owner.” Extra note when the typed value is `@owner` | register |
| `descr` | text | stored `descr`, typed on refusal | “What this group is for”; “Shown beside the group's name.” | register; edit when the record is visible. Otherwise **disabled** with “This group's record is not visible to you, so its description is left as it is.” |
| `members` | textarea, 8 rows | members one per line; typed text on refusal | `user@realm` / `#agent@realm` / `@nested-group`; “One user, `#agent` or nested group per line; `@owner` is reserved for ACLs.” | every editor |
| `edit_personal` | hidden `1` | — | says the form carried Personal | rendered only with *assign* |
| `personal` | checkbox “Personal” | stored flag; typed on refusal | “Puts this group in its Owner's Personal view…”; with assign: “A Personal group's members and Maintainers may name only its Owner and the Owner's own agents.” | *assign*; else **disabled** (“The protected group is never Personal.” or “Shown for reference: only this group's Owner or the daemon Owner may change it.”) |
| `edit_sharing` | hidden `1` | — | — | edit with *assign* |
| `maintainers` | textarea, 5 rows | stored, one per line; typed on refusal | “One user, group or agent per line. They change this group's members as its Owner does.” | edit only; *assign*, else **disabled** (protected: “has no Maintainers”) |
| `secret` | textarea, 4 rows, `autocomplete=off`, `spellcheck=false` | never | `TOKEN=...`; register: “Optional. Every member reads it back with `agent-bus secret`.” Edit: “Leave empty to keep the stored secret…” | register; edit when visible and `can_manage`; else **disabled**, “Only this group's Owner and Maintainers write its secret.” (an Administrator is not a manager by rank) |

Error paragraph: `p.warn id=group-error`.

## POST /groups

| Step | Behaviour |
|---|---|
| 1 | `action` ≠ `save` (including the retired `delete`) → local refusal **400** “That group action is not available.” |
| 2 | `members` split on whitespace, sorted; empty is `[]`, never absent |
| 3 | Change set: `descr` if the field was submitted; `maintainers` (split) only with `edit_sharing`; `personal` (`on` → true, else false) only with `edit_personal` |
| 4 | *Create path* when `new=1`, or the name is `@administrators`, or (`GET /groups`) the lower-cased name does not exist yet — so a settings save of an unknown name registers it |
| 5 | Existing group: one `POST /manage` `{"name","descr"?,"allow": members,"maintainers"?,"personal"?}` |
| 6 | New group with `personal` true and a name not starting `@<you>/` (case-insensitive) → local refusal **400** “A Personal group is named for its owner: call it @<you>/<name>.” before any call |
| 7 | Create path: `POST /group` `{"Name","Members"}` (capitalised keys). Then `POST /manage` `{"name","descr"?,"personal"?}` only if there is something: a new group's non-empty description and a true Personal; `@administrators` its description. Maintainers are never sent on this path. A new group whose second call fails → local refusal **502** “The group was registered and its description or classification was not stored: … Change them on its settings page.” |
| 8 | No error and a non-empty secret → `POST /secret` `{"name","secret"}`; failure → local refusal **502** “The group was saved and its secret was not stored: <reason> Set it with: agent-bus secret <name> '...'” |
| 9 | Success → **303** `/group?name=<typed name>` |

**Refusal.** The daemon message is matched against the typed `members` (and
`maintainers` with `edit_sharing`) lines; a hit prefixes `Line N: ` and marks
that field whatever the code. Preserved code or a line hit → form again with
the refusal status; retained `name`, `members`, `new`, `descr`, `maintainers`,
`personal`, never `secret`. `new=1` → register page, field defaults to `name`.
Otherwise field defaults to `members`, groups are re-read, and the edit page is
shown only if the group still exists and the caller may edit it (else problem
page). Any other refusal → problem page (e.g. 403 “that record belongs to
someone else: owner must remain an administrator” when the Owner is removed
from `@administrators`).

<details><summary>Daemon rules and refusals seen</summary>

| Case | Answer |
|---|---|
| name `@owner` | 400 `bad name: @owner is a runtime ACL term and cannot be created`, `name` marked |
| unknown member | 404 `Line 2: no such name: group member nosuchuser`, `members` marked |
| unknown maintainer | 404 `Line 2: no such name: maintainer ghost`, `maintainers` marked |
| Personal on an unprefixed existing group | 400 `unknown record kind: a personal group is named @<owner>/<name>, and @ops is not`, `members` marked |
| Personal group naming another user | 400 `Line 2: a personal record's allow and maintainers name only its owner and the owner's agents: …` |
| saving a group you do not manage | 403 problem page |
| secret not `KEY=value` | 502 local refusal after the group was created |
| `@administrators` | daemon Owner only; direct Users only; must keep the Owner; no Maintainers; never Personal |
| a `@<user>/…` name | reserved for that user as Owner |
| register with an existing name you may edit | accepted: SetGroup **replaces** its membership, 303 to the group |

</details>

## Group Danger Zone `/service-danger?name=<group>`

The record Danger Zone (`records.md`) as a group reaches it.

| Route | `name` |
|---|---|
| Access | `GET /lookup?name=`; record `can_manage` (Owner, Maintainer, daemon Owner), else **403** “only the owner or an assigned Maintainer can manage this record”. An Administrator who does not manage the group is refused |
| Daemon calls | `/status`, `/lookup` |

| Section | Shown when | Form |
|---|---|---|
| **Replace configuration** | always (a group holds a configuration) | `id=form-configure`, `POST /service`, hidden `name`, `action=configure`; `textarea name=config` 6×60, required, `autocomplete=off`, never prefilled or retained. Face checks JSON: invalid → 400 “Configuration must be valid JSON. The submitted configuration is not shown again.”, `config` marked. Daemon: `POST /configure` `{"Name","Config"}`. Success → **303** `/group?name=` |
| **Transfer ownership** | `can_transfer` (Owner, daemon Owner), name ≠ `@administrators`, name ≠ owner. **Still shown for a prefixed `@<user>/…` group** | `id=form-transfer`, `POST /service-confirm`, hidden `name`, `action=transfer`; `input name=owner` required, retained on refusal. Button **Continue to confirmation** |
| Removal | never | muted “A group is retired by emptying its members, never removed…” |

**Transfer flow.**

| Step | Behaviour |
|---|---|
| `POST /service-confirm` | `GET /lookup`; not `can_transfer` → 403 “only this record's owner or the daemon owner can transfer it”; empty owner → Danger Zone, 400 “New owner is required.”, `owner` marked |
| Confirm page | title `Confirm ownership transfer · <name>`; **Back to the Danger Zone**; “Transfer `<name>` from `<owner>` to `<new>`?”; form `POST /service` hidden `action=transfer`, `name`, `owner`, `expected_owner`, `confirmed=1`; button **Transfer ownership** |
| `POST /service` | re-`GET /lookup`; owner ≠ `expected_owner`, lost `can_transfer`, or self-owned → **409** *The conditions changed* (back link: Review the current Danger Zone). Then `POST /manage` `{"name","owner"}` |
| Refused | preserved codes → Danger Zone with the message under Transfer, `owner` retained, no field marked. Prefixed group: 400 `unknown record kind: @bob/team is named for bob and cannot be transferred; create @carol/… instead` |
| Done | `GET /lookup`: still visible → `/group?name=`, else `/groups` |

## Account `/account`

| Route | `GET /account`, no parameters; reached from the header name |
|---|---|
| Access | any signed-in principal (User, Agent, service); stranger 401 sign-in |
| Daemon calls | `/status`; `/users` (own row, if any); `/ls` (own record, owned records, kinds); `/names` (held credentials). `/users` or `/ls` failure → problem page; `/names` failure → message in the Credentials section only |

| Section | Content |
|---|---|
| Title | `Account · agent-bus`; h1 🪪 Account + ⓘ (facts are the daemon's; a fingerprint does not reveal a credential; rotation keeps the replaced one until the next) |
| Identity | Own `/users` row: large photo/initial, bold person name (else name), name in `code`; pills “👤 User” (fixed text), authority, raw `status` (`active`); **Groups** chips → `/group?name=` if any. Else own record in `/ls`: “<kind label> `<name>`” + “This identity has a registered record and no person profile visible here.” Else `<you>` + “No person profile or caller-visible identity record was returned…” |
| Owned records | `/ls` records with owner = you, excluding your own record (active only): link by kind + kind pill. None: muted “No records owned by this identity are visible.” |
| Credentials | `h2 id=credentials`; table Name · Kind (`person` → “👤 User”, `unregistered` → “Unregistered”, else kind label) · Owner (column only when some row's owner ≠ you; that cell `code`, else —) · Fingerprint · Issued (`YYYY-MM-DD HH:MM` or muted `unavailable`) · Last used (or muted `not this run`). Empty: “You hold no credential.” |
| Rotation | note **Rotate your identity credential**: `agent-bus-token <you> --rotate`; “The replaced credential remains valid until the next rotation.” No button |

No forms. Links: group chips, owned-record links.

## Older docs that disagree

| Doc | Says | Code |
|---|---|---|
| [pages](../../Plans/MVP/web/pages.md#user-username), [forms](../../Plans/MVP/web/forms.md#the-set) | Lifecycle Pause/Ban/Activate; confirm for ban | Active/Inactive only: Deactivate… (confirmed), Reactivate (direct) |
| pages § Groups, forms | group delete: “the handler accepts it” | `action=delete` → 400 local refusal |
| [discovery](../05-discovery.md#dashboard), pages § Groups | Group registration entry and route only for Administrators / conditional | offered to every signed-in user; the daemon decides |
| forms § remaining work | Activate offered to an already-active user | only the applicable transition is offered |
| pages § User | state in the identity section | separate Access section |

## Worth fixing in the rewrite

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
