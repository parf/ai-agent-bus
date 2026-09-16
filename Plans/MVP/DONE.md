# DONE — MVP

Completed implementation and review evidence. Remaining requirements and stage
gates are in [TODO](TODO.md#objective).

| Work | Result | Evidence |
|---|---|---|
| H.5.5, H.5.4 | A start deletes every record whose owner it knows nothing about, to a fixed point, and then drops the credentials that answered for them; the interim guard between the two sweeps is gone. Group membership now goes with a deleted name, which it did not | [orphan services](done/orphan-services.md#checks) |
| H.5.7 | A paused or banned user's services refuse calls in the daemon, on the called name rather than on who is asking; nothing is destroyed and the state lifts cleanly | [suspended owner](done/suspended-owner.md#checks) |
| H.5.6 | The group deletion verb is gone from the API and the dashboard, not only the page; a group is retired by emptying its membership | [group retirement](done/group-retirement.md#checks) |
| H.5.9 | `agent-bus-admin user add` creates the user it adds, so a fresh install can onboard somebody; an unreachable daemon refuses rather than leaving a key that works before the name exists | [user add evidence](done/user-add-provisions.md#checks) |
| F.13.5 partial | Referenced receipt correlation and qualified response links built; overview, activity and installed acceptance remain open | [exchange evidence](done/exchange-evidence.md#checks) |
| H.5.8 | The caller, its state and its authority over the target are settled under the hold the operation writes under; issuing and removal each became one held operation | [gate window](done/gate-window.md#scope) |
| Access review and smoke migration | Explicit fixture provisioning; initial unknown-principal gates checked; concurrent creation and issuance gaps reported | [access evidence](done/access-review.md#scope) |
| F.13.1/F.13.4 partial | The directory slice, before the meanings pass: user/other-identity classification, bounded directory, explicit credential cleanup and session revocation built; remaining journeys and installed acceptance stay open | [directory evidence](done/identity-cleanup.md#verification) |
| F.13.1 | Declared state, observed state and health are three different things on every page, and a value the daemon did not report is not guessed | [meanings evidence](done/web-meanings.md#what-proves-it) |
| F.6, F.6.1 and F.7–F.11 | Required dashboard tabs, user administration, protected maintainers, owner controls and activity graphs built | [owner-control evidence](done/owner-controls.md#verification) |
| A | Call, receipt, deadline and shared-reader behavior built | [wave evidence](done/wave-evidence.md#done--mvp) |
| B and C | Principal credentials, ownership, ACL and subscriptions built | [wave evidence](done/wave-evidence.md#done--mvp) |
| E | Restart persistence built | [wave evidence](done/wave-evidence.md#done--mvp) |
| F.1, F.3–F.5; F.6 partial | Filtered faces, dashboard and existing views built | [wave evidence](done/wave-evidence.md#done--mvp) |
| G | Process split and foreground service controls built | [wave evidence](done/wave-evidence.md#done--mvp) |
| H.2–H.7 | Setup, administration and token programs built | [wave evidence](done/wave-evidence.md#done--mvp) |
| H.8 and H.9–H.9.3 implementation | Runtime launchers and bundled MCP wiring built; live and installed gates remain | [launcher evidence](done/launcher-implementation.md#scope) |
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
