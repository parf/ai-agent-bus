# MAC.2 native daemon and reduced setup

📌 **TL;DR:** The daemon, setup and admin binaries build natively on Apple Silicon. The focused launchd, private-storage, TLS request/reply, restart and orphan checks passed on 2026-10-30. The initial `dscl` role deletion was refused; the owner later removed both roles with `sysadminctl`, and the CEO verified absence. Full archive validation remains with MAC.1.

## Source and limits

- Runtime source: `9e6cbea3`, stamped `0.9.214`, `build_info: MAC2-9e6cbea3`.
- Mac: darwin/arm64, Go **1.27.3**, Bun **1.4.4**, both from `/opt/homebrew/bin`. This node uses Go **1.26.5**; the version difference is recorded, not hidden. `GOTOOLCHAIN=local` prevented another toolchain download.
- One existing clone, `~/agent-bus-dev/ai-agent-bus`, was used. Missing Git objects were streamed into it, with no archive file or extra clone. Each run returned it to clean `main`, `7c6555aa`.
- `GOCACHE`, `GOMODCACHE` and Bun's cache were under `~/agent-bus-dev/.cache`. Build output, fixture keys and those caches were removed after each run. `~/go` and `~/Library/Caches/go-build` stayed absent.
- No Homebrew, profile, firewall or other system-setting change was made. All mutants ran on this node under `smoke-gate`, never on the Mac.

## Checks and results

| Check | Result |
|---|---|
| Native build of `agent-busd`, `agent-bus-setup`, `agent-bus-admin`, plus all `TestDarwin` setup checks | Passed, 13.158 s including cache cleanup and return to main. |
| Initial four-program build | The Go CLI fails on Linux-only `unix.TCGETS` in `help.go:104`; that is MAC.1's portability work. The MAC.2 run builds its three programs and uses the Python client. |
| Strict-umask parent-directory regression | Removing `walkableParents` fails an assertion: a fresh parent is `0700`, and the account cannot pass. Restored check passes. |
| First-user administration regression | Before the fix, the helper was given neither the Darwin role's home nor its socket, and the new assertion failed. The explicit handoff fixes its Linux-account fallback; native first-user enrolment passed. |
| Launchd identity and storage | Daemon runs as `_agentbusd`, never root; hidden account, `/usr/bin/false`, private `0700` storage, and the installing user cannot read it. |
| Pinned TLS request/reply | One request consumed once; its reply is `pong` from the registered agent's identity. No live provider call. |
| Launchd restart | Registry entry remains after `kickstart -k`. |
| Supervisor killed outright | A replacement supervisor returns, the old bus child is gone, and the registry entry remains. |
| Uninstall | Job booted out; plist, home, sockets and test binary tree removed. User deletion failed as described below. The whole install run returned nonzero, not a green cleanup result. |

The native install and teardown run took **34.075 s**. It used `--exec` with the three focused binaries, not a full release archive. Archive construction and installation still depend on MAC.1/distribution acceptance; this result does not claim that artifact check.

## Node mutation evidence

Under confirmed 15-minute `smoke-gate` holds, twelve setup mutants compiled and failed an owning assertion: job as root, missing job account, public storage, closed socket directory, database initialization as root, a login shell, a password field other than `*`, visible account, open umask, missing TLS, accepted Linux-only flags, and omitted walkable parents. Restored `TestDarwin` checks pass.

A real private daemon on this node registers an entry, stops and restarts, and finds the entry intact. Deleting only that fixture database before restart gives the named assertion `registry lost across restart`. It uses the same registry/store path as the native Mac daemon; no mutation ran on the Mac. The first-user handoff check also failed on the old call and passes with the explicit home and socket.

## Admin defaults follow-up

`fe9d8728` adds `TestInstalledAdminDefaultsAreForThisOS` before the fix. On the Mac it fails an assertion: the account is `agent-busd` and the socket `/run/agent-bus/bus.sock`, rather than the Darwin defaults. That focused run took 11.802 s including cleanup.

`53ae56f4` selects `_agentbusd`, `/usr/local/var/agent-bus/daemon` and the platform's installed socket. The credential hint uses the local sudo/token form on macOS; Linux keeps its existing SSH hint and defaults. Native account/socket, home-default and hint checks pass without sudo, 11.942 s including cleanup. The full Linux admin test package also passes.

These follow-up runs return the clone to clean `main`, `7c6555aa`; the workspace is now **54M** because of the additional Git objects. Build output and `.cache` were deleted, and `~/go` and `~/Library/Caches/go-build` remain absent. No sudo command was added after the owner took over role cleanup. The focused fast `go_checks` passes in 5.074 s. After the final main rebase it passes in 6.940 s; the initial full fast run was 25 passed, 1 failed because this worktree lacked the TypeScript client dependency link. After linking all three dependency directories as required by the worker role, the complete fast run passes 26/0 in 7.844 s.

