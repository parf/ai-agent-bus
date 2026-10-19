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
            body = subprocess.check_output([sys.executable, program, '--units-file', 'unused', 'help'], env=env, text=True)
            ask = subprocess.check_output([sys.executable, program], env={**env, 'AGENT_BUS_ASK':'help'}, text=True)
            self.assertEqual(body, ask)
            for method, required in agent.TIERS.items():
                prefix = method + (':' if method == 'list' else ' ')
                self.assertEqual(any(line.startswith(prefix) for line in body.splitlines()), required <= tier, method)
            self.assertIn('declared units', body)
            self.assertIn('Example:', body)
            if tier:
                self.assertIn('1..1000', body)
                self.assertIn('since', body)

    def request(self, method, unit='nginx.service', **extra):
        return json.dumps({'method': method, **({} if method == 'list' else {'unit': unit}), **extra})

    def run_call(self, body, roles=(), status=0):
        env = {'AB_ROLE_' + role.upper(): '1' for role in roles}
        with patch.dict(os.environ, env, clear=True), patch.object(agent, 'configuration', return_value=['nginx.service', 'worker@one.service']), patch.object(agent, 'execute', return_value={'stdout': 'host result', 'stderr': 'permission denied' if status else '', 'status': status}) as run:
            value = json.loads(agent.answer(body, 'units.json'))
        return value, run

    def test_delivered_role_matrix(self):
        for roles, tier in [((), 0), (('owner', 'manager'), 0), (('operate',), 1), (('admin',), 2)]:
            for method, required in agent.TIERS.items():
                with self.subTest(roles=roles, method=method):
                    value, run = self.run_call(self.request(method), roles)
                    self.assertEqual('error' in value, tier < required)
                    self.assertEqual(run.call_count, int(tier >= required))
                    if tier >= required:
                        argv = run.call_args.args[0]
                        self.assertEqual(argv[:3], ['journalctl', '--no-pager', '-o'] if method == 'logs' else ['systemctl', '--no-pager', '--no-ask-password'])
                        self.assertEqual(argv[3], 'short-iso' if method == 'logs' else 'list-units' if method == 'list' else method)

    def test_admin_cannot_escape_the_declared_units(self):
        for roles in [(), ('operate',), ('admin',)]:
            for method in agent.TIERS:
                if method == 'list':
                    continue
                with self.subTest(roles=roles, method=method):
                    value, run = self.run_call(self.request(method, 'sshd.service'), roles)
                    self.assertIn('error', value)
                    run.assert_not_called()
        value, run = self.run_call(self.request('list'), ('admin',))
        self.assertEqual(run.call_args.args[0][-3:], ['--', 'nginx.service', 'worker@one.service'])

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
                self.assertTrue(agent.answer(text, 'units.json').startswith('error:'))
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
        with patch.dict(os.environ, {}, clear=True), patch.object(agent, 'configuration', return_value=['nginx.service']), patch.object(agent, 'execute', return_value={'stdout': 'active\n', 'stderr': '', 'status': 0}):
            self.assertEqual(agent.answer('status nginx.service', 'units.json'), 'active')
            self.assertEqual(json.loads(agent.answer(self.request('status'), 'units.json'))['stdout'], 'active\n')

    def test_configuration_is_root_owned_and_never_an_empty_or_pattern_scope(self):
        import argparse
        import tempfile
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'units.json'
            for uid, mode, allowed in [(0, 0o100644, True), (1000, 0o100644, False), (0, 0o100666, False)]:
                path.write_text('{"units":["nginx.service"]}')
                with patch.object(agent.os, 'fstat', return_value=argparse.Namespace(st_uid=uid, st_mode=mode)):
                    if allowed:
                        self.assertEqual(agent.configuration(path), ['nginx.service'])
                    else:
                        with self.assertRaisesRegex(ValueError, 'root-owned'):
                            agent.configuration(path)
            for value in [{"units": []}, {"units": ['*.service']}, {"units": ['nginx.service', 'nginx.service']}, {"units": ['nginx.service'], "role": 'admin'}]:
                path.write_text(json.dumps(value))
                with patch.object(agent.os, 'fstat', return_value=argparse.Namespace(st_uid=0, st_mode=0o100644)):
                    with self.assertRaises(ValueError):
                        agent.configuration(path)
