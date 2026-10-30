"""Authenticated envelopes, the shared send helper and RD141 admission on a real fixture daemon only."""
import json
import os
from pathlib import Path
import select
import signal
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest

import test_imap  # noqa: F401  (sets the import path)
from agent_bus import Bus
from forwarding import Forwarder, JOURNAL, PROGRESS, shared
from test_forwarding import Clock, Source, ledger

HERE = Path(__file__).resolve().parent

# A separate process runs the real migration and is SIGKILLed by its own hand
# after the new row is stored and before the old key is deleted.
KILLED_MIGRATION = """
import json, os, signal, sys
sys.path.insert(0, sys.argv[1])
import test_imap
from agent_bus import Bus
from forwarding import Forwarder
from test_forwarding import Clock, Source
info = json.load(sys.stdin)
bus = Bus(info['name'], addr=info['addr'], token=info['token'], timeout=2)
kv = type(bus.record(info['name']).kv)
delete = kv.delete
def killed(self, key, *args, **kwargs):
    if key == 'sub.aabbccdd':
        os.kill(os.getpid(), signal.SIGKILL)
    return delete(self, key, *args, **kwargs)
kv.delete = killed
Forwarder(bus, info['name'], info['work'], Source(), Clock(), Clock())
"""


class Fixture:
    def __init__(self, test):
        path = Path(os.environ.get('IMAP_BUS_FIXTURE', HERE.parents[2] / 'tmp/imap-client-fixture'))
        test.assertTrue(path.is_file(), 'build the isolated clients/fixture binary')
        self.process = subprocess.Popen([str(path)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, text=True)
        self.buses = []
        test.assertTrue(select.select([self.process.stdout], [], [], 5)[0], 'fixture did not start')
        self.info = json.loads(self.process.stdout.readline())

    def client(self, name):
        bus = Bus(name, addr=self.info['addr'], token=self.info['tokens'][name], timeout=2)
        self.buses.append(bus)
        return bus

    def close(self):
        for bus in self.buses:
            bus.close()
        self.process.stdin.close()
        try:
            self.process.wait(timeout=3)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait(timeout=2)
        self.process.stdout.close()
        self.process.stderr.close()


class SubscriptionBusTests(unittest.TestCase):
    def test_authenticated_author_admission_revocation_and_delete(self):
        fixture = Path(os.environ.get('IMAP_BUS_FIXTURE', HERE.parents[2] / 'tmp/imap-client-fixture'))
        self.assertTrue(fixture.is_file(), 'build the isolated clients/fixture binary')
        root = HERE.parents[2] / 'tmp'
        root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='imap-subs-bus-', dir=root) as work:
            process = subprocess.Popen([str(fixture)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                       stderr=subprocess.PIPE, text=True)
            buses, worker = [], None
            try:
                self.assertTrue(select.select([process.stdout], [], [], 5)[0], 'fixture did not start')
                info = json.loads(process.stdout.readline())
                def client(name):
                    bus = Bus(name, addr=info['addr'], token=info['tokens'][name], timeout=2)
                    buses.append(bus)
                    return bus
                owner, author, imap, receiver, stranger = [client(name) for name in (
                    'owner@fixture', '#caller@fixture', '#worker@fixture', '#helper@fixture', '#outsider@fixture')]
                owner.record(receiver.name).manage(allow=[author.name, imap.name])
                source, clock = Source(), Clock()
                # No send_one: copies leave through the engine's own helper process, as the agent sends them.
                worker = Forwarder(imap, imap.name, work, source, clock, clock)
                author.send(imap.name, 'sub add ' + receiver.name)
                envelope = imap.consume(wait=.1)
                self.assertEqual(envelope.envelope['from'], author.name)
                sub = worker.add(envelope, receiver.name, '')
                key = 'sub.' + sub['id']

                source.arrive(100)
                worker.tick()
                received = receiver.consume(wait=1)
                self.assertIsNotNone(received)
                self.assertEqual((received.envelope['from'], received.envelope.get('tag')), (imap.name, sub['id']))
                self.assertIn('created by ' + author.name, received.body.split('\n')[0])
                self.assertIsNone(receiver.consume(wait=.05))
                self.assertEqual((imap.record(imap.name).kv.get(key, kind='json')['sent'], worker.sent), (1, 1))

                # Receiver still admits IMAP, but the authenticated creator has lost
                # admission: RD141 drops the next copy and the subscription stays.
                owner.record(receiver.name).manage(allow=[imap.name])
                source.arrive(101)
                clock.advance(60)
                worker.tick()
                self.assertIsNone(receiver.consume(wait=.05))
                row = imap.record(imap.name).kv.get(key, kind='json')
                self.assertEqual((row['sent'], row['dropped']), (1, 1))
                self.assertEqual(worker.warnings, {'admission_refused': 1})
                with self.assertRaises(shared.SubscriptionError):
                    worker.add(envelope, receiver.name, '')

                stranger.send(imap.name, 'sub delete ' + sub['id'])
                with self.assertRaises(shared.SubscriptionError):
                    worker.delete(imap.consume(wait=.1), sub['id'])
                self.assertIn(sub['id'], worker.text(envelope))
                worker.delete(envelope, sub['id'])
                self.assertNotIn(sub['id'], worker.text(envelope))
            finally:
                if worker:
                    worker.close()
                for bus in buses:
                    bus.close()
                process.stdin.close()
                try:
                    process.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=2)
                process.stdout.close()
                process.stderr.close()


    def test_a_process_killed_between_the_add_and_the_delete_leaves_one_subscription_delivered_once(self):
        root = HERE.parents[2] / 'tmp'
        root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='imap-migrate-kill-', dir=root) as work:
            fixture, worker = Fixture(self), None
            try:
                owner, author, imap, receiver = [fixture.client(name) for name in (
                    'owner@fixture', '#caller@fixture', '#worker@fixture', '#helper@fixture')]
                owner.record(receiver.name).manage(allow=[author.name, imap.name])
                kv = imap.record(imap.name).kv
                kv.set('sub.aabbccdd', {'id': 'aabbccdd', 'target': receiver.name, 'created_by': author.name,
                                        'created_at': '2026-10-29T12:00:00+00:00', 'filter': '', 'ledger': ledger()},
                       kind='json', mode='add')
                kv.set(PROGRESS, {'version': 1, 'identity': 'account', 'validity': 7, 'cursor': 0, 'retry': None,
                                  'baseline': 90, 'observe': 100, 'history': 89, 'history_ceiling': 89},
                       kind='json', mode='add')
                child = subprocess.run([sys.executable, '-B', '-c', KILLED_MIGRATION, str(HERE)], cwd=HERE, timeout=30,
                                       input=json.dumps({'name': imap.name, 'addr': fixture.info['addr'],
                                                         'token': fixture.info['tokens'][imap.name], 'work': work}),
                                       capture_output=True, text=True)
                self.assertEqual(child.returncode, -signal.SIGKILL, child.stderr)
                names = {row['name'] for row in kv.list()['json']}
                self.assertTrue({'sub.aabbccdd', 'sub.aabbccdd0000', JOURNAL + 'aabbccdd'} <= names, names)

                source, clock = Source(), Clock()
                worker = Forwarder(imap, imap.name, work, source, clock, clock)
                names = {row['name'] for row in kv.list()['json']}
                self.assertEqual(sorted(n for n in names if n.startswith('sub.')), ['sub.aabbccdd0000'])
                self.assertNotIn(JOURNAL + 'aabbccdd', names)
                source.arrive(100)
                worker.tick()
                received = receiver.consume(wait=1)
                self.assertIsNotNone(received)
                self.assertEqual(received.envelope.get('tag'), 'aabbccdd0000')
                self.assertIsNone(receiver.consume(wait=.05))
                self.assertEqual(kv.get('sub.aabbccdd0000', kind='json')['sent'], 5)
                listed = worker.text(SimpleNamespace(envelope={'from': author.name, 'roles': [], 'original_to': imap.name}))
                self.assertEqual(listed.count(receiver.name + ' by=' + author.name), 1)
            finally:
                if worker:
                    worker.close()
                fixture.close()


if __name__ == '__main__':
    unittest.main()
