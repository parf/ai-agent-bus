# MAC.2 native daemon and reduced setup

📌 **TL;DR:** The daemon, setup and admin binaries build natively on Apple Silicon. The focused launchd, private-storage, TLS request/reply, restart and orphan checks passed on 2026-10-30. Removing the test role account over SSH was refused; that cleanup is with the owner. Full archive validation remains with MAC.1.

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

## Owner cleanup still open

`sudo -n dscl . -delete /Users/_agentbusd` returned exit 40, `eDSPermissionError` / `-14120`. The owner is handling it; the CEO and director instructed the worker to run no more sudo on the Mac.

Remaining state is the `_agentbusd` user and group, UID/GID **206**, plus empty setup parent directories. The job, plist, daemon home, socket directory and test binary tree are gone. This does not affect MAC.1's client/runner checks; W1B was told `mac free`.

The owner also saw the same `dscl` refusal at the Mac console. The acceptance cleanup now deletes the user with `sysadminctl -deleteUser` and the group with `dseditgroup -o delete`, rather than `dscl`. It attempts both and verifies their absence with directory listings, including when a deletion command returns zero. A failed deletion, leftover entry or unreadable listing exits nonzero with the role names and [Full Disk Access recovery](../../../docs/07-setup.md#uninstall-the-macos-test-daemon). Live cleanup and the permission change remain with the owner; no worker sudo was run for this fix.

`python3 src/acceptance/test_mac2_cleanup.py` runs only local subprocess stubs. Four checks cover successful removal, already absent entries, both deletion failures, a zero-status deletion leaving both entries, an unreadable directory and a group-only failure. On the old cleanup, five assertions fail, including the unreadable-directory case incorrectly printing `PASS uninstall`; all pass after the fix. The harness is now committed under `src/acceptance/`; the node-local copy is kept in sync. Under the `smoke-gate` lock, three independent cleanup mutants also fail assertions: restoring `dscl` user deletion, trusting a zero deletion status without checking directory presence, and skipping group deletion. Restored checks pass; the lock is released.

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
