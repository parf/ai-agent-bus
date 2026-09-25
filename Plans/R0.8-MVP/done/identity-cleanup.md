# Directory classification and cleanup

## Scope

Implementation evidence for the directory part of F.13.1/F.13.4, with owner
and maintainer cleanup authority settled by Q56. Current contracts are
[person records](../../../docs/01-identity-and-roles.md#users-and-profiles) and
[credential cleanup](../../../docs/02-access.md#ownerless-credentials).
This does not close the remaining web redesign, H.5.5 orphan-service deletion,
or installed F.12 acceptance.

## Verification

| Check | Evidence |
|---|---|
| Identity and caller scope | `TestDirectoryClassifiesFactsAndPreservesCallerScope`: blank and test-like profiles remain users; records and credential-only names remain visible without invented lifecycle; ordinary and inactive maintainer views cannot enumerate other identities |
| Current eligibility | `TestIdentityCleanupRechecksAuthorityAndCurrentState`: owner and maintainer success; ordinary-user denial; GET is nonmutating; a profile, record or owned service added after listing prevents removal |
| Concurrent registration | `TestCleanupSerializesRegistrationWithCredentialRemoval`: record/profile creation cannot commit while the credential removal is being persisted |
| Persistence and sessions | Auth change `b91fa61`, reviewed with Claude; failed writes preserve credentials, successful removal ends both token generations and only that name's sessions, including the absent-token case |
| Directory journey | `TestDirectoryShowsJunkWithoutCallingItUsers`: separate visible categories, preserved caller scope, search/pagination/return state, protected records, explicit cleanup, origin and return checks; directory loads status and users once, without avatar requests |
| Browser fixture | Real browser at desktop and narrow viewport, populated with registered users, self-owned records, retained service owners and unused credentials. One main landmark; narrow page does not overflow; explicit removal reduces other-identity count by one and leaves adjacent entries visible. Disposable fixture only |
| Independent review | Claude reviewed authority, persistence and stale-list requirements; oab reviewed core/API implementation read-only. Inactive-maintainer listing gap corrected; full-store write under the core lock is an explicit tradeoff for atomic eligibility and removal |
| Repository acceptance | Final immutable worktree at `b91fa61` plus this slice: `PORT=23911 src/smoke.sh --slow`, **534 passed, 0 failed**, including race checks and MCP/launcher checks. Docs checker: **95 files, 2250 local links, 0 errors** |

## Mutations

Isolated source copies were changed, never the live checkout or installation.

| Mutation | Failing check |
|---|---|
| Remove caller filtering | `TestDirectoryClassifiesFactsAndPreservesCallerScope` |
| Let an inactive maintainer enumerate identities | `TestDirectoryClassifiesFactsAndPreservesCallerScope` |
| Give non-users an active lifecycle | `TestDirectoryClassifiesFactsAndPreservesCallerScope` |
| Skip removal eligibility recheck | `TestIdentityCleanupRechecksAuthorityAndCurrentState` |
| Release the core lock before credential persistence | `TestCleanupSerializesRegistrationWithCredentialRemoval`, both record and profile cases |
| Hide the other-identity section | `TestDirectoryShowsJunkWithoutCallingItUsers` |
| Drop return/filter state after cleanup | `TestDirectoryShowsJunkWithoutCallingItUsers` |
| Bypass both form-origin checks | `TestDirectoryShowsJunkWithoutCallingItUsers` |

Removing only the form handler's origin check survived: the dashboard's outer
middleware independently rejects the same request. That surviving mutation is
not claimed as evidence for the inner check.

## Installed verification

Commit `12c109f` deployed from the tested immutable tree. All six Go programs
report `0.5.28`, build `parf@parf.us 2026-09-15 22:01:00`; the running supervisor,
bus and web executables were checked after the systemd restart.

| Check | Observed |
|---|---|
| Retained data | One registered user, five credentials, four agent records; owners unchanged and all four readers reattached |
| Installed directory | Authenticated `/users` shows both identity categories, one closed main landmark and no avatar requests |
| Verification isolation | Browser mutations used disposable fixtures only; installed checks created and then ended one temporary browser session |

This checks the deployed directory slice, not the remaining installed F.12
acceptance. The earlier startup cleanup was deployed separately; this restart
is not evidence of another garbage cohort being removed.

## Remaining work

Service/channel journeys, form preservation, broader visual polish and the
remaining acceptance belong to [F.13](../TODO.md#web-redesign). The directory
does not infer historical origin for credential-only names or decide that a
name is test debris from its spelling. Orphan-service deletion remains the
[pending daemon task](../TODO.md#remaining-work).
