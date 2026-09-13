# MVP

## Purpose

Somebody other than the author can install it and use it safely on a shared host.
MVP is active; completion still depends on [remaining work and installed acceptance](TODO.md#objective).

## Scope

[Current docs](../../docs/00-overview.md#document-ownership) own the MVP contracts and distinguish built behavior from pending requirements. This table tracks stage coverage, not completion of release acceptance.

| Area | Status | Canonical contract |
|---|---|---|
| Identity, credentials and local isolation | Built; person profiles and maintainer policy pending | [identity](../../docs/01-identity.md#status), [access](../../docs/02-access.md#status) |
| Registry, topics and private configuration | Built; method metadata pending | [services](../../docs/03-services-and-topics.md#status) |
| Messaging and restart persistence | Built | [messaging](../../docs/04-messaging.md#status) |
| API, CLI, MCP and dashboard | Built; generated method information and people view pending | [discovery](../../docs/05-discovery.md#status) |
| Foreground services and adapters | Built | [runner](../../docs/08-runner-role.md#status) |
| Installation and service account | Setup built; package and installed acceptance pending | [setup](../../docs/09-setup.md#status) |
| Process isolation | Split built; installed capability and confinement checks pending | [processes](../../docs/11-processes.md#status) |

## Boundaries

The MVP trusts the bus with bodies ([access § encrypted sessions](../../docs/02-access.md#encrypted-sessions)). Distributed identity, managed services and encryption are proposed in [R1](../R1/README.md#scope). Other follow-up is indexed in [future work](FUTURE.md#follow-up).

## Evidence

[Completion log](DONE.md#done--mvp) records completed waves. [The original plan](done/TODO-before-rewrite.md#mvp-plan-before-the-documentation-rewrite) preserves task IDs and earlier acceptance detail; it is historical, not active work. Acceptance conventions live in [working rules](../../CLAUDE.md#verification).
