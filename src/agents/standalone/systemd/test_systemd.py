import importlib.util
import json
import os
import subprocess
import sys
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('systemd_agent', Path(__file__).with_name('systemd.py'))
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)


class Systemd(unittest.TestCase):
    def test_help_forms_and_role_filtered_methods(self):
        program = str(Path(__file__).with_name('systemd.py'))
        for roles, tier in [({}, 0), ({'AB_ROLE_OPERATE':'1'}, 1), ({'AB_ROLE_ADMIN':'1'}, 2)]:
            env = {k:v for k,v in os.environ.items() if not k.startswith(('AB_ROLE_', 'AGENT_BUS_ASK'))}
            env.update(roles)
            body = subprocess.check_output([sys.executable, program, 'help'], env=env, text=True)
            ask = subprocess.check_output([sys.executable, program], env={**env, 'AGENT_BUS_ASK':'help'}, text=True)
            self.assertEqual(body, ask)
            for method, required in agent.TIERS.items():
                prefix = method + (':' if method in ('timers', 'failed') else ' ')
                self.assertEqual(any(line.startswith(prefix) for line in body.splitlines()), required <= tier, method)
            self.assertIn('every unit', body)
            self.assertIn('Asks: --help, --help-config, --help-secret', body)
            if tier:
                self.assertIn('1..1000', body)
                self.assertIn('since', body)

    def request(self, method, unit='nginx.service', **extra):
        return json.dumps({'method': method, **({} if method in ('list', 'timers', 'failed') else {'unit': unit}), **extra})

    def run_call(self, body, roles=(), status=0):
        env = {'AB_ROLE_' + role.upper(): '1' for role in roles}
        with patch.dict(os.environ, env, clear=True), patch.object(agent, 'execute', return_value={'stdout': 'host result', 'stderr': 'permission denied' if status else '', 'status': status}) as run:
            value = json.loads(agent.answer(body))
        return value, run

    def test_units_not_declared_at_install_are_accessible(self):
        for unit in ['sshd.service', 'backup.timer', 'docker.socket']:
            for method, roles in [('status', ()), ('start', ('operate',))]:
                with self.subTest(unit=unit, method=method):
                    value, run = self.run_call(self.request(method, unit), roles)
                    self.assertNotIn('error', value)
                    self.assertEqual(run.call_args.args[0][-2:], ['--', unit])
        value, run = self.run_call('{"method":"list"}')
        self.assertNotIn('--', run.call_args.args[0], 'list must not restrict the node unit inventory')
        value, run = self.run_call('{"method":"timers"}')
        self.assertNotIn('error', value)
        self.assertIn('list-timers', run.call_args.args[0])

    def test_delivered_role_matrix(self):
        for roles, tier in [((), 0), (('manager',), 0), (('owner',), 2), (('maintainer',), 2), (('operate',), 1), (('admin',), 2)]:
            for method, required in agent.TIERS.items():
                with self.subTest(roles=roles, method=method):
                    value, run = self.run_call(self.request(method), roles)
                    self.assertEqual('error' in value, tier < required)
                    self.assertEqual(run.call_count, int(tier >= required))
                    if tier >= required:
                        argv = run.call_args.args[0]
                        self.assertEqual(argv[:3], ['journalctl', '--no-pager', '-o'] if method == 'logs' else ['systemctl', '--no-pager', '--no-ask-password'])
                        self.assertEqual(argv[3], 'short-iso' if method == 'logs' else {'list':'list-units', 'timers':'list-timers', 'failed':'list-units'}.get(method, method))

    def test_list_types_and_no_argument_methods(self):
        for kind in ['service', 'timer', 'socket', 'mount', 'path', 'target', 'slice', 'scope', 'automount', 'swap', 'device']:
            value, run = self.run_call(json.dumps({'method':'list', 'type':kind}))
            self.assertNotIn('error', value)
            self.assertEqual(run.call_args.args[0], ['systemctl','--no-pager','--no-ask-password','list-units','--all','--plain','--no-legend','--type='+kind])
        for body in ['list --all', 'list *.service', 'list service extra', 'timers extra', 'failed extra', '{"method":"list","type":null}', '{"method":"list","type":[]}', '{"method":"list","unit":"x.service"}']:
            with patch.object(agent, 'execute') as run:
                self.assertIn('error', agent.answer(body))
                run.assert_not_called()
        value, run = self.run_call('{"method":"failed"}')
        self.assertEqual(run.call_args.args[0], ['systemctl','--no-pager','--no-ask-password','list-units','--failed','--all','--plain','--no-legend'])

    def test_no_failed_units_is_explicit_in_text(self):
        with patch.object(agent, 'execute', return_value={'stdout':'', 'stderr':'', 'status':0}):
            self.assertEqual(agent.answer('failed'), 'no failed units')
            self.assertEqual(json.loads(agent.answer('{"method":"failed"}'))['units'], [])

    def test_failed_json_has_unit_state_columns(self):
        with patch.object(agent, 'execute', return_value={'stdout':'bad.service loaded failed failed Broken worker\n', 'stderr':'', 'status':0}):
            value = json.loads(agent.answer('{"method":"failed"}'))
            self.assertEqual(value['units'], [{'unit':'bad.service','load':'loaded','active':'failed','sub':'failed','description':'Broken worker'}])
            self.assertEqual(agent.answer('failed'), 'bad.service loaded failed failed Broken worker')

    def test_body_roles_and_extra_arguments_are_refused(self):
        for extra in [{'role': 'admin'}, {'roles': ['admin']}, {'AB_ROLE_ADMIN': '1'}, {'args': ['--all']}]:
            value, run = self.run_call(self.request('start', **extra))
            self.assertIn('error', value)
            run.assert_not_called()

    def test_unit_validation_precedes_systemctl(self):
        for unit in ['--all', '-x.service', '*', '*.service', '/tmp/x.service', '../x.service', 'x.service;id', 'x.service\nstop y.service', 'x.service x.service', 'x', 'x@.service', 'x@@y.service', 'x' * 256 + '.service', None, ['x.service']]:
            with self.subTest(unit=unit):
                value, run = self.run_call(self.request('status', unit), ('admin',))
                self.assertIn('invalid unit', value['error'])
                run.assert_not_called()
        for text in ['start nginx.service --all', 'status nginx.service;id', 'list nginx.service']:
            with patch.object(agent, 'execute') as run:
                self.assertTrue(agent.answer(text).startswith('error:'))
                run.assert_not_called()

    def test_every_systemctl_refusal_is_an_error_answer(self):
        for method in agent.TIERS:
            with self.subTest(method=method):
                value, run = self.run_call(self.request(method), ('admin',), status=1)
                self.assertEqual(run.call_count, 1)
                self.assertIn('error', value)
                self.assertEqual(value['status'], 1)
                self.assertEqual(value['stderr'], 'permission denied')

    def test_text_and_json_return_the_same_result(self):
        with patch.dict(os.environ, {}, clear=True), patch.object(agent, 'execute', return_value={'stdout': 'active\n', 'stderr': '', 'status': 0}):
            self.assertEqual(agent.answer('status nginx.service'), 'active')
            self.assertEqual(json.loads(agent.answer(self.request('status')))['stdout'], 'active\n')
