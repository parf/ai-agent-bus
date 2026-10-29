# Changelog 0.9

📌 **TL;DR:** Changes on the 0.9 development line, newest first, one or two
lines per version. 0.9 is an odd line: it is development and may be broken.
The previous line is [changelog 0.8](Plans/CHANGELOG.0.8.md#changelog-08).

## 0.9.148 — 2026-10-28

Add `#fetch`: public-only, pinned HTTP(S) reads as bounded text or Markdown, with verified TLS, redirect checks and an unconditional external-content warning. Deploy disabled; no public URL is called by the checks.

## 0.9.147 — 2026-10-28

The release archive no longer packages the db agent, whose host-copied native libraries setup refused (Q209); a check holds every packaged file against setup's list.

## 0.9.146 — 2026-10-28

A release install writes `#cron`'s and `#at`'s scripts executable (0.9.142 wrote them 0644); every bundled runner command is now checked.

## 0.9.145 — 2026-10-28

RT33 for the subscription agents: Slack, Telegram and logwatch `sub` also require that the subscription's author may send to the target (`/may-send?who=`), checked at add, at an edit that moves the target (Slack, Telegram) and before every forward; a refusal or a vanished target disables or pauses the entry visibly, and a check that cannot be answered sends nothing.

## 0.9.144 — 2026-10-28

`#cron` and `#at` require that an entry's creator may send to its target (RD141), at add and before every send, through `/may-send?who=`: a refusal stores nothing or pauses the entry, an unavailable check sends nothing, with a per-pass circuit breaker.

## 0.9.143 — 2026-10-28

Add `#webhook`: bounded authenticated inbound routes to queues or agents, and single-attempt outbound POSTs to configured URLs. Ship it disabled, with no public listener.

## 0.9.142 — 2026-10-28

Release archives ship `#cron` and `#at` with the shared scheduler engine: `agents/package.sh`, setup's bundle list and the outside-checkout runtime check now include them.

## 0.9.141 — 2026-10-28

New `#at`: one deferred call, `in 30m` or `at 09:30` (the next real instant in the agent's zone, echoed with its offset), on cron's engine moved to `src/agents/shared/scheduler.py`; a vanished target's pause keeps its reason across a restart.

## 0.9.140 — 2026-10-28

RT33: `GET /may-send?who=<name>&name=<target>` answers by Send's rule for `who`, only for a target the caller can see, echoing the canonical `who` so a client rejects an older daemon's caller-only answer; no refusal counter or activity. Python `Bus.may_send(who, name)` and TypeScript `bus.maySend(who, name)` return a boolean and raise on anything else. Needs a daemon restart.

## 0.9.139 — 2026-10-28

New `#cron`: `sub add/list/delete` schedules and one-shot `later` sends to a queue or agent; it sends under its own name and the target's ACL decides (RD140), each firing tagged with its creator; one send per firing, an unconfirmed send or an ACL refusal pauses the entry (RD138), other refusals retry with backoff.

## 0.9.138 — 2026-10-28

J16: a `call` made under a name whose inbox a session's face reads no longer loses its answer to the face. The CLI names its own inbox as `reply_to`, and the daemon holds that reply (and its receipts) for the caller's filtered read until the deadline, then offers it to the blocked face; needs a daemon restart to take effect.

## 0.9.136 — 2026-10-28

Fourth published 0.9 changeset: 0.9.136, squashed.

| Area | Changes |
|---|---|
| Mail | New `#imap`: verified IMAPS/app-password or Gmail XOAUTH2, read-only EXAMINE and BODY.PEEK, local list/search/read. |
| Cache | Private bounded UIDVALIDITY/UID cache, atomic writes, LRU eviction and repair; bounded sync progress, reported skips and capped persisted refusal retries. |
| Publication | Explicit one-UID copy to a fixed destination: external-content warning plus raw text, at most 256 KiB; no HTML rendering, resource fetching or automatic forwarding. |
| Packaging and docs | Self-serving runtime/client/license inventories, IMAP guide and docs index, loader/fixture checks; installed instances stay disabled until the operator enables them. |
| Agent docs | Every agent page opens in plain words — the problem, the idea, when to use it and use cases — with technical detail in closed blocks; the index shows how incoming events reach a queue or an agent, with a mail help-desk example. |

## 0.9.135 — 2026-10-28

Third published 0.9 changeset: 0.9.115–0.9.135, squashed.

| Area | Changes |
|---|---|
| Bundled agents | New: `#voyage` embeddings and API proxy, `#qdrant` proxy with read/write/admin roles (default write), `#db` proxy for MySQL, PostgreSQL and ClickHouse (default read), `#smtp` with verified TLS, app passwords and Gmail XOAUTH2. `#ai` gains `decide` with per-choice probabilities. Qdrant runs as a plain container the operator places (RD131). |
| Apps | `#rag` 0.1.0, versioned on its own: JSON CRUD and prompts built from bus agents. |
| Docs | Every bundled agent has its own page in one structure, linked from its help, help-json and web page; `docs/agents/README.md` opens with a one-line-per-agent list. |
| Launchers | One live session per bus name (RD129): a refused face says so in every tool, `--take-over` evicts a verified holder, a fork gets a `.job` name. A daemon restart is a quiet reconnect. |
| Runner | Status asks give a one-line health summary; the synthetic `no-tags` sentinel is gone and stored ones migrate at startup. |
| Web | Configuration as colorized JSON in its own section, inline Updated and Last used, empty policy rows hidden, aligned activity strips, KV summaries without zero counts, calm teal primary and 100 rows per page. |

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
