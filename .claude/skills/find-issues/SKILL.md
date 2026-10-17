---
name: find-issues
description: Reviews what changed, one read-only reviewer per touched subsystem (a map it keeps in Plans/subsystems.md), and writes verified issues to the day's file, Plans/issues/YYYY-MM/YYYY-MM-DD.md, in the Q&A markup — severity first, file:line, a 💡 fix the user can accept or decline on the page. Never edits code. Pair: /fix-issues acts on the user's triage.
argument-hint: "[scope] [focus]"
disable-model-invocation: true
---

# /find-issues [scope] [focus]

Adapted for agent-bus from mdhouse's skill of the same name. Writes issues for the owner to triage on the
page — mdhouse serves this repo at `http://localhost:7777/ai-agent-bus/` — and `/fix-issues` acts on the
triage. States, identities, committing: [qa-states.md](../qa-states.md). The item form:
[plan markup](../../../Plans/MARKUP.md#an-item). Never edits code or docs.

- `scope` — default: **since the last review** — `<sha>..HEAD`, the `<sha>` after `..` in the last `# Issues — … · <from>..<sha>`
  heading of the newest `Plans/issues/*/*.md`. The very first run (no issues file yet) reviews the whole
  project at HEAD — once. Or: `release`, `all`, a `<rev>..` range, a path
- `focus` — the rest: subsystems to review; default every touched one

There is no `mine`: every agent here commits as the same git user, so `--author` selects everything.
Several agents commit at once — a range holds their work too, and that is meant to be reviewed.

## Subsystems — the skill keeps their map

`Plans/subsystems.md` maps paths to subsystems, first match wins, each with what its reviewer checks —
a table `| Letter | Path prefix | Subsystem | Check |`, one row per subsystem. The first run
creates it here; mdhouse's own `Plans/subsystems.md` is a worked example.

- **Missing** → build it before the first review from [src/MODULES.md § modules](../../../src/MODULES.md#modules)
  (the layers and what each module owns), the tree under `src/` (`internal/`, `cmd/`, `web/`, `agents/`,
  `clients/`, `launchers/`, `mcp/`), `docs/` and `Plans/`; a handful of rows, not one per file.
- **Every run**: list the scope's files — `git log --no-merges --name-only --pretty=format: <scope>` —
  and map each. A file that matches no row →
  add a row (a new subsystem, or a wider prefix for an existing one) and say so in the report. A
  subsystem whose paths are all gone → drop its row.
- Every subsystem has its own letter; a letter is never changed or reused.
- ⚠️ marks a **scary** subsystem: authentication and tokens, ACLs and authority, storage, snapshot and
  restore, parsers (the env-file parser, names, wire input), the runner's process boundary and sandbox,
  setup and install run as root.
- Commit `Plans/subsystems.md` with the run's issues file.

## Reviewers

Agents, in parallel, read-only:

- **one per touched subsystem**;
- **two for a ⚠️ one**, the checklist split: correctness, security, architecture · compatibility,
  operations, tests, docs, efficiency;
- a subsystem with under ~50 changed lines goes to a neighbour's agent instead of its own;
- **at most 10** — past that, more agents mostly find the same. Typical: 3–5 between releases, 8–10
  for a release.

Each gets its
subsystem's row (the paths, what to check), its files, `git log -p --no-merges <scope> -- <its paths>`,
**[review-checklist.md](../review-checklist.md)** — every section of it, the meta-signals deciding where to
look hardest — the owning [design boundary](../../../CLAUDE.md#design-boundaries) sections for its paths,
the format below and these rules:

- **Never trust a suspicion — verify before reporting**: the problem must exist and matter. Read the
  line, run the test, grep the symbol, a scratch daemon (as `src/smoke.sh` starts one) or a probe in a
  gitignored `local/` or `tmp/` — never the dev node's live daemon, its database or the web face it
  serves. A claim that something is absent or "already met" names every implementation it checked, not
  one call site. `file:line` as at HEAD; a bug added and fixed inside the range is not an issue.
- **Never raise again** what the last 5 days' files in `Plans/issues/` and `Plans/done/issues/` already
  hold, open or closed.
  A `🚫` / `⏸️` / `🎫` one comes back only when its **premise** changed — not merely its file — and the
  new item starts with what changed and links the old one (`issues/2026-10/2026-10-07.md#B44`).
- **One cause, one item**: issues that share a root cause (one stale doc, one missing helper) are one
  item listing every `file:line`.
- `Plans/done/` and each plan's `done/` are history and raise nothing — unless a live doc describes them
  wrongly. A requirement `docs/` or a plan marks **pending** is not a missing feature; a doc that calls
  something built when it is not, or pending when it is built, is.
- At most 10, worst first. Return Markdown, plus one line: what was checked and found sound.

## Format

**One file per day: `Plans/issues/YYYY-MM/YYYY-MM-DD.md`** (today's date) — create it on the day's first run; a
later run the same day appends to it. Each run opens with one heading, subsystems under it as `##` —
`<from>..<sha>` is what was reviewed: the last run's sha, or on the first run the first commit
(`git rev-list --max-parents=0 HEAD`): `Issues — Oct 17, 2026 · 16c3fb5..b34933b`, then
`Issues — Oct 18, 2026 · b34933b..7bb6875`:

```markdown
# Issues — <Mon D, YYYY> · <from>..<HEAD short sha>

- 🔴 C1 `src/internal/envfile/envfile.go:30` a NUL byte in a secret is saved, then refused at exec — the agent never starts
  - Evidence: `Parse("TOKEN=ab\x00cd")` → no error; `exec.Cmd` with it → "environment variable contains NUL"
  - Impact: any secret pasted with a NUL — its agent fails at every start until the secret is fixed
  > 💡👾 Refuse a NUL in the shared parser, by line number, and add the case to the common table.

## <Subsystem>

- 🟠 B2 `src/internal/core/manage.go:287` the ACL walk recurses where role assignment iterates — RD87 says iterate
  - Evidence: `memberThrough` calls itself; `assigned_roles.go:62-91` uses a stack and a visited map
  - Impact: none today, every group is visited once; it is a settled requirement not met
  > 💡👾 Walk with a stack and the visited map, as role assignment does.
- ❓ 🟠 W1 `src/web/pages/<page>.tsx:<line>` an unparsable `limit` falls back silently — no cap on rows
  - Evidence: `/<page>?limit=abc` → every row
  - Impact: any client that sends a bad limit — one slow page
  - ( ) clamp every `limit` through one helper 🌟
  - ( ) refuse a non-numeric `limit` with 400

Checked and sound: <one line per subsystem — what was checked>.
```

The examples show the form. C1 and B2 are real cases from 2026-10-17 (C1 fixed in 0.8.154, B2 pending as
R1 RGW1); W1 is invented.

- **Every issue has a unique id** `<letter><n>` (`A14`) — the subsystem's letter; ids reset for every file
  (file-per-day). Writing externally — `date#id`: `issues/2026-10/2026-10-07.md#B44`.
- **Serious first**: an issue that loses stored records or secrets, leaks a secret or a credential, or
  lets a caller past an authority boundary goes right under the section heading, above the subsystems.
- **An item is a title line, a sub-list and the fix** — fields are a sub-list, because mdhouse collapses
  plain `Label:` lines into one paragraph ([why](../../../Plans/MARKUP.md#why-the-fields-are-a-sub-list)):
  1. the severity glyph first (🔴 wrong / unsafe now · 🟠 matters, not now · ⚪ low · 🔵 informational — treated as done or not relevant),
     the id, `file:line` (each one, for a shared cause), **the claim and its consequence — one line**;
  2. `- Evidence:` — what was run or read, and what it gave;
  3. `- Impact:` — who hits it, how often, what it costs (qa-states.md); the severity follows it — no
     one hits it → ⚪ or not raised;
  4. the fix: a `> 💡👾` line (✓ accept / ✗ ignore on the page) — or, for two real ways, `- ( )` options
     and the line starts `❓` before the severity. Nothing but the thread follows the sub-list.
- **Each line one short sentence, ~120 characters at most.** `Evidence:` — the strongest proof only,
  not the whole session. Several cases of one issue → more sub-items after the `Impact:` one and
  before the fix (`- the runner: …`, `- the Slack agent: …`), one per line. A case never opens with a glyph
  (`- ❓ icon: …` is a question of its own, open forever) — name it in words: `- the question icon: …`.
- No "as its doc says", no restating the code — the reader opens the line.
- `Checked and sound:` — one line per subsystem, names only (functions, edge cases), no prose.
- No `✅`, no `💬` in a new item — triage is the user's.

## Steps

1. `git status -s` — other agents' work in progress is normal here and is not a reason to stop or to
   touch it. Review the committed range; when an uncommitted file sits in a touched subsystem, add
   `- 🔵 not reviewed — uncommitted: <files>` to the section.
2. Resolve the scope. **Re-check the open issues — untriaged, ❓, ⏳, ⚠️, 🎫 — whose files changed since the
   last review** (`git diff --name-only <sha>..HEAD`): gone, and a commit names it → `✅` +
   `` 💬👾 `<sha>` — gone `` (qa-states.md "Already done?"). An issue whose files did not change is not
   re-checked. An empty scope does only this, then stops.
3. Map the scope's files to subsystems (keep `Plans/subsystems.md`); launch one reviewer per touched one.
4. Append the section; drop duplicates across reviewers, keeping the better-evidenced one.
5. Commit the day's issues file (and `Plans/subsystems.md` if it changed) — nothing else.
6. Report: the serious ones first, each in one line; counts per severity; tail:
   `🟥🟥🟥 issues in Plans/issues/YYYY-MM/YYYY-MM-DD.md — triage on the page`.
