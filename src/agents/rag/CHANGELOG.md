# RAG changelog

## 0.4.2

Large Markdown files: `upsert` takes a `bigfile` reference up to 1 MiB, chunked by heading with per-chunk hashes, vector reuse for unchanged text, and heading-path and line citations; `get` with `check:true` answers a free completeness check. Existing records keep answering unchanged.

## 0.4.1

Add `count` (live records and records per label for a filter), `similar` (records nearest a stored record's vectors) and `complete` (endings of a typed word from stored text, through a prefix index `keyword_index` now creates), all free, and `classify` (labels for a text without storing it), one paid `#ai` call, off unless the new `classify` setting is true.

## 0.4.0

Rerank: `search` with `rerank:true` reorders its candidate window, at most four times `limit`, in one paid Voyage rerank call with the `rerank_model` setting.
A failed or invalid rerank keeps the unreranked order and answers `reranked:false` with a named `rerank_failure`; without a model, `rerank:true` is refused as `rerank_unavailable`.

## 0.3.0

Retrieve namespaced evidence with keyword, dense semantic or Qdrant RRF hybrid search.
Require source provenance and immutable namespace model bindings; refuse legacy data.
Add free static capabilities and manager-only namespace inventory and confirmed, resumable drop.
Refuse an embedding answer that names another model than configured (`model_mismatch`), for documents and queries alike; an answer naming no model is accepted.

## 0.2.0

Add keyword search: a `keyword_search` setting (`builtin` or `external`), BM25 ingest into a
`text` sparse vector, `search` over each record's live head, and `keyword_index` to add the
vector and backfill an existing collection. A half-built index and an exhausted search window
are refused rather than answered partly. Help, status and the other asks answer on an
unconfigured instance; a plain-text call is data and is refused until it is configured.

## 0.1.0

Add JSON CRUD, one-record classification and batched embeddings through bus blocks,
revision-safe replacement, scoped cleanup, prompt caching and bounded reads.
