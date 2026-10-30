# Team v2

📌 **TL;DR:** An idea, not decided: a layered AI team for this repository. The
owner talks only to `#ab-ceo`. `#ab-dir` splits tickets, two leads route work to
workers, QA checks every diff, and a script agent `#ab-gate` alone lands on
main. The coordinating layer runs on `/fast` and reads only short receipts. The
heavy work runs below it on ordinary models.

## Status

Idea from the owner, 2026-10-29; partly running from 2026-10-30 with two teams
([decision](DECISIONS.md#recorded-decisions)). Node-local, not in Git, under
`local/`: the role prompts, `start-ceo.sh` for the `#ab-ceo` kitty window,
`gate.sh` behind `#ab-gate`, and the `#tmux-sessions/ab-dvp@parf.us`
Configuration that starts `DIR`, `LEAD1..2`, `W1A..C`, `W2A..C`, `QA1..2` and `GATE`. The
team before it is in [today's team](#today). Open choices are in
[questions](QUESTIONS.md#open-questions).

## Shape

Two teams run ([TD3](DECISIONS.md#recorded-decisions)): `#ab-lead1` leads team 1 and
`#ab-lead2` team 2, each with three workers and a QA. Names carry the project
prefix `ab-` ([TD2](DECISIONS.md#recorded-decisions)). Team 1 runs in the repository root and team 2 in `Plans/`; `#ab-ceo` and
`#ab-dir` have their own directories, `local/ceo/` and `local/dir/`. Each name
resumes its own conversation.

```mermaid
%%{init: {"flowchart": {"wrappingWidth": 420}}}%%
flowchart TD
  parf(["parf — owner"]) <-->|talks only to| ceo
  ceo["`**#ab-ceo** · kitty · Claude /fast · local/ceo/
  owner's single contact, answers at once
  writes tickets, hands them to #ab-dir
  keeps the waiting-on-owner list
  reports results; never executes or waits`"]
  dir["`**#ab-dir** · tmux DIR · Claude /fast · local/dir/
  splits tickets into tasks on disjoint paths
  picks the team by subsystem, balances load
  sets deadlines, requeues overdue tasks
  settles cross-team conflicts; no code`"]
  lead1["`**#ab-lead1** · team 1
  repository root`"]
  lead2["`**#ab-lead2** · team 2
  Plans/`"]
  gate[["`**#ab-gate** · script, no model
  sole lander on main`"]]
  main[("main")]
  ceo -->|ticket| dir
  dir -->|task| lead1 & lead2
  lead1 & lead2 -->|land| gate
  gate -->|ff-merge| main
```

Inside team N (1 or 2), every session runs in the team's directory:

```mermaid
%%{init: {"flowchart": {"wrappingWidth": 420}}}%%
flowchart TD
  lead["`**#ab-leadN** · tmux LEADN · Claude /fast
  assigns tasks by fit, reads receipts only
  sends to QA, then to #ab-gate; decides bump`"]
  wa["`**#ab-wNa**
  Claude
  account 1`"]
  wb["`**#ab-wNb**
  Claude
  account 2`"]
  wc["`**#ab-wNc**
  Codex`"]
  qa["`**#ab-qaN**
  opencode, z.ai
  read-only`"]
  gate[["#ab-gate"]]
  lead -->|task| wa & wb & wc
  lead -->|check| qa
  lead -->|land| gate
```

A task's path, each arrow one bus message:

```mermaid
sequenceDiagram
  participant ceo as ab-ceo
  participant dir as ab-dir
  participant lead as ab-leadN
  participant w as ab-wNx
  participant qa as ab-qaN
  participant gate as ab-gate
  ceo->>dir: ticket T-n
  dir->>lead: task: goal, plan item, paths, acceptance, deadline
  lead->>w: task T-n
  Note over w: own worktree tmp/<name>-T-n<br/>commits, tests, mutant, docs, plan
  w->>lead: ready <branch> T-n
  lead->>qa: check <branch> T-n
  qa->>lead: no high | HIGH …
  alt HIGH finding
    lead->>w: finding, fix in the same branch
  else no high
    lead->>gate: <branch> T-n [bump] -- <line>
    Note over gate: rebase on main · smoke-gate lock<br/>--normal, one rerun · bump only if marked<br/>ff-merge main · release lock
    gate->>lead: landed main <commit>, <version> | red | refused
    alt landed
      lead->>w: remove worktree and branch
      lead->>dir: done T-n
      dir->>ceo: done T-n
    else red or refused
      lead->>w: receipt, fix and resend
    end
  end
```

## Roles

| Role | Runs as | Does | Never |
|---|---|---|---|
| `#ab-ceo` | `/fast`, a kitty window rather than tmux | is the owner's single contact; answers at once from its board and status events; writes the ticket, queues it, acknowledges it with its ID; reports results; keeps the list of what waits on the owner | calls down, waits, executes |
| `#ab-dir` | `/fast`, tmux | takes tickets; splits large ones into tasks on disjoint paths; sets deadlines; balances leads; requeues timeouts; settles cross-team conflicts over shared files and order | code |
| `#ab-lead1`, `#ab-lead2` | `/fast`, tmux | assigns a task to a worker by fit; sends the finished branch to QA and to `#ab-gate` together; reads receipts; marks done or failed; returns a red result to the same worker | gates, merges, code fixes, docs, reading logs or diffs |
| `#ab-w<team>a..c` | three per team: Claude on account 1, Claude on account 2 and Codex; the lead gives architecture and refactors to Claude, scoped fixes and tests to Codex; tmux | one task at a time in its own worktree; done means commits, tests, a mutant where [verification](../../CLAUDE.md#mutation-first-then-belief) asks for one, docs, and the plan's TODO and DONE in the same change; runs fast checks only; sends a receipt on done or blocked | the version file, the changelog, merging to main |
| `#ab-qa1`, `#ab-qa2` | z.ai through opencode, a vendor other than every worker's | checks every diff of its team; findings carry file:line and a fix, reproduced first; doc drift is a standard finding; advisory, blocks a merge only at high severity | code |
| `#ab-gate` | a script agent, no model | sole reader of the gate queue; rebases on main, runs the normal gate, bumps the version and adds the changelog line only when the lead marks the landing `bump`, fast-forwards main; the lead sends a branch only after QA reports no high finding; reruns once on a flake; sends a receipt | — |

## Principles

| Principle | Why |
|---|---|
| The `/fast` layer reads receipts only | `/fast` costs more per token; logs, diffs and long reports belong below it |
| Only `#ab-gate` lands on main | removes landing before QA, version slot races and gate lock contention in one place |
| Teams own subsystems | team paths follow the letters in [the subsystem map](../subsystems.md), so tasks rarely collide; for example core, daemon, runner and install in one team, faces, web and agents in the other |
| State lives on the bus | the tasks queue and the board hold it, not a model's context, so closing the `#ab-ceo` window loses nothing |
| Owner-only actions stay the owner's | push, live sends, a daemon restart and widening scope; `#ab-ceo` shows them as a list instead of blocking silently |
| Plans stay canonical | workers update the owning `Plans/<release>/TODO.md` and `DONE.md`; there is no separate team plan file |

## Window directories

Each window is meant to run in its own directory, `local/<bus name without '#'>/` (for example `local/ab-w1a/`), so its
Claude memory is its own and not shared with every other window started from the same directory. `DIR` and the CEO already
have theirs and keep them. The mechanism is node-local, like the rest of `local/`, and the owner's side applies it.

| Step | What it does |
|---|---|
| `python3 local/windows/windows.py` (the dry run) | lists, per window, the new directory, where its notes come from, where they go, and which would be copied, already there, or kept because the target holds a different file; writes nothing |
| `python3 local/windows/windows.py apply --window ab-w1a` | for the named windows only: backs up the launcher config to `local/t13-backup/`, makes the directory, copies the window's notes into its new memory, and points that window's `cwd` at the directory; `--all` names every window |

- **Copy, never move:** the originals stay where they are; a note already in the target is kept, never overwritten; DIR's memory is not touched.
- **One window's own memory:** `apply` writes only under the chosen window's new directory and its memory, and the chosen windows' entries in the config. Which account a window uses decides the memory it copies from: `ab-claude -2` is `~/.claude2`, `ab-claude` is `~/.claude`; the Codex and opencode windows get a directory and no notes.
- **Role files:** a window starts in `local/<name>/`, so the worker, lead and QA role files say so and give their worktree, link and removal commands as absolute `/usr/local/src/ai-agent-bus/` paths; run from `local/<name>/`, `git -C /usr/local/src/ai-agent-bus worktree add /usr/local/src/ai-agent-bus/tmp/<you>-T-n ...` makes the worktree at the repository root and its `node_modules` links resolve.
- **Restart cost:** a window resumes its saved session by its directory, so one moved to a new directory starts without its old transcript; apply it between tasks.
- **Checks:** `python3 -m unittest local/windows/test_windows.py` covers the dry run writing nothing, one window's apply touching no other, the originals staying, the no-overwrite rule and DIR's exemption; each of those has a mutant that fails it.

## Today

The team as it runs on 2026-10-29, as reported by `#claude/ab2`.

<details>
<summary>Sessions, flow and pain points</summary>

| Session | Role in practice |
|---|---|
| `#claude/ab-dvp-b` | manager: turns owner decisions into tasks, assigns, relays owner approvals, deploys some agents, pushes |
| `#claude/ab`, `#claude/ab2`, `#codex/xab`, `#codex/xab2` | implementers; each gates and fast-forwards its own work |
| `#claude/ab2-review` | main reviewer, usually after landing |
| `#opencode/oab` | occasional reviewer |

Tmux session `AB-DVP` holds the windows, kept by `#tmux-sessions/ab-dvp@parf.us`.
There is no task queue: an assignment is a bus message plus the TODO line.

| Pain point | Answer in this design |
|---|---|
| one gate lock, waits of minutes, port collisions | `#ab-gate` is the only gate runner |
| flaky gate, manual reruns | one automatic rerun, reported in the receipt |
| review after merge, one step costing several versions | a high finding blocks the merge |
| version slot races | only `#ab-gate` bumps |
| long free-text reports | fixed receipts upward |
| owner-only actions block without a visible queue | the `#ab-ceo` list |
| session identity leaking into tests | fixed names, unchanged |

</details>
