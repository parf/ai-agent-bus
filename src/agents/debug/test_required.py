import importlib.util
from pathlib import Path
import unittest

spec=importlib.util.spec_from_file_location('metadata',Path(__file__).parents[1]/'shared/metadata.py')
metadata=importlib.util.module_from_spec(spec)
spec.loader.exec_module(metadata)

class RequiredHelp(unittest.TestCase):
    def test_required_optional_and_defaults_never_show_values(self):
        rows=metadata.secrets("A= # required\nB=private-default # required explanation\nC=\nD=public-default\nE='literal # required'\nF= # requiredness\n")
        self.assertEqual([(r['required'],r['has_default']) for r in rows],[(True,False),(True,True),(False,False),(False,True),(False,True),(False,False)])
        self.assertNotIn('private-default',str(rows))
