"""check-markers.py fails a diff that adds a git conflict marker, and passes a Markdown setext heading (J29)."""
import importlib.util
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('check_markers', Path(__file__).with_name('check-markers.py'))
cm = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cm)
ROOT = Path(__file__).resolve().parent.parent

# The markers are built, never written whole: a literal one at the start of a line in this file would fail
# the check on its own diff.
OPEN, BASE, MID, CLOSE = '<' * 7, '|' * 7, '=' * 7, '>' * 7


def diff(*added, path='Plans/x.md', context=()):
    """A unified diff of one file that adds these lines after the given unchanged ones."""
    lines = ['diff --git a/%s b/%s' % (path, path), '--- a/%s' % path, '+++ b/%s' % path,
             '@@ -1,%d +1,%d @@' % (len(context), len(context) + len(added))]
    lines += [' ' + c for c in context] + ['+' + a for a in added]
    return '\n'.join(lines) + '\n'


def found(text):
    return cm.scan(cm.added_from_diff(text))


class Markers(unittest.TestCase):
    def test_each_marker_fails_on_its_own(self):
        for line in (OPEN + ' HEAD', OPEN, BASE + ' parent of df995378 (J23 records the leak)', BASE, CLOSE + ' branch', CLOSE):
            self.assertEqual(len(found(diff('text', line))), 1, line)

    def test_a_whole_block_reports_every_line_including_the_middle(self):
        lines = [OPEN + ' HEAD', 'ours', BASE + ' base', 'old', MID, 'theirs', CLOSE + ' other']
        self.assertEqual([t for _, _, t in found(diff(*lines))], [OPEN + ' HEAD', BASE + ' base', MID, CLOSE + ' other'])

    def test_a_setext_heading_underline_passes(self):
        self.assertEqual(found(diff('A heading', MID, '', 'text', 'Another', MID)), [])
        self.assertEqual(found(diff('A heading', MID, context=('intro',))), [])

    def test_a_lone_middle_after_a_closed_block_passes(self):
        lines = [OPEN + ' a', BASE + ' b', MID, CLOSE + ' c', 'Heading', MID]
        self.assertEqual([t for _, _, t in found(diff(*lines))][-1], CLOSE + ' c')

    def test_text_that_only_contains_or_extends_a_marker_passes(self):
        for line in ('see ' + OPEN + ' in the guide', OPEN + OPEN, '<' * 6 + ' x', '>' * 8, BASE + 'x', MID + ' =', ' ' + OPEN):
            self.assertEqual(found(diff(line)), [], line)

    def test_only_added_lines_count(self):
        removed = '\n'.join(['diff --git a/x.md b/x.md', '--- a/x.md', '+++ b/x.md', '@@ -1,2 +1,1 @@', '-' + BASE + ' parent', ' kept'])
        self.assertEqual(found(removed), [])
        self.assertEqual(found(diff('new', context=(BASE + ' parent',))), [])

    def test_the_line_number_is_the_new_files(self):
        self.assertEqual(found(diff('a', BASE + ' x', context=('one', 'two')))[0][:2], ('Plans/x.md', 4))

    def test_a_quoted_header_is_unquoted_in_a_diff_on_standard_input(self):
        quoted = '\n'.join(['diff --git "a/unicode-\\320\\264.md" "b/unicode-\\320\\264.md"', '--- "a/unicode-\\320\\264.md"',
                           '+++ "b/unicode-\\320\\264.md"', '@@ -0,0 +1,1 @@', '+' + BASE + ' parent']) + '\n'
        self.assertEqual([p for p, _, _ in found(quoted)], ['unicode-\u0434.md'])
        self.assertEqual(cm.unquote('"a\\tb\\"c\\\\d"'), 'a\tb"c\\d')
        self.assertEqual(cm.unquote('plain.md'), 'plain.md')

    def test_a_marker_in_one_file_does_not_open_a_block_in_another(self):
        a = diff(OPEN + ' h', path='a.md')
        b = diff('Heading', MID, path='b.md')
        self.assertEqual([p for p, _, _ in found(a + b)], ['a.md'])


class RealTree(unittest.TestCase):
    """A real repository: a marker in a commit, in the working tree and in an untracked file all fail."""
    def setUp(self):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        self.repo = Path(tempfile.mkdtemp(prefix='markers-', dir=ROOT / 'tmp'))
        self.addCleanup(shutil.rmtree, self.repo, True)
        self.git('init', '-q', '-b', 'main')
        self.git('config', 'user.email', 't@t')
        self.git('config', 'user.name', 't')
        (self.repo / 'a.md').write_text('base\n')
        self.git('add', '.')
        self.git('commit', '-q', '-m', 'base')
        self.git('checkout', '-q', '-b', 'work')

    def git(self, *a):
        subprocess.run(['git', '-C', str(self.repo), *a], check=True, capture_output=True)

    def lines(self):
        return [t for _, _, t in cm.scan(cm.added_from_tree(self.repo))]

    def test_a_clean_branch_passes(self):
        (self.repo / 'a.md').write_text('base\nmore\n')
        self.git('commit', '-qam', 'edit')
        self.assertEqual(self.lines(), [])

    def test_a_committed_marker_fails(self):
        (self.repo / 'a.md').write_text('base\n' + BASE + ' parent of abc\n')
        self.git('commit', '-qam', 'leftover')
        self.assertEqual(self.lines(), [BASE + ' parent of abc'])

    def test_an_unstaged_marker_fails(self):
        (self.repo / 'a.md').write_text('base\n' + OPEN + ' HEAD\n')
        self.assertEqual(self.lines(), [OPEN + ' HEAD'])

    def test_a_marker_in_a_file_with_a_non_ascii_or_awkward_name_fails(self):
        # With git's default core.quotePath such a name is quoted in the diff header (octal bytes), which
        # once made the path unreadable and the marker invisible.
        for name in ('unicode-\u0434.md', 'with space.md', 'tab\there.md', 'quote"d.md'):
            (self.repo / name).write_text('x\n' + BASE + ' parent\n')
            self.git('add', '--', name)
            self.git('commit', '-qm', 'add ' + name)
            self.assertIn(BASE + ' parent', self.lines(), name)

    def test_a_committed_marker_the_working_tree_has_removed_is_not_reported(self):
        (self.repo / 'a.md').write_text('base\n' + BASE + ' parent\n')
        self.git('commit', '-qam', 'leftover')
        (self.repo / 'a.md').write_text('base\n')
        self.assertEqual(self.lines(), [])

    def test_a_marker_added_and_removed_by_later_commits_is_not_reported(self):
        (self.repo / 'a.md').write_text('base\n' + OPEN + ' h\n')
        self.git('commit', '-qam', 'leftover')
        (self.repo / 'a.md').write_text('base\n')
        self.git('commit', '-qam', 'fixed')
        self.assertEqual(self.lines(), [])

    def test_an_untracked_file_with_a_marker_fails_and_a_setext_one_passes(self):
        (self.repo / 'new.md').write_text('Title\n' + MID + '\n')
        self.assertEqual(self.lines(), [])
        (self.repo / 'new.md').write_text(CLOSE + ' x\n')
        self.assertEqual(self.lines(), [CLOSE + ' x'])


if __name__ == '__main__':
    unittest.main()
