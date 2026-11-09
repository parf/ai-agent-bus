# Local 1.0.0 release preparation — PUBLICATION ON HOLD

The owner requested minor documentation fixes before push. No v1.0.0 tag exists and no push or registry publication has run.

- Main: VERSION1.0.0, local STABLE77f813bb, changelog foldd69f9edb; later documentation fixes stay on main. Re-freeze source after the owner finishes the minor fixes.
- Prebuilt snapshot: tmp/release-1.0-prep, source d69f9edb. Six Linuxx64/Darwinarm64/Darwinx64 full/client archives verified, their SHA256SUMS, six local npm tarballs and readiness.json. All staged archives contain current PolyForm Strict LICENSE and NOTICE.
- This snapshot precedes the documentation relocation. It is provisional: final binaries must stamp the final source, and archived INSTALL/help must include the final docs.
- Linuxarm64 needs its native GitHubActions job after push; no native local ARM64 builder or new Mac container build was used. The complete publication set is eight archives plus their exact checksum set, then six platform npm packages and two command packages.
- Release notes: .ai-team/release-1.0/release-notes.md. Workflow: .github/workflows/release.yml; previous detailed runbook: .ai-team/ab-w2b/release-1.0.md. No workflow, test budget or runtime behavior was changed for preparation.
- Next after explicit push go: final-source stamp/build; fast-forward push main; annotated v1.0.0 tag on the final versioned source; observe both native Linux jobs, both Darwin jobs and verified release creation. Download/check all eight published archives; wrap both Linux CPUs plus Darwin client archives using the complete SHA256SUMS; publish platform packages first, then client and bus at exact1.0.0.
- npm authentication currently answers parf73. gh CLI is absent and no stored GitHub REST token was found; SSH dry-run push succeeds. The release workflow can create assets with its own GitHub token. Release note editing may need the owner's GitHub UI/session if no API credential becomes available.
- Container publication is a separate remaining release step from the approved runbook: build from the verified final Linuxx64 archive, lock any Podman work, then push only after publication hold is lifted. No container push or live daemon/runner deployment was included in this local preparation.
