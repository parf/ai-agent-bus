# Final Fable delta — team 2

Baseline: Fable review of af1c7e06 (0.9.294). Freeze candidate: c815ac3f (0.9.303), tree c7de8b7d9b1969f1fce3652991107fadef9bd280. No new provider calls were used for this summary.

| Item | Landing | Result and proof |
|---|---|---|
| A2 | 09018860 / 0.9.297 | Log/subscription calls resolve live entities; inactive names and Users cannot expose private logs. |
| A3+A4 | 4d0fb211 / 0.9.299 | Reject #bus and targets that cannot admit #bus; Personal cohort unchanged. Check caller reach first, preventing a target-side oracle. Full hidden-vs-absent reply equality; caller-check-removal mutant assertion-red. |
| C1 | 4b2610a5 / 0.9.302 | Global10/s burst20 snapshot bucket, one transaction across dirty rings, ~100ms group commit; dropped unchanged. Clean stop drains within3s, warns with unsaved-ring count on failure. SQLite contexts, retirement prune serialized with pending snapshots, rollback retains alerts. 100logs20burst/<=30transactions over1s,1,100alerts retained. Real daemon40/40 alerts survive same-SQLite SIGTERM/restart. Initial10+delta6 compiling assertion-red mutants; race suites green. C3 restart-test gap covered; optional-port fallback not changed. |
| J1 | 506d7239 / 0.9.300 | Codex pushed turns decline FILE_TOOLS and unusable/missing tool identity, with sanitized once-only logs. Launcher prompts only FILE_TOOLS. Fixture12pass/8fail before,20/0 after;4 typechecked assertion-red mutants;22 related unit tests green. |
| J92 | 5b590ccc / no bump | Shared guide has/lacks use here-strings, avoiding producer SIGPIPE under pipefail. Old1MiB early-match pipeline returns141; real-helper regression red before/green after. Remaining quiet-grep/early-head candidates are deferred J93. |

Docs: closeout3 landed4f40e3a2. Four FR rows completed; A2/A3/A4/C1/J1 original findings retained in done history with landing evidence. J93 remains open/deferred, not a release blocker. Local runbook step7 now expects8archives+SHA256SUMS+2Darwin-client npm tarballs, create-or-upload; archive verifier inputs exclude npm tgz; step8 selects explicit Linux x86_64 ARCHIVE. Version commands use --version. DB agent is not claimed in release archives.

Reads sent CLEAN to both leads: D1 d9359e75 (Go/Py/TS18accepted/30rejected canonical-name vectors; scratch TS uppercase mutant assertion-red, restored122expectations green); I1 64fdc069 (real tbody row, pre-pagination q,8expectations green); I2 delta268340ff..282e1964 (inactive view clause matches A2 and rendered messages, no runs).

No live deploy, publish, tag, origin operation or extra provider call. Await/perform only the authorized frozen-tip slow-all run; no retry on red. Final slow result is separate evidence, not assumed by this summary.

## Corrected final release check

Authorized frozen tip06fabb3c6de88626dc341ea15284bdd1412f4544/tree82388f930a03213fd9f26d074a2ca722e7c8368e after J94+closeout: plain setsid --slow all GREEN1305passed/0failed,rc0,91.976s on2026-11-02 22:30:30–22:32:02UTC. Own lock released after zero live descendants; tree removed. All four prior failing top-level checks now pass, including full OwnersFlow. No skipped-section line reported. Raw new log/status under tmp/gate-logs/20261102-R1-final-slow2; prior red/confounded run retained separately. No release/publish/tag action performed.
