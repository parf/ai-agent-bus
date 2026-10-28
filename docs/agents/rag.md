# RAG core (pending implementation)

📌 **TL;DR:** This R.6 contract stores records through Qdrant, classifies through
AI and embeds through Voyage. Implementation is pending; the JSON manifest is
reviewed before parser code. The app starts at its own version 0.1.0.

## Why you'd want it

Keep classified records in one collection while reusing the bus's provider
proxies and their private host credentials.

## What it does

[agent.json](../../src/agents/rag/agent.json) owns the JSON-only methods, types,
bounds and examples. No text aliases are accepted. Upsert fully replaces the
record: omitted key1/key2 clear them; an empty key string is valid. Input
`classification_ids` is refused; Jev derives that output label-to-probability
object. Identity/filter strings must already be NFC, not silently normalized.
Duplicate JSON keys, NaN/Infinity, numeric booleans, BOM, NUL and invalid UTF-8
are refused.

### Replacement and recovery

Every chunk carries namespace/tableName/id and `_rag_rev`. The revision is a
SHA-256 of canonical typed source, MIME, optional keys, selected label IDs,
embedding model, chunk-format version and final float32 vector bytes. Chunk
UUIDs contain identity, revision and chunk index; the head UUID is stable for
the identity. Full probabilities stay in the head only.

Upsert new chunks with `wait=true`, publish the head with `wait=true`, then
delete stale chunks by **identity and revision != new revision filter**, with
`wait=true`. Cleanup never depends on a list saved in the head. A failed
upsert preserves the previous head; a later failure can leave old+new chunks.
Repeating upsert/delete repairs those orphans. Delete filters every revision;
it never only deletes IDs read from the head. R.7 must check search hits
against the canonical head's revision before returning them.

An uncertain write says the outcome is **unknown** and repeating the same
upsert/delete is safe. There are no automatic retries or cross-call transaction
claims. UUID namespace, tuple encoding and chunk-boundary version are permanent
storage contracts; changing them requires a migration.

### Bounds before spending

Source is at most 32 KiB UTF-8, split deterministically into at most eight
4 KiB chunks. HTML drops script/style without fetching and bounds depth/size;
Markdown uses source text and JSON renders canonically. Jev classifies the
extracted text once per record (at most 32 KiB), not each chunk. Reject unknown
or missing labels and nonfinite probabilities; threshold/top-k selects by
probability descending, then label ID. The decision model is
`typesafe/jev-1.14`. Voyage makes at most two batches of four chunks; any batch
failure aborts before any storage mutation. Each encoded provider request is
below 1 MiB.

Before **any** provider call, budget ASCII-escaped source and metadata, the
largest configured top-k label output, vector scalar count times 25 bytes
(24 characters for a shortest JSON float plus its delimiter), and fixed
overhead. Refuse an estimate above the 256 KiB full-record cap, naming the
estimate and limit. The per-field maxima are not a promise that every
combination fits: high dimensions or heavily escaped text may refuse early.
Vectors round-trip through float32 then use standard shortest JSON float
representation; reject nonfinite values and float32 overflow. There are at
most 8192 vector scalars across all chunks.

Head payload stores data, metadata, labels and ordered chunk IDs, **not a
second vector copy**. `get` fetches vectors for at most eight chunk points to
reconstruct the full record. `list` returns summaries without `data` or
`embeddings`, at most 16 summaries of at most 32 KiB each. Backend scroll
excludes vectors. That is at most 512 KiB plus the wrapper, below the proxy's
1 MiB reply cap. Check actual sizes on every backend reply and before writes.

List order is canonical point ID. Backend UUID offsets remain opaque and
shape-checked; callers get tokens bound to their filters and page size from a
128-entry LRU map. Expiry is ten minutes, restart or eviction; a stale token
gets an error. Listing across calls is not a transactional snapshot.

Prompt edits do not reclassify existing records. Prompt cache hits make no KV
call; set/delete invalidates on success and uncertain timeout/disconnect,
never seeds from a write. External KV edits appear after restart. Cache keys
are `rag.prompts.` plus SHA-256 of the typed namespace/table tuple: 76 ASCII
bytes, below the daemon's 256-byte UTF-8 KV-name limit. R.5 still chooses the
keyword encoder; R.6 claims neither hybrid search nor summary generation.

