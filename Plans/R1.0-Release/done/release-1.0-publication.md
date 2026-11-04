# Release 1.0.0 publication

📌 **TL;DR:** The owner authorized pushing and publishing 1.0.0 on 2026-11-04. Build fresh artifacts from the final tag; earlier RC/preparation output is not this release.

## Pre-publication checks

- Clean main runtime from `500b1ab3`; `VERSION` is 1.0.0. No remote 1.0.0 tag existed at preflight, and both npm command names were still at their reserved 0.0.1 versions.
- Full `src/smoke.sh --slow all`: exit 0, 1301 passed, 0 failed, 84.383 seconds. All 63 shards selected at slow. The smoke-gate lock was held and released afterward.
- Independent release/package audits found no source blocker. The old `tmp/release-1.0-prep` was based on `d69f9edb` and lacked Linux arm64 artifacts; none is reused.
- Full/client archives are required for Linux and Darwin, both CPUs. The hosted workflow verifies them before creating a release. Foreign binary verification is structural, not a new native Mac run.

## Publication sequence

1. Push the reviewed main and version tag; await the hosted release workflow.
2. Download the eight archives plus SHA256SUMS and verify the complete set.
3. Run clean Podman acceptance against the new downloaded Linux artifacts before npm publication, as the owner requested.
4. Assemble npm packages for both Linux CPUs and both Darwin clients from the same checksum set. Publish six platform packages, then the client command package, then the bus command package.
5. Verify the public release, npm versions/dependencies and a clean installation of the published packages. Record exact results here.

Publication is in progress; this record does not claim completed upload, registry publication or final-container acceptance yet.
