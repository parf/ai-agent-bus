# Release 1.0.0 publication

📌 **TL;DR:** The owner authorized pushing and publishing 1.0.0 on 2026-11-04. Build fresh artifacts from the final tag; earlier RC/preparation output is not this release.

## Pre-publication checks

- Clean main runtime from `500b1ab3`; `VERSION` is 1.0.0. No remote 1.0.0 tag existed at preflight, and both npm command names were still at their reserved 0.0.1 versions.
- Full `src/smoke.sh --slow all`: exit 0, 1301 passed, 0 failed, 84.383 seconds. All 63 shards selected at slow. The smoke-gate lock was held and released afterward.
- Independent release/package audits found no source blocker. The old `tmp/release-1.0-prep` was based on `d69f9edb` and lacked Linux arm64 artifacts; none is reused.
- Full/client archives are required for Linux and Darwin, both CPUs. The hosted workflow verifies them before creating a release. Foreign binary verification is structural, not a new native Mac run.

## Published release

- [GitHub release v1.0.0](https://github.com/parf/ai-agent-bus/releases/tag/v1.0.0), public on 2026-11-04; not a draft or prerelease. Tag targets `5e5fcccc22f37462f4f99680c2a8fc8df11e754d`.
- [Hosted release workflow](https://github.com/parf/ai-agent-bus/actions/runs/41789667996): all five jobs succeeded. Eight full/client archives cover Linux and Darwin on x86_64 and aarch64, with SHA256SUMS. Downloading the published archives and running `src/release-verify.sh --os 'linux darwin'` passed for all eight. This is structural verification of the foreign binaries, not new native Mac acceptance.

## Clean Podman acceptance

The Linux x86_64 archives were downloaded from this GitHub release and wrapped with `src/npm/pack.py`; no earlier RC archive was reused. Their SHA256 values are:

| Archive | SHA256 |
|---|---|
| Full Linux x86_64 | `c2f7bfc70e27a1238ac22f7a81d5f4d32e8a4cdd993581dbb3407bcd4645735d` |
| Client Linux x86_64 | `93785f2450ed2951f63ae851ccdd8b5628d77b8238423db623bb4ced5da2c194` |

- `src/npm/container-check.sh`: 7 tests, 0 failures, 16.097 seconds. Clean Debian container with npm and Python, without the checkout, Go or Bun: checked manifest staging, installed bus startup, installed-agent request/reply and client-only joining as a second account.
- `src/container/acceptance.sh --image=localhost/agent-bus-release:1.0.0 1.0.0`: 11 passed, 0 failed. Separate daemon and runner containers exchange a request/reply; recreating the daemon preserves registration and credentials; a sandbox request is refused.
- All Podman work held the smoke-gate lock; fixture containers and private storage were cleaned afterward.
- The first image recipe uses `4b912e5e`, with the unchanged tagged release archive as its binary input. Before first image publication, its default command was corrected to `--help`; the production-script regression failed on the previous `help` command, and the reverse mutant failed on an assertion. Normal Q then passed 31/0. Its first attempt exposed a renamed logwatch fast-test entry; correcting the selector restored the gate without changing runtime code or release archives.

## Registry publication

[Docker Hub image](https://hub.docker.com/r/parf/agent-bus): `docker.io/parf/agent-bus:1.0.0` and `:latest` both resolve to `sha256:cacfe762f56e07c42e656375e05d3a413191a5b5ac8b1f56abb65710627ac024`, verified independently with `skopeo inspect`. This first image is Linux amd64, not a multi-platform image.

All eight npm packages are public at 1.0.0. The six platform packages were uploaded before the two command packages. After registry processing completed, every tarball was downloaded from its public registry URL: all SHA256 values match the uploaded files, SHA512 matches registry integrity, and dependencies/optionalDependencies match the generated exact 1.0.0 pins.

- `ai-agent-bus-linux-x64`, `ai-agent-bus-linux-arm64`.
- `ai-agent-bus-client-linux-x64`, `ai-agent-bus-client-linux-arm64`, `ai-agent-bus-client-darwin-x64`, `ai-agent-bus-client-darwin-arm64`.
- [ai-agent-bus](https://www.npmjs.com/package/ai-agent-bus) and [ai-agent-bus-client](https://www.npmjs.com/package/ai-agent-bus-client).

A fresh private-prefix `npm install ai-agent-bus@1.0.0 --ignore-scripts` from the public registry installed four packages. The installed CLI and daemon both report 1.0.0 and stamped commit `5e5fcccc22f37462f4f99680c2a8fc8df11e754d`. No host-global install or live-node deployment was performed.

The public-downloaded tarballs then passed a second clean Podman installed-tree run: 7 tests, 0 failures, 16.033 seconds. The fixture resolved only those downloaded artifacts through its private registry. This repeats actual setup, installed-agent request/reply and client-only joining; it does not merely run version/help commands.

## Dependency audit follow-up

The MCP dependency audit is not clean. The compiled runtime includes the SDK and Zod, so its alerts cannot all be dismissed as development-only. Review SDK/Zod updates and explicit redirect refusal for the stdio face's outgoing StreamableHTTP client; no unauthenticated daemon-wide exploit was demonstrated in the release review. This publication does not change already-tagged dependency code or claim zero vulnerabilities.
