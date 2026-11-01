# F2 install validation preserves live overrides

📌 **TL;DR:** A refused package replacement leaves the live agent's merged
settings intact. Validation computes bytes and digests without writing them;
only apply publishes merged files.

## Change

- `loadSources` and `shippedOverlays` return a validated in-memory merge plan.
- Apply holds `reloadMu` and writes a complete private sibling directory.
  Linux `renameat2(RENAME_EXCHANGE)` swaps that directory with the live output
  without removing the bind source; the old tree at the temporary path is
  retained until the next apply, after affected agents have retired. If the
  filesystem answers EINVAL, a two-rename fallback retains a backup through
  retirement and restores it on publication failure. The next apply
  recovers a crash in that fallback and removes interrupted temporary output.
- Recovery logs errors by key, refuses only that key (and its instances if
  the package directory itself is unreadable), and continues recovering and
  loading other agents. It also covers keys now missing or refused. Published
  trees are not swept; temporary output is never adopted.
- Output is under `.merged/<key>/.files`, so publishing an agent and one of
  its instances cannot replace each other's files. It is a generated cache;
  the author's files and the host's overrides retain their paths.
- This is recovery from interrupted apply, not a claim of power-loss durability.

Current behaviour: [settings overrides](../../../docs/runner/configuration.md#overriding-a-settings-file).

## Checks

| Test | Assertion |
|---|---|
| `TestStagedInstallCannotChangeLiveMergedOverrides` | A valid staging load computes its own digest without changing the live bytes; a real refused install dropping a shipped file preserves the live file and SHA-256 |
| `TestOverlayValidationWritesNothingAndApplyReplacesCompleteMerge` | Validation creates no cache; reload publishes changed contents without obsolete files in the new tree; removing all overrides stops mounting the retained cache |
| `TestOverlayApplyDiscardsInterruptedOutputAndRecoversPreviousMerge` | Interrupted temporary output is removed, never served; the previous complete merge is restored even when the new key is refused |
| `TestFailedOverlayBuildLeavesPreviousMergeUntouched` | A failure while building the temporary tree leaves old bytes intact and removes temporary output |
| `TestFailedOverlayPublicationRestoresPreviousMerge` | A refused fallback publication rename restores the old tree and removes temporary output |
| `TestOverlayRecoveryDoesNotSweepPublishedFileNames` | Recovery leaves published settings intact even when the filename resembles temporary output |
| `TestAgentAndInstanceOverlayPublicationStayIndependent` | Publishing the base agent leaves the instance's merged files intact |

Before review, five production mutants ran under the slow lock, each compiling and failing
on its owning assertion (no build failures):

| Mutant | Named assertion |
|---|---|
| Restore `RemoveAll` in staging validation | `TestStagedInstallCannotChangeLiveMergedOverrides`: `valid staging changed live merge` |
| Skip apply's interrupted-output recovery | `TestOverlayApplyDiscardsInterruptedOutputAndRecoversPreviousMerge`: `interrupted publication lost previous merge` |
| Delete live output before building temporary files | `TestFailedOverlayBuildLeavesPreviousMergeUntouched`: `failed merge changed previous files` |
| Sweep inside published output | `TestOverlayRecoveryDoesNotSweepPublishedFileNames`: `recovery swept published settings` |
| Skip rollback after a refused publish rename | `TestFailedOverlayPublicationRestoresPreviousMerge`: `failed publication did not restore previous merge` |

All five sources were restored; all seven focused regression tests passed
(0.017 s), then the lock was released. The pre-review full runner suite passes
with the race detector (`go test -race ./cmd/agent-bus-runner -count=1
-timeout 90s`, 33.902 s). The existing real-sandbox settings
mount test remains in the full runner suite. `go vet` and the Darwin arm64
cross-build pass. No host deployment was performed; this shipped behaviour
fix needs a bump at landing.

## Review delta

The whole-runner recovery failure and the absent bind-source window are fixed.

| Test | Assertion |
|---|---|
| `TestOverlayRecoveryRefusesOnlyTheUnreadableKeyAndContinues` | A mode-000 key is refused by name; a healthy neighbour still loads, updates its merged output and removes stale temporary output |
| `TestOverlayPublicationExchangesDirectoriesWithoutRemovingTheBindSource` | The real Linux exchange swaps both complete trees and never calls a rename that withdraws the live source |
| `TestOverlayPublicationFallsBackOnlyWhenExchangeIsUnsupported` | Injected EINVAL falls back through both renames; EIO refuses and preserves the old tree |

Before the advisory fold, four locked delta mutants compiled and failed on assertions: restoring the
whole-load `list=nil` error (`one recovery error stopped healthy agents`),
skipping exchange (`atomic exchange not used`), removing the EINVAL fallback
(`EINVAL exchange did not fall back`), and restoring staging `RemoveAll`
(`valid staging changed live merge`). Sources restored; all ten focused
regressions pass (0.020 s), then the lock was released.

### Previous cache layout

Pre-F2 caches have merged files directly under `.merged/<key>/`. New loads
never mount those files; they are inert. This fix does not delete them
automatically. They may be removed by hand once no old runner process uses
them. When all overrides are removed, the last private published cache is
likewise retained but no longer mounted.

### Advisory fold checks

The exchange and EINVAL tests also assert that the old tree remains readable
through the current apply, then disappears at the next recovery. The final
full runner race suite passes (33.384 s); vet, Darwin arm64 compile and links
pass. Two additional mutants removing that retention are prepared but not
run: the daemon deployment HOLD arrived before their lock was granted.
No lock is held or pending.
