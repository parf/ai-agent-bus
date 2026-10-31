"""Fault injection against the real guard; no podman, sudo or live bus."""
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import tempfile
import time
import unittest

HERE = Path(__file__).resolve().parent


class Liveness(unittest.TestCase):
    def setUp(self):
        scratch = HERE.parents[1] / 'tmp'
        scratch.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix='guard-liveness-', dir=scratch)
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.bin = self.work / 'bin'
        self.bin.mkdir()
        self.base = self.work / 'base'
        self.base.mkdir()
        self.live = self.base / 'run.live'
        (self.live / 'source.git').mkdir(parents=True)
        (self.live / 'source.git/HEAD').write_text('the live repository')
        stat = Path('/proc/self/stat').read_text().rsplit(') ', 1)[1].split()
        self.owner = f'{os.getpid()}-{stat[19]}'
        (self.live / 'owner').write_text(self.owner)
        self.calls = self.work / 'calls'
        self.calls.touch()
        self.faults = self.work / 'faults'
        self.faults.touch()
        real_sed = shlex.quote(shutil.which('sed'))
        self.script('sed', '#!/bin/sh\ncase "$*" in *"/proc/$FAULT_PID/stat"*) echo failed >>"$FAULTS"; exit 1;; esac\nexec ' + real_sed + ' "$@"\n')
        self.script('agent-bus', '#!/bin/sh\necho "smoke-gate@parf.us slow fixture 14m left"\n')
        self.script('podman', '''#!/usr/bin/env python3
import os, pathlib, shutil, sys
with open(os.environ['CALLS'], 'a') as log: log.write(sys.argv[1] + '\\n')
if sys.argv[1] == 'unshare':
    assert sys.argv[2:4] == ['rm', '-rf'], sys.argv
    path = pathlib.Path(sys.argv[4])
    assert path.is_relative_to(pathlib.Path(os.environ['WORK']))
    shutil.rmtree(path, ignore_errors=True)
''')
        self.env = dict(os.environ, PATH=str(self.bin) + ':' + os.environ['PATH'],
                        FAULT_PID=str(os.getpid()), FAULTS=str(self.faults), CALLS=str(self.calls), WORK=str(self.work))

    def script(self, name, text):
        path = self.bin / name
        path.write_text(text)
        path.chmod(0o755)

    def test_sweep_keeps_live_mounted_repository_when_stat_read_fails(self):
        run = self.base / 'run.new'
        run.mkdir()
        result = subprocess.run(['bash', '-c', 'podman=("$3"); . "$1"; guard "$2" keep; echo guarded',
                                 'fixture', str(HERE / 'guard.sh'), str(run), str(self.bin / 'podman')],
                                env=self.env, capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        # Let the child's watcher finish before removing its fake command.
        deadline = time.monotonic() + 3
        while self.calls.read_text().count('ps\n') < 2 and time.monotonic() < deadline:
            time.sleep(.02)
        self.assertGreaterEqual(self.calls.read_text().count('ps\n'), 2, 'watcher never finished')
        self.assertTrue((self.live / 'source.git/HEAD').is_file(),
                        'dead-run sweep deleted a live mounted repository after /proc stat read failed')

    def test_watcher_keeps_live_mounted_repository_when_stat_read_fails(self):
        result = subprocess.run(['bash', '-c', '. "$1"; printf %s "$GUARD_WATCH"',
                                 'fixture', str(HERE / 'guard.sh')], env=self.env,
                                capture_output=True, text=True, timeout=5, check=True)
        watcher = subprocess.Popen(['sh', '-c', result.stdout, 'fixture', self.owner,
                                    str(self.live), '', str(self.bin / 'podman')],
                                   env=self.env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            # Wait for a classification: old code exits after cleanup; correct
            # code reaches a second failed read while the same owner stays live.
            deadline = time.monotonic() + 5
            while watcher.poll() is None and self.faults.read_text().count('failed\n') < 2 and time.monotonic() < deadline:
                time.sleep(.02)
            self.assertTrue(watcher.poll() is not None or self.faults.read_text().count('failed\n') >= 2,
                            'watcher never classified the injected stat failure')
            self.assertTrue((self.live / 'source.git/HEAD').is_file(),
                            'watcher deleted a live mounted repository after /proc stat read failed')
            self.assertIsNone(watcher.poll(), 'watcher stopped treating the live owner as live')
        finally:
            if watcher.poll() is None:
                watcher.terminate()
            watcher.wait(timeout=3)


if __name__ == '__main__':
    unittest.main()
