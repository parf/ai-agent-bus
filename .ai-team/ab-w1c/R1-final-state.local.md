# R1 takeover — W1C lead, W2C support, 2026-11-02

Owner says all other interactive agents are out of usage. Read the complete
.ai-team/dir/R1-release-handoff.md. Existing worker role is superseded for release
coordination only; no new publish/push/daemon-restart authority inferred.

Frozen candidate: main c815ac3f1c19d2668e2e9a5c0f2e09de3a0ee9c7,
tree c7de8b7d9b1969f1fce3652991107fadef9bd280, VERSION0.9.303.
All 12 Fable blockers fixed/closed; team2 closeout4f40e3a2 and team1
closeoutc815ac3f landed through gate, docs/no bump. No new landing before slow.

Landings:
- G1 88a1b6dc0.9.295, B1 307ec8620.9.296, A2 090188600.9.297,
  Q1 a115609c0.9.298, A3/A4 4d0fb2110.9.299, J1 506d72390.9.300,
  B2 2a4cc2330.9.301, C1 4b2610a50.9.302, D1 f32f605d0.9.303.
- F1 ac28de6d, I1 3a3a2b97, I2 0ffd8572, J92 5b590ccc all no bump.
- C1 two MEDs independently re-read CLEAN at6b493750 (landed4b2610a5):
  bounded Close/gate wait+SQL context, atomic retirement excludes stale flush.
- I2 initial lead2 CLEAN, one inactive-view LOW fixed, W2C delta CLEAN;
  W1C gated it, branch/tree removed. I1 and D1 CLEAN from W2C, landed/removed.
- B2 corrected gate regression: provider capability only caches/bounds GitHub;
  local file deletion immediately refuses. Mutant assertion-red, enrolment27/0.

W2C tasks:
1 Update LOCAL runbook .ai-team/ab-w2b/release-1.0.md: actual Darwin/full/client/npm
  assets, create-or-upload publication, explicit ARCHIVE= step8. No network.
2 One authorized release slow-all on EXACT frozen c815ac3f, detached own tree,
  fresh CLI parf lock sole call/read grant, setsid, log
  tmp/gate-logs/20261102-R1-final-slow.log and exit status. No load/burners.
  Kill/reap own descendants on interruption; release immediately; remove tree.
3 Final Fable-delta summary, no new model/provider calls.
W2C was told no deployment/tag/push authority, no new gates or landings meanwhile.

Main checkout clean; own finished worktrees/branches removed. W2C closeout3
cleanup requested. Other W2B finished A2/A3 worktrees left intact (not ours).
Disk141GB free; load1=1.43 before run. No own held lock/run.

Remaining boundaries: handoff orders one backed-up daemon/CLI deploy before
final release, but restart/push are CEO-only under current local rules. CEO is
out. Prepare concrete reviewed action and ask owner only after slow proof.
No publication/tag until owner go; npm login is owner-only. Final CEO/Fable delta
review is also unavailable: do not relabel Codex review as Fable. STABLE only after
required proof/review decision, through gate stable mode, not a manual main edit.
No fetch/pull/push under current rules.

## First final slow run RED; triage, not release evidence

W2C ran c815ac3f with an external Python supervisor that enabled subreaper but
only reaped after smoke exited. Result1301/4,90.46s,rc1; log/status preserved.
Three cleanup reds (watcher, supervisor, launcher) are confounded by adopted
zombies remaining visible to kill0/ps; do not call them product regressions.
W2C confirmed/released lock, no live descendants; no blind retry authorized.

Fourth red independently reproduced: git OwnersFlow post-attempt script127,
fixture host-c parent mode0775 from ambient umask0002. Production guard is right.
J94 branch92008bad (tmp/ab-w1c-R1-triage) only changes fixture mkdirs to0700.
Same OwnersFlow test before red0.4s, after green11.3s underexplicitumask0002.
W2C read requested; gate after CLEAN, then small issues closeout, then new frozen
candidate/ordinary setsid slow run WITHOUT subreaper. No runtime publication.

