# T-38 S3: live acceptance for #dbsync (spec, no build)

📌 **TL;DR:** S3 is proven on fixtures and the fake #db only. A later live run needs four things from the owner: one engine, a read-only account, one probe table, and a #db instance that points at it. Nothing on this node is built or started by this spec.

Facts, 2026-11-06: no PostgreSQL or MySQL server runs on this node (only the `psql` and `mysql` clients; `/var/lib/mysql` is empty); `#db` is disabled, with no record and no credentials.

- ❓ LIVE-1 Which engine the live acceptance runs on
  - Context: both dialects pass the fixture engines in normal. PostgreSQL's `timestamptz` exercises the time-zone guard and mixed-case identifiers, which the fake #db only imitates
  - ( ) PostgreSQL, a local instance on this node (Fedora `postgresql-server`, loopback only) 🌟
  - ( ) MySQL/MariaDB, a local instance on this node ⭐
  - ( ) An existing database the owner names (host, engine, schema)
  > 💡👾 PostgreSQL: the zoned `updated` path is the riskiest one the fixtures cover, and a loopback-only local instance keeps the test off any shared data.

- ❓ LIVE-2 The read-only account
  - Account: `dbsync_ro`, login over loopback only, password in the #db instance's Configuration secret, never in a file under the repo
  - Grants: `CONNECT` on the database, `USAGE` on the schema, `SELECT` on `dbsync_probe` only; `ALTER ROLE dbsync_ro SET default_transaction_read_only = on` (MySQL: `GRANT SELECT ON probe.dbsync_probe`, nothing else)
  - Writes come from a second account, `dbsync_rw`, used by hand for the test steps only
  - ( ) Two accounts as above, read for #db and write for the test steps 🌟
  - ( ) One read-only account; the owner makes the row changes as a superuser ⭐
  > 💡👾 Two accounts: #db never holds a credential that can write, and the test steps do not need a superuser.

- ❓ LIVE-3 The probe table
  - Shape: `dbsync_probe(id bigint primary key, updated_at timestamptz(6) not null default now(), note text, gone boolean not null default false)`
  - `updated_at` is set on every update by a `BEFORE UPDATE` trigger (MySQL: `timestamp(6) … on update current_timestamp(6)`)
  - DBSync Configuration: `table dbsync_probe`, `key id`, `updated updated_at`, `columns note`, `deleted gone`, `deleted_value true`, `every 10s`, `lookback 2m`
  - ( ) This one table, in its own `probe` schema 🌟
  - ( ) The same table in the default schema ⭐
  > 💡👾 Its own schema: the grant stays on one table and dropping the schema removes the whole test.

- ❓ LIVE-4 Where #db points
  - Instance: `#db/probe@<node>` → `127.0.0.1:5432`, database `probe`, account `dbsync_ro`; it grants `#dbsync/probe@<node>` its `read` role and nothing else
  - Consumer: one queue `dbsync-probe` as the only subscriber, read with `agent-bus consume`
  - ( ) A new `#db/probe` instance for this test only, removed after 🌟
  - ( ) Point the main `#db` instance at the probe database ⭐
  > 💡👾 A separate instance: enabling it is an owner step (record, credential, runner reload) and removing it leaves the main #db untouched.

## Run, once the four are answered

| Step | Expect |
|---|---|
| `sub add dbsync-probe` on an empty table, then insert ids 1–3 | three `changed` messages in key order; `status` shows `scan` at id 3 and 0 held after confirms |
| update id 2 | one `changed` for id 2 with the new `updated` |
| a late commit: open a transaction, insert id 4, insert and commit id 5 elsewhere, then commit id 4 within `lookback` | id 4 arrives once, after id 5 |
| set `gone = true` on id 1 | one `deleted` for id 1, without columns |
| `sub add dbsync-probe2 replay:false` | no rows from before it; the next insert reaches both |
| stop confirming on one queue past `park_after` | that subscriber `parked`, the other keeps receiving |
| `UPDATE` as `dbsync_ro` by hand | refused by the database: the read account cannot write |

Owner-only steps: installing the database server, creating the accounts, registering `#db/probe` and `#dbsync/probe`, handing their tokens to the runner, the runner reload, and removing all of it after.
