# TODO MVP

📌 **TL;DR:** MVP is complete (2026-09-23, 0.8.30): every task row and both
installed stage gates passed, with their evidence in [DONE](DONE.md#done--mvp).
What remains in this file is the record of how the gates close; follow-up ideas
live in [FUTURE](FUTURE.md) and later releases in their own plans.

## Objective

Finish the [MVP scope](README.md#scope). Built wave results are in [DONE](DONE.md#done--mvp); historical tasks retain their IDs in the [archived plan](done/TODO-before-rewrite.md#mvp-plan-before-the-documentation-rewrite).

## Next step

[Constitution conformance](#constitution-conformance): the 2026-09-24 review's
fixes. Otherwise none for MVP. The [0.7 constitution work](0.7.0-TODO.md#next-step) is stable
at 0.7.20 and MVP closed on the 0.8 line; the next stage is [R1](../R1.0-Release/README.md#scope).

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

The [specifications](web-handoff/README.md#what-each-document-owns) supersede parts of
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
| Operational acceptance | Complete 2026-09-23: H.5.2–H.5.3, H.9.5–H.9.6, G.1.2–G.1.3, F.12 and H.1.1 are done with installed evidence and named mutation failures | [DONE](DONE.md#done--mvp) |
| Fresh installed release | Complete 2026-09-23: [H.1 package](done/fresh-install.md#checks), [F.12 browser](done/installed-browser-acceptance.md#checks) and [runtime acceptance](done/fresh-host-runtime.md#checks) (H.8, H.9–H.9.6) on a host without `/rd` or the checkout | [fresh-host runtime](done/fresh-host-runtime.md#checks) |

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
proposal](../R2.0-Future/storage.md#storage) is not a remaining MVP database requirement.

## Constitution conformance

The [2026-09-24 review](constitution-review.md#code-violations) found where
0.8.51 departs from the [constitution](../../docs/constitution.md#project-constitution).
Each row closes one finding there; its mutation must compile and fail on an
assertion.

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| K.23 | Done 0.8.79: a name freed by an ignored record inherits nothing — a record born under it takes the other records' references to it out in the same commit (`TestANameFreedByAnIgnoredRecordInheritsNothing`); a name never held keeps its advance listings | — | registering it leaves no stored ACL, Maintainer or Group-member reference admitting the new holder; removing the cleanup fails the check |
| K.24 | Done 0.8.80: ownership follows `user_id` — each record stores its owner's `user_id` (schema 8, filled from the owner's name), and a record whose `user_id` is not its named owner's is ignored at load (`TestAUserRecreatedUnderAVanishedNameOwnsNothingOfItsRecords`, `TestSchemaSevenFillsEachRecordsOwnerID`) | — | a User recreated under a vanished User's name owns none of its records after restart; comparing by name fails the check |
| K.25 | Done 0.8.82: waiting readers are refused only after the commit — a write marks the readers to recheck and the commit releases them once it lands (`TestAFailedCommitLeavesWaitingReadersWaiting`) | — | a failed commit leaves a waiting reader waiting; signalling before the commit fails the check |
| K.26 | Done 0.8.83: an ignored User keeps its credential — marked ignored like an ignored record, so the start's sweep passes it by (`TestAnIgnoredUserKeepsItsCredential`) | — | the start sweep leaves it; dropping the ignored mark fails the check |
| K.27 | Done 0.8.52: a published copy takes its recipient's TTL | — | a copy expires under its recipient's TTL; keeping the topic's expiry fails the check |
| K.28 | Done 0.8.84: a kind refuses fields it cannot have — 📣 queue settings (0.8.52), `script` on an 👾 alone (0.8.76), and `addr` and `protocol` on a 📡 alone, a stored one elsewhere dropped at load with a warning (`TestOnlyAServiceTakesAnAddress`, `TestAStoredAgentLosesTheAddrAnOlderRunnerWrote`, smoke) | — | removing each refusal fails its check |
| K.29 | Done 0.8.85: a corrupt token row is never repaired silently — an Agent's empty pair is ignored and reported at start and refused when presented; a User's is still bound; an issue over a row ignored as corrupt is reported (`TestACorruptCredentialRowIsNeverRepairedSilently`) | — | an agent row with an empty pair and a re-issue over an ignored row are both reported; silent binding fails the check |
| K.30 | A User inside a Group on a 📣's `deliver_to` is skipped at publication with an error-log warning, not counted as a drop, and the skip is said (Q126) | — | the chosen outcome is asserted; a silent skip fails the check |
| K.31 | `/group` de-duplicates members | — | a repeated member is stored once; removing the de-duplication fails the check |
| K.32 | A User name has no template part | — | `tmpl/eve` is refused as a User; removing the refusal fails the check |
| K.33 | Start and token-store failures reach the error log and syslog | — | each named case writes an `error.log` line; removing the report fails the check |
| K.34 | Registration refuses the fields it does not write | — | `status`, `maintainers`, `owner` and counters on `/register` are refused, and the reply carries the stored `created_at` |
| K.35 | Done 2026-09-30: the constitution's text matches the code, a copy failing through a forwarding 👾 or 📮 counted in the listed recipient's `dropped` (Q127), and Q125 settled as the code behaves | — | every [doc correction](constitution-review.md#doc-corrections) row is applied; links and anchors check clean |
| K.36 | Done 2026-09-30: the untested claims have checks — the flush ticker (smoke `restart`), syslog severity (`TestReportReachesSyslogAtItsSeverity`, smoke `corrupt_pair`), the logrotate rule (`TestSetupInstallsTheLogrotateRule`, fresh-install gate), an ownerless 👤 record, a peer Administrator's reactivation through a profile update, and the web route states; the reader release already had one | — | each [untested claim](constitution-review.md#untested-claims) has a check that fails when its code is removed |
| K.37 | The MCP face speaks only the latest MCP specification: waits on the clients under the [2025-11-25 exception](../../docs/constitution.md#external-protocols) (Q143). The v2 SDK (`@modelcontextprotocol/server` 2.2.0) serves 2026-07-28 and refuses a 2025 `initialize` with -32022 | Claude Code delivering channels on 2026-07-28; Codex and OpenCode speaking it | the face and its smoke checks negotiate only 2026-07-28; a 2025 `initialize` is refused; push reaches Claude Code on that revision. Pinning the old SDK or accepting an old revision fails the check. Recheck the clients on each release |

## Authority model

Built: the [authority specification](../../docs/01-identity-and-roles.md#role-names-and-scopes) has no pending change, and the historical fixture cleanup is [done](DONE.md#done--mvp). Each dependent web control is verified against it in [F.12](done/installed-browser-acceptance.md#checks)'s installed browser acceptance.
