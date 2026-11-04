# Native macOS RC2 installation evidence

📌 **TL;DR:** The same 1.0.0RC2 source tested on Linux passes native Apple Silicon archive installation, launchd identity/private storage, TLS request/reply, restart persistence and teardown. Local offline npm client installation and two script-agent calls pass too. No release or npm publication ran.

## Candidate and machine

- Source: local tag `1.0.0RC2`, commit `ca7c7396cd057d9f0e1b22cd396f18c4d9492eaa`, package version `1.0.0`.
- Tested 2026-11-03 through `ssh mac`: Apple Silicon `arm64`, macOS `26.5.2`; Homebrew Node/npm already present.
- Full and client `darwin-aarch64` archives were cross-built on this Linux node from that exact candidate. `release-verify.sh` passed both archives; the native harness queried their versions and exact build-source stamps before installation.
- These are the RC2 source/license snapshot. The later PolyForm Strict edit does not change the tested installation/runtime code; final artifacts need the intended legal files, not another Linux behavior run solely for that text change.
- No build, dependency installation, Homebrew operation, profile change, extra checkout or container ran on the Mac. The existing clone returned to clean `main`, `c49d51f3973522a5d3d1af57c253996488abd21e`.

| Darwin archive | SHA-256 |
|---|---|
| `agent-bus-1.0.0-darwin-aarch64.tar.gz` | `79a4d4c7ddba231d8b07e699e40882e112bf7a156c7505a8d178be7b11ef1531` |
| `agent-bus-client-1.0.0-darwin-aarch64.tar.gz` | `c25ac2aeaf7ec527978fde1c35911bce2183965a065a98cca09a62ffa8f0ff7a` |

## Full archive run

Ran `src/acceptance/mac2-install-check.py` from the exact RC2 source with its revision and `--archive` pointing to the full native archive. The full run exited 0. [Unaltered full log](rc2-mac-full.log) lists every direct sudo command and each result.

| Check | Observed result |
|---|---|
| Archive integrity | Checksums, full manifest, native program versions and exact source stamps pass before setup. |
| Delivered setup | Installs directly from the archive, without `--exec` or rebuilt Mac binaries; installed links and daemon bytes match. |
| Service identity and state | Safe existing hidden `_agentbusd` role is reused, the daemon is not root, and storage is 0700 and unreadable by the installing user. |
| Pinned TLS | One request consumed exactly once; `pong` comes from the registered fixture agent. |
| Launchd restart | Supervisor changes and the registered agent remains. |
| Abrupt supervisor exit | The old bus child exits; replacement returns and the registry remains. |
| Teardown | Test job, plist, release, storage, sockets and created command links removed. Safe role account/group kept by design. |

## Local npm client and runner

[Passing client log](rc2-mac-client.log): locally assembled `ai-agent-bus-client` and `ai-agent-bus-client-darwin-arm64` tarballs installed into a workspace-only prefix with npm offline, scripts disabled and cache inside the workspace. The installed CLI reported `1.0.0` and the exact RC2 source stamp.

- A private loopback/Unix-socket daemon from the full archive supplied the test bus; no production bus or outside provider was used.
- Requesting the unavailable Mac sandbox was refused by name with nonzero status.
- The npm CLI started the same user's script with sandbox explicitly off. Two calls returned their distinct expected bodies (`mac rc2: first`, `mac rc2: second`) and the exact agent identity.
- This proves native Apple Silicon client/runner behavior, not a Linux sandbox, native Intel execution or a macOS web service.

An initial client fixture looked up a qualified agent name after starting a short name and timed out. Correcting the fixture to use the same explicit name made the owning check pass; no RC code changed. That first fixture also used the CLI's default work location; its four identified fixture files/directories were removed. The corrected run directs state and caches into the workspace. The first diagnostic log remains node-local, not counted as a product failure or pass.

## Cleanup and boundaries

[Cleanup log](rc2-mac-cleanup.log) confirms the installed job, release/state/socket paths and command are absent, the clone is clean on its original main, all own fixture processes and files are removed, and `~/go` and `~/Library/Caches/go-build` remain absent.

- Workspace: **64M before**, **66M after**; only the missing Git objects streamed into the existing clone remain. Input archives, npm prefix/cache, extraction and temporary scripts were removed.
- Disk: 460 Gi total, 173 Gi used, 263 Gi available before and after (rounded filesystem readings).
- Node build/package work held smoke-gate with a 15-minute lease and released it after the batch. No Linux installation acceptance was repeated for the Mac run.
- No public npm/GitHub download or publication, real external account, native Intel or actual update from a separately released older version is claimed. The [Linux RC evidence](rc-linux-installation.md#npm-browser-and-remaining-boundaries) remains valid independently.
