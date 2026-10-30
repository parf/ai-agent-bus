# Database: let your AI run SQL without the password

📌 **TL;DR:** Lets you or your AI ask a MySQL, PostgreSQL or ClickHouse database
questions (in SQL, the database query language) without handing out its
password. Best practice: one agent per real database account, such as
`#mysql/billing-ro` (read only) and `#mysql/billing-rw` (read and write), so the
database itself decides what each caller may do.

## Why you'd want it

**The problem.** Your AI or a teammate needs numbers from a database that lives
elsewhere. Sharing the password gives away everything that account can do, forever.

**The idea.** The password stays on the server; callers get a door to one
database account. **Prefer real SQL users with correct permissions**, one agent
each. Use this agent's roles only when you cannot create such users.

**When to use it**

- An AI should answer questions from live data ("how many unpaid invoices?").
- A script or report needs a few rows from a remote database.
- A teammate needs read-only access without a database login.

**When not to:** bulk export or big data moves (results are capped), long
transactions over several calls (one statement per call), or searching texts by
meaning (use [RAG](rag.md#what-it-does)).

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| AI answers a business question | Ask through `#mysql/billing-ro` | A count or a few rows |
| Morning report | A script runs a fixed SELECT | Today's numbers |
| Billing script marks an invoice paid | Call `#mysql/billing-rw` | The number of rows changed |
| Analytics for the team | A ClickHouse instance with a read-only account | Report rows |
| Safe exploration | A read-only agent | Any write is refused by the database |

## What it does

| Call | Does |
|---|---|
| `query <SQL>` | Runs one statement; returns columns and rows, or how many rows changed |

Long results are cut at `max_rows` and the answer says so. Who may read or
write is set by [roles](#who-may-use-it).

## You'll need

- A running database, and a database account for each access level you want to offer.
- A runner with Python 3; the package brings its own drivers.

<details><summary>Technical details: network</summary>

A nonempty `needs.network` in `agent.json` enables network access (it is not a host
filter; replace `database` with your host). A unix socket needs none.

</details>

## Try this first

```sh
agent-bus call '#mysql/billing-ro@node.example' 'query SELECT 1'
```

## Use examples

| I want to… | Command |
|---|---|
| Count unpaid invoices | `agent-bus call '#mysql/billing-ro@node.example' 'query SELECT count(*) FROM invoices WHERE paid = 0'` |
| Peek at recent rows | `agent-bus call '#mysql/billing-ro@node.example' 'query SELECT id, total FROM invoices LIMIT 5'` |
| Mark an invoice paid | `agent-bus call '#mysql/billing-rw@node.example' 'query UPDATE invoices SET paid = 1 WHERE id = 7'` |
| Call from a script as JSON | `agent-bus call '#mysql/billing-ro@node.example' '{"method":"query","sql":"SELECT 1"}'` |

The same UPDATE through the read-only agent is refused by the database. On a
fallback instance, grant roles in its access list, e.g. `bob(write)`, `ops(admin)`.

## Installing it

An administrator installs the package once per database account, sets where the
database is and puts the account's password in the runner's private file. It is
installed from a checkout, never from a release: the release archive does not
ship its native drivers.

<details><summary>Step by step</summary>

Build with `src/agents/db/package.sh <agent.d/db>`. It copies `pymysql` (MIT) and
`psycopg` 3 (LGPL-3.0-only, [psycopg.org](https://psycopg.org)) from the packaging
host's Python into `support/lib`, with their licenses and a `NOTICE`; replace
either by installing another build there. Enable it in the
[runner inventory](../runner/configuration.md#inventory). One instance serves one
database.

</details>

## Configuration

Which kind of database, where it is, and how many rows to return at most.

<details><summary>Keys</summary>

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

Database logins live only in the private host `db/env` file
([environment files](../runner/configuration.md#environment-files)), never in
Configuration or a call.

<details><summary>Keys</summary>

| Key | Meaning |
|---|---|
| `DB_USER`, `DB_PASSWORD` | One account for every caller |
| `DB_READ_*`, `DB_WRITE_*`, `DB_ADMIN_*` (`_USER`, `_PASSWORD`) | One account per tier. Set any `_USER` key and the instance is per-tier: a tier without its account is refused, never served by a stronger one |

</details>

## Who may use it

**Preferred: one agent per real SQL user.** Create database accounts with the
rights you want (read-only, read-write) and run one agent per account, named for
it. The access list says who may call each; the database enforces what each may do.

**Fallback: roles on one agent**, only when you cannot create normal SQL users
with correct permissions. Give `read` its own read-only account (`DB_READ_*`) if
you can. A caller with no role can only read (the opposite of
[`#qdrant`](qdrant.md#who-may-use-it), where no role means write).

The caller's role picks a tier ([roles](../03-records-agent.md#roles)); the highest held wins:

| Role on the message | Tier | May run |
|---|---|---|
| none, `read`, or anything else | read | SELECT only |
| `write` | write | also INSERT, UPDATE, DELETE |
| `admin`, `owner`, `maintainer` | admin | DDL and everything else the account may do |

The database itself refuses a write from `read`; the agent never parses SQL.

**`write` and `admin` are separated only by accounts**: with one account both run
with its full authority, and the roles limit what the agent tries, not what the
database allows.

**A reader can read whatever its account can**, including system tables. Never
point `read` at a superuser account for an untrusted caller.

<details><summary>Technical details: how read-only is enforced, and what it does not stop</summary>

The agent never parses the SQL to decide. `read` is enforced by the database,
which returns its own error for a write:

| Engine | `read` is enforced by |
|---|---|
| MySQL, MariaDB | `START TRANSACTION READ ONLY` plus a read-only session, always rolled back; multi-statements are off |
| PostgreSQL | a read-only transaction plus a read-only session default; the extended protocol, so a second statement is refused by the server |
| ClickHouse | `readonly=1` sent with every request, which also stops a statement changing it |

Each call opens and closes its own connection. One statement per call is part of
the guarantee: do not replace the PostgreSQL extended protocol or enable MySQL
multi-statements. Fixture checks show each refused statement succeeding under
`admin`, so a refusal is the database's. MySQL itself is not in the fixture:
checks run against MariaDB (same protocol and read-only transaction), PostgreSQL
and ClickHouse. Run them against your MySQL version before relying on it.

A read-only transaction does not stop what a privileged account may do on its
own connection: PostgreSQL `dblink_exec`, `COPY … TO`, file functions; MySQL
`SELECT … INTO OUTFILE` with the FILE privilege; ClickHouse `url()`, `file()`,
`remote()` with those grants. It can also read tables such as `mysql.user`. The
fixture checks show the limit.

</details>

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#db` reports these. The page that shows them is in work.

| Level | When | What to do |
|---|---|---|
| alert | The database refuses the connection or the login | Check `host`, `port` or `socket`, and the account in the host `db/env` file |
| alert | Configuration is invalid, or the engine's driver is missing from the package | Fix `engine` and the address; reinstall the package |
| warning | A statement reached its deadline | Check the database load; a write may have happened |
| warning | A caller was refused because their tier has no account on this instance | Add that tier's keys, or stop granting that role |

## Troubleshooting

| If you see… | Means |
|---|---|
| `error: no <tier> account is configured on this instance` | Per-tier accounts are in use and that tier has none; add its keys or use one account |
| `error: database connection or authentication failed` | Host, port or password is wrong; the answer never says which or names the account |
| `error: database request timed out` | The deadline passed; a write may have happened, check before retrying. The database also stops the statement on its own limit |
| A database error | Returned unchanged, minus the password |

Logs hold only the engine, tier, byte count and status, never the SQL or password.
