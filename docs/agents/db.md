# Database

📌 **TL;DR:** Let the bus run SQL against one MySQL, PostgreSQL or ClickHouse
database without handing out its password. Make one agent per real SQL account
(`#mysql/billing-ro`, `#mysql/billing-rw`) so the database itself enforces what
each caller may do. A caller's bus role is only a fallback for when you cannot
create such accounts, and there only `read` is enforced by the database.

## Why you'd want it

Your AI or a teammate needs to look at, or change, a database that lives
elsewhere. This agent keeps the password on the server, runs one statement per
call and returns rows, with a bound on time and size.

## What it does

`query <SQL>` runs one statement and returns the columns and rows, or the number
of rows changed. Rows beyond `max_rows` (default 1000) are cut and the answer says
so. Each call opens its own connection and closes it.

The caller's role decides the tier. A caller with no role, or with a label the
agent does not know, gets `read`. This is the opposite of
[`#qdrant`](qdrant.md), where a caller with no role may write; do not copy one default to the other.

| Role on the message | Tier | May run |
|---|---|---|
| none, `read`, or anything else | read | SELECT only |
| `write` | write | also INSERT, UPDATE, DELETE |
| `admin`, `owner`, `maintainer` | admin | DDL and everything else the account may do |

The highest role held wins. See [roles](../03-records-agent.md#roles).

**What the database enforces.**

| Engine | `read` is enforced by |
|---|---|
| MySQL, MariaDB | `START TRANSACTION READ ONLY` plus a read-only session, always rolled back; multi-statements are off |
| PostgreSQL | a read-only transaction plus a read-only session default; the extended protocol, so a second statement is refused by the server |
| ClickHouse | `readonly=1` sent with every request, which also stops a statement changing it |

The agent never reads the SQL to decide. A statement that writes fails in the
database and the database's own error is the answer. A fixture check also shows
each refused statement succeeding under `admin`, so a refusal is the database's,
not a typo.

**`write` and `admin` are separated by accounts only.** One account cannot tell a
caller who may INSERT from one who may DROP. With per-tier accounts (below) the
database does the separation. With one account, `write` and `admin` both run with
that account's authority and only `read` is enforced: the roles then limit what the
agent will try, not what the database allows.

## You'll need

- A running database, and a SQL account for each tier you want to offer.
- Python 3 on the host. The package bundles `pymysql` and `psycopg` 3 (see [Installing it](#installing-it)).
- Network access: a nonempty `needs.network` in `agent.json` enables it (the list is not yet a filter; replace `database` with your host). A unix socket needs none.

## Try this first

```sh
agent-bus call '#mysql/billing-ro@node.example' 'query SELECT 1'
agent-bus call '#mysql/billing-ro@node.example' 'query SELECT id, total FROM invoices LIMIT 5'
```

A JSON call is `{"method":"query","sql":"SELECT 1"}`.

## Use examples

```sh
# a caller with the write role
agent-bus call '#mysql/billing@node.example' "query UPDATE invoices SET paid = 1 WHERE id = 7"
# a read caller trying the same: the database refuses it
agent-bus call '#mysql/billing-ro@node.example' "query UPDATE invoices SET paid = 1 WHERE id = 7"
```

Grant roles in the agent's ACL, for example `bob(write)` and `ops(admin)`
([roles](../03-records-agent.md#roles)).

## Installing it

Build with `src/agents/db/package.sh <agent.d/db>`. It copies `pymysql` (MIT) and
`psycopg` 3 (LGPL-3.0-only, [psycopg.org](https://psycopg.org)) from the packaging
host's Python into `support/lib`, with their licenses and a `NOTICE`; replace
either by installing another build there. Enable the package in the
[runner inventory](../runner/configuration.md#inventory). One instance serves one
database.

## Configuration

| Key | Meaning |
|---|---|
| `engine` | `mysql` (also MariaDB), `postgres` or `clickhouse` |
| `host`, `port` | Where the database listens; the engine's usual port when absent |
| `socket` | A unix socket instead of host (MySQL, PostgreSQL) |
| `database` | The database name |
| `secure` | ClickHouse over HTTPS |
| `max_rows` | At most this many rows, 1 to 10000 |

## Secret keys

Put them in the private host `db/env` file, never in Configuration or a call.

| Key | Meaning |
|---|---|
| `DB_USER`, `DB_PASSWORD` | One account for every caller |
| `DB_READ_*`, `DB_WRITE_*`, `DB_ADMIN_*` (`_USER`, `_PASSWORD`) | One account per tier. Set any of the `_USER` keys and the instance is per-tier: a tier without its account is refused, never served by a stronger one |

## Who may use it

**Preferred: one agent per real SQL user.** Create the database accounts with the
grants you want (a read-only one, a read-write one) and run one agent instance per
account, named for it. The ACL then says who may call each instance and the
database enforces what the account may do. The role mapping above adds nothing
you need.

**Fallback: roles on one instance.** Use it only when you cannot create normal SQL
users with correct permissions. Give the `read` tier a separate read-only account
if the database lets you; then `DB_READ_*` makes it enforced twice.

**A `read` caller can read whatever its account can.** The read-only transaction
stops writes inside the transaction, not what a privileged account may do on its
own connection: PostgreSQL `dblink_exec`, `COPY … TO`, file functions; MySQL
`SELECT … INTO OUTFILE` with the FILE privilege; ClickHouse `url()`, `file()`,
`remote()` with those grants. It can also read system tables such as `mysql.user`.
Never point `read` at a superuser account for an untrusted caller; give it an
account without those privileges. The fixture checks show the limit.

One statement per call is part of the guarantee: do not replace the PostgreSQL
extended protocol or enable MySQL multi-statements.

MySQL itself is not in the fixture: the checks run against MariaDB (the same
protocol and the same read-only transaction) with PostgreSQL and ClickHouse in
containers. Run them against your MySQL version before relying on it.

## Troubleshooting

- `error: no <tier> account is configured on this instance`: per-tier accounts are in use and that tier has none. Add its keys or use one account.
- `error: database connection or authentication failed`: the host, port or password is wrong. The answer never says which, and never names the account.
- `error: database request timed out`: the call deadline passed; an uncertain write may have happened, check before retrying. The database also stops the statement on its own limit.
- A database error comes back unchanged, minus the password. Logs hold only the engine, tier, byte count and status, never the SQL or the password.
