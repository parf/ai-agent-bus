# Changelog 0.9

📌 **TL;DR:** Archived changes **0.9.0–0.9.303**, 2026-10-19–2026-11-02, grouped by area. These record development transitions, including choices later replaced. Earlier: [0.8](CHANGELOG.0.8.md#changelog-08); next: [1.0](../../CHANGELOG.1.0.md#changelog-10).

## Protocol, discovery and command line

- **0.9.0–0.9.100:** reserved in-daemon `#bus` answered help, status, version and listing. Added target-permission checks, tag-based listing/calls, strict tag syntax and machine-readable method/Configuration/Secret help from package metadata.
- **0.9.101–0.9.114:** asks were checked before ordinary methods, with unsupported asks refused and runner lifecycle status answered by the runner. Scripts could opt out of asks. Added man-style command help; `agent-bus token` replaced the general token-helper command while its SSH wrapper remained. Introduced `config`, `ls --table` and `register --service-addr`, retaining prior spellings at that point.
- **0.9.158–0.9.161:** the daemon Owner gained validated, audited, durable `#bus` Configuration for the default realm and agent-event settings. First start derived the realm from the host; saves applied immediately and entered backups. A web Bus setup page exposed the same operation.
- **0.9.192–0.9.198:** multiple tags matched by intersection. Repeated `--tag` options were joined rather than overwritten, and empty values were refused instead of silently removing the filter.
- **0.9.212–0.9.242:** consume gained optional host observations, retained for two minutes in a bounded memory-only listing. `asks=only` readers could wait beside an ordinary reader without becoming a second unfiltered reader; the ordinary reader had priority for arriving/requeued asks.
- **0.9.261–0.9.303:** froze the protocol-1 route contract and added `/identity` compatibility fields, named refusal kinds and one name grammar through the daemon/helpers. All six clients ran shared conformance cases; unknown routes used `no_route`, and missing data used `not_found`.

## Authentication and authority

- **0.9.0–0.9.100:** authentication preceded bounded HTTP body reads. Runner output, errors and diagnostics masked tokens/private values; linked settings were refused and mapped-user/reload/debug-probe authority was tightened.
- **0.9.137–0.9.150:** `GET /may-send?who=&name=` let an admitted caller check another name's permission to a visible target; Python and TypeScript exposed matching calls.
- **0.9.163–0.9.189:** iterative Group traversal handled deep/cyclic ACL, directory and delivery chains without stack exhaustion. Lock waiters gained FIFO handoff, cancellation and give-up removal. A shared masker covered complete secret values and individual lines of multiline secrets; debug env output used an allowlist.
- **0.9.261–0.9.303:** key proof signed only the bus namespace, validated challenges and carried rotation through the proof flow. Enrolment could start without a token. GitHub directory lookups used a short cache and one daemon-wide uncached budget; local files stayed uncached so revocation applied immediately. Inactive agent/User log access matched a missing-name refusal, and subscription admission checked destination rights without revealing unavailable records.

## Calls, readers and client outcomes

- **0.9.137–0.9.150:** a waiting call reserved its answer against another face reading the same inbox.
- **0.9.163–0.9.189:** zero/sub-millisecond send waits used a plain send across clients rather than an invalid `wait=0` request; negative waits remained refused.
- **0.9.199–0.9.211:** self-call handling and shared cases prevented a client from accepting its own request as the answer. Cancelled-reader handoffs were purged/requeued in arrival order instead of losing the handed message.
- **0.9.205–0.9.242:** Go and then PHP consume polls over HTTPS used HTTP/1.1 while other supported traffic could retain HTTP/2, fixing replaced-reader cancellation losses.
- **0.9.261–0.9.303:** documented uncertain external effects: a caller timeout did not establish that a write failed, and uncertain writes were not automatically repeated.

## Backups, KV and bigfiles

- **0.9.101–0.9.114:** added Owner-only `age`-encrypted full/partial exports of permanent records, Configuration, Secret, KV and credentials, with readable non-secret `.info` sidecars. Import required explicit conflict choices before an all-or-nothing transaction.
- **0.9.250–0.9.260:** introduced the then-named big-key store: expiring private files or requested memory storage, up to 32 MiB, with SHA-256 on transfer and lifecycle cleanup. A record's Maintainers wrote/deleted and its allow list read/listed. CLI, Go/Python/TypeScript, MCP and web exposed the operations; MCP moved file contents outside model context. An unusable store returned 503 while the daemon remained up.
- **0.9.262–0.9.263:** renamed big keys to bigfiles across faces, clients and IMAP. Moved their index into the database; the first migration removed pre-index files rather than importing them, with the removal count logged.
- **0.9.270–0.9.281:** transfer body handling used an idle timeout within a size-scaled total budget, with the CLI and Python/TypeScript following that budget. Empty strings/bytes and JSON survived KV import exactly; an unused global KV preflight scan was removed. MCP bigfile paths were fenced by `AGENT_BUS_BIGFILE_DIR`, and each user-runner person gained a private files directory.

## Runner and agent installation

- **0.9.0–0.9.100:** acknowledgments shared the call deadline; Configuration was bounded and supplied as `AGENT_BUS_CONFIG`. Required env keys, killed-cgroup recovery and reader replacement became explicit runtime behavior.
- **0.9.115–0.9.162:** status asks reported health, and the synthetic no-tags sentinel was removed. Added a person's user-manager runner for agents such as tmux; its sessions and state directory survived runner lifecycle changes. A managed runner taking over a dedicated-run record cleared the stale script field.
- **0.9.163–0.9.189:** dependency order was validated, with missing/cyclic dependencies refused. Dedicated scripts read their Secret per call. Kept sandboxed agents restarted after replacement of the mounted daemon socket; schedulers exited after 30 consecutive bus errors for supervised recovery.
- **0.9.199–0.9.211:** settings changes sent SIGHUP where supported; confinement/limit changes restarted the kept process and signal exits were reported. Added ID-correlated pools of kept JSONL children; settings overlays required their sandbox path. Dedicated runs reconnected after daemon restart.
- **0.9.212–0.9.242:** shared code loaded before bundled packages; bundled entries required enablement and metadata-only standalone services were skipped. Multiple runners could share one Agent with host reporting/override. Bus-driven install accepted bundled code or a full-commit-pinned git source, refused unsafe destinations/hooks and preserved data/tokens on removal. Agent packagers, including standalone tmux, built before replacing the entire destination tree and refused concurrent publication.
- **0.9.250–0.9.260:** setup refreshed a user runner as its person; a failed development refresh reported the manual recovery rather than disguising it.
- **0.9.261–0.9.303:** staged merged overrides published atomically with per-key recovery; reload and answer-size handling received further race/boundary fixes.

## Shared subscriptions and event logs

- **0.9.137–0.9.150:** scheduling and forwarding checked the creator's may-send at add and before each delivery, while the target ACL judged the sending helper's own identity. Unconfirmed scheduling sends paused rather than being blindly replayed.
- **0.9.163–0.9.189:** Python/TypeScript subscription engines shared vectors for filters, revocation, authority failures and refused copies. Added standalone `#eventlog` with named notice/warning/alert rings, bounded retention, private lock mode, null-Configuration defaults and clean interrupted-request shutdown.
- **0.9.199–0.9.242:** logwatch, IMAP, Slack, Telegram, cron and at moved to the shared engine. Each refused copy was dropped/counted with a warning while later matches remained usable. Deterministic or random 12-hex IDs replaced older IDs through interruption-safe migration; damaged legacy state stayed inert. `sub edit` was replaced by delete-and-add.
- **0.9.261:** subscription filters used CPU budgets rather than wall time to avoid load-induced false timeouts.
- **0.9.289–0.9.302:** added private per-agent event rings in the daemon and `#bus` write/read/list/pull/clean and subscription operations. Runner starts, masked stderr warnings and exits fed those logs; the web exposed them only to the Agent's managers. Schema 10 persisted subscriptions and schema 11 persisted bounded alert snapshots; 0.9.302 grouped snapshot writes and flushed them on clean shutdown.

## Bundled helpers and applications

- **0.9.0–0.9.100:** added git, logwatch, debug doctor, AI proxy and Telnyx SMS. Systemd supported declared unit caps; Slack/Telegram shared reply routing with per-chat routes and a drop handler.
- **0.9.101–0.9.135:** added tmux command-window lifecycle, Voyage embeddings/API proxy, role-tiered Qdrant (default write for admitted callers) and database proxies (default read), and verified-TLS SMTP with app-password/Gmail XOAUTH2 authentication. Qdrant remained an operator-run service rather than a bus-managed container. AI gained structured decisions/probabilities; RAG began its separately versioned CRUD/prompt application.
- **IMAP, 0.9.136–0.9.150:** began with read-only EXAMINE/BODY.PEEK, a bounded private UID cache, local list/search/read, newest-first backfill and explicit warned raw-mail publication. Added future-mail subscriptions with a durable bounded outbox and visible paused outcomes.
- **IMAP, 0.9.154–0.9.249:** added live Seen status, read marking by default with `--no-mark`, and explicit mark controls, then folder-specific caches and subscription/IDLE or polling state. Expanded to UID-set flags, server-side search, headers/MIME parts, copy/move/trash/delete, folder operations and raw append, with destructive-operation controls and protected folders. Background publication stayed non-marking; the agent as a whole was no longer read-only.
- **IMAP/database packaging, 0.9.250–0.9.262:** IMAP parts/append gained byte-preserving stored-file input/output, subsequently named bigfile options. The database package used pinned, hash-checked wheels instead of host site-packages.
- **Webhook/fetch, 0.9.137–0.9.157:** added authenticated inbound routes, single-attempt outbound POST and warned public HTTP(S) reads. Fetch later added bounded raw curl responses and per-instance site/URL allowlists, with explicit private grants, per-hop checks, cross-origin credential stripping and uncertain-write outcomes. Webhook emitted strict GitHub/GitLab push/deploy events only to an admitted destination.
- **Git, 0.9.160–0.9.162:** event-driven autopull replaced timers, supporting fast-forward or explicit mirror, revision deployment, startup catch-up and correlated per-checkout results. Administrator-installed post-attempt hooks used trusted selection, bounded processes and separate script-failure records; later startup diagnostics named executable failures.
- **Discord, 0.9.205–0.9.242:** implemented REST/gateway methods and token-redaction controls, then removed the helper from the R1 export/setup/package inventories. Its retained implementation moved to the later-release plan rather than remaining a shipped R1 helper.
- **Slack, 0.9.212–0.9.303:** distinguished starting from ready, learned identity before receiving events and retried temporary startup outages without treating permanent refusals as retryable. Shared support files became readable in the checkout sandbox.

## AI sessions and MCP

- **0.9.0–0.9.100:** Claude used a channel plugin; launchers gained tags, automatic descriptions, crash backoff and tmux startup. MCP handled help asks without involving the model and improved rename/push recovery.
- **0.9.115–0.9.135:** enforced one live session per bus name, reported refused faces in tools, supported verified takeover and separate fork names, and quietly reconnected after daemon restarts.
- **0.9.163–0.9.204:** release launchers refreshed their Claude plugin under a holder-named lock. Named sessions selected their own saved transcript by explicit ID rather than the directory's newest conversation.
- **0.9.212–0.9.245:** `--fresh` kept a name while giving it a new session ID. Codex/OpenCode retained a consumed message across supported compaction recovery and delivered it first after restart; non-recoverable push failures were logged/dropped. New generated names used the configured realm without renaming existing identities.
- **0.9.261–0.9.303:** Codex unattended approval declined file tools and unidentified tools; file-touching MCP operations were not treated as blanket safe approvals.

## Web and user documentation

- **0.9.0–0.9.135:** added machine-readable agent help, compact record/settings views, last-use observations, tag/owner badges, collapsed method cards, token login and self-hosted assets. Configuration gained its own JSON editor, lists used 100-row pages and the project homepage linked code/docs.
- **0.9.151–0.9.198:** improved key-name wrapping and added searchable pagination to Account and Owner User listings.
- **0.9.212–0.9.242:** new record forms followed the configured realm while keeping explicitly entered and existing names exact.

## Distribution, platforms and verification tooling

- **0.9.0–0.9.114:** packaged bundled helpers and notices, added deployment selftests and bounded development cleanup. Repaired the packaged web asset omission: 0.9.101 archives were rejected, and the missing parser shipped at 0.9.103.
- **0.9.163–0.9.189:** compiled web/MCP/launchers into `agent-bus-ts` and built full/client archives, removing Bun/Node requirements from the archive runtime. Third-party notices moved beside LICENSE under `licenses/NOTICE.md`. The initial shell installer/signing workflow verified signed checksums, rechecked privileged copies and refused quiet downgrades or conflicting command files.
- **0.9.212–0.9.242:** npm became the selected distribution path with `ai-agent-bus`/`ai-agent-bus-client` names and the existing `agent-bus` command. Signing was removed in favor of SHA256SUMS plus full manifests; the secondary shell installer kept those integrity checks. Wrapper/manifest/version/build-stamp validation ran before installation/upload, and setup restarted web only for changed runtime inputs.
- **macOS, 0.9.212–0.9.288:** added native client/runner support and reduced non-root launchd setup. Full/client Darwin archives for both CPUs carried matching manifests and Mach-O/source stamps; npm Darwin remained client-only. Native Apple Silicon full-archive acceptance was recorded on 0.9.267, while the tagged workflow later required both Linux builds and both Darwin archive pairs. This did not establish native Intel execution or a public tag run.
- **0.9.190–0.9.194:** container/database fixtures used capped private logs, suppressed host-journal events and identified runs by PID plus start time. Cleanup handled interrupted startup and dead-run sweep; unreadable inventory/journal data failed closed. Fixture startup required the smoke-gate lock before any Podman call.
- **0.9.163–0.9.303:** a digest-pinned container image ran daemon/runner roles from release files; later image builds verified downloaded archives against SHA256SUMS. Release workflows used pinned actions and version tags on main, verified every artifact before upload and could update an existing release's assets.

The early folds recorded fast/normal/slow subsystem selection and mutation checks. Their “under one minute” statement was a gate timing target/evidence, not a runtime guarantee. Later five-distribution Linux RC and native Mac RC results are [separate release-candidate evidence](../R1.0-Release/done/rc-linux-installation.md#candidates-and-order), not changes shipped by a 0.9 patch.
