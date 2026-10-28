# RAG service

📌 **TL;DR:** A simple RAG service on the bus. It stores records, classifies
them, embeds them and finds them again by meaning, by keywords and by filters.
The parts are a Qdrant container for hybrid search, Jev (via OpenRouter) for
classification on upsert, `#voyage` for embeddings, and `#ai` for an optional
summary. R1 work since 2026-10-27 (owner: "move it to R1 and give it to xAB; start with block building"); steps in [TODO](TODO.md#steps).

**Why it exists:** agents are the building blocks, and a simple app is built on
top of them (owner, 2026-10-27: "that's the idea - use agents as building
blocks; and build a simple app on top of them"). The RAG agent is the first such
app: it owns only its own logic and reaches everything else (`#voyage`, `#ai`,
the Qdrant store through `#qdrant`) by ordinary bus calls. When it needs something an agent lacks,
that agent grows, as `#ai` does for Jev; the RAG agent never gains a copy.

## Parts

| Part | Choice | Why |
|---|---|---|
| Hybrid search store | **Qdrant**, any reachable instance (typically a podman container), reached through the `#qdrant` agent | owner, 2026-10-27; one light binary, dense and sparse vectors fused in one query, indexed payload filters |
| Classification | **Jev** by TypeSafe AI, through OpenRouter, called on upsert **through `#ai`**, which learns OpenRouter's Decisions API, so the OpenRouter key stays in one place (owner, 2026-10-27, settling Q197) | owner, 2026-10-27; a decision model, not a text generator: fast, cheap, answers typed questions with probabilities |
| Embeddings | `#voyage` ([B.19](../TODO.md#bundled-agents)) | the owner's key, already proxied |
| Summary | `#ai`, optional | the existing OpenRouter proxy |

<details><summary>Jev in short</summary>

- It is not a chat model. It is called through OpenRouter's Decisions API, not `/chat/completions`.
- The input is a state (text, and images) plus typed questions. The question types are Choice (one of a set), Noul (yes/no probability), Score (a position on a scale) and Box (a region of an image).
- The answer is typed, with a probability. Billing is per input token only, and output is free.
- Text over about 32 KB runs in a slower, less accurate long-context mode.
- Models: `typesafe/jev-1.14`, alias `~typesafe/jev-latest`. In beta on OpenRouter.
- Sources: [OpenRouter Jev guide](https://openrouter.ai/docs/guides/community/jev), [Jev 1.14 listing](https://openrouter.ai/typesafe/jev-1.14).

</details>

## What a record carries

The owner's input list. The wire format waits for the owner, as for every
record shape ([working rules](../../../CLAUDE.md#working-rules)).

| Field | Role |
|---|---|
| namespace | a plain indexed field, filtered as `WHERE namespace = A` (owner, 2026-10-27: "namespace/table - we just support them as WHERE namespace=A and table=B") |
| tableName | a plain indexed field, filtered as `AND table = B` |
| id | the record's id within its table; namespace, tableName and id together are its identity |
| key1, key2 | filters |
| classification ids | the list Jev returns on upsert |
| embeddings | optional; computed through `#voyage` when absent |
| data | the content |
| mime-type | selects how text is extracted and chunked |

## Rules

| Rule | |
|---|---|
| Idempotent upsert | the point id derives from namespace, tableName and id, so the same record upserted twice replaces itself, and its old chunks go first |
| One model per instance | all records share one collection, so one embedding model and vector size; a vector of another size is refused at once |
| Hybrid search | dense vectors from Voyage plus sparse keyword vectors, fused in one Qdrant query; filters on tableName, key1, key2 and classification ids. The keyword half is chosen by a trial on the owner's real Russian and English texts: Qdrant's built-in BM25 against the agent's own sparse vectors with stemming ([Q194](../QUESTIONS.md#rag-service)) |
| Size | up to about 100k records per namespace: full float vectors, all in memory, no quantization (owner, 2026-10-27, settling Q195) |
| Rerank | optional, through `#voyage`'s pass-through to Voyage rerank |
| Classification on upsert | Jev assigns the classification ids before the record is stored; a Jev failure fails the upsert, explicitly |
| Many labels from one answer | Jev returns a probability for **every** option, not one label: a foundation model for classification (owner, 2026-10-27). The labels are defined per namespace and table (owner, 2026-10-27), held with the prompts below. One Choice question over that pair's labels gives the whole distribution; a record keeps the labels at or above a threshold (default 0.5), at most the top k (default 5), both set per namespace and table next to the prompts, with their probabilities stored (owner, 2026-10-27, settling Q193) |
| Prompts in KV | classification labels and questions and the summary prompt live in the instance record's [key-value store](../../../docs/03-records-kv.md#key-value-store), keyed by namespace and table (owner, 2026-10-27: "use KV to store classification/summary prompts") |
| Read cache | the agent reads them through an in-process cache and never asks the daemon on a hit; a write invalidates the cached entry, never updates it in place (owner: "we can implement READ cache - write just invalidates"); prompts are edited only through the agent's own methods, which invalidate as they write; a KV edit made elsewhere (web, CLI) is not seen until the agent restarts (owner, 2026-10-27, settling Q198) |
| Summary | optional; `#ai` writes a short summary at upsert, stored and searched as its own chunk |
| Access is a name | the ACL grants a whole instance (`#rag`, or `#rag/<name>` for a separate store), as for every bundled agent ([catalogue rules](../../R1.2/services.md#rules-they-all-obey)); namespace and table are filters inside it, not grants |
| The store is anywhere | Qdrant is **just a podman container** the operator runs and maps as they like, on this host or any other; the bus neither installs nor limits it (owner, 2026-10-28: "Qdrant - just a podman; we map it where we want to", "it can be anywhere; not our task to limit it"). `#qdrant` is configured with its URL and holds its API key in the runner env; the docs give one `podman run` example |
| `#qdrant` is a block of its own | a transparent proxy to the Qdrant HTTP API holding its key in the runner env, like `#voyage`; any app may use it, `#rag` is the first (manager proposal under the owner's "agents as building blocks") |
| No daemon change | the RAG agent is an ordinary runner agent calling other agents |

## Versioning

The `#rag` app keeps its **own SemVer**, starting at `0.1.0`, separate from the
bus version (owner, 2026-10-27: "auto bump RAGs subproject own semver"). The
implementer bumps PATCH automatically on every landed `#rag` step, without
asking, with one line in the RAG changelog beside its code. The blocks it uses
(`#voyage`, `#ai`, `#qdrant`) are bus agents and stay on the shared version
([versioning](../../../CLAUDE.md#versioning)).

## Methods

Basic CRUD plus extra functionality (owner, 2026-10-27: "we should provide basic
CRUD + extra functionality (like search completion, ..)"). Names only; arguments
wait for the owner.

| Group | Methods |
|---|---|
| CRUD | `upsert` (create or replace), `get`, `list` (filtered, paged), `delete` (by id, or filtered by namespace and table) |
| Search | `search`: hybrid, filtered, optional rerank; each hit carries namespace, table, id, key1, key2, score, labels and the matched chunk's text, and the full data comes from `get` (owner, 2026-10-27, settling Q196) |
| Prompts | read, set and delete the labels, questions and summary prompt of a namespace and table; each write invalidates the cache |
| Extra (proposed) | `complete`: search completion, suggestions for a typed prefix; `similar`: records like a given id; `count`: matches and per-label counts for a filter; `classify`: Jev labels for a text without storing it |

## Open choices

[Q194](../QUESTIONS.md#rag-service), settled by the trial. Decisions: [RD127](../DECISIONS.md#recorded-decisions).
