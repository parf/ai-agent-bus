# Runtime integration delivery

📌 **TL;DR:** H.8 passed on 2026-09-23. From a program dir built by
`src/build.sh`, installed into clean runtime profiles, a bus message reaches
each live interactive session — Codex and OpenCode through the pusher, Claude
through its channel — and the session's own model answers with a correlated
reply. Removing Claude's channel activation or Codex's (or OpenCode's) tool
configuration fails the exchange, and a reply made outside the session does not count.

## Scope

Development host, disposable daemons; the live node was not touched. 0.8.27
program dir, Codex CLI 0.156.1, OpenCode 1.18.30 (upstream build), Claude Code
2.1.281. Each runtime profile is new: a fresh `HOME`, `CODEX_HOME`, XDG
directories and, for Claude, a configuration home holding only a copied
claude.ai login. The [current contract](../../../docs/08-runner-role.md#runtime-integration-delivery)
owns the behaviour. Installation on a host without `/rd` or the checkout
stays in the [fresh installed release gate](../TODO.md#installed-stage-gate).

## Checks

| Runtime | Gate | Evidence (`tmp/a78/ev-h92/`) | What the exchange proves |
|---|---|---|---|
| Codex | [interactive](../../../src/acceptance/runtime-interactive.ts) | `int-codex.log` 23/0 | The peer's message is pushed into the live TUI's thread; the runtime's own MCP `ab_send` returns the random service answer, which the TUI also renders |
| OpenCode | same | `int-opencode.log` 23/0 | Same, through the OpenCode server the TUI is attached to |
| Claude | [live channel](../../../src/acceptance/claude-channel-live.ts) | `claude-live.log` 10/0 | The peer's message reaches the real model as an `agent-bus` channel event with nobody typing; the model answers with `ab_reply`, correlated by tag and carrying the peer's random value |

A headless reply cannot pass: Codex and OpenCode must show the answer in the
native TUI that received the push, and Claude's transcript must hold the
channel event and the `ab_reply` call of the launched session.

## Mutations

| Mutant | Named failure | Log |
|---|---|---|
| Codex tool configuration removed (`mcp_servers.agent-bus`) | `runtime did not load the real MCP tools` | `tmp/a78/mut-h92-logs/mutations-1.log` (`no-tools codex`) |
| OpenCode tool configuration removed | same | same (`no-tools opencode`) |
| Claude channel activation removed | `timed out: correlated reply from the model` | `tmp/a78/mut-rest-logs/mutations-1.log` (`no-channel`) |
| Channel removed, and a reply sent headless under the session's own address | `the message reached the model as an agent-bus channel event` | `mutations-3.log` (`headless-reply`) |
