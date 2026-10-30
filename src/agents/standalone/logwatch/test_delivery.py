"""Logwatch's real shared-engine path against a private daemon; no live bus or journal.

Like every fixture run, this suite is run while the caller holds smoke-gate.
"""
import json
import os
from pathlib import Path
import select
import subprocess
import sys
import tempfile
import time
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
sys.path.insert(0, str(HERE))
from logwatch import Agent
from watcher import configuration
from agent_bus import Bus


class SharedDelivery(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        cls.work = tempfile.TemporaryDirectory(prefix='logwatch-delivery-', dir=ROOT / 'tmp')
        cls.addClassCleanup(cls.work.cleanup)
        cls.fixture = str(Path(cls.work.name) / 'fixture')
        result = subprocess.run(['go', 'build', '-o', cls.fixture, './clients/fixture'],
                                cwd=ROOT / 'src', capture_output=True, text=True, timeout=30)
        if result.returncode: raise AssertionError(result.stderr)

    def setUp(self):
        self.clients = []
        self.agent = None
        self.process = subprocess.Popen([self.fixture], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.DEVNULL, text=True)
        self.addCleanup(self.close)
        self.assertTrue(select.select([self.process.stdout], [], [], 5)[0], 'private daemon did not start')
        info = json.loads(self.process.stdout.readline())
        def client(name):
            value = Bus(name, addr=info['addr'], token=info['tokens'][name], timeout=2)
            self.clients.append(value)
            return value
        self.owner, self.author, self.source, self.target = [client(name) for name in
            ('owner@fixture', '#caller@fixture', '#worker@fixture', '#helper@fixture')]
        self.owner.record(self.target.name).manage(bound=1, overflow='strict', allow=[self.author.name, self.source.name])
        self.warnings = []
        self.agent = Agent(self.source, configuration({'sources': [], 'excludes': []}), log=self.warnings.append)

    def close(self):
        if self.agent is not None: self.agent.close()
        for client in self.clients: client.close()
        if self.process.stdin: self.process.stdin.close()
        try: self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.kill(); self.process.wait(timeout=5)
        self.process.stdout.close()

    def command(self, body):
        self.author.send(self.source.name, body)
        message = self.source.consume(wait=1)
        self.assertIsNotNone(message)
        return self.agent.handle(message)

    def forward(self, text):
        self.agent.forward([self.agent.ring.add('fixture', text)])

    def take(self):
        return self.target.consume(wait=.01)

    def expect_body(self, body):
        received = self.take()
        self.assertIsNotNone(received, 'expected inbox copy is missing')
        self.assertEqual(received.body, body)

    def counts(self):
        rows = json.loads(self.command('sub list'))['subs']
        self.assertEqual(len(rows), 1)
        return rows[0]['sent'], rows[0]['dropped']

    def test_refused_copy_warns_once_and_the_next_match_arrives_once(self):
        self.assertIn('id', json.loads(self.command('sub add '+self.target.name+' /error/i')))
        self.forward('ERROR repeated')
        got = self.take(); self.assertIsNotNone(got)
        self.assertEqual((got.body, got.envelope['from'], got.topic, got.tag),
                         ('[fixture] ERROR repeated', self.source.name, 'logwatch', 'fixture'))
        self.assertIsNone(self.take())
        self.owner.send(self.target.name, 'fill')
        self.forward('ERROR repeated')
        self.assertEqual(self.warnings, ['subscription: send_refused'])
        self.expect_body('fill')
        self.assertIsNone(self.take(), 'the refused copy was delivered')
        self.forward('ERROR repeated')
        self.expect_body('[fixture] ERROR repeated')
        self.assertIsNone(self.take(), 'the next match was delivered twice')
        self.assertEqual(self.counts(), (2, 1))
        self.assertEqual(self.warnings, ['subscription: send_refused'])
        self.assertFalse(self.agent.subscriptions.processes)

    def test_revoked_author_drops_a_copy_without_pausing_the_subscription(self):
        self.command('sub add '+self.target.name)
        self.owner.record(self.target.name).manage(allow=[self.source.name])
        self.forward('revoked target')
        self.assertIsNone(self.take())
        self.assertEqual(self.warnings, ['subscription: admission_refused'])
        self.owner.record(self.target.name).manage(allow=[self.source.name, self.author.name])
        self.forward('restored target')
        self.expect_body('[fixture] restored target')
        self.assertIsNone(self.take())
        self.assertEqual(self.counts(), (1, 1))
        self.owner.record(self.source.name).manage(allow=[])
        self.forward('revoked source')
        self.assertIsNone(self.take())
        self.assertEqual(self.warnings, ['subscription: admission_refused'] * 2)


if __name__ == '__main__': unittest.main()
