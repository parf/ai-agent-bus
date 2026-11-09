# #ab-w2c checkpoint — runner chain, then J50, then RT24

Read `/usr/local/src/ai-agent-bus/.ai-team/worker2.md`, CLAUDE.md and CLAUDE.local.md first. Identity #ab-w2c; lead #ab-lead2. This checkpoint is for the requested fresh restart; do not start RT24 in the old window.

## Order and rules

1. Finish/land the M.3 and M.5 source slices (lead2 sends gate; worker never normal gate, bumps, merges or pushes).
2. **J50**, newly assigned, test-only, due 2026-10-31 18:00, after the restart and before RT24: runner-level tests for a well-formed unknown-id answer line and an unparseable line. Each must be logged/dropped, child stays serving, and no message is answered by it. Mutants that answer or crash must fail assertions. Do not fold this into M.5: its runtime must stay byte-identical to the reviewed source.
3. Resume RT24 after J50 (scope below).

All mutants and deliberate CPU load take smoke-gate@parf.us slow; all Podman fixtures do too. Fresh holds, no reservations/handoffs/chains. Gate landings take priority. Never message #ab-gate (script interprets every message as a landing). Stop/release at a round boundary if lead says a landing is queued. No PORT since J20. Owner questions go up the bus, never prompt the user in this window. Owner review URLs are mdhouse links checked200.

## Decisions (landed)

- Main **8353af0f**, no bump. `ab-w2c/runner-decisions` worktree/branch removed after landed instruction.
- One docs commit records four owner choices at **2026-10-30 15:25**, with named index rows linking substance, not duplicating values:
  - M.3a option(a): `{ "message_id": "<echoed>", "body": "<answer>" }`; absent/empty body means done; malformed/unknown-id line logged/dropped; child exit logs held messages and restarts with backoff.
  - M.6a A: optional consume `host`; approved details from .ai-team/w2a/m6a-member-host.md copied into runner-implementation.md (not a .ai-team/ link):1–64 ASCII letters/digits/dot/hyphen,400 otherwise; refreshed read begin/end;2-minute memory-only window;32-host cap/oldest forgotten with truncation note; same readers visibility; CLI Hosts line/web/lookup, no routing/authority.
  - M.8 A+B: bundled name or git URL pinned to full commit; branch/tag/abbreviated rev refused; host needs git+route; private repo uses host key, never call-supplied key; C/D later.
  - Q260(1): consume parameter selects ask-set messages, leaves ordinary queued, may coexist with unfiltered read under existing authority. Q260 removed from QUESTIONS; substance in RT25 TODO.

## Reviewed source and split

- Takeover authorized from **e80f5143**, branch `ab-w2b/M3a` (original untouched/frozen). Assignment's61039f82 was a typo; lead2 explicitly confirmed e80f5143.
- e80f5143 was fully read by both readers. It already removes Waiting==0 and installs api.ConnContext in the runner HTTP fixture (J39). The approved answer line exactly matches streamAnswer, so no seam change.
- Intermediate **dccfebe1** is e80f5143's28 commits rebased onto8353af0f; its temporary combined branch/worktree were removed after the slices were committed. `git diff e80f5143 dccfebe1 -- src/cmd/agent-bus-runner src/internal/ports/sandbox.go src/internal/sandbox/bwrap.go` is empty. This commit is only a source reference; do not land it.

### M.3

- Branch **ab-w2c/M3**, worktree **tmp/ab-w2c-M3**, ready tip **a75acf51** on8353af0f.
- Lead2 says #ab-w1b's split-only read is clean/no HIGH; **M.3 is at #ab-gate with a bump**. Leave branch/worktree until landed notice.
- M.3-only production differences from combined source: jsonl.go omits Job.Over; serve.go omits launchSettings.over/snapshot, installed.Over non-bwrap refusal, and Job.Over. All stream safety fixes remain. The test fixture keeps its pre-M5 shape+J39 ConnContext; token-return helper goes with M5's sibling test. Overlay-only files and settings-guide hunks deferred.
- Verified under slow: full focused runner `go test -race ./cmd/agent-bus-runner -count=1 -timeout=120s -v`: **146 top-level passes,0 skips,29.730s**. All split-touched runner paths covered. Both runner guides docs-order2/0. Original owning mutants retained as inherited proof; split got its own read.
- TODO M.3 row and DONE M.3 section updated; full evidence in commit body and ignored worktree/tmp/m3-proof/runner.log.

### M.5 ready

