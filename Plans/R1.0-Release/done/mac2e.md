# MAC.2 acceptance keeps its role account

## Decision and scope

Owner, 2026-11-01 13:58: repeated deletion of `_agentbusd` is unnecessary. The [current run list](../mac2b-run-list.md#role-account-reuse) owns preflight and cleanup; [decision](../DECISIONS.md#recorded-decisions) supersedes Q298. Setup already returns without creating an account when `_agentbusd` exists (`darwinLayout.ensureAccount`); this change touches acceptance tooling and docs only, not shipped setup.

## Existing native evidence

Fresh role creation was proven by the 2026-10-31 full-archive run at `2214ba4e`, 0.9.242, arm64: [saved unaltered log](mac2d-native-2026-10-31.log), line 85, `PASS role account, launchd identity and private storage`. The prior preflight required both role records absent; setup then created the role and the run verified the daemon's dedicated non-root identity and private storage. The later SSH `sysadminctl` failure records the old cleanup policy, not a reason to recreate the account on every rerun.

MAC.2 remains pending until the CEO runs the updated archive acceptance. No native Mac access, host changes or sudo was used by the worker.

## Local checks

The owning cleanup/archive stub suite verifies a valid reusable role, absent role pair, refusals for login shell, visibility, IDs, home, incomplete pair or directory lookup failure, and every other preflight leftover. Teardown keeps the account with an explicit message and exit 0, still refusing foreign command links, remaining paths/jobs and preserving earlier acceptance failures. The normal setup check invokes this suite through `TestDarwinAcceptanceCleanupStubs`.

- Accepting any existing account: the login/visibility test fails two assertions (`0 == 0`), 0.084 s.
- Restoring the old existing-account refusal: the correct-role acceptance fails its assertion (`1 != 0`), 0.058 s.
- Skipping other setup-path refusals: six leftover-path assertions fail, 0.315 s.

Each mutant ran only its owning test under the slow lock, compiled and failed on assertions (no ERROR); source restored and lock released immediately. Restored cleanup/archive suite: 20/0; owning Go wrapper: PASS. No shipped program behavior or version changes.

## Review delta

Preflight additionally checks direct `GroupMembership` in both `admin` and `wheel`, refusing a privileged role. Tests isolate UID boundaries 0, 199 and 400 with matching group IDs. Teardown rechecks the kept account before printing its success message; a role changed to a login shell fails cleanup. Current setup/TODO and historical MAC.2/MAC.2d notes now point explicitly to this replacement policy. Restored suite: 22 tests; owning Go wrapper passes. Review mutants each fail on assertions, no ERROR: skipping privileged memberships gives two failures (admin and wheel); dropping the UID lower bound gives two failures (0 and 199); skipping the teardown safety recheck gives one failure. All sources were restored and the slow lock released.
