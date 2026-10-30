# Unsigned release build artifacts

📌 **TL;DR:** P.1a removes release signing from the build and installer paths under [RD178](../distribution.md#release-artifacts). Archives retain checksum, manifest, version, build-stamp and license checks. npm installation and macOS npm platforms remain P.1b.

## Result

- Actions requires both native Linux builds, verifies every full/client archive before upload, verifies the complete downloaded set and publishes archives plus `SHA256SUMS`. No owner key, signature or signed draft remains.
- `src/release-sign.sh` keeps its historical filename for fixture callers; it only generates `SHA256SUMS`, requiring the complete Linux set and either both macOS client archives or neither. Optional CI hash comparison remains.
- `release-verify.sh` rejects obsolete `--signers`; unsigned sets require exact archives and checksums, then the existing manifest, source version, build stamp, license and notices checks. Individual-archive verification remains available.
- The deferred shell installer checks the downloaded archive and, for a full node, checks its root-owned copy again before setup. Both client and root verify the inner manifest. Installer fixtures use unsigned releases; enrollment SSH key proofs remain unchanged.
- An unsigned checksum is an integrity check against the downloaded list. The fixture does not claim to reject an attacker rewriting the payload, manifest and checksum list together.

## Checks

2026-10-30, local Linux x86_64, no tag, release, push, npm login or publication:

| Owning check | Result |
|---|---|
| `bash src/release-verify.test.sh` | Pass: exact workflow build command and both native matrix entries, bad full/client evidence, unsigned set and obsolete option refusal |
| `go -C src test ./cmd/agent-bus-setup -run '^Test(InstallSh\|ReleaseSums)' -count=1` | Pass, including unsigned install with an `ssh-keygen` sentinel, missing/wrong/ambiguous checksums, root rehash, both manifests, Linux and simulated macOS client update/rollback, Mac fixture construction |
| `src/package.sh` and `RELEASE_DIST=… bash src/release-verify.real.test.sh` | Built real full/client 0.9.224 archives; unsigned verification passes; tamper, missing archive, renamed checksum entry and absent checksums fail |
| `bash src/acceptance/distro/run.sh --dist … --keep debian13` | 11 PASS: clean host, checksum refusal, missing-bwrap warning, manifest refusal, full install, six check-deploy checks, second-account client request/reply, compiled MCP `ab_ls`, node/client update to 0.9.225, client rollback and node downgrade |

The Debian fixture changes a payload without updating its inner manifest. Its tampered variant retains the old outer checksum; its forged variant recalculates the outer checksum and reaches the manifest check. Both refuse before writing installed files. The Mac fixture construction is tested locally; no new native Mac run is claimed. AlmaLinux and Ubuntu were not rerun for this slice.

## Assertion evidence

Each mutant ran only its owning check under the `slow` lock; sources were restored and the owning suites passed afterward. A syntax-error attempt at the release checksum mutant was discarded, corrected and rerun; it contributes no proof.

| Mutant | Assertion failure |
|---|---|
| Workflow restored to pre-P.1a signing | `AssertionError: release workflow must have no signing, owner key or draft wait` |
| Verifier accepts unstamped native binary | `unstamped: accepted a bad release` |
| Verifier ignores program version | `altered: accepted a bad release` |
| Verifier ignores license | `nolicence: accepted a bad release` |
| Verifier skips outer checksum | `checksum mismatch: accepted a bad release` |
| Installer skips download checksum | `TestInstallShRefusesBeforeWritingAnything/wrong_checksum`: `installed anyway` |
| Installer skips client manifest | `TestInstallShRefusesAnArchiveOfTheWrongShape/manifest_client`: `installed, or refused for another reason: <nil>` |
| Installer skips root manifest | `TestInstallShRefusesAnArchiveOfTheWrongShape/manifest_root`: `root ran it, or refused for another reason: <nil>` |
| Installer skips root rehash | `TestInstallShFullNodeRefusesAnArchiveSwappedBeforeRootCopiesIt`: `a swapped archive was not refused as changed: <nil>` |
| Mac fixture uses old signing API | `TestReleaseSumsThrowawayMac`: `throwaway fixture: exit status 2 usage: release-sign.sh <dir> <version> [<ci-sums>]` |

## Completed acceptance

Retire release-signing steps and signed-draft expectations in `.github/workflows/release.yml`, `src/release-sign.sh`, release verification and installer-dependent fixtures. Publish unsigned GitHub build artifacts with checksums.

Remove `src/release-verify.sh --signers` and its `ssh-keygen -Y` signature-verification branch; replace `src/release-verify.real.test.sh`'s throwaway-key cases with unsigned checksum/manifest cases. A valid unsigned release verifies without invoking `ssh-keygen`; an altered archive, wrong version, unstamped binary, missing license or incomplete manifest fails its named check. Bypassing the checksum comparison makes the altered-archive assertion fail. Remove obsolete signing assertions from owning Go/workflow tests; enrollment key proofs stay unchanged.

The owner-approved installer extension removes its signature requirement, preserves SHA256SUMS and manifest refusals with assertion-red mutants, and updates `install_sh_test.go`, `distro/run.sh` and `mac-throwaway-release.sh`. Shell installer retirement stays in [FUTURE](../FUTURE.md#shell-installer); npm clean-install and macOS npm platforms are separate work.
