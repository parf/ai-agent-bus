# T-43 — RAG app 0.3 for mdhouse

Plan for approval only, 2026-11-05. Owner: parf; lead: #ab-lead1. No implementation, provider call, container, deployment or source changes made for this plan.

## Contract and baseline

Read-only consumer contract: `/home/parf/src/mdhouse/Plans/R3/RAG-CONTRACT.md`, including the AB answers to RAG-Q1–Q12. Provider baseline read on local main `3874bb63`: [current guide](../../docs/agents/rag.md), [RAG plan](../../Plans/R1.0-Release/rag/README.md), its TODO/DONE, and `src/agents/rag/` (app 0.2.0).

The existing provider has deterministic namespace/table/id identities, live-head publication and tombstone cleanup, bounded list cursors, BM25 sparse search, classification and document embeddings, and hashed namespace/table prompt KV. It does not have dense/hybrid retrieval, namespace inventory/drop, persistent namespace model bindings or complete citation metadata. `keyword_index`, its missing-vector count/backfill and its completeness cache are collection-wide today. CRUD and prompts already require namespace; `keyword_index` is the parser exception. `get` does not call `scope()`, but its UUID includes namespace/table/id and it verifies the returned head's identity: this is not a wrong-namespace ID leak. Today's admitted callers can explicitly name any namespace, under the documented whole-instance grant.

Code evidence: `rules.py:118–158` parser, `:200–227` identity/filter; `core.py:39–54` dispatch, `:56–120` upsert, `:122–152` get, `:181–280` BM25/index, `:281–310` delete; `prompts.py:21–38` cache; `blocks.py:12–70` shared deadlines/transport; `rag.py:40–61` method dispatch and error surface. These are a dated map, not canonical future line numbers.

## Scope and settled choices

Owner direction is retrieval evidence, not generation: keyword, semantic and hybrid search; mandatory data scope; namespace administration; citations; free capabilities; guide and independent RAG changelog/version. Sources, ingestion and refresh remain FileScan/T-38 work.

Lead rulings recorded during this plan:

