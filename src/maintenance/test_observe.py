import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('observe', Path(__file__).with_name('node-observe.py'))
o = importlib.util.module_from_spec(spec)
spec.loader.exec_module(o)
SCRATCH = Path(__file__).resolve().parents[2] / 'tmp'
SCRATCH.mkdir(exist_ok=True)


class ObservationTests(unittest.TestCase):
    def test_growth_requires_complete_same_path_measurements(self):
        now = {'cache': {'path': '/cache', 'status': 'ok', 'bytes': 50},
               'logs': {'path': '/logs', 'status': 'partial', 'bytes': 90},
               'moved': {'path': '/new', 'status': 'ok', 'bytes': 100}}
        old = {'sizes': {'cache': {'path': '/cache', 'status': 'ok', 'bytes': 70},
                         'logs': {'path': '/logs', 'status': 'ok', 'bytes': 80},
                         'moved': {'path': '/old', 'status': 'ok', 'bytes': 80}}}
        o.growth(now, old)
        self.assertEqual(now['cache']['growth_bytes'], -20)
        self.assertIsNone(now['logs']['growth_bytes'])
        self.assertIsNone(now['moved']['growth_bytes'])

    def test_du_permission_failure_is_a_lower_bound(self):
        result = subprocess.CompletedProcess([], 1, '4096\t/private\n4096\ttotal\n', 'denied')
        with patch.object(o.subprocess, 'run', return_value=result):
            self.assertEqual(o.measure([Path('/private')]), {'status': 'partial', 'bytes': 4096})
        with patch.object(o.subprocess, 'run', side_effect=subprocess.TimeoutExpired('du', 60)):
            self.assertEqual(o.measure([Path('/private')]), {'status': 'timeout', 'bytes': None})

    def test_real_du_and_missing_path(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as d:
            p = Path(d)
            (p / 'data').write_bytes(b'x' * 8192)
            row = o.directory(p)
            self.assertEqual(row['status'], 'ok')
            self.assertGreaterEqual(row['bytes'], 8192)
            self.assertEqual(o.directory(p / 'missing'), {'status': 'missing', 'bytes': None})

    def test_old_releases_excludes_current_and_symlink_aliases(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as d:
            root = Path(d)
            (root / 'releases').mkdir()
            for name in ['old', 'active']:
                (root / 'releases' / name).mkdir()
            (root / 'current').symlink_to('releases/active')
            (root / 'releases/alias').symlink_to('old')
            with patch.object(o, 'measure', return_value={'status': 'ok', 'bytes': 1}) as measure:
                o.old_releases(root)
            self.assertEqual(measure.call_args.args[0], [root / 'releases/old'])

    def test_previous_report_keeps_corrupt_evidence(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as d:
            p = Path(d)
            valid = {'observed_at': 'baseline', 'sizes': {}}
            (p / 'report-1.json').write_text(json.dumps(valid))
            (p / 'report-2.json').write_text('broken')
            self.assertEqual(o.previous_report(p), valid)
            self.assertEqual((p / 'report-2.json').read_text(), 'broken')

    def test_observation_appends_private_reports_without_overwrite(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH) as d:
            output = Path(d) / 'reports'
            def command(args, **kwargs):
                return subprocess.CompletedProcess(args, 0, '/cache\n' if args[0] == 'go' else 'fixture df\n', '')
            with patch.object(o.os, 'geteuid', return_value=1000), \
                 patch.object(o.subprocess, 'run', side_effect=command), \
                 patch.object(o, 'directory', return_value={'status': 'ok', 'bytes': 10}), \
                 patch.object(o, 'old_releases', return_value={'status': 'ok', 'bytes': 0}), \
                 patch('sys.stdout', new_callable=io.StringIO):
                first = o.observe(Path('/repo'), output)
                paths = list(output.glob('report-*.json'))
                saved = paths[0].read_bytes()
                second = o.observe(Path('/repo'), output)
            self.assertEqual(len(list(output.glob('report-*.json'))), 2)
            self.assertEqual(paths[0].read_bytes(), saved)
            self.assertEqual(paths[0].stat().st_mode & 0o777, 0o600)
            self.assertEqual(second['previous_at'], first['observed_at'])
            self.assertEqual(second['sizes']['go_cache']['growth_bytes'], 0)
            self.assertIsNone(first['sizes']['go_cache']['growth_bytes'])

    def test_root_refused_before_any_output_write(self):
        with patch.object(o.os, 'geteuid', return_value=0), patch.object(Path, 'mkdir') as mkdir:
            with self.assertRaisesRegex(SystemExit, 'ordinary user'):
                o.observe(Path('/repo'), Path('/output'))
            mkdir.assert_not_called()


if __name__ == '__main__':
    unittest.main()
