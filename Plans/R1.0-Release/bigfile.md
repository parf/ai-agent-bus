# Bigfiles: pass large data by name

📌 **TL;DR:** Bigfiles let agents hand off attachments and large results without putting the bytes in a message or an AI conversation. Upload once, pass the record and file name, then let an authorized recipient download it. This is temporary transfer storage, not an archive.

## Why it is needed

- **A large attachment does not fit in a normal call.** IMAP can expose a mail part through a bigfile and return its size and hash instead of truncating it into a small answer.
- **Putting binary data in text makes it larger and harder to handle.** Upload and download stream the bytes; messages carry only the reference.
- **An AI usually needs the result, not every byte of a PDF or archive.** MCP transfers files locally and returns metadata rather than filling the model's context with contents.
- **A local path is not a handoff between hosts.** The recipient downloads from the bus under the record's access rules, without a shared filesystem or a separately operated file service.

## What it is and is not

| Choice | Reason |
|---|---|
| A value attached to a record | Reuse record ownership and access instead of inventing another permission system |
| A name in an ordinary message | Keep routing, queue limits and reply handling unchanged |
| Streamed bytes outside the message queue | Transfer large data without enlarging every message or copying it into each recipient's inbox |
| File storage by default; memory when requested | File storage can survive a daemon restart; memory is temporary and disappears on restart |
| Mandatory expiry and size limits | Bound temporary storage; the sender keeps the authoritative original |
| Size and SHA-256 returned after upload | Let the recipient check the transferred bytes; a hash is not encryption or proof that content is safe |

## Use it

Use the [bigfile commands and limits](../../docs/03-records-kv.md#bigfiles). The same reference covers the HTTP API, supported clients, MCP's allowed local directory and the web's list/delete controls.

- Upload the data, pass its record and bigfile name, and keep it available long enough for the recipient to download.
- Passing a name does not grant access, reserve a queue delivery or guarantee a download before expiry.
- Names can be replaced. Compare the expected hash when the identity of the bytes matters.

### Authority

The record's Owner, Maintainers and own Agent upload, replace and delete. Callers admitted by its access list can download and list. Choose a record whose readers may see the entire payload; naming a file in a private message does not narrow that record's access.

## How the IMAP agent uses it

- `part --bigfile <name>` stores the decoded attachment on the IMAP instance and returns metadata instead of the attachment bytes; it does not mark the mail Seen.
- `append --bigfile <record>/<name>` takes raw mail from a bigfile. The caller uploads it to a record they manage and grants the IMAP instance download access. Appending still requires the instance's destructive-operation switch and must not be retried after an unknown outcome.

See [IMAP parts](../../docs/agents/imap.md#what-it-does) and [append](../../docs/agents/imap.md#folder-management-and-append) for the supported forms and mail limits.

## Trust and durability

- The daemon can read the bytes. Use TLS or SSH between hosts; stored bigfiles are not encrypted at rest.
- File uploads are committed whole or remain absent after recovery. Memory uploads are lost on restart. All bigfiles expire.
- A record export or a database-only backup does not contain the payload files. Keep an original outside this temporary store.
- File contents remain untrusted data. A filename or matching hash does not make instructions inside a file safe to follow.

The [storage and access reference](../../docs/03-records-kv.md#bigfiles) defines recovery behavior. [Completion evidence](DONE.md#b32-status-close-out) records the implementation and checks.

## Acceptance

- A Maintainer uploads, downloads, lists and deletes; an admitted non-Maintainer downloads and lists but is refused put and rm; a stranger is refused all four, with the daemon's refusal and nothing stored.
- A 32 MiB value round-trips byte-identical and its SHA-256 matches; one byte over is refused before storing; the per-record and daemon totals refuse the upload that would pass them.
- An expired key is gone from list and get; the sweep removes its file.
- A crash during upload leaves no key and no stray file after start; a memory key is gone after a daemon restart.
- A removed record's keys go with it; a transfer keeps them, as the key-value store does.
- Files are `0600` in a `0700` directory; a name never reaches a path; a link in the directory is not followed.
- IMAP `part --bigfile` stores the decoded part and the answer carries no bytes; `append --bigfile` without the destructive switch or without admission is refused, with the fixture seeing no write.
- **Mutants** (compiling, assertion-red): the ACL read from the wrong record; download allowed to a stranger; put allowed to an admitted non-Maintainer; the size cap off by one; the record total ignored; expiry ignored; the temporary file renamed before sync; a name used as a path; a memory key written to a file; a removed record's key surviving.
