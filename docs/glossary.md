# Glossary

📌 **TL;DR:** Names and terms, linked to their definitions: the programs, the
vocabulary and the glyphs used in the CLI and dashboard.

## Names

| Name | Meaning | Definition |
|---|---|---|
| agent-busd | Daemon | [programs](user/cli.md) |
| agent-bus | User CLI | [programs](user/cli.md) |
| agent-bus-setup | Installer | [installation](07-setup.md#install) |
| agent-bus token | Credential program | [programs](user/cli.md) |
| agent-bus-admin | Administration program | [SSH administration](administration.md#ssh-admin) |
| agent-bus-web | The web face: TypeScript on bun, its own account and systemd unit | [the web face](08-processes.md#the-web-face) |
| ab_ | MCP tool prefix only; never CLI or prose shorthand | [faces](05-discovery.md#faces) |
| lock | a named right to be the one holder on a record, for a stated ttl | [shared locks](01-identity-and-authority.md#shared-locks) |
| key-value store | a record's named string, int and JSON values, edited atomically by its Owner, Maintainers and own Agent | [key-value store](03-records-kv.md#key-value-store) |
| position | what a principal is toward the node or a record — daemon Owner, Administrator, Owner, Maintainer or Member — deciding what it may manage; never called a role | [positions and scopes](01-identity-and-authority.md#positions-and-scopes) |
| role | what a sender holds toward the record it addressed — generated and assigned roles — told to an Agent on each message as `roles` and as `AB_ROLE_<NAME>=1`. The daemon never interprets one, and it grants no position | [roles](03-records-agent.md#roles) |
| process role | which part a process plays: supervisor, bus, web face, runner | [processes](08-processes.md#the-processes) |

## Vocabulary

- **Service discovery** is the overall capability; **registration** is an operation.
- **Channel** names a record; **topic** labels a message ([channels](03-records-channel.md#channel-not-topic)).
- **Deliver-To** lists PubSub recipients; its ACL admits publishers. A recipient removes itself with `unsubscribe`; the API route remains `/subscribe` and its wire field `subs` ([delivery](04-messaging.md#deliver-to)).
- **Helper subscriptions** forward selected external events, such as Slack messages or new mail ([subscriptions](agents/subscriptions.md)).
- **Service** means an external system; a program answering through the bus is an **Agent** ([kinds](03-records.md#record-kinds)).
- **Personal** classifies a record, rather than defining its kind ([Personal records](03-records.md#personal-and-shared)).
- **Realm** is the name's namespace. A realm-less name and the same local part with a realm are distinct ([names](01-identity-and-authority.md#names)).
- **Inactive** is the status besides active; use that term for the [record state](constitution.md#common-record-fields).

## Glyphs

Most glyphs identify entity types; `🔱` and `👮` identify authority.
See [web glyph usage](../src/web/web-face/shell.md#glyphs) for placement and text labels.

| Glyph | Code point | Name | Means |
|---|---|---|---|
| 👤 | `U+1F464` | User | one registered person |
| 👥 | `U+1F465` | Group | a group or team |
| 👾 | `U+1F47E` | Agent | an agent, and the queue named after it |
| 🪪 | `U+1FAAA` | Identity | an identity as such, no entity type asserted |
| 🔑 | `U+1F511` | Credentials | credentials proving an identity; never the secret value |
| 🔱 | `U+1F531` | Daemon owner | the one authority on a node that cannot be delegated |
| 👮 | `U+1F46E` | Maintainers | the named list that may edit a record's settings and ACL |
| ⚙️ | `U+2699 U+FE0F` | Daemon | the daemon itself, never a record |
| 📡 | `U+1F4E1` | Service | something external, not on this bus |
| 📮 | `U+1F4EE` | Queue | a registered queue |
| 📣 | `U+1F4E3` | PubSub | a pub/sub channel |

`📡`, `📮` and `📣` label [record kinds](03-records.md#record-kinds); `⚙️`
labels no record and is held for the daemon.

## Terms

| Term | Meaning | Definition |
|---|---|---|
| Principal | Credential-authenticated identity | [definition](01-identity-and-authority.md#identities) |
| Canonical name | Routing identity | [definition](01-identity-and-authority.md#names) |
| Token | Principal credential | [definition](02-access.md#what-a-call-carries) |
| Local socket | Account credential | [definition](02-access.md#local-socket) |
| Remote access | Reaching the bus from another host: HTTP, HTTPS or a forwarded socket | [definition](02-access-remote.md#choosing-a-way) |
| Dual listener | One TCP port answering TLS and plain HTTP, sniffed per connection | [definition](08-processes.md#the-tcp-listener) |
| TLS directory | Where setup keeps the node's certificate, chain and key | [definition](administration.md#tls) |
| Fingerprint, pin | The certificate's SHA-256, which a client checks instead of a CA | [definition](02-access-remote.md#over-https) |
| Record ACL | Visibility and use policy, including runtime `@owner` | [definition](02-access.md#acl) |
| Record authority | The Owner's and Maintainers' standing over a record; never *resource authority*, since a resource is the 📚 kind | [definition](01-identity-and-authority.md#record-authority) |
| Owner | Highest authority within the named scope | [definition](01-identity-and-authority.md#positions-and-scopes) |
| Daemon Owner | Root-like authority over the node; assigned through setup | [definition](01-identity-and-authority.md#daemon-owner) |
| Administrator | Manages daemon users and groups | [definition](01-identity-and-authority.md#daemon-administrators) |
| Maintainer | Explicitly assigned to manage a record | [definition](01-identity-and-authority.md#record-authority) |
| User | Registered person | [definition](01-identity-and-authority.md#users-and-profiles) |
| Member | Basic access to a record | [definition](01-identity-and-authority.md#positions-and-scopes) |
| Person profile | Identifying and descriptive information | [definition](01-identity-and-authority.md#users-and-profiles) |
| Agent template | The unconfigured source a configured name was made from | [definition](03-records-agent.md#agent-templates) |
| Personal | Owner-tagged classification of any kind, a `?personal=1` filter on the web face; access remains ordinary record access | [definition](03-records.md#personal-and-shared) |
| Protocol hint | How a caller reaches an external service | [definition](03-records-service.md#how-to-call-it) |
| Registry configuration | JSON settings visible to Owner and Maintainers, and read by an Agent itself | [definition](03-records-agent.md#configuring-a-template) |
| Record kind | What a record is, as one of seven stored values | [definition](03-records.md#record-kinds) |
| Resource | A 📚 card for data an MCP client may read, answered by its source; 🧩 a template | [definition](03-records-resource.md#what-a-resource-is) |
| Sandbox | The limits a runner or the dedicated runner puts on a script's processes: filesystem, network, privilege and resources | [definition](06-dedicated-runner.md#sandboxing) |
| Service secret | Credential held on an external service record, read by its Owner and Maintainers | [definition](03-records-service.md#secrets) |
| Channel | The 📮 or 📣 **record** published to: a name nobody acts as | [definition](03-records-channel.md#the-two-channel-kinds) |
| Topic | A **label on one message**, matched by a filtered read — never a record | [definition](04-messaging.md#envelope) |
| Inbox | Queue belonging to a registered name: 👤 users, 👾 agents and 📮 queues have one; a 📣 keeps nothing, and 📡, 👥 and 📚 hold no messages | [definition](04-messaging.md#inbox-queues) |
| Receipt | Receiver acknowledgement of progress | [definition](04-messaging.md#receipts) |
| Envelope | Message routing and body container | [definition](04-messaging.md#envelope) |
| Shared reader | Member of an explicit consumer pool | [definition](04-messaging.md#several-readers-may-wait-when-they-say-so) |
| Audience | Callers allowed to see and use a name | [definition](05-discovery.md#audience) |
| Listing observations | Daemon-observed state | [definition](05-discovery.md#what-a-listing-answers) |
| Dedicated runner | `agent-bus run`: one script behind one agent, in the foreground; never called just "runner" | [definition](06-dedicated-runner.md#what-it-does) |
| Runner | `agent-bus-runner`: a person's service hosting their installed agents | [guide](runner/README.md) |
| Session launcher | `ab-claude`, `ab-codex`, `ab-opencode`: a live AI session behind a name | [definition](06-launchers.md#smart-launchers) |
| Adapter | Runtime transport or port implementation | [definition](06-launchers.md#adapters) |
| Supervisor and bus | Listener lifetime and request processing roles | [definition](08-processes.md#the-processes) |
| Port and face | Dependency seam and entry point | [definition](../src/MODULES.md#the-rule) |
