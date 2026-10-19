import importlib.util
from pathlib import Path
import sys
import unittest
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
sys.path.insert(0, str(HERE.parents[1] / 'shared'))
from metadata_contract import check
spec = importlib.util.spec_from_file_location('systemd_agent', HERE / 'systemd.py')
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)

class Metadata(unittest.TestCase):
    def test_help_matches_executable_methods(self):
        check(self, HERE, agent, 'systemd')
