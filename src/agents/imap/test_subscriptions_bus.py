"""Authenticated envelopes, the shared send helper and RD141 admission on a real fixture daemon only."""
import json
import os
from pathlib import Path
import select
import subprocess
import tempfile
import unittest

import test_imap  # noqa: F401  (sets the import path)
from agent_bus import Bus
from forwarding import Forwarder, shared
from test_forwarding import Clock, Source

HERE = Path(__file__).resolve().parent


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


if __name__ == '__main__':
    unittest.main()
