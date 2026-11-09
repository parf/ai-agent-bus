"""windows.py: a dry run writes nothing; apply copies one window's notes, moves and overwrites nothing, touches no other window."""
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / os.environ.get('WINDOWS_SCRIPT', 'windows.py')


def entry(name, bus, cwd, command):
    return {'name': name, 'cwd': str(cwd), 'env': {'AGENT_BUS_NAME': '#' + bus}, 'command': command}


class Windows(unittest.TestCase):
    def setUp(self):
        scratch = Path(__file__).resolve().parents[2] / 'tmp'   # never /tmp
        scratch.mkdir(exist_ok=True)
        self.tmp = tempfile.TemporaryDirectory(dir=scratch, prefix='windows-test-')
        self.addCleanup(self.tmp.cleanup)
        base = Path(self.tmp.name)
        self.local, self.home, root = base / 'repo/local', base / 'home', base / 'repo'
        self.local.mkdir(parents=True)
        (self.local / 'dir').mkdir()
        self.root = root
        config = {'session': 'T', 'windows': [
            entry('DIR', 'ab-dir', self.local / 'dir', 'ab-claude --x'),
            entry('W1A', 'ab-w1a', root, 'ab-claude --restart'),
            entry('W1B', 'ab-w1b', root, 'env -u X ab-claude -2 --restart'),
            entry('W1C', 'ab-w1c', root, 'ab-codex --restart'),
            entry('GATE', 'ab-gate', root, 'agent-bus run x'),
        ]}
        (self.local / 'ab-dvp-v2-config.json').write_text(json.dumps(config, indent=1) + '\n')
        self.config = config
        for account, text in (('.claude', 'one'), ('.claude2', 'two')):
            memory = self.home / account / 'projects' / self.slug(root) / 'memory'
            memory.mkdir(parents=True)
            (memory / 'MEMORY.md').write_text('- [A](a.md) - ' + text + '\n')
            (memory / 'a.md').write_text('note a ' + text + '\n')
        dirmemory = self.home / '.claude/projects' / self.slug(self.local / 'dir') / 'memory'
        dirmemory.mkdir(parents=True)
        (dirmemory / 'dir.md').write_text('dir note\n')

    @staticmethod
    def slug(path):
        import re
        return re.sub(r'[^A-Za-z0-9]', '-', str(path))

    def run_script(self, *args, ok=True):
        env = dict(os.environ, WINDOWS_LOCAL=str(self.local), WINDOWS_HOME=str(self.home))
        p = subprocess.run([sys.executable, str(SCRIPT), *args], capture_output=True, text=True, env=env, timeout=20)
        if ok:
            self.assertEqual(p.returncode, 0, p.stderr)
        return p

    def tree(self):
        """Every file under the temp base with its bytes, plus every directory: a change of any kind shows."""
        base = Path(self.tmp.name)
        return {str(p.relative_to(base)): (p.read_bytes() if p.is_file() else None) for p in sorted(base.rglob('*'))}

    def memory(self, account, path):
        return self.home / account / 'projects' / self.slug(path) / 'memory'

    def test_a_dry_run_writes_nothing_and_says_what_goes_where(self):
        before = self.tree()
        out = self.run_script('plan').stdout
        self.assertEqual(self.tree(), before)
        self.assertIn(f'{self.local}/ab-w1a', out)
        self.assertIn('copy 2, already there 0, kept as they are 0', out)
        self.assertIn('no Claude memory (not ab-claude)', out)      # the codex window: directory only
        self.assertEqual(self.run_script().stdout, out)              # no argument is the dry run

    def test_apply_copies_one_windows_notes_and_touches_no_other_window(self):
        before = self.tree()
        self.run_script('apply', '--window', 'ab-w1a')
        new = self.memory('.claude', self.local / 'ab-w1a')
        self.assertEqual((new / 'a.md').read_text(), 'note a one\n')
        self.assertEqual((new / 'MEMORY.md').read_text(), '- [A](a.md) - one\n')
        for other in ('ab-w1b', 'ab-w1c', 'ab-gate', 'ab-dir'):
            self.assertFalse((self.local / other).exists(), other)
            self.assertFalse(self.memory('.claude', self.local / other).exists(), other)
            self.assertFalse(self.memory('.claude2', self.local / other).exists(), other)
        after = self.tree()
        for path, content in before.items():
            if path.endswith('ab-dvp-v2-config.json'):
                continue   # the one file apply rewrites, for the chosen window only (checked below)
            self.assertEqual(after.get(path), content, 'an original changed or went: ' + path)   # copy, never move
        config = json.loads((self.local / 'ab-dvp-v2-config.json').read_text())
        cwds = {w['name']: w['cwd'] for w in config['windows']}
        self.assertEqual(cwds['W1A'], str(self.local / 'ab-w1a'))
        self.assertEqual({k: v for k, v in cwds.items() if k != 'W1A'}, {w['name']: w['cwd'] for w in self.config['windows'] if w['name'] != 'W1A'})
        backups = list((self.local / 't13-backup').glob('ab-dvp-v2-config.json.*'))
        self.assertEqual(len(backups), 1)
        self.assertEqual(json.loads(backups[0].read_text()), self.config)

    def test_the_second_account_copies_from_its_own_memory(self):
        self.run_script('apply', '--window', 'ab-w1b')
        self.assertEqual((self.memory('.claude2', self.local / 'ab-w1b') / 'a.md').read_text(), 'note a two\n')
        self.assertFalse(self.memory('.claude', self.local / 'ab-w1b').exists())

    def test_a_note_already_in_the_target_is_never_overwritten(self):
        target = self.memory('.claude', self.local / 'ab-w1a')
        target.mkdir(parents=True)
        (target / 'a.md').write_text('newer, written there since\n')
        plan = self.run_script('plan').stdout
        self.assertIn('differs', plan)
        self.run_script('apply', '--window', 'ab-w1a')
        self.assertEqual((target / 'a.md').read_text(), 'newer, written there since\n')
        self.assertEqual((target / 'MEMORY.md').read_text(), '- [A](a.md) - one\n')   # the missing one was still copied

    def load(self):
        """The script as a module, pointed at this fixture's directories."""
        import importlib.util
        os.environ['WINDOWS_LOCAL'], os.environ['WINDOWS_HOME'] = str(self.local), str(self.home)
        spec = importlib.util.spec_from_file_location('windows_under_test', SCRIPT)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_a_note_written_into_the_target_while_the_copy_starts_is_not_overwritten(self):
        # The window writes its own note into its new memory between the plan and the copy: whatever way the copy
        # first touches the destination, the newer note must still be there afterwards.
        from unittest.mock import patch
        w = self.load()
        target = self.memory('.claude', self.local / 'ab-w1a')
        raced = []

        def newer_writer(real):
            def wrapped(*args, **kw):
                dest = Path(args[1] if real.__name__ == 'copy2' else args[0])
                if dest.name == 'a.md' and not raced:
                    raced.append(dest)
                    dest.write_text('newer note written by the window itself\n')
                return real(*args, **kw)
            return wrapped
        with patch.object(w.os, 'open', side_effect=newer_writer(os.open)), \
                patch.object(w.shutil, 'copy2', side_effect=newer_writer(__import__('shutil').copy2)):
            config = json.loads((self.local / 'ab-dvp-v2-config.json').read_text())
            w.apply(config, ['ab-w1a'])
        self.assertTrue(raced, 'the copy never touched a.md: the race was not exercised')
        self.assertEqual((target / 'a.md').read_text(), 'newer note written by the window itself\n')
        self.assertEqual((target / 'MEMORY.md').read_text(), '- [A](a.md) - one\n')

    def test_a_dangling_link_in_the_target_is_not_followed(self):
        target = self.memory('.claude', self.local / 'ab-w1a')
        target.mkdir(parents=True)
        outside = Path(self.tmp.name) / 'elsewhere.md'
        (target / 'a.md').symlink_to(outside)
        self.run_script('apply', '--window', 'ab-w1a')
        self.assertFalse(outside.exists(), 'the copy wrote through a dangling link')
        self.assertTrue((target / 'a.md').is_symlink())

    def test_a_copied_note_keeps_its_mode_and_time(self):
        source = self.memory('.claude', self.root) / 'a.md'
        os.chmod(source, 0o640)
        os.utime(source, (1_000_000_000, 1_000_000_000))
        self.run_script('apply', '--window', 'ab-w1a')
        made = self.memory('.claude', self.local / 'ab-w1a') / 'a.md'
        self.assertEqual((made.stat().st_mode & 0o777, int(made.stat().st_mtime)), (0o640, 1_000_000_000))

    def test_dir_keeps_its_directory_and_its_memory(self):
        before = self.tree()
        p = self.run_script('apply', '--window', 'ab-dir')
        self.assertIn('unchanged', p.stdout)
        after = self.tree()
        self.assertEqual({k: v for k, v in after.items() if 't13-backup' not in k}, before)

    def test_apply_needs_a_window_and_an_unknown_one_is_refused(self):
        before = self.tree()
        self.assertNotEqual(self.run_script('apply', ok=False).returncode, 0)
        p = self.run_script('apply', '--window', 'ab-nobody', ok=False)
        self.assertNotEqual(p.returncode, 0)
        self.assertIn('no such window', p.stderr)
        self.assertEqual({k: v for k, v in self.tree().items() if 't13-backup' not in k}, before)


if __name__ == '__main__':
    unittest.main()
