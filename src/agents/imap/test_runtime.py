"""Exported entrypoint, fixture bus and IMAPS; no host account or provider."""
import json
import os
from pathlib import Path
import select
import socket
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import test_imap as fixtures
from agent_bus import Bus, CallReply

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


class Runtime(unittest.TestCase):
    setUpClass = classmethod(fixtures.IMAPTests.setUpClass.__func__)

    def setUp(self):
        fixtures.IMAPTests.setUp(self)

    def test_exported_help_without_credentials_or_identity(self):
        package = Path(self.work.name) / 'exported'
        subprocess.run([str(HERE / 'package.sh'), str(package)], check=True, capture_output=True)
        env = {'PATH': os.environ['PATH'], 'PYTHONDONTWRITEBYTECODE': '1', 'AGENT_BUS_ASK': 'help-json',
               'AGENT_BUS_NAME': '#fixture@invalid', 'AGENT_BUS_TOKEN': 'fixture-only', 'AGENT_BUS_ADDR': 'http://127.0.0.1:1'}
        result = subprocess.run([str(package / 'imap.py')], env=env, cwd=self.work.name, capture_output=True, text=True, timeout=3)
        self.assertEqual(result.returncode, 0, result.stderr)
        metadata = json.loads(result.stdout)
        self.assertIn('publish', metadata['methods'])
        self.assertEqual(metadata['body_formats'], ['text', 'json'])
        self.assertTrue(metadata['docs'].endswith('/imap.md'))
        self.assertTrue(all(not field['has_default'] for field in metadata['secrets']))
        self.assertEqual(json.loads((package / 'agent.json').read_text())['serves'], 'itself')
        self.assertTrue((package / 'support/clients/LICENSE.md').is_file())

    def test_real_bus_entrypoint_publishes_raw_text_and_ignores_replies(self):
        fixture = Path(os.environ.get('IMAP_BUS_FIXTURE', ROOT / 'tmp/imap-client-fixture'))
        self.assertTrue(fixture.is_file(), 'build clients/fixture for the isolated bus check')
        patch.stopall()
        actual = socket.create_connection
        process = subprocess.Popen([str(fixture)], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        agent = None
        buses = []
        try:
            ready, _, _ = select.select([process.stdout], [], [], 5)
            self.assertTrue(ready, 'fixture bus did not start')
            info = json.loads(process.stdout.readline())
            from urllib.parse import urlsplit
            port = urlsplit(info['addr']).port
            def only_fixtures(address, *args, **kwargs):
                if address[0] != '127.0.0.1' or address[1] not in {port, self.server.server_address[1]}:
                    raise AssertionError('external connection forbidden')
                return actual(address, *args, **kwargs)
            patch('socket.create_connection', side_effect=only_fixtures).start()
            def client(name):
                bus = Bus(name, addr=info['addr'], token=info['tokens'][name], timeout=2)
                buses.append(bus)
                return bus
            owner, caller, destination = [client(name) for name in ('owner@fixture', '#caller@fixture', '#helper@fixture')]
            owner.record(destination.name).manage(allow=['#worker@fixture'])
            package = Path(self.work.name) / 'exported'
            subprocess.run([str(HERE / 'package.sh'), str(package)], check=True, capture_output=True)
            cfg = {**self.cfg, 'target': destination.name}
            env = {key: value for key, value in os.environ.items() if not key.startswith(('AGENT_BUS_', 'AB_ROLE_', 'IMAP_', 'SMTP_'))}
            env.update(self.env)
            env.update(AGENT_BUS_NAME='#worker@fixture', AGENT_BUS_TOKEN=info['tokens']['#worker@fixture'],
                       AGENT_BUS_ADDR=info['addr'], AGENT_BUS_CONFIG=json.dumps(cfg), AGENT_BUS_WORK=self.work.name,
                       PYTHONDONTWRITEBYTECODE='1')
            agent = subprocess.Popen([str(package / 'imap.py')], env=env, cwd=self.work.name,
                                     stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            result = caller.call('#worker@fixture', 'publish 7', timeout=5)
            self.assertIsInstance(result, CallReply)
            self.assertTrue(result.message.body.startswith('published UID 7'), result.message.body)
            delivered = destination.consume(wait=.1)
            self.assertIsNotNone(delivered)
            self.assertEqual(delivered.envelope['from'], '#worker@fixture')
            self.assertEqual(delivered.body, fixtures.proxy.WARNING + fixtures.RAW.decode())
            self.assertEqual(len(self.server.fetches), 2)
            caller.reply(result.message, 'publish 7')
            again = caller.call('#worker@fixture', 'list 1', timeout=5)
            self.assertIn('UID 7 (cached; unread)', again.message.body)
            self.assertIsNone(destination.consume(wait=.05))
            self.assertEqual(len(self.server.fetches), 2)
            agent.terminate()
            self.assertEqual(agent.wait(timeout=3), 0)
        finally:
            if agent is not None:
                if agent.poll() is None:
                    agent.terminate()
                    try:
                        agent.wait(timeout=3)
                    except subprocess.TimeoutExpired:
                        agent.kill(); agent.wait(timeout=2)
                agent.stdout.close(); agent.stderr.close()
            for bus in buses:
                bus.close()
            process.stdin.close()
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                process.kill(); process.wait(timeout=2)
            process.stdout.close(); process.stderr.close()


if __name__ == '__main__':
    unittest.main()
