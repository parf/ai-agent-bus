# Changelog 0.9

📌 **TL;DR:** Changes on the 0.9 development line, newest first, one or two
lines per version. 0.9 is an odd line: it is development and may be broken.
The previous line is [changelog 0.8](Plans/CHANGELOG.0.8.md#changelog-08).

## 0.9.177 — 2026-10-30

group walks for ACL, directory and delivery are iterative, so a deep or cyclic group chain no longer exhausts the stack

## 0.9.176 — 2026-10-30

release-verify.sh checks release evidence on signed archives; x86_64 verified, aarch64 not built here

## 0.9.175 — 2026-10-30

a container image on a digest-pinned AlmaLinux 10 base runs the daemon or a runner from the release archive, with its own acceptance as smoke subsystem Q

## 0.9.174 — 2026-10-30

a dedicated runner script reads its own bus secret on each message, which never sets its credential and never reaches logs or argv

## 0.9.173 — 2026-10-30

TypeScript twin of the unified subscriptions engine passes the shared vectors (B.29). Also: replace "TS twin ✅ (T-1)" in the B.29 row of Plans/R1.0-Release/TODO.md with "TS twin ✅ <the new version>".

## 0.9.172 — 2026-10-30

the debug agent env answer masks every bus-prefixed secret (AGENT_BUS_CONFIG, control token, record secret keys), showing only an allowlist

## 0.9.171 — 2026-10-30

Shared Python subscription engine and language-neutral vectors: filters, authority re-checked at delivery, refused-send and revoked-author handling, damaged rows as invalid_state.

## 0.9.170 — 2026-10-30

An explicit bus name resumes its own conversation, else an unbound one, else a fresh one, so two names sharing a directory never refuse each other.

## 0.9.169 — 2026-10-29

`release-sign.sh` requires the CI hash list (or a loud `--no-ci-check`), a strict version and regular archive files.

## 0.9.168 — 2026-10-29

`install.sh` verifies a full node again as root in a root-owned copy, refuses a quiet downgrade, pins the key with `--key-fingerprint` and never replaces a real file in `~/.local/bin`; `release-sign.sh` signs exactly one release's four archives matching the CI hashes; the workflow builds only tags on main with pinned actions.

## 0.9.167 — 2026-10-29

`install.sh` installs a signed release, the full node or `--client` under `~/.local`, verifying the `SHA256SUMS` signature and the archive before unpacking; `release-sign.sh` and a draft-release workflow produce one. The Claude plugin lock judges a pid only in its own namespace.

## 0.9.166 — 2026-10-29

The Claude plugin lock names its holder; only a lock whose holder has exited is taken over, by one waiter, and a holder removes only its own lock.

## 0.9.165 — 2026-10-29

Launchers starting together after an upgrade replace the Claude plugin one at a time under a lock; an unreadable marketplace record is left alone.

## 0.9.164 — 2026-10-29

A release launcher reinstalls the Claude channel plugin when another release installed it; the release build refuses a half-rewritten plugin manifest.

## 0.9.163 — 2026-10-29

A release carries the web face, the MCP face and the launchers as one compiled `agent-bus-ts` and needs no bun or Node; `package.sh` also builds the client edition archive.

## 0.9.162 — 2026-10-29

Git autopull can run admin-installed success/failure scripts after its result publication attempt, with trusted allowlist selection, bounded processes and separate correlated script-failure records. Stderr alone fails a script; optional-log delivery failures never suppress the hooks.

## 0.9.161 — 2026-10-29

The web has a Bus setup page, the `#bus` Configuration on the edit page, linked in the left navigation for the daemon Owner alone.

## 0.9.160 — 2026-10-29

`#git` follows configured pubsub events with safe fast-forward or explicit mirror updates, revision deploy, per-checkout result records and startup catch-up; reinstall retires the old timers. Post-attempt script integration remains separate.

## 0.9.159 — 2026-10-29

The bus setup refuses a repeated key, audits a realm saved from the host name at start, and shows the daemon Owner its Configuration digest.

## 0.9.158 — 2026-10-29

The `#bus` Configuration is the bus setup: the daemon Owner alone saves the default realm (the host name at first start) and the agent-event keys; a save is validated, audited, applied at once and kept in backups, and `status` shows the realm.

## 0.9.157 — 2026-10-29

A `#webhook` route can publish a GitLab or GitHub push, or a pipeline's deploy request, to a pubsub channel as a strict new-code event; a channel it may not publish to keeps the listener closed.

## 0.9.156 — 2026-10-29

The Secret and Configuration key tables on the agent pages break a key name only after an underscore, never inside a word.

## 0.9.155 — 2026-10-29

A runner taking over an existing agent clears the `script` an earlier `agent-bus run` left on its record, so listings stop saying `runs exec …`.

## 0.9.154 — 2026-10-29

IMAP list/search show live read status and list can filter unread mail; read marks Seen by default with a no-mark option, and mark sets or clears only Seen. Background subscriptions and publication remain PEEK/read-only.

## 0.9.153 — 2026-10-29

`#fetch` adds bounded curl requests with raw status/headers/body and per-instance site/URL allowlists. Exact entries can grant private access; wildcard/default scope remains public-only, with per-hop checks, cross-origin credential stripping and explicit uncertain outcomes.

## 0.9.152 — 2026-10-29

The runner's overlapping-work-directory refusal says how to keep both: disable the shorter key and give its record a sibling key with its name.

## 0.9.151 — 2026-10-29

`#tmux-sessions` keeps each instance's `XDG_STATE_HOME` across the move to the user-account runner: new `state_home` Configuration, defaulting to the old standalone unit's `/var/lib` directory (RT31).

## 0.9.150 — 2026-10-28

Fifth published 0.9 changeset: 0.9.137–0.9.150, squashed.

| Area | Changes |
|---|---|
| Daemon | J16: a `call` keeps its answer from a face reading the same inbox. RT33: `GET /may-send?who=&name=` answers whether another name may send to a target the caller can see; Python `Bus.may_send`, TypeScript `bus.maySend`. Both need a daemon restart. |
| Scheduling | New `#cron` (schedules and one-shot `later`) and `#at` (`in 30m`, `at 09:30`) on one shared scheduler engine; each send goes under the agent's own name, the target's ACL decides (RD140), the creator's may-send is checked at add and before every send (RD141), an unconfirmed send pauses (RD138). |
| Mail | `#imap` subscriptions: new mail polled every 60 s and forwarded to queues or agents, with author may-send checks, a durable bounded outbox and visible pauses instead of resends; compact Date/From/Subject lists and newest-first manual backfill. |
| Web | New `#webhook` (authenticated inbound routes, single-attempt outbound POSTs) and `#fetch` (public-only, pinned HTTP(S) reads with an external-content warning, RD142). Both ship disabled. |
| Subscriptions | Slack, Telegram and logwatch `sub` also check the author's may-send at add, on a moved target and before every forward. |
| Runner | RT31: a person's user-account runner, `#runner/<person>-user`, run as them under their user manager for agents such as `#tmux-sessions`; tmux sessions survive its reloads, stops and crashes. |
| Release | `#cron`/`#at` scripts packaged and installed executable; the db agent is left out of the release archive until Q209, and a check holds every packaged file against setup's list. |

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
