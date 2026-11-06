# Large Markdown files in RAG

📌 **TL;DR:** A plan, approved by the owner on 2026-11-06 (Q368–Q371, all recommended options). Markdown files over 32 KiB reach RAG today only as a skip. FileScan would hand RAG a bigfile reference, and RAG would chunk the file by its headings. Every chunk carries its source, hash and citation. RAG confirms a file only once all of it is stored, and a free [completeness check](#the-completeness-check) proves it.

Owner, 2026-11-06, through mdhouse: "let AB plan and implement it if it makes sense". The need comes from mdhouse's [RAG contract](http://localhost:7777/mdhouse/Plans/R3/RAG-CONTRACT.md): five of its 194 Markdown files are over 32 KiB, and coverage must never be reported as full while they are skipped.

## Today

| Part | Behaviour |
|---|---|
| FileScan `as rag` | A file over 32 KiB, or not text, is skipped for RAG and named in `status`. A delete is sent for its id, because RAG may still hold an earlier version ([FileScan § RAG](../../../docs/agents/filescan.md#reference-for-a-consumer)) |
| FileScan `bigfile` | Hands a queue subscriber a file of up to 32 MiB as a bigfile on the FileScan instance's own record, named by its SHA-256 and kept for `bigfile_ttl`. It cannot be combined with `as rag` |
| RAG record | One source of at most 32 KiB, split into at most eight chunks of 4 KiB. One head per record names the record's current revision and its chunks ([technical details](../../../docs/agents/rag.md#technical-details)) |
| RAG upsert | Writes the new chunks, then publishes the head, then deletes chunks of any other revision. Search checks each hit against its head, so a chunk the head does not name never answers |
| RAG delete | Hides the head, deletes every chunk revision of the record, then removes the tombstone. It answers `ok`, or `not_found` for an id it does not hold |

## Direction

Settled by `#ab-dir`, 2026-11-06: RAG does the chunking, and FileScan is only the transport. RAG gets the bytes through FileScan's bigfile reference and never opens a host path ([Q368](QUESTIONS.md#large-files)).

- **One file is one record.** The record keeps today's id, which is the path. A file that grows past 32 KiB keeps its record, and only its chunking changes.
- **Inline files are unchanged.** A file of at most 32 KiB keeps today's path and chunk format. The heading chunker applies only to files that arrive by reference.
- **RAG reads the reference once, then works from its copy.** It downloads the bigfile and checks its SHA-256 against the reference before any paid call. A mismatch or a failed download is refused by name, and nothing is stored or paid for.

## Chunk identity

A chunk keeps the same identity across re-syncs when its text has not changed.

- A chunk's identity is the record identity, the heading path (the titles of every enclosing heading, outermost first), and its position within that heading's section.
- It is never a running index over the whole file. An edit in one section therefore leaves the identities of every other section unchanged.
- A section too long for one chunk splits at paragraph, then line, then whitespace boundaries. The parts are numbered within the section only.
- Text before the first heading has an empty heading path.
- **Repeated sibling headings:** siblings with the same title under the same parent are told apart by their ordinal among same-titled siblings, counted from 1 in file order. Two `## Notes` sections under one `# Plan` are `Plan / Notes #1` and `Plan / Notes #2`.
- **Headings follow CommonMark.** A heading is an ATX heading: at most three leading spaces, one to six `#`, then a space or the end of the line. Setext headings (text underlined with `===` or `---`) are not headings. A `#` line inside a fenced code block is text: a fence opens with three or more backticks or tildes, closes only with the same character at least as long, and an unclosed fence runs to the end of the file. `#` lines in YAML front matter (a `---` block on the first line) are not headings. Fence content never starts a section or moves a split point. A fence is split only when it alone exceeds a chunk, and then at line boundaries.
- This is a new chunk format, with its own permanent version beside today's ([technical details](../../../docs/agents/rag.md#technical-details)). The file's text lives only in its chunks. A large file's head holds the whole-file hash and the chunk list, never the text, because today's head stores the full `data` under the 32 KiB data limit.

## Hashes

- **Whole file:** the record's `content_hash` is the SHA-256 of the file's exact bytes. It is the same value FileScan names the bigfile by, so the reference, the record and the confirmation all carry one hash.
- **Per chunk:** each chunk carries the SHA-256 of its own text.
- **Reuse:** a re-sync embeds only the chunks whose identity or hash is new. A chunk with the same identity and hash keeps its stored vector. Editing one section of a large file then costs that section's embeddings, not the whole file's.

## Citation

Every hit from a large file carries:

- its source, which is the path FileScan sent;
- its heading path;
- its line range in the file, first and last line, counted from 1.

It also carries what every hit has today: namespace, table, record id, `content_hash`, an excerpt, scores and `indexed_at`.

## Replacing a file's chunks

A re-sync replaces the record's chunk set as one change. A stale chunk never answers.

- The new head names exactly the chunks of the new version: reused ones and new ones.
- Search answers a chunk only when the current head names that chunk, with that chunk's hash. A chunk left over from an interrupted re-sync is invisible, then swept up, as today.
- Until the new head is published, the previous version answers in full. Once it is published, only the new version answers. No search sees a mix of the two.

## Whole-file confirmation

**A per-file confirmation means RAG has ingested the whole file, never just that the transport delivered it.** Team 2's finite-sync completion rule links here.

- RAG answers a file's upsert only after every chunk is stored and the new head is published.
- The answer is the stored record. It repeats the identity (namespace, table, id and source) and the whole-file hash that RAG ingested. FileScan already counts an upsert as stored only when every one of those fields matches what it sent ([FileScan § RAG](../../../docs/agents/filescan.md#reference-for-a-consumer)).
- Because the answer echoes the hash it ingested, an answer for an older version never confirms a file that changed mid-sync: its hash differs, and the file stays pending. A rerun compares hashes and skips files whose confirmed hash is unchanged.
- A large upsert is not waited on inline. FileScan's delivery rounds are short, so the upsert goes out with a long sender deadline and counts as maybe-delivered. Nothing resends it while that deadline runs. On a later round, the confirmation comes from the echoed reply or from a [completeness check](#the-completeness-check) whose record matches its hash ([transport](#transport)).
- An answer that does not come within the deadline, or comes incomplete, confirms nothing.
- **Resend authority, standing watch:** an automatic resend of a large file runs the free completeness check first. It resends only when the check does not answer `complete: true` with the file's current hash and source: that is, on `not_found`, on `complete: false`, or on another hash or source. A file edited mid-sync therefore answers with the old hash and is sent again. A send refused outright, never queued, is certain non-delivery and may be resent without a check. Resends stay within the watch's existing resend bound (the shared core's confirmed delivery with resend, [S1](../../R1.1/filescan.md#stages)). Thanks to chunk reuse, a resend embeds only the chunks not yet stored.
- **Resend authority, on-demand sync:** a file whose outcome is unknown is never resent automatically. Only an explicit caller resends, and a refusal fails the file ([sync rule 4](../../R1.1/filescan.md#on-demand-sync)).
- **A free completeness check proves completion for that hash.** RAG publishes the head last: after classification, after every chunk of the new version is stored, and carrying the labels, metadata and the list of chunks it names. A [completeness check](#the-completeness-check) that answers `complete: true`, with namespace, table, id, source and whole-file hash all matching, proves three things for that hash: the chunks are stored, the head is committed, and the new set replaced the old one for every answer. Team 2's sync runs this check before any paid resend. The hash alone proves less, as `rag.md` says ([unknown outcome](../../../docs/agents/rag.md#if-an-upserts-outcome-is-unknown)): an older head can sit beside an equal hash with another source, so the source must match too.
- **Gap: physical cleanup.** Chunks of the previous version are deleted after the head is published, and the completeness check does not prove that this finished. They never answer, because search keeps only the chunks the current head names, and a later upsert or delete sweeps them up. A sync needs nothing more for a file to count as complete; only the storage they still take is unproved.
- **Deletion, today:** a file gone from the folder gets FileScan's `as rag` delete for its id, the path. RAG's delete of that id removes the head and every chunk of every revision. It answers `ok`, or `not_found` when nothing is held, and FileScan counts both as done. An interrupted delete answers `incomplete: repeat the same delete to continue`; FileScan sends it again until it finishes. So a confirmed removal by path, covering every chunk, exists today, and a large file stored as one record inherits it.
- A file over the size cap ([Q371](QUESTIONS.md#large-files)) is skipped and named in `status`. Its earlier version is deleted, as today, because stale text must not answer. It is never cut short or chunked partway.

## The completeness check

The interface is frozen here for team 2, which builds against it. It ships in L.1; `rag.md` and the method help describe it once it is built.

Today `get` reads the head and every chunk it names, and answers "record changed or is incomplete" if one is missing. But it also returns the data and every vector under its read caps, so a large record fails with "stored record exceeds read cap". The check is therefore an option of `get` that returns neither. `get` without it is unchanged, caps included.

- **Call:** `get` with `"check": true` beside the usual `namespace`, `tableName` and `id`. Example: `{"method":"get","namespace":"mdhouse","tableName":"docs","id":"Plans/R3/README.md","check":true}`.
- **Free:** it reads storage only, with no provider call. It reads the head and asks whether each chunk the head names exists, without vectors.
- **Every record:** it works the same for today's small records and for large ones.

The answer always carries all seven fields:

| Field | Value |
|---|---|
| `namespace` | as sent |
| `tableName` | as sent |
| `id` | as sent |
| `source` | the stored version's source |
| `content_hash` | the stored version's whole-file SHA-256: of the exact text for a small record, of the file's bytes for a large one |
| `indexed_at` | when that version was stored, as `get` reports it |
| `complete` | `true` when the head is committed and every chunk it names exists; `false` when a chunk the head names is still missing after one re-read |

It never carries `data`, `embeddings`, labels or chunk text. Only `complete: true` with the expected hash and source confirms a file. `complete: true` with another hash or source means another version is stored, and confirms nothing. `complete: false` confirms nothing either, and can be transient: a replacing upsert may be racing the check. A resend after it is allowed, and cheap through chunk reuse.

Refusals, worded as `get`'s:

| Refusal | When |
|---|---|
| `not_found: record is absent from this namespace` | no head for the id in this namespace, a deleted record, or an id held only in another namespace |
| `check must be true` | `check` is present and is anything but `true` |
| `invalid canonical head`, `invalid chunk references`, `chunk identity mismatch`, `invalid stored record` | the stored head is damaged, as for `get` |
| the argument refusals of `get` | a missing or invalid `namespace`, `tableName` or `id`, or an unknown argument |
| a storage error or deadline | the outcome is unknown; run the check again, never a paid upsert on its strength |

## The upsert by reference

Frozen for team 2's L.2, which sends it; it ships in L.1.

**The reference.** A large file's upsert carries `bigfile` in place of `data`, and never `embeddings`. RAG reads four fields from it:

| Field | Meaning | RAG checks |
|---|---|---|
| `record` | the FileScan instance's record holding the bigfile | it may download from it, or the upsert is refused `bigfile_forbidden` |
| `name` | the bigfile's name on that record | it exists, or the upsert is refused `bigfile_unavailable` (absent or expired) |
| `size` | the file's size in bytes | 1 byte to 1 MiB; the download has exactly this size |
| `sha256` | the SHA-256 of the file's bytes | it equals the upsert's `content_hash`; the download hashes to it |

- The rest of the upsert is as today: `namespace`, `tableName`, `id`, `source`, optional `key1` and `key2`. `content_hash` is required and is the same SHA-256. `mime-type` is `text/markdown`, which is also the default.
- **Checked before any paid call:** these four fields when the call is parsed, then the download's size and hash, then that the bytes are UTF-8 text.

**The deadline.** RAG works to the envelope's `deadline`, which is the sender's stated wait (the send's `wait` option), never to the message's `ttl`.

- **`deadline`:** RAG stops before its next paid call or write once it passes. A large upsert gets at most 30 minutes, and less when the sender's wait is shorter. A send with no stated wait gets the 30 minutes. A deadline that passes before the head is written is the definite `deadline: not stored; prior record unchanged`; one that passes after it is unknown.
- **Message `ttl`:** it bounds only the wait in RAG's queue. A message whose ttl ends there is never delivered, so RAG does no work and pays nothing. Once RAG has taken the message, its ttl no longer matters; a download already under way is not dropped for it.
- **The sender:** FileScan sends with a `wait` of 30 minutes and a `ttl` at least as long, and does not block on it. Within that wait it never resends. It may run the free check early: `complete: true` with the current hash and source confirms the file at once.
- **Three different bounds:** `bigfile_ttl` ([Q370](QUESTIONS.md#large-files), 1 hour) is the reference's lifetime, and an expired one is refused `bigfile_unavailable` and re-uploaded. The `wait` is RAG's working time. The message `ttl` is the queue wait.

**How it ends.** Three classes, and only the last needs the completeness check:

| Class | Answers | Meaning |
|---|---|---|
| Refused before any paid call | a parse refusal; `bigfile_forbidden`; `bigfile_unavailable`; `bigfile_mismatch`; `bigfile is not UTF-8 text`; `deadline_too_short` | nothing was paid for or stored; the previous version answers |
| Stopped early, definite | `deadline: not stored; prior record unchanged`; `not stored; prior record unchanged: …` for a storage refusal before the head; a `#voyage` or `#ai` failure partway, which also comes before the head | the head was never written, so the previous version answers in full; chunks paid for may sit stored but unpublished, and a resend reuses them |
| Unknown | `storage outcome unknown; …`, a storage error or deadline after the head write, or a lost answer | run the check |

A sender that cannot read the reply learns every outcome from the check alone: `not_found`, or another hash or source, after a definite stop; `complete: true` with the current hash and source once the file is stored.

- **Parse refusals**, worded as the parser words them: `bigfile needs record, name, size and sha256`; `bigfile size integer out of range` (outside 1 byte to 1 MiB); `bigfile sha256 must be lower-case SHA256`; `content_hash must equal the bigfile sha256`; `bigfile replaces data and embeddings`; `a bigfile upsert takes text/markdown only`. A `content_hash` that differs from `sha256` is one of these, never `bigfile_mismatch`, which names a download that is not the referenced version.
- **A late start does not eat the wait.** The deadline is absolute and RAG takes one call at a time, so a message can be taken late. When less than 120 seconds remain, RAG refuses `deadline_too_short` before the download and before any paid call.
- **Paid work is kept.** RAG stores each embedded batch as soon as it is paid for, and publishes the head last. A resend reuses every stored chunk with the same identity and hash, whether a published version or an interrupted attempt stored it. Reuse copies the stored vector to the new version's chunk, which is a read, never a paid call. So a resend never pays again for a chunk RAG still holds. Jev is the exception: every attempt classifies the file again before its first batch, one paid `#ai` call. A sender that cannot read replies therefore pays one Jev call per resend after a definite stop, bounded only by its resend bound.
- **What reuse keeps:** only the vector. A reused chunk is written again with the new version's payload: its labels, revision, provenance, `indexed_at` and source. Reuse requires the same namespace, table and id, the namespace's embedding model and dimensions, and stored text equal byte for byte to the new chunk's, not just an equal hash field; a vector is never reused for different text. The chunk identity rule is unchanged: point ids stay per version.
- **A delete sweeps what never published.** A delete of an id with no head answers `not_found` as today, and first removes every chunk that id still holds, so the chunks of a file that never published do not stay behind.
- **The reference on a resend:** FileScan keeps the bigfile key until the file is confirmed, so a resend reuses it while it lives. A resend after its hour is refused `bigfile_unavailable` before any paid call, and FileScan uploads it again.

## Cost and limits

- Each new chunk is one paid embedding through `#voyage`, and a chunk is at most 4 KiB of text. Reused chunks cost nothing.
- A file's first sync pays for all its chunks. A re-sync pays only for changed chunks.
- Classification by Jev runs once per record over at most 32 KiB. For a large file it classifies the first 32 KiB of extracted text, and `rag.md` will say so.
- The per-file cap is 1 MiB, about 256 chunks ([Q371](QUESTIONS.md#large-files)), which bounds what one file can cost.
- A large file needs more embedding calls than today's 120-second budget allows. RAG's budget comes from the sender's deadline: `deadline` in a serves-itself agent's `agent.json` is only its stop wait. So FileScan sends a large upsert with a deadline that grows with the file's size, up to what the cap allows. A call that runs out is an unknown outcome, handled as in [whole-file confirmation](#whole-file-confirmation).

## Transport

FileScan's side is step L.2, owned by team 2.

- **FileScan:** lift the refusal of `bigfile` together with `as rag`. For an `as rag` subscriber, a text file over 32 KiB and within the cap is sent as RAG's upsert with a bigfile reference in place of the text. A file over the cap, or not text, stays skipped and named in `status`.
- **Not waited on inline:** FileScan delivers in rounds of at most 8 seconds and waits 5 seconds for a confirmation (`change_sync.py`), short on purpose for the hold wait. A large upsert therefore goes out with its long deadline and is left in flight. While that deadline runs, the file counts as maybe-delivered and is never resent. Without this, every large file would time out each round and be sent again, and RAG, which handles one call at a time, would run duplicate paid upserts. A later round confirms the file from the echoed reply, or from a completeness check whose record matches the identity and hash.
- **Mixed subscriptions:** one `as rag` subscription can carry inline files and reference files together, so FileScan tracks bigfile keys per file.
- **Read grant:** RAG downloads the bigfile from the FileScan instance's record, so that record's allow list must admit the RAG instance ([Q369](QUESTIONS.md#large-files)). FileScan cannot check this at `sub add`. An unadmitted RAG is refused at download, and the file stays pending with that reason.
- **Expiry:** RAG downloads at the start of the call, before any paid work, so `bigfile_ttl` must cover the time a file waits, not RAG's processing time ([Q370](QUESTIONS.md#large-files)). RAG handles one call at a time, so a file can wait in RAG's queue longer than `bigfile_ttl`. An expired reference is refused by name, and FileScan uploads again on its next attempt, which covers that wait. FileScan keeps the key until RAG has confirmed the file.

## Steps

Estimates are AI time, from observed agent wall-clock. Each step is one branch, one reader and fixture tests with fake providers. No live calls.

| Step | Builds | Estimate |
|---|---|---|
| L.1 | RAG: download and check a bigfile reference; the heading chunker and its chunk identity; per-chunk hashes and vector reuse; the head naming hashed chunks; the hit citation (heading path, lines); the [completeness check](#the-completeness-check); `get` keeps its caps. Today's small-record heads keep answering unchanged, with no reindex. Tests: repeated sibling headings and a `#` line inside a fence, before any reuse claim. Mutants: drop the head check, a completeness check that answers with a chunk missing, cite a running index, confirm before the head is published, reuse a vector when the heading path changed | ~60 min |
| L.2 | FileScan: `as rag` with a bigfile reference within the cap; skip and delete over it. The large upsert goes with a long sender deadline and is not waited on inline; it counts as maybe-delivered, with no resend while within its deadline; a later round confirms it from the echoed reply or a completeness check (`get` with `check`) by hash. Re-upload an expired reference; bigfile keys tracked per file for a subscription mixing inline and reference files; the unadmitted-download reason in `status`. Mutants: resend while in flight, confirm on download, keep a stale key | ~40 min |
| L.3 | Docs (`rag.md`, `filescan.md`), the replay fixture, and a fake-provider end-to-end for a 200 KiB Markdown file: edit one section and only that section is re-embedded; delete the file and no chunk answers | ~25 min |

The three steps total about two hours and a quarter, plus review and one agent-only gate per step. L.1 and L.3 are RAG's and L.2 is team 2's; the completeness check is frozen first, so L.2 builds against it.
