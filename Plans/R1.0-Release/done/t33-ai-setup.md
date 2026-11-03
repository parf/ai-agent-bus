# T-33 AI setup workflow

📌 **TL;DR:** The root AI-SETUP.md guides a fresh install or tuning an existing installation, using the main docs and agent help. Documentation checks and read-only discovery of an installed node pass. Fresh-system and update journeys are not yet claimed as accepted.

## Delivered entry point

- [AI-SETUP.md](../../../AI-SETUP.md#how-to-start) is linked from the main README, setup guide and shipped INSTALL.md's hosted link.
- Existing command, release or service state leads to an offer to tune, diagnose or explain; it never triggers reinstall or upgrade automatically.
- The AI must read the main docs and selected agent guides before advising. It consults installed help and preserves unrelated settings and credentials.
- IMAP/SMTP private host credentials and Telegram record Secret remain distinct. Real mail/chat actions require the user's designated account and permission.
- Requested updates use the current installation's documented backup/recovery path; repeated setup returns to inspection and tuning.
- T-34's Web Face chat moved to R1.1 by the owner on 2026-11-03.

## Checks and limits

- The docs-order checker returns no failures for AI-SETUP.md. Changed-file links and git diff whitespace checks pass.
- Read-only existing-node discovery: installed CLI reports 0.9.303 with its build stamp, and status exits 0. No install, upgrade, credential write, reload or provider call ran.
- A clean Podman acceptance was not started: this session's bus identity was refused the smoke-gate lock (record belongs to someone else). No lock was bypassed and no container ran.
- Earlier Linux RC2 journeys prove the referenced installation procedures on five systems, not this new AI workflow's acceptance. Fresh installation, second-run preservation, requested update and designated-provider setup remain explicit acceptance items in [T-33](../TODO.md#ai-assisted-installation-and-agent-setup).
