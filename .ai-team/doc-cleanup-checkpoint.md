# Documentation cleanup and legacy planning checkpoint

Updated 2026-11-03. Read this after compaction; task is NOT complete.

## Owner intent and authority

- Apply cleanup to EVERY offered end-user doc and relevant source guide, not just search hits. Full block review + semantic dedup, current shipped behavior, no project release/history/MVP/Built/Status/roadmap/evidence/editorial filler.
- Concepts and authority first; Owner first in role list. Identity page ends with KV penultimate and locks last. Names is short, plain, .2/.3 in bullets.
- Fresh-install first commands must not depend on private development agents (#ab-w1b, parf.us, ab-dvp). Raw full JSON by default; jq only for an actual processing task.
- Simple configuration stays simple. Release users enable bundled packages, not source-build them. Keep legitimate contributor builds and real security/platform/recovery constraints.
- Push, publication and final release tag stay ON HOLD. Main VERSION is 1.0.0. No runtime/product gate solely for prose.
- Owner explicitly requested /goal (created); UI subsequently paused it, then owner explicitly said "continue with cleanup; it does not interfere". Continue authorized work. get_goal still reports paused; tool cannot resume it. Do not mark complete prematurely.
- Owner explicitly authorized as many agents as desired and specifically many Astra agents. Four concurrency slots total including parent.
- Separate independent code review after changes, using local find-issues skill; then paired fix-issues skill. Do not invent a resolve skill; searched, only fix-issues exists.

## NEW legacy request (planning first)

Owner: "lets get rid of ALL legacy stuff (check plans for R1.0.1), find out what legacy we still support; move legacy removal to R1; plan mode". Then: no release was made, unused old options/code must NOT ship in R1. Keep docs cleanup in parallel.

No legacy implementation or plan edits yet. User asked plan mode for this new task: inventory + proposed scope/order/acceptance first. Existing statements saying removal deferred to1.0.1 are now superseded by owner direction, but change those coherently with the approved plan. Distinguish obsolete prerelease compatibility from live functional recovery (interrupted current installs), standards/dependency support, not mere words like 'old'. There is NO actual Plans/R1.0.1 directory found. The 1.0.1 compatibility work is currently only in Plans/doc-cleanup-rules.md C01/C02 and review notes; stderr events-off separate deferred item is not automatically legacy.

Verified active legacy candidates (inventory incomplete, do not claim exhaustive):
- src/internal/protocol/envelope.go:103-122 MaintainerList.UnmarshalJSON accepts legacy scalar string and emits array; tests maintainers_test.go.
- Personal bool mirror: protocol/envelope.go:183-186,319-320 and protocol/tags.go:65+ SyncTags; CLI --personal, MCP/web/client fields. Need distinguish obsolete wire mirror from actual 'personal' tag/semantics.
- Empty-tag sentinel migration: core/legacy_tags.go MigrateLegacyEmptyTags; snapshot.go:295-306 and bus.go legacyEmptyTags field.
- Snapshot legacy establishment: core/accounts.go EstablishAccounts first/legacy; snapshot.go:215-236 accepts absent owner/accounts established markers. Check SQLite schema migrations separately.
- daemon ignored --web: cmd/agent-busd/main.go:56-60 accepts old dashboard flag.
- Go flag parser still accepts single-dash long flags: internal/display/flags.go comment; CLI custom split only treats -- so inventory each program, do not claim CLI parser identical.
- Setup old rollback payloads: cmd/agent-bus-setup/package_install.go legacyFiles agent-bus-web, legacyBundles mcp/server.js and launchers/launcher.js, old no-exec handling near598. Old syslog-unit migration in upgrade.go. Preserve current transactional upgrade/recovery functionality.
- Launcher session bindings/dot-name migration: launchers/smoke.ts:136-189; actual production path needs locate. launchers/sessions.ts:128 accepts legacy fixed pid lock owner filename (TODO after0.9).
- SDK/MCP older daemon paths: Python/TS unmarked legacy reply handling; protocol negotiation warns for absent/older versions across6clients; MCP catalogue reading/readers fallback; maintainer scalar compatibility comment mcp/bus.ts.
- Agents' subscription migrations: shared/subscription-migration.ts + subscription-port.ts, shared/scheduler.py, standalone/logwatch/subscription_migration.py, IMAP forwarding/progress/cache/folder legacy adoption. Extensive migration tests and package inventories include helpers.
- Git standalone timers retirement cleanup; logwatch legacy cleaning/defaults; Slack old session-router state. Separate actual supported behavior from obsolete artifacts.
- Web legacy addresses: web/pages/records.tsx around892. More inventory needed.

## Main, worktrees and DO NOT TOUCH owner edit

- main HEAD f308c291; all previous parent docs cleanup commits are merged here. Parent branch docs/r1-user-doc-cleanup and worktree tmp/docs-release-install-rule also at f308c291, CLEAN.
- Main has ONE OWNER EDIT in docs/02-access.md: line20 adds `🎯` before the existing `✅ A stated trust boundary` bullet. It is a user mdhouse selection; do NOT undo, stash, reset or accidentally stage it. Incoming authority merge will touch this file; preserve the owner edit and plan integration without overwriting it. Do not assume dirty main is an agent error.
- No locks held, no runtime runs or builds active from parent.
- Own docs worktree retained for continued parent work; do not remove it mid-task.
- Other old worktrees (calendar/A2/A3/gate/release prep) are unrelated; don't touch them.

## THREE Astra writers RUNNING in separate worktrees

1. /root/doc_analysis, reused original Astra, branch docs/astra-authority in tmp/docs-astra-authority, base f308c291. It was briefly interrupted on owner's pause then explicitly resumed. Own ONLY docs/01-identity-and-authority.md,02-access.md,02-access-remote.md,03-records.md,03-records-agent.md,03-records-channel.md,03-records-resource.md,03-records-service.md,constitution.md. Full cleanup and verified correctness fixes; preserve current compatibility. Return commits, per-page dedup disposition and removed-anchor mapping. It must not edit other files/source. Current modifications uncommitted as of checkpoint.
2. /root/docs_runtime, Astra, branch docs/astra-runtime in tmp/docs-astra-runtime, base f308c291. Own docs/03-records-kv.md,04-messaging.md,05-discovery.md,06-dedicated-runner.md,06-launchers.md,08-processes.md,09-daemon-api.md,glossary.md,decisions.md. Full cleanup, unsafe KV/reply claims corrected, decisions reduced to current rationale retaining anchors. Current drafted changes uncommitted; static pass then commit expected.
3. /root/docs_source, Astra, branch docs/astra-source in tmp/docs-astra-source, base f308c291. Own Markdown under src/ ONLY (38 original README/INSTALL/MODULES + DB guide). No code/manifest/tool edits. Fixes SDK first-use prerequisites, PHP snippets, MCP approval truth/installed command, container inputs, source history/filler/dedup and bridge guides. Current drafts uncommitted, checking before commit.

Writers were told not to globally edit inbound links outside exclusive scope, not to edit main, no product builds/runtime suites/provider/host calls/locks, preserve images/source quotes and actual compatibility. Parent repairs inbound anchors and integrates. They are writers; independent review agents must be separate.

## Anchor maps already received

Runtime:
- docs/06-launchers.md#runtime-integration-delivery -> #adapters
- docs/04-messaging.md#status -> #durability
- docs/05-discovery.md#status -> #dashboard
- docs/06-dedicated-runner.md#status -> #script-agents
- docs/08-processes.md#status -> #the-processes
- docs/09-daemon-api.md#status -> #stability-and-compatibility
- docs/05-discovery.md#browser-acceptance -> src/web/web-face/site-map.md#every-address for current user refs; historical evidence refs -> Plans/done/R0.8-MVP/done/web-acceptance.md#budgets
- docs/decisions.md#open -> Plans/README.md#stages
- decisions.md current topic anchors and #settled retained. Dated rows removed; Git retains originals, parent can archive from f308c291 if needed.
- New verified runtime facts: core/kv.go rejects top-level null; core/bus.go only validates receipt enum, NOT receipt provenance/correlation; api/server.go overwrites supplied From rather than rejecting it. Writer corrected docs accordingly.

Source:
- src/INSTALL.md#from-a-checkout -> #contributor-checkout (install/update anchors unchanged)
- src/MODULES.md#07-implementation-requirements -> #implementation-requirements
- src/testdata/client-conformance/README.md#state-identity-and-refusal-vectors-t-24 -> #state-identity-and-refusal-vectors
- same file #event-logs-t-31a -> #event-logs; #event-subscriptions-t-31d -> #event-subscriptions
- source standalone git/server-info/systemd removed redirect-only headings: point to docs/agents/<name>.md same anchor except:
  git #roles-and-boundaries -> #who-may-use-it; #install-and-identity -> #installing-it
  server-info #sections/#help -> #calls; #install-and-remove -> #installing-it
  systemd #calls -> #who-may-use-it; #install -> #installing-it
- Slack source #the-slack-app -> docs/agents/slack.md#create-the-slack-app; #the-bus -> #connect-it-to-the-bus; #calling-it -> #use-examples; #running-it -> source #contributor-package for build refs or user docs #installing-it.
- Telegram source #methods -> docs/agents/telegram.md#use-examples.

## Parent exclusive remaining cleanup

Own root README/AI-SETUP (mostly improved), docs/agents/ excluding relocated DB/Discord, docs/runner/, docs/user/, docs/administration.md, docs/platforms/macos.md, docs/10-writing-an-agent.md, docs/dedicated-runner/agent-json.md, governing rules + Plans cleanup ledger. Do not overlap writer scopes.

Parent next useful batch: user guides raw portable examples + correctness/dedup. Read relevant sections already; no edits started yet.
- user/README and cli/daemon first-use status outputs still private #ab-w2a/parf.us; remove example transcript, retain agent-bus status + explain you. CLI 'firstfive minutes ran today' details duplicates Help/list/may-send; can remove while keeping functional standalone commands.
- user/ai-cli private real-node table, #reviewer@parf.us -> newly created bare #reviewer or explicit user placeholder. READERS0 means no waiting read, not exited. Bus realm not necessarily host. File list review requires same accessible checkout, not merely same machine. Review prompt: ack only if needed; answer expected findings even none, never done instead of answer.
- user/dedicated-runner: registration doesn't make everyone see private record; only permitted callers. No blind retry after uncertain side effects. Local stop/log authority follows OS account. Timeout has multiple causes, READERS0 not proof dead.
- user/cli pubsub sequence creates empty channel then immediately publishes (current contract refuses empty routing). Add valid recipient setup/rights or explain it is syntax-only. manage supports --deliver-to, --add-deliver-to, --add-to-set-deliver-to; source usage.go:33 verified. Keep current unknown-outcome warning. Don't describe inactive records hidden from managers. Optional receipts.
- user/backup: exports deliberately exclude queued messages even though clean restart retains them; remove claim generic installed weekly retention (developer node only); replace src/check-deploy with installed/read-only checks after restore. Create sensitive archives private from outset (administration stopped backup currently tar then chmod too late).
- user/daemon: dashboard has visitor sessions/TLS keys, though no privileged bus credential/state. Crash checkpoint may lose arrivals OR replay consumption. Increasing backlog may mean overload, not just stopped reader. Status READERS0 no waiting read only.
- Three source-linked authoring tutorials: simplistic Secret parser != dotenv; five-second unrenewed lock + separate writes cannot guarantee consistent count/history; short hash stamp demonstration, not crypto proof; runner token redirection lacks private ownership/mode; readiness loops unbounded. Don't silently alter source-linked quoted code without coordinating examples. May document exact limitation or remove unsafe claim, keep source-quote markers. Contributor builds legitimate.
- Remaining agent guide full semantic dedup, simplicity; initial install/config fixes and private transcript removal already applied. Need final per-page ledger dispositions, not claim complete from grep. Source-only DB guide moved, source writer owns it now.
- Some new operator refs still need tightening/full final review; current Python check-links only Markdown links, misses HTML href (root README old07#storage was fixed manually). Consider careful HTML link audit, not unnecessary product changes.

## Existing completed work (already main)

- Cleanup rules .claude/docs/WRITING.md + Plans/doc-cleanup-rules.md, page ledger Plans/doc-cleanup-review.md (108 initial candidates; all68 docs +root2 and38 source candidates fully analyzed by Astra; implementation/finaldedup still pending).
- Setup reduced893->64lines, recommended AI-SETUP first; operator details moved docs/administration, runner/setup, platforms/macos; contributor deployment .claude/docs/deployment; old transitions Plans/done/setup-upgrade-history. Inbound Markdown links retargeted.
- Empty Technical details editorial block removed15pages; checker now makes technical part optional. Regression failed old checker, 27docs-checker tests pass.
- Identity conceptual reorder, simple Names, Owner-first role bullets, portable status/full JSON. Most other core first-use examples fixed (status,ls--kind,helpkv) before writers.
-17agent guides private dev-node session blocks removed; useful cron outcomes_since explanation preserved, actual source-vs-role rechecks clarified. No images lost.
- Agent install guides use supplied bundled enablement; standalone commands installed path. Simple Voyage/model,systemd/units,Qdrant/url,SMS config. Real constraints retained.
- Catalogue generated from src/agents/package.sh shipping inventory (agent_docs.py change), excludes DB/Discord; static negative regression fails prior generator. agent_docs+docs_order33tests pass. DB user guide moved src/agents/db/guide.md; Discord guide Plans/R1.1/discord-user-guide.md; metadata docs URLs/goldens moved too (no runtime behavior).
- Root README pruned future/test promises and repeated KV table/patterns, preserves3images+allMermaid, fresh #bus example and --no-asks simple script. Main root still may benefit final semantic pass.
- roles/modules thin R1 plans deleted, current links + unique conformance rule merged into src/clientsREADME. GoogleChat movedR1.2. clients-events audit moved issues/done/2026-11/2026-11-02-client-api-audit.md. Bigfile plan rewritten with why/rationale; canonicalIMAP options updated. No pushes.

## Independent review + resolution skill instructions

Read exact local skills:
- .claude/skills/find-issues/SKILL.md
- .claude/skills/review-checklist.md
- .claude/skills/qa-states.md (NOT .claude/qa-states.md)
- .claude/skills/fix-issues/SKILL.md

User explicitly wants separate good code review and then resolve using own skills. Scope sensible: 90d4fc77..finalHEAD (all cleanup). Separate reviewers not authors for owned files, map Plans/subsystems.md. Pure docs across N/O/P and source K/J/G/Q; tooling changes in L (agent_docs.py/docs_order_test.py) need specific code review. At most4 concurrent inclroot; schedule readers after writers finish. Read last5days issues incl archived; don't duplicate known issues. Verify findings by source/static/focused probes; no live daemon/database/provider tests. No generic product gate solely for prose. Journal/podman/load/mutants require smoke-gate lock15m if ever needed; none held.

find-issues report: Plans/issues/2026-11/2026-11-03.md (not created yet), scope heading range, severity+unique subsystemIDs, file:line, Evidence/Impact sub-bullets, >💡👾 proposal. Serious first. Parent integrates/dedups and commits report alone; checked-sound line. User already authorizes docs cleanup fixes; apply via fix-issues with reproduction, minimal fix, meaningful verification and commit outcome records; behavior changes separate legacy task and plan-mode approval boundaries. Never impersonate user triage or edit owner turns.

## Integration and verification

- Commit parent-only scoped batches in docs/r1-user-doc-cleanup; cherry-pick/merge writer branches after commits, then repair inbound anchors globally centrally.
- Main owner dirty selection must be preserved; don't overwrite/stage it accidentally. Can integrate first into own worktree branch; main merge only once safely preserving owner's edit.
- python3 src/check-links.py; git diff --check; python3 -m unittest src.agents.docs_order_test src.agents.agent_docs_test are focused static doc checks. Do not treat report --report directories as valid; it accepts explicit files, not dirs.
- Parent scripts under ownworktree tmp/scripts are scratch, not tracked. No broad gitadd; stage own named paths.
- Update Plans/doc-cleanup-review.md actual file-by-file full/dedup disposition + unresolved findings, don't mark all done prematurely. Finish goal only when every required fix/check/review/resolution complete.

## Latest owner task: ideal agent checklist (2026-11-03 evening)

Owner explicitly requested as many subagents as possible. Main remains f308c291 + owner access.md selection. Parent worktree branch now has commits 81d6a536 (3 help findings), 4ee18444 (checklist+full bundle audit), 1f15eba0 (three minimal help fixes cherry-picked from agent8a416456). Not merged to main yet as of this note.

Canonical checklist docs/10-writing-an-agent.md now distinguishes essential behavior, safety required when capability applies, suggested useful diagnostics and optional features. authoring.md links it. Plans/agent-quality-review.md inventories 20 bundled packages + #bus, source-only db/discord separately, source/test evidence for all, honest executed/static limits, optional useful improvements. Plans/issues/2026-11/2026-11-03.md records L1 tmux body-help, L2 SMTP JSONhelp, L3 server-info scoped machinehelp.

Two Astra auditors split all20+#bus. Third Astra ran24 focused mocked/metadata/catalogue tests (allpass). Author implemented three fixes in tmp/agent-help-contract-fixes agents/help-contract-fixes8a416456; newtests failed oldcode/passedfixed. Three syntax-valid reverse mutants each assertion-red under parentheldlock, restoredgreen. Reviewer baseline nofindings; second reviewer help_checks nohigh, caveat SMTP main malformed Configuration parsing precedes bodyhelp (valid bus JSON unaffected; don't claim malformed-config independence).

Parent current normal gate: unified exec session92125, cwd tmp/docs-release-install-rule, log tmp/agent-quality-logs/normal.log. Parent holds slow smoke-gate as parf ttl15m (acquired~21:54 local). Release immediately when run completes; never leave held across restart. GOCACHE=/usr/local/src/ai-agent-bus/tmp/gocache. Dependencies symlinked in ownworktree for src/mcp,web,clients/typescript. Normaldefault selection29shards; changed Lspecificagentsnormal, restfast. Currentbuildstageasof21:55. Needs check results, fix materialfailuresifany, append records, commit and integrateto main preserving owneraccessedit. No versionbump/newrelease/push.

Writers now ALL DONE, unmerged:
- docs/astra-authority2318fc757c93e12f5368041894cc7ada14ee1991 (9files)
- docs/astra-runtime72d445c4 (9files)
- docs/astra-sourceee84e507fd14dacc118ae07b82c720b56087670e (34files)
Their inboundmaps above remain needed; independentbroadercleanupreviewstillpending. Do not confuse latestagentchecklistcompletion with completebroadcleanupgoal.

Update~22:00: firstnormal session92125 rc1 (59pass/1failedshard): go_unit exposed stale packaged-runtime server-info expectedmethods, now fixed by5e2be8a3 (cherry2e88c965). Four focused packaging/mismatch checks pass; root/otheragents exactsets retained. New acceptancehelper mutant syntax-valid, own TestAgentRuntimeScopeKeepsExactMethodComparison assertion-red thenrestoredgreen. Independent doc_analysisreview nofindings. Own __pycache__ debris removed; allsubsequentPythonusesPYTHONDONTWRITEBYTECODE=1.

Prompt synced b74e96a3: create-agent-prompt reads canonicalchecklist; bodyhelp optional but any declaredgrammar mustwork, nativehelp shares scope. Finalnormal now session74769, log tmp/agent-quality-logs/normal-final.log, parentholds slowlockparf15m~21:59. Itselects61shards dueacceptanceMchange. RELEASEoncompletion. Earliercheckpoint session92125done/lockreleased.

Broaderindependentdocreviews: agent_help_checks(sourcecommit) nofindings; agent_baseline_audit(authority+runtime) oneLOW decisions.md equates privatewrite with management. Authorfixed in docs/astra-runtime563decf4 above72d445c4; verify source, mergebothlater. Allthreewriterbranchesstillunmerged. Mainownerdirtyaccessselectionuntouched.

LATEST completed ideal-agent task: all newest changes merged main01dbbb77. Owner docs/02-access.md modification preserved unchanged. Three advertisedhelp defects fixed, scopedinstalledtreeacceptancecorrected, four reverse mutants assertionred, twoindependentcode reviews nohigh. Last normal session95955 PASSED1294/0 (61shards,54.550swall;1slow/hostskip); lockreleased. Docs/reportlocalhostURLs200. Helpfixworktree+branchremoved after proving codeequalmain. Parentworktree tmp/docs-release-install-rule branchdocs/r1-user-doc-cleanup cleanatmain01dbbb77, retainedforbroadercleanup.

Allbroaderdocwritercommitsstill UNMERGED: authority2318fc75, runtime72d445c4+563decf4, sourceee84e507. They haveindependentreviews (sourcecleared, authority/runtimeonlyoneLOWnowfixed). Repairinboundanchorsandintegratethese next; preserveowneraccess.mdselection. Remainingparentuser/runner/agentguidescleanup+legacyplanninginventory stillpending; don'tmarkoverallgoalcomplete. Newcanonicalchecklist+auditreport task iscomplete, no push/release.

LATEST STATE: read .ai-team/clean-r1-checkpoint.md; it supersedes older partial handoffs below.


Completed onmain500b1ab3; see .ai-team/clean-r1-checkpoint.md and the durable Plans verification records. Cleanup branches/worktrees removed. Push stays onhold.
