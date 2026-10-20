"""The pool must run every file, report a failure, and stay process-isolated."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class Pool(unittest.TestCase):
    def test_all_files_overlap_and_failures_are_counted(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            fake = root / 'bun'
            fake.write_text('''#!/usr/bin/env python3
import json,os,sys,time
from pathlib import Path
root=Path(os.environ['POOL_FIXTURE']);name=Path(sys.argv[2]).name
(root/(name+'.started')).touch()
end=time.monotonic()+3
while len(list(root.glob('*.started')))<3 and time.monotonic()<end:time.sleep(.005)
if len(list(root.glob('*.started')))<3:print(' 1 fail');sys.exit(1)
with (root/'ran').open('a') as f:f.write(name+'\\n')
if name=='bad.test.ts':print(' 1 fail');sys.exit(1)
print(' 1 pass\\n 0 fail')
''')
            fake.chmod(0o755)
            names = ['a.test.ts', 'b.test.ts', 'bad.test.ts', 'last.test.ts']
            for name in names:
                (root / name).touch()
            result = subprocess.run([sys.executable, str(Path(__file__).with_name('smoke-agent-pool.py')), str(root)],
                                    env={**os.environ, 'PATH': str(root)+os.pathsep+os.environ['PATH'],
                                         'POOL_FIXTURE': str(root)}, capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(sorted((root/'ran').read_text().splitlines()), sorted(names))
            self.assertIn(' 3 pass\n 1 fail', result.stdout)
            self.assertEqual(result.stdout.count('FILE_TIMING'), 4)
