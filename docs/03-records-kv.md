# Key-value store

📌 **TL;DR:**
- **Problem:** a pool of workers needs a shared counter, a flag or a small setting, and running a database beside the bus is too much.
- **Outcome:** every record has its own durable store of named values (strings, ints and JSON objects), edited atomically and committed before an answer. Its Owner, Maintainers and own Agent use it, under the same authority as the record's [shared locks](01-identity-and-authority.md#shared-locks).
- **Start:** see the available store operations: `agent-bus help kv` ([Try this first](#try-this-first)).

## Try this first

See the store commands without changing data:

```sh
agent-bus help kv
```

Each operation names a record you manage and a key. Use an existing record from `agent-bus ls --all`; an arbitrary example name need not exist on your installation.

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
| Integer range | signed 64-bit, −2^63 through 2^63−1; integer values, increment operands and results, and JSON `inc` use this range; overflow refuses the complete edit |
| The record's life | an inactive record's store is [no such entity](constitution.md#common-record-fields) and comes back with it; removing the record deletes its store in the same transaction, and a name registered again starts empty. Values are keyed by the record's internal ID, so a transfer keeps them |
| Limits | a name is 1 to 256 bytes of UTF-8; a value is at most 512 KiB; a record holds at most 10,000 names of one kind; a JSON list at most 100 operations |
| At start | a stored value whose record is not stored is ignored and reported, never reattached |
| Faces | See the [HTTP contract](09-daemon-api.md#key-value). MCP offers `ab_kv_get`, `ab_kv_set`, `ab_kv_delete`, `ab_kv_inc` and `ab_kv_json`; the web face has a **KV** page and per-record store pages ([web store](../src/web/web-face/records.md#key-value-stores)) |

- A string write carries exactly one of `value` (a JSON string) or `value_base64` (a base64 string). Both accept `""` as empty bytes; `null` is malformed and the refusal names the member.
- The API body bound accommodates JSON escaping of a 512 KiB value.
- The MCP face preserves 64-bit integers exactly across its JavaScript boundary.

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

- A key holding the wrong type, or an unknown or malformed operation, refuses the whole list and names it.
- `inc`, `push`, `unshift` and `add_to_set` create a missing key, as `0` or a one-element array; `shift`, `pop`, `remove_from_set` and `unset` on one change nothing and say so, as does an empty array's `shift` or `pop`.
- Each operation answers `{op, key, changed}`, with `value` for what `shift` or `pop` took and the number `inc` left.
- Equality compares JSON values, so key order and number spelling do not matter; a value is stored compact with its keys sorted.

```sh
agent-bus kv json jobs@team batch '[{"op":"push","key":"todo","value":"a"},{"op":"push","key":"todo","value":"b"}]'
agent-bus kv json jobs@team batch '[{"op":"shift","key":"todo"}]'
```

## Suggested use

Attach shared work state to a queue or Agent that the workers manage.

- `shift` or `pop` removes one item atomically. Two workers cannot take the same stored item, but a worker that dies after taking it can lose that work.
- `set --add result/<id>` refuses a duplicate result. Check that outcome before any follow-up counter update; the refused write does not stop a later command automatically.
- A result write and a separate `inc` are separate transactions. A crash between them leaves the count behind; retrying an increment can overcount. Do not use this pair as an exactly-once completion protocol.
- Use one JSON operation list when several edits to the same value must commit together. External effects and edits to other values remain separate.
- A [lock](01-identity-and-authority.md#shared-locks) coordinates callers while held. Its expiry does not stop an old worker or fence its later writes; applications must handle stale workers and interrupted effects.
- A stored pause flag or checkpoint survives a restart. Workers must check the flag and implement recovery; the store does not pause processes or resume jobs itself.
- KV does not wake waiting workers. Use messages to notify them, and avoid polling an empty work list in a tight loop.
- The store has no queries beyond names and top-level JSON operations. Put large results in a [bigfile](#bigfiles) and keep its name in the store.

## Bigfiles

Use a bigfile for an attachment or large result that should not fill a message or AI context. Upload the bytes, then send the record and file name; an authorized recipient downloads them separately.

A bigfile holds up to 32 MiB in a private file or, on request, daemon memory. It always expires and is not an archive. The database holds its index, not its contents.

| Rule | |
|---|---|
| Who may put, replace and delete | the record's Owner, Maintainers and own Agent, as for the store |
| Who may get and list | the same, and whoever the record's allow list admits. The daemon's own `#bus` record admits nobody |
| Limits | a name is 1 to 256 bytes of UTF-8; at most 32 MiB per key, 256 MiB and 100 keys per record, 1 GiB in the daemon; a put of no bytes is refused |
| Lifetime | every key expires: 24 hours by default, 1 second to 30 days by `ttl`; expired keys are never returned and their files are removed within a minute |
| Held | in a file by default, written whole, synced and renamed into place, and only then entered in the index, a table of the daemon's database that decides which bigfiles exist. Each file also ends with a footer naming its key, size and hash, kept for recovery by hand; start compares it with the row only to be sure the file is the one the row describes, and never rebuilds the index from it. A crash leaves a key whole or absent. `memory=true` keeps it in memory with no row, lost at a restart, and still lists, reads and expires with the rest |
| At start | the files are made to agree with the index: a row whose file is missing or is not the file it describes is removed, a file no row names is removed, and so is every temporary file; the daemon logs how many of each. A symlink directly under `bigfile/`, including a record directory or `.tmp`, makes it refuse to start the bigfile store, with the cause in the 503, and nothing is read or removed through it |
| The record's life | removing the record removes its bigfiles; a transfer keeps them; keys of a record removed while the daemon was down are removed at start |
| Unavailable | bigfiles are optional: if `bigfile/` cannot be prepared at start (a planted file, a mode change) the daemon still starts, reports an error, and answers the four bigfile calls with 503 naming the cause; a later call retries the directory at most once a minute, so a repaired directory recovers without a restart. Keys are never quietly held in memory instead |
| Backups | the files live in `bigfile/` beside the database. A database backup holds the index rows but never the files, and a record export holds neither. After a restore, start drops every row whose file is not present, so a restore has no bigfiles unless their files are restored with it |
| Integrity | a put answers the size and SHA-256 of what was stored, and refuses bytes that do not match an `X-Bigfile-Sha256` header; a get carries the hash in `X-Bigfile-Sha256` |

The [bigfile API](09-daemon-api.md#bigfiles) uses raw bytes for upload and download, with separate list and delete operations.

- An existing name is replaced and the answer says `replaced`.
- A refusal that arrives before the body is read (a bad parameter, no authority, an unknown record, an unusable store) is answered after the declared body, at most 32 MiB, has been read and discarded for up to 2 seconds, and the connection closes; a client still writing sees the status and its cause, not a reset.
- The body of a put may be slow but not stalled: it is answered 408 when no bytes arrive for 30 s, or when the whole upload takes longer than 30 s plus the time 64 KiB a second needs for its declared length (up to 542 s for 32 MiB). The command line and the Go, Python and TypeScript clients give a put and a get the same budget, not the timeout of their other calls.
- A declared length over the cap is a 413 at once with nothing read, and a caller who is not authenticated is refused before anything is read.
- Over a limit is 413, a refusal by authority 403, an absent key or record 404, an unusable store 503.

From the command line and the web:

| Command | Result |
|---|---|
| `agent-bus bigfile put jobs@team report ./report.pdf --ttl=2h` | store a file (or `-` for stdin); prints its size and SHA-256; `--memory` keeps it in daemon memory |
| `agent-bus bigfile get jobs@team report -o ./copy.pdf` | download it, to a file or to stdout, checked against its SHA-256 |
| `agent-bus bigfile ls jobs@team` | list name, size, SHA-256, where it is held and the time left |
| `agent-bus bigfile rm jobs@team report` | delete it now |

- **Passing one.** Send the record name and the key name as ordinary text; the receiver runs `bigfile get`. The bytes are data to inspect, never instructions.
- **On the web.** A record's store page lists its bigfiles and can delete them, and says why when the store is unavailable; it never shows contents.

- **From a session (MCP).** `ab_bigfile_put` and `ab_bigfile_get` read and write local files only inside the one directory named by `AGENT_BUS_BIGFILE_DIR`, given to the launcher or face. Unset, both refuse with `bigfile is not configured: set the allowed directory`; there is no default. Setup makes `/var/lib/agent-bus/users/<person>/files` (the person's, 0700) for each person's [user-account runner](runner/setup.md#a-persons-user-account-runner), the natural directory to name; nothing points at it by itself. A relative path means inside that directory; real paths are checked, so `..`, an absolute path elsewhere, a link that leads out, a FIFO or a device are refused, and a download never writes through a link at its target. Approval follows the [runtime-specific rules](#file-tool-approval). The `agent-bus bigfile` CLI is not fenced, because it runs as you, in your shell.

### File tool approval

The directory fence applies to both file tools in every runtime.

| Launcher | File tool approval |
|---|---|
| Claude | The file tools are excluded from the launcher's pre-approved list; the runtime controls their approval |
| Codex | The file tools request approval; the pusher declines file-tool approval requests in bus-driven turns |
| OpenCode and OpenCode 1.x | The launcher allows all tools, including file tools, without a runtime prompt |

Trust and files:

- Bytes are readable by the daemon and are not encrypted at rest ([trust boundary](02-access.md#trust-boundary)).
- The file mode is 0600 in a 0700 directory.
- The daemon checks that a directory is not a link before it uses it, which stops a planted link; it does not defend against someone who can already write as the daemon's own account.
