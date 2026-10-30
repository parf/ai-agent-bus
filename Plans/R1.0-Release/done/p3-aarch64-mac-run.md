# P.3 aarch64 run on the owner's Mac

📌 **TL;DR:** 2026-10-30, steps 1, 2 and 4 of the [Mac run list](../mac-run-list.md#a-linux-aarch64) pass on the Mac's arm64 podman machine: build and release evidence from 0.9.220 to 0.9.223, and the clean install (21 `PASS`, exit 0) from 0.9.223 once J59 stopped setup restarting units faster than systemd's start limit. The first install runs, on 0.9.222, failed and are explained below. Step 3 (a signed set) was dropped by [RD178](../DECISIONS.md#recorded-decisions): there is no owner signing step.

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
| 4 Clean install, first runs (0.9.222, J58 only) | `fresh-install.sh` with J58 builds the Debian 13 systemd image and runs: 20 `PASS` lines (packaged checksum, real unit under the service account, accounts and sockets, build info, setup over a running node, web sign-in, a script agent, bundled agents, idempotent reinstall, samples, TLS on the daemon port) and then `FAIL the web face does not answer TLS`, exit 1; the same on four runs | first run 21 s with the image build; later runs 8 s |
| 4 Clean install, after J59 (0.9.223, branch `ab-w1b/J59b`) | 21 `PASS` lines and exit 0: the web face restarts only when something it reads changed, and the fixture waits for the daemon's start-limit window before each setup run that restarts it | release build 172 s, `release-verify` 4 s, the install 28 s including the waits |

## The passing run's result

The final run, run by `#ab-ceo` on the Mac on 2026-10-30 from the J59b tree `351f1a9e` (version 0.9.224), native arm64, Debian 13 base. `release-verify` printed `release-verify: agent-bus-0.9.224-linux-aarch64 ok (version 0.9.224, full, queried)` and `release-verify: agent-bus-client-0.9.224-linux-aarch64 ok (version 0.9.224, client, queried)`, exit 0. `fresh.log`: 21 `PASS`, no `FAIL`, `fresh rc=0`. Timings (s): image pull 22, release build 185, `release-verify` 5, clean install 36.

The `PASS` lines of `result.log`, in order:

1. packaged checksum and archive
2. web missing artifact fails before host mutation
3. compiled missing artifact fails before host mutation
4. agent-helper missing artifact fails before host mutation
5. real generated systemd unit runs under its service account and starts API plus dashboard
6. first-user provisioning waits for the daemon account credential socket
7. two actual accounts discover their own sockets and authenticate as themselves
8. cross-account sockets and the tokenless shared socket are refused
9. credential-bearing sockets have their exact owner and mode
10. installed commands and the running node report the package's build_info (`root@… 2026-10-30 22:36:10`)
11. setup over a running node applies a changed unit, waits for the release, and reports the durable Owner
12. supervisor alone holds CAP_CHOWN; the bus holds none as agent-busd; the web face holds none as agent-bus-web; all with NoNewPrivs
13. the installed web face signs the owner in and serves the Overview
14. the web face's ps line names its version and counts the requests it served
15. new user calls a real script agent using only installed programs; a User is refused as an agent
16. complete stamped release is installed without build-host paths
17. bundled agents and their helpers load outside the checkout
18. identical package reinstall is idempotent
19. setup adds the sample node, the web face shows it, and removal takes exactly it away
20. setup turns TLS on with a self-signed certificate; the port answers both; the pinned CLI and the forced command agree, and a re-run keeps it
21. the web face serves TLS with its own copy, redirects plain HTTP, keeps /healthz, signs in with a Secure cookie, and --tls off takes it all back

Cleanup by explicit image ID and path: the three images and the run directory were removed, leaving `~/agent-bus-dev` at 57M (the clone).

## Why step 4 failed on the first runs

A copy of the fixture that dumps the unit's journal when a check fails shows it. The web face starts and listens, and `setup` restarts its unit about eight times in six seconds (once per change); the last start, the one with TLS (`listening on https://127.0.0.1:6780/`), is stopped by the next restart, and systemd then refuses a start: `Start request repeated too quickly` and `Failed with result 'start-limit-hit'`. The unit is left failed, nothing listens on 6780, and `curl` says `Failed to connect … after 0 ms`.

On x86_64 the same fixture passes, so the restarts there are slower than systemd's start limit. What it means for a real host is a question for `agent-bus-setup`, not for the Mac run: on a fast host, setup can leave the web face dead after a run of changes. Not fixed here; recorded as a finding for the lead.

## MAC.1 R2 on the same Mac

- **Authorization:** `#ab-ceo`, on the owner's ruling of 2026-10-30 (no token for the owner's own login; a credential issued for the run, handed over privately, revoked after). It registered `#ab-mac@parf.us` and `#ab-mac-echo@parf.us`, put each token only on the Mac in a mode-600 file outside any repository, and removed both files and the registrations at the end. No credential value is recorded anywhere in the repository.
- **Path:** a reverse tunnel bound to the Mac's loopback, `127.0.0.1:16767` to the node's daemon (the Mac has no route to it), open only for the run; the pin was the node certificate's fingerprint (public). The forwarded login socket was offered and not used.
- **Attempts:** the first three stopped at the credential. `run` registers the agent under the launcher, then asks for that agent's own token (`run.go:121-137`); that is allowed for the daemon admin, the agent itself, or the record's Owner, so a Maintainer grant did not help. The agent also removes its record when it stops cleanly with an empty queue, so a token issued for one run is stale for the next. The fourth attempt used the agent's own fresh token: `run` served `sh -c "id -un"`, and a call from the same identity answered `parf`, the account that started the runner, sandbox off.
- **Not run:** R3, a standard non-administrator account: optional in the plan, and MAC.2 proved the non-root service account.

## The web unit's journal on the first runs

The failing run's unit journal, taken from a copy of the fixture that prints it when a check fails (four runs failed the same way; only the last printed the journal), in order: `Started` and `listening on http://127.0.0.1:6780/` three times, then a restart with `listening on https://127.0.0.1:6780/ (plain HTTP there is redirected)`, then `Stopping`, `Deactivated successfully`, `Start request repeated too quickly`, `Failed with result 'start-limit-hit'`, `Failed to start`. About eight starts fall in six seconds.

## Cleaned up

The build container ran with `--rm`; the three images this run pulled or built (`golang:1.26-bookworm`, `debian:13`, `agent-bus-fresh-install:debian13-systemd`) were removed by ID, and the run directory under `~/agent-bus-dev` is gone. `du -sh ~/agent-bus-dev`: 57M, the clone.
