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

What the owner's Mac runs to finish the aarch64 evidence: [Mac run list](mac-run-list.md#a-linux-aarch64).

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

Settled with the owner, 2026-10-29 ([RD163](DECISIONS.md#recorded-decisions)):

| Choice | Decision |
|---|---|
| Published from | the public `parf/ai-agent-bus` repository: `install.sh` through `raw.githubusercontent.com`, archives as its GitHub Release assets; no separate repository |
| Signature | `ssh-keygen -Y sign` / `verify` with one release ed25519 key, its own namespace; one signed `SHA256SUMS` per release; the public key embedded in `install.sh`, room for the next key; no `ssh-keygen -Y` on the host means no install |
| Key | held by the owner off this node, passphrase-protected; never in the repository or CI; the owner signs before publishing |
| Build | GitHub Actions builds linux x86_64 and aarch64 on a version tag into a draft release, unsigned; if that does not work out, a build script on this node (owner: "попробуем") |
| Layout | as `agent-bus-setup` today: `/usr/local/lib/agent-bus/releases/<version>/`, an atomic `current` link, links in `/usr/local/bin`, rollback by switching the link; `--client` the same under `~/.local` |

### Install acceptance

`src/acceptance/distro/run.sh [--dist <dir>] [almalinux10 ubuntu2604 debian13]`
builds this host's CPU's archives (or takes them), signs `SHA256SUMS` with a
throwaway key and fills that key into a copy of `install.sh`. It serves the
release over HTTPS as `releases.test` inside one rootless podman systemd
container per distribution. In each, the person `alice` runs
`curl … | sh` with sudo:

| Step | Must |
|---|---|
| clean host | no Go, bun or Node, nothing installed |
| tampered archive, signed hash unchanged | refused, host untouched |
| forged `SHA256SUMS` for that archive | refused by the signature, host untouched |
| good release | daemon and compiled web face running at the release's version |
| runner, debug registered and handed its token | `src/check-deploy.sh` passes |

Containers and their image store live under the main checkout's
`tmp/distro-acceptance`. Containers and each run's data are removed afterward,
and `--clean` drops the store. A warm run takes about 25 s for all three.

<details><summary>Mutants seen to fail, and findings</summary>

| Mutant | Fails |
|---|---|
| `install.sh` ignores the signature | the forged release installed |
| `install.sh` ignores the signed hash | the tampered release is refused only by the root re-check, not for its hash |
| `install.sh` creates `/usr/local/lib/agent-bus` before verifying | the refused release changed the host |
| `agent-bus-ts` in the archive replaced by a stub | the compiled web face does not answer |

Findings from the first runs, 2026-10-30:

| Finding | Where it bites |
|---|---|
| the system runner's default sandbox needs `bubblewrap`, which the install needs do not list | every distribution: debug is refused until it is installed |
| RHEL's sudo `secure_path` omits `/usr/local/bin` | AlmaLinux: `sudo agent-bus-setup` is not found, so the harness calls it by path |
| debug serves itself, so its Owner registers it and hands the runner its token; [debug § installing it](../../docs/agents/debug.md#installing-it) says a reload grants it | any node following that page |
| `GOARCH=arm64 src/package.sh` fails on x86_64 (`gspt` needs cgo) | aarch64 archives come only from a native runner |

</details>

## macOS

Owner, 2026-10-30 ([RD164](DECISIONS.md#recorded-decisions), [RD165](DECISIONS.md#recorded-decisions)):

| Choice | Decision |
|---|---|
| Test host | a Claude session runs on the owner's Mac, on the same LAN; this node has no ssh to it. The session does what the Mac run list says and reports back over the bus |
| Signing | macOS binaries are unsigned: no paid Apple Developer account, no notarization |
| Parts | the full set in three steps, cheapest first, without a sandbox at first ([RD166](DECISIONS.md#recorded-decisions), [macOS plan](macos.md#order)) |

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
