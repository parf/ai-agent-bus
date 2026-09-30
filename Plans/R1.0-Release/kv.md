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
| Who | the record's **Owner, Maintainers and own Agent** read and write it — the same authority as its [private values](../../docs/constitution.md#-private-values) and its [shared locks](locks.md#shared-locks) (Q107). Its allow list grants use of the record, not of its store |
| A value | `int`, `string`, `json` or `blob`, under a name |
| The record's life | an inactive record's store is no such entity; it comes back with the record |
| Where it lives | the daemon's own state, behind the same persistence ports as everything else it stores. SQLite is the first implementation and the default; the other [backends](../R1.1/storage.md#backends) serve it as they serve the rest |

| Operation | |
|---|---|
| `kv_get(record, name)` | read one name |
| `kv_set(record, name, value, how)` | write one name; `how` is **`set`** (write it, default), **`add`** (only if absent) or **`replace`** (only if present), and a refused `add` or `replace` says so |
| `kv_delete(record, name)` | remove one name |
| `kv_inc(record, name, n)` | add to an `int` in one step |
| `kv_json(record, name, ops)` | edits inside a `json` value: a list of [JSON operations](#json-operations), applied all or none |

## JSON operations

**A small, fixed set of edits the daemon applies inside a `json` value, in one
step** (owner, 2026-09-30). A path is a JSON Pointer (RFC 6901): `/jobs/3/status`,
with `""` for the whole value.

| Op | Does |
|---|---|
| `set path value` | writes at `path`, creating missing object keys on the way |
| `unset path` | removes a key or an array element |
| `inc path n` | adds `n` to a number |
| `push path value` | appends one value to an array |
| `unshift path value` | prepends one value to an array |
| `shift path` | removes and returns an array's first element |
| `pop path` | removes and returns an array's last element |
| `pull path value` | removes every element equal to `value` |
| `add_to_set path value` | appends `value` unless an equal one is present |

- **All or none:** a list of ops is one write. A wrong type or a path that
  cannot hold the op refuses the whole list, naming the op and the path.
- **Missing paths:** `inc`, `push`, `unshift` and `add_to_set` create one, as
  `0` or a one-element array; `shift`, `pop`, `pull` and `unset` on one change
  nothing and say so. An empty array's `shift` or `pop` returns nothing; that
  is not an error.
- **Answer:** each op's result — the element `shift` or `pop` took, the number
  `inc` left — never the whole document.
- **Equality:** `pull` and `add_to_set` compare deep JSON, after the same
  compaction as [`config`](../../docs/03-records.md#why-a-digest-at-all).
- **Only `json`:** any other type refuses every op; nothing is converted.
- **In core, not SQL:** the daemon applies the list in Go inside one store
  transaction and stores the result, so every backend gets the same semantics.

<details>
<summary>Why this set</summary>

| | |
|---|---|
| Dividing work | `push` and `shift` are a queue: no two workers `shift` the same element. `unshift` puts a failed job back at the front, and `pop` takes the newest |
| No conditional op | "only if it is still mine" is a read and a write under the record's [shared lock](locks.md#shared-locks), or data shaped so a `shift` already made it yours |
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
