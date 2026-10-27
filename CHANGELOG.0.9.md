# Changelog 0.9

📌 **TL;DR:** Changes on the 0.9 development line, newest first, one or two
lines per version. 0.9 is an odd line: it is development and may be broken.
The previous line is [changelog 0.8](Plans/CHANGELOG.0.8.md#changelog-08).

## 0.9.109

Start tmux sessions with the person’s user-manager PATH and a terminal type. Report started only after observing a running pane and bus reader; immediate exits and startup deadlines return failure with the session output tail.

## 0.9.108

Export permanent records and credentials to age-encrypted backups with public info sidecars. Import validates explicit conflict and realm choices before committing records, private KV and token bindings atomically; setup and cleanup manage a separate bounded backup directory.

## 0.9.107

Rename the standalone session starter to tmux-sessions, with optional exact-case tmux names and shared strict tag validation. Set-wide tags reach launchers alongside their automatic runtime tags.

## 0.9.106

Show ls [-h] [--all] [<name>] in the help overview, keeping its shared column. List global options alongside command options on every CLI, admin and runner page.

## 0.9.105

Complete command help with visible short aliases, descriptions, positional arguments, enum values, defaults and examples. Wrap DESCRIPTION prose at 80 columns while keeping command and option rows on one line.

## 0.9.104

Group CLI help in man style, with complete per-command options and full documentation URLs. Use config, ls --table and register --service-addr; keep legacy spellings accepted but hidden.

## 0.9.103

Ship the shared tag parser required by the web face. The packaged web has been broken since at least 0.9.100 because this runtime dependency was absent.

## 0.9.102

Retrieve credentials through agent-bus token; keep a fixed SSH wrapper at the old path, validate SSH token arguments before every branch, and migrate callers and packaging without changing entitlements. Fix the RT25 provider-helper manifest omission: 0.9.101 archives are rejected by setup.

## 0.9.101

Dispatch asks before ordinary work in every agent kind; add status routing, no-asks script policies and runner-observed starting/stopping/failed answers where its reader is attached.

## 0.9.100 — 2026-10-26

First published 0.9 changeset: 0.9.0–0.9.100, squashed. The 0.8 line closed at 0.8.243.

| Area | Changes |
|---|---|
| Bus | Reserved in-daemon `#bus` answers help, status, version and ls. `agent-bus may-send` checks target permission. `call --tagged` and `ls --tag` reach agents by tag. Tags allow only `a-z 0-9 - _`. Group publication copies need the destination Owner. |
| Security | HTTP bodies read after authentication with per-handler limits. Tokens and private values stay out of runner stderr, exit reasons and diagnostics. Linked settings files are refused. Mapped-user, debug-probe and reload authority tightened. |
| Runner | Acknowledgments share the message deadline. Configuration is capped at 64 KiB and passed as `AGENT_BUS_CONFIG`. Required env keys are marked `# required`. Killed cgroups are recreated. Reload swaps readers without leaving old ones. |
| Bundled agents | New: git, logwatch, debug doctor, AI proxy (`#ai`) and Telnyx SMS. Systemd manages all unit types with Configuration caps. Telegram and Slack share one router, with per-chat routes and `handler: "drop"`. Every agent publishes machine-readable help from its package metadata. |
| Launchers | Claude loads the bus as a channel plugin. Added `--tag`, the `ai-cli` tag, automatic `<runtime> <cwd>` descriptions, crash restart with backoff, and a tmux session starter. |
| MCP face | Help asks are answered without the model. Rename and push recovery. Fewer receipts (RD103). |
| Web | Machine-readable help pages. Compact record and settings pages with one Messages display and waiting/lost chips. A Last used column. Token-only login. Self-hosted fonts, icons and charts with no CDN. |
| CLI | `agent-bus help <agent>` prints readable help. Long options use `--name=value`. `agent-bus-token` works without a name. `run` defaults to args. |
| Operations | The daemon logs to syslog only with `--syslog`. Weekly cleanup bounds caches, releases, backups and logs. Releases ship every bundled agent with license notices. Deploy checks run each agent's selftest. |
| Tests | `src/smoke.sh` runs fast, normal and slow sets per subsystem; the normal gate is under one minute. Mutants run only for new checks and ⚠️ subsystems. |