- Branch **ab-w2c/M5**, worktree **tmp/ab-w2c-M5**, ready tip **71e047a0**, stacked on M.3a75acf51. Clean. Lead2 receives ready with this checkpoint; land only after M.3.
- Restored combined source and M5 guide changes. Staged runner/ports/sandbox source is **byte-identical to e80f5143**, verified with `git diff --cached --exit-code e80f5143 -- src/cmd/agent-bus-runner src/internal/ports/sandbox.go src/internal/sandbox/bwrap.go`.
- M5 changes: settings JSON merge/mount/refusal/digest plus bus read/write/delete; sibling no-SIGHUP test+token fixture helper. Reviewed null-patch, delegate-crash skip, retirement and deadline fixes are all retained.
- Focused check completed under slow: `go -C src test -race ./cmd/agent-bus-runner ./internal/sandbox -count=1 -timeout=120s -v`: **154 top-level passes,0 skips** (runner29.758s,sandbox1.055s); runner guides3/0 docs-order. Log worktree/tmp/m5-proof/focused.log. **Lock released; no active test/load**. No changes for inherited J50 LOW.
- Final committed runtime/tests AND runner guides match e80f5143 byte-for-byte, checked after commit. TODO M5 and DONE M5 updated; full source/proof details in71e047a0 body. No worker bump, gate, merge or deploy.

## RT24 parked — approved, no implementation yet

- Branch **ab-w2c/RT24**, worktree **tmp/ab-w2c-RT24**, clean parked commit **8579feaf**. Docs-only: RT24 plan substance in TODO + named DECISIONS row, same commit, no bump. No runtime edits were made; all implementation discussion below is only a plan.
- Source decision note: `http://localhost:7777/ai-agent-bus/.ai-team/w2c/rt24-choices.md` (checked200). #ab-dir settled it via bus33c3a2cb19cd42e9; lead2 copied. Due2026-11-02 12:00, bump via gate.
- **Text default**, plus **explicit --json** on body methods `status`, `version`, `ls`. JSON returns **existing API result as is, no new schema/preamble/fence**. `help` text and `help-json` metadata remain; ask status retains RT23 text. Version JSON maps to **GET /identity** (no /version endpoint); status JSON must include full authorized **GET /status** result, not only core.Status. Listing/lookup selection/fields remain existing API's; empty list is[].
- Counts A: existing **StatusFor** node totals. Records are all live kinds, not caller-visible/agents-only. Readers count outstanding polls incl.filtered, not unique/healthy agents; busy handler need not wait. Uptime current process; queued current depth; dropped/expired cumulative. OwnerInactive detail remains absent except Owner/active Administrators, never fabricated zero for ordinary caller.
- `ok` means authenticated status request served; no degraded branch until an existing observable fault is named. No new host probes/thresholds/health score.
- Authorized implementation paths: **src/internal/api/daemon_agent.go, its tests, docs/agents/bus.md**. **Do not touch core(A); stop/telldir if needed.** Plan/decision edits are already authorized. On resume rebase onto landed runner decisions first; DECISIONS conflict may need keeping both rows. Existing RT24 index row repeats some substance; can reduce it to named row+link consistently with the reviewer rule.
- Acceptance: text/status asks preserved; valid direct JSON incl.empty[]; two ACL callers have different ls but same general totals/privileged detail gated; fixtures for inactive/all kinds, filtered polls/unique agents, busy handlers, current/cumulative counters. Format wrapper, broadened listing, wrong counts and privileged disclosure mutants must fail assertions. No core implementation/mutation permission assumed.

## Other branch/review status

