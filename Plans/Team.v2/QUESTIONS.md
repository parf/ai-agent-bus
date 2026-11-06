# Team v2 questions

## Open questions

Only unresolved choices for [Team v2](README.md#team-v2).

- ❓ Q216 Who may restart a stuck worker session
  - Details: restarting another session is the owner's own decision today (`CLAUDE.local.md`); a lead or `#dir` needs an explicit grant on the tmux runner to do it
  - Settled by: owner
- ❓ Q217 The receipt and the board
  - Details: what a receipt carries upward and how the board is stored on the bus
  - Format: to approve before any code, only when the owner asks
  - Settled by: owner
- ❓ Q343 After a host reboot, relaunch the team or only alert
  - Context: the host powered off on 2026-11-03 at 00:59 EST; nothing relaunched AB-DVP until 11-05 20:07 and nothing alerted, so the team was off the bus for ~2.6 days (J107). Since 1.0.26 the launcher watchdog flags a window that comes up without its bus face within 20 s
  - ( ) Relaunch at boot: a user unit, ordered after the runner, asks `#tmux-sessions/ab-dvp` to start AB-DVP once the bus is up; each window resumes its session through the normal launcher and the watchdog flags one without its face. A flag file (`~/.config/ab-dvp/no-autostart`) skips it after a deliberate shutdown 🌟
  - ( ) Alert only: at boot a unit tells the owner AB-DVP is down, by desktop notification or mail (a live send), since `#ab-ceo` is down too; the team stays idle until someone starts it ⭐
  - ( ) Neither: the owner starts the team after a reboot, as today
  > 💡👾 Relaunch at boot: windows resume their own sessions and the watchdog catches a deaf face; a deliberate shutdown can stop the unit.

## Less overhead (2026-11-06 review)

Facts, 2026-11-05 18:00 → 11-06 01:40: 338 commits, of which 47 version bumps, 47 changelog edits and 41 issue-file commits; 107 gate runs, at least 3 of them green only on a rerun. Of about 35 landed items, roughly a third were real bugs or features (T-38, J104–J106, B6, B7, C2-E2, F1, F2, J90, A5), a third polish or edge cases (F4, F5, F7, K1–K3, G2, J4, D2/D3, B4/B4b, A8, G5, N-docs), and a third test plumbing (J79, J93, J108, J108b, J110, J112, J114). Each small item still drew two readers, mutants, a gate, a version bump, a changelog line and a separate close-out commit.

- ❓ Q348 Which findings become tasks
  - ( ) Only a bug the owner or a user would hit, a security gap, or a MED+ finding; LOW and polish findings go to one list in the plan and are fixed when their file is touched anyway 🌟
  - ( ) Every finding from an audit or recheck table becomes a task, as now ⭐
  > 💡👾 Owner-visible bugs and MED+ only: half of tonight's items were LOW wording, examples and edge cases.

- ❓ Q349 How many reads a change gets
  - ( ) Risk-based: a ⚠️ subsystem (authority, tokens, storage, parsers, runner, install) keeps two readers and a mutant; everything else gets one reader and no mutant; QA reads ⚠️ only 🌟
  - ( ) Two readers and mutants for every behaviour change, as now ⭐
  > 💡👾 Risk-based: the readers are the scarcest quota, and a wording fix does not need two of them.

- ✅ Q350 → [TD6](DECISIONS.md#TD6)

- ❓ Q351 Close-out and issue bookkeeping
  - ( ) The issue/TODO update rides in the landing commit; no separate close-out commit or gate 🌟
  - ( ) A separate close-out commit after each landing, as now ⭐
  > 💡👾 Ride the landing: 41 issue-file commits tonight, each needing its own pass.

- ❓ Q352 Flaky gates
  - ( ) A check that goes green only on rerun is quarantined at once (skipped with a J-item) instead of being rerun; the gate never reruns silently 🌟
  - ( ) Rerun and diagnose in parallel, as now ⭐
  > 💡👾 Quarantine: each rerun costs a full gate, about 90 s at load.
