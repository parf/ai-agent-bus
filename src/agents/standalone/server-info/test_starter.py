"""Starter policy checks; no host units or bus records are changed."""
import argparse
import importlib.util
import os
from pathlib import Path
import pwd
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('starter', Path(__file__).parents[1] / 'install.py')
starter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(starter)


class StarterPolicy(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.args = argparse.Namespace(action='install', name='#server-info@parf.us', user='nobody',
            script=sys.executable, allow=None, bus_binary=sys.executable, token_binary=sys.executable, systemctl=sys.executable,
            useradd=sys.executable, unit_dir=str(self.root / 'units'), config_dir=str(self.root / 'config'))
        self.env = patch.dict(os.environ, {'AGENT_BUS_ADDR': 'http://localhost:1', 'AGENT_BUS_TOKEN': 'fixture'})
        self.env.start()
        self.addCleanup(self.env.stop)
        self.calls = []
        self.fragment = ''
        def command(argv, env=None, optional=False):
            self.calls.append(argv)
            out = '[]' if argv[1] == 'ls' else self.fragment if argv[1] == 'show' else 'agent-credential' if argv[1].startswith('#') else ''
            return subprocess.CompletedProcess(argv, 3 if argv[1] == 'is-active' else 0, out, '')
        self.command = patch.object(starter, 'command', side_effect=command)
        self.command.start()
        self.addCleanup(self.command.stop)

    def test_readable_names_and_instance(self):
        self.assertEqual(starter.install(self.args), 'agent-bus-server-info.service')
        self.args.name = '#systemd/web@h'
        self.assertEqual(starter.install(self.args), 'agent-bus-systemd-web.service')

    def test_foreign_unit_is_refused_by_name_unchanged(self):
        unit = self.root / 'units/agent-bus-server-info.service'
        unit.parent.mkdir()
        unit.write_text('[Service]\nExecStart=/bin/true\n')
        before = unit.read_bytes()
        with self.assertRaisesRegex(ValueError, 'agent-bus-server-info.service'):
            starter.install(self.args)
        self.assertEqual(unit.read_bytes(), before)
        self.assertFalse(list((self.root / 'config').glob('*.env')))
        self.assertFalse(any(a[1] == 'register' for a in self.calls))

    def test_vendor_unit_is_refused(self):
        self.fragment = '/usr/lib/systemd/system/agent-bus-server-info.service\n'
        with self.assertRaisesRegex(ValueError, 'agent-bus-server-info.service'):
            starter.install(self.args)
        self.assertFalse(list((self.root / 'units').glob('*.service')))

    def test_sanitized_or_other_host_collision_cannot_install_or_remove(self):
        self.args.name = '#systemd/web@h'
        starter.install(self.args)
        before = {p: p.read_bytes() for p in self.root.rglob('*') if p.is_file()}
        for name in ['#systemd-web@h', '#systemd/web@other']:
            for action in ['install', 'remove']:
                with self.subTest(name=name, action=action):
                    self.args.name, self.args.action = name, action
                    with self.assertRaisesRegex(ValueError, 'agent-bus-systemd-web.service'):
                        starter.install(self.args)
                    self.assertEqual({p: p.read_bytes() for p in before}, before)

    def test_sudo_uses_dedicated_account_and_retains_it_on_remove(self):
        self.args.user = None
        real_lookup = pwd.getpwnam
        def lookup(name):
            if name == 'ab-server-info':
                raise KeyError(name)
            return real_lookup(name)
        with patch.dict(os.environ, {'SUDO_UID': str(pwd.getpwnam('nobody').pw_uid)}), patch.object(starter.pwd, 'getpwnam', side_effect=lookup):
            starter.install(self.args)
        text = (self.root / 'units/agent-bus-server-info.service').read_text()
        self.assertIn('User=ab-server-info\n', text)
        created = next(a for a in self.calls if '--system' in a)
        self.assertIn('--no-create-home', created)
        self.assertEqual(created[created.index('--home-dir') + 1], '/nonexistent')
        self.assertTrue(created[created.index('--shell') + 1].endswith('/nologin'))
        self.assertEqual(created[-1], 'ab-server-info')
        self.args.action = 'remove'
        starter.install(self.args)
        self.assertFalse(any('userdel' in a[0] for a in self.calls))

    def test_refused_account_creation_falls_back_to_nobody(self):
        self.args.user = None
        original = starter.command
        def command(argv, env=None, optional=False):
            if '--system' in argv:
                return subprocess.CompletedProcess(argv, 1, '', 'refused')
            return original(argv, env, optional)
        real_lookup = pwd.getpwnam
        def lookup(name):
            if name == 'ab-server-info':
                raise KeyError(name)
            return real_lookup(name)
        with patch.object(starter, 'command', side_effect=command), patch.object(starter.pwd, 'getpwnam', side_effect=lookup):
            starter.install(self.args)
        self.assertIn('User=nobody\n', (self.root / 'units/agent-bus-server-info.service').read_text())

    def test_existing_dedicated_account_is_reused_but_a_human_is_refused(self):
        self.args.user = None
        for shell, home, allowed in [('/usr/sbin/nologin', '/nonexistent', True), ('/bin/bash', '/home/person', False)]:
            account = argparse.Namespace(pw_uid=123, pw_shell=shell, pw_dir=home)
            with patch.object(starter.pwd, 'getpwnam', return_value=account):
                if allowed:
                    starter.install(self.args)
                else:
                    with self.assertRaisesRegex(ValueError, 'ab-server-info'):
                        starter.install(self.args)
        self.assertFalse(any('--system' in a for a in self.calls))

    def test_root_requires_explicit_user(self):
        self.args.user = 'root'
        starter.install(self.args)
        self.assertIn('User=root\n', (self.root / 'units/agent-bus-server-info.service').read_text())
        self.assertFalse(any('--system' in a for a in self.calls))

    def test_only_the_agents_credential_is_stored(self):
        starter.install(self.args)
        token_file = self.root / 'config/agent-bus-server-info.env'
        text = token_file.read_text()
        self.assertIn('AGENT_BUS_TOKEN=agent-credential\n', text)
        self.assertNotIn('AGENT_BUS_TOKEN=fixture', text)
        unit = (self.root / 'units/agent-bus-server-info.service').read_text()
        self.assertIn('"--stay-registered"', unit)

    def test_remove_needs_the_callers_credential_without_reading_the_stored_one(self):
        starter.install(self.args)
        with patch.dict(os.environ, {'AGENT_BUS_TOKEN': ''}):
            with self.assertRaisesRegex(ValueError, 'AGENT_BUS_TOKEN'):
                self.args.action = 'remove'
                starter.install(self.args)
        self.assertTrue((self.root / 'config/agent-bus-server-info.env').exists())

    def test_remove_does_not_need_a_stored_token_file(self):
        starter.install(self.args)
        (self.root / 'config/agent-bus-server-info.env').unlink()
        self.args.action = 'remove'
        starter.install(self.args)
        self.assertFalse((self.root / 'units/agent-bus-server-info.service').exists())

    def test_generated_unit_has_no_systemd_warnings(self):
        self.args.user = "root"
        unit = starter.install(self.args)
        verified = subprocess.run(["systemd-analyze", "verify", "--man=no",
                                   str(self.root / "units" / unit)],
                                  capture_output=True, text=True, timeout=20)
        self.assertEqual(verified.returncode, 0, verified.stderr)
        self.assertEqual(verified.stdout + verified.stderr, "", "systemd verification must have no warnings")

    def test_environment_path_rejects_relative_or_special_characters(self):
        for value in ["relative", "/tmp/a b", "/tmp/a%b", "/tmp/a$b", '/tmp/a"b', "/tmp/a\\b"]:
            with self.subTest(value=value):
                self.args.config_dir = str(self.root / value.removeprefix("/tmp/")) if value.startswith("/tmp/") else value
                with self.assertRaisesRegex(ValueError, "config-dir"):
                    starter.install(self.args)
        self.assertEqual(self.calls, [])

    def test_fixture_rejects_quoted_environment_path(self):
        unit = starter.install(self.args)
        path = self.root / "units" / unit
        text = path.read_text()
        envfile = str(self.root / "config/agent-bus-server-info.env")
        path.write_text(text.replace("EnvironmentFile=" + envfile, 'EnvironmentFile="' + envfile + '"'))
        fixture = Path(__file__).parents[1] / "systemctl-fixture.py"
        result = subprocess.run([sys.executable, str(fixture), "start", unit],
                                env={**os.environ, "STANDALONE_TEST_ROOT": str(self.root)},
                                capture_output=True, text=True, timeout=5)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("EnvironmentFile path is not absolute", result.stderr)
        self.assertFalse((self.root / (unit + ".pid")).exists())
