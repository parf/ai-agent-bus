#!/usr/bin/env python3
"""Run the static essential-operation allowlist for one fast shard."""
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parent
child = None

def stop(_signal, _frame):
    if child is not None:
        try: os.killpg(child.pid, signal.SIGKILL)
        except ProcessLookupError: pass
        child.wait()
    raise SystemExit(143)

def run(check):
    global child
    commands = json.loads((ROOT/'smoke-map.json').read_text())['fast'].get(check)
    if not commands:
        print('FAIL no fast allowlist for '+check)
        return 1
    started = time.monotonic()
    env = dict(os.environ, PYTHONDONTWRITEBYTECODE='1')
    for command in commands:
        print('FAST '+check+': '+' '.join(command['argv']), flush=True)
        child = subprocess.Popen(command['argv'], cwd=ROOT/command.get('cwd',''), env=env,
                                 stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, start_new_session=True)
        try:
            output, _ = child.communicate(timeout=max(.01,15-(time.monotonic()-started)))
        except subprocess.TimeoutExpired:
            os.killpg(child.pid, signal.SIGKILL)
            output, _ = child.communicate()
            print(output, end='')
            print('FAST OVER 15s: '+check+' — report and narrow the allowlist', flush=True)
            return 1
        code = child.returncode
        child = None
        print(output, end='', flush=True)
        argv = command['argv']
        empty = ('bun' == argv[0] and not re.search(r'^\s*[1-9]\d* pass$',output,re.M)) or '[no tests to run]' in output or 'Ran 0 tests' in output
        if code or empty:
            print('FAIL fast command '+check+' (nonzero exit or empty test selection)', flush=True)
            return 1
    elapsed = time.monotonic()-started
    print(f'FAST_TIMING {elapsed:.3f}s {check}\npassed 1, failed 0', flush=True)
    return 0

if __name__ == '__main__':
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    sys.exit(run(sys.argv[1]))
