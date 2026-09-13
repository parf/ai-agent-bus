# DONE — MVP

Completed implementation and review evidence. Remaining requirements and stage
gates are in [TODO](TODO.md#objective).

| Work | Result | Evidence |
|---|---|---|
| A | Call, receipt, deadline and shared-reader behavior built | [wave evidence](done/wave-evidence.md#done--mvp) |
| B and C | Principal credentials, ownership, ACL and subscriptions built | [wave evidence](done/wave-evidence.md#done--mvp) |
| E | Restart persistence built | [wave evidence](done/wave-evidence.md#done--mvp) |
| F.1, F.3–F.5; F.6 partial | Filtered faces, dashboard and existing views built | [wave evidence](done/wave-evidence.md#done--mvp) |
| G | Process split and foreground service controls built | [wave evidence](done/wave-evidence.md#done--mvp) |
| H.2–H.7 | Setup, administration and token programs built | [wave evidence](done/wave-evidence.md#done--mvp) |
| Shared version and build information | Program versions and process titles verified | [version checks](#version-and-build-checks) |
| Documentation migration | Current MVP separated from future plans; history and open questions retained | [migration record](done/document-migration.md#scope) |

## Version and build checks

The earlier full slow smoke passed 491 checks, with TypeScript typechecking and
named mutation failures recorded in [version evidence](done/wave-evidence.md#version-and-build-checks).
This documentation rewrite did not rerun runtime checks.

## What the reviews and the mutants caught

Detailed findings are archived in [mutation evidence](done/wave-evidence.md#what-the-reviews-and-the-mutants-caught).

## Nine harness traps

Historical harness lessons remain in [harness evidence](done/wave-evidence.md#nine-harness-traps).
