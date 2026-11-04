import importlib.util
from pathlib import Path
import sys
import unittest
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
sys.path.insert(0, str(HERE.parents[1] / 'shared'))
from metadata_contract import check
spec = importlib.util.spec_from_file_location('server_info', HERE / 'server-info.py')
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)

class Metadata(unittest.TestCase):
    def test_help_matches_executable_methods(self):
        check(self, HERE, agent, 'server-info')

    def test_all_help_forms_share_effective_account_scope(self):
        import io, json, os
        from contextlib import redirect_stdout
        from unittest.mock import patch
        for uid in (0,1000):
            with patch.object(agent.os,'geteuid',return_value=uid), patch.object(agent,'collect',side_effect=AssertionError('host command during help')):
                expected=set(agent.available())|{'all','help'}
                for ask in ('help','help-json','help-config','help-secret'):
                    with patch.dict(os.environ,{'AGENT_BUS_ASK':ask},clear=True):
                        out=io.StringIO()
                        with redirect_stdout(out): agent.main()
                    if ask=='help-json': self.assertEqual(set(json.loads(out.getvalue())['methods']),expected)
                    elif ask=='help':
                        self.assertEqual('health:' in out.getvalue(),uid==0)
                self.assertEqual(set(json.loads(agent.answer('{"method":"help"}'))['methods']),expected)
        self.assertEqual(json.loads((HERE/'agent.json').read_text()).get('help'), 'native')
