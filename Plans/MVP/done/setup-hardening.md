# Setup hardening

📌 **TL;DR:** 0.8.21 closes H.10 from the 0.7 pre-STABLE review. Setup over a
running node applies a changed unit or release by a restart, waits for that
release and build, and reports the durable Owner; `--reinstall` refuses a
daemon holding its home outside systemd and rolls a midway failure back.

## Result

The current behaviour is in [setup § install](../../../docs/09-setup.md#install)
and the [reinstall procedure](../0.7-cutover.md#procedure).

## Checks

| Claim | Check | Mutant that failed it |
|---|---|---|
| The drop-ins move aside | `agent-bus-setup` TestSetAsideMovesHomeAndDropInsIntoARootOnlyDirectory | drop-in move removed: "drop-in directory still holds [override.conf]" |
| The set-aside directory is root-only | the same test, under umask 0 | chmod to 0755: "set-aside directory mode -rwxr-xr-x, want 0700"; the 0700 `Mkdir` alone is redundant with the chmod and survives |
| A rollback restores home and drop-ins and keeps what the failed install wrote | TestRestoreAsidePutsTheNodeBack | each of the three moves removed |
| A process holding the home is found | TestHoldersNamesAProcessWithTheHomeOpen | no pid recorded; prefix without `/` |
| A version without `build_info` is refused | TestProgramVersionReadsTheBuildStamp | prefix check removed |
| A daemon outside systemd blocks the reinstall | reinstall gate: a hand-started `agent-busd` holds the database | holder check removed: "reinstall proceeded while a daemon outside systemd held the database" |
| A midway failure leaves a running node | reinstall gate: a directory in place of the logrotate rule | rollback removed: "a failed reinstall left the node stopped" |
| The node reports the archive's build | reinstall and fresh-install gates compare `/identity` and each command's `build_info` with the archive's | identity reports an unstamped build: setup's own wait refuses it first; with that comparison also removed, fresh "node build_info is not the package's" and reinstall "the node does not report the archive's build_info" |
| Setup over a running node reports the durable Owner | fresh-install gate: owner transferred to alice@fresh, setup run with `--owner owner@fresh` | seed printed: "setup did not report the durable Owner" |
| A changed unit is applied and waited for | fresh-install gate: setup with `--addr 127.0.0.1:6768`, then one look at 6768 | no restart: "setup over the running node with a changed unit failed", its own wait timing out; no wait: "setup returned before the daemon answered on its changed address" |

Slow smoke 783/0 before the gate mutants. The reinstall gate ran green over
0.6.17, with this change packaged as 0.8.20 before it was renumbered 0.8.21
on rebase. The fresh-install gate passes every check up to its
installed-browser step. That step, `installed-browser.py`, is stale against
the 0.8 web: `/channels` is now Queues, so it fails before the call and
browser-roles checks. F.12 owns it.
