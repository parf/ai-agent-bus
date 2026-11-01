# F2 install validation preserves live overrides

📌 **TL;DR:** A refused package replacement leaves the live agent's merged
settings intact. Validation computes bytes and digests without writing them;
only apply publishes merged files.

## Change

- `loadSources` and `shippedOverlays` return a validated in-memory merge plan.
- Apply holds `reloadMu`, writes a complete private sibling directory, then
  renames the old output to a backup and the completed output into place.
  A failed publication restores the backup; the next apply recovers a crash
  between the renames and removes interrupted temporary output, even for a
  refused or removed key. Temporary output is never adopted.
- Output is under `.merged/<key>/.files`, so publishing an agent and one of
  its instances cannot replace each other's files. It is a generated cache;
  the author's files and the host's overrides retain their paths.
- This is recovery from interrupted apply, not a claim of power-loss durability.

Current behaviour: [settings overrides](../../../docs/runner/configuration.md#overriding-a-settings-file).

## Checks

| Test | Assertion |
|---|---|
| `TestStagedInstallCannotChangeLiveMergedOverrides` | A valid staging load computes its own digest without changing the live bytes; a real refused install dropping a shipped file preserves the live file and SHA-256 |
| `TestOverlayValidationWritesNothingAndApplyReplacesCompleteMerge` | Validation creates no cache; reload publishes changed contents and removes obsolete merged files |
| `TestOverlayApplyDiscardsInterruptedOutputAndRecoversPreviousMerge` | Interrupted temporary output is removed, never served; the previous complete merge is restored even when the new key is refused |
| `TestFailedOverlayBuildLeavesPreviousMergeUntouched` | A failure while building the temporary tree leaves old bytes intact and removes temporary output |
| `TestAgentAndInstanceOverlayPublicationStayIndependent` | Publishing the base agent leaves the instance's merged files intact |

Mutation and final runner-suite results are recorded after the locked proof.
