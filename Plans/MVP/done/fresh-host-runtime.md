# Fresh-host runtime acceptance

📌 **TL;DR:** The runtime half of the fresh installed release gate passed on
2026-09-23 with 0.8.30. The package was installed by its own
`agent-bus-setup` as a real systemd node on a disposable host with no `/rd`, no
checkout and no Go. The installed `ab-codex`, `ab-opencode` and `ab-claude`
passed delivery, the MCP minimum, launch, failure handling, naming and recovery:
16 gate runs, 0 failures. Two launcher defects were found and fixed in 0.8.30.

## Scope

The gates are the development-host ones ([delivery](runtime-delivery.md#checks),
[MCP minimum](mcp-minimum.md#checks), [launch](runtime-launch.md#checks),
[recovery](runtime-recovery.md#checks)). Here they run against
`/usr/local/bin` on the fresh host. One gate is new:
[runtime-installed.ts](../../../src/acceptance/runtime-installed.ts). An actual
mapped account runs an installed launcher with no bus address, token or name.
The launcher must find the systemd node through that account's own socket, and
the session must answer a peer. The other gates start disposable daemons from
the installed `agent-busd`. The live node, its unit and its accounts were not touched.

## Checks

```
src/package.sh "$PWD/tmp/frt/dist"
src/acceptance/fresh-runtime.sh tmp/frt/dist/agent-bus-0.8.30-linux-x86_64.tar.gz tmp/frt/final2 /home/parf/.claude2
```

The [driver](../../../src/acceptance/fresh-runtime.sh) bundles the gate programs
with `bun build`, so no source tree goes onto the host. It mounts the archive,
the bundles and read-only copies of the runtime executables. It then runs the
[container script](../../../src/acceptance/fresh-runtime-container.sh) on two
disposable hosts. **Result: exit 0.** Evidence is under `tmp/frt/final2/`.

| Host condition | Offline host | Claude host |
|---|---|---|
| Image | `localhost/agent-bus-fresh-install:arch-systemd` (Arch Linux, systemd 261.3, glibc 2.44), `--privileged --systemd=always`, hostname `fresh` | same |
| Network | `--network=none`: only `lo`, and `api.anthropic.com` unreachable (asserted) | `pasta`, for the real Claude model only |
| Absent | `/rd`, any `smoke.sh`, `go.mod` or `*.go`, a Go toolchain (asserted) | same |
| Installed | 0.8.30 archive, checksum verified, `agent-bus-setup --owner owner@fresh`; `alice@fresh` added and mapped to the account `alice` | same |
| Runtimes | Codex CLI 0.156.1, OpenCode 1.18.30 (upstream build), Claude Code 2.1.281 in `/usr/local/bin`; Bun 1.4.2 in `/usr/bin` | same |
| Second accounts | `agent-bus-runner` (created by setup), `nobody` | same |
| Claude login | none; a literal dummy key against the loopback model | `.credentials.json` and the account fields of `.claude.json`, copied into `alice`'s disposable profile and deleted afterwards; never in the image or the evidence |

Every gate runs as `alice` under `timeout`. Its log is kept as
`evidence-<host>/<gate>.log`, and `commands.txt` holds each command.

| Gate (harness) | Codex | OpenCode | Claude |
|---|---|---|---|
| Installed node (`runtime-installed`) | 13/0 | 13/0 | 14/0 (real model, channel event answered by `ab_reply`) |
| Interactive, MCP minimum, H.9.5 (`runtime-interactive`) | 23/0 | 23/0 | 19/0 (dummy key, filtered `ab_consume`) |
| H.8 channel delivery (`claude-channel-live`) | — | — | 0 failed |
| H.9, H.9.1 (`runtime-launch launch,failure`) | 63/0, enforced mode included | 62/0 | 56/0 |
| H.9.3 (`runtime-launch names`) | 57/0 | 57/0 | 52/0 |
| H.9.6 (`runtime-recovery`) | 68/0 | 68/0 | 54/0 |

The installed-node gate asserts these points. The account's socket
authenticates `alice@fresh`. The session registers under that principal with
this host's realm. A peer, using its own token on the shared socket, gets
exactly one reply: from the session, addressed to the peer, correlated by topic
and tag, and carrying its random value. For Codex and OpenCode, the runtime's
own MCP `ab_send` carried the reply. On SIGTERM, every process of the session
exits, and its reader leaves the installed node. The launch gate's "no bus"
case hides the installed node's `/run/agent-bus` inside a namespace.

## Defects found

| Before | After, 0.8.30 |
|---|---|
| The launcher named the account socket with Bun's `userInfo()`. That reads `$USER`, or gives `unknown` without it. An installed launcher started without `USER` looked for `user-unknown.sock` and quietly ran a plain, bus-less session. A changed `USER` named another account's socket | [local.ts](../../../src/launchers/local.ts) finds the account by uid (`id -nu`), as the daemon and the Go CLI do ([smart launchers](../../../docs/08-runner-role.md#smart-launchers)) |
| `claude mcp add` exits 0 and prints "Added" when its configuration directory is read-only and nothing was written, so a failed channel registration went unreported | The launcher checks the configuration file itself. The launch gate's Claude fixture now makes the directory read-only; making only the file read-only never failed, because Claude replaces the file by rename |

## Mutations

Each mutant is a copied tree with one edit. It is packaged by `package.sh` and
run through the same driver on a new host. **3/3 caught at an assertion.**

| Mutant | Gate | Named failure |
|---|---|---|
| `local.ts` back to `userInfo().username` | `launchers/local.test.ts` | `Received: …/user-someone-else.sock` |
| same, packaged (`tmp/frt/mut-user/`) | installed-codex, installed-opencode | `timed out: the session's inbox has its reader on the installed node` |
| channel check back to the exit status alone (`tmp/frt/mut-channel/`) | launch-claude | `[helper-failure] timed out: the launcher reports the failed channel registration` |

The first form of the installed-node gate sent its peer message through the
account socket. That socket always speaks as its account, so the "peer" was
`alice@fresh`, and Claude's 13 checks passed on a reply to the wrong
principal. The peer now uses the shared socket, and the reply's `to` is
asserted.

## Review

A read-only review from the live OpenCode session of `git diff main...HEAD`
found no either-way checks, leaks or wrong-process signals. It raised five low
findings:

| Finding | Outcome |
|---|---|
| The TL;DR said 18 gate runs; there are 16 | Fixed |
| The login copy was staged inside the evidence directory, removed only by the EXIT trap | Fixed: it is staged beside it, as `<out>.login` |
| An unchecked `systemctl restart` and an unbounded socket wait would be blamed on alice's socket | Fixed: each fails with its own sentence |
| Without `id` on `PATH`, discovery finds no account and the launcher starts a plain session | Kept. `id` is coreutils, and the launcher needs `bun` on the same `PATH` |
| A swallowed transcript wait can hide a slow transcript behind the next check's name | Kept. The next two checks re-assert the transcript directly |

The gate was rerun after the fixes (`tmp/frt/final2/`).

## Repository verification

`PORT=38000 src/smoke.sh --slow` passed **784/0** on the 0.8.30 tree.

## Not claimed

- The Codex and OpenCode models are the loopback fixture, not a provider.
- Distributions other than Arch Linux.
- Bun and the runtimes are prerequisites the host brings; the package installs neither.
