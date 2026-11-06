# Large Markdown files in RAG

📌 **TL;DR:** A plan, with no code until the owner approves it. Markdown files over 32 KiB reach RAG today only as a skip. FileScan would hand RAG a bigfile reference, and RAG would chunk the file by its headings. Every chunk carries its source, hash and citation. RAG confirms a file only once all of it is stored. Open choices: [Q368–Q371](QUESTIONS.md#large-files).

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

Settled by `#ab-dir`, 2026-11-06: RAG does the chunking, and FileScan is only the transport. How RAG gets the bytes is [Q368](QUESTIONS.md#large-files); this plan assumes the recommended bigfile reference.

- **One file is one record.** The record keeps today's id, which is the path. A file that grows past 32 KiB keeps its record, and only its chunking changes.
- **Inline files are unchanged.** A file of at most 32 KiB keeps today's path and chunk format. The heading chunker applies only to files that arrive by reference.
- **RAG reads the reference once, then works from its copy.** It downloads the bigfile and checks its SHA-256 against the reference before any paid call. A mismatch or a failed download is refused by name, and nothing is stored or paid for.

## Chunk identity

A chunk keeps the same identity across re-syncs when its text has not changed.

- A chunk's identity is the record identity, the heading path (the titles of every enclosing heading, outermost first), and its position within that heading's section.
- It is never a running index over the whole file. An edit in one section therefore leaves the identities of every other section unchanged.
- A section too long for one chunk splits at paragraph, then line, then whitespace boundaries. The parts are numbered within the section only.
- Text before the first heading has an empty heading path.
- This is a new chunk format, with its own permanent version beside today's ([technical details](../../../docs/agents/rag.md#technical-details)).

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
- A large upsert is not waited on inline. FileScan's delivery rounds are short, so the upsert goes out with a long sender deadline and counts as maybe-delivered. FileScan does not resend it while that deadline runs. On a later round, the confirmation comes from the echoed reply or from a `get` of the record that matches its hash ([transport](#transport)).
- An answer that does not come within the deadline, or comes incomplete, confirms nothing, and only then does FileScan send the file again. Thanks to chunk reuse, a repeat embeds only the chunks not yet stored.
- **Deletion, today:** a file gone from the folder gets FileScan's `as rag` delete for its id, the path. RAG's delete of that id removes the head and every chunk of every revision. It answers `ok`, or `not_found` when nothing is held, and FileScan counts both as done. An interrupted delete answers `incomplete: repeat the same delete to continue`; FileScan sends it again until it finishes. So a confirmed removal by path, covering every chunk, exists today, and a large file stored as one record inherits it.
- A file over the size cap ([Q371](QUESTIONS.md#large-files)) is skipped and named in `status`. Its earlier version is deleted, as today, because stale text must not answer. It is never cut short or chunked partway.

## Cost and limits

- Each new chunk is one paid embedding through `#voyage`, and a chunk is at most 4 KiB of text. Reused chunks cost nothing.
- A file's first sync pays for all its chunks. A re-sync pays only for changed chunks.
- Classification by Jev runs once per record over at most 32 KiB. For a large file it classifies the first 32 KiB of extracted text, and `rag.md` will say so.
- The per-file size cap, and with it the most one file can cost, is the owner's choice: [Q371](QUESTIONS.md#large-files).
- A large file needs more embedding calls than today's 120-second budget allows. RAG's budget comes from the sender's deadline: `deadline` in a serves-itself agent's `agent.json` is only its stop wait. So FileScan sends a large upsert with a deadline that grows with the file's size, up to what the cap allows. A call that runs out is an unknown outcome, handled as in [whole-file confirmation](#whole-file-confirmation).

## Transport

These steps assume Q368's bigfile reference.

- **FileScan:** lift the refusal of `bigfile` together with `as rag`. For an `as rag` subscriber, a text file over 32 KiB and within the cap is sent as RAG's upsert with a bigfile reference in place of the text. A file over the cap, or not text, stays skipped and named in `status`.
- **Not waited on inline:** FileScan delivers in rounds of at most 8 seconds and waits 5 seconds for a confirmation (`change_sync.py`), short on purpose for the hold wait. A large upsert therefore goes out with its long deadline and is left in flight. While that deadline runs, the file counts as maybe-delivered and is never resent. Without this, every large file would time out each round and be sent again, and RAG, which handles one call at a time, would run duplicate paid upserts. A later round confirms the file from the echoed reply, or from a `get` whose record matches the identity and hash.
- **Mixed subscriptions:** one `as rag` subscription can carry inline files and reference files together, so FileScan tracks bigfile keys per file.
- **Read grant:** RAG downloads the bigfile from the FileScan instance's record, so that record's allow list must admit the RAG instance ([Q369](QUESTIONS.md#large-files)). FileScan cannot check this at `sub add`. An unadmitted RAG is refused at download, and the file stays pending with that reason.
- **Expiry:** RAG downloads at the start of the call, before any paid work, so `bigfile_ttl` must cover the time a file waits, not RAG's processing time ([Q370](QUESTIONS.md#large-files)). RAG handles one call at a time, so a file can wait in RAG's queue longer than `bigfile_ttl`. An expired reference is refused by name, and FileScan uploads again on its next attempt, which covers that wait. FileScan keeps the key until RAG has confirmed the file.

## Steps

Estimates are AI time, from observed agent wall-clock. Each step is one branch, one reader and fixture tests with fake providers. No live calls.

| Step | Builds | Estimate |
|---|---|---|
| L.1 | RAG: download and check a bigfile reference; the heading chunker and its chunk identity; per-chunk hashes and vector reuse; the head naming hashed chunks; the hit citation (heading path, lines); the size-scaled deadline. Mutants: drop the head check, cite a running index, confirm before the head is published | ~60 min |
| L.2 | FileScan: `as rag` with a bigfile reference within the cap; skip and delete over it. The large upsert goes with a long sender deadline and is not waited on inline; it counts as maybe-delivered, with no resend while within its deadline; a later round confirms it from the echoed reply or a `get` of the record by hash. Re-upload an expired reference; bigfile keys tracked per file for a subscription mixing inline and reference files; the unadmitted-download reason in `status`. Mutants: resend while in flight, confirm on download, keep a stale key | ~40 min |
| L.3 | Docs (`rag.md`, `filescan.md`), the replay fixture, and a fake-provider end-to-end for a 200 KiB Markdown file: edit one section and only that section is re-embedded; delete the file and no chunk answers | ~25 min |

The three steps total about two hours and a quarter, plus review and one agent-only gate per step. The owner's answers to Q368–Q371 come first.
