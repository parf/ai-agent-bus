# TODO MVP

## Objective

Finish the [MVP scope](README.md#scope). Built wave results are in [DONE](DONE.md#done--mvp); historical tasks retain their IDs in the [archived plan](done/TODO-before-rewrite.md#mvp-plan-before-the-documentation-rewrite).

## Next step

Resolve the [method metadata, packaging and local-account administration questions](QUESTIONS.md#open-questions). Complete live-runtime and installed acceptance of the built launchers under the [delivery contract](../../docs/08-runner-role.md#runtime-integration-delivery). This plan does not choose a schema or package format.

## Remaining work

The required dashboard has [implementation and mutation evidence](done/owner-controls.md#verification); resource confinement still needs implementation and installed acceptance, as the [installed inspection](installed-acceptance.md#resource-confinement) confirms.

H.8 and H.9–H.9.3 have [built implementation and automated evidence](done/launcher-implementation.md#automated-verification); their rows retain the full live and installed acceptance requirements.

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| F.2 | Generated service method information | Q7 | A registered service's method description reaches both faces; removing metadata propagation must lose that description and fail. An ordinary service remains callable as a control |
| H.1 | Distributable package and installation instructions | Q10; all shipped programs and faces | A new user installs on a fresh host and calls a service following only the instructions. Remove a required binary or face artifact from the package and the same exercise must fail |
| H.8 | [Runtime integration delivery](../../docs/08-runner-role.md#runtime-integration-delivery) | Built adapters; H.1 for installed acceptance | Install into clean runtime profiles from the shipped artifacts. A bus message reaches each live interactive session and a correlated reply returns. Remove channel activation or Codex tool configuration separately and the relevant exchange fails; an isolated headless reply does not count |
| H.9 | [Smart launchers](../../docs/08-runner-role.md#smart-launchers) | Built adapters and V2 credentials; H.8 assets; H.1 for installed acceptance | From a path containing spaces, launch with and without prior history and verify the intended conversation, exact forwarded arguments and active integration. Force resume on an empty history, drop an argument or attach the pusher to a different App Server: each fails its corresponding check. Two concurrent launches must receive only their addressed messages; reuse one launch's endpoint or identity and the isolation check fails |
| H.9.1 | Launcher failure handling | H.9 | Exercise missing runtime, absent bus configuration, helper startup failure and runtime exit. Check diagnostics, plain-session fallback, exit status and cleanup while a separate session stays alive. Suppress readiness failure, force a zero exit status or remove cleanup separately: each fails its check. Supply contrary runtime mode options and verify the [enforced mode](../../docs/08-runner-role.md#smart-launchers) still applies; removing the enforcement must fail |
| H.9.2 | [MCP minimum](../../docs/05-discovery.md#mcp-minimum) in both runtimes | H.8, H.9; existing MCP face | In each installed, launcher-started session, list a known allowed service and call it for a unique response. Hide a known forbidden service and refuse a direct call to it. Remove tool loading, replace the listing with empty output, drop the response or bypass the ACL separately: each fails its corresponding check. An acceptance receipt cannot satisfy the service-response assertion |
| H.9.3 | [Assigned session names](../../docs/08-runner-role.md#session-names) | H.9; verify each runtime's name/session lookup | Check explicit bus identity, an assigned runtime name and missing-name fallback separately. Ignore an available assigned name or remove fallback and the corresponding assertion fails. Launch sessions with colliding titles or derived names and deliver a unique message to each; merge their bindings and delivery fails. Rename during a live launch with a reply pending: changing its inbox or credential must fail reply delivery and identity assertions. On restart, verify the documented address-change and old-inbox behavior; removing the rename, discarding queued messages or changing an explicit address must fail its corresponding check |
| H.9.4 | `ab-opencode` launcher and opencode push adapter | H.9; opencode's server API | Launch a session, deliver a bus message to it from another terminal and get a correlated reply. Break the server handshake, the config injection or the session binding separately: each fails its corresponding check. A headless session the person is not attached to does not count |
| H.5.1 | ACL and local account mapping administration | Q9 | An operator's change affects both visibility and use, survives restart, and leaves an unrelated user unchanged. Disable persistence or enforcement separately and the corresponding check fails |
| G.1.1 | Resolve and verify the daemon exec boundary | Q30 | Acceptance follows the approved boundary; if exec is prohibited, a live enrolment must still succeed while an attempted child exec is refused. Removing the exec restriction must fail the refusal check |
| G.1.2 | Implement and verify dashboard resource confinement | [process boundary](../../docs/11-processes.md#the-rule) | Under the installed unit, drive the web child to its configured resource limit while bus calls continue. Remove the web limit and the limit assertion fails; a dead web child cannot satisfy the positive control |

## Installed stage gate

Setup and the supervisor are already built. Prepare an isolated systemd host before accepting local isolation; B's socket ownership depends on that installation, not on packaging being the last task. Development checks alone do not close this gate.

The [development-host inspection](installed-acceptance.md#inspection) records partial positive evidence; mutation checks and independent-host acceptance remain open.

| Gate | Dependencies | Required evidence and mutation |
|---|---|---|
| Account and state | H.2–H.4 built; [setup contract](../../docs/09-setup.md#the-two-accounts) | Inspect running uid, home and state ownership. Start as the installing user instead and the check fails |
| Per-user sockets | B.2 and G.1 built; account gate | Two actual OS accounts can use their own sockets and cannot use each other's. Wrong ownership or permissive mode must fail a negative check while each positive call still succeeds |
| Capabilities | Account gate; [capability boundary](../../docs/11-processes.md#why-the-supervisor-holds-cap_chown) | Inspect the live supervisor and bus capability sets under the unit. Giving the bus the supervisor's capability or removing it from the supervisor must fail; an unprivileged development run is not evidence |
| Fresh installation | H.1, H.8 and H.9–H.9.3; process boundary and confinement work | Run the package and runtime integration acceptance above on a host without `/rd` or the checkout; retain the commands, results and host conditions in the completion evidence |

## Questions

All unresolved choices are owned by [QUESTIONS](QUESTIONS.md#open-questions). The CLI selector question remains open without silently blocking unrelated work. The [Future storage proposal](../Future/storage.md#storage) is not a remaining MVP database requirement.
