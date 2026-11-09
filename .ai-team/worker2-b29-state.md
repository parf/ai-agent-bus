# Worker 2c checkpoint

## Current status

- B.29 is DONE: main **2a9e8b3c**, version **0.9.205**, deployed (CEO report). Both readers approved; main ancestry and version checked.
- Cleanup verified: B29 worktree, branch and proof scratch are removed. No active tests or smoke-gate lock.
- RT8 is handed to **#ab-w2b from 83fa37a0**. Do not touch that branch again. The parked Python/TS/Go branches are handed-off history, not active work.
- Next task is **J38 (S.10 flake), after J37 lands**. Wait for #ab-lead2's assignment/start signal. Do not revive B.29 or take J39 (rerouted).

## J38 evidence so far

- Gate log `tmp/gate-logs/20261030-132212-ab-w2c_B29.log:2571`: S.10 failed a `toEqual` assertion. The smoke wrapper omitted the expected/received values and source line. Passing the rerun did not close it.
- Owning file: `src/agents/slack/s10.test.ts`, standalone bundles routing real bus replies to Slack threads and keeping Telegram serving.
- Same unmodified test passed **10/10 on B.29 39b9c556 and 10/10 on main 8c2df258**. Lead2 accepted it as a pre-existing harness flake and escalated it as J38. The exact cause remains unproven.
- Fixed 100 ms event delays and 50 ms empty-inbox probes are timing suspects only. Tests use disposable private daemons/fake providers; no Podman or live provider calls.
- Full repeat logs were scratch, removed on landing as instructed. Results are retained in the bus messages.
- Separate queue-test import isolation was fixed in B.29 (`forward.ts?queue-fixture`); that is not the S.10 failure.
- Parallel focused tests without SMOKE_BIN can race rebuilding the common `tmp/agent-tests/agent-busd` path (ENOENT). Build one private binary and supply SMOKE_BIN, as the gate does.

## Standing rules and handoffs

- Read .ai-team/worker2.md, CLAUDE.md, CLAUDE.local.md. Identity #ab-w2c, tasks from #ab-lead2.
- Own worktree/branch only; never main, VERSION/CHANGELOG, normal gate, merge or push. Focused checks allowed. All mutants and any Podman fixtures take smoke-gate slow; one batch per hold, name next taker on release. Never message #ab-gate. No PORT since J20.
- Owner questions go through lead2, never a prompt in this window. Owner review links use checked-200 mdhouse URLs.
- M.3/M.5 review finished on 9696522e, no high; every finding closed, review trees removed. RT21 PHP, logwatch and IMAP reviews also completed/cleaned.
- B.15 observation remains assigned: main's `python3 -B src/maintenance/node-observe.py --repo /usr/local/src/ai-agent-bus`, a few times/day into ignored main/tmp/b15. Last recorded run here was 2026-10-30 12:40 local. No timers/root/deletion; tell lead2 if handing off.

## J38 assigned and filed

- Branch `ab-w2c/J38`, worktree `tmp/ab-w2c-J38`, clean head **bec8b00c**.
- Only the issue entry is committed, at the end of `## Smoke` in `Plans/issues/2026-10/2026-10-30.md`.
- Both gate reds verified: B29 log132212 expect.toEqual9.88s; J35a log140340 expect.toBe9.06s. Focused10/10 does not reproduce all-normal load or close this issue.
- Diagnosis and code changes wait for lead2's J37-landed signal. Fix scope S10 test only, bounded conditions instead of fixed sleeps; acceptance50 runs under all-normal load green plus assertion-red behaviour mutant. Workers do not run normal gates: coordinate the required load with lead2 when diagnosis starts.
- Due 2026-11-01 12:00. No tests or lock active.

## J38 in progress (2026-10-30, after restart)

- Top priority from lead2; worktree `tmp/ab-w2c-J38`, branch `ab-w2c/J38`. Fix committed and rebased on main66e7bfaa. Only S10 test + J38 issue entry changed; no runtime change.
- J37 log142447 proves premature observations: old MCP post at line67; empty subscription inbox at72. event now waits for its own post/inbox effect; FIFO next event covers self echoes and silent chatter; bus answer orders spoof check.
- Baseline passes1/0,33 assertions (~6.3s). Production mutant removing routed `thread_ts` in slack.ts compiled into package and failed exact toMatchObject assertion at S10:65 (~5.7s); restored source. Mutant proof `tmp/ab-w2c-J38/tmp/j38-proof/wrong-thread.log`.
- Approved acceptance is24 (`nproc`) busy loops +50 S10 focused runs under slow. Harness prepared `tmp/j38-proof/load.py` (5 concurrent private fixtures, immutable SMOKE_BIN=`src/tmp/j38-bin`). J39 deploy done14:41 per lead2, load cleared. Queuing fresh slow now; no load yet.
- Current lock rule supersedes old handoff rule: acquire fresh, release after run, no reservations/chained holds. Never message gate.
- After J38: small M5 delta review e80f5143 vs61039f82, only Waiting==0 removal + api.ConnContext in runner fixture. Low priority, not started.
- Discord B27a review2afee708 found MED leak-proof gap: actual-token error mutant passes all30/251; lead1 notified. Runtime Secret-only. Detached review tree retained awaiting fix. T9subscriptions64599516 done, noHIGH/MED; LOW unexecuted placeholder examples. Old-page ENFORCED mutant assertion-red, restored26/0, reviewtree removed.

