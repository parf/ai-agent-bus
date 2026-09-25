# Distribution

Status: required R1 scope, not built.

## Release artifacts

| Artifact | Contract |
|---|---|
| Installable npm package | Install the bus programs and MCP face from npm without a repository checkout or local Go build; document runtime prerequisites and startup |
| Ready-to-use Docker image | Pull a published image and start the bus using the supplied instructions, without building locally; include a working message-delivery example |

Artifacts follow the shared [versioning and build evidence rules](../../CLAUDE.md#versioning) and [distribution licensing rules](../../CLAUDE.md#licensing). Package names, image registry and supported platforms are chosen during implementation.

## Container runtime

Target runtimes: Docker and Podman.

One image supports separate daemon and runner roles in separate containers,
preserving their [secret domains](../../docs/09-setup.md#the-two-accounts).

| Concern | Contract |
|---|---|
| Persistence | Mount the daemon's [storage](../../docs/09-setup.md#storage) on a persistent volume so replacing a container preserves registry and credentials |
| Access | People authenticate over the port with tokens; the local socket serves the container's own processes ([access](../../docs/02-access.md#what-a-call-carries)) |
| Sandboxing | The container is the boundary; document that script sandboxing is off. Explicitly requesting unavailable sandboxing must fail under the existing [sandbox contract](../../docs/08-runner-role.md#sandboxing) |

The later [catalogue image](../R1.1/image.md#the-image) extends this distribution.
