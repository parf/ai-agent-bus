# RAG: a memory your AI can search

📌 **TL;DR:**
- **Problem:** an AI only knows what is in front of it, and your notes, mails and documents are scattered, so it cannot look them up.
- **Outcome:** a shared notebook that stores each text sorted into your own topics, so you or your AI can find evidence by words, meaning or both.
- **Start:** give a table its topic list (no AI call yet): [Try this first](#try-this-first).

## What it does

- ✅ **Store a text:** it is sorted into your topics, fingerprinted and kept (`upsert`).
- ✅ **Read it back:** one record, or a page of records in a namespace (`get`, `list`).
- ✅ **Find evidence:** `search` selects keyword, semantic or hybrid retrieval, with one live excerpt per record and its source, hash and indexing time. An optional rerank reorders the hits in one paid call. It returns evidence, without generating an answer.
- ✅ **Discover without payment:** `capabilities` names the methods, modes, limits and configured model, even before setup. It makes no storage or provider call and does not claim readiness.
- ✅ **Manage namespaces:** the agent's Owner or Maintainers can list names and live-record counts, or explicitly confirm removal of one namespace.
- ✅ **Build the keyword index:** `keyword_index` adds the vector search needs to a collection that lacks it and gives the selected namespace's stored texts their words; safe to repeat.
- ✅ **Delete:** one record, or a whole table when you confirm it (`delete`).
- ✅ **Keep topic lists:** read or set a table's topics (`prompts`); changing a list affects only texts added afterwards.
- ✅ **Recover from an interrupted call:** the answer distinguishes **nothing changed**, **unknown** and **incomplete**. Follow the operation's recovery instructions; repeating an upsert can spend provider balance again and replace concurrent changes. Nothing retries by itself.
- ⚙️ **Records:** live in a **namespace** (say, a team) and a **table** (say, `notes`), each with its own `id`; two optional keys (`key1`, `key2`) tag records further; texts can be plain, Markdown, HTML or JSON.
- 🚫 **Not for:** a few pages (just paste them), exact lookups by id or number (use a database), or anything that must stay out of an AI provider (the text goes to the classifier and embedding services).

## How it is built

- **An app using other helpers;** it never holds any provider key itself.

| Block | Its job here |
|---|---|
| [`#ai`](ai.md#typed-decisions) | Reads the text and picks its topics from your list (the *Jev* classifier) |
| [`#voyage`](voyage.md#use-examples) | Makes the meaning fingerprint |
| [`#qdrant`](qdrant.md#what-it-does) | Stores texts and fingerprints |
| bus KV | Keeps your topic lists (prompts) |

## You'll need

- The three blocks running: [`#qdrant`](qdrant.md#what-it-does), [`#voyage`](voyage.md#use-examples), [`#ai`](ai.md#typed-decisions).
- A storage collection the owner creates once ([below](#owner-provisions-the-collection)).

## Try this first

Use the complete name of your [enabled instance](#installing-it) in the commands below.

- Give a table its topic list; this makes no AI call yet:

```sh
agent-bus call '#rag@node.example' '{"method":"prompts","action":"set","namespace":"team","tableName":"notes","data":{"labels":{"drink":"Beverages","other":"Other topics"},"instructions":"Choose the topic."}}'
```

## Use examples

- **Store a note (sorted and fingerprinted):** `agent-bus call '#rag@node.example' --wait=120s '{"method":"upsert","namespace":"team","tableName":"notes","id":"tea","data":"Tea is a drink.","source":"notes/tea.md"}'`
- **Read it back:** `agent-bus call '#rag@node.example' '{"method":"get","namespace":"team","tableName":"notes","id":"tea"}'`
- **Find by words:** `agent-bus call '#rag@node.example' '{"method":"search","namespace":"team","query":"green tea","limit":5}'` returns the effective `mode` and live citations; `tableName`, `key1` and `key2` narrow it. Set `mode` to `semantic` for meaning or `hybrid` for both.
- **Best first, paid:** add `"rerank":true` to a search to reorder its hits with the `rerank_model`; one Voyage call per search.
- **See what is stored:** `agent-bus call '#rag@node.example' '{"method":"list","namespace":"team","limit":16}'`
- **Wipe one table:** `agent-bus call '#rag@node.example' '{"method":"delete","namespace":"team","tableName":"notes","all":true}'`

<details>
<summary>Rules for namespace and delete</summary>

- Every data call needs an explicit namespace, including prompts and `keyword_index`. Blank, omitted and wildcard namespaces are refused. Only static `capabilities` and manager-only namespace inventory are namespace-free.
- Delete by id requires tableName and forbids key filters and `all`; without id, `all:true` confirms the selection.
- Delete returns `{"status":"ok"}` without a count and never deletes prompt KV. A missing id gives `not_found`, including when that id exists only elsewhere; a retained tombstone can finish an interrupted cleanup.

</details>

## Installing it

1. Use the RAG package included with the bus installation.
2. Register the name with owner-only access and hand its token privately to the runner, as for [agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves); the runner does not register this mode.
3. Set `enabled:true` for `rag` in the [runner inventory](../runner/configuration.md#inventory), preserving other entries.
4. Copy the [Configuration example](../../src/agents/rag/config.json.dist), set the record Configuration, provision the collection and grants below, reload the runner.

- It is a kept, sandboxed package with one worker, serializing replacement and prompt-cache changes.
- Help works before Configuration is set: `agent-bus help '#rag@node.example'` and the status ask answer on an unconfigured helper, while ordinary text is refused as invalid JSON. `capabilities` also works before setup; other valid data calls require Configuration.

### Owner provisions the collection

- **The app never creates storage or gives itself rights;** the owner does both once.

<details>
<summary>Commands (example: voyage-3.5, 1024 dimensions, collection <code>notes</code>)</summary>

Run as owner or admin of the Qdrant proxy. Use one collection per instance.

```sh
agent-bus call '#qdrant@node.example' 'request PUT /collections/notes {"vectors":{"size":1024,"distance":"Cosine","on_disk":false},"sparse_vectors":{"text":{"modifier":"idf"}},"strict_mode_config":{"enabled":true},"metadata":{"embedding_model":"voyage-3.5"}}'
for field in namespace tableName id key1 key2 _rag_kind _rag_labels _rag_rev; do
  agent-bus call '#qdrant@node.example' "request PUT /collections/notes/index?wait=true {\"field_name\":\"$field\",\"field_schema\":\"keyword\"}"
done
```

Then grant the blocks to this app, keeping existing grants:

```sh
agent-bus manage '#qdrant@node.example' --add-to-set-allow '#rag@node.example(write)'
agent-bus manage '#voyage@node.example' --add-to-set-allow '#rag@node.example'
agent-bus manage '#ai@node.example' --add-to-set-allow '#rag@node.example'
```

- At startup and on every data operation the app reads collection information to verify model metadata, vector settings and indexes; missing or mismatched settings refuse use.
- Changing the index list later is an owner or admin action.
- `sparse_vectors.text` with the `idf` modifier is what `search` uses. A collection made without it keeps working for everything else, and keyword or hybrid search refuses until its index is ready; semantic search still works.
- **Build it on an existing collection:** `agent-bus call '#rag@node.example' --wait=120s '{"method":"keyword_index","namespace":"team"}'`, as the helper's Owner or a Maintainer. It adds the vector with the `idf` modifier and then gives that namespace's stored texts their words in batches; run it again if it answers `incomplete`, or after an interruption, and it continues where it stopped; each update is kept under 512 KiB of vectors, one scroll and one update per batch of at most 64 texts.
- A second run on a finished collection changes nothing and answers `indexed: 0`.
- Keyword or hybrid search checks once a minute that no text lacks its words (another writer, an older build or a restore can add some) and refuses with `keyword index incomplete` until `keyword_index` has run.
- **If the proxy refuses to add the vector,** the answer names one command to run as the Qdrant proxy's owner: `request PUT /collections/notes/vectors/text {"sparse":{"modifier":"idf"}}`, because adding a vector needs the proxy's admin role and the helper only has write. Then run `keyword_index` again.
- The [collection API](https://api.qdrant.tech/api-reference/collections/create-collection) supports model metadata; [strict filtering](https://qdrant.tech/documentation/search/text-search/text-filtering/) requires payload indexes.
- [Collection information](https://api.qdrant.tech/api-reference/collections/get-collection) defines `config.metadata` and the keyword payload type; the [Decisions reference](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request) defines string state, typed Choice questions and `answers.<name>.probabilities`.
- A collection removed between verification and a point read may give a missing result; repeating the read verifies again.

</details>

## Configuration

- **One model and vector size per instance.**

<details>
<summary>Keys</summary>

| Key | Meaning |
|---|---|
| `qdrant` | Complete Qdrant proxy bus name; needs this app's own write grant |
| `voyage` | Complete Voyage proxy bus name; needs this app's own allow grant |
| `ai` | Complete Decisions proxy bus name; needs this app's own allow grant |
| `collection` | Owner-created dedicated collection; never auto-created |
| `embedding_model` | Fixed embedding model, checked against collection metadata |
| `dimensions` | Fixed vector size the model supports |
| `decision_model` | Jev model, default `typesafe/jev-1.14` |
| `keyword_search` | `builtin` (default): `search` uses the collection's BM25 sparse vector `text`; `external` is accepted but has no connected engine in this helper and cannot search |
| `rerank_model` | Voyage rerank model for `search` with `rerank:true`, such as `rerank-2.5`; empty (default) turns rerank off |

</details>

## Secret keys

- **None.**
- Provider keys stay with the blocks and are never copied here.

## Who may use it

- **Only the owner at first.**
- **One trusted application chooses the namespace:** any caller admitted to this instance can name its namespaces. Namespace filtering prevents accidental cross-project selection; mdhouse must enforce its own user/project grants. Use separate instances for untrusted callers.
- Namespace inventory and drop, and `keyword_index`, require the agent's delivered Owner or Maintainer role. Naming a role in the body grants nothing.
- **RAG calls the blocks under its own name,** not the caller's.

## Search and source evidence

| Mode | Retrieval | Provider work |
|---|---|---|
| `keyword` (default) | BM25 word matches | None |
| `semantic` | Dense similarity using the namespace's embedding model | One query embedding |
| `hybrid` | Qdrant RRF fusion of dense and BM25 matches | One query embedding |

- An unavailable mode or index refuses; it never silently falls back. Query embedding uses the same configured model and dimensions as stored documents.
- Every hit carries namespace, table, record id, chunk index, source/path, `content_hash`, `excerpt`, `scores` and `indexed_at`. Scores are labelled `bm25`, `dense` or `rrf`; a fusion score is not a probability or a fabricated pair of component scores.
- Upsert requires `source`: a path or URI supplied as data, never opened by RAG. `content_hash` is SHA-256 of the exact submitted text's UTF-8 bytes, before extraction; a supplied hash must match. `indexed_at` is the published revision's UTC time.
- Keep the citation's hash and excerpt: replacing a record removes its old content. `get` reads the current record, not historical revisions.

### Rerank

- **On:** `"rerank":true` in a search, with `rerank_model` set. It works in every mode. Without a model, or with `keyword_search` external, the search is refused as `rerank_unavailable` before any paid call.
- **Cost:** each reranked search is **one paid Voyage call**, on top of the query embedding that semantic and hybrid already make. A search with no hits makes none.
- **What is reranked:** the query's own candidates after the filters, at most four times `limit` (64 at most), never the whole namespace. Each candidate's excerpt is sent with the query.
- **Answer:** `reranked: true`, the hits best first by the model, and each hit's `scores` gains `rerank` beside its `bm25`, `dense` or `rrf` score. The other fields are unchanged.
- **If rerank fails:** the hits keep their unreranked order and scores, and the answer says `reranked: false` with `rerank_failure`. There is no partial reorder. The failure is one of:

| `rerank_failure` | Meaning |
|---|---|
| `provider_unavailable` | `#voyage` failed or did not answer |
| `provider_refused` | Voyage refused the request, such as an unknown model |
| `deadline` | The message deadline was reached first |
| `model_mismatch` | Voyage answered with another model than `rerank_model` |
| `invalid_answer` | The answer was not one relevance score for each candidate |
| `request_too_large` | The candidates' excerpts exceed the 1 MiB request cap; nothing was sent |

## Namespace state and removal

- A new namespace binds immutably to the configured model and dimensions before any paid work. A failed first insertion can leave a known empty namespace. Binding drift refuses before payment.
- Use a new matching collection or instance and explicitly re-feed content when changing models. A new namespace in a mismatched collection is insufficient.
- Namespace-free records, bindingless nonempty namespaces and records lacking current provenance are refused as `legacy_data`. Keep an owner backup and re-feed into fresh storage; there is no implicit default or automatic migration.
- Manager-only `namespaces` returns bounded pages of known names and exact live-head counts, including zero for known empty namespaces. It reveals no record content and never substitutes zero for a failed count. A page is not a snapshot across calls.
- Manager-only `drop_namespace` requires `confirm` to equal `namespace` exactly. It removes that namespace's points, prompt KV and binding, never another namespace or the shared collection.
- Drop persists its intent first. A partial or uncertain outcome blocks ordinary calls to that namespace across restart; continue only by explicitly sending the same confirmed drop. Nothing retries destruction automatically. A completed repeat reports `absent`.

## Troubleshooting

| If you see… | Do this |
|---|---|
| A refusal about collection settings or grants | Create the collection and grants as in [Installing it](#installing-it) |
| `not stored; prior record unchanged` | Fix the request; nothing changed |
| The outcome is **unknown** | Inspect the affected record and follow that operation's recovery instructions; a repeated upsert can spend provider balance again or replace newer content |
| `incomplete: repeat the same delete to continue` | Repeat the delete |
| A stale-cursor error | Start that listing again without a cursor |
| `model_mismatch` or `legacy_data` | Inspect the namespace binding and [storage requirements](#namespace-state-and-removal); never infer a model from old records. `model_mismatch` is also raised when the embedding provider answers with a different model than configured: nothing is stored or searched, so fix the provider or the setting and repeat. An answer that names no model is accepted |
| `busy` after a namespace drop | Explicitly confirm continuation of that drop |
| `not_found` | The id is absent from the selected namespace |
| `search` says `keyword index not built yet` | Run `keyword_index` ([Installing it](#installing-it)) |
| `search` says `keyword index incomplete (n chunks left)` | An interrupted build left some texts without words; run `keyword_index` again until it answers `ok`; searches are refused until then rather than answering from part of the data |
| `search` answers `incomplete: search window exhausted before enough records were found` | The best matches were texts nobody can read any more (never-published drafts, replaced revisions awaiting cleanup) or too few records match; narrow the query, add `tableName` or keys, or remove the leftovers |
| `search` says the text vector lacks the idf modifier | Recreate that vector with `{"sparse":{"modifier":"idf"}}`; Qdrant cannot change a modifier in place |
| `rerank_unavailable` | Set `rerank_model`, or search without `rerank`; external keyword search cannot be reranked |
| `reranked: false` | The hits are valid, in their unreranked order; `rerank_failure` names why ([rerank](#rerank)) |
| `search` says no external keyword engine is connected | Set `keyword_search` to `builtin`; this helper has no external engine |

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#rag` reports these.

| Level | When | What to do |
|---|---|---|
| alert | Its collection is missing or unhealthy, or its vector size, model or indexes do not match Configuration | Create or fix the collection as in [Installing it](#installing-it) |
| alert | A block (`#qdrant`, `#voyage`, `#ai`) refuses RAG's own grant | Grant RAG on that block |
| alert | Configuration lacks the block names or the collection | Set them |
| warning | The collection is optimizing (yellow) | Nothing; it clears by itself |
| warning | A call's outcome is unknown, or a delete is incomplete | Inspect an unknown outcome before retrying; follow the named continuation for an incomplete delete and check the block's own entries |

## Technical details

<details>
<summary>Input rules and how records are stored</summary>

Input:

- Upsert fully replaces the record: omitted key1 and key2 clear them; an empty key string is valid; input `classification_ids` is refused, Jev derives that output label-to-probability object.
- Calls are JSON only; [agent.json](../../src/agents/rag/agent.json) owns the fields, types, bounds and examples; identity and filter strings must already be NFC.
- Duplicate JSON keys, NaN and Infinity, numeric booleans, BOM, NUL and invalid UTF-8 are refused; there are no text aliases.
- Size limits are checked before any provider call; one Jev call per record, at most two Voyage batches; a Jev or Voyage failure aborts before any storage change.

Replacement and recovery:

- Every chunk carries namespace, tableName, id and `_rag_rev`, a SHA-256 of canonical typed source, provenance, MIME, optional keys, selected label IDs, embedding model, chunk-format version and final float32 vector bytes.
- With `keyword_search` builtin and a collection that has the `text` sparse vector, each chunk is also stored with its words: lower-cased Unicode words hashed to Qdrant sparse indices, weighted by saturated term frequency (BM25, k1 1.2, b 0.75); Qdrant's `idf` modifier adds the rarity weight, and `search` queries those chunks by the query's words. `search` returns a chunk only while its record's current head lists it, so a text whose first upsert failed, an old revision awaiting cleanup and a record being deleted are never returned; it returns the best chunk per record and never grants access beyond the filters.
- Chunk UUIDs contain identity, revision and chunk index; the head UUID is stable per identity; full probabilities stay in the head only.
- Upsert writes new chunks (`wait=true`), publishes the head (`wait=true`), then deletes stale chunks by **identity and revision != new revision filter** (`wait=true`); cleanup never depends on a list saved in the head.
- A failure before head publication keeps the previous head; a later failure can leave old and new chunks, which repeating upsert or delete repairs.
- Delete hides heads with a `deleted` tombstone, filter-deletes all chunk revisions, then removes the tombstone; it also sweeps headless chunks, including a failed first insertion.
- A tombstone keeps source text until cleanup finishes; `incomplete: repeat the same delete to continue` resumes a large wipe.
- A definite first-write refusal says **not stored; prior record unchanged**: 400, 401, 403, 404, 405, 413, 415, 422 or a local proxy role refusal; 408, 409, 429, 499 and other ambiguous failures never claim the prior state is unchanged; no cross-call transactions.
- UUID namespace, tuple encoding and chunk-boundary version are permanent storage contracts; changing them makes persisted data incompatible.
- Splitting prefers the last paragraph, line or space boundary in the latter half of a 4096-byte window, else hard-cuts at a complete UTF-8 character; identical prepared content rewrites the same points; a new indexing time, source, Jev labels or vectors give a new revision.
- Search filters `_rag_kind=chunk` and checks each hit's revision against its head.

</details>

<details>
<summary>Limits</summary>

- Source at most 32 KiB UTF-8, split into at most eight chunks of at most 4 KiB; whitespace break points may need more chunks and refuse early, naming the count and limit.
- HTML drops script, style, noscript and template without fetching and bounds stack depth and size, accounting for common omitted end tags; Markdown uses source text; JSON renders canonically.
- Jev (`typesafe/jev-1.14`) classifies the extracted text (at most 32 KiB) once per record; unknown or missing labels and nonfinite probabilities are refused; threshold and top-k select by probability descending, then label ID.
- Voyage: at most two batches of four chunks; each encoded request below 1 MiB; explicit `output_dimension` and float output; choose a size the model supports (Voyage 3.5: 256, 512, 1024, 2048); its refusal is not bypassed.
- Before any provider call the full record is estimated (ASCII-escaped source and metadata, largest top-k label output, 25 bytes per vector scalar, fixed overhead) and refused above 256 KiB, naming estimate and limit; at most 8192 vector scalars across chunks.
- Vectors round-trip through float32 and shortest JSON float; nonfinite values and float32 overflow are refused.
- The head stores data, metadata, labels and ordered chunk IDs, not a second vector copy, plus one float32 placeholder vector (1.0 then zeros): `dimensions × 4` bytes per record.
- `get` fetches vectors and identity/provenance fields for at most eight chunks without chunk text; if a concurrent writer removes a chunk, it re-reads the head once, then returns a clear incomplete-record error; chunk text is stored for search snippets and counts in write budgets.
- `list` returns at most 16 summaries (no data or vectors) of at most 32 KiB, in canonical point-ID order; sizes are checked on every backend reply and before writes; cursor tokens are bound to filters and page size, kept in a 128-entry LRU map, and expire after ten minutes, restart or eviction with an error; listing is not a snapshot across calls.
- Namespace metadata is one own-KV catalogue, bounded at 128 namespaces, 256 tables per namespace and 256 KiB encoded JSON, reserving space for drop intents. Confirmed metadata writes precede paid calls; uncertain writes do not seed caches.
- Prompt cache hits make no prompt-KV call; set and delete invalidate on success and on uncertain timeout or disconnect, never seed from a write; external KV edits appear after restart; keys are `rag.prompts.` plus SHA-256 of the typed namespace and table tuple (76 ASCII bytes).
- The 120-second message cap is one budget across all stages; the caller's earlier deadline wins; provider calls get at most min(30 seconds, remaining); Qdrant and KV calls min(10 seconds, remaining), without resetting between stages.

</details>

### If an upsert's outcome is unknown

A timeout, a lost reply or an answer that says the outcome is **unknown** means RAG may or may not have stored the record. RAG has no deduplication: an upsert carries no request id or idempotency key, so a repeat is a new write.

- **Check first, at no cost:** `get` the same `namespace`, `tableName` and `id`. It reads storage only and makes no provider call.
- **Compare `content_hash`:** it is the SHA-256 of the exact text's UTF-8 bytes. Compute the same hash of the text you sent.
- **Same hash, and the same `source`, `tableName` and `id` as you sent:** the text is stored. Do not retry for the text. The hash proves the stored text only: a head that failed to publish can leave an old `source` beside an equal hash, and classification, metadata or old-chunk cleanup are not proved by it. If any field differs, do not repeat blindly: a different `source` or metadata may be a newer concurrent version. Compare `indexed_at` (and your own record of which version should win), or decide explicitly to overwrite, and only then repeat the upsert.
- **`not_found`:** it is not stored (or was deleted). A retry classifies and embeds the text again, so it can spend provider balance a second time.
- **Another hash:** a different version is stored, yours or a newer one. A retry replaces it, so compare `indexed_at` and retry only when yours should win.
- **Wait the call's own deadline before checking:** an attempt still in flight can store the record after your check, and a retry then pays twice.

### What 0.3 does not claim

- **No end-to-end ingestion journey:** RAG stores what it is sent. It does not fetch, watch or schedule anything, and it keeps no job, progress or retry record. Feeding a folder is [FileScan](filescan.md#what-it-does)'s job.
- **No historical revisions or generations:** `get` and `search` read the current record. Replacing a record removes its old content, `indexed_at` is the current revision's time, and there is no list of earlier versions and no rollback.
- **No deduplication:** the same text sent twice is classified and embedded twice.
