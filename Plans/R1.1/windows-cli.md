# Windows CLI client

📌 **TL;DR:** A Windows client edition: the `agent-bus` CLI over pinned HTTPS, then limited `ab-claude` and `ab-codex` launchers on the compiled bun program. No daemon, runner, sandbox, keeper or `run`. About 14 working days in two parts (CLI 5, launchers 8.5), proved by a cross-build in every gate and a Windows CI job; a Go port of the launchers is costed and not recommended now. R1.1, so R1's 11-04 does not slip. Owner answers of 2026-10-30 are in [decisions](DECISIONS.md#recorded-decisions); open choices are [Q265–Q267](QUESTIONS.md#open-questions).

## Why

- A Windows user can send, call and read on a bus without a Linux box of their own, and run Claude or Codex sessions on it by name.
- The CLI is one static Go binary: the work is removing Unix assumptions, not writing a new client.

## What you get

| Outcome | Where it stops |
|---|---|
| `agent-bus ls`, `send`, `call`, `consume`, `reply`, `kv`, `lock`, `token`, `status` against a daemon on another machine | no local daemon, no local socket discovery; `agent` and `run` refuse and say why |
| Pinned HTTPS with `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN`, `AGENT_BUS_TLS_FINGERPRINT` | same variables as Linux and macOS |
| `ab-claude` and `ab-codex` start or resume a named session with the bus tools ([limited launchers](#launchers-on-windows)) | reduced features, listed below |
| `agent-bus-client-<version>-windows-x86_64.zip` in the release, verified like the others | arm64 Windows only if asked |

## Files the client writes and reads

The owner asked what files a simple client needs. Measured in `cmd/agent-bus` with the keeper (`agent`) and `run` left out:

| File | Who | Why | Windows |
|---|---|---|---|
| `<cache>\agent-bus\<caller>\<message-id>.json` and a `.swept` marker | `consume` and every read that shows a message (`remember`, `main.go`) | so `reply` works later without `--to`; swept after a day | `os.UserCacheDir()` (`%LocalAppData%`) instead of `$XDG_CACHE_HOME` or `~/.cache`; the user profile's own ACL protects it; names go through `safe()`, which already neutralises path characters |
| `~\.ssh\id_ed25519` (read only) | `enrol`, through `ssh-keygen -Y sign` | the key proof | unchanged; `ssh-keygen` ships with Windows 10 1809 and later as the OpenSSH client |
| a lock file | nobody | `internal/authkeys` locks `authorized_keys` for the daemon; the CLI links it only because it imports `internal/api` | none written; the Windows build gets a stub that errors if ever called |

So the CLI needs no state directory and no lock file. The earlier plan's state directory was for the keeper's notes; with the keeper out it is dropped, and the file-name encoding for reserved characters with it ([Q261](DECISIONS.md#recorded-decisions)). The launchers do need a state directory; that is in their own section.

## What a Windows build hits

Measured on this node, 2026-10-30, `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/agent-bus`. The compiler reports one error group per package, so the table also lists what a read of the CLI's dependencies found.

| Area | Where | Today | Plan |
|---|---|---|---|
| Process title | `internal/proctitle` (`gspt`) | no Go file builds off Linux and macOS cgo | a no-op beside the cgo one, the same split [MAC.1](../R1.0-Release/macos.md#mac1-client-and-runner-on-macos) made for macOS; the Windows build reuses it |
| File lock | `internal/authkeys` (`syscall.Flock`) | undefined | a stub that returns an error where the daemon would lock; never reached by the client |
| Symlink-refusing read | `internal/agentmeta` (`O_NOFOLLOW`) | undefined | per-OS open: on Windows refuse a reparse point after the open |
| Terminal test | `cmd/agent-bus/help.go` (`unix.IoctlGetTermios`) | undefined | `os.Stdout.Stat()` mode check; colour stays off when it is not a console |
| Keeper and `run` | `cmd/agent-bus/agent.go` (`Flock`, `Kill(pid, 0)`, `/proc/<pid>/fd`, `SIGTERM`), `run.go` | undefined | left out of the Windows build with a refusal naming them ([Q262](DECISIONS.md#recorded-decisions)); this also removes the signal and child-process questions |
| Local socket | `internal/api/server.go`: `XDG_RUNTIME_DIR`, `os.Getuid`, `/run/agent-bus`, `unix` dial | compiles; the paths mean nothing | no discovery on Windows: `AGENT_BUS_ADDR` is required, and its absence says so |
| Peer credentials | `internal/api/peer_*` | per-OS stubs already exist | nothing |
| File modes | `0o700` and `0o600` on the cache | accepted and ignored on NTFS | the profile's ACL; tokens stay in the environment |

## Reaching a remote daemon

- Over HTTPS to `AGENT_BUS_ADDR` with the certificate pinned by `AGENT_BUS_TLS_FINGERPRINT`, as on the [Mac](../R1.0-Release/macos.md#order). The pin does not use the Windows certificate store.
- The token comes from `AGENT_BUS_TOKEN` or the key proof; the user sets environment variables in PowerShell (`$env:AGENT_BUS_ADDR`) or the user environment.
- The daemon decides who may do what; the Windows client adds no authority and no new call.

## Launchers on Windows

The owner wants `ab-claude` and `ab-codex` on Windows, in R1.1 with the CLI (owner, 2026-10-30; changes the earlier "launchers out"). They are TypeScript on bun (`src/launchers/`, 1.4k lines plus 1.7k of tests), starting the runtime beside the MCP face (`src/mcp/`, 2.5k lines plus 2.4k of tests); `agent-bus-ts` is both compiled, and bun compiles for `bun-windows-x64`.

Does it reopen [Q262](DECISIONS.md#recorded-decisions)? No. The launchers call neither the Go keeper nor `agent-bus run` (no call to either in `src/launchers` or `src/mcp`), so the keeper and `run` stay out. What they do need that the CLI does not:

| Need | Where | Windows |
|---|---|---|
| Entry command | `ab-claude`, `ab-codex` are POSIX `sh` | `ab-claude.cmd` and `ab-codex.cmd` calling `agent-bus-ts.exe launcher <runtime>` |
| The MCP face | `agent-bus-ts mcp`, over `fetch` to `AGENT_BUS_ADDR` | the same compiled program; TCP/TLS only |
| A state directory | session bindings under `~/.local/state/agent-bus/sessions`, per-run directories, reply cache | `%LocalAppData%\agent-bus`; its own decision for the launchers only |
| The binding lock | `sessions.ts` publishes a lock by renaming a directory onto an existing one and reads `EEXIST` or `ENOTEMPTY` | NTFS answers `EPERM` or `EACCES`; the code and its tests accept them |
| Process groups | `process.kill(-pid, signal)` kills a runtime's whole tree | no groups: kill the tree with `taskkill /T`, or a job object |
| Signals | `SIGHUP` and `SIGTERM` handlers, `SIGUSR2` | only Ctrl-C and window close arrive; the handlers keep working for those |
| `--take-over` | `face-mark.ts` signals the live holder with `SIGUSR2` and reads `/proc/<pid>/stat` for its start time | neither exists: the start-time check already falls back to `kill(pid, 0)` as on macOS; the take-over signal does not ([Q265](QUESTIONS.md#open-questions)) |
| Who owns a runtime's port | `runtime-listener.ts` reads `/proc/net/tcp`, then `lsof`, then `ps` | `netstat -ano`; only Codex's App Server needs it |
| Local socket discovery | `local.ts`: `XDG_RUNTIME_DIR` | none: `AGENT_BUS_ADDR` required |
| Finding the runtime | `Bun.which` of `claude` or `codex` | also tries `.exe` and `.cmd`; the runtimes themselves are the vendors' to ship for Windows ([Q266](QUESTIONS.md#open-questions)) |
| Terminal tab colour and title | kitty, Konsole, `tmux` and OSC escapes | the plain OSC title stays; kitty and Konsole parts are skipped |

Parts that stay: start or resume a named session, `--fresh`, `--descr`, `--tag`, `--restart`, rename, the bus tools and channel push, and the Claude channel plugin steps. Parts that go: `--take-over` (until [Q265](QUESTIONS.md#open-questions)), kitty and Konsole tab colours, local socket discovery, and the OpenCode launchers.

### Three ways to get them

| Option | What it is | Parts | Estimate |
|---|---|---|---|
| (a) Limited launchers on bun | the table above, on the compiled `agent-bus-ts.exe`; the features listed as going stay out | a Windows build target, the `.cmd` shims, the state directory and lock fix, tree kill, `netstat`, binary lookup, a Windows CI job for launcher and face tests, docs. A first day is a spike on a real Windows machine: compiled `agent-bus-ts launcher claude` starts a runtime and the face reaches a bus | about 8.5 days: spike 1, build and shims 1.5, state and lock 1, process tree and signals 1.5, listener 0.5, binary lookup and plugin steps 0.5, CI tests 1.5, docs 1 |
| (b1) Port what the launchers need to Go, Windows only | a second implementation of the launcher (1.4k lines) and the MCP face (2.5k) in Go, shipped only for Windows | two programs that must behave the same, with no shared specification; every launcher or face change lands twice; the Linux tests (4k lines) do not run against the Go one | about 28 days (22 to 35): ~4k lines of code and an equal amount of test, plus a parity suite; the CLI part stays 5 |
| (b2) The same port, for every OS | one Go launcher and face replace bun in the launcher path on Linux, macOS and Windows; `agent-bus-ts` stays only for the web face | the `ab-*` shims, `faceCommand`, the Claude channel plugin's command, `package.sh` and `install.sh` (`client_files`), every launcher and face test and the launcher smoke, and the MCP protocol handling that the TypeScript SDK gives today ([K.37](TODO.md#mcp-latest-revision) tracks its next revision) | about 35 days (30 to 45) with a soak on this team's own windows, which run on the launchers all day |

Recommendation: **(a)**. It reuses what already exists and is tested on Linux, costs about a third of either port, and its first day answers whether bun's Windows build is good enough. If that spike fails, (b1) is the fallback. (b2) has a real benefit, no bun on any client, but it rewrites the one path every worker here depends on; done in R1 it would put R1's 11-04 at risk, so it is not R1 work and not needed for Windows. Whether to schedule it later, as R1.2 or R2.0 work of its own, is [Q267](QUESTIONS.md#open-questions).

## What stays out

| Out | Why |
|---|---|
| Daemon, setup, admin | Linux services and accounts; macOS gets a reduced one in [MAC.2](../R1.0-Release/macos.md#mac2-daemon-with-a-reduced-setup), Windows none |
| Runner and sandbox | cgroups and bubblewrap; a sandbox request would be refused anyway |
| `agent` (keeper) and `run` | owner, 2026-10-30; refused with a message naming them |
| OpenCode launchers (`ab-opencode`, `ab-opencode1`) | not asked for ([Q266](QUESTIONS.md#open-questions)) |
| `install.sh` | POSIX; Windows installs from the zip, or later through the npm wrapper |
| Code signing and SmartScreen | the binary is unsigned, so SmartScreen warns, as Gatekeeper does on the Mac |

## Testing without a Windows box

| Option | Proves | Cost | Verdict |
|---|---|---|---|
| Cross-build in the gate (`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build`, as the darwin build in `go_checks`) | every build tag and stub compiles, on every landing | about 10 s | do it |
| wine on this node | a PE binary starts | a root-installed package, ~200 MB, NTFS and ACL behaviour wrong, needs the gate lock | no |
| GitHub Actions `windows-latest` job on the release workflow | the zip runs: `--version` with its build stamp; `go test` of the CLI on Windows against an in-process pinned-TLS test daemon (the handler the CLI's own tests already use): a request and a reply, a wrong fingerprint refused, the reply-context cache | one Windows job, a few minutes, only on a tag the owner pushes | do it |
| A real daemon, reached from Windows | the same against the real daemon | needs a reachable, isolated daemon with TLS, a token and teardown, which a GitHub-hosted job cannot give a Windows job; it is a prerequisite, not a step | the owner provides a Windows server when WIN.1b is due (owner, 2026-10-30); the team then asks `#ab-ceo` through the lead. Not counted in the estimate |

Recommended: the first and third rows; the fourth is the owner's, once. Behaviour that only Windows can show (the cache directory, console detection) is tested with `go test` in that CI job, never in the gate.

## Steps and estimate

| Step | Work | Days |
|---|---|---|
| 1 Build (WIN.1) | tag the Linux-only files, the per-OS stubs and open, the terminal test, a refusal for the left-out subcommands, `AGENT_BUS_ADDR` required; the gate cross-build | 2 |
| 2 Behaviour (WIN.1) | the cache directory, key proof through `ssh-keygen`, Windows unit tests, the in-process TLS test daemon for the Windows CI job | 1 |
| 3 Package (WIN.2) | `package.sh` writes the zip, `release-verify` reads it as it reads macOS archives, `release-sign` takes it with the others | 1.5 |
| 4 CLI docs | the setup guide's Windows page, the CLI guide's differences, the CI job | 0.5 |
| 5 Launchers (WIN.3) | option (a) above | 8.5 |
| Total | | 13.5, say 14; range 11 to 19 if the spike fails or `ssh-keygen` proofs surprise. The real-server run (WIN.1b) is not in it |

The CLI part fell from 6 to 5 days: no state directory and no file-name encoding without the keeper. The launchers add 8.5.

## Release

R1.1, owner-moved 2026-10-30, the launchers with the CLI. R1's 11-04 date holds: nothing here is in R1's critical path, and the shared files (`internal/proctitle`, `package.sh`) are the ones [MAC.1](../R1.0-Release/macos.md#mac1-client-and-runner-on-macos) already touched. Acceptance is in [WIN.1](TODO.md#windows-cli-client) to WIN.3.
