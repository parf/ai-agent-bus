"""Run fresh-install preflight with local command stubs, never a container."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class FreshInstall(unittest.TestCase):
    def test_paths_and_images_on_both_cpus_with_and_without_realpath(self):
        realpath = shutil.which('realpath')
        self.assertIsNotNone(realpath)
        (ROOT / 'tmp').mkdir(exist_ok=True)
        for cpu, base in (('x86_64', 'docker.io/library/archlinux:latest'),
                          ('arm64', 'docker.io/library/debian:13'),
                          ('aarch64', 'docker.io/library/debian:13')):
            for fallback in (False, True):
                with self.subTest(cpu=cpu, fallback=fallback), tempfile.TemporaryDirectory(dir=ROOT / 'tmp') as tmp:
                    work = Path(tmp)
                    tools = work / 'bin'
                    tools.mkdir()
                    # Model macOS's realpath: no options. The second mode
                    # also models a host that cannot resolve missing parents.
                    (tools / 'realpath').write_text('#!/bin/sh\ncase "$1" in -*) exit 98;; esac\n' +
                        ('exit 1\n' if fallback else 'exec "' + realpath + '" "$@"\n'))
                    (tools / 'uname').write_text('#!/bin/sh\nprintf "%s\\n" ' + cpu + '\n')
                    (tools / 'podman').write_text('#!/usr/bin/env python3\nimport json,os,sys\n'
                        'from pathlib import Path\nPath(os.environ["CALLS"]).write_text(json.dumps(sys.argv[1:]))\n'
                        'sys.exit(77)\n')
                    for p in tools.iterdir():
                        p.chmod(0o755)
                    (work / 'archive.tar.gz').write_bytes(b'archive fixture')
                    (work / 'archive.tar.gz.sha256').write_text('checksum fixture\n')
                    # Both existing archive and new nested output resolve
                    # through a physical path when the caller uses a symlink.
                    (work / 'link').symlink_to(work, target_is_directory=True)
                    env = {k: v for k, v in os.environ.items() if k != 'FRESH_INSTALL_IMAGE'}
                    env.update(PATH=str(tools) + ':' + os.environ['PATH'], CALLS=str(work / 'calls'))
                    r = subprocess.run(['bash', str(HERE / 'fresh-install.sh'),
                        'link/archive.tar.gz', 'link/new/nested/output'], cwd=work,
                        env=env, capture_output=True, text=True, timeout=5)
                    self.assertEqual(r.returncode, 77, r.stdout + r.stderr)
                    calls = json.loads((work / 'calls').read_text())
                    self.assertEqual(calls[0], 'build')
                    self.assertIn('--pull=missing', calls)  # first build needs no manually cached base
                    self.assertIn('BASE=' + base, calls)
                    out = work / 'new/nested/output'
                    self.assertEqual((out / 'package/archive.tar.gz').read_bytes(), b'archive fixture')
                    self.assertEqual((out / 'package/archive.tar.gz.sha256').read_text(), 'checksum fixture\n')
                    self.assertTrue((out / 'fixture/run.sh').is_file())
                    self.assertTrue((out / 'fixture/agent-runtime.py').is_file())


class DaemonWindow(unittest.TestCase):
    """J59: the fixture spaces its daemon-changing setup runs by the unit's own start limit."""
    script = (HERE / 'fresh-install-container.sh').read_text()

    def function(self):
        start = self.script.index('daemon_window() {')
        return self.script[start:self.script.index('\n}\n', start) + 3]

    def run_window(self, work, interval, starts, burst='5'):
        tools = work / 'bin'
        tools.mkdir()
        log = work / 'calls'
        # The journal answers `starts` lines at first, one fewer on every sleep.
        (tools / 'journalctl').write_text('#!/bin/sh\necho "$*" >> "$LOG"\nn=$(cat "$COUNT")\n'
            'i=0; while [ $i -lt $n ]; do echo "Started agent-busd.service - x"; i=$((i+1)); done\n'
            'echo "Stopped agent-busd.service - x"\n')
        (tools / 'sleep').write_text('#!/bin/sh\necho sleep >> "$LOG"\nn=$(cat "$COUNT"); [ $n -gt 0 ] && echo $((n-1)) > "$COUNT"; exit 0\n')
        (tools / 'systemctl').write_text('#!/bin/sh\ncase "$3" in\n  StartLimitIntervalUSec) echo "$INTERVAL" ;;\n  *) echo "$BURST" ;;\nesac\n'
)
        (tools / 'systemd-analyze').write_text('#!/bin/sh\nsecs=$(echo "$2" | sed "s/s$//")\n'
            'echo "Original: $2"; echo "       μs: $((secs * 1000000))"; echo "   Human: $2"\n')
        for p in tools.iterdir():
            p.chmod(0o755)
        (work / 'count').write_text(str(starts))
        env = dict(os.environ, PATH=str(tools) + ':' + os.environ['PATH'], LOG=str(log), COUNT=str(work / 'count'),
                   INTERVAL=interval, BURST=burst)
        r = subprocess.run(['bash', '-c', 'set -euo pipefail\n' + self.function() + '\ndaemon_window'],
                           env=env, capture_output=True, text=True, timeout=20)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        return log.read_text().splitlines() if log.exists() else []

    def test_waits_while_the_window_is_full_and_reads_the_interval_from_the_unit(self):
        with tempfile.TemporaryDirectory(dir=ROOT / 'tmp') as tmp:
            calls = self.run_window(Path(tmp), '20s', 4)
        self.assertEqual(calls.count('sleep'), 1)  # 4 starts fill burst-1; one wait empties it
        self.assertTrue(all('--since -20s' in c for c in calls if c != 'sleep'), calls)

    def test_does_not_wait_when_the_window_has_room_or_the_limit_is_off(self):
        with tempfile.TemporaryDirectory(dir=ROOT / 'tmp') as tmp:
            self.assertEqual(self.run_window(Path(tmp), '10s', 3).count('sleep'), 0)
        with tempfile.TemporaryDirectory(dir=ROOT / 'tmp') as tmp:
            self.assertEqual(self.run_window(Path(tmp), '0s', 9), [])

    def test_every_daemon_changing_setup_run_waits_and_nothing_resets_the_unit(self):
        lines = self.script.splitlines()
        runs = [i for i, l in enumerate(lines) if l.startswith('./agent-bus-setup --owner owner@fresh') and '--key' not in l]
        self.assertEqual(len(runs), 6, runs)
        for i in runs:
            self.assertEqual(lines[i - 1], 'daemon_window', lines[i])
        self.assertNotIn('reset-failed', self.script)


if __name__ == '__main__':
    unittest.main()
