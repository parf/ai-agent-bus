# Project Constitution

📌 **TL;DR:** The model every face and the daemon implement: Users own
everything, names say their kind, one write-through SQLite store, and an
inactive or incorrect entity is no such entity. **MUST**, **MUST NOT**,
**SHOULD** and **MAY** are normative. Built in 0.7; what is still pending is
the [0.7 plan](../Plans/R0.8-MVP/0.7.0-TODO.md#storage-and-identity)'s, never this page's.

Each section states its essentials first; the full rules are in the closed
blocks below them, and are just as normative.

## Persistence and loading

One write-through SQLite store: a write validates, commits as one transaction,
then publishes, so no reader sees an uncommitted or partial change. Startup
loads every durable entity and ignores — never repairs — an incorrect record,
reporting it. Queue contents are flushed in batches, so a crash MAY lose the
last minute of traffic.

0.7 uses SQLite. Configurable [additional backends](../Plans/R1.0-Release/storage.md#backends)
are R1 work. Every backend MUST preserve the same identity, authority and
durability rules, behind the existing storage ports.

<details>
<summary>Write rules</summary>

| Rule | Requirement |
|---|---|
| Durable | entities, credentials, queue contents, per-queue counters |
| A management write | validates, commits the complete change as one transaction, then publishes |
| Publication | no reader ever observes a write before its commit: a write stages its entities under the one registry lock every reader also takes, and a failed commit restores every one of them before the lock is released |
| Atomicity | invalid input fails the whole write; no partial update is ever visible |
| Startup | load every durable entity, then build derived indexes such as token to principal and user ID to status. An incorrect record is always ignored — not loaded, not repaired — and reported as a [conceptual error](#errors-and-alerts); the rest of the node still starts |
| List fields | the API MUST provide atomic add, add-if-absent and remove for `allow`, `maintainers` and `deliver_to` |

That ordering is write-through. It covers record lifecycle changes too.

</details>

<details>
<summary>Queue checkpoint</summary>

Queue contents and their `in`, `out`, `dropped` and `expired` counters keep the
[checkpoint boundary](04-messaging.md#durability): in memory during traffic, flushed as one batch every minute and on graceful shutdown, never once
per message, so a crash MAY lose changes since the last flush. State from
before this model is never imported: an installation moves to it by a
[clean reinstall](../Plans/R0.8-MVP/0.7-cutover.md#procedure).
A durable queue whose record is absent or cannot hold a queue is such an
incorrect record: startup MUST ignore and report it, and MUST NOT silently drop
or reattach that backlog.

</details>

<details>
<summary>What startup ignores</summary>

Startup also ignores and reports what no write of this version allows, and
keeps an ignored record's credential for the operator who repairs it:

| Stored | Ignored because |
|---|---|
| a record | it shares an internal ID with an earlier one |
| a Group | a nested member names no stored Group, so its grant would pass to whoever created the name |
| a local account mapping | its principal is no User or Agent; its socket is [not served](02-access.md#local-socket) |

</details>

<details>
<summary>Identity changes</summary>

Identity changes MUST invalidate stale credentials and grants in durable state
and memory together:

| Change | Requirement |
|---|---|
| Ownership transfer | update and commit the owned record, publish the new view, rewrite no unrelated record |
| Removing an Agent or record | drop every stored ACL, Maintainer, Group-member and Deliver-To reference to that name in the same transaction |
| Reusing a name | inherit no authority and no delivery from the former holder |
| Users | never removed |

</details>

<details>
<summary>Lock, publication failure, driver and reload</summary>

One active daemon SHOULD use a database, and it MUST take whatever exclusive
lock the storage engine offers before serving, so a competing instance cannot
start. Detecting a lock lost afterwards is not required. Failing to publish a
committed change leaves the daemon unable to answer with authority, and it MUST
then refuse every read and write with a stated reason rather than answer from
its cached view. Whether the process exits is its own decision.

The SQLite driver is `modernc.org/sqlite`.

A reload API or SIGHUP (`kill -HUP <pid>`) MAY be added later; no current
operation needs one. It would replace one complete view with another and rebuild the
derived indexes, never exposing a partly reloaded state.

</details>

## Logs

Three logs under `/var/log/agent-bus/`: an on-demand `debug.log` of requests,
an `audit.log` of administrative actions and entity edits, and an `error.log`
of what needs attention, copied to syslog.

<details>
<summary>The three logs</summary>

The daemon writes three logs under `/var/log/agent-bus/`, which the unit creates
for the daemon account and the `adm` group may read; setup installs an ordinary
`logrotate` configuration for them.

| File | Holds | Like |
|---|---|---|
| `debug.log` | **off by default, written only on demand** — a daemon flag at start, or an enable/disable call only the daemon Owner may make while it runs: one line per request — time, caller, route, result and duration | an nginx access log |
| `audit.log` | every administrative action and every [entity edit](#-registry-record) | an audit trail |
| `error.log` | warnings and errors only — something needs attention — and each line also to syslog at matching severity | an nginx error log |

No log may contain a token, secret, configuration body or message body.

</details>

## Errors and alerts

A conceptual error is a condition this model says cannot occur: a violated
invariant, corrupt stored state, or a failure to answer with authority. It is
reported twice, to syslog and the error log. An ordinary refusal — malformed
request, denied permission, unknown name — is not one, and goes only to its
caller.

<details>
<summary>Where each goes</summary>

Every conceptual error and alert MUST be reported twice: to syslog at a
severity matching the condition, and to the [error log](#logs). An ordinary
refusal is answered to its caller and appears nowhere else, save as its request's
line while the debug log is on.
[Entity-edit logging](#-registry-record) goes to the audit log: authorized
edits, not impossible states.

</details>

## Entities

### 👤 User

A User has a stable internal `user_id`, a globally unique name whose realm is
part of its identity, and exactly one 👤 record that is never removed. An
inactive User makes every record it owns inactive too.

<details>
<summary>Fields</summary>

| Field | Requirement |
|---|---|
| `user_id` | stable internal `uint32`; persisted, never reused, never the public identity |
| `name` | canonical `user` or `user@team`; globally unique and required. The realm is optional and part of the identity, so `alice` and `alice@team` are different Users |
| unique secondary identifiers | normalized email, GitHub login, Twitter/X username compared case-insensitively; each unique when present |
| profile | person name, company, location, photo or Gravatar; not identifiers and need not be unique |
| Ed25519 public key | when key-based enrolment is used |
| `created_at`, `updated_at`, `last_used_at` | |
| `status` | `active` or `inactive` |

</details>

<details>
<summary>The 👤 record, inactivity and reactivation</summary>

Every User has exactly one 👤 record: `kind` = `user`, `owner_id` = that
`user_id`. It MUST NOT be removed while the User exists, and removing it is
refused. A User without its 👤 record, found at startup, is corrupt state and
a [conceptual error](#errors-and-alerts), and so is a 👤 record whose
`owner_id` names no User; like every incorrect record, each is ignored.

An inactive User makes every record it owns inactive too, under the
[inactive-record rule](#common-record-fields): hidden and `404`, not a
separate suspended state.

Under the owning [User-state contract](01-identity-and-roles.md#user-states),
an Administrator may reactivate an ordinary User but not another
Administrator; only the daemon Owner may reactivate an Administrator.

</details>

### 😈 Daemon

The daemon is configuration, not a registry entity. It is owned by the daemon
Owner; version, build information, hostname, uptime and call counters are
runtime facts rather than durable daemon fields.

<details>
<summary>Fields</summary>

| Field | Requirement |
|---|---|
| `owner_id` | the daemon Owner |
| description | |
| listen addresses | TCP and Unix-socket addresses where applicable |

</details>

### 🔐 Token

A token acts as its User, or as one Agent that User owns. Transferring an
Agent moves its tokens in the same commit. A token whose User and Agent do not
match is corrupt state: ignored, reported at `alert`, never repaired.

<details>
<summary>Fields</summary>

| Field | Requirement |
|---|---|
| `token` | |
| `user_id` | stable `user_id`; always present |
| `agent_id` | stable `registry_id` of an Agent, or absent |
| `created_at`, `updated_at`, `last_used_at` | `last_used_at` follows the [statistics persistence schedule](10-modules.md#statistics-persistence) and means credential use, not necessarily a browser login |

</details>

<details>
<summary>User and Agent tokens, transfer, and a mismatched pair</summary>

- Without `agent_id` the token acts as the User alone.
- With `agent_id` the named User MUST be that Agent's Owner. The Agent is then
  the acting principal for access, routing and delivery, and the User is who it
  acts for; the User's inactivity refuses the token exactly as it suspends the
  Agent.
- Reassigning an Agent MUST reassign its tokens in the same committed update as
  the ownership change, never as a later step. A failed token write MUST abandon
  the transfer rather than commit an ownership change the tokens do not follow.

A mismatched pair can therefore come only from a failed write, and is corrupt
state rather than a refusal path. The daemon MUST ignore that token, which then
authenticates nothing, and MUST report it as a
[conceptual error](#errors-and-alerts) at `alert` severity naming the token's
User, Agent and current Owner. It MUST NOT repair the row or reinterpret it as
a User token.

</details>

### 📋 Registry record

Every registry record, and the daemon, is owned by a 👤 User. An Agent may
manage what it does not own, but creating or managing an object never makes it
the owner or grants it authority. Every direct edit and administrative action
writes one audit-log entry, and every field is validated by its own contract.

<details>
<summary>Ownership and Maintainers</summary>

A registry record MUST be owned by a 👤 User, and so MUST the daemon. An Agent
MAY manage what it does not own; managing or creating an object never makes it
the owner, and creating one grants the Agent no authority over it. Where an
Agent must manage a record, the Owner names it among the Maintainers: a rare,
explicit exception.

</details>

<details>
<summary>Audit entries</summary>

Every direct edit to a User, registry record or Group, and every other
administrative action, MUST write one entry to the [audit log](#logs):

| Entry | Requirement |
|---|---|
| actor | the authenticated User or Agent |
| operation, target, result | always |
| client IP | when one exists; a Unix socket request has none and MUST NOT invent one |
| a `status` change | is such an edit, so suspension is never silent |
| credential operations, reads, sends, consumes | write no audit-log entry; while the debug log is on, each is a request line there |

</details>

<details>
<summary>Validation</summary>

Every field MUST be validated and normalized by its own contract, which MUST
state that normalization explicitly. Validating a format authorizes no
interpretation of application-specific values.

</details>

#### 🏷️ Record kind

The closed set of record kinds:

| Kind | What it is | Channel |
|---|---|---|
| 👤 `user` | the queue belonging to a User, under that User's name | yes |
| 👾 `agent` | an Agent and the queue it reads | yes |
| 📮 `queue` | a named competing-consumer queue | yes |
| 📣 `pubsub` | fan-out: keeps nothing, copies each publication to its `deliver_to` | yes |
| 📡 `service` | information about an external service, protected by an ACL | no |
| 👥 `group` | a named list of typed actors | no |

#### Actors and ASCII textarea syntax

An **actor** is a 👤 User, 👾 Agent, or 👥 Group. ACL, Maintainer and
group-member textareas accept one ASCII term per line, and the prefix alone
says what a term names:

| Actor | Term |
|---|---|
| 👤 User | `alice@team` |
| 👾 Agent | `#worker@team` |
| 👥 Group | `@support` |
| Every active registered User and every active Agent | `*` |
| Owner and its directly owned Agents | `@owner` |
| The record's own Agent | `@agent` |

<details>
<summary>Prefixes, runtime terms and input rules</summary>

- A term is stored exactly as written, and for the first three it **is** the
  record's canonical name: a bare `alice@team` names a User and nothing else.
- An Agent's `#`, like a Group's `@`, is part of its canonical name everywhere —
  registry key, token identity, envelope `from` and `to`, what the Agent is told
  it serves, what a caller sends to. Nothing strips it, so `name` is unique
  across kinds and no term needs a lookup to say what it names.
- Every face MUST require the prefix wherever an Agent is named — API bodies,
  CLI arguments, MCP arguments, registration, credential issue, configuration
  files and what a process is told it serves — and MUST NOT complete, guess or
  tolerate an unprefixed one. An unprefixed name names a User or nothing.
- Because a shell reads an unquoted `#` as a comment, the CLI MUST also accept
  `--agent worker@srv1`, which names `#worker@srv1`. It adds the `#` only when
  absent, never a second one; a quoted `'#worker@srv1'` stays valid.
- In a URL the `#` MUST be percent-encoded as `%23`, since an unescaped one
  starts a fragment and the rest of the name never reaches the daemon.
- The last three terms resolve at each check instead of naming a stored entity.
  Only the first three may be created, and none of the last three may be stored
  as an entity or nested in a group. `@owner` and `@agent` are therefore
  reserved names: no Group named `@owner` or `@agent` can exist.
- `@agent` is valid only on a 👾 record, which is the only kind with an Agent of
  its own; on any other kind it is refused, and the whole update with it.
- Whitespace is trimmed, duplicate terms are rejected or normalized
  deterministically, and one invalid term rejects the complete update.
- Unicode glyphs are for display only and MUST NOT be required in editable or
  machine-readable values.

The [ACL rules](02-access.md#acl), including the empty-list default, wildcard
eligibility and `@owner`, remain in force unless explicitly revised.

</details>

#### Common record fields

Every record has an owner, a kind, a globally unique name, a description, a
status and Personal, Maintainer and allow settings; the rest depend on the
kind, and a field a kind cannot have is refused. **A record whose `status` is
not `active` MUST be treated as no such entity.** A message succeeds while at
least one recipient takes it.

| Field | 👾 | 📮 | 📣 | 📡 | 👥 |
|---|---|---|---|---|---|
| `ttl`, `bound`, `overflow` | ✓ | ✓ | — | — | — |
| `deliver_to` | one slot | one slot | list | — | — |
| `config`, `secret` | ✓ | — | — | ✓ | ✓ |
| `addr`, `protocol` | — | — | — | ✓ | — |

<details>
<summary>Fields every record carries</summary>

| Field | Requirement |
|---|---|
| `registry_id` | stable internal ID; persisted, never reused, never the public identity |
| `owner_id` | the owning User |
| `kind` | one value from the closed record-kind enum |
| `name` | canonical `name`, `name@team` or `template/instance@team`; globally unique and required. The realm is optional and the last `@` separates it. A 👾 name begins with `#` and a 👥 name with `@`, so the name alone says the kind. The ordinary rules still apply after the prefix, so `@support@srv1` is a Group with a realm |
| `description` | |
| `personal` | the intended audience is the Owner and the Agents that Owner owns; the record's `allow` and `maintainers` admit that cohort and nothing wider |
| `maintainers` | typed actor terms |
| `allow` | typed actor terms: the ACL on every kind that has one, and the membership list on a 👥 |
| `status` | `active` or `inactive` |
| `created_at`, `updated_at` | maintained by the system, not editable by callers |

Every record carries those, except that a 👤 has neither `maintainers` nor
`allow`. The rest depend on the kind, as the table above gives them.

</details>

<details>
<summary>The 👤 record, and fields a kind cannot have</summary>

A 👤 record is the User's own inbox: it takes `ttl`, `bound` and `overflow` and
nothing else from this table, who may reach it follows the
[User delivery rule](#-channels) rather than a list, and its `personal` is fixed true —
everything not Personal is shared, in the web views as everywhere else, and a
User is not.

A `—`, and any field a kind is not listed as carrying, means the kind cannot
have it: submitting one is refused, never stored and ignored.

</details>

<details>
<summary>An inactive record is no such entity</summary>

**A record whose `status` is not `active` MUST be treated as no such entity**,
whatever its kind:

| | |
|---|---|
| reads, discovery, lookup, a Service's secret | show nothing |
| every operation naming it | refused |
| what it grants | nothing, so no membership path through it reaches an actor |
| waiting readers that lose authority | released |
| the record itself | stays stored, keeps its canonical name reserved, and MUST refuse registration under that name |
| the one exception | a dedicated read-only call for the web face shows inactive records to the actors their `allow` admits, and always to the daemon Owner. It reads; it sends, consumes, drains, transfers and removes nothing |
| reactivation | a status edit by the record's Owner, a Maintainer or the daemon Owner, never a re-creation; to anyone else the record stays no such entity |

</details>

<details>
<summary>Flow outcomes: absent and inactive recipients</summary>

An absent record is the same case. Whether the caller is refused depends on
whether anything still gets through:

| This message | Outcome |
|---|---|
| at least one recipient takes it | it succeeds; each failed recipient is counted as its own `dropped` and written to the error log |
| no recipient takes it | an error to the caller before anything is stored or counted |

The error log takes a flow that was configured and broke — a publication's
lost or refused copies, and a route whose destination is gone or no longer
allows its source. A direct send to an absent or inactive name is an ordinary
refusal: its caller is told, and only the debug log records it.

A missing recipient beside working ones MUST NOT break the flow that works.
A missing sole recipient is the flow: a direct send to an absent or inactive
name, a publication whose recipients have all failed, and a forwarding route
whose one destination is gone are all refused to the caller.

</details>

#### Authority rules

| Who | MAY | MUST NOT change |
|---|---|---|
| Owner | transfer ownership, replace the Maintainers list, and everything a Maintainer may | |
| Maintainer | edit the description, ACL, status and the operational fields allowed for that kind | name, kind, owner, Maintainers, Personal classification, `created_at`, `updated_at` |
| The matching Agent principal, on its own record | what a Maintainer may | what a Maintainer may not |

`@agent` is an input alias for that same principal.

<details>
<summary>Authorization and ownership transfer</summary>

A modifying operation MUST be authorized against current state, including the
caller's right to change each submitted field, and the complete candidate record
MUST then be validated. Proposed changes MUST NOT supply their own authority.
Checking and applying a multi-field update MUST be one atomic operation.

Ownership transfer is an ordinary authorized record change: the old Owner
changes the owner, the daemon commits that record update and publishes the new
complete view, and subsequent checks use the new Owner. Runtime terms such as
`@owner` resolve against it. Restart loads that same saved ownership; it does
not authorize the transfer again or restore the old Owner.

</details>

## Kind-specific fields

### 📢 Channels

`deliver_to` is the recipient list on a 📣 and zero or one forwarding
destination on a 👾 or 📮, resolved against the registry. A 👾 or 📮 MAY forward
through that one slot: for `sender → A → B` the sender passes A's ACL and B's
ACL lists A. A forwarded envelope keeps its original sender, carries
`original_to` and a forward counter, and a step past ten is an error.

<details>
<summary>What <code>deliver_to</code> accepts</summary>

`deliver_to` is the recipient list on a 📣 and zero or one forwarding
destination on a 👾 or 📮. Which it is depends on the record a term resolves to,
so the daemon MUST resolve it against the registry:

| Field | Accepts |
|---|---|
| `deliver_to` on 📣 | 👾, 👥, 📮, 📣 |
| `deliver_to` on 👾 / 📮, one slot | 👾, 📮, 📣 |

- Every other term is refused, an unresolvable one included. An entry is refused
  for its kind rather than for permission, and a refusal names the term and
  stores nothing.
- A 👥 term is expanded at publication, and its membership is actors only. An
  inactive Group is ignored and logged: having no inbox, it contributes neither
  a recipient nor a `dropped`. If that leaves the publication with no recipient
  at all, the caller is refused like any other broken flow.
- A 👤 User takes a direct send from an Agent exactly when that Agent's `allow`
  admits the User: if the User may reach the Agent, the Agent may answer it.
  The Agent's ACL is the only list consulted. The daemon does not decide
  whether a message is a reply: the sender knows its own tags and matches the
  answer by topic and tag, as [reply routing](04-messaging.md#reply-routing)
  already works. A person does not send to a person, so no User sends to a
  User. A User is never a published copy's recipient or a route's destination;
  such an attempt MUST be answered with an error, never discarded and never
  counted as a drop.
- In the one-slot form, add succeeds only while empty and otherwise names the
  occupied field; remove clears it; replacement is an explicit whole-field
  write, never an add overwriting a concurrent choice. A write carrying two
  destinations stores nothing.

</details>

<details>
<summary>Forwarding checks</summary>

**Forwarding — owner-corrected September 20, 2026.** A 👾 Agent or 📮 Queue
record MAY forward to another destination. For `sender → A → B`:

| Check | Requirement |
|---|---|
| the sender | MUST pass A's ACL |
| B's ACL | MUST list the forwarding record A, both before the route is stored and again at delivery; the sender needs no access to B |
| a substitute | neither the original sender's nor A's Owner's access to B stands in for B allowing A |
| whose right | the daemon's selected source record, never a caller-supplied sender or provenance field |

For a 📮 source that is a channel-name ACL reference, not a credential-bearing
principal. Revoking B's permission denies forwarding without clearing
`deliver_to`, and restoring it resumes; human faces MUST distinguish a
configured route from a currently allowed one.

</details>

<details>
<summary>The forwarded envelope, destination rules and counters</summary>

A forwarded envelope keeps its original sender and MUST carry one `original_to`
naming the destination it came through, plus a forward counter. Every step
increments that counter, a topic's fan-out into another channel included, and a
step past ten MUST be an error — that is what ends a cycle between two topics,
or a queue routing back into the topic that fed it.

The destination applies its TTL and deadline, bound, overflow and counter rules
as for a direct send; forwarding adds no policy of its own. A refusal under
those rules rejects the original send before anything is stored or counted, and
so does an inactive or absent destination, the one slot leaving the message
nowhere else to go.

| Outcome | Counters |
|---|---|
| forwarded | the source keeps no copy and changes neither `in` nor `out`; the destination increments `in`, then `out` when a reader receives it |
| strict overflow at the destination | the original send is refused and no counter changes |
| ring overflow at the destination | its oldest message is evicted and its own `dropped` increments |

The forwarding record counts neither overflow case.

</details>

#### PubSub routing

A 📣 routes and stores nothing of its own. Its `in` counts publications at
least one recipient took and its `out` counts accepted copies. Each recipient
is an independent branch under its own rules, so one broken recipient never
breaks a working pipeline.

<details>
<summary>Counters and branches</summary>

A 📣 routes and stores nothing of its own: a copy is delivered when a recipient
inbox accepts it — through that recipient's own route included — not when a
reader consumes it. Its `in` and `out` are routing counters, neither depth nor
consumption:

| Counter | Increments |
|---|---|
| `in` | once per publication at least one recipient took |
| `out` | once per accepted copy, and never again when that copy is read |

A failed copy increments neither, so total failure and an empty recipient set
leave both unchanged. Persist them under the
[statistics schedule](10-modules.md#statistics-persistence).

Each recipient is an independent branch answered under its own destination's
rules, and a failed one MUST NOT roll back or prevent any other: one broken
recipient may not break a working pipeline. The caller's answer is the
[flow outcome](#common-record-fields) — success while anything got through, an
error when nothing did — and a partial failure is also a warning in the daemon
log and syslog, carrying no credential and no body. Forwarding into a 📣 is
answered the same way, and each branch it fans out to increments the forward
counter.

</details>

### 📡 Service

| Field | Requirement |
|---|---|
| `addr`, `protocol` | required |
| `secret` | optional; see [private values](#-private-values) |

### 🔒 Private values

`config` and `secret` are private bodies. The Owner or a Maintainer writes
them. The record's own principal reads them where one exists, and otherwise
the actors in `allow` do. Everyone else sees a SHA-256 digest.

<details>
<summary>Readers and validation</summary>

`config` and `secret` are private bodies. Both are written by the Owner or a Maintainer, read by the record's own
principal where one exists and otherwise by the actors in `allow`, and shown to
everyone else as a SHA-256 digest. Their content stays opaque and is the user's
responsibility. A 👥 has no principal of its own, so its `allow` — its
membership — reads them: every member reads a Group's secret.

| | Validation |
|---|---|
| `config` | JSON, compacted before storage and hashing ([normalization](03-records.md#why-a-digest-at-all)) |
| `secret` | an env file, checked for basic syntax and stored as written |

Invalid input rejects the complete write.

</details>

### 👥 Group

A Group's `allow` is its membership of Users, live Agents and Groups, and its
name begins with `@`. Nested resolution uses a visited set. A name carrying
its Owner's prefix is reserved to that User and never transferred. The
protected `@administrators` belongs to the daemon Owner alone.

<details>
<summary>Membership and who edits it</summary>

A Group is an ordinary record whose `allow` is its membership: typed User,
Agent or Group terms naming Users and live Agents that exist — never a queue, a
topic, a service or a name nothing holds — which its Owner, its Maintainers and
the daemon's Administrators MAY add and remove. Its `name`
begins with `@`.

This extends the existing
[group administration](01-identity-and-roles.md#groups) model: a User or an
Agent gains explicitly assigned control while daemon Owner and Administrator
authority remains.

Nested resolution MUST use a visited set, and grants membership only when a
finite path reaches the requested actor.

</details>

<details>
<summary>Owner-prefixed names</summary>

A name MAY carry its Owner's prefix: `@<owner>/<name>[@realm]`, where
`<owner>` is the User's whole name, realm included, and the realm after the
group's own name is the Owner's choice — `@alice@srv1/friends@batch1` is
`alice@srv1`'s. The prefix reserves the name: only that User creates or holds
it. A Personal Group MUST carry its Owner's prefix, so Users never compete for
a personal name, and one without it is refused. A prefixed Group MUST NOT be
transferred, because its name says whose it is and a name never changes; moving
one is an administrator editing the database and restarting the daemon. A
stored Group whose prefix is not its Owner, or a Personal one without a prefix,
is incorrect and ignored at load. Unprefixed names are the shared namespace.

</details>

<details>
<summary>The protected <code>@administrators</code></summary>

The protected `@administrators` group is outside this model: its Owner MUST be
the daemon Owner and MUST NOT be assigned independently of daemon ownership, it
has no Maintainers, and only the daemon Owner changes its direct membership.
Ordinary Group ownership or Maintainer assignment MUST NOT bypass that
boundary.

</details>

## Open questions

The [question index](../Plans/R0.8-MVP/QUESTIONS.md#open-questions) owns every open
choice; none is open on this page.

## History

What this model replaced, and the owner clarifications of September 19, 2026,
are [history](../Plans/R0.8-MVP/done/constitution-history.md#what-this-replaced);
nothing here depends on them.
