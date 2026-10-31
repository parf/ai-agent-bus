# macOS npm platform acceptance

📌 **TL;DR:** P.1c’s native Mac acceptance passed on Apple Silicon using local npm tarballs from main `2214ba4e` (0.9.242). The installed client answered its version and completed a correlated bus call; the full wrapper refused the unsupported daemon. The CEO confirmed cleanup. Publication remains the owner’s separate decision.

## Build and gate checks

Both Darwin client platform packages are built from real client archives by `pack.py --darwin-client`, with names, OS/CPU, files and exact-version pins checked by `src/npm/darwin_check.py`. The normal npm shard runs that check; five assertion-red mutants were recorded for P.1c. Its small stand-ins test packing and resolution; the native run below exercises the real compiled archive bytes. The npm bus platform remains Linux-only; Darwin npm provides the client.

The original task’s implementation and acceptance record was:

- State P.1c 2026-10-30, branch `ab-w2b/P1c`: `pack.py --darwin-client <archive>` (one per CPU) makes `ai-agent-bus-client-darwin-arm64` and `-darwin-x64` platform packages from the macOS client archives and lists them in `ai-agent-bus` and `ai-agent-bus-client`; the bus has no macOS platform package, and its daemon commands refuse there by name. [`src/npm/darwin_check.py`](../../../src/npm/README.md) pins names, `os`, `cpu`, files, the pins, and `agent-bus` resolving as macOS arm64; five mutants fail it. Mac acceptance (the CEO runs it, `~/agent-bus-dev` only): install the two local tarballs, `agent-bus --version`, one call, clean up with `du -sh`.

## Native acceptance

The CEO ran the check on 2026-10-31 at 02:18–02:19, arm64. Source: `2214ba4e1d4697592018065595ffba9b92da318e`, version `0.9.242`; the CEO reported Homebrew Node `23.11.0`. The [unaltered result log](p1c-mac-native-2026-10-31.log) has SHA256 `b1b93d69327a58e64d72a5b4c53888c4f218bd05b7fe0db0821e1187fd24a4e7`.

| Acceptance | Evidence |
|---|---|
| Local package input | All six tarball SHA256 checks report OK; no public npm publication or login |
| Install client | `npm install -g ai-agent-bus-client@0.9.242` through the local registry, exit 0 |
| Installed version | `agent-bus --version`, exit 0; 0.9.242 and build_info naming the exact source commit |
| One call | `call '#debug@parf.us' --wait 10s ping` over an SSH-forwarded login socket, exit 0; reply from that agent with `re=789458b0e70206c5` |
| Install full wrapper | `npm install -g ai-agent-bus@0.9.242`, exit 0 |
| Client-only Darwin boundary | `agent-busd --version`, exit 1: `unsupported platform darwin-arm64` |
| Cleanup | CEO’s post-run report: Mac clean; `~/agent-bus-dev` 122M |

The log’s 280M measurement is **before cleanup**; the 122M result comes from the CEO’s later report relayed by #ab-lead2. The log records the command package path, not the exact nested platform-package path. Native execution and the stamped CLI version are established; no native Intel run is claimed.

These results close P.1c’s written Mac acceptance (install both local command packages, version, one call, cleanup and disk-size report). They do not close the parent P.1 item’s remaining release/publication work or MAC.2 daemon teardown. The temporary build-node `tmp/p1c-mac` input is removed after retaining this evidence.
