# IMAP large transfer

**Parked (owner, 2026-11-01): behind [bigfiles](bigfile.md#how-the-imap-agent-uses-it), which replace these chunked calls.**

📌 **TL;DR:**
- **Plan only; no code until the owner approves** ([B.31](TODO.md#bundled-agents), questions Q281–Q287, Q285 settled).
- **Problem:** `part` and `append` of the [IMAP folders plan](imap-folders.md#help-text) are capped at 25 MB, but a bus answer holds 256 KiB and a request 1 MiB.
- **Recommendation: (b) chunked calls.** The caller reads or writes one window per call, with offsets, resume and a SHA-256 over the whole; the agent keeps a bounded staging file in its own work directory.
- **Why not a file handoff:** the sandboxed agent's only writable path is its own work folder, which a remote caller cannot read and a local one may not; (b) works for every caller.

## Help text

Written as `help` will print it. The landed lines of [`help.golden.json`](../../src/agents/imap/help.golden.json) (0.9.249) are kept word for word; additions are marked in bold below the block. Only the two `part` windowing options and the `upload` lines are new.

```text
part <uid> <part-id> [--offset <n>] [--limit <n>] [--folder <name>]: Decoded, base64 for binary, 25 MB cap, no Seen. Refuse above 26214400 encoded bytes before decoding. Return what fits in the 256 KiB answer cap and label truncation. With --offset or --limit the answer is one window of the decoded part as base64, at most 160 KiB, led by one line giving the part's type, total size, the window's offset and length and the SHA-256 of the whole part; repeat with the next offset until the total is reached.
[destructive] append <folder> [--flags f,..] [--date rfc3339]\n<raw RFC 5322 body>: Append raw RFC 5322 text after the first newline (JSON raw). Report APPENDUID when supplied; otherwise explicitly report UID unavailable. Mail cap 25 MB; the existing 1048576-byte bus request cap including encoded JSON and envelope is lower. Requires allow_destructive; never retry an unknown write. Mail above the request cap goes through upload and append --upload.
[destructive] append <folder> --upload <id> [--flags f,..] [--date rfc3339]: Append the staged mail from upload, which must hold exactly its size and match the SHA-256 given at begin; the staging is deleted either way. Report APPENDUID when supplied; otherwise explicitly report UID unavailable. Requires allow_destructive; never retry an unknown write.
upload begin <size> [--sha256 <hex>]: Start staging a mail of up to 25 MB for append --upload and get an upload id. At most one staging per caller; it expires after an hour.
upload put <id> <offset> <base64>: Add the next window, up to 512 KiB. The offset must equal the bytes already held; repeating the last window with the same bytes is accepted, so a lost reply is safe to retry.
upload status <id>: Get the bytes held, the size and the time left. Read it to resume after an interruption.
upload abort <id>: Delete the staging.
```

**Added to `part`:** the last sentence only. **Added to `append`:** the last sentence of the body form, and the `--upload` form, whose closing clauses are the landed ones.

**Behaviour change, windowed `part` only:** a window is base64 for every type, so reassembled windows are byte-exact and match the hash; a text part cut in the middle of a character would otherwise show hex escapes at the edge. A plain `part` without `--offset` or `--limit` stays as landed: decoded text for text, base64 for binary, truncated with a label. Recommendation: this split ([Q287](#parked-questions)).

JSON spellings: `folder` as everywhere; `{"method":"upload.put","id":"…","offset":0,"data":"<base64>"}`; `{"method":"part","uid":7,"part":"2","offset":0,"limit":163840}`. `upload` is the only new method name; `begin`, `put`, `status` and `abort` follow it as `sub` has `add`, `list` and `delete`.

A caller sends a finished RFC 5322 mail, attachments included. The agent builds no MIME ([Q284](#parked-questions)).

## The three options

| | (a) file handoff | (b) chunked calls | (c) generic large body |
|---|---|---|---|
| Where the bytes live | a file the agent writes and the caller reads | the bus carries one window per call; staging in the agent's work folder | the bus stores large bodies for every agent |
| Remote callers | cannot read the file | work unchanged | work |
| Trust boundary | the file sits outside the bus ACL: readable by whoever the host lets in | each window obeys the ACL like any answer | the daemon holds up to 25 MB bodies readable by it |
| Durability | file on disk, no expiry owner | staging is private, bounded and expiring; the bus keeps nothing | queues flush every minute in one SQLite database; big bodies bloat that flush |
| Process boundary | the agent would need a path both it and the caller reach; the sandbox adds none | stays inside the agent | daemon and protocol change |
| Cost | cleanup and permissions per host | about 160 calls for 25 MB down, 50 up | ⚠️ daemon, protocol, storage; a release of its own |

### (a) File handoff

- **Where:** the only writable host path of a sandboxed agent is its work directory ([visible files and writes](../../docs/runner/security.md#visible-files-and-writes)); `needs.writable` adds none.
- **Who can read:** the work folder belongs to the runner's own account, so a caller on another account cannot, and a caller on another host never can.
- **Cleanup:** nothing owns it; the disk check is soft ([limits](../../docs/runner/security.md#processes-and-limits)).
- **Trust and process boundary:** a file outside the ACL breaks [what a call carries](../../docs/02-access.md#what-a-call-carries): the host's permissions, not the record's access list, would decide who sees private mail.
- Verdict: not for remote callers, not for other accounts; rejected.

### (b) Chunked calls (recommended)

- **Download:** the first `part` call fetches the whole part from the server in bounded windows with PEEK, decodes it on the fly into a staging file, hashes it, and serves windows from that file. Later windows read the staging, not the server. The mail cache is untouched; it holds only mail up to 2 MiB.
- **Upload:** `upload put` appends in order into a staging file; `append --upload` streams that file to the server as the literal, only after the size and hash match.
- **Resume and integrity:** offsets are exact, a repeat of the last window is accepted, a different byte at a held offset is refused; the whole-part SHA-256 is the end-to-end check, and a mismatch refuses before any server write.
- **Bounds:** staging total and lifetime ([Q282](#parked-questions)); one staging per caller; deleted on completion, abort, expiry and at agent start; `0600` files in a `0700` directory under `AGENT_BUS_WORK`, never followed through links, like the [mail cache](../../docs/agents/imap.md#technical-details).
- **Trust:** each window is private mail passing the daemon like any body today; off loopback the [trust boundary](../../docs/02-access.md#trust-boundary) applies unchanged: use TLS or SSH. An attachment is untrusted bytes; the answer says so and nothing is opened or rendered.
- **Durability:** nothing large enters a queue; the [queue flush](../../docs/04-messaging.md#durability) sees only windows of at most a few hundred KiB.
- **Authority:** `part` and `upload put/status/abort` need ordinary admission (staging is the caller's own and bounded); `append` stays behind the destructive switch of the [folders plan](imap-folders.md#authority).
- **No daemon, runner or protocol change.** Code in `src/agents/imap` only: `wire.py` (partial FETCH, literal APPEND), `imap.py` (parser: ⚠️ parsers, mutants), a staging module beside `cache.py`.

### (c) A generic large-body mechanism

- Other agents could reuse it, but none asks for it yet: only IMAP exceeds the caps.
- It changes the daemon store and the wire: ⚠️ storage and protocol subsystems, a bump of its own, and the [durability](../../docs/04-messaging.md#durability) rule that queues flush as one batch a minute does not suit 25 MB bodies.
- The owner first chose Redis for this (2026-11-01), then replaced it the same day with [bigfiles](bigfile.md#what-it-is-and-is-not), a store inside the bus ([decision](DECISIONS.md#recorded-decisions)).

## Order of work

Depends on T-17 step 2 (`part`) and step 4 (`append`); both exist as call names before this lands.

| Step | Does | Who | ETA after approval |
|---|---|---|---|
| 1 | `part` windows: bounded partial FETCH, decode to staging, hash, serve; staging module with bounds and expiry | team 2 worker | one day |
| 2 | `upload` and `append --upload`: staging in order, resume, hash check, literal APPEND | same worker | one day |
| 3 | guide, help, golden; a worked caller loop for each direction in `docs/agents/imap.md` | same worker | half a day |

Each step is its own commit and landing; the mutants take the `slow` lock.

## Acceptance (for the TODO item)

- A 25 MB part comes back byte-identical across windows and its hash matches; a window never exceeds the answer cap; a larger `--limit` is capped, not refused.
- A part whose server literal exceeds the wire bound is read in bounded windows; the stream is never read past a bound.
- An upload of 25 MB lands as one mail with the right UID; a lost `put` reply retried changes nothing; a gap, an overlap with different bytes and a window past the size are refused.
- A wrong size or SHA-256 sends nothing to the server and deletes the staging.
- Staging expires, is removed at start, is bounded per caller and in total, and is never visible to another caller.
- `append` without the destructive switch is refused with the fixture seeing no write.
- **Mutants** (compiling, assertion-red): window longer than the cap; offset not checked on `put`; a different byte at a held offset accepted; hash not compared before APPEND; staging kept after a failed append; the expiry ignored; another caller's id accepted; staging path followed through a link; Seen set by `part`; the whole part cached in the mail cache.

## Parked questions

Open choices of this plan, parked with B.31 behind [bigfile](bigfile.md#help-text) (owner, 2026-11-01). They return to [R1 questions](QUESTIONS.md#imap-large-transfer) only if B.31 resumes.

- ❓ Q281 Chunked calls or a file handoff
  - Options: chunked `part` and `upload` calls; or a file the agent writes and the caller reads; or a generic large-body mechanism
  - Recommendation: chunked calls; the file is unreadable to remote callers and to other accounts, and the generic mechanism is daemon work no other agent needs
  - Impact: decides whether anything outside `src/agents/imap` changes

- ❓ Q282 Staging bounds and lifetime
  - Options: 100 MB in total, one staging per caller, upload expires after an hour, download staging after 15 minutes; or tighter or looser
  - Recommendation: those numbers; they fit the default 1 GiB disk check with room for the mail cache
  - Impact: a lower total refuses 25 MB while another caller is mid-transfer

- ❓ Q283 Caller convenience for a hundred calls
  - Options: a documented shell loop in the guide only; or a CLI verb that loops; or an MCP helper
  - Recommendation: the documented loop now; a verb only if the owner wants it, as client work outside this plan
  - Impact: an AI caller can loop itself; a person at a shell needs the example

- ❓ Q284 Should the agent build the MIME for an upload
  - Options: the caller sends a finished RFC 5322 mail; or the agent assembles one from a text body and attachment files
  - Recommendation: the caller sends the finished mail; MIME assembly is a parser surface the agent need not own
  - Impact: building attachments is the caller's job

- ❓ Q286 Is the SHA-256 at `upload begin` required
  - Options: optional, checked when given; or required
  - Recommendation: optional; `part` always reports the hash of the whole part
  - Impact: required makes every upload verifiable but forces callers to hash first
- ❓ Q287 Do windowed `part` answers use base64 for text too
  - Options: base64 for every type when `--offset` or `--limit` is given, and a plain `part` unchanged; or decoded text for text parts in windows as well
  - Recommendation: base64 in windows; exact reassembly and the hash, and a split character never shows escapes
  - Impact: the other choice cannot promise the whole-part hash for text parts
