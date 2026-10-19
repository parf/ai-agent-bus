import json
import os
import subprocess
import sys
import unittest
from pathlib import Path
from unittest.mock import patch
from test_systemd import agent


class Dedicated(unittest.TestCase):
    def invoke(self, body, units='a.service,b.timer', role='ADMIN'):
        with patch.dict(os.environ, {'AGENT_BUS_CONFIG': json.dumps({'units': units}), 'AB_ROLE_'+role:'1'}, clear=True), patch.object(agent, 'execute', return_value={'stdout':'', 'stderr':'', 'status':0}) as run:
            answer = agent.answer(body)
        return answer, run

    def test_unlisted_units_never_execute(self):
        for method in ['status','start','restart','reload','stop','enable','disable','logs']:
            answer, run = self.invoke(method + ' sshd.service')
            self.assertIn('sshd.service', answer)
            self.assertTrue(answer.startswith('error:'), method)
            run.assert_not_called()

    def test_inventory_and_journal_scope_are_instance_specific(self):
        for units in ['a.service,b.timer', 'other.service,c.timer']:
            names = units.split(',')
            for method in ['list','list timer','timers','failed']:
                answer, run = self.invoke(method, units)
                self.assertEqual(run.call_args.args[0][-3:], ['--', *names], method)
            answer, run = self.invoke('logs', units)
            self.assertEqual(run.call_args.args[0][-4:], ['-u',names[0],'-u',names[1]])
            answer, run = self.invoke('logs '+names[0], units)
            self.assertEqual(run.call_args.args[0][-2:], ['-u',names[0]])
            answer, run = self.invoke('start '+names[0], units, 'OPERATE')
            self.assertFalse(answer.startswith('error:'), answer)

    def test_malformed_scope_answers_without_execution(self):
        program = str(Path(__file__).with_name('systemd.py'))
        for units in ['', None, [], 'a.service,', '*.service', 'a.service, bad*service']:
            result = subprocess.run([sys.executable, program, 'help'], env={**os.environ,'AGENT_BUS_CONFIG':json.dumps({'units':units})}, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, repr(units))
            self.assertTrue(result.stdout.startswith('error: Configuration units:'), result.stdout)
            self.assertIn(repr(units) if not isinstance(units,str) else repr(units.split(',')[-1].strip()), result.stdout)

    def test_help_names_scope_and_keeps_role_filter(self):
        answer, run = self.invoke('help')
        self.assertIn('a.service, b.timer', answer)
        self.assertNotIn('whole journal', answer)
        self.assertNotIn('every unit', answer)
        run.assert_not_called()
        answer, run = self.invoke('start a.service', role='OWNER')
        self.assertIn('requires operate', answer)
        run.assert_not_called()

    def test_scope_trims_whitespace(self):
        answer, run = self.invoke('status b.timer', ' a.service, b.timer ')
        self.assertEqual(answer, 'ok')
        self.assertEqual(run.call_args.args[0][-1], 'b.timer')
