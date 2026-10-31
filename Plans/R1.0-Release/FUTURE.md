# R1 future

📌 **TL;DR:** Follow-up the owner has decided is not part of R1. Each entry names the decision that deferred it and what would start it.

## Deb and rpm packages

- **What:** `.deb` and `.rpm` packages and the repositories that serve them, so a host installs with its own package manager.
- **Deferred:** not in R1 ([RD171](DECISIONS.md#recorded-decisions), owner, 2026-10-30). The install path is [npm](distribution.md#simple-install).
- **To start it:** the owner asks for it, and names where the package repositories are hosted; nothing is built for it before that.

## Logwatch subscription start boundary

- Deferred from B.29 by lead1 (2026-10-30): enqueue a line, add a subscription, then drain the forward queue; the pre-add line is forwarded. Pin the subscription-start boundary before changing this existing queue behavior.

## Concurrent chat subscription migration

- B.29 runs with one process per Slack/Telegram instance. Two processes migrating the same legacy key can race the journal's add: the second gets 412 and stops serving until reload. If multiple processes are enabled, re-read and finish the winning journal before serving; test simultaneous startup. The existing journal keeps old/new rows from both being served after recovery.

## macOS uninstall command

Deferred by the owner on 2026-10-30; use the [manual macOS uninstall steps](../../docs/07-setup.md#uninstall-the-macos-test-daemon).

## Shell installer

- **Deferred P.4:** the owner chose simple npm install commands on 2026-10-30 ([RD178](DECISIONS.md#recorded-decisions)). `curl … | sh`, `install.sh` distribution and its installer-specific acceptance are outside R1.
- **To start it:** the owner asks for a shell installer. Reuse HTTPS downloads, `SHA256SUMS`, manifests and untouched-host refusal checks; release signing, keys and attestations are not requirements.
- **History:** [earlier installer acceptance](done/installer-acceptance.md#checks). Existing scripts and throwaway-key fixtures remain unchanged until the P.1 migration; they do not require a real owner key or a public signed release.

## Setup waits out the daemon's start limit

- Setup restarts `agent-busd` once per run that changes it, and systemd refuses a start after `StartLimitBurst` starts inside `StartLimitIntervalUSec`: a script that runs setup many times within the window fails on the later runs, loudly and with the wait named ([J59](../issues/2026-10/2026-10-30.md)). **To start it:** setup reads the unit's two limits, counts its recent starts and waits for the window before a restart it must make, as the fresh-install fixture does. Not raising `StartLimit*` and not clearing the failed state are settled.

## M.8 deployment follow-ups

- Monorepo subdirectory packages: deferred by #ab-dir on 2026-10-30; M.8 requires a self-contained package with root `agent.json`.
- A runner-owned deployment root: deferred by #ab-dir on 2026-10-30; it changes the [process boundary](../../docs/08-processes.md#the-rule) and requires the owner's decision. M.8 uses a host-provisioned writable `--agents` directory.

## Asks-only read in the Python client

- **What:** `asks: "only"` on the Python client's consume, as the TypeScript client has it ([RT25](TODO.md#bundled-agents), `#ab-dir`, 2026-10-30).
- **To start it:** a Python agent with a slow start that should answer `status: starting: <what>`; nothing is built before that.

## IMAP folders follow-up

- A hard expunge (permanent delete) for [B.30](TODO.md#bundled-agents), only if the owner asks; off by default, never a bare EXPUNGE.
