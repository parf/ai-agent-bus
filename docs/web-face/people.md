# People pages: users, groups, account

📌 **TL;DR:** Every user, group and account page of the web face
(`agent-bus-web`, `src/web`): the Users directory, one user, registering and
editing users, deactivation and credential removal, Groups with their Personal
filter, one group and its editor, the group Danger Zone, and Account. The frame,
problem pages and form recovery are in [shell](shell.md#problem-page).

## Shared rules

Every page here starts the same way; the per-page sections state only what
differs.

| Rule | Behaviour |
|---|---|
| Signed out | the sign-in page at the requested URL, **401**, with `return` = that URL ([shell § signed-out requests](shell.md#signed-out-requests)) |
| Caller facts | `GET /status` on every page: `you`, `administrator`, `daemon_owner` |
| Bus refusal → page | the [problem page](shell.md#problem-page) table |
| POST guard | the [origin check](shell.md#origin-checks) and the 1 MiB body cap |
| Preserved refusal | a daemon refusal with code `400, 404, 409, 412, 429` re-renders the form **with that status**: alert summary *Check this form*, the daemon message, *Review the submitted fields* → `#form-<action>`, and the same message under the fields. Other codes → problem page |
| Field error | only the attributed field gets `aria-invalid="true"` and `aria-describedby` → the form's error id; every other field carries `aria-invalid="false"` |
| Secrets | a secret or configuration is never prefilled and never returned after a refusal. CRLF in a secret is stored as LF |
| `return` (user pages) | kept only when it is a local URL whose path is `/users` or `/diagnostics` (query kept, fragment dropped); anything else becomes `/users` |
| Directory load (`load`) | user pages (not `/users/new`) call `GET /users`, `GET /ls` and `GET /inactive` (every caller-visible record, inactive ones marked). Any failure → problem page |
| Names | a user or group name is matched exactly, then case-insensitively |

### Roles

| Role | Where the face learns it |
|---|---|
| daemon Owner | `/status.daemon_owner`; per directory row `daemon_owner` |
| Administrator | `/status.administrator` (member of `@administrators`; the Owner always is); per row `administrator` |
| group Owner / Maintainer | the group record from `/ls`: `owner`, `maintainers`, `can_manage` (Owner, Maintainer, daemon Owner), `can_transfer` (record Owner or daemon Owner) |
| the user themself | row `name` = `you` |
| per-row authority | daemon-derived on each `/users` row: `can_edit` (daemon Owner edits anyone; an Administrator edits non-Administrators other than self), `can_activate` (= `can_edit`), `can_remove` (Administrator, and the name has neither profile nor other-owned record), `can_set_email` (self) |

`GET /users` answers **only the caller's own row** unless the caller is an
Administrator, who gets every name: Users (`kind: user`), self-owned records
without a profile (`kind: record`) and leftover credentials (`kind: credential`).

## Users `/users`

| Route | |
|---|---|
| Method, path | `GET /users` |
| `q` | search text, trimmed; case-insensitive substring of name, person name, email, GitHub login, company, location, Twitter/X joined by spaces. Default empty |
| `state` | `active` (default; any other value), `inactive`, `all` |
| `page` | 1-based integer, 25 rows per page, clamped to `1…last`; junk reads as 1 |
| `kind` | `kind=other` → **303** `/diagnostics#leftovers` once signed in. Anything else ignored |

| Access | |
|---|---|
| stranger | sign-in page, 401 |
| Administrator, daemon Owner | every User row; **Register user** |
| any other signed-in user | only their own row (daemon rule); no Register |

**Daemon calls:** `load`. The face filters to `kind: user` rows and computes
every count.

**Content, in order**

| Part | Content and source |
|---|---|
| Title | `Users · agent-bus`; h1 👤 Users + ⓘ popover (four bullets: what a User is, Active/Inactive and struck names, Last used, face-computed counts); **Register user** → `/users/new` (Administrators) |
| Tabs | **All (N)** → `/users`, always current; N = Users in the current state. **Register user** (Administrators) |
| Search form | `GET /users`; `input type=search name=q id=record-query`, placeholder “Search by name, identity, email or GitHub login”, visually hidden label “Search users”; hidden `state` when not active |
| Status filter | **Active (a)** · **Inactive (i)** · **All states (n)**, counts over the search; links keep `q`, drop `page` |
| Table | “Showing s–e of m matching users.”; columns below |
| Pager | `nav aria-label="Directory pages"`: **Previous page**, `Page N of M`, **Next page** (links keep `q`, `state`) |

| Column | Cell |
|---|---|
| User | photo (`data:image/png;base64` of `photo_png`, `alt=""`) or initial; person name in bold if set; 🔱 mark for the daemon Owner; name in `code` linked `/user?name=<name>&return=<this page URL>`; inactive: person name and name struck (`<s>`) + badge `INACTIVE`; company as a muted line |
| Authority | “🔱 Daemon owner” · “Daemon administrator” · “User” |
| Contact | email (`mailto:`); “GitHub `<login>`”; neither: muted — |
| Agents (numeric) | agents among the visible records whose owner is this user; 0 muted |
| Last used | `last_used` of the record named like the user: relative, with `datetime` and a `YYYY-MM-DD HH:MM` title. None: inactive → muted — (`title` “Not reported while inactive”); active → muted `never` |

| State | Shows |
|---|---|
| no match with `q` | “No user matches this search”, “A User is a person on this node; every record belongs to one.”, **Clear the search** → `/users` |
| `state=inactive`, none | “No inactive users” + the sentence |
| otherwise empty | “No users yet” + the sentence + **Register a user** (Administrators) |
| bus down | 502 problem page |

Controls are omitted, never disabled.

## Register user `/users/new`

| Route | |
|---|---|
| `GET /users/new` | no parameters |
| `GET /user` with empty or no `name` | the same page for an Administrator; anybody else gets 404 *No such name* |

| Access | |
|---|---|
| Administrator, daemon Owner | the form |
| other signed-in user | **403** *Not yours to see*, “only a daemon Administrator can register a user” |

**Daemon calls:** `/status` only (`/user` also loads the directory first).

**Content:** title `Add user`; **Back to directory** → `/users`; h1 Add user;
error summary; the **Profile** form; card **SSH access**: “Public keys are added
on the host after the profile is saved.” + ⓘ popover naming
`agent-bus-admin user add <user@realm> <key.pub>`.

**Form** `id=form-create`, `POST /user`. Hidden: `action=create`,
`return=/users`. Fields: [profile fields](#profile-fields). Button **Save
profile**, **Cancel**. Submit: [POST /user](#post-user).

## User `/user?name=`

| Route | |
|---|---|
| `name` | directory name |
| `return` | Back target, see [shared rules](#shared-rules); default `/users`. Diagnostics links here with `return=/diagnostics` |

| Access | |
|---|---|
| Administrator | any row |
| other user | own row only; any other name 404 |
| Profile editable | `can_edit` → **Edit profile** and **Refresh from GitHub** |
| Own email | `can_set_email`, not `can_edit`, and the row is yours → the email form |
| Access change | `can_activate` and target not daemon Owner |

**Daemon calls:** `load`. Shown facts come from the `/users` row, except
owned records: row `services` filtered to caller-visible records, plus visible
non-group records whose owner is this user, inactive ones included, sorted.

### A User (`kind: user`)

Title `<name>`; **Back to directory** → `return`; large photo or initial, h1
person name (else name), then the name with a copy button, **one** identity
pill (“🔱 Daemon owner”, “Daemon administrator” or “User”) and the state pill;
**Edit profile** → `/user/edit?name=&return=` when `can_edit`. Then:

| Section | Content |
|---|---|
| Profile `id=profile-edit` | `dl`, each pair only when set: Person name · Email · GitHub login (`@login` → `https://github.com/<login>`) · Company · Location · Twitter/X (`@handle` → `https://x.com/<handle>`); “No profile details yet.” when none. `can_edit` with a GitHub login: form `id=form-refresh-github` → `action=refresh-github`, button **Refresh from GitHub**. Own email: form `id=form-email`, field `email` “Your email”, button **Save email** (`action=email`). Neither: muted “Trusted profile fields are edited by a daemon administrator.” |
| Owned records | links by kind, each with its kind pill and `INACTIVE` when inactive; none: muted “None” |
| Groups | chips → `/group?name=`, from row `groups` (direct and nested membership, daemon-computed); none: muted “No memberships” |
| Access | ⓘ help; “Current: Active” or “Inactive”; inactive → POST form **Reactivate** (`action=active`); active → GET form **Deactivate…** → `/user-deactivate` |

### A non-user identity (`kind: record` or `credential`, Administrators only)

h1 🪪 mark + name; pill “Self-owned record” or “Credential only” and muted “No
user lifecycle state”; ⓘ popover on credential removal. Card **Registered
name** (“A self-owned record exists. *Inspect its registration and queues*” →
record page, then each owned record) or **Credential only** (“No registered
record remains for this credential.”). With `can_remove`: GET form →
`/credential-remove`, hidden `name`, `return`, button **Review credential removal…**.

| State | Shows |
|---|---|
| inactive user | Current Inactive; Reactivate; owned records still listed |
| unknown or hidden name | 404 *No such name* |
| refused GitHub refresh or email | this page with the summary, status of the refusal ([POST /user](#post-user)) |

## Edit user `/user/edit?name=`

| Route | `name`, `return` as above |
|---|---|
| Access | row `kind: user` and `can_edit`: daemon Owner for anyone; Administrator for non-Administrators other than self. Else **403** “that profile cannot be edited by you”; name absent from `/users` → 404 |
| Daemon calls | `load` |

Title `Edit <name>`; **Back to `<name>`** → `/user?name=&return=`; photo or
initial and h1 “Edit `<name>`”; error summary. **Form** `id=form-save`,
`POST /user`, hidden `action=save`, `return`, `name`. Fields:
[profile fields](#profile-fields), prefilled from the row. Button **Save
profile**, **Cancel**.

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

The error paragraph is `id=profile-error`. A refusal marks `name` on a create
`412`, `email` when the message names email, and `name` on a create whose
message names the name.

## POST /user

| `action` | Daemon call (JSON body) | Success | Refusal |
|---|---|---|---|
| `create` | `POST /user` with the [profile body](#profile-body), `create: true` | **303** `/user?name=<stored name>` | preserved codes: Add user page again (Administrators; else problem page), status = code |
| `save` | `POST /user`, profile body without `create` | **303** `return` | preserved codes: edit page with the typed values |
| `email` | `POST /profile` `{"email"}` | **303** `/user?name=` | preserved codes: the user page, `email` marked |
| `inactive`, `active` | `POST /user/state` `{"name","status"}` | **303** `return` (the user's page) | always a problem page |
| `remove-credential` | `POST /identity/remove` `{"name"}` | **303** `return` | always a problem page; e.g. **409** “…credential is now backed by a user or a record; refresh the directory” |
| `refresh-github` | `POST /user/github-refresh` `{"name"}` | **303** `/user?name=&return=` | preserved codes: the user page with the summary, anchored `#form-refresh-github` |
| anything else | none | — | local refusal **400** “That user action is not available.” |

Each success carries a flash message.

### Profile body

`name`, `person_name`, `email`, `github_user`, `github_company`,
`github_location`, `github_twitter_username` from the form, plus
`profile_details_set: true` so empty company, location and Twitter/X clear
them. The daemon keeps the stored status.

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

Title `Confirm deactivation · <name>`; **Back to user** →
`/user?name=&return=`; h1 Confirm deactivation; “Deactivate `<name>`?”;
bullets: bus access stops and owned records become inactive; queued work and
tokens kept, processes not stopped; then “Only the daemon Owner can reactivate
this Administrator later.” (target is an Administrator) or “An authorized
Administrator or the daemon Owner can reactivate this user later.”

Form `POST /user`: hidden `name`, `return` (the user's page); button
`name=action value=inactive` **Deactivate user**; **Cancel**. Reactivation has
no confirmation: the Access form posts `action=active` directly.

## Confirm credential removal `/credential-remove?name=`

| Route | `name`, `return` (Diagnostics passes `/diagnostics`) |
|---|---|
| Access | row not `kind: user` and `can_remove` (Administrators). Else **403** “that credential cannot be removed by you”; absent → 404 |
| Daemon calls | `load` |

Title `Confirm credential removal · <name>`; **Back to identity** → user page;
h1 Confirm credential removal; “Remove the credential for `<name>`?”; bullets:
current and previous credentials stop authenticating; every browser session
made with it ends; no user or record is removed. Form `POST /user`: hidden
`name`, `return`; button `name=action value=remove-credential` **Remove
credential**; **Cancel**. Success → `return`.

## Groups `/groups`

| Route | `GET /groups`; `personal=1` shows the Personal groups |
|---|---|
| Access | any signed-in principal; stranger 401 sign-in |
| Daemon calls | `/status`; `GET /groups` (every live group name, members only where readable); `GET /ls` (group records the caller may see: member or manager) |

| Part | Content |
|---|---|
| Title | `Groups` (`Groups · Personal`); h1 👥 Groups + ⓘ (tooltip and five bullets: what a group is; `@owner` is ACL syntax; anyone may register; who edits; who sees what); **Register group** → `/groups/new` (**Register Personal group** → `/groups/new?personal=1`), **for everybody** |
| Tabs | **All (N)** (shared groups) · **Personal (N)** → `/groups?personal=1` |
| Table | caption “N groups”; rows by name |

| Column | Cell |
|---|---|
| Group | 👥 mark, name in `code` → `/group?name=`; `@administrators` adds pill `protected`; the description as a muted line |
| Owner (not on Personal) | record `owner` in `code`; record not visible: muted “Not visible to you” |
| Maintainers | comma list in `code`, or muted None; record not visible: muted “Not visible to you” |
| Members | caller is Administrator or record visible, and members readable: one per line, or muted “No members”; else muted “Not visible to you” |

Empty: “No groups registered”, or “No Personal groups yet” with the naming rule
`@<you>/…`.

## Register group `/groups/new`

| Route | `GET /groups/new`; `personal=1` starts the form Personal |
|---|---|
| Access | any signed-in principal; the daemon decides at submit (a User, or an Agent acting for one, becomes Owner) |
| Daemon calls | `/status`, `/groups`, `/ls` |

Title `Register group`; **Back to Groups**; h1 👥 Register group; error
summary; form `id=form-save`, `POST /groups`, hidden `action=save`, `new=1`,
[group fields](#group-fields) in their registration state. Button **Register
group**, **Cancel**. Success → **303** `/group?name=<stored name>`.

## Group `/group?name=`

| Route | `name` |
|---|---|
| Access | any signed-in principal for any live group name; absent → 404 *No such name* |
| Daemon calls | as `/groups` |

| Part | Content |
|---|---|
| Top | title `Group <name>`; **Back to Groups** (the Personal list for a Personal group); h1 👥 + name with a copy button; **Edit group** → `/group/edit?name=` when the caller may edit |
| Meta | record visible: `Owner` chip, “👮 Maintainers: a, b” or “none” (`@administrators`: “none — its Owner follows daemon ownership”), a `Personal` pill when set. Hidden: muted “Owner and Maintainers are not visible to you.” |
| `@administrators` only | pill `protected` + ⓘ (direct users only, never empty, the Owner always a member; adding a name creates a profile; nesting it elsewhere grants no authority); cards **What membership grants** (Ordinary users · Ordinary groups · Membership lists) and **What it does not grant** (The Owner, or each other · Services, Queues and PubSub · This group) — static text |
| Description | when visible and set |
| Members `id=members-edit` | one per line with the count, or muted “No members”; unreadable: muted “Membership is not visible to you.” Then muted “Only the daemon owner changes this protected group.” or “Its Owner, its Maintainers and the daemon Administrators change this group's membership.” |
| Used by visible records | table Record (→ its page by kind) · Kind (pill) · Uses this group (`ACL`, `Maintainers`, `ACL via @outer`, `Maintainers via @outer`, joined ` · `; on a group record `member` and `member via @outer`). Face-computed from `/ls` `allow` and `maintainers`, following nested groups through `/groups` memberships. None: muted “No caller-visible record refers to this group.” |
| Danger Zone | link → `/service-danger?name=` when the record is visible and `can_manage`, naming configuration and, where it applies, transfer |

**May edit** (face rule, the daemon decides again): `@administrators` → daemon
Owner only; any other group → Administrator or record `can_manage`.

| Viewer of `@ops` (Owner bob, Maintainer carol) | Sees |
|---|---|
| bob (Owner) | Owner, Maintainers, members, Edit group, Danger Zone (with Transfer) |
| carol (Maintainer) | same; Danger Zone without Transfer |
| Administrator, not member | “not visible” meta, members, Edit group, no Danger Zone |
| other user | “not visible” meta and membership, no controls |

## Edit group `/group/edit?name=`

| Route | `name` |
|---|---|
| Access | the may-edit rule above; else **403** “that group's membership cannot be changed by you”; absent → 404 |
| Daemon calls | as `/groups` |

Title `Edit <name>`; **Back to `<name>`** → `/group?name=`; h1 👥 Edit
`<name>`; error summary; form `id=form-save`, `POST /groups`, hidden
`action=save`, `name`, [group fields](#group-fields). Button **Save group**,
**Cancel**.

### Group fields

*Assign* = registering, or editing a group other than `@administrators` whose
record is visible with `can_transfer` (its Owner or the daemon Owner).

| `name` | Input | Prefill | Placeholder, help | Enabled for |
|---|---|---|---|---|
| `name` | text, required (register) | typed value on refusal | `@<you>/team or @operators`; “One name for the set. It cannot be changed afterwards. You become its Owner. A Personal group is named `@<you>/…`.” Extra note when the typed value is `@owner` | register |
| `descr` | text | stored `descr`, typed on refusal | “What this group is for”; “Shown beside the group's name.” | register; edit when the record is visible. Otherwise **disabled** with “This group's record is not visible to you, so its description is left as it is.” |
| `members` | line-list textarea, 8 rows | members one per line; typed text on refusal | `user@realm` / `#agent@realm` / `@nested-group`; “One user, #agent or nested group per line; @owner is reserved for ACLs.” | every editor |
| `edit_personal` | hidden `1` | — | says the form carried Personal | rendered only with *assign* |
| `personal` | checkbox “Personal” | stored flag; typed on refusal; on from `?personal=1` | with assign: “Puts this group in its Owner's Personal view. A Personal group's members and Maintainers may name only its Owner and the Owner's own agents.” | *assign*; else **disabled** (“The protected group is never Personal.” or “Shown for reference: only this group's Owner or the daemon Owner may change it.”) |
| `edit_sharing` | hidden `1` | — | — | edit with *assign* |
| `maintainers` | line-list textarea | stored, one per line; typed on refusal | “One user, group or agent per line. They change this group's members as its Owner does.” | edit only; *assign*, else **disabled** (protected: “The protected group has no Maintainers.”) |
| `secret` | textarea, 4 rows, `autocomplete=off`, `spellcheck=false` | never | `TOKEN=...`; register: “Optional. Every member reads it back with agent-bus secret.” Edit: “Leave empty to keep the stored secret. Anything here replaces it.” | register; edit when visible and `can_manage`; else **disabled**, “Only this group's Owner and Maintainers write its secret.” (an Administrator is not a manager by rank) |

Error paragraph: `id=group-error`.

## POST /groups

| Step | Behaviour |
|---|---|
| 1 | `action` ≠ `save` → local refusal **400** “That group action is not available.” |
| 2 | `members` split on whitespace, sorted; empty is `[]`, never absent |
| 3 | Register (`new=1`) of a name that exists (case-insensitive) → the form again, **409** “that name is already registered”, `name` marked, before any call |
| 4 | Register with `personal` and a name not starting `@<you>/` (case-insensitive) → the form again, **400** “A Personal group is named for its owner: call it @<you>/<name>.”, `name` marked, before any call |
| 5 | Existing group other than `@administrators`: one `POST /manage` `{"name","allow": members,"descr"?,"maintainers"?,"personal"?}`: `descr` if the field was submitted, `maintainers` only with `edit_sharing`, `personal` only with `edit_personal` |
| 6 | Otherwise (a new group, or `@administrators`): `POST /group` `{"Name","Members"}`, then `POST /manage` `{"name","descr"?,"personal"?}` only if there is something: a non-empty description, and a true Personal on a new group. A failed second call → **502** saved-in-part page “The group <name> was registered, and its description or classification was not stored: … Change them on its settings page.” |
| 7 | A non-empty secret → `POST /secret` `{"name","secret"}`; failure → **502** saved-in-part page “The group <name> was saved, and its secret was not stored: <reason> Set it with: agent-bus secret <name> '...'” |
| 8 | Success → **303** `/group?name=<stored name>` |

**Refusal.** The daemon message is matched against the typed `members` (and
`maintainers` with `edit_sharing`) lines; a hit prefixes `Line N: ` and marks
that field whatever the code. Preserved code or a line hit → the form again with
the refusal status; retained `name`, `members`, `new`, `descr`, `maintainers`,
`personal`, never `secret`. With no line hit the field is `name` on a register
or on the Personal-naming refusal, else `members`. Any other refusal → problem
page (e.g. 403 “that record belongs to someone else: owner must remain an
administrator” when the Owner is removed from `@administrators`).

<details><summary>Daemon rules and refusals seen</summary>

| Case | Answer |
|---|---|
| name `@owner` | 400 `bad name: @owner is a runtime ACL term and cannot be created`, `name` marked |
| unknown member | 404 `Line 2: no such name: group member nosuchuser`, `members` marked |
| unknown maintainer | 404 `Line 2: no such name: maintainer ghost`, `maintainers` marked |
| Personal on an unprefixed existing group | 400 `unknown record kind: a personal group is named @<owner>/<name>, and @ops is not`, `name` marked |
| Personal group naming another user | 400 `Line 2: a personal record's allow and maintainers name only its owner and the owner's agents: …` |
| saving a group you do not manage | 403 problem page |
| secret not `KEY=value` | 502 saved-in-part page after the group was created |
| `@administrators` | daemon Owner only; direct Users only; must keep the Owner; no Maintainers; never Personal |
| a `@<user>/…` name | reserved for that user as Owner |

</details>

## Group Danger Zone

`/service-danger?name=<group>`: the [record Danger Zone](records.md#danger-zone)
as a group reaches it.

| Route | `name` |
|---|---|
| Access | record `can_manage` (Owner, Maintainer, daemon Owner), else **403** “only the owner or an assigned Maintainer can manage this record”. An Administrator who does not manage the group is refused |
| Daemon calls | `/status`, `/inactive`, `/lookup` |

| Section | Shown when | Form |
|---|---|---|
| **Replace configuration** | always (a group holds a configuration) | `id=form-configure`, `POST /service`, hidden `name`, `action=configure`; `textarea name=config`, 6 rows, required, never prefilled or retained. Face checks JSON: invalid → 400 “Configuration must be valid JSON. The submitted configuration is not shown again.”, `config` marked. Daemon: `POST /configure` `{"Name","Config"}`. Success → **303** `/group?name=` |
| **Transfer ownership** | `can_transfer` (Owner, daemon Owner), name ≠ `@administrators`, name ≠ owner, not a `@<user>/…` group | `id=form-transfer`, `POST /service-confirm`, hidden `name`, `action=transfer`; `input name=owner` required, retained on refusal. Button **Continue to confirmation** |
| Transfer note | a `@<user>/…` group | muted: never transferred; its new owner creates their own |
| Removal | never | muted “A group is retired by emptying its members, never removed…” |

**Transfer flow** is the record one ([POST /service-confirm](records.md#post-service-confirm),
[POST /service](records.md#post-service)); a done transfer lands on
`/group?name=` while the group is still visible, else on `/groups`.

## Account `/account`

| Route | `GET /account`, no parameters; reached from the top bar's account link |
|---|---|
| Access | any signed-in principal (User, Agent, service); stranger 401 sign-in |
| Daemon calls | `/status`; `/users` (own row, if any); `/ls` and `/inactive` (own record, owned records); `/names` (held credentials). `/users` or `/ls` failure → problem page; `/names` failure → `Credentials unavailable: …` in that card only |

| Section | Content |
|---|---|
| Title | `Account · agent-bus`; h1 🪪 Account + ⓘ (facts are the daemon's; a fingerprint does not reveal a credential; rotation keeps the replaced one until the next) |
| Identity | own `/users` row: large photo or initial, person name (else name), the name with a copy button, the one identity pill and the state pill; **Groups** chips → `/group?name=` if any; **Open your profile** → `/user?name=`. Else own record: kind pill + name + “This identity has a registered record and no person profile visible here.” Else `<you>` + “No person profile or caller-visible identity record was returned for this identity.” |
| Credentials `id=credentials` | table Name · Kind (`person` → “👤 User”, `unregistered` → “Unregistered”, else kind label) · Owner (column only when some row's owner ≠ you; that cell `code`, else —) · Fingerprint · Issued (`YYYY-MM-DD HH:MM` or muted `unavailable`) · Last used (relative, or muted `not this run`). Empty: “You hold no credential.” |
| Rotation | note **Rotate your identity credential**: `agent-bus-token <you> --rotate`; “The replaced credential remains valid until the next rotation.” No button |
| Owned records | visible records with owner = you, active and inactive, excluding your own record: link by kind, kind pill, `INACTIVE` when inactive. None: muted “None” |

No forms. Links: group chips, profile link, owned-record links.

## Worth fixing in the rewrite

History: the Go face's defects and its differences from older docs are in
[web-go-face-differences § people](../../Plans/R0.8-MVP/done/web-go-face-differences.md#peoplemd).
