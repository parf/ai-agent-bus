# Subsystems

📌 **TL;DR:** The map [/find-issues](../.claude/skills/find-issues/SKILL.md) reviews by: each path prefix
belongs to one subsystem, the first matching row wins, and the Check column is what that subsystem's
reviewer looks hardest at. ⚠️ marks a scary one, which gets two reviewers. A letter is never changed or
reused; a subsystem whose paths are gone loses its row. Built from
[src/MODULES.md § modules](../src/MODULES.md#modules) on 2026-10-17.

| Letter | Path prefix | Subsystem | Check |
|---|---|---|---|
| A | `src/internal/core/` | ⚠️ Registry core | ACL and authority, ownership, enrolment, record kinds and the restore gate (`validateKind`), queues, delivery and counters; every walk over groups |
| B | `src/internal/auth/`, `src/internal/authkeys/`, `src/internal/keyproof/`, `src/internal/signature/`, `src/internal/directory/`, `src/internal/tlsdir/` | ⚠️ Authentication and keys | token minting, lifetime and rotation, sessions, `authorized_keys` parsing and locking, proof of possession, TLS pinning; secrets never logged or echoed |
| C | `src/internal/store/`, `src/internal/ports/`, `src/internal/journal/`, `src/internal/activity/`, `src/internal/callstats/` | ⚠️ Storage and logs | SQLite schema and migrations, transactions, what a failed write leaves, load and restore, log contents |
| D | `src/internal/envfile/`, `src/internal/protocol/` | ⚠️ Parsers and wire | names, records and envelopes as JSON, the env-file syntax; input bounds, refusals that name without echoing |
| E | `src/internal/api/`, `src/internal/duallisten/` | HTTP API | routes, credential extraction, error contract, client dial, TLS sniffing |
| F | `src/cmd/agent-bus-runner/`, `src/internal/sandbox/`, `src/internal/cgroup/`, `src/internal/script/` | ⚠️ Runner and process boundary | exec, environment delivery, sandbox and cgroup limits, privilege, kept processes, reload and stop |
| G | `src/cmd/agent-bus-setup/`, `src/cmd/agent-bus-admin/`, `src/build.sh`, `src/git-install.sh`, `src/package.sh`, `src/release.sh` | ⚠️ Setup, install and release | runs as root: file modes and owners, systemd units, idempotence, build stamping |
| H | `src/cmd/`, `src/internal/` | Daemon, CLI and small modules | `agent-busd`, `agent-bus`, token tool, locks, display, version and process titles; CLI help against behaviour |
| I | `src/web/` | Web face | forms and redirects, `returnTo` allowlist, CSRF and caching, what a page claims against the daemon |
| J | `src/mcp/`, `src/launchers/` | MCP face and launchers | tool contracts, session files, receipts and routing, launcher arguments |
| K | `src/clients/`, `examples/` | Client libraries | the six languages against the protocol and each other, examples run, MIT licensing |
| L | `src/agents/` | Bundled agents | Slack and debug agents: secrets, parsing, reconnects |
| M | `src/acceptance/`, `src/smoke.sh`, `src/smoke-process.sh`, `src/` | Tests and harness | checks that cannot fail, the mutation-first shapes, shard isolation, scratch in tracked paths |
| N | `docs/`, `README.md`, `CHANGELOG`, `LICENSE.md`, `NOTICE.md` | Current docs | built against pending, canonical homes, links and anchors, docs against code |
| O | `Plans/` | Plans | lifecycle, question and decision migration, IDs, item markup, stale status |
| P | `.claude/`, `CLAUDE.md`, `AGENTS.md`, `.gitignore` | Agent configuration | skills and repo rules against how the repo actually works |
