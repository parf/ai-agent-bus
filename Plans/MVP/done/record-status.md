# Record status

📌 **TL;DR:** 0.7.9 replaces paused, banned and the record `disabled` switch
with one status, active or inactive, on Users and records. An inactive one is
no such entity: hidden, `404` to every caller, its name reserved, readable
only in the web face's read-only view and brought back by its Owner or a
Maintainer with a status-only edit. Evidence for
[K.11 and K.12](../DONE.md#done--mvp).

## Result

| Area | Built |
|---|---|
| Liveness | a record is live when its own status is active and its User is; `entity()` is the lookup every operation uses |
| Caller | an inactive User, or an agent inactive itself or through its User, is refused `403 suspended` |
| Target | an inactive record answers `404 unknown` to every operation but its reactivation, the daemon Owner included; no drain |
| Name | registration, configuration and creation over an inactive record's name are refused |
| Reactivation | a status-only edit by the Owner or a Maintainer; a 👤 record takes its status from its User |
| Users | an Administrator changes ordinary Users only, the daemon Owner anyone, and stays active |
| Fan-out | a failed recipient beside live ones is its own `dropped` and an error-log warning; a publication none takes is refused, nothing stored or counted |
| View | `GET /inactive`, shown to whom the ACL admits and always to the daemon Owner |
| Web | Deactivate…/Reactivate for Users and records; Active/Inactive filters fed by `/ls` and `/inactive`; Overview "Inactive and work is held" |
| Also | review fixes: a transfer takes the agent off its old Owner's Personal records; `@administrators` takes Users only; a restore ignores and reports an administrator who is no User instead of manufacturing one |

## Checks

| Check | Mutation that fails it |
|---|---|
| `core` TestAnInactiveRecordIsNoSuchEntity | listing inactive records; registering over one; taking a non-status edit |
| `core` TestOnlyTheOwnerOrAMaintainerReactivates, TestTheInactiveViewShowsWhomTheACLAdmitsAndTheOwner | the view shown to strangers |
| `core` TestAnInactiveCallerIsSuspendedAndAnInactiveTargetUnknown, TestWhoChangesAUsersStatus | an inactive User or agent acting |
| `core` TestAPublicationAnInactiveRecipientCannotTakeCountsAsItsDrop, TestAPublicationNoRecipientTakesIsRefused | a dead recipient skipped silently; an all-failed publication accepted |
| `core` TestPolicyChangesCancelBlockedReaders | a reader not released when its inbox goes inactive |
| `core` TestARestoredInactiveRecordStaysInactive | an unknown status accepted at load |
| `api` TestAnInactiveOwnersRecordsAreNoSuchEntity and the rest of suspended_owner_test | the User's status not reaching its records |
| `core` TestATransferTakesTheAgentOutOfItsOldOwnersPersonalRecords, TestAdministratorsTakeUsersOnly, TestRestoreIgnoresAnAdministratorWhoIsNoUser | each review fix removed |
| web: TestRecordDeactivationIsConfirmedByItsManagerOnly, TestOverviewNamesWorkHeldByAnInactiveRecord and the user journeys | the `/inactive` merge, the inactive-detail fallback, the deactivate guards, the Reactivate control |
