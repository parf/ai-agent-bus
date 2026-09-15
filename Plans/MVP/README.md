# MVP

## Purpose

Somebody other than the author can install it and use it safely on a shared host.
MVP is active; completion still depends on [remaining work and installed acceptance](TODO.md#objective).

## Getting tokens

![A user obtains a token through an authenticated local socket, an entitled SSH key or key-possession proof. A launching runner uses the owner's credential to register a service and obtain its separate token, then switches to the service identity.](getting-tokens.svg)

User token acquisition follows [access](../../docs/02-access.md#getting-a-token). Ownership authorizes service-token acquisition ([token scope](../../docs/02-access.md#token-scope)); the foreground runner obtains that credential before serving ([script services](../../docs/08-runner-role.md#script-services)).

## User to service

![User sends a request with their token to the bus; the service reads with its own token and receives the verified sender and message, without the user's token.](user-to-service.svg)

The daemon authenticates the caller and enforces the destination's access policy before delivery ([token scope](../../docs/02-access.md#token-scope), [ACL](../../docs/01-identity.md#acl)). The service receives the message through its authenticated inbox read ([delivery](../../docs/04-messaging.md#push-and-pull)); replies use the [reply route](../../docs/04-messaging.md#reply-routing).

Mapped local accounts can authenticate through their [own socket](../../docs/02-access.md#local-socket). The diagram's plaintext boundary is defined in [access](../../docs/02-access.md#encrypted-sessions).

## Scope

[Current docs](../../docs/00-overview.md#document-ownership) own the MVP contracts and distinguish built behavior from pending requirements. This table tracks stage coverage, not completion of release acceptance.

| Area | Status | Canonical contract |
|---|---|---|
| Identity, credentials and local isolation | Built and pending; see linked status | [identity](../../docs/01-identity.md#status), [access](../../docs/02-access.md#status) |
| Registry, topics and private configuration | Built | [services](../../docs/03-services-and-topics.md#status) |
| Messaging and restart persistence | Built | [messaging](../../docs/04-messaging.md#status) |
| API, CLI, MCP and dashboard | Built and pending; see linked status | [discovery](../../docs/05-discovery.md#status) |
| Foreground services and adapters | Built, including launchers; live-runtime and fresh-host acceptance pending | [runner](../../docs/08-runner-role.md#status) |
| Installation and service account | Setup built; package and installed acceptance pending | [setup](../../docs/09-setup.md#status) |
| Process isolation | Split built; installed capability and confinement checks pending | [processes](../../docs/11-processes.md#status) |

## Boundaries

The MVP trusts the bus with bodies ([access § encrypted sessions](../../docs/02-access.md#encrypted-sessions)). Distributed identity, managed services and encryption are proposed in [R1](../R1/README.md#scope). Other follow-up is indexed in [future work](FUTURE.md#follow-up).

## Evidence

[Completion log](DONE.md#done--mvp) records completed waves. [The original plan](done/TODO-before-rewrite.md#mvp-plan-before-the-documentation-rewrite) preserves task IDs and earlier acceptance detail; it is historical, not active work. Acceptance conventions live in [working rules](../../CLAUDE.md#verification).

## Web redesign

The [web interface proposal](web-interfaces.md#proposal) covers requirements, page structure, data gaps and Go tooling, grounded in the [browser and source review](done/web-review.md#scope). It is proposed, not implemented; if accepted, [F.13 work](TODO.md#web-redesign) precedes final browser acceptance.
