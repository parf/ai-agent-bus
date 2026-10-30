# Earlier installer acceptance

History only: the curl installer checks below predate the owner’s 2026-10-30 npm decision. They are not the R1 installation or signing contract. Current work is [npm acceptance](../distribution.md#install-acceptance); the installer is [deferred](../FUTURE.md#shell-installer).

## Checks


`src/acceptance/distro/run.sh [--dist <dir>] [--next-dist <dir>] [almalinux10 ubuntu2604 debian13]`
builds this host's CPU's archives (or takes them), and a "next" release from a
copy of the tree with VERSION one patch up. It signs `SHA256SUMS` with a
throwaway key and fills that key into a copy of `install.sh`. It serves the
release over HTTPS as `releases.test` inside one rootless podman systemd
container per distribution. In each, the person `alice` runs
`curl … | sh` with sudo:

| Step | Must |
|---|---|
| clean host | no Go, bun or Node, nothing installed |
| tampered archive, signed hash unchanged | refused, host untouched; without bubblewrap, a warning |
| forged `SHA256SUMS` for that archive | refused by the signature, host untouched |
| good release | daemon and compiled web face running at the release's version |
| runner, debug registered and handed its token | `src/check-deploy.sh` passes |
| `bob`, another account, `--client` without root | nothing outside his home changes; his `debug` call is refused until allowed, then answered |
| bob's compiled `agent-bus-mcp`, tool `ab_ls` | lists debug |
| the same command, next release out | updated, one copy of the old release kept; check-deploy and bob's token still work; setup's `/etc/agent-bus` files, the runner's `runner.env` and a record's Configuration unchanged |
| bob updates the client, then `--rollback` | next, then the old version again |
| `--version <old>` | the node back on the old release, check-deploy passing |

Containers and their image store live under the main checkout's
`tmp/distro-acceptance`. Containers and each run's data are removed afterward;
a run killed by any signal, `SIGKILL` included, is cleaned up by its watcher or,
at the latest, by the next run ([J32](../../issues/2026-10/2026-10-30.md#fixtures)).
Containers log to capped files in the run directory, never the host journal.
`--clean` drops the store. A warm run with both releases built takes about 90 s for all three. It holds the `smoke-gate` lock (guard.sh refuses without it) and runs as smoke check `distro_install` in the slow set only: `--slow` for install, setup and distribution paths, `--slow all` for a release.

<details><summary>Mutants seen to fail, and findings</summary>

| Mutant | Fails |
|---|---|
| `install.sh` ignores the signature | the forged release installed |
| `install.sh` ignores the signed hash | the tampered release is refused only by the root re-check, not for its hash |
| `install.sh` creates `/usr/local/lib/agent-bus` before verifying | the refused release changed the host |
| `agent-bus-ts` in the archive replaced by a stub | the compiled web face does not answer |
| `agent-bus-ts` dropped from the full archive, manifest remade (2026-10-30) | setup refuses it: `package is incomplete: agent-bus-ts is not in MANIFEST.sha256`; the good release did not install |
| `agent-bus-ts` dropped from the client archive only, manifest remade | refused before anything is written: `is incomplete: agent-bus-ts is missing`; before that fix, the MCP tool call failed |
| the client completeness loop emptied | the incomplete client archive installed (`TestInstallShRefusesAnIncompleteClientArchive`, three cases) |
| setup's upgrade appends to `runner.env` (next release from a copy) | the update changed the node's configuration files |
| the next daemon reads every Configuration as `{}` | the update changed `#kept`'s Configuration |
| `install.sh` without its bubblewrap warning | install.sh did not warn that bubblewrap is missing |
| `install.sh --client --rollback` switches to the current release | the client edition is not the old version after its rollback |
| setup's upgrade deletes the release it replaced | the update kept 0 copies of the old version for rollback |
| setup's root hints keep the bare name (`TestSudoLineNamesSetupByItsPath`) | the hint lacks setup's path |
| root's re-hash of its copy removed | an archive swapped after the user's check reaches root (`TestInstallShFullNodeRefusesAnArchiveSwappedBeforeRootCopiesIt`) |
| root's manifest or top-directory check removed | a signed archive of the wrong shape reaches setup (`TestInstallShRefusesASignedArchiveOfTheWrongShape`, root cases) |
| the client's manifest or top-directory check removed | the same archive installs as the client edition (its client cases) |
| `--rollback` without the command-name preflight | a person's own `~/.local/bin/agent-bus` is replaced (`TestInstallShClientInstallsUpdatesAndRollsBack`) |

Findings from the first runs, 2026-10-30:

| Finding | Where it bites |
|---|---|
| the system runner's default sandbox needs `bubblewrap` | every distribution: debug is refused until it is installed; the install Needs now list it and install.sh warns |
| RHEL's sudo `secure_path` omits `/usr/local/bin` | AlmaLinux: a bare `sudo agent-bus-setup` is not found; the docs and setup's hints now give the path |
| debug serves itself, so its Owner registers it and hands the runner its token; [debug § installing it](../../../docs/agents/debug.md#installing-it) said a reload grants it | any node following that page; corrected |
| `install.sh --client` installed a client archive without `agent-bus-ts`: unlike setup, it had no completeness check (2026-10-30) | fixed: the client edition requires every command it links, in the release and its manifest, before writing anything (`TestInstallShRefusesAnIncompleteClientArchive`) |
| `GOARCH=arm64 src/package.sh` fails on x86_64 (`gspt` needs cgo) | aarch64 archives come only from a native runner; the Mac run list builds one |

</details>
