import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('cleanup', Path(__file__).with_name('node-cleanup.py'))
c = importlib.util.module_from_spec(spec)
spec.loader.exec_module(c)


class CleanupTests(unittest.TestCase):
    def test_release_retention_protects_current(self):
        with tempfile.TemporaryDirectory() as d:
            paths = [Path(d) / str(i) for i in range(4)]
            for i, p in enumerate(paths):
                p.mkdir()
                os.utime(p, (i, i))
            c.retain(paths, 2, [paths[0]])
            self.assertEqual([p.exists() for p in paths], [True, False, False, True])

    def test_record_archives_have_separate_bounds_and_sidecars(self):
        with tempfile.TemporaryDirectory() as d:
            directory = Path(d)
            for kind, total in [('full', 7), ('partial', 12)]:
                for i in range(total):
                    p = directory / f'{kind}-host-{i:02}.tar.gz.age'
                    p.touch()
                    os.utime(p, (i, i))
                    p.with_name(p.name + '.info').touch()
            foreign = directory / 'stopped-daemon.tar.gz'
            foreign.touch()
            c.retain_record_backups(directory)
            for kind, count in [('full', 5), ('partial', 10)]:
                archives = list(directory.glob(kind + '-*.tar.gz.age'))
                self.assertEqual(len(archives), count)
                self.assertEqual(len(list(directory.glob(kind + '-*.info'))), count)
            self.assertTrue(foreign.exists())

    def test_tmp_attribution_rejects_foreign_and_symlinks(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / 'tmp.fixture'
            p.mkdir()
            self.assertFalse(c.ours_tmp(p, os.getuid(), set()))
            (p / 'agent-busd').touch()
            self.assertTrue(c.ours_tmp(p, os.getuid(), set()))
            self.assertFalse(c.ours_tmp(p, os.getuid() + 1, set()))
            link = Path(d) / 'tmp.link'
            link.symlink_to(p, target_is_directory=True)
            self.assertFalse(c.ours_tmp(link, os.getuid(), set()))
            c.remove(link)
            self.assertTrue((p / 'agent-busd').exists())

    def test_crash_attribution_ignores_foreign_applications(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d)
            (p / 'cmdline').write_text('/usr/bin/Telegram')
            self.assertFalse(c.ours_crash(p))
            (p / 'cmdline').write_text('/usr/bin/bun /usr/local/src/ai-agent-bus/tmp/test.ts')
            self.assertTrue(c.ours_crash(p))

    def test_cache_thresholds_do_not_clean_small_caches(self):
        with patch.object(c, 'size', return_value=0), patch.object(c.subprocess, 'run') as run:
            c.user_cleanup()
            run.assert_not_called()
        with patch.object(c, 'size', return_value=11*c.GiB), patch.object(c.subprocess, 'run') as run:
            c.user_cleanup()
            self.assertEqual(run.call_count, 2)
            self.assertEqual(run.call_args_list[0].args[0], ['go', 'clean', '-cache'])
            self.assertIn('tmp/cargo-target', run.call_args_list[1].args[0][-1])


if __name__ == '__main__':
    unittest.main()
