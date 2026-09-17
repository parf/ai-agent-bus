# Setup and operation

📌 **TL;DR:** Build, install, provision users and operate the node.

## Status

| MVP | Scope |
|---|---|
| Built | Installer, administration, token helper, accounts and daemon unit; stamped Go builds. |
| Pending | Distributable package including [runtime integrations and launchers](08-runner-role.md#runtime-integration-delivery), editable local-account configuration and [installation acceptance](#installation-acceptance). |

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

**`user add` creates the user it adds.** A credential is only issued to a name
the daemon already holds a profile or a record for
([getting a token](02-access.md#getting-a-token)), so writing the
`authorized_keys` line alone would leave a key whose forced command is refused
— which is no way in at all. The verb writes the line and creates the user, and
`--admin` additionally grants [Administrator membership](01-identity-and-roles.md#groups). Authority is granted
only where it was asked for: Administrator standing is membership, derived by the
daemon, never a field a caller may claim.

The two halves are kept together. The key line is written first because it is
the half that can be taken back — a user is never deleted
([user lifecycle](01-identity-and-roles.md#user-states)) — and it is removed again if
the daemon refuses. **The daemon has to be running**: with no way to create the
name, `user add` refuses rather than leaving a key that works before the name
exists. Start the daemon and run it again.

The implemented admin verbs are `user add`, `user list`, `user remove` and
`token`. The token operation is delegated to the token helper with the key entitlement
and original request kept separate: `token --rotate` rotates that identity;
asking for another identity is refused. Console and SSH use the same program. Bundle administration and regeneration of keys are not
part of this grammar.

## Administrator name migration

Upgrading from the old administrative name migrates membership to the
[protected Administrator group](01-identity-and-roles.md#groups).
Existing service Maintainer assignments and ACL references follow the rename.

If an ordinary group already uses the destination name, it is preserved as
`@administrators-legacy` (with a numeric suffix when necessary), and its existing
references follow it. Neither existing groups nor unresolved record references
are reused for that preserved name. Its members do not become Administrators.
The former administrative name cannot be recreated; subsequent starts leave
the migrated groups unchanged.

Upgrade the daemon and its clients together: the user directory's administrative
flag is now `administrator`, replacing `maintainer`. Service/channel Maintainer
assignments retain their meaning.

**Downgrade:** an older daemon does not recognize the renamed group as granting
non-owner administrative standing. Its members still retain the access and
service maintenance granted through ordinary group references. Downgrading does
not restore the old name automatically.

## Empty ACL upgrade

From 0.5.44, existing empty ACLs adopt the [restricted default](02-access.md#acl);
there is no grandfathered open access. Unrelated callers lose both delivery
access and visibility: listings omit the record and lookup answers as hidden.

Before upgrading, resource owners should explicitly grant any intended peer
access, including access to reply inboxes. `*` opts into broad sharing; do not
add it merely to silence a refusal. This release also stops implicit master
access to empty-ACL resources. The daemon owner needs resource-level authority
there until the [node-wide override](01-identity-and-roles.md#daemon-owner) is built.

Fresh automatic registrations by faces and launchers also follow the default;
configured sharing survives [metadata re-registration](01-identity-and-roles.md#registration).
Script runners can state grants with [their start options](08-runner-role.md#script-services).

## Install

Enabling `-web` requires bubblewrap (`bwrap` on `PATH`) and working unprivileged
user namespaces under the unit's restrictions. Build with `src/build.sh`:
its static web binary needs no host libraries inside the
[web sandbox](11-processes.md#web-authority-boundary). Missing sandbox support
is a startup failure, never an unconfined fallback. No setuid helper or extra
capability is granted; the web listener retains the host's unprivileged port
rules. Fresh-host acceptance must verify this dependency on the target host.

**Built:** build the programs, then run `sudo agent-bus-setup`. Setup creates
the accounts and tree, writes/enables the daemon unit, and installs the first
user's key through the admin program. Without root it refuses and prints the
command to run. `--dry-run` and `--print-unit` are read-only and need no root.

**MVP ships its own install script and nothing else.** No `npm install` path,
no package registry: the programs are built and `agent-bus-setup` puts them in
place. Publishing the MCP face to npm stays [R1](../Plans/R1/distribution.md#container-runtime)
work. One supported way in is what the [installation acceptance](#installation-acceptance)
below can actually be run against.

**Development install.** `src/git-install.sh` symlinks the built programs into
`/usr/local/bin` instead of copying them, so a change is a build and a restart
rather than a reinstall. The checkout must live outside `/home`: the daemon runs
behind `ProtectHome=yes` and cannot exec a binary in a home directory at all.
The script refuses one that does and says where to move it; `--revert` copies
real binaries back. Daemon state under `/var/lib/agent-bus` is untouched.

**Pending:** the intended package installation is not implemented, and in MVP
it is not meant to be: [one script is the whole way in](#install), and a package
format is [R1 distribution](../Plans/R1/distribution.md#container-runtime).
A fresh-host install and the running service account must still be verified as
[stage gates](../Plans/MVP/TODO.md#installed-stage-gate).

## Administering the account map

**Pending.** The local user-to-account map is administered with the credentials
that already exist and no third one: an **operator SSH key**, whose forced
command is `agent-bus-admin <principal>` ([SSH admin](#ssh-admin)), or an
ordinary **user token** against the API ([getting a token](02-access.md#getting-a-token)).

Nothing new is minted for it and no separate administrative password exists.
The entitlement is the principal in the key's entry, exactly as it is for every
other operator action, so who may edit the map is answered by the same thing
that answers who may do anything else.

## Installation acceptance

**Required MVP, pending.** Installation includes operation of an existing node:
an upgrade preserves credentials, registry and queued state, access policy,
local account mappings and operator configuration. Every running component
must use the intended release. Instructions must cover recovery from an
interrupted upgrade and restoring a consistent set of credentials and state.
Package format is not an MVP choice at all — [one script](#install) is, and packaging is [R1](../Plans/R1/distribution.md#container-runtime).
This requirement does not select the future runner backup mechanism.

SSH onboarding is accepted through an actual sshd installation for both
ordinary and operator keys, including the documented token command,
entitlement enforcement and restricted access. Checking generated
`authorized_keys` text alone is insufficient. The [SSH onboarding evidence](../Plans/MVP/done/ssh-onboarding.md#checks) records the real-sshd exercise and its environment limits.

[H.1.1](../Plans/MVP/TODO.md#remaining-work) retains upgrade/recovery acceptance;
[H.5.2](../Plans/MVP/done/ssh-onboarding.md#checks) records the completed SSH exercise.

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
| bus username | the principal it *is* — `parf@localhost`, `parf@github`, `parf@realmo` ([identity § names](01-identity-and-roles.md#names)) |

From that the daemon opens one socket per user
([access § local socket](02-access.md#local-socket)) and knows who is calling
on every request.

Result: a bus with AUTH off that **serves every user on the host at once**, so
service ACLs apply per user with nothing for anyone to configure. The person
who ran setup **holds master** ([identity § acl](02-access.md#acl)).
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

The daemon account uses `/bin/sh` because sshd runs even forced commands through
the account shell; each issued key is restricted to its named helper, without
PTY or forwarding. The runner keeps `nologin` and receives no SSH keys. Setup
repairs the daemon account’s former `nologin` default while preserving custom
shells; an operator choosing one must ensure it can execute the forced helpers.
Runtime sockets live at the [access location](02-access.md#local-socket),
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
format is not implemented. Per-record ACL editing is available through the
[dashboard](05-discovery.md#required-tabs).
