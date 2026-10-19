# Changelog 0.9

📌 **TL;DR:** Changes on the 0.9 development line, newest first, one or two
lines per version. 0.9 is an odd line: it is development and may be broken.
The previous line is [changelog 0.8](Plans/CHANGELOG.0.8.md#changelog-08).

## 0.9.14 — 2026-10-19

Limit compact Configuration to 64 KiB at write and load so script environments remain executable.

## 0.9.13 — 2026-10-19

Match Slack bot author filters by bot_id for both modern and classic message events.

## 0.9.12 — 2026-10-19

Log corrupt Telegram seen-cache values by key and start those keys empty, preserving transient KV retries.

## 0.9.11 — 2026-10-19

Restart kept agents on reload when their installed package files change; unchanged packages keep running.

## 0.9.10 — 2026-10-19

Filter Slack subscriptions by resolved user and bot authors with from_users; edit or clear the list while retaining channel and regex matching.

## 0.9.9 — 2026-10-19

Install daemon shutdown handlers before serving requests or announcing readiness, so an immediate SIGTERM still drains and saves cleanly.

## 0.9.8 — 2026-10-19

Add the standalone git agent with fixed repositories, delivered-role controls, configured SSH keys and bounded transport lifetimes.

## 0.9.7 — 2026-10-19

Verify runtime listener ownership before sending launcher credentials, reject malformed session listings, and disable standalone Python bytecode writes.

## 0.9.6 — 2026-10-19

Show Configuration to Owner and Maintainers, preserve edits, and pass compact AGENT_BUS_CONFIG at each script start; keep credentials in Secret.

## 0.9.5 — 2026-10-19

Say “no failed units” when systemd triage finds none; JSON still returns an empty unit list.

## 0.9.4 — 2026-10-19

Call visible agents by tag with `call --tagged`, bounded concurrent exchanges and one shared deadline; expose `ls --tag` while preserving single-call correlation flags.

## 0.9.3 — 2026-10-19

Manage all host unit types through systemd's delivered role tiers, with typed listing, timers, failed-unit triage and whole-journal logs. Discover standalone systemd instances by their package tag.

## 0.9.2 — 2026-10-19

Require agent.json descriptions for runner and standalone agents, and publish them on the bus. Expand standalone help and report bare-metal virtualization as a successful “none”.

## 0.9.1 — 2026-10-19

Publish launcher locks with their owner PID already inside. Stale cleanup removes only the observed dead owner, preserving a competing live replacement and fixed session names.

## 0.9.0 — 2026-10-19

0.9 begins: the 0.8 line closed at 0.8.243.
