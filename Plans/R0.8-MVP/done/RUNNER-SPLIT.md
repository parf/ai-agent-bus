# The runner split — decisions of 2026-09-11

A design session, not a wave. The runner stopped being part of `agent-busd`
and became its own program, and most of what follows falls out of that one
move. Each row is a **claim**; the section it links to owns the values, and
[decisions](../../../docs/decisions.md#mvp-decisions) indexes every one of them.

Recorded here because the change crosses more documents than any single one of
them owns, and a reader who knows only the end state cannot tell which parts
were argued for.

## What moved

| | Decision | Stated in |
|---|---|---|
| 1 | **The runner is outside the daemon** — its own program under its own account, not a supervised child. So the claim is not "only one child may exec" but *no process the daemon starts may exec at all* | [processes § nothing the daemon runs may exec](../../../docs/11-processes.md#nothing-the-daemon-runs-may-exec) |
| 2 | **Two accounts, one per secret domain**: the daemon holds every credential, the runner holds every configuration, and neither reads the other's | [setup § the two accounts](../../../docs/09-setup.md#the-two-accounts) |
| 3 | **The runner is a service on the bus**, reached by a call like anything else. No second ssh door; who may deploy on a host is that service's ACL | [runner § reaching the runner](../../R1.0-Release/runner.md#reaching-the-runner) |
| 4 | **A service is a checkout; an instance is an env file beside it.** What a service *is* stays externally controlled and holds no secret; what a host *decided* lives apart, so a `git pull` moves no local state | [runner § what an instance is](../../R1.0-Release/runner.md#what-an-instance-is) |
| 5 | **Configuration is three layers overlaid, and the more secret one wins.** Precedence and visibility run in opposite directions | [runner § the three env layers](../../R1.0-Release/runner.md#the-three-env-layers) |
| 6 | **A declared surface does two jobs**: it makes an upload checkable, and it says whether a service needs an instance at all — so there is no second flag to disagree with | [runner § the three env layers](../../R1.0-Release/runner.md#the-three-env-layers) |
| 7 | **The author owns the command line, the host owns whether and how many.** Neither file restates the other | [runner § what an instance is](../../R1.0-Release/runner.md#what-an-instance-is) |
| 8 | **Installed, enabled and running are three states with one home each** — systemd's split, and the reason the runner keeps a list rather than walking a tree | [runner § what an instance is](../../R1.0-Release/runner.md#what-an-instance-is) |
| 9 | **Sandboxing is off by default, opted into per service.** No secret reaches a child as a file, so confinement is hardening rather than what makes the layout correct | [runner § sandboxing](../../../docs/08-runner-role.md#sandboxing) |
| 10 | **A script holds no credential; a linked service holds its own.** The runner asks for one for a name it owns, like anyone else, and may never mint one | [runner § what the child is told](../../R1.0-Release/runner.md#what-the-child-is-told) |
| 11 | **Configuration is write-only** — it goes in and is never handed back. Whoever truly needs the bytes becomes root | [runner § reaching the runner](../../R1.0-Release/runner.md#reaching-the-runner) |
| 12 | **The verb is `start`, never `run`** — one word whether you sit in front of it or the runner does it for you | [runner § what the runner does](../../R1.0-Release/runner.md#what-the-runner-does) |
| 13 | **A bus that is away is not a service that failed.** The client reconnects; the runner restarts nothing | [runner § where it runs](../../R1.0-Release/runner.md#where-it-runs) |

## What each stage gets

The split is **not** MVP work. What the MVP already ships is the thing the
runner wraps ([stages § MVP](../README.md#scope)); the runner itself
is [stages § R1](../../R1.0-Release/README.md#scope) and is planned in
[Plans/R1.0-Release](../../R1.0-Release/TODO.md#todo-r1).

| | |
|---|---|
| MVP | one command line publishes a service in the foreground, confined if it asks to be. The install lays out both accounts and the tree they own; no runner program and no installed state |
| R1 | the runner as its own account and program: installed instances, write-only configuration, autostart, restart policy, on-demand start |

## What it cost the MVP

Three code changes, because a decision that only reaches the docs is a decision
the code will contradict later. Each was watched failing before it was
believed, per [PoC § mutation first, then belief](../../../CLAUDE.md#mutation-first-then-belief):

| | |
|---|---|
| the daemon's account was renamed, so the account and the CLI are no longer the same word | mutant: the old name restored — two named checks red |
| an unset sandbox setting means off, like an explicit one, and `Pick` lost its best-effort branch | mutant: the best-effort branch restored — the named check red |
| the installer makes **both** accounts and the three directories, each its owner's at the mode the design gives it | four mutants: the mode on each home, the world-readable one, and the second account never created — one named check red apiece |

## What was superseded

Rows moved to [decisions § superseded](decisions-before-rewrite.md#superseded)
in the course of this, each listed there with what replaced it. The pattern is
the same throughout: **an arrangement that needed a rule to be safe was
replaced by one that does not need the rule.**

## Still open

| ❓ | Where it waits |
|---|---|
| how a dormant name is woken, and what the daemon has to learn to do it | [runner § what an instance is](../../R1.0-Release/runner.md#what-an-instance-is) — R1 |
| whether one kept child may have several messages in flight | [runner § long-lived services](../../R1.0-Release/runner.md#long-lived-services) — R1 |

The environment a child is told about is **deliberately not closed** — it is
what the child is serving rather than who it is, and it will grow when
services are actually being written.
