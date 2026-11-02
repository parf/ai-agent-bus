# DONE — R1

## T-32 Good-agent guide

- The [current guide](../../docs/10-writing-an-agent.md), three worked pages and isolated-bus example shard are built. The five author rules and the event-log owning homes resolve the original gaps; [landed evidence](done/t32-agent-guide.md#landed-evidence) includes Q327's runner checks at normal.

## Small fixes from the range review (E3, K1, K2, N2, J70)

- Landed in 0.9.285 (`8a30dcdd` to `b031259f`): a HEAD request to `/consume` is refused and takes no message (E3); the Go client sends a negative wait or ttl for the daemon to refuse, and a call whose context has ended reports its timeout and sends nothing (K1); the client hub says what the six libraries share (K2); the node page spec names `status.records` (N2); the status example is labelled as adapted. J70 (the user-account runner refresh) closed with `3ef1ae15`. Details are in the [closed issues](../issues/done/2026-11/2026-11-01.md#small-fixes-from-the-range-review-and-earlier).

## T-24 S2v the Rust, PHP and Java clients run the shared vectors

- [The vectors](../../src/testdata/client-conformance/README.md#state-identity-and-refusal-vectors-t-24) for kv (int64 digits, the 512 KiB and 10,000-name edges), locks and holders, identity and refusals by kind run through each client's own calls: `rust/tests/conformance_state.rs`, `php/tests/vectors.php` (required by `run.php`) and a block of sections in `BusTest.java`. Bigfile is not in these three. Each case gets its own record or daemon.
- The vectors found three real differences, each fixed with its own test that was red first: the Rust client asked the daemon `/status` before `/identity`, so a daemon it refused was still asked a request (it now reads `/identity` first); the PHP and Java clients sent a lock ttl over 24 h or a wait over 60 s to the daemon (they now refuse them locally, as Go, Python, TypeScript and Rust do); the Java client's local refusal of a name taking its own lock again did not say to extend it, as the daemon's text does.
- Two things the vectors cannot say in Rust: a negative lock ttl cannot be written as a `Duration`, so the adapter asks for zero, the nearest value; `holders()` returns the daemon's `{record, locks}` object, which the adapter reads. The Rust, PHP and Java docs now quote the warning "daemon too old: protocol is older than this client" and the 500-only rule as the code emits them.

## Task review 2026-11-01

- Applied from the pre-release sweep (`#ab-dir`): RT10, M.6 (M.6a `9871870d`) and B.27 (Slack `62d90c9e`, Telegram B.1) are ✅; RT17, the P.4 pointer and RT30 ([moved to done](done/rt30-tmux-in-the-runner.md)) leave the TODO; the Objective and Next step say what is built and what is left; P.1's remainder is the first publication; missing evidence is filled for RT16, RT21, B.9, B.10 and MAC.1 and B.4 is marked a planning row.
- RW14 (0.8.211, enable or disable future serving apart from starting or stopping now) is built; it left the R1.1 list, where it sat among open rows.

## RT11 web security checks (RW19, RW21, RW25, RW27)

- RW18–RW27 are met; RW4/RW8 moved to [R1.1](../R1.1/TODO.md#runner-web-additions) by the owner ([Q311](DECISIONS.md#recorded-decisions)): [the checklist](runner/web-face.md#minimum-checklist) has each item with its test and mutants. RW19 and RW25 were already built (`a2ee21cc`) and are now recorded; RW21 gained a forged-post test in `src/web/test/runners.test.ts`, a drift check between its fake runner and the real runner's managing verbs, and smoke checks at a real runner; RW27 gained seven mutants, all assertion-red. No hole was found, so no production code changed.

## B1/B2/B3 key proof signs only the bus's namespace, rotates and parses `--key`

- 🟠 B1 `agent-bus token --key` and `agent-bus enrol` signed the namespace and message a node sent back. Result: `keyproof.Sign` has no namespace parameter and always signs the nonce in `agent-bus`; both callers refuse an answer with another namespace, name or a nonce that is not 32 hex digits, naming the member and never echoing it ([proof](../../docs/02-access.md#proving-possession)).
- 🟠 B2 `--rotate` with `--key` was dropped. Result: the proof's credential is used to rotate, and the new one is printed. 🟠 B3 `--key <word>` took the word as the file: now only `--key=<file>` or a bare `--key`, and a word that is an existing file is refused with "--key takes its file as --key=<file>". After review: the rotation is sent to the shared listener, because a user socket ignores a presented token; a namespace that is present but empty, null or not text is refused (only an absent one is the older answer); an explicit empty `--key=` is refused. Mutants are named in the ready note; the node-in-the-middle relay is in [FUTURE](FUTURE.md#key-proof-bound-to-the-node-it-was-made-for).

## J1 MCP bigfile tools fenced and never pre-approved

- 🔴 J1 of the 2026-11-01 range review: `mcp__plugin_agent-bus_agent-bus__*` was pre-approved in `ab-claude`, so `ab_bigfile_put/get` read and wrote model-chosen paths with no prompt. Result: the Claude allow list names every safe tool and omits the two file-touching ones; Codex asks for them by tool; the launcher passes `AGENT_BUS_BIGFILE_DIR` to the face; and [the face works only inside that directory](../../docs/03-records-kv.md#bigfiles) and refuses when it is unset (owner, 2026-11-01, [decision](DECISIONS.md#recorded-decisions)).
- `ab-opencode` has no per-tool list: its server runs `permission: "allow"` for the whole session, so the directory is its only control. The CLI `agent-bus bigfile` is not fenced.
- Proof: `bigfile-fence.test.ts`, `tool-access.test.ts` (finds file-touching tools by schema, so an unlisted new one fails) and the updated big tests in `face.test.ts`; mutants named in the commit report.

## T-24 client compatibility

- Go, Python and TypeScript clients, CLI and MCP check HTTP identity protocol compatibility and surface refusal kinds; [behavior and checks](done/t24-clients.md#checks). Five independent too-new mutants fail on assertions. The server, the contract documentation and the vectors are separate slices.
- T-24 D1 and V1, `0.9.272` `c1a89aa0`: [the stability policy and client contract](../../docs/09-daemon-api.md#stability-and-compatibility) with a table of every route and its class, and [shared vectors](../../src/testdata/client-conformance/README.md#state-identity-and-refusal-vectors-t-24) for key-value, locks, bigfiles, identity and refusals run by the Go, Python and TypeScript clients, plus a check that every registered route has a row. Wiring the Rust, PHP and Java adapters to the vectors (S2v) is team 2's, still open.

## RT31 user-account runner close-out

- Implementation and state handover were already accepted (`c952c241`, 0.9.149; `9905f3ff`, 0.9.151). The 2026-11-01 read-only close-out checks the live runner, both instance records, state paths and retained journal. CEO removed the two disabled standalone unit files; both system and user rechecks list zero.
- [Acceptance map and evidence limits](done/rt31-user-account-runner.md#acceptance-map): no pre-switch baseline was observed. The plain unconfigured instance answers status but has no running session; this is recorded and accepted for RT31. No lifecycle action was performed for the audit.

## Docs rewrite DW11

- Rewrite complete across all 16 pages: three `src/*.md` guides, the client hub, six language READMEs, the examples hub and its five example pages. `d4de8959` enforced the initial batch; PHP joined in `300cc54a` and its final two-identity example landed in `d4b4833c`. The close-out rechecked all 16 against the order/first-fifth checker and ENFORCED list; links and anchors resolve.
- DW11b closes the remaining checkout install/update and final PHP example acceptance; [verified result](#docs-rewrite-dw11b). The original rewrite did not itself run the checkout root commands.

## Docs rewrite DW7

- Landed `dd56e796`, after distribution: setup follows why, what you get, how to start and technical detail, with real read-only CLI examples. npm publication and macOS distribution limits remain explicit; SHA256SUMS, secondary archive installation, checkout deployment, MAC.2 cleanup, J49 replacement and P.3 queries are retained.
- Close-out audit against main: the page passes the order, first-fifth, example and paragraph checks; changed and inbound links resolve. The original rewrite checked 216 inbound Markdown/HTML references. No pending release feature is promoted to built by this status change; no runtime or host change.

## B.32 status close-out

- Bigfiles landed in stages, each its own gated landing (VERSION read at each commit): 2a `90dddc07` (0.9.252, the API and a memory holder, Maintainer-write and allow-list-read rules, the `#bus` record refused), 2b `36d43221` (0.9.253, the file holder, limits, expiry, atomic upload, 503 naming the cause when the directory cannot be prepared), 2d `5d259db5` (0.9.258, an early refusal of a large upload is drained so the client sees its status), 2c `1a95ec19` (0.9.260, IMAP `part` and `append` through a bigfile), T-22 `07a3422c` (0.9.262, called bigfile everywhere a user sees it; the `bigfile/` directory, with a start-up migration of `big-keys/`), T-23 `516fae57` (0.9.263, the index is a table in the daemon's database, start reconciles files with it).
- Checked against main on 2026-11-01: the CLI (`agent-bus bigfile put|get|ls|rm`), the Go, Python and TypeScript clients, the four `ab_bigfile_*` MCP tools, the web store page's "Big files" card and the IMAP `part`/`append` `--bigfile` option (in `help.golden.json`) are all present; the guide is [bigfiles](../../docs/03-records-kv.md#bigfiles). The Rust, PHP and Java clients do not carry bigfile, by decision. The limits, in the plan: 32 MiB per key, 256 MiB and 100 keys per record, 1 GiB per daemon, every key expiring (30 days at most).
- The ⚠️ stages (2a, 2b, 2d, T-22's directory migration, T-23) carried assertion-red mutants; the T-23 `--slow` pass was red only on two checks outside bigfile (the smoke driver test missing `clients/cargo-target.sh`, and a journal-quiet check in the db agent). No code changes in this close-out.
- E1 (bump): a bigfile put no longer dies at the fixed 30 s body deadline. The daemon's body deadline for `PUT /bigfile` is an idle one moved by every read (30 s of silence is `408`) inside a total of 30 s plus the declared length at 64 KiB/s (542 s for 32 MiB); an early refusal's drain keeps its own 2 s bound; every other route keeps 30 s. The CLI's `bigfile put` and `get` use the same budget instead of the 2-minute client timeout. The Go library never had a whole-request timeout; E1b gave the Python and TypeScript clients' bigfile put and get the same budget instead of the Bus timeout (30 s by default), which a TypeScript caller's own `timeout` still overrides. [Guide](../../docs/03-records-kv.md#bigfiles).

## B.30 status close-out

- All five steps of the approved call list landed on main on 2026-10-31, each its own gated landing: step 1 `785fa93c` (0.9.244, `folders`, `--folder` and a cache per folder), completed by `871c71c3` (0.9.245, the search argument and manifest reload fix), step 2 `e2d01c52` (0.9.246, flags, server-side search, headers, MIME parts), step 3 `386d26f2` (0.9.247, copy, move, trash, delete, `compact_trash` behind `allow_destructive`), step 4 `569ca838` (0.9.248, folder create, rename and delete, append), step 5 `ddbf2b61` (0.9.249, `sub add --folder`, per-folder forwarding frontiers and status, read-only IDLE).
- Checked against main on 2026-11-01: the agent's `help.golden.json` lists `folders`, `--folder` on the per-mail calls, `flag`, `headers`, `parts`, `part`, `copy`, `move`, `trash`, `delete`, `compact_trash`, `folder create|rename|delete`, `append` and `sub add --folder`; `src/agents/imap` carries folder, destructive, folder-admin and folder-watch tests beside the earlier ones. Current behaviour is in the [IMAP guide](../../docs/agents/imap.md#what-it-does).
- Q270 was answered by the owner as one shared `cache_bytes` budget, least recently used across folders ([decision](DECISIONS.md#recorded-decisions)). The option `--big` became `--bigfile` later, with [bigfiles](bigfile.md#what-it-is-and-is-not) (T-20 2c, T-22). No code changes in this close-out.

## RT25 status close-out

- Initial ask-first dispatch (0.9.101), RT25a `7b03df8b` (0.9.236) and RT25b `0430a095` (0.9.237) are landed. Every task acceptance line is mapped to its owning test in [the close-out](done/rt25-asks-close-out.md#acceptance-map); the original task detail is retained there. No code changes.

## RT24 status close-out

- Landed as `bfeb5f58` (0.9.224): the settled read-only help/status/version/ls set is complete; management methods remain future scope. Every task acceptance line is mapped to its owning test in [the close-out](done/rt24-bus-close-out.md#acceptance-map); the original task detail is retained there. No code changes.

## B.29 status close-out

- All planned adapters are landed, including cron and `#at` in `823e17c0` (0.9.222). B.29b `9ca154bd` (no bump) closes the last legacy `gone` claim regression: cron and `#at` drop it at restart, warn once and never replay it. Every task acceptance line is mapped to its owning test in [the close-out](done/b29-subscriptions-close-out.md#acceptance-map); the original task detail is retained there. No code changes.

## M.8 status close-out

- Implementation `6945c1a2`, final gate/bump `e952c374` (0.9.228), with M.8b `20769d8f` (0.9.233) and M.8c `44497ffe` (no bump). Every task acceptance line is mapped to its owning test in [the close-out](done/m8-deploy-close-out.md#acceptance-map); the original task detail is retained there. No code changes.

## RT22 Bus setup web acceptance

- The web-form fix landed as `dcf5e419` (0.9.234). The remaining launcher/installer naming acceptance is completed by [RT22b](#rt22b-automatic-names).
- Existing daemon and page work: `cc72fd15`, `bf251737`, `bf748e72`, `549180dd`. The audit found one real gap: web create forms never read the configured realm. They now suggest an editable realm suffix from the current status on each fresh form; submitted names are never expanded or rewritten. Current contract: [bus Configuration](../../docs/agents/bus.md#configuration).

| Acceptance line | Owning checks and result |
|---|---|
| Only the daemon Owner sees and saves the page; a second non-Owner is refused on page and API | `src/web/test/contract.test.ts`, `bus setup`: Owner form and navigation, non-Owner `bob` GET/POST refusal, invalid/valid Owner saves. `src/internal/api/daemon_agent_test.go`, `TestBusSetupOverHTTP`: ordinary and Administrator credentials refused for config read/write; Owner succeeds. `src/internal/core/bus_settings_test.go`, `TestBusSetupDefaultsToTheHostAndOnlyTheDaemonOwnerWritesIt`: ordinary User, Administrator and Owner-owned Agent refused. All pass unchanged. |
| A new record defaults to the configured realm | New contract test `new record forms use the live configured realm without rewriting typed or existing names`: all five record create forms suggest the configured realm, a subsequent save changes the next form's default, and completing the suggested Agent name creates that exact record. Assertion-red on main: expected `#@forms.one`, received empty. Restored fix green (38 assertions). User-realm mutant: expected `#@forms.one`, received `#@test`; one assertion failure, no build error. Launcher/installer paths remain RT22b. |
| An existing name never changes silently | Existing core `TestBusSetupDefaultsToTheHostAndOnlyTheDaemonOwnerWritesIt` looks up the same old name after a realm save. The new web regression also keeps an existing Agent's name, explicit realm-less and foreign-realm registrations, and submitted names (including blank) after a refused form. All pass. |

- Verification: six focused core/API bus-setup tests pass; the whole web suite passes 200/0 (34.24 s), TypeScript typecheck passes, fast smoke passes 1/0 (124 ms). The independent mutant ran only its owning test under the `slow` lock; source restored and lock released. No normal gate or live host change was run by the worker.

## RT22b automatic names

- RT22b completes RT22's remaining automatic naming paths. The reviewed source is `ab-w1b/RT22b` at `7c6018d6`, rebased without code changes onto current main as `ab-w1c/RT22b`; the landing gate remains required.
- Launchers use the exact configured bus realm; explicit names and the existing `AGENT_BUS_REALM` normalization stay unchanged. Bound sessions rebuild their base in the saved realm, including long-title truncation, and resume without a new realm query. New names refuse a missing realm or one that leaves no room within 64 characters.
- The installer reuses a person's one owned runner record under any realm, names and refuses multiple records, and otherwise uses the bus realm with a length refusal before allocation. System and user-account runners have separate checks.
- Checks: `launchers/local.test.ts` covers exact realm spelling, missing/oversize realms, the longest fitting name, bound resume and title changes; `rename-smoke.ts` covers launch and rename call sites against a non-host realm. `cmd/agent-bus-setup/realm_test.go` covers both installer paths. Independent reader checks pass (48 launcher/MCP tests and two installer tests); exact-realm slugging, missing launcher length refusal and installer off-by-one mutants each fail owning assertions, then restored checks pass. Earlier independent call-site and record-reuse mutants also fail assertions. No live deployment.

## M.8 deploy over the bus

- Gate follow-up: the core unknown-call assertion still named the pre-deploy verbs, so both normal gate attempts failed on the correct new refusal. Updated that expectation and added `TestRunnerUnknownCallMatchesOwningSmokeAssertion`, which applies the owning shell grep expression to a real runner reply: assertion-red before the expectation fix, green afterward (0.009 s). No runtime behavior changed.

- Implementation `6945c1a2`, final gate/bump `e952c374` (0.9.228), with M.8b/M.8c follow-ups completed (see [close-out](done/m8-deploy-close-out.md#acceptance-map)): `install [--replace] <agent> [git <url> <full-commit>]` and `remove <agent>`, with direct Owner/Maintainer roles. Current contract: [deployment calls](../../docs/runner/operations.md#deploying-an-agent).
- Sources: only bundled roots or checked HTTPS/SSH URLs with full 40/64-hex pins. Git uses isolated configuration and host-provisioned `runner/.deploy/` credentials, verifies HEAD, removes Git metadata and never returns remote output. The real-Git fixture fetches an older reviewed commit while HEAD points to different code, serves the reviewed result, and refuses a subdirectory-only commit without changing the live tree or inventory pin.
- Publication: private staging under writable `--agents`, root manifest and confinement checks, existing-name refusal without `--replace`, whole-tree swap with rollback on inventory failure. Removal keeps work, tokens and bus records; a bundled name is refused with “disable it instead”. Existing configuration calls remain write-only.
- Proof: fifteen deploy checks pass under the race detector (1.952 s); the whole rebased owning runner package passes under the race detector (31.773 s). The shipped standalone fixture runs the real tmux-sessions packager and loads its copied code without the release tree; restoring copy-only publication compiles and fails its assertion with a missing metadata import, and the restored check passes (0.068 s). Source-selection mutants (short pin, URL userinfo, shared-before-bundled), wrong fetched HEAD, implicit replacement, retained old files, token deletion, nested manifest, escaping link, bundled removal and omitted management gating each compile and fail their named assertion. A mutant that chmods a read-only root also fails the refusal assertion; removing the mode check alone survives because the filesystem still refuses writes, so it earns no credit. Additional checks prove a fresh bundle is served, outside-ACL calls never arrive, invalid inventory/host overrides leave the live package unchanged, and an inventory-write failure restores the previous whole tree. Removing the staged-refusal check or inventory rollback each fails its owning assertion. Fast smoke passes 26/0 after rebasing (8.248 s); links, whitespace and the enforced doc-shape checks pass. Mutants ran under the smoke-gate lock, with the source restored and lock released.

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

### Docs rewrite dw9

- ✅ DW9 Agent manifests: `agent.json` descriptions and method help
  - Result: built in 0.9.190 (`07065bcf`, `9005e89a`) and 0.9.191 (`99b1f94f`), each batch after its normal gate; Discord shipped in 0.9.206 (`4e74e3b6`) with its own fixture. All 22 manifests, 174 methods, lead with outcomes and are pinned by `help.golden.json`; evidence in [batch 9 progress](docs-rewrite.md#batch-9-progress)

### Docs rewrite dw10

- ✅ DW10 Web page intros and CLI help text
  - Result: built in 0.9.189 (`79f03e08`): help pages and page help open with what the command or page is for; owning tests unchanged and green

## P.3 Release-run evidence

- The release workflow verifies every archive in each native build before uploading it. The draft job requires both builds, so verification failure prevents draft creation.
- `src/release-verify.test.sh` executes the actual workflow command against full/client fixtures. An unstamped binary, an altered version and an omitted license each stop that command on the corresponding diagnostic; a wrong-version client also fails beside a valid full archive. Existing notice and manifest checks remain.
- Under the smoke-gate lock, suppressing the workflow command's failure with `|| true` failed the test assertion `unstamped: accepted a bad release`; restored wiring passed. The test also pins verification after packaging and before upload, without a conditional or `continue-on-error`, and the draft dependency on the builds.
- `src/release-verify.sh` queries the compiled TypeScript program (`agent-bus-ts mcp --version` and `launcher claude --version` must answer the release version; the web part has no version query and would start a server) and the SSH forced-command wrapper `agent-bus-token`, where it runs, and reads their bytes elsewhere; the build stamp must be in the TypeScript binary. Before, only the five Go programs were queried and `agent-bus-ts` was grepped for the version string, so a file holding the version in a comment passed. The owning test refuses a comment-only stub, a wrong answer from either part, an unstamped binary, and a stale or missing `agent-bus-token`; going back to grep fails it on an assertion. Launchers (`ab-*`) are shell scripts and are covered by the manifest only.
- Native aarch64, on the owner's Mac (arm64 podman machine, 2026-10-30): the Linux aarch64 full and client archives build in 185 s and `release-verify` prints `ok (… queried)` for both, so the aarch64 binaries ran and passed the version, stamp and license checks; the clean install of the same release passes its 21 checks ([result](done/p3-aarch64-mac-run.md#steps), [PASS list](done/p3-aarch64-mac-run.md#the-passing-runs-result)). Step 3's signed set was dropped by RD178.
- No tag was pushed and no live release was created. Acceptance is met: both distributions' programs are queried against the release source, with a stamp and the license required, and the unstamped, altered-version and no-license mutants each fail their check.

## P.4 Simple install

- `src/acceptance/distro/run.sh` on main `f0064cc7` plus this branch, 2026-10-30: AlmaLinux 10.2, Ubuntu 26.04.1 LTS and Debian 13 each pass in rootless podman. A clean host refuses a tampered archive and a forged `SHA256SUMS` before any file is written; `curl | sh` installs a node that passes `src/check-deploy.sh` (6 checks); `bob` joins with `--client` without root and lists `#debug` through the compiled `agent-bus-mcp`; the same command updates to the next release keeping setup's configuration files, `runner.env`, a record's Configuration, registrations, tokens and one rollback copy; `--rollback` and `--version` go back. Containers and run data are removed afterwards; the journal stays quiet
- Mutants, debian13 each, under the lock: signature check skipped → `the forged release installed`; `agent-bus-ts` dropped from either archive → refused before anything is written (setup for the full node; `install.sh --client` since its completeness check, whose own mutant installs the incomplete archive); upgrade rewriting `runner.env` and a daemon losing Configuration → their configuration assertions. [Earlier installer mutants](done/installer-acceptance.md#checks)
- Runs as smoke check `distro_install` in the slow set only
- Historical pending state superseded by [RD178](DECISIONS.md#recorded-decisions): no owner release keys remain required. Native aarch64 evidence is still tracked in the [Mac run list](mac-run-list.md#a-linux-aarch64); the current install path is npm.

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
- The owner application/server choice remains [Q244](../R1.1/discord.md#open-for-the-owner) before live deployment; this task creates no account, token or live registration.

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
- Evidence: the actual driver's mode and lock checks run against a fake CLI, including missing and mismatched holders and failed authority reads; guard removal, checking after session clearing, exempting a hand-run shard and dropping the driver launch stamp each fail assertions. Restored `go_checks` fast passes in 4.628 s; see [J47](../issues/done/2026-10/2026-10-30.md#gate).

## MAC.2 daemon and reduced setup

Cleanup details below are historical, superseded by [MAC.2e](done/mac2e.md#decision-and-scope), which keeps the verified role account.

- Close-out audit: `f11c9d2a` landed the native reduced setup, orphan-startup refusal and verified cleanup. MAC.2 remains pending for MAC.2b: the written acceptance requires installing a full Darwin daemon archive, while the recorded native run installed focused binaries with `--exec` and main currently produces client-only Darwin archives. The CEO must supply the archive-based Mac proof; the earlier run is not relabelled as that acceptance.

- Native daemon/setup/admin build, launchd identity, private storage, pinned TLS request/reply, registry restart and orphan checks passed on the owner's Apple Silicon Mac. [Evidence and every sudo invocation](done/mac2.md#checks-and-results).
- The fresh-parent permission HIGH is fixed and assertion-proven; first-user administration explicitly stays in the Darwin role's home and socket. Node setup mutants fail on assertions. Admin's installed account/home/socket and credential hint now follow the OS; the native defaults test failed before the fix and the focused native checks pass afterward, without sudo.
- The release archive check remains with MAC.1; deleting the test role over SSH was refused and is an owner item. Runtime files and build caches were removed, the clone is clean on main, and the Mac was handed back.
- Acceptance cleanup now uses `sysadminctl` for the user and `dseditgroup` for the group, checks directory absence independently and fails with named leftovers and Full Disk Access recovery. Local command-stub checks fail on the old cleanup and pass on the fixed harness; the owner-run live proof is recorded below. Historical cleanup is superseded by [MAC.2e](done/mac2e.md#decision-and-scope), which keeps the verified role account.
- Owner-run cleanup passed at 17:23 on 2026-10-30: `sysadminctl` removed both user and group without Full Disk Access; the CEO confirmed both directory entries and the user identity are absent. Stubs cover the group already removed by user deletion and a nonzero group deletion followed by verified absence; both succeed. Historical cleanup is superseded by [MAC.2e](done/mac2e.md#decision-and-scope), which keeps the verified role account.
- Supervisor startup review: the non-Linux child verifies its spawning supervisor's explicit PID before startup, then watches that same parent. Local process fixtures cover parent death before and after watcher startup and prove the inherited listener is released; the setup package runs the cleanup stub regression checks.
- Archive acceptance passed on the Mac, 2026-11-01 15:14 (`rc=0`, 27.059 s, `c49d51f3`, 0.9.267, arm64): the stamped full archive installed through reduced setup, role account reused and kept, group restored by [MAC.2g](done/mac2g.md#scope), nothing left behind. [Result and log](done/mac2.md#archive-acceptance).

## RT33 may-send for another name

Verified 2026-10-30 after B.29: refusal at add, revocation before send, unavailable-authority refusal and the unchanged caller-only API all have owning tests across cron, at, IMAP, Slack, Telegram and logwatch. Three adapter checks and two handler checks close coverage gaps; eight authority-bypass mutants fail on assertions, restored checks pass. No runtime change or live deployment. [Mapping and evidence](done/rt33-may-send.md#acceptance-mapping).

## P.1a unsigned release artifacts

The workflow publishes checked archives and SHA256SUMS without signing; release verification and the deferred shell installer preserve checksum, manifest, version, build-stamp and license refusals. Assertion-red mutants, real archives and the Debian 13 install/update/rollback/MCP acceptance pass. npm clean-install and macOS npm platforms remain P.1b. [Result and evidence](done/p1a-unsigned-release.md#checks).

## MAC.2b full Darwin archive build

Both full and client Darwin archives build for arm64 and x64; per-edition service-program checks, release manifests, stamps/version and Mach-O CPU checks pass. The acceptance harness can install the checked archive without `--exec` and clean the fresh release plus its command links. npm Darwin stays client-only. [Build evidence](done/mac2b.md#checks); [CEO steps](mac2b-run-list.md#run-the-archive-check). **MAC.2 stays pending** until the CEO's archive-based native acceptance and verified uninstall pass.

## MAC.2c archive revision and release set checks

The native archive harness compares every program’s stamped commit with the release-source revision; the build-node command supplies that stamp. Directory verification preserves Darwin client-only sets and requires the complete selected full set when present, matching the checksum collector. [Checks and mutants](done/mac2c.md#checks). MAC.2’s native archive run remains pending.

## Docs rewrite DW11b

The checker extracts INSTALL.md's checkout shell blocks and runs them verbatim in a clean systemd container: install, both services, a real update and restart, then the installed CLI version/build stamp and `hi: ping` call pass (4/0, 27.109 s). Replacing only the document's `git pull` with `true` keeps compilation working and fails the update-marker assertion (one failure/no error, 23.390 s). Every phase is bounded. The PHP README's two identities and exact printed output pass in the normal owning PHP suite (141/0), with T-14 flags and zero journal entries. [Evidence and acceptance map](done/dw11b-examples.md#checkout-container-acceptance). All 16 DW11 pages are complete; runtime/docs commands unchanged, no host root action or bump. J62 stays with #ab-w1b.

## MAC.2d account-only teardown reporting

Historical cleanup is superseded by [MAC.2e](done/mac2e.md#decision-and-scope), which keeps the verified role account.

The archive harness reports a verified `-14120` account-only cleanup partial with exit 3 and the exact owner local-terminal removal/absence check. Other teardown failures remain strict. [Native failed run and local checks](done/mac2d.md#native-run); [owner recovery](mac2b-run-list.md#account-removal-blocked-over-ssh). MAC.2 remains pending the revised run and CEO account-absence confirmation. Historical cleanup is superseded by [MAC.2e](done/mac2e.md#decision-and-scope), which keeps the verified role account.

## P.1c macOS npm platform acceptance

The CEO’s arm64 run on 2026-10-31 passed local npm client/full-wrapper installation, the stamped installed CLI version, one correlated bus call and the expected unsupported-daemon refusal; cleanup was confirmed at 122M. [Acceptance and native log](done/p1c-macos-npm.md#native-acceptance). P.1c is complete; publication and the parent P.1 remain separate.

## M.11 real-host acceptance of the system runner

Every `agent-bus-setup --runner` verb (install, a second install, enable, disable, uninstall, uninstall `--purge`) and a refused non-User ran on a real systemd host, a disposable rootless-podman container on 0.9.262: 38 checks passed, 0 failed, and each verb left the account, unit, template, private directory and shared `agent.d` as [the setup guide](../../docs/07-setup.md#a-persons-runner) states. The bus record and `runner.env`'s mode were read back, not asserted. [Evidence and the unaltered log](done/m11.md#what-each-verb-left); open gaps are in [FUTURE](FUTURE.md#m11-acceptance-gaps). No code change, no bump.


## Agent runtime diagnostics and selection

- Exact method-set mismatches now name the agent, expected/actual methods and missing/extra methods. All bundled manifest selections retain the existing general L rule and explicitly name their reason; [checks and scope](done/agent-runtime-check.md#proof). Both diagnostic and selection mutants fail on assertions. Test/harness only; no bump.

## F2 install validation preserves live overrides

Staging validation no longer changes the live merged files. Apply publishes complete merged output under the reload mutex and recovers interrupted replacements. [Checks and mutant](done/f2-overlay-staging.md#checks).

## J78 runner reload settings snapshots

Inbox polls snapshot the shared-reader setting without locking a blocking consume; delayed JSONL retirement and shutdown capture their deadlines and jails before scheduling their goroutines. The reload/poll race reproduces on main; four compiling owning mutants fail on race reports or a bounded assertion. [Field sweep and proof](done/j78-runner-settings.md#proof).

## Q304 missing-data refusal kind

- Owner YES ([Q304 decision](DECISIONS.md#recorded-decisions)): the daemon emits `not_found` for missing data, including inactive records, missing KV/secrets, absent bigfiles and unheld locks. `Reasons()`, the web diagnostic reason list, CLI/MCP checks and all six clients' kind handling/fixtures use that name. `no_route` remains the separate 404/405 route refusal.
- Protocol stays **1**: protocol 1 was never published. The release changelog must state that this is the pre-publication `unknown` → `not_found` rename, with `no_route` unchanged; a shared PATCH bump accompanies the shipped change.
- Shared vectors now name `not_found`. `api.TestDaemonMissingDataAndRoutesMatchSharedRefusalVectors` compares real daemon lookup/router responses with those vectors. Under a confirmed smoke lock, a compiling mutant emitting `unknown` fails `kind-not_found-404`: `daemon = 404 kind "unknown", shared vector wants 404 kind "not_found"`. Restored, all three cases pass.
- Focused native checks pass: Go protocol/shared refusal vectors; Python protocol/shared refusal tests (3); TypeScript protocol/shared vectors (6); Rust protocol (7); PHP protocol (36 checks); Java protocol (17 checks); full API/CLI Go packages; MCP typecheck and face tests (28), web typecheck and unit tests (34). Rust/PHP/Java adapter logic is unchanged; their in-progress shared-vector wiring remains separate.
- The literal sweep of `docs/` and `src/` leaves no old refusal-kind use. Remaining `unknown` literals describe uncertain delivery outcomes, runtime/status labels, types or invalid-command fixtures. Current API/refusal tables and the captured daemon-status example use `not_found`.
- The per-person bigfile-directory sentence in `src/mcp/README.md` is a separate commit, without plan IDs or setup-completion claims. C1-mcp (`912b5016`) is retained by rebase.

- Gate follow-up: the first normal runs still queried the bare shell reason `count unknown` in `smoke.sh`, missed by the quoted-literal sweep. The core counter block failed 5 passed / 1 failed on a disposable daemon; replacing both queries with `count not_found` and its label passes 6/0. No production change; the follow-up sweep includes unquoted shell counter names.

## F3 oversized JSONL answers

Oversized JSONL lines are drained and answered only to the named request;
unidentifiable output kills its child and explicitly fails that child's pending
calls. Concurrent and later requests remain usable. [Regression and mutants](done/f3-jsonl-answers.md#proof).

## J76 deterministic shared CPU-budget proof

The subscription test uses a fake CPU clock instead of relying on the signal
timer to exhaust an exact budget. Twenty runs under load pass; a fresh-budget
per-pattern mutant fails the named assertion. Production timing is unchanged.
[Cause and proof](../issues/done/2026-11/2026-11-01.md#subscription-filter-test-timing).
