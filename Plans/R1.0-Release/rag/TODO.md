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

- ☐ R.2 `#ai` learns OpenRouter's Decisions API, for Jev
  - Builds: a `decide` method passing a decisions request to OpenRouter unchanged, with the key added; same bounds, masking and logging rules as `ask`
  - Acceptance: against a fake OpenRouter, a Choice question returns every option's probability as given; an upstream refusal is the answer; the key is in no reply or log
  - Mutant: drop the key, rewrite the body, or leak the key in an error; a check must fail
  - **R.2 plan for read-only review (2026-10-27; no code yet):**
    - **Shape/files:** extend `src/agents/ai/ai.py`, its metadata, fake-provider tests and current AI docs with `decide <raw JSON>`. JSON method calls carry the raw request as a string so outgoing bytes stay unchanged. Use the existing provider HTTP helper; no new package, dependency, credential store or Configuration key. Bare prompts and existing chat/ask forms retain their exact behavior.
    - **Provider contract:** POST to the configured API origin's `/api/alpha/decisions`, the endpoint in the [official Decisions reference](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request). Keep chat's configured base path unchanged. The decisions request supplies its own model (e.g. `typesafe/jev-1.14`); do not inject the chat default, convert questions, select labels or discard probabilities. Return the full provider JSON. Keep the same host env key, verified TLS, one attempt, no redirects/streaming, size/deadline bounds and masking.
    - **Tests/mutants:** loopback fake OpenRouter only, external connections blocked; pin endpoint, Authorization and exact outgoing bytes, every Choice option's probability including zero, upstream refusal/429, key echoes and logs, caps/deadline and one attempt. Re-run all chat tests and the original byte comparison. Owning mutants omit auth, rewrite decisions bytes, leak a key and discard a probability; each must fail on an assertion. Metadata asks must perform no provider work.
    - **Landing:** after reviewer approval, acquire the smoke lock for mutants and normal, take the fresh shared PATCH with a changelog line, commit locally, deploy only AI and reload the runner. Verify help and check-deploy without a real provider request or reading any host key. Ask for a finished-block review, resolve findings, then mark R.2 done and move version/commit/gate/review evidence to DONE. No git push.

