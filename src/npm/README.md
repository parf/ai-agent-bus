# npm packaging

The client package contains the CLI, runner, MCP face and launchers. The bus
package adds the daemon, setup and admin, and depends on the client at the exact
same version. Publish both together; this tooling never publishes. Package
names have one default map in `pack.py`, with `NPM_BUS_NAME` and
`NPM_CLIENT_NAME` overrides. Platform package names derive from those names.

Build the ordinary full and client archives first. Verify the release's signed
`SHA256SUMS` through the release process before wrapping it:

```sh
src/npm-pack.sh FULL.tar.gz CLIENT.tar.gz SHA256SUMS OUTPUT
```

The builder checks the archive hashes and each archive's manifest, and records
the `SHA256SUMS` hash in every npm package. It emits a client platform package,
a bus platform package, and two thin command packages. Both thin packages pin
their platform dependencies. Linux x64 and arm64 are declared; publish the
matching platform artifacts for both before publishing the command packages.
No Darwin package is declared before it is built. Thin packages built from the
same complete `SHA256SUMS` have the same content on either architecture.

The bus platform contains the full release's files absent from the client,
plus its full manifest. The large client binaries are stored only once. Bus
command shims expose the transitive client's commands because npm does not put
transitive dependencies' bins in a global install's bin directory.

Installation trusts npm's registry integrity. The checksum provenance is not a
replacement for release signature verification; the signed GitHub archives
remain the verify-before-unpack route.

## Explicit setup

No lifecycle hook runs at installation. `agent-bus-setup` requires an explicit
sudo invocation for a real installation. It checks client/bus package and
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
The systemd-container check separately exercises explicit setup on a clean
host with npm but without Go or Bun.
