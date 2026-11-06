# Team v2 — shared rules

📌 **TL;DR:** rules every AB-DVP role follows on top of its own role file and [Team v2](../README.md#roles):
the task lifecycle, a review bound to one commit, the hand-off checklist, usage control, doing simple
work directly, area affinity and change-type review triggers. Adopted from the mdhouse team (owner,
2026-11-05).

## On start

Read your role file, this file, `CLAUDE.md` and `CLAUDE.local.md`, then your checkpoint if you have one.
Confirm your role and current assignment to whoever told you to read it, in one line. If you have no
assignment, wait: a team launch is not a task.

## Owner questions

Every owner choice is an item in the owning plan's QUESTIONS.md, in [mdhouse's agent markup](http://localhost:7777/mdhouse/doc/agent-markup.md) (owner, 2026-11-05), so the owner answers it in the document: `- ❓ Qnnn title`, details indented, `( )` options with 🌟/⭐, one `> 💡👾` suggestion, IDs mentioned as `№Qnnn`. It reaches the owner through `#ab-ceo`, never as a chat-only question.

## Light on the host

Owner, 2026-11-05: "do not run heavy tests unless really needed". Run focused tests by default. A normal gate runs only for a landing, through `#ab-gate`. No `--slow`, repeat or load loops, or podman runs unless the task truly needs one and the lead approves it. Never run suites concurrently. Kill what you started by pid before you stop.

Owner, 2026-11-06: "do not run tests for minor changes, do not build release bundles - we are not close to release". A minor change (docs, plans, comments, wording, metadata) gets no test runs and no gate of its own: a link check at most, or it rides the next real landing. Whether a small code fix counts as minor is the lead's call; when in doubt, ask #ab-dir. No release bundles, container images, release.sh or podman matrix builds until the owner opens the release.

Owner, 2026-11-06: the live `#ai@parf.us` runs on a paid production key shared with the realmo.com site. Never call it, or any live paid provider (`#voyage`, SMS, mail), for a test, a check or a demo; tests use their fixture servers. A live call needs the owner's OK each time. `models` is free and allowed.

Owner, 2026-11-06: a change to one agent (`src/agents/<name>/`) is tested by that agent's own tests only; it runs no daemon, runner or other subsystem checks. Its deploy replaces `agent.d/<name>` and restarts that one agent (`agent-bus call '#runner/<person>@<node>' restart <name>`), never the runner.

Smoke lock identity: [CLAUDE.md § Verification](../../../CLAUDE.md#verification).

## Less overhead

Owner, 2026-11-06 ([TD7](../DECISIONS.md#TD7)):

| Rule | Means |
|---|---|
| Findings | A task is a bug the owner or a user would hit, a security gap, or a MED+ finding. Be sceptical: reject a finding with no real impact, saying why. LOW and polish findings go to one list in the plan and are fixed only when their file is touched anyway |
| Reads | A ⚠️ subsystem of [the subsystem map](../../subsystems.md) (authority, tokens, storage, parsers, runner, install) keeps two readers and a mutant; everything else gets one reader and no mutant. QA reads ⚠️ changes only |
| Bookkeeping | The issue and TODO update rides in the landing commit; no separate close-out commit or gate |
| Flaky checks | A check that goes green only on a rerun is quarantined at once (skipped, with a J-item to fix it), never rerun silently |
| Batches | Small and medium tasks land together in one branch and one gate; one version bump per deploy ([TD6](../DECISIONS.md#TD6)) |

## Task lifecycle

`assigned` → `working` → `ready` → `reviewed` → `landed` → `done`.

| State | Means |
|---|---|
| `ready` | the author committed the scoped change and reported checks and gaps, with the exact commit and `git diff --stat main...<branch>` |
| `reviewed` | the required verdicts name that exact commit. A later fix, rebase or conflict resolution makes it a new commit, which needs a fresh verdict before the gate |
| `landed` | `#ab-gate` put it on main |
| `done` | the close-out names the landed commit and readers, and the plan item is marked done. A WIP or preservation commit is never `ready` or `done` |

A reader answers with the commit it read: `T-n <short sha>: no high` or `T-n <short sha>: HIGH …`.
A lead sends the gate only a branch whose tip is the commit that was reviewed.

## Hand-off checklist

Before a pause, a compact or restart, a re-assignment, or a usage limit, the worker writes its checkpoint
with every line below, then reports it to the lead:

- task and plan link; owner
- worktree and branch; last commit
- uncommitted changes, if any (commit owned work first when it is at a safe boundary)
- checks run and their result
- what is left; blocker; next step

## Usage and workload control

`#ab-dir` owns it. During active work, about every hour and on resume:

- read every working window's limits: a Claude status line (`5h N%`, `7d N%` = quota **left**), a Codex status
  line (`weekly N% left`), a QA pane's reset time (China time, EST = −13 h);
- record the time, what is left, the resets and the decision in `local/dir/` notes;
- when a window is low for its planned work: give it smaller tasks, move work elsewhere, or tell it to finish
  its current step, checkpoint and stop. Keep capacity for review and hand-off; keep the Claude windows for
  authority and secret work;
- tell `#ab-ceo` in one line when a limit changes a plan or an ETA.

Workers and readers report low quota to their lead at once, with the reset time. Nobody polls while the team
is idle. This is workload coordination, not purchases or account changes.

## Simple tasks go direct

A simple task does not pass through every role. `#ab-ceo` does small plan and QUESTIONS edits. A lead may
write a trivial docs or status change itself (a close-out note, a TODO flip, a typo, a link) on a branch and
send it to the gate, without a worker. Code always goes to a worker.

## Area affinity

A worker keeps one area while it has work there: consecutive tasks from the same plan or subsystem go to the
worker that already holds that context. Before accepting unrelated work, a worker says which area it knows.

## Review triggers

Two independent readers, and the mutant proof where [CLAUDE.md](../../../CLAUDE.md#mutation-first-then-belief)
asks for one, are required for a change in a ⚠️ subsystem of [the map](../../subsystems.md) **and** for any
change, wherever its files are, that touches:

- authority, ACL, tokens or secrets;
- git operations, pushes or deploy steps;
- bus control: registration, Allow, subscriptions, runner management;
- file writes outside the agent's own work directory, path checks, deletions;
- store migrations or anything destructive;
- a substantial architecture change.

Anything else needs one reader. An author never counts as a reader.
