# R1 final finding-resolution audit

Candidate c815ac3f1c19d2668e2e9a5c0f2e09de3a0ee9c7 (0.9.303),
tree c7de8b7d9b1969f1fce3652991107fadef9bd280.
Baseline: the ten-reviewer Fable pass on af1c7e06, recorded in c6e5b4c9.
This is W1C's Codex audit of the fixes/review evidence, NOT a new Fable pass.
W2C's supplemental audit is .ai-team/ab-w2c/fable-delta.md.

All twelve orange findings are closed; no red/orange finding remains open in
Plans/issues/2026-11/2026-11-02.md. Deferred LOWs/Qs remain explicit.

| Item | Landing | Independent review and meaningful proof |
|---|---|---|
| G1 |88a1b6dc0.9.295| W1B read; actual workflow step executed with gh shim, absent/create and present/upload--clobber argv asserted; old step red. No GitHub publication proved locally. |
| B1 |307ec8620.9.296| W1B author/lead review; real tokenless enrol request tests, all other commands still require credentials; enrolment fixture green. |
| Q1 |a115609c0.9.298| Lead1 reviewed two image-layer fixes; actual checksum RUN scratch cases cover aggregate-only, tampered, missing-line and legacy-sidecar layouts. Gate built the multistage image. |
| B2 |2a4cc2330.9.301| Lead1 read. Positive/negative5min provider cache, daemon-wide12uncached/h,429+Retry-After. Original local-file revocation regression fixed; real file deletion refuses immediately, cache-all mutant red, enrolment27/0. Core/API/directory race suites pass; negative-cache/bound mutants assertion-red. |
| D1 |f32f605d0.9.303| W2C read. Go delegates ParseName; helper twins share18accept/30reject vectors. Actual Owner+realm+target/source cases; Go/Python narrow-pattern mutants assertion-red; core/subscription race, Python16/0 and TS44/0. |
| F1 |ac28de6d no bump| Lead1 read: masked stderr forwarding and log readers disclosed. events-off host setting explicitly remains after1.0. |
| I1 |3a3a2b97 no bump| W2C read. Real page must return200 and own tbody row on403/503 summary; production rethrow403 fails new assertion, restored6/0 and typecheck. |
| I2 |0ffd8572 no bump| Lead2 initial read + W2C inactive-view delta. Contract matches source, source authority independent of record visibility, fresh read/summary-only list and distinct card outcomes. Links/docs-order green. |
| A2 |090188600.9.297| Lead1/lead2 read history; live entity resolution, inactive/missing equivalence after ACL. W1C independently checked it against I2's inactive-view clause. |
| A3/A4 |4d0fb2110.9.299| Lead1 review; target admits#bus check follows caller reach. #bus/self and undeliverable Personal target refused; cohort widening parkedQ328. |
| C1 |4b2610a50.9.302| W1C two-MED delta CLEAN: ctx covers3s gate wait/SQL, retirement holds gate across prune+pending removal, rollback retains dirty. No gate→core callback edge; transaction cancellation cannot partially publish. Group commit keeps accepted alerts, not dropped writes. |
| J1 |506d72390.9.300| W1C read: shared FILE_TOOLS, valid decline result, fail-closed unknown identity, sanitized exact-once log.20/0 fake-App-Server cases, four compiling assertion-red mutants. |

J92 test-only5b590ccc fixes quiet-grep/SIGPIPE in guide assertions; W1C read
CLEAN after empty/multiline pattern sweep. Remaining sites are deferredJ93.
Closeouts4f40e3a2/c815ac3f preserve original findings as history and exact landed
SHAs; B2a records authenticated profile paths outside the provider allowance.

Scope audit: af1c7e06..candidate is61files,+2805/-157; code changes match the
routed Fable fixes plus J92. VERSION/CHANGELOG changes are gate-created bumps
0.9.295..0.9.303, no worker version edit. Main is clean; no pending feature work.
Slow-all result is separate evidence at tmp/gate-logs/20261102-R1-final-slow.log;
it is not assumed green by this audit.

Limits: no post-fix Fable/CEO model pass (those accounts are unavailable), no
live deployment, no STABLE commit, no origin operation, no tag or publication.
The first real tag still proves hosted runners, artifacts and release assets;
Mac native published-client check and npm owner login remain later runbook steps.


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
