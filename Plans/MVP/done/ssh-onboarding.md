# SSH onboarding

📌 **TL;DR:** Real SSH keys reach only their entitled token or administration commands.

## Scope

H.5.2, implemented in 0.5.45. The [SSH contract](../../../docs/09-setup.md#ssh-admin)
and [account setup](../../../docs/09-setup.md#the-two-accounts) own current behavior.

The operator helper previously discarded its key's entitled name when delegating
`token`. It now passes the trusted name as argv and retains the original SSH
request for the token helper to validate. A mismatched name is refused;
`--rotate` is retained. An empty request is not a new console token alias.

Setup previously assigned `nologin` to the daemon account, preventing sshd from
executing even forced commands. It now gives that account `/bin/sh`, repairs
former `nologin` defaults on upgrade, and preserves custom shells. The runner
remains `nologin`. Each issued SSH key still has `restrict,command=`; an
executable account shell does not turn client command text into shell input.
OpenSSH runs commands through the account shell
([sshd login process](https://man.openbsd.org/sshd.8#LOGIN_PROCESS)).

## Checks

`src/acceptance/ssh-onboarding.sh <built-program-dir> <new-output-dir>` drives
real OpenSSH in a disposable rootless container. The fixture uses generated
setup accounts, generated authorized-key entries and the generated unit's
ExecStart command. The bus and SSH client/server are real, with fresh keys and
no host accounts, state, sockets or credentials mounted.

The final container exercise, `tmp/ssh-onboarding/acceptance-final.log`, passed:

| Check | Positive and negative evidence |
|---|---|
| Setup shells | Fresh daemon shell executes commands; runner stays nologin; old default repaired; custom bash preserved |
| Both key classes | Tokens obtained through SSH work as their exact entitled identities against the real bus |
| Entitlement | Both classes explicitly refuse a token request naming the other identity |
| Shell | Client `touch` request refuses and creates no marker |
| SSH channels | Both classes refuse PTY allocation and TCP forwarding |
| Administration | Operator lists, adds an ordinary key that works, and removes an existing key |
| Rotation | Both classes receive a different, working token |
| Isolation of removal | Removed key cannot authenticate; unrelated key still returns its unchanged token |

Cross-identity rotation was not separately exercised: mismatched-name requests
and valid rotation requests are distinct controls here. Unit tests also cover
forced-command argument parsing, missing entitlement and malformed combinations.
OpenCode reviewed source and acceptance through the agent bus.

Frozen full repository smoke (`src/ssh-onboarding-smoke.local.sh --slow`):
**589 passed, 0 failed**, exit 0; vet and race passed. Measured in
`tmp/ssh-onboarding/slow-final.log`, lines 10–11 and 931. Tracked and frozen
scripts share SHA-256
`be8db7b2dc8f34234bb58f350d49aea8b329a5d95bf956fd111dff9bc94889c3`.
The source/version hash manifest was unchanged throughout the run.

## Mutations

Six separately built Go overlays were run through the real-sshd acceptance
exercise. Each failed its named check; these are container acceptance runs,
not full repository smoke runs per mutation. Logs and replay are under
`tmp/ssh-onboarding/mutations/`, `mutations.log` and `mutate.py`.

| Mutation | Observed failure |
|---|---|
| Discard entitled argv during delegation | Operator key token command |
| Clear the original SSH request | Operator token request silently accepts a different requested name |
| Remove key restrictions | PTY becomes permitted |
| Restore nologin for fresh daemon accounts | Daemon shell check |
| Skip repair of the old shell | Upgrade repair check |
| Disable the token helper | Ordinary key token command through real SSH |

## Environment and limits

The host's OpenSSH 10.2p1 programs are copied into a locally cached Arch Linux
base image; absent shared libraries are copied alongside them. The image needs
base account/PAM tools. No network pull occurs; the container uses its own
loopback with `--network=none`. The fixture supplies a PAM account/session
policy and disables password and keyboard-interactive authentication. This is
one explicit SSH configuration, not certification of every distribution's PAM
or sshd policy. SELinux container labeling is disabled for the disposable mounts.

PID-1 systemd is **stubbed**: setup writes its real unit, but the harness starts
its command directly with the configured account and CAP_CHOWN. This does not
close [installed capability acceptance](../TODO.md#installed-stage-gate), H.1's
fresh-host package exercise, or upgrade/recovery acceptance. H.5.2's key,
forced-command and protocol exercise is the bounded result.

Initial probes reproduced both recorded failures before source edits. An early
container run failed on evidence-mount permissions; another reached the SSH
checks but lacked `cmp` for its final assertion. Those red logs remain under
`tmp/ssh-onboarding/`; only the corrected final run is acceptance evidence.

## 0.7 re-run

K.17, 2026-09-23, on the 0.7.19 build (SQLite, realm-less Owner `owner`).
`tmp/k17-ssh-4` passed 17 checks. Added to the checks above:

| Check | Evidence |
|---|---|
| Realm-less names | `ordinary` and `ordinary@ssh` are separate principals with separate keys; the `ordinary` key is refused `token ordinary@ssh` |
| Rotation | After one SSH rotation current and previous authenticate; after a second the oldest answers 401 |
| Restart | After a daemon restart both kept credentials authenticate and asking again returns the current one |
| Readiness | The fixture waited on `/healthz`, which 0.7 does not serve, and never failed; it now waits on `/identity` and fails when the daemon is not up |

Mutants, each failing an assertion: entitlement compared without the realm
(`ordinary key obtained ordinary@ssh`), previous token not authenticated
(`previous credential stopped at the first rotation`), previous token dropped
on load (`previous credential after restart`).

