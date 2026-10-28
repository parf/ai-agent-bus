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

- ☐ R.3 Qdrant container
  - Builds: Qdrant in podman under a systemd unit (quadlet); listens on localhost only, with an API key; data on a named volume; a pinned image version; a snapshot backup command
  - Acceptance: the unit starts after a reboot; the port is closed to other hosts; a request without the key is refused; a snapshot restores into a fresh container
  - Install: through setup like other host pieces, never by hand; root steps need the owner's OK
  - **R.3 revised plan for review/owner approval (2026-10-27; no code or host changes yet):**
    - **Shape/files:** an idempotent `agent-bus-setup --qdrant install|snapshot|check|uninstall`, Go installer/fake-command tests, and acceptance helper in `src/acceptance/`; setup renders rootful Quadlet container/volume units under `/etc/containers/systemd/`. Coordinate package/setup/smoke inventories with xAB; no daemon/runner unit changes or RAG application logic.
    - **Image/network:** pin `docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2` ([release](https://github.com/qdrant/qdrant/releases/tag/v1.20.0)). Networking is **blocked on [Q199](../QUESTIONS.md#rag-service)**: bridge plus an owner-approved firewall rule, or host networking with Qdrant itself bound to `127.0.0.1` and gRPC disabled. Host binding is recommended to avoid loopback-DNAT ambiguity, but is not selected silently. Telemetry is disabled. Preflight reports `route_localnet`, installed Podman version and occupied ports; check connects to each host non-loopback IPv4 address at the API port and requires refusal, not merely the output of `podman port`. Any external listener/accepted connection fails installation acceptance. A same-L2 crafted-route test needs an owner-provided second host if bridge is chosen; loopback publication alone is not proof.
    - **Hardening/storage:** container/unit `agent-bus-qdrant`, named volume `agent-bus-qdrant` at `/qdrant/storage`, UID/GID `1000:1000`; setup/Quadlet create the volume with those owners. Main and restore containers use the same UID, `NoNewPrivileges=true`, `DropCapability=ALL`, `ReadOnly=true`, and a bounded `/tmp` tmpfs; snapshots, temporary storage and the initialization marker stay under the writable data volume. Use `Pull=never`: only explicit install pulls. Put `MemoryMax=2G` and `CPUQuota=200%` in `[Service]`, `PidsLimit=512` in `[Container]`; 2 GiB is a proposal the owner may widen before real indexing. Bound startup/restart waits. `[Install] WantedBy=multi-user.target` gives boot activation; do not enable the generated service. Dry-run renders temporary units and runs the installed Quadlet generator's `--dryrun`, showing output and rejecting unsupported keys before permanent writes.
    - **Key:** generate 32 random bytes encoded as 64 lowercase hex characters once; preserve on reinstall. Store Qdrant's small config containing `service.api_key` in root-only `/etc/agent-bus/qdrant/config.yaml` and import it as a **mounted Podman secret**, `agent-bus-qdrant-config`, `uid=1000,gid=1000,mode=0400`; invoke Qdrant with `--config-path /run/secrets/agent-bus-qdrant-config`. The runner gets the identical value as `QDRANT_API_KEY` in `/var/lib/agent-bus/parf-ab-runner/qdrant/env` (runner-owned `0600`). Do not use a secret environment variable: `--env-file` hides argv but still exposes resolved values to privileged container inspect. Test equality without printing bytes and ensure container inspect/argv/setup logs contain no key. Podman's privileged `secret inspect --showsecret` is explicitly outside this guarantee and is never invoked. Its default secret inspection returns metadata only. Refuse conflicting existing files/resources; no silent rotation/adoption. Only R.4 enables the proxy.
    - **Snapshot:** authenticated local full-storage snapshot, at most **2 GiB**, at most **300 s** including creation/download, no redirect/retry. Check free disk first and refuse below 30 GiB or without snapshot budget plus 1 GiB spare. Require and verify the returned SHA-256 checksum; missing/mismatched checksum is failure. Write a new root-owned `0600` partial file in root-only `/var/lib/agent-bus/qdrant.backup/`, fsync it, atomically rename to an owned UTC-timestamped `.snapshot`, then fsync the directory. Retain the latest **7 completed owned snapshots** only, pruning after a successful new backup; never remove arbitrary files. Delete the just-created Qdrant-side snapshot after streaming (also on failure, without touching other snapshots), and remove partial local output on failure, so repeated backups do not grow server storage without bound.
    - **Check/data boundary:** `check` is commissioning-only: **refuse before any write/restart if any collection already exists**. It creates only `ab_r3_acceptance`, writes/queries synthetic vectors, tests a no-key `/collections` refusal, restarts **only the live Qdrant unit** to prove persistence (**brief downtime; never run this on live RAG data**), and snapshots that fixture-only store. Delete its fixture collection and its created server/local snapshots on every exit. Restore only into the fresh, labeled fixture volume/container below; verify exact point id/vector/payload after recovery. A normal operation against production uses `snapshot`, not `check`.
    - **Restore/SELinux:** no backup-directory bind mount and no relabel of it. Package the verified fixture snapshot as a single safe tar entry `restore/r3-acceptance.snapshot`, owned `1000:1000`, import it with `podman volume import` into the fresh named fixture volume, then recover from `/qdrant/storage/restore/r3-acceptance.snapshot`. Main and restore run the pinned Qdrant executable directly as UID/GID `1000:1000` (not an implicit image privilege transition). Named volumes/secrets use Podman's SELinux handling. The fixture API uses **127.0.0.1:16333**; fail before creating resources if occupied. Refuse any pre-existing fixture container/volume. Label both `agent-bus.fixture=r3` plus a per-run id; cleanup validates captured ids and labels before removing only resources created by that run, including after failures and port conflicts. Cleanup failure is reported, never claimed as success.
    - **Tests/mutants:** normal uses private temporary roots, fake podman/systemctl/generator/HTTP and no host journal. Assert secret/runner key equality and permissions, clean inspect/argv/logs, idempotence and conflict refusal, image/network/hardening/boot fields, bounded/checksummed atomic snapshots, retention, refusal on non-fixture data, labeled cleanup and SELinux-safe import. Owning mutants remove a required field/bound/checksum/privacy/data/label check or make the restore verifier accept absent point data. The owner-approved real acceptance is separate and uses only synthetic data. Actual reboot proof stays **pending** in DONE until an owner-scheduled reboot.
    - **Uninstall:** stop only the owned unit, remove only owned Quadlets and reload systemd. **Keep** production volume, Podman secret, config, runner env key and backups by default, so reinstall can recover unchanged. No data/key purge flag is implemented or authorized in R.3; any later explicit purge option needs another owner-approved command list. No image/volume prune, firewall/sysctl change or other service restart.
    - **Landing:** after plan review and Q199/host approval, build/test and get code review; take the smoke lock for owning mutants and normal, bump the shared version/changelog, and commit locally by path. Apply only approved component actions below, run `check-deploy`, report commit/version/check counts to AB2/xAB, and request finished-block review. xAB owns completion TODO/DONE updates. R.4 gets a separate plan after R.3; no git push.

    <details><summary>Owner approval required: revised host command list (pending Q199)</summary>

    None has run. These are component setup entry points, not permission for arbitrary root work. The final approved networking branch will be written explicitly before apply. `src/agent-bus-setup` is the locally committed, stamped binary; build is unprivileged and restarts no bus service.

    ```sh
    sudo src/agent-bus-setup --qdrant install --for parf --dry-run
    sudo src/agent-bus-setup --qdrant install --for parf
    sudo src/agent-bus-setup --qdrant check
    sudo src/agent-bus-setup --qdrant snapshot
    src/check-deploy.sh
    ```

    Preflight/dry-run child commands (read-only except an owned temporary render directory, cleaned on exit):

    ```sh
    podman --version
    sysctl -n net.ipv4.conf.all.route_localnet
    env QUADLET_UNIT_DIRS=/run/agent-bus-qdrant-validate /usr/lib/systemd/system-generators/podman-system-generator --dryrun
    podman container exists agent-bus-qdrant
    podman volume exists agent-bus-qdrant
    podman secret inspect agent-bus-qdrant-config
    ```

    Setup creates private config/runner-env/backup paths above; validates existing owned resource labels using metadata-only formatted inspection (never full env/config output), and uses these install children. Generator-owned volume creation is equivalent to `podman volume create --uid=1000 --gid=1000 --label io.agent-bus.component=qdrant agent-bus-qdrant`; conflict checks precede it.

    ```sh
    podman pull docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2
    podman secret create --label io.agent-bus.component=qdrant agent-bus-qdrant-config /etc/agent-bus/qdrant/config.yaml
    systemctl daemon-reload
    systemctl start agent-bus-qdrant.service
    systemctl is-active agent-bus-qdrant.service
    ```

    `check` first requires an empty store and free port 16333, then additionally runs `systemctl restart agent-bus-qdrant.service` and the following fixture children. The helper creates/removes only its root-private fixture snapshot/tar paths; the tar contains only the verified synthetic snapshot. `RUN_ID` below is the same generated UUID recorded by the helper; `NETWORK_ARGS` is **not approved yet**: Q199 selects `--network=host` (config binds 127.0.0.1, gRPC off; HTTP port override 16333) or the bridge publication `--publish=127.0.0.1:16333:6333` with separately approved firewall rules. `FIXTURE_HTTP_PORT` is respectively `16333` or `6333`; dry-run prints the chosen literal values before approval.

    ```sh
    podman container exists agent-bus-qdrant-restore-check
    podman volume exists agent-bus-qdrant-restore-check
    podman volume create --uid=1000 --gid=1000 --label agent-bus.fixture=r3 --label agent-bus.run=RUN_ID agent-bus-qdrant-restore-check
    podman volume import agent-bus-qdrant-restore-check /var/lib/agent-bus/qdrant.backup/r3-acceptance.tar
    podman run --detach --pull=never --name agent-bus-qdrant-restore-check --label agent-bus.fixture=r3 --label agent-bus.run=RUN_ID --user=1000:1000 --read-only --cap-drop=ALL --security-opt=no-new-privileges --tmpfs=/tmp:rw,nosuid,nodev,noexec,size=64m --pids-limit=512 --memory=2g --cpus=2 NETWORK_ARGS --volume agent-bus-qdrant-restore-check:/qdrant/storage --secret agent-bus-qdrant-config,type=mount,uid=1000,gid=1000,mode=0400 --env QDRANT__SERVICE__HTTP_PORT=FIXTURE_HTTP_PORT --env QDRANT_INIT_FILE_PATH=/qdrant/storage/.qdrant-initialized --entrypoint /qdrant/qdrant docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2 --config-path /run/secrets/agent-bus-qdrant-config --storage-snapshot /qdrant/storage/restore/r3-acceptance.snapshot
    podman container inspect --format '{{.Id}} {{index .Config.Labels "agent-bus.fixture"}} {{index .Config.Labels "agent-bus.run"}}' agent-bus-qdrant-restore-check
    podman volume inspect --format '{{.Name}} {{index .Labels "agent-bus.fixture"}} {{index .Labels "agent-bus.run"}}' agent-bus-qdrant-restore-check
    podman rm --force agent-bus-qdrant-restore-check
    podman volume rm agent-bus-qdrant-restore-check
    ```

    No removal occurs unless those ids/labels match this run; cleanup runs on all exit paths. `check` and `snapshot` make bounded authenticated local HTTP calls internally, with no key in argv/output, and delete only their own Qdrant-side snapshot/fixture collection as specified above. Snapshot preflight also runs `df -h /var/lib/agent-bus/qdrant.backup`. Uninstall, if needed, is separately approved as `sudo src/agent-bus-setup --qdrant uninstall` and retains all data and keys. Any additional host/root/podman/systemd command requires another owner OK.

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
