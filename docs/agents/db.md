# Database: let your AI run SQL without the password

📌 **TL;DR:**
- **Problem:** your AI or a teammate needs numbers from a database that lives elsewhere, and sharing the password gives away everything that account can do, forever.
- **Outcome:** they ask SQL questions through a door to one database account; the password stays on the server and the database itself decides what each caller may do.
- **Start:** ask what the helper can do: `agent-bus help '#mysql/billing-ro@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 **Sharing a password gives everything away.** Now the password stays on the server; callers get a door to one account.
- 🧱 **An AI must not change data by accident.** Now a read-only account refuses any write, and it is the database that refuses.
- 📈 **Questions about live data wait for someone.** Now an AI answers "how many unpaid invoices?" itself.
- 🧩 **MySQL, PostgreSQL and ClickHouse all differ.** Now one call works for all three.

## What it does

- ✅ **Run one statement:** you get columns and rows, or how many rows changed (`query <SQL>`).
- ✅ **Answer business questions:** a count or a few rows, for an AI, a script or a teammate.
- ✅ **Mark an invoice paid:** a read-write agent returns the number of rows changed.
- ✅ **Safe exploration:** a read-only agent; any write is refused by the database.
- ✅ **Capped results:** long results are cut at `max_rows` and the answer says so.
- ✅ **Best practice:** one agent per real database account, such as `#mysql/billing-ro` (read only) and `#mysql/billing-rw` (read and write); who may read or write is set by [roles](#who-may-use-it).
- 🚫 **Not for:** bulk export or big data moves (results are capped), long transactions over several calls (one statement per call), or searching texts by meaning (use [RAG](rag.md#what-it-does)).

## You'll need

- A running database, and a database account for each access level you want to offer.
- A runner with Python 3; the package brings its own drivers.

<details>
<summary>Network</summary>

A nonempty `needs.network` in `agent.json` enables network access (it is not a host filter; replace `database` with your host). A unix socket needs none.

</details>

## Try this first

- Ask the helper what it can do:

```sh
agent-bus help '#mysql/billing-ro@node.example'
```

- Then send one harmless `query` of `SELECT 1`.

## Use examples

- **Count unpaid invoices:** ask the read-only agent for a `query` with a `SELECT count(*)` statement.
- **Peek at recent rows:** ask it for a `SELECT` with a `LIMIT`.
- **Mark an invoice paid:** ask the read-write agent for an `UPDATE` statement.
- **Call from a script as JSON:** send `{"method":"query","sql":"..."}`.
- **The same UPDATE through the read-only agent** is refused by the database.
- **On a fallback instance,** grant roles in its access list, for example `bob(write)`, `ops(admin)`.

## Installing it

- An administrator installs the package once per database account, sets where the database is and puts the account's password in the runner's private file.
- It is installed from a checkout, never from a release: the release archive does not ship its native drivers.

<details>
<summary>Step by step</summary>

- Build with `src/agents/db/package.sh <agent.d/db>`.
- It copies `pymysql` (MIT) and `psycopg` 3 (LGPL-3.0-only, [psycopg.org](https://psycopg.org)) from the packaging host's Python into `support/lib`, with their licenses and a `NOTICE`; replace either by installing another build there.
- Enable it in the [runner inventory](../runner/configuration.md#inventory); one instance serves one database.

</details>

## Configuration

- **What:** which kind of database, where it is, and how many rows to return at most.

<details>
<summary>Keys</summary>

| Key | Meaning |
|---|---|
| `engine` | `mysql` (also MariaDB), `postgres` or `clickhouse` |
| `host`, `port` | Where the database listens; the engine's usual port when absent |
| `socket` | A unix socket instead of host (MySQL, PostgreSQL) |
| `database` | The database name |
| `secure` | ClickHouse over HTTPS |
| `max_rows` | Rows returned at most, 1 to 10000, default 1000 |

</details>

## Secret keys

- **Database logins live only** in the private host `db/env` file ([environment files](../runner/configuration.md#environment-files)), never in Configuration or a call.

<details>
<summary>Keys</summary>

| Key | Meaning |
|---|---|
| `DB_USER`, `DB_PASSWORD` | One account for every caller |
| `DB_READ_*`, `DB_WRITE_*`, `DB_ADMIN_*` (`_USER`, `_PASSWORD`) | One account per tier. Set any `_USER` key and the instance is per-tier: a tier without its account is refused, never served by a stronger one |

</details>

## Who may use it

- **Preferred: one agent per real SQL user.** Create database accounts with the rights you want and run one agent per account, named for it; the access list says who may call each, the database enforces what each may do.
- **Fallback: roles on one agent,** only when you cannot create normal SQL users with correct permissions; give `read` its own read-only account (`DB_READ_*`) if you can.
- **No role means read only:** the opposite of [`#qdrant`](qdrant.md#who-may-use-it), where no role means write.
- **The highest role held picks the tier** ([roles](../03-records-agent.md#roles)):

| Role on the message | Tier | May run |
|---|---|---|
| none, `read`, or anything else | read | SELECT only |
| `write` | write | also INSERT, UPDATE, DELETE |
| `admin`, `owner`, `maintainer` | admin | DDL and everything else the account may do |

- **The database refuses a write from `read`;** the agent never parses SQL.
- **`write` and `admin` are separated only by accounts:** with one account both run with its full authority, and the roles limit what the agent tries, not what the database allows.
- **A reader can read whatever its account can,** including system tables: never point `read` at a superuser account for an untrusted caller.

## Troubleshooting

| If you see… | Means |
|---|---|
| `error: no <tier> account is configured on this instance` | Per-tier accounts are in use and that tier has none; add its keys or use one account |
| `error: database connection or authentication failed` | Host, port or password is wrong; the answer never says which or names the account |
| `error: database request timed out` | The deadline passed; a write may have happened, check before retrying; the database also stops the statement on its own limit |
| A database error | Returned unchanged, minus the password |

- Logs hold only the engine, tier, byte count and status, never the SQL or password.

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#db` reports these.

| Level | When | What to do |
|---|---|---|
| alert | The database refuses the connection or the login | Check `host`, `port` or `socket`, and the account in the host `db/env` file |
| alert | Configuration is invalid, or the engine's driver is missing from the package | Fix `engine` and the address; reinstall the package |
| warning | A statement reached its deadline | Check the database load; a write may have happened |
| warning | A caller was refused because their tier has no account on this instance | Add that tier's keys, or stop granting that role |

## Technical details

<details>
<summary>How read-only is enforced, and what it does not stop</summary>

The agent never parses the SQL to decide. `read` is enforced by the database, which returns its own error for a write:

| Engine | `read` is enforced by |
|---|---|
| MySQL, MariaDB | `START TRANSACTION READ ONLY` plus a read-only session, always rolled back; multi-statements are off |
| PostgreSQL | a read-only transaction plus a read-only session default; the extended protocol, so a second statement is refused by the server |
| ClickHouse | `readonly=1` sent with every request, which also stops a statement changing it |

- Each call opens and closes its own connection.
- One statement per call is part of the guarantee: do not replace the PostgreSQL extended protocol or enable MySQL multi-statements.
- Fixture checks show each refused statement succeeding under `admin`, so a refusal is the database's.
- MySQL itself is not in the fixture: checks run against MariaDB (same protocol and read-only transaction), PostgreSQL and ClickHouse; run them against your MySQL version before relying on it.
- A read-only transaction does not stop what a privileged account may do on its own connection: PostgreSQL `dblink_exec`, `COPY … TO`, file functions; MySQL `SELECT … INTO OUTFILE` with the FILE privilege; ClickHouse `url()`, `file()`, `remote()` with those grants.
- It can also read tables such as `mysql.user`; the fixture checks show the limit.

</details>
