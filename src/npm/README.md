# npm packaging

📌 **TL;DR:** Install the full bus with `npm install -g ai-agent-bus`, or join an existing bus with `npm install -g ai-agent-bus-client`. Build the packages from checked release archives and test local tarballs without publication. Only the owner authorizes a public npm release.

The client package contains the CLI, runner, MCP face and launchers. The bus
package adds the daemon, setup and admin, and depends on the client at the exact
same version. They ship together after the owner’s go; workers run neither npm publish nor npm login. This tooling never publishes. Package
names have one default map in `pack.py`, with `NPM_BUS_NAME` and
`NPM_CLIENT_NAME` overrides. Platform package names derive from those names.

Use the ordinary stamped full and client archives from the GitHub build release, downloaded over HTTPS with its `SHA256SUMS`. Verify those checksums and the manifests before wrapping; release signatures and signing keys are not required. The wrapping command is:

```sh
src/npm-pack.sh FULL.tar.gz CLIENT.tar.gz SHA256SUMS OUTPUT
```

The builder checks the archive hashes and each archive's manifest, and records
the `SHA256SUMS` hash in every npm package. It emits a client platform package,
a bus platform package, and two thin command packages. Both thin packages pin
their platform dependencies. Linux x64 and arm64 are declared; the release set must include matching platform artifacts for both before the owner authorizes publication of the command packages.
macOS has the client edition only: `pack.py ... --darwin-client agent-bus-client-<version>-darwin-x86_64|aarch64.tar.gz` (one per CPU, checked against `SHA256SUMS`, the same version and the Linux client's file list) adds `<client>-darwin-arm64` and `<client>-darwin-x64` platform packages and lists them in both command packages, so `agent-bus` resolves on a Mac; the daemon commands there refuse with `unsupported platform darwin-<cpu>`. No Darwin package is declared before its archive is built. Thin packages built from the
same complete `SHA256SUMS` have the same content on either architecture.

The bus platform contains the full release's files absent from the client,
plus its full manifest. The large client binaries are stored only once. Bus
command shims expose the transitive client's commands because npm does not put
transitive dependencies' bins in a global install's bin directory.

Installation trusts npm’s registry integrity. The wrapper builder verifies the release checksums and manifests; there is no release-signing stage. The owner chose `ai-agent-bus` / `ai-agent-bus-client` while the command stays `agent-bus`, and the account `parf73`, without an npm organisation. Private development `@agent-bus/` projects remain unpublished.

## Explicit setup

No lifecycle hook runs at installation. `agent-bus-setup` requires an explicit
sudo invocation for a real installation from a root-owned global prefix.
The Node executable, package directories, verifier, command shim, release files and every parent
must be root-owned and not group/other-writable; unsafe paths are named in the
refusal. Do not sudo a command from nvm or another user-owned npm prefix: a
replacement verifier could bypass its own checks. The administrator must
install into the trusted prefix before invoking sudo setup.
Only a sole help or version argument is informational; combining it with setup
arguments still requires all installation guards and manifest staging.
It checks client/bus package and
platform versions, creates a root-owned 0700 temporary directory under `/run`
outside the npm prefix, and copies the split files according to the full
manifest. Every copied byte is checked before the packaged setup program runs.
The unchanged installer then installs into the ordinary releases/current tree.
The staging directory is removed on success, failure and handled termination.

## Local checks

`src/npm-pack.test.sh` builds archives and tests installing **only the bus**
from a private loopback fixture registry. `NPM_DIST` reuses an existing directory
of archives. Test reports cover the exact pins, installed commands, build
stamps, license/provenance, request/reply, MCP and clear missing-platform
refusal. All prefixes and registry state are temporary; nothing is published.
`src/npm/container-check.sh <tarball-directory>` exercises explicit setup on a clean systemd host with npm but without Go or bun, using only local `npm pack` tarballs. The [P.1 acceptance](../../Plans/R1.0-Release/distribution.md#install-acceptance) requires request/reply, an MCP tool call and assertion-red omitted-executable and MCP-asset mutants. The fixtures do not require npm publication or login.

## Clean-install acceptance

`src/acceptance/npm-clean-install.sh` is the P.1 acceptance, on local tarballs only. `src/npm/acceptance.py` runs on the host: an unchanged release passes the binary and MCP-tool-call assertions and installs exactly the bytes its manifest lists; a release without an executable, or without the compiled MCP face, still wraps and installs and fails its own runtime assertion; a corrupt archive, an altered file and a file the manifest does not check are each refused before any package is built. The container half (`container-check.sh`) adds an installed agent that answers a call and the client package alone installed by a second account, which has no daemon or setup command, starts no daemon, writes only under its prefix and npm cache, and calls the installed agent over the running bus. The container half takes the smoke-gate lock like any podman run.

`src/npm/darwin_check.py` is the macOS build check: names, `os`, `cpu` and files of the two platform packages, their pins in both command packages, the launcher resolving `agent-bus` as macOS arm64 from an `--os=darwin` install, and the refusal of a mismatched archive. `src/npm-pack.test.sh` runs it after `check.py`, so the normal gate does (about 8 s of its own: it packs stand-in archives, not the real ones, and the Mac run covers the real binaries). The Mac acceptance steps are in the [P.1c plan](../../Plans/R1.0-Release/TODO.md#distribution-acceptance).
