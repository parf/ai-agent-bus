# R1 future

📌 **TL;DR:** Follow-up the owner has decided is not part of R1. Each entry names the decision that deferred it and what would start it.

## Deb and rpm packages

- **What:** `.deb` and `.rpm` packages and the repositories that serve them, so a host installs with its own package manager.
- **Deferred:** not in R1 ([RD171](DECISIONS.md#recorded-decisions), owner, 2026-10-30). The Linux install is the signed archive through [simple install](distribution.md#simple-install), with `npm` as the other channel.
- **To start it:** the owner asks for it, and names where the package repositories are hosted; nothing is built for it before that.

## Logwatch subscription start boundary

- Deferred from B.29 by lead1 (2026-10-30): enqueue a line, add a subscription, then drain the forward queue; the pre-add line is forwarded. Pin the subscription-start boundary before changing this existing queue behavior.

## Concurrent chat subscription migration

- B.29 runs with one process per Slack/Telegram instance. Two processes migrating the same legacy key can race the journal's add: the second gets 412 and stops serving until reload. If multiple processes are enabled, re-read and finish the winning journal before serving; test simultaneous startup. The existing journal keeps old/new rows from both being served after recovery.
