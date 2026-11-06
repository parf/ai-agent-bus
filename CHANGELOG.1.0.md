# Changelog 1.0

📌 **TL;DR:** Changes on the stable 1.0 line, newest first.
The development history is [changelog 0.9](Plans/CHANGELOG.0.9.md#changelog-09).

## 1.0.28 — 2026-11-06

The credential store refuses to load when two names share one token, naming both, and the daemon does not start

## 1.0.27 — 2026-11-05

The daemon's default --ssh-token is the helper beside its own resolved binary, and a missing or non-executable helper refuses the start

## 1.0.26 — 2026-11-05

Launcher warns NO BUS and marks the tab when a Claude session has had no live bus face for 20 s

## 1.0.25 — 2026-11-05

ls --tag lists only agents being read now, like plain ls; --all or --kind lists every matching record

## 1.0.24 — 2026-11-05

Container start tightens dirs it owns to 0700 and refuses foreign unusable or group-writable state dirs untouched, as root or --user

## 1.0.23 — 2026-11-05

runner shutdown waits out an in-flight apply and starts nothing once closing, so no process outlives the runner

## 1.0.22 — 2026-11-05

every pushed bus message names its reply route again, so call to a Codex or OpenCode session gets its answer

## 1.0.21 — 2026-11-05

the chat agents' reply router stays under a size budget, prunes freed tags, and answers a reply to an expired or evicted route by name

## 1.0.20 — 2026-11-05

cancelling a runner package build kills its whole process group, not just the shell

## 1.0.19 — 2026-11-05

only exact agent-bus-token or agent-bus-admin key lines name an owner, so user remove never deletes a foreign key; a bad --ssh-token refuses the daemon start

## 1.0.18 — 2026-11-05

a restore no longer refuses the start over another User's legal ACL edit; restored group terms must still name an existing group

## 1.0.17 — 2026-11-05

runner install and remove staging is claimed atomically and swept only when its lock is free; a package link into a guarded directory is refused, failing closed

## 1.0.16 — 2026-11-05

ls -h and bigfile ls -h print the human table even when piped; --help after any subcommand is local help

## 1.0.15 — 2026-11-05

eventlog and logwatch check a subscription's source at creation only, imap still on every delivery; a Personal Allow may name #bus

## 1.0.14 — 2026-11-05

agent-bus -h is help again; ls prints a table on a terminal (JSON when piped, --json/--table force either); call --body prints only the reply body

## 1.0.13 — 2026-11-05

ai image calls get a 90 s ceiling (75 s generation), other methods keep 60 s; a too-short --wait names the minimum

## 1.0.12 — 2026-11-05

The agent-bus CLI takes its identity from the launcher session file, so a renamed session acts as its new name; a no-bus launcher restart continues the conversation

## 1.0.11 — 2026-11-05

#ai serves itself and generates images into its own bigfile store for its Owner and Maintainers

## 1.0.10 — 2026-11-05

a runner reload that changes an agent process count restarts its pool at the new size

## 1.0.9 — 2026-11-05

the Slack agent stops retrying its identity after a definitive refusal and still retries bus and Slack hiccups

## 1.0.8 — 2026-11-05

Launcher tab titles show ❓ when an agent waits on the operator and 🟥 when its last answer ends with 🟥🟥🟥 (Claude hooks, Codex events; OpenCode ❓ only)

## 1.0.7 — 2026-11-05

the daemon refuses to start when its store lacks a durable event or subscription port, instead of keeping them in memory

## 1.0.6 — 2026-11-05

runner install fetches and builds without the reload lock, so stop, disable and status answer during an install

## 1.0.5 — 2026-11-05

#ai models answers a table of context, prices and inputs with a filter word, --json and the configured default

## 1.0.4 — 2026-11-05

#ai lists image models and their prices, names a refused provider key, and takes embed and image as JSON only so plain prompts stay chat

## 1.0.3 — 2026-11-05

Launcher tab titles lead with a runtime glyph and a short cleaned label; agents read and set it with ab_tab_title

## 1.0.2 — 2026-11-05

#ai adds models, model, balance and key (Owner/Maintainers), embed with a derived batch cap, and per-call tokens and cost on ask

## 1.0.1 — 2026-11-05

Clarify whole-route overrides for all sidecar pushes

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
