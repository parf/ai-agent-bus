# Setup and operation

## Install

| Step | What happens |
|---|---|
| `npm install -g agent-bus` (or `pnpm`) | one package brings `agent-busd` and the `agent-bus` CLI |
| `agent-bus setup` | creates the **`agent-bus` system user**, asks the two questions below, writes the config. **No keys.** |
| start the service | `systemd` where present (`agent-busd.service`), otherwise whatever the host has; the runner supervises the rest |

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
who ran setup gets **`agent-bus-admin`** in the master ACL
([identity § acl](01-identity.md#acl)). The same three steps on a team node
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

## The service account

**`agent-bus setup` installs the separate-user arrangement**, not the
personal one — the two are [runner § who it runs as](08-runner-role.md#who-it-runs-as),
and an install that serves more than its installer has to be the first. Setup
creates the account; nothing runs as root at any point.

Its home is **`/var/lib/agent-bus`** — state a program writes, which is what
`/var/lib` is for and what every other daemon account on a host uses. `/usr`
is read-only shareable program data, so `/usr/lib/agent-bus` cannot hold a
home, a store or a dump.

| Under it | Holds |
|---|---|
| the store | tokens and whatever else is decided ([storage](#storage)) |
| the dumps | queues and stats across a restart ([messaging § durability](04-messaging.md#durability)) |

The per-user sockets are **not** under it: they belong in the host's runtime
directory, which a reboot clears ([access § local socket](02-access.md#local-socket)).

## Config locations

`/etc/agent-bus/` · `~/.config/agent-bus/` · unit `agent-busd.service`.
