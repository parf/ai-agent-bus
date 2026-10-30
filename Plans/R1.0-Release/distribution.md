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

## Simple install

Owner, 2026-10-29: one command is the main way onto Linux, with npm as a thin
wrapper over the same binaries ([RD162](DECISIONS.md#recorded-decisions)).

| Piece | Contract |
|---|---|
| `install.sh` | `curl -fsSL <published URL>/install.sh \| sh`; detects OS and CPU, downloads that platform's release archive from GitHub Releases, verifies its checksum and signature before anything is unpacked, installs it |
| Default | the **full node**: needs root (it asks for sudo), installs every program, then runs `agent-bus-setup` for the accounts, systemd units and TLS |
| `--client` | the client-only edition without root, into `~/.local`: CLI, runner, MCP face and launchers, to join an existing bus |
| Update | the same command again, keeping configuration and data; the previous version kept for rollback |
| Runtimes | Go programs as they are; web face, MCP face and launchers built with `bun build --compile` into self-contained binaries, so no bun or Node is needed; `python3` from the host for bundled agents |
| npm | `npm i -g agent-bus` and `npx agent-bus`: the client-only edition, the same binaries as per-platform optional packages |
| Later | `.deb` and `.rpm` packages with their repositories; macOS through the same script once the [Mac test host](TODO.md#objective) settles the macOS questions |

To propose for approval before code: the signature tool and key handling, the
published URL, and the archive layout.

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
