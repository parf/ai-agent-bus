# G.1.1 — the daemon never executes what a user supplied

Verified at 0.5.37. No behaviour changed and no version was bumped: this is
acceptance for a boundary the daemon already holds. Evidence for the row as it
stood in [TODO](../TODO.md#remaining-work) after it was corrected.

## The row was wrong before the work started

It asked for a blanket prohibition on child exec — that an attempted child exec
be refused, and that removing "the exec restriction" fail that check. No such
restriction exists and the contract does not ask for one.
[Q30](../../../docs/decisions.md#settled), settled 2026-09-15, put the boundary
at **no user services, not no exec**: a fixed verifier the daemon ships and
invokes with arguments it built is not a stranger's program, and collapsing the
two would buy nothing but a literal claim
([the contract](../../../docs/11-processes.md#nothing-the-daemon-runs-may-exec)).

codex declined to start until the row matched the settled boundary rather than
widening the task to fit it. That is the finding worth keeping: acceptance
written against a superseded decision reads exactly like acceptance written
against the current one.

## What the daemon actually does

| | |
|---|---|
| the supervisor | executes its own children, which the boundary allows and names |
| the signature verifier | `exec.Command("ssh-keygen", "-Y", "verify", …)`, no shell, arguments built by the daemon |
| user scripts | run in the separate foreground runner, not in the daemon |
| anything else | no service execution entry point was found in the daemon-facing packages |

## What proves it

[`exec_boundary_test.go`](../../../src/internal/api/exec_boundary_test.go), over
the real HTTP API.

| Claim | Test |
|---|---|
| a user-supplied value stays data through registration, configuration and messaging | `TestUserServiceInputsRemainData` |
| a real signed enrolment still succeeds through the shipped verifier | `TestSignedEnrolmentUsesTheShippedVerifier` |

The first plants a real executable that writes a unique marker, **proves the
marker appears when it is run**, then removes it — so the absence checked
afterwards is the absence of execution rather than the absence of a working
probe. The program path is then registered as a description and an address with
a protocol hint of `exec`, stored in a configuration alongside script text, and sent and
consumed as a message body including shell-substitution text, with the marker
checked absent after each phase. The second generates real Ed25519 keys and a
directory file: a wrong key is refused `403` leaving no record, and a correctly
signed nonce answers `200` with a self-owned record and a credential that then
authenticates as that name. `--slow` enrols against a real daemon process
separately.

**Five isolated mutations, each caught by a named assertion**
(`tmp/g11/mutations.log`, replayable with `tmp/g11/mutations.py`): executing the
supplied value at registration, at configuration and at message handling, each
caught by its own phase's assertion; disabling the `ssh-keygen` verifier, caught
by the positive enrolment turning `403`; and accepting any signature, caught by
the wrong key answering `200`.

The mutations run the two named API tests, not the full smoke suite.
Full `--slow`: **573 passed, 0 failed**, exit 0 (`tmp/g11/slow.log`).

## What this is not

Measured on the installed unit, read-only: bus and web run with
`NoNewPrivs=1`, `Seccomp=0` with no filters, and empty effective and ambient
capability sets; the unit sets no `ExecPaths` or `NoExecPaths` and
`SystemCallFilter=~`.

So this is **an application boundary, not an OS-enforced one**. Nothing stops a
future handler from calling `exec`, and none of this is proof against a
compromised daemon. A helper that executes an unrelated binary under the service
account does not violate the rule as settled. An OS-enforced executable
allowlist would be a new requirement, and its exception for the verifier needs a
concrete mechanism before anybody implements it.

opencode reviewed and closed with two limits recorded: the inputs exercised are
the ones named above rather than every input the daemon accepts, and the checks
are synchronous, so execution deferred beyond the assertion would not be seen.
