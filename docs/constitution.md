# Project Constitution

**Status:** In discussion  
**Date:** September 19, 2026

This document describes the intended model. It is not a statement that every
rule below is implemented already. Open choices are named explicitly.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

## Clarified direction

Owner clarification, September 19, 2026. Each item is now owned by the section
beside it.

| Clarification | Owned by |
|---|---|
| Objects belong to Users; an Agent's management authority never makes it an owner | [registry record](#-registry-record) |
| Group ownership and Maintainers extend daemon administration rather than replacing it | [group](#-group) |
| Persistence follows a write-through cache model | [persistence](#persistence-and-loading) |
| `banned` is gone: a User is `active` or `inactive`, and record `active` replaces Disabled with the opposite polarity | [user](#-user), [common fields](#common-record-fields) |
| ACL syntax is extended, not replaced; the Agent marker is additive | [actor terms](#actors-and-ascii-textarea-syntax) |

The current topic docs still need reconciliation. Their agent-owned records,
ownership chains and self-owned non-User records describe neither this model nor
evidence that such objects exist on a running node.

## Persistence and loading

0.7 uses SQLite. Configurable [additional backends](../Plans/R1/storage.md#backends)
are R1 work. Every backend MUST preserve the same identity, authority and
durability rules, behind the existing storage ports.

| Rule | Requirement |
|---|---|
| Durable | entities, credentials, queue contents, per-queue counters |
| A management write | validates, commits the complete change as one transaction, then publishes |
| Publication | one complete new view, never a mutation of the live maps in place |
| Atomicity | invalid input fails the whole write; no partial update is ever visible |
| Startup | load every durable entity, then build derived indexes such as token to principal and user ID to status |
| List fields | the API MUST provide atomic add and remove for `allow`, `maintainers`, `members` and `deliver_to` |

This ordering is write-through: memory serves the loaded view, and a management
write reaches durable state before the changed view is published. It applies to
record lifecycle changes too, although 0.7 adds no exhaustive deletion-specific
crash matrix. It replaces the memory-before-persistence behavior of the 0.6
[persistence-failure contract](04-messaging.md#administrative-crash-recovery).

Queue contents and their `in`, `out`, `dropped` and `expired` counters keep the
existing [checkpoint boundary](04-messaging.md#durability): they update in
memory during traffic and flush as one consistent batch every minute and on
graceful shutdown, never once per message. A crash MAY lose queue changes since
the last successful flush. Startup MUST reject a durable queue whose record is
absent or cannot hold a queue, and MUST NOT silently drop or reattach that
backlog. The JSON dump is cutover input, not a second runtime store after 0.7
activation.

Identity changes MUST invalidate stale credentials and grants in durable state
and memory together:

| Change | Requirement |
|---|---|
| Ownership transfer | update and commit the owned record, publish the new view, rewrite no unrelated record |
| Removing an Agent or record | drop every stored ACL, Maintainer, Group-member and Deliver-To reference to that name in the same transaction |
| Reusing a name | inherit no authority and no delivery from the former holder |
| Users | never removed |

One active daemon MAY use a database. The daemon MUST establish exclusive
ownership before serving and reject a competing instance.

Two conditions leave the daemon unable to answer with authority: losing that
exclusivity, and failing to publish a committed change. In either the daemon
MUST refuse every read and write with a stated reason and MUST NOT answer from
its cached view. These observable rules do not prescribe whether the process
keeps running or exits.

A future reload API or SIGHUP (`kill -HUP <pid>`) MAY reload durable entities.
It is required for no 0.7 operation, ownership transfer included. If added, it
MUST replace one complete in-memory view with another, rebuild the derived
indexes, and never let a caller observe a partly reloaded state.

## Errors and alerts

A conceptual error is a condition this model says cannot occur: a violated
invariant, corrupt stored state, or a failure that leaves the daemon unable to
answer with authority. An ordinary refusal is not one — a malformed request, a
denied permission or an unknown name is answered to its caller and reported
nowhere else.

Every conceptual error and every alert MUST be reported twice:

| Copy | Destination |
|---|---|
| the message | syslog, at a severity matching the condition |
| the same message again | the daemon's error log |

No log may contain a token, secret, configuration body or message body.
[Entity-edit logging](#-registry-record) is separate: it records authorized
edits, not impossible states.

## Entities

### 👤 User

| Field | Requirement |
|---|---|
| `user_id` | stable internal `uint32`; persisted, never reused, never the public identity |
| `name` | canonical `user` or `user@team`; globally unique and required. The realm is optional and part of the identity, so `alice` and `alice@team` are different Users |
| unique secondary identifiers | normalized email, GitHub login, Twitter/X username compared case-insensitively; each unique when present |
| profile | person name, company, location, photo or Gravatar; not identifiers and need not be unique |
| Ed25519 public key | when key-based enrolment is used |
| `created_at`, `updated_at`, `last_used_at` | |
| `status` | `active` or `inactive`; this model has no `banned` state |

`inactive` replaces the former paused/banned distinction under the owning
[User-state contract](01-identity-and-roles.md#user-states): an Administrator
may reactivate an ordinary User but cannot change another Administrator, only
the daemon Owner may reactivate an inactive Administrator, and no second
suspension level remains.

### 😈 Daemon

The daemon is configuration, not a registry entity.

| Field | Requirement |
|---|---|
| `owner_id` | the daemon Owner |
| description | |
| listen addresses | TCP and Unix-socket addresses where applicable |

Version, build information, hostname, uptime and call counters are runtime facts
rather than durable daemon fields.

### 🔐 Token

| Field | Requirement |
|---|---|
| `token` | |
| `user_id` | stable `user_id`; always present |
| `agent_id` | stable `registry_id` of an Agent, or absent |
| `created_at`, `updated_at`, `last_used_at` | `last_used_at` follows the [statistics persistence schedule](10-modules.md#statistics-persistence) and means credential use, not necessarily a browser login |

- Without `agent_id` the token acts as the User alone.
- With `agent_id` the named User MUST be that Agent's Owner. The Agent is then
  the acting principal for access, routing and delivery, and the User is who it
  acts for; the User's inactivity refuses the token exactly as it suspends the
  Agent.
- Reassigning an Agent MUST reassign its tokens in the same committed update as
  the ownership change, never as a later step. A failed token write MUST abandon
  the transfer rather than commit an ownership change the tokens do not follow.

A mismatched pair can therefore arise only from a failed write. Such a row is
corrupt state rather than a refusal path: the daemon MUST ignore that token, which authenticates nothing,
and MUST report the mismatch as a conceptual error at `alert` severity through
[both destinations](#errors-and-alerts), naming the token's User, Agent and
current Owner. It MUST NOT repair the row, reinterpret it as a User token, or
treat the condition as routine.

### 📋 Registry record

A registry record MUST be owned by a 👤 User, and so MUST the daemon. An Agent
MAY have management authority but MUST NOT become an owning principal; creating or managing an object as an Agent does not make that Agent
its owner.

Every direct edit to a User, registry record or Group MUST write one daemon log
file entry:

| Entry | Requirement |
|---|---|
| actor | the authenticated User or Agent |
| operation, target, result | always present |
| client IP | when one exists; a Unix socket request has none and MUST NOT invent one |
| a `status` change between `active` and `inactive` | is such an edit and MUST be logged |
| credential operations, reads, sends, consumes | MUST NOT write entity-edit entries |

Every field MUST be validated and normalized according to its own contract, and
secrets and configuration MUST pass their format validation before a write is
accepted. Format validation authorizes no generic "sanitization" or
interpretation of application-specific values; normalization must be stated
explicitly in the field's contract.

#### 🏷️ Record kind

The closed set of record kinds:

| Kind | What it is | Channel |
|---|---|---|
| 👤 `user` | the queue belonging to a User; it shares that User's canonical name | yes |
| 👾 `agent` | an Agent and the queue it reads | yes |
| 📮 `queue` | a named competing-consumer queue | yes |
| 📣 `pubsub` | a fan-out channel that retains no messages of its own and copies publications to its `deliver_to` recipients | yes |
| 📡 `service` | information about an external service, protected by an ACL | no |
| 👥 `group` | a named list of typed actors | no |

#### Actors and ASCII textarea syntax

An **actor** is a 👤 User, 👾 Agent, or 👥 Group. ACL, Maintainer and
group-member textareas accept one ASCII term per line:

| Actor | Term |
|---|---|
| 👤 User | `alice@team` |
| 👾 Agent | `#worker@team` |
| 👥 Group | `@support` |
| Existing ACL wildcard | `*` |
| Owner and its directly owned Agents | `@owner` |
| The record's own Agent | `@agent` |

- A term is stored exactly as written. An Agent term MUST carry the leading `#`,
  exactly as a Group term carries `@`, so a bare `alice@team` names a User and
  nothing else.
- The marker decides a term's kind without a registry lookup, and lets the
  daemon reject a term whose stored kind does not match.
- The last three terms resolve at each check instead of naming a stored entity.
  Only the first three may be created, and none of the last three may be stored
  as an entity or nested in a group.
- The `#` MUST NOT appear in an Agent's canonical name, token identity, URL,
  registry key or message route. The daemon strips it when resolving the term,
  after validating that the target is an Agent.
- Lists naming an Agent without the marker are retyped by the
  [cutover](../Plans/MVP/0.7-cutover.md#rewrite-and-activation); afterwards an
  untyped Agent name in a list is refused like any other invalid term.
- Whitespace is trimmed, duplicate terms are rejected or normalized
  deterministically, and one invalid term rejects the complete update.
- Unicode glyphs are for display only and MUST NOT be required in editable or
  machine-readable values.

The existing [ACL rules](02-access.md#acl), including the empty-list default,
wildcard eligibility and `@owner`, remain in force unless explicitly revised.
Typed Agent terms do not widen `*` or make ACL-only terms valid as Maintainers
or Group members.

#### Common record fields

| Field | Requirement |
|---|---|
| `registry_id` | stable internal ID; persisted, never reused, never the public identity |
| `owner_id` | the owning User |
| `kind` | one value from the closed record-kind enum |
| `name` | canonical `name`, `name@team` or `template/instance@team`; globally unique and required. The realm is optional; the last `@` separates it |
| `description` | |
| `maintainers` | typed actor terms |
| `allow` | typed actor terms; this is the ACL |
| `status` | `active` or `inactive` |
| `created_at`, `updated_at` | maintained by the system, not editable by callers |

For records with a delivery switch, `active` means the former `disabled=false`
and `inactive` the former `disabled=true` — two spellings of one control, not
independent switches.

**A record whose `status` is not `active` MUST be treated as no such entity**,
whatever its kind:

| | |
|---|---|
| reads, discovery, lookup, a Service's secret | show nothing |
| every operation naming it | refused |
| what it grants | nothing, so no membership path through it reaches an actor |
| waiting readers that lose authority | released |
| the record itself | stays stored, keeps its canonical name reserved, and MUST refuse registration under that name |
| reactivation | an authorized status edit on that record, never a re-creation |

An absent record and an inactive one are the same case: it accepts no read and
no write, every such operation is rejected, and nothing is ever stored for it.
Who learns of the rejection depends on who named the record:

| Delivery | Outcome |
|---|---|
| direct: the caller names the record itself | an error to that caller, before anything is stored or counted |
| indirect: a 📣 copy or a forwarded message | discarded, counted as the missing record's own `dropped`, and written to the daemon log |

One inactive subscriber therefore MUST NOT stop a topic: the publication
succeeds for every other recipient.

#### Authority rules

| Who | MAY | MUST NOT change |
|---|---|---|
| Owner | transfer ownership, replace the Maintainers list, and everything a Maintainer may | |
| Maintainer | edit the description, ACL, status and the operational fields allowed for that kind | name, kind, owner, Maintainers, Personal classification, `created_at`, `updated_at` |
| The matching Agent principal, on its own record | what a Maintainer may | what a Maintainer may not |

The Agent's authority over its own record is a direct rule that depends on no
stored group; `@agent` is an input alias for that same principal.

A modifying operation MUST be authorized against current state, including the
caller's right to change each submitted field, and the complete candidate record
MUST then be validated. Proposed changes MUST NOT supply their own authority.
Checking and applying a multi-field update MUST be one atomic operation.

Ownership transfer is an ordinary authorized record change: the old Owner
changes the owner, the daemon commits that record update and publishes the new
complete view, and subsequent checks use the new Owner. Runtime terms such as
`@owner` resolve against it. Restart loads that same saved ownership; it does
not authorize the transfer again or restore the old Owner.

## Kind-specific fields

### 📢 Channels

The channel kinds are 👤 `user`, 👾 `agent`, 📮 `queue` and 📣 `pubsub`.

| Field | Where | Meaning |
|---|---|---|
| `personal` | 👾 only | the intended audience is the Owner and the Agents that Owner owns; its ACL and Maintainer rules admit that cohort and nothing wider |
| `ttl`, `bound`, `overflow` | 👤, 👾, 📮 | invalid on 📣 |
| `deliver_to` | 📣, 👾, 📮 | on 📣 the recipient list; on 👾 or 📮 zero or one forwarding destination. A 👤 User has no such field |

What an entry means is decided by the record its term resolves to, so the daemon
MUST resolve it against the registry:

| Field | Accepts |
|---|---|
| `deliver_to` on 📣 | 👾, 👥, 📮, 📣 |
| `deliver_to` on 👾 / 📮, one slot | 👾, 📮, 📣 |

- Every other term is refused, an unresolvable one included. An entry is refused
  for its kind rather than for permission, and a refusal names the term and
  stores nothing.
- A 👥 term is expanded at publication, and its membership is actors only.
- A 👤 User receives only replies to what it sent: no ordinary send, no
  published copy, no route. Any other delivery attempt to a User MUST be
  answered with an error, never discarded and never counted as a drop.
- In the one-slot form, add succeeds only while empty and otherwise returns an
  error naming the occupied field; remove clears it. Replacement is an explicit
  whole-field write, never an add that silently overwrites a concurrent choice.
  A whole-field write carrying two or more destinations stores nothing.

**Forwarding — owner-corrected September 20, 2026.** A 👾 Agent or 📮 Queue
record MAY forward to another destination. For `sender → A → B`:

| Check | Requirement |
|---|---|
| the sender | MUST pass A's ACL |
| B's ACL | MUST list the forwarding record A; the sender needs no access to B |
| a substitute | neither the original sender's nor A's Owner's access to B stands in for B allowing A |
| when | the destination MUST NOT be stored unless its ACL allows the forwarding record, and the daemon MUST check that permission again at delivery |
| whose right | the daemon's selected source record, never a caller-supplied sender or provenance field |

For a 📮 source this is a channel-name ACL reference, not a credential-bearing
principal. Removing A from B's ACL leaves `deliver_to` configured but denies
forwarding, and restoring the permission resumes it without editing the field.
Human faces MUST distinguish a configured route from one the destination's ACL
currently allows.

A forwarded envelope retains the original sender and MUST carry one
`original_to` value naming the prior destination it came through, and a forward
counter. Every forwarding step increments that counter, a topic's fan-out into
another channel included, and a step that would raise it above ten MUST be an
error. The counter is what ends a cycle: two topics listing each other, or a
queue routing back into the topic that fed it, stop with a stated error rather
than looping.

The destination applies its TTL and deadline, bound, overflow policy and
counter rules as for a direct send; forwarding adds no policy of its own. A
refusal under those rules rejects the original send with a stated error before
anything is stored or counted. An inactive or absent destination is the
exception: the sender named A, not B, so the message is dropped and counted on
B under the [indirect-delivery rule](#common-record-fields).

| Outcome | Counters |
|---|---|
| forwarded | the source keeps no copy and changes neither `in` nor `out`; the destination increments `in`, then `out` when a reader receives it |
| strict overflow at the destination | the original send is refused and no counter changes |
| ring overflow at the destination | its oldest message is evicted and its own `dropped` increments |

The forwarding record counts neither overflow case.

### 📡 Service

| Field | Requirement |
|---|---|
| `addr`, `protocol` | required |
| `secret` | optional; writable by the Owner or a Maintainer, readable by actors in `allow` |

**Secret format — owner-settled September 19, 2026.** A supplied secret MUST be
an env file of environment-variable assignments, and the daemon MUST validate
basic env-file syntax before accepting it; invalid syntax MUST reject the
complete write. The user is responsible for the remaining content and its
suitability for the consuming application. Arbitrary unvalidated bytes are no
longer accepted, which replaces the earlier rule that `KEY=value` was only a
caller convention.

A Service has no queue, so it carries no TTL, bound, overflow policy,
`deliver_to`, Personal classification or channel delivery switch.

**Status rule — owner-settled September 19, 2026.** See
[what `inactive` means](#common-record-fields) for the Service's read visibility
and name reservation.

### 👾 Agent configuration

An Agent MAY carry `config`:

| Rule | Requirement |
|---|---|
| writable by | the Owner or a Maintainer |
| readable by | the matching Agent principal only; everyone else sees its SHA-256 digest |
| format | JSON, whose syntax the daemon MUST validate before accepting it; invalid input MUST reject the complete write |
| application fields | remain opaque |
| storage | validated JSON MUST be compacted before storage and hashing, retaining the existing [configuration normalization](03-records.md#why-a-digest-at-all) |

### 👥 Group

A Group is a named list of typed actors, owned by a User.

| Field | Requirement |
|---|---|
| `registry_id` | the record's stable ID, from the one space every record shares; there is no separate group ID |
| `kind` | `group`, from the same closed enum as every other record |
| `name` | `@group_name` |
| `status` | `active` or `inactive`; see [what `inactive` means](#common-record-fields) |
| `owner_id` | the owning User |
| `maintainers` | controlled by the Group Owner |
| `members` | typed User, Agent or Group terms; Group Maintainers MAY add and remove them |
| `allow` | not used: a Group carries no ACL, and its Owner and Maintainers are its only control |
| `created_at`, `updated_at` | |

An Agent MAY be a Group Maintainer through its `#agent@team` term. This extends the existing
[group administration](01-identity-and-roles.md#groups) model: Users and Agents
gain explicitly assigned control while daemon Owner and Administrator authority
remains.

Nested-group resolution MUST use a visited set, and grants membership only
when a finite path reaches the requested actor.

The protected `@administrators` group is outside this model: it has neither a
Group Owner nor Maintainers, only the daemon Owner may change its direct
membership, and ordinary Group ownership or Maintainer assignment MUST NOT
bypass that boundary.

## Open questions

None. The plan's [question index](../Plans/MVP/QUESTIONS.md#open-questions) owns
any choice raised later; forwarding permissions and mechanics are settled above.
