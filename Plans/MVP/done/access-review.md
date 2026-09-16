# Known-principal access review

## Scope

Historical review on 2026-09-15 during the concurrent access revision, based on
`45aaac9` plus the in-flight access changes. The review followed API guards,
credential issuance, registration/configuration, removal, browser sessions,
mapped sockets and client provisioning. It is not a claim that every line of
all programs was reviewed or that the remaining MVP access gates are complete.
The [access contract](../../../docs/02-access.md#what-a-call-carries) and
[remaining work](../TODO.md#remaining-work) own current requirements and status.

## Findings

| Finding | Evidence at review | Disposition |
|---|---|---|
| Unknown names previously authenticated on an empty lifecycle state | API probes with a stored credential but no profile or record | Strict access revision removes self-registration and refuses token issuance for an unknown name; initial gates verified below |
| Configuration bypassed protected-realm registration | An ordinary user received refusal from `/register`, then success from `/configure` and `/token` for the same unproved name | Reproduced through API; handed to Claude for the common creation check |
| A removed caller could create an orphan | Request admitted while its caller's record existed; body reader removed that record before configuration executed | Reproduced through API; caller eligibility must be checked at creation under the registry lock |
| A removed caller could recreate itself | Same interleaving, targeting its own name through both registration and configuration | Reproduced after the first configuration fix; general self-creation exception remained reachable without enrolment |
| Token existence check and mint were separate | `Known` released the registry lock before `Issue`; removal can fit between them | Source interleaving, not a stress-test reproduction; handed to Claude for atomic issuance |
| Issuance authority can change after its check | Ownership lookup preceded the proposed locked issuance helper | Follow-up review: the locked decision must include caller authority, not only target existence |
| SSH key provisioning does not create a bus user | Admin `user add` writes the key entry; strict token issuance requires an existing name | Recorded as H.5.9; generated-key checks do not prove fresh-user onboarding |
| Paused/banned owners' service credentials need propagation | Authentication checks the service name's state, without the owner's state | Existing H.5.7 requirement remains pending; no new policy question |

Enrolment remains the separately documented
[proof-of-key path](../../../docs/01-identity.md#proving-possession), backed by an
operator-configured directory. The review did not replace it with an anonymous
self-registration exception. No user-delete operation was added.

## Harness migration

`009c674` changes only `src/smoke.sh`. Fixture users are created explicitly;
where an inboxless actor is required, that user's own credential unregisters
only its backing record. Service fixtures are provisioned by an existing
principal before asking for their credentials. `tok()` remains mint-only.

| Verification | Result and limit |
|---|---|
| Fast run | 422 passed, 0 failed |
| Full isolated `src/smoke.sh --slow` | 543 passed, 0 failed, including race, MCP and launchers; frozen access snapshot before later audit fixes |
| Remove the token target's existence check | Full mutation run: three named shell assertions failed on mint, rotation and the resulting stored credential; the Go check failed too |
| Remove mapped-socket authentication | Six named shell assertions failed; illicit self-registration then caused fixture creation to abort. This was a caught mutation, not a completed full run |
| Every private API route | All 24 routes refused an unknown token, unknown browser session and mapped unknown socket: 72 refusals, each 401; no record created. This checks initial admission, not concurrent state changes |
| Independent review | Oab read the harness diff and accepted the fixture semantics; Codex reviewed that advice and independently reproduced the creation bypasses |

## Reproduction

Ignored scratch under `tmp/q57-review/` contains `fast.log`, `slow.log`,
`mint-unknown/log`, `socket-bypass/log`, and `access_audit_test.go` with Go
overlays. The audit probes intentionally assert the reproduced defect;
invert them when promoting them to regression tests. Each reported API
reproduction ran a named test; an initial overlay path mismatch ran no tests
and supplied no evidence.

The immutable snapshot is `tmp/qwt`. Its source was not edited during any
smoke invocation. Production data was untouched and no release was installed
by this review. Later combined verification and deployment need their own
record; the isolated result above does not certify them.
