# T-33 AI setup workflow

📌 **TL;DR:** The root AI-SETUP.md guides a fresh install or tuning an existing installation, using the main docs and agent help. Documentation checks and read-only discovery of an installed node pass. Linux installation uses the completed RC2 acceptance; it is not a new release blocker.

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
- The MCP identity could not take smoke-gate; the ordinary terminal identity was the resource Owner and obtained/released it through the CLI. A short Ubuntu RC2 workflow check finished before the owner's request to stop repeating Linux checks: 32/32 command statuses plus hello, KV and bigfile content assertions passed; second-run read-only discovery/help preserved inventory, Configuration and KV. Its container was removed. No more Linux installation reruns are required for this Markdown change.
- Existing Linux RC2 evidence covers the installation procedures on five systems. No actual version upgrade or real provider action ran in this short check; those claims are not inferred from it. Follow the documented update procedure when requested, and use designated accounts only with authorization. The next platform acceptance is the RC on macOS.
