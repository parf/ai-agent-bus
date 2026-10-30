# Windows CLI client

📌 **TL;DR:** A Windows client edition: the `agent-bus` CLI over pinned HTTPS, and `ab-claude`, `ab-codex` and `ab-opencode` launchers written in Go as one module meant for every OS. No daemon, runner, sandbox, keeper or `run`, and no bun on Windows. About 43 working days in two parts (CLI 5, Go launchers and face 38), proved by a cross-build in every gate and a Windows CI job. R1.1; the Linux and macOS switch to the Go client is [R1.2](../R1.2/TODO.md#go-only-client). Owner answers of 2026-10-30 are in [decisions](DECISIONS.md#recorded-decisions); the open choice is [Q268](QUESTIONS.md#open-questions).

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

Owner, 2026-10-30: `ab-claude`, `ab-codex` and `ab-opencode` on Windows, in R1.1 with the CLI, written in Go with no bun on Windows, as one Go module meant for every OS so the later Linux and macOS move reuses it ([decisions](DECISIONS.md#recorded-decisions)).

### Which runtimes

| Runtime | Windows CLI | Source | In scope |
|---|---|---|---|
| Claude Code | native, Windows 10 1809+ or Server 2019+, x64 and ARM64; Git for Windows optional | [Claude Code setup](https://code.claude.com/docs/en/setup#set-up-on-windows) | yes: `ab-claude` |
| Codex | native Windows support with its own Windows sandbox; WSL2 stays an option; Windows 10 is best effort | [Codex on Windows](https://developers.openai.com/codex/windows) | yes: `ab-codex` |
| OpenCode | runs directly on Windows; WSL is recommended, not required | [OpenCode on Windows](https://opencode.ai/docs/windows-wsl/) | yes: `ab-opencode` (not the archived `ab-opencode1`) |

Checked 2026-10-30 against the vendors' pages; the Windows server will show what each does in practice.

### The Go module

One Go package tree in the repository's module, with the OS differences behind build tags and none in the shared logic:

| Part | Replaces | Size today |
|---|---|---|
| a launcher package and an `agent-bus-launcher` command, or a `launcher` subcommand of `agent-bus` | `src/launchers/*.ts` | 1.4k lines, 1.7k of tests and smokes |
| an MCP face package, run as `agent-bus mcp` | `src/mcp/*.ts` (face, Codex and OpenCode adapters, channel push, resources) | 2.5k lines, 2.4k of tests |
| `ab-claude.cmd`, `ab-codex.cmd`, `ab-opencode.cmd` shims | the POSIX `sh` launchers | trivial |

Nothing in it is Windows-only except what must be: the per-OS parts are the places the TypeScript already treats specially.

| Need | TypeScript today | Go module: shared logic | Windows part | Linux and macOS part |
|---|---|---|---|---|
| Session bindings and locks | rename a directory onto an existing one, read `EEXIST` | one lock interface | `LockFileEx` on a lock file | `flock` or the rename scheme |
| Process tree | `process.kill(-pid)` | one `Stop(tree)` | a job object | process groups |
| Live holder and take-over | `SIGUSR2` and `/proc/<pid>/stat` | the holder check by start time | `--take-over` refused with a message ([decisions](DECISIONS.md#recorded-decisions)); start time from the process API | `SIGUSR2`, `/proc` or `ps` |
| Who owns a port | `/proc/net/tcp`, `lsof`, `ps` | one question | `netstat -ano` or the IP helper API | the current readers |
| Entry and state | `XDG_*`, `~/.local/state` | one state-directory function | `%LocalAppData%\agent-bus` | XDG as today |
| Terminal title and colours | OSC, kitty, Konsole | the OSC part | OSC in Windows Terminal | kitty and Konsole |
| Reaching the bus | `fetch` over a socket or TCP | the CLI's own client, pinned TLS | TCP/TLS only | unix socket and TCP |

Reaching the bus in Go reuses the CLI's client, so the face and the launcher gain no second client.

### What it costs

| Part | Days |
|---|---|
| Launcher in Go: argument handling, bindings, identity and claim, restart, rename, tags, plugin steps, Claude and Codex adapters | 14 |
| MCP face in Go: tools, resources, channel push, Codex and OpenCode adapters | 12 |
| `ab-opencode` (adapter and launcher part) | 3 |
| Windows parts, shims, packaging in the zip | 3 |
| Keeping the shared logic OS-free: the interfaces above, a test per interface on Linux | 3 |
| Windows CI job for the module's tests | 1.5 |
| Docs | 1.5 |
| Total | 38 (32 to 50) |

Two risks inside it. The face today uses the TypeScript MCP SDK, so the Go face either hand-writes the stdio protocol it needs (initialize, tools, resources and the Claude channel notification) or takes a Go SDK that meets the revision in use: [Q268](QUESTIONS.md#open-questions). And a Go face and a TypeScript face would both exist until the R1.2 switch, so a tool change before then lands twice.

### Rejected alternative: limited launchers on bun

The compiled `agent-bus-ts.exe` with the parts that cannot work on Windows removed (take-over, process-group kill, the `/proc` checks, kitty and Konsole colours) would cost about 8.5 days. The owner chose Go instead: no bun on Windows, and the module is the first half of the Go-only client. Kept here as the cost of not doing it.

### R1.2: Linux and macOS move to the Go client

Linux and macOS keep bun until then. The switch is R1.2 work, [Go-only client on Linux and macOS](../R1.2/TODO.md#go-only-client): the `ab-*` shims and the Claude channel plugin call the Go binaries, `agent-bus-ts` stays for the web face only, and the launcher tests and smokes run against the Go module. About 10 days plus a soak on this team's own windows, after the Windows module exists. It does not touch R1.

## What stays out

| Out | Why |
|---|---|
| Daemon, setup, admin | Linux services and accounts; macOS gets a reduced one in [MAC.2](../R1.0-Release/macos.md#mac2-daemon-with-a-reduced-setup), Windows none |
| Runner and sandbox | cgroups and bubblewrap; a sandbox request would be refused anyway |
| `agent` (keeper) and `run` | owner, 2026-10-30; refused with a message naming them |
| `ab-opencode1` (the archived OpenCode 1.x launcher) | archived; `ab-opencode` is in |
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
| 5 Go launchers and face (WIN.3) | [what it costs](#what-it-costs) | 38 |
| Total | | 43 (range 36 to 55). The real-server run (WIN.1b) is not in it |

The CLI part is 5 days, down from 6: no state directory and no file-name encoding without the keeper. The earlier bun option for the launchers was 8.5 days and is rejected.

## Release

R1.1, owner-moved 2026-10-30, the Go launchers with the CLI. R1's 11-04 date holds: nothing here is in R1's critical path, and the shared files (`internal/proctitle`, `package.sh`) are the ones [MAC.1](../R1.0-Release/macos.md#mac1-client-and-runner-on-macos) already touched. Acceptance is in [WIN.1](TODO.md#windows-cli-client) to WIN.3.