- ☐ R.3 Qdrant container
  - Builds: Qdrant in podman under a systemd unit (quadlet); listens on localhost only, with an API key; data on a named volume; a pinned image version; a snapshot backup command
  - Acceptance: the unit starts after a reboot; the port is closed to other hosts; a request without the key is refused; a snapshot restores into a fresh container
  - Install: through setup like other host pieces, never by hand; root steps need the owner's OK
  - **R.3 plan for review and host approval (2026-10-27; no code or host changes yet):**
    - **Shape/files:** add an idempotent `agent-bus-setup --qdrant install|snapshot|check|uninstall` component, its Go installer/fixture tests, and Qdrant acceptance helper under `src/acceptance/`. Setup renders a rootful Quadlet container and named-volume unit in `/etc/containers/systemd/`; no daemon/runner unit changes. Update setup/operations docs and only required package/runtime fixtures, coordinated with xAB. No RAG wire format or application logic.
    - **Image and exposure:** pin `docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2` ([official release](https://github.com/qdrant/qdrant/releases/tag/v1.20.0), registry manifest verified). Unit/container `agent-bus-qdrant`; named volume `agent-bus-qdrant` mounted at `/qdrant/storage`; snapshots in that volume until downloaded. Publish **only** `127.0.0.1:6333:6333`; no gRPC port publication, host networking or auto-update. Disable telemetry. Quadlet `[Install] WantedBy=multi-user.target` supplies reboot activation; do not try to enable its generated service. Proposed limits: 2 GiB memory, 200% CPU and 512 tasks, with restart-on-failure and a bounded startup wait.
    - **Key:** setup generates one random key if absent, preserves it on reinstall, writes `/etc/agent-bus/qdrant/env` as root-only `0600` for Qdrant, and the corresponding `QDRANT_API_KEY` into the existing person's runner `qdrant/env` layer (`/var/lib/agent-bus/parf-ab-runner/qdrant/env` here, runner-owned `0600`). Never print key bytes, pass them in argv, read unrelated runner env files, or silently replace an existing conflicting key. Only R.4 enables the agent. Existing conflicting Quadlets/container/volume are refused, not adopted or removed.
    - **Backups/restore:** `snapshot` requests a full-storage Qdrant snapshot and streams it to `/var/lib/agent-bus/qdrant.backup/` (root-only directory/files), with size/time bounds, partial-file cleanup and no redirects. `check` uses only synthetic collection `ab_r3_acceptance`: create/upsert/query, reject a no-key `/collections` request, snapshot, restore into the fresh named fixture volume/container below, then verify the same point and payload. Never restore over the live volume. Stop/start the owned unit to check persistence and verify the generator's boot dependency; actual reboot verification waits for an owner-scheduled reboot.
    - **Tests/mutants:** normal uses temporary filesystem roots and fake podman/systemctl/HTTP only, never a real unit or host journal. Check key/file permissions, idempotence/conflict refusal, loopback-only port binding, pinned image, named data volume, boot wiring, bounded/masked snapshot download and failure cleanup. Owning mutants: wildcard bind, missing API-key environment, world-readable key, omitted volume or unpinned image, and a restore verifier accepting missing data. The approved real acceptance runs separately, bounded, on synthetic data; it must prove a fresh-volume restore, not merely a successful command. No other service restarts, image/volume pruning or real provider calls.
    - **Landing:** after plan review, build/test, get code review, take the smoke lock for owning mutants and the normal gate, bump the shared bus version/changelog, and commit locally by path. Apply only the owner-approved setup actions below, run `check-deploy`, report commit/version/check counts to AB2 and xAB, then ask `#claude/ab2-review` for the finished-block review. xAB folds completion evidence into TODO/DONE. R.4 gets its own reviewed plan after R.3; no git push.

    <details><summary>Owner approval required: exact host command entry points and their effects</summary>

    No command in this list has been run. The implementation's dry-run must show these paths and child commands before the apply step. `src/agent-bus-setup` means the locally committed, stamped setup binary; building it is unprivileged and does not restart the bus.

    ```sh
    sudo src/agent-bus-setup --qdrant install --for parf --dry-run
    sudo src/agent-bus-setup --qdrant install --for parf
    sudo src/agent-bus-setup --qdrant check
    sudo src/agent-bus-setup --qdrant snapshot
    src/check-deploy.sh
    ```

    `install` creates the private key/backup directories and key files described above, writes `/etc/containers/systemd/agent-bus-qdrant.container` and `.volume`, then runs these exact child commands (without key material in arguments):

    ```sh
    podman pull docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2
    systemctl daemon-reload
    systemctl start agent-bus-qdrant.service
    systemctl is-active agent-bus-qdrant.service
    podman port agent-bus-qdrant
    ```

    The Quadlet generator creates the named volume and starts the container from the pinned image with the private env file and the sole loopback mapping; its generated argv is inspected in dry-run/tests. `check` additionally restarts **only** `agent-bus-qdrant.service`, uses the fixed fixture snapshot `/var/lib/agent-bus/qdrant.backup/r3-acceptance.snapshot`, and runs:

    ```sh
    podman volume create agent-bus-qdrant-restore-check
    podman run --detach --name agent-bus-qdrant-restore-check --publish 127.0.0.1:6335:6333 --volume agent-bus-qdrant-restore-check:/qdrant/storage --volume /var/lib/agent-bus/qdrant.backup:/restore:ro --env-file /etc/agent-bus/qdrant/env --entrypoint /qdrant/qdrant docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2 --storage-snapshot /restore/r3-acceptance.snapshot
    podman rm --force agent-bus-qdrant-restore-check
    podman volume rm agent-bus-qdrant-restore-check
    ```

    Fixture cleanup is scoped to those exact owned names and its synthetic collection/snapshot, including on failure; pre-existing fixture resources cause refusal. Both listeners are checked to be loopback-only. All authenticated HTTP calls and snapshot transfers happen inside the helper, with bounded local URLs and no key in CLI arguments/output. If installation must be rolled back, the separately approved command is `sudo src/agent-bus-setup --qdrant uninstall`: stop only its unit, remove only its owned Quadlets and reload systemd; **retain** the data volume, key files and backups. No production volume deletion or package installation is approved by this plan. Any required command outside this list returns to the owner first.

    </details>

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
