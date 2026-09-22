# Overview

📌 **TL;DR:** One daemon connects agents and services through names, queues and
permissions. This documentation covers the whole MVP, marking built and
pending scope in every topic, each value owned by one section the others link
to. Stage status stays in the [plan index](../Plans/README.md#stages).

## Document ownership

Values have one owning section; other pages link to it.

| Topic | Owns |
|---|---|
| [Constitution](constitution.md#project-constitution) | Intended model and accepted changes awaiting reconciliation with implementation and topic docs |
| [Identity and roles](01-identity-and-roles.md#identities) | Names, users, roles, groups and resource lifecycle |
| [Access](02-access.md#what-a-call-carries) | Authentication, credentials, ACLs, sockets and the trust boundary |
| [Records](03-records.md#five-record-kinds) | Record kinds, registration, agent templates, configuration and Personal |
| [Services](06-services.md#what-a-service-is) | The external 📡 case: address, protocol, secrets and what it has no queue for |
| [Channels](07-channels.md#the-two-channel-kinds) | The 📮 and 📣 kinds: delivery, retention and what publish stamps |
| [Messaging](04-messaging.md#inbox-queues) | Delivery, receipts, deadlines, TTL, overflow and snapshots |
| [Discovery](05-discovery.md#faces) | Catalog, listing, dashboard, administration and what a refusal answers |
| [Runner](08-runner-role.md#script-agents) | Foreground script agents, push adapters, pending runtime integrations and launchers, sandboxing |
| [Setup](09-setup.md#the-programs) | Programs, accounts, paths, installation and build information |
| [Modules](10-modules.md#the-rule) | Implementation boundaries, languages and dependency rules |
| [Processes](11-processes.md#the-processes) | Supervisor, bus and web; privileges, listeners and process titles |
| [Daemon API](13-daemon-api.md#how-a-call-is-made) | The HTTP routes, grouped; each one's meaning stays with its topic |
| [Glossary](glossary.md#names) | Current vocabulary |
| [Decisions](decisions.md#settled) | Index of current contracts and accepted pending MVP requirements |

## Goal

One daemon provides registry and message queues for agents and services.
The CLI, MCP face and dashboard expose that bus. It requires no external broker.

## Principles

- Credentials identify callers, and the bus enforces access before delivery
  ([access](02-access.md#what-a-call-carries), [ACL](02-access.md#acl)).
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
| [Trusted host](02-access.md#trust-boundary) | The daemon can read bodies and stored configuration |
| [Filtered waits](04-messaging.md#one-reader-per-inbox) | A simultaneous unfiltered reader can take an exchange's next reply between waits |
