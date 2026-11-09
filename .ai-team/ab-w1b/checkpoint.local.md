# #ab-w1b checkpoint (2026-11-05 ~21:50, written for a compact before T-38 S1)

Role: worker 1b, team 1, lead #ab-lead1 (Plans/Team.v2/roles/worker1.md and common.md apply: hand-off checklist, area affinity, one task at a time). Rules: CLAUDE.md, CLAUDE.local.md. Worktrees under tmp/, stage by path, one decision per commit with `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, never main/push/fetch, no VERSION/CHANGELOG, no Plans/issues edits on code branches. Gate lock: MCP `ab_lock smoke-gate@parf.us slow 15m` in its own call, read the answer, mutants/focused runs only, kill burners before release (J87), `ab_lock_release`; if "held by X" wait (Monitor on `agent-bus holders`). Run shell loops in bash, not zsh (no word splitting). Run gate-like things with setsid, never nohup. Run focused tests from my window with `env -u AGENT_BUS_TITLE_OWNER` / `-u AGENT_BUS_SESSION_FILE` (my window is an ab-* launcher session).

Areas I know: agent-bus CLI and enrol, smoke harness (SMOKE_CPU_PERCENT default off, Go caps, shared client fixture), agent_guide shard and the three worked pages, client conformance vectors/examples (Java, PHP, Go), setup/install units, launcher tab titles (read), event-log/subscription reads.

## Open branches (all on main as of their last rebase; none merged by me)

