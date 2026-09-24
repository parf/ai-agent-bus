# TODO MVP

📌 **TL;DR:** Finish accepted behavior and the remaining browser and runtime
journeys on the installed release. Built results live in DONE, the 0.7
constitution work is tracked in its own plan, and the task IDs and acceptance
below stay in force.

## Objective

Finish the [MVP scope](README.md#scope). Built wave results are in [DONE](DONE.md#done--mvp); historical tasks retain their IDs in the [archived plan](done/TODO-before-rewrite.md#mvp-plan-before-the-documentation-rewrite).

## Next step

The [0.7 constitution work](0.7.0-TODO.md#next-step) is stable at 0.7.20; the
0.8 line continues the rows below, whose acceptance and task IDs remain in force.

H.1 and H.1.1 have [fresh-install](done/fresh-install.md#checks) and [populated-upgrade](done/upgrade-recovery.md#checks) evidence. H.9.5 has [Codex/OpenCode](done/runtime-interactive.md#checks) and [Claude channel](done/runtime-interactive.md#claude-channel-checks) evidence. F.12 has [installed browser evidence](done/installed-browser-acceptance.md#checks). The installed runtimes passed on a [fresh host](done/fresh-host-runtime.md#checks). H.5.3 has [administrative crash-recovery evidence](done/administrative-durability.md#checks). H.5.2 has [real-SSH evidence](done/ssh-onboarding.md#checks); G.1.2 and G.1.3 have installed [resource](done/web-resources.md#checks) and [authority-isolation](done/web-isolation.md#checks) evidence. The [review evidence](done/release-gap-review.md#findings) distinguishes reproduced failures from unverified risks. Choices are tracked in [QUESTIONS](QUESTIONS.md#open-questions).

The owner reviewed the web design through rendered pages and iterative
corrections. Remaining [web work](#web-redesign) is acceptance, not a design
approval gate.

Accepted feature work remains in [authority](#authority-model).
This section is unfinished MVP work, not optional follow-up.

## Remaining work

The required dashboard has [implementation and mutation evidence](done/owner-controls.md#verification); its [resource confinement](done/web-resources.md#checks) is built and exercised in a disposable generated unit.

H.8, H.9 and H.9.1–H.9.3 passed live on the development host ([delivery](done/runtime-delivery.md#checks), [launchers](done/runtime-launch.md#checks), [MCP minimum](done/mcp-minimum.md#checks)). H.9.4’s OpenCode launcher and adapter are also [built](../../docs/08-runner-role.md#smart-launchers); its [live acceptance](DONE.md#done--mvp) passed 2026-09-23. A built component does not close its row.

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
Known upgrade hazard, not a row: a 0.7.<20 development database whose user record carries a list is ignored with its User's records.

## Web redesign

The [specifications](web/README.md#what-each-document-owns) supersede parts of
the [earlier proposal](web-interfaces.md#proposal); the [audit](done/web-review.md#findings)
records observed shortcomings. F.13.0 is complete through the owner's rendered
page selection and iterative browser corrections
([evidence](done/web-design-owner-review.md#evidence)). F.13.1–F.13.7 are
[done](DONE.md#done--mvp); no redesign row remains open, and the installed
rerun of the redesigned journeys, [F.12](DONE.md#done--mvp), is done. The owner's
preference remains compact, plain administration rather than decorative polish.

Local mapping administration is the operator key or a user token ([administering the account map](../../docs/09-setup.md#administering-the-account-map)); it is not hidden inside a new Settings page. [G.1.2 resource limits](done/web-resources.md#checks) are complete. UI development can use an isolated populated daemon while installed isolation work proceeds separately.

## Installed stage gate

The package, setup and supervisor are built. Continue the remaining checks on isolated systemd hosts; development checks alone do not close installed runtime or browser gates.

The service-account, per-user-socket and capability rows are complete on a
[package-only real-systemd host](done/installed-shared-host.md#checks). The
remaining rows concern broader operational and runtime/browser acceptance.

| Gate | Dependencies | Required evidence and mutation |
|---|---|---|
| Operational acceptance | H.5.2–H.5.3, H.9.5–H.9.6, G.1.2–G.1.3; [F.12 complete](done/installed-browser-acceptance.md#checks); [H.1.1 complete](done/upgrade-recovery.md#checks) | Retain each task's installed evidence and named mutation failure; fixture-only results cannot close an installed or live-runtime requirement |
| Fresh installed release | [H.1 package exercise complete](done/fresh-install.md#checks); [F.12 browser complete](done/installed-browser-acceptance.md#checks); [runtime acceptance complete](done/fresh-host-runtime.md#checks) (H.8, H.9–H.9.6); operational acceptance remains | Closes with operational acceptance; no browser or runtime run remains on the fresh host |

The installed gates do not replace feature acceptance. Before closing MVP, also complete the accepted feature sections below and the remaining F.13 acceptance. Keep deferred decisions out of that gate: Q69 hardening and broader transfer-recipient eligibility were deferred, while Q63 and Administrator control of ordinary group membership confirm existing behavior ([decisions](../../docs/decisions.md#settled)).

## Default service access

Completed in 0.5.44; see [implementation and checks](done/empty-acl.md#checks).
The anchor stays for existing references. Personal tagging and the authority
changes below remain separate work.

## Personal agents

Completed in 0.5.50–0.5.51; see [core checks](done/personal-services-core.md#checks)
and [web checks](done/personal-services-web.md#checks). The owner classification,
assignment limits, dedicated owner view and main-list exclusion are built without
changing ordinary authorization or delivery.

## Inbox selection and filters

Completed in 0.5.52; see [inbox-selection checks](done/inbox-selection.md#checks).
The API, CLI and MCP face select an inbox explicitly while topic and tag remain
filters; omission selects the caller's own inbox. The former topic-address
overload and its spelling-dependent 404 are gone.

## Reader visibility

Completed in 0.5.53; see [reader visibility checks](done/reader-visibility.md#checks).
WEB, human CLI and MCP render one count across filtered and unfiltered waits;
the compatibility boolean remains wire-only. The anchor stays for existing
references.

## Identity display labels

Completed in 0.5.54; see [identity display label checks](done/identity-display-labels.md#checks).
WEB and human CLI derive User, Agent and Service labels from daemon facts;
machine values and editable syntax remain plain. The anchor stays for existing
references.

## Questions

The 0.7 [open questions](QUESTIONS.md#open-questions) name the rows they block;
the storage rows are unblocked.
Implementation gaps are not reopened policy questions. The [Future storage
proposal](../Future/storage.md#storage) is not a remaining MVP database requirement.

## Authority model

Built: the [authority specification](../../docs/01-identity-and-roles.md#role-names-and-scopes) has no pending change, and the historical fixture cleanup is [done](DONE.md#done--mvp). Each dependent web control is verified against it in [F.12](done/installed-browser-acceptance.md#checks)'s installed browser acceptance.
