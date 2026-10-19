# Distribution

Status: required R1 scope, not built.

## Release artifacts

| Artifact | Contract |
|---|---|
| Installable npm package | Install the bus programs and MCP face from npm without a repository checkout or local Go build; document runtime prerequisites and startup |
| Ready-to-use Docker image | Pull a published image and start the bus using the supplied instructions, without building locally; include a working message-delivery example |

### Two editions

Owner, 2026-10-19: every installable form comes in two editions.

| Edition | Contains | For |
|---|---|---|
| **Full** | the daemon, setup and admin tools, web face, runner, CLI, token helper, MCP face, launchers and bundled agents | the node that hosts the bus |
| **Client-only** | the runner and the basic CLI (`agent-bus`, `agent-bus-token`), plus the shared starter for standalone agents | any other machine that joins an existing bus: it serves agents and calls the bus over HTTPS, with no daemon or web face |

The client-only edition needs only `AGENT_BUS_ADDR`, a token and, for a self-signed node, `AGENT_BUS_TLS_FINGERPRINT`; it never installs or starts `agent-busd`. Whether the MCP face and launchers belong in the client edition is still to choose.

Artifacts follow the shared [versioning and build evidence rules](../../CLAUDE.md#versioning) and [distribution licensing rules](../../CLAUDE.md#licensing). Package names, image registry and supported platforms are chosen during implementation.

## Container runtime

Target runtimes: Docker and Podman.

One image supports separate daemon and runner roles in separate containers,
preserving their [secret domains](../../docs/07-setup.md#the-two-accounts).

| Concern | Contract |
|---|---|
| Persistence | Mount the daemon's [storage](../../docs/07-setup.md#storage) on a persistent volume so replacing a container preserves registry and credentials |
| Access | People authenticate over the port with tokens; the local socket serves the container's own processes ([access](../../docs/02-access.md#what-a-call-carries)) |
| Sandboxing | The container is the boundary; document that script sandboxing is off. Explicitly requesting unavailable sandboxing must fail under the existing [sandbox contract](../../docs/06-dedicated-runner.md#sandboxing) |

The later [catalogue image](../R1.2/image.md#the-image) extends this distribution.
