# Glossary

📌 **TL;DR:** Names and terms, linked to their definitions: the programs, the
current vocabulary, the glyphs in use, and the words reserved for later
stages.

## Names

Current naming index. Canonical values and definitions remain in the linked sections.

| Name | Meaning | Definition |
|---|---|---|
| agent-busd | Daemon | [programs](09-setup.md#the-programs) |
| agent-bus | User CLI | [programs](09-setup.md#the-programs) |
| agent-bus-setup | Installer | [installation](09-setup.md#install) |
| agent-bus-token | Credential program | [programs](09-setup.md#the-programs) |
| agent-bus-admin | Administration program | [SSH administration](09-setup.md#ssh-admin) |
| agent-bus-web | The web face: TypeScript on bun, its own account and systemd unit | [the web face](11-processes.md#the-web-face) |
| ab_ | MCP tool prefix only; never CLI or prose shorthand | [faces](05-discovery.md#faces) |

## Vocabulary

Use service discovery for the overall capability, registration for one
operation. Use realm for the name's authority namespace, and **agent template**
for the unconfigured capability a configured name was made from — the CLI verb
is `agent-template`, renamed from `service-template` in 0.6.6.

**Channel is the record; topic is the message label.** From 0.6.7 they are
two words for two things ([channel, not topic](07-channels.md#channel-not-topic)).
A channel is published to and subscribed; a topic rides on one envelope and is
what a filtered read matches. `channel` is the CLI verb, the `publish` flag and
the wire field; `topic` stays on `send`, `consume` and `reply`.

**Deliver-To is who receives a 📣 copy; the ACL is who may publish.** From
0.6.15 a pub/sub topic carries both ([subscribers](04-messaging.md#subscribers)).
Say Deliver-To for the list and recipient for a name on it; *subscriber* stays
readable prose and `subs` stays the wire field. Subscribing named the old
self-service join, which the manager's list replaces.

**Service means the external case and nothing else.** From 0.6.3 it is one of
the [record kinds](03-records.md#record-kinds): a card
describing something this bus does not run. Anything running behind a bus name
is an **agent**. Say record for a registered name whose kind does not matter to
the sentence. Personal is an owner-selected
[classification](03-records.md#personal-and-shared) of any kind, not a kind.
`generic`, `topic` as a **kind**, and delivery *mode* named kinds before 0.6.3
and name nothing now.

**The name says the kind.** From 0.7 an agent's name begins with `#`
(`#worker@srv1`), a group's with `@`, and any other name is a user, a channel
or a service ([names](01-identity-and-roles.md#names)). The realm is optional:
`parf` and `parf@srv1` are two names. **Inactive** is the one status besides
active, and an inactive entity is no such entity
([record status](constitution.md#common-record-fields)); *paused*, *banned*
and *disabled* name nothing now.

## Glyphs

One glyph per meaning, and this table is where a meaning is fixed. Most rows
name an entity type; `🔱` and `👮` name an authority, which is allowed because
neither can appear often enough to become a column heading.
[Web glyphs](../Plans/R0.8-MVP/web-handoff/glyphs.md#the-rule-that-matters-most) owns where a
glyph may appear, how it renders and what it must never carry on its own.

| Glyph | Code point | Name | Means | Status |
|---|---|---|---|---|
| 👤 | `U+1F464` | User | one registered person | built |
| 👥 | `U+1F465` | Group | a group or team | built |
| 👾 | `U+1F47E` | Agent | an agent, and the queue named after it | built in 0.6.1 |
| 🪪 | `U+1FAAA` | Identity | an identity as such, no entity type asserted | built |
| 🔑 | `U+1F511` | Credentials | credentials proving an identity; never the secret value | built |
| 🔱 | `U+1F531` | Daemon owner | the one authority on a node that cannot be delegated | built in 0.6.2 |
| 👮 | `U+1F46E` | Maintainers | the named list that may edit a record's settings and ACL | built in 0.6.2 |
| ⚙️ | `U+2699 U+FE0F` | Daemon | the daemon itself, never a record | reserved; labels nothing yet |
| 📡 | `U+1F4E1` | Service | something external, not on this bus | built in 0.6.3 |
| 📮 | `U+1F4EE` | Queue | a registered queue | built in 0.6.3 |
| 📣 | `U+1F4E3` | PubSub | a pub/sub channel | built in 0.6.3 |

`📡`, `📮` and `📣` arrived with the
[record kinds](03-records.md#record-kinds) in 0.6.3, where
`📡` took Service from `⚙️`. `⚙️` labels no record now and is held for the
daemon; nothing carries it yet.
History: `📥 Inbox` labelled an agent's record in 0.5.84 and was replaced by
`👾` in 0.6.1.

## Terms

| Term | Meaning | Definition |
|---|---|---|
| Principal | Credential-authenticated identity | [definition](01-identity-and-roles.md#identities) |
| Canonical name | Routing identity | [definition](01-identity-and-roles.md#names) |
| Token | Principal credential | [definition](02-access.md#what-a-call-carries) |
| Local socket | Account credential | [definition](02-access.md#local-socket) |
| Record ACL | Visibility and use policy, including runtime `@owner` | [definition](02-access.md#acl) |
| Owner | Highest authority within the named scope | [definition](01-identity-and-roles.md#role-names-and-scopes) |
| Daemon Owner | Root-like authority over the node; assigned through setup | [definition](01-identity-and-roles.md#daemon-owner) |
| Administrator | Manages daemon users and groups | [definition](01-identity-and-roles.md#daemon-administrators) |
| Maintainer | Explicitly assigned to manage a record | [definition](01-identity-and-roles.md#record-authority) |
| User | Registered person | [definition](01-identity-and-roles.md#users-and-profiles) |
| Member | Basic access to a record | [definition](01-identity-and-roles.md#role-names-and-scopes) |
| Person profile | Identifying and descriptive information | [definition](01-identity-and-roles.md#users-and-profiles) |
| Agent template | The unconfigured source a configured name was made from | [definition](03-records.md#agent-templates) |
| Personal agent | Owner-tagged agent classification with a dedicated web view; access remains ordinary record access | [definition](03-records.md#personal-and-shared) |
| Protocol hint | How a caller reaches an external service | [definition](06-services.md#how-to-call-it) |
| Registry configuration | Private setup fetched by the record it belongs to | [definition](03-records.md#configuring-a-template) |
| Record kind | What a record is, as one of five stored values | [definition](03-records.md#record-kinds) |
| Service secret | Credential held on an external service record, read by principals it admits | [definition](06-services.md#secrets) |
| Channel | The 📮 or 📣 **record** published to: a name nobody acts as | [definition](07-channels.md#the-two-channel-kinds) |
| Topic | A **label on one message**, matched by a filtered read — never a record | [definition](04-messaging.md#envelope) |
| Inbox | Queue belonging to a registered name; the four kinds that are not 📡 have one | [definition](04-messaging.md#inbox-queues) |
| Receipt | Receiver acknowledgement of progress | [definition](04-messaging.md#receipts) |
| Envelope | Message routing and body container | [definition](04-messaging.md#envelope) |
| Shared reader | Member of an explicit consumer pool | [definition](04-messaging.md#several-readers-may-wait-when-they-say-so) |
| Audience | Callers allowed to see and use a name | [definition](05-discovery.md#audience) |
| Listing observations | Daemon-observed state | [definition](05-discovery.md#what-a-listing-answers) |
| Foreground runner | User-launched script agent | [definition](08-runner-role.md#script-agents) |
| Adapter | Runtime transport or port implementation | [definition](08-runner-role.md#adapters) |
| Supervisor and bus | Listener lifetime and request processing roles | [definition](11-processes.md#the-processes) |
| Port and face | Dependency seam and entry point | [definition](10-modules.md#the-rule) |

## Future vocabulary

Future terms stay with their owning design: [R1](../Plans/R1.0-Release/README.md#scope),
[R1.1](../Plans/R1.1/README.md#scope), [R1.2](../Plans/R1.2/README.md#scope).
The [historical glossary](../Plans/R0.8-MVP/done/glossary-before-rewrite.md#glossary) preserves earlier names and proposed verbs; it is not a list of shipped commands.