## You'll need

R.4's Qdrant proxy, Voyage and AI, with this app's own downstream grants; an
owner-provisioned dedicated collection and indexes. Live use remains pending.

## Try this first

Once implemented and configured, set a table's classification prompts with the
manifest's `prompts` example. No provider call occurs until ingestion.

## Use examples

Pending implementation; these show the exact accepted contract:

```sh
agent-bus call '#rag@node.example' --wait=120s '{"method":"upsert","namespace":"team","tableName":"notes","id":"tea","data":"Tea is a drink."}'
agent-bus call '#rag@node.example' '{"method":"get","namespace":"team","tableName":"notes","id":"tea"}'
agent-bus call '#rag@node.example' '{"method":"list","namespace":"team","limit":16}'
agent-bus call '#rag@node.example' '{"method":"delete","namespace":"team","tableName":"notes","all":true}'
```

Namespace is always required. Empty/null/blank namespace, or table alone,
refuses rather than matching everything. Delete by id requires tableName and
forbids key filters/all; without id, `all:true` explicitly confirms the filtered
selection. This flag is method control, not stored record data. Delete returns
`{"status":"ok"}` without a count, and never deletes prompt KV.

## Installing it

The app is not runnable yet. It will be a kept, sandboxed runner package with
one worker, preserving serialization of replacement and prompt-cache changes.
No live data, provider calls or grant changes are part of R.6 deployment.

### Owner provisions the collection

Run these only as owner/admin of the configured Qdrant proxy after R.4 is
installed. They are documentation, never commands the app runs. Substitute
block names and use a separate collection per instance. This example matches
`embedding_model=voyage-3.5`, `dimensions=1024`, collection `notes`:

```sh
agent-bus call '#qdrant@node.example' 'request PUT /collections/notes {"vectors":{"size":1024,"distance":"Cosine","on_disk":false},"strict_mode_config":{"enabled":true},"metadata":{"embedding_model":"voyage-3.5"}}'
for field in namespace tableName id key1 key2 _rag_kind _rag_labels _rag_rev; do
  agent-bus call '#qdrant@node.example' "request PUT /collections/notes/index?wait=true {\"field_name\":\"$field\",\"field_schema\":\"keyword\"}"
done
```

The [collection API](https://api.qdrant.tech/api-reference/collections/create-collection)
supports model metadata; [strict filtering](https://qdrant.tech/documentation/search/text-search/text-filtering/)
requires payload indexes. The implementation will check this provisioning list against one constant
used by startup verification, including `_rag_rev`. The app reads collection information
to verify model metadata, vector settings and indexes; missing/mismatched
settings refuse use. It never creates collections/indexes or escalates grants.
Changing that index contract later is an owner/admin provisioning action.

Grant downstream access while preserving existing grants:

```sh
agent-bus manage '#qdrant@node.example' --add-to-set-allow '#rag@node.example(write)'
agent-bus manage '#voyage@node.example' --add-to-set-allow '#rag@node.example'
agent-bus manage '#ai@node.example' --add-to-set-allow '#rag@node.example'
```

## Configuration

The manifest lists block names, collection, embedding model/dimensions and the
decision model. One model/vector size per instance. The 120-second message cap
shares one budget across all stages; the caller's earlier deadline wins.
Provider calls get at most min(30 seconds, remaining); Qdrant/KV calls get
min(10 seconds, remaining), without resetting the budget between stages.

## Secret keys

None. The runner supplies this app's bus credential; provider keys stay in the
blocks' host environments and are never copied into this app.

## Who may use it

Owner-only initially. Namespace/table are filters, not permissions: every
caller later admitted can read/delete all namespaces. This app uses its own
bus identity, not the caller's, with explicit Voyage/AI and Qdrant write grants.

## Troubleshooting

Missing/mismatched collection settings or grants refuse live operations.
Unknown writes are not retried automatically; repeat the same request to
converge. Prompt changes affect future upserts only. No real provider call is
made without owner approval.
