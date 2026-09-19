# Version comparison

Release stages, in order. Each section describes the major change from the preceding stage; the linked plans own scope and status. Program release numbers follow [shared versioning](../CLAUDE.md#versioning), and individual shipped changes belong in the [changelog](../CHANGELOG.md#changelog).

Credential lifetime follows the same [manual-change policy](../docs/02-access.md#token-lifetime) throughout these stages; future encryption does not introduce scheduled token retirement.

## PoC

**Prove that agents can discover each other and exchange useful work.** The completed PoC established the working bus, connected live agent sessions and made scripts callable as services. Its achievement was the end-to-end interaction; shared-host administration and operational hardening were left for the next stage (PoC scope (PoC plan, removed 2026-09-18), completion evidence (PoC plan, removed 2026-09-18)).

| Major capability | What the PoC established | Detail |
|---|---|---|
| Discovery and delivery | Participants register, find a destination and exchange messages through the bus | PoC results (PoC plan, removed 2026-09-18) |
| Useful conversations | A service takes work and answers; callers correlate the response with the request | PoC scope (PoC plan, removed 2026-09-18) |
| Agent integration | Live sessions can receive work through runtime adapters and answer through the MCP face | PoC scope (PoC plan, removed 2026-09-18) |
| Script services | Existing scripts become callable without implementing a bus client themselves | PoC scope (PoC plan, removed 2026-09-18) |
| Deliberate limits | Owner-operated, trusted-host use; no shared-user policy, restart recovery or dashboard | PoC boundaries (PoC plan, removed 2026-09-18) |

## MVP

**Turn the working prototype into a bus other people can install and share.** The major change is independent caller identity with enforced access, supported by restart recovery, a dashboard and a process split. Much of this is built, but MVP remains active: people administration, packaging and installed isolation evidence still prevent stage acceptance ([MVP scope](MVP/README.md#scope), [remaining work](MVP/TODO.md#remaining-work)).

| Major change from PoC | What changes for users and operators | Status and detail |
|---|---|---|
| **Shared-host identity and access** | Users become distinct callers; the daemon limits both discovery and use to their authority | Built: [credentials](../docs/02-access.md#scope), [ACL](../docs/02-access.md#acl) |
| More reliable work handling | Callers receive clearer completion signals, stale work can be skipped, and reader pools can share an inbox | Built: [messaging](../docs/04-messaging.md#status) |
| Publish to subscribers | Topic delivery expands from competing consumers to fan-out into subscriber inboxes | Built: [subscribers](../docs/04-messaging.md#subscribers) |
| **Recovery across restarts** | Registry and waiting work can return after a restart, within the documented loss window | Built: [durability](../docs/04-messaging.md#durability) |
| Operational visibility | Operators can inspect permitted records, backlog, losses and message activity through a signed-in dashboard | Built, with people view pending: [dashboard views](../docs/05-discovery.md#what-it-shows) |
| **Installation and privilege separation** | The daemon gains a managed installation; separate processes and optional script confinement narrow responsibilities | Setup and split built; package and installed checks pending: [setup](../docs/09-setup.md#status), [stage gate](MVP/TODO.md#installed-stage-gate) |
| Usable runtime integrations | Packaged integrations and smart launchers make the existing adapters usable from a fresh installation | Built implementation; live-runtime and fresh-host acceptance pending: [integration delivery](../docs/08-runner-role.md#runtime-integration-delivery), [launchers](../docs/08-runner-role.md#smart-launchers) |
| Trusted people and service descriptions | Maintainer-controlled profiles and a description on every record make the registry more useful to people and agents | Built: [person records](../docs/01-identity-and-roles.md#users-and-profiles), [service description](../docs/03-services-and-topics.md#service-and-template); generated method information moved to [R1](R1/discovery.md#method-metadata) |
| Release identification | Programs report a consistent release identity, and running processes expose operational context | Built: [build information](../docs/09-setup.md#build-information), [process titles](../docs/11-processes.md#process-titles) |

The trust boundary has not changed yet: the MVP daemon can read message bodies and stored configuration ([current trust boundary](../docs/02-access.md#trust-boundary)).

## R1

**Move from a shared local bus to managed services and distributed operation.** R1 proposes the largest architectural expansion: distributed identity, federation, service-scoped credentials, end-to-end body encryption and a runner that keeps deployments operating without a foreground session. These are future targets, not shipped behavior; scope and several security and lifecycle choices remain open ([R1 scope](R1/README.md#scope), [dependencies](R1/TODO.md#dependencies)).

| Major change from MVP | Proposed result | Owning design |
|---|---|---|
| **Distributed identity and policy** | Identity administration can span deployments, with richer organizational access decisions | [AUTH](R1/auth.md#bundle), [groups and roles](R1/identity.md#groups-and-roles) |
| **A stronger trust boundary** | Credentials can be limited to their destination, and endpoint encryption can keep bodies unreadable to the bus | [Scoped credentials](R1/access.md#token-scope), [encryption](R1/access.md#encrypted-sessions) |
| Federation | Discovery and calls can reach upstream services; peer nodes can exchange registry records | [Chaining](R1/federation.md#chaining), [peer registry](R1/registry.md#registry-sync) |
| **Managed service lifecycle** | Services become installed deployments with startup and restart behavior; kept children can retain state between messages | [Managed runner](R1/runner.md#what-the-runner-does), [long-lived services](R1/runner.md#long-lived-services) |
| Distributed work and coordination | Workers can serve a shared name across hosts, and shared resources can be coordinated through the bus | [Pools](R1/runner.md#one-name-on-many-hosts), [locks](R1/locks.md#shared-locks) |
| Deployment recovery | A user can recover deployment configuration from an encrypted backup without giving the daemon recovery authority | [Backup contract](R1/runner.md#backing-it-up) |
| Installable distributions | Published artifacts make installation and container startup possible without building from source | [Release artifacts](R1/distribution.md#release-artifacts) |
| Richer operations | Health, load history, deployment controls and record origin extend the MVP's current-state view | [Dashboard extensions](R1/discovery.md#dashboard-extensions), [reload](R1/operations.md#reload) |
| Broader clients and exchanges | Additional client implementations and streamed answers become possible once their protocol contract is agreed | [Clients](R1/modules.md#modules), [streaming](R1/access.md#streaming) |

Encrypted delivery to absent receivers, peer authenticity and clocks, namespace composition and runner activation remain [open R1 choices](R1/QUESTIONS.md#open-questions). Recording a target does not close those dependencies.

## R1.1

**Make the platform useful out of the box with a catalogue of ordinary services.** Where R1 builds the operating mechanisms, R1.1 proposes tools that use them, adds the catalogue to the image and improves service-to-service and person-facing workflows. It is not started and depends on R1; catalogue entries are intended to use the ordinary service interface, while other stage extensions may require daemon work ([R1.1 scope](R1.1/README.md#scope), [prerequisites](R1.1/TODO.md#dependencies)).

| Major change from R1 | Proposed result | Owning design |
|---|---|---|
| **Bundled useful services** | Teams can deploy existing tools instead of first writing every integration themselves | [Catalogue](R1.1/services.md#the-catalogue) |
| An ordinary extension path | Bundled tools exercise the same interface available to independently written services | [Catalogue contract](R1.1/services.md#rules-they-all-obey) |
| **Catalogue distribution** | The image gains useful services already installed | [Image](R1.1/image.md#the-image) |
| Service-to-service credentials | A running service can obtain a destination-scoped credential without a person at a keyboard | [Service credentials](R1.1/access.md#service-to-service) |
| Registry housekeeping | Incidental records can expire while deliberately retained and actively served records remain | [Record lifetime](R1.1/records.md#how-long-a-record-lives) |
| Reaching people | Contact preferences and ordinary delivery services can route messages and operational alerts to humans | [Contact routes](R1.1/people.md#how-to-reach-a-person), [alerting](R1.1/services.md#the-bus-watching-itself) |

Catalogue selection, shared-state authority and contact visibility still have [open choices](R1.1/QUESTIONS.md#open-questions). The separate upstream integration request remains an [external dependency](R1.1/README.md#external-dependency).

## R1.2

**Use experience with real tools to decide where shared state, service identity and daemon responsibilities should live.** R1.2 is exploratory and unscheduled. Its potential changes are architectural choices to evaluate after the catalogue exists, not a committed feature bundle ([R1.2 scope](R1.2/README.md#scope), [research objective](R1.2/TODO.md#objective)).

| Potential major change after R1.1 | What is being evaluated | Owning question |
|---|---|---|
| **Independent service identity and shared state** | How services obtain their own key and share secrets or mutable state, including whether the storage provider must be unable to read it | [Service identity and state](R1.2/exploration.md#shared-secrets-and-a-kv-with-locks) |
| **Daemon components as ordinary services** | Whether some built-in responsibilities should move behind the same service interface as the catalogue | [Component placement](R1.2/exploration.md#whether-the-daemons-own-parts-become-services) |
| Cross-provider person linkage | How separately authenticated principals can be established as the same person | [Identity linkage](R1.2/QUESTIONS.md#identity-linkage) |

## Future

**Keep useful ideas visible without assigning them a release.** Future is an unassigned holding area, not the version after R1.2. Its proposals become release work only after an owner chooses scope and dependencies ([Future topics](Future/README.md#topics)).

| Direction | Potential change | Owning proposal |
|---|---|---|
| Paid services | Add charging and balance enforcement around routed use | [Billing](Future/billing.md#billing-role--future) |
| Additional identity and public lookup | Broaden identity sources or expose a public directory, subject to their distinct access decisions | [Directory integration](Future/ldap-ad.md#one-source-not-two), [public directory](Future/public-directory.md#a-public-directory-of-people-and-their-keys) |
| Storage alternatives | Reconsider persistence, encryption at rest and replication after evaluating the actual engine properties | [Storage](Future/storage.md#storage) |
| Additional adapters and diagnostics | Extend host/runtime support and troubleshooting as concrete needs arise | [Candidates](Future/FUTURE.md#candidates), [debug tracing](Future/debug.md#debug-mode) |
