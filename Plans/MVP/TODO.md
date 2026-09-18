# TODO MVP

📌 **TL;DR:** Finish accepted behavior and the remaining browser/runtime journeys on the installed release.

## Objective

Finish the [MVP scope](README.md#scope). Built wave results are in [DONE](DONE.md#done--mvp); historical tasks retain their IDs in the [archived plan](done/TODO-before-rewrite.md#mvp-plan-before-the-documentation-rewrite).

## Next step

H.1 and H.1.1 have [fresh-install](done/fresh-install.md#checks) and [populated-upgrade](done/upgrade-recovery.md#checks) evidence. H.9.5 has [concurrent native Codex/OpenCode evidence](done/runtime-interactive.md#checks); Claude still needs a [channel-enabled acceptance configuration](done/runtime-interactive.md#claude-prerequisite). Continue browser and live-runtime acceptance. H.5.3 has [administrative crash-recovery evidence](done/administrative-durability.md#checks). H.5.2 has [real-SSH evidence](done/ssh-onboarding.md#checks); G.1.2 and G.1.3 have installed [resource](done/web-resources.md#checks) and [authority-isolation](done/web-isolation.md#checks) evidence. The [review evidence](done/release-gap-review.md#findings) distinguishes reproduced failures from unverified risks. Choices are tracked in [QUESTIONS](QUESTIONS.md#open-questions).

Owner-requested web slices are built; the remaining [web redesign](#web-redesign) still depends on F.13.0, the owner’s specification review. That gate does not block independently accepted work below or shared-host safety fixes.

Accepted feature work remains in [authority](#authority-model).
This section is unfinished MVP work, not optional follow-up.

## Remaining work

The required dashboard has [implementation and mutation evidence](done/owner-controls.md#verification); its [resource confinement](done/web-resources.md#checks) is built and exercised in a disposable generated unit.

H.8 and H.9–H.9.3 have [built implementation and automated evidence](done/launcher-implementation.md#automated-verification); their rows retain the full live and installed acceptance requirements. H.9.4’s OpenCode launcher and adapter are also [built](../../docs/08-runner-role.md#smart-launchers); its acceptance remains open. A built component does not close its row.

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| H.8 | [Runtime integration delivery](../../docs/08-runner-role.md#runtime-integration-delivery) | Built adapters; [H.1 package complete](done/fresh-install.md#checks) | Install into clean runtime profiles from the shipped artifacts. A bus message reaches each live interactive session and a correlated reply returns. Remove channel activation or Codex tool configuration separately and the relevant exchange fails; an isolated headless reply does not count |
| H.9 | [Smart launchers](../../docs/08-runner-role.md#smart-launchers) | Built adapters and V2 credentials; H.8 assets; [H.1 package complete](done/fresh-install.md#checks) | From a path containing spaces, launch with and without prior history and verify the intended conversation, exact forwarded arguments and active integration. Force resume on an empty history, drop an argument or attach the pusher to a different App Server: each fails its corresponding check. Two concurrent launches must receive only their addressed messages; reuse one launch's endpoint or identity and the isolation check fails |
| H.9.1 | Launcher failure handling | H.9 | Exercise missing runtime, absent bus configuration, helper startup failure and runtime exit. Check diagnostics, plain-session fallback, exit status and cleanup while a separate session stays alive. Suppress readiness failure, force a zero exit status or remove cleanup separately: each fails its check. Supply contrary runtime mode options and verify the [enforced mode](../../docs/08-runner-role.md#smart-launchers) still applies; removing the enforcement must fail |
| H.9.2 | [MCP minimum](../../docs/05-discovery.md#mcp-minimum) in each shipped runtime | H.8, H.9; existing MCP face | In each installed, launcher-started session, list a known allowed service and call it for a unique response. Hide a known forbidden service and refuse a direct call to it. Remove tool loading, replace the listing with empty output, drop the response or bypass the ACL separately: each fails its corresponding check. An acceptance receipt cannot satisfy the service-response assertion |
| H.9.3 | [Assigned session names](../../docs/08-runner-role.md#session-names) | H.9; verify each runtime's name/session lookup | Check explicit bus identity, an assigned runtime name and missing-name fallback separately. Ignore an available assigned name or remove fallback and the corresponding assertion fails. Launch sessions with colliding titles or derived names and deliver a unique message to each; merge their bindings and delivery fails. Rename during a live launch with a reply pending: changing its inbox or credential must fail reply delivery and identity assertions. On restart, verify the documented address-change and old-inbox behavior; removing the rename, discarding queued messages or changing an explicit address must fail its corresponding check |
| H.9.4 | Built `ab-opencode` launcher and push adapter: remaining live acceptance | H.9; existing OpenCode adapter | Launch a session, deliver a bus message to it from another terminal and get a correlated reply. Break the server handshake, the config injection or the session binding separately: each fails its corresponding check. A headless session the person is not attached to does not count |
| H.9.5 | [Endpoint authentication](done/runtime-endpoint-auth.md#checks) and [Codex/OpenCode interactive checks](done/runtime-interactive.md#checks); Claude co-exercise remains open; [interactive sidecar isolation](../../docs/08-runner-role.md#runtime-isolation-and-recovery) | H.9; H.9.4 for opencode; installed accounts | From a second actual OS account, attempt to attach to each shipped session-control endpoint and read or steer the first account's disposable session. Refuse access while the intended TUI, MCP tools and pusher still work. Remove the boundary and the refusal check fails; random loopback ports alone do not satisfy it |
| H.9.6 | [Live-runtime recovery](../../docs/08-runner-role.md#runtime-isolation-and-recovery) | H.8, H.9; H.9.4 for opencode | Keep each shipped interactive runtime open across a graceful daemon restart and an isolated bus-child crash. Verify correlated delivery resumes with the same session binding, or an actionable inactive diagnostic appears. Exercise a failed sidecar separately and follow the recovery instructions to a successful exchange. Drop a post-recovery reply, change the session binding or hide inactive integration separately: each fails its check. Respect the existing snapshot and at-most-once loss boundaries |
| F.12 | [Installed browser acceptance](../../docs/05-discovery.md#browser-acceptance); [session/restart foundation](done/installed-browser-foundation.md#checks) and [current authority matrix](done/installed-browser-role-matrix.md#checks) complete | F.13.6 for the redesigned journeys after F.13.0 | Retain and rerun the installed five-role service/channel, user/group, denial, activity, cookie and separate web/bus restart checks against the approved redesign. Break form-origin validation or authorization separately: each fails its check |

## Web redesign

F.13.0 remains an owner gate for the remaining redesign. The [specifications](web/README.md#what-each-document-owns) supersede parts of the [earlier proposal](web-interfaces.md#proposal); the [audit](done/web-review.md#findings) records observed shortcomings. Independently requested slices are built, as linked below; they neither approve the whole specification nor close these acceptance checks. The owner’s preference is plain administration, not additional design polish; scope review must reconcile the proposed visual work with that preference.

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| F.13.0 | Review page designs and settle scope | Owner review of proposal; existing required dashboard scope | Map each existing required view/control to its new home. Review populated overview, service detail, directory, channel, activity and error/narrow layouts before coding. Remove an existing capability from the map or substitute an empty-only design: completeness review fails. Promote accepted substance and index decisions; unaccepted extensions stay proposed |
| F.13.2 | Shared page shell, templates and recovery; [header, footer and the public node call built](done/node-identity.md#what-this-is-not); [title marks built](done/web-title-help.md#checks); [form recovery and keyboard skip built](done/web-form-recovery.md#checks) | F.13.0; [F.13.1](done/web-meanings.md#scope) for affected labels | Every page has current navigation, account/sign-out, a unique title, landmarks, focus and scoped errors. Auth-required deep links return to their local destination after sign-in; foreign return URLs are refused. Preserve nonsensitive invalid form input, never tokens/configuration. Missing and hidden records share not-found presentation; actionable permission refusal, expired session and unavailable bus get distinct recovery. Remove shell inclusion, lose return/filter state, echo a submitted secret or turn backend failure into an empty result separately: each fails |
| F.13.5 | Overview, activity and envelope diagnostics; [receipt correlation built](done/exchange-evidence.md#checks); [resource activity built](done/service-activity.md#checks); [compact diagnostics presentation built](done/web-administration-redesign.md#checks) | [F.13.1](done/web-meanings.md#scope); F.13.2; [F.13.3 complete](done/service-channel-journeys.md#checks) | Complete the Overview and remaining diagnostics journeys. Retain the built graph values, real-time positions, shared scale, partial interval, restart-empty history and zero-versus-absent distinction. Exercise reused topic/tag across participants, receipt references, redirected replies and a missing original request; never assert completion without evidence. Merge unrelated exchanges, label dequeued completed, or restore uncontrolled refresh separately: each fails. Keep MVP bounded diagnostics, not the R1 explorer |
| F.13.6 | Performance, accessibility and migration acceptance | F.13.2–F.13.5 | Browser checks at desktop, narrow and zoomed sizes with populated/empty/error fixtures, keyboard operation, measured text contrast, labelled graph/table access and all legacy entry points. Track browser bytes/requests and bus calls separately; benchmark representative larger directories and record host/data conditions before setting budgets. Remove viewport/reflow treatment, reduce contrast, break a legacy link or reintroduce row-proportional directory fetches separately: each named check fails. Run required repository verification; then hand to F.12 for installed role/action and restart acceptance |

Local mapping administration is the operator key or a user token ([administering the account map](../../docs/09-setup.md#administering-the-account-map)); it is not hidden inside a new Settings page. [G.1.2 resource limits](done/web-resources.md#checks) are complete. Read-only screenshots do not close F.12. UI development can use an isolated populated daemon while installed isolation work proceeds separately.

## Installed stage gate

The package, setup and supervisor are built. Continue the remaining checks on isolated systemd hosts; development checks alone do not close installed runtime or browser gates.

The service-account, per-user-socket and capability rows are complete on a
[package-only real-systemd host](done/installed-shared-host.md#checks). The
remaining rows concern broader operational and runtime/browser acceptance.

| Gate | Dependencies | Required evidence and mutation |
|---|---|---|
| Operational acceptance | H.5.2–H.5.3, H.9.5–H.9.6, F.12, G.1.2–G.1.3; [H.1.1 complete](done/upgrade-recovery.md#checks) | Retain each task's installed evidence and named mutation failure; fixture-only results cannot close an installed or live-runtime requirement |
| Fresh installed release | [H.1 package exercise complete](done/fresh-install.md#checks); H.8 and H.9–H.9.6, browser and operational acceptance remain | Run the remaining browser and runtime integration acceptance on a host without `/rd` or the checkout; retain commands, results, runtime versions and host conditions in the completion evidence |

The installed gates do not replace feature acceptance. Before closing MVP, also complete the accepted feature sections below and the remaining F.13 work after owner review. Keep deferred decisions out of that gate: Q69 hardening and broader transfer-recipient eligibility were deferred, while Q63 and Administrator control of ordinary group membership confirm existing behavior ([decisions](../../docs/decisions.md#settled)).

## Default service access

Completed in 0.5.44; see [implementation and checks](done/empty-acl.md#checks).
The anchor stays for existing references. Personal tagging and the authority
changes below remain separate work.

## Personal services

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

No open choices are currently recorded in [QUESTIONS](QUESTIONS.md#open-questions); implementation gaps are not reopened policy questions. The [Future storage proposal](../Future/storage.md#storage) is not a remaining MVP database requirement.

## Authority model

Implement the pending changes in the [authority specification](../../docs/01-identity-and-roles.md#role-names-and-scopes); retain already-built permissions. Verify each dependent web control against the resulting authority before final browser acceptance.

| Work | Acceptance |
|---|---|
| Historical smoke fixture cleanup | [Read-only provenance review](live-fixture-cleanup.md#measured-state) completed: all three remain active profiles/self-owned Agent records without stored SSH keys or tokens, and `chief@srv1` remains a direct Administrator. Cleanup awaits the Owner's authority decision; revoke or remove nothing implicitly |
| Service-defined roles | Store and return service-defined labels without interpreting their application meaning while preserving owner-only assignment of the reserved Maintainer role; syntax/transport proposals are not automatically adopted |
