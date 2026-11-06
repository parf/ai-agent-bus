# R1.1

## Scope

Not started. Discovery and observability, operations, additional storage
backends and the peer registry, plus [runner backups](backups.md#backing-it-up) moved from R1. The release's own extensions — record-defined roles, federation chaining, locks, the key-value store, Resource records,
method metadata, the runner, the client libraries and installable
distribution — stayed in [R1](../R1.0-Release/README.md#scope); the tools
stage is [R1.2](../R1.2/README.md#scope).

| Topic | Canonical knowledge |
|---|---|
| Streaming answers (proposed) | [Streaming](streaming.md#streaming) |
| Web Face AI setup chat (T-34, moved from R1) | [Chat scope](ai-setup.md#web-face-chat) |
| Discovery and observability | [Discovery and observability](discovery.md#where-a-member-says-it-is) |
| Runner hardening (M.14) | moved to R1.2 on 2026-11-06: [Runner hardening](../R1.2/runner-hardening.md#runner-hardening-proposal) |
| Runner extensions moved from R1 | [Edges, named hosts and root runner](runner.md#edge-runners); [web additions](runner-web.md#useful-additions); [macOS sandbox](macos.md#mac3-sandbox) |
| Agent capabilities (RT23) | [Proposal](agent-capabilities.md#proposal); [Q191](QUESTIONS.md#agent-capabilities) |
| Node cleanup (B.15) | [Lasting bounds](node-cleanup.md#lasting-bounds) |
| Runner backups | [Backup contract](backups.md#backing-it-up); [RT13 and RBK1](TODO.md#backups) |
| Operations | [Operations](operations.md#reload) |
| File scanner (FS.1, plan only) | [Filescan plan](filescan.md#steps-and-estimate); [task](TODO.md#filescan) |
| Database sync (DS.1, plan only) | [DBSync plan](dbsync.md#steps-and-estimate); [task](TODO.md#dbsync) |
| Calendar agent (CAL.1, not planned) | [task](TODO.md#calendar) |
| Storage backends | [Storage backends](storage.md#backends) |
| Peer registry | [Registry sync](registry.md#registry-sync) |
| MCP agents: `--mcp` answering a tools list, and the `mcp` tag (proposed) | [MCP agents](mcp-agents.md#mcp-agents) |
| MCP latest revision (waiting for Codex and OpenCode) | [K.37](TODO.md#mcp-latest-revision); the rule is the [constitution's](../../docs/constitution.md#external-protocols) |

Open choices are in [questions](QUESTIONS.md#open-questions); recorded choices
are in [decisions](DECISIONS.md#recorded-decisions). Execution prerequisites
are in [TODO](TODO.md#objective).

New identity, AUTH distribution, scoped credentials, encryption and their module
boundaries wait in [R2.0](../R2.0-Future/README.md#topics) (owner, 2026-11-05).
