#!/usr/bin/env python3
"""Run independent Bun test files in three processes per platform shard."""
import concurrent.futures
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import threading
import time

children = set()
mutex = threading.Lock()
stopping = threading.Event()


def stop(_sig, _frame):
    stopping.set()
    with mutex:
        for child in children:
            try:
                os.killpg(child.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
    raise SystemExit(143)


def run(path):
    started = time.monotonic()
    with mutex:
        if stopping.is_set():
            return path, 1, 'pool interrupted', 0
        child = subprocess.Popen(['bun', 'test', str(path), *sys.argv[2:]],
                                 stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                 text=True, start_new_session=True)
        children.add(child)
    try:
        try:
            output, _ = child.communicate(timeout=120)
        except subprocess.TimeoutExpired:
            os.killpg(child.pid, signal.SIGKILL)
            output, _ = child.communicate()
            output += '\nerror: file exceeded 120s\n'
        return path, child.returncode, output, time.monotonic() - started
    finally:
        with mutex:
            children.discard(child)


if __name__ == '__main__':
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    files = sorted(Path(sys.argv[1]).glob('*.test.ts'))
    if not files:
        sys.exit('error: no Bun test files')
    passed = failed = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
        for path, code, output, seconds in pool.map(run, files):
            print(f'FILE_TIMING {seconds:.3f}s {path}', flush=True)
            counts = [re.findall(r'^\s*(\d+) '+kind+r'$', output, re.M) for kind in ('pass', 'fail')]
            p, f = [int(values[-1]) if values else 0 for values in counts]
            passed += p
            failed += f
            if code and not f or not p and not f and not sys.argv[2:]:
                failed += 1
                output += '\nerror: file exited without counted assertions\n'
            # Keep named assertions on failures; passing-case durations remain
            # available for diagnosis, without interleaved worker output.
            print(output, end='', flush=True)
    if not passed:
        failed += 1
        print('error: no test matched')
    print(f'\n {passed} pass\n {failed} fail', flush=True)
    sys.exit(bool(failed))
