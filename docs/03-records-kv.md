# Key-value store

📌 **TL;DR:** Every record has its own durable store of named values (strings,
ints and JSON objects), edited atomically and committed before an answer. Its
Owner, Maintainers and own Agent use it, the same authority as the record's
[shared locks](01-identity-and-authority.md#shared-locks). It coordinates a pool of workers
without a database beside the bus.

- **Three kinds, three namespaces:** a **string** (any bytes, the default), an **int** (signed 63-bit) and a **json** object.
- **Not the allow list:** the allow list grants use of the record, never of its store.
- **One store per record:** the same name on two records is two values.
- **Lives with its record:** it goes when the record goes, and is kept through a transfer ([rules](#rules)).
- **Stored, not cached:** it lives in the daemon's database, never only in memory.

## Where to attach it

| Attach it to | For example | Who uses it |
|---|---|---|
| 👾 an Agent | a worker's checkpoint: `kv set --json '#indexer@team' progress '{"cursor":1200}'` | its Owner, its Maintainers, and the Agent itself |
| 📮 a queue or 📣 a PubSub | the batch behind the work sent there: `kv json jobs@team batch '[{"op":"push","key":"todo","value":"img-1"}]'` | its Owner and Maintainers |
| 📡 a Service | facts about the thing outside: `kv set --int db@team schema 42`, the migration it is on | its Owner and Maintainers |
| 👥 a Group | the team's shared state: `kv set @oncall@team current alice@team` | its Owner and Maintainers — not its members, whom the Group grants elsewhere |
| 👤 a User's own record | a person's own notes and settings: `kv set alice@team theme dark` | that User |

## Verbs

| Verb | String | Int | JSON |
|---|---|---|---|
| read one name | `kv get` | `kv get --int` | `kv get --json` |
| write one; `--add` only if absent, `--replace` only if present, and a refused one says so | `kv set` | `kv set --int` | `kv set --json` |
| remove one; the answer says whether it was there | `kv delete` | `kv delete --int` | `kv delete --json` |
| edit in one step | | `kv inc <record> <name> [n]`, an absent name counting from 0 | `kv json <record> <name> <ops>` |

## Rules

| Rule | |
|---|---|
| Durable | a write is committed before it is answered, never left to the queue checkpoint |
| Atomic | every edit — a mode, an increment, a list of JSON operations — is one transaction on the value it replaces |
| Integer range | −2^62 through 2^62−1, inclusive; integer values, increment operands and results, and JSON `inc` use this range; overflow refuses the complete edit |
| The record's life | an inactive record's store is [no such entity](constitution.md#common-record-fields) and comes back with it; removing the record deletes its store in the same transaction, and a name registered again starts empty. Values are keyed by the record's internal ID, so a transfer keeps them |
| Limits | a name is 1 to 256 bytes of UTF-8; a value at most 128 KiB; a record holds at most 10,000 names of one kind; a JSON list at most 100 operations |
| At start | a stored value whose record is not stored is ignored and reported, never reattached |
| Faces | the API's `GET /kv` and `POST /kv/set`, `/kv/delete`, `/kv/inc`, `/kv/json` take `record`, `kind` and `name`; a string travels as a JSON string, or `value_base64` when its bytes are not UTF-8, an int as a number and a JSON value as the object. `GET /kv/list` answers a record's names with sizes and previews, or with no record every store the caller may use. The MCP face has `ab_kv_get`, `ab_kv_set`, `ab_kv_delete`, `ab_kv_inc` and `ab_kv_json`; the web face shows a record's store on its page, and on the **KV** page and its own pages ([web face](../src/web/web-face/records.md#key-value-stores)) |

The value cap leaves room for JSON escaping inside the API's request-body
limit. **Pending enforcement:** the implementation still accepts values up to
its former cap; reduce it to the limit above for both string and JSON writes.
The narrowed integer range also awaits enforcement; the implementation still
uses signed 64-bit bounds. MCP must preserve integers exactly across its
JavaScript boundary: narrowing to 63 bits alone does not prevent rounding.

## JSON operations

A JSON value is an object, and each operation names one of its top-level keys.
A list of them is applied whole or not at all; an absent name starts as `{}`.

| Op | Does |
|---|---|
| `set key value` | writes the key |
| `unset key` | removes the key |
| `inc key n` | adds the integer `n` to an integer |
| `push key value` | appends one value to an array |
| `unshift key value` | prepends one value to an array |
| `shift key` | removes and answers an array's first element |
| `pop key` | removes and answers an array's last element |
| `add_to_set key value` | appends `value` unless an equal one is present |
| `remove_from_set key value` | removes every element equal to `value`; answers nothing |

A key holding the wrong type, or an unknown or malformed operation, refuses
the whole list and names it. `inc`, `push`, `unshift` and `add_to_set` create
a missing key, as `0` or a one-element array; `shift`, `pop`,
`remove_from_set` and `unset` on one change nothing and say so, as does an
empty array's `shift` or `pop`. Each operation answers `{op, key, changed}`,
with `value` for what `shift` or `pop` took and the number `inc` left.
Equality compares JSON values, so key order and number spelling do not
matter; a value is stored compact with its keys sorted.

```sh
agent-bus kv json jobs@team batch '[{"op":"push","key":"todo","value":"a"},{"op":"push","key":"todo","value":"b"}]'
agent-bus kv json jobs@team batch '[{"op":"shift","key":"todo"}]'   # each worker gets its own job
```

Why it is shaped this way is the [plan's reasoning](../Plans/R1.0-Release/done/kv.md#per-record-storage).

## Suggested use

The store coordinates a pool without a database beside the bus. Put it on the
record the work belongs to — a 📮 queue or the pool's 👾 Agent — so the
workers reach it as that Agent, or as its Maintainers. Three rules make every
pattern below safe: **claim with an atomic op** (`shift`, `pop`,
`set --add`), **make results idempotent** (`--add` on a result name, so a
retried item writes once), and **pair a store with a [lock](01-identity-and-authority.md#shared-locks)
when liveness matters** — the lock's ttl says who is alive, the store says how
far they got, and survives them.

Below, `batch.todo` is the key `todo` of the JSON value `batch`, edited with
`kv json`; `batch/total` is a name of its own.

| Pattern | Coordinator | Each worker | Why it holds |
|---|---|---|---|
| **Scatter** | `push` every item onto `batch.todo`, then `set --int batch/total N` | `shift batch.todo` until it answers nothing | no two workers get one item; an empty `shift` is the end, not an error |
| **Gather** | waits for one message, not a poll | writes `set --add result/<id> …`, then `inc batch/done`; the worker whose `inc` answers `N` sends the coordinator "gathered" | `--add` makes a retry write nothing twice; exactly one `inc` sees the total |
| **Retry** | — | a failed item goes back with `unshift batch.todo`, or to `push batch.failed` after its third try, counted in the item | the queue keeps it through any crash but the worker's own |
| **Sharding** | `set --int shards N`; items go to shard `hash(key) mod N` | claims a shard with `set --add shard/<n> <me>`, holds `lock <record> shard/<n> --ttl 30s` and `extend`s it while working, keeps `set --int cursor/<n> <offset>` | the claim is exclusive; a dead worker's lock expires, and whoever takes shard `n` next — `delete shard/<n>`, then `--add` — resumes from its cursor |
| **Rebalancing** | takes `lock <record> rebalance`, rewrites the `shards` map, bumps `inc epoch` | reads `epoch` before each item and re-claims when it moved | one rebalance at a time; a worker never mixes two maps |
| **Suspend** | `set --int paused 1` | reads `paused` before each claim, stops taking new items, and checkpoints `set --json job/<id>` with its cursor and state | a pool pauses without stopping a process; nothing in flight is lost |
| **Resume** | `set --int paused 0` | picks up from each `job/<id>` it held, or any whose worker is gone, by the same claim | the checkpoint is stored and committed before its answer, so a daemon restart resumes too |

<details>
<summary>Scatter and gather from the CLI</summary>

```sh
# coordinator
agent-bus kv json jobs@team batch '[{"op":"push","key":"todo","value":"img-1"},{"op":"push","key":"todo","value":"img-2"}]'
agent-bus kv set --int jobs@team batch/total 2

# each worker, in a loop
job=$(agent-bus kv json jobs@team batch '[{"op":"shift","key":"todo"}]')   # "changed":false, no value, when empty
agent-bus kv set --add jobs@team result/img-1 "done:42"                   # a retry is refused, not doubled
[ "$(agent-bus kv inc jobs@team batch/done)" = 2 ] && agent-bus send coordinator@team gathered
```

</details>

What the store is not: a message queue — nothing wakes a waiting worker, so a
worker that finds `todo` empty waits for a message, not a tight loop — and not
a database: no queries, only names and top-level keys. A value is at most
512 KiB, so large results go elsewhere and the store keeps where.
