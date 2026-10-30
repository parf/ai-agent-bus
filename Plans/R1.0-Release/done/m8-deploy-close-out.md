# Deploy over the bus

📌 **TL;DR:** Implementation `6945c1a2`; final gate/bump commit `e952c374` (0.9.228), with M.8b `20769d8f` (0.9.233) and M.8c `44497ffe` (no bump).
This records the acceptance on main on 2026-10-30; it is completion evidence.

## Acceptance map

Bare Go filenames below are in `src/cmd/agent-bus-runner/`.

| Acceptance | Test on main |
|---|---|
| Install and serve through runner calls without a host shell | `src/cmd/agent-bus-runner/install_git_test.go:TestDeployFromGitInstallsOnlyTheReviewedRootPackage`; `install_test.go:TestDeployFreshBundleAndFailedChecksKeepHostState`. |
| Configuration writes do not return values; outside ACL and non-manager calls refuse | `secret_read_test.go:TestEnvReadNeverReturnsValues`; `install_test.go:TestDeployCallsRequireDirectManagementAndNeverReturnSecrets`, `TestDeployOutsideTheRunnerACLIsRefusedBeforeDelivery`. Reads expose key names/metadata, never secret values. |
| Bundled name or a full pinned Git commit; refuse shortened/branch/tag pins and URL credentials; host-only credentials and checked HEAD | `install_source_test.go:TestDeploySourcesAreBundledNamesOrFullPinnedGitCommits`, `TestDeployURLsNeverCarryCredentialsOrSelectOtherProtocols`, `TestBundledDeploySelectionNeverUsesSharedOrArbitraryHostCode`; `install_git_test.go:TestDeployFetchChecksHeadAndIsolatesCredentials`. |
| Explicit replacement, whole-tree swap and rollback rather than overlay | `install_test.go:TestDeployReplacementIsExplicitAndReplacesTheWholeTree`, `TestDeployInventoryFailureRestoresThePreviousWholeTree`. |
| Remove keeps data, token and record; bundled removal says disable it instead | `install_test.go:TestDeployRemoveKeepsDataTokenAndBusRecord`, `TestDeployRefusesBundledRemovalInsteadOfExposingItsFallback`. |
| Root manifest, no subdirectory-only package or escaping link; read-only agents directory names its exact refusal | `install_test.go:TestDeployRefusesSubdirectoryPackagesAndEscapingLinks`, `TestDeployRefusesReadOnlyAgentsDirectory`; `install_git_test.go:TestDeployFromGitInstallsOnlyTheReviewedRootPackage`. |
| Git hooks never run during deploy; all defined hook names armed | `install_git_test.go:TestDeployFetchNeverRunsRepositoryHooks` (M.8b guard proof, M.8c complete hook-name loop). |
| Named refusal mutants and management tests | Original [M.8 evidence](../DONE.md#m8-deploy-over-the-bus); M.8b exact-read-only/guard mutants and M.8c hook fixture are in the landed commits above. All required refusal categories map to the preceding tests. |

## Verification

`e952c374` is the actual final landing/bump commit even though its subject names the smoke-refusal fix: it raises VERSION from 0.9.227 to 0.9.228 and includes the changelog. The final gate log `tmp/gate-logs/20261030-184453-ab-w1c_M8.log` reports 1262 passed, 0 failed. The deployment implementation itself is `6945c1a2`.

Each named check and its assertions were inspected on main. This close-out changes no code and adds no test result or mutant claim; the owning implementation evidence records its runs. Changed links and whitespace are checked for this edit.

## Archived task detail

The following is the pre-close-out task text, including its historical pending statements; it is not current status.

- ⏳ M.8 Deploy over the bus
  - Builds: install, configure and remove as calls to the runner ([reaching the runner](../runner-implementation.md#reaching-the-runner)); configuration write-only
  - Code sources settled by owner, 2026-10-30 15:25, A+B: a bundled agent name, or a git URL pinned to a full commit id; a branch, tag or abbreviated revision is refused. The host needs git and a route; a private repository uses a host key, never one sent in the call. Options C and D come later. The calls' bodies remain the implementation contract ([reaching the runner](../runner-implementation.md#reaching-the-runner)).
  - Acceptance: an agent is installed and served with no shell on the host; reading a configuration back is refused; a caller outside the runner's ACL is refused
  - Mutants: shortened pins, URL credentials, wrong fetched HEAD, implicit replacement, old-file overlay, token loss, subdirectory layout, changing a read-only root, escaping symlinks, bundled-name removal and missing management checks each fail an owning assertion.
  - Built on `ab-w1c/M8`, awaiting review and gate: management-gated install/remove, exact pinned Git fetch with isolated credentials, checked root packages, whole-tree replacement, bundled-name removal refusal and preserved work/tokens/records. Focused, real-bus and real-Git fixtures plus assertion-red mutants are in [the evidence](../DONE.md#m8-deploy-over-the-bus).
  - Follow-ups from review (LOW, 2026-10-30): (1) the explicit read-only `--agents` check in `deployRoot` is shadowed by `MkdirTemp`'s error, which carries the same "writable --agents" text: mutating only the check survives, so the operator may not see the settled "read-only agents dir" message in every case; make the check run first and assert its exact wording, and drop the shadow. Worth doing soon. (2) `-c core.hooksPath=/dev/null` on every deploy git call has no test: removing it survives; a fetch brings no hooks and `init.templateDir` is empty, so it is defence in depth, but an assertion that no hook runs would pin it
