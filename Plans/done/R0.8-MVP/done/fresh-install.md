# Fresh installation

📌 **TL;DR:** 0.5.68 ships one verified archive whose setup script installs a
complete release and starts a usable daemon plus dashboard on a fresh host.

## Result

`src/package.sh` assembles all six Go programs, the MCP face, runtime adapter,
three launchers, license and standalone instructions. The package manifest is
an exact allowlist; setup verifies every hash, executable bit and shared version
before changing accounts, state, the unit or selected release.

Releases live under `/usr/local/lib/agent-bus/releases/<version>-<digest>`.
Stable commands point through `current`; setup prepares all links before one
atomic symlink rename selects the complete release. Re-running an identical
archive reuses it. A failed first install may leave only unselected release
data or dangling stable links, and the documented recovery is to rerun setup.

The generated unit now starts the dashboard. `--exec` remains the explicit
source-build and package-less acceptance bypass; it does not install a bundle.

## Checks

A rootless Podman fixture runs a real systemd 260 PID 1 with unified cgroup v2
and bubblewrap. Its runtime network is disabled and it mounts the archive,
fixture driver and evidence directory only — no checkout or `/rd`.

The measured fresh-host run passes 8/8 checks on systemd 260.1:

- verifies the archive checksum;
- removes `agent-bus-web`, `mcp/server.js` and `launchers/launcher.js`
  separately and proves each setup fails before accounts, state, unit or
  `current` exists;
- installs the valid archive, starts the generated systemd unit and reads the
  public API plus dashboard;
- verifies six stamped programs, three stable launcher commands, both bundled
  JavaScript faces, the installed manifest and absence of build-host paths;
- follows packaged instructions to register and call a script service, checking
  its unique reply; and
- reruns the identical package and observes the same selected release and one
  release directory.

The first exploratory valid run receives no credit: the staging directory kept
`MkdirTemp`'s `0700`, so systemd correctly refused to execute the daemon as its
service account. The fixed source sets `0755` before the release rename and a
unit test pins it. A second run used a nonexistent `/healthz`; the harness and
instructions now use the built public `/identity`. A third launched the sample
under the prepared runner account although current foreground services run as
the invoking user. A fourth omitted the explicit reply-inbox grant required by
the restrictive ACL default. Each red is retained in `tmp/` without credit.
Two later probes of the service account also receive no credit: `ps` truncated
the account name, and a `/proc` ownership check sampled the process while
systemd was still changing identity. The final check waits for `/identity` and
reads the unit's declared and running user after readiness.

Ten targeted source mutations fail independently: omitting the MCP face,
loosening the exact manifest, ignoring executable bits or hashes, conflating
same-version bundles, skipping the link preflight, retaining the private stage
mode, trusting an existing damaged release, switching `current` non-atomically,
and omitting the dashboard from the generated unit. The fast repository run is
488/0. The slow repository run passes 609/0 with all 24 frozen candidate-file
hashes unchanged.

## Limits

The manifest and adjacent archive checksum detect incomplete or damaged
delivery; they are not a package-signing system. The fixture image supplies the
documented host prerequisites and does not prove every Linux distribution.

The digest-addressed layout prepares H.1.1; this result does not accept upgrade,
rollback or populated-state recovery. It includes runtime integration artifacts
without exercising runtime CLIs, so it does not close H.8, H.9 or their live
gates. Starting the dashboard is only F.12's prerequisite, not installed browser
journey acceptance.