J94 CLEAN fromW2C; gate landed92008bad, normal46s/no bump; closeout06fabb3c
landednormal8s. Both own worktree/branch removed. New frozen main
06fabb3c6de88626dc341ea15284bdd1412f4544,
tree82388f930a03213fd9f26d074a2ca722e7c8368e, still0.9.303.
Authorized W2C slow-all2 on exacttip with reviewed run-final-slow2.py: NOsubreaper,
ordinary setsid, own recorded-PID/starttime cleanup; CLIlocksolecall/readgrant,
new final-slow2 log/status preserved. No new landing until result. No owner
live-action authorization requested yet; no deploy/push/tag/STABLE performed.

## Corrected final slow GREEN and staged cutover

Exact06fabb3c/tree82388f930a03213fd9f26d074a2ca722e7c8368e:1305passed/0failed,
rc0,87.804sdriver/91.976stotal,22:30:30–22:32:02UTC. No skipped-section report.
All4 prior failed areas nowgreen. No subreaper, no own background or descendants.
CLIlockreleased; independently holders{}; W2C detached slowtree removed.
Evidence tmp/gate-logs/20261102-R1-final-slow2.log and.status.json.

Staging worktree tmp/ab-w1c-R1-deploy-stage, detached06fabb3c. Documentedbuild.sh
PROGRAMS='./cmd/agent-busd ./cmd/agent-bus',BUILD_COMMIT=fullcandidate. Both
--version report0.9.303 with exactcommit, buildtime2026-11-02 17:33:06.
SHA256 daemon63b11ce4aa3e5f9d4cd6c5f141ac374c0cc16036906240ba35bab6444d3ebaef,
CLI f0976f373b7ce52fc68bf5bbeab89d2a775c265d2db7e201f7e76e37697ef056.
No live binary/service changed. W2C reads .ai-team/ab-w1c/R1-deploy-plan.local.md
before owner request. Current no locks or own active run.

Deploy-plan W2C review: initial MED (running MCP pushers not refreshed bydaemon/
CLIswap) explicitly fixed; CLEAN0e2007bb66a774e1. Plan now pins default'.'
buildoutput, backup0600 fromcreation, destination-local atomicrename. Owner
approval questions are in LOCALplan itemform; no live operation authorized yet.


## Current release state — approved cutover complete; local STABLE reviewed

Main is 87b6b918 (0.9.303). Q329/Q330 were approved by the owner; their
selected options and original owner replies are preserved in R1 DECISIONS.
QUESTIONS contains only unresolved choices and is currently empty.

Q329 completed in one stop/start cycle: daemon and Go CLI report 0.9.303,
source 06fabb3c6de88626dc341ea15284bdd1412f4544, protocol 1/1; check-deploy
passed all six checks. Existing MCP/pushers and runner/web were not refreshed.
Backup (root-only, stopped home plus old binaries and effective unit):
/var/lib/agent-bus/daemon.backup/R1-0.9.293-to-0.9.303-20261102-192258-U0mGGU
Archive daemon.tar was created mode 0600 inside a mode 0700 directory.
Execution: .ai-team/ab-w1c/R1-cutover.local.sh; public identity metadata retained.

The corrected full slow run on exact source 06fabb3c/tree
82388f930a03213fd9f26d074a2ca722e7c8368e passed 1305/0, rc0; evidence:
tmp/gate-logs/20261102-R1-final-slow2.log and .status.json.
First red evidence retained; J94 fixture fix and wrapper confound documented.

Q330 accepts independent Codex review instead of the unavailable CEO/Fable
final delta review for this release only. It authorizes local STABLE preparation
for review only. W2C prepared ab-w2c/R1-stable-local at
10f6b481065e7e1af75c21bcb60115db2a0f88a0, exactly one commit above 87b6b918.
W1C reviewed the complete diff: VERSION 1.0.0, archived unchanged 0.9 entries
with one relative link corrected, new 1.0 changelog, Plans/VERSIONS links.
Four paths, +12/-3; no executable code or owner answer changes. W2C reports
links and internal/version green, markers/diff clean. Worktree stays parked.
No STABLE gate send/landing, push, tag or publication has occurred.
No held lock or active own run. Next: owner review of this local candidate.

