# Project constitution

📌 **TL;DR:** Users own resources; the daemon enforces identity, access and durable changes.

## Status

Intended model, incorporating owner decisions through September 19, 2026.
These rules govern the target design; they are not a claim of implementation.
The [topic docs](00-overview.md#document-ownership) retain built/pending status
and must be reconciled where this model changes them. No release is assigned
to the additions here. MUST, MUST NOT, SHOULD and MAY are normative.

## Identity and entities

Users and Agents authenticate and act. Groups collect actors and hold no
credential. Every registry record, Group and daemon belongs to a User;
management by an Agent does not give it ownership.

Public names are globally unique canonical names, using the existing
[name grammar](01-identity-and-roles.md#names). Internal IDs are persisted;
User and record IDs are never reused or exposed as public identities.

| Entity | Stored information |
|---|---|
| User | `user_id` (`uint32`), canonical name, active/inactive status, profile, Ed25519 enrolment public key when used, creation/update/last-use times |
| Record | `registry_id`, User `owner_id`, kind, canonical name, description, Maintainers, ACL, active/inactive status, system-managed creation/update times |
| Group | Stable `group_id`, `@name`, User `owner_id`, Maintainers, members, creation/update times |
| Token | Credential, principal type (`user` or `agent`), principal ID, creation/update/last-use times |
| Daemon configuration | User `owner_id`, description, listen addresses; version, build, hostname, uptime and counters are runtime facts |

User email, GitHub login and Twitter/X username are unique when supplied and
compared after normalization; Twitter/X comparison is case-insensitive. Person
name, company, location and photo/Gravatar are profile data, not identifiers.

## Record kinds

The closed set contains five kinds. A Group is separate.

| Kind | Meaning | Kind-specific data |
|---|---|---|
| `user` | User's inbox, sharing that User's name | TTL, bound, overflow; optional forwarding |
| `agent` | Agent and its inbox | TTL, bound, overflow, Personal, configuration; optional forwarding |
| `queue` | Named competing-consumer queue | TTL, bound, overflow; optional forwarding |
| `pubsub` | Fan-out channel, retaining no messages itself | Deliver-To actors |
| `service` | ACL-protected external-service information | Required address and protocol; optional secret |

Here, channels include User and Agent inboxes, Queue and PubSub. A **topic**
remains a message correlation label. A Service has no queue, reader, TTL, bound,
overflow policy, forwarding, Personal flag or delivery switch.

## Authority and ACL

The daemon Owner manages the node. Administrators manage ordinary Users and
Groups below their authority. Node administration does not itself grant message
use; [resource access](02-access.md#acl) still applies.

Resource Owners may transfer ownership and assign Maintainers. Maintainers may
edit description, ACL, status and permitted operational fields, but not name,
kind, Owner, Maintainers, Personal or system timestamps. An Agent has
Maintainer-equivalent authority over its own record.

Authorize the caller and changed fields against **current state**, then validate
the complete candidate. Check and apply the update atomically. Proposed changes
MUST NOT authorize themselves. Ownership transfer takes effect after persistence
and publication in memory; restart loads the saved Owner without reauthorizing
the transfer.

An empty ACL admits the resource Owner and assigned Maintainers; a User or Agent
may read its own inbox without listing itself. Existing ACL syntax remains valid.
Editors accept one ASCII term per line:

| Term | Meaning |
|---|---|
| `alice@team` | User, or existing plain-name Agent reference |
| `#worker@team` | Explicit Agent reference; validate its kind, then strip `#` |
| `@support` | Group |
| `*` | Existing registered-user wildcard, where permitted |
| `@owner` | Direct Owner and that User's directly owned Agents; ACL only |

`#` never enters canonical names, routes, URLs or token identities. Trim
whitespace; handle duplicates deterministically. One invalid term rejects the
whole update. ACL-only terms do not become Group members or Maintainers.

Group Owners control Maintainers; Group Maintainers, including Agents, control
membership. Existing daemon administration remains. Nested membership uses a
visited set and grants access only along a finite path to the actor.
`@administrators` retains direct User membership and daemon-Owner-only control;
ordinary Group grants cannot bypass it.

Personal applies only to Agents and is Owner-controlled. It permits only other
Agents in its ACL, with no Groups, wildcard, `@owner` or assigned Maintainers.
It changes web classification, not the ordinary resource access rules.

## State and credentials

Users have `active` or `inactive` status; there is no `banned` state. Inactivity
suspends access and owned records, retaining credentials and queued work.
Record inactivity replaces the old Disabled switch: inactive channels refuse
reads and writes, retain their queues and count discarded PubSub copies as drops.

Inactive Services are hidden from reads, including discovery, lookup and secret
reads. Their records remain stored and reserve their names. New registration
under an occupied name MUST fail, including when its Service is inactive.

Tokens authenticate exactly one known User or Agent. They follow the existing
[token lifetime](02-access.md#token-lifetime) and rotation rules. Token last-use
time records credential use, not browser login, and MAY be persisted at most
once per ten minutes. Suspension cannot be bypassed through browser sessions
or account sockets.

## Private values

| Value | Validation and storage | Write | Read |
|---|---|---|---|
| Service secret | Basic env-file syntax; remaining content is the user's responsibility | Owner or Maintainer | ACL-admitted callers while active |
| Agent configuration | Valid JSON, compacted before storage and SHA-256 hashing; application fields are opaque | Owner or Maintainer | Matching Agent only, including exclusion of its Owner |

Invalid format rejects the complete write. Other record responses carry digests,
never secret or configuration bodies. The daemon is trusted with plaintext;
these permissions do not provide encryption from the host.

## Persistence and audit

Load all durable entities at startup and build derived lookup indexes. Management
uses write-through ordering: authorize, validate, persist the complete change,
then publish it in memory. Invalid input leaves no partial change. The API MUST
provide atomic add/remove operations for ACL, Maintainers, members and Deliver-To
lists. Serialize policy checks and writes so concurrent changes cannot invalidate
the authority used by an operation.

API/SIGHUP reload may be added. It replaces the complete view and rebuilds
indexes; queues and waiting readers follow existing restart behavior and the
[snapshot durability boundary](04-messaging.md#durability).

Audit every modification with actor, operation, target, result, request ID and
origin address when present. Never log credentials, private values or message
bodies. Unix-socket requests have no client IP.

## Messaging and runtime

The daemon routes messages and enforces access. CLI, MCP and web faces use the
same authority checks; the web acts as its visitor. Core owns policy, adapters
provide I/O, and runners execute user programs outside the daemon.

Consumption is at most once. Replies and receipts are ordinary messages; the
daemon keeps no exchange state. TTL limits retention; a caller's deadline limits
its wait. Offline consumers leave bounded queued work. A crash may lose traffic
since the last snapshot; administrative writes use the persistence rule above.

For PubSub, ACL controls publishing and Deliver-To controls recipients. Managers
set that list; a recipient may remove itself. Expand Groups and deduplicate at
publication, retaining copies only in eligible User and Agent inboxes.

User, Agent and Queue records may forward to another channel. The User or Agent
MUST have access to the destination. Remaining
[forwarding choices](../Plans/Future/QUESTIONS.md#constitution-forwarding)
must be settled before implementation.
