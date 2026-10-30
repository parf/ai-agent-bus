# Mac run list

📌 **TL;DR:** Linux aarch64 release-archive acceptance remains the owner's separate run. The darwin client and runner follow [MAC.1](macos.md#mac1-client-and-runner-on-macos); MAC.2's native daemon/setup checks have [recorded results](done/mac2.md#checks-and-results). Worker Mac runs are focused, through SSH; containers and mutants stay on this node.

## Prerequisites

| Need | Why |
|---|---|
| This repository checked out at the release commit, clean | the archives are built from it |
| `podman` with a running arm64 Linux machine (`podman machine init && podman machine start`), 4 GB RAM, 20 GB disk | the build and the install run in Linux arm64, never on macOS itself |
| Inside that machine: Go (as in `src/go.mod`), a C compiler, `bun`, `sudo`, `tar`, OpenSSH 8.1+ | `src/package.sh` builds Go with cgo and compiles the TypeScript parts with bun |
| Network for the first image build and package downloads | `src/acceptance/fresh-install.Containerfile` pulls its base image and bun |

Run the machine's commands in a shell inside it (`podman machine ssh`), with the repository mounted or cloned there. On an arm64 Linux the host CPU is aarch64, so `package.sh` needs no `GOARCH` and no cross compiler.

## a) Linux aarch64

| Step | Command (in the Linux arm64 machine, from the repository root) | Expected | Time (estimate, record the real one) |
|---|---|---|---|
| 1 Build | `src/package.sh tmp/dist` | prints four paths: `agent-bus-<ver>-linux-aarch64.tar.gz` and `agent-bus-client-<ver>-linux-aarch64.tar.gz`, each with `.sha256` | x86_64 took 17 s here; allow 1-3 min emulated or native |
| 2 Evidence, archives | `src/release-verify.sh tmp/dist/agent-bus-*-linux-aarch64.tar.gz` | one `release-verify: <name> ok (version <ver>, full\|client, queried)` per archive, exit 0; `queried` proves the binaries ran | seconds |
| 3 Evidence, signed set | none yet: a signed four-archive set needs the x86_64 archives from CI or this node beside these, and the owner's signing ([RD163](DECISIONS.md#recorded-decisions)). Until then step 2 is the aarch64 evidence | | |
| 4 Clean install | `src/acceptance/fresh-install.sh tmp/dist/agent-bus-<ver>-linux-aarch64.tar.gz tmp/mac-fresh` (needs a new output directory; it builds its own real-systemd container) | `PASS` lines ending the run, exit 0; `tmp/mac-fresh/evidence/result.log` holds them | several minutes, mostly the image build |
| 5 Deploy check | `fresh-install.sh` stops its container when it ends, so `src/check-deploy.sh` cannot run in it as it is. It needs the harness to keep the container, or run `check-deploy` before exit (a change in `src/acceptance/`, team 1 with the podman distro harness). Then: `podman exec <container> check-deploy` as an allowed caller | exit 0, no failed or missing selftest | seconds |

Step 5 is code first, a Mac run second: the run list is not complete until that change lands. Steps 1, 2 and 4 need nothing new.

Evidence back to the repository: the step 2 output and `result.log` pasted into a plan `done/` file (not `tmp/`), the real times, and the Mac's `uname -m`, podman and Go versions. Then the P.3 bullet in [Distribution acceptance](TODO.md#distribution-acceptance) loses its "aarch64 checked on the owner's Mac" note, and the Objective's "macOS test host" line and the "Later" row of [simple install](distribution.md#simple-install) are brought up to date.

Build natively, never cross: `package.sh` cannot cross-build (a Go dependency needs cgo), so the aarch64 archives are built on the Mac, in the arm64 podman machine, and nowhere else. #ab-w1a's harness `src/acceptance/distro/run.sh` (it lands with T-6a; not on main yet) follows the host CPU, so on the Mac it runs on arm64. A CI build on a tag would publish a release, which only the owner does, so it stays a note, not a step.

## The Mac session

The table below retains the earlier owner-managed Linux aarch64 session plan ([RD164](DECISIONS.md#recorded-decisions)). MAC.2 instead runs focused native checks through `ssh mac` from this node, using only the existing checkout and workspace-local caches; its [execution record](done/mac2.md#source-and-limits) owns those details. The table is not an instruction for the MAC.2 worker to create a bus credential or run containers.

| Item | What the session gets |
|---|---|
| Bus address | `AGENT_BUS_ADDR=https://<this node's LAN host>:<port>` and `AGENT_BUS_TLS_FINGERPRINT=sha256:…` of the node's self-signed certificate ([setup § TLS](../../docs/07-setup.md#tls)); the owner supplies both, nothing here names them |
| Token | one bus token for one agent name made for this (for example `#ab-mac`), issued by the owner with `agent-bus token`; handed over outside the repository and this chat; never written into a file the session commits |
| Checkout | a clone of `parf/ai-agent-bus` at the commit the run names, or a `git bundle` the owner copies over if that commit is not yet public; read-only: the session pushes nothing and edits nothing |
| Tools | `podman` with a running arm64 machine; the Linux machine needs Go, a C compiler and `bun` as the prerequisites table says; the Mac itself needs neither Go nor bun for the Linux aarch64 run |
| Run list | this file, section [a)](#a-linux-aarch64) |
| Evidence back | one bus message to `#ab-lead1` (topic `mac-run`, tag `ab-dvp`) naming each step with its exit status and its real time, plus the step 2 and step 4 output; the team files it in a plan `done/` file. Failures are reported as they are, with the output |

## b) darwin-arm64

| Question | Answer |
|---|---|
| Does R1 ship it? | yes, in steps ([RD166](DECISIONS.md#recorded-decisions)): the client and runner first, as [MAC.1](macos.md#mac1-client-and-runner-on-macos); the Mac is [RD164](DECISIONS.md#recorded-decisions) and the binaries are unsigned ([RD165](DECISIONS.md#recorded-decisions)), so Gatekeeper will warn |
| What runs | `package.sh` refuses non-Linux hosts today, so the first step is code, not a Mac run: a client edition build for darwin-arm64 (`GOOS=darwin GOARCH=arm64` for `agent-bus` and `agent-bus-runner`, `bun build --compile --target=bun-darwin-arm64` for `agent-bus-ts`). The Mac then runs `agent-bus --version`, `agent-bus-runner --version` (first line the release version, second `build_info:`), the client `install.sh --client` against a throwaway bus reached over HTTPS, and one request/reply; Gatekeeper's verdict on the unsigned binaries is recorded as evidence |

Exact commands and times for darwin are set when [MAC.1](macos.md#mac1-client-and-runner-on-macos) is built.


## c) darwin-arm64 daemon

The focused native MAC.2 run is recorded in [the evidence](done/mac2.md#checks-and-results), including the source, commands, timings and sudo list. It builds the daemon, setup and admin directly; it does not claim a full darwin archive install.


Full archive validation follows MAC.1/distribution. All mutants stay on this node, under smoke-gate; never run them on the Mac. The owner completed [role-account cleanup](done/mac2.md#owner-cleanup-verified), verified read-only by the CEO; no further worker sudo is authorized.

## d) client install and runner on macOS (MAC.1)

What the Mac checks of [MAC.1](macos.md#mac1-client-and-runner-on-macos) beyond the CLI and the MCP face ([Mac run: CLI and MCP face](mac-cli-run.md#mac-run-cli-and-mcp-face)): the installer, then the runner. The release is signed with a throwaway key made on the development node, so nothing here touches the real key or a real release.

### Make the throwaway release (on the development node, before the Mac run)

| Step | Command | Expected |
|---|---|---|
| 1 archives | `GOOS=darwin GOARCH=arm64 src/package.sh <dir>`, then the same with `GOARCH=amd64` | the two `agent-bus-client-<version>-darwin-<aarch64\|x86_64>.tar.gz` and their `.sha256` in `<dir>` |
| 2 release | `src/acceptance/mac-throwaway-release.sh <dir> <out> [port]` | `<out>/serve/download/v<version>/` with the two macOS archives, four Linux stand-ins, `SHA256SUMS` and its signature; `<out>/install-throwaway.sh`, a copy of `install.sh` that trusts only that key and fetches from `http://127.0.0.1:<port>`; the private half of the key is deleted |

The owner copies `<out>` to the Mac. For the run of 2026-10-30 it is `tmp/mac-out/throwaway/` beside `tmp/mac-out/archives/` in the worktree `tmp/ab-w1b-mac1` on the development node.

### On the Mac

Needs `python3`, `curl`, `tar`, `ssh-keygen` and `shasum`, all in the system or the Xcode command line tools. Work in a home of its own so nothing touches the real one: `export HOME=~/ab-mac-home; mkdir -p $HOME` in every shell below. Report what you see; do not repair.

| Step | Command | Expected |
|---|---|---|
| I1 serve | `cd <out>/serve && python3 -m http.server 8787 --bind 127.0.0.1 &` | listens on 8787; leave it running |
| I2 install | `sh <out>/install-throwaway.sh --client --version <version>` | the release key's fingerprint, then `client edition <version> is in $HOME/.local/lib/agent-bus; commands are linked into $HOME/.local/bin`; exit 0 |
| I3 version | `export PATH=$HOME/.local/bin:$PATH; agent-bus --version; agent-bus-runner --version` | each: first line `<version>`, second `build_info: …`; neither says `cannot execute`, `killed` or `bad CPU type` |
| I4 update keeps one old | run I2 again with the same version | `client edition <version> is already current`; nothing changes |
| I5 tampered refused | copy `<out>` to `<out>-bad`, flip one byte in its `agent-bus-client-<version>-darwin-*.tar.gz` for your CPU (`printf '\x00' \| dd of=<file> bs=1 seek=100 conv=notrunc`), serve that copy on port 8788 after editing the port in a copy of `install-throwaway.sh`, install into a second `HOME` | `does not match its signed hash: nothing installed`; the second `HOME` holds no `.local` |
| I6 full node refused | `sh <out>/install-throwaway.sh --version <version>` (no `--client`) | `on macOS only the client edition installs: use --client`; nothing written |

Needs the bus from [the Mac session](mac-run-list.md#the-mac-session): `AGENT_BUS_ADDR`, `AGENT_BUS_TLS_FINGERPRINT`, one token and `AGENT_BUS_NAME`, with a name the token may register agents under (the owner says which).

| Step | Command | Expected |
|---|---|---|
| R1 sandbox refused | `perl -e 'alarm 20; exec @ARGV' agent-bus run '<name>-sbx' 'echo hi' --sandbox=on; echo exit=$?` | non-zero exit with a message that a sandbox is not available; it must not print `hi` and must not stay running (an alarm kill is a failure) |
| R2 a script agent runs | `perl -e 'alarm 60; exec @ARGV' agent-bus run '<name>-echo' 'id -un' --allow='*' &`, then `agent-bus call '<name>-echo' go` | the reply is the account that started the runner; stop it with `kill %1` |
| R3 not the owner's login | optional, the owner decides: repeat R2 as a standard, non-administrator account (`sudo -u <account> …`) | the reply is that account, not yours: the runner's scripts hold only that account's rights |

### Evidence back

As in [the CLI run](mac-cli-run.md#evidence-back): one bus message to `#ab-lead1`, topic `mac-run`, with `uname -m`, `sw_vers -productVersion`, every step's number with its exit status, and the output of I2, I3, I5, I6, R1 and R2. Never the token.