## J38 ready

- Ready sent to lead2: branch `ab-w2c/J38`, tip **8e6036ae**, base main362991a6. Clean; only S10 + issue J38. Final rebase resolved only issue overlap, retaining J42 verbatim and J38 at Smoke end; S10 byte-identical to tested tree.
- Acceptance complete under slow:24 busy loops,5 concurrent fixture runners,50/50 green,33 assertions each (~6.9–7.2s). Loops stopped. Wrong-thread mutant repeated post-load: package builds then direct thread_ts toMatchObject assertion fail5.7s; runtime restored. Lock released.
- Full evidence in commit body and ignored `tmp/ab-w2c-J38/tmp/j38-proof/`. Await review/gate; leave branch until landed notice.

## J38 review fixes ready

- Final head **395c84a4** on ab-w2c/J38. One bus ready line mistakenly said61abff11; immediately corrected to lead2. Never use that hash.
- Reader MEDs proved under slow: removing ordinary-chatter guard causes count+1 Expected3/Received4 atS10:73; removing reply sendercheck causes spoof barrier Expected2/Received3 atS10:68. Real packages built,5.8/5.7s assertion failures. Runtime restored, released; restored S10 1/0,33 assertions6.8s.
- Only comments plus entry updated: single-worker serve active capacity/awaited onReply order, unchanged50ms quiet window, explicit dir14:34 load approval, exact J37 diffs. Full proof in395c84a4 commit body. Await reader/gate.

## J38 final rebase ready

- Tip **5850fd04**, base main9fac93eb after J37. Plan conflicts resolved retaining every other entry byte-for-byte (remove J38 => exact main file), J38 last in Smoke. S10 identical395c84a4; clean.
- Ready sent to lead2 for direct gate resend. Leave branch alone. M5e80f5143 small delta read completed, no high; static-only, no additional load.

## J38 done; J43 review pending fix

- J38 landed5850fd04; worktree and branch removed on lead2 instruction. No active J38 work.
- J43 read fa7d58d8 in detached `tmp/ab-w2c-J43-review`. Runtime consumes onlyHTTP1.1; curl_reset prevents option bleed; docs correct. No HIGH.
- MED sent lead2: tests/replaced_reader.php catches anyThrowable as abandoned. Under slow, replace oldpoll with `throw new RuntimeException('no poll sent')` =>6/0 claiming50 abandoned with no poll. Require real curl deadline/admitted-poll precondition and red mutant.
- Baseline focused6/0; pin-off mutant failsprotocol andloss2/50 unloaded (4pass2fail). Runtime restored, clean tree; lock released. Evidence under reviewtree/tmp/j43-proof; keep until follow-up. No load orPodman run.

## J45 ready; queue (latest)

- J45 branch `ab-w2c/J45`, worktree tmp/ab-w2c-J45, **72e02f5d** (clean rebase main5228a7ac). Only2testfiles+J45 issue entry; no runtime/bump. Ready sentlead2. Keep branch awaitingreader/gate.
- Parser CPU usage user+system, original24x ratio/4K32Kinputs. Authority fixturehandshake recordsactualmay-send request, holdsreplyuntilteardown, unchanged1.5s response/cleanupbound + no-worker/dropasserts. Explicitno-requestassert.
- Acceptance: after actualnormalshardslaunched,30/30each of3parserCPUcases +30/30delivery. EarliermissingMCP-link earlyexit and30build-onlyrounds discarded. --normalallloadstopped, lockreleased. Fullenvfile28/0,Delivery3/0 baseline.
- Mutantsunderlock: quadraticprefix scan =>2requestedratiostofail; oldunquote regex=>thirdratiofail; timeoutSDKalone survives(copyalarmstillbounds), bothSDK+copyalarmremoved=>1.947>1.5 assertion; bypassadmission=>no-handshakeassert. Allsourcesrestored. Evidencecommitbody+ignoredtmp/j45-proof.
- J41 reviewf5838edd no runtimeHIGH; MEDunboundedWaiting==1andgonejoin test. Ownno-oldpollmutation=>hangglobal2stimeout; pin-off=>protocolassertfailrun0; baseline3targetspass. Detachedreviewtree tmp/ab-w2c-J41-review retainedpendingfix.
- J43 reviewfa7d58d8 MEDcatchanyThrowablecountsabandonedpoll; no-oldpollmutantpasses6/0. Pin-offred2lost/50 unloaded. Detachedtmp/ab-w2c-J43-review retainedpendingfix.
- Nextqueue: DW11read ab-w2b/DW11 **80fbb345** (docs16pages+1comment; ENFORCED; PHPuntouched); thenRT24 planchoices only inlocal/w2c withrecommendation(senddir,copylead2,no codeuntilsettled). J48maypreemptRT24ifw2busageout.
- J38landed5850fd04,ownbranch+worktreeremoved. B29done,RT8handedoff. Noactiveprocess/lockcurrently.

## Latest authoritative checkpoint

Use **.ai-team/w2c/runner-rt24-checkpoint.md** for the fresh restart: M3a75acf51 at gate; M5 ready71e047a0; J50 first after restart, then parked RT24 at8579feaf. No active lock/tests/load.
