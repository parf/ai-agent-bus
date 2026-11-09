# #ab-w1a start notes

Read at every start, after .ai-team/worker1.md.

## The lock comes first

Before any mutant, load, timing or repeat run, take the smoke-gate lock:
`agent-bus lock smoke-gate@parf.us slow --ttl=900s --wait=60s`, looped until held.

- This includes cold-cache Go builds, measuring wall or CPU time, running a test several times, guard.test.sh and anything with podman.
- Extend it before the 15 minutes run out (`agent-bus extend smoke-gate@parf.us slow --ttl=900s`). A run that outlives the ttl ran unlocked, and its evidence does not count.
- Release it right after the last run (`agent-bus release smoke-gate@parf.us slow`). Never hold it across reading or editing.
- A script that waits for the lock must stop when it does not get it: `got=; for i in $(seq 9); do agent-bus lock … && { got=1; break; }; done; [ -n "$got" ] || exit 1`. A wait loop that falls through runs unlocked (15:10, J45).
- Note the start and end clock times of every locked run, so an overlap question gets an exact answer.

Why: on 2026-10-30 three of my runs were unlocked: a lapsed ttl (J35), a guard.test.sh run (J36) and cold-cache timing runs inside the B27a gate (J45). The gate is CPU-bound, so its timing probes go red beside extra load.

## J47 parked (2026-10-30 15:17, for B.27m)

Branch ab-w1a/J47, worktree tmp/ab-w1a-J47, no commits yet. Findings:
- Check before smoke.sh's AGENT_BUS_* unset loop: the caller's env decides identity.
- Caller: `agent-bus status` JSON field `you` (#ab-w1a in a session; `parf` with the session vars unset, which is the gate's identity).
- Holder: `agent-bus holders smoke-gate@parf.us` → `smoke-gate@parf.us slow <holder> Nm left`, or "holds no locks".
- Only --normal/--slow, only the driver (empty SHARD); smoke-options.sh sets MODE/SHARD.