## Linux RC validation and license steering

Local1.0.0RC tag10f6b481, RC2ca7c7396; neither pushed/published. Linux journey collection on Ubuntu24.04.5/Archrolling/Debian13, then owner addedAlma10.2/latestFedora44stable. Original errors owner/person mismatch, shippedINSTALL missing-relative-links/checkout-first, hostname unavailable onminimalArch/Fedora. Collected before fixes543f137a. RC2 exact packaged commands pass all5; Ubuntu32/0+11assertions, other4each61steps/59success+initdegraded/immediaterestartobservations followed by readiness/persistence/final3unitsgreen. npm ordinary user3/3RC1+RC2. Actualbrowser onUbuntuRC1 only, HTTP onothers. Results/listdocs landed7297ab1a, normal10s. Evidence Plans/R1.0-Release/done/rc-linux-installation.md; rawlocal/ab-w1c/RC andlocal/ab-w2c/RC2. All own containers/volumes removed, lock released.

Owner then selected exact PolyFormStrict1.0.0 and confirmed root/bus/runnerStrict, src/clientsMIT. Landed49295278(normal68s,no bump): officialterms byteexact except separate retainedcopyright; all22manifest/goldens and Go/Py/TShelp labels; private npmSEEfile/pluginLicenseRef; currentdecisions+oldarchive. SDKMIT/thirdparty unchanged; independentW2CCLEAN; exportedruntime+MITbytechecks andfocusedmetadata green. OldRCtags remain unmodified historicallicense snapshots. No new stablelanding/tag/publication/live cutover implied. Mac read-only preflight next via existing sshmac, previous MACrules and ~/agent-bus-dev only. Quota watcher intentionally runs every900s; threshold<=5 sends one TG, atlastcheck13percent. No held lock or active test/build.

Mac preflight sshmac timed out onport22 (BatchMode/ConnectTimeout10s), no remote command executed or host change. Native Mac acceptance blocked on target accessibility. Own finished RC1/RC2/deploy-stage worktrees and license export removed; local tags/archives/logs retained. Main49295278clean; W2CstableRC2branch stillparked.

2026-11-03 owner adds final-release TODOs (no build requested): T-33 AI Markdown install/config/update workflow doing most work then guided email/Telegram/etc setup; T-34 WebFace chat backed by ab-* session, inspect mdhouse first. Branchab-w1c/R1-ai-setup-plan b9e9f675; newPlanai-setup.md +README/TODO/decision,4files+50. Links/diff/markers clean, W2CCLEAN, sentgate. Implementation explicitly pending; Mac/release tomorrow perowner. No live/provider/newtag actions. No lock/run.
T33/T34 plan landed mainb9e9f675 (normal9s/no bump); ownplanWT/branchremoved; treeclean. Both tasks remainpendingimplementation; mdhousereadfirst forchat. No newruntimeaction.


2026-11-03 owner session: native Mac RC2 ca7c7396 passes full archive install/checksum/manifest/native version+stamp, nonroot/private role, TLS once, restart/orphan persistence, strict teardown; role206 kept safe by current policy. Offline local npm client plus two exact-body/identity standalone-runner calls pass; sandbox-on refused. First client fixture's short/qualified name mismatch corrected, no product edit. Mac clean original mainc49d51f3, workspace66M (was64M; added Gitobjects), caches/build/input/own fixture state gone. No Mac Go build/brew/profile/provider/publication. Node build lock released. Report committed ab-w1c/RC-mac-evidence50137709; review tree and candidate archives tmp/RC2-mac-dist retained; build/npm scratch removed. T33 branch74a57621 and call-body R1.1 brancha3caf68f remain pending review/landing. T34 now R1.1 on T33 branch. Final legal files need current Strict in final artifacts, no Linux runtime rerun solely forlicense.
