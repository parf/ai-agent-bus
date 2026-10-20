# Setup and operation

📌 **TL;DR:** Build, install, provision users and operate the node. Privilege
is what separates the programs and nothing else does: root is needed once for
installation and never again, each account's files are that account's own, and
ordinary use needs neither.

## The programs

For installing and operating scripts under the runner, use its
[dedicated guide](runner/README.md).

Privilege is what separates them, and nothing else does: root is needed once
and never again, each account's own files are that account's, and what an
ordinary user needs is neither. The count is deliberately not in the heading.

| Program | Runs as | What it is for |
|---|---|---|
| `agent-bus-setup` | **root**, and refuses otherwise, printing the `sudo` line to run | creates the accounts ([the accounts](#the-two-accounts)) and their homes, chowns them, writes and enables the daemon and web units, then hands over to `agent-bus-admin` for the first user and their local person name; `--runner` installs a [person's runner](#a-persons-runner) |
| `agent-bus-admin` | the **`agent-busd` account**; re-runs itself under `sudo -u agent-busd` when it is not | user and local-account-map administration, plus the `token` verb, which it hands to the program below rather than implementing twice, and `tls`, which describes the served certificate and needs no account. **Not for ordinary users** |
| `agent-bus-token` | **any user** | hands out a credential, and does nothing else; `--fingerprint`, also over the SSH forced command, prints the node's TLS pin. What an ordinary user reaches over SSH ([access § getting a token](02-access.md#getting-a-token)) |
| `agent-bus` | **any user** | the ordinary client, over the unix socket or TCP ([access § local socket](02-access.md#local-socket)) |
| `agent-bus-runner` | a person's **`<person>-ab-runner` account**, started by its unit instance ([a person's runner](#a-persons-runner)) | manages installed agents and their lifecycle, configuration files and status; [current operations](runner/operations.md#running-and-checking-the-runner), [isolation](runner/security.md#isolation-today) |
| `agent-busd` | the **`agent-busd` account**, started by its unit | the daemon: a supervisor and its children ([processes](08-processes.md#processes-and-privileges)) |
| `agent-bus-web` | its own **`agent-bus-web` account**, started by its own unit; TypeScript run by `/usr/bin/bun`, not a binary | the web face, speaking the API with each visitor's session and holding no write path of its own ([processes § the web face](08-processes.md#the-web-face)) |

## SSH admin

Setup adds the installer's public key through the admin program. Each entry
in the daemon account's `authorized_keys` is restricted to a forced command:
`agent-bus-token <principal>` for an ordinary key, or
`agent-bus-admin <principal>` for an operator. The entitlement is the principal
in that entry; `SSH_ORIGINAL_COMMAND` is parsed as a request, not executed as
shell text. The forced command names the program as it was invoked
(`/usr/local/bin/…`), which follows the live release, never the release
directory a later deploy prunes. `agent-bus-admin` becomes the daemon account
through sudo, which drops `AGENT_BUS_ADDR`, so with an `AGENT_BUS_ADDR` naming
another daemon and no `AGENT_BUS_HOME` it refuses rather than edit the
installed one.

**`user add` creates the user it adds.** A credential is only issued to a name
the daemon already holds a profile or a record for
([getting a token](02-access.md#getting-a-token)), so writing the
`authorized_keys` line alone would leave a key whose forced command is refused
— which is no way in at all. The verb writes the line and creates the user, and
`--admin` additionally grants [Administrator membership](01-identity-and-authority.md#groups). Authority is granted
only where it was asked for: Administrator standing is membership, derived by the
daemon, never a field a caller may claim.

The two halves are kept together. The key line is written first because it is
the half that can be taken back — a user is never deleted
([user lifecycle](01-identity-and-authority.md#user-states)) — and it is removed again if
the daemon refuses. **The daemon has to be running**: with no way to create the
name, `user add` refuses rather than leaving a key that works before the name
exists. Start the daemon and run it again.

**From the web face.** Whoever may edit a User — the daemon Owner,
or an Administrator for an ordinary User — may paste that User's
`ssh-ed25519` public key on Add user or the User's page. The daemon writes the
same line `user add` writes, forced to `agent-bus-token <principal>`, under a
file lock both writers take; setup's unit passes `--ssh-keys` (the account's
`authorized_keys`) and `--ssh-token` (the program beside the installed daemon),
and without them the call answers that keys are added on the host. Only the
key itself reaches the file — the daemon parses the one line and rebuilds it,
so no option or comment is copied. A second key replaces the first; other key
types and operator keys stay host-side, and an operator's line is never
replaced from the web.

`user import-local <user[@realm]> <account>` fills a blank person name from the
OS account database. It accepts an account name rather than profile text and
preserves an existing name. Setup runs it when it installs the first user's
key; an operator may run it later when setup found no key.

`user add-local <user[@realm]> <account>` creates a User with no key and maps
that local account's socket to it, so the socket is its only way in; the
account must exist on the host, and a User or mapping already in place is not
a failure. Setup no longer runs it for a runner: since 0.8.149 a person's runner
holds its own token ([a person's runner](#a-persons-runner)).

The implemented admin verbs are `user add`, `user add-local`, `user import-local`, `user list`,
`user remove`, `account list`, `account set`, `account remove`, `token` and
`tls`. A forced command answers `token --fingerprint` with no name or credential.
The token operation is delegated to the token helper with the key entitlement
and original request kept separate: `token --rotate` rotates that identity;
asking for another identity is refused. Console and SSH use the same program. Bundle administration and regeneration of keys are not
part of this grammar.

## Administrator name migration

Upgrading from the old administrative name migrates membership to the
[protected Administrator group](01-identity-and-authority.md#groups).
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

From 0.6.0 `kind` is a closed set ([records](03-records.md#record-kinds)).
`generic`, `topic` and the `mode` field are gone, and **a stored record holding
any of them is ignored at start and reported** by name, never converted
([what refuses the start](constitution.md#what-refuses-the-start)). There is no
compatibility obligation before 1.2, so nothing is migrated at load time.

Upgrading a node that has run an earlier line therefore means editing its
snapshot while the daemon is stopped: the procedure, the mapping and what it
was verified against are in the
[cutover runbook](../Plans/done/R0.8-MVP/cutover.md#the-procedure). Two things to expect
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

Before upgrading, record Owners should explicitly grant any intended peer
access, including access to reply inboxes. `*` opts into broad sharing; do not
add it merely to silence a refusal. This release also stops implicit master
access to empty-ACL resources. The daemon Owner can discover and manage those
resources through separate [node-wide authority](01-identity-and-authority.md#daemon-owner),
but still cannot use their message interface without an ACL grant.

Fresh automatic registrations by faces and launchers also follow the default;
configured sharing survives [metadata re-registration](01-identity-and-authority.md#registration).
The dedicated runner states grants with [its options](06-dedicated-runner.md#script-agents).

## Owner ACL and master removal

From 0.5.74, `@owner` in an ACL means the record's direct Owner and the agents
directly owned by that Owner. It is evaluated from current
registry ownership and is never stored as a group. The `ab-claude`, `ab-codex`,
`ab-opencode` and `ab-opencode1` launchers add it while preserving existing
explicit grants; already-running launcher sessions acquire it when next
restarted.

`@owner` has no stored-group form. Creating or nesting it is refused, and a
snapshot containing a group with that reserved name fails closed.

The same release removes `--master`, `no_master` and the implicit master ACL
grant. A daemon Owner still
discovers and manages every resource through
[node-wide authority](01-identity-and-authority.md#daemon-owner), but message access
now comes only from record authority or the record's ACL.

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

The web face needs **bun at `/usr/bin/bun`**; its unit executes that path and
the libraries `ldd` reports for it, nothing else
([processes § the web face](08-processes.md#the-web-face)). A missing bun is
reported and the daemon still installs, without its web face. No setuid helper
or capability is granted; the face's listener keeps the host's unprivileged
port rules.

**The web face.** Setup creates the `agent-bus-web` account with the real
code directory as its home, and fills that directory into the release's unit:
`/usr/local/lib/agent-bus/current/web` for a release, or the checkout's `src/web`
for development. `WorkingDirectory`, `ExecStart` and `HOME` name it directly;
there is no `/var/lib/agent-bus/web` link. Setup computes `ExecPaths` for this
host, enables and restarts the unit and waits for `/healthz`. `--upgrade`
refreshes it; `release.sh` restarts it after a switch. Development uses
`sudo src/web/install-dev.sh`.

**Built:** `src/package.sh [output-directory]` creates one version/platform/
architecture archive and a portable SHA-256 file. The archive contains all six
Go programs, the MCP face, the web face's sources and unit, four runtime
launchers, their shared adapter, bundled agent runtimes, license, exact artifact manifest and
standalone `INSTALL.md`. Build it, verify
and unpack it, then run `sudo ./agent-bus-setup`. Setup validates the complete
manifest before changing the host, installs a digest-addressed release under
`/usr/local/lib/agent-bus`, creates stable commands under `/usr/local/bin`,
creates the accounts and tree, writes/enables the daemon and web units,
and installs the first user's key through the admin program. The manifest names
every file under `web/`, `agents/` and `clients/`, each checked and installed.
The agent tree includes Slack/Telegram bundles, native agent scripts and installers,
their shared Python metadata helper, and the Python client with its MIT license.
They keep their relative paths under the installed release, so no source checkout
is needed. Native agents require Python 3; their external tools are listed in the
[agent guides](agents/README.md#choose-an-agent). To build an archive, install the
MCP and TypeScript client dependencies first (`bun install --cwd src/mcp` and
`bun install --cwd src/clients/typescript`). `--upgrade` holds the
release it replaces to that release's own manifest, so one from before 0.8.50,
with the Go web binary and no `web/` tree, remains a rollback target. Without root it
refuses and prints the command to run. `--dry-run` and `--print-unit` are
read-only and need no root.

**The release ships its own install script and nothing else.** The tar archive is a
transport for that script, not a second installer. There is no `npm install`
path or package registry; other distribution is
[R1 work](../Plans/R1.0-Release/distribution.md#container-runtime). `INSTALL.md` leads with the git install; its release-archive section is the
packaged path exercised by [installation acceptance](#installation-acceptance).

**Development install — how the development node runs.** `sudo src/git-install.sh`
builds the checkout and symlinks its programs and launchers into
`/usr/local/bin`; the web face's unit and account name the checkout's
`src/web` directly. The CLI and every tool are then always the checkout's. A change goes
live by committing it, then running `src/build.sh && sudo systemctl restart
agent-busd`, and `agent-bus-web` too after a web edit; back up the database
first when the daemon changes.
- **Checkout location:** it must live outside `/home`, since the daemon runs
  behind `ProtectHome=yes` and cannot exec a binary in a home directory. The
  script refuses one that does and says where to move it.
- **Undoing it:** `--revert` copies real binaries back.
- **State:** daemon state under `/var/lib/agent-bus` is untouched, except by
  `--reinstall`.

**Deploying a fixed release.** `sudo src/release.sh [commit]` is for a node
that runs a release copy rather than the checkout.
- **How it builds:** from that commit (HEAD by default) through `git archive`,
  never the working tree, into
  `/usr/local/lib/agent-bus/releases/<version>-<sha>`.
- **How it switches:** it moves `/usr/local/lib/agent-bus/current` to that
  release with one rename, points `/usr/local/bin` through `current`, restarts
  the daemon and checks it reports that version. The last five releases are
  kept, and `--rollback` switches to the previous one.
- **Not on the development node:** there it would replace the checkout's links
  with a release copy. `src/git-install.sh` puts them back.

**Reinstall.** `agent-bus-setup --reinstall` (and, on a checkout,
`src/git-install.sh --reinstall`) is the one explicit way to start a node over:
it stops the daemon, moves its whole home and the unit's drop-ins into a
root-only `daemon.before-0.7-<time>` directory that nothing reads again, and
installs fresh — new database, generated unit, the installer as Owner. Every
old credential stops working. It replaces whichever release is installed,
0.6 included; the [reinstall procedure](../Plans/done/R0.8-MVP/0.7-cutover.md#procedure)
owns the steps. It refuses while any process outside systemd
holds a file in the daemon home, and a failure after the set-aside puts the
home, drop-ins, unit and release back and restarts a node that was running.

**Setup over a running node.** Plain setup restarts a running
daemon when the unit it writes differs or the running release and
`build_info` are not the installed program's, then waits, as `--upgrade`
does, until `/identity` reports that release and build. It prints the node's
durable Owner, which `--owner` only seeded
([daemon ownership](#daemon-ownership-upgrade)).

The current link changes by one atomic rename after the complete release and
all stable command links exist. A failed first install is recovered by running
the same setup again; an identical release is reused. Plain setup refuses to
replace a different release. The standalone package instructions define the
`--upgrade` and interrupted `--recover` paths; their populated-node behavior is
accepted under [H.1.1](../Plans/done/R0.8-MVP/done/upgrade-recovery.md#checks).

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
must exist. The daemon Owner may map any principal. An Administrator may map only their own User identity or an Agent they own; they may not map another User. The daemon account's own socket is implicit and cannot be reassigned
or removed. A successful edit is in the database before it is acknowledged,
but listeners belong to the supervisor: the command reports `restart required`
until a full daemon restart applies the desired map. A bus-child restart alone
keeps the existing listeners.

The first current start seeds this map from setup's `--user` flags. Once the
database carries the establishment marker, the stored map is authoritative;
changing or retaining an old flag cannot restore a removed mapping. Startup
rejects malformed mappings, duplicate accounts, missing OS accounts and any
attempt to put the implicit daemon account into the editable map. Retired socket
paths are removed when the new supervisor starts.

**Downgrade:** an older daemon ignores the durable map and returns to its
configured `--user` flags until the current version returns. Review those flags
before rollback; they may describe an earlier identity assignment.

## Installation acceptance

**Fresh installation is built and accepted.** A disposable host with real
systemd and the system bun installs from only the archive and standalone
instructions, starts the generated daemon and web units and completes a real
call over the bus. Removing a daemon binary, MCP face, web face or launcher face separately
fails before accounts, state, unit or current release exist. The retained
[H.1 evidence](../Plans/done/R0.8-MVP/done/fresh-install.md#checks) records the host and
claim limits.

**Upgrade/recovery is built and accepted.** The installer verifies and stages
the next release while the old daemon serves, then stops cleanly, backs up the
whole daemon home, switches `current`, starts and checks both public identity
and the unit's real process tree. The existing unit and drop-ins remain byte
identical. A failed start restores the prior release with its matching database,
credentials and SSH authorization; an interrupted transaction leaves a durable
marker for the documented `--recover` command. The retained
[H.1.1 evidence](../Plans/done/R0.8-MVP/done/upgrade-recovery.md#checks) covers credentials,
registry, queued state, ACLs, local mappings and operator configuration.

The fresh-install and reinstall gates compare every installed command's and
the running node's `build_info` with the archive's own, not only VERSION; the
[H.10 evidence](../Plans/done/R0.8-MVP/done/setup-hardening.md#checks) lists their checks.

The fresh-install gate also runs [sample data](#sample-data) and [TLS](#tls)
on the installed node.

<details>
<summary>Samples and TLS in the fresh-install gate</summary>

| Step | Checked |
|---|---|
| `--samples`, twice | idempotent; the web face shows them; `--remove-samples` takes exactly them away |
| `--tls self-signed` | daemon TLS beside plain HTTP; the pin reaches it and a wrong pin is refused; the web face serves its own copy, plain HTTP there is a `301` while `/healthz` stays plain, and the session cookie is `Secure` |
| `--tls off` | the daemon unit's `--tls-dir` and the web drop-in are gone; plain HTTP answers |

The real-browser gate has a `TLS=1` variant ([browser acceptance](05-discovery.md#browser-acceptance)).

</details>

SSH onboarding is accepted through an actual sshd installation for both
ordinary and operator keys, including the documented token command,
entitlement enforcement and restricted access. Checking generated
`authorized_keys` text alone is insufficient. The [SSH onboarding evidence](../Plans/done/R0.8-MVP/done/ssh-onboarding.md#checks) records the real-sshd exercise and its environment limits; its [0.7 re-run](../Plans/done/R0.8-MVP/done/ssh-onboarding.md#07-re-run) adds realm-less names, two-token rotation and credentials surviving a restart.

[Backup and restore](#backup-and-restore) is accepted on packaged hosts.

[H.1.1](../Plans/done/R0.8-MVP/done/upgrade-recovery.md#checks) records completed upgrade/recovery acceptance;
[H.5.2](../Plans/done/R0.8-MVP/done/ssh-onboarding.md#checks) records the completed SSH exercise.

## Build information

Building the daemon and CLI needs cgo and a C compiler for
[processes § process titles](08-processes.md#process-titles).

`src/build.sh [output-directory]` builds all Go programs with one stamp;
the output directory is relative to `src/` and defaults to that directory.
The build supplies `build_info` as `user@hostname YYYY-MM-DD HH:MM:SS`, using
the builder's local time, through Go linker flags; `release.sh` appends the
commit's short SHA. It never rewrites source.
Every Go program's `--version` prints the shared SemVer on the first line
and `build_info: <stamp>` on the second; no daemon connection or privileges
are needed. `--help` on each Go program opens with `<program> <version>`, on
stdout, exit 0; bare `agent-bus` prints the same text on stderr and exits 1.

A direct `go build` reports `development (unstamped)` instead of inventing a
build date. The interpreted MCP and web faces report the shared SemVer only,
read from `VERSION`.
Release builds follow [working rules § versioning](../CLAUDE.md#versioning).

## Local users

Setup asks for two things per user, and they are **configuration, not
credentials**:

| Config field | Purpose |
|---|---|
| local account | the OS account on this host; resolved to its uid |
| bus username | the principal it *is* — `parf@localhost`, `parf@github`, `parf@realmo` ([identity § names](01-identity-and-authority.md#names)) |

From that the daemon opens one socket per user
([access § local socket](02-access.md#local-socket)) and knows who is calling
on every request.

Result: a bus that **serves every user on the host at once**, so
record ACLs apply per user with nothing for anyone to configure. Setup's
initial principal is the first daemon Owner; after transfer, the stored current
Owner holds node-wide management authority
([identity § daemon owner](01-identity-and-authority.md#daemon-owner)).
## Storage

| Built store | Holds |
|---|---|
| SQLite database `agent-bus.db`, mode 0600, held exclusively | Daemon Owner, local-account map, users, registry, groups, credentials and issued times, queue contents, counters, each record's [key-value store](03-records-kv.md#key-value-store), its [activity history](05-discovery.md#activity-history) by day, and the clean-stop marker; an older schema is migrated forward at open |
| Memory only | Browser sessions, outstanding readers, [locks](01-identity-and-authority.md#shared-locks), uptime and recent envelope feed |

It is reached through `modernc.org/sqlite` behind the store ports. The
daemon is told where the database is with `--db`; setup runs `agent-busd --init`
once, as the daemon account, to create it, and the unit never passes `--create`.
An empty file a create left behind is finished by the next create, and
refused as empty without one; a file holding anything else is never taken for one.
Every record and user carries an internal ID: stable, persisted,
never on an answer, and never handed out twice, because the database keeps each
high-water mark rather than deriving it from what is left.
Other database backends are [R1.1 work](../Plans/R1.1/storage.md#backends).
Durability is defined in [messaging § durability](04-messaging.md#durability).
SQLite state and backups retain the daemon account's private directory and file
boundary. History: the [0.7 transition](../Plans/done/R0.8-MVP/0.7-cutover.md#scope) was a
clean reinstall, reading no earlier store.

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
  `agent.d` are not part of the backup.
- A backup taken without the stop is not supported: the database file lacks
  what is still in the WAL and in memory.

**Accepted** by `src/acceptance/backup-restore.sh`: a
populated disposable host is backed up twice, an older backup restored over
newer uncleanly stopped state returns exactly that older state, and a fresh
host restored from the newer one matches its listings, owners, `config_sha`,
`secret_sha`, queued and in/out/dropped counts and credentials, then keeps
unflushed traffic over a graceful restart and loses only post-flush traffic
in a bus-child crash ([durability](04-messaging.md#durability)).

</details>

## Logs

The daemon writes the [three logs](constitution.md#logs)
under `--log-dir`, which setup makes `/var/log/agent-bus/`: owned by
`agent-busd`, group `adm`, mode `2750`, so every log the daemon creates there is
readable by `adm` and by nobody else. The files are `0640`.

| File | Written |
|---|---|
| `audit.log` | always: one line per administrative action or entity edit — actor, operation, target, result and, over TCP, the client address |
| `error.log` | always: warnings, errors and alerts; managed units also enable the [syslog copy](user/daemon.md#environment-defaults) |
| `debug.log` | only when asked for, by `--debug-log` at start or by the daemon Owner with `POST /debug {"on": true}` at run time |

Without `--log-dir` a development daemon writes them to `logs/` beside its
database. Setup installs `/etc/logrotate.d/agent-bus`: weekly, eight kept,
compressed, by copy and truncate so the daemon never reopens a file.

## The two accounts

The installer creates two system accounts, the daemon's and the web face's,
each running its own program under its own unit. A person's runner gets an
account of its own when it is installed, never by default
([a person's runner](#a-persons-runner)). The heading keeps its name for existing links.

| Account | Home | Shell | Runs |
|---|---|---|---|
| `agent-busd` | `/var/lib/agent-bus/daemon` | `/bin/sh`, for forced commands | the daemon |
| `agent-bus-web` | the [web code directory](#install), read only | `nologin` | the [web face](08-processes.md#the-web-face); owns nothing on disk |

| Directory under `/var/lib/agent-bus` | Owner | Mode | Current purpose |
|---|---|---|---|
| `daemon/` | `agent-busd` | 700 | Daemon home and its database |
| `agent.d/` | root | 755 | Shared public agent code, readable by every runner account |
| `<person>-ab-runner/` | that runner account | 700 | Its inventory, overrides, env files, tokens and work directories |

The daemon account uses `/bin/sh` because sshd runs even forced commands through
the account shell; each issued key is restricted to its named helper, without
PTY or forwarding. A runner's account has `nologin` and receives no SSH keys. Setup
repairs the daemon account’s former `nologin` default while preserving custom
shells; an operator choosing one must ensure it can execute the forced helpers.
Runtime sockets live at the [access location](02-access.md#local-socket),
not under any home.

### The two units

**Built:** `/etc/systemd/system/agent-busd.service` for the daemon and
`/etc/systemd/system/agent-bus-web.service` for the web face, whose
settings [processes § the web face](08-processes.md#the-web-face) owns. The
daemon unit no longer passes `--web` or delegates cgroup controllers.

| Daemon unit setting | Purpose |
|---|---|
| `User=agent-busd` | Run as the service account |
| Working/state directories and explicit state mode | Keep state under its own home |
| `RuntimeDirectory=agent-bus` | Recreate the runtime socket directory |
| `Restart=on-failure` | Restart a failed daemon |
| `AmbientCapabilities=CAP_CHOWN`, bounded to that capability | Own the per-user sockets without running the daemon as root |

The packaged [installed shared-host checks](../Plans/done/R0.8-MVP/done/installed-shared-host.md#checks)
run two actual mapped accounts through their own sockets, refuse both cross-
account attempts, and verify from `/proc` that only the supervisor retains the
capability.

### A person's runner

📌 A [runner](../Plans/R1.0-Release/runner.md#runner) is a person's, and
setup installs one only when asked, for one person at a time:
`sudo agent-bus-setup --runner install|uninstall|enable|disable [--for <person>]`.
Every verb looks first and does only what is missing, so a second run changes
nothing, and `--dry-run` prints the plan.

| For person `alice` | What it is |
|---|---|
| `alice-ab-runner` | a system account of its own, `nologin`, so her runner's secrets are apart from everybody else's |
| `/var/lib/agent-bus/agent.d/` | shared agents' code, root-owned, 755, readable by every runner account |
| `/var/lib/agent-bus/alice-ab-runner/` | `alice-ab-runner`'s alone, 700: its token in `runner.env`, the tokens of the agents it created, their work directories |
| `#runner/alice@<host>` | her agent on the bus, allowing `@owner`; the runner is started with its token, never with hers ([who it runs as](../Plans/R1.0-Release/runner-implementation.md#who-it-runs-as)) |
| `agent-bus-runner@alice.service` | an instance of one template, `/etc/systemd/system/agent-bus-runner@.service`: her account, only its private directory writable, `Delegate=yes`, no capability, `Wants=`/`After=` the daemon, never `Requires=`, and no start while `runner.env` is missing |

| Verb | Leaves the host with |
|---|---|
| `install` | everything above, started; `--for` defaults to whoever ran `sudo`, and must be an active bus User |
| `disable` | the unit stopped and disabled, every file kept |
| `enable` | the unit enabled and started; refused when nothing is installed |
| `uninstall` | no account, unit instance, `/var/lib/agent-bus/alice-ab-runner/` or `#runner/alice@<host>`, and no template once the last runner goes; shared `agent.d/` always stays, even with `--purge`, and the agents it created stay hers |

The runner accepts `--agents <dir>` (default `/var/lib/agent-bus/agent.d`)
and required `--host <dir>` for its private directory; `--home` is removed.
There is no `runners/` directory or per-person agent code tree.

**Unreleased development migration:** setup refuses an existing
`/var/lib/agent-bus/runners/` before doing anything. Stop the old runner, keep
a root-private tar of that tree, then move its `agent.d` to the shared path
and its `runner/` to the account-named path. Refuse any destination or code-name
clash; do not merge or overwrite it. A clash after an interrupted move may be
a partial migration: restore the old layout from the private tar and set aside
partial destinations before retrying. Make shared code root-owned and readable,
keep private state owned by the runner account at 0700, update the account's
home, remove the empty old directories, and run `--runner install --for <person>`
to refresh the unit. Start and verify before removing any backup.

Install and enable refuse an existing runner record unless it is an Agent
owned by the intended person. A lookup failure other than a missing record
aborts the plan, including uninstall, before local recovery files are removed.

The runner reaches the bus through the shared socket with its token, so no
account is mapped for it. `runner.env` is setup's alone to write and holds one
line, `AGENT_BUS_TOKEN=<token>`, a token being hex digits: systemd reads it as
the unit's `EnvironmentFile`, setup reads it with the bus's
[env-file parser](03-records-service.md#env-file-syntax), and that one line
means the same to both, so it is no second syntax to keep in step. A node with no runner still serves every face.

## Sample data

`sudo agent-bus-setup --samples` fills a running node with something to look
at, from a galaxy far, far away and from Spaceballs, each name in its own
realm; `sudo agent-bus-setup --remove-samples` takes exactly that away again.
Both write as the daemon Owner on the daemon account's socket; with
`AGENT_BUS_ADDR` set to a development daemon's owner socket they need no root.

| Kind | Samples |
|---|---|
| Users | `luke@tatooine`, `leia@alderaan`, `han@corellia`, `yoda@dagobah`, `vader@empire`, `lone-starr@druidia`, `vespa@druidia`, `dark-helmet@spaceball`, `yogurt@vega` |
| Agents | `#r2d2@naboo` (Maintainer, configuration), `#c3po@tatooine`, `#k2so@scarif`, `#dot-matrix@druidia` (Personal), `#barf@druidia` |
| Services | `death-star@empire` (Maintainers, secret, configuration), `death-star-ii@endor` (inactive), `jedi-archives@coruscant`, `spaceball-one@spaceball`, `schwartz@vega` (secret) |
| Queues | `rebel-alerts@hoth`, `jabba-debts@tatooine`, `air-supply@druidia`, `falcon-repairs@corellia` (Personal), `droid-inbox@tatooine` (Deliver-To route to `#r2d2@naboo`) |
| PubSub | `imperial-news@empire`, `spaceballs-merch@spaceball`, `jedi-journal@dagobah` (Personal) |
| Groups | `@rebels`, `@jedi`, `@empire`, `@spaceballs` |

| Rule | |
|---|---|
| Idempotent | a second `--samples` changes nothing and sends no message twice |
| Real data | a name that exists without its sample description — or a user with another person name — is left alone, on adding and on removing, and named |
| Removal | sample records are drained and unregistered; sample groups are emptied, which is how a group retires; sample users are deactivated, since the daemon never removes a user. `--samples` again brings them back |

## TLS

Setup asks, at a terminal, whether to enable SSL on the daemon's port, but
only when the node has neither TLS nor an earlier `--tls off`, and never with
`--upgrade`, `--recover`, `--dry-run` or `--print-unit`. Yes means a certificate
it generates or one you provide, chain included. Otherwise the flags decide,
and without flags a node keeps what it has. An unreadable TLS directory fails
with its permission error; setup never infers TLS-off from it or widens its modes.
Writing TLS files restarts a running daemon even when its unit and release are
unchanged. Reinstall sets the TLS directory aside with the old state and restores
it on rollback. The port answers TLS beside plain HTTP
([remote access § over HTTPS](02-access-remote.md#over-https)).

| Flag | |
|---|---|
| `--tls self-signed` | generate an ECDSA P-256 certificate, valid ten years, for `localhost`, `127.0.0.1`, `::1`, the host's names and addresses, and each `--tls-name`; run again, it generates a new one, so clients fetch the fingerprint again |
| `--tls files --tls-cert c --tls-key k [--tls-chain ch]` | check that the key is the certificate's, that the chain parses and that the certificate has not expired, then copy them |
| `--tls off` | plain HTTP only, recorded as a `disabled` file in the directory, so later runs keep it so until `--tls` turns it on again; the files stay where they are |
| no `--tls` | keep what the node has: a re-run and `--upgrade` never drop TLS; `--tls` itself is refused with `--upgrade` or `--recover` |

<details>
<summary>Files, modes and what setup prints</summary>

- **Where:** `/etc/agent-bus/tls/`, `root:agent-busd 0755`, so any user can read the public certificate.
  Setup and upgrade repair older directory modes without rotating the certificate.
- **Files:** `cert.pem` and `chain.pem` `0644`; `key.pem` `root:agent-busd 0640`.
  Each is written aside and renamed into place. A choice without a chain
  removes an old `chain.pem`, which would otherwise be appended to the new
  certificate.
- **Unit:** it carries `--tls-dir /etc/agent-bus/tls` exactly while TLS is on.
  A present directory that does not load stops setup before the unit is
  written.
- **Output:** setup ends with the fingerprint and the client settings,
  `AGENT_BUS_ADDR=https://<host>:<port>` and `AGENT_BUS_TLS_FINGERPRINT=sha256:…`.
- **The web face:** it gets its own copy in `/etc/agent-bus/web-tls/`,
  `root:agent-bus-web 0755`, with the key `0640`, so neither account reads the
  other's key.
  - A drop-in, `agent-bus-web.service.d/tls.conf`, names that copy
    (`AGENT_BUS_WEB_TLS_DIR`).
  - Its port then serves TLS and redirects plain HTTP there to `https://`.
  - Setup waits for its health over `https://`, pinned to the copy.
  - `--tls off` removes the drop-in, and a re-run or `--upgrade` keeps what
    the node has.

</details>

## Config locations

Current daemon configuration comes from command flags and environment; setup
writes the flags into its unit. The per-program defaults are in
[daemon source](../src/cmd/agent-busd/main.go). A general configuration-file
format is not implemented. Per-record ACL editing is available through the
[web face](../src/web/web-face/records.md#settings).

## Status

| MVP | Scope |
|---|---|
| Built | Distributable archive, installer, administration, token helper, accounts, daemon and web units and stamped builds; fresh-host installation, populated upgrade/recovery, [backup/restore](#backup-and-restore) and installed [runtime](../Plans/done/R0.8-MVP/done/fresh-host-runtime.md#checks) acceptance; optional [TLS](#tls) on the daemon port and the web port. The TypeScript face passed installed [browser acceptance](05-discovery.md#browser-acceptance); `TLS=1` walks it over `https://`. |

## Post-deploy check

Run `src/check-deploy.sh` after each deploy; its optional target overrides the
default `#debug@<realm>` derived from `agent-bus status` (or the host name if no
realm can be read). For a specific instance, use `src/check-deploy.sh '#debug@<node>'`, as a caller allowed by debug’s `@owner` ACL. A failed or missing selftest fails the check; see [the bus doctor](agents/debug.md#check-the-bus-in-a-minute).


## Deploy only the changed component

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

The [build stamp](#build-information) is required for shipped Go binaries.
A live Git checkout is never a reset/clean/push/autopull test fixture.
