> **Latest takeover state (W1C, 2026-11-02 17:33):** all Fable blockers and
> close-outs landed. Frozen main `06fabb3c` (0.9.303), tree
> `82388f930a03213fd9f26d074a2ca722e7c8368e`; corrected final slow **1305/0**,
> rc0,87.804s driver. First slow1301/4 retained with diagnosed wrapper/fixture
> causes; J94 test-only correction landed. Only W1C/W2C have quota. Stamped
> daemon/CLI binaries staged, no live deployment/STABLE/push/tag/publication.
> Current details: `.ai-team/ab-w1c/R1-final-state.local.md`; pending CEO-only live
> restart authority and final unavailable CEO/Fable delta-review decision.

# R1 (1.0.0) release handoff — #ab-dir, 2026-11-02 ~17:02 EST

> **Update 2026-11-05: R1 IS RELEASED.** STABLE `5e5fcccc` (11-04 02:03), VERSION 1.0.0, tag `v1.0.0`, CHANGELOG.0.9 moved to Plans/, CHANGELOG.1.0.md started; archives published and container acceptance clean (`8864d6d0`), public npm packages verified (`97fa11f8`); main = origin/main `97fa11f8` (pushed). Everything below is the 11-02 snapshot, kept as history.

Main: `4d0fb211`, VERSION `0.9.299`. Tree is FROZEN since T0 `af1c7e06` (0.9.294); only the Fable-review fixes below may land.

## Where the runbook stands

