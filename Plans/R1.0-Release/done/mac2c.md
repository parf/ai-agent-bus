# Darwin archive revision and release sets

📌 **TL;DR:** MAC.2c closes the two MAC.2b LOWs. Native archive version queries must name the release-source commit. Directory verification accepts the collector’s Darwin client-only set; a present full edition must include both selected CPUs. Native Mac acceptance remains pending.

## Changes

- The [owner run list](../mac2b-run-list.md#prepare-the-owner-session) supplies the full `BUILD_COMMIT` to the existing Go and TypeScript build stamps. Before privileged installation, every native Go program and both TypeScript version queries must report that exact commit, as well as the expected version and build stamp. The former checkout-to-itself assertion is removed.
- `release-verify.sh --os "linux darwin" <dir>` preserves the checksum collector’s existing client-only compatibility. If any full Darwin archive is present, the selected full set is required. Checksums, exact archive membership, manifests and per-program checks still apply.
- The directory verifier’s accepted release sets change, so this is shipped release-tool behaviour and needs a bump. No worker changed VERSION, published a release, pushed, or ran Mac/sudo commands.

## Checks

| Check | Result |
|---|---|
| Same version, different revision | Before the comparison, all seven program/query variants accept the wrong commit; each fails `AssertionError not raised`. With the fix, all are refused. Missing, shortened and malformed commits are refused too |
| Collector-to-verifier fixture | Before the fix, a real checksummed client-only Darwin set fails on missing full archives. After: both tools accept client-only and complete full/client sets; both refuse the missing half of a full pair |
| Revision mutant | Ignore the stamped commit’s equality with the source: seven assertion failures, no load error, 0.060 s |
| Release-set mutant | Restore unconditional Darwin full requirements: `checksummed Darwin client-only set refused`, assertion exit 1, 3.141 s |
| Restored local acceptance | All 17 cleanup/archive/revision tests pass in 0.558 s; the release verifier fixture passes; owning Go cleanup/checksum tests pass in 1.126 s |

Both mutants ran only owning checks under the smoke-gate slow lock, with bounded subprocesses and source restoration in `finally`; the lock was released immediately afterward. The native [archive run](../mac2b-run-list.md#run-the-archive-check) is still the owner’s pending acceptance.
