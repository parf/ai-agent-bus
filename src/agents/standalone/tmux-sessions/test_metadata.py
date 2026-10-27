import sys
from pathlib import Path
import unittest
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
sys.path.insert(0, str(HERE.parents[1] / 'shared'))
import sessions
from metadata_contract import check

class Metadata(unittest.TestCase):
    def test_help_matches_executable_methods(self):
        check(self, HERE, sessions, 'tmux-sessions')
