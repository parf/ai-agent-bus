# RAG service

📌 **TL;DR:** A simple RAG service on the bus. It stores records, classifies
them, embeds them and finds them again by meaning, by keywords and by filters.
The parts are a Qdrant container for hybrid search, Jev (via OpenRouter) for
classification on upsert, `#voyage` for embeddings, and `#ai` for an optional
summary. The owner asked for it on 2026-10-27; no release is assigned yet.

## Parts

| Part | Choice | Why |
|---|---|---|
| Hybrid search store | **Qdrant**, a podman container | owner, 2026-10-27; one light binary, dense and sparse vectors fused in one query, indexed payload filters |
| Classification | **Jev** by TypeSafe AI, through OpenRouter, called on upsert | owner, 2026-10-27; a decision model, not a text generator: fast, cheap, answers typed questions with probabilities |
| Embeddings | `#voyage` ([B.19](../R1.0-Release/TODO.md#bundled-agents)) | the owner's key, already proxied |
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
record shape ([working rules](../../CLAUDE.md#working-rules)).

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
| Hybrid search | dense vectors from Voyage plus sparse keyword vectors, fused in one Qdrant query; filters on tableName, key1, key2 and classification ids |
| Rerank | optional, through `#voyage`'s pass-through to Voyage rerank |
| Classification on upsert | Jev assigns the classification ids before the record is stored; a Jev failure fails the upsert, explicitly |
| Many labels from one answer | Jev returns a probability for **every** option, not one label: a foundation model for classification (owner, 2026-10-27). The labels are defined per namespace and table (owner, 2026-10-27), held with the prompts below. One Choice question over that pair's labels gives the whole distribution; the ids are the labels at or above a threshold, and the probabilities are stored with them |
| Prompts in KV | classification labels and questions and the summary prompt live in the instance record's [key-value store](../../docs/03-records-kv.md#key-value-store), keyed by namespace and table (owner, 2026-10-27: "use KV to store classification/summary prompts") |
| Read cache | the agent reads them through an in-process cache and never asks the daemon on a hit; a write invalidates the cached entry, never updates it in place (owner: "we can implement READ cache - write just invalidates"); how a write made outside the agent reaches the cache is [Q198](QUESTIONS.md#open-questions) |
| Summary | optional; `#ai` writes a short summary at upsert, stored and searched as its own chunk |
| Access is a name | the ACL grants a whole instance (`#rag`, or `#rag/<name>` for a separate store), as for every bundled agent ([catalogue rules](../R1.2/services.md#rules-they-all-obey)); namespace and table are filters inside it, not grants |
| The store stays local | Qdrant listens on localhost only, data on a podman volume, under a systemd unit; backups are Qdrant snapshots |
| No daemon change | the RAG agent is an ordinary runner agent calling other agents |

## Methods

Basic CRUD plus extra functionality (owner, 2026-10-27: "we should provide basic
CRUD + extra functionality (like search completion, ..)"). Names only; arguments
wait for the owner.

| Group | Methods |
|---|---|
| CRUD | `upsert` (create or replace), `get`, `list` (filtered, paged), `delete` (by id, or filtered by namespace and table) |
| Search | `search`: hybrid, filtered, optional rerank |
| Extra (proposed) | `complete`: search completion, suggestions for a typed prefix; `similar`: records like a given id; `count`: matches and per-label counts for a filter; `classify`: Jev labels for a text without storing it |

## Open choices

[Q193–Q197](QUESTIONS.md#open-questions).
