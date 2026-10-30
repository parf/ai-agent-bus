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
