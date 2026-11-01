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
