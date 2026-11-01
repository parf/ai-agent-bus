# MAC.2 missing role group recovery

## Scope

The CEO's 0.9.263 rerun stopped in preflight: `_agentbusd` existed without its group. The old teardown had removed the group while TCC blocked user deletion. Before this fix, `darwinLayout.ensureAccount` returned immediately for any existing user and did not restore its group.

Setup now keeps the user unchanged, reuses the matching group, or recreates only a missing group at that user's unused PrimaryGroupID and verifies it. A named group at another GID or another group owning the GID refuses with both identities named. Directory lookup errors refuse rather than triggering creation.

The [current run list](../mac2b-run-list.md#role-account-reuse) owns preflight and cleanup. Preflight allows a safe user-only state; after setup and at teardown the complete safe pair is required. Other leftovers still refuse. MAC.2 remains pending native archive acceptance; no Mac access or host changes were used for this work.

## Local checks

`TestDarwinExistingRoleRestoresMissingGroup` asserts exactly one `dseditgroup -o create -i 201` command and an unchanged user. Other command-stub tests cover GID clashes, the wrong named group, matching reuse, failed verification and lookup errors. `TestDarwinAcceptanceCleanupStubs` runs the Python suite in normal setup checks; it includes user-only acceptance, group-only and GID-clash refusals, plus both admin and wheel reads failing closed on a nonzero exit, stderr or absent GroupMembership.

All seven production mutants ran only the owning test under the slow lock, compiled and failed on assertions (no build failure or ERROR), then source was restored and the lock released:

| Mutant | Failing check and assertion | Seconds |
|---|---|---|
| Existing-user early return skips recreation | `TestDarwinExistingRoleRestoresMissingGroup`: commands empty, wanted only group creation | 0.709 |
| Skip GID clash refusal | `TestDarwinExistingRoleRefusesGroupIDClash`: clash not refused, nil error | 0.717 |
| Accept group-only leftover | `Cleanup.test_preflight_refuses_group_without_user`: exit 0 unexpectedly | 0.099 |
| Skip primary GID owner check | `Cleanup.test_preflight_refuses_primary_gid_owned_by_another_group`: exit 0 unexpectedly | 0.093 |
| Ignore privileged read exit | `Cleanup.test_privileged_group_read_error_is_refused`: admin and wheel unexpectedly exit 0 | 0.144 |
| Ignore privileged stderr | `Cleanup.test_privileged_group_stderr_is_refused`: admin and wheel unexpectedly exit 0 | 0.150 |
| Treat missing membership as empty | `Cleanup.test_privileged_group_missing_membership_is_refused`: admin and wheel unexpectedly exit 0 | 0.145 |

Restored Python suite: 28/0; focused `TestDarwin` suite passes, including its normal owning Python wrapper. These are local stubs; native recovery is not claimed.

## Native directory output and fail-closed review

The CEO supplied read-only Mac directory output on 2026-11-01: a group search prints `staff` followed by two tabs and `PrimaryGroupID = (` on the first line, `    20` on the next and `)` on the last; a search for unused GID 99999 has empty stdout and exit 0. The user record prefixes `IsHidden` with `dsAttrTypeNative:`. The stubs retain these bytes verbatim, alongside the one-line scalar form. Preflight parses complete records and refuses unrecognised output rather than guessing an owner.

Only `user.UnknownUserError` lets setup create a fresh user. Darwin's Go `os/user` uses `getpwnam_r` (also without cgo): ENOENT or a successful lookup without a result becomes that type; other errno values return an error. It does not return raw dscl `-14136`/`-14120` codes. A lookup failure therefore refuses setup instead of treating the existing account as absent.

Additional command-stub checks cover a failed user lookup, failed GID lookup without creation, nonnumeric PrimaryGroupID, a created group at GID 999 instead of 201 (both named), and an explicit dseditgroup failure (the error names the uncreated group and preserves its cause). Readback failure still says `was not created`. These close the reader's surviving-mutant coverage gaps. In the revised locked batch **16/16 mutants failed on assertions; zero survived**, all compiled and none errored. The original seven were rerun and killed. Additional results:

| Mutant | Owning assertion | Seconds |
|---|---|---|
| Ignore a non-unknown user lookup error | `TestDarwinExistingRoleRefusesUserLookupFailure`: failed lookup accepted | 0.708 |
| (c) Ignore a non-unknown GID lookup error | `TestDarwinExistingRoleRefusesGroupIDLookupFailure`: failed GID lookup accepted | 0.698 |
| (d) Ignore invalid PrimaryGroupID | `TestDarwinExistingRoleRefusesInvalidPrimaryGroupID`: invalid GID accepted | 0.736 |
| (f) Accept wrong created GID | `TestDarwinExistingRoleRefusesWrongCreatedGroupID`: wrong created GID accepted or not named | 0.742 |
| Ignore dseditgroup failure | `TestDarwinExistingRoleReportsGroupCreationFailure`: creation failure not named | 0.736 |
| Restore one-line-only search parsing | `Cleanup.test_preflight_accepts_expected_hidden_nonlogin_role`: exit 1 instead of 0 on native output | 0.110 |
| Ignore the native attribute prefix | Same valid-role assertion: exit 1 instead of 0 | 0.113 |
| Bypass safety for a user-only record | `Cleanup.test_user_only_still_requires_all_role_safety_checks`: ten unexpected exit-0 assertions | 0.527 |
| Permit the missing group at teardown | `Cleanup.test_teardown_requires_the_complete_role_pair`: exit 0 instead of 1 | 0.113 |

The readback-not-created test retains its message assertion. Sources were restored after each mutant and the lock released immediately after the batch. Final baseline: Python 32/0; whole setup package PASS in 16.652 s; restored focused Darwin checks pass. No native acceptance is claimed.
