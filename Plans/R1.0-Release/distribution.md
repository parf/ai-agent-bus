# Distribution

📌 **TL;DR:** Install with `npm install -g ai-agent-bus`, or `npm install -g ai-agent-bus-client` to join an existing bus. Both wrap the release binaries; no checkout or local Go build. The owner chose npm, without release signing, on 2026-10-30 ([RD178](DECISIONS.md#recorded-decisions)). Publication and remaining platform acceptance are pending.

## Why

- One install command for a laptop, a server or a team’s nodes.
- The installed package carries the programs and MCP assets it needs.

## Release artifacts

| Artifact | Contract |
|---|---|
| Installable npm packages | The full bus or client-only edition, with executable and MCP runtime assets; explicit runtime prerequisites and startup instructions |
| Ready-to-use Docker image | Pull and start without building locally; include a working message-delivery example |

### Two editions

| Edition | Contains | Install |
|---|---|---|
| Full | daemon, setup and admin tools, web face, runner, CLI, token helper, MCP face, launchers and bundled agents | `npm install -g ai-agent-bus` |
| Client-only | CLI, token helper, runner and shared starter, MCP face and launchers; no daemon or web face | `npm install -g ai-agent-bus-client` |

The client joins an existing bus with `AGENT_BUS_ADDR`, a token and, for a self-signed node, `AGENT_BUS_TLS_FINGERPRINT`. The full package depends on the client at exactly its own version; ship both together ([RD167](DECISIONS.md#recorded-decisions)).

## Simple install

Install the selected npm package, then use its commands. Installing the full package creates no service or account: explicitly run its setup from a root-owned Node executable and global prefix, as the [setup guide](../../docs/07-setup.md#npm-package) describes. A user-owned prefix is suitable for the client, never for sudo setup.

| Need | Contract |
|---|---|
| Runtime | Node 18 or later for the npm command shims; release binaries contain the TypeScript runtime, so no separate bun or Go; Python for Python agents; Linux sandbox prerequisites remain in the [runner guide](../../docs/runner/security.md#isolation-today) |
| Update | Install the selected package at the new shared version, then explicitly upgrade the node; preserve configuration, credentials and data |
| Platforms | Linux x64 and arm64 packages exist in the builder; add and prove macOS packages with [MAC.1](macos.md#mac1-client-and-runner-on-macos). Windows is [R1.1](../R1.1/windows-cli.md#windows-cli-client) |
| Other installers | `curl … \| sh` and the `install.sh` path are [deferred](FUTURE.md#shell-installer), not an R1 publication prerequisite |

## npm

Owner, 2026-10-30, superseding the earlier installer and signing choices ([RD178](DECISIONS.md#recorded-decisions)):

| Choice | Decision |
|---|---|
| Names | Unscoped `ai-agent-bus` for the bus and `ai-agent-bus-client` for the client; platform packages derive those stems, such as `ai-agent-bus-linux-x64` and `ai-agent-bus-client-linux-x64` |
| Account | The owner’s only npm account, `parf73`; no npm organisation and no `@agent-bus/` scope for published packages |
| Publication | Only after the owner gives the go. Workers do not run npm publish or npm login; acceptance uses local tarballs |
| Trust | npm’s registry integrity; the wrapper builder also checks release archive checksums and each archive’s manifest |
| Signature, key and spare key | None: no release signature, owner-held release key or attestation |
| Build source | Actions builds stamped full/client archives on a version tag and publishes the GitHub release with `SHA256SUMS`; npm wrapping uses those artifacts, not a separate source build |
| Release path | Verify each HTTPS-downloaded archive against that release’s `SHA256SUMS`, then its `MANIFEST.sha256`; produce the platform and command packages with `npm pack`. No draft waiting for a signature |
| Layout | npm owns its package files and command shims; explicit full-node setup stages a checked release and uses the existing releases/current layout and rollback |

The installed command stays `agent-bus`. npm refused the earlier `agent-bus` package name as too similar to `agentbus`; the CEO reserved `ai-agent-bus` at placeholder version 0.0.1 on 2026-10-30. Read-only `npm view ai-agent-bus-client --json` returned E404 on the same date; the owner subsequently reserved it too. Both names are now reserved; the [first real publication step](TODO.md#distribution-acceptance) records the version floor. Workers reserve or publish nothing.

`src/npm/pack.py` emits the split platform and command manifests; T-16’s separate implementation commit changes its defaults to these names, preserving the installed commands. The three tracked `@agent-bus/` manifests are private development projects, not published packages. The exact manifest inventory and pending migration are [P.1](TODO.md#distribution-acceptance).

### Install acceptance

Use local `npm pack` tarballs on a clean supported host in a disposable container with npm, without the checkout, Go or bun. A private loopback registry may serve the tarballs for exact dependency resolution; no public registry write or login.

| Exercise | Must |
|---|---|
| Install only the full command package | Resolve its exact-version client and platform dependencies from the local tarballs; all installed commands and licenses are present |
| Explicit setup and start | Start the installed daemon and an installed agent, complete a request/reply, then call an MCP tool through the installed face |
| Client-only package, another account | Join the fixture node without installing a daemon or writing outside its prefix; complete a request/reply |
| Omit an executable or MCP runtime asset, repack | The corresponding installed-program or MCP assertion fails; a build error is not the mutant proof |
| Wrong archive checksum or incomplete manifest | Package assembly or setup refuses before installing unchecked files |
| Update | Keep configuration, data and credentials; preserve the setup rollback behavior |

Existing local checks are in [npm packaging](../../src/npm/README.md#local-checks). P.1 must record the clean-container result and the named assertion-red omitted-asset mutant. [Earlier curl-installer evidence](done/installer-acceptance.md#checks) is historical, not proof of this npm acceptance.

Artifacts follow the shared [versioning and build evidence](../../CLAUDE.md#versioning) and [licensing](../../CLAUDE.md#licensing) rules. Native Linux aarch64 build evidence is [recorded](done/p3-aarch64-mac-run.md#steps); remaining native install work is in the [Mac run list](mac-run-list.md#a-linux-aarch64).

## macOS

The owner’s Mac is the test host ([RD164](DECISIONS.md#recorded-decisions)); Apple code signing and notarization are not required ([RD165](DECISIONS.md#recorded-decisions)). The [macOS plan](macos.md#order) owns the supported parts. Native checks already recorded do not establish npm platform support: P.1 must package and exercise it.

## Container runtime


Target runtimes: Docker and Podman.

One image supports separate daemon and runner roles in separate containers,
preserving their [secret domains](../../docs/07-setup.md#the-two-accounts).

| Concern | Contract |
|---|---|
| Persistence | Mount the daemon's [storage](../../docs/07-setup.md#storage) on a persistent volume so replacing a container preserves registry and credentials |
| Access | People authenticate over the port with tokens; the local socket serves the container's own processes ([access](../../docs/02-access.md#what-a-call-carries)) |
| Registry | `docker.io/parf/agent-bus`, tagged with the release's VERSION and `latest` |
| Sandboxing | The container is the boundary; document that script sandboxing is off. Explicitly requesting unavailable sandboxing must fail under the existing [sandbox contract](../../docs/06-dedicated-runner.md#sandboxing) |

The later [catalogue image](../R1.2/image.md#the-image) extends this distribution.
