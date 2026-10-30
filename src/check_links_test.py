"""check-links.py fails a change that leaves a link to nothing, including in a file the change did not touch (J53)."""
import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('check_links', Path(__file__).with_name('check-links.py'))
cl = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cl)
SCRATCH = Path(__file__).resolve().parent.parent / 'tmp'   # never /tmp


class Links(unittest.TestCase):
    def setUp(self):
        SCRATCH.mkdir(exist_ok=True)
        self.tmp = tempfile.TemporaryDirectory(dir=SCRATCH, prefix='check-links-')
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name).resolve()
        self.git('init', '-q', '-b', 'main')
        self.git('config', 'user.email', 't@example.test')
        self.git('config', 'user.name', 'T')
        self.write('Plans/a.md', '# A\n\nSee [b](b.md#part) and [c](sub/c.md) and [d](d.md).\n')
        self.write('Plans/b.md', '# B\n\n## Part\n\ntext\n')
        self.write('Plans/sub/c.md', '# C\n')
        self.write('Plans/d.md', '# D\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'base')
        self.git('checkout', '-qb', 'work')

    def git(self, *args):
        return subprocess.run(['git', '-C', str(self.root), *args], check=True, capture_output=True, text=True).stdout

    def write(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def problems(self, **kw):
        return [(f, link, why) for f, _, link, why in cl.check(self.root, **kw)]

    def test_a_clean_tree_passes(self):
        self.assertEqual(self.problems(), [])

    def test_a_moved_file_fails_on_the_link_in_a_file_nobody_edited(self):
        self.git('mv', 'Plans/b.md', 'Plans/b2.md')
        self.git('commit', '-qam', 'move b')
        self.assertEqual(self.problems(), [('Plans/a.md', 'b.md#part', 'moved or deleted by this change')])
        # What the old check did: it reads only the changed files, here the moved file itself, which has no link.
        self.assertEqual(self.problems(inbound=False), [])

    def test_a_deleted_file_fails_the_same_way_and_retargeting_fixes_it(self):
        self.git('rm', '-q', 'Plans/d.md')
        self.git('commit', '-qm', 'drop d')
        self.assertEqual(self.problems(), [('Plans/a.md', 'd.md', 'moved or deleted by this change')])
        self.write('Plans/a.md', '# A\n\nSee [b](b.md#part) and [c](sub/c.md) and [d](https://example.test/old/d.md).\n')
        self.assertEqual(self.problems(), [])

    def test_an_unstaged_move_and_an_untracked_new_file_count_too(self):
        (self.root / 'Plans/sub/c.md').rename(self.root / 'Plans/sub/c2.md')
        self.write('Plans/new.md', '# New\n\n[x](nowhere.md)\n')
        self.assertEqual(sorted(self.problems()), [('Plans/a.md', 'sub/c.md', 'moved or deleted by this change'),
                                                   ('Plans/new.md', 'nowhere.md', 'no such file')])

    def test_a_changed_file_is_checked_for_missing_files_and_anchors(self):
        self.write('Plans/a.md', '# A\n\n[b](b.md#nope) [e](e.md) [ok](b.md#part) [self](#a)\n')
        self.assertEqual(self.problems(), [('Plans/a.md', 'b.md#nope', 'no such anchor'), ('Plans/a.md', 'e.md', 'no such file')])

    def test_a_section_moved_out_of_a_retained_file_fails_untouched_inbound_links(self):
        self.write('Plans/b.md', '# B\n\nThe section moved.\n')
        self.write('Plans/done/b.md', '# B\n\n## Part\n\narchived\n')
        self.git('add', 'Plans/b.md', 'Plans/done/b.md')
        self.git('commit', '-qm', 'archive the section, retain the day file')
        self.assertEqual(self.problems(), [('Plans/a.md', 'b.md#part', 'no such anchor')])
        self.assertEqual(self.problems(inbound=False), [])
        self.write('Plans/a.md', '# A\n\nSee [history](done/b.md#part).\n')
        self.assertEqual(self.problems(), [])

    def test_a_lost_anchor_sweep_does_not_claim_an_old_missing_anchor(self):
        self.write('Plans/old.md', '[old](b.md#never-existed)\n')
        self.git('add', 'Plans/old.md')
        self.git('commit', '-qm', 'old broken anchor')
        self.git('checkout', '-q', 'main')
        self.git('merge', '-q', '--ff-only', 'work')
        self.git('checkout', '-qb', 'next')
        self.write('Plans/b.md', '# B\n')
        self.assertEqual(self.problems(), [('Plans/a.md', 'b.md#part', 'no such anchor')])

    def test_an_untouched_file_with_an_old_broken_link_is_not_this_changes_problem(self):
        self.write('Plans/old.md', '[gone](gone.md)\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'old break')
        self.git('checkout', '-q', 'main')
        self.git('merge', '-q', '--ff-only', 'work')
        self.git('checkout', '-qb', 'next')
        self.write('Plans/b.md', '# B\n\n## Part\n\nmore\n')
        self.assertEqual(self.problems(), [])

    def test_fenced_code_code_spans_schemes_and_repeated_headings_are_read_right(self):
        self.write('Plans/a.md', '# A\n\n```\n[x](missing.md)\n```\n`[y](missing.md)` [z](https://example.test/x) [m](mailto:a@b.c)\n'
                                 '[one](#h) [two](#h-1)\n\n## H\n\n## H\n')
        self.assertEqual(self.problems(), [])


if __name__ == '__main__':
    unittest.main()
