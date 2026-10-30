# Docs rewrite

📌 **TL;DR:**
- **Problem:** the docs explain parts (daemon, queues, records) before they say why anyone cares, and few pages show something you can run.
- **Outcome:** every doc answers, in this order, why it exists, what you get, how to use it with a real working example, and only then the technical details.
- **How:** one standard, [WRITING.md](../../WRITING.md#how-we-write-docs), pointed to from [CLAUDE.md](../../CLAUDE.md#writing-conventions), a per-page check, and batches on disjoint paths that both teams can work in parallel.
- **Size:** about 130 working hours over 11 batches, no version bump. Status: approved 2026-10-30 by the owner; the rule lands in CLAUDE.md, then `docs/runner/README.md` is written first for the owner's review.

## The rule

- **Owner:** [WRITING.md](../../WRITING.md#how-we-write-docs) holds the whole standard: order, voice, form, examples and the page checklist.
- **Applies to:** every page under `docs/` (except the pages below), agent and runner guides, client READMEs, each `agent.json` description and method help, the CLI help pages, and the intro text of each web page.

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
| 3 | `docs/agents/` pages: imap, logwatch, slack, telegram, cron | 11 | landed in 0.9.187, `cdeb04a7` |
| 4 | `docs/agents/` pages: `debug.md`, `eventlog.md` | 4 | landed in 0.9.187, `cdeb04a7` and `41b313eb` |
| 5 | `docs/00`-`05` | 14 | none known |
| 6 | `docs/06`-`09`, except 07 | 10 | none known |
| 7 | `docs/07-setup.md` (978 lines, the biggest) | 6 | team 1 distribution (T-6) holds it; start after it lands |
| 8 | `docs/runner/*`, `docs/user/*` | 18 | none known |
| 9 | agent manifests: `agent.json` text and golden fixtures, in 2-3 sub-batches by agent | 16 | `src/agents/shared`, golden help, `smoke-map.json` are team 1's |
| 10 | web page intros (`src/web/pages`) and CLI help text (`src/cmd/*`) | 9 | none known |
| 11 | client READMEs and `src/*.md` | 7 | RT8 (w2c) is paused in `src/clients/`; `src/clients/README.md` also changes on the wait=0 branch |

- **Order:** the CLAUDE.md rule, then `docs/runner/README.md` (owner review), then the batches; each batch lands after one owner OK. Batch 0 (the checker) can run in parallel with the first page.
- **No two batches share a path:** a held file is skipped and listed in its batch until the holder lands.

## Batch 9 progress

- 9a rewrites `ai`, `at`, `cron`, `debug`, `eventlog`, `fetch` and `webhook` descriptions and method help (42 methods). Each owns `help.golden.json`; the shared text/JSON check runs in fast checks and the normal Go shard. A manifest-only description mutant fails on an assertion, then passes after restoration.
- Retained examples ran read-only on `parf.us` on 2026-10-30: debug `ping`, `hello smoke`, `roles`, `sandbox`, `sleep 0.1`, `big 1`, `unicode`; at `list`; cron `sub list`, `later list`; eventlog `list`, `sub list`, `help`. Webhook's `routes` returned `error: Configuration, required Secret or listener unavailable`; that unsuccessful example is not retained in its manifest. The guide already shows this exact unconfigured-helper reply. Unverified examples were removed. No image or asset was removed.
- 9b rewrites standalone `git`, `logwatch`, `server-info`, `systemd` and `tmux-sessions` (65 methods), each with its own golden fixture. Non-prose metadata is unchanged; unverified or mutating examples are removed. No image or asset was removed.
- 9b's retained examples ran read-only on `parf.us` on 2026-10-30: git `list`, `autopull ab`, `status ab`, `log ab 5`, `rev ab`, `diff ab`; logwatch `tail 5`, `grep error 5`, `since 5m 5`, `sources`, `stats`, `sub list`; systemd `list service`, `timers`, `failed`, `status agent-bus-web.service`, `logs agent-bus-web.service 5 since 1h`; server-info `system`, `memory`, `disks`, `load`, `tls`, `uptime`, `os`, `cpu`, `inodes`, `pressure`, `units`, `time`, `dns`, `ports`, `users`, `virt`; tmux-sessions `list`, `--status` (the latter truthfully reports the absent session).
- 9c rewrites `db`, `imap`, `qdrant`, `rag`, `slack`, `sms-telnyx`, `smtp`, `telegram` and `voyage` descriptions and method help (57 methods), each with its `help.golden.json`, and regenerates the agent index. Examples that cannot run today are removed; kept are slack `whoami`, `channels alerts` and `sub list`, telegram `whoami` and `sub list`, qdrant `request GET /collections` and imap `status` and `sub list` (run read-only on `parf.us` on 2026-10-30), and rag's five, which `test_rag.py` parses. A manifest-only help mutant (slack `whoami`, no fixture change) fails `GoldenHelp.test_authored_help_matches_reviewed_fixtures` on an assertion, then passes after restoration. No image or asset was removed.
- Discord shipped with B.27a in 0.9.206 (`4e74e3b6`): its manifest leads with outcomes, carries no examples and owns its `help.golden.json`. The manifest inventory is 22 agents and 174 methods, every one pinned.
- 9a landed at `07065bcf` and 9b at `9005e89a` (0.9.190), 9c at `99b1f94f` (0.9.191), each after its normal gate. 9b's method-help-only mutant fails `GoldenHelp.test_authored_help_matches_reviewed_fixtures` on an assertion, and the restored fixture passes. The golden check is `python3 -B -m unittest discover -s src/agents/shared -p test_help_golden.py -v` from the checkout root.

## Acceptance

- **Standard:** every page meets [the checklist in WRITING.md](../../WRITING.md#-checklist-for-a-page); the rest of this list says how each point is checked.
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
