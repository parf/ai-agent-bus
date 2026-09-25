# MVP release gap review

Historical review completed 2026-09-13 against the development checkout and
the [installed development host](../installed-acceptance.md#inspection).
The owner accepted the additional release gates. This records findings,
not completion of the resulting implementation or acceptance work.
The [active TODO](../TODO.md#next-step) owns execution and priority.

## Findings

| Area | Evidence | Limit |
|---|---|---|
| Web authority | A read-only permissions probe entered the installed web child's mount namespace, dropped to its UID/GID and enabled no-new-privileges. `test -r` and `test -w` succeeded for the daemon token and snapshot files; the daemon account socket also passed the access check | No secret contents were read and no file was modified. This tests UID/mount permissions, not exploitation of the web application or every possible security-domain constraint |
| SSH operator token | With an isolated admin home, invoking the admin helper for `owner@review` with `SSH_ORIGINAL_COMMAND=token` exited 1 with `which name?`; delegation drops the entitled name | Forced-command invocation was reproduced directly, not through an authenticated SSH connection |
| SSH account shell | Setup selects `/usr/sbin/nologin`; invoking that shell with a harmless command exited 1 without running it. OpenSSH's [login process](https://man.openbsd.org/sshd.8#LOGIN_PROCESS) executes commands through the account's shell | Real installed SSH onboarding still needs its own exercise |
| Administrative recovery | On an isolated real daemon, a user ban returned 200 and the old token returned 403. Killing only the bus child before its next periodic snapshot triggered supervisor recovery; the same token then returned 200 | This follows the current snapshot behavior. Stronger durability was not assumed or implemented |
| Runtime sidecar isolation | The Codex launcher selects a loopback WebSocket and its client connects without an application credential in the examined path | Unauthorized cross-account attachment was not reproduced; this is a missing acceptance check, not a confirmed exploit |
| Upgrade and browser operation | The previous fresh-install and launcher gates lacked explicit existing-installation recovery and real-browser administrative workflows | Absence of those acceptance requirements is not evidence that every workflow fails |

## Reproduction locations

The disposable crash probe is `tmp/mvp-review/crash_access.py`, with its
isolated daemon logs under `tmp/mvp-review/crash-*/`. It creates an active
user, performs a graceful baseline save, starts again, bans the user and kills
only that run's bus child. Tokens stay inside the scratch fixture and are
never printed. The binaries used for this probe were development builds,
not shipped artifacts.

The SSH helper reproduction used the same scratch binary directory and an
explicit isolated admin home; it required no credential or live daemon.
The installed web probe used `nsenter` for the mount namespace and `setpriv`
for the service UID/GID, with permission tests only. Repeating it must resolve
the current web PID rather than reusing an old process ID.
