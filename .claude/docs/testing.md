# Testing and the gate

📌 **TL;DR:** How `src/smoke.sh` picks and runs checks, the gate lock, and how a check proves it can fail. The every-session summary is [CLAUDE.md § verification](../../CLAUDE.md#verification); this page holds the detail.

## Gate sets

| Run | Does | Landing gate |
|---|---|---|
| `--normal` | normal checks for changed subsystems, fast checks for every other subsystem, in parallel; target under 60 seconds, two-minute ceiling | yes |
| no flag (alias `--fast`) | fast checks for touched subsystems | NOT A LANDING GATE |
| bare `--slow` | derives the changed subsystems for a huge change, runs those at slow and the rest at fast | yes |
| `--slow all` | every subsystem at slow; a release or STABLE commit uses it | yes |
| `--normal A,F`, `--slow L` | explicit overrides | yes |

- Release packaging runs only in the release run: the checks in `src/smoke-map.json` `release` (archives, cross-builds, the container image, distro installs) are selected only by `--slow all`.
- Fast checks use explicit essential-operation allowlists in that map; they can overlap normal.
- A fast candidate over 15 seconds fails with a report and must be narrowed, never given a bigger budget.
- Normal retains its regression and security checks; slow-set moves require proving the same production mutant fails a faster normal check.
- Documentation-only changes need path/anchor and scope/status checks (`src/check-links.py` reads every link in the changed files and every link, in any file, to a path the change moved or deleted), without a version bump or runtime gate solely for prose.

## Selection

- Selection uses the worktree diff against main, including staged and untracked files. In the main checkout on main this is the uncommitted pre-commit diff; a clean or ignored-only tree prints "nothing changed: all fast" and "NOT A LANDING GATE".
- It uses the path letters in [the subsystem map](../../Plans/subsystems.md), and reverse dependencies in `src/smoke-map.json`.
- Shared test/build dependencies or unmapped paths promote every subsystem to the requested set.
- It always ignores `src/internal/version/VERSION`, `src/agents/VERSION`, `src/agents/CHANGELOG.md` and root `CHANGELOG.*.md`, whether generated or manual.
- The output names the set and reason for each subsystem.

### Agent shards

The agent shards live in `src/agents/smoke.sh`, and each agent names the shared files it uses:

| Diff | Agent checks |
|---|---|
| Only inside `src/agents/`, with prose at most | The checks that own a changed path, plus `agent_pack`, the pack contract; every other check is skipped. An agent file no check owns runs every agent check |
| Any other | Like every subsystem: an agent shard tagged with a changed letter, or a reverse dependency of one, runs at the requested set, the rest at fast |

## Quarantine

A check that goes green only on the gate's rerun is quarantined in the `quarantine` key of `src/smoke-map.json`, by Go test name or assertion title, with its open J-item; a ⚠️ test is filed for its lead instead.

- Fast and normal skip an entry and print a counted `QUARANTINED` line; slow runs it.
- go_checks fails an entry whose J is closed, so the fix removes it.
- `src/smoke-quarantine.py` owns these rules.

## Host limits

- Normal and fast runs write nothing to the host journal or syslog; systemd's own one-line scope notice for a `systemd-run --scope` the check starts is not our output and is allowed (Q327).
- Smoke caps Go at `GOMAXPROCS=6` and `-p 4` and cargo at 4 jobs, and builds the client fixture once; `SMOKE_CPU_PERCENT=<n>` adds an optional whole-run CPU quota (default off).
- A normal over all subsystems measured 95 s at peak load 21 on 24 cores (2026-11-02).
- Normal and fast fixtures must not provoke kernel OOM kills or PID-controller rejections. Read the configured limits from a live agent’s cgroup in normal; deliberate over-limit enforcement belongs only to slow.

## Who runs what

- Claude workers run `smoke.sh` from their session shell as-is, keeping `AGENT_BUS_SESSION_FILE`: it checks their identity before clearing the session itself.
- Codex shells stay identity-less: Codex workers take required locks through MCP under their own bus name. Hand-run focused test commands keep `env -u AGENT_BUS_SESSION_FILE -u AGENT_BUS_TOKEN`.

## The gate lock

- Before any normal, `--slow`, or mutant gate, take the `slow` bus lock on `smoke-gate@parf.us` with a 15-minute ttl, extend before expiry if needed, and release it afterward.
- A hand-run suite or mutant run beyond one focused test takes that lock under the worker's own bus name; focused non-mutant checks need no lock.
- This resource record (`lock://smoke-gate`) is owned by `parf`; `#ab-gate` takes it as `parf` for every landing, and its Maintainers are the workers `#ab-w1a..c` and `#ab-w2a..c` for their mutant runs ([Team v2](../../Plans/Team.v2/README.md#roles)). If absent, its Owner registers it and grants those Maintainers before gates run.
- Normal and slow smoke runs, including hand-run shards, refuse when the caller does not hold that lock. Marked children reuse their driver's check. This prevents accidental unlocked runs; it is not a security boundary: deliberately exporting `SMOKE_CHILD_SHARD`, `SMOKE_CHILD_PLAN` and `SMOKE_PLAN` for an existing plan file can bypass it.
- No agent holds or reserves the lock for a chain of future gates. Release it before deployment. A small fix gets the next slot after the active gate.

## Mutation first, then belief

**A green check is evidence of nothing until it has been seen to fail**, and a mutant credited by a build failure takes no credit — it must compile and fail on an assertion.

| Change | Proof it can fail |
|---|---|
| A new check, or a fix in a ⚠️ subsystem of [the subsystem map](../../Plans/subsystems.md) (authority, tokens, storage, parsers, runner, install) | a mutant: break the fix, watch the named check fail on an assertion, restore |
| Any other behaviour fix | the check fails on the code before the fix and passes after; no mutant |
| Docs, text or Configuration only | none |

A mutant runs only the test that owns its check, never a whole gate; one normal gate on the final tree follows.

### Shapes found here

Every shape below was actually found in this repository:

| Shape | The check that had it |
|---|---|
| taking the sender's word for a delivery | `ab_reply` said it replied; the routing was nonsense |
| counting nothing | a doubled delivery passed a check that only looked at the last one |
| grepping output that is non-empty either way | `$(cmd; echo -n nothing)` contains *nothing* whatever `cmd` did |
| grepping a word the success answer also contains | a refused receipt and an accepted one both say `receipt` |
| exercising a different path than the one it names | a *waiting reader* check whose inbox still held a message never blocked, so it measured the queued path twice |
| leaning on the refusal to end the command | `agent-bus run` exits when refused and runs forever when not, so removing the refusal hung the check instead of failing it — bound anything whose success does not return |
| signalling the wrong process | `f() { ...; } &` backgrounds a subshell, so the service never got the signal |
| asking a dead process what it left behind | an orphan is reparented to init the moment its parent dies, so `pgrep -P <that parent>` is empty however badly it left — take the pids *before* the kill |
| matching a class name in the inline stylesheet | every page carries the stylesheet, so assert the element form |
| asserting an identity is absent from a listing | the signed-in name is in the page header, so assert the listing's own row |
| a refusal aimed at a subject that does not exist | `/subscribe` was refused for the *caller* being unregistered, so the check passed whichever field the daemon read |
| a presence check against a paginated listing | the listing is paginated, so it passed or failed on where the name sorted — narrow it with `?q=<name>` |

**Naming a shape does not remove it.** Sweep for repeats as part of the fix, not as a later tidy. A mutant that survives because its check was skipped or filtered out is the worst kind of green: name the test that failed.
