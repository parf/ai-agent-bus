# RAG changelog

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
