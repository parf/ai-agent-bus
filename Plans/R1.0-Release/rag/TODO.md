# TODO: RAG service

## Objective

Build the [RAG service](README.md#rag-service) from agents as building blocks:
blocks first, then the app on top. Owner, 2026-10-27: "move it to R1 and give it
to xAB; start with block building; use MD as plan; go over each step".

## Process

**xAB (`#codex/xab`) controls the plan's execution**; **xAB2 (`#codex/xab2`) builds R.3 and R.4 in parallel**, under the same process, and xAB folds its results into this file (owner, 2026-10-27: "it should
control plan execution, mark done items, and do review after meaningful progress").

1. One step at a time. Before code, xAB writes the step's plan here, under the step,
   in a few lines: shape, files, tests, mutants. `#claude/ab2-review` reviews it before code starts (from R.2; the manager approved R.1).
2. Each step lands under [CLAUDE.md](../../../CLAUDE.md#verification):
   - mutants for ⚠️ parts (parsers, network input, credentials);
   - the normal gate under the `smoke-gate` lock;
   - a bump: the shared version for a block, the [RAG's own version](README.md#versioning) for `#rag`, automatic, with a changelog line;
   - deploy only that component, then `check-deploy`.
3. xAB keeps this file current: it marks a step ✅ when it lands, moves its detail to
   [DONE](DONE.md) with the version, commit and gate numbers, and starts the next.
4. After meaningful progress (a finished block, or each `#rag` step), xAB asks the
   read-only reviewer `#claude/ab2-review` to review it, fixes or answers every
   finding, and records the result in DONE.
5. xAB reports each landed step to the manager, who reports to the owner.
6. No real provider call (Voyage, OpenRouter) and no live data without the
   owner's approval. Tests use fakes.
7. A choice for the owner goes to [questions](../QUESTIONS.md#rag-service), never picked silently.

## Steps

Blocks first (R.1–R.4), then the app (R.5–R.9).

- ⏳ R.1 `#voyage`: a transparent Voyage proxy; this is [B.19](../TODO.md#bundled-agents)
  - Acceptance and mutants: B.19.
  - **Plan for manager review (2026-10-27; no code yet):**
    - **Shape:** a separate Python runner package, pooled and sandboxed like `#ai`, with the normal per-name ACL (initially `@owner`). `embed` accepts text, optional model and input type; the default model is `voyage-3.5`. Generic pass-through preserves the request path and body bytes, adds the key, and accepts only paths within `/v1/` on the fixed, certificate-verified `api.voyageai.com` origin. Configuration holds only the default model. Keep `#ai` behavior unchanged while sharing only its small HTTP, masking and deadline helpers, without a new dependency or proxy framework.
    - **Credentials and bounds:** `VOYAGE_API_KEY= # required` in `env.dist`; the runner supplies the existing host `voyage/env` layer. Never read, print, copy or set the installed key file during implementation. No bus Secret or Configuration credential, redirects, retries or streaming; retain `#ai` request/response caps and the earlier message deadline. Provider errors and 429 are normal masked answers. Logs contain only model, byte counts and status.
    - **Files:** add `src/agents/voyage/` (entry point, manifest, env declaration, package script, README and fake-provider tests); extract the common helper into `src/agents/shared/provider_http.py` and include it in both proxy packages. Update `src/agents/package.sh`, setup's package inventory and delivery/runtime fixtures, `src/acceptance/agent-runtime.py`, the smoke shard/map and shared metadata checks. Add `docs/agents/voyage.md` and the agents index row. Include every new runtime helper in exported bundles; verify execution outside the checkout.
    - **Tests and mutants:** loopback fake Voyage only, with external connections blocked. Pin embedding output/defaults, exact pass-through bytes/path/Authorization, errors and 429, missing key, escaped key echoes, path traversal, size/time bounds, one attempt and metadata asks doing no provider work. Mutants: omit Authorization, leak the key in an error, rewrite the outgoing body, escape `/v1/` or remove a bound; each runs only its owning check. Run the existing AI suite to prove shared-code compatibility.
    - **Landing:** after review, take the smoke lock for edits/mutants and the normal gate, bump and changelog, then deploy only the proxy packages changed by shared code. Enable only `voyage` in the existing runner inventory and reload, preserving other entries and grants; verify help, configured state and `check-deploy`. Do not send an embedding or any real provider request: the manager and owner make the first live call. Request a read-only review from `#claude/ab2-review`, fix or answer every finding, and record the version, commit, gate numbers and review result in DONE. Mark R.1 done in TODO and report to the manager before submitting the R.2 plan. All commits stay local; never push.

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