## Owner cleanup

`sudo -n dscl . -delete /Users/_agentbusd` returned exit 40, `eDSPermissionError` / `-14120`. The owner took over; the CEO and director instructed the worker to run no more sudo on the Mac. The verified final result is below.

State at that handoff was the `_agentbusd` user and group, UID/GID **206**, plus empty setup parent directories. The job, plist, daemon home, socket directory and test binary tree are gone. This does not affect MAC.1's client/runner checks; W1B was told `mac free`.

The owner also saw the same `dscl` refusal at the Mac console. The acceptance cleanup now deletes the user with `sysadminctl -deleteUser` and the group with `dseditgroup -o delete`, rather than `dscl`. It checks both and verifies their absence with directory listings, including when a deletion command returns zero. A leftover entry or unreadable listing exits nonzero with the role names and [Full Disk Access recovery](../../../docs/07-setup.md#uninstall-the-macos-test-daemon). This records the initial handoff; the owner cleanup result is below; no worker sudo was run for this fix.

`python3 src/acceptance/test_mac2_cleanup.py` runs only local subprocess stubs. Four checks cover successful removal, already absent entries, both deletion failures, a zero-status deletion leaving both entries, an unreadable directory and a group-only failure. On the old cleanup, five assertions fail, including the unreadable-directory case incorrectly printing `PASS uninstall`; all pass after the fix. The harness is now committed under `src/acceptance/`; the node-local copy is kept in sync. Under the `smoke-gate` lock, three independent cleanup mutants also fail assertions: restoring `dscl` user deletion, trusting a zero deletion status without checking directory presence, and skipping group deletion. Restored checks pass; the lock is released.

### Owner cleanup verified

At 17:23 on 2026-10-30 the owner ran `sudo sysadminctl -deleteUser _agentbusd` at the Mac console. It removed both the user and group without Full Disk Access. The CEO verified read-only: `dscl -read` on both `/Users/_agentbusd` and `/Groups/_agentbusd` returned `eDSRecordNotFound` (`-14136`), and `id _agentbusd` reported no such user. The earlier refusal therefore does not establish that Full Disk Access is required for `sysadminctl`.

The cleanup treats a group already removed by user deletion as success and skips `dseditgroup`. If a group disappears between lookup and deletion, a nonzero `dseditgroup` status also succeeds after verified absence. The new disappearance stub fails on the prior code's exit-status check; all six stub checks pass with absence deciding success. No worker Mac command or sudo was run for this follow-up. Full Disk Access remains a conditional recovery step for a persistent permission refusal, not a prerequisite.

### Supervisor startup review

The second reader reproduced a HIGH in the non-Linux watcher: if the supervisor died before `watchParent` captured `Getppid`, the child watched its new parent and retained the inherited listeners. The spawning supervisor now supplies `AGENT_BUS_SUPERVISOR_PID`, replacing any inherited value. The child verifies that parent before startup and watches the same PID afterward; a missing or invalid expected PID refuses startup.

`TestParentWatchReleasesListenersBeforeAndAfterStartup` exercises the actual fallback on Linux without a kernel parent-death signal. A launcher hands a TCP listener to its child; the check kills the launcher either before releasing the child's startup barrier or after the watcher is ready, and requires that a replacement can bind the listener. Both pass. Separate checks pin the supervisor's environment handoff and invalid-PID refusal. The full daemon and setup packages pass (1.222 s and 13.286 s); `TestDarwinAcceptanceCleanupStubs` now runs the six Python stub checks from the owning setup package, so normal Go smoke includes them. Under the smoke lock, restoring the old current-parent capture fails the early-orphan assertion, disabling parent monitoring fails the late-death assertion, and restoring deletion-exit-status failure fails the cleanup assertion. Restored focused checks pass (1.044 s and 0.392 s); the lock is released.

Native no-sudo verification at `e6720401` also passes on the owner's darwin/arm64 Mac with Go 1.27.3: the actual parent watcher, PID handoff and listener-release fixture (1.476 s), plus all `TestDarwin` checks including cleanup stubs (0.566 s). This run used the existing clone and workspace-only Go caches and temporary files. It returned to clean `main` `7c6555aa`; all task output and `.cache` were removed, and `~/go` and `~/Library/Caches/go-build` remain absent. The workspace is 57M after importing Git objects (54M before); no profile, package or host-setting change was made. W1B received `mac free` for its next run. Final fast smoke passes 26/0 in 7.603 s; link and whitespace checks pass, including the deferred uninstall-command entry in FUTURE.

Immediately after install teardown, the clone was clean on `main` and the workspace was **53M**; the later admin proof leaves it **54M** as recorded above. `~/go` and `~/Library/Caches/go-build` are absent. `df -h` reported 460 Gi total, 192 Gi used / 244 Gi available before, and 193 Gi used / 244 Gi available after; these are the filesystem's rounded readings, not a claim that global disk usage was unchanged.

## Sudo commands actually run

Every direct invocation is listed below in execution order, including read-only preflights and repeated status reads. The two `SETUP CHILD SUDO` lines describe the fresh-install child invocations: successful database initialization and first-user enrolment confirm both ran. No token value or private key is printed.

<details>
<summary>Complete sudo invocation list</summary>

```text
SUDO: sudo -n /usr/bin/true
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /usr/local/var/agent-bus/daemon
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /usr/local/var/run/agent-bus
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /usr/local/lib/agent-bus/mac2-check
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /Library/LaunchDaemons/agent-busd.plist
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /Library/LaunchDaemons
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /usr/local
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /usr/local/var
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /usr/local/lib
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /usr/local/var/agent-bus
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /usr/local/lib/agent-bus
SUDO: sudo -n /bin/sh -c 'test -e "$1" || test -L "$1"' mac2-check /usr/local/var/run
SUDO: sudo -n /usr/bin/install -d -m 0755 /usr/local/lib/agent-bus/mac2-check
SUDO: sudo -n /usr/bin/install -m 0755 /Users/parf/agent-bus-dev/mac2-install-h0ybfo7f/bin/agent-busd /usr/local/lib/agent-bus/mac2-check/agent-busd
SUDO: sudo -n /usr/bin/install -m 0755 /Users/parf/agent-bus-dev/mac2-install-h0ybfo7f/bin/agent-bus-setup /usr/local/lib/agent-bus/mac2-check/agent-bus-setup
SUDO: sudo -n /usr/bin/install -m 0755 /Users/parf/agent-bus-dev/mac2-install-h0ybfo7f/bin/agent-bus-admin /usr/local/lib/agent-bus/mac2-check/agent-bus-admin
SETUP CHILD SUDO (fresh state): sudo -u _agentbusd /usr/local/lib/agent-bus/mac2-check/agent-busd --owner mac2-check@localhost --db /usr/local/var/agent-bus/daemon/agent-bus.db --init
SETUP CHILD SUDO: sudo -u _agentbusd env AGENT_BUS_HOME=/usr/local/var/agent-bus/daemon AGENT_BUS_ADDR=/usr/local/var/run/agent-bus/user-_agentbusd.sock /usr/local/lib/agent-bus/mac2-check/agent-bus-admin user add mac2-check@localhost - --admin
SUDO: sudo -n /usr/bin/env AGENT_BUS_TLS_DIR=/usr/local/var/agent-bus/daemon/tls /usr/local/lib/agent-bus/mac2-check/agent-bus-setup --owner mac2-check@localhost --addr 127.0.0.1:54923 --exec /usr/local/lib/agent-bus/mac2-check/agent-busd --key /Users/parf/agent-bus-dev/mac2-install-h0ybfo7f/fixture-key.pub --tls self-signed
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /usr/bin/stat -f %Lp /usr/local/var/agent-bus/daemon
SUDO: sudo -n -u _agentbusd /usr/bin/openssl x509 -in /usr/local/var/agent-bus/daemon/tls/cert.pem -noout -sha256 -fingerprint
SUDO: sudo -n -u _agentbusd /usr/bin/curl --fail --silent --show-error --max-time 3 --unix-socket /usr/local/var/run/agent-bus/user-_agentbusd.sock -H 'Content-Type: application/json' --data '{"name": "mac2-check@localhost"}' http://localhost/token
SUDO: sudo -n -u _agentbusd /usr/bin/curl --fail --silent --show-error --max-time 3 --unix-socket /usr/local/var/run/agent-bus/user-_agentbusd.sock -H 'Content-Type: application/json' --data '{"name": "#mac2-check@localhost"}' http://localhost/token
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl kickstart -k system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/kill -KILL 20988
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl print system/agent-busd
SUDO: sudo -n /bin/launchctl bootout system/agent-busd
SUDO: sudo -n /usr/bin/pkill -KILL -u _agentbusd
SUDO: sudo -n /bin/rm -f /Library/LaunchDaemons/agent-busd.plist
SUDO: sudo -n /bin/rm -rf /usr/local/var/agent-bus/daemon /usr/local/var/run/agent-bus /usr/local/lib/agent-bus/mac2-check
SUDO: sudo -n /usr/bin/dscl . -delete /Users/_agentbusd
```

</details>

## Reproduction

The focused install/uninstall harness is [mac2-install-check.py](../../../src/acceptance/mac2-install-check.py), with a node-local copy at `local/w1c/mac2-install-check.py`; the historical complete output is `tmp/ab-w1c-MAC2/tmp/mac-install.local.log`. No harness or log is retained on the Mac. Re-running while the owner cleanup is outstanding is forbidden; the harness also refuses an existing role, job or setup tree.
