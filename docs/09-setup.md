# Setup and operation

📌 **TL;DR:** Build, install, provision users and operate the node. Privilege
is what separates the programs and nothing else does: root is needed once for
installation and never again, each account's files are that account's own, and
ordinary use needs neither.

## Status

| MVP | Scope |
|---|---|
| Built | Distributable archive, installer, administration, token helper, accounts, daemon unit and stamped builds; fresh-host installation, populated upgrade/recovery and [backup/restore](#backup-and-restore) acceptance. |
| Pending | Installed runtime/browser acceptance. |

## The programs

Privilege is what separates them, and nothing else does: root is needed once
and never again, each account's own files are that account's, and what an
ordinary user needs is neither. The count is deliberately not in the heading.

| Program | Runs as | What it is for |
|---|---|---|
| `agent-bus-setup` | **root**, and refuses otherwise, printing the `sudo` line to run | creates the **two accounts** ([the two accounts](#the-two-accounts)) and their homes, chowns them, writes and enables the unit, then hands over to `agent-bus-admin` for the first user and their local person name |
| `agent-bus-admin` | the **`agent-busd` account**; re-runs itself under `sudo -u agent-busd` when it is not | user and local-account-map administration, plus the `token` verb, which it hands to the program below rather than implementing twice. **Not for ordinary users** |
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

`user import-local <user[@realm]> <account>` fills a blank person name from the
OS account database. It accepts an account name rather than profile text and
preserves an existing name. Setup runs it when it installs the first user's
key; an operator may run it later when setup found no key.

The implemented admin verbs are `user add`, `user import-local`, `user list`,
`user remove`, `account list`, `account set`, `account remove` and `token`.
The token operation is delegated to the token helper with the key entitlement
and original request kept separate: `token --rotate` rotates that identity;
asking for another identity is refused. Console and SSH use the same program. Bundle administration and regeneration of keys are not
part of this grammar.

## Administrator name migration

Upgrading from the old administrative name migrates membership to the
[protected Administrator group](01-identity-and-roles.md#groups).
Existing record Maintainer assignments and ACL references follow the rename.

If an ordinary group already uses the destination name, it is preserved as
`@administrators-legacy` (with a numeric suffix when necessary), and its existing
references follow it. Neither existing groups nor unresolved record references
are reused for that preserved name. Its members do not become Administrators.
The former administrative name cannot be recreated; subsequent starts leave
the migrated groups unchanged.

Upgrade the daemon and its clients together: the user directory's administrative
flag is now `administrator`, replacing `maintainer`. Record Maintainer
assignments retain their meaning.

**Downgrade:** an older daemon does not recognize the renamed group as granting
non-owner administrative standing. Its members still retain the access and
record maintenance granted through ordinary group references. Downgrading does
not restore the old name automatically.

## Record kind upgrade

From 0.6.0 `kind` is a closed set of five
([records](03-records.md#record-kinds)). `generic`, `topic` and the `mode`
field are gone, and **a daemon of this line refuses to start on a snapshot
holding any of them**, naming the record rather than converting it. There is no
compatibility obligation before 1.1, so nothing is migrated at load time.

Upgrading a node that has run an earlier line therefore means editing its
snapshot while the daemon is stopped: the procedure, the mapping and what it
was verified against are in the
[cutover runbook](../Plans/MVP/cutover.md#the-procedure). Two things to expect
even where the kinds look right: a record that becomes 📡 must lose the queue
settings every old record carried, and its inbox must be empty.

## GitHub profile snapshot upgrade

From 0.5.71, User profiles may retain public provider metadata and one bounded
local PNG thumbnail. An older daemon ignores those unknown snapshot fields and
drops them on its next snapshot write. Imported Person name and Email remain
because they use the existing profile fields. The packaged rollback restores
the matching earlier snapshot; do not run an older binary directly against a
0.5.71 snapshot if retaining the optional provider fields matters.

## Maintainers-list upgrade

From 0.5.62, a resource stores a list of Maintainers rather than one group.
Existing string values load as a one-entry list; an empty string becomes an
empty list. Every non-empty later answer and snapshot writes the array form;
an empty list is omitted. Legacy noncanonical terms are preserved but remain
inert until the Owner replaces the list. Registration never imports this
authority from caller-supplied metadata.

Upgrade daemon and clients together. The new daemon accepts the legacy string
input during migration, but an older daemon cannot read the array form written
by 0.5.62 and fails to restore that snapshot rather than guessing authority.

## Nested group upgrade

From 0.5.57, an `@group` entry inside an ordinary group is a membership edge.
Earlier snapshots may contain such entries as inert strings. Review existing
group lists for stray `@`-names before upgrading: a group that exists now,
or is populated later, grants its effective members every ACL and Maintainer
permission carried by the containing group.

Cycles terminate without granting anybody unless another path reaches them;
unknown groups remain inert. `@administrators` stays direct-only and a snapshot
that nests a group inside it is refused at startup. Personal agents still
reject group ACL entries and Maintainer assignments, so this upgrade does not
widen their assignment rules; 0.7 revises them through
[Personal](03-records.md#personal-and-shared).

## Empty ACL upgrade

From 0.5.44, existing empty ACLs adopt the [restricted default](02-access.md#acl);
there is no grandfathered open access. Unrelated callers lose both delivery
access and visibility: listings omit the record and lookup answers as hidden.

Before upgrading, resource owners should explicitly grant any intended peer
access, including access to reply inboxes. `*` opts into broad sharing; do not
add it merely to silence a refusal. This release also stops implicit master
access to empty-ACL resources. The daemon Owner can discover and manage those
resources through separate [node-wide authority](01-identity-and-roles.md#daemon-owner),
but still cannot use their message interface without an ACL grant.

Fresh automatic registrations by faces and launchers also follow the default;
configured sharing survives [metadata re-registration](01-identity-and-roles.md#registration).
Script runners can state grants with [their start options](08-runner-role.md#script-agents).

## Owner ACL and master removal

From 0.5.74, `@owner` in an ACL means the record's direct Owner and the agents
directly owned by that Owner. It is evaluated from current
registry ownership and is never stored as a group. The `ab-claude`, `ab-codex`
and `ab-opencode` launchers add it while preserving existing explicit grants;
already-running launcher sessions acquire it when next restarted.

`@owner` has no stored-group form. Creating or nesting it is refused, and a
snapshot containing a group with that reserved name fails closed.

The same release removes `-master`, `no_master` and the implicit master ACL
grant. A daemon Owner still
discovers and manages every resource through
[node-wide authority](01-identity-and-roles.md#daemon-owner), but message access
now comes only from resource authority or the record's ACL.

## Daemon ownership upgrade

`--owner user[@realm]` (or `AGENT_BUS_OWNER`) is required; the daemon no longer
derives authority from the OS account name. On the first 0.5.55 start it seeds
legacy state and is written into the snapshot. Later starts trust the stored
Owner, so changing the flag does not transfer authority. Use the daemon-owner
transfer API instead.

The daemon OS account's private socket answers as the current daemon Owner,
not the configured value: a transfer moves it at once, without a restart
([local socket](02-access.md#local-socket)). A current snapshot whose stored
Owner is damaged fails startup.

**Downgrade:** an older daemon ignores the durable Owner fields and derives
authority from its configured `--owner` again. Resource ACLs and Administrator
membership remain stored, but daemon ownership reverts to that seed until the
current version returns.

## Install

Enabling `-web` requires bubblewrap (`bwrap` on `PATH`) and working unprivileged
user namespaces under the unit's restrictions. It also requires unified cgroup
v2 and systemd 254 or newer for delegated controllers and
`DelegateSubgroup=supervisor`. Build with `src/build.sh`:
its static web binary needs no host libraries inside the
[web sandbox](11-processes.md#web-authority-boundary). Missing sandbox support
is a startup failure, never an unconfined fallback. No setuid helper or extra
capability is granted; the web listener retains the host's unprivileged port
rules. Fresh-host acceptance must verify this dependency on the target host.

The generated unit supplies the dashboard's [separate resource group](11-processes.md#web-resource-limits).
Starting `agent-busd -web` outside that unit deliberately leaves web down. For
a development run, use a transient **user** service with `Delegate=cpu`,
`Delegate=memory`, `Delegate=pids`, `DelegateSubgroup=supervisor` and
`AGENT_BUS_WEB_USER_DELEGATION=1`; the supervisor still verifies ownership,
controllers, subgroup name and writability before starting web.

**Built:** `src/package.sh [output-directory]` creates one version/platform/
architecture archive and a portable SHA-256 file. The archive contains all six
Go programs, the MCP face, three runtime launchers, their shared adapter,
license, exact artifact manifest and standalone `INSTALL.md`. Build it, verify
and unpack it, then run `sudo ./agent-bus-setup`. Setup validates the complete
manifest before changing the host, installs a digest-addressed release under
`/usr/local/lib/agent-bus`, creates stable commands under `/usr/local/bin`,
creates the accounts and tree, writes/enables the daemon plus dashboard unit,
and installs the first user's key through the admin program. Without root it
refuses and prints the command to run. `--dry-run` and `--print-unit` are
read-only and need no root.

**MVP ships its own install script and nothing else.** The tar archive is a
transport for that script, not a second installer. There is no `npm install`
path or package registry; publishing the MCP face or a container stays
[R1](../Plans/R1/distribution.md#container-runtime) work. The packaged
`INSTALL.md` is the standalone supported path exercised by
[installation acceptance](#installation-acceptance).

**Deploying a commit.** From 0.8.19 `sudo src/release.sh [commit]` is how a
checkout's node changes: it builds that commit (HEAD by default) from
`git archive`, never the working tree, into
`/usr/local/lib/agent-bus/releases/<version>-<sha>`, switches
`/usr/local/lib/agent-bus/current` to it with one rename, points
`/usr/local/bin` through `current`, restarts the daemon and checks it reports
that version. Uncommitted edits, anyone's, never reach the live node. The last
five releases are kept; `--rollback` switches to the previous one.

**Development install.** `src/git-install.sh` symlinks the built programs into
`/usr/local/bin` instead of copying them, so a change is a build and a restart
rather than a reinstall. That serves the working tree itself, so a node that
other workers share is deployed with `release.sh` instead. The checkout must live outside `/home`: the daemon runs
behind `ProtectHome=yes` and cannot exec a binary in a home directory at all.
The script refuses one that does and says where to move it; `--revert` copies
real binaries back. Daemon state under `/var/lib/agent-bus` is untouched, except
by `--reinstall`.

**Reinstall.** `agent-bus-setup --reinstall` (and, on a checkout,
`src/git-install.sh --reinstall`) is the one explicit way to start a node over:
it stops the daemon, moves its whole home and the unit's drop-ins into a
root-only `daemon.before-0.7-<time>` directory that nothing reads again, and
installs fresh — new database, generated unit, the installer as Owner. Every
old credential stops working. It replaces whichever release is installed,
0.6 included; the [reinstall procedure](../Plans/MVP/0.7-cutover.md#procedure)
owns the steps.

The current link changes by one atomic rename after the complete release and
all stable command links exist. A failed first install is recovered by running
the same setup again; an identical release is reused. Plain setup refuses to
replace a different release. The standalone package instructions define the
`--upgrade` and interrupted `--recover` paths; their populated-node behavior is
accepted under [H.1.1](../Plans/MVP/done/upgrade-recovery.md#checks).

## Administering the account map

**Built.** The durable local-account map is administered by the daemon Owner or
an Administrator with credentials that already exist: an **operator SSH key**,
whose forced command is `agent-bus-admin <principal>` ([SSH admin](#ssh-admin)),
or that principal's **user token** against the API
([getting a token](02-access.md#getting-a-token)).

Nothing new is minted for it and no separate administrative password exists.
The entitlement is the principal in the key's entry, exactly as it is for every
other operator action, so who may edit the map is answered by the same thing
that answers who may do anything else.

```sh
agent-bus-admin account list
agent-bus-admin account set <local-account> <user[@realm]>
agent-bus-admin account remove <local-account>
sudo systemctl restart agent-busd
```

The mapped principal must already be known and active, and the local OS account
must exist. The daemon account's own socket is implicit and cannot be reassigned
or removed. A successful edit is in the snapshot before it is acknowledged,
but listeners belong to the supervisor: the command reports `restart required`
until a full daemon restart applies the desired map. A bus-child restart alone
keeps the existing listeners.

The first current start seeds this map from setup's `-user` flags. Once the
snapshot carries the establishment marker, the stored map is authoritative;
changing or retaining an old flag cannot restore a removed mapping. Startup
rejects malformed mappings, duplicate accounts, missing OS accounts and any
attempt to put the implicit daemon account into the editable map. Retired socket
paths are removed when the new supervisor starts.

**Downgrade:** an older daemon ignores the durable map and returns to its
configured `-user` flags until the current version returns. Review those flags
before rollback; they may describe an earlier identity assignment.

## Installation acceptance

**Fresh installation is built and accepted.** A disposable host with real
systemd, cgroup v2 and bubblewrap installs from only the archive and standalone
instructions, starts the generated daemon/dashboard unit and completes a real
call over the bus. Removing a daemon binary, MCP face or launcher face separately
fails before accounts, state, unit or current release exist. The retained
[H.1 evidence](../Plans/MVP/done/fresh-install.md#checks) records the host and
claim limits.

**Upgrade/recovery is built and accepted.** The installer verifies and stages
the next release while the old daemon serves, then stops cleanly, backs up the
whole daemon home, switches `current`, starts and checks both public identity
and the unit's real process tree. The existing unit and drop-ins remain byte
identical. A failed start restores the prior release with its matching snapshot,
credentials and SSH authorization; an interrupted transaction leaves a durable
marker for the documented `--recover` command. The retained
[H.1.1 evidence](../Plans/MVP/done/upgrade-recovery.md#checks) covers credentials,
registry, queued state, ACLs, local mappings and operator configuration. This
does not select the future runner backup mechanism.

SSH onboarding is accepted through an actual sshd installation for both
ordinary and operator keys, including the documented token command,
entitlement enforcement and restricted access. Checking generated
`authorized_keys` text alone is insufficient. The [SSH onboarding evidence](../Plans/MVP/done/ssh-onboarding.md#checks) records the real-sshd exercise and its environment limits; its [0.7 re-run](../Plans/MVP/done/ssh-onboarding.md#07-re-run) adds realm-less names, two-token rotation and credentials surviving a restart.

[Backup and restore](#backup-and-restore) is accepted on packaged hosts.

[H.1.1](../Plans/MVP/done/upgrade-recovery.md#checks) records completed upgrade/recovery acceptance;
[H.5.2](../Plans/MVP/done/ssh-onboarding.md#checks) records the completed SSH exercise.

## Build information

Building the daemon and CLI needs cgo and a C compiler for
[processes § process titles](11-processes.md#process-titles).

`src/build.sh [output-directory]` builds all Go programs with one stamp;
the output directory is relative to `src/` and defaults to that directory.
The build supplies `build_info` as `user@hostname YYYY-MM-DD HH:MM:SS`, using
the builder's local time, through Go linker flags; `release.sh` appends the
commit's short SHA. It never rewrites source.
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
record ACLs apply per user with nothing for anyone to configure. Setup's
initial principal is the first daemon Owner; after transfer, the stored current
Owner holds node-wide management authority
([identity § daemon owner](01-identity-and-roles.md#daemon-owner)).
## Storage

| Built store | Holds |
|---|---|
| SQLite database `agent-bus.db`, mode 0600, held exclusively | Daemon Owner, local-account map, users, registry, groups, credentials and issued times, queue contents, counters, each record's [day of activity](05-discovery.md#activity-history) and clean-stop marker; schema 5 from 0.8.12, migrated from 4 at open |
| Memory only | Browser sessions, outstanding readers, uptime and recent envelope feed |

**Built in 0.7.1**, through `modernc.org/sqlite` behind the store ports. The
daemon is told where the database is with `-db`; setup runs `agent-busd -init`
once, as the daemon account, to create it, and the unit never passes `-create`.
Every record and user carries an internal ID from 0.7.3: stable, persisted,
never on an answer, and never handed out twice, because the database keeps each
high-water mark rather than deriving it from what is left.
The [0.7 transition](../Plans/MVP/0.7-cutover.md#scope) uses clean reinstall: the
0.6 JSON dump and token file are neither read nor kept as stores. Additional
database adapters remain [R1 work](../Plans/R1/storage.md#backends). Durability is
defined in [messaging § durability](04-messaging.md#durability).

Follow the [reinstall procedure](../Plans/MVP/0.7-cutover.md#procedure) for 0.7
bootstrap and client reconnection. SQLite state and backups retain the daemon
account's private directory and file boundary.

## Backup and restore

**A backup is the stopped daemon home.** The daemon holds its database
exclusively, so no online copy exists: `sqlite3 .backup` and every other reader
get `database is locked`. A graceful stop flushes queues and counters and
checkpoints the WAL into `agent-bus.db`, so the stopped home is complete and
consistent. The pause is the stop plus the start.

| Step | As root |
|---|---|
| Back up | `systemctl stop agent-busd`, then `tar -C /var/lib/agent-bus -cpf <backup>.tar daemon`, `chmod 600 <backup>.tar`, `systemctl start agent-busd` |
| Restore | Install the same release (on a new host, `agent-bus-setup` as in [install](#install)), `systemctl stop agent-busd`, `rm -rf /var/lib/agent-bus/daemon`, `tar -C /var/lib/agent-bus -xpf <backup>.tar`, `systemctl start agent-busd` |

<details>
<summary>What it covers and what to keep in mind</summary>

- The home holds the database (users, records, groups, configurations,
  secrets, credentials, ownership, queue contents and counters) and
  `.ssh/authorized_keys`; together they are the node. The archive holds live
  credentials and secrets: keep it root-only.
- Restore **replaces the whole home**. Copying only `agent-bus.db` over a
  home that still has an `agent-bus.db-wal` from the state being replaced —
  left by any unclean stop — lets SQLite replay that newer WAL onto the older
  database.
- A restore is exactly the backed-up state: later users, keys, records,
  rotations and traffic are gone, and credentials issued after the backup
  stop authenticating.
- The local-account map is restored with the database; a new host needs the
  same OS accounts for their sockets. Logs, the unit and the runner's
  `service.d` are not part of the backup.
- A backup taken without the stop is not supported: the database file lacks
  what is still in the WAL and in memory.

**Built and accepted in 0.7.19** by `src/acceptance/backup-restore.sh`: a
populated disposable host is backed up twice, an older backup restored over
newer uncleanly stopped state returns exactly that older state, and a fresh
host restored from the newer one matches its listings, owners, `config_sha`,
`secret_sha`, queued and in/out/dropped counts and credentials, then keeps
unflushed traffic over a graceful restart and loses only post-flush traffic
in a bus-child crash ([durability](04-messaging.md#durability)).

</details>

## Logs

**Built in 0.7.2.** The daemon writes the [three logs](constitution.md#logs)
under `-log-dir`, which setup makes `/var/log/agent-bus/`: owned by
`agent-busd`, group `adm`, mode `2750`, so every log the daemon creates there is
readable by `adm` and by nobody else. The files are `0640`.

| File | Written |
|---|---|
| `audit.log` | always: one line per administrative action or entity edit — actor, operation, target, result and, over TCP, the client address |
| `error.log` | always: warnings, errors and alerts, each also sent to syslog |
| `debug.log` | only when asked for, by `-debug-log` at start or by the daemon Owner with `POST /debug {"on": true}` at run time |

Without `-log-dir` a development daemon writes them to `logs/` beside its
database. Setup installs `/etc/logrotate.d/agent-bus`: weekly, eight kept,
compressed, by copy and truncate so the daemon never reopens a file.

## The two accounts

The installer creates both system accounts now. Only the daemon's runtime is
installed; the second account and directories prepare a later runner.

| Directory under `/var/lib/agent-bus` | Owner | Mode | Current purpose |
|---|---|---|---|
| `daemon/` | `agent-busd` | 700 | Daemon home and its database |
| `runner/` | `agent-bus-runner` | 700 | Prepared runner home; no managed instances installed |
| `service.d/` | `agent-bus-runner` | 755 | Prepared shareable script directory |

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

The packaged [installed shared-host checks](../Plans/MVP/done/installed-shared-host.md#checks)
run two actual mapped accounts through their own sockets, refuse both cross-
account attempts, and verify from `/proc` that only the supervisor retains the
capability.

The installer also maps the prepared runner account to a local socket. The
future runner unit is defined in [R1 operations](../Plans/R1/operations.md#runner-unit).

## Config locations

Current daemon configuration comes from command flags and environment; setup
writes the flags into its unit. The per-program defaults are in
[daemon source](../src/cmd/agent-busd/main.go). A general configuration-file
format is not implemented. Per-record ACL editing is available through the
[dashboard](05-discovery.md#required-tabs).
