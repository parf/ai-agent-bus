# RAG: a memory your AI can search

📌 **TL;DR:**
- **Problem:** an AI only knows what is in front of it, and your notes, mails and documents are scattered; keyword search misses "car" when you typed "vehicle".
- **Outcome:** a shared notebook that stores each text sorted into your own topics, with its meaning remembered, so you or your AI can read it back; search by meaning is in work.
- **Start:** give a table its topic list (no AI call yet): [Try this first](#try-this-first).

## Why you'd want it

- 🧠 **An AI forgets.** Now texts live in one place and a session can recall them later.
- 🗂️ **Texts are scattered.** Now each is sorted automatically into **your own** categories.
- 🔎 **Keyword search misses meaning.** Now each text carries a "meaning fingerprint" (an *embedding*: numbers where texts about the same thing land close together).
- 👥 **Knowledge stays in one head.** Now several people or AI sessions share the same texts.

## What it does

- ✅ **Store a text:** it is sorted into your topics, fingerprinted and kept (`upsert`).
- ✅ **Read it back:** one record, or a page of records in a namespace (`get`, `list`).
- ✅ **Delete:** one record, or a whole table when you confirm it (`delete`).
- ✅ **Keep topic lists:** read or set a table's topics (`prompts`); changing a list affects only texts added afterwards.
- ✅ **Safe to repeat:** a failure says plainly whether **nothing changed** or the outcome is **unknown**; repeating the same call is always safe; nothing retries by itself.
- ✅ **Use cases:** sort a support inbox into billing, bug, sales and spam; a team knowledge base; meeting and call notes; research clippings; facts an AI session wants to keep.
- ⚙️ **Records:** live in a **namespace** (say, a team) and a **table** (say, `notes`), each with its own `id`; two optional keys (`key1`, `key2`) tag records further; texts can be plain, Markdown, HTML or JSON.
- 🚫 **Not for:** a few pages (just paste them), exact lookups by id or number (use a database), or anything that must stay out of an AI provider (the text goes to the classifier and embedding services).
- ℹ️ **Search by meaning, keyword search and summaries are in work.**

## How it is built

- **A small app made of other helpers,** like Lego blocks; it never holds any provider key itself.

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

- Give a table its topic list; this makes no AI call yet:

```sh
agent-bus call '#rag@node.example' '{"method":"prompts","action":"set","namespace":"team","tableName":"notes","data":{"labels":{"drink":"Beverages","other":"Other topics"},"instructions":"Choose the topic."}}'
```

<details>
<summary>See what an unconfigured helper says (ran today)</summary>

```sh
agent-bus call '#rag@parf.us' '{"method":"list","namespace":"team","limit":16}' | tail -1 | jq -r .body
```

```text
error: configure RAG block names and collection
```

- On the development node the helper is running but not yet configured; the output is from 2026-10-30.
- Step by step: [Installing it](#installing-it).

</details>

## Use examples

- **Store a note (sorted and fingerprinted):** `agent-bus call '#rag@node.example' --wait=120s '{"method":"upsert","namespace":"team","tableName":"notes","id":"tea","data":"Tea is a drink."}'`
- **Read it back:** `agent-bus call '#rag@node.example' '{"method":"get","namespace":"team","tableName":"notes","id":"tea"}'`
- **See what is stored:** `agent-bus call '#rag@node.example' '{"method":"list","namespace":"team","limit":16}'`
- **Wipe one table:** `agent-bus call '#rag@node.example' '{"method":"delete","namespace":"team","tableName":"notes","all":true}'`

<details>
<summary>Rules for namespace and delete</summary>

- Namespace is always required; empty, blank or table alone is refused rather than matching everything.
- Delete by id requires tableName and forbids key filters and `all`; without id, `all:true` confirms the selection.
- Delete returns `{"status":"ok"}` without a count and never deletes prompt KV.

</details>

## Installing it

- An administrator does this once: install the package, create the storage collection, let RAG use the three blocks, set its Configuration.

<details>
<summary>Step by step</summary>

1. Build `src/agents/rag/package.sh <agent.d/rag>`.
2. Register the name with owner-only access and hand its token privately to the runner, as for [agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves); the runner does not register this mode.
3. Enable `rag` in the [runner inventory](../runner/configuration.md#inventory).
4. Copy the [Configuration example](../../src/agents/rag/config.json.dist), set the record Configuration, provision the collection and grants below, reload the runner.

- It is a kept, sandboxed package with one worker, serializing replacement and prompt-cache changes.
- Help works before Configuration is set.

</details>

### Owner provisions the collection

- **The app never creates storage or gives itself rights;** the owner does both once.

<details>
<summary>Commands (example: voyage-3.5, 1024 dimensions, collection <code>notes</code>)</summary>

Run as owner or admin of the Qdrant proxy. Use one collection per instance.

```sh
agent-bus call '#qdrant@node.example' 'request PUT /collections/notes {"vectors":{"size":1024,"distance":"Cosine","on_disk":false},"strict_mode_config":{"enabled":true},"metadata":{"embedding_model":"voyage-3.5"}}'
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

- At startup and on every operation the app reads collection information to verify model metadata, vector settings and indexes; missing or mismatched settings refuse use.
- Changing the index list later is an owner or admin action.
- The [collection API](https://api.qdrant.tech/api-reference/collections/create-collection) supports model metadata; [strict filtering](https://qdrant.tech/documentation/search/text-search/text-filtering/) requires payload indexes.
- The index list above matches one constant used by startup verification, including `_rag_rev`.
- [Collection information](https://api.qdrant.tech/api-reference/collections/get-collection) defines `config.metadata` and the keyword payload type; the [Decisions reference](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request) defines string state, typed Choice questions and `answers.<name>.probabilities`; tests use these shapes, not invented APIs.
- A collection removed between verification and a point read may give a missing result; repeating the read verifies again.

</details>

## Configuration

- **What:** which blocks to use, where to store, and which models.
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

</details>

## Secret keys

- **None.**
- Provider keys stay with the blocks and are never copied here.

## Who may use it

- **Only the owner at first.**
- **Namespace and table are labels, not locks:** anyone admitted later can read and delete every namespace.
- **RAG calls the blocks under its own name,** not the caller's.

## Troubleshooting

| If you see… | Do this |
|---|---|
| A refusal about collection settings or grants | Create the collection and grants as in [Installing it](#installing-it) |
| `not stored; prior record unchanged` | Fix the request; nothing changed |
| The outcome is **unknown** | Repeat the same request; nothing retries automatically |
| `incomplete: repeat the same delete to continue` | Repeat the delete |
| A stale-cursor error | Start `list` again without a cursor |

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#rag` reports these. The page that shows them is in work.

| Level | When | What to do |
|---|---|---|
| alert | Its collection is missing or unhealthy, or its vector size, model or indexes do not match Configuration | Create or fix the collection as in [Installing it](#installing-it) |
| alert | A block (`#qdrant`, `#voyage`, `#ai`) refuses RAG's own grant | Grant RAG on that block |
| alert | Configuration lacks the block names or the collection | Set them |
| warning | The collection is optimizing (yellow) | Nothing; it clears by itself |
| warning | A call's outcome is unknown, or a delete is incomplete | The caller repeats the same call; if it repeats, check that block's own entries |

## Technical details

<details>
<summary>Input rules and how records are stored</summary>

Input:

- Upsert fully replaces the record: omitted key1 and key2 clear them; an empty key string is valid; input `classification_ids` is refused, Jev derives that output label-to-probability object.
- Calls are JSON only; [agent.json](../../src/agents/rag/agent.json) owns the fields, types, bounds and examples; identity and filter strings must already be NFC.
- Duplicate JSON keys, NaN and Infinity, numeric booleans, BOM, NUL and invalid UTF-8 are refused; there are no text aliases.
- Size limits are checked before any provider call; one Jev call per record, at most two Voyage batches; a Jev or Voyage failure aborts before any storage change.

Replacement and recovery:

- Every chunk carries namespace, tableName, id and `_rag_rev`, a SHA-256 of canonical typed source, MIME, optional keys, selected label IDs, embedding model, chunk-format version and final float32 vector bytes.
- Chunk UUIDs contain identity, revision and chunk index; the head UUID is stable per identity; full probabilities stay in the head only.
- Upsert writes new chunks (`wait=true`), publishes the head (`wait=true`), then deletes stale chunks by **identity and revision != new revision filter** (`wait=true`); cleanup never depends on a list saved in the head.
- A failure before head publication keeps the previous head; a later failure can leave old and new chunks, which repeating upsert or delete repairs.
- Delete hides heads with a `deleted` tombstone, filter-deletes all chunk revisions, then removes the tombstone; it also sweeps headless chunks, including a failed first insertion.
- A tombstone keeps source text until cleanup finishes; `incomplete: repeat the same delete to continue` resumes a large wipe.
- A definite first-write refusal says **not stored; prior record unchanged**: 400, 401, 403, 404, 405, 413, 415, 422 or a local proxy role refusal; 408, 409, 429, 499 and other ambiguous failures never claim the prior state is unchanged; no cross-call transactions.
- UUID namespace, tuple encoding and chunk-boundary version are permanent storage contracts; changing them needs a migration.
- Splitting prefers the last paragraph, line or space boundary in the latter half of a 4096-byte window, else hard-cuts at a complete UTF-8 character; identical prepared content rewrites the same points; different Jev labels or vectors give a new revision.
- The search now in work will filter `_rag_kind=chunk` and check each hit's revision against its head.

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
- `get` fetches vectors for at most eight chunks without chunk text; if a concurrent writer removes a chunk, it re-reads the head once, then returns a clear incomplete-record error; chunk text is stored for search snippets and counts in write budgets.
- `list` returns at most 16 summaries (no data or vectors) of at most 32 KiB, in canonical point-ID order; sizes are checked on every backend reply and before writes; cursor tokens are bound to filters and page size, kept in a 128-entry LRU map, and expire after ten minutes, restart or eviction with an error; listing is not a snapshot across calls.
- Prompt cache hits make no KV call; set and delete invalidate on success and on uncertain timeout or disconnect, never seed from a write; external KV edits appear after restart; keys are `rag.prompts.` plus SHA-256 of the typed namespace and table tuple (76 ASCII bytes).
- The 120-second message cap is one budget across all stages; the caller's earlier deadline wins; provider calls get at most min(30 seconds, remaining); Qdrant and KV calls min(10 seconds, remaining), without resetting between stages.

</details>
