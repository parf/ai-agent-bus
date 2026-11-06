# Changelog 1.0

📌 **TL;DR:** Changes on the stable 1.0 line, newest first.
The development history is [changelog 0.9](Plans/CHANGELOG.0.9.md#changelog-09).

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
