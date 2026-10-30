"""A runner upgrade replaces public code, including obsolete nested modules."""
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]


class Package(unittest.TestCase):
    def setUp(self):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        temp = tempfile.TemporaryDirectory(dir=ROOT / 'tmp', prefix='sessions-package-')
        self.addCleanup(temp.cleanup)
        self.work = Path(temp.name)

    def package(self, dest, **kwargs):
        return subprocess.run([str(HERE / 'package.sh'), str(dest)],
                              capture_output=True, text=True, timeout=10, **kwargs)

    def snapshot(self, dest):
        return {str(p.relative_to(dest)): (p.stat().st_mode & 0o777,
                hashlib.sha256(p.read_bytes()).hexdigest() if p.is_file() else None)
                for p in dest.rglob('*')}

    def test_upgrade_replaces_stale_files_with_complete_shipped_tree(self):
        fresh, upgrade = self.work / 'fresh', self.work / 'upgrade'
        result = self.package(fresh)
        self.assertEqual(result.returncode, 0, result.stderr)
        files = {str(p.relative_to(fresh)) for p in fresh.rglob('*') if p.is_file()}
        self.assertEqual(files, {'sessions.py', 'agent.json', 'config.json.dist',
            'support/agents/shared/metadata.py', 'support/agents/shared/envfile.py',
            'support/internal/version/VERSION', 'LICENSE.md', 'licenses/NOTICE.md'})
        shutil.copytree(fresh, upgrade)
        (upgrade / 'obsolete.py').write_text('raise RuntimeError("stale module")\n')
        (upgrade / 'support/agents/shared/obsolete.py').write_text('stale helper\n')
        (upgrade / 'sessions.py').write_text('incomplete old package\n')
        result = self.package(upgrade)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.snapshot(upgrade), self.snapshot(fresh),
                         'upgrade retained stale files or omitted shipped content')
        self.assertEqual(upgrade.stat().st_mode & 0o777, 0o755)
        self.assertFalse(list(self.work.glob('upgrade.new.*')))
        self.assertFalse(list(self.work.glob('upgrade.old.*')))

    def test_failed_build_keeps_previous_tree(self):
        dest = self.work / 'upgrade'
        dest.mkdir()
        (dest / 'sessions.py').write_text('old working code\n')
        (dest / 'stale.py').write_text('old helper\n')
        expected = self.snapshot(dest)
        tools = self.work / 'bin'
        tools.mkdir()
        real = shutil.which('install')
        self.assertIsNotNone(real)
        stub = tools / 'install'
        stub.write_text('#!/bin/sh\n'
            'n=0; [ ! -f "$COUNT" ] || n=$(cat "$COUNT"); n=$((n+1)); echo "$n" >"$COUNT"\n'
            '[ "$n" != 2 ] || exit 77\nexec "' + real + '" "$@"\n')
        stub.chmod(0o755)
        result = self.package(dest, env=dict(os.environ, PATH=str(tools) + ':' + os.environ['PATH'],
                                           COUNT=str(self.work / 'count')))
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.snapshot(dest), expected, 'failed build changed installed code')
        self.assertFalse(list(self.work.glob('upgrade.new.*')))
        self.assertFalse(list(self.work.glob('upgrade.old.*')))


if __name__ == '__main__':
    unittest.main()
