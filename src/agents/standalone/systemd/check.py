#!/usr/bin/env python3
"""Root test only: private bus, private unit controller, no host systemd writes."""
import json
import os
from pathlib import Path
import pwd
import shutil
import signal
import subprocess
import sys
import time
import unittest

HERE = Path(__file__).resolve().parent
BIN = Path(os.environ['CLIENT_BIN'])
BASE = Path(os.environ['AB_TEST_DIR'])
ROOT = BASE / 'systemd-fixture'
OWNER = os.environ['AB_TEST_OWNER']
NAME = '#systemd/web@localhost'
ROLES = {'plain': '', 'operator': '(operate)', 'admin': '(admin)'}
ENV = {k: v for k, v in os.environ.items() if not k.startswith(('AGENT_BUS_', 'AB_ROLE_'))}
ENV.update(AGENT_BUS_ADDR=os.environ['AB_TEST_TCP'], AGENT_BUS_TOKEN=os.environ['AB_TEST_OWNER_TOKEN'],
           XDG_STATE_HOME=str(ROOT / 'admin-state'), XDG_CACHE_HOME=str(ROOT / 'admin-cache'),
           B2_UNIT_ROOT=str(ROOT), B2_HOST_STATE=str(ROOT / 'host.json'), B2_HOST_CALLS=str(ROOT / 'calls.jsonl'))
TOKENS = {}
ACTING = ['start', 'restart', 'reload', 'stop', 'enable', 'disable']


def cli(*args, token=None):
    return subprocess.run([str(BIN / 'agent-bus'), *args], env={**ENV, 'AGENT_BUS_TOKEN': token or ENV['AGENT_BUS_TOKEN']}, capture_output=True, text=True, timeout=30)


def starter(action, user='root'):
    return subprocess.run([sys.executable, str(HERE / 'install.py'), action, '--name', NAME,
        '--user', user,
        '--allow', ','.join('#' + name + '@localhost' + role for name, role in ROLES.items()),
        '--bus-binary', str(ROOT / 'bin/agent-bus'), '--token-binary', str(BIN / 'agent-bus-token'),
        '--systemctl', str(HERE / 'fixture-unit.py'), '--unit-dir', str(ROOT / 'units'),
        '--config-dir', str(ROOT / 'config')], env=ENV, capture_output=True, text=True, timeout=60)


def request(who, method, unit='nginx.service', **extra):
    body = {'method': method, **({} if method in ('list', 'timers', 'failed') else {'unit': unit}), **extra}
    result = cli('call', NAME, '--wait', '10s', json.dumps(body), token=TOKENS[who])
    if result.returncode:
        raise AssertionError('fixture call failed: ' + result.stderr)
    return json.loads(json.loads(result.stdout)['body'])


def ready():
    until = time.monotonic() + 10
    while time.monotonic() < until:
        if any(r['name'] == NAME and r.get('readers', 0) for r in json.loads(cli('ls', '--all').stdout)):
            return
        time.sleep(.02)
    raise AssertionError('private unit never attached its reader')


