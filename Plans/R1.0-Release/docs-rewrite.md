# Docs rewrite

📌 **TL;DR:**
- **Problem:** the docs explain parts (daemon, queues, records) before they say why anyone cares, and few pages show something you can run.
- **Outcome:** every doc answers, in this order, why it exists, what you get, how to use it with a real working example, and only then the technical details.
- **How:** one rule in [CLAUDE.md](../../CLAUDE.md#writing-conventions), a per-page check, and batches on disjoint paths that both teams can work in parallel.
- **Size:** about 130 working hours over 11 batches, no version bump. Status: approved 2026-10-30 by the owner; the rule lands in CLAUDE.md, then `docs/runner/README.md` is written first for the owner's review.

## The rule

- **One order for every page:**
  1. **Why:** the problem solved and for whom, in plain words.
  2. **What you get:** the outcome.
  3. **How to use it:** real working examples, commands or messages that ran on a real node today, copied from the run, in a closed `<details>` when long.
  4. **Technical details:** internals, formats, limits, last, in `<details>` when long.
- **First 10-20% rule:** by length, the first fifth of a page holds why, what you get and the one way to start. A reader who stops there knows enough.
- **Bullets over prose:** short bullets, one simple idea each; a paragraph longer than about three lines becomes bullets or a table.
- **No invented examples:** an example that was not run is deleted, not guessed.
- **No jargon leads:** broker, registry, daemon, queue, wire and storage terms appear only in step 4 or after a plain-word lead.
- **Applies to:** every page under `docs/` (except the three contract pages below), agent and runner guides, client READMEs, each `agent.json` description and method help, the CLI help pages, and the intro text of each web page.

## Not rewritten

- `docs/constitution.md`: the contract; stays as the model, other pages link to it.
- `docs/decisions.md`: an index of decisions.
- `docs/glossary.md`: vocabulary (owner, 2026-10-30).
- `Plans/`, `Plans/done/`, changelogs: history and plans.
- Built and pending status, section anchors and every link target stay as they are, except where a heading moves (then the old anchor is kept as an explicit anchor or every inbound link is fixed in the same commit).

## Inventory and size

Counts are at main `04b9ebf7`. Hours are my estimate for a full rewrite plus real runs of the examples plus one review round. A page of about 200 lines takes about 1.5 hours.

| Area | Files | Lines | Hours |
|---|---|---|---|
| `docs/agents/*.md`, 22 agent pages | 22 | 4,500 | 40 |
| `docs/agents/` README, authoring, create-agent-prompt | 3 | 450 | 5 |
| `docs/00-09` topic pages | 17 | 5,100 | 28 |
| `docs/user/*.md` | 6 | 1,440 | 10 |
| `docs/runner/*.md` | 6 | 1,060 | 8 |
| Client READMEs (`src/clients/**/README.md`) and examples | 9 | 540 | 4 |
| `src/*.md` (INSTALL, README, MODULES) | 3 | 420 | 3 |
| `agent.json` descriptions and method help | 20 manifests, 154 methods | n/a | 12 |
| Golden help fixtures and one normal gate per manifest batch | n/a | n/a | 4 |
| CLI help pages (`agent-bus help`, runner and admin help) | 3 programs | n/a | 5 |
| Web page intros (`src/web/pages`, 10 pages) | 10 | n/a | 4 |
| The checker (batch 0) with mutants | 1 | n/a | 5 |
| Root README | 1 | 281 | 2 (the T-8 branch holds its opening) |
| **Total** | | | **about 130** |

Out of scope and left alone: `src/web/web-face/*.md` (design contracts, 6 files), `src/agents/**/README.md` (implementer notes, 15 files); both can follow later if the owner wants.

## Batches, on disjoint paths

Each batch is one worktree and one landing through `#ab-gate`, no bump; manifest batches need a normal gate. Batches share no file, so both teams can take them.

| Batch | Paths | Hours | Held by another task |
|---|---|---|---|
| 0 | the checker, `src/agents/docs_order_test.py` (fast list) and its mutants | 5 | `src/smoke-map.json` is team 1's |
| 1 | `docs/agents/` pages: ai, db, fetch, git, git-autopull, rag, qdrant, voyage, server-info, systemd | 15 | none |
| 2 | `docs/agents/` pages: smtp, sms, webhook, tmux-sessions, bus, at, README | 11 | `at.md` is cron's twin and T-2's engine; check before starting |
| 3 | HELD pages: imap, logwatch, slack, telegram, cron | 11 | subscription guides: T-2 docs branch (w2a); start after it lands |
| 4 | HELD: `debug.md`, eventlog.md when it exists | 4 | team 1 P.4 and T-2 |
| 5 | `docs/00`-`05` | 14 | none known |
| 6 | `docs/06`-`09`, except 07 | 10 | none known |
| 7 | `docs/07-setup.md` (978 lines, the biggest) | 6 | team 1 distribution (T-6) holds it; start after it lands |
| 8 | `docs/runner/*`, `docs/user/*` | 18 | none known |
| 9 | agent manifests: `agent.json` text and golden fixtures, in 2-3 sub-batches by agent | 16 | `src/agents/shared`, golden help, `smoke-map.json` are team 1's |
| 10 | web page intros (`src/web/pages`) and CLI help text (`src/cmd/*`) | 9 | none known |
| 11 | client READMEs and `src/*.md` | 7 | RT8 (w2c) is paused in `src/clients/`; `src/clients/README.md` also changes on the wait=0 branch |

- **Order:** the CLAUDE.md rule, then `docs/runner/README.md` (owner review), then the batches; each batch lands after one owner OK. Batch 0 (the checker) can run in parallel with the first page.
- **No two batches share a path:** a held file is skipped and listed in its batch until the holder lands.

## Acceptance

- **Per page, checked by the checker where it can be, by a reviewer where it cannot:**
  - Order: the four parts appear in order (why, what you get, how, technical); the technical part is last.
  - First 10-20%: why, what you get and one way to start sit within the first fifth of the page by length.
  - No walls of text: one simple idea per bullet; any paragraph longer than about three lines is bullets or a table.
  - No lead jargon: the first screen has none of the internal terms from the rule.
- **Real examples:** each example was run on a real node (this node or a disposable one) and the output is the run's; the batch's report lists the commands run and the node. An example that cannot be run is removed.
- **Links and anchors:** every link and anchor in a changed file resolves (`linkcheck` with the file list), and inbound links to a moved heading are fixed.
- **Status unchanged:** a reviewer diffs each batch for built/pending claims; none changes, none is added without evidence.
- **Checker mutants** (batch 0, [mutation rule](../../CLAUDE.md#mutation-first-then-belief)): the checker fails when a page puts the details first, when why is missing from the first fifth, when an example block is empty, and when a "why" paragraph opens with a jargon term. Each mutant compiles and fails on an assertion.
- **Manifest batches:** golden help fixtures updated in the same commit and a normal gate green.
- **Owner review:** `docs/runner/README.md` goes to the owner as an mdhouse URL first; each later batch lands after one owner OK.

## Proposed questions (not filed)

- Include `docs/user/` (6 files) and the client READMEs, as the owner's upgrade says: planned in, yes.
- Rewrite `src/web/web-face/*.md` and the implementer READMEs under `src/agents/`? Planned out; owner to say.
- A moved heading: keep the old anchor or fix inbound links? Planned: fix inbound links, no explicit anchors.
