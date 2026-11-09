# Frozen slow run — red, wrapper-confounded

- Tip c815ac3f1c19d2668e2e9a5c0f2e09de3a0ee9c7; tree c7de8b7d9b1969f1fce3652991107fadef9bd280; both verified unchanged after the run.
- setsid --wait bash -c 'exec src/smoke.sh --slow all'; 2026-11-02 22:21:35–22:23:05 UTC; total90.460s, driver86.330s; rc1;1301passed/4failed. No skipped-section report emitted.
- Retained log tmp/gate-logs/20261102-R1-final-slow.log and .status.json. CLI slow lock was freshly granted as parf, then successfully released immediately after completion/cleanup. Status records no live descendants/background processes; adopted zombies were reaped before return.

Failures:
1. go_checks, guard-escape.test.sh:56: watcher still ran30s. Uses kill-0 at52.
2. supervisor, smoke.sh:3812: orphan bus3451142. gone() at3793–94 tests ps PID existence.
3. mcp_launcher, launchers/smoke.ts:405: all4 runtimes,8 child-gone failures. Predicate403 uses process.kill(pid,0).
4. agent_git, test_autopull_e2e.py:246 OwnersFlow.test_owners_flow_pipeline_then_webhook: host-c.test script127, post-attempt script or directory untrusted. W1C investigates.

My wrapper mistake: .ai-team/ab-w2c/run-final-slow.py:4 enabled PR_SET_CHILD_SUBREAPER but40–42 only monitored, postponing waitpid until54–57 after driver exit. It adopted dead orphans and retained them as zombies. All first3 predicates treat zombies as alive, unlike established plain setsid semantics. This is a confound, not evidence of three product defects. No per-assertion state=Z snapshot was retained, so do not overstate the historical PID evidence. Wrapper is retained for review and must NOT be reused. No retry was attempted; next run needs W1C authorization.

Other evidence: DB72 tests, real tmpfs/implicit-volume/SIGTERM checks green; no fixture container outlives its test. DB/RAG/container/distro journal-quiet lines green. ResourceWarnings remain in raw log, not hidden. No new provider, model, publish, deploy, tag or origin call.
