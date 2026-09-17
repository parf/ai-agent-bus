# Overview

The current documentation covers the MVP, including its pending requirements.
Every topic distinguishes built behavior from pending scope. The
[plan index](../Plans/README.md#stages) owns stage status and future work.

## Document ownership

Values have one owning section; other pages link to it.

| Topic | Owns |
|---|---|
| [Authority model](01-owners-and-maintainers.md#role-names-and-scopes) | Role names, authority scope and profile permissions |
| [Identity](01-identity.md#principals) | Names, enrolment, record ownership, ACL and person records |
| [Access](02-access.md#what-a-call-carries) | Credentials, rotation, sockets and the current trust boundary |
| [Services](03-services-and-topics.md#service-kinds) | Registration, configuration and topic properties |
| [Messaging](04-messaging.md#inbox-queues) | Delivery, receipts, deadlines, TTL, overflow and snapshots |
| [Discovery](05-discovery.md#faces) | Catalog, listing, dashboard, administration and what a refusal answers |
| [Runner](08-runner-role.md#script-services) | Foreground script services, push adapters, pending runtime integrations and launchers, sandboxing |
| [Setup](09-setup.md#the-programs) | Programs, accounts, paths, installation and build information |
| [Modules](10-modules.md#the-rule) | Implementation boundaries, languages and dependency rules |
| [Processes](11-processes.md#the-processes) | Supervisor, bus and web; privileges, listeners and process titles |
| [Glossary](glossary.md#names) | Current vocabulary |
| [Decisions](decisions.md#settled) | Index of current contracts and accepted pending MVP requirements |

## Goal

One daemon provides registry and message queues for agents and services.
The CLI, MCP face and dashboard expose that bus. It replaces Legacy-V1's
external broker; it does not require one.

## Principles

- Credentials identify callers, and the bus enforces access before delivery
  ([access](02-access.md#what-a-call-carries), [ACL](01-identity.md#acl)).
- A name owns its inbox; the process serving it may disappear while work waits
  ([messaging](04-messaging.md#inbox-queues)).
- Replies and receipts are ordinary messages; the daemon keeps no exchange state
  ([messaging](04-messaging.md#reply-routing)).
- The dashboard sees a filtered envelope feed, with bodies removed in the bus
  ([discovery](05-discovery.md#dashboard)). This is not body encryption.
- Persistent credentials and restart snapshots are separate adapters
  ([setup](09-setup.md#storage)).
- Runtime privilege boundaries and code dependency boundaries are separate
  ([processes](11-processes.md#the-rule), [modules](10-modules.md#the-rule)).

## Roles

The built runtime is described in [processes § the processes](11-processes.md#the-processes).
Optional future roles are owned by their release plans, not this document.

## Trade offs

| Choice | Accepted cost |
|---|---|
| [At most once consumption](04-messaging.md#one-reader-per-inbox) | A reader dying after dequeue can lose the message |
| [In-memory queues with snapshots](04-messaging.md#durability) | An unclean stop can lose traffic since the last snapshot |
| [Trusted host](02-access.md#encrypted-sessions) | The daemon can read bodies and stored configuration |
| [Filtered waits](04-messaging.md#one-reader-per-inbox) | A simultaneous unfiltered reader can take an exchange's next reply between waits |
