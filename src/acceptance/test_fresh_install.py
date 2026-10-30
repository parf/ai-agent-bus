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


if __name__ == '__main__':
    unittest.main()
