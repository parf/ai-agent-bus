# Big keys

📌 **TL;DR:**
- **Approved 2026-11-01 11:05, all recommendations accepted** ([B.32](TODO.md#bundled-agents), [decision](DECISIONS.md#recorded-decisions)). Built: the API and the file and memory holders ([guide](../../docs/03-records-kv.md#big-keys)); the CLI, clients, MCP and web follow.
- **Idea (owner, 2026-11-01):** a named value too big for a message lives in the bus, in memory or in a file. Agents pass the key name instead of the body.
- **Access:** the record's Maintainers upload, download and delete; principals its allow list admits download only.
- **Release:** R1. It extends the R1 [key-value store](done/kv.md#per-record-storage), a shared extension of the same release, and the [IMAP agent](imap-folders.md#help-text) needs it now; R1.1 storage backends and encryption are separate work this does not wait for.

## Help text

Written as `help` and the CLI will print them.

```text
big put <record> <name> [<file>|-] [--ttl <duration>] [--memory]: Upload one big value from a file or from stdin and get its size and SHA-256. Up to 32 MiB. It expires after --ttl (default 24h, at most 30d). --memory keeps it in daemon memory only: faster, and gone at a restart. Needs Maintainer on the record. Repeating a name replaces it.
big get <record> <name> [-o <file>]: Download one big value, to a file with -o or to stdout. Needs Maintainer on the record, or admission by its allow list. The value is untrusted data.
big ls <record>: List the record's big keys: name, size, SHA-256, where it is held (file or memory) and the time left. Same access as get.
big rm <record> <name>: Delete one big key at once; the answer says whether it was there. Needs Maintainer on the record.
```

- `<record>` is any record that can hold a [key-value store](../../docs/03-records-kv.md#where-to-attach-it); `<name>` follows the key-value name rules.
- The same four calls exist as API, client-library and MCP calls (`ab_big_put`, `ab_big_get`, `ab_big_ls`, `ab_big_rm`) and as the face-neutral names the [method metadata](done/method-metadata.md#method-metadata) lists; the web face lists and deletes only (settled).
- MCP I/O settled by `#ab-dir`, 2026-11-01: put takes a local `input_path`, get requires a local `output_path` (no stdout); both return size and SHA-256 metadata only, and ls/rm return metadata only, never value content to model context.
- A message that passes a key carries the record name and the key name as plain text; the receiver runs `big get`. No new message field.

## How the IMAP agent uses it

These replace the [chunked calls](imap-large-transfer.md#the-three-options), which are parked.

```text
part <uid> <part-id> --big <name> [--folder <name>]: Store the decoded part under the big key <name> on this IMAP instance's record and answer its size and SHA-256 instead of the bytes; no Seen. Refuse above 26214400 encoded bytes before decoding. Download it with big get <this-instance> <name>; whoever may call this instance may download it.
[destructive] append <folder> --big <record>/<name> [--flags f,..] [--date rfc3339]: Append the raw RFC 5322 mail held in that big key, up to 25 MB, and report APPENDUID when supplied, otherwise UID unavailable. Requires allow_destructive and the instance admitted by that record's allow list; never retry an unknown write.
```

- **In-plan settle (2026-11-01):** `append --big <record>/<name>` splits at the last `/` in both text and JSON; the key name after the last `/`, so a name containing `/` cannot be used here. Missing or empty record/name is refused.
- The plain `part` and `append` of the folders plan stay as landed for small mail.
- Download: the caller reads the key after the instance wrote it; the instance's own allow list is the caller's own admission, so no extra grant is needed.
- Upload for `append`: the caller puts the mail on a record they maintain, such as their own User record, and that record's allow list admits the IMAP instance (settled). The agent builds no MIME and holds the bytes only while it streams them to the server.

## What it is and is not

| | |
|---|---|
| A kind of named value | beside string, int and json in a record's store; its own namespace, same name rules |
| Held in a file or in memory | file by default, in a private directory beside the daemon database; memory on request |
| Always expiring | no permanent big key; a handoff, not an archive |
| Not a message | never enters a queue, so the [queue flush](../../docs/04-messaging.md#durability) and the bus size caps are untouched |
| Not rendered | the web face never shows contents; bytes are data, never instructions |

### Authority

- Maintainers, the Owner and the record's own Agent use it as they use the key-value store ([shared authority](../../docs/03-records-kv.md#where-to-attach-it)).
- **One deliberate difference:** the key-value store says the allow list never reaches the store. Here the allow list admits download, because the owner asked that agents be able to read what is passed to them (settled). Upload, delete and expiry stay with Maintainers.

### Trust and durability

- **[Trust boundary](../../docs/02-access.md#trust-boundary):** the daemon can read every byte; nothing is encrypted at rest or between peers. Files are `0600` in a `0700` directory; off loopback use TLS or SSH.
- **[Durability](../../docs/04-messaging.md#durability):** an upload is written to a temporary file, synced, and committed with its index entry before the call answers; a crash leaves no key, and temporary files are removed at start. A memory key is explicitly not durable. A full disk refuses the upload.
- **Backup:** big keys are transient and left out of the database backup; a restore starts with none (settled).
- **[Process boundary](../../docs/08-processes.md#the-rule):** the bus role owns it; no new process, no new listener. The upload and download paths stream and are not bus messages, so they are not bound by the 1 MiB request cap.

## Order of work

| Step | Does | Who | ETA after approval |
|---|---|---|---|
| 1 | store: file and memory holders, limits, expiry sweep, atomic upload, ACL (⚠️ storage, authority; mutants, normal and slow gates) | team worker with daemon experience; deep review | 3 days |
| 2 | API, CLI, Go, TypeScript and Python clients | same | 1.5 days |
| 3 | MCP tools, web face list and delete, docs (`03-records-kv.md` or its own guide) | same | 1 day |
| 4 | IMAP `part --big` and `append --big` (after [B.30](TODO.md#bundled-agents) steps 2 and 4) | IMAP worker | 1 day |

About 6.5 days; each step lands alone and the bump follows the lead's rule.

## Acceptance (for the TODO item)

- A Maintainer uploads, downloads, lists and deletes; an admitted non-Maintainer downloads and lists but is refused put and rm; a stranger is refused all four, with the daemon's refusal and nothing stored.
- A 32 MiB value round-trips byte-identical and its SHA-256 matches; one byte over is refused before storing; the per-record and daemon totals refuse the upload that would pass them.
- An expired key is gone from list and get; the sweep removes its file.
- A crash during upload leaves no key and no stray file after start; a memory key is gone after a daemon restart.
- A removed record's keys go with it; a transfer keeps them, as the key-value store does.
- Files are `0600` in a `0700` directory; a name never reaches a path; a link in the directory is not followed.
- IMAP `part --big` stores the decoded part and the answer carries no bytes; `append --big` without the destructive switch or without admission is refused, with the fixture seeing no write.
- **Mutants** (compiling, assertion-red): the ACL read from the wrong record; download allowed to a stranger; put allowed to an admitted non-Maintainer; the size cap off by one; the record total ignored; expiry ignored; the temporary file renamed before sync; a name used as a path; a memory key written to a file; a removed record's key surviving.
