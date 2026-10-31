# Full Darwin archive build

📌 **TL;DR:** MAC.2b builds both full and client archives for Apple Silicon and Intel. Full archives carry the daemon and reduced setup; clients exclude service programs. Cross-build checks pass; MAC.2 remains pending until the CEO's archive-based native run passes.

## Delivered behavior

- `package.sh` builds all five Go programs for Darwin without cgo, plus the compiled TypeScript face and existing runtime files. Both editions carry manifests and archive checksums.
- `release-verify.sh` accepts full Darwin archives and checks Mach-O CPU headers, version bytes and stamps on this foreign host; native programs keep their version queries. The checksum collector takes optional complete Darwin full/client CPU pairs.
- npm Darwin packages remain client-only; their builder and publication path are unchanged. No publish, login, release or host operation ran here.
- The acceptance harness takes `--archive`: it checks the delivered bytes and native version answers, runs the archive's setup without `--exec`, then runs the original MAC.2 checks. Cleanup verifies exact created command-link targets, removes the fresh release tree, uses `sysadminctl`, and treats an already removed group as success.

## Checks

| Check | Evidence |
|---|---|
| New archive test before packaging fix | Assertion-red: `output must contain both full and client archives and their hashes` |
| Real Darwin artifacts | `package-darwin.test.sh` passes full/client builds for both CPUs, daemon inclusion/exclusion, manifest/checksum, Mach-O CPU, stamp/version and launchd setup-path checks |
| Same archive reader as the owner run | Both actual full archives pass `archive_bundle`; runtime files are checked by the manifest, with the existing non-runtime review-settings example exception; only the four exact inward launcher aliases may be links |
| Verifier refusals | Full Darwin fixture accepted; corrupted daemon header with otherwise valid version/stamp/manifest refused; existing release refusal checks pass |
| Checksum collection | Four `TestReleaseSums*` tests pass, including incomplete/full CPU-pair refusal and complete eight-archive set |
| Acceptance fixture | Fourteen local stub/archive checks pass through existing `TestDarwinAcceptanceCleanupStubs`; no Mac or sudo invoked |

## Mutation evidence

All seven mutants ran on this node under one 15-minute `slow` lock, only through their owning checks; each source file was restored in `finally`. The lock was released immediately after the batch. No build or load error is credited.

| Mutant | Assertion-red result | Seconds |
|---|---|---|
| Put daemon in client tar | `package-darwin.test.sh`: `client archive holds agent-busd` | 13.219 |
| Remove daemon from full tar and manifest | `package-darwin.test.sh`: `full archive missing agent-busd` | 9.008 |
| Bypass daemon Mach-O header check | `release-verify.test.sh`: `wrong Darwin daemon header accepted` | 2.356 |
| Accept only the present half of a full CPU pair | `TestReleaseSumsTakesTheMacOSFullBothOrNone`: `partial full Darwin pair`, unexpected success | 0.288 |
| Skip outer archive hash | `Archive.test_corrupt_archive_refused_before_extract`: expected assertion not raised, one failure/no error | 0.068 |
| Allow an unchecked runtime file | `Archive.test_unchecked_archive_member_refused`: expected assertion not raised, one failure/no error | 0.070 |
| Remove a command link with a different target | `Cleanup.test_different_command_target_refuses_cleanup`: cleanup incorrectly returns 0, one failure/no error | 0.105 |

The real artifact mutants used arm64 only through the test's `DARWIN_TEST_ARCHES` override; the restored baseline builds **both** CPUs. Final restored fast smoke passed **26/0**, 7.959 s wall; the combined fast `go_checks` passed **7.891 s**, below 15 s. The final owning setup stub/checksum tests pass in **0.695 s**; native Mac acceptance is not included in any of these times. No worker ran the normal gate.

## Native acceptance remains pending

The [CEO run list](../mac2b-run-list.md#run-the-archive-check) installs the delivered archive, checks launchd identity, private storage, pinned TLS request/reply, registry persistence and orphan cleanup, and verifies uninstall. It names exact prerequisites, command, log capture, cleanup and evidence return. No worker has Mac access for this item. The [earlier native run](mac2.md#checks-and-results) used focused binaries and `--exec`, so it cannot close this artifact acceptance.
