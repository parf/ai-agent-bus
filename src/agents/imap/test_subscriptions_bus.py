"""Authenticated envelopes and RD141 admission on a real fixture daemon only."""
import copy
import json
import os
from pathlib import Path
import select
import subprocess
import tempfile
import unittest

import test_imap
from agent_bus import Bus
import delivery
import ledger
import outbox
from subscriptions import SubscriptionError, Subscriptions

HERE = Path(__file__).resolve().parent


class SubscriptionBusTests(unittest.TestCase):
    def test_authenticated_author_admission_revocation_and_delete_release_bodies(self):
        fixture = Path(os.environ.get('IMAP_BUS_FIXTURE', HERE.parents[2] / 'tmp/imap-client-fixture'))
        self.assertTrue(fixture.is_file(), 'build the isolated clients/fixture binary')
        root = HERE.parents[2] / 'tmp'
        root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='imap-subs-bus-', dir=root) as work:
            process = subprocess.Popen([str(fixture)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                       stderr=subprocess.PIPE, text=True)
            buses = []
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
                store = Subscriptions(imap, imap.name, lambda: ('account', 7, 100))
                author.send(imap.name, 'sub add ' + receiver.name)
                envelope = imap.consume(wait=.1)
                self.assertEqual(envelope.envelope['from'], author.name)
                sub = store.add(envelope, receiver.name)
                with outbox.Outbox(work) as bodies:
                    def save(value):
                        sub['ledger'] = copy.deepcopy(value)
                        store.save(sub)
                    raw = b'From: mail@example.test\r\nSubject: fixture\r\n\r\nexternal mail'
                    value = delivery.queue(sub['ledger'], 100, raw, bodies, sub['created_by'], sub['id'], [], save)
                    value = delivery.execute(value, 100, bodies, sub['created_by'], sub['id'], [], save,
                                             lambda body: imap.send(sub['target'], body), 1000,
                                             target=sub['target'], authorize=store.author_allowed)
                    received = receiver.consume(wait=.1)
                    self.assertIsNotNone(received)
                    self.assertEqual(received.envelope['from'], imap.name)
                    self.assertIn('created by ' + author.name, received.body.split('\n')[0])
                    self.assertEqual(value['sent'], 1)
                    self.assertIsNone(receiver.consume(wait=.01))
                    # Receiver still admits IMAP, but the authenticated creator
                    # has lost admission: RD141 must stop the next forward.
                    owner.record(receiver.name).manage(allow=[imap.name])
                    value = delivery.queue(value, 101, raw, bodies, sub['created_by'], sub['id'], [], save)
                    value = delivery.execute(value, 101, bodies, sub['created_by'], sub['id'], [], save,
                                             lambda body: imap.send(sub['target'], body), 1010,
                                             target=sub['target'], authorize=store.author_allowed)
                    self.assertEqual(value['pause'], 'acl')
                    self.assertEqual(value['entries']['101']['attempts'], 0)
                    self.assertIsNone(receiver.consume(wait=.01))
                    with self.assertRaises(SubscriptionError):
                        store.add(envelope, receiver.name)
                    self.assertEqual(len(store.all()[0]), 1)
                    stranger.send(imap.name, 'sub delete ' + sub['id'])
                    stranger_message = imap.consume(wait=.1)
                    with self.assertRaises(SubscriptionError):
                        store.delete(stranger_message, sub['id'])
                    def cleanup():
                        values, damaged = store.all()
                        self.assertEqual(damaged, {})
                        keep = {outbox.filename(v['ledger']['identity'], v['ledger']['validity'], uid)
                                for v in values for uid in ledger.retained(v['ledger'])}
                        bodies.sweep(keep)
                    store.cleanup = cleanup
                    store.delete(envelope, sub['id'])
                    self.assertEqual(bodies.listing(), {})
            finally:
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
