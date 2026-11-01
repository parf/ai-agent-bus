"""The db package vendors its drivers from pinned wheels, not from the host's site-packages (J67)."""
import hashlib
import os
import subprocess
import tempfile
import unittest
import zipfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
NAMES = {'pymysql': 'pymysql', 'psycopg': 'psycopg', 'psycopg-binary': 'psycopg_binary'}


def wheel(directory, dist, marker):
    module = NAMES[dist]
    path = directory / f'{module}-1.0-py3-none-any.whl'
    info = f'{module}-1.0.dist-info'
    with zipfile.ZipFile(path, 'w') as z:
        z.writestr(f'{module}/__init__.py', f'MARK = {marker!r}\n')
        z.writestr(f'{info}/METADATA', f'Metadata-Version: 2.1\nName: {dist}\nVersion: 1.0\n')
        z.writestr(f'{info}/WHEEL', 'Wheel-Version: 1.0\nGenerator: test\nRoot-Is-Purelib: true\nTag: py3-none-any\n')
        z.writestr(f'{info}/licenses/LICENSE', f'licence of {dist}\n')
        z.writestr(f'{info}/RECORD', '')
    return path


class Package(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.wheels = self.root / 'wheels'
        self.wheels.mkdir()
        self.pins = []
        for dist in NAMES:
            w = wheel(self.wheels, dist, 'wheel')
            self.pins.append((dist, hashlib.sha256(w.read_bytes()).hexdigest()))
        # A host whose pymysql metadata has no RECORD (dist.files is None) and whose psycopg has none at all.
        self.host = self.root / 'host'
        (self.host / 'pymysql-9.9.dist-info').mkdir(parents=True)
        (self.host / 'pymysql-9.9.dist-info/METADATA').write_text('Metadata-Version: 2.1\nName: pymysql\nVersion: 9.9\n')
        (self.host / 'pymysql').mkdir()
        (self.host / 'pymysql/__init__.py').write_text("MARK = 'host'\n")

    def requirements(self, wrong=None):
        path = self.root / 'requirements.txt'
        path.write_text(''.join(f'{d}==1.0 --hash=sha256:{wrong or h}\n' for d, h in self.pins))
        return path

    def build(self, requirements):
        dest = self.root / 'out' / 'db'
        env = {**os.environ, 'HOME': str(self.root), 'PYTHONPATH': str(self.host), 'AB_DB_WHEELS': str(self.wheels),
               'AB_DB_REQUIREMENTS': str(requirements), 'PIP_CONFIG_FILE': os.devnull}
        run = subprocess.run(['sh', str(HERE / 'package.sh'), str(dest)], env=env, capture_output=True, text=True, timeout=120)
        return run, dest

    def test_drivers_come_from_the_pinned_wheels_and_not_from_the_host(self):
        run, dest = self.build(self.requirements())
        self.assertEqual(run.returncode, 0, run.stdout + run.stderr)
        lib = dest / 'support/lib'
        for module in NAMES.values():
            self.assertEqual((lib / module / '__init__.py').read_text(), "MARK = 'wheel'\n", module)
            self.assertTrue((lib / f'{module}-1.0.dist-info/licenses/LICENSE').is_file(), module)
        self.assertFalse(any(lib.rglob('RECORD')) or any(lib.rglob('INSTALLER')) or any(lib.rglob('*.pyc')) or any(lib.rglob('__pycache__')))

    def test_a_wheel_that_does_not_match_its_pin_is_refused_and_nothing_is_installed(self):
        run, dest = self.build(self.requirements(wrong='0' * 64))
        self.assertNotEqual(run.returncode, 0, run.stdout + run.stderr)
        self.assertFalse(dest.exists(), 'a refused build left a package behind')


if __name__ == '__main__':
    unittest.main()
