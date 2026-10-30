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
        tools = self.work / 'bin';tools.mkdir();real = sys.executable
        self.assertIsNotNone(real)
        (tools / 'python3').write_text('#!/bin/sh\n'
            'n=0; [ ! -f "$COUNT" ] || n=$(cat "$COUNT"); n=$((n+1)); echo "$n" >"$COUNT"\n'
            '[ "$n" != 2 ] || exit 77\nexec "' + real + '" "$@"\n')
        (tools / 'python3').chmod(0o755)
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

    def test_concurrent_initial_publications_never_nest_or_report_false_success(self):
        tools = self.work / 'bin';tools.mkdir()
        marks = self.work / 'marks';marks.mkdir()
        # Interpose both the old mv and the portable rename implementation.
        # Both children reach publication while target is absent; A publishes
        # first and B then attempts its already-decided initial publication.
        for tool, real, source_arg in [('mv', shutil.which('mv'), '$1'),
                                        ('python3', sys.executable, '$3')]:
            self.assertTrue(real)
            shim = ('#!/bin/sh\nsource=' + source_arg + '\n'
                    'case "$source" in *.new.*)\n'
                    ' if [ "$LABEL" = A ]; then\n'
                    '  i=0; while [ ! -e "$MARKS/B-ready" ]; do i=$((i+1)); [ "$i" -lt 300 ] || exit 91; sleep .01; done\n'
                    '  "' + real + '" "$@"; rc=$?; touch "$MARKS/A-done"; exit "$rc"\n'
                    ' else\n'
                    '  touch "$MARKS/B-ready"\n'
                    '  i=0; while [ ! -e "$MARKS/A-done" ]; do i=$((i+1)); [ "$i" -lt 300 ] || exit 92; sleep .01; done\n'
                    ' fi ;;\nesac\nexec "' + real + '" "$@"\n')
            (tools / tool).write_text(shim);(tools / tool).chmod(0o755)
        script = ('set -eu; . "$HELPER"; package_begin "$TARGET"; '
                  'echo "$LABEL" > "$dest/code"; package_commit')
        processes = []
        for label in ('A', 'B'):
            env = dict(os.environ, PATH=str(tools) + ':' + os.environ['PATH'],
                       HELPER=str(HERE / 'package-tree.sh'), TARGET=str(self.work / 'agent'),
                       LABEL=label, MARKS=str(marks))
            proc = subprocess.Popen(['/bin/sh', '-c', script], env=env,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            processes.append(proc)
            self.addCleanup(lambda p=proc: p.kill() if p.poll() is None else None)
        results = [(p.communicate(timeout=8), p.returncode) for p in processes]
        self.assertEqual(results[0][1], 0, results)
        if results[1][1] != 0:
            error = results[1][0][1]
            self.assertTrue('FileExistsError' in error or 'Directory not empty' in error, results)
        target = self.work / 'agent'
        self.assertEqual(sorted(str(p.relative_to(target)) for p in target.rglob('*')), ['code'],
                         'concurrent initial publications nested a stage inside the package')
        # It is safe either to replace all of A with B or to refuse B. Success
        # must always name the tree actually published, never a nested stage.
        self.assertEqual((target / 'code').read_text(), 'B\n' if results[1][1] == 0 else 'A\n',
                         'a concurrent packager reported success without publishing its code')
        self.assertFalse(list(self.work.glob('agent.new.*')))
        self.assertFalse(list(self.work.glob('agent.old.*')))

    def test_release_exports_the_shared_packaging_helper(self):
        dest = self.work / 'release'
        result = subprocess.run([str(HERE / 'package.sh'), str(dest)],
                                capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        helper = dest / 'agents/package-tree.sh'
        self.assertTrue(helper.is_file(), 'release omitted the shared packaging helper')
        self.assertEqual(helper.read_bytes(), (HERE / 'package-tree.sh').read_bytes())
        # The build-time native scripts are not runtime repackagers; the
        # standalone one is, and must be delivered alongside its dependency.
        self.assertEqual(sorted(str(p.relative_to(dest)) for p in dest.rglob('package.sh')),
                         ['agents/standalone/tmux-sessions/package.sh'])


if __name__ == '__main__':
    unittest.main()
