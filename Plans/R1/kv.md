# Key-value store

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Per-name storage

**The daemon keeps a named store for each User and each registry record, in
SQLite, with atomic operations on it.** It gives a 👤 User or an 👾 Agent
somewhere durable to put what it is doing — several workers dividing a batch,
or the status of each job — without a database of its own beside the bus.

| | |
|---|---|
| Scope | one store per User and one per registry record; the same name in two stores is two values |
| A value | `int`, `string`, `json` or `blob`, under a name |
| Where it lives | the daemon's own [SQLite state](../../docs/09-setup.md#storage), under the storage rules that state already follows |

| Operation | |
|---|---|
| `get`, `set`, `delete` | read, write and remove one name |
| `set_if` | write when the condition holds, and return what the store was left with |
| `delete_if` | remove when the condition holds, and return what was removed |
| `inc` | add to an `int` in one step |
| JSON operations | atomic edits inside a `json` value, `push`, `pull` and add-to-set among them |

**Atomic is the whole point of it.** Two workers reading, deciding and writing
back cannot divide a list between them; `set_if` and `inc` can, because the
daemon that holds the value is the one that decides. That is the same argument
as [shared locks](locks.md#shared-locks) — one authority a pool already shares
— applied to the value rather than to the right to act. The lock is the daemon's
memory and goes with it; a value put here is stored and does not.

**Not the R1.2 store.** [Shared secrets and a KV with
locks](../R1.2/exploration.md#shared-secrets-and-a-kv-with-locks) asks about a
network-shared store for services that may be blind to what it holds. This one
is the daemon's own state, reachable by whoever may already reach the bus.
