# TODO MVP

## Objective

Finish the [MVP scope](README.md#scope). Built wave results are in [DONE](DONE.md#done--mvp); historical tasks retain their IDs in the [archived plan](done/TODO-before-rewrite.md#mvp-plan-before-the-documentation-rewrite).

## Next step

First close the shared-host safety gaps: G.1.3 (web authority), H.5.2 (SSH onboarding), H.5.3 (administrative crash recovery), and H.9.5 (runtime sidecars). The [review evidence](done/release-gap-review.md#findings) distinguishes reproduced failures from unverified risks. Then complete upgrade/recovery, browser and live-runtime acceptance alongside the existing packaging and local-account work. Open choices remain in [QUESTIONS](QUESTIONS.md#open-questions).

The [web redesign](#web-redesign) is authorized and in progress. Outstanding visual review and acceptance remain explicit below; it does not postpone the shared-host safety work.

## Remaining work

The required dashboard has [implementation and mutation evidence](done/owner-controls.md#verification); resource confinement still needs implementation and installed acceptance, as the [installed inspection](installed-acceptance.md#resource-confinement) confirms.

H.8 and H.9–H.9.3 have [built implementation and automated evidence](done/launcher-implementation.md#automated-verification); their rows retain the full live and installed acceptance requirements.

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| H.1 | Distributable package and installation instructions | All shipped programs and faces; the installer is our own script ([install](../../docs/09-setup.md#install)) | A new user installs on a fresh host and calls a service following only the instructions. Remove a required binary or face artifact from the package and the same exercise must fail |
| H.1.1 | [Upgrade and recovery](../../docs/09-setup.md#installation-acceptance) | H.1; existing installed state | Upgrade a populated installation and verify credentials, ACLs, queues and local mappings, preserved operator configuration, and the intended build in every running program and face. Interrupt an upgrade and recover by the documented procedure. Omit a component, overwrite configuration or restore mismatched credentials/state separately: each fails its check |
| H.8 | [Runtime integration delivery](../../docs/08-runner-role.md#runtime-integration-delivery) | Built adapters; H.1 for installed acceptance | Install into clean runtime profiles from the shipped artifacts. A bus message reaches each live interactive session and a correlated reply returns. Remove channel activation or Codex tool configuration separately and the relevant exchange fails; an isolated headless reply does not count |
| H.9 | [Smart launchers](../../docs/08-runner-role.md#smart-launchers) | Built adapters and V2 credentials; H.8 assets; H.1 for installed acceptance | From a path containing spaces, launch with and without prior history and verify the intended conversation, exact forwarded arguments and active integration. Force resume on an empty history, drop an argument or attach the pusher to a different App Server: each fails its corresponding check. Two concurrent launches must receive only their addressed messages; reuse one launch's endpoint or identity and the isolation check fails |
| H.9.1 | Launcher failure handling | H.9 | Exercise missing runtime, absent bus configuration, helper startup failure and runtime exit. Check diagnostics, plain-session fallback, exit status and cleanup while a separate session stays alive. Suppress readiness failure, force a zero exit status or remove cleanup separately: each fails its check. Supply contrary runtime mode options and verify the [enforced mode](../../docs/08-runner-role.md#smart-launchers) still applies; removing the enforcement must fail |
| H.9.2 | [MCP minimum](../../docs/05-discovery.md#mcp-minimum) in both runtimes | H.8, H.9; existing MCP face | In each installed, launcher-started session, list a known allowed service and call it for a unique response. Hide a known forbidden service and refuse a direct call to it. Remove tool loading, replace the listing with empty output, drop the response or bypass the ACL separately: each fails its corresponding check. An acceptance receipt cannot satisfy the service-response assertion |
| H.9.3 | [Assigned session names](../../docs/08-runner-role.md#session-names) | H.9; verify each runtime's name/session lookup | Check explicit bus identity, an assigned runtime name and missing-name fallback separately. Ignore an available assigned name or remove fallback and the corresponding assertion fails. Launch sessions with colliding titles or derived names and deliver a unique message to each; merge their bindings and delivery fails. Rename during a live launch with a reply pending: changing its inbox or credential must fail reply delivery and identity assertions. On restart, verify the documented address-change and old-inbox behavior; removing the rename, discarding queued messages or changing an explicit address must fail its corresponding check |
| H.9.4 | `ab-opencode` launcher and opencode push adapter | H.9; opencode's server API | Launch a session, deliver a bus message to it from another terminal and get a correlated reply. Break the server handshake, the config injection or the session binding separately: each fails its corresponding check. A headless session the person is not attached to does not count |
| H.9.5 | [Cross-account runtime sidecar isolation](../../docs/08-runner-role.md#runtime-isolation-and-recovery) | H.9; H.9.4 for opencode; installed accounts | From a second actual OS account, attempt to attach to each shipped session-control endpoint and read or steer the first account's disposable session. Refuse access while the intended TUI, MCP tools and pusher still work. Remove the boundary and the refusal check fails; random loopback ports alone do not satisfy it |
| H.9.6 | [Live-runtime recovery](../../docs/08-runner-role.md#runtime-isolation-and-recovery) | H.8, H.9; H.9.4 for opencode | Keep each shipped interactive runtime open across a graceful daemon restart and an isolated bus-child crash. Verify correlated delivery resumes with the same session binding, or an actionable inactive diagnostic appears. Exercise a failed sidecar separately and follow the recovery instructions to a successful exchange. Drop a post-recovery reply, change the session binding or hide inactive integration separately: each fails its check. Respect the existing snapshot and at-most-once loss boundaries |
| H.5.1 | ACL and local account mapping administration | Built SSH admin and tokens ([administering the account map](../../docs/09-setup.md#administering-the-account-map)) | An operator's change affects both visibility and use, survives restart, and leaves an unrelated user unchanged. Disable persistence or enforcement separately and the corresponding check fails |
| H.5.2 | [Installed SSH onboarding](../../docs/09-setup.md#installation-acceptance) | H.2–H.5 built; an isolated sshd host | Through real SSH, exercise the documented token command for ordinary and operator keys and the operator administration grammar. Verify key entitlement, removal and disallowed shell/PTY/forwarding requests while an unrelated key still works. Discard the entitled principal, prevent forced-command execution or remove a key restriction separately: each fails its corresponding check |
| H.5.3 | [Administrative crash recovery](../../docs/04-messaging.md#administrative-crash-recovery) | Built administration; the guarantee is settled | Apply a ban, remove group membership and tighten an ACL separately; after each acknowledged change kill the bus child before the next snapshot. Check old tokens, existing sessions and mapped sockets against the approved recovery policy, with an unrelated user as a positive control. Exercise failed persistence as well. Remove the approved durability or recovery enforcement and the corresponding check fails |
| F.12 | [Installed browser acceptance](../../docs/05-discovery.md#browser-acceptance) | H.1; required dashboard tabs built; F.13.6 if redesign accepted | In a real browser on the installed host, sign in/out and exercise service/channel, user and group controls as owner, maintainer and ordinary user; check denial paths and activity graphs. Restart web and bus separately and verify documented session behavior. Break cookie handling, form-origin validation or authorization separately: each fails its check |
| G.1.2 | Implement and verify dashboard resource confinement | [process boundary](../../docs/11-processes.md#the-rule) | A web-specific limit does not exist yet ([inspection](installed-acceptance.md#resource-confinement)), so this builds one and then verifies it: a shared unit limit is not evidence of web-only confinement while the bus survives. Drive the web child to its configured limit while bus calls continue, on a **disposable installation** carrying the same generated policy, with its own state, sockets and credentials — the live install gets read-only checks only, and they are recorded separately from this evidence. Remove the web limit and the limit assertion fails; a dead web child cannot satisfy the positive control |
| G.1.3 | [Web credential and state isolation](../../docs/11-processes.md#web-authority-boundary) | Installed account gate; existing web API | Under the web child's actual confinement, on a **disposable installation** carrying the same generated policy rather than the live one, refuse reading or modifying daemon credentials, snapshots and SSH authorization, and refuse mapped account sockets that grant independent authority. Use disposable canaries for write probes. Normal visitor-authenticated shared-socket calls must succeed. Remove each protection separately and its negative check fails |

## Web redesign

F.13 implementation is authorized and in progress; its full acceptance remains open. The [design proposal](web-interfaces.md#proposal) owns the target; the [audit](done/web-review.md#findings) owns observed shortcomings. Existing completed F tasks remain historical evidence, not proof that these new acceptance checks pass.

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| F.13.0 | Review page designs and settle scope | Owner review of proposal; existing required dashboard scope | Map each existing required view/control to its new home. Review populated overview, service detail, directory, channel, activity and error/narrow layouts before coding. Remove an existing capability from the map or substitute an empty-only design: completeness review fails. Promote accepted substance and index decisions; unaccepted extensions stay proposed |
| F.13.2 | Shared page shell, templates and recovery; [header, footer and the public node call built](done/node-identity.md#what-this-is-not) | F.13.0; [F.13.1](done/web-meanings.md#scope) for affected labels | Every page has current navigation, account/sign-out, a unique title, landmarks, focus and scoped errors. Auth-required deep links return to their local destination after sign-in; foreign return URLs are refused. Preserve nonsensitive invalid form input, never tokens/configuration. Missing and hidden records share not-found presentation; actionable permission refusal, expired session and unavailable bus get distinct recovery. Remove shell inclusion, lose return/filter state, echo a submitted secret or turn backend failure into an empty result separately: each fails |
| F.13.3 | Service and channel journeys | [F.13.1](done/web-meanings.md#scope); F.13.2 | Search by description/address, independently filter availability/reader state, sort and page with state retained, then inspect owner/access/queue information as a read-only visitor. Use populated pub/sub and queue fixtures, edit each with authorized credentials, and return to the affected detail. A protected resource remains hidden. Remove descriptions, hide summaries behind edit permission, conflate delivery modes or redirect a channel edit to Services separately: each fails |
| F.13.4 | Users, groups and account journeys; [directory classification and cleanup built](done/identity-cleanup.md#scope) | [F.13.1](done/web-meanings.md#scope); F.13.2 | Mixed directory fixture exceeds a page and includes long names, missing profiles and unclassified identities. Find a person, inspect memberships/resources and exercise lifecycle under owner/maintainer/ordinary hierarchy. Inspect own fingerprints; retain hidden-membership distinction and protected-group behavior. Remove paging/filter retention, label hidden membership empty, offer protected deletion or expose another user's credentials separately: each fails. Requests to the bus per directory page must not grow with rendered avatar count; restore per-avatar full-directory fetching and that check fails |
| F.13.5 | Overview, activity and envelope diagnostics | [F.13.1](done/web-meanings.md#scope); F.13.2–F.13.3 | Use known nonzero traffic, a full queue, losses/refusals, irregular sample times, a partial interval and restart-empty history. Graph values and time positions match the source; zero differs from absent data. Exercise reused topic/tag across participants, receipt references, redirected replies and a missing original request; never assert completion without evidence. Replace timestamp positioning with sample index, merge unrelated exchanges, label dequeued completed, or restore uncontrolled refresh separately: each fails. Keep MVP bounded diagnostics, not the R1 explorer |
| F.13.6 | Performance, accessibility and migration acceptance | F.13.2–F.13.5 | Browser checks at desktop, narrow and zoomed sizes with populated/empty/error fixtures, keyboard operation, measured text contrast, labelled graph/table access and all legacy entry points. Track browser bytes/requests and bus calls separately; benchmark representative larger directories and record host/data conditions before setting budgets. Remove viewport/reflow treatment, reduce contrast, break a legacy link or reintroduce row-proportional directory fetches separately: each named check fails. Run required repository verification; then hand to F.12 for installed role/action and restart acceptance |

Local mapping administration is the operator key or a user token ([administering the account map](../../docs/09-setup.md#administering-the-account-map)); it is not hidden inside a new Settings page. G.1.2/G.1.3 remain confinement dependencies for release. H.5.3 owns administrative crash durability, and an acknowledged restriction now holds until lifted ([crash recovery](../../docs/04-messaging.md#administrative-crash-recovery)): no success message may promise more than the daemon delivers. Read-only screenshots do not close F.12. UI development can use an isolated populated daemon while installed isolation work proceeds separately.

## Installed stage gate

Setup and the supervisor are already built. Prepare an isolated systemd host before accepting local isolation; B's socket ownership depends on that installation, not on packaging being the last task. Development checks alone do not close this gate.

The [development-host inspection](installed-acceptance.md#inspection) records partial positive evidence; mutation checks and independent-host acceptance remain open.

| Gate | Dependencies | Required evidence and mutation |
|---|---|---|
| Account and state | H.2–H.4 built; [setup contract](../../docs/09-setup.md#the-two-accounts) | Inspect running uid, home and state ownership. Start as the installing user instead and the check fails |
| Per-user sockets | B.2 and G.1 built; account gate | Two actual OS accounts can use their own sockets and cannot use each other's. Wrong ownership or permissive mode must fail a negative check while each positive call still succeeds |
| Capabilities | Account gate; [capability boundary](../../docs/11-processes.md#why-the-supervisor-holds-cap_chown) | Inspect the live supervisor and bus capability sets under the unit. Giving the bus the supervisor's capability or removing it from the supervisor must fail; an unprivileged development run is not evidence |
| Operational acceptance | H.1.1, H.5.2–H.5.3, H.9.5–H.9.6, F.12, G.1.3 | Retain each task's installed evidence and named mutation failure; fixture-only results cannot close an installed or live-runtime requirement |
| Fresh installation | H.1, H.8 and H.9–H.9.6; process boundary and confinement work; operational acceptance | Run the package, browser and runtime integration acceptance above on a host without `/rd` or the checkout; retain commands, results, runtime versions and host conditions in the completion evidence |

## Personal services

Enforce the [owner-only empty ACL rule](../../docs/02-access.md#acl) for Personal
and non-Personal records. Check the owner succeeds and other principals cannot
gain access through the empty-list branch or existing implicit grants; retain a
non-empty ACL positive control. Restore the open-empty behavior and the refusal
check must fail. Update form help together with enforcement.

Implement the [Personal service requirements](../../docs/03-services-and-topics.md#personal-and-shared).
Persist the owner's choice; omitted tagging remains non-Personal. Verify an
allowed service entry and a refused user entry, including a user with a backing
record. Show the user's Personal services and the daemon owner's per-user view,
with non-Personal controls excluded from those results. Ignoring the tag,
accepting a user entry or mixing owners must fail its corresponding check.
Enforcement also requires settling [implicit access](QUESTIONS.md#personal-service-access).

## Questions

All unresolved choices are owned by [QUESTIONS](QUESTIONS.md#open-questions). The CLI selector question remains open without silently blocking unrelated work. The [Future storage proposal](../Future/storage.md#storage) is not a remaining MVP database requirement.

## Authority model

Implement the [current authority specification](../../docs/01-identity-and-roles.md#role-names-and-scopes).

| Work | Acceptance |
|---|---|
| Daemon-owner override and transfer | Owner can manage and transfer any service/channel; configured daemon ownership survives restart after transfer |
| Explicit setup ownership | Startup requires established ownership rather than deriving it from the runtime OS account |
| Profile editing and provenance | Enforce the [profile rules](../../docs/01-identity-and-roles.md#users-and-profiles), including protected names and permitted PersonName sources |
| Historical smoke fixture cleanup | Review existing `plain@srv1`, `chief@srv1` and `piped@srv1` live identities and chief's administrative standing before revoking/removing anything; [escaped provisioning evidence](done/administrator-names.md#live-verification-and-harness-correction) |
| Administrator unbanning | Administrator can unban an ordinary user, but cannot edit peer Administrators or the owner |
| Effective Maintainer membership | Enforce owner control through direct and nested group changes; blocked on [membership choice](QUESTIONS.md#authority-model) |
| Nested groups | Resolve nested membership with defined cycle handling; current membership is flat |
| Service-defined roles | Store/resolve and return service-defined labels while preserving owner-only assignment of the reserved Maintainer role; syntax/transport proposals are not automatically adopted |
| Transfer recipient | Apply the resolved [recipient rule](QUESTIONS.md#authority-model) |
