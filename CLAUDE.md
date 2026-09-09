# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

Design documentation for **agent-bus** — a single daemon (`agent-busd`) that is
registry, broker, MCP server and dashboard for AI agents, bots and services.

**There is no code.** No build, no tests, no lint, no dependencies — the repo is
`README.md`, `HANDOFF.md` and `docs/*.md` only. V1 (NATS JetStream) is implemented
elsewhere: code at `/rd/service/agent-bus/` (`README.md`, `HOWTO.md`), normative
design at `/rd/vhosts/realty/Plans/PRF-25/`. Read those, not Linear, when a V1 fact
is needed; `docs/v1-original.md` §1 is the verified summary. This repo designs V2.
Work here is editing Markdown, and the only tooling is git.

Language for the future implementation: **Go** first, bun/NPM later; client libs
Go, PHP, Rust, JS, Python.

## Document map and authority

| File | Role |
|---|---|
| `HANDOFF.md` | the owner's full discussion record — most complete, includes rejected/superseded ideas (§8) and the verified GitHub API findings. Read first. |
| `docs/00-overview.md` | canonical: principles, `agent-busd` roles, chaining, storage, trade-offs, **Decision log (2026-09-09)**, **Open** items |
| `docs/01-identity-and-auth.md` | principals, GitHub/LDAP key directories, groups/ACL/roles, ownership, delegation, sealed private config |
| `docs/02-keys-sessions-replication.md` | access-key modes, encrypted sessions, signed generations in git, SSH admin |
| `docs/03-services-and-discovery.md` | service kinds, messaging, topics, discovery, health, stats |
| `docs/04-runner.md` | runner role: adapters, self-supervision, sandboxing, in-process queue |
| `docs/v1-original.md` | what V1 is, and the V1 → V2 mapping |

Docs 00–04 are meant to be read in order and are the design of record. On
**decisions**, `docs/00-overview.md` (decision log) and the doc it names win;
`HANDOFF.md` predates the 2026-09-09 round in places and carries inline
*(2026-09-09: …)* corrections rather than rewrites. On **rationale and history**
(why something was rejected, what was verified), `HANDOFF.md` is more complete.
When they disagree, check the decision log first, then fix the stale side.

## Working rules

- **No data models, no schemas, no wire formats until the owner asks.** This is
  explicit and repeated in `HANDOFF.md`; the design is deliberately at the level
  of ideas. Do not invent tables, JSON shapes or endpoint lists.
- **Every settled decision gets two edits**: the substance goes into the doc it
  belongs to, and a one-line entry goes into the "Decision log" of
  `docs/00-overview.md` naming that doc. Open items live in `00` under "Open",
  each with *what would settle it*.
- Decisions listed in the `HANDOFF.md` table and the `00` decision log are
  closed — do not reopen without a reason from the owner.
- Superseded ideas (NATS, Redis Streams, JWT keys) are kept on purpose in
  `HANDOFF.md` §8. Do not delete them; add to them when something else is dropped.

## Writing conventions

- Small files, main ideas only, **tables over prose**. Terse; the owner writes
  shorthand and corrects directly — update without ceremony.
- English (the owner reads English and Russian).
- Glyphs follow <https://parf.dev/ai-skills/Glyphs.md>: **no glyph by default**;
  ❓ open question (with what settles it), ❌ failure, ⛔ impossible, 🚫 cancelled,
  ⚠️ partial, ✅ done. One glyph per cell; if most rows would carry one, none do.

## Naming (enforced throughout)

- `agent-busd` — the daemon · `agent-bus` — the CLI
- `ab_` — **MCP tool prefix only** (`ab_list_services`); never in CLI or config
- `agent-bus` is the keyword everywhere else: `/etc/agent-bus/`,
  `~/.config/agent-bus/`, the `agent-bus` system user, `agent-busd.service`
- Instance id = address = `unique-name@host`
- Say "service discovery", not "registration" (registration is one operation on it)
- "personal" / "shared" for scope (shared is the default) — not user-scoped/system,
  bound/unbound, single-/multi-tenant

## Design invariants worth knowing before editing

Getting these wrong produces drift that is easy to miss:

- **No external broker.** `agent-busd` *is* the broker; point-to-point connections
  plus bounded in-memory queues. Never reintroduce one; if a single flow needs
  durability, give that one flow a WAL.
- **Authentication is always on; the AUTH *role* is optional.** In minimal mode
  the token in `AGENT_BUS_USER_TOKEN` **is the whole identity** — no key, no AUTH
  calls. AUTH on = central identities, groups, hourly derived keys.
- **Required minimum** is the core: registry (services *and* topics) + queues +
  API + MCP + dashboard, with AUTH off. WEB and AUTH are child processes of
  `agent-busd`; only the AUTH child holds `master_secret`.
- **Two kinds of data.** AUTH data = offline-signed generations in git (rare
  changes, admin authority). Registry data = live records in `agent-busd`,
  signed by the writing principal *when it has a key* (static-token writes are
  unsigned), snapshotted to git. Live state (health, stats, queue contents) is
  neither signed nor snapshotted.
- **Chaining queries upstream; it never replicates it.** Peer nodes at the same
  level sync registry via git push/pull on start, newer record wins per entry.
- Ed25519 everywhere a key exists; no passwords, no client secrets, no TLS/PKI.
- Queues and stats are memory-only: restart empties them, overflow drops the oldest.

## Git

Commit subjects are a single terse imperative-ish sentence describing the
decision or edit, no prefixes or tags (`Topics are first-class records registered
like services`, `Static-token principals do not sign; signatures only where a key
exists`). One decision per commit.

## Other agent configs

An OpenAI Codex config exists at `~/.codex/config.toml`. To import user-level
items (MCP servers, slash commands, subagents, skills, instructions), reply
`/import` to see what is importable, then `/import --yes=<digest>` to apply it.