| Step | State |
|---|---|
| 0 Freeze | ✅ 16:22, T0 `af1c7e06` |
| 1 `--slow all` on T0 | ✅ 1305/0, rc 0, 90 s (W1B), log `tmp/gate-logs/20261102-162237-ab-w1b_R1-freeze.log` |
| 2 Fable review | ✅ `c6e5b4c9` → 0 🔴, 12 🟠, 42 ⚪/🔵; R1 TODO block "## 1.0 final review" (FR.*) |
| 2b Fix the 12 🟠 | ⏳ in progress (table below) |
| 2c Batched deploy | ⏳ after the last fix lands — daemon + CLI (+ runner if touched), ONE daemon restart, hold both leads first |
| 1' `--slow all` again | ⏳ code changed after T0 → rerun (W1C or whoever has quota; lock first, setsid, kill→release→remove) |
| 2' CEO delta review | ⏳ |
| 3+ STABLE commit (gate stable mode, VERSION 1.0.0, CHANGELOG.0.9 → Plans/), changelog fold + push (#ab-ceo), deploy, tag v1.0.0 (owner go), docker push (runbook step 8 now unblocked by Q1; runbook still needs the `ARCHIVE=` line — W2C), npm publish (owner login), Mac check on published 1.0.0, release note | ⏳ owner steps are the long poles |

## The 12 🟠 fixes

| Item | What | Writer | State |
|---|---|---|---|
| G1 | release.yml create-or-upload | W1C | ✅ `88a1b6dc` 0.9.295 |
| B1 | enrol tokenless | W1B | ✅ `307ec862` 0.9.296 — CLI deploy-ready, batched |
| A2 | inactive logs = missing for every caller | W2B | ✅ `09018860` 0.9.297 — daemon, batched |
| Q1 | Containerfile verifies SHA256SUMS | W1C | ✅ `a115609c` 0.9.298 |
| A3+A4 | refuse sub targets that can't admit #bus, and #bus itself (option 2; widening parked as Q328 R1.1) | W2B→W2C | ✅ `4d0fb211` 0.9.299 — daemon, batched |
| B2 | enrol lookup cache (pos+neg 5 min) + 12 uncached/h daemon-wide, 429 | W1C | `329c8ebc` gate RED x2 (17:03): cache delayed revocation for LOCAL file directories → W1C: cache/bound GitHub realms only, local uncached, test + mutant, ~17:20 (D1 paused); daemon+CLI batched, B1 deploy-ready waits for it. LOW post-1.0: authenticated profile paths bypass the allowance |
| J1 | Codex pusher declines FILE_TOOLS | W2C | built, green 20/0, mutants running; read by W1C ~17:30 |
| C1 | alert-ring group commit (bucket bounds snapshots, ≤100 ms crash window, flush on stop) | W2C | `b9652aeb` back from W1C read with 2 MED (bounded shutdown drain ~3 s; invalidate pending before prune) → ~17:50 |
| D1 | one name grammar via protocol.ParseName + shared vector Go/Py/TS | W1C | queued after B2 (~17:35) |
| F1 | security.md: masked stderr reaches the #bus log (docs only); `"events": "off"` switch → 1.0.1 | W1C | queued (~17:42) |
| I1 | web 403 test asserts the page renders | W1C | queued (~17:55) |
| I2 | web-face/records.md Events card | W1C | queued (~18:05) |

Also: J92 (test-only, no bump) — `printf | grep -q` under pipefail → 141 random reds in agent_guide; W2C, ~17:20, with a sweep. Expected last landing ~18:15.

Close-outs: each team flips its FR.* rows in ONE close-out commit after its fixes (team 2: FR.A2, A3, C1, J1 + J92 entry; team 1: the rest + the B2 LOW as ⚪).

## Capacity — the real constraint

| Window | State |
|---|---|
| W1B, W2B (Claude) | OUT: 5h ~1–2% until ~20:30–20:40, weekly 1% (resets in ~2d15h) |
| W1C, W2C (Codex) | ~19–20% weekly; carry all remaining writes. Codex content filter may block authority/secret code (J85) — stop-and-report, then decide 1.0.1 |
| QA1, QA2 | out until Tue 11-03 18:26 EST |
| W1A, W2A | out until 11-05 |
| Leads | do authority reads as authorized one-offs (lead1 reads team 2's A*, lead2 reads team 1's B*) |

1.0.1 candidates if quota runs short (tell #ab-ceo before dropping): I1, I2; B2 only if blocked (it wasn't). Any authority-level finding in CEO's delta review has no Claude writer until a reset.

## Rules in force today

- Freeze: nothing lands but routed FR fixes.
- Every "ready" report shows `git diff --stat main...<branch>` (A3's stale worktree once reverted G1 + VERSION).
- Burner rule: kill load/burner processes by pid and confirm BEFORE releasing the lock; no loops while a gate runs.
- Smoke cap: B/C/D kept (GOMAXPROCS=6, -p 4, cargo 4 jobs, fixture once); CPU quota off by default (`SMOKE_CPU_PERCENT`).
- Lock first, alone; Go landings → "deploy-ready <sha> <program>"; daemon deploys hold both leads.
- Never fetch/pull/push origin; only #ab-ceo pushes.

## Open after 1.0.0 (parked)

Q323 (reply ACL), Q326 (#eventlog creation-time), Q328 (Personal cohort admits #bus), J90 telegram getMe retry, F1 events-off switch, B2 profile-path LOW, agent_guide restart flap (cause unconfirmed; assertion now catches it), gate.sh load wait (<12), -race on changed packages only (R1.1 Q), DS.1 DBSync re-estimate.

Detailed running log: `.ai-team/dir/handoff.md`.

## Live takeover update — 2026-11-02, #ab-w1c

Owner says every interactive agent except W1C/W2C is out of usage. W1C now
coordinates release; W2C supplies supplemental work/reviews. No fetch/push.

- Main now `0ffd8572`, VERSION `0.9.303`.
- B2 fixed with provider-only cache/budget (local files uncached); landed
  `2a4cc233` 0.9.301. Local-revocation mutant red; enrolment shard 27/0 green.
- J1 landed `506d7239` 0.9.300; J92 `5b590ccc` no bump.
- C1 including both review fixes landed `4b2610a5` 0.9.302. W1C read clean:
  context-bounded drain; retirement gate excludes stale snapshot/prune race.
- D1 landed `f32f605d` 0.9.303; W2C CLEAN, mutants + owning suites green.
- F1 landed `ac28de6d` no bump. I1 `3a3a2b97` no bump. I2 `0ffd8572`
  no bump, including inactive-view clause; W2C delta CLEAN.
- Remaining: W2C closeout3 (team2 status/issues + deferred J93); W1C FR-close
  (team1 status/issues + profile-path quota follow-up). Gate closeouts serially.
- W2C updates LOCAL release runbook for current Linux/Darwin assets, existing
  release upload path, explicit ARCHIVE= for image step; no publish/runtime run.
- After closeouts: freeze exact tip/tree; one locked slow-all run, final delta
  review. No STABLE/tag/publication yet. CEO-only push, owner tag go and npm
  credentials remain explicit boundaries; no new authority inferred from takeover.


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
