# Web resource confinement

📌 **TL;DR:** G.1.2 limits web separately; exhausting it restarts web while the bus keeps serving.

## Scope

The generated system unit delegates CPU, memory and task controllers to the
supervisor. The supervised dashboard and its bubblewrap descendants enter one
web-only cgroup before executing, limited to 256 MiB memory, no swap, 64 tasks
and one CPU. The supervisor and bus remain outside it.

Missing or unusable delegation leaves web down with an operator diagnostic;
the bus still serves. Restarts reuse the same emptied cgroup, and supervisor
shutdown removes it. The current contract is in [processes](../../../docs/11-processes.md#web-resource-limits).

## Checks

`src/acceptance/web-resources.py` creates a disposable real systemd unit from
`agent-bus-setup --print-unit`, with separate state, sockets, credentials and
web port. It runs the actual supervisor, bus, static web binary and bubblewrap.
A bounded unprivileged helper is placed in the measured web cgroup to exercise
the cgroup policy; this is not a claim that a particular HTTP request can force
the renderer to allocate those resources.

| Pressure or failure | Observed result |
|---|---|
| CPU | `cpu.stat` records throttling; authenticated bus calls continue; the real page still renders |
| Tasks | `pids.events` records refusal; authenticated bus calls continue; the real page still renders |
| Memory | `memory.events` records group OOM; the real web PID is replaced under the same limits after logged backoff; the bus survives |
| Repeated restart | One cgroup is reused and then removed on shutdown; no directory accumulates per restart |
| Missing delegation | No web process starts; the bus answers and the log names `Delegate=` and `DelegateSubgroup=` recovery |

The helper's `/proc/<pid>/cgroup` is checked before each pressure phase so an
event in another group cannot satisfy the result. The renderer must return a
real sign-in page after every phase; a dead web process is not a positive
control.

The first installed run found a kernel race in the harnessed implementation:
writing `cgroup.kill` against an already empty group immediately before
`clone3` could kill the arriving wrapper. The final implementation checks
`populated` first. A second run found the acceptance reader lacked permission
to inspect delegated files and was corrected to read them through the fixture's
root driver. Both red runs are retained under `tmp/web-resources/`.

Final disposable acceptance: **25 checks, zero failures** in
`tmp/web-resources/installed-final.log`.

## Mutations

The installed mutation runner builds isolated source copies and fresh
disposable units. The final run caught all **8 of 8** overlays in
`tmp/web-resources/mutations-final-4.log`:

| Change | Named failure |
|---|---|
| Remove memory, CPU or task limit separately | Configured limit readback |
| Start the wrapper outside the web cgroup | Web is separate from bus and supervisor |
| Continue after delegation discovery fails | Missing delegation starts no renderer |
| Remove `DelegateSubgroup=supervisor` | Real web never becomes ready |
| Kill an already empty group before `clone3` | Real web never becomes ready |
| Replace the real web binary with a process that immediately exits | Real renderer positive control |

The first mutation run used a dynamically linked copied web binary, which the
minimal sandbox correctly could not execute. A later run had one malformed
fallback mutant that crashed before demonstrating an unlimited launch, and the
next caught the corrected mutant at an earlier diagnostic assertion than its
runner expected. Those runs are retained and receive no mutation credit; the
final run receives the mutation credit.

Frozen `--slow`: **589 passed, 0 failed**, with vet and race green
(`tmp/web-resources/slow-final.log:936`, lines 10–11). Every source hash in
`tmp/web-resources/source-frozen.sha256` remained unchanged through the run.
Tracked `src/smoke.sh` and its frozen copy shared SHA-256
`e401546193343b037b16df4c5e408d1f208aa35f6cbe8180a64274a0e6e23b21`.

## Live postflight

Commit `cca46ec` was pushed before deployment. The generated base unit replaced
the prior non-delegating unit, systemd was reloaded, and the daemon restarted
once on 0.5.49. Read-only checks found:

- `Delegate=yes` with CPU, memory and task controllers;
- supervisor and bus together in the named `supervisor` subgroup;
- web alone in one sibling `web-*` cgroup with the exact five configured values;
- the real sign-in page and mapped-user status call both serving; and
- the existing OpenCode session reconnecting without manual registration.

No pressure, OOM, task storm or production-state write was used for postflight.

## Limits

This host exercise uses its installed systemd, cgroup v2 and service account;
it is not fresh-host packaging acceptance. The outer fixture unit carries a
separate 768 MiB safety cap, above the helper's bounded allocation, so a missing
web limit cannot consume the host and cannot masquerade as the web limit.

Production postflight is read-only. No pressure helper, OOM or task storm is
run against the live daemon. A direct `agent-busd -web` launch deliberately
leaves web unavailable unless it is in the documented delegated construction.
