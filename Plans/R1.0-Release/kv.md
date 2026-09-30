# Key-value store

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Per-record storage

**The daemon keeps a store on each record, in its database, with atomic
operations on it** (owner, 2026-09-30). It gives an 👾 Agent, a 👤 User — whose
own record carries one too — or anything else on the bus somewhere durable to
put what it is doing: several workers dividing a batch, or the status of each
job, without a database of its own beside the bus.

| | |
|---|---|
| Scope | one store per record; the same name on two records is two values |
| Three kinds | a **string** (any bytes, the default), an **int** and a **json** value, each its own table and namespace: `count` as a string and `count` as an int are two values |
| Who | the record's **Owner, Maintainers and own Agent** read and write it — the same authority as its [private values](../../docs/constitution.md#-private-values) and its [shared locks](locks.md#shared-locks) (Q107). Its allow list grants use of the record, not of its store |
| The record's life | an inactive record's store is no such entity; it comes back with the record. Removing the record deletes its store in the same transaction, and a name registered again starts empty |
| Durable | a write is committed before it is answered, never left to the queue checkpoint: a `shift` handed to one worker must not come back after a crash for another |
| Where it lives | the daemon's own state, behind the same persistence ports as everything else it stores. SQLite is the first implementation and the default; the other [backends](../R1.1/storage.md#backends) serve it as they serve the rest |

| Operation | String | Int | JSON |
|---|---|---|---|
| read one name | `kv_get` | `kv_int_get` | `kv_json_get` |
| write one name, `how` = **`set`** (default), **`add`** (only if absent) or **`replace`** (only if present); a refused `add` or `replace` says so | `kv_set` | `kv_int_set` | `kv_json_set` |
| remove one name | `kv_delete` | `kv_int_delete` | `kv_json_delete` |
| edit in one step | | `kv_int_inc(record, name, n)` | `kv_json(record, name, ops)`: a list of [JSON operations](#json-operations), applied all or none |

Each call takes the record and the name first; the kind is the call, so a
value never meets an operation for another kind.

<details>
<summary>Tables in SQLite</summary>

Keyed by the record's internal ID, which is never reused, so a name freed and
registered again inherits nothing, and a transfer keeps the store. At load, a
row whose record is not stored is ignored and reported, never reattached.

```sql
CREATE TABLE kv (
  record_id INTEGER NOT NULL,
  name      TEXT    NOT NULL,
  value     BLOB    NOT NULL,
  PRIMARY KEY (record_id, name)
) WITHOUT ROWID;

CREATE TABLE kv_int (
  record_id INTEGER NOT NULL,
  name      TEXT    NOT NULL,
  value     INTEGER NOT NULL,
  PRIMARY KEY (record_id, name)
) WITHOUT ROWID;

CREATE TABLE kv_json (
  record_id INTEGER NOT NULL,
  name      TEXT    NOT NULL,
  value     TEXT    NOT NULL,  -- compacted JSON
  PRIMARY KEY (record_id, name)
) WITHOUT ROWID;
```

</details>

## JSON operations

**A small, fixed set of edits the daemon applies to the top-level keys of a
`json` value, in one step** (owner, 2026-09-30). The value is an object, and
each op names one of its keys: `push jobs x`, `inc done 1`. Nesting deeper is
another store name, such as a `job3` value of its own.

| Op | Does |
|---|---|
| `set key value` | writes the key |
| `unset key` | removes the key |
| `inc key n` | adds `n` to a number |
| `push key value` | appends one value to an array |
| `unshift key value` | prepends one value to an array |
| `shift key` | removes and returns an array's first element |
| `pop key` | removes and returns an array's last element |
| `add_to_set key value` | appends `value` unless an equal one is present |
| `remove_from_set key value` | removes every element equal to `value`, so it also clears duplicates `push` left; returns nothing |

- **All or none:** a list of ops is one write. A key holding the wrong type,
  or a stored value that is not an object, refuses the whole list, naming the
  op and the key.
- **Missing:** an absent name starts as `{}`. `inc`, `push`, `unshift` and
  `add_to_set` create a missing key, as `0` or a one-element array; `shift`,
  `pop`, `remove_from_set` and `unset` on one change nothing and say so. An empty array's
  `shift` or `pop` returns nothing; that is not an error.
- **Answer:** each op's result — the element `shift` or `pop` took, the number
  `inc` left — never the whole document.
- **Equality:** `add_to_set` and `remove_from_set` compare JSON values, so key order and
  number spelling do not matter.
- **In core, not SQL:** the daemon applies the list in Go inside one store
  transaction and stores the result, so every backend gets the same semantics.

<details>
<summary>Why this set</summary>

| | |
|---|---|
| Dividing work | `push` and `shift` are a queue: no two workers `shift` the same element. `unshift` puts a failed job back at the front, and `pop` takes the newest |
| `remove_from_set`, not `pull` | MongoDB's `pull` reads like `shift` and `pop`, which hand an element back; this returns nothing, and pairs with `add_to_set` |
| No conditional op | "only if it is still mine" is a read and a write under the record's [shared lock](locks.md#shared-locks), or data shaped so a `shift` already made it yours |
| Top-level keys only | every case above works one level down, and the store's names give the next level, each its own atomic unit. A path language — JSON Pointer, `/jobs/3/status` — can be added later without breaking anything, a leading `/` opting in |
| Left out | queries inside a value and index arithmetic: whoever needs them wants a database |
| Why not SQL | SQLite's JSON functions can do each op in one `UPDATE` (tried 2026-09-30 on the driver's 3.53.4), but they compare minified text, so key order and `1` against `1.0` matter, and the ops would be rewritten for every [backend](../R1.1/storage.md#backends). The daemon is the store's only writer, so a read, apply and commit in one transaction is just as atomic. SQL pays only for large values, which a size cap rules out |

</details>

**Atomic is the whole point of it.** Two workers reading, deciding and writing
back cannot divide a list between them; `kv_set … add` and `kv_inc` can,
because the daemon that holds the value is the one that decides. That is the
same argument as [shared locks](locks.md#shared-locks) — one authority a pool
already shares — applied to the value rather than to the right to act. A lock
is the daemon's memory and goes with it; a value put here is stored and does
not.

**Not the R1.3 store.** [Shared secrets and a KV with
locks](../R1.3/exploration.md#shared-secrets-and-a-kv-with-locks) asks about a
network-shared store for services that may be blind to what it holds. This one
is the daemon's own state, reachable by whoever may already reach the record.
