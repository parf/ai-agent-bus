# Team v2

📌 **TL;DR:** An idea, not decided: a layered AI team for this repository. The
owner talks only to `#ceo`. `#dir` splits tickets, two leads route work to
workers, QA checks every diff, and a script agent `#gate` alone lands on
main. The coordinating layer runs on `/fast` and reads only short receipts. The
heavy work runs below it on ordinary models.

## Status

Idea from the owner, 2026-10-29; partly running from 2026-10-30 with one team
([decision](DECISIONS.md#recorded-decisions)). Node-local, not in Git, under
`local/team-v2/`: the role prompts, `start-ceo.sh` for the `#ceo` kitty window,
`gate.sh` behind `#gate`, and the `#tmux-sessions/ab-dvp@parf.us`
Configuration that starts `DIR`, `LEAD1`, `W1A..C`, `QA1` and `GATE`. The
team before it is in [today's team](#today). Open choices are in
[questions](QUESTIONS.md#open-questions).

## Shape

One team runs today ([TD1](DECISIONS.md#recorded-decisions)); a second adds
`#ab-lead2`, `#ab-w2a..c` and `#ab-qa2` beside it. Names carry the project
prefix `ab-` ([TD2](DECISIONS.md#recorded-decisions)). Two sessions of one
account never share a directory, so each resumes its own conversation.

```
 parf (owner)
   │  talks only to
   ▼
 #ab-ceo ─────────── kitty window "CEO" · Claude /fast · local/team-v2/
   │  ticket T-n · "waiting on owner" list · results back to parf
   ▼
 #ab-dir ─────────── tmux DIR · Claude /fast · repo root
   │  task: goal, plan item, paths, acceptance, deadline
   ▼
 #ab-lead1 ───────── tmux LEAD1 · Claude /fast · Plans/
   │  assigns by fit · reads receipts only · decides "bump"
   ├──────────────────┬──────────────────┐
   ▼                  ▼                  ▼
 #ab-w1a            #ab-w1b            #ab-w1c          one task each, own worktree tmp/<name>-T-n
 tmux W1A           tmux W1B           tmux W1C
 Claude (account 2) Codex              opencode / z.ai
 repo root          repo root          repo root

 #ab-qa1 ─────────── tmux QA1 · Claude (account 2) · Plans/ · read-only
 #ab-gate ────────── tmux GATE · script local/team-v2/gate.sh · no model · sole lander on main

 #tmux-sessions/ab-dvp@parf.us keeps every tmux window; #ab-ceo starts and restarts them through it
```

A task's path, each arrow one bus message:

```
 #ab-ceo ──T-n──► #ab-dir ──task──► #ab-lead1 ──task──► #ab-w1x
                                        ▲                  │ "ready <branch> T-n"
                                        └──────────────────┘
 #ab-lead1 ──"check <branch> T-n"──► #ab-qa1 ──"no high" | "HIGH …"──► #ab-lead1
      HIGH ──► back to the same worker

 #ab-lead1 ──"<branch> T-n [bump] -- <line>"──► #ab-gate
      #ab-gate: rebase on main → smoke-gate lock → src/smoke.sh --normal (one rerun)
                → if bump: PATCH + CHANGELOG → ff-merge main → release lock
      "landed: main <commit>, <version>" | "red: …" | "refused: …" ──► #ab-lead1
      red/refused ──► back to the same worker
      landed ──► worker removes worktree and branch; "done T-n" ──► #ab-dir ──► #ab-ceo ──► parf
```

## Roles

| Role | Runs as | Does | Never |
|---|---|---|---|
| `#ceo` | `/fast`, a kitty window rather than tmux | is the owner's single contact; answers at once from its board and status events; writes the ticket, queues it, acknowledges it with its ID; reports results; keeps the list of what waits on the owner | calls down, waits, executes |
| `#dir` | `/fast`, tmux | takes tickets; splits large ones into tasks on disjoint paths; sets deadlines; balances leads; requeues timeouts; settles cross-team conflicts over shared files and order | code |
| `#lead1`, `#lead2` | `/fast`, tmux | assigns a task to a worker by fit; sends the finished branch to QA and to `#gate` together; reads receipts; marks done or failed; returns a red result to the same worker | gates, merges, code fixes, docs, reading logs or diffs |
| `#w<team>a..c` | ordinary models by task type: Claude for architecture and refactors, Codex for scoped fixes, z.ai for tests and boilerplate; tmux | one task at a time in its own worktree; done means commits, tests, a mutant where [verification](../../CLAUDE.md#mutation-first-then-belief) asks for one, docs, and the plan's TODO and DONE in the same change; runs fast checks only; sends a receipt on done or blocked | the version file, the changelog, merging to main |
| `#qa1`, `#qa2` | a vendor other than the author's | checks every diff of its team; findings carry file:line and a fix, reproduced first; doc drift is a standard finding; advisory, blocks a merge only at high severity | code |
| `#gate` | a script agent, no model | sole reader of the gate queue; rebases on main, runs the normal gate, bumps the version and adds the changelog line only when the lead marks the landing `bump`, fast-forwards main; the lead sends a branch only after QA reports no high finding; reruns once on a flake; sends a receipt | — |

## Principles

| Principle | Why |
|---|---|
| The `/fast` layer reads receipts only | `/fast` costs more per token; logs, diffs and long reports belong below it |
| Only `#gate` lands on main | removes landing before QA, version slot races and gate lock contention in one place |
| Teams own subsystems | team paths follow the letters in [the subsystem map](../subsystems.md), so tasks rarely collide; for example core, daemon, runner and install in one team, faces, web and agents in the other |
| State lives on the bus | the tasks queue and the board hold it, not a model's context, so closing the `#ceo` window loses nothing |
| Owner-only actions stay the owner's | push, live sends, a daemon restart and widening scope; `#ceo` shows them as a list instead of blocking silently |
| Plans stay canonical | workers update the owning `Plans/<release>/TODO.md` and `DONE.md`; there is no separate team plan file |

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
| one gate lock, waits of minutes, port collisions | `#gate` is the only gate runner |
| flaky gate, manual reruns | one automatic rerun, reported in the receipt |
| review after merge, one step costing several versions | a high finding blocks the merge |
| version slot races | only `#gate` bumps |
| long free-text reports | fixed receipts upward |
| owner-only actions block without a visible queue | the `#ceo` list |
| session identity leaking into tests | fixed names, unchanged |

</details>
