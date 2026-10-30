# Mac run list

📌 **TL;DR:** What the owner's Mac must run on 2026-10-31 to close the aarch64 half of [P.3](TODO.md#objective): build the Linux aarch64 archives there, run the release evidence check on them, then a clean install and `check-deploy` in a Linux arm64 container. The darwin-arm64 build runs only if [Q221](QUESTIONS.md#distribution) says R1 ships it. Everything here is a plan; nothing has run yet.

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

Evidence back to the repository: the step 2 output and `result.log` pasted into a plan `done/` file (not `tmp/`), the real times, and the Mac's `uname -m`, podman and Go versions. P.3 then drops its "aarch64 pending the owner's Mac" line.

Build natively, never cross: `package.sh` cannot cross-build (a Go dependency needs cgo), so the aarch64 archives are built on the Mac, in the arm64 podman machine, and nowhere else. #ab-w1a's harness `src/acceptance/distro/run.sh` follows the host CPU, so on the Mac it runs on arm64. A CI build on a tag would publish a release, which only the owner does, so it stays a note, not a step.

## The Mac session

Owner, 2026-10-30 ([RD164](DECISIONS.md#recorded-decisions)): the run list is carried out by a Claude session on the owner's Mac, on the same LAN as this node. This node has no ssh to the Mac; the session reaches the team over the bus.

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
| Does R1 ship it? | decided by [Q221](QUESTIONS.md#distribution) (which parts ship, and how); the Mac is [RD164](DECISIONS.md#recorded-decisions) and the binaries are unsigned ([RD165](DECISIONS.md#recorded-decisions)), so Gatekeeper will warn |
| If no | nothing runs; macOS stays out of R1 and P.3 is Linux only |
| If yes | `package.sh` refuses non-Linux hosts today, so the first step is code, not a Mac run: a client edition build for darwin-arm64 (`GOOS=darwin GOARCH=arm64` for `agent-bus` and `agent-bus-runner`, `bun build --compile --target=bun-darwin-arm64` for `agent-bus-ts`). The Mac then runs `agent-bus --version`, `agent-bus-runner --version` (first line the release version, second `build_info:`), the client `install.sh --client` against a throwaway bus reached over HTTPS, and one request/reply; Gatekeeper's verdict on the unsigned binaries is recorded as evidence |

Time and exact commands for darwin are set when Q221 is answered.
