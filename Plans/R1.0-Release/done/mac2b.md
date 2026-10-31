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
| Same archive reader as the owner run | Both actual full archives pass `archive_bundle`; every regular file is in the manifest, and only the four exact inward launcher aliases may be links |
| Verifier refusals | Full Darwin fixture accepted; corrupted daemon header with otherwise valid version/stamp/manifest refused; existing release refusal checks pass |
| Checksum collection | Four `TestReleaseSums*` tests pass, including incomplete/full CPU-pair refusal and complete eight-archive set |
| Acceptance fixture | Thirteen local stub/archive checks pass through existing `TestDarwinAcceptanceCleanupStubs`; no Mac or sudo invoked |

## Native acceptance remains pending

The [CEO run list](../mac2b-run-list.md#run-the-archive-check) installs the delivered archive, checks launchd identity, private storage, pinned TLS request/reply, registry persistence and orphan cleanup, and verifies uninstall. It names exact prerequisites, command, log capture, cleanup and evidence return. No worker has Mac access for this item. The [earlier native run](mac2.md#checks-and-results) used focused binaries and `--exec`, so it cannot close this artifact acceptance.
