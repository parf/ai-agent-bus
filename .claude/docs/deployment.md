# Build and deploy from a checkout

📌 **TL;DR:** Maintainer procedures for build stamps, component deployment and development-node maintenance. Installed-product users start with [setup](../../docs/07-setup.md#install).

### Build information

Building the daemon and CLI needs cgo and a C compiler for
[processes § process titles](../../docs/08-processes.md#process-titles).

- `src/build.sh [output-directory]` builds all Go programs with one stamp; the output directory is relative to `src/` and defaults to that directory.
- The build supplies `build_info` as `user@hostname YYYY-MM-DD HH:MM:SS`, using the builder's local time, through Go linker flags; `release.sh` appends the commit's short SHA.
- It never rewrites source.
- Every Go program's `--version` prints the shared SemVer on the first line and `build_info: <stamp>` on the second; no daemon connection or privileges are needed.
- `--help` on each Go program opens with `<program> <version>`, on stdout, exit 0; bare `agent-bus` prints the same text on stderr and exits 1.

A direct `go build` reports `development (unstamped)` instead of inventing a
build date. The interpreted MCP and web faces report the shared SemVer only,
read from `VERSION`.
Release builds follow [working rules § versioning](../../CLAUDE.md#versioning).

#### Release evidence

- `src/release-verify.sh <archive>...` checks release archives against the source: the archive name, packed `VERSION` and Go programs’ versions agree, build stamps and `LICENSE.md` are present, and `MANIFEST.sha256` matches.
- Foreign-CPU binaries are read rather than run.
- The compiled TypeScript binary answers `mcp --version` and `launcher claude --version` with the release version where it can run, and carries a build stamp; the `agent-bus-token` wrapper answers its version too.

- The release build supplies `SHA256SUMS`; npm assembly checks each downloaded archive against it and then checks the archive’s manifest.
- Release signatures and signing keys are not part of installation or publication.
- Both native Linux builds verify their archives before upload.
- The release job verifies the complete unsigned set before publishing archives and `SHA256SUMS`; `--signers` is refused.
- There is no signing step or owner key.

### Development node cleanup

The development node has two weekly timers. Their root-owned script is
`/usr/local/libexec/agent-bus-node-cleanup.py`, installed from
`src/maintenance/node-cleanup.py`. Both take the shared gate lock; a busy gate
makes maintenance skip that run. They never delete session records.

| Scope | Bound | Job |
|---|---|---|
| parf's Go build cache | clean above 10 GiB | user `agent-bus-cache-cleanup.timer`, Sunday 04:00 |
| checkout `tmp/cargo-target` | clean above 5 GiB | same user timer |
| release directories | current plus one newest other release | root `agent-bus-node-cleanup.timer`, Sunday 05:00 |
| stopped-daemon backup archives | newest three; matching unit sidecars follow their archive | same root timer |
| record export archives in `/var/lib/agent-bus/record-backups/` | newest five full and ten partial; each `.info` follows its archive | same root timer |
| our ABRT entries | 100 MiB total; oldest removed first | same root timer; other applications untouched |
| attributable test `/tmp` directories | older than one day, not referenced by a live process | same root timer |
| daemon log files | daily rotation, 10 MiB early rotation, three archives | `/etc/logrotate.d/agent-bus` |

- `/tmp` attribution requires parf ownership plus a bus executable, a Go import of this module, or a test function name from this checkout.
- Symlinks are not followed.
- Unrelated caches, crash dumps and temporary files stay untouched.
- Runner and standalone agents use the shared system journal, not private log files; its host retention policy remains in force.
- Maintenance does not vacuum other applications' journal entries.

- Install or refresh the root-owned script and unit files from `src/maintenance/`.
- Enable the cache timer with `systemctl --user enable --now agent-bus-cache-cleanup.timer` and the root timer with `sudo systemctl enable --now agent-bus-node-cleanup.timer`.
- Inspect them with `systemctl --user list-timers` and `sudo systemctl list-timers`.
- Caches rebuild on demand.
- The thresholds are weekly high-water checks, not hard quotas between runs; logrotate checks its size bound on its daily run.

### Post-deploy check

- Run `src/check-deploy.sh` after each deploy; its optional target overrides the default `#debug@<realm>` derived from `agent-bus status` (or the host name if no realm can be read).
- For a specific instance, use `src/check-deploy.sh '#debug@<node>'`, as a caller allowed by debug’s `@owner` ACL.
- A failed or missing selftest fails the check; see [the bus doctor](../../docs/agents/debug.md#check-the-bus-in-a-minute).

### Deploy only the changed component

Commit before deployment. A shared version bump alone does not require
rebuilding or restarting unrelated components. Keep existing ACLs,
Configuration and Secrets; use read-only probes unless the task asks for a post
or an operation.

| Changed component | Minimum deployment | Verify |
|---|---|---|
| Web TypeScript | Restart `agent-bus-web`; no Go build or daemon restart | The affected signed-in page and handler |
| Runner | Stamped `src/build.sh`, restart its `agent-bus-runner@<person>` unit | Runner status, the affected agent and `src/check-deploy.sh` |
| Slack, Telegram or debug | Run that agent's `package.sh` into its installed `agent.d` directory, then call its runner's `reload`; a shared helper changes each bundle that includes it | The affected agent's `whoami` or `ping`, its new behaviour and process restart |
| Standalone Python | Restart its unit; reinstall only when its unit arguments or installer changed | A read-only method and preserved ACL |
| Daemon | Back up the stopped daemon home, stamped `src/build.sh`, restart `agent-busd` | Reconnection, the changed API/authority path and `src/check-deploy.sh` |
| Documentation | Link and accuracy checks, then commit | No version bump, mutant, build or restart |

- Packagers at `src/agents/<name>/package.sh` replace their public code directory as a whole, removing files the package no longer ships.
- Keep cursors, caches and other runtime state in the runner’s work directory, outside `agent.d`. A failed build leaves the previous code tree intact.
- Reload the runner after packaging an enabled agent. A running process may otherwise fail when it imports a file from the replaced tree.
- A forced kill or power loss between the two directory moves can leave the installed directory missing. The previous tree remains in a sibling `<name>.old.XXXXXX/previous` directory; restore it to the agent’s installed path before reloading.
- The next packaging run does not sweep leftover `<name>.new.XXXXXX` or `<name>.old.XXXXXX` directories. Once the installed tree is verified and no packager is running, remove those leftovers; keep any `previous` tree needed for recovery.
- The standalone tmux-sessions packager uses the same replacement helper, shipped beside it in the release, so it also removes obsolete files when repackaging on an installed host.

The [build stamp](deployment.md#build-information) is required for shipped Go binaries.
A live Git checkout is never a reset/clean/push/autopull test fixture.

### From a checkout

Use the contributor [checkout installation procedure](../../src/INSTALL.md#from-a-checkout).
