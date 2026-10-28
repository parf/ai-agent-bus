# TODO: RAG service

## Objective

Build the [RAG service](README.md#rag-service) from agents as building blocks:
blocks first, then the app on top. Owner, 2026-10-27: "move it to R1 and give it
to xAB; start with block building; use MD as plan; go over each step".

## Process

1. **xAB** (`#codex/xab`) takes one step at a time. Before code, it writes the step's
   plan here, under the step, in a few lines: shape, files, tests, mutants.
   The manager reviews it before code starts.
2. Each step lands under [CLAUDE.md](../../../CLAUDE.md#verification):
   - mutants for ⚠️ parts (parsers, network input, credentials);
   - the normal gate under the `smoke-gate` lock;
   - a bump and a changelog line;
   - deploy only that component, then `check-deploy`;
   - a report with numbers.
3. The manager (AB2, `#claude/ab-dvp-b` until the handoff) reviews each step,
   moves it to [DONE](DONE.md), and reports to the owner.
4. No real provider call (Voyage, OpenRouter) and no live data without the
   owner's approval. Tests use fakes.
5. A choice for the owner goes to [questions](../QUESTIONS.md#rag-service), never picked silently.

## Steps

Blocks first (R.1–R.4), then the app (R.5–R.9).

- ⏳ R.1 `#voyage`: a transparent Voyage proxy; this is [B.19](../TODO.md#bundled-agents)
  - Acceptance and mutants: B.19.

- ☐ R.2 `#ai` learns OpenRouter's Decisions API, for Jev
  - Builds: a `decide` method passing a decisions request to OpenRouter unchanged, with the key added; same bounds, masking and logging rules as `ask`
  - Acceptance: against a fake OpenRouter, a Choice question returns every option's probability as given; an upstream refusal is the answer; the key is in no reply or log
  - Mutant: drop the key, rewrite the body, or leak the key in an error; a check must fail

- ☐ R.3 Qdrant container
  - Builds: Qdrant in podman under a systemd unit (quadlet); listens on localhost only, with an API key; data on a named volume; a pinned image version; a snapshot backup command
  - Acceptance: the unit starts after a reboot; the port is closed to other hosts; a request without the key is refused; a snapshot restores into a fresh container
  - Install: through setup like other host pieces, never by hand; root steps need the owner's OK

- ☐ R.4 `#qdrant`: a transparent proxy to the Qdrant HTTP API
  - Builds: pass-through of any Qdrant API path with the key added from the runner env, as `#voyage` does; no logic of its own
  - Acceptance: a collection is created, a point upserted and found through the bus; a path outside the API is refused; the key is in no reply or log
  - Mutant: drop the key, or allow a path outside the API; a check must fail

- ☐ R.5 Keyword trial, settling [Q194](../QUESTIONS.md#rag-service)
  - Compares Qdrant's built-in BM25 with the agent's own sparse vectors with stemming, on the owner's real Russian and English texts
  - Needs: a sample from the owner; the results go into the question, and the owner picks
  - Output: a short report under `done/`; no product code

- ☐ R.6 `#rag` core: CRUD and prompts
  - Builds: `upsert`, `get`, `list`, `delete`, and the prompt methods; Jev labels on upsert through `#ai`; embeddings through `#voyage` when absent; the read cache; idempotent point ids; chunking by mime-type
  - Acceptance: an upsert twice leaves one record; a delete by namespace and table removes only that; a Jev or Voyage failure fails the upsert and stores nothing; a prompt write invalidates the cache, and a cache hit makes no daemon call
  - Mutant: a duplicate on re-upsert, a delete beyond its filter, a stale prompt after a write, or a store despite a failed classification; a check must fail

- ☐ R.7 `#rag` search
  - Builds: hybrid `search` with filters and optional rerank, the keyword half chosen by R.5
  - Acceptance: on a fixture set, filters hold exactly; a keyword-only match and a meaning-only match both appear; rerank reorders; hits carry the agreed fields

- ☐ R.8 `#rag` extras: `complete`, `similar`, `count`, `classify`
  - Owner to confirm the list before this step

- ☐ R.9 Summary, docs and end-to-end
  - Builds: the optional summary chunk through `#ai`; a user guide under `docs/agents/`; one end-to-end run on the owner's data, approved by the owner
