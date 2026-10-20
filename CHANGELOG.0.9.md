# Changelog 0.9

📌 **TL;DR:** Changes on the 0.9 development line, newest first, one or two
lines per version. 0.9 is an odd line: it is development and may be broken.
The previous line is [changelog 0.8](Plans/CHANGELOG.0.8.md#changelog-08).

## 0.9.34 — 2026-10-19

The web shows agent methods and settings from machine-readable help, with a bounded query, visitor-scoped cache and text fallback; Secret values remain hidden.

## 0.9.33 — 2026-10-19

All bundled agents describe their methods, arguments, Configuration and Secret keys in package metadata; generated text and JSON help replace duplicate prose and stay checked against executable parsers.

## 0.9.32 — 2026-10-19

Packaged scripts answer method, Configuration and environment help from strictly checked metadata, without executing their command or exposing secret values.

## 0.9.31 — 2026-10-19

Smart launchers optionally restart crashed foreground sessions after a bounded wait, retaining their names and sidecars; clean exits still quit.

## 0.9.30 — 2026-10-19

Runner env reads return metadata only; web editors stay empty, debug masks private values, and child stderr/status redact injected secrets.

## 0.9.29 — 2026-10-19

Logwatch serves bounded file, syslog and journal history, marks, searches and filtered alerts as a long-lived standalone agent with its own credential.

## 0.9.28 — 2026-10-19

Systemd and git grant delivered owners and maintainers their top role while preserving instance caps. Agent pages show automatic and assigned roles; standalone manifests publish the roles each agent understands.

## 0.9.27 — 2026-10-19

Setup and upgrade make public TLS certificates readable by ordinary users while keeping private keys restricted. Fingerprint errors distinguish unreadable certificates from absent ones; agent installation docs show local and SSH token/pin commands.

## 0.9.26 — 2026-10-19

Runner acknowledgments share the message deadline, releasing expired work even when delivery stalls. Standalone `agent-bus run` scripts also acknowledge only work still pending after two seconds, without redundant receipts for fast replies.

## 0.9.25 — 2026-10-19

Slack routes addressed messages and session thread replies through the shared Telegram/Slack router; channel subscriptions stay independent. MCP-style answers correlate without `re`, and failed incoming handlers retry before marking messages seen.

## 0.9.24 — 2026-10-19

RD103 reduces receipt noise: AI instructions and push footers request receipts only when needed; the runner acknowledges only scripts still pending after two seconds.

## 0.9.23 — 2026-10-19

Telegram supports per-chat routes, scoped open participation and channel announcements; dictionary lookups exclude inherited properties.

## 0.9.22 — 2026-10-19

Git roots and cumulative read/deploy/commit caps live in Configuration; delivered deploy/commit roles cannot exceed a repository cap, including automatic pulls.

## 0.9.21 — 2026-10-19

Telegram routes listed users through permanent session tags and seven-day reply mappings, returns session answers to Telegram, and exposes bounded refusal diagnostics.

## 0.9.20 — 2026-10-19

Give Telegram long polls an HTTP timeout margin so idle chats return normally instead of recording a poll error.

## 0.9.19 — 2026-10-19

Opt-in git autopull uses install-time systemd timers for ff-only pulls, preserving dirty trees and reporting the last result.

## 0.9.18 — 2026-10-19

Systemd instances may restrict unit inventory, actions and journals through Configuration; malformed unit lists return a named error without running commands.

## 0.9.17 — 2026-10-19

Recreate killed agent cgroups before reuse, avoiding spurious SIGKILL on kernels with the clone3 kill-sequence bug.

## 0.9.16 — 2026-10-19

Cancel per-message runner Configuration reads at the message or agent deadline and release the worker slot.

## 0.9.15 — 2026-10-19

Include caller identity, lookup and registration in the shared deadline for single and tagged calls.

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
