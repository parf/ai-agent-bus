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

- ✅ R.1 `#voyage`: built and deployed in 0.9.123 — [result and review](DONE.md#r1-voyage-proxy)

- ✅ R.2 `#ai` typed decisions: built and deployed in 0.9.124 — [result and review](DONE.md#r2-ai-typed-decisions)

- ✅ R.3 External Qdrant store: RD131 removal and rootless local deployment completed in 0.9.127 — [result and superseded history](DONE.md#r3-external-qdrant-store)
  - History: .126 setup-managed source was reviewed/gated, but its four-step host approval was placed on HOLD before any root apply; RD131 voided it and .127 removed that component. [The closed history](DONE.md#r3-external-qdrant-store) retains the approval messages and evidence. No old rootful commissioning step remains required.

- ✅ R.4 `#qdrant`: built and deployed in 0.9.130 — [result and review](DONE.md#r4-qdrant-proxy)

- ☐ R.5 Keyword trial, settling [Q194](../QUESTIONS.md#rag-service)
  - Compares Qdrant's built-in BM25 with the agent's own sparse vectors with stemming, on the owner's real Russian and English texts
  - Needs: a sample from the owner; the results go into the question, and the owner picks
  - Output: a short report under `done/`; no product code

- ☐ R.6 `#rag` core: CRUD and prompts
  - Builds: `upsert`, `get`, `list`, `delete`, and the prompt methods; Jev labels on upsert through `#ai`; embeddings through `#voyage` when absent; the read cache; idempotent point ids; chunking by mime-type
  - Acceptance: an upsert twice leaves one record; a delete by namespace and table removes only that; a Jev or Voyage failure fails the upsert and stores nothing; a prompt write invalidates the cache, and a cache hit makes no daemon call
  - Mutant: a duplicate on re-upsert, a delete beyond its filter, a stale prompt after a write, or a store despite a failed classification; a check must fail
  - **R.6 plan for read-only review (2026-10-28; no code yet):**
    - **Shape/version:** a kept, sandbox-on, standard-library Python runner agent in `src/agents/rag/`, initially owner-only, with one worker to serialize replacement and cache invalidation. Use the existing Python bus client and ask dispatcher; every downstream call runs as this instance's own identity. No provider HTTP, provider credential, new daemon API or shared helper behavior. Canonical app version is `agent.json`'s supported `version`, initially **0.1.0**, with `CHANGELOG.md` beside it; later landed app steps bump PATCH automatically. Help/status expose the app version. R.4 keeps shared bus .128; shared package/setup integration, smoke edits and gates are coordinated with xAB2 after its release.
    - **Files/package:** small entry point/core, downstream-bus adapter, prompt-cache and MIME/chunking modules; `agent.json`, empty credential declaration, package script, generic configuration example, local changelog and fake-block tests. Ship the existing client/metadata support and required licenses. Add current `docs/agents/rag.md` and index entry; coordinate additive package/setup delivery inventories, acceptance/runtime fixtures and an L-tagged smoke shard/map without dropping Qdrant's entries. Verify the exported app outside the checkout.
    - **Configuration/authority:** [RD135](../DECISIONS.md#recorded-decisions) settles collection provisioning and JSON-only calls. The owner creates the collection and its keyword payload indexes once; the app has only its own `#qdrant(write)`, `#voyage` and `#ai` grants and never creates, escalates or edits grants. Verify model metadata, unnamed Cosine vector size and every required index via read-tier collection info before use. Namespace/table are filters: any caller later admitted to this instance can read/delete every namespace. Keep it owner-only until a broader policy is explicitly granted. No live collection/grant changes in R.6; exact owner provisioning commands go in the app docs. Manifest types/bounds/examples are reviewed before any public parser code, with no aliases or new public record fields.
    - **CRUD/ingest:** implement upsert/get/list/delete and prompt read/set/delete against the existing record definition. Deterministic typed-tuple UUIDs identify records/chunks; one canonical record payload permits get/list without pagination through half a record. Initial bounded MIME handlers cover UTF-8 plain text, Markdown, HTML and JSON; unsupported types refuse before downstream work. Prepare all chunks, Jev Choice labels and supplied/computed vectors before deleting/replacing anything; a Jev/Voyage failure leaves the previous record untouched. Validate finite probabilities, configured labels, threshold/top-k and vector sizes. Upsert new revision-specific chunks with `wait=true`, then publish a stable canonical record point with `wait=true`, then delete stale chunks by exact identity AND `_rag_rev != new_revision` FILTER, with `wait=true`. Every chunk carries identity/revision; never clean by a list fetched from the head. Record deletion likewise filters all matching revisions, so repeat operations remove orphans left by failed cleanup. Never delete old points before the new upsert is confirmed. Failures leave the old record or old+new points, not an empty record; repeats converge. Preserve supplied vectors. Validate every backend reply; report uncertain/partial backend writes explicitly, never success or a cross-call transaction guarantee, and never blindly retry. Bound body/chunks/list pages and every downstream call by the original message deadline.
    - **Prompts/keyword seam:** own KV holds namespace/table labels, questions, thresholds/top-k and the future summary prompt, behind a bounded read cache. A hit makes zero daemon calls; set/delete invalidates (does not seed) the entry on both success and uncertain timeout/disconnect; definite failures never install a new value. External KV edits are seen after restart, as settled. Missing classification setup refuses ingestion. Keyword encoding has one injectable interface; R.6 neither chooses BM25/stemming nor claims hybrid search while R.5 waits for owner samples. Summary/search remain R.7–R.9 work.
    - **Tests/mutants:** fake `#qdrant`/`#voyage`/`#ai` only, plus a disposable bus to prove trusted sender identity and correlation; prohibit external connections and never use live provider keys. Check idempotent replacement including shrinking chunks, tuple-collision resistance, scoped delete and pagination, MIME parsing/bounds, vectors/probabilities, label threshold/top-k, provider failure before mutation, uncertain backend writes, zero-call cache hits, invalidation/restart, asks doing no work and app-version packaging. Owning assertion-red mutants break IDs, widen a deletion filter, retain stale cache, write after a failed classification, bypass vector/parser bounds or spoof a downstream caller. Real block/provider calls stay out of every test/gate.
    - **Reviewed storage refinement:** revision is SHA-256 of canonical typed input (source, MIME, optional keys, selected label IDs, model, chunk-format version and final float32 vector bytes); classification probabilities stay only in the head. Revision-specific chunk UUIDs include identity/revision/index; the stable head UUID contains only identity and a head marker. Publish new chunks first, then head, then filter-delete identity-matching chunks with another revision. Repeat upsert/delete repairs abandoned revisions; R.7 must validate hits against the head revision before returning them. The same required-index list includes identity, `_rag_rev`, kind and label IDs in provisioning/startup verification. No vector copy in head payload: get fetches at most eight named chunk vectors; list omits source and embeddings.
    - **Cost/size refinement:** one Jev call on extracted text per record (<=32 KiB), never per chunk; <=8 deterministic 4 KiB UTF-8 chunks, <=8192 float32 scalars, and <=2 Voyage batches of <=4 chunks. Every encoded provider request stays below 1 MiB. BEFORE provider work, compute worst-case ASCII-escaped source/metadata + configured top-k label budget + vector scalar count times 25 bytes (24-character shortest JSON float repr plus delimiter) + fixed overhead; refuse above the 256 KiB full-record cap with actual estimate/limit, even if each individual field fits. Round-trip vectors through float32, then use the standard shortest JSON float representation; reject float32 overflow and nonfinite/boolean values. Head payload has source/metadata/labels and ordered chunk IDs only. Summaries omit source/vectors, each <=32 KiB; list <=16 summaries (512 KiB plus wrapper), with vectors excluded from backend scroll. Verify actual sizes at every boundary and never write unreadable records.
    - **Parser/MIME/paging refinement:** namespace required for get/list/delete; table-only/null/empty/blank is refused. All identity/filter strings must already be NFC; no silent normalization. Empty key1/key2 is valid, omitted upsert keys clear old values. Delete by id forbids key filters; any delete without id requires explicit `all:true`, a control flag rather than record data; it never deletes prompts KV. Reject duplicate keys, NaN/Infinity, numeric booleans, BOM/NUL/invalid UTF-8. HTML drops script/style without fetching, with depth/size bounds; Markdown uses source text and JSON is canonicalized before chunking. Freeze UUID namespace, tuple and chunk-format version; migrations precede changes. UUID scroll offsets are opaque and shape-checked; a bounded 128-entry LRU cursor map binds filters/limit and expires after 10 minutes/restart/eviction with explicit errors. Unknown writes say unknown, safe to repeat, no auto-retry.
    - **Prompt/deadline refinement:** prompt edits never reclassify existing records; set/delete invalidate even on uncertain KV errors. KV keys are `rag.prompts.` plus SHA-256 of the canonical namespace/table tuple (76 ASCII bytes, under core's 256-byte UTF-8 key bound); cache is bounded and restart-scoped. Validate Jev labels against the configured set, rejecting unknown/missing/nonfinite probabilities; selected labels sort by probability then ID. Default decision model remains `typesafe/jev-1.14`. Manifest deadline is 120s; caller's earlier deadline wins; provider calls take at most min(30s,remaining), Qdrant/KV calls min(10s,remaining), with no budget reset between stages.
    - **Additional owning mutants:** cleanup by head ID list after an injected stale-delete failure; same-ID chunk overwrite across revisions; paid work before worst-case budget refusal; embedded vector duplication/list projection mismatch;  delete-before-upsert with a failing new upsert; dropped/empty namespace clauses with exact filter assertions; missing payload-index/model checks; unreadable record/page boundaries; per-chunk classification or oversized batches; cached data kept after an uncertain KV write; duplicate/nonfinite/boolean acceptance; HTML script retention; arbitrary/mismatched scroll cursor acceptance. Real identity/correlation fixtures remain on a disposable bus.
    - **Landing/review:** after plan and manifest review, run owning mutants and normal under the smoke lock, record app 0.1.0/changelog and the exact shared integration bump only if setup behavior requires it. Deploy only the app/integration pieces changed; no real data ingestion, provider call, collection creation or owner session restart. Verify package help/configuration state and check-deploy, request finished-step read-only review, resolve findings, mark R.6 done and move version/commit/gate/review evidence to DONE. R.5 and the R.4 bus integration remain explicitly pending until their prerequisites land. No push.

- ☐ R.7 `#rag` search
  - Builds: hybrid `search` with filters and optional rerank, the keyword half chosen by R.5
  - Acceptance: on a fixture set, filters hold exactly; a keyword-only match and a meaning-only match both appear; rerank reorders; hits carry the agreed fields

- ☐ R.8 `#rag` extras: `complete`, `similar`, `count`, `classify`
  - Owner to confirm the list before this step

- ☐ R.9 Summary, docs and end-to-end
  - Builds: the optional summary chunk through `#ai`; a user guide under `docs/agents/`; one end-to-end run on the owner's data, approved by the owner
