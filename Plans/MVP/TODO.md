# TODO MVP

📌 **TL;DR:** Finish accepted behavior, close shared-host safety gaps, then prove the installed release.

## Objective

Finish the [MVP scope](README.md#scope). Built wave results are in [DONE](DONE.md#done--mvp); historical tasks retain their IDs in the [archived plan](done/TODO-before-rewrite.md#mvp-plan-before-the-documentation-rewrite).

## Next step

First close the shared-host safety gaps: G.1.3 (web authority), H.5.2 (SSH onboarding), H.5.3 (administrative crash recovery), and H.9.5 (runtime sidecars). The [review evidence](done/release-gap-review.md#findings) distinguishes reproduced failures from unverified risks. Then complete upgrade/recovery, browser and live-runtime acceptance alongside the existing packaging and local-account work. Choices are tracked in [QUESTIONS](QUESTIONS.md#open-questions).

Owner-requested web slices are built; the remaining [web redesign](#web-redesign) still depends on F.13.0, the owner’s specification review. That gate does not block independently accepted work below or shared-host safety fixes.

Accepted feature work remains in [access](#default-service-access), [Personal services](#personal-services), [inbox selection](#inbox-selection-and-filters), [Readers](#reader-visibility), [display labels](#identity-display-labels) and [authority](#authority-model). These sections are unfinished MVP work, not optional follow-up.

## Remaining work

The required dashboard has [implementation and mutation evidence](done/owner-controls.md#verification); resource confinement still needs implementation and installed acceptance, as the [installed inspection](installed-acceptance.md#resource-confinement) confirms.

H.8 and H.9–H.9.3 have [built implementation and automated evidence](done/launcher-implementation.md#automated-verification); their rows retain the full live and installed acceptance requirements. H.9.4’s OpenCode launcher and adapter are also [built](../../docs/08-runner-role.md#smart-launchers); its acceptance remains open. A built component does not close its row.

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| H.1 | Distributable package and installation instructions | All shipped programs and faces; the installer is our own script ([install](../../docs/09-setup.md#install)) | A new user installs on a fresh host and calls a service following only the instructions. Remove a required binary or face artifact from the package and the same exercise must fail |
| H.1.1 | [Upgrade and recovery](../../docs/09-setup.md#installation-acceptance) | H.1; existing installed state | Upgrade a populated installation and verify credentials, ACLs, queues and local mappings, preserved operator configuration, and the intended build in every running program and face. Interrupt an upgrade and recover by the documented procedure. Omit a component, overwrite configuration or restore mismatched credentials/state separately: each fails its check |
| H.8 | [Runtime integration delivery](../../docs/08-runner-role.md#runtime-integration-delivery) | Built adapters; H.1 for installed acceptance | Install into clean runtime profiles from the shipped artifacts. A bus message reaches each live interactive session and a correlated reply returns. Remove channel activation or Codex tool configuration separately and the relevant exchange fails; an isolated headless reply does not count |
| H.9 | [Smart launchers](../../docs/08-runner-role.md#smart-launchers) | Built adapters and V2 credentials; H.8 assets; H.1 for installed acceptance | From a path containing spaces, launch with and without prior history and verify the intended conversation, exact forwarded arguments and active integration. Force resume on an empty history, drop an argument or attach the pusher to a different App Server: each fails its corresponding check. Two concurrent launches must receive only their addressed messages; reuse one launch's endpoint or identity and the isolation check fails |
| H.9.1 | Launcher failure handling | H.9 | Exercise missing runtime, absent bus configuration, helper startup failure and runtime exit. Check diagnostics, plain-session fallback, exit status and cleanup while a separate session stays alive. Suppress readiness failure, force a zero exit status or remove cleanup separately: each fails its check. Supply contrary runtime mode options and verify the [enforced mode](../../docs/08-runner-role.md#smart-launchers) still applies; removing the enforcement must fail |
| H.9.2 | [MCP minimum](../../docs/05-discovery.md#mcp-minimum) in each shipped runtime | H.8, H.9; existing MCP face | In each installed, launcher-started session, list a known allowed service and call it for a unique response. Hide a known forbidden service and refuse a direct call to it. Remove tool loading, replace the listing with empty output, drop the response or bypass the ACL separately: each fails its corresponding check. An acceptance receipt cannot satisfy the service-response assertion |
| H.9.3 | [Assigned session names](../../docs/08-runner-role.md#session-names) | H.9; verify each runtime's name/session lookup | Check explicit bus identity, an assigned runtime name and missing-name fallback separately. Ignore an available assigned name or remove fallback and the corresponding assertion fails. Launch sessions with colliding titles or derived names and deliver a unique message to each; merge their bindings and delivery fails. Rename during a live launch with a reply pending: changing its inbox or credential must fail reply delivery and identity assertions. On restart, verify the documented address-change and old-inbox behavior; removing the rename, discarding queued messages or changing an explicit address must fail its corresponding check |
| H.9.4 | Built `ab-opencode` launcher and push adapter: remaining live acceptance | H.9; existing OpenCode adapter | Launch a session, deliver a bus message to it from another terminal and get a correlated reply. Break the server handshake, the config injection or the session binding separately: each fails its corresponding check. A headless session the person is not attached to does not count |
| H.9.5 | [Cross-account runtime sidecar isolation](../../docs/08-runner-role.md#runtime-isolation-and-recovery) | H.9; H.9.4 for opencode; installed accounts | From a second actual OS account, attempt to attach to each shipped session-control endpoint and read or steer the first account's disposable session. Refuse access while the intended TUI, MCP tools and pusher still work. Remove the boundary and the refusal check fails; random loopback ports alone do not satisfy it |
| H.9.6 | [Live-runtime recovery](../../docs/08-runner-role.md#runtime-isolation-and-recovery) | H.8, H.9; H.9.4 for opencode | Keep each shipped interactive runtime open across a graceful daemon restart and an isolated bus-child crash. Verify correlated delivery resumes with the same session binding, or an actionable inactive diagnostic appears. Exercise a failed sidecar separately and follow the recovery instructions to a successful exchange. Drop a post-recovery reply, change the session binding or hide inactive integration separately: each fails its check. Respect the existing snapshot and at-most-once loss boundaries |
| H.5.1 | ACL and local account mapping administration | Built SSH admin and tokens ([administering the account map](../../docs/09-setup.md#administering-the-account-map)) | An operator's change affects both visibility and use, survives restart, and leaves an unrelated user unchanged. Disable persistence or enforcement separately and the corresponding check fails |
| H.5.2 | [Installed SSH onboarding](../../docs/09-setup.md#installation-acceptance) | H.2–H.5 built; an isolated sshd host | Through real SSH, exercise the documented token command for ordinary and operator keys and the operator administration grammar. Verify key entitlement, removal and disallowed shell/PTY/forwarding requests while an unrelated key still works. Discard the entitled principal, prevent forced-command execution or remove a key restriction separately: each fails its corresponding check |
| H.5.3 | Build and verify [administrative crash recovery](../../docs/04-messaging.md#administrative-crash-recovery) | Built administration; durable acknowledgement is not implemented | Implement the settled durability guarantee, including refusing success when persistence fails. Apply a ban, remove group membership and tighten an ACL separately; after each acknowledged change kill the bus child before the next snapshot. Check old tokens, existing sessions and mapped sockets against the approved recovery policy, with an unrelated user as a positive control. Exercise failed persistence as well. Remove the approved durability or recovery enforcement and the corresponding check fails |
| F.12 | [Installed browser acceptance](../../docs/05-discovery.md#browser-acceptance) | H.1; required dashboard tabs built; F.13.6 for the redesigned journeys after F.13.0 | In a real browser on the installed host, sign in/out and exercise service/channel, user and group controls as daemon Owner, Administrator, resource Owner/Maintainer and ordinary user; check denial paths and activity graphs. Restart web and bus separately and verify documented session behavior. Break cookie handling, form-origin validation or authorization separately: each fails its check |
| G.1.2 | Implement and verify dashboard resource confinement | [process boundary](../../docs/11-processes.md#the-rule) | A web-specific limit does not exist yet ([inspection](installed-acceptance.md#resource-confinement)), so this builds one and then verifies it: a shared unit limit is not evidence of web-only confinement while the bus survives. Drive the web child to its configured limit while bus calls continue, on a **disposable installation** carrying the same generated policy, with its own state, sockets and credentials — the live install gets read-only checks only, and they are recorded separately from this evidence. Remove the web limit and the limit assertion fails; a dead web child cannot satisfy the positive control |
| G.1.3 | [Web credential and state isolation](../../docs/11-processes.md#web-authority-boundary) | Installed account gate; existing web API | Under the web child's actual confinement, on a **disposable installation** carrying the same generated policy rather than the live one, refuse reading or modifying daemon credentials, snapshots and SSH authorization, and refuse mapped account sockets that grant independent authority. Use disposable canaries for write probes. Normal visitor-authenticated shared-socket calls must succeed. Remove each protection separately and its negative check fails |

## Web redesign

F.13.0 remains an owner gate for the remaining redesign. The [specifications](web/README.md#what-each-document-owns) supersede parts of the [earlier proposal](web-interfaces.md#proposal); the [audit](done/web-review.md#findings) records observed shortcomings. Independently requested slices are built, as linked below; they neither approve the whole specification nor close these acceptance checks. The owner’s preference is plain administration, not additional design polish; scope review must reconcile the proposed visual work with that preference.

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| F.13.0 | Review page designs and settle scope | Owner review of proposal; existing required dashboard scope | Map each existing required view/control to its new home. Review populated overview, service detail, directory, channel, activity and error/narrow layouts before coding. Remove an existing capability from the map or substitute an empty-only design: completeness review fails. Promote accepted substance and index decisions; unaccepted extensions stay proposed |
| F.13.2 | Shared page shell, templates and recovery; [header, footer and the public node call built](done/node-identity.md#what-this-is-not) | F.13.0; [F.13.1](done/web-meanings.md#scope) for affected labels | Every page has current navigation, account/sign-out, a unique title, landmarks, focus and scoped errors. Auth-required deep links return to their local destination after sign-in; foreign return URLs are refused. Preserve nonsensitive invalid form input, never tokens/configuration. Missing and hidden records share not-found presentation; actionable permission refusal, expired session and unavailable bus get distinct recovery. Remove shell inclusion, lose return/filter state, echo a submitted secret or turn backend failure into an empty result separately: each fails |
| F.13.3 | Service and channel journeys | [F.13.1](done/web-meanings.md#scope); F.13.2 | Search by description/address, independently filter availability/reader state, sort and page with state retained, then inspect owner/access/queue information as a read-only visitor. Use populated pub/sub and queue fixtures, edit each with authorized credentials, and return to the affected detail. A protected resource remains hidden. Remove descriptions, hide summaries behind edit permission, conflate delivery modes or redirect a channel edit to Services separately: each fails |
| F.13.4 | Users, groups and account journeys; [directory classification and cleanup built](done/identity-cleanup.md#scope) | [F.13.1](done/web-meanings.md#scope); F.13.2 | Mixed directory fixture exceeds a page and includes long names, missing profiles and unclassified identities. Find a person, inspect memberships/resources and exercise lifecycle under daemon Owner/Administrator/ordinary-user authority. Inspect own fingerprints; retain hidden-membership distinction and protected-group behavior. Remove paging/filter retention, label hidden membership empty, offer protected deletion or expose another user's credentials separately: each fails. Requests to the bus per directory page must not grow with rendered avatar count; restore per-avatar full-directory fetching and that check fails |
| F.13.5 | Overview, activity and envelope diagnostics; [receipt correlation built](done/exchange-evidence.md#checks) | [F.13.1](done/web-meanings.md#scope); F.13.2–F.13.3 | Use known nonzero traffic, a full queue, losses/refusals, irregular sample times, a partial interval and restart-empty history. Graph values and time positions match the source; zero differs from absent data. Exercise reused topic/tag across participants, receipt references, redirected replies and a missing original request; never assert completion without evidence. Replace timestamp positioning with sample index, merge unrelated exchanges, label dequeued completed, or restore uncontrolled refresh separately: each fails. Keep MVP bounded diagnostics, not the R1 explorer |
| F.13.6 | Performance, accessibility and migration acceptance | F.13.2–F.13.5 | Browser checks at desktop, narrow and zoomed sizes with populated/empty/error fixtures, keyboard operation, measured text contrast, labelled graph/table access and all legacy entry points. Track browser bytes/requests and bus calls separately; benchmark representative larger directories and record host/data conditions before setting budgets. Remove viewport/reflow treatment, reduce contrast, break a legacy link or reintroduce row-proportional directory fetches separately: each named check fails. Run required repository verification; then hand to F.12 for installed role/action and restart acceptance |

Local mapping administration is the operator key or a user token ([administering the account map](../../docs/09-setup.md#administering-the-account-map)); it is not hidden inside a new Settings page. G.1.2/G.1.3 remain confinement dependencies for release. H.5.3 must implement the settled [crash-recovery guarantee](../../docs/04-messaging.md#administrative-crash-recovery); today an acknowledged restriction can still be lost before a snapshot. Read-only screenshots do not close F.12. UI development can use an isolated populated daemon while installed isolation work proceeds separately.

## Installed stage gate

Setup and the supervisor are already built. Prepare an isolated systemd host before accepting local isolation; B's socket ownership depends on that installation, not on packaging being the last task. Development checks alone do not close this gate.

The [development-host inspection](installed-acceptance.md#inspection) records partial positive evidence; mutation checks and independent-host acceptance remain open.

| Gate | Dependencies | Required evidence and mutation |
|---|---|---|
| Account and state | H.2–H.4 built; [setup contract](../../docs/09-setup.md#the-two-accounts) | Inspect running uid, home and state ownership. Start as the installing user instead and the check fails |
| Per-user sockets | B.2 and G.1 built; account gate | Two actual OS accounts can use their own sockets and cannot use each other's. Wrong ownership or permissive mode must fail a negative check while each positive call still succeeds |
| Capabilities | Account gate; [capability boundary](../../docs/11-processes.md#why-the-supervisor-holds-cap_chown) | Inspect the live supervisor and bus capability sets under the unit. Giving the bus the supervisor's capability or removing it from the supervisor must fail; an unprivileged development run is not evidence |
| Operational acceptance | H.1.1, H.5.2–H.5.3, H.9.5–H.9.6, F.12, G.1.2–G.1.3 | Retain each task's installed evidence and named mutation failure; fixture-only results cannot close an installed or live-runtime requirement |
| Fresh installation | H.1, H.8 and H.9–H.9.6; process boundary and confinement work; operational acceptance | Run the package, browser and runtime integration acceptance above on a host without `/rd` or the checkout; retain commands, results, runtime versions and host conditions in the completion evidence |

The installed gates do not replace feature acceptance. Before closing MVP, also complete the accepted feature sections below, H.5.1 and the remaining F.13 work after owner review. Keep deferred decisions out of that gate: Q69 hardening and broader transfer-recipient eligibility were deferred, while Q63 and Administrator control of ordinary group membership confirm existing behavior ([decisions](../../docs/decisions.md#settled)).

## Default service access

Completed in 0.5.44; see [implementation and checks](done/empty-acl.md#checks).
The anchor stays for existing references. Personal tagging and the authority
changes below remain separate work.

## Personal services

Implement the [Personal service requirements](../../docs/03-services-and-topics.md#personal-and-shared).
Persist the owner's choice; omitted tagging remains non-Personal. Verify an
allowed service entry and a refused user entry, including a user with a backing
record, and refuse group entries (including a service-only group), the wildcard
and Maintainer assignments on a Personal service.
Show Personal services in the user's dedicated tab and the daemon owner's
per-user view, excluding them from the main web pages. Keep non-Personal fixture
records as positive controls on the main pages. For otherwise identical valid
configurations, toggling Personal must not change authorization or delivery.
Allowing a group entry must fail its named refusal check.
Ignoring the tag, accepting a forbidden assignment, mixing owners or treating
web filtering as access denial must fail its corresponding check.

## Inbox selection and filters

Implement [Q21's explicit inbox selector](../../docs/04-messaging.md#inbox-selection-and-filters)
across the CLI and daemon, with matching face semantics. Verify default own-inbox
reads, explicit authorized inbox reads, topic/tag filters on either, and refusal
of unauthorized inbox selection. Adding a tag or registering a channel whose
name matches a filter must not change the selected inbox. Restore implicit
selection from a filter and its named check must fail. Update help and callers
that relied on the former overload; keep documentation examples unambiguous.

## Reader visibility

Implement the web's [single Readers count](../../docs/05-discovery.md#readers).
Verify no waiters, one unfiltered waiter, a filtered-only waiter, and mixed
shared/filtered waits using nonzero fixtures. Completion, cancellation and
timeout must remove the corresponding wait from the count. Excluding filtered
waits or retaining an ended wait must fail a named check. Render one count,
without a filter breakdown; retain caller visibility and do not label it health
or completion. Typical services retain their full-inbox reader.

## Identity display labels

Implement the [web and human-readable CLI labels](../../docs/05-discovery.md#identity-labels-in-web-and-cli). Verify User, Agent and Service fixtures use the daemon’s identity facts, with visible text beside each glyph; role and health labels remain separate. JSON/API kinds and MCP output stay unchanged. Keep ACL textareas, CLI arguments and copyable examples in the existing plain-text syntax. A display glyph inserted into an editable ACL or a changed machine-readable kind must fail its check.

## Questions

No open choices are currently recorded in [QUESTIONS](QUESTIONS.md#open-questions); implementation gaps are not reopened policy questions. The [Future storage proposal](../Future/storage.md#storage) is not a remaining MVP database requirement.

## Authority model

Implement the pending changes in the [authority specification](../../docs/01-identity-and-roles.md#role-names-and-scopes); retain already-built permissions. Verify each dependent web control against the resulting authority before final browser acceptance.

| Work | Acceptance |
|---|---|
| Daemon-owner override and transfer | Owner can manage and transfer any service/channel; configured daemon ownership survives restart after transfer |
| Explicit setup ownership | Startup requires established ownership rather than deriving it from the runtime OS account |
| Profile editing and provenance | Enforce the [profile rules](../../docs/01-identity-and-roles.md#users-and-profiles), including protected names and permitted PersonName sources |
| Historical smoke fixture cleanup | Review existing `plain@srv1`, `chief@srv1` and `piped@srv1` live identities and chief's administrative standing before revoking/removing anything; [escaped provisioning evidence](done/administrator-names.md#live-verification-and-harness-correction) |
| Administrator unbanning | Administrator can unban an ordinary user, but cannot edit peer Administrators or the owner |
| Nested groups | Resolve nested membership with defined cycle handling; current membership is flat |
| Service-defined roles | Store and return service-defined labels without interpreting their application meaning while preserving owner-only assignment of the reserved Maintainer role; syntax/transport proposals are not automatically adopted |
