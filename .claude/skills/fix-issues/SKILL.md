---
name: fix-issues
description: Acts on the user's triage of issues in the Q&A markup — does what the user approved, answers requests for more, verifies every premise by execution, and records each outcome in place with ✅ and the commit. Use after the user triaged a day's issues file (Plans/issues/YYYY-MM/YYYY-MM-DD.md) written by /find-issues.
argument-hint: "[file]"
disable-model-invocation: true
---

# /fix-issues [file]

Adapted for agent-bus from mdhouse's skill of the same name.

**Read [qa-states.md](../qa-states.md) first** — who is who, the states, replying, doing a task (premise
and side-effect gates, proof, versioning), writing a stage, committing, the report. This file says only
what `/fix-issues` does with each state.

**Never trust the task** — an issue, a ✓ yes, a ⏳ are claims: first check by execution that the problem
exists and needs doing; if not, do not do it — reply with the evidence.

- `file` — default: the newest `Plans/issues/*/*.md`; a day's file, or `all` — every file in `Plans/issues/*/`
  with something open

**Before the first change** — read the whole file, then one line: `n to do (🎯 k) in c groups by cause ·
m AUTO · r replies · s wait on you`, and the groups with their items. The premise gate runs per group,
in parallel read-only subagents, before any edit; then group by group.

| State | Do |
|---|---|
| answered | **do it** as qa-states.md "Doing a task" says — what the answer names (a `no — <another way>`: that way) → `✅` |
| asks for more (incl. a bare `no`) | reply: the next `💡` or options; nothing left → "🚫 reject or ⏸️ defer?"; set `❓` |
| `⏳` `⚠️` `🎫` | as qa-states.md says |
| `🎯` + untriaged | **do it** (qa-states.md "🎯 Selected") → `✅` |
| untriaged | **AUTO** only (below) |
| anything else | nothing |

**AUTO** — untriaged, 🟠 or ⚪, confirmed by execution, mechanical, local: a stale comment, a doc that
contradicts the code, a broken link, `!=` → `!==`. Never 🔴 or 🔵, never an item with options or a choice
(a choice is the user's), never wording or taste, never a change of exit codes, ports, service or CLI
behaviour, never a file another agent has uncommitted changes in or has claimed on the bus (`git blame`
cannot tell agents apart here: every commit has one author) — reply `💡👾` naming the conflict, set `❓`.
Say `AUTO` in the record.

A disproved untriaged 🟠 / ⚪: reply with the evidence and set `🚫` — the one stage you set unasked,
because nobody decided anything. A disproved 🔴: reply and set `❓` — the user decides.

**Done means verified here**: the fix's own check seen to fail without it, then `src/smoke.sh --slow`
green ([verification](../../../CLAUDE.md#verification)); a behaviour fix also bumps PATCH with a changelog
line, one bump per commit group. Deploying is not part of the run — say in the report what a deploy
would need, and leave it to the owner.
