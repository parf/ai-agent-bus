# Setup and operation

## The five programs

Privilege is what separates them, and nothing else does: root is needed once
and never again, the account's own files are the admin's, and what an ordinary
user needs is neither.

| Program | Runs as | What it is for |
|---|---|---|
| `agent-bus-setup` | **root**, and refuses otherwise, printing the `sudo` line to run | creates the **two accounts** ([the two accounts](#the-two-accounts)) and their homes, chowns them, writes and enables the unit, then hands over to `agent-bus-admin` for the first user |
| `agent-bus-admin` | the **`agent-busd` account**; re-runs itself under `sudo -u agent-busd` when it is not | everything that edits what lives in the home — `user add`, `user list`, `user remove` today — plus the `token` verb, which it hands to the program below rather than implementing twice. **Not for ordinary users** |
| `agent-bus-token` | **any user** | hands out a credential, and does nothing else. What an ordinary user reaches over SSH ([access § getting a token](02-access.md#getting-a-token)) |
| `agent-bus` | **any user** | the ordinary client, over the unix socket or TCP ([access § local socket](02-access.md#local-socket)) |
| `agent-busd` | the **`agent-busd` account**, started by the unit | the daemon: a supervisor and its children ([processes](11-processes.md)) |

**Reaching a node over SSH runs one of these, never a shell.** Every key lives
in the `agent-busd` account's `authorized_keys` behind a forced command, and
which command is what separates an operator from everybody else
([AUTH role § SSH admin](06-auth-role.md#ssh-admin)):

| The line says | What that key reaches |
|---|---|
| `command="…/agent-bus-token"` | `token <name>`, and nothing else |
| `command="…/agent-bus-admin <admin>"` | the admin grammar, **and the same `token <name>`** |

**`ssh agent-busd@<host> token <name>` answers the same however the key is
listed.** An operator's line is a superset, not a different path: the token
half is one piece of code both programs call, so nobody has two ways to get a
credential. What an operator's line adds is verbs — and a user who only ever
needs a token never reaches the admin program at all.

Administering a node from across the network and from its own console are
likewise one program: `ssh agent-busd@<host> <args>` and
`sudo -u agent-busd agent-bus-admin <args>` are the same thing.

❓ **Editing the ACL and the user-to-account map** needs somewhere to edit
them: today both are the daemon's command line, written once by the unit
([the two accounts](#the-two-accounts)). A file the daemon re-reads is a
shape nobody has asked for yet. *Settled by:* owner.

## Install

| Step | What happens |
|---|---|
| `npm install -g agent-bus` (or `pnpm`) | one package brings all five ([the five programs](#the-five-programs)) |
| `sudo agent-bus-setup` | creates the **two system users**, asks the two questions below, writes the config and the unit, and starts it. **No keys.** |
| the first user | `agent-bus-setup` calls `agent-bus-admin` with the installer's own public key, which is what puts a line in that account's `authorized_keys`. It does not learn a second way to write that file |

`agent-busd` and the CLI are Go; the MCP face and the push adapters are bun —
[modules § languages](10-modules.md#languages).

❓ **How a Go binary is installed by npm** — PoC runs the built binary and MVP
says `npm install -g`, so the package has to carry or fetch a per-platform
binary. *Settled by:* owner, when MVP packaging is real.

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
who ran setup **holds master** ([identity § acl](01-identity.md#acl)). The same three steps on a team node
plus `auth: on` make it an AUTH replica ([AUTH role](06-auth-role.md)).

## Storage

SQLite by default (single file, zero ops); MySQL/PostgreSQL optional behind
the `store` port — swapping one for another is a single adapter
([modules](10-modules.md)).

The git repo (over SSH) holds signed AUTH bundles — authority
([AUTH role § bundle](06-auth-role.md#bundle)) — and unsigned registry
snapshots — backup and peer sync
([services § registry sync](03-services-and-topics.md#registry-sync)).

Queues and stats are memory, dumped to Parquet
([messaging § durability](04-messaging.md#durability)).

**Tokens are durable** and belong in the store — they must survive a restart or
a reloaded queue cannot be decrypted
([access § token lifetime](02-access.md#token-lifetime)).

⚠️ Credentials are so far the *only* durable thing, and the MVP keeps them in a
text file — one line per principal, mode 0600 — behind the same port
([modules § modules](10-modules.md#modules)). A database is one more adapter
and no change anywhere inward, which is what the port is for.

❓ **What else lives in SQLite** — AUTH data is git, the registry is live
records, queues and stats are Parquet, tokens are durable. *Settled by:* owner.

## Reload

Zero-downtime reload for `agent-busd` itself via socket inheritance
(`cloudflare/tableflip`-style); 2× RAM during the overlap.

## The two accounts

**`agent-bus-setup` installs the separate-user arrangement**, not the
personal one — the two are [runner § who it runs as](08-runner-role.md#who-it-runs-as),
and an install that serves more than its installer has to be the first. It is
the one program that needs root, and it needs it once; nothing runs as root
afterwards.

Two accounts, because there are two secret domains and neither may read the
other's. The daemon holds every credential; the runner holds every managed
service's configuration. Compromising one does not yield the other, and that
is the whole argument for running the runner outside the daemon
([processes § nothing the daemon runs may exec](11-processes.md#nothing-the-daemon-runs-may-exec)).

Everything lives under **`/var/lib/agent-bus`** — state a program writes,
which is what `/var/lib` is for and what every other daemon account uses.
`/usr` is read-only shareable program data, so it cannot hold a home, a store
or a dump.

| Directory | Mode | Owner | Holds |
|---|---|---|---|
| `daemon/` | 700 | **`agent-busd`** | its home: the store, the dumps, the ACL — [storage](#storage), [messaging § durability](04-messaging.md#durability) |
| `service.d/` | 755 | `agent-bus-runner` | what each service *is*: its code or a symlink to it, `config.json`, `env.dist`. World-readable because it holds no secret, and usually a `git clone` nobody edits by hand |
| `runner/` | 700 | **`agent-bus-runner`** | its home: `autostart.json`, and the env files, one directory per service ([runner § what an instance is](08-runner-role.md#what-an-instance-is)) |

**The two are split so that updating a service touches no local state.** `git
pull` under `service.d` and every decision this host made — secrets, worker
counts, confinement, whether it runs at all — is still sitting under `runner/`
untouched.

**Neither account is one you log in as.** Both are nologin. Only the daemon's
has an `authorized_keys`, and every line in it is a forced command
([the five programs](#the-five-programs)) — never a shell. The runner has none
at all: it is reached as a service on the bus
([runner § reaching the runner](08-runner-role.md#reaching-the-runner)), so
there is no second ssh door to lock down. Confining a child on top of that
is a per-service option rather than what makes the arrangement correct, because
no secret reaches a child as a file in the first place
([runner § the three env layers](08-runner-role.md#the-three-env-layers)).

Each directory being the account's actual `$HOME` is what removes a branch:
the paths a personal run writes under your own home are the paths a system
install writes under these, with no special case in between.

The per-user sockets are **not** under any of them: they belong in the host's
runtime directory, which a reboot clears ([access § local socket](02-access.md#local-socket)).

The unit it writes is `/etc/systemd/system/agent-busd.service`, and it is the
whole privileged arrangement in one file:

| The unit says | So that |
|---|---|
| `User=agent-busd` | the daemon is never root and never the installer — the check reads the *running* process, so a developer's hand-started one fails it |
| `WorkingDirectory` and `StateDirectory` are the home | the store and the dumps land where the account can keep them, and nowhere else |
| `RuntimeDirectory=agent-bus`, mode 0711 | the sockets are outside every home and a reboot clears them ([access § local socket](02-access.md#local-socket)) |
| `Restart=on-failure` | a daemon that dies comes back |
| `AmbientCapabilities=CAP_CHOWN` with a bounding set of exactly that | the one capability is given, not taken, and no second one can be picked up ([processes § why the supervisor holds CAP_CHOWN](11-processes.md#why-the-supervisor-holds-cap_chown)) |

It refuses rather than half-installing without root, and says which `sudo`
line to run. `--dry-run` names the steps and `--print-unit` prints the unit;
neither needs anything. The installer's own account is given a socket without
being asked for — they are a user of the bus like anyone else.

## Config locations

`/etc/agent-bus/` · `~/.config/agent-bus/` · unit `agent-busd.service`.
