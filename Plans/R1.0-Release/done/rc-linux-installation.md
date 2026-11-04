# Linux RC installation evidence

📌 **TL;DR:** The first candidate's ordinary installation journeys exposed three documentation defects. They were collected before fixes, corrected together, then checked on five Linux distributions with the second candidate. These are isolated container results, not public release or native macOS acceptance.

## Candidates and order

| Candidate | Source commit | Full x86_64 archive SHA-256 |
|---|---|---|
| Local tag `1.0.0RC` | `10f6b481065e7e1af75c21bcb60115db2a0f88a0` | `cf40e4861a7c53f36895333e07aa2a1bd09990f803356423f32a5fd2264ddfe7` |
| Local tag `1.0.0RC2` | `ca7c7396cd057d9f0e1b22cd396f18c4d9492eaa` | `1767402e0982f7ead69eecf55937932e8e785f27258e060d0dfbe1ffaed47633` |

- Both candidates report package version `1.0.0`; the local tags identify the candidate iterations. Neither tag nor artifact was pushed or published.
- Both full and client archives passed `release-verify.sh`, including checksums, manifests, version queries and build stamps.
- Ubuntu, Arch and Debian collection finished before fixes. The owner later added AlmaLinux and Fedora; their original-candidate failures were collected before their corrected-candidate runs.
- The source difference between the candidates is documentation only: `543f137a`, independently read by W2C and landed after a normal gate, 1302 passed / 0 failed in 66 seconds. Runtime behavior and protocol are unchanged.

## Collected defects and fixes

| ID | Original observation | Correction in `543f137a` | Retest |
|---|---|---|---|
| RC-L01 | The archive's printed qualified owner installs, but a personal runner for local `alice` refuses because that plain bus User does not exist. Reproduced on Ubuntu, Debian and AlmaLinux. | Use the ordinary login's plain `$USER` in the archive example. | The unchanged extracted RC2 install block creates the same person that runner setup uses; all five managed agents answer. |
| RC-L02 | Shipped `INSTALL.md` begins with checkout/build instructions and links to four absent local `../docs/` targets. | Put the release archive first, keep checkout instructions separately and use canonical hosted topic links. | RC2 archive member/link check passes; hosted paths and anchors match the source; checkout test headings and command blocks are retained. |
| RC-L03 | Minimal Arch and Fedora lack `hostname`; the printed expression produces `alice@` and setup correctly refuses it. | The archive no longer needs that expression; runner getting-started uses `uname -n`, matching the daemon's hostname. | Exact new archive and runner-name blocks work without installing `hostname`. |

The direct Python example ended on an uncertain consume when the daemon restarted. This matches the client's no-replay contract; no reconnect or retry loop was added. The process guide now explains supervision and points to that existing rule.

## RC2 systems and results

Checked on 2026-11-03 UTC (the evening of November 2 on the test host).

| System | Observed release | Ordinary install and downstream result |
|---|---|---|
| Ubuntu | 24.04.5 LTS | 32 command results, all successful; 11 separate content assertions passed. |
| Arch Linux | Rolling, VERSION_ID `20261101.0.620205` | 61 recorded steps; all functional assertions and final services passed. |
| Debian | 13 (trixie) | 61 recorded steps; all functional assertions and final services passed. |
| AlmaLinux | 10.2 (Lavender Lion) | 61 recorded steps; all functional assertions and final services passed. |
| Fedora | 44, Container Image, RELEASE_TYPE `stable` | 61 recorded steps; all functional assertions and final services passed. |

- Fresh rootless Podman containers used systemd, `--privileged`, private state and run labels on x86_64. They share the host kernel. Only selected local RC archives were installed; no live host daemon or runner was changed.
- The new archive install block was extracted from the shipped `INSTALL.md` and executed unchanged as ordinary Alice, with sudo for setup. The runner guide's person placeholder was filled as instructed; its `uname -n` command was executed.
- Checks covered samples add/repeat/remove; personal runner setup/reload; exact hello answer; queue message identity/body; KV and uncontended locks; bigfile byte equality; subscription add/list and exact copy sender/topic/tag/body; matched/sent/dropped counts; and Configuration, KV, alert, bigfile or subscription persistence after restart.
- Arch, Debian, AlmaLinux and Fedora each recorded 59 successful statuses and two fixture/timing observations: initial systemd `degraded`, and an immediate command before the restarted socket was ready. Bounded readiness then succeeded; persistence assertions passed and daemon, web and runner were all active. These observations were not erased or reported as data loss.
- An early Arch diagnostic script used the wrong consume syntax and an unavailable comparison utility. An initial RC2 Arch extractor also had a fixture permission error. Those invalid harness attempts were retained; separate fresh corrected runs supplied the passing evidence. No product change was hidden in those corrections.

### Images

Base image digests resolved during the runs:

| System | Base image digest |
|---|---|
| Ubuntu | `sha256:78d4744ead1f5b19b04d0df558ab182a5300903900755e515b2ca4d10eb2044d` |
| Arch | `sha256:42217d5f09e11d3fe7ad3b309b7fde891d458464dab5d674e629073765610427` |
| Debian | `sha256:e2914ec98e08b24b4cee3cefcb9f90dfb5519f8d215a2ccf0c9a49b10b6e43de` |
| AlmaLinux | `sha256:be4feaca99462d71397c21275f3a8a000e5175aaa94bb9fb52adc4494662a442` |
| Fedora latest | `sha256:aad5fee562d2727d20aeed9f2d7241b08a8cfb438e1bcf9475a1de5d58c58697` |

The [Fedora Project](https://fedoraproject.org/) and [official download page](https://fedoraproject.org/workstation/download/) identified 44 as the current stable release at verification time; 45 was beta/RC. A fresh pull of `registry.fedoraproject.org/fedora:latest` was checked against its actual `os-release`, not inferred from a cached tag.

## npm, browser and remaining boundaries

- Local npm packages were assembled from each candidate's verified archives. The normal installed-user cases `ExplicitSetup.test_03_clean_host_installs_starts_and_answers`, `test_04_an_installed_agent_serves_a_request` and `test_05_the_client_package_alone_joins_as_a_second_account_without_a_daemon` passed 3/3 on each: RC1 in 12.800 seconds; RC2 in 12.347 seconds.
- Those cases exercise explicit full-node setup, daemon/web, real request/reply and MCP `ab_ls`, an installed script agent, and a second person's client-only installation calling that agent. Only the local fixture registry was used. No tamper, stress or security cases were selected for these journey runs.
- On Ubuntu RC1, a real browser exercised sign-in, sample lists, an agent alert mark, the exact 3-of-4 preview and expanded 4-of-4 log, settings save, runner inventory/enable and re-login after daemon restart. The RC2 changes do not touch web code. Arch, Debian, AlmaLinux and Fedora web checks were HTTP only, not browser interaction.
- Public npm/GitHub download and publication, a genuine update from a separately released older archive, bare-metal installation, other CPUs and native macOS were not established by these checks. Re-running the same installer was checked on Ubuntu. macOS is the next separate acceptance task.
- All test containers were removed with `rm -fv`; own-label listings and private volume listings were empty before each owner's lock release. No burners or live provider calls were used.

The current concise system list lives in [setup](rc-linux-installation.md). W1C led Ubuntu, npm and browser checks; W2C independently ran the other four systems and reviewed the installation fix.
