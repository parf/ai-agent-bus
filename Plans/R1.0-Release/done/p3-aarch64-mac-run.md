# P.3 aarch64 run on the owner's Mac

📌 **TL;DR:** 2026-10-30, steps 1 and 2 of the [Mac run list](../mac-run-list.md#a-linux-aarch64) pass on the Mac's arm64 podman machine, twice (0.9.220, then 0.9.222). Step 4 starts after J58 and passes 20 checks, then fails the same way on four runs: setup restarts the web face's unit so fast that systemd refuses the next start. Step 3 (a signed set) was dropped by [RD178](../DECISIONS.md#recorded-decisions): there is no owner signing step.

## Host

| | |
|---|---|
| Mac | `uname -m` arm64, macOS 26.5.2 |
| Podman | 5.4.2, the existing machine `podman-machine-default` (applehv, Fedora CoreOS 41, aarch64, 5 CPUs, 12.6 GiB); not created or resized |
| Build image | `docker.io/library/golang:1.26-bookworm` (go1.26.10 linux/arm64, gcc), bun 1.4.4 installed in the container |
| Install image | `docker.io/library/debian:13` base, the arm64 choice J58 made in `fresh-install.sh` |
| Source | `git archive` of main, extracted into a new directory under `~/agent-bus-dev`; the Mac's clone was not touched. First `c38b05d0` (0.9.220), then `94970509` (0.9.222, with J58) |

## Steps

| Step | Result | Time |
|---|---|---|
| 1 Build | `package.sh ../tmp/dist` in the container: `agent-bus-0.9.222-linux-aarch64.tar.gz` and `agent-bus-client-0.9.222-linux-aarch64.tar.gz`, each with `.sha256`; exit 0. (0.9.220: 83958650 and 62987713 bytes.) | image pull 11 s; apt, bun, `bun install` and the build 173 s in all; `package.sh` itself 24 s |
| 2 Evidence | `release-verify: agent-bus-0.9.222-linux-aarch64 ok (version 0.9.222, full, queried)` and `release-verify: agent-bus-client-0.9.222-linux-aarch64 ok (version 0.9.222, client, queried)`; exit 0, so the aarch64 binaries ran. 0.9.220 gave the same | 4 s |
| 3 Release set | not run here: the run list's step 3 now verifies the combined native artifacts with `SHA256SUMS` before npm wrapping, with no owner signing ([RD178](../DECISIONS.md#recorded-decisions)); at the time of this run it was written as waiting on the owner's signing | |
| 4 Clean install | `fresh-install.sh` with J58 builds the Debian 13 systemd image and runs: 20 `PASS` lines (packaged checksum, real unit under the service account, accounts and sockets, build info, setup over a running node, web sign-in, a script agent, bundled agents, idempotent reinstall, samples, TLS on the daemon port) and then `FAIL the web face does not answer TLS`, exit 1; the same on four runs | first run 21 s with the image build; later runs 8 s |

## Why step 4 fails

A copy of the fixture that dumps the unit's journal when a check fails shows it. The web face starts and listens, and `setup` restarts its unit about eight times in six seconds (once per change); the last start, the one with TLS (`listening on https://127.0.0.1:6780/`), is stopped by the next restart, and systemd then refuses a start: `Start request repeated too quickly` and `Failed with result 'start-limit-hit'`. The unit is left failed, nothing listens on 6780, and `curl` says `Failed to connect … after 0 ms`.

On x86_64 the same fixture passes, so the restarts there are slower than systemd's start limit. What it means for a real host is a question for `agent-bus-setup`, not for the Mac run: on a fast host, setup can leave the web face dead after a run of changes. Not fixed here; recorded as a finding for the lead.

## MAC.1 R2 on the same Mac

- **Authorization:** `#ab-ceo`, on the owner's ruling of 2026-10-30 (no token for the owner's own login; a credential issued for the run, handed over privately, revoked after). It registered `#ab-mac@parf.us` and `#ab-mac-echo@parf.us`, put each token only on the Mac in a mode-600 file outside any repository, and removed both files and the registrations at the end. No credential value is recorded anywhere in the repository.
- **Path:** a reverse tunnel bound to the Mac's loopback, `127.0.0.1:16767` to the node's daemon (the Mac has no route to it), open only for the run; the pin was the node certificate's fingerprint (public). The forwarded login socket was offered and not used.
- **Attempts:** the first three stopped at the credential. `run` registers the agent under the launcher, then asks for that agent's own token (`run.go:121-137`); that is allowed for the daemon admin, the agent itself, or the record's Owner, so a Maintainer grant did not help. The agent also removes its record when it stops cleanly with an empty queue, so a token issued for one run is stale for the next. The fourth attempt used the agent's own fresh token: `run` served `sh -c "id -un"`, and a call from the same identity answered `parf`, the account that started the runner, sandbox off.
- **Not run:** R3, a standard non-administrator account: optional in the plan, and MAC.2 proved the non-root service account.

## Why step 4 fails: the journal

The failing run's unit journal, taken from a copy of the fixture that prints it when a check fails (four runs failed the same way; only the last printed the journal), in order: `Started` and `listening on http://127.0.0.1:6780/` three times, then a restart with `listening on https://127.0.0.1:6780/ (plain HTTP there is redirected)`, then `Stopping`, `Deactivated successfully`, `Start request repeated too quickly`, `Failed with result 'start-limit-hit'`, `Failed to start`. About eight starts fall in six seconds.

## Cleaned up

The build container ran with `--rm`; the three images this run pulled or built (`golang:1.26-bookworm`, `debian:13`, `agent-bus-fresh-install:debian13-systemd`) were removed by ID, and the run directory under `~/agent-bus-dev` is gone. `du -sh ~/agent-bus-dev`: 57M, the clone.
