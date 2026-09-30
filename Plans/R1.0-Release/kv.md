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
| JSON operations | atomic edits inside a `json` value, `push`, `pull` and add-to-set among them |

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
