# Setup and operation

## Status

| MVP | Scope |
|---|---|
| Built | Installer, administration, token helper, accounts and daemon unit; stamped Go builds. |
| Pending | Distributable package, editable ACL/account configuration and installed-host acceptance. |

## The programs

Privilege is what separates them, and nothing else does: root is needed once
and never again, each account's own files are that account's, and what an
ordinary user needs is neither. The count is deliberately not in the heading.

| Program | Runs as | What it is for |
|---|---|---|
| `agent-bus-setup` | **root**, and refuses otherwise, printing the `sudo` line to run | creates the **two accounts** ([the two accounts](#the-two-accounts)) and their homes, chowns them, writes and enables the unit, then hands over to `agent-bus-admin` for the first user |
| `agent-bus-admin` | the **`agent-busd` account**; re-runs itself under `sudo -u agent-busd` when it is not | everything that edits what lives in the home — `user add`, `user list`, `user remove` today — plus the `token` verb, which it hands to the program below rather than implementing twice. **Not for ordinary users** |
| `agent-bus-token` | **any user** | hands out a credential, and does nothing else. What an ordinary user reaches over SSH ([access § getting a token](02-access.md#getting-a-token)) |
| `agent-bus` | **any user** | the ordinary client, over the unix socket or TCP ([access § local socket](02-access.md#local-socket)) |
| `agent-busd` | the **`agent-busd` account**, started by its unit | the daemon: a supervisor and its children ([processes](11-processes.md#processes-and-privileges)) |
| `agent-bus-web` | the **`agent-busd` account**, started by the daemon as a child | the dashboard, speaking the API like any other client and holding no write path of its own ([discovery § dashboard](05-discovery.md#dashboard)) |

## SSH admin

Setup adds the installer's public key through the admin program. Each entry
in the daemon account's `authorized_keys` is restricted to a forced command:
`agent-bus-token <principal>` for an ordinary key, or
`agent-bus-admin <principal>` for an operator. The entitlement is the principal
in that entry; `SSH_ORIGINAL_COMMAND` is parsed as a request, not executed as
shell text.

The implemented admin verbs are `user add`, `user list`, `user remove` and
`token`. The token operation is delegated to the token helper. Console and SSH
use the same program. Bundle administration and regeneration of keys are not
part of this grammar.

## Install

**Built:** build the programs, then run `sudo agent-bus-setup`. Setup creates
the accounts and tree, writes/enables the daemon unit, and installs the first
user's key through the admin program. Without root it refuses and prints the
command to run. `--dry-run` and `--print-unit` are read-only and need no root.

**Pending:** the intended package installation is not implemented. The package
format and binary delivery are [MVP questions](../Plans/MVP/QUESTIONS.md#open-questions).
A fresh-host install and the running service account must still be verified as
[stage gates](../Plans/MVP/TODO.md#installed-stage-gate).

## Build information

Building the daemon and CLI needs cgo and a C compiler for
[processes § process titles](11-processes.md#process-titles).

`src/build.sh [output-directory]` builds all Go programs with one stamp;
the output directory is relative to `src/` and defaults to that directory.
The build supplies `build_info` as `user@hostname YYYY-MM-DD HH:MM:SS`, using
the builder's local time, through Go linker flags. It never rewrites source.
Every Go program's `--version` prints the shared SemVer on the first line
and `build_info: <stamp>` on the second; no daemon connection or privileges
are needed.

A direct `go build` reports `development (unstamped)` instead of inventing a
build date. The interpreted MCP face reports the shared SemVer only.
Release builds follow [working rules § versioning](../CLAUDE.md#versioning).

## Local users

Setup asks for two things per user, and they are **configuration, not
credentials**:

| Config field | Purpose |
|---|---|
| local account | the OS account on this host; resolved to its uid |
| bus username | the principal it *is* — `parf@localhost`, `parf@github`, `parf@realmo` ([identity § names](01-identity.md#names)) |

From that the daemon opens one socket per user
([access § local socket](02-access.md#local-socket)) and knows who is calling
on every request.

Result: a bus with AUTH off that **serves every user on the host at once**, so
service ACLs apply per user with nothing for anyone to configure. The person
who ran setup **holds master** ([identity § acl](01-identity.md#acl)).
## Storage

| Built store | Holds |
|---|---|
| Text-file adapter, mode 0600 | Principal credentials and issued times; current and previous tokens |
| JSON snapshot adapter | Registry, queue contents, counters and clean-stop marker |
| Memory only | Browser sessions, outstanding readers, uptime and recent envelope feed |

The ports let a backend change without changing delivery. Database selection,
encrypted stores and store replication remain [generic undecided work](../Plans/Future/storage.md#storage).
Snapshot behavior is defined in [messaging § durability](04-messaging.md#durability).

## The two accounts

The installer creates both system accounts now. Only the daemon's runtime is
installed; the second account and directories prepare a later runner.

| Directory under `/var/lib/agent-bus` | Owner | Mode | Current purpose |
|---|---|---|---|
| `daemon/` | `agent-busd` | 700 | Daemon home, credentials and snapshots |
| `runner/` | `agent-bus-runner` | 700 | Prepared runner home; no managed instances installed |
| `service.d/` | `agent-bus-runner` | 755 | Prepared shareable service-code directory |

Both accounts have nologin shells. Only the daemon account receives the forced
SSH commands. Runtime sockets live at the [access location](02-access.md#local-socket),
not under either home.

### The two units

**Built:** `/etc/systemd/system/agent-busd.service`. The heading is retained
for existing references; a second unit is not installed in the MVP.

| Setting | Purpose |
|---|---|
| `User=agent-busd` | Run as the service account |
| Working/state directories and explicit state mode | Keep state under its own home |
| `RuntimeDirectory=agent-bus` | Recreate the runtime socket directory |
| `Restart=on-failure` | Restart a failed daemon |
| `AmbientCapabilities=CAP_CHOWN`, bounded to that capability | Own the per-user sockets without running the daemon as root |

The installer also maps the prepared runner account to a local socket. The
future runner unit is defined in [R1 operations](../Plans/R1/operations.md#runner-unit).

## Config locations

Current daemon configuration comes from command flags and environment; setup
writes the flags into its unit. The per-program defaults are in
[daemon source](../src/cmd/agent-busd/main.go). A general configuration-file
format and an ACL editor are not implemented.
