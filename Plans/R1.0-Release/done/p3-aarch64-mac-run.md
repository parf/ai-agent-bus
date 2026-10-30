# P.3 aarch64 run on the owner's Mac

📌 **TL;DR:** 2026-10-30, steps 1 and 2 of the [Mac run list](../mac-run-list.md#a-linux-aarch64) pass on the Mac's arm64 podman machine; step 4 cannot start there, for two concrete reasons in the harness. Step 3 waits on the owner's signing.

## Host

| | |
|---|---|
| Mac | `uname -m` arm64, macOS 26.5.2 |
| Podman | 5.4.2, the existing machine `podman-machine-default` (applehv, Fedora CoreOS 41, aarch64, 5 CPUs, 12.6 GiB); not created or resized |
| Build image | `docker.io/library/golang:1.26-bookworm` (go1.26.10 linux/arm64, gcc), bun 1.4.4 installed in the container |
| Source | `git archive` of main `c38b05d0`, version 0.9.220, extracted into a new directory under `~/agent-bus-dev`; the Mac's clone was not touched |

## Steps

| Step | Result | Time |
|---|---|---|
| 1 Build | `package.sh ../tmp/dist` in the container: `agent-bus-0.9.220-linux-aarch64.tar.gz` (83958650 bytes) and `agent-bus-client-0.9.220-linux-aarch64.tar.gz` (62987713 bytes), each with `.sha256`; exit 0 | image pull 12 s; apt, bun, `bun install` and the build 172 s in all; `package.sh` itself 24 s |
| 2 Evidence | `release-verify: agent-bus-0.9.220-linux-aarch64 ok (version 0.9.220, full, queried)` and `release-verify: agent-bus-client-0.9.220-linux-aarch64 ok (version 0.9.220, client, queried)`; exit 0, so the aarch64 binaries ran | 5 s |
| 3 Signed set | not run: waits on the owner's signing | |
| 4 Clean install | did not start. `fresh-install.sh` stops at `realpath: illegal option -- m` (macOS `realpath` has no `-m`); and `podman pull docker.io/library/archlinux:latest` answers "no image found in image index for architecture arm64" | under 1 s |

## What step 4 needs

The run list said steps 1, 2 and 4 need nothing new; step 4 does:

- `src/acceptance/fresh-install.sh` and its siblings use `realpath -m`, which macOS lacks: a portable replacement, or run the harness inside a Linux machine;
- `src/acceptance/fresh-install.Containerfile` builds from `archlinux:latest`, which has no arm64 image: an arm64 base with systemd, such as the distributions [P.4](../TODO.md#distribution-acceptance) already installs on.

This is J58, a change in `src/acceptance/` owned by `#ab-w1c`; step 4 is rerun on the Mac after it lands.

## Cleaned up

The build container ran with `--rm`; the pulled build image and the run directory under `~/agent-bus-dev` are removed by ID at the end of the run.
