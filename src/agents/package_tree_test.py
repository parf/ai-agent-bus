"""Real package upgrades preserve only shipped code, never old import shadows."""
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class PackageTrees(unittest.TestCase):
    def setUp(self):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=ROOT / 'tmp', prefix='package-tree-')
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)

    def package(self, name, dest, **kwargs):
        return subprocess.run([str(HERE / name / 'package.sh'), str(dest)],
                              capture_output=True, text=True, timeout=30, **kwargs)

    def snapshot(self, directory):
        return {str(p.relative_to(directory)): (
            p.stat().st_mode & 0o777,
            hashlib.sha256(p.read_bytes()).hexdigest() if p.is_file() else None
        ) for p in directory.rglob('*')}

    def test_every_runner_package_replaces_its_tree_exactly(self):
        for script in sorted(HERE.glob('*/package.sh')):
            with self.subTest(agent=script.parent.name):
                name = script.parent.name
                fresh, upgrade = self.work / (name + '-fresh'), self.work / (name + '-upgrade')
                first = self.package(name, fresh)
                self.assertEqual(first.returncode, 0, first.stderr)
                expected = self.snapshot(fresh)
                shutil.copytree(fresh, upgrade)
                (upgrade / 'obsolete.py').write_text('raise RuntimeError("obsolete module")\n')
                (upgrade / 'obsolete-dir').mkdir()
                (upgrade / 'obsolete-dir' / 'old.py').write_text('old\n')
                second = self.package(name, upgrade)
                self.assertEqual(second.returncode, 0, second.stderr)
                self.assertEqual(self.snapshot(upgrade), expected,
                                 name + ': deploying over an old tree retained an unshipped file')
                self.assertEqual(upgrade.stat().st_mode & 0o777, 0o755)
                self.assertFalse(list(self.work.glob(name + '-upgrade.new.*')))
                self.assertFalse(list(self.work.glob(name + '-upgrade.old.*')))

    def test_imap_upgrade_imports_the_shipped_shared_subscription_module(self):
        dest = self.work / 'imap'
        dest.mkdir()
        (dest / 'subscriptions.py').write_text('from publication import new_id, validate_creator, validate_id\n')
        (dest / 'ledger.py').write_text('raise RuntimeError("obsolete ledger")\n')
        result = self.package('imap', dest)
        self.assertEqual(result.returncode, 0, result.stderr)
        check = ('import pathlib,runpy,sys; m=runpy.run_path(sys.argv[1]); '
                 'assert pathlib.Path(m["shared"].__file__).resolve()==pathlib.Path(sys.argv[2]).resolve(); '
                 'print("all IMAP imports resolve inside the shipped package")')
        run = subprocess.run([sys.executable, '-I', '-B', '-c', check, str(dest / 'imap.py'),
                              str(dest / 'support/agents/shared/subscriptions.py')],
                             cwd=self.work, capture_output=True, text=True, timeout=5,
                             env={'PATH': os.environ['PATH'], 'PYTHONDONTWRITEBYTECODE': '1'})
        self.assertEqual(run.returncode, 0, run.stderr)
        self.assertIn('all IMAP imports resolve', run.stdout)
        self.assertFalse((dest / 'subscriptions.py').exists())
        self.assertFalse((dest / 'ledger.py').exists())

    def test_failed_build_keeps_the_previous_package_unchanged(self):
        dest = self.work / 'imap';dest.mkdir()
        (dest / 'imap.py').write_text('previous working code\n')
        (dest / 'obsolete.py').write_text('previous tree\n')
        expected = self.snapshot(dest)
        tools = self.work / 'bin';tools.mkdir()
        real = shutil.which('install')
        self.assertIsNotNone(real)
        (tools / 'install').write_text('#!/bin/sh\n'
            'n=0; [ ! -f "$COUNT" ] || n=$(cat "$COUNT"); n=$((n+1)); echo "$n" >"$COUNT"\n'
            '[ "$n" != 2 ] || exit 77\nexec "' + real + '" "$@"\n')
        (tools / 'install').chmod(0o755)
        env = dict(os.environ, PATH=str(tools) + ':' + os.environ['PATH'], COUNT=str(self.work / 'count'))
        result = self.package('imap', dest, env=env)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.snapshot(dest), expected, 'failed build changed the installed package')
        self.assertFalse(list(self.work.glob('imap.new.*')))
        self.assertFalse(list(self.work.glob('imap.old.*')))

    def test_failed_commit_restores_the_previous_package(self):
        dest = self.work / 'imap';dest.mkdir()
        (dest / 'previous.py').write_text('previous\n');expected = self.snapshot(dest)
        tools = self.work / 'bin';tools.mkdir();real = shutil.which('mv')
        self.assertIsNotNone(real)
        (tools / 'mv').write_text('#!/bin/sh\n'
            'n=0; [ ! -f "$COUNT" ] || n=$(cat "$COUNT"); n=$((n+1)); echo "$n" >"$COUNT"\n'
            '[ "$n" != 2 ] || exit 77\nexec "' + real + '" "$@"\n')
        (tools / 'mv').chmod(0o755)
        result = self.package('imap', dest, env=dict(os.environ, PATH=str(tools) + ':' + os.environ['PATH'],
                                                   COUNT=str(self.work / 'count')))
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.snapshot(dest), expected)
        self.assertFalse(list(self.work.glob('imap.new.*')))
        self.assertFalse(list(self.work.glob('imap.old.*')))

    def test_a_destination_symlink_is_refused_without_touching_its_target(self):
        target = self.work / 'state';target.mkdir();(target / 'keep').write_text('keep')
        link = self.work / 'imap';link.symlink_to(target, target_is_directory=True)
        result = self.package('imap', link)
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(link.is_symlink())
        self.assertEqual((target / 'keep').read_text(), 'keep')
        self.assertEqual(sorted(p.name for p in target.iterdir()), ['keep'])


if __name__ == '__main__':
    unittest.main()
