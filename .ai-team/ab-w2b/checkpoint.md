# W2B checkpoint, 2026-11-06 ~01:20 (context ~78%; lead plans a compact after J112 lands)

## Branches / worktrees (all reported "ready"; nothing in progress)
- **J112** `ab-w2b/J112` 1e659186, worktree `tmp/ab-w2b-j112`: telegram t4.test.ts fake answers getMe itself (3 lines). Proof: without the fix t4 alone failed 3 of 16 and 1 of 8 earlier (Expected 2, Received 3 at t4:122); with it 14 of 14 and once more green. Test only, no bump. Remove worktree + branch after "landed". NO more repeat loops (owner rule 01:20: light on the host).
- **J93F** `ab-w2b/J93F` 0e6fd396, worktree `tmp/ab-w2b-j93f`: release.sh:73 `grep -vxF`. Reported ready. Remove after "landed".
- **F5** `ab-w2b/F5` 13618e63 (branch only, no worktree): overlay.go decodeJSON with UseNumber. Reported ready. Delete the branch after "landed".
- Kept: `tmp/ab-w2b-sasl` (empty dir the PHP tests need).

## Owed
- **F7 comparison:** F7 was dropped from my branch because team 1's G-low (ab-w1b/G-low b29b8525) lands it. My F7 commit is 4268f9c9 (reachable in the reflog; `git cat-file -t 4268f9c9`): shared const `runnerCalls` in cmd/agent-bus-setup/runner_install.go plus test `runner_descr_test.go` that reads the runner's own refusal line in agent-bus-runner/main.go and requires every call in both descriptions. After G-low lands, compare with its test; if mine covers more, send a delta rebased on G-low to #ab-lead1, copy #ab-lead2.
- J112's other findings were filed by the lead as J108/J108b, J113 (runner -race), J114 (tight budgets), J115 (eventlog ordering). None is mine unless assigned.

## Rules in force
- Lock: MCP ab_lock record smoke-gate@parf.us, name slow, ttl short, in its own step; release at once; none during a HOLD ("release" ends it). Never the parf CLI lock.
- Every hand run strips AGENT_BUS_*: env -u AGENT_BUS_SESSION_FILE -u AGENT_BUS_TOKEN -u AGENT_BUS_NAME -u AGENT_BUS_ADDR -u AGENT_BUS_CONTROL_ADDR -u AGENT_BUS_CONTROL_TOKEN; zsh does not word-split a variable, so write the env command out.
- Owner: light on the host: focused tests, no repeat loops, no podman without the lead's OK, no heavy runs.
- Commits: one decision, plain imperative subject, trailer Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>. Never VERSION/CHANGELOG, Plans/issues, main, fetch/push.
- LESSON: always run git from the worktree root (git rev-parse --show-toplevel) and stage by path; a `cd ..` once left me in tmp/ (the main repo): nothing was committed there, but check.
- Reviews: read-only, reply "<id> <sha>: no high / HIGH ...".
