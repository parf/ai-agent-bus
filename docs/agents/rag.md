# RAG core

📌 **TL;DR:** Store records through Qdrant, classify through AI and embed through Voyage.
The JSON-only core has its own version, starting at 0.1.0. Search and summaries
are later steps; live provider ingestion remains unverified.

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
`wait=true`. Cleanup never depends on a list saved in the head. A failure before head publication preserves the previous head; a later
failure can leave old+new chunks.
Repeating upsert/delete repairs those orphans. Delete hides heads first using a `deleted` tombstone, then filters all chunk
revisions, then removes the tombstone. It never only deletes IDs read from the
head. Filtered deletion also sweeps matching chunks without a head, including
a failed first insertion. A tombstone retains source text until cleanup
finishes; `incomplete: repeat the same delete to continue` resumes a large wipe. R.7 must check search hits
against the canonical head's revision before returning them.

A definite first-write refusal (400/401/403/404/405/413/415/422 or a local
proxy role refusal) says **not stored; prior record unchanged**.
408/409/429/499 and other ambiguous failures never claim the prior state is
unchanged. An uncertain write or a later partial-write failure says the outcome is **unknown** and repeating the same
upsert/delete is safe. There are no automatic retries or cross-call transaction
claims. UUID namespace, tuple encoding and chunk-boundary version are permanent
storage contracts; changing them requires a migration. The initial splitting
rule prefers the last paragraph, line or space boundary in the latter half
of a 4096-byte window, and hard-cuts only at a complete UTF-8 character when
none fits. Repeating input can get a new revision when Jev selects different
labels or final vectors change; identical prepared content rewrites the same
points.

### Bounds before spending

Source is at most 32 KiB UTF-8, split deterministically into at most eight
chunks of at most 4 KiB. This is an upper bound, not unconditional acceptance:
whitespace break points may require more chunks and refuse early, reporting
the observed count and limit. HTML drops script/style/noscript/template without fetching and bounds actual
stack depth/size, accounting for common omitted end tags;
Markdown uses source text and JSON renders canonically. Jev classifies the
extracted text once per record (at most 32 KiB), not each chunk. Reject unknown
or missing labels and nonfinite probabilities; threshold/top-k selects by
probability descending, then label ID. The decision model is
`typesafe/jev-1.14`. Voyage makes at most two batches of four chunks; any batch
failure aborts before any storage mutation. Each encoded provider request is
below 1 MiB. Voyage receives explicit `output_dimension` and float output;
unsupported model/dimension combinations are left to its refusal. Choose a
size supported by the configured model (Voyage 3.5: 256/512/1024/2048).

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
second vector copy in payload**. The unnamed-vector collection still needs
one float32 placeholder vector on each head (1.0 in position 0, zeros elsewhere), adding
`dimensions × 4` bytes per record before backend overhead. All R.7 searches
must filter `_rag_kind=chunk` and verify the hit revision against its head.
`get` fetches vectors for at most eight chunk points to
reconstruct the full record, requesting no chunk payload/text. Chunk text is
stored for R.7 snippets and is included in write-budget checks. If a concurrent
writer removes a chunk between reads, re-read the head once, then return a
clear incomplete-record error. `list` returns summaries without `data` or
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

Once configured, set a table's classification prompts with the
manifest's `prompts` example. No provider call occurs until ingestion.

## Use examples

These calls use the accepted JSON contract:

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

Build `src/agents/rag/package.sh <agent.d/rag>`, then enable `rag` in the
[runner inventory](../runner/configuration.md#inventory). Copy the generic
[Configuration example](../../src/agents/rag/config.json.dist), set the record
Configuration, provision the collection and grants below, then reload the
runner. It is a kept, sandboxed package with one worker, serializing replacement
and prompt-cache changes. Help works before Configuration is set.
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
requires payload indexes. The implementation checks this provisioning list against one constant
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

### Contract evidence and limits

The [Decisions reference](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request)
defines string state, typed Choice questions and `answers.<name>.probabilities`;
R.2 supplies the transparent call. [Collection information](https://api.qdrant.tech/api-reference/collections/get-collection)
defines `config.metadata` and the payload schema's keyword data type. Tests
use these shapes, not invented HTTP APIs. Live Jev classification remains
unverified until the owner approves a provider call. The live store had no
collection to inspect during R.6's read-only check; no collection was created.
Every CRUD operation deliberately verifies its collection again within the
same deadline. A collection removed between verification and a point read may
produce a missing result; repeating the read verifies the collection again.
The private Python KV request boundary is pinned by a real-bus fixture.
