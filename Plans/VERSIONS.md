# Version comparison

Release stages, in order. Each section describes the major change from the preceding stage; the linked plans own scope and status. Program release numbers follow [shared versioning](../CLAUDE.md#versioning), and individual shipped changes belong in the changelog for their release line: the current [0.8](../CHANGELOG.0.8.md#changelog-08), and the older [0.7](CHANGELOG.0.7.md#changelog-07), [0.6](CHANGELOG.0.6.md#changelog-06) and [0.5](CHANGELOG.0.5.md#changelog-05) kept here.

Credential lifetime follows the same [manual-change policy](../docs/02-access.md#token-lifetime) throughout these stages; future encryption does not introduce scheduled token retirement.

## MVP

**A bus other people can install and share.** Released on the 0.8 line, complete 2026-09-23: independent caller identity with enforced access, write-through storage, a web face and a process split, packaged for a fresh host ([MVP scope](R0.8-MVP/README.md#scope), [completion](R0.8-MVP/DONE.md#done--mvp)). Follow-up is [constitution conformance](R0.8-MVP/TODO.md#constitution-conformance).

| Major change | What it gives users and operators | Status and detail |
|---|---|---|
| **Shared-host identity and access** | Users become distinct callers; the daemon limits both discovery and use to their authority | Built: [credentials](../docs/02-access.md#scope), [ACL](../docs/02-access.md#acl) |
| More reliable work handling | Callers receive clearer completion signals, stale work can be skipped, and reader pools can share an inbox | Built: [messaging](../docs/04-messaging.md#status) |
| Publish to subscribers | Topic delivery expands from competing consumers to fan-out into subscriber inboxes | Built: [subscribers](../docs/04-messaging.md#subscribers) |
| **Recovery across restarts** | Registry and waiting work can return after a restart, within the documented loss window | Built: [durability](../docs/04-messaging.md#durability) |
| Operational visibility | Operators can inspect permitted records, backlog, losses and message activity through a signed-in dashboard | Built, as the TypeScript web face under its own unit from 0.8.50: [dashboard views](../docs/05-discovery.md#what-it-shows) |
| **Installation and privilege separation** | The daemon gains a managed installation; separate processes and optional script confinement narrow responsibilities | Built and accepted on fresh hosts: [setup](../docs/09-setup.md#status), [stage gate](R0.8-MVP/TODO.md#installed-stage-gate) |
| Usable runtime integrations | Packaged integrations and smart launchers make the existing adapters usable from a fresh installation | Built and accepted on fresh hosts: [integration delivery](../docs/08-runner-role.md#runtime-integration-delivery), [launchers](../docs/08-runner-role.md#smart-launchers) |
| Trusted people and service descriptions | Maintainer-controlled profiles and a description on every record make the registry more useful to people and agents | Built: [person records](../docs/01-identity-and-roles.md#users-and-profiles), [service description](../docs/03-records.md#agent-templates); generated method information moved to [R1](R1.0-Release/method-metadata.md#method-metadata) |
| Encrypted transport between hosts | The daemon's port answers TLS beside plain HTTP when setup turns it on; clients pin a self-signed certificate by fingerprint | Built 0.8.60: [TLS](../docs/02-access-remote.md#over-https) |
| Release identification | Programs report a consistent release identity, and running processes expose operational context | Built: [build information](../docs/09-setup.md#build-information), [process titles](../docs/11-processes.md#process-titles) |

The trust boundary: the daemon can read message bodies and stored configuration ([current trust boundary](../docs/02-access.md#trust-boundary)).

## R1

**The release's own extensions, and what one bus needs to connect to another.** R1 keeps the shared extensions (locks, the key-value store, Resource records, method metadata), federation, installable distribution, the managed runner and the client libraries. These are future targets, not shipped behavior; several choices remain open ([R1 scope](R1.0-Release/README.md#scope), [dependencies](R1.0-Release/TODO.md#objective)). Distributed identity, encryption and observability moved to [R1.1](#r11).

| Major change from MVP | Proposed result | Owning design |
|---|---|---|
| Federation (on hold) | Discovery and calls can reach upstream services | [Chaining](R1.0-Release/federation.md#chaining) |
| Record-defined roles | A record states what its caller may do, and the daemon resolves but never interprets it | [Roles](R1.0-Release/roles.md#record-defined-roles) |
| **Managed service lifecycle** | Services become installed deployments with startup and restart behavior; kept children can retain state between messages | [Managed runner](R1.0-Release/runner.md#what-the-runner-does), [long-lived services](R1.0-Release/runner.md#long-lived-services) |
| Distributed work and coordination | Workers can serve a shared name across hosts, and shared resources can be coordinated through the bus | [Pools](R1.0-Release/runner.md#one-name-on-many-hosts), [locks](R1.0-Release/locks.md#shared-locks) |
| Deployment recovery | A user can recover deployment configuration from an encrypted backup without giving the daemon recovery authority | [Backup contract](R1.0-Release/runner.md#backing-it-up) |
| Installable distributions | Published artifacts make installation and container startup possible without building from source | [Release artifacts](R1.0-Release/distribution.md#release-artifacts) |
| Broader clients | Client libraries in five languages become possible once their protocol contract is agreed | [Clients](R1.0-Release/modules.md#modules) |
| Resource records | An MCP Resource becomes a registry record kind the bus connects to rather than stores | [Resource records](R1.0-Release/resources.md#resource-records) |
| Shared state | Per-name key-value storage and shared locks coordinate work through the bus | [Key-value store](R1.0-Release/kv.md#per-record-storage), [shared locks](R1.0-Release/locks.md#shared-locks) |

Namespace composition, runner activation and what the kept topics need from R1.1 remain [open R1 choices](R1.0-Release/QUESTIONS.md#open-questions). Recording a target does not close those dependencies.

## R1.1

**Move from a shared local bus to distributed identity and a stronger trust boundary.** The old R1's identity half: distributed identity and policy, scoped credentials, end-to-end body encryption, and the observability that supervises them. It is not started and follows R1 ([R1.1 scope](R1.1/README.md#scope), [prerequisites](R1.1/TODO.md#dependencies)).

| Major change from MVP | Proposed result | Owning design |
|---|---|---|
| **Distributed identity and policy** | Identity administration can span deployments, with richer organizational access decisions | [AUTH](R1.1/auth.md#bundle) |
| **A stronger trust boundary** | Credentials can be limited to their destination, and endpoint encryption can keep bodies unreadable to the bus | [Scoped credentials](R1.1/access.md#token-scope), [encryption](R1.1/access.md#encrypted-sessions) |
| Richer operations | Health, load history, deployment controls and record origin extend the MVP's current-state view | [Dashboard extensions](R1.1/discovery.md#dashboard-extensions), [reload](R1.1/operations.md#reload) |
| Streamed answers | Long answers can stream once their protocol contract is agreed | [Streaming](R1.1/access.md#streaming) |
| Peer registry | Peer nodes can exchange registry records | [Peer registry](R1.1/registry.md#registry-sync) |

Peer authenticity and clocks, encrypted delivery to absent receivers and authorization freshness remain [open R1.1 choices](R1.1/QUESTIONS.md#open-questions).

## R1.2

**Make the platform useful out of the box with a catalogue of ordinary services.** Where R1 and R1.1 build the operating mechanisms, R1.2 proposes tools that use them, adds the catalogue to the image and improves service-to-service and person-facing workflows. It is not started and depends on R1 and R1.1; catalogue entries are intended to use the ordinary service interface, while other stage extensions may require daemon work ([R1.2 scope](R1.2/README.md#scope), [prerequisites](R1.2/TODO.md#dependencies)).

| Major change from R1.1 | Proposed result | Owning design |
|---|---|---|
| **Bundled useful services** | Teams can deploy existing tools instead of first writing every integration themselves | [Catalogue](R1.2/services.md#the-catalogue) |
| An ordinary extension path | Bundled tools exercise the same interface available to independently written services | [Catalogue contract](R1.2/services.md#rules-they-all-obey) |
| **Catalogue distribution** | The image gains useful services already installed | [Image](R1.2/image.md#the-image) |
| Service-to-service credentials | A running service can obtain a destination-scoped credential without a person at a keyboard | [Service credentials](R1.2/access.md#service-to-service) |
| Registry housekeeping | Incidental records can expire while deliberately retained and actively served records remain | [Record lifetime](R1.2/records.md#how-long-a-record-lives) |
| Reaching people | Contact preferences and ordinary delivery services can route messages and operational alerts to humans | [Contact routes](R1.2/people.md#how-to-reach-a-person), [alerting](R1.2/services.md#the-bus-watching-itself) |

Catalogue selection, shared-state authority and contact visibility still have [open choices](R1.2/QUESTIONS.md#open-questions). The separate upstream integration request remains an [external dependency](R1.2/README.md#external-dependency).

## R1.3

**Use experience with real tools to decide where shared state, service identity and daemon responsibilities should live.** R1.3 is exploratory and unscheduled. Its potential changes are architectural choices to evaluate after the catalogue exists, not a committed feature bundle ([R1.3 scope](R1.3/README.md#scope), [research objective](R1.3/TODO.md#objective)).

| Potential major change after R1.2 | What is being evaluated | Owning question |
|---|---|---|
| **Independent service identity and shared state** | How services obtain their own key and share secrets or mutable state, including whether the storage provider must be unable to read it | [Service identity and state](R1.3/exploration.md#shared-secrets-and-a-kv-with-locks) |
| **Daemon components as ordinary services** | Whether some built-in responsibilities should move behind the same service interface as the catalogue | [Component placement](R1.3/exploration.md#whether-the-daemons-own-parts-become-services) |
| Cross-provider person linkage | How separately authenticated principals can be established as the same person | [Identity linkage](R1.3/QUESTIONS.md#identity-linkage) |

## R2.0

**Keep useful ideas visible without assigning them a release.** R2.0 holds unassigned ideas; it is not a scheduled version. Its proposals become release work only after an owner chooses scope and dependencies ([R2.0 topics](R2.0-Future/README.md#topics)).

| Direction | Potential change | Owning proposal |
|---|---|---|
| Paid services | Add charging and balance enforcement around routed use | [Billing](R2.0-Future/billing.md#billing-role--future) |
| Additional identity and public lookup | Broaden identity sources or expose a public directory, subject to their distinct access decisions | [Directory integration](R2.0-Future/ldap-ad.md#one-source-not-two), [public directory](R2.0-Future/public-directory.md#a-public-directory-of-people-and-their-keys) |
| Storage alternatives | Reconsider persistence, encryption at rest and replication after evaluating the actual engine properties | [Storage](R2.0-Future/storage.md#storage) |
| Additional adapters and diagnostics | Extend host/runtime support and troubleshooting as concrete needs arise | [Candidates](R2.0-Future/FUTURE.md#candidates), [debug tracing](R2.0-Future/debug.md#debug-mode) |
