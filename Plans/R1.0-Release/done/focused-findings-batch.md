# Focused setup, install and webhook fixes

📌 **TL;DR:** Registry failures cannot trigger shared-template removal; package
builds receive no runner token; container releases normalize ownership/modes;
transient webhook startup failures exit for the existing runner backoff.
State-root staging leftovers use the existing safe sweep policy.

## Scope

One shipped-behaviour bump for this batch; one implementation commit per item.
No new retries, process groups, account changes or archive-builder ownership
changes. F3's framing allowance landed separately and is not part of this batch.

| Item | Implementation |
|---|---|
| G3 `dcf1d33d` | Return the `userRecords` error from `observeUser`, as refresh already does |
| F4 `e1c62b6d` | Set only the packager command's environment through `script.ChildEnv(os.Environ())`; the import is aliased `scriptEnv` because the local script path already uses `script` |
| Q1 `b87ff000` | Add only `--no-same-owner --no-same-permissions` to the Containerfile tar extraction |
| L2 `887a3f37` | Drop `BusError` from the inner caught tuple; the existing outer handler returns 1, while invalid Configuration remains a standing failure |
| T-27 follow-up `4babfb74` | Call the existing `sweepStale` on the open state root, then retain its users-directory sweep |

The runner's kept loop (`itself.go`) restarts ordinary nonzero exits with its
existing 1-to-30-second backoff. Exit 78 still retires until reload and is not
used for the webhook's transient startup error.

## Proof

Before-fix checks and compiling mutants ran only their owning assertions under
a separately acquired and read MCP lock held as `#ab-w1c`. All sources were
restored afterward.

| Broken behaviour | Named failing check | Assertion |
|---|---|---|
| Swallow registry listing errors | `TestUserUninstallRefusesARegistryListingFailureBeforePlanningRemoval` | Unavailable, malformed and transport-error listings each allow `unregister remove template` instead of refusing; 0.004 s |
| Inherit the packager environment | `TestBundledPackageBuildNeverReceivesTheRunnerToken` | A real `package.sh` sees the fake runner token; 0.014 s |
| Catch startup BusError as Configuration failure | `Startup.test_transient_configuration_or_secret_failure_exits_for_runner_restart` | Configuration and Secret cases return 0 instead of 1; two assertion failures, no test errors, 0.003 s |
| Restore old tar flags | `check-release-tree.sh` inside an image built with the actual Containerfile | The completed build keeps the fixture archive's uid/gid or writable modes; the named runtime assertion fails |
| Skip the state-root sweep | `TestPersonFilesSweepsStateRootStagingLeftBeforeUsersWasCreated` | Old empty `.new-stale` survives; 0.004 s |

The Q1 focused archive contains a manifest, version, inert executable stubs and
an ordinary probe, all with numeric uid 1234/gid 2345 and mode 0777. This is an
ownership/extraction test, not a daemon-role acceptance run. All three actual
Containerfile builds succeed: pre-fix runtime assertion red, fixed green,
drop-flags mutant runtime assertion red. The batch takes 17.553 s, and its own
containers/images and temporary archive are removed. The same runtime ownership
and group/other-write assertion is wired into the existing full release-image
acceptance, which will use real release binaries at the normal gate.

The state-root check also preserves fresh staging, a nonempty old staging
directory, a regular file, a link and its target, and an old ordinary directory.
All person-files checks pass (0.007 s). Restored G3 and F4 checks pass; the full
webhook suite passes 31/31 (3.411 s).

Full restored owning Go packages pass under race checking: setup 23.599 s,
runner 33.563 s. The worker-held lock is released after the checks. Changed
links and shell syntax checks pass.

After the final rebase, focused setup race checks pass (1.037 s), packager and
JSONL-framing boundary race checks pass (1.153 s), and webhook passes 31/31
(3.436 s). Explicit Podman inventories find zero `ab-w1c-q1-proof-*` containers
and zero image tags belonging to that fixture. The MCP lock is released.
