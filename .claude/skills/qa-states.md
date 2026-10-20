# Q&A states — shared by /find-issues and /fix-issues

Adapted for agent-bus from mdhouse's `.claude/skills/qa-states.md` (`/home/parf/src/mdhouse`), which
also serves its `/ask-questions` and `/resolve-questions`; those two are not installed here, because
questions and decisions follow [CLAUDE.md § working rules](../../CLAUDE.md#working-rules).

One reference, so the skills never drift: every rule below is stated here only. It is how mdhouse
renders the file — the skills read it the way the page does. The item form, the stage glyphs and the
fields: [plan markup](../../Plans/MARKUP.md#an-item), whose [glyph table](../../Plans/MARKUP.md#glyphs)
is the subset mdhouse acts on; what every glyph means: [glossary § glyphs](../../docs/glossary.md#glyphs).
Where this file and the plan markup disagree, the plan markup wins.

## Never trust the task

**Before doing anything, check that there is a problem and that it needs doing** — by execution: run
it, reproduce it, read the line, grep the symbol. Every task is a claim: a reviewer's issue, the user's
✓ yes, a ⏳, "do it", your own earlier reply. No problem, or nothing to gain → do not do it; reply with
the evidence. Doing a task whose premise is false breaks what works.

**Then weigh how real it is — the impact**: who hits it (the user every day, a fresh npm install, nobody
but a crafted request), how often, and what it costs when it does (lost text, a broken release, a wrong
colour). Say it in a few words: `Impact: every save on a read-only root — the answer is lost`. Little
impact and a costly or risky change → propose not doing it.

## Who is who

- **The user** — a `💬` signed with a person or team badge (`👤parf`, `👥design`), or not signed at all.
  The page signs with the user's name (prefs `"me"`, else git); it always signs `settled`, `yes`, `no`,
  `done` and `elaborate`. A `💡` signed by a person or team is their answer, not a proposal.
- **The agent** — always signs a bare `👾`, glued: `💬👾 …`, `💡👾 …` — no name, whichever agent it is.
- **A source** — `📡slack`, `📡mail`: relayed, not a decision. If you relay one, add your own
  `💬👾 …` after it.

## A thread

One `💬` per turn, indented into its item, a blank `>` between turns. A partial answer: `💬 ⚠️ 👤name …`
(the ⚠️ may also follow the badge).

```markdown
- ❓ Do we keep the old URLs?
  > 💬👾 Redirect for a year?
  >
  > 💬 👤parf elaborate — which links are out there?
```

Append; never edit or delete a turn, never touch the user's words. The agent's **records** —
`💬👾 → …`, `` 💬👾 `<sha>` — … ``, `💬👾 🎫<ID> → …` — are not turns: they never
reopen an item and never count as its last word.

## 🎯 Selected

`🎯` first in an item (before its stage and severity: `- 🎯 🔴 …`) selects it for the next run. **When
any item in the file carries `🎯`, `/fix-issues` acts on the `🎯` items only** — the rest wait.
Acting on one removes its `🎯` (`- 🎯 🔴 …` → `- ✅ 🔴 …`, or `- ❓ 🔴 …` after a reply). Never add `🎯`
yourself.

A `🎯` on an **untriaged** issue is the user's yes: do it — the newest `💡`, else the fix the item states
→ `✅`, any severity; with options and no pick → reply "which one?", `❓`. A `🎯` on an unanswered `❓`
→ as a bare `⏳`: find out, reply.

## The state of an item

`✅` `🚫` `⏸️` are **closed** — the agent never acts on them. To reopen, the user sets `⏳` or `❓`.
A closed item's closing turn says who closed it: `✅` + `💬 👤<name> settled …` — the user, nothing to
carry; `✅` without it — the agent, and for the agent the `✅` alone is enough.

| First glyph | Condition | Who acts — what |
|---|---|---|
| `❓` `⁉️` (question) | not answered, not asking for more | nobody — waits on the user |
| | **answered** (below) | the agent: carry it over / do it → `✅` |
| | **asks for more** (below) | the agent: reply, keep `❓` |
| `🔵` (issue) | informational — treated as done or not relevant | nobody |
| `🔴` `🟠` `⚪` (issue) | untriaged — nothing from the user | `/fix-issues` AUTO only |
| | **answered** | the agent: do it → `✅` |
| | **asks for more** | the agent: reply, set `❓` first |
| `⏳` | the user's last `💬` says what | do it → `✅`, or reply → `❓` |
| | the agent's own `💬` is last (it proposed something) | the user said yes: do what you proposed → `✅` |
| | no `💬` — a question / an issue with one `💡` / with options | find out, reply, `❓` / do the `💡` → `✅` / reply "which one?", `❓` |
| `⚠️` | the user's `💬` (else the item text) says what is missing | finish → `✅`; unclear → reply "what is missing?", `❓` |
| `🎫` | a ticket requested; the user's `💬` names who (`👤name` / `👥team`) | this repo tracks work in its plans, not GitHub: add an item to the owning plan's `TODO.md` in the [plan markup](../../Plans/MARKUP.md#an-item), with an ID by its [rules](../../Plans/MARKUP.md#ids) and the issue linked → `✅` + `💬👾 🎫<ID> → 👤name`. Anything outside the repo (a GitHub issue, a message) only on the owner's word |
| `⛔` `❌` | blocked / failed | nobody — list them in the report |
| `✅` `🚫` `⏸️` | closed | nobody |
| `📌` | a decision — settled, applies | nobody acts on it as a task; follow it in all work; never change it — reply under it |

**Answered** — only the newest `💡` counts (an older one is superseded); when it is still undecided,
nothing is answered. Otherwise any of:

- that `💡` decided — `✅ 💡` + `💬 … agree …` / `accept`, or `🚫 💡` + `💬 … cancel` / `ignore` (the item is `🚫`;
  an older `no — <their alternative>`: do that way)
  (an issue's `ignore` closes it: `🚫`);
- a one-of option picked `(x)`;
- any-of: the item's own `💬 … done` (zero ticks = "none of these"; a `💬` under an option is a comment
  on that option);
- otherwise the last turn is the user's and whole — not `💬 ⚠️`, not `elaborate`, not a bare `no`.

A last turn from `👾` / `📡` (a record is not a turn) means not answered — it asks the user again, even
after a pick.

**Asks for more** — the last entry is the user's: `💬 ⚠️ …`, `💬 … elaborate — …`, a bare `no`, or a
plain reply under an undecided `💡`.

A user's answer that is **only** a stage word — "leave it", "drop it" → `🚫`; "later" → `⏸️` — is that
stage: set it with `> 💬👾 🚫 — per your "leave it"`. A sentence that contains the word is an
answer, not a stage. Ask once, never twice.

## Replying

`> 💬👾 <the answer, with evidence you verified — read the code, run it>`, a blank `>` before it.
Where a choice is the next step, the `💡` or the options go **under** that reply, never instead of it.

## Doing a task

1. **Premise gate — never skipped** (see *Never trust the task*): is there a problem, does it need doing?
   Reproduce it: run the test, read the line, a scratch instance. Disproved after the user approved →
   reply with the evidence, set `❓` (the user decides).
2. **Already done?** — the premise no longer reproduces and a commit names the claim (not only the line)
   → record the commit, `✅`.
3. **Side-effect gate** — the change removes a deliberate design (a comment says why), crosses a
   [design boundary](../../CLAUDE.md#design-boundaries), contradicts a [settled decision](../../docs/decisions.md#settled),
   changes what another caller or a test expects, or changes more than the item names and the user sees
   (an exit code, a wire field, a URL, a CLI verb) → reply with what it touches, set `❓`. The user's yes
   covers the item, not its side effects. A tightened validator is one: `validateKind` is also the restore
   gate, so a stored record it now refuses is ignored at the next start.
4. **Worth it?** — confirmed, but nobody can hit it and a mistake would cost little → reply with the
   evidence and a `💡👾 🚫 — <why>`, set `❓`: the user decides.
5. Change minimally. **Proof grows with what a mistake would cost and how often it hits**: storage and
   restore, credentials and ACLs, the runner's process boundary, setup run as root → a scratch daemon
   and a check seen to fail first; a wording nit → a quick check. Every code change ends with
   `src/smoke.sh` green, and every behavioural fix is [mutation first](../../CLAUDE.md#mutation-first-then-belief):
   break it, watch the named check fail on an assertion. Update the docs it touches; a behaviour fix
   bumps PATCH with a changelog line ([versioning](../../CLAUDE.md#versioning)).
6. **The proof fails** → undo your own change only, by editing your hunks back — never `git checkout --`,
   `git reset` or `git stash`, which also undo other agents' work in progress ([git](../../CLAUDE.md#git)) —
   set `❌` + `💬👾` with the output; never `✅`, never a second try that widens the change. **Cannot
   start** (needs the live daemon, a secret, a missing tool) → `⛔` + the obstacle.

## Writing a stage

Replace the first glyph; a severity stays after it: `- ❓ …` → `- ✅ …`, `- 🔴 …` → `- ✅ 🔴 …`,
`> ❓ …` → `> ✅ …`. Never two stage glyphs (`✅ ❓`); never write `✅ 💡` / `🚫 💡` (the user's agree / cancel, accept / ignore);
never tick `(x)` / `[x]`. A record is optional — add one when it helps the user find the result:

```markdown
  > 💬👾 → `DECISIONS.md`: <the decision, short> · `TODO.md`: <the action>
  > 💬👾 `<sha>` — <what, and the proof>
```

## Committing

- **The Q&A file after a page triage is dirty by design** — the user's answers; the page saves, it does
  not commit. Running the skill is the ask: commit that file alone first (`Record the owner's triage of
  the <date> issues`), before the first change. The dirty-file rule below covers every other file.
- Commit subjects follow [CLAUDE.md § git](../../CLAUDE.md#git): one plain sentence, no prefix or tag;
  stage by path, never `git add -A` / `.` / `commit -a`. Other agents commit here at the same time.
- **Group by root cause first**: items with one cause get one change and **one commit per group** —
  the change and the docs it touched — `git commit <those paths>`. A crash between groups leaves nothing half.
- **Update the Q&A file every time you deal with a task** — done, replied, refused, failed: its stage
  and record (`` 💬👾 `<sha>` — … ``, 🎯 off) written right after it, and that file committed alone
  (`Record issues <ids> as <stage>`) before the next task.
- The commit body names every issue it closes: `Fixes issues/2026-10/2026-10-07.md#B2, #B3`.
- A file with the user's or another agent's uncommitted or staged edits (`git diff --quiet HEAD -- <file>`
  fails, or it is untracked): add your lines, do not commit it, say so in the report — or ask the peer on
  the bus. Leftovers that are clearly yours (your records, `— from <file>` entries) — commit them.
- Never push, never publish. Never restart or deploy the dev node's live services (`agent-busd`,
  `agent-bus-web`) or touch their database unless the owner asks; mdhouse on :7777 is where the page is
  read, never written by a skill.

## Report

**Serious first**: every confirmed issue that loses stored records or secrets, leaks a secret or a
credential, or lets a caller past an authority boundary — done or not. Then counts: done (with commits), carried, replied, untouched. Listed by line:
`⛔` / `❌` items, every `🚫` the agent set, and every AUTO with its commit — code nobody asked for. Every item that waits on the user on its own line starting with ❓. Last line:
`🟥🟥🟥 <n> wait on you in <file>` — `/find-issues` has its own.

The normal gate is `src/smoke.sh` (no flag); `--fast` is edit-loop feedback.
Use `--slow` only before a release/STABLE or for huge cross-cutting, storage/schema
or sandbox changes, as [Verification](../../CLAUDE.md#verification) requires.