class SystemdInstalled(unittest.TestCase):
    def test_all_units_roles_privilege_and_install_lifecycle(self):
        self.assertEqual(os.geteuid(), 0, 'this isolated check needs sudo; it creates no host units')
        original_mode = BASE.stat().st_mode & 0o777
        BASE.chmod(0o755)  # nobody must traverse only this disposable fixture.
        ROOT.mkdir(mode=0o755)
        (ROOT / 'bin').mkdir()
        shutil.copy2(BIN / 'agent-bus', ROOT / 'bin/agent-bus')
        (ROOT / 'bin/agent-bus').chmod(0o755)
        (ROOT / 'bin/systemctl').symlink_to(HERE / 'fixture-host.py')
        (ROOT / 'host.json').write_text('{"nginx.service":"active","sshd.service":"active","backup.timer":"active","bad.service":"failed"}')
        (ROOT / 'host.json').chmod(0o644)
        (ROOT / 'calls.jsonl').touch(mode=0o666)
        (ROOT / 'calls.jsonl').chmod(0o666)
        try:
            for name in ROLES:
                result = cli('register', '#' + name + '@localhost', '--kind', 'agent', '--allow', NAME)
                self.assertEqual(result.returncode, 0, result.stderr)
                token = subprocess.run([str(BIN / 'agent-bus-token'), '#' + name + '@localhost'], env=ENV, capture_output=True, text=True, timeout=10)
                self.assertEqual(token.returncode, 0)
                TOKENS[name] = token.stdout.strip()
            installed = starter('install')
            self.assertEqual(installed.returncode, 0, installed.stderr)
            unit = installed.stdout.strip()
            self.assertEqual(unit, 'agent-bus-systemd-web.service')
            ready()
            record = next(r for r in json.loads(cli('ls', '--all').stdout) if r['name'] == NAME)
            self.assertIn('systemd', record.get('tags', []))
            self.assertEqual(record.get('descr'), json.loads((HERE / 'agent.json').read_text())['description'])
            pidfile = ROOT / (unit + '.pid')
            pid = pidfile.read_text()
            again = starter('install')
            self.assertEqual(again.returncode, 0, again.stderr)
            self.assertEqual(pidfile.read_text(), pid)
            self.assertEqual(len(list((ROOT / 'units').glob('*.service'))), 1)
            self.assertEqual(sum(r['name'] == NAME for r in json.loads(cli('ls', '--all').stdout)), 1)
            for who, allowed in [('plain', []), ('operator', ACTING[:3]), ('admin', ACTING)]:
                for method in ['list', 'timers', 'failed', 'status', *ACTING]:
                    with self.subTest(who=who, method=method):
                        result = request(who, method)
                        self.assertEqual('error' in result, method not in ['list', 'timers', 'failed', 'status', *allowed])
                        if 'error' not in result:
                            self.assertEqual(result['status'], 0)
            # Neither of these names was provided to the installer.
            for target in ['sshd.service', 'backup.timer']:
                self.assertIn(target, request('plain', 'list')['stdout'])
                self.assertNotIn('error', request('plain', 'status', target))
                self.assertNotIn('error', request('operator', 'start', target))
            timers = request('plain', 'list', type='timer')
            self.assertIn('backup.timer', timers['stdout'])
            self.assertNotIn('sshd.service', timers['stdout'])
            self.assertIn('backup.timer', request('plain', 'timers')['stdout'])
            self.assertEqual(request('plain', 'failed')['units'][0]['unit'], 'bad.service')
            self.assertIn('error', request('plain', 'start', role='admin'))
            self.assertIn('error', request('admin', 'start', '--all'))
            calls = [json.loads(row) for row in (ROOT / 'calls.jsonl').read_text().splitlines()]
            self.assertTrue(calls)
            self.assertTrue(all(c['uid'] == 0 for c in calls))
            self.assertEqual(json.loads((ROOT / 'host.json').read_text())['sshd.service'], 'start')
            # Configuration is read by each args-mode call, without reinstalling.
            self.assertEqual(cli('agent-template', NAME, '{"units":" nginx.service, backup.timer "}').returncode, 0)
            self.assertNotIn('sshd.service', request('plain', 'list')['stdout'])
            self.assertIn('backup.timer', request('plain', 'timers')['stdout'])
            self.assertEqual(request('plain', 'failed')['units'], [])
            before = (ROOT / 'calls.jsonl').read_text()
            for method in ['status', *ACTING, 'logs']:
                refused = request('admin', method, 'sshd.service')
                self.assertIn('sshd.service', refused['error'])
            self.assertEqual((ROOT / 'calls.jsonl').read_text(), before)
            self.assertNotIn('error', request('operator', 'start', 'backup.timer'))
            self.assertEqual(cli('agent-template', NAME, '{}').returncode, 0)
            # The same unit with a real unprivileged UID loses write permission
            # on host.json; no fake environment flag stands in for privilege.
            changed = starter('install', user='nobody')
            self.assertEqual(changed.returncode, 0, changed.stderr)
            ready()
            for method in ['list', 'timers', 'failed', 'status']:
                self.assertNotIn('error', request('plain', method))
            for method in ACTING:
                result = request('admin', method)
                self.assertIn('error', result, 'lost privilege must never look successful')
                self.assertNotEqual(result['status'], 0)
                self.assertIn('Permission denied', result['stderr'])
            calls = [json.loads(row) for row in (ROOT / 'calls.jsonl').read_text().splitlines()]
            self.assertTrue(any(c['uid'] == pwd.getpwnam('nobody').pw_uid for c in calls))
            removed = starter('remove')
            self.assertEqual(removed.returncode, 0, removed.stderr)
            self.assertFalse(list((ROOT / 'units').glob('*.service')))
            self.assertFalse(list((ROOT / 'config').glob('*.env')))
            self.assertFalse(any(r['name'] == NAME for r in json.loads(cli('ls', '--all').stdout)))
            self.assertEqual(starter('remove').returncode, 0)
        finally:
            starter('remove')
            for pidfile in ROOT.glob('*.pid'):
                subprocess.run([str(HERE / 'fixture-unit.py'), 'disable', '--now', pidfile.name[:-4]], env=ENV, timeout=45)
            shutil.rmtree(ROOT)
            BASE.chmod(original_mode)


if __name__ == '__main__':
    unittest.main(verbosity=2, failfast=True)
