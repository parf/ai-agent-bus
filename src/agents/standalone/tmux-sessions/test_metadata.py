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

    def test_advertised_body_help_without_configuration_or_commands(self):
        import io, json, os
        from contextlib import redirect_stdout
        from unittest.mock import patch
        for body in ('help', '{"method":"help"}'):
            with patch.dict(os.environ, {'AGENT_BUS_CONFIG':'invalid'}, clear=True), patch.object(sys, 'argv', ['sessions.py',body]), patch.object(sessions,'command',side_effect=AssertionError('host command during help')):
                out=io.StringIO()
                with redirect_stdout(out): sessions.main()
                self.assertEqual(out.getvalue().strip(), sessions.dispatch(HERE,'help'))
