# Project Constitution

**Status:** In discussion  
**Date:** September 19, 2026

This document describes the intended model. It is not a statement that every
rule below is implemented already. Open choices are named explicitly.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

## Clarified direction

Owner clarification, September 19, 2026:

- Objects belong to Users; an Agent's management authority does not make it an
  owner. Existing documentation allowing non-User ownership needs reconciliation.
- Group ownership and Maintainers extend existing daemon administration, adding
  control by Users and Agents rather than replacing administrative authority.
- Persistence follows a write-through cache model.
- `banned` is removed; User state is `active` or `inactive`. Record `active`
  replaces the old Disabled switch with the opposite polarity.
- ACL syntax is extended, not replaced. Existing terms remain supported; the
  explicit Agent marker is additive.

These are intended rules, not evidence that the running implementation already
enforces them. The current topic docs still need reconciliation after discussion.

## Persistence and loading

0.7 uses SQLite. Configurable [additional backends](../Plans/R1/storage.md#backends)
are R1 work. Durable entities, credentials, queue contents and durable per-queue
counters MUST be persisted in the selected backend. A management change MUST
commit as a transaction before its new in-memory view is published. All backends
MUST preserve the same identity, authority and durability rules. Storage access
stays behind the existing ports.

Queue contents and their `in`, `out`, `dropped`, and `expired` counters retain
the existing [checkpoint boundary](04-messaging.md#durability): they update in
memory during traffic and flush as one consistent batch every minute and on
graceful shutdown. There is no database write per message. A crash MAY lose
queue changes since the last successful flush. The JSON dump is cutover input,
not a second runtime store after 0.7 activation. Startup MUST reject a durable
queue whose record is absent or cannot hold a queue; it MUST NOT silently drop or
reattach that backlog.

Only one active daemon MAY use a database. The daemon MUST establish exclusive
ownership before serving and reject a competing instance. After losing that
exclusivity, it MUST refuse every read and write with a stated reason and MUST
NOT answer from its cached view. This observable rule does not prescribe whether
the process remains running or exits. It applies to every supported backend.

All durable entities MUST be loaded into memory at startup.

Normal management APIs MUST validate and persist a complete change before it
becomes visible in memory. Invalid input MUST make the whole write fail
atomically.

Write-through identity changes MUST invalidate stale credentials and grants in
both durable state and memory. Reusing a record name MUST NOT inherit the former
record's authority or deliveries. Users are never removed. Ownership transfer
updates the owned record, commits it, and publishes the new complete in-memory
view; it does not rewrite unrelated records. Removing an Agent or record is
different: the same transaction MUST remove every stored ACL, Maintainer,
Group-member and Deliver-To reference to that name. A later record with the same
name starts with no authority or delivery inherited from the removed record.

After a transaction commits, core MUST construct and publish one complete new
view rather than mutate the live maps in place. If publication cannot complete,
the daemon MUST refuse every read and write with a stated reason rather than
answer with authority older than the committed state. This observable rule does
not prescribe whether the process remains running or exits. This invariant
applies to record lifecycle changes even though 0.7 does not add an exhaustive
deletion-specific crash matrix.

This is write-through behavior: memory serves the loaded view, and a management
write updates durable state before publishing the changed view. The existing
[persistence-failure contract](04-messaging.md#administrative-crash-recovery)
is labeled as built through 0.6; its memory-before-persistence failure behavior
is replaced in 0.7 by this ordering.

A future reload API or SIGHUP (`kill -HUP <pid>`) MAY reload durable entities.
If added, reloading MUST replace one complete in-memory view with another;
callers must never observe a partly reloaded state. It is not required for
ownership transfer or any other 0.7 operation.

Derived indexes, such as token → principal and user ID → status, MUST be built
when state is loaded and rebuilt after a reload.

The API MUST provide atomic add and remove operations for list fields such as
`allow`, `maintainers`, `members`, and `deliver_to`.

## Errors and alerts

A conceptual error is a condition this model says cannot occur: a violated
invariant, corrupt stored state, or a failure that leaves the daemon unable to
answer with authority. An ordinary refusal is not one — a malformed request, a
denied permission or an unknown name is answered to its caller and reported
nowhere else.

Every conceptual error and every alert MUST go to syslog at a severity matching
the condition, and the same message MUST also be written to the daemon's error
log. Reporting to one destination is not enough: syslog is what reaches an
operator who is not reading the daemon's files, and the error log is what stays
beside its other output. Neither copy may contain a token, secret,
configuration value or message body.

This is separate from [entity-edit logging](#-registry-record), which records
authorized edits rather than impossible states.

## Entities

### 👤 User

- `user_id`: stable internal `uint32`; persisted, never reused, and not exposed
  as the public identity.
- `name`: canonical `user` or `user@team` identity; globally unique and
  required. The realm is optional and part of the identity, so `alice` and
  `alice@team` are different Users.
- Unique secondary identifiers, when present:
  - normalized email;
  - GitHub login;
  - Twitter/X username, compared case-insensitively.
- Profile fields, which are not identifiers and need not be unique:
  - person name;
  - company;
  - location;
  - photo or Gravatar.
- Ed25519 public key, when key-based enrolment is used.
- `created_at`, `updated_at`, and `last_used_at`.
- `status`: `active` or `inactive`. This model has no separate `banned` state.

`inactive` replaces the former paused/banned distinction under the owning
[User-state contract](01-identity-and-roles.md#user-states): an Administrator
may reactivate an ordinary User but cannot change another Administrator, only
the daemon Owner may reactivate an inactive Administrator, and no second
suspension level remains.

### 😈 Daemon

The daemon is configuration, not a registry entity.

- `owner_id`;
- description;
- listen addresses, including TCP and Unix-socket addresses where applicable.

Version, build information, hostname, uptime, and call counters are runtime
facts rather than durable daemon fields.

### 🔐 Token

Tokens currently authenticate actors: 👤 Users and 👾 Agents.

Each token MUST name a 👤 User, and MAY additionally name a 👾 Agent:

- `token`;
- `user_id`: stable `user_id`; always present;
- `agent_id`: stable `registry_id` of an Agent, or absent;
- `created_at`, `updated_at`, and `last_used_at`.

When `agent_id` is present, the named User MUST be that Agent's Owner. The
Agent is then the acting principal for access, routing and delivery, and the
User is who it acts for; the User's inactivity refuses the token exactly as it
suspends the Agent. A token without `agent_id` acts as the User alone.

The stored pair MUST match current ownership. Reassigning an Agent MUST
reassign its tokens: the new Owner replaces the old one on every token of that
Agent, in the same committed update as the ownership change and never as a
later step. Failing to write the tokens MUST abandon the transfer rather than
commit an ownership change the tokens do not follow. The invariant therefore
holds by construction, and a mismatch can arise only from a failed write. A mismatched row is therefore corrupt state rather than an ordinary
refusal path: the daemon MUST ignore that token, which then authenticates
nothing, and MUST report the mismatch as a conceptual error at `alert`
severity through [both destinations](#errors-and-alerts), naming the token's
User, Agent and current Owner. It MUST NOT repair the row,
reinterpret it as a User token, or treat the condition as routine.

`last_used_at` follows the [statistics persistence schedule](10-modules.md#statistics-persistence).
The value means credential use, not necessarily a browser login.

### 📋 Registry record

A registry record MUST be owned by a 👤 User. An Agent MAY have management
authority but MUST NOT become the owning principal. The same User-only ownership
rule applies to Groups and daemon ownership. Creating or managing an object as
an Agent does not make that Agent its owner.

The current documentation's agent-owned records, ownership chains and self-owned
non-User records do not describe this intended model. Their presence in the docs
is not evidence that such objects exist on a running node.

Every direct edit to a User, registry record, or Group MUST write one daemon log
file entry with the actor, operation, target, result, and client IP when one
exists. The actor MUST be the authenticated User or Agent. A Unix socket request
has no client IP and MUST NOT invent one. A change to an entity's `status`
between `active` and `inactive` is such an edit and MUST be logged. Credential
operations, reads, sends, and consumes MUST NOT write entity-edit entries.

Logs MUST NEVER contain sensitive information. Current examples include tokens,
secret bodies, configuration bodies, and message bodies.

Every field MUST be validated and normalized according to its own contract.
Secrets and configuration MUST pass their required format validation before a
write is accepted. Format validation does not authorize generic “sanitization”
or interpretation of application-specific values; normalization must be stated
explicitly in the field's contract.

#### 🏷️ Record kind

The closed set of record kinds is:

- 👤 `user`: the queue belonging to a User; it shares that User's canonical
  name.
- 👾 `agent`: an Agent and the queue it reads.
- 📮 `queue`: a named competing-consumer queue.
- 📣 `pubsub`: a fan-out channel that retains no messages of its own and copies
  publications to its `deliver_to` actors.
- 📡 `service`: information about an external service, protected by an ACL.

A 👥 Group is a separate entity, not a sixth record kind.

#### Actors and ASCII textarea syntax

An **actor** is a 👤 User, 👾 Agent, or 👥 Group.

ACL, Maintainer, and group-member textareas accept one ASCII term per line. An
Agent term MUST carry the leading `#`, exactly as a Group term carries `@`, and
the stored term keeps the marker. A bare `alice@team` in such a list therefore
names a User and nothing else: without the marker nothing in `xx@yy` says
whether a User or an Agent was meant.

| Actor | Term |
|---|---|
| 👤 User | `alice@team` |
| 👾 Agent | `#worker@team` |
| 👥 Group | `@support` |
| Existing ACL wildcard | `*` |
| Owner and its directly owned Agents | `@owner` |
| The record's own Agent | `@agent` |

A term is stored exactly as it is written. The last three resolve at each check
instead of naming a stored entity, and only the first three may be created.

The existing [ACL rules](02-access.md#acl), including the empty-list default,
wildcard eligibility and `@owner`, remain in force unless explicitly revised.
Adding typed Agent terms does not widen `*` or make ACL-only terms valid as
Maintainers or Group members.

The leading `#` types the term. It is part of the stored actor term and MUST
NOT appear in the Agent's canonical name, token identity, URL, registry key, or
message route; the daemon strips it when resolving the term, after validating
that the target is an Agent.

Names remain globally unique. The marker makes the actor type explicit to the
reader, lets the daemon reject a term whose stored kind does not match, and
decides a term's kind from the term itself rather than from a registry lookup.
Lists that name an Agent without the marker are retyped by the
[cutover](../Plans/MVP/0.7-cutover.md#rewrite-and-activation); afterwards an
untyped Agent name in a list is refused like any other invalid term. Unicode glyphs are for display only and MUST NOT be required in editable
or machine-readable values.

Whitespace is trimmed, duplicate terms are rejected or normalized
deterministically, and one invalid term rejects the complete update.

#### Common record fields

- `registry_id`: stable internal ID; persisted, never reused, and not exposed as
  the public identity.
- `owner_id`: the owning User.
- `kind`: one value from the closed record-kind enum.
- `name`: canonical `name`, `name@team` or `template/instance@team`; globally
  unique and required. The realm is optional; the last `@` separates it.
- `description`.
- `maintainers`: typed actor terms.
- `allow`: typed actor terms; this is the ACL.
- `status`: `active` or `inactive`.
- `created_at` and `updated_at`, maintained by the system rather than editable
  by callers.

For records with a delivery switch, `active` means the former `disabled=false`,
and `inactive` means the former `disabled=true`. These are two spellings of one
control, not independent switches. A Service also carries status; its read
visibility and name reservation follow the [Service rule](#-service).

#### Authority rules

The Owner MAY transfer ownership and replace the Maintainers list.

A Maintainer MAY edit the description, ACL, status, and the operational fields
allowed for that record's kind. A Maintainer MUST NOT change:

- name or kind;
- owner;
- Maintainers;
- Personal classification;
- `created_at` or `updated_at`.

For an Agent record, the matching Agent principal has Maintainer-equivalent
authority over its own record. This is a direct rule and does not depend on a
stored group. `@agent` is an input alias for that same principal, accepted in
`allow` and `maintainers`; like `@owner` it is resolved at each check and never
created, stored or nested.

A modifying operation MUST be authorized against current state, including the
caller's right to change each submitted field. The complete candidate record
MUST then be validated. Proposed changes MUST NOT supply their own authority.
Checking and applying a multi-field update MUST be one atomic operation.

Ownership transfer is an ordinary authorized record change: the old Owner
changes the owner, the daemon commits that record update and publishes the new
complete view, and subsequent authority checks use the new Owner. The update
does not walk or rewrite unrelated records; runtime terms such as `@owner`
resolve against the new published owner. Restart loads that same saved
ownership; it does not authorize the transfer again or restore the old Owner.

## Kind-specific fields

### 📢 Channels

The channel kinds are `user`, `agent`, `queue`, and `pubsub`.

- `personal`: allowed only on an Agent. It states that the intended audience is
  the Owner and the Agents that Owner owns, and its ACL and Maintainer rules
  admit that cohort and nothing wider.
- `ttl`, `bound`, and `overflow`: allowed on User, Agent, and Queue; invalid on
  PubSub.
- `deliver_to`: one kind-dependent list. On PubSub it contains typed actor
  terms. On User, Agent, or Queue it contains zero or one destination channel
  for forwarding. PubSub refuses destination-channel entries; the other three
  kinds refuse actor entries and a second destination with an explicit caller
  error. A whole-field write containing two or more destinations stores nothing.

What makes an entry one sort or the other is the record a term resolves to,
not a separate spelling, so the daemon MUST resolve it: the term has to name an
existing registry record, and that record's stored kind decides the answer.

A forwarding destination MUST be a channel — `user`, `agent`, `queue` or
`pubsub`, the kinds that have an inbox. A 📡 Service is not one, and neither is
a 👥 Group: a group is an actor, not somewhere a message lands. A term naming no
record is refused as well, since the destination has to exist for its ACL to be
checked against the forwarding record.

On a PubSub, `deliver_to` says who receives a copy, and each copy lands in the
recipient's own inbox, so that list holds actors: 👤 User, 👾 Agent and 👥 Group
terms. A 📮 Queue or 📣 PubSub term is refused there, those two kinds being
destinations rather than readers, so a topic cannot fan out into a queue or
chain into another topic. 👤 and 👾 are both actors and channels, so the same
term may be valid in either role; what each kind refuses is the terms outside
its own role, never a different spelling.

An entry is refused for its kind, not for permission — the caller's right to
use that record elsewhere makes it neither a recipient nor a destination — and
the refusal names the offending term and stores nothing, because one invalid
term rejects the complete update. A Group term on a PubSub is expanded at
publication, and Group membership is itself restricted to actors, so no Queue
or PubSub reaches that list through a group.

For the one-slot form, add succeeds only while empty and returns an error naming
the occupied field otherwise; remove clears it. Replacing an existing
destination is an explicit whole-field write, never an add that silently
overwrites a concurrent choice. One invalid entry rejects the complete update.

When a channel is inactive:

- reads and writes are refused with an explicit error;
- PubSub copies addressed to that inactive channel are discarded and counted as
  drops.

**Forwarding — owner-corrected September 20, 2026.** A User, Agent or Queue
record MAY forward to another channel. For `sender → A → B`, the sender MUST
pass A's ACL, and B's ACL MUST list the forwarding record A. The sender needs
no access to B. Neither the original sender's nor A's Owner's access to B
substitutes for B allowing A.

The destination MUST NOT be stored unless its ACL allows the forwarding record.
The daemon MUST check that permission again at delivery. For a Queue source,
this is a channel-name ACL reference, not a credential-bearing principal.
Forwarding authority comes from the daemon's selected source record, never from
caller-supplied sender or provenance fields.

The destination applies its active state, TTL and deadline, bound, overflow
policy and counter rules as for a direct send. Any current-rule refusal rejects
the original send with a stated error before anything is stored or counted.
Removing A from B's ACL leaves `deliver_to` configured but denies forwarding;
restoring that permission resumes forwarding without editing the field. Human
faces MUST distinguish a configured route from one currently allowed by the
destination's ACL.

A forwarded envelope retains the original sender and MUST carry one
`original_to` value naming the one prior destination through which it was
forwarded. Forwarding depth is one, so this is never a chain. Forwarding moves
the message: the source inbox keeps no copy and changes neither `in` nor `out`;
the destination increments `in`, then `out` only when a reader receives it.

The destination's overflow policy is unchanged: strict overflow refuses the
original send and changes no counter; ring overflow evicts its oldest message
and increments the destination's own `dropped`. The forwarding record records
neither case because the moved message was never accepted into its inbox.

Forwarding has a maximum depth of one. If the selected destination itself has a
forwarding destination, the original send is refused with a stated error before
anything is stored. No second message exists to be dropped. Forwarding adds no
TTL or deadline policy: the destination queue applies the ordinary queue and
message rules.

### 📡 Service

A Service requires:

- `addr`;
- `protocol`;
- optional `secret`, writable by the Owner or a Maintainer and readable by
  actors in `allow`.

**Secret format — owner-settled September 19, 2026.** A supplied secret MUST be
an env file containing environment-variable assignments, and the daemon MUST
validate basic env-file syntax before accepting it. Invalid syntax MUST reject
the complete write. The user is responsible for the remaining content and its
suitability for the consuming application. Arbitrary unvalidated bytes are no
longer accepted by this model; this replaces the earlier rule that `KEY=value`
was only a caller convention.

A Service has no queue, so it carries no TTL, bound, overflow policy,
`deliver_to`, Personal classification, or channel delivery switch.

**Status rule — owner-settled September 19, 2026.** An inactive Service MUST be
hidden from reads, including discovery, record lookup and secret reads. Its
record remains stored and reserves its canonical name. A new registration using
that name MUST be rejected because the name already exists; inactivity does not
unregister the Service or make its name available for reuse.

### 👾 Agent configuration

An Agent MAY carry `config`:

- writable by the Owner or a Maintainer;
- readable only by the matching Agent principal;
- represented to everyone else by its SHA-256 digest;
- JSON; the daemon MUST validate JSON syntax before accepting it. Invalid input
  MUST reject the complete write. Application-specific fields remain opaque.

Validated JSON MUST be compacted before storage and SHA-256 hashing, retaining
the existing [configuration normalization](03-records.md#why-a-digest-at-all).

### 👥 Group

A Group is a named list of typed actors, owned by a User.

- `group_id`: stable internal ID.
- `name`: `@group_name`.
- `owner_id`: the owning User.
- `maintainers`.
- `members`: typed User, Agent, or Group terms.
- `created_at` and `updated_at`.

The Group Owner controls its Maintainers. Group Maintainers MAY add or remove
members. An Agent MAY be a Group Maintainer by using its `#agent@team` actor
term.

This extends the existing [group administration](01-identity-and-roles.md#groups)
model: Users and Agents gain explicitly assigned control of a Group, while
daemon Owner and Administrator authority remains. An Agent may maintain a Group
but cannot own it.

Nested-group resolution MUST use a visited set. Cycles must terminate and grant
membership only when a finite path reaches the requested actor.

The protected `@administrators` group is outside the ordinary Group authority
model. It has neither a Group Owner nor Maintainers, and only the daemon Owner
may change its direct membership. Ordinary Group ownership or Maintainer
assignment MUST NOT bypass that boundary.

## Open questions

None. The plan's [question index](../Plans/MVP/QUESTIONS.md#open-questions) owns
any choice raised later; forwarding permissions and mechanics are settled above.
