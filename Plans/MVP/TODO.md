# TODO MVP

## Objective

Finish the [MVP scope](README.md#scope). Built wave results are in [DONE](DONE.md#done--mvp); historical tasks retain their IDs in the [archived plan](done/TODO-before-rewrite.md#mvp-plan-before-the-documentation-rewrite).

## Next step

Resolve the [method metadata, packaging and people-policy questions](QUESTIONS.md#open-questions). Installed acceptance preparation can proceed independently. Implementation remains pending where an owner decision is needed; this rewrite does not choose a schema or package format.

## Remaining work

| ID | Deliverable | Depends on | Acceptance and mutation |
|---|---|---|---|
| F.2 | Generated service method information | Q7 | A registered service's method description reaches both faces; removing metadata propagation must lose that description and fail. An ordinary service remains callable as a control |
| F.6 | People view and maintainer-controlled profiles | Q9, Q28, Q29; [pending people](../../docs/01-identity.md#pending-person-records) | An allowed maintainer updates a profile visible in the view; the subject and a peer maintainer cannot. Bypass each write check separately and watch the corresponding refusal fail; render no profiles and the positive check fails |
| F.6.1 | Identifier normalization and uniqueness | Q28, F.6 | Two equivalent spellings cannot claim different people; distinct valid identifiers still work. Bypass normalization or uniqueness separately and the duplicate rejection must fail |
| H.1 | Distributable package and installation instructions | Q10; all shipped programs and faces | A new user installs on a fresh host and calls a service following only the instructions. Remove a required binary or face artifact from the package and the same exercise must fail |
| H.5.1 | ACL and local account mapping administration | Q9 | An operator's change affects both visibility and use, survives restart, and leaves an unrelated user unchanged. Disable persistence or enforcement separately and the corresponding check fails |
| G.1.1 | Resolve and verify the daemon exec boundary | Q30 | Acceptance follows the approved boundary; if exec is prohibited, a live enrolment must still succeed while an attempted child exec is refused. Removing the exec restriction must fail the refusal check |
| G.1.2 | Verify dashboard resource confinement | [process boundary](../../docs/11-processes.md#the-rule) | Under the installed unit, drive the web child to its configured resource limit while bus calls continue. Remove the web limit and the limit assertion fails; a dead web child cannot satisfy the positive control |

## Installed stage gate

Setup and the supervisor are already built. Prepare an isolated systemd host before accepting local isolation; B's socket ownership depends on that installation, not on packaging being the last task. Development checks alone do not close this gate.

| Gate | Dependencies | Required evidence and mutation |
|---|---|---|
| Account and state | H.2–H.4 built; [setup contract](../../docs/09-setup.md#the-two-accounts) | Inspect running uid, home and state ownership. Start as the installing user instead and the check fails |
| Per-user sockets | B.2 and G.1 built; account gate | Two actual OS accounts can use their own sockets and cannot use each other's. Wrong ownership or permissive mode must fail a negative check while each positive call still succeeds |
| Capabilities | Account gate; [capability boundary](../../docs/11-processes.md#why-the-supervisor-holds-cap_chown) | Inspect the live supervisor and bus capability sets under the unit. Giving the bus the supervisor's capability or removing it from the supervisor must fail; an unprivileged development run is not evidence |
| Fresh installation | H.1; process boundary and confinement work | Run the package acceptance above and retain the commands, results and host conditions in the completion evidence |

## Questions

All unresolved choices are owned by [QUESTIONS](QUESTIONS.md#open-questions). The CLI selector question remains open without silently blocking unrelated work. The [Future storage proposal](../Future/storage.md#storage) is not a remaining MVP database requirement.
