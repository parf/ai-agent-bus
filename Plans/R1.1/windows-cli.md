# Windows CLI client

📌 **TL;DR:** A Windows build of the `agent-bus` CLI only: it reaches a remote daemon over pinned HTTPS, as the [Mac client](../R1.0-Release/macos.md#order) does. No daemon, runner, sandbox or launchers. About six working days in four steps, proved by a cross-build in every gate plus one real Windows run in CI. Placed in R1.1 so R1's 11-04 does not move. Owner reversal of the 2026-10-19 "no windows"; open choices are [Q261–Q264](QUESTIONS.md#open-questions).

## Why

- A Windows user can send, call and read on a bus without a Linux box of their own.
- The client is one static Go binary; the work is removing Unix assumptions, not writing a new client.

## What you get

| Outcome | Where it stops |
|---|---|
| `agent-bus ls`, `send`, `call`, `consume`, `kv`, `lock`, `token`, `status` against a daemon on another machine | no local daemon, no local socket discovery |
| Pinned HTTPS with `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN`, `AGENT_BUS_TLS_FINGERPRINT` | same variables as Linux and macOS |
| A `agent-bus-client-<version>-windows-x86_64.zip` in the release, verified like the others | arm64 Windows follows only if asked ([Q264](QUESTIONS.md#open-questions)) |

## What a Windows build hits

Measured on this node, 2026-10-30, `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/agent-bus`. The compiler reports one error group per package, so the table also lists what a read of the CLI's dependencies found.

| Area | Where | Today | Plan |
|---|---|---|---|
| Process title | `internal/proctitle` (`gspt`) | no Go file builds off Linux and macOS cgo | a no-op beside the cgo one, the same split [MAC.1](../R1.0-Release/macos.md#mac1-client-and-runner-on-macos) adds for macOS; the Windows build reuses it |
| File lock | `internal/authkeys` (`syscall.Flock`), `cmd/agent-bus/agent.go` | undefined | per-OS lock file: `LockFileEx` through `golang.org/x/sys/windows`, already a module dependency |
| Symlink-refusing read | `internal/agentmeta` (`O_NOFOLLOW`) | undefined | per-OS open: on Windows refuse a reparse point after the open; the CLI reads only a file the user names |
| Terminal test | `cmd/agent-bus/help.go` (`unix.IoctlGetTermios`) | undefined | `os.Stdout.Stat()` mode check, or `GetConsoleMode` on Windows; colour stays off when it is not a console |
| Keeper subcommand | `cmd/agent-bus/agent.go`: `Flock`, `Kill(pid, 0)`, `/proc/<pid>/fd`, `SIGTERM` | undefined | left out of the Windows build, with a refusal that names it ([Q262](QUESTIONS.md#open-questions)) |
| Signals | `run.go` `signal.NotifyContext(os.Interrupt, SIGTERM)` | compiles; `SIGTERM` is never delivered on Windows | keep; Ctrl-C works through `os.Interrupt` |
| Local socket | `internal/api/server.go`: `XDG_RUNTIME_DIR`, `os.Getuid`, `/run/agent-bus`, `unix` dial | compiles; the paths mean nothing | no discovery on Windows: `AGENT_BUS_ADDR` is required, and its absence says so |
| Peer credentials | `internal/api/peer_*` | per-OS stubs already exist | nothing |
| File modes | `0o700` and `0o600` on keys, state and tokens | accepted and ignored on NTFS | state under `%LOCALAPPDATA%\agent-bus`, protected by the user profile's ACL; tokens stay in the environment ([Q261](QUESTIONS.md#open-questions)) |
| Paths and home | `$HOME/.local/state`, `.ssh/id_ed25519` | `UserHomeDir` works; the XDG layout is Linux | the state directory above; the key proof keeps `.ssh\id_ed25519` and calls `ssh-keygen`, which Windows 10 1809 and later ship as the OpenSSH client |
| Child processes | `agent-bus run` (`exec.Command`) | compiles | refused or kept as a plain command ([Q262](QUESTIONS.md#open-questions)) |
| Names with `@`, `#`, `/` as file names | `agent.go` segment function | `#` is legal; `<>:"|?*` are not and `/` becomes a segment | the segment function (today it replaces only `/`) gains an encoding for the reserved characters, the device names (`CON`, `NUL`, `COM1` and the rest) and a trailing dot or space; it matters only for what Windows keeps by name, which is nothing while the keeper is left out ([Q262](QUESTIONS.md#open-questions)). A test names each case |

## Reaching a remote daemon

- Over HTTPS to `AGENT_BUS_ADDR` with the certificate pinned by `AGENT_BUS_TLS_FINGERPRINT`, as on the [Mac](../R1.0-Release/macos.md#order). The pin does not use the Windows certificate store.
- The token comes from `AGENT_BUS_TOKEN` or the key proof; the user sets environment variables in PowerShell (`$env:AGENT_BUS_ADDR`) or the user environment.
- The daemon decides who may do what; the Windows client adds no authority and no new call.

## What stays out

| Out | Why |
|---|---|
| Daemon, setup, admin | Linux services and accounts; macOS gets a reduced one in [MAC.2](../R1.0-Release/macos.md#mac2-daemon-with-a-reduced-setup), Windows none |
| Runner and sandbox | cgroups and bubblewrap; a sandbox request would be refused anyway |
| Launchers `ab-claude`, `ab-codex`, `ab-opencode*` | POSIX shell scripts; `agent-bus-ts` and the MCP face are a follow-up, since bun compiles for `bun-windows-x64` ([Q264](QUESTIONS.md#open-questions)) |
| `install.sh` | POSIX; Windows installs from the zip, or later through the npm wrapper |
| Code signing and SmartScreen | the binary is unsigned, so SmartScreen warns, as Gatekeeper does on the Mac |

## Testing without a Windows box

| Option | Proves | Cost | Verdict |
|---|---|---|---|
| Cross-build in the gate (`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build`, as the darwin build in `go_checks`) | every build tag and stub compiles, on every landing | about 10 s | do it |
| wine on this node | a PE binary starts | a root-installed package, ~200 MB, NTFS and ACL behaviour wrong, needs the gate lock | no |
| GitHub Actions `windows-latest` job on the release workflow | the zip runs: `--version` with its build stamp; `go test` of the CLI on Windows against an in-process pinned-TLS test daemon (the handler the CLI's own tests already use): a request and a reply, a wrong fingerprint refused, the state directory and the lock file | one Windows job, a few minutes, only on a tag the owner pushes | do it |
| A real daemon, reached from Windows | the same against the real daemon | needs a reachable, isolated daemon with TLS, a token and teardown, which a GitHub-hosted job cannot give a Windows job; it is a prerequisite, not a step ([Q263](QUESTIONS.md#open-questions)) | by hand on the owner's Windows machine against a daemon the owner names; not counted in the estimate |

Recommended: the first and third rows; the fourth is the owner's, once. Behaviour that only Windows can show (lock contention, ACL, console detection) is tested with `go test` in that CI job, never in the gate.

## Steps and estimate

| Step | Work | Days |
|---|---|---|
| 1 Build | tag the Linux-only files, add the per-OS lock, open, terminal and process-title files, a refusal for the left-out subcommands, `AGENT_BUS_ADDR` required; the gate cross-build | 2 |
| 2 Behaviour | state directory and the file-name encoding, key proof through `ssh-keygen`, Windows unit tests, the in-process TLS test daemon for the Windows CI job | 1.5 |
| 3 Package | `package.sh` writes the zip, `release-verify` reads it as it reads macOS archives, `release-sign` takes it with the others | 1.5 |
| 4 Docs | the setup guide's Windows page, the CLI guide's differences, the CI job | 1 |
| Total | | 6, range 5 to 8 if `ssh-keygen` proofs or the zip layout surprise; the real-daemon run is not in it |

## Release

R1.1, owner-moved 2026-10-30. R1's 11-04 date holds: nothing here is in R1's critical path, and the shared files (`internal/authkeys`, `internal/proctitle`, `package.sh`) are the ones [MAC.1](../R1.0-Release/macos.md#mac1-client-and-runner-on-macos) already touches, so it starts after MAC.1 lands. Acceptance is in [WIN.1](TODO.md#windows-cli-client).