- **J45 ready ab-w2c/J45 72e02f5d**, worktree tmp/ab-w2c-J45; await reader/gate. Test-only CPU parser ratios + authority handshake. Actual normal-shard load30/30each of3parsercases+30/30delivery; all named mutants assertion-red; missing-handshake pin red. Sources restored, no load. Full body and tmp/j45-proof contain evidence. First missing-MCP-link run and build-only30rounds explicitly discarded.
- DW11 read80fbb345: no HIGH/MED; docs-order26/26;308relative links/anchors resolve inclMODULES#modules;15pages rewritten (PHP16thpending), PHP untouched; J39 source change comment-only. Hashes/bytes+INSTALL envelope match code. Review tree removed.
- J41 readf5838edd: no runtimeHIGH; MED unbounded admission Waiting==1 loop/gonejoin. No-oldpoll variant compiles then global2s timeout; pin-off assertion-red run0;3focusedbaselinechecks pass. Reviewtree tmp/ab-w2c-J41-review retained pending follow-up.
- J43 readfa7d58d8: MED catch-anyThrowable falsely counts abandoned polls; no-oldpoll mutant6/0 falselygreen. Pin-off independently protocol+2lost/50red; baseline6/0. Reviewtree tmp/ab-w2c-J43-review retained pending follow-up.
- Discord B27a second review2afee708 found MED unpinned token-redaction error; actualtoken-echo mutant all30/251green. Secret-only runtime. Lead says read done; detached tmp/ab-w2c-B27a-review still exists.
- J38 landed5850fd04; worktree/branch removed. B29 landed/deployed/cleaned. RT8 handed to #ab-w2b from83fa37a0, do not touch its parked branches.
- Standing B15 observer ownership remains: run main's `python3 -B src/maintenance/node-observe.py --repo /usr/local/src/ai-agent-bus` a few times/day into ignored tmp/b15; no root/timers/deletions. Last recorded observation was12:40local2026-10-30; another is due when between commits.

2026-10-30 fresh-session update: M.3 landed 8816905b and M.5 landed aa6631e3; both own worktrees/branches removed. J50 active on ab-w2c/J50. Queue per lead2: J50, then M.12 (runner; due 11-02 12:00; rebase after w2b M.6a share wiring), then RT24 (due 11-02 18:00; parked 8579feaf).

J50 ready: ab-w2c/J50 at 7cb7c8da, worktree tmp/ab-w2c-J50. Real shell child, production parser, FIFO stdout barrier for unknown-id/unparseable lines; four answer/kill mutants assertion-red plus log-removal mutant red in both cases; focused JSONL -race green and ten repeats green. No runtime changes/bump. Reported lead2; keep branch until landed. Next M.12 (after w2b M.6a share wiring), then parked RT24. No lock or process running.

Queue update: after M.12, J52 test-only (due 11-02 12:00): envfile parser checks need an inner deadline; repeated-prefix mutant must fail inside it on an assertion, not outer timeout; file in Plans/issues. Then RT24. M.12 paused at 01d0c2a2 while J45 rebase is resolved.

Commit style reminder from dir: plain sentence subjects only; no prefixes or parenthesized tags/ids. J45 landed 2e77d438 and own worktree/branch removed. M.12 in progress.

M.12 ready: ab-w2c/M12 at cd5263cb, worktree tmp/ab-w2c-M12; rebased cleanly on main 2e77d438. Approved spellings recorded in DONE.md:192 and current docs/runner/configuration.md#agent-directories; TODO row status done. Full runner -race green (29.8s), final focused -race green, docs-order 2/2, links checked. Eight mutants assertion-red in commit body. No lock/test process remains. Lead says do not wait for M.6a wiring; M.12 may land first. J45 landed/cleaned. Next J52, then parked RT24 (8579feaf). B15 observation appended 2026-10-30 to tmp/b15/2026-10-30-worker2.log.

Queue update from lead2: M.12 MED, J52, RT24, then J40 (--fresh launcher, due 11-03 12:00; spec Plans/issues J40). M.12 MED reproduced on copied git tree outside /usr: cron fails import cronexpr without support binds. Fix adds only git-bundled read-only roots agents/shared, clients/python, clients/typescript, internal/version. Also fixing stale serving.dir after source fallback; tests see original failure. Empty --bundled refused, empty shared shadow documented. Delta proof running; no final ready yet.

M.12 review repair ready: ab-w2c/M12 33e4bed6, clean rebase on current main d6e378b3. Real enabled cron native-help startup runs inside default bwrap on copied git tree outside /usr; original keeper mount list and missing shared bind fail ModuleNotFoundError assertions. Only git-bundled roots added readonly: src/agents/shared, src/clients/python, src/clients/typescript, src/internal/version; scope guards mutant-proven. Also refuse empty --bundled, document empty shared shadow, retire/readopt when selected Source changes (old snapshot assertion-red). Six credited delta proofs after confirmed grant; preliminary refused-lock batch not credited. Full runner -race green 29.9s; final focused -race green; docs-order 2/2, mdhouse HTTP200; clean tree; no lock/process. Reported ready to lead2; keep worktree until landed. Next J52, RT24, J40.

Queue update from lead2 (J54, message fcbde71cd2e5fe39): M.12 is at ab-gate; leave its branch unchanged. Next J52, then J54, then RT24, then J40. J54 is test-only, due 2026-11-02 18:00: start one real bundled Bun agent from a copied checkout src/agents outside /usr in the real sandbox, and count its answer to a message. Dropping the clients/typescript or node_modules bind must compile and fail on an assertion.

