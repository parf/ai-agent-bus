# MAC.2 account-only teardown partial

📌 **TL;DR:** The native full-archive run passed its runtime checks but left `_agentbusd` after `sysadminctl` returned `-14120` over SSH. The harness now names that account-only partial and exits 3; it never calls it clean. MAC.2 stays pending the revised run and CEO confirmation of local-terminal account removal.

## Native run

The [unaltered log](mac2d-native-2026-10-31.log) is copied byte-for-byte from `tmp/mac2b-dist/result.log`. It records revision `2214ba4e1d4697592018065595ffba9b92da318e`, version `0.9.242`, arm64, the 2026-10-31 01:09 Mac run, exit 1 and 26.662 s. SHA256: `38bfbe1b728dea83531fa81537d775c177793aa9eed424b7932e2fba097a1017`.

The six PASS lines cover checked archive bytes and stamps, archive setup without `--exec`, role/storage, pinned-TLS exactly-once request/reply, launchd restart and abrupt-supervisor-exit recovery. Teardown invokes the existing removals, then `sysadminctl` prints `### Error:-14120` and the user remains. The old harness raises before its final absence assertions; this log establishes a failed teardown, not complete cleanup. The CEO reports that everything else was removed. The retained log contains no token or private-key value.

## Partial result

Only a positively verified leftover user plus `-14120` from its deletion, with the group absent, can produce the named account-only exception. Teardown defers that exception until all remaining path/parent checks pass. Any other cleanup error remains a normal failure; a pre-existing acceptance failure is preserved. Exit 3 prints the exact local command and requires `dscl . -read /Users/_agentbusd` to fail with record-not-found. No sudo invocation, permission change or alternate deletion route is added.

The [owner run list](../mac2b-run-list.md#account-removal-blocked-over-ssh) names the local-terminal step. The worker has not contacted the Mac or run a host privileged command. MAC.2 remains ⏳ until the rerun log and CEO account-absence confirmation.

## Local checks

- New stub check requires exit 3, the named account, local removal command and verification command, no clean-uninstall PASS, and group inspection.
- Group deletion failure, directory lookup failure, a leftover setup path, and a leftover without the specific error still require ordinary exit 1.
- Disabling TCC recognition fails the named partial-exit assertion (1 != 3, one failure/no ERROR, 0.059 s).
- Relaxing the setup-path assertion for a partial fails the strict teardown check (3 != 1, one failure/no ERROR, 0.206 s).
- Restored cleanup/archive tests: 20/0 in 1.191 s. Both mutants ran only owning checks under the slow lock; sources restored and lock released immediately.
