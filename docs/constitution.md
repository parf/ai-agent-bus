# Project Constitution

📌 **TL;DR:** Users own records; credentials identify Users or their Agents; access and management are separate. This reference defines validation, delivery, persistence and privacy guarantees. **MUST**, **MUST NOT**, **SHOULD** and **MAY** are normative.

For practical commands, start with [identity and authority](01-identity-and-authority.md#try-this-first) or [access](02-access.md#try-this-first).

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
| public key | none stored: key-based enrolment verifies a signature against the key the realm's directory publishes for that login |
| `created_at`, `updated_at` | |
| last use | not stored on the User; derived from its credential's use |
| `status` | `active` or `inactive` |

</details>

<details>
<summary>The 👤 record, inactivity and reactivation</summary>

Every User has exactly one 👤 record: `kind` = `user`, `owner_id` = that
`user_id`. It MUST NOT be removed while the User exists, and removing it is
refused. A User without its 👤 record, found at startup, is corrupt state and
a [conceptual error](#errors-and-alerts), and so is a 👤 record whose
`owner_id` names no User. Either refuses startup; neither is silently omitted.

An inactive User makes every record it owns inactive too, under the
[inactive-record rule](#common-record-fields): hidden and `404`, not a
separate suspended state.

Under the owning [User-state contract](01-identity-and-authority.md#user-states),
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
| listen addresses | TCP and Unix-socket addresses where applicable |

</details>

### 🔐 Token

A token acts as its User, or as one Agent that User owns. The stored identity pair determines which principal authenticates.

<details>
<summary>Fields</summary>

| Field | Requirement |
|---|---|
| current and previous token | the previous one still authenticates until the next rotation ([token lifetime](02-access.md#token-lifetime)) |
| `user_id` | stable `user_id` |
| `agent_id` | stable `registry_id` of an Agent, or absent |
| issued, used | no `updated_at`; use follows the [statistics persistence schedule](../src/MODULES.md#statistics-persistence) and means credential use, not necessarily a browser login |

</details>

<details>
<summary>User and Agent tokens, transfer, and a mismatched pair</summary>

- Without `agent_id` the token acts as the User alone.
- A credential MUST be issued only for a registered principal with its current internal IDs bound. A stored empty or structurally invalid pair refuses startup; no deferred binding is performed.
- With `agent_id` the named User MUST be that Agent's Owner. The Agent is then
  the acting principal for access, routing and delivery, and the User is who it
  acts for; the User's inactivity refuses the token exactly as it suspends the
  Agent.
- Reassigning an Agent MUST reassign its tokens in the same committed update as
  the ownership change, never as a later step. A failed token write MUST abandon
  the transfer rather than commit an ownership change the tokens do not follow.

A mismatched pair is corrupt stored state. The daemon MUST ignore that token, which then
authenticates nothing, and MUST report it as a
[conceptual error](#errors-and-alerts) at `alert` severity naming the token's
User, Agent and current Owner. It MUST NOT repair the row or reinterpret it as
a User token.

</details>

### 📋 Registry record

Every registry record, and the daemon, is owned by a 👤 User. Record management, auditing and validation follow the rules below.

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
| client IP | when one exists; a Unix socket request has none and MUST NOT invent one. An edit made in the web face reaches the daemon over its socket and carries no visitor IP |
| a `status` change | is such an edit, so suspension is never silent |
| setting a secret | audited with its target only, never its body |
| key-based enrolment | its actor is written as `(enrolment signature)`, since no credential exists yet |
| startup seeding | the daemon Owner, accounts and first token seeded at start are not audited |
| a token minted or rotated for another name | audited with that name only, never the token |
| a caller's own token and session operations, reads, sends, consumes | write no audit-log entry; while the debug log is on, each is a request line there |

</details>

<details>
<summary>Validation</summary>

Every field MUST be validated and normalized by its own contract, which MUST
state that normalization explicitly. Validating a format authorizes no
interpretation of application-specific values.

</details>

#### 🏷️ Record kind

The closed set of record kinds:

| Kind | What it is | Message delivery |
|---|---|---|
| 👤 `user` | the inbox belonging to a User, under that User's name | inbox |
| 👾 `agent` | an Agent and the inbox it reads | inbox |
| 📮 `queue` | a named competing-consumer queue | inbox |
| 📣 `pubsub` | fan-out: keeps nothing, copies each publication to its `deliver_to` | recipient copies |
| 📡 `service` | information about an external service, protected by an ACL | no |
| 👥 `group` | a named list of typed actors | no |
| 📚 `resource` | a card for data an MCP client may read; its source answers ([Resource records](03-records-resource.md#what-a-resource-is)) | no |

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

- A term is stored in canonical form, trimmed and lower-case, and for the first
  three it **is** the record's canonical name. In an ACL a bare name is a User
  or a 📮 or 📣 route source; it is not checked for existence, and `*` never
  admits a channel. A 👥's membership takes the three actor terms alone.
- An Agent's `#`, like a Group's `@`, is part of its canonical name everywhere —
  registry key, token identity, envelope `from` and `to`, what the Agent is told
  it serves, what a caller sends to. Nothing strips it, so `name` is unique
  across kinds and no term needs a lookup to say what it names.
- Agent identities in API and MCP arguments, registration, credential issue and configuration use the `#` prefix. An unprefixed record target does not select an Agent. The dedicated runner requires the same complete Agent name; it never guesses or adds a prefix.
- Quote Agent names in the shell: `'#worker@srv1'`. An unquoted `#` starts a shell comment.
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
kind, and a field a kind cannot have is refused. Inactive records and failed delivery branches follow the rules below.

| Field | 👾 | 📮 | 📣 | 📡 | 👥 | 📚 |
|---|---|---|---|---|---|---|
| `ttl`, `bound`, `overflow` | ✓ | ✓ | — | — | — | — |
| `deliver_to` | one slot | one slot | list | — | — | — |
| `config`, `secret` | ✓ | — | — | ✓ | ✓ | — |
| `addr`, `protocol` | — | — | — | ✓ | — | — |
| `script` | ✓, written by the runner | — | — | — | — | — |
| `roles` | ✓, the roles it understands ([roles](03-records-agent.md#roles)) | — | — | — | — | — |
| `resource` | — | — | — | — | — | ✓ ([Resource records](03-records-resource.md#what-a-resource-is)) |

<details>
<summary>Fields every record carries</summary>

| Field | Requirement |
|---|---|
| `registry_id` | stable internal ID; persisted, never reused, never the public identity |
| `owner_id` | the owning User |
| `kind` | one value from the closed record-kind enum |
| `name` | canonical `name`, `name@team` or `template/instance@team`; globally unique and required. The realm is optional and the last `@` separates it. A 👾 name begins with `#` and a 👥 name with `@`, so the name alone says the kind. The ordinary rules still apply after the prefix, so `@support@srv1` is a Group with a realm |
| `description` | |
| wire names | the API spells `description` as `descr`, `deliver_to` as `subs`, `updated_at` as `at`, and `owner_id` as `owner`, the owning User's name; the store keeps the owner's `user_id` beside it, on no answer |
| `tags` containing `personal` | the intended audience is the Owner and the Agents that Owner owns; its `allow` and `maintainers` admit that cohort and nothing wider, except that `allow` may also name the daemon's `#bus` ([Personal rules](03-records.md#personal-and-shared)) |
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
[User delivery rule](#-channels) rather than a list, and its `personal` tag is fixed —
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
| a read already waiting | finishes as admitted; the next request is refused ([authority at the door](#authority-at-the-door)) |
| the record itself | stays stored, keeps its canonical name reserved, and MUST refuse registration under that name |
| the one exception | a dedicated read-only call for the web face shows inactive records to their Owner, Maintainers and own Agent, to the actors their `allow` admits, and always to the daemon Owner. It reads; it sends, consumes, drains, transfers and removes nothing |
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
| daemon Owner | every Owner edit on any record, transfer included, but not its private values | |
| Owner | transfer ownership, replace the Maintainers list, and everything a Maintainer may | |
| Maintainer | edit the description, ACL, status and the operational fields allowed for that kind; write the [private values](#-private-values); remove the record | name, kind, owner, Maintainers, Personal classification, `created_at`, `updated_at` |
| Administrator, on a 👥 other than `@administrators` | add and remove members only | other record fields, unless acting as its Owner or a Maintainer |
| The matching Agent principal, on its own record | manage its settings, read its private values and use its locks/KV | private-value writes without a Maintainer grant; everything else a Maintainer may not change |

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

## Authority at the door

A request uses the caller identity and authority admitted when it starts. Revocation affects the next request; it does not recall delivered work.

A check of the records’ own routing state (a forward’s destination ACL, a
group recipient’s Owner consent) is not caller authority.

<details>
<summary>Rules</summary>

| Rule | Requirement |
|---|---|
| One check per request | authentication and the operation's authority are decided when the request is handled, once, under one hold when it reads and writes together |
| No mid-operation check | a lock wait, a long poll, a delivery hand-over or a second write does not re-ask a decision the request already made |
| Restart | every stored identity and grant is checked again at load ([what refuses the start](#what-refuses-the-start)) |
| Revocation | takes effect on the next request; an operation already admitted finishes as admitted |

</details>

## Kind-specific fields

### 📢 Channels

Channels route messages through `deliver_to`: a recipient list on pubsub or one forwarding destination on an Agent or queue. Routing preserves the original sender.

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
  for its kind, and a refusal names the term and stores nothing.
- Adding a Deliver-To entry requires management authority over its destination — the record, or a 👥 Group where groups are permitted. An unseen destination is answered as unknown. This is checked when added: a stored list keeps its entries when management changes. A one-slot route also requires its destination's ACL to admit the source, both at configuration and delivery.
- A 👥 term is the exception: each record it expands to receives a copy only
  while that record’s Owner manages the channel, checked at every publication.
- A 👥 term is expanded at publication, and its membership is actors only. A
  listed inactive Group is ignored and logged, and one nested inside a listed
  Group is skipped without a log line: having no inbox, it contributes neither
  a recipient nor a `dropped`. If that leaves the publication with no recipient
  at all, the caller is refused like any other broken flow.
- A 👤 User takes a direct send from an Agent exactly when that User may use
  the Agent: its `allow` admits the User, or the User is its Owner or a
  Maintainer. If the User may reach the Agent, the Agent may answer it,
  and no list of the User's is consulted. The daemon does not decide
  whether a message is a reply: the sender knows its own tags and matches the
  answer by topic and tag, as [reply routing](04-messaging.md#reply-routing)
  already works. A User may send to its own inbox but not another User's. A User is never a published copy's recipient or a route's destination:
  naming one on `deliver_to` MUST be answered with an error, and one reached
  through a Group at publication is skipped with an error-log warning and
  counted as no drop.
- In the one-slot form, add succeeds only while empty and otherwise names the
  occupied field; remove clears it; replacement is an explicit whole-field
  write, never an add overwriting a concurrent choice. A write carrying two
  destinations stores nothing.

</details>

<details>
<summary>Forwarding checks</summary>

A 👾 Agent or 📮 Queue record MAY forward to another destination. For `sender → A → B`:

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
increments that counter, a 📣's fan-out into another channel included, and a
step past ten MUST be an error. One send also passes through at most ten 📣
channels in all, side by side included. The eleventh is a Deliver-To loop
(two 📣 channels listing each other, or a queue routing back into the 📣
that fed it). The send answers an error with an error-log entry, and copies
already delivered on earlier branches stay. The limit
reports a loop; it does not undo one.

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

A 📣 has no inbox. Its routing counters describe publications and accepted copies, as defined below.

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
[statistics schedule](../src/MODULES.md#statistics-persistence).

Each recipient is an independent branch answered under its own destination's
rules, and a failed one MUST NOT roll back or prevent any other: one broken
recipient may not break a working pipeline. The caller's answer is the
[flow outcome](#common-record-fields) — success while anything got through, an
error when nothing did — and a partial failure is also a warning in the daemon
error log and, when enabled, syslog, carrying no credential and no body. Forwarding into a 📣 is
answered the same way, and each branch it fans out to increments the forward
counter. The whole-send fan-out loop limit above is the exception: it answers
an error while keeping earlier delivered copies.

A branch that fails beyond its listed recipient — through a forwarding 👾 or 📮,
or inside a nested 📣 — is counted in the listed recipient's `dropped`, never
the final destination's.

</details>

### 📡 Service

| Field | Requirement |
|---|---|
| `addr`, `protocol` | required |
| `secret` | optional; see [private values](#-private-values) |

### 🔒 Private values

Keep secrets in Secret and config in Configuration; see [configuration](03-records-agent.md#configuring-a-template).

Configuration is JSON visible to its record's **Owner and Maintainers** in the CLI and web. Secret is an env file whose stored values are never displayed on web pages. Both use the private-value authority below; an allow list grants use, not access to these values.

<details>
<summary>Readers and validation</summary>

Configuration and Secret MUST be written only by the
record's Owner or a Maintainer (Maintainer terms resolve as everywhere, nested
groups and `@owner` included), and read only by them and, on an 👾, by that
Agent itself. The allow list, a 👥's membership and the daemon Owner's office
grant neither: the daemon Owner reaches them only by being made a Maintainer
or by issuing the Owner's token, and both are audited. Everyone else gets neither bytes nor digests; a caller who may see
the record and not read its values is told they are private, and one who may
not see it that there is no such name. Their content stays opaque and is the
user's responsibility. The same authority — Owner, Maintainers and the
record's own Agent — uses a record's [shared locks](01-identity-and-authority.md#shared-locks)
and its [key-value store](03-records-kv.md#key-value-store).

| | Validation |
|---|---|
| `config` | JSON, compacted before storage and hashing ([normalization](03-records-agent.md#why-a-digest-at-all)) |
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
pubsub channel, a service or a name nothing holds — which its Owner, its Maintainers and
the daemon's Administrators MAY add and remove. Administrator authority permits
membership edits only; any other edit requires the Group’s Owner or a
Maintainer. Its `name`
begins with `@`.

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
transferred, because its name says whose it is and a name never changes. Create a correctly named group instead. A
stored Group whose prefix is not its Owner, or a Personal one without a prefix,
is incorrect and refused at load. Unprefixed names are the shared namespace.

</details>

<details>
<summary>The protected <code>@administrators</code></summary>

The protected `@administrators` group is outside this model: its Owner MUST be
the daemon Owner and MUST NOT be assigned independently of daemon ownership, it
has no Maintainers, and only the daemon Owner changes its direct membership.
Ordinary Group ownership or Maintainer assignment MUST NOT bypass that
boundary.

</details>

## Agent self-description

**An agent describes itself; the daemon never does.** What an agent offers
is knowledge of the agent's own code and changes with it, so it lives in the
agent's package and nowhere in the registry. The daemon stores none of it,
interprets none of it and only carries the asks:

| What | Where it is written | Asked with |
|---|---|---|
| the agent: description, license, version, author, homepage | `agent.json` | `--help`, `--help-json` |
| its methods and their arguments, with optional types, enums, roles and a destructive flag | `agent.json` `methods` | `--help`, `--help-json` |
| its Configuration keys: name, type, help, example, default | `agent.json` `config` | `--help-config` |
| its secret and environment keys: names, comments, required; never values | commented `env.dist` | `--help-secret` |

The format is in [package help](runner/configuration.md#package-help); where
secret values live is [two places for secrets](runner/security.md#secrets).

**Ask an agent what it can do: `agent-bus call <to> --help`.** The ask is the
envelope's `ask` field, never body text. An agent that serves calls SHOULD
answer `ask: help` with a description written for an AI reader:
what it is for, each method with its arguments, defaults and answer, an
example call, and what it sends on its own. Every bundled agent MUST. The
description MUST cover only what the caller may do: an action the caller's
roles would be refused is left out, so admin-only actions are not shown to
regular users. It is an ordinary answer and grants nothing; each call is
still checked.

<details>
<summary>Rules</summary>

| Rule | Requirement |
|---|---|
| The call | the envelope's `ask` is `help`, `help-config`, `help-secret`, `help-json`, `status`, or `mcp` for the tools, and its body is empty; `agent-bus call <to> --help`, `--help-config`, `--help-secret`, `--help-json` or `--mcp` sends it. No body text is reserved: `--help` in a body is ordinary input |
| Ask first | Every kind dispatches a nonempty ask before Configuration-dependent work, body parsing or normal handling. A supported ask is answered; every other ask answers `unsupported: <ask>` as ordinary text, not an error. Only no ask reaches normal work. The daemon still refuses asks outside the closed set |
| Script forwarding | The runner answers what package metadata can describe; any other ask reaches the script as `AGENT_BUS_ASK=<ask>` with an empty body. `--no-asks` on `agent-bus run`, or `"asks":"none"` in the package, answers unsupported instead of running the script |
| Observed status | While it owns the reader, the runner answers known starting, stopping or failed states itself; otherwise status reaches the running script, or answers unsupported under no-asks. It never attaches a fallback reader that could steal ordinary input: for a refused or unconfigured entry that has a token, and for a kept agent until its process runs, it reads asks only, answers `status` and the manifest's help asks, `unsupported: <ask>` for the rest, never runs the script and takes no ordinary message |
| Running status content | If an agent supports the empty-body `status` ask, it describes what it is doing, its backlog, its last error and a safe configuration summary. Never include secrets or raw private values. This is suggested support, not a requirement for every agent; runner-observed lifecycle states remain the runner's responsibility |
| Slow initialization | A slow-start agent must attach its reader and answer asks with `starting: <what it is loading>` before initializing, while ordinary messages stay queued. It uses the ask-only read (`asks=only`; `asks: "only"` in the TypeScript client; Slack does this while it loads the workspace). Simple agents need no starting phase |
| A packaged script | the runner and `agent-bus run --package-dir` normally render package metadata; `help: native` delegates to the script when runtime Configuration determines scope ([package help](runner/configuration.md#package-help)) |
| An unpackaged or self-serving agent | answers its own asks; a script receives `AGENT_BUS_ASK` and no input. At least `help` is required; its help names which extra asks it answers |
| No MCP tools | An unpackaged script has no tools: its runner answers `mcp` without starting it. Offering tools requires a package declaring `help: native`. An agent that answers asks but has no MCP tools answers `mcp` with `{"tools": []}`, the MCP shape for none, so every caller gets one shape |
| The answer | text for `help`, `help-config` and `help-secret`; the whole description as JSON for `help-json`; methods, arguments, returns and required roles from the package. Help names the supported extra asks. `help-config` describes Configuration keys; `help-secret` gives environment names, comments and required status, never values |
| Access | written from the roles the daemon asserts on the message, never from what the caller claims |
| AI sessions | a session behind the MCP face asks an unfamiliar agent `--help` before calling it, and answers `--help` with what it can do for that caller |

</details>

## Persistence and loading

One write-through SQLite store: a write validates, commits as one transaction,
then publishes, so no reader sees an uncommitted or partial change. Startup
validates the registry snapshot as a whole and refuses an invalid one; it does
not repair fields or serve a partial registry. Queue contents are flushed in batches, so a crash MAY lose the
last minute of traffic.

<details>
<summary>Write rules</summary>

| Rule | Requirement |
|---|---|
| Durable | entities, credentials, queue contents, per-queue counters, activity days ([durability](04-messaging.md#durability)) |
| A management write | validates, commits the complete change as one transaction, then publishes |
| Publication | no reader ever observes a write before its commit: a write stages its entities under the one registry lock every reader also takes, and a failed commit restores every one of them before the lock is released |
| Atomicity | invalid input fails the whole write; no partial update is ever visible |
| Startup | validate the complete durable snapshot before loading entities and building derived indexes. Incorrect state [refuses the start](#what-refuses-the-start); no partial registry, field repair or record salvage is published |
| List fields | the API MUST provide atomic add, add-if-absent and remove for `allow`, `maintainers` and `deliver_to` |

That ordering is write-through. It covers record lifecycle changes too.

</details>

<details>
<summary>Queue checkpoint</summary>

Queue contents and their `in`, `out`, `dropped` and `expired` counters keep the
[checkpoint boundary](04-messaging.md#durability): in memory during traffic, flushed as one batch every minute and on graceful shutdown, never once
per message, so a crash MAY lose changes since the last flush.
A durable queue whose record is absent or cannot hold a queue is such an
incorrect record: startup MUST refuse it, and MUST NOT silently drop
or reattach that backlog.

</details>

<details>
<summary>What refuses the start</summary>

#### What refuses the start

Startup validates the whole registry snapshot before replacing registry state. These errors refuse the start:

| Stored state | Reason |
|---|---|
| Unknown, duplicate or differently cased schema members | Only the declared format is accepted; Configuration payloads remain opaque JSON |
| Missing markers, invalid or duplicate internal IDs | Identity and account-map boundaries must be unambiguous |
| A missing daemon Owner, User record or record Owner | The complete authority graph must exist |
| Duplicate unique profile fields, broken Group references or invalid Personal assignments | Grants and identities cannot be selected or repaired by load order |
| A queue without a receiving record, or message rows without their queue | Backlogs cannot be dropped or attached to another identity |
| Invalid tags, disallowed inbox/endpoint fields or oversized Configuration | A stored record must satisfy the same constraints as a write |
| A damaged `@administrators` or invalid account mapping | Administrative authority and local login principals must be valid |
| A mapped OS account that does not exist | Its socket cannot be assigned safely |

The daemon does not skip bad entities or strip bad fields to make a partial node start. Well-formed [ownerless credentials](02-access.md#ownerless-credentials) have their own cleanup rule.

</details>

<details>
<summary>Identity changes</summary>

Identity changes MUST invalidate stale credentials and grants in durable state
and memory together:

| Change | Requirement |
|---|---|
| Ownership transfer | commit the new Owner, rebind Agent credentials and remove grants on the former Owner's Personal records that the transferred Agent can no longer hold; publish together |
| Removing an Agent or record | drop every stored ACL, Maintainer, Group-member and Deliver-To reference to that name in the same transaction |
| Reusing a name | inherit no authority and no delivery from the former holder |
| Users | never removed |

</details>

<details>
<summary>Lock, publication failure, driver and reload</summary>

One active daemon SHOULD use a database, and it MUST take whatever exclusive
lock the storage engine offers before serving, so a competing instance cannot
start. Detecting a lock lost afterwards is not required.

Publication has no failure of its own: a write stages in memory under the
registry lock, and a failed commit restores what it staged and refuses that
write alone.

The SQLite driver is `modernc.org/sqlite`.

SIGINT and SIGTERM stop the daemon cleanly and remove its sockets. SIGHUP is
not handled: it ends the supervisor and leaves its sockets, while the bus child
still flushes and stops. Use a full restart for daemon settings that require new listeners.

</details>

## Logs

Request diagnostics, administrative audit and operational errors have separate logs. Tokens, private values and message bodies MUST NOT appear in them.

The syslog copies below apply only with the daemon’s `--syslog` flag, which
installed systemd units pass explicitly. Standalone and development
processes keep diagnostics in their own log directory and stderr only.

<details>
<summary>The three logs</summary>

The daemon writes three logs under `/var/log/agent-bus/`, which setup creates
for the daemon account, set-group-ID `adm` so that group may read them
([setup § logs](administration.md#logs)); setup also installs an ordinary `logrotate`
configuration for them.

| File | Holds | Like |
|---|---|---|
| `debug.log` | **off by default, written only on demand** — a daemon flag at start, or an enable/disable call only the daemon Owner may make while it runs: one line per request — time, caller, route, result and duration | an nginx access log |
| `audit.log` | every administrative action and every [entity edit](#-registry-record) | an audit trail |
| `error.log` | warnings and errors only — something needs attention — and each line also to syslog at matching severity; a warning that traffic raises is written at most once a minute per text, the next one counting those held back | an nginx error log |

A field of a debug or audit line keeps its first 200 characters and is marked
`…cut` past them, so one request cannot grow a log by what it sent.

</details>

## Errors and alerts

A conceptual error is a condition this model says cannot occur: a violated
invariant, corrupt stored state, or a failure to answer with authority. A malformed request, denied permission or unknown name is an ordinary refusal instead.

<details>
<summary>Where each goes</summary>

Every conceptual error and alert MUST reach the [error log](#logs), and syslog at a matching severity when syslog is enabled. An ordinary
refusal is answered to its caller and reaches neither of them: a refused
administrative action is still [audited](#-registry-record) with its result,
and any request is a line in the debug log while that is on.
[Entity-edit logging](#-registry-record) goes to the audit log: authorized
edits, not impossible states.

</details>

## External protocols

The MCP face uses the [2025-11-25 Model Context Protocol](https://modelcontextprotocol.io/specification/2025-11-25) through its MCP SDK, which negotiates a supported revision during initialization. Resource descriptors and reads follow that interface. Client capabilities still determine which features a connected client can use.
