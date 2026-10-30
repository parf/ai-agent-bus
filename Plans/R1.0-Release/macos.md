# macOS

📌 **TL;DR:** macOS comes in three steps, cheapest first: the client and the runner without a sandbox, then the daemon with a reduced setup, then a sandbox only if the owner asks. A sandbox request on macOS is refused, never silently ignored. Test host, signing and the full-set direction are in [distribution § macOS](distribution.md#macos).

## Order

Owner, 2026-10-30 ([RD166](DECISIONS.md#recorded-decisions)):

| Step | Ships | Without |
|---|---|---|
| 1 Client and runner | CLI, runner, MCP face and launchers, the client edition for darwin-arm64 | any sandbox |
| 2 Daemon | the daemon with a reduced setup for macOS | the full Linux setup (systemd units, per-role accounts, cgroup limits) |
| 3 Sandbox | an equivalent of the Linux sandbox | deferred; built only if the owner asks |

## Rules without a sandbox

| Rule | Meaning |
|---|---|
| Refused, not ignored | a sandbox request on macOS fails with the reason, as the [runner contract](../../docs/06-dedicated-runner.md#sandboxing) already requires of any unavailable sandbox; nothing runs unsandboxed after asking for one |
| Dedicated account | the runner runs scripts as its own unprivileged account, never as an administrator or the owner |
| Said plainly | the docs say agent scripts then hold that account's rights, and that this suits one owner's own Mac, not a shared one |

## What stops a darwin build today

Measured on this node with `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build`:

| Program | Result |
|---|---|
| `agent-bus-setup`, `agent-bus-admin` | compile |
| `agent-bus`, `agent-busd` | stop in the process-title dependency (`internal/proctitle`), which needs cgo |
| `agent-bus-runner` | stops at two Linux-only process attributes that start a script inside its cgroup (`itself.go`, `serve.go`) |

Beyond the compile, the Linux-only parts are the cgroup limits (`internal/cgroup`), the `/proc` readers, the sandbox backend, and everything `agent-bus-setup` does for systemd and accounts. A darwin build is first a build-tag exercise on this node, then a run on the owner's Mac.

## Tasks

Team 2's runner work ([M.5](TODO.md#runner-waves), [DR.1](TODO.md#runner-waves)) edits the same directories. Files marked ⚑ are shared with it: split the code by that mark.

| ID | Step | Paths |
|---|---|---|
| MAC.1 | client and runner | `src/internal/proctitle/` (no-op where there is no cgo), `src/cmd/agent-bus-runner/` ⚑, `src/internal/cgroup/` ⚑, `src/internal/sandbox/` ⚑, `src/cmd/agent-bus/` (`/proc` and lock readers), `src/internal/authkeys/`, `src/internal/agentmeta/`, `src/build.sh`, `src/package.sh`, `install.sh`, docs |
| MAC.2 | daemon and reduced setup | `src/cmd/agent-busd/`, `src/cmd/agent-bus-setup/` (a macOS path beside the Linux one), the store and listener files `GOOS` touches |
| MAC.3 | sandbox | `src/internal/sandbox/` ⚑ — deferred, only if the owner asks |

### MAC.1 client and runner on macOS

- First Mac run, 2026-10-30: the CLI and the MCP face only, by the [Mac run: CLI and MCP face](mac-cli-run.md#mac-run-cli-and-mcp-face); the runner and the archive follow
- Acceptance, on this node: `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build` of `agent-bus` and `agent-bus-runner` succeeds, and the Linux build and its normal checks are unchanged
- Acceptance, on the owner's Mac (the Mac session of the Mac run list): the client archive builds natively as `agent-bus-client-<version>-darwin-arm64`, `release-verify` passes it with the binaries queried, the npm client package installs it (P.1), and from it a request, a reply and an MCP call complete against a bus over HTTPS; earlier shell-installer evidence is historical under the replacement [distribution decision](distribution.md#npm)
- Acceptance, on the Mac: `agent-bus-runner --sandbox on` exits non-zero naming the missing sandbox and starts no script; a script agent started by the runner reports the dedicated account's name, not the installing user's
- Docs: the client edition's macOS page says scripts hold the dedicated account's rights and the sandbox is absent
- Mutants: make the runner accept `--sandbox on` on darwin and the refusal check fails; run scripts as the installing user and the account check fails; drop the darwin build tag from one Linux-only file and the cross-build check fails on this node; ship an unstamped binary and `release-verify` fails

### MAC.2 daemon with a reduced setup

Native code and focused setup acceptance are [verified](done/mac2.md#checks-and-results). MAC.2b builds full and client archives for both Darwin CPUs; its [owner archive run](mac2b-run-list.md#run-the-archive-check) remains pending. The owner removed the original test roles and the CEO [verified absence](done/mac2.md#owner-cleanup-verified).

- Reduced setup: one dedicated account, a launchd job for the daemon, the storage directory private to that account, TLS as on Linux; no per-role accounts, no cgroup limits, no web unit until asked
- Acceptance, on the Mac: install the darwin archive, the daemon runs under launchd as the dedicated account and not root, a registered agent completes a request and a reply, a restart of the job keeps the registry, and the installing user cannot read the storage directory
- Mutants: run the job as root and the account check fails; make the storage directory world-readable and the privacy check fails; lose the registry across a restart and the persistence check fails

### MAC.3 sandbox

Deferred ([RD166](DECISIONS.md#recorded-decisions)). When the owner asks, the macOS backend (the system's own sandbox profile mechanism) goes behind the existing sandbox port; its acceptance then mirrors the Linux profiles, with a mutant that lets a denied access through.
