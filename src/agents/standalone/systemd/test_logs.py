import importlib.util
import json
import os
from pathlib import Path
import sys
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('logs_agent', Path(__file__).with_name('systemd.py'))
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)


class Logs(unittest.TestCase):
    def answer(self, body, role='operate'):
        with patch.dict(os.environ, {'AB_ROLE_' + role.upper(): '1'} if role else {}, clear=True), patch.object(agent, 'execute', return_value={'stdout': 'entry\n', 'stderr': '', 'status': 0}) as run:
            value = agent.answer(body)
        return value, run

    def test_fixed_argv_and_all_is_the_whole_journal(self):
        value, run = self.answer('{"method":"logs"}')
        self.assertNotIn('error', json.loads(value))
        self.assertEqual(run.call_args.args[0], ['journalctl', '--no-pager', '-o', 'short-iso', '-n', '100'])
        self.assertEqual(run.call_args.kwargs, {'output_limit': 256 * 1024})
        for body in ['logs nginx.service 200 since 1h', '{"method":"logs","unit":"nginx.service","lines":200,"since":"1h"}']:
            value, run = self.answer(body)
            self.assertEqual(run.call_args.args[0], ['journalctl', '--no-pager', '-o', 'short-iso', '-n', '200', '--since', '1 hour ago', '-u', 'nginx.service'])
        _, timer = self.answer('logs backup.timer')
        self.assertEqual(timer.call_args.args[0][-2:], ['-u', 'backup.timer'])
        _, run = self.answer('logs 50')
        self.assertEqual(run.call_args.args[0][5], '50')

    def test_logs_names_and_roles_are_enforced(self):
        for body, role in [('{"method":"logs","unit":"*.service"}', 'admin'),
                           ('{"method":"logs"}', '')]:
            value, run = self.answer(body, role)
            self.assertIn('error', json.loads(value))
            run.assert_not_called()

    def test_bad_lines_since_and_extra_fields_never_reach_journal(self):
        for key, values in [('lines', [0, -1, 1001, True, 1.5, '20', None]),
                            ('since', ['', '0h', '1s', '100000h', '1h --unit sshd', 'yesterday', None]),
                            ('role', ['admin'])]:
            for value in values:
                with self.subTest(key=key, value=value):
                    answer, run = self.answer(json.dumps({'method': 'logs', key: value}))
                    self.assertIn('error', json.loads(answer))
                    run.assert_not_called()
        for body in ['logs 0', 'logs 1001', 'logs 2 extra', 'logs since yesterday', 'logs --all']:
            answer, run = self.answer(body)
            self.assertTrue(answer.startswith('error:'))
            run.assert_not_called()

    def test_output_cap_is_real_and_reports_truncation(self):
        result = agent.execute([sys.executable, '-c', 'import sys;sys.stdout.write("x"*300000);sys.stderr.write("y"*100)'], output_limit=256 * 1024)
        self.assertEqual(result['status'], 0)
        self.assertLessEqual(len(result['stdout'].encode()) + len(result['stderr'].encode()), 256 * 1024)
        self.assertTrue(result['truncated'])

    def test_help_only_lists_the_callers_methods(self):
        import io
        from contextlib import redirect_stdout
        for role, expected in [('', ['list', 'timers', 'failed', 'status']), ('operate', ['list', 'timers', 'failed', 'status', 'start', 'restart', 'reload', 'logs']), ('admin', list(agent.TIERS))]:
            out = io.StringIO()
            with patch.dict(os.environ, {'AGENT_BUS_ASK': 'help', **({'AB_ROLE_' + role.upper(): '1'} if role else {})}, clear=True), redirect_stdout(out):
                agent.main()
            listed = [line.split()[0].rstrip(":") for line in out.getvalue().splitlines() if line.split() and line.split()[0].rstrip(":") in agent.TIERS]
            self.assertEqual(listed, sorted(expected))
