# Key-value store

Status: **built** in 0.8.77. The contract is the
[key-value store](../../docs/01-identity-and-authority.md#key-value-store); this
page holds the reasoning.

## Per-record storage

**The daemon keeps a store on each record, in its database, with atomic
operations on it** (owner, 2026-09-30). It gives an 👾 Agent, a 👤 User — whose
own record carries one too — or anything else on the bus somewhere durable to
put what it is doing: several workers dividing a batch, or the status of each
job, without a database of its own beside the bus.

| | |
|---|---|
| Who | the record's Owner, Maintainers and own Agent — the same authority as its [private values](../../docs/constitution.md#-private-values) and its [shared locks](locks.md#shared-locks) (Q107) |
| Three kinds | string, int and JSON, each its own table and namespace (owner, 2026-09-30). The owner's call families — `kv_get`, `kv_int_get`, `kv_json_get` and so on — are one verb with a kind on every face: `kv get --int` on the CLI, `ab_kv_get` with `kind` in MCP |
| Where it lives | the daemon's own state, behind the same persistence ports as everything else it stores: SQLite first, the other [backends](../R1.1/storage.md#backends) as they serve the rest |

**Atomic is the whole point of it.** Two workers reading, deciding and writing
back cannot divide a list between them; an `add`, an `inc` or a `shift` can,
because the daemon that holds the value is the one that decides. That is the
same argument as [shared locks](locks.md#shared-locks) — one authority a pool
already shares — applied to the value rather than to the right to act. A lock
is the daemon's memory and goes with it; a value put here is stored and does
not.

<details>
<summary>Why it is built this way</summary>

| | |
|---|---|
| Keyed by internal ID | a record's ID is never reused, so a name freed and registered again inherits nothing, and a transfer keeps the store |
| Committed before answered | a `shift` handed to one worker must not come back after a crash for another, so the queue checkpoint's "may lose a minute" does not apply |
| Not in memory | records and queues are small and hot; a store may hold many names, and SQLite's page cache already makes a read cheap |
| No timestamps | nobody needed one (owner, 2026-09-30) |
| Top-level keys only | every case works one level down, and the store's names give the next level, each its own atomic unit. A path language — JSON Pointer, `/jobs/3/status` — can be added later without breaking anything, a leading `/` opting in |
| Dividing work | `push` and `shift` are a queue: no two workers `shift` the same element. `unshift` puts a failed job back at the front, and `pop` takes the newest |
| `remove_from_set`, not `pull` | MongoDB's `pull` reads like `shift` and `pop`, which hand an element back; this answers nothing, and pairs with `add_to_set` |
| No conditional op | "only if it is still mine" is a read and a write under the record's [shared lock](locks.md#shared-locks), or data shaped so a `shift` already made it yours |
| In core, not SQL | SQLite's JSON functions can do each op in one `UPDATE` (tried 2026-09-30 on the driver's 3.53.4), but they compare minified text, so key order and `1` against `1.0` matter, and the ops would be rewritten for every backend. The store has one connection and is the daemon's alone, so a read, apply and commit in one transaction is just as atomic |
| Left out | queries inside a value and index arithmetic: whoever needs them wants a database |

</details>

**Not the R1.3 store.** [Shared secrets and a KV with
locks](../R1.3/exploration.md#shared-secrets-and-a-kv-with-locks) asks about a
network-shared store for services that may be blind to what it holds. This one
is the daemon's own state, reachable by whoever may already reach the record.
