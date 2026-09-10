# Setup and operation

## Install

| Step | What happens |
|---|---|
| `npm install -g agent-bus` (or `pnpm`) | one package brings `agent-busd` and the `agent-bus` CLI |
| `agent-bus setup` | creates the **`agent-bus` system user**, asks the two questions below, writes the config. **No keys.** |
| start the service | `systemd` where present (`agent-busd.service`), otherwise whatever the host has; the runner supervises the rest |

Implementation: **Go** first (`agent-busd`, `agent-bus`); a bun/NPM build
later. Client libraries: Go, PHP, Rust, JS, Python.

❓ The install line is npm while the first build is Go — one of the two is
wrong for the first release. *Settled by:* owner.

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

SQLite by default (single file, zero ops); MySQL/PostgreSQL optional behind one
thin store layer.

The git repo (over SSH) holds signed AUTH bundles — authority
([AUTH role § bundle](06-auth-role.md#bundle)) — and unsigned registry
snapshots — backup and peer sync
([services § registry sync](03-services-and-topics.md#registry-sync)).

Queues and stats are memory, dumped to Parquet
([messaging § durability](04-messaging.md#durability)).

❓ **What actually lives in SQLite** — AUTH data is git, the registry is live
records, queues and stats are Parquet. *Settled by:* owner.

## Reload

Zero-downtime reload for `agent-busd` itself via socket inheritance
(`cloudflare/tableflip`-style); 2× RAM during the overlap.

## Config locations

`/etc/agent-bus/` · `~/.config/agent-bus/` · unit `agent-busd.service`.
