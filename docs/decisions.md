# MVP decisions

## Settled

Migrated 2026-09-13. Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

| Decision topic | Substance | Earlier rows |
|---|---|---|
| Versioning | [definition](../CLAUDE.md#versioning) | D1 |
| Build information | [definition](09-setup.md#build-information) | D2 |
| Process titles | [definition](11-processes.md#process-titles) | D3 |
| Names | [definition](01-identity.md#names) | D4, D5, D67, D68, D116, D126, D127 |
| The two accounts | [definition](09-setup.md#the-two-accounts) | D8, D14, D195 |
| The programs | [definition](09-setup.md#the-programs) | D9, D10, D13 |
| Getting a token | [definition](02-access.md#getting-a-token) | D11, D43, D44, D47 |
| Proving possession | [definition](01-identity.md#proving-possession) | D12, D33, D34, D35 |
| Acl | [definition](01-identity.md#acl) | D15, D25, D36 |
| Registration | [definition](01-identity.md#registration) | D16, D17 |
| Profile fields | [definition](01-identity.md#profile-fields) | D19, D248 |
| Who may write a record | [definition](01-identity.md#who-may-write-a-record) | D20, D21 |
| Every identifying field is unique | [definition](01-identity.md#every-identifying-field-is-unique) | D22 |
| Audience | [definition](05-discovery.md#audience) | D31 |
| Encrypted sessions | [definition](02-access.md#encrypted-sessions) | D32 |
| Ownership | [definition](01-identity.md#ownership) | D39, D52 |
| What a call carries | [definition](02-access.md#what-a-call-carries) | D42 |
| Token scope | [definition](02-access.md#token-scope) | D45 |
| Local socket | [definition](02-access.md#local-socket) | D48, D56, D57 |
| Token lifetime | [definition](02-access.md#token-lifetime) | D49, D50, D51 |
| Storage | [definition](09-setup.md#storage) | D53 |
| Durability | [definition](04-messaging.md#durability) | D54, D55 |
| Service and template | [definition](03-services-and-topics.md#service-and-template) | D70, D71, D76, D77 |
| Configuring a template | [definition](03-services-and-topics.md#configuring-a-template) | D72, D73, D74, D184 |
| Why a digest at all | [definition](03-services-and-topics.md#why-a-digest-at-all) | D75 |
| Topics | [definition](03-services-and-topics.md#topics) | D78 |
| Inbox queues | [definition](04-messaging.md#inbox-queues) | D81, D185, D231 |
| Verbs | [definition](04-messaging.md#verbs) | D82, D237 |
| Message fields | [definition](04-messaging.md#message-fields) | D83 |
| Receipts | [definition](04-messaging.md#receipts) | D84, D229, D230, D253 |
| Message ttl | [definition](04-messaging.md#message-ttl) | D85, D255 |
| Push and pull | [definition](04-messaging.md#push-and-pull) | D86, D176 |
| Overflow | [definition](04-messaging.md#overflow) | D87, D235 |
| Envelope | [definition](04-messaging.md#envelope) | D89 |
| What a listing answers | [definition](05-discovery.md#what-a-listing-answers) | D90, D91, D92, D93, D188 |
| What it shows | [definition](05-discovery.md#what-it-shows) | D94, D247 |
| Refusals | [definition](05-discovery.md#refusals) | D95 |
| Dashboard | [definition](05-discovery.md#dashboard) | D96, D97, D98 |
| Where it listens | [definition](05-discovery.md#where-it-listens) | D99 |
| What is shared | [definition](11-processes.md#what-is-shared) | D101, D192 |
| How a child is started | [definition](11-processes.md#how-a-child-is-started) | D102 |
| Adapters | [definition](08-runner-role.md#adapters) | D109, D177 |
| 12-stages | [definition](12-stages.md#stages) | D110 |
| Request and reply | [definition](04-messaging.md#request-and-reply) | D112, D227, D236, D238, D239, D240 |
| Script services | [definition](08-runner-role.md#script-services) | D117, D250, D251, D252, D254 |
| One reader per inbox | [definition](04-messaging.md#one-reader-per-inbox) | D169, D172, D220, D221, D222 |
| Reply routing | [definition](04-messaging.md#reply-routing) | D170 |
| Languages | [definition](10-modules.md#languages) | D179 |
| External tools | [definition](10-modules.md#external-tools) | D180, D182, D216 |
| Http is built in | [definition](10-modules.md#external-tools) | D181 |
| Our own small module | [definition](10-modules.md#external-tools) | D183 |
| How to call it | [definition](03-services-and-topics.md#how-to-call-it) | D186, D187 |
| The rule | [definition](10-modules.md#the-rule) | D189, D215, D217 |
| The rule | [definition](11-processes.md#the-rule) | D190, D193 |
| The processes | [definition](11-processes.md#the-processes) | D191 |
| Nothing the daemon runs may exec | [definition](11-processes.md#nothing-the-daemon-runs-may-exec) | D194 |
| The three doors | [definition](02-access.md#the-three-doors) | D196 |
| Why the supervisor holds cap_chown | [definition](11-processes.md#why-the-supervisor-holds-cap_chown) | D214 |
| Goal | [definition](00-overview.md#goal) | D218 |
| Sandboxing | [definition](08-runner-role.md#sandboxing) | D223, D224 |
| Stopping it and reading what it said | [definition](08-runner-role.md#stopping-it-and-reading-what-it-said) | D225, D226 |
| Several readers may wait when they say so | [definition](04-messaging.md#several-readers-may-wait-when-they-say-so) | D228 |
| Subscribers | [definition](04-messaging.md#subscribers) | D232, D233, D234 |
| Rules it is built to | [definition](05-discovery.md#rules-it-is-built-to) | D241, D242, D244 |
| Signing in | [definition](05-discovery.md#signing-in) | D245, D246 |
| Current and future documentation ownership | [working rules](../CLAUDE.md#working-rules) | 2026-09-13 owner instruction |

## Open

Unresolved choices live in [MVP questions](../Plans/MVP/QUESTIONS.md#open-questions).

## History

Original wording and superseded choices are preserved in [decision history](../Plans/MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../Plans/MVP/done/document-migration.md#decision-mapping).
