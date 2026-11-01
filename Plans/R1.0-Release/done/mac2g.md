# MAC.2 missing role group recovery

## Scope

The CEO's 0.9.263 rerun stopped in preflight: `_agentbusd` existed without its group. The old teardown had removed the group while TCC blocked user deletion. Before this fix, `darwinLayout.ensureAccount` returned immediately for any existing user and did not restore its group.

Setup now keeps the user unchanged, reuses the matching group, or recreates only a missing group at that user's unused PrimaryGroupID and verifies it. A named group at another GID or another group owning the GID refuses with both identities named. Directory lookup errors refuse rather than triggering creation.

The [current run list](../mac2b-run-list.md#role-account-reuse) owns preflight and cleanup. Preflight allows a safe user-only state; after setup and at teardown the complete safe pair is required. Other leftovers still refuse. MAC.2 remains pending native archive acceptance; no Mac access or host changes were used for this work.

## Local checks

`TestDarwinExistingRoleRestoresMissingGroup` asserts exactly one `dseditgroup -o create -i 201` command and an unchanged user. Other command-stub tests cover GID clashes, the wrong named group, matching reuse, failed verification and lookup errors. `TestDarwinAcceptanceCleanupStubs` runs the Python suite in normal setup checks; it includes user-only acceptance, group-only and GID-clash refusals, plus both admin and wheel reads failing closed on a nonzero exit, stderr or absent GroupMembership.

Mutation results are recorded after the locked owning-test batch.
