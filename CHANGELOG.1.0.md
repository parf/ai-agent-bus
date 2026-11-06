# Changelog 1.0

📌 **TL;DR:** Changes on the stable 1.0 line, newest first.
The development history is [changelog 0.9](Plans/changelogs/CHANGELOG.0.9.md#changelog-09).

## 1.0.51 — 2026-11-06

Second published 1.0 changeset: 1.0.1–1.0.51, squashed.

| Area | Changes |
|---|---|
| Daemon and authority | `#bus` answers any asker, and any other answer needs the asker's Allow; a Personal Allow may name `#bus`. One lane rule from the parsed method serves every `#bus` call, with `sub add` and `sub delete` counted as mutations; text `#bus write` keeps its text verbatim and accepts tab and Unicode separators. `sub list` for one log shows an author their own rows, and the ring total reads its defaults from the bus settings. The daemon refuses to start when its store lacks a durable event or subscription port, when two names share one token (naming both), or when its `--ssh-token` helper is missing or not executable; by default that helper is the one beside its own resolved binary. A restore accepts another User's legal ACL edit, but group terms must still name an existing group. Only exact `agent-bus-token` or `agent-bus-admin` key lines name an owner, so `user remove` never deletes a foreign key; a key-file login ends at the first space or tab, may end in CRLF and matches without regard to ASCII case. Last-use writes no longer block calls, and a credential re-issued during a flush does not take the old one's time. Bigfile sweep, index-read and listing errors are reported instead of ignored. |
| CLI | The CLI takes its identity from the launcher session file, so a renamed session acts as its new name. `-h` is help again and `--help` after any subcommand is local help; `ls` and `bigfile ls` print a table on a terminal or with `-h`, JSON when piped, and `--json`/`--table` force either; `ls --tag` lists only agents being read now, with `--all` or `--kind` for every matching record; `call --body` prints only the reply body. |
| Runner | Controls run on one FIFO worker, so status, stop and disable answer during an install, and a stop or remove during an install's fetch supersedes its publish. Install and remove staging is claimed atomically, a package link into a guarded directory is refused, and cancelling a build kills its whole process group. A reload that changes a process count restarts the pool at the new size; the override merge keeps every number as written; shutdown waits out an in-flight apply and starts nothing more; a taken cgroup is removed only after its service has stopped and drained; the stop notice names the real cause. `needs.dir` binds read-only inside configured roots. The runner checks each bundled agent against the daemon protocol its pack declares. The root runner unit starts the current release's runner, so an upgrade moves it. |
| Launchers and MCP | Every pushed message names its reply route again, so a call to a Codex or OpenCode session gets its answer; whole-route overrides apply to all sidecar pushes. Tab titles lead with a runtime glyph and a short label (agents set it with `ab_tab_title`), show ❓ when an agent waits on the operator and 🟥 on a failed answer, and warn NO BUS after 20 s without a live bus face. A no-bus restart continues the conversation. |
| Agents | `#ai` adds `models` (a table of context, prices and inputs), `model`, `balance`, `key`, `embed` with a derived batch cap, image models and prices, per-call tokens and cost, and image generation into its own bigfile store; images get a 90 s ceiling and a too-short `--wait` names the minimum. New FileScan watches an Owner-granted directory and syncs file changes to subscribers, with renames, an on-demand sync, big files and a RAG form that confirms each upsert. New DBSync pages one MySQL or PostgreSQL table through a read-only `#db` and sends new, changed and soft-deleted rows. RAG grows from 0.3 (namespaces, keyword, semantic and hybrid search, citations) through 0.4.1 (rerank, count, similar, complete, classify) to 0.4.2, which indexes Markdown over 32 KiB in chunks with a completeness check. `#db` masks secrets only inside text. Slack and Telegram retry their startup identity without retrying a definitive refusal; the chat reply router stays under a size budget. Eventlog and logwatch check a subscription's source at creation, IMAP on every delivery. The agent pack has its own version. |
| Setup and release | Deploy restarts the active root runners and installs the logrotate rule atomically; setup from a checkout starts the runner beside it; node-cleanup skips only absent directories. Container start tightens the directories it owns to 0700 and refuses foreign or group-writable state directories. Scripts with pipefail no longer misread a closed `grep` or `head`; a rollback with no earlier release says so. Release packaging runs only in the release gate. |

## 1.0.0 — 2026-11-04

First stable release: named messaging, record-defined access, managed agents,
six client libraries, the web console and installable Linux/macOS distributions.


| Area | What ships |
|---|---|
| Messaging and access | Named calls and durable inboxes, record permissions and roles, queues, groups, tags, locks, KV and bigfiles. |
| Agents and tools | Managed runners, bundled mail/chat/server/data helpers, MCP and named AI CLI sessions. |
| Clients and web | Six MIT client libraries, exact protocol validation and the web management console. |
| Installation | Verified Linux and macOS archives, npm full/client packages, and [AI-guided setup and tuning](AI-SETUP.md#how-to-start). |
| Canonical interfaces | One supported set of commands, fields and package/state formats; retired aliases, converters and prerelease compatibility are removed. |
| Licensing | Bus and runner use PolyForm Strict 1.0.0; client libraries remain MIT. Third-party notices ship with the packages. |
