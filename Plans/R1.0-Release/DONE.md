# DONE — R1

`RR` labels the original unnumbered result rows; existing KV IDs are unchanged.

Completed R1 work, built ahead of the release on the 0.8 line. Remaining work
is in [TODO](TODO.md).

- ✅ B.7 [Server-info](../../src/agents/standalone/server-info/README.md#server-info-agent)
  - Result: all accepted host-fact sections, with a shared standalone systemd starter for №B.2; fixed section calls in text and JSON, root-only sections absent when unprivileged
  - Evidence: `agent_server_info` under `--slow`: real host interfaces, argument refusal, install idempotence, restart and removal after graceful stop or crash; sandbox and argument-forwarding mutants

- ✅ RR1 [Assigned roles](roles.md#role-syntax)
  - Result: Built in 0.8.110; [current contract](../../docs/03-records-agent.md#roles)
  - Evidence: [verification](done/assigned-roles.md#verification)

- ✅ RR2 [Client libraries](done/client-libraries.md#scope)
  - Result: Built in 0.8.100, 2026-10-01: Python, TypeScript, Go, Rust, Java and PHP clients, MIT, each reviewed by another author, with [native examples](../../src/clients/examples/README.md) that call each other
  - Evidence: each client's suite and compiling mutants through the `--slow` client shards; 929/0 at 0.8.101

- ✅ RR3 [Roles are for Agents](roles.md#roles-are-for-agents)
  - Result: Built in 0.8.97–0.8.99, 2026-09-30: a delivered message carries the generated `owner` and `maintainer` roles, worked out at `original_to` and passed through forwards; a script gets them as `AB_ROLE_*`; a role crossing to a record its source's Owner does not answer for fails loudly; an Agent lists the roles it understands. Assigned roles are now built ([contract](../../docs/03-records-agent.md#roles))
  - Evidence: `core` roles_test.go, `agent-bus` TestAChildHoldsOnlyTheRolesItWasHanded under sh and dash, smoke script_done and script_confined roles checks; reviewed by ag-bus and Codex

- ✅ RR4 [Key-value store](done/kv.md#per-record-storage)
  - Result: Built in 0.8.77, 2026-09-30: [contract](../../docs/03-records-kv.md#key-value-store)
  - Evidence: below

- ✅ KV.1 Store and authority
  - Result: String, int and JSON values round-trip, the same name in two kinds staying two values; a write answered before a SIGKILL survives it; an inactive record's store is no such entity and comes back with it; a name removed and registered again starts empty; the Owner, a Maintainer through a group and the record's own Agent use it, a caller on the allow list is refused
  - Evidence: `core` TestKVIsTheRecordsManagersAlone, TestKVStoreIsNoSuchEntityWhileItsRecordIsInactive, TestKVNameRemovedAndRegisteredAgainStartsEmpty, TestKVKindsAreSeparateNamespaces; `sqlite` TestKVRoundTripsAndSurvivesAReopen, TestKVGoesWithItsRecord, TestKVWritesOnlyForAStoredRecord, TestSchemaSixMigratesToKeepValues; smoke `restart` (SIGKILL) and the key-value section. Each mutant — the allow list let in, status ignored, the drop skipped, the stored-record check or the commit removed, the kind ignored on a read — failed an assertion

- ✅ KV.2 `set` modes
  - Result: `add` refuses a present name and `replace` an absent one, each saying so; fifty concurrent `add`s of one name, one success
  - Evidence: `core` TestKVSetModes, TestKVConcurrentAddsHaveOneWinner; mutants treating `add` as `set` or skipping `replace`'s check failed

- ✅ KV.3 Increment and JSON operations
  - Result: 1,000 concurrent increments end at the exact sum; each operation does what its row says; 200 elements shifted by ten workers go out once each; a list with one refused op changes nothing
  - Evidence: `core` TestKVIntIncIsExactUnderConcurrency, TestKVJSONOperationsDoWhatTheirRowSays, TestKVConcurrentShiftsHandEachElementOnce, TestKVJSONListIsAllOrNone, TestKVJSONEqualityIgnoresKeyOrderAndNumberSpelling, TestKVLimits; a read-modify-write outside the store's lock, a partly applied list, text equality and each op's own mutant failed

- B.29 / B.28 step 1a: shared Python subscriptions, language-neutral vectors and runtime inventory wiring completed on `task/eventlog`, pending QA/landing; [verification](agent-events/DONE.md#shared-subscriptions). Eventlog, the TypeScript twin and consumer migrations remain in TODO.

- ✅ P.2 [Container image](../../src/container/README.md#container-image)
  - Result: `docker.io/parf/agent-bus:0.9.179` and `:latest`, digest `sha256:b6b7ebc48d5a9dfad21f868288eac37f749723358b1743c7e28102d0b1a08089`, built from the stamped 0.9.179 release archive, 2026-10-30
  - Evidence: pulled by digest into an empty rootless podman store on this node, `acceptance.sh --image=` passed 10/10: both roles, a request/reply, a recreated daemon keeping the service and both credentials, sandbox refusal. Mutants daemon-entry, runner-entry, storage-outside and sandbox-downgrade each failed on assertions

- ✅ T-11 KV CLI checks live under `src/clients/tests/kv/`, moved from root `examples/` with their README and script retained.
  - Result: smoke invocation and ownership map select the new path; root-guide links and the archived Slack smoke comment are retargeted. No image or asset was removed.
  - Evidence: the relocated check passes twice against a disposable real daemon (82 comparisons each, create then reuse). Under the smoke-gate lock, a compiling integer-increment mutant adds one too much and fails the check's `expected [1], got [2]` assertion; source restored. Link/anchor checks accompany the move; no version bump.

## Locks, runner, notices and eventlog

### J27 lock waiters in arrival order

- ✅ J27 Lock waiters are served in arrival order
  - Result: built in 0.9.185 (`de98f55d`; docs `28456974`, `771dcfb2`), deployed 2026-10-30: a release or expiry hands the lock to the oldest waiter, so a take at the moment of release cannot overtake it. The daemon caps one wait at 60 s ([J18](../issues/2026-10/2026-10-30.md#locks)); the owner settled on 2026-10-30 that 60 s stays and J18 stays settled
  - Evidence: `src/internal/locks/fifo_test.go`: five queued waiters granted in queue order, a take after a release refused naming the waiter, expiry hands off the same way, a waiter that gives up or is cancelled leaves the queue; mutants (last waiter first, a given-up waiter left in the queue, a cancelled waiter keeping a handed lock, wake-all) each fail an assertion

### DR.1 the dedicated runner applies the agent's bus secret

- ✅ DR.1 The dedicated runner applies the agent's bus secret
  - Result: built in 0.9.174 (`70fcfa4a`): `agent-bus run` reads the agent's own bus secret with its token and puts it in the script's environment, sandboxed or not, per message; the secret wins over the shell's value ([№Q174](DECISIONS.md#recorded-decisions)); never in `agent-bus logs`, the process title or an error. The debug agent's `env` answer shows no bus-prefixed secret (0.9.172, `a9c54575`)
  - Evidence: ten mutants (skip the layer, read only at start, leave it out of the sandboxed job's environment, and others) each fail an assertion

### T-10 one notice file

- ✅ T-10 One notice file, `licenses/NOTICE.md`
  - Result: built in 0.9.189: `96f563ad` merges `THIRD-PARTY-NOTICES.md` into it; `adae7926` (npm), `a80bbe78` (`release-verify` refuses an archive without it), `0e7868ee` (agent packagers) ship it beside `LICENSE.md`; `17c2034e` puts it in the distribution acceptance
  - Evidence: mutants (`release.sh` archiving without `licenses`, `package.sh` not requiring it, an agent packager dropping it, `release-verify` without its check) each fail their named test on an assertion

### B.28 eventlog agent

- ✅ B.28 `#eventlog` agent ([design](agent-events/eventlog.md#what-it-does), [evidence](agent-events/DONE.md#eventlog-agent))
  - Result: built in 0.9.183 (`0d60d4ff`), fixed through 0.9.187 (`197daeb5`, `238c508b`, `1082ba3b`), deployed 2026-10-30: named logs with notice, warning and alert rings. The web page is not built

## Docs rewrite

Owner, 2026-10-30: every doc leads with why, then what you get, then real examples, then technical details ([plan](docs-rewrite.md#the-rule)). Rows still open are in [TODO](TODO.md#docs-rewrite).

### Docs rewrite dw0

- ✅ DW0 The docs-order checker with mutants
  - Result: built in 0.9.183 (`2cb73ea2`): `src/agents/docs_order_test.py`: it fails when a page puts details first, drops why from the first fifth, has an empty example block or opens with a jargon term; each mutant fails on an assertion

### Docs rewrite dw1

- ✅ DW1 Agent guides, first batch: ai, db, fetch, git, git-autopull, rag, qdrant, voyage, server-info, systemd
  - Result: built in 0.9.186 (`777b9075`): all ten pass the checker and are enforced (`33d942bc`, then `777b9075`)

### Docs rewrite dw2

- ✅ DW2 Agent guides: smtp, sms, webhook, tmux-sessions, bus, at, README
  - Result: built in 0.9.186 (`0cafa5f0`): all seven pass the checker and are enforced

### Docs rewrite dw3

- ✅ DW3 Agent guides: imap, logwatch, slack, telegram, cron
  - Result: built in 0.9.187 (`cdeb04a7`): each passes the checker and is enforced

### Docs rewrite dw4

- ✅ DW4 Agent guides: debug and eventlog
  - Result: built in 0.9.187 (`41b313eb`): debug is enforced with the batch-3 pages (`cdeb04a7`); the eventlog guide (`41b313eb`) is enforced too

### Docs rewrite dw5

- ✅ DW5 `docs/00`-`05`
  - Result: built in 0.9.189 (`74bc3646`): the twelve core pages are enforced by the checker

### Docs rewrite dw6

- ✅ DW6 `docs/06`-`09` except setup
  - Result: built in 0.9.189 (`d3f2ab22`): pages 06, 08 and 09 are enforced by the checker

### Docs rewrite dw8

- ✅ DW8 `docs/runner/*` and `docs/user/*`
  - Result: built in 0.9.190 (`cded895b`): all twelve pass the checker and are enforced

### Docs rewrite dw10

- ✅ DW10 Web page intros and CLI help text
  - Result: built in 0.9.189 (`79f03e08`): help pages and page help open with what the command or page is for; owning tests unchanged and green

## P.3 Release-run evidence

- The release workflow verifies every archive in each native build before uploading it. The draft job requires both builds, so verification failure prevents draft creation.
- `src/release-verify.test.sh` executes the actual workflow command against full/client fixtures. An unstamped binary, an altered version and an omitted license each stop that command on the corresponding diagnostic; a wrong-version client also fails beside a valid full archive. Existing notice and manifest checks remain.
- Under the smoke-gate lock, suppressing the workflow command's failure with `|| true` failed the test assertion `unstamped: accepted a bad release`; restored wiring passed. The test also pins verification after packaging and before upload, without a conditional or `continue-on-error`, and the draft dependency on the builds.
- No tag was pushed and no live release was created. Native aarch64 evidence remains the owner's Mac check.

## P.4 Simple install

- `src/acceptance/distro/run.sh` on main `f0064cc7` plus this branch, 2026-10-30: AlmaLinux 10.2, Ubuntu 26.04.1 LTS and Debian 13 each pass in rootless podman. A clean host refuses a tampered archive and a forged `SHA256SUMS` before any file is written; `curl | sh` installs a node that passes `src/check-deploy.sh` (6 checks); `bob` joins with `--client` without root and lists `#debug` through the compiled `agent-bus-mcp`; the same command updates to the next release keeping setup's configuration files, `runner.env`, a record's Configuration, registrations, tokens and one rollback copy; `--rollback` and `--version` go back. Containers and run data are removed afterwards; the journal stays quiet
- Mutants, debian13 each, under the lock: signature check skipped → `the forged release installed`; `agent-bus-ts` dropped from either archive → refused before anything is written (setup for the full node; `install.sh --client` since its completeness check, whose own mutant installs the incomplete archive); upgrade rewriting `runner.env` and a daemon losing Configuration → their configuration assertions. [All mutants](distribution.md#install-acceptance)
- Runs as smoke check `distro_install` in the slow set only
- Owner-pending, not met: the RD170 release keys in `install.sh`, and the native aarch64 run on the owner's Mac

## B.29 Logwatch subscriptions

- Logwatch delegates subscription admission, CRUD, matching, visibility, sends and counters to the unchanged shared Python engine. Each matching arrival is sent once, including repeated source/text pairs; no pause, fold, retry or local subscription implementation remains. History grep and file monitoring keep their existing behavior.
- Legacy definitions map into shared rows with their author and valid durable counts. Obsolete pause/disabled fields disappear; a missing creation time records migration time. Bare literal filters are escaped into case-insensitive shared patterns, and unqualified destinations are resolved through the registry. Unsupported filters stay listed and inert; malformed rows remain manager-only repair. Migration is idempotent and replays no history.
- `test_j21_invalid_filter_is_refused_at_add_without_storing` fails on the old code: `/(/` returns an id. The migrated path answers `invalid_filter` with no KV write. Owning checks cover repeated arrivals, one refused copy/one warning followed by a delivered match, author revocation at both target and helper, creator/direct-manager visibility, inert bad filters, migration and shared-module identity.
- `test_delivery.py` exercises the production shared send helper against a private real daemon, including a full destination followed by a delivered match and revoked/restored authors. The standalone lifecycle check retains installed own-token identity, ACL refusal, forwarding and restart coverage. No shared module or frozen vector changed.
- Verification: 30 owning tests pass. Nine compiled adapter mutants fail named assertions: shadow the shared module, ignore the filter, fold repeated lines, pause after refusal, bypass author checks, broaden management, skip migration, skip literal escaping, and skip canonical destination resolution. Folding, pausing and author bypass also fail the real-daemon checks. The installed standalone lifecycle fixture is green; every fixture and mutant batch held smoke-gate.
- Legacy non-12-hex IDs move to deterministic, collision-checked IDs. A checkpoint retains old/new values before the new row commits; only then is the old key deleted. Interrupted writes/deletes resume before serving. A real SIGKILL between new commit and old delete is checked by one listed entry and exactly one counted inbox delivery after restart. `sub edit` is refused with delete/add guidance in both text and JSON; help and its golden fixture agree. Six additional compiled mutants fail assertions: random IDs, ignored collisions, premature old-key deletion, retained old copies after restart, removed edit refusal, and omission of `agents/shared/subscriptions.py` from setup's package inventory. Restored setup packager checks pass; only logwatch's file entry changes in the shared inventories.
- Gate follow-up: add the migration helper to setup's independent `requiredBundleContract`, matching its existing runtime and delivery inventories. The gate's `TestBundleMustBeCompleteBeforeInstallation` failed before this fixture correction; it and both packager-contract checks pass afterward.
- The pre-existing pre-add queue window is [deferred](FUTURE.md#logwatch-subscription-start-boundary), outside this migration's acceptance.

## B.29 Slack and Telegram

- Result: both agents use the unchanged shared TypeScript engine, with one-shot delivery, creator/direct-staff management and sent/dropped counts. Journaled deterministic id migration finishes before serving; `sub edit` directs callers to delete/add. The shared engine and filter worker are bundled in each main artifact.
- Evidence: Slack 135/0; Telegram 101/0; shared TypeScript 43/0 and Python/golden help 12/0. Real-daemon checks cover accepted/full/next delivery (2 sent, 1 dropped), revoked authors and a real SIGKILL between the migration writes, recovering one listed row and exactly one delivery. Both J22 checks failed before the migration and pass after.
- Mutants: 14 focused, compiling assertion failures under the smoke-gate lock; full list and named checks in the commit bodies. Includes the Port's absent `wait` option before SDK normalization, both compiled engines, both package inventory entries and malformed legacy authors retained as damaged definitions. Review, normal gate and deployment remain pending.

## RT25 ask follow-ups

- The documentation follow-up is done: the constitution's self-description, the authoring checklist and the create-agent prompt carry the ask-first rule, and the runner guide holds the shell template (`case "$AGENT_BUS_ASK"`).
- The two unpackaged-script examples (runner getting started, remote access) declare `"asks":"none"` / `--no-asks`: the asks are answered as `unsupported: <ask>` without starting the script, and in the getting-started package the runner answers `help` from its description. Checks: link and docs-order checkers only.

## RT8 Shared client conformance

- `send.json`, `call.json` and `self-call.json` in `src/testdata/client-conformance` are run by all six client adapters (Python, TypeScript and Go, then Rust, PHP and Java) against a disposable daemon, with the fixture names mapped to each daemon's; every adapter walks the expected fields of each case. No schema or generator.
- Mutants, each failing on an assertion under the smoke-gate lock: Rust, PHP and Java with `topic` left out of the emitted send (the send cases; Java and PHP also the call case).

## B.27a Discord

- Discord serves the approved methods through a Bun bundle using the shared TypeScript reply router and subscription engine. The record's Secret supplies the token; fixture URL overrides accept only loopback addresses. Configuration refuses an alias without an id and duplicate resolved ids, naming both keys. Guild checks apply before every action, and posting without a configured default is refused.
- Fake REST and gateway checks cover method requests and results, chronological history, splitting and truncation, own-author edits, reply routing, mentions, tags, DMs, subscription admission and management, counted delivery/refusal/revocation, deadlines and bounded queues. Gateway checks cover intents, heartbeats and missing ACKs, reconnect, resume, duplicate suppression and fatal close codes. No live Discord call was made.
- Private-daemon checks run the installed bundle outside the checkout, use the record token despite a different environment token, execute the guide's read-only CLI example, check help and edit refusal, and require clean shutdown. A full destination drops one copy with one warning; the next match is delivered once. Revoked author rights at the target and helper both prevent a copy. The package includes both license files with the required modes.
- Verification before the final main rebase: 30 owning tests, 251 assertions; package/setup contracts 3/3, manifest golden 1/0, runner metadata checks, docs order and generated index 31/0. The pure fast allowlist runs four tests/12 assertions in about 15 ms; waiting and fixture checks stay normal. Frozen subscription vectors and shared engine source are unchanged by this task.
- Under smoke-gate, 38 compiling/transpiling mutants each failed an owning assertion: unknown Configuration, missing alias id, duplicate channel id, invalid read limit, oversize chunks, external fixture URL, environment token, ignored Retry-After, repeated invalid token, uncertain write retry, missing content intent, identify instead of resume, replayed dispatch, ignored missing ACK, fatal reconnect loop, newest-first history, outside-guild action, guessed default, another author's edit, unsplit post, refused send as success, bypassed author check, broadened list/delete management, missing edit guidance, own/duplicate message forwarding, ignored reply map, ignored caller deadline, unbounded gateway queue, omitted notices, stale golden, missing setup inventory, missing agent export, missing manifest method, oversize API reply, ignored invalid-request burst and repeated 429 retry. Sources were restored; all 30 tests passed afterward.
- Final registered owning shard after the package-inventory rebase: `src/smoke.sh --normal --shard agent_discord`, under smoke-gate, passes 30 tests/251 assertions in 14.475 s (`passed 1, failed 0`). Package contracts and runner metadata pass again; the guide and manifest review URLs answer HTTP 200. The final main rebase keeps the landed J35b scheduling and RT25 documentation.
- The owner application/server choice remains [Q244](messaging/discord.md#open-for-the-owner) before live deployment; this task creates no account, token or live registration.

## M.3 kept jsonl pools

- Source is the fully reviewed `e80f5143` stack: kept processes, per-child in-flight correlation, growth/idle shrink, bounded stdin, retirement/drain, per-child deadline kill and replay refusal. The owner's answer option (a) matches the existing parser; no seam change.
- M.3-only split omits M.5's settings-overlay type/fields, mount/refusal paths and guides. The ConnContext test fixture retains J39; the Waiting==0 workaround is absent. The M.5 test-helper refactor is deferred with its sibling test.
- Focused runner package on this split: `go test -race ./cmd/agent-bus-runner -count=1 -timeout=120s -v`, 146 top-level passes, no skips, 29.7 s. Both changed runner guides pass docs-order.
- Original owning mutants and reviews remain with the reviewed source; this split receives its own non-author read. The rebased combined runner/ports/sandbox source is byte-identical to `e80f5143`.

## M.5 host controls and settings overrides

- Stacked after M.3: sibling no-SIGHUP proof with a second registered agent/token; shipped JSON merge patches, read-only overlay mounts, digest/restart, and bus read/write/delete. Invalid/null patches, unknown files and unsupported sandbox modes fail closed by file name.
- Combined runner, ports/sandbox runtime and tests are byte-identical to fully reviewed `e80f5143` after its rebase; no source fixes or new answer-line seam. Existing readers and owning mutants apply to that exact source.
- Focused `go test -race ./cmd/agent-bus-runner ./internal/sandbox -count=1 -timeout=120s -v`: 154 top-level passes, no skips (runner29.758s; sandbox1.055s). All three runner guides pass docs-order. Lock released, no normal gate or bump by the worker.
- Runner-level malformed and unknown-id line tests remain J50; they are deliberately not folded into this byte-identical landing.

## M.12 Agent directories

- The runner loads shared packages before bundles; the first name wins. Bundles require explicit enablement, expose their selected package path, and include standalone manifests. Reload discovers new bundles and falls back when a shared package is removed. [Current contract and approved spellings](../../docs/runner/configuration.md#agent-directories).
- Accepted spellings (#ab-dir, 2026-10-30, within Q175): `--bundled=<directory>`, per-row `agent_dir`, `state:"available"`, explicit `enabled:true`, and the `standalone/` manifest scan. Defaults follow the release/current path or the git checkout, resolving launcher links before checkout detection.
- Evidence: actual request/reply and author-file reads from both sources, bundled enable/disable, shared replacement/removal, upgrade discovery, standalone discovery, instance overrides and launcher defaults. Full runner package passes under `-race`; both owning guides pass docs-order. Eight compiling mutants fail on assertions: omit bundles, reverse precedence, auto-enable bundles, skip standalone, use the wrong shipped-override root, use a release default for git, use shared code paths for bundles, and read author files only from shared packages. Sources restored; no live deployment.
- Existing release build already calls `agents/package.sh`, including Slack's package builder; no build or installer inventory change is needed.

- Sandbox review repair: an enabled real checkout cron runs its actual Python entry point inside the default bubblewrap sandbox on a copied git tree outside `/usr`. Its native help proves startup imports and metadata access without a live send. Restoring the original keeper mount list, or dropping the shared-engine bind, fails on `ModuleNotFoundError: cronexpr` at an assertion. Git bundles alone gain read-only `src/agents/shared`, `src/clients/python`, `src/clients/typescript` and `src/internal/version`; the whole checkout and host settings remain unmounted. Scope mutants fail if shared or release packages inherit these roots. All three launch forms use the same read-list snapshot.
- The source-fallback check now asserts the serving snapshot's package directory too. It failed before retiring/re-adopting a changed source; restoring that stale snapshot compiles and fails the same assertion. Empty `--bundled=` is refused, with a refusal mutant; an emptied shared directory's refusal/shadowing is documented.

## J47 smoke driver lock

- Result: normal and slow drivers refuse before starting work unless their original caller holds `smoke-gate@parf.us`'s `slow` lock; the gate's `parf` identity remains accepted. Hand-run normal/slow shards require the lock; only marked children inherit a checked driver run, and their marker is consumed.
- Evidence: the actual driver's mode and lock checks run against a fake CLI, including missing and mismatched holders and failed authority reads; guard removal, checking after session clearing, exempting a hand-run shard and dropping the driver launch stamp each fail assertions. Restored `go_checks` fast passes in 4.628 s; see [J47](../issues/2026-10/2026-10-30.md#gate).

## MAC.2 daemon and reduced setup

- Native daemon/setup/admin build, launchd identity, private storage, pinned TLS request/reply, registry restart and orphan checks passed on the owner's Apple Silicon Mac. [Evidence and every sudo invocation](done/mac2.md#checks-and-results).
- The fresh-parent permission HIGH is fixed and assertion-proven; first-user administration explicitly stays in the Darwin role's home and socket. Node setup mutants fail on assertions.
- The release archive check remains with MAC.1; deleting the test role over SSH was refused and is an owner item. Runtime files and build caches were removed, the clone is clean on main, and the Mac was handed back.
