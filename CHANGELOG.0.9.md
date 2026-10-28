# Changelog 0.9

📌 **TL;DR:** Changes on the 0.9 development line, newest first, one or two
lines per version. 0.9 is an odd line: it is development and may be broken.
The previous line is [changelog 0.8](Plans/CHANGELOG.0.8.md#changelog-08).

## 0.9.126

Add separately approved local Qdrant setup with a pinned hardened Quadlet, private mounted API key, bounded verified backups and fixture-only restore acceptance.

## 0.9.125

One live session per bus name (RD129, J14): a face refused its name says so in every tool, `--take-over` evicts a verified holder, a fork gets a `.job` name; a daemon restart is a quiet reconnect (J15); `numberedName` keeps a realm-less name whole.

## 0.9.124

Add AI decide for OpenRouter typed decisions, preserving request bytes and every returned probability under the same key and ACL as ask; bound model log fields.

## 0.9.123

Add the Voyage embedding and byte-preserving API proxy, keeping its provider key in the runner host environment. Share the bounded HTTP boundary with AI without changing its wire behavior.

## 0.9.122

Align activity strips after uPlot finishes layout, including resize, instead of measuring before its plot area exists.

## 0.9.121

Keep Configuration SHA in its header, color JSON safely, align activity strips with their plots, and simplify KV summaries and zero cells.
Runner status asks give a health line; status methods retain JSON. Remove synthetic no-tags output and migrate legacy stored sentinels at startup.

## 0.9.120

Hide empty Allow and Maintainers rows in Status & Policy; show Updated and Last used inline with their timestamps, omitting absent times.

## 0.9.119

Keep ACL and Maintainers visible in Status & Policy while hiding empty Roles. Declare AI ask and SMS send methods, preserving bare requests through the same handlers.

## 0.9.118

Put record tags once in the header metadata line for agents, groups, services and resources; reuse shared badges linking to the agent tag filter.

## 0.9.117

Show stored Configuration as an expandable, manager-only JSON preview in the record page’s left column; let About metadata flow without splitting values.
Use Serg Parf as the author, compact runner paths into a tooltip row, and remove redundant permission prose.

## 0.9.116

Use one calm teal-green primary for actions and selections, muted tags, and grey secondary store links. Keep the runner Danger zone open after actions and preserve refused confirmation text.

## 0.9.115

Show 100 rows per page in record lists and the user directory, preserving page clamps. Use contrasting indigo and sage badges; reserve red/orange UI accents for alarms and dangerous actions.

## 0.9.114 — 2026-10-27

Second published 0.9 changeset: 0.9.101–0.9.114, squashed.

| Area | Changes |
|---|---|
| Asks | Every agent kind checks the ask first and answers `unsupported: <ask>` for one it does not handle. `status` is an ask: the runner answers starting, stopping and failed itself; `--no-asks` and `"asks": "none"` let any script run unchanged. |
| CLI | `agent-bus token` replaces the `agent-bus-token` program, which stays only as the SSH forced-command wrapper. Help is man style: grouped commands with main ones first, complete per-command pages with values, defaults, examples and global options, full documentation URLs. New spellings `config`, `ls --table` (`-h`) and `register --service-addr`; old ones still accepted. |
| Backups | `agent-bus-admin export` and `import`: the daemon Owner's full or partial backup of permanent records, Configuration, Secret, KV and credentials, `age`-encrypted to the Owner's key, with an open `.info` beside each archive. Import resolves every conflict explicitly before one transaction. |
| tmux sessions | `#tmux-sessions` keeps one tmux session of named command windows: start, stop, restart and list, with status from the real windows. Shared Python env-file parser, a `config.json.dist` example. |
| Web | Tag badges, a "you" owner badge, agent status on its page, collapsed Methods, no `no-tags` sentinel. |
| Project | Homepage https://parf.dev/agent-bus/, with code and docs on GitHub. #bus help carries license, author and homepage. |
| Fixes | 0.9.101 archives are rejected by setup. Packaged web was broken from at least 0.9.100 until 0.9.103. |

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
