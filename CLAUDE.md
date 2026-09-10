# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

Design documentation for **agent-bus** — a single daemon (`agent-busd`) that is
registry, broker, MCP server and dashboard for AI agents, bots and services.

**There is no code.** No build, no tests, no lint, no dependencies — the repo is
`README.md` and `docs/*.md` only. V1 (NATS JetStream) is implemented elsewhere:
code at `/rd/service/agent-bus/` (`README.md`, `HOWTO.md`), normative design at
`/rd/vhosts/realty/Plans/PRF-25/`. Read those, not Linear, when a V1 fact is
needed. This repo designs V2. Work here is editing Markdown, and the only
tooling is git.

## Document map

`docs/00-overview.md` indexes the set and says which document owns what — read
it first rather than duplicating the index here. Beyond the numbered docs:

| File | Role |
|---|---|
| `docs/glossary.md` | **normative for naming.** Every name and term, one line each. Check a name here before inventing one |
| `docs/decisions.md` | index of settled / open / superseded decisions — rows link, they never state the rule |
| `docs/future/` | designed but deferred; not part of the current scope |
| `legacy/` | history only, not spec — never cite it, never update it |

On **decisions**, the doc that `docs/decisions.md` links to wins. When two docs
disagree, fix the stale one.

## Working rules

- **No data models, no schemas, no wire formats until the owner asks.** The
  design is deliberately at the level of ideas. Do not invent tables, JSON
  shapes or endpoint lists.
- **Canonical home.** Every fact that carries a *value* — a path, a command, a
  mode name, a field, a number, a list of kinds — is stated in exactly one
  section. `docs/00-overview.md` says which.
- **Claim vs value.** Any other doc may restate the *claim* in one sentence and
  must link to the canonical section in the same breath. The test: *if the
  value changed, would this line need editing?* If yes and this is not its
  home, delete the value and keep the link. This is why a socket path once had
  to be edited in seven places.
- **Cross-references are section links**, never bare doc numbers. From inside
  `docs/`, write `[access § local socket](02-access.md#local-socket)`; from
  `README.md` or this file, prefix `docs/`. Link text is *short doc name §
  section*. Headings used as targets are plain words — no backticks or
  punctuation, so the anchor stays predictable — and are not renamed casually.
- **Every settled decision gets two edits**: the substance into the doc that
  owns it, and one row in `docs/decisions.md` that *names* it and links. If you
  can learn the rule from the row, the row is too long.
- **Revising a decision** is three edits: change the doc, add a new row, move
  the old row to `## Superseded` with what replaced it.
- **Open items** live as a `❓` with *Settled by:* at the point in the topic doc
  where a reader hits the gap, and are indexed in `docs/decisions.md`. Never
  state a count of them anywhere — it drifts.
- **Summaries** (`README.md`, this file, the overview's principles) may restate claims,
  comparisons and consequences; they may not restate values. The one exception
  is the README's CLI sample block.
- Decisions in `docs/decisions.md` are closed — do not reopen without a reason
  from the owner.

## Writing conventions

- Small files, main ideas only, **tables over prose**. Terse; the owner writes
  shorthand and corrects directly — update without ceremony.
- English (the owner reads English and Russian).
- Glyphs follow <https://parf.dev/ai-skills/Glyphs.md>: **no glyph by default**;
  ❓ open question (with what settles it), ❌ failure, ⛔ impossible, 🚫 cancelled,
  ⚠️ partial, ✅ done. One glyph per cell; if most rows would carry one, none do.

## Design invariants worth knowing before editing

Getting these wrong produces drift that is easy to miss. Stated as rules, with
the mechanism behind the link:

- **Modular by layer**: protocol → ports → core, with adapters and faces
  outside. Dependencies point inward; core never imports an adapter; every
  external dependency (database, directory, sandbox, dump, git) sits behind a
  port so it can be replaced by writing one adapter — `docs/10-modules.md`.
- **No external broker.** `agent-busd` *is* the broker; point-to-point plus
  bounded in-memory queues. Never reintroduce one; if a single flow needs
  durability, give that one flow a WAL.
- **Authentication is always on; the AUTH *role* is optional.** A call carries
  exactly two parameters, and the local socket supplies them rather than
  replacing them — `docs/02-access.md`.
- **Names are `user@realm`**, and **the name is the identity**. Provider
  numeric ids are stored only as a re-check comparison; never reintroduce
  `github:<id>`-style ids as principal ids — `docs/01-identity.md`.
- **Registration is a record you state.** A provider is an alternative to
  typing it and is not needed after enrolment. MVP is manual + GitHub.
- **ACL is service first, then master**, and a service may refuse master
  access — `docs/01-identity.md`.
- **Required minimum** is registry (services *and* topics) + queues + API +
  MCP + dashboard, AUTH off.
- **Supervisor plus least-privilege children**, systemd-style: the supervisor
  holds only `CAP_CHOWN` and no state; bus, runner, web, auth, billing and
  health are separate processes, each with a declared privilege set; nothing
  shared implicitly — unix sockets and passed fds only. Only auth holds
  `master_secret`; only the runner may exec — `docs/11-processes.md`.
- **Two kinds of data.** AUTH data = offline-signed generations in git.
  Registry data = live records, writer-signed where a key exists, snapshotted
  to git. Live state (health, stats, queue contents) is neither.
- **Chaining queries upstream; it never replicates it.** Peers sync registry
  via git, newer record wins per entry.
- **Ed25519 everywhere a key exists**; no passwords, no client secrets, no
  TLS/PKI.
- **Do not reinvent the wheel.** Call the system's tools — `ssh-keygen`,
  `openssl`, `ldapsearch`, `curl`, `git`, `age`, `systemd-run`, `sshd` —
  rather than linking a library or writing our own. Each sits behind a port,
  in an adapter. The only exception is the per-message hot path, which cannot
  spawn a process: there use a well-known library, never our own crypto or
  protocol implementation — `docs/10-modules.md`.
- **Bodies are end-to-end encrypted**; the bus and its dashboard see envelopes
  only. Don't write anything implying the bus reads payloads.
- **Queues and stats are memory**, dumped to Parquet on graceful restart. A
  consumer being down is fine — its queue waits.

## Git

Commit subjects are a single terse imperative-ish sentence describing the
decision or edit, no prefixes or tags (`Topics are first-class records registered
like services`, `Static-token principals do not sign; signatures only where a key
exists`). One decision per commit.

## Other agent configs

An OpenAI Codex config exists at `~/.codex/config.toml`. To import user-level
items (MCP servers, slash commands, subagents, skills, instructions), reply
`/import` to see what is importable, then `/import --yes=<digest>` to apply it.