- Every data method, prompts and `keyword_index` must use one namespace-bound access layer. No omitted/blank namespace, wildcard namespace or cross-namespace default.
- Namespace discovery is an explicit exception: manager-only (the record's Owner or Maintainers), names and counts only. It must enumerate namespaces, so it cannot pretend to be scoped to one project.
- `capabilities` is the other namespace-free exception: static app/configuration metadata only; version, methods, modes, limits and configured embedding model. No backend verification, record lookup, classification or embedding call. Not a readiness/health assertion. An unconfigured instance can advertise its methods/limits and say the model is unconfigured without guessing it.
- `drop_namespace` is namespace-bound AND manager-only, plus confirmation.
- One configured model and dimension per instance. Each namespace has an immutable persisted binding; drift refuses before a paid call. No multi-model collection. The consumer's “new namespace on model change” is a gap: it also needs matching owner-provisioned collection/instance changes.
- Namespace-free legacy data is refused. No default namespace, automatic migration, retired decoder or compatibility mode.

Names `namespaces`, `drop_namespace` and search modes `keyword`, `semantic`, `hybrid` are proposed method vocabulary to review in `agent.json` before parser code. No alternate spellings. Keep keyword as the explicit documented default when mode is omitted, preserving the existing free search behavior; never silently substitute a different mode after a failure. The answer always names the effective mode.

## One data boundary

Introduce one namespace-bound repository/access object, constructed after canonical request validation and before dispatch to any data method. It holds the validated namespace immutably. Core method handlers receive this object, not a raw backend or a caller-supplied filter. Only this layer builds point identities, filters, payloads and namespace KV keys and reads their responses.

- Every data query/count/scroll/delete/backfill, including both hybrid prefetches, carries its namespace and the relevant head/chunk kind. Filters are composed by the layer; caller fields cannot replace or OR away namespace.
- Exact-ID and batch head/chunk reads use namespace-constrained backend reads as well as deterministic IDs. The layer verifies namespace, full identity, point ID, revision and head/chunk membership before returning anything. Writes stamp namespace and derive IDs inside the layer. A backend returning another namespace is refused, not trusted because an ID looks right.
- `get` and id `delete` absent in the selected namespace return named `not_found`, identical whether the same local id exists elsewhere or nowhere. A matching local id in the requested namespace remains a separate valid record. Do not query other namespaces to distinguish these cases. Do the existence check before tombstoning an id delete.
- Prompt get/set/delete use only the bound namespace's keys. Prompt writes, namespace binding and associated cleanup metadata are persisted before acknowledging; cache invalidation remains mandatory on success AND uncertain writes. No metadata/cache is keyed just by table or record id.
- `keyword_index` requires namespace. Its data scan/backfill and missing-vector completeness state become namespace-bound. A collection-level vector definition is shared infrastructure, provisioned/verified at collection level; no hidden all-namespace data backfill. If the sparse vector needs an admin action, return the existing owner instruction rather than widening grants.
- Collection configuration/health reads do not return project data and remain infrastructure verification. Namespace enumeration has a visibly separate manager-only metadata path; no data handler may obtain an unbound “all records” repository.

Boundary proof: seed A and B with distinct secrets plus overlapping table/id/key values. Run every data method in A, including prompts, keyword-index, semantic and both hybrid branches; assert exact outputs and recorded backend predicates. An isolation mutant removes the layer's namespace predicate while IDs/revision checks remain intact: a B-only searchable/listable record must appear and fail the named cross-namespace assertion. Also feed an intentionally foreign backend response to prove the response-side check. A separate mutant removes response validation. This avoids a green mutant that another check accidentally masks.

This is a data-selection boundary inside one trusted application identity, not a new per-project bus ACL. mdhouse alone chooses the project namespace and enforces its user grants; a caller admitted to the same RAG instance is not restricted to a configured single namespace. A separate instance/grant is required for an untrusted caller.

## Namespace state, model consistency and existing data

Use the instance's existing own-KV facility for a single canonical namespace metadata catalogue, with one worker serializing mutations as today. It records immutable namespace model/dimension bindings, the table identities needed to remove that namespace's hashed prompt keys, and destructive-operation state. Do not add a daemon table or parallel model registry. Bound the catalogue and its encoded KV value under the existing owning KV/body limits; fail before changes if it cannot grow. Final exact field declarations belong in the reviewed manifest/storage design, not a second wire specification here.

On first namespace use, verify that it is genuinely new (no stored heads/chunks) before creating its binding from the verified configured model/dimension. Persist and confirm the binding before classification or embeddings. An uncertain binding write stops before payment and is inspected/re-read on a later explicit call; it does not silently select a model. Existing bindings survive restart. Every upsert and semantic/hybrid query compares the binding, Configuration and verified collection settings before invoking Voyage. Query embedding uses `input_type=query`; document embedding stays `document`, with the SAME model and output dimension. No caller-selected query model override is needed. A differing configured/query model is refused even if dimensions happen to match.

The namespace catalogue is metadata, not a promise of an atomic KV+Qdrant transaction. Empty namespaces can remain after a failed first upsert and are listed with count zero. Missing binding on a nonempty legacy namespace is a named legacy-data refusal, never automatic inference from its first record. Namespace-free rows, malformed bindings and unsupported old payloads are refused by the acceptance/provisioning check; no hidden “default” bucket. Preserve old storage read-only for owner backup/re-feed into a fresh canonical namespace or collection. No in-place migration/conversion is part of this patch. Existing correctly labelled records that lack the new binding/citation contract need an explicit re-feed too; do not invent hashes, paths, timestamps or model provenance for them.

A model change is owner provisioning work: new matching collection/instance, model/dimension verification, explicit re-feed and mdhouse route switch. Merely choosing a new namespace in the old instance is insufficient. Preserve UUID tuple/chunk identity contracts where unchanged; do not conflate the new source hash with `_rag_rev`, which also incorporates model, labels and vectors.

## Retrieval and citations

- Keyword: retain existing BM25 tokenization and sparse vector semantics; no Voyage/AI call.
- Semantic: exactly one bounded query embedding through `#voyage`, then a filtered dense Qdrant query using the collection's existing dense vector. Reuse float32/dimension/nonfinite checks; no classifier or generation call.
- Hybrid: one query embedding; Qdrant dense and existing BM25 sparse prefetches, each namespace-filtered, fused by Qdrant RRF. Do not implement an unrelated client-side score sum or substitute Qdrant's newer text inference for today's sparse encoder.
- All modes retain current live-head/revision checks, bounded candidate windows, de-duplication to the best live chunk per record, deterministic ties and explicit incomplete-window errors. A missing keyword vector refuses keyword/hybrid; it does not block semantic search. Sparse completeness caches are keyed by namespace and invalidated by writes/backfill/drop.
- Preserve existing result/page/body and end-to-end budget limits unless manifest review finds citation overhead cannot fit; reduce candidate projection to stay within the existing response cap rather than silently truncating evidence. No paid retry on timeout/refusal/invalid embedding. Unsupported mode is a named refusal.

Add source/path metadata to the canonical upsert contract, carried by FileScan/mdhouse as data, never opened/fetched by RAG. Require real provenance for new records; do not fabricate a source from a record id. Compute the content hash over the exact submitted source text bytes before extraction; if a producer supplies a hash, verify it rather than trusting it. This is a content fingerprint, not a revision/embedding fingerprint. Attach source, hash, chunk index and UTC indexed time to the published live revision. A search hit copies provenance from that revision and reports an excerpt from its live matched chunk, never a draft or replaced head.

Each hit contains the owner-requested namespace, record id (and table, preserving the current identity), chunk index, source/path, content hash, excerpt, scores and indexed_at. Score semantics are labelled by effective mode: BM25, dense similarity or RRF fusion. Qdrant's fused hit score is not a calibrated probability or the pair of component scores; do not invent dense/BM25 component numbers that the backend did not return. The manifest states which score is present. Exact cited bytes are not retained after replacement: mdhouse must retain citation/hash/excerpt and can detect changed/removed content via get; historical revision retrieval is out of scope.

Primary API checks (read-only, 2026-11-05): [Qdrant hybrid query documentation](https://qdrant.tech/documentation/search/hybrid-queries/) describes prefetch fusion; [point counts](https://api.qdrant.tech/api-reference/points/count-points) support filtered exact counts. Use the existing pinned backend version from the RAG fixture for request-contract fixtures. No backend/provider was contacted by this plan.

## Namespace inventory and destructive drop

`namespaces` checks the delivered Owner/Maintainer role before any catalogue/backend read. It lists known canonical namespace names and exact counts of live record heads, not chunks, drafts or tombstones. Empty known namespaces count zero. Resolve counts through each namespace-bound repository; return only names/counts plus bounded paging metadata. No content, source paths, labels, prompt text or model secrets. On a failed count, return a named error/incomplete outcome rather than an invented zero. Reads are free of paid providers; paging is bounded and not a cross-page snapshot.

`drop_namespace` requires the same manager role and an explicit confirmation equal to the requested namespace (proposed confirm token; no wildcard or truthy coercion). Validate role, namespace and confirmation before any write. Missing, wrong or cross-namespace confirmation refuses with no storage mutation or provider call. This is distinct from the existing table/filtered `delete all:true`.

Persist a dropping intent in that namespace's canonical metadata before destructive work. Block ordinary reads/writes/search/indexing into a dropping namespace; one worker ensures no concurrent upsert passes it. Remove every head/chunk/revision in that namespace through filtered acknowledged deletion, then that namespace's prompt KV and caches/cursors, and finally its model/catalogue metadata. Retain the intent on uncertain or partial failure, including across restart. The caller must explicitly confirm a continuation; nothing retries a destructive call automatically. A completed repeat can report the namespace already absent. No other namespace's prompt, binding or points change. Never delete the shared Qdrant collection or sparse-vector definition.

Mutants: omit confirmation -> the named missing-confirmation assertion goes red on a changed sentinel; bypass manager guard -> an admitted non-manager's request mutates a sentinel and fails; omit namespace filter on drop -> namespace B's sentinel fails. Inject failures at the Qdrant/KV boundaries and restart to prove incomplete state remains blocked and inspectable. Exact counts after a drop are proven from fixture heads, not a success log.

## Authorization and deployment boundary

Owner provisions one `#rag/mdhouse` and its collection/block grants, with Allow limited to mdhouse's bus identity. mdhouse sets namespace=<project> on every data call. Namespace discovery/drop require delivered Owner/Maintainer; a plain admitted caller does not acquire those roles by naming itself in JSON. Daemon Owner behavior follows actual delivered manager roles, not an untrusted body field. Method help must match enforcement.

The current RAG does not enforce a general read-vs-write role split: only keyword_index checks manager roles. This scope adds the two explicitly approved manager checks; it must not claim the broader RAG-R8/Q10 role split is already satisfied. Keep that contract gap visible for owner follow-up, rather than silently adding an unapproved writer role or granting mdhouse maintenance access. No record creation, Allow edit, paid call or live deploy is authorized by this local plan.

## Contract-versus-scope gaps

The consumer's unanswered checkboxes are not automatically additional T-43 features. Its AB answers narrow several older requests; mdhouse owns that document and must reconcile it.

| Consumer requirement | 0.3 coverage / remaining gap |
|---|---|
| RAG-R1 / Q1 / A3 | Required explicit data namespace and exact no-cross-scope behavior; one trusted mdhouse identity selects projects. Two-instance/untrusted-caller test and project-grant enforcement belong to owner provisioning/mdhouse, not per-namespace RAG ACLs |
| R2–R5 / Q3–Q5 / A4 | No RAG source registry, connectors, ingestion jobs, progress, cancellation or refresh API. FileScan/T-38 is the separate producer; its integration is not yet proved by this retrieval patch |
| R5 / Q7 / A5 | Content hash and indexed time included. No consistent index generations, last-successful-refresh state, retained old revisions or mixed-model collection. A new model requires new matching collection/instance as well as namespace and re-feed |
| R6 / Q8 / A9 | Keyword/semantic/hybrid with effective mode; existing table/key filters only. Explicit multi-source selection can map to a table/key only when mdhouse defines that mapping; no new source-id registry, arbitrary source-set filter, generation/freshness requirement or rerank |
| R7 / Q9 | Required citation fields covered. No source revision history, line/page/section location, index generation or exact historical content resolver; source/path is caller provenance, not a filesystem grant. No fabricated component scores |
| R8 / Q10 | Bus Allow + mdhouse project grants; approved manager-only namespace administration. General read-vs-index/write role split in Q10 is not implemented by today's RAG and is not silently added here |
| R9 / Q11 / A7 | Existing unknown/incomplete outcomes, no automatic paid replay; resumable confirmed namespace drop. No durable request-id outcomes, duplicate-spend guarantee or general job cancellation |
| R10 / Q6 / A6 | No new event/job protocol, reconnect cursor or status event implementation in T-43. Existing runner/event-log behavior is not a RAG-specific progress feed; FileScan status/reconciliation and mdhouse #bus subscriptions require separate integration |
| R11 | Keys remain in provider blocks; source/path is data only. Connector and jailed-path grants remain FileScan/mdhouse work; no RAG source fetch |
| R12 / A8 | One search/evidence contract usable by manual and scheduled callers. No mdhouse flow/UI adapter or scheduled integration test in this patch |
| A1 | Review the existing JSON manifest and examples, including new methods/errors; no independent schema layer for its own sake |
| A2 | Describe provisioning and fake private fixture. Actual owner instance/Allow/collection setup and live verification remain separate authorized steps |
| A6 | Fake provider refusal/timeout, restart/binding/drop partial-state tests included. End-to-end source revocation, FileScan cancel and mdhouse crash/reconnect are outside this scope |
| Paid acceptance | No paid call required for deterministic acceptance. At most one live embedding check only after owner's explicit OK through #ab-ceo; never generation/classification or a paid retry |

## Steps and AI-time estimate

Elapsed AI working time from implementation approval; queue waits and owner provisioning are additional. No code begins until this plan is approved. Estimate reflects that namespace isolation touches existing CRUD, cache, backfill and deletion, not just search.

| Step | Paths / result | Focused proof | AI time; cumulative |
|---|---|---|---|
| 1. Review contract/manifest and invariants | rag agent.json, rules, proposed scoped repository surface; static capabilities; agree named refusal surface and catalogue limits | parser/capability fixtures, no paid/backend calls for static capabilities | 20–30 min; 20–30 |
| 2. Central namespace boundary + state | core/rules/blocks/prompts and a focused namespace module if useful; all CRUD, prompt, index and count paths; immutable model binding, legacy refusal | A/B identity/cursor/cache/backend-response cases; cross-scope and model-drift mutants | 45–60 min; 65–90 |
| 3. Dense and RRF hybrid | core query embedding through existing Voyage block; scoped BM25+dense query and live-head filtering | fake vectors rank meaning-only vs keyword-only records; inspect each prefetch filter and effective mode; wrong query model/ignored dense branch mutants | 35–50 min; 100–140 |
| 4. Evidence | upsert source/hash/index metadata and retrieval citations, preserving revision-safe publication | exact source/hash/chunk/excerpt/time assertions on replace/delete/draft data; missing-provenance mutant | 25–35 min; 125–175 |
| 5. Namespace admin | manager-only names/counts and confirmed resumable drop, KV/cache/cursor cleanup | ordinary-reader refusals; missing/wrong confirm; B survives; injected partial failures/restart; destructive mutants | 35–45 min; 160–220 |
| 6. Private bus contract + docs/version | rag guide, README, manifest help golden and RAG CHANGELOG; app version 0.3.0 in its own manifest; owning plan status/decision links after acceptance | real private bus + fake block agents; methods/errors/capabilities with zero paid calls; docs/link/package metadata checks | 20–30 min; 180–250 |
| 7. Two independent reads | exact committed diff; one Claude reader for the boundary/destructive paths | both readers name final tip, recorded mutants compiled and assertion-red; correct findings and re-read deltas | 30–45 min; 210–295 |

Expected author ready: about 3–4 h 10 min from approval; reviewed: about 3 h 30 min–4 h 55 min, excluding gate queue/fixes. If lead splits ownership, the boundary/model contract must land or be pinned first; retrieval and evidence cannot safely diverge on it.

Focused checks select named tests from `test_rag.py` / new fake-only files / `test_runtime.py`; do not use wildcard discovery that imports `test_real_qdrant.py` and starts a container. No normal/slow/stress/podman by the worker. Mutants take the shared smoke-gate lock via MCP under the worker's own name, only the owning focused test, restored immediately. The gate owns required landing checks; any additional real-Qdrant proof needs lead approval, stays provider-fake and is separate from paid permission. Fake RRF fixtures prove our construction and interpretation, not a fresh live backend deployment.

Versioning: bump the RAG APP to 0.3.0 and record its own changelog. Do not change bus VERSION/root CHANGELOG. T-39's independent agent pack branch is concurrent: rebase/integrate its packaging/version changes when landed; RAG's explicit app version must still win over pack metadata.

## Readiness and next action

Plan only. Approval first, then one task branch from fresh local main; no code or host state changed here. Two readers, one Claude, are mandatory for namespace/destructive code. Existing namespace-free data stays refused. Final review must trace every data method through the one boundary and inspect failure cleanup, not only the search happy path.

Weekly usage observed in own tmux pane: 31% left; context 33% at the observation. Tell lead immediately if weekly drops under 20%, and checkpoint before a context pause.