Lead queue correction: short focused non-mutant checks may run during a gate (dir); lock required for mutants, -race repeats, loops and heavy runs. M.12 landed 0654815c/0.9.214 and own worktree/branch removed. J43 log audit sent. Queue: J52, J55 (rename-smoke bounded identity + diagnostic, then locked repeats; due 10-31 18:00), J56 (skip standalone-only packages instead of refused command-required; bump, due 10-31 18:00), J54, RT24, J40. J52 active branch ab-w2c/J52 from main0654815c.

J52 ready: ab-w2c/J52 1ebff584 (tmp/ab-w2c-J52), clean rebase on main. Only envfile.test.ts and appended J52 Smoke issue entry differ. Existing CPU-ratio checks now benchmark in a child; parent 2s watchdog kills/reaps, named assertion before 10s outer test bound. Exact w1c repeated-prefix lines and split variants compiled and failed the named inner-deadline assertion in 2.012/2.015s under confirmed slow grant. Restored/final 28/0, 46 assertions, 209ms. Runtime parser unchanged; no bump, lock released. Reported ready; keep worktree until landed. Queue next J55, J56, J54, RT24, J40.

Queue update from lead2, message 5f99a03ca120cbfb: J55, J56, B.29 cron/#at second read, J54, RT24, J40. Review ab-w2b/B29-cron cfa3b266, reader 2: scheduler.py at-most-once delivery; RD161 one warning/drop, recurring entry active, one-shot removed, no pause/retry; kill fixture counts deliveries; legacy-id migration/restart recovery; sub edit refused. Seven mutants claimed; verify proof and send verdict to lead2. Do this after J55/J56 and before J54.

J55 ready: ab-w2c/J55 9806f392, worktree tmp/ab-w2c-J55. Bounded identity guard covers first+resume, reads complete lines and re-reads after exit, prints child diagnostic on missing event. Ten quiet focused mcp_rename shards passed under slow; no unloaded flake. Fresh-hold three mutants assertion-red with diagnostic: missing-first4.388s, missing-resume5.527s, live-stall12.624s. Fixture restored, final focused green, clean rebase; test source byte-identical across rebase. Only rename-smoke.ts + appended J55 issue differ. Initial shard refused without running because CLI caller was parf; then existing worker token used without rotation so CLI matches MCP holder. No lock/process remains. J52 landed and cleaned. Starting J56 next, then B29-cron reader2, J54, RT24, J40.

J55 MED repair in progress: pipe reads can hang if a child ignores TERM. J56 paused clean at ab-w2c/J56 4ffe98d9 (unit green, mutants/full/docs pending). J55 adds dedicated launcher group, group TERM/grace/KILL, bounded partial-output reads. Need prove exact reviewer TERM-ignore/Bun.sleep(600000) fixture, no leftovers, restored baseline, then ready. No lock held yet for repair proof.

J55 repaired/ready: ab-w2c/J55 73dc1f7a, clean rebase and only harness+J55 issue diff. Dedicated launcher process groups; group TERM, up to4s cleanup grace, group KILL; exit collection1s fallback, both pipe reads2s concurrently retaining partial text/(still open). Exact reviewer TERM-ignore+600s sleep before identity fails named checks, diagnostic printed, whole owning shard31.412s, no descendant PIDs from this fixture path. Restored owning shard green/no descendants. Confirmed slow grant used, lock released. Test bytes unchanged by final rebase. J56 remains clean at4ffe98d9; resuming proof/docs after ready report.

J56 ready: ab-w2c/J56 9467ee0f, clean rebase/test bytes unchanged. Skips valid native metadata without a command only in bundled standalone scan; file controls agree; command-bearing tmux remains available. Empty declared commands (including COMMAND casefold), malformed metadata, shared/flat broken packages remain refused. Real shipped five-manifest fixture reproduces four refusals before fix. Three compiled mutants assertion-red under confirmed slow hold; full runner -race29.981s green, final focused green, docs-order green. No lock/process. J55 waiting review at73dc1f7a. Queue now T14 PHP journal flags, B29-cron reader2 cfa3b266, J54, RT24, J40.

STOPPED FOR T-13 MOVE. Latest concise checkpoint: .ai-team/w2c/t13-move-checkpoint.md. J55 73dc1f7a, J56 9467ee0f, RT24 8579feaf retained. Empty T14 tree/branch removed; no edits there. Start nothing until ab-ceo says moved.
