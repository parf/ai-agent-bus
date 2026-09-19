# Glossary

📌 **TL;DR:** Names and terms, linked to their definitions.

## Names

Current naming index. Canonical values and definitions remain in the linked sections.

| Name | Meaning | Definition |
|---|---|---|
| agent-busd | Daemon | [programs](09-setup.md#the-programs) |
| agent-bus | User CLI | [programs](09-setup.md#the-programs) |
| agent-bus-setup | Installer | [installation](09-setup.md#install) |
| agent-bus-token | Credential program | [programs](09-setup.md#the-programs) |
| agent-bus-admin | Administration program | [SSH administration](09-setup.md#ssh-admin) |
| agent-bus-web | Dashboard process | [processes](11-processes.md#the-processes) |
| ab_ | MCP tool prefix only; never CLI or prose shorthand | [faces](05-discovery.md#faces) |

## Vocabulary

Use service discovery for the overall capability, registration for one operation.
Use realm for the name's authority namespace, and service template for the
unconfigured capability. Personal is an owner-selected
[service classification](03-services-and-topics.md#personal-and-shared), not a record kind.

## Glyphs

One glyph per meaning, and this table is where a meaning is fixed.
[Web glyphs](../Plans/MVP/web/glyphs.md#the-rule-that-matters-most) owns where a
glyph may appear, how it renders and what it must never carry on its own.

| Glyph | Code point | Name | Means | Status |
|---|---|---|---|---|
| 👤 | `U+1F464` | User | one registered person | built |
| 👥 | `U+1F465` | Group | a group or team | built |
| 👾 | `U+1F47E` | Agent | an agent, and the queue named after it | built in 0.6.1 |
| 🪪 | `U+1FAAA` | Identity | an identity as such, no entity type asserted | built |
| 🔑 | `U+1F511` | Credentials | credentials proving an identity; never the secret value | built |
| ⚙️ | `U+2699 U+FE0F` | Service | a service identity | built, and **retiring** |
| 📡 | `U+1F4E1` | Service | something external, not on this bus | pending |
| 📮 | `U+1F4EE` | Queue | a registered queue | pending |
| 📣 | `U+1F4E3` | PubSub | a pub/sub topic | pending |

Pending rows arrive with [five record kinds](03-services-and-topics.md#five-record-kinds)
in [0.6.0](../Plans/MVP/0.6.0-TODO.md#the-enum), where `📡` takes Service from
`⚙️` and `⚙️` stops labelling a record at all, staying with the daemon.
`📥 Inbox` labelled an agent's record in 0.5.84 and was replaced by `👾` in 0.6.1.

## Terms

| Term | Meaning | Definition |
|---|---|---|
| Principal | Credential-authenticated identity | [definition](01-identity-and-roles.md#identities) |
| Canonical name | Routing identity | [definition](01-identity-and-roles.md#names) |
| Token | Principal credential | [definition](02-access.md#what-a-call-carries) |
| Local socket | Account credential | [definition](02-access.md#local-socket) |
| Service ACL | Visibility and use policy, including runtime `@owner` | [definition](02-access.md#acl) |
| Owner | Highest authority within the named scope | [definition](01-identity-and-roles.md#role-names-and-scopes) |
| Daemon Owner | Root-like authority over the node; assigned through setup | [definition](01-identity-and-roles.md#daemon-owner) |
| Administrator | Manages daemon users and groups | [definition](01-identity-and-roles.md#daemon-administrators) |
| Maintainer | Explicitly assigned to manage a service or channel | [definition](01-identity-and-roles.md#services) |
| User | Registered person | [definition](01-identity-and-roles.md#users-and-profiles) |
| Member | Basic access to a service/channel | [definition](01-identity-and-roles.md#role-names-and-scopes) |
| Person profile | Identifying and descriptive information | [definition](01-identity-and-roles.md#users-and-profiles) |
| Service and service template | Configured service and its unconfigured source | [definition](03-services-and-topics.md#service-and-template) |
| Personal service | Owner-tagged service classification with a dedicated web view; access remains ordinary service access | [definition](03-services-and-topics.md#personal-and-shared) |
| Protocol hint | How a caller reaches an external service | [definition](03-services-and-topics.md#how-to-call-it) |
| Registry configuration | Private setup fetched by its service | [definition](03-services-and-topics.md#configuring-a-template) |
| Record kind | What a record is, as one of five stored values | [definition](03-services-and-topics.md#five-record-kinds) |
| Service secret | Credential held on an external service record, read by principals it admits | [definition](03-services-and-topics.md#service-secrets) |
| Channel | Service-like entity without an actual service process | [definition](01-identity-and-roles.md#channels) |
| Topic | Delivery term for a channel | [definition](03-services-and-topics.md#topics) |
| Inbox | Queue belonging to a registered name | [definition](04-messaging.md#inbox-queues) |
| Receipt | Receiver acknowledgement of progress | [definition](04-messaging.md#receipts) |
| Envelope | Message routing and body container | [definition](04-messaging.md#envelope) |
| Shared reader | Member of an explicit consumer pool | [definition](04-messaging.md#several-readers-may-wait-when-they-say-so) |
| Audience | Callers allowed to see and use a name | [definition](05-discovery.md#audience) |
| Listing observations | Daemon-observed state | [definition](05-discovery.md#what-a-listing-answers) |
| Foreground runner | User-launched script service | [definition](08-runner-role.md#script-services) |
| Adapter | Runtime transport or port implementation | [definition](08-runner-role.md#adapters) |
| Supervisor and bus | Listener lifetime and request processing roles | [definition](11-processes.md#the-processes) |
| Port and face | Dependency seam and entry point | [definition](10-modules.md#the-rule) |

## Future vocabulary

Future terms stay with their owning design: [R1](../Plans/R1/README.md#scope),
[R1.1](../Plans/R1.1/README.md#scope), [R1.2](../Plans/R1.2/README.md#scope).
The [historical glossary](../Plans/MVP/done/glossary-before-rewrite.md#glossary) preserves earlier names and proposed verbs; it is not a list of shipped commands.