| Branch | Worktree | Last commit | State |
|---|---|---|---|
| ab-w1b/G3 | tmp/ab-w1b-G3 (node_modules links removed) | 4f364d01 | ready since 21:10, reported; W1C reviewed (asked for mutant commands, I sent them). A managed release's runner unit ExecStart → installRoot/current/agent-bus-runner; mutants red. Waiting for the gate / landing. Shipped behaviour changes (bump-worthy). |
| ab-w1b/G-low | tmp/ab-w1b-Glow | b29b8525 (9 commits) | ready, W1C found 3 MEDs, fixed (list-units failure, atomic logrotate install, only FileNotFoundError skipped), re-reported "ready" at 21:35; both readers re-read pending, QA1 queued 4933623c (lead redirects). G2 skipped by agreement (7 tests stub systemctl via PATH). F7, G5, G16, G4 done with mutants. |
| ab-w1b/G-flags | tmp/ab-w1b-Gflags (node_modules linked) | d82b73ec | ready 21:43. Setup already writes canonical flags; test of whole ExecStart lines + `unitChanged()` + old single-dash unit rewrite; 7 mutants red. standalone: none. No bump. Waiting for readers. |
| ab-w1b/T-38-S0 | tmp/ab-w1b-S0 | 0f2c0be7 (5 commits; last: soft delete "Planned for S3") | ready (fixes for W1C's MED/2 LOW sent 21:55: soft delete only in S3, key diff deferred; Q315 rule stated; DECISIONS rows are names+links); docs only (Plans/R1.1 filescan.md, dbsync.md, TODO.md, DECISIONS.md); W1C re-reads; lead accepted the Q306 reconciliation (inline; bigfile above the limit for an opted-in subscriber; else size+hash; `as rag` skips and says so; never truncated). |

Landed this session (worktrees removed): T-32c, T-32d, T-31f, J88, B1, guide-unset (eea2cf6f).

## Reads in flight / done for others
- J-batch 61e5a07f delta: verdict sent (no high, LOW: `-h` alone still refuses); mutants red; tmp/ab-w1b-rJ removed.
- T-35 (6245a439, 65a15c8a) and TRIAGE b056d9da, J-batch f18cd418, G3-review: verdicts sent.

## Next
1. Wait for W1C's re-read of S0 (77fbbf1d); then S1 when lead confirms.
2. Wait for the compact, then T-38 S1 (the shared core library: state file, subscriptions with authority, delivery with confirm/resend, poll timer) once lead confirms after S0 lands; S3 (DBSync) moves to W1A if my weekly is under 40% (7d was 89% left, 5h 49% at 21:45). Plans: Plans/R1.1/filescan.md (#stages), dbsync.md, TODO FS.1/DS.1, resolved.md Q305–Q320 (Q307 replay by default, Q308 system paths, Q319 MySQL/PG first).
3. Land cleanup after each "landed": for each branch `for m in src/mcp src/web src/clients/typescript; do rm -f tmp/<wt>/$m/node_modules; done; git worktree remove --force tmp/<wt>; git branch -D <branch>`.
4. tmp/go-cache-w1b and any tmp/agent-guide.* of mine may still exist; remove at close-out.

## Facts worth keeping
- smoke knobs: SMOKE_CPU_PERCENT (default off; N wraps once in a systemd user scope; inner systemd-run scopes escape it), SMOKE_GO_PROCS / SMOKE_GO_P, CARGO_BUILD_JOBS, SMOKE_CLIENT_FIXTURE (built once by the driver).
- git-install.sh now restarts active root-account runners (git-install-runners.sh) and installs the tracked logrotate rule (git-install-logrotate.sh) — on G-low, not yet landed.
- Mutants must compile and fail on a named assertion; restore single files; commit before mutating.


## UPDATE 2026-11-05 ~23:00 (T-38 S1/S2)
- Landed: T-38 S0 (d19f3589); worktree/branch removed. T-37 review sent (no high).
- S1: branch ab-w1b/T-38-S1, worktree tmp/ab-w1b-S1, tip 2e82b7b3 (src/agents/shared/change_sync.py + test_change_sync.py, 56 tests). W1A found 4 LOWs, all fixed (a3e2cd59 code, a7eb1ab9 tests) + 2e82b7b3 (pending action create/change/delete). TODO: run the mutants under my MCP lock (script scratchpad mut.py: python3 mut.py [names]; needs lock smoke-gate@parf.us slow; waiting, held by parf/w2a), then report "ready ab-w1b/T-38-S1 <sha>" (W1A re-reads delta, W1C reads whole).
- S2: branch ab-w1b/T-38-S2 worktree tmp/ab-w1b-S2, based on S1 tip (rebased); F0 done (needs.dir runner bind, bind_dir.go + tests + docs; checkpoint F0 sent). Do NOT send S2 ready until S1 landed; then `git rebase main`.
- S2 left: F1 agent core (src/agents/filescan/: agent.json, filescan.py, scan.py; kept process, bus.serve(handler,workers,stop) + PollTimer, deliver via change_sync with body builder), F2 inotify (ctypes), F3 mass-delete guard + `as rag`, F4 smoke lines + docs/agents/filescan.md + token-handoff deploy step; wire registries like eventlog (src/agents/package.sh, docs_order_test.py, setup package_install, smoke-map, smoke.sh, acceptance/agent-runtime.py); mutants for F0 (bind_dir: forged env, root/home/state refusals, ro-bind) and S2.
- Facts: client `bus.call` conflicts with raw consume -> use bus.serve; reply ok = {"status":"ok"}; timers: ROUND_MAX 8 < HOLD_WAIT 10.


## UPDATE ~23:35 (quota 5h 26% left at 22:33, resets ~23:50)
- S1 ab-w1b/T-38-S1 tip 2e82b7b3: 35 mutants red (run, lock released). W1A delta: no high (2 new LOWs to fold into S2, see below). Waiting W1C second read (~22:50). Do not change S1 sha.
- S2 ab-w1b/T-38-S2 (worktree tmp/ab-w1b-S2, on S1 tip): commits F0 (needs.dir; 15 Go mutants RED via scratchpad mutgo.py), F1 agent (src/agents/filescan: settings.py scan.py path_filter.py filescan.py + tests, 47 tests), F3 guard + rag form (committed e2c2ae88). UNCOMMITTED: scan.py poll_paths/stat_under refactor (tests pass 18), watch.py (inotify ctypes, tested by hand only).
- S2 TODO: (a) tests for watch.py + poll_paths; wire Watcher into FileScan (modes poll/watch/both, effective_mode in status, watch-limit fallback to timer, scan_paths with guard, overflow -> full scan); (b) lead's LOW-A: deadline check before each send after _admit (docstring: only waits are bounded) and LOW-B: save per subscriber only when something was confirmed, keep sent/dropped counters in memory until the round's final save (change_sync.py, with tests) — in S2 not S1; (c) reset loses pending deletes: say so in agent doc; (d) F4: agent.json (methods, config, needs.dir true), env.dist, config.json.dist, package.sh, help.golden.json, register like eventlog (src/agents/package.sh, docs_order_test.py, setup package_install.go + 3 tests, acceptance/agent-runtime.py, smoke-map/smoke.sh shard agent_filescan), docs/agents/filescan.md (+ token-handoff deploy step, roots in agents.json, reset note, rename = delete+create no `renamed` pairing unless built), Plans TODO FS.1 status; (e) mutants for S2 python parts (symlink never followed, guard, rag skip, watch overflow...) under lock; (f) README/DECISIONS rows done for roots. Rename pairing ('renamed' change) NOT built — report as a gap.
- Do not send S2 ready until S1 landed (then git rebase main).


## UPDATE ~23:55 (T-38)
- S1 ab-w1b/T-38-S1 tip 3dca3e30 reported ready (after MEDs from W1A/W1C: maybe record, restore unattempted, '?' marker, docstring limits); readers: W1A+W1C delta reads pending; 43 mutants red. S1 NOT landed yet -> S2 not ready-able.
- S2 ab-w1b/T-38-S2 (worktree tmp/ab-w1b-S2) is rebased on S1 c196c32c (NOT 3dca3e30): need ONE more `git rebase ab-w1b/T-38-S1` (change_sync.py conflicts in deliver: keep S1 logic + S2 skip(None) branch with _restore + `or count['skipped'] or restored` save condition + dirty/LOW-B tail) once S1 settles, and then `git rebase main` after S1 lands.
- S2 done+committed: F0 (needs.dir, roots, 15 Go mutants red), F1 agent, F2 watch (watch.py, poll_paths; NO rename pairing), F3 guard+rag form, package (agent.json, help.golden, package.sh), docs/agents/filescan.md + README + docs_order, setup lists (+shared change_sync) + acceptance, smoke shard agent_filescan.
- S2 left: (1) Plans TODO FS.1 / filescan.md status lines; (2) S2 python mutants under lock (list ideas: symlink followed (O_NOFOLLOW removed / dir link not pruned), guard thresholds, rag skip, rag id hash, watch overflow, poll_paths gone-dirs, settle, roles gate, reset manager, confirm manager); (3) run full filescan tests + shared tests; (4) after S1 lands rebase main; then report "ready ab-w1b/T-38-S2 <sha>" with diff stat, F0 TOCTOU window statement: re-resolve at launch (3 sites), NOT fd-bound; window = path swap between check and bwrap start needs write access to a parent dir of dir inside an owner root (account that owns the folder); what bind then gets = whatever the swapped path names, BUT the launch-time bindDir re-check refuses anything outside roots / host dirs at the moment of check; residual race microseconds; a system dir/outside roots cannot be reached unless swapped in that window by someone with write access under a root.


## UPDATE ~00:05 (after S2 ready)
- S1 landed (5337290a), worktree/branch removed. S2 ready: ab-w1b/T-38-S2 3f00ba06 (worktree tmp/ab-w1b-S2, on main 5337290a), with W1C and W1A reading.
- Waiting for the lead's "released" (HOLD on smoke-gate lock for J102). Then I get the FIRST lock slot: re-run the Go F0 mutants (scratchpad mutgo.py, ~8 min, full set; the helper was swapped to `within`), then release. The Python mutants (mut2.py, 31) already ran green.
- If readers rate the F0 residual window high: add fd-bind (--ro-bind-fd) or a post-mount source check in internal/sandbox Wrap; fixes go into the same branch, re-report "ready".
- After "landed": rm node_modules symlinks, `git worktree remove --force tmp/ab-w1b-S2`, `git branch -D ab-w1b/T-38-S2`; also G3, G-low, G-flags, tmp/go-cache-w1b cleanup still open.
- Next planned: S3 DBSync (may move to W1A if my weekly <40%).

## UPDATE ~23:20 (after W1C's read of S2 98f58d9c: no high, 2 MED + LOW; do after quota reset ~23:50)
- MED1 false delete: event dir that is unreadable -> walk yields ('error','',EACCES), poll_paths marks only '' present and deletes its tracked child. Fix: unreadable event dir withholds every delete under its full path (scan.py walk/poll_paths). Test: chmod 000 one/blocked, poll_paths(['one/blocked/']) -> deletes==[]. Repro probe: .ai-team/ab-w1c/s2-delta-review.local.py.
- MED2: basename withholding blocks same-named dirs forever. Replace os.fwalk+onerror by a walker that knows each parent's relative path (record full dir-relative path), withhold exactly that subtree; delete is_blocked basename rule.
- LOW: "excluded dir never read" test must prove non-traversal (spy on os.open/scandir), and restore mutant 'excluded event directory still read' (mut2.py) instead of calling it equivalent.
- One red test + one mutant per fix (lock: own MCP, `slow`, 15m; rule: ask via lead if holds). Then "ready ab-w1b/T-38-S2 <sha>" + diff stat. Worktree tmp/ab-w1b-S2 (clean at 98f58d9c). Scripts: scratchpad mut2.py (python), mutgo.py (go).
