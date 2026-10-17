# Markup for plan items

📌 **TL;DR:** Questions, decisions and tasks under `Plans/` are written as list
items with a leading glyph, an ID and a sub-list of labelled fields — not as
table rows. The form is mdhouse's item markup, which mdhouse parses, renders and
writes back, so it is a contract with a tool rather than a house style. This
file is the project's profile: what we add, what we override, and how a file
converts without losing anything.

| | |
|---|---|
| Upstream | `doc/agent-markup.md` and `doc/qa.md` in the mdhouse checkout at `/home/parf/src/mdhouse`, served on this node at [agent-markup](http://localhost:7777/mdhouse/doc/agent-markup.md). Every upstream rule holds unless this file overrides it; the rules most easily missed are restated below |
| Reviewed against | the parser — `src/lib/qa.ts`, `qa-html.ts`, `legacy.ts` — on 2026-10-17. A claim here about rendering was run, not reasoned |
| Glyph meanings | [glossary § glyphs](../docs/glossary.md#glyphs) owns this project's glyph vocabulary. The stage table below is the subset mdhouse acts on, not a second vocabulary |
| Why a profile, not a fork | the upstream rules are generic and three skills under `/rd/.claude/skills/` read them. Ours are the field set, the ID rule and the conversion procedure |

## Why this is not cosmetic

mdhouse finds an item on a list or quote block and nowhere else. A table row is
never an item, so a question in a table has no answer form, no thread, no 🎯
selection and no place in the page's summary counts. Every question, decision
and task we hold today is in a table, which is why none of them can be worked
from the page.

| | |
|---|---|
| What stays a table | reference material: the stage table in [Plans](README.md#stages), rule and field tables in topic files, anything with no ID and no lifecycle |
| What becomes an item | anything with an ID, a state, or something waiting on somebody: `TODO.md`, `QUESTIONS.md`, `DECISIONS.md`, `DONE.md`, `FUTURE.md`, and the items inside a topic file |
| What is not converted | `Plans/done/` and every `done/` — history, which never holds a current contract. It carries its own older markup, including a second complete one in `R0.8-MVP/done/TODO-before-rewrite.md`, and rewriting it risks the record for no working gain |

## An item

```markdown
- ⏳ M.3 Processes
  - Builds: process ranges, the jsonl frame, in_flight, signals, idle, restart
  - Depends: M.1, M.2
  - Format: the jsonl frame, to approve before any code (№Q160)
  - Acceptance: a kept process takes a second message without re-exec
  - Mutant: drop the keep-alive and the second message re-execs — that must fail
  - Home: [runner waves](R1.0-Release/TODO.md#runner-waves)
  > 💬👾 Q160 asks whether each frame carries the message ID.
```

| Part | Rule |
|---|---|
| Glyphs | the stage first, then severity; a space after each; **at most four, 🎯 counted among them** |
| ID | right after the glyphs, as the item's label |
| Title | one line, short, reading as a sentence; no phrase cut in half |
| Fields | a sub-list under the title, one `- Label: value` per line |
| Thread | a quote under the sub-list, one `💬` or `💡` per turn, a blank `>` between turns |
| Signature | the agent signs `👾` glued (`💬👾 …`); a person is `👤parf` |

Append only. Never edit or delete a turn, and never touch a person's words.

### Why the fields are a sub-list

Because any other form is unreadable in the tool this markup exists to serve.
`qa-html.ts:110` splits an item's body on `Evidence:` and `Impact:` and nothing
else, and `inline()` (`qa-html.ts:50`) collapses every newline to a single
space. Six plain `Label:` lines therefore render as one run-on paragraph with
each label glued to the end of the previous value. As a sub-list they parse as
`case` children and render one per line.

Two consequences, both absolute:

- **Nothing follows the sub-list except the thread.** A `Label:` line after it is
  absorbed into the list.
- **No field ever follows the thread.** `listNode` (`qa.ts:294-319`) ends the
  item's head at the first quote; a line after the quote is in the file, and in
  the item's hash, and renders nowhere. mdhouse appends every turn at the item's
  end, so the bottom of an item is precisely where "append only" tempts an agent
  to add one.

## Glyphs

Only these lead an item line. A glyph outside the set stops the reader and the
line stops being an item. Today exactly one legend exists in all of `Plans/` —
`R1.0-Release/TODO.md:6-7`, for `☑` and `☐`. This table replaces it.

| Stage | Means here | Who acts |
|---|---|---|
| ❓ | open question — no answer exists yet | the owner |
| ⁉️ | two sources contradict; name both | the owner |
| ⏳ | decided, and an agent is to build it | an agent |
| ⛔ | blocked — nothing ran, because something it needs is unsettled | nobody, until that clears |
| ⚠️ | partly done, follow-up named | an agent |
| ✅ | done — with the version or commit that did it | nobody |
| 📌 | a decision much other work follows | nobody |
| 🚫 | cancelled or rejected by decision; nothing failed | nobody |
| ⏸️ | deferred, on hold | nobody |
| ❌ | it ran and did not pass | report it |
| 🎫 | a ticket was requested; the person's `💬` must name who | an agent files it, then ✅ |
| 🎯 | selected for the next run — a person sets it, never an agent | |

Severity is 🔴 high, 🟠 medium, ⚪ low and 🔵 information, written after the
stage and only where items differ in it. Most plan items carry none. 🔵 is a
severity in the parser, not a stage.

Three traps:

- **`⏳` says an agent is working.** `qa-html.ts:193` renders it "waiting on
  agent". Work that is stuck on an unanswered question is **⛔**, with
  `Depends:` naming the question — never ⏳, or the page tells the owner an agent
  has something that is in fact on their own desk.
- **`🎫` starts the ticket flow.** `applyQa` (`qa.ts:566`) refuses a 🎫 action
  whose text names no `👤`/`👥`, and the item counts as open until a ticket is
  filed. Work *moved to another plan* is ✅ with `Moved:`, not 🎫.
- **🎯 consumes one of the four leading glyphs**, and `setGlyphs` writes it
  first. A person targeting an item that already has four pushes the ID out of
  reach of `ITEM_ID`, and the item loses its anchor with no error.

## Fields

Every column our plan files carry today has a home here.

| Field | Holds | From |
|---|---|---|
| `Details:` | the explanation a one-line title cannot carry, in the source's own words | the continuation lines of a checklist item or a prose row |
| `State:` | a dated or versioned fact: `built in 0.8.66` | the TODO *State* column |
| `Result:` | what came out of it | DONE's *Result* |
| `Evidence:` | how it was proved | DONE's *Evidence*; upstream |
| `Impact:` | who hits it, how often, what it costs | upstream |
| `Builds:` | what the task itself covers, when the title cannot | *Builds* in the wave tables |
| `Depends:` | what must precede it, including a question it waits on | *Depends on*, *Must precede it*, *Prerequisite*, *Dependency* |
| `Parts:` | the items this one is a parent of | a row that heads a sub-table |
| `Format:` | a wire format or representation to approve **before** any code | *Format to approve first* |
| `Acceptance:` | the falsifiable check, carried over in the source's own words | *Acceptance*, *Done when* |
| `Mutant:` | the change that must make the check fail | *mutation that must fail*; the word this repo already uses |
| `Settled by:` | who decides, and at which gate — `owner, before M.3` | QUESTIONS' *Settled by* |
| `Why:` | the reason recorded with a decision | DECISIONS' *Why* |
| `Date:` | the date a decision was taken | DECISIONS' *Date* |
| `Supersedes:` / `Replaced by:` | the design on either side of a replacement | *Earlier design*, *Replacement*, *Replaced by* |
| `Earlier rows:` | provenance into the pre-rewrite index | DECISIONS' *Earlier rows* |
| `Moved:` | where an item went, with the date | the `⏩` row |
| `Open:` | what is still open on something otherwise done | web DONE's *Open* |
| `Home:` | the owning section, when the item is not it | [canonical home](../CLAUDE.md#working-rules) |

Rules:

- One label per line, in this order. A label not defined here is ordinary text
  to every reader; add it here before using it twice.
- An empty cell becomes an absent field. Never carry an `—` across.
- `Acceptance:` keeps the source's wording and all of its parts. Where a cell
  joins a check and its mutation with a `;`, the check goes to `Acceptance:` and
  the mutation to `Mutant:` — that split is the only rewording a conversion may
  do, and the clauses themselves are carried verbatim.
- `Mutant:` is required where one exists and **recorded as missing where it does
  not**. Documentation-only work has no code mutant by rule
  ([verification](../CLAUDE.md#verification)); write
  `Mutant: none — documentation; link and anchor check only`. A migration never
  invents proof that was not there.

## State is evidence, not status

The glyph is the status. `State:` holds the fact that explains it.

mdhouse's writer changes glyphs and the thread and leaves every field byte for
byte. Settling an item whose body says `State: not built` produces `✅` above
that unchanged line, on the page, with nobody told. So:

| Write | Not |
|---|---|
| `✅` + `State: built in 0.8.66` | `State: built` with no version |
| `⏳` with no State | `State: not built` |
| `⏸️` + `State: on hold since 2026-09-30, on the owner's word` | `State: on hold` alone |

The owner's ✅ button sets the glyph and appends their turn; it writes no
`State:`. Supplying the version afterwards is the agent's job, in the same
session that acted on the item — not a sweep somebody schedules later.

## States `☐` currently flattens

`☐` says *not built* and hides which of several things is true. Resolving it is
the one place a reader gains knowledge — a judgement each time, read off the
row's own words, never a mechanical mapping. Where a row's box and its own text
disagree — `TODO.md:69`, `☐ M.1 … (done: … deployed 2026-10-16)` — the text
wins and the item says what is built.

| Today | Becomes |
|---|---|
| `☑` + `built in <version>` | ✅ + `State: built in <version>` |
| `☐`, decided, nothing in the way | ⏳ |
| `☐`, partly built | ⚠️ + `State:` naming what is built |
| `☐`, waiting on an unsettled choice | ⛔ + `Depends:` naming it |
| `☐`, parked | ⏸️ + `State:` saying since when and on whose word |

A task that waits on a question keeps its own identity. It does **not** become
that question: the task is work, the question is a choice, and only the question
carries ❓.

Never leave `- ☐` or `- ☑` as a list item. mdhouse rewrites four box forms —
`- ☐`→❓, `- ☑`→✅, `- ✔️`→✅, `- ☒`→🚫 — and it converts the **whole file**, not
the item being edited (`src/lib/legacy.ts`, applied in `src/server.ts:160`).

It needs a list marker, so `| ☐ |` inside a table cell and a bare `☐` paragraph
are left alone. Other forms it rewrites on sight: `**Q:**` / `**A:**`
paragraphs, `::: ` fences, and inside a quote `?` `?!` `Q:` `A:` `T:` `!` `!!`
`[!TIP]` `[!QUESTION]`. That last group fires on a converted item's own thread —
a reply beginning `A:` or `T:` or `! ` is silently rewritten — so a thread line
never starts with one.

## IDs

| | |
|---|---|
| Grammar | `ITEM_ID` is `^([A-Z][A-Z.-]*\d+[a-z]?(?:\.\d+)?)(?=\s\|$)` (`qa.ts:193`). `M.3`, `B.2`, `Q160`, `K.24`, `Q-R14` and `A.4b` match; lowercase or digit-initial does not, and a label that does not match is swallowed into the title with no anchor and no error |
| Questions | `Q…` is one namespace across every plan, including settled ones ([working rules](../CLAUDE.md#working-rules)). Allocated only when the owner has a choice to make. A conversion never allocates one |
| Waves and tasks | `M.3`, `B.2`, `P.1`, `K.24` — scoped to their series, not globally unique, and **never renumbered**. `B.1`–`B.4` already means two unrelated things: bundled agents in `R1.0-Release/TODO.md`, credentials in the archived pre-rewrite plan |
| Migration labels | a row with no ID gets a scoped label from a new series whose prefix no live plan uses — `RT.1`, `RD.1`, `RW.1`. It is a label, not a question: it names a row that already existed. Record the series and its meaning where it is first used |
| Definition against reference | one item defines an ID; everything else mentioning it is a reference. The same ID in `TODO.md` and `DONE.md` for the same work is one entity in two stages, not a clash |
| Reference | `№Q165` in the same file, a link from another. Never a bare ID, never `(Q165)` |

An ID range has no `№` form — `Q150–Q158` stays as it is written.

## Headings stay; an ID is not an anchor

An ID is the item's label and the anchor **mdhouse** renders for it
(`qa-html.ts:160`); ordinary Markdown gives a list item no `id` at all, so
`#M.3` resolves on the mdhouse page and nowhere else. GitHub and a plain editor
are first-class readers here.

Section headings and their anchors therefore survive conversion unchanged. 140
links from tracked files outside `Plans/` point into `Plans/*.md#section`, at 87
distinct anchors — and inside `Plans/` itself there are about 2550 more.

| Linking to | Write |
|---|---|
| a section, from anywhere | the heading anchor, as today |
| an item, in the same file | `№Q145` |
| an item, from another file | `[№Q145](R1.0-Release/QUESTIONS.md#open-questions)` — the `№` names the item, the link lands on the section that holds it |

**Items stay under the heading that holds them today.** In `QUESTIONS.md` every
question item stays under `## Open questions`; the detail sections — `## Record
tags`, `## Assigned roles` and the rest — keep their prose as context and refer
to the item. Moving items out to their detail sections would leave
`## Open questions` empty while four inbound links to `#open-questions` kept
resolving and stopped being true, which neither check below would catch.

## Options and threads

Upstream features that are the point of working from the page, and that a
conversion must not quietly drop.

| Form | |
|---|---|
| Pick one | `- ( ) choice` sub-items; `(x)` is the person's pick |
| Pick several | `- [ ] choice`; `[x]` picked |
| Suggestion marks | `🌟` the agent's pick, `⭐` runner-up, at the end of the option |
| A suggested answer | `💡👾 …` — the person agrees (`✅ 💡`) or cancels (`🚫 💡`) |
| A rejected alternative | `- 🚫 <alternative> — <why>` under a ✅ or 📌 item, rendered muted and uncounted. Never "X was rejected" in prose |

A markup-only conversion preserves existing alternatives and recorded preferences.
When the owner asks for answers or ideas, also prepare the decision: give
concrete selectable options and a `💡👾` recommendation with a brief reason.
Mark the recommended alternative `🌟` and a credible runner-up `⭐`; make the
trade-off clear. Use `( )` for mutually exclusive choices and `[ ]` for
independent selections. Do not force a runner-up onto independent requirements
or invent weak alternatives just to fill a list.

A recommendation is not a decision. Leave options unselected until the owner
chooses; never add a new undecided proposal to an already settled item.
Keep explanations and all earlier turns, and put the options before the thread.

`💡` has a trap: the newest `💡` with no `✅`/`🚫` in front of it makes a
question permanently unanswered (`qa.ts:181-182`), whatever the owner types
afterwards. An ordinary reply is `💬👾`; `💡👾` is only for a proposal awaiting
a verdict.

## Converting a file

Conversion is markup only. The wording, the order and the facts are carried
across unchanged; nothing is removed, merged or reordered unless the owner said
so. Where a table's order is meaningful it says so in its own text — for
example `R1.0-Release/TODO.md:69-79` — and the item order must match it.

- [ ] **Account for every cell, not every row.** Before the edit, write out each
  row as its cells. After it, every cell must be findable in a field or the
  title. An ID, a title and a link surviving proves nothing: a row keeps all
  three while its acceptance, dependency and provenance go missing.
- [ ] **Check what the page shows, not only what the file says.** Open the
  converted file in mdhouse and read the items. A field that renders into the
  middle of another field's value has been lost to every reader who uses the
  tool, however correct the source looks.
- [ ] **Anchors are unchanged, and headings keep their content.** A link that
  still resolves to an emptied section is a worse failure than a broken one.
- [ ] One row becomes one item. A cell holding two facts becomes two fields,
  never a run-on title.
- [ ] Where one piece of work is a row in two tables of the same file — as
  Federation, Runner and Client libraries are in both of `TODO.md`'s tables —
  it is one item, and the second row's cells become fields on it. Say so in the
  commit, since that is the one place rows are merged.
- [ ] Resolve `☐` and `☑` by reading each row, and record the evidence for the
  choice in `State:`.
- [ ] Account for **every** residual box form and legacy form in the file, not
  only the rows being converted, because the first edit through mdhouse converts
  the file as a whole.
- [ ] Every ID mentioned anywhere in the project gets `№` and, across files, a
  link. That reaches beyond the file being converted.
- [ ] A header with no rows is a schema someone kept on purpose. Leave it, or
  ask.

One file at a time, committed on its own, so a conversion that lost something
can be read in a single diff.

## What does not change

Terse English, small files, tables for reference material, and the
[plan lifecycle](../CLAUDE.md#working-rules): README holds stable knowledge,
TODO unfinished work, DONE results, FUTURE follow-up, QUESTIONS open choices,
DECISIONS the dated record. This markup changes how an item is written inside
those files, not which file it belongs in.

The `📌 **TL;DR:**` opener stays a paragraph. It is not an item, and the 📌 in
it is not the decision glyph.
