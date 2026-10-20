"""Keep the live SMS recipient out of tracked examples and test inputs."""
from pathlib import Path
import os
import subprocess
import unittest


class RepositoryPrivacy(unittest.TestCase):
    def test_live_recipient_is_absent_from_tracked_files(self):
        root = Path(__file__).resolve().parent.parent
        forbidden = ('617' + '372' + '5150').encode()
        names = subprocess.check_output(['git', 'ls-files', '-z'], cwd=root).split(b'\0')
        found = []
        for name in filter(None, names):
            path = root / os.fsdecode(name)
            if not path.exists() and not path.is_symlink():
                continue
            value = os.readlink(path).encode() if path.is_symlink() else path.read_bytes()
            if forbidden in value:
                found.append(os.fsdecode(name))
        self.assertEqual(found, [], 'live recipient found in tracked files: ' + ', '.join(found))


if __name__ == '__main__':
    unittest.main()
