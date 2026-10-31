# Darwin archive revision and release sets

📌 **TL;DR:** MAC.2c closes the two MAC.2b LOWs. Native archive version queries must name the release-source commit. Directory verification accepts the collector’s Darwin client-only set; a present full edition must include both selected CPUs. Native Mac acceptance remains pending.

## Changes

- The [owner run list](../mac2b-run-list.md#prepare-the-owner-session) supplies the full `BUILD_COMMIT` to the existing Go and TypeScript build stamps. Before privileged installation, every native Go program and the TypeScript binary’s `web --version` query must report that exact commit. Its MCP and launcher queries report the version alone and remain checked for that version. The former checkout-to-itself assertion is removed.
- `release-verify.sh --os "linux darwin" <dir>` preserves the checksum collector’s existing client-only compatibility. If any full Darwin archive is present, the selected full set is required. Checksums, exact archive membership, manifests and per-program checks still apply.
- The directory verifier’s accepted release sets change, so this is shipped release-tool behaviour and needs a bump. No worker changed VERSION, published a release, pushed, or ran Mac/sudo commands.

## Checks

| Check | Result |
|---|---|
| Same version, different revision | Before the comparison, all seven original stubbed program/query variants accept the wrong commit; each fails `AssertionError not raised`. With the fix, all are refused. Missing, shortened and malformed commits are refused too |
| Collector-to-verifier fixture | Before the fix, a real checksummed client-only Darwin set fails on missing full archives. After: both tools accept client-only and complete full/client sets; both refuse the missing half of a full pair |
| Revision mutant | Ignore the stamped commit’s equality with the source: seven assertion failures, no load error, 0.060 s |
| Release-set mutant | Restore unconditional Darwin full requirements: `checksummed Darwin client-only set refused`, assertion exit 1, 3.141 s |
| Restored local acceptance | All 18 cleanup/archive/revision/real-answer tests pass in 0.666 s; the release verifier fixture passes; owning Go cleanup/checksum tests pass in 1.126 s |

Both mutants ran only owning checks under the smoke-gate slow lock, with bounded subprocesses and source restoration in `finally`; the lock was released immediately afterward. The native [archive run](../mac2b-run-list.md#run-the-archive-check) is still the owner’s pending acceptance.

## TypeScript query review fix

The original test invented `build_info` for the MCP and launcher answers. A real compiled `package.sh` full archive at `784c56f8` returns only `0.9.241\n` for `mcp --version` and `launcher claude --version`. From the same complete extracted archive, `web --version` exits 0 with `0.9.241` and `build_info: parf@parf.us 2026-10-30 21:38:55 784c56f8aac1734649bf42c6b8bf28496c3594e0`. This supported query supplies the single compiled TypeScript binary’s stamp; no product query behaviour changes.

`Archive.test_real_typescript_version_answers_are_accepted` runs the actual TypeScript entry point with stamped build environment and feeds its real answers into the native archive checker. It fails on the old checker’s missing-`build_info` assertion, then passes with version-only MCP/launcher answers and a stamped web answer. The web tsconfig is explicit for the source-run regression; compiled archive queries were reproduced separately above.

The corrected checker passes against that complete real compiled host archive, including all five Go programs and the three TypeScript queries. Review-fix mutants under the lock: requiring a stamp on the real version-only MCP answer fails the real-answer assertion (one failure, 0.128 s); skipping the TypeScript web stamp comparison accepts a wrong TS commit and fails `AssertionError not raised` (one failure, 0.060 s). Sources restored; all 18 local tests pass in 0.637 s, the owning Go cleanup/checksum tests pass in 0.792 s, and the release-set fixture remains green.
