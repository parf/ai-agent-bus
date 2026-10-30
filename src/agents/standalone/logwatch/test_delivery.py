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
import subscription_migration as migration
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


    def test_sigkill_between_new_commit_and_old_delete_finishes_before_serving(self):
        old_key = 'sub.old-format'
        legacy = {'id':'old-format','target':self.target.name,'filter':'','created_by':self.author.name}
        kv = self.source.record(self.source.name).kv
        kv.set(old_key, legacy, kind='json', mode='add')
        code = (
            'import os, sys, time; sys.path.insert(0, %r)\n'
            'from logwatch import Agent\n'
            'from watcher import configuration\n'
            'from agent_bus import Bus\n'
            'with Bus(os.environ["SOURCE_NAME"], addr=os.environ["SOURCE_ADDR"], token=os.environ["SOURCE_TOKEN"]) as bus:\n'
            '    agent = Agent(bus, configuration({"sources":[],"excludes":[]}), log=lambda _:None)\n'
            '    real = agent.subscriptions.kv.set\n'
            '    def committed(key, value, **options):\n'
            '        result = real(key, value, **options)\n'
            '        if key.startswith("sub.") and key != %r and options.get("mode") == "add":\n'
            '            print(key, flush=True); time.sleep(600)\n'
            '        return result\n'
            '    agent.subscriptions.kv.set = committed\n'
            '    agent.load()\n'
        ) % (str(HERE), old_key)
        child = subprocess.Popen([sys.executable, '-B', '-c', code], stdout=subprocess.PIPE, stderr=subprocess.PIPE,
             text=True, env={**os.environ, 'SOURCE_NAME':self.source.name, 'SOURCE_ADDR':self.source._transport.addr,
                             'SOURCE_TOKEN':self.source._transport.token})
        def close_child():
            if child.poll() is None: child.kill()
            child.wait(timeout=5); child.stdout.close(); child.stderr.close()
        self.addCleanup(close_child)
        self.assertTrue(select.select([child.stdout], [], [], 8)[0], 'migration never reached the committed-new-row barrier')
        new_key = child.stdout.readline().strip()
        self.assertEqual(new_key, 'sub.'+migration.migration_id(self.source.name, old_key))
        self.assertIn(old_key, [r['name'] for r in kv.list().get('json', [])], 'old row was removed before the new commit barrier')
        self.assertEqual(kv.get(old_key, kind='json'), legacy, 'old row was removed before the new commit barrier')
        self.assertEqual(kv.get(new_key, kind='json')['id'], new_key[4:])
        child.kill(); child.wait(timeout=5)
        self.agent.load()   # restart: finish the checkpoint before any serve/forward
        names = [r['name'] for r in kv.list().get('json', [])]
        self.assertNotIn(old_key, names)
        self.assertEqual([n for n in names if n.startswith('sub.')], [new_key])
        self.assertFalse(any(n.startswith(migration.PREFIX) for n in names))
        reply = self.command('sub list')
        self.assertTrue(reply.startswith('{'), reply)
        rows = json.loads(reply)['subs']; self.assertEqual(len(rows), 1)
        self.agent.load(); self.forward('after migration restart')
        self.expect_body('[fixture] after migration restart')
        self.assertIsNone(self.take(), 'both old and new subscriptions forwarded the match')
        self.assertEqual(self.counts(), (1, 0))


if __name__ == '__main__': unittest.main()
