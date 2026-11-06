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
  - ( ) Relaunch at boot: a user unit starts AB-DVP through `#tmux-sessions` (the normal launcher), then alerts the owner which names have readers 0 🌟
  - ( ) Alert only: after boot, tell the owner (Telegram) which ab-dvp names have readers 0; the owner starts the team ⭐
  - ( ) Neither: the owner starts the team after a reboot, as today
  > 💡👾 Relaunch at boot: windows resume their own sessions and the watchdog catches a deaf face; a deliberate shutdown can stop the unit.
