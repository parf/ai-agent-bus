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

- ☐ R.4 `#qdrant`: a transparent proxy to the Qdrant HTTP API
  - Builds: pass-through of any Qdrant API path with the key added from the runner env, as `#voyage` does; no logic of its own
  - Acceptance: a collection is created, a point upserted and found through the bus; a path outside the API is refused; the key is in no reply or log
  - Mutant: drop the key, or allow a path outside the API; a check must fail
  - **R.4 plan delta for role review (2026-10-28; RD133, including no-role=write; no proxy code yet):**
    - **Shape:** a stdlib per-message runner package `src/agents/qdrant/`, following Voyage: one `request` method, text `request VERB /api/path[?query] [raw JSON body]` or the equivalent application JSON `{ "method":"request", "verb":"PUT", "path":"/collections/example", "body":"..." }`. Path ends at the first ASCII space; raw spaces in a URL must be percent-encoded, never silently truncated into a valid path. JSON form rejects extra/contradictory text fields. Verbs are case-sensitive GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS. Refuse a body on GET/HEAD/OPTIONS; validate the other raw JSON bodies without reserializing their bytes. Forward once with Qdrant's `api-key` header; do not reinterpret point/query JSON or add RAG operations. Keep the response's JSON/text content, selected by Content-Type (masking an echoed key); errors retain HTTP status and redacted body, never raw exception text. HEAD/204 return status and redacted response-header metadata only. Send Accept-Encoding: identity and refuse nonidentity encodings; read cap+1 and fail on exceed. Binary or oversized bodies fail explicitly rather than corrupting a bus text answer; no storage schema.
    - **Configuration/key:** required `url`, an operator-chosen HTTP(S) base URL, including an optional reverse-proxy path prefix; **no localhost default**. Refuse userinfo, query/fragment, controls and malformed ports; HTTPS uses normal certificate verification. The runner host env holds `QDRANT_API_KEY` (`# required` in env.dist), already installed on this node. No key in Configuration, URLs, argv, answers or logs. `needs.network:["*"]` declares the arbitrary operator-selected endpoint; today's runner network grant is coarse, not a host allowlist. Default ACL `@owner`. Declare `roles:["read","write","admin"]` in the manifest. Interpret only daemon-delivered envelope roles, never claims inside the application body; the runner also supplies those roles as `AB_ROLE_*`. Highest understood tier wins: generated `owner`/`maintainer` or explicit `admin` => admin, else explicit `write` => write, else explicit `read` => read, else the ACL-admitted default is write. Unrecognized labels add no tier. A required tier above the caller is refused before connecting and named in the answer. Bare grants such as `#rag` get write; `@team(read)` restricts, `#rag(write)` is explicit, and admin grants convey full-store administration. Any operator HTTP(S) URL is allowed; non-loopback plain HTTP emits one safe notice per per-message invocation, and docs say the key travels unencrypted (RD133 retains this part of RD132).
    - **API boundary:** callers supply a normalized absolute API path, never a URL/authority/fragment. Before any connection, require one leading `/` and reject `%2e`/`%2f`/`%5c` case-insensitively, `//`, semicolons, backslashes, dot segments, fragments, controls, non-ASCII and raw whitespace in path/query. Join the once-trimmed base prefix by string concatenation and send a request target, never a URL. Permit Qdrant's documented top-level API roots (including service/health roots and `/`), plus their subpaths, and preserve a bounded query string; reject non-API roots such as `/etc` or `/dashboard`. The fixed operator base prefix stays in front of every path. No caller headers, redirects, retries or cross-origin follow-up. Pass every API path through when the caller tier permits it, including recovery/upload/shard recovery, cluster, issues, telemetry, metrics and locks; there is no administrative endpoint deny list. The explicit method/path table below classifies known data operations; every unmatched combination defaults to admin, even a GET. API roots come from the [pinned upstream OpenAPI](https://github.com/qdrant/qdrant/blob/v1.20.0/docs/redoc/master/openapi.json); there is no per-endpoint payload validator.
    - **Bounds/files:** reuse the shared provider helper's 1 MiB request/answer cap and at most 60 s/deadline-minus-0.1, bounded envelope input, alarm cleanup and redaction of body/error/header echoes; logs contain verb/path without query, byte counts and status, never query/body/header values; use generic `http.client.request` for Qdrant's verb and header, without changing AI/Voyage behavior. Package its own entry/manifest/env.dist plus shared helper/metadata/version; update package/setup inventories, independent packaging fixtures, outside-checkout acceptance, agents guide/README, and a new L smoke shard/map entry. Coordinate the smoke edit lock and shared inventories before editing; no daemon changes.
    - **Checks/mutants:** fake HTTP fixtures prove exact verb/path/query/body/key, configurable remote host/port/base prefix (not just localhost), HTTP/HTTPS selection, case-sensitive verbs, exact raw body bytes, body-on-GET refusal, text query-space rejection, identity encoding, Content-Type/HEAD/204/error status behavior, no-key refusal, JSON/escaped-key/header redaction, non-API/traversal/absolute-URL rejection **before any connection**, no redirect/retry, bounded trickle/size/deadline, generated help and outside-checkout packaging. Owning compiling mutants let write pass with read, admin pass with write, or an unknown path pass below admin; separately prove no-role write succeeds and no-role admin fails. Transport mutants drop the key, allow an outside path/origin or lose the base prefix, skip redaction/caps/deadline, or retry an uncertain write; assertions must fail. Final normal under the smoke lock, fresh shared bump/changelog, source review and finished-block review.
    - **Deploy/acceptance:** after source landing, package/reload only Qdrant with `url=http://127.0.0.1:6333` as this node's explicit local choice, preserving the host key; register `#qdrant@parf.us` ACL `@owner`. Real bus acceptance creates one uniquely named synthetic collection, upserts a synthetic point, queries and verifies it, then deletes that collection in try/finally cleanup. Keep its unique prefix in a variable and assert the DELETE target has that prefix before sending. Other writers may change collection lists: assert only this fixture was absent before and absent after, without requiring global equality. Touch no other data. Live acceptance also checks explicit read cannot upsert, explicit write cannot create/drop a collection, and a bare/no-role grant can mutate points but cannot perform admin calls. Use temporary synthetic principals/grants for these checks and remove them; never change existing grants. Check outside-API refusal and sentinel key absence with fakes, live help/config/role metadata and `check-deploy`. No external provider calls, host/container/bus restarts, sudo, snapshots, real RAG data or push. xAB folds the results into TODO/DONE.


    **Explicit HTTP method/path tiers (c = one normalized collection segment; point id = numeric/UUID; query string does not change tier):**

    | HTTP method | API path (exact except named segments/sets) | Minimum tier |
    |---|---|---|
    | GET, HEAD | `/collections`, `/collections/{c}` | read |
    | GET, HEAD | `/collections/{c}/points/{point_id}` | read |
    | POST | `/collections/{c}/points` (retrieve by ids) | read |
    | POST | `/collections/{c}/points/{scroll,count,query}`; `query/{batch,groups}` | read |
    | POST | `/collections/{c}/points/search` and `search/{batch,groups,matrix/pairs,matrix/offsets}` | read |
    | POST | `/collections/{c}/points/recommend` and `recommend/{batch,groups}`; `discover` and `discover/batch` | read |
    | PUT | `/collections/{c}/points`, `/collections/{c}/points/vectors` | write |
    | POST | `/collections/{c}/points/{delete,batch}`; `points/vectors/delete` | write |
    | POST, PUT | `/collections/{c}/points/payload` | write |
    | POST | `/collections/{c}/points/payload/{delete,clear}` | write |
    | Every other allowed verb/path pair | Collection create/delete/update, indexes, aliases, snapshots/recovery/upload, shard/cluster/admin/service operations, and **unknown** paths | admin |

    Read/write/admin are cumulative. Match the parsed path's whole segments and exact suffix, not a prefix or a substring; all API-root/path and verb validation happens before tier classification, then authorization before opening a connection. Unknown paths **within** permitted API roots are forwarded only for admin; paths outside the API boundary are always refused. The single `request` method stays visible in generic help because no-role callers have write; its help explains per-call tiers rather than pretending one method-level role describes every verb.


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
    - **Configuration/authority:** bind the three block names, collection and one embedding model/vector size per instance. Namespace/table remain filters, never grants. Strictly validate inputs and dimensions before provider work; compile filters from accepted fields, never accept raw backend filters. Calls to `#qdrant` use R.4's frozen `request` form and this instance's write grant. **❓ [Q204](../QUESTIONS.md#rag-service): recommend the owner creates the collection once and grants only write; do not silently create it or obtain admin.** Verify its vector settings before writes; missing/mismatched collection is a clear refusal. **❓ [Q205](../QUESTIONS.md#rag-service): the design still reserves public wire/argument approval for the owner; recommend JSON-only core calls using the existing record fields, with no aliases, before freezing their parser/help.** These choices do not block reviewing the internal core/fakes; no live collection or grant changes in R.6.
    - **CRUD/ingest:** implement upsert/get/list/delete and prompt read/set/delete against the existing record definition. Deterministic typed-tuple UUIDs identify records/chunks; one canonical record payload permits get/list without pagination through half a record. Initial bounded MIME handlers cover UTF-8 plain text, Markdown, HTML and JSON; unsupported types refuse before downstream work. Prepare all chunks, Jev Choice labels and supplied/computed vectors before deleting/replacing anything; a Jev/Voyage failure leaves the previous record untouched. Validate finite probabilities, configured labels, threshold/top-k and vector sizes. Replace only that exact record's chunks, preserve supplied embeddings and make repeated upserts converge. Validate every backend reply; report uncertain/partial backend writes explicitly, never success or a cross-call transaction guarantee, and never blindly retry. Bound body/chunks/list pages and every downstream call by the original message deadline.
    - **Prompts/keyword seam:** own KV holds namespace/table labels, questions, thresholds/top-k and the future summary prompt, behind a bounded read cache. A hit makes zero daemon calls; successful set/delete invalidates (does not seed) the entry; failed writes cannot install a new cache value. External KV edits are seen after restart, as settled. Missing classification setup refuses ingestion. Keyword encoding has one injectable interface; R.6 neither chooses BM25/stemming nor claims hybrid search while R.5 waits for owner samples. Summary/search remain R.7–R.9 work.
    - **Tests/mutants:** fake `#qdrant`/`#voyage`/`#ai` only, plus a disposable bus to prove trusted sender identity and correlation; prohibit external connections and never use live provider keys. Check idempotent replacement including shrinking chunks, tuple-collision resistance, scoped delete and pagination, MIME parsing/bounds, vectors/probabilities, label threshold/top-k, provider failure before mutation, uncertain backend writes, zero-call cache hits, invalidation/restart, asks doing no work and app-version packaging. Owning assertion-red mutants break IDs, widen a deletion filter, retain stale cache, write after a failed classification, bypass vector/parser bounds or spoof a downstream caller. Real block/provider calls stay out of every test/gate.
    - **Landing/review:** after plan approval and owner wire clarification, run owning mutants and normal under the smoke lock, record app 0.1.0/changelog and the exact shared integration bump only if setup behavior requires it. Deploy only the app/integration pieces changed; no real data ingestion, provider call, collection creation or owner session restart. Verify package help/configuration state and check-deploy, request finished-step read-only review, resolve findings, mark R.6 done and move version/commit/gate/review evidence to DONE. R.5 and the R.4 bus integration remain explicitly pending until their prerequisites land. No push.

- ☐ R.7 `#rag` search
  - Builds: hybrid `search` with filters and optional rerank, the keyword half chosen by R.5
  - Acceptance: on a fixture set, filters hold exactly; a keyword-only match and a meaning-only match both appear; rerank reorders; hits carry the agreed fields

- ☐ R.8 `#rag` extras: `complete`, `similar`, `count`, `classify`
  - Owner to confirm the list before this step

- ☐ R.9 Summary, docs and end-to-end
  - Builds: the optional summary chunk through `#ai`; a user guide under `docs/agents/`; one end-to-end run on the owner's data, approved by the owner
