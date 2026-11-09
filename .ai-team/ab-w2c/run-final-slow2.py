"""Plain setsid release check. Prepare only; require an explicitly authorized tip."""
from pathlib import Path
import datetime
import json
import os
import signal
import subprocess
import sys
import time

REPO = Path('/usr/local/src/ai-agent-bus')
WORKTREE = REPO / 'tmp/ab-w2c-r1-final-slow2'
EVIDENCE = REPO / 'tmp/gate-logs/20261102-R1-final-slow2'


def process_table():
    table = {}
    for entry in Path('/proc').iterdir():
        if not entry.name.isdigit():
            continue
        try:
            raw = (entry / 'stat').read_text()
            fields = raw[raw.rfind(')') + 2:].split()
            table[int(entry.name)] = (int(fields[1]), fields[0], fields[19])
        except (OSError, ValueError, IndexError):
            continue
    return table


def run(authorized_tip):
    tip = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=WORKTREE, text=True).strip()
    tree = subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], cwd=WORKTREE, text=True).strip()
    expected = subprocess.check_output(['git', 'rev-parse', authorized_tip], cwd=REPO, text=True).strip()
    if tip != expected:
        raise SystemExit('worktree does not match authorized tip')
    if subprocess.check_output(['git', 'status', '--porcelain'], cwd=WORKTREE, text=True):
        raise SystemExit('worktree is not clean')
    for suffix in ('.log', '.status.json', '.pid'):
        if EVIDENCE.with_suffix(suffix).exists():
            raise SystemExit(f'refusing to replace evidence: {EVIDENCE.with_suffix(suffix)}')

    seen = {}
    interrupted = False
    started = time.monotonic()
    begin = datetime.datetime.now(datetime.timezone.utc).isoformat()

    def track():
        table = process_table()
        # Continue tracking recorded living descendants even if they reparent.
        family = {os.getpid()} | {
            pid for pid, born in seen.items()
            if pid in table and table[pid][2] == born
        }
        while True:
            added = {pid for pid, (parent, _, _) in table.items() if parent in family}
            if added <= family:
                break
            family |= added
        for pid in family - {os.getpid()}:
            if pid in table:
                seen[pid] = table[pid][2]
        return table

    def live_recorded():
        table = track()
        return [pid for pid, born in seen.items()
                if pid in table and table[pid][2] == born and table[pid][1] != 'Z']

    def signal_recorded(sig):
        # Verify starttime immediately before each PID signal; no pattern kill.
        for pid in live_recorded():
            try:
                table = process_table()
                if pid in table and table[pid][2] == seen[pid] and table[pid][1] != 'Z':
                    os.kill(pid, sig)
            except ProcessLookupError:
                pass

    def interrupt(_sig, _frame):
        nonlocal interrupted
        interrupted = True
        signal_recorded(signal.SIGTERM)

    signal.signal(signal.SIGTERM, interrupt)
    signal.signal(signal.SIGINT, interrupt)
    with EVIDENCE.with_suffix('.log').open('x') as log:
        command = 'setsid --wait bash -c "exec src/smoke.sh --slow all"'
        log.write(f'TIP {tip}\nTREE {tree}\nCOMMAND {command}\nSTART {begin}\nWRAPPER plain setsid; no child-subreaper or orphan-reaping changes\n')
        log.flush()
        script = f'printf "%s\\n" "$$" > {EVIDENCE.with_suffix(".pid")}; exec src/smoke.sh --slow all'
        child = subprocess.Popen(['setsid', '--wait', 'bash', '-c', script], cwd=WORKTREE,
                                 stdout=log, stderr=subprocess.STDOUT)
        while child.poll() is None:
            track()
            if interrupted:
                break
            time.sleep(.5)
        left = live_recorded()
        initial_left = left.copy()
        if left:
            signal_recorded(signal.SIGTERM)
            until = time.monotonic() + 3
            while live_recorded() and time.monotonic() < until:
                time.sleep(.1)
            signal_recorded(signal.SIGKILL)
            until = time.monotonic() + 3
            while live_recorded() and time.monotonic() < until:
                time.sleep(.1)
        # Wait only for the direct setsid child; orphan reaping remains the
        # responsibility of the ordinary system parent, exactly as plain setsid.
        status = child.wait(timeout=10)
        result = dict(tip=tip, tree=tree, command=command, started=begin,
                      ended=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                      wall_seconds=round(time.monotonic() - started, 3), exit_status=status,
                      interrupted=interrupted, descendants_left_at_exit=initial_left,
                      own_background_left=live_recorded(), child_subreaper=False)
        EVIDENCE.with_suffix('.status.json').write_text(json.dumps(result, indent=2) + '\n')
        log.write('\nFINAL ' + json.dumps(result) + '\n')
        log.flush()
        print(json.dumps(result), flush=True)


if __name__ == '__main__':
    if len(sys.argv) != 2:
        raise SystemExit('requires an explicitly authorized exact frozen tip')
    run(sys.argv[1])
