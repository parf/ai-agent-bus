# Clean R1 input verification

📌 **TL;DR:** The [canonical-input cleanup](../legacy-removal.md#implementation-and-checks) passed its integrated normal gate. This records the focused mutation evidence before temporary worktrees and logs are removed. It is local verification, not deployment or publication.

## Mutation runs

The tables record **45 credited focused mutation runs during implementation**. Each reached an owning assertion, production bytes were restored, and the owning check passed afterward. Rows describe their implementation-stage revisions; they do not claim that every earlier single-line mutant still defeats additional guards added later. The final normal gate verifies the integrated runtime tree.

Go mutations compiled through `go test`. The parent web mutations also passed TypeScript checking before their assertion runs. Other Bun mutations executed successfully through their test entrypoints and failed assertions; separate mutant TSC runs are not claimed. Missing-dependency, import-error and host-TLS-lookup attempts were discarded and rerun correctly, with no credit for those attempts.

All runs held `smoke-gate@parf.us` with a 15-minute lease and released it at the batch boundary. Fixtures used private daemons, fake providers or private filesystem paths; no live deployment was performed.

### Registry, credentials and startup

| Broken protection | Owning check | Assertion-red evidence |
|---|---|---|
| Fresh local session credential grant omitted | `TestFreshLocalPrincipalSessionAuthenticatesWithoutPriorTokenRequest` | Status returns 401 instead of authenticated success |
| Stored JSON duplicate guard omitted | `TestStoredRecordJSONRefusesUnsupportedMembers/duplicate` | Identical duplicate members are accepted |
| Stored credential pair check omitted | `TestStoredCredentialIDsAreRequiredWithoutConversion` | Invalid IDs are accepted |
| Whole-snapshot validation omitted | `TestRestoreCanonicalStateOrRefuseBeforeMutation` | Ten unsupported snapshots are accepted |
| SQLite schema refusal omitted | `TestRetiredSchemaFourRefusedWithoutMigration` | Retired schema 4 is accepted |
| Fresh account-map seed validation omitted | `TestSupervisorRejectsUnknownSeedWithoutEstablishingState` | Unknown initial mapping is accepted |
| Supervisor fatal-alert call omitted | `corrupt_pair` owning smoke shard | Expected startup alert is absent |

The first five are recorded in `8753d96c`; startup and alert proof is recorded in `bd011b4d`. Restored owning shards pass core 623/0, supervisor 9/0, corrupt-pair 16/0 and stored-state refusal 24/0.

### Web inputs and typed maps

| Broken protection | Owning check | Assertion-red evidence |
|---|---|---|
| Personal boolean accepted | `only the personal tag selects the Personal classification` | Retired record field no longer refuses |
| Retired query/form guard omitted | `retired query and form fields refuse instead of changing or clearing a filter` | Unsupported fields no longer refuse |
| Retired channel route restored | `retired channel routes do not redirect or invoke a compatibility handler` | Removed route becomes known |
| HTTP canonical-input call omitted | `the HTTP entry point refuses retired fields before a handler can act` | Request no longer returns the required refusal |
| Boolean reader fallback restored | `a retired reading flag never triggers an agent status call` | Retired flag triggers a status request |
| Retired environment-variable guard omitted | `web configuration has one installed socket and one TLS directory form` | Unsupported environment variable is accepted |
| Version accepted among other arguments | `version is the sole web argument and cannot bypass invalid options` | Mixed version arguments are accepted |
| Unsupported subscription ID skipped | `unsupported subscription ids refuse the whole reload without writes` | Reload resolves instead of refusing |
| Non-bus configure action accepted | `the bus configure action refuses other records without changing Configuration` | Action succeeds instead of returning 400 |
| Retired users kind filter accepted | `the retired users kind filter refuses` | Page returns success instead of 400 |
| Typed-map duplicate guard omitted | `TestTypedMapKeysAreUnique` | `duplicate host entry accepted` |

The early web batch restored six owning checks. The later batch restored all five targeted checks; the combined focused web run passed 153/0, including 110 disposable-daemon contract cases. The typed-map check passed after restoration.

### Setup, protocol and agent state

| Broken protection | Owning check | Assertion-red evidence |
|---|---|---|
| Recovery journal duplicates collapsed through a map | `TestUpgradeJournalRejectsObsoletePayload` | Duplicate `unit_sha256`: `unsupported journal accepted` |
| Actual release switch skips layout validation | `TestReleaseSwitchRequiresCurrentLayout` | `refused switch changed current: "releases/003"` |
| Python exact protocol pair omitted | `test_protocol.Protocol.test_handshake` | Expected BusError is absent for 0/0, 2/1 and 1/0 |
| TypeScript exact protocol pair omitted | `protocol handshake rejects too-new and malformed before any application request` | Promise resolves instead of rejecting |
| MCP accepts `-version` | `MCP accepts only sole canonical info flags before loading session state` | Exit 0 instead of 2 |
| npm allows `-h` to bypass setup guards | `check.CanonicalLaunch.test_only_sole_canonical_info_flags_bypass_setup_guard` | Spawn count 1 instead of 0 |
| Scheduler stored-field check omitted | `test_scheduler_state.State.test_noncanonical_state_refuses_before_any_recovery_or_write` | `paused` and `migrated_from` no longer raise ValueError |
| Logwatch subscription-state validation omitted | `test_logwatch.WatcherTests.test_startup_rejects_noncanonical_state_without_writes` | Three unsupported-state cases no longer raise ValueError |
| IMAP state validation omitted | `test_forwarding.UnsupportedStateTests.test_unsupported_state_is_unchanged_and_never_delivered` | Five unsupported-state cases no longer raise ValueError |
| Rollback target preflight omitted | `TestRollbackRefusesInvalidTargetBeforeEffects` | Invalid targets change current or run service commands |
| Setup positional refusal omitted | `TestSetupRejectsPositionalsBeforeActions` | `--print-unit --tls off junk` reaches the action |
| Missing macOS role group accepted | `TestDarwinExistingRoleRequiresConsistentGroupWithoutMutation` | Missing/unavailable group returns no error |
| Launcher `-V` acceptance restored | `launcher version accepts the canonical flag and refuses its retired alias` | Exit 0 instead of 1 |
| Sample group fields use uppercase names | `TestSamplesPreserveRealGroupsAndUsers` | Noncanonical Members field; group receives HTTP 400 |
| Runner discovery restores unsupported q parameter | `TestPersonRunnerNameFollowsTheRecordOrTheBusRealm` | Runner listing receives HTTP 400 instead of finding the realm |

Every restored owning test passed. The final positional regression explicitly disables host TLS; the earlier host-lookup failure is not counted.

### CLI, runner and sandbox

| Broken protection | Owning check | Assertion-red evidence |
|---|---|---|
| Run stdin uses ordinary JSON decoder | `TestRunJSONUsesExactUniqueFields` | Noncanonical Name member is accepted |
| Inventory uses ordinary JSON decoder | `TestRunnerInventoryUsesExactUniqueFields` | Noncanonical Enabled member is accepted |
| Retired template dispatch restored | `TestRetiredAliasesRefusedBeforeDaemonCalls` | Alias reaches config handling instead of unknown-command refusal |
| Log command infers Agent prefix | `TestLogsRefusePlainAgentName` | Plain name succeeds and prints the fake answer |
| Token accepts bare key flag | `TestKeyRequiresAValueWithEitherLexicalForm` | Missing key value is accepted |
| CLI accepts arguments beside help | `TestOverviewHelpTakesNoExtraArguments` | Help plus junk exits 0 |
| Runner status-row validation omitted | `unsupported runner rows cannot expose agent controls` | Unsupported status renders controls instead of a refusal |
| Runner accepts arguments beside help | `TestRunnerHelpAndRealExitCodes` | Help plus junk exits 0 |
| IMAP generation lock omitted | `TestImapGenerationLockInsideTheRunnerSandbox` | Concurrent generation is admitted |
| Obsolete pending body accepted | `TestImapGenerationLockInsideTheRunnerSandbox` | Pending body is adopted |
| Go builtin short-help refusal omitted | `TestCanonicalHelpHasNoBuiltinShortAliasOrTrailingArguments` | `--h` returns flag.ErrHelp |
| Shared mixed-help refusal omitted | `TestCanonicalHelpHasNoBuiltinShortAliasOrTrailingArguments` | Mixed help arguments return flag.ErrHelp |

All restored checks passed. The runner-row restored check passed 13 assertions; the full runner-page fixture passed 25/0 before the final integrated gate.

## Final integration and static checks

The [integration review](../../issues/2026-11/2026-11-04.md) records found defects and fixes. The [cleanup ledger](../../reviews/doc-cleanup-review.md#final-verification) owns final runtime, links, assets and executed-guide coverage. Temporary raw logs are not a remaining acceptance dependency.
