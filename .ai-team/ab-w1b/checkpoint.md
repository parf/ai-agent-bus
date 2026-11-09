# #ab-w1b checkpoint, 2026-11-01 ~16:42

Rules: CLAUDE.md, CLAUDE.local.md. Locks: unit mutants/focused runs take smoke-gate@parf.us `slow` with the MCP ab_lock tool under my own name (own call, read the answer), release after and tell waiters; CLI lock as parf ONLY for a real smoke.sh normal/slow run, then release at once (rule until J79). Never touch a lock "you already hold" that I did not just take. HOLD from lead = no mutants/gates/podman/lock until "release". No git fetch/push; stage by path; commit before a mutant; never edit VERSION/CHANGELOG; "ready <branch> <sha>". Run client/mcp/web tests with AGENT_BUS_* unset (web tests from src/web). New owner questions: Plans/MARKUP.md item form (never table rows).

## Landed
V1/D1, E1, E1b, T-26(+readme), T-24-close, C1-mcp, T-27, RAG-help, MAC.2-close.

## With readers / gate
- RT11 `ab-w1b/RT11` (tmp/ab-w1b-RT11) tip d09037b2: with W1C+W2C. After "landed": remove worktree + branch.
- docs-pass `ab-w1b/docs-pass` (tmp/ab-w1b-docs) tip 25531cfd: with W2B. Its ✅ shas for N1/N3/N4/O1 are branch shas (5cfd75dc 8c9bb30f 9e0df7b5 852764ff): if the gate rebases, send a one-line follow-up with landed shas. After landed: remove worktree + branch.

## Reads sent (nothing pending): S2v, A2, F3, F3-slack, J78.

## Queue
- J74 follow-up done by W1C (J76). Nothing else queued; next task from #ab-lead1.
