import copy
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import test_imap
from agent_bus.errors import Refusal
from forwarding import Forwarder, PROGRESS
from test_subscriptions import KV
import wire

SELF = '#imap/account@t'


def raw(subject='invoice'):
    return ('From: sender@example.test\r\nSubject: ' + subject + '\r\n\r\nexternal mail').encode()


class Clock:
    def __init__(self):
        self.value = 1000.0
    def __call__(self):
        return self.value
    def advance(self, seconds=60):
        self.value += seconds


class Source:
    def __init__(self):
        self.identity, self.validity, self.uidnext = 'account', 7, 100
        self.messages = {99: raw('backlog')}
        self.masks, self.bodies, self.header_reads = [], [], []
        self.cached, self.searches = {}, []
        self.snapshots, self.closed, self.refused = 0, 0, set()
    def snapshot(self):
        self.snapshots += 1
        return self.identity, self.validity, self.uidnext
    def legacy(self):
        return {'identity': self.identity, 'validity': self.validity, 'cursor': 0, 'retry': None}
    def close(self):
        self.closed += 1
    def search(self, start, end):
        self.searches.append((start, end))
        return [uid for uid in self.messages if start <= uid <= end]
    def headers(self, uid):
        if (self.validity, uid) not in self.cached:
            self.header_reads.append(uid)
        return self.messages[uid].split(b'\r\n\r\n')[0] + b'\r\n\r\n'
    def body(self, uid):
        key = self.validity, uid
        if key not in self.cached:
            self.bodies.append(uid)
            if uid in self.refused:
                raise wire.Refused('fixture unavailable')
            self.cached[key] = self.messages[uid]
        return self.cached[key]
    def arrive(self, uid, subject='invoice'):
        self.messages[uid] = raw(subject)
        self.uidnext = max(self.uidnext, uid + 1)


class Bus:
    def __init__(self):
        self.name, self.kv = SELF, KV()
        self.author_allowed, self.sent, self.tries = True, [], []
        self.fail_target = None
        self.may_send = Mock(side_effect=lambda *_: self.author_allowed)
    def record(self, name):
        return SimpleNamespace(kv=self.kv) if name == SELF else SimpleNamespace(lookup=lambda: {'name': name, 'kind': 'queue'})
    def send(self, target, body, **options):
        self.tries.append(target)
        if target == self.fail_target:
            raise Refusal(403, 'fixture denied')
        self.sent.append((target, body, options))
        return 'fixture-' + str(len(self.sent))


class ForwardingTests(unittest.TestCase):
    def setUp(self):
        root = Path(__file__).resolve().parents[2] / 'tmp'
        root.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix='imap-forward-', dir=root)
        self.addCleanup(self.temp.cleanup)
        self.clock, self.source, self.bus = Clock(), Source(), Bus()
        self.worker = self.start()
        self.addCleanup(lambda: self.worker.close())
        self.author = SimpleNamespace(envelope={'from': '#author@t', 'roles': [], 'original_to': SELF})
    def start(self):
        return Forwarder(self.bus, SELF, self.temp.name, self.source, self.clock, self.clock)
    def add(self, target='#target@t', pattern=''):
        return self.worker.add(self.author, target, pattern)
    def tick(self, seconds=60):
        self.clock.advance(seconds)
        self.worker.tick()

    def test_zero_subscriptions_performs_no_background_mailbox_fetch(self):
        self.worker.tick()
        self.assertEqual(self.source.snapshots, 0)
        self.assertEqual(self.source.bodies, [])
        self.assertEqual(self.source.header_reads, [])
        self.assertIn('manual cache only', self.worker.status())

    def test_only_new_mail_is_forwarded_once_even_after_restart(self):
        sub = self.add(pattern='/invoice/i')
        self.worker.tick()
        self.assertEqual(self.bus.sent, [])
        self.source.arrive(100)
        self.tick()
        self.assertEqual(len(self.bus.sent), 1)
        self.assertIn('created by #author@t.', self.bus.sent[0][1])
        self.assertEqual(self.source.bodies, [100])
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 101)
        self.worker.close()
        self.worker = self.start()
        self.tick()
        self.assertEqual(len(self.bus.sent), 1)
        self.assertEqual(self.source.bodies, [100])
        self.assertEqual(self.worker.subs.get(sub['id'])['ledger']['sent'], 1)

    def test_every_matching_subscription_gets_one_copy_but_body_fetched_once(self):
        self.add(pattern='invoice')
        self.add(pattern='/sender@example.test/')
        self.source.arrive(100)
        self.worker.tick()
        self.assertEqual(len(self.bus.sent), 2)
        self.assertEqual(self.source.bodies, [100])
        self.assertNotEqual(self.bus.sent[0][2]['tag'], self.bus.sent[1][2]['tag'])
        self.assertEqual(self.worker.bodies.listing(), {})

    def test_unmatched_mail_is_header_only_and_not_a_counted_loss(self):
        sub = self.add(pattern='^never$')
        self.source.arrive(100)
        self.worker.tick()
        self.assertEqual(self.source.header_reads, [100])
        self.assertEqual(self.source.bodies, [])
        self.assertEqual(self.bus.sent, [])
        self.assertEqual(self.worker.subs.get(sub['id'])['ledger']['skipped'], 0)

    def test_later_subscription_starts_at_its_own_uidnext(self):
        self.add('#first@t')
        self.source.arrive(100)
        self.add('#second@t')
        self.worker.tick()
        self.assertEqual([target for target, _, _ in self.bus.sent], ['#first@t'])
        self.source.arrive(101)
        self.tick()
        self.assertEqual(sorted(target for target, _, _ in self.bus.sent), ['#first@t', '#first@t', '#second@t'])

    def test_acl_paused_target_does_not_block_a_healthy_subscription(self):
        bad = self.add('#bad@t')
        self.add('#healthy@t')
        self.bus.fail_target = '#bad@t'
        self.source.arrive(100)
        self.worker.tick()
        self.assertEqual([target for target, _, _ in self.bus.sent], ['#healthy@t'])
        self.assertEqual(self.worker.subs.get(bad['id'])['ledger']['pause'], 'acl')
        before = self.bus.tries.count('#bad@t')
        self.source.arrive(101)
        self.tick()
        self.assertEqual(self.bus.tries.count('#bad@t'), before)
        self.assertEqual([target for target, _, _ in self.bus.sent], ['#healthy@t', '#healthy@t'])
        self.worker.delete(self.author, bad['id'])
        self.assertEqual(self.worker.bodies.listing(), {})

    def test_restart_between_sub_writes_preserves_body_and_never_duplicates(self):
        self.add('#first@t')
        self.add('#second@t')
        original = self.bus.kv.set
        pending = 0
        def interrupted(key, value, **options):
            nonlocal pending
            if key.startswith('sub.') and value['ledger']['entries'].get('100', {}).get('state') == 'pending':
                pending += 1
                if pending == 2:
                    raise Refusal(503, 'fixture stop between subscriptions')
            return original(key, value, **options)
        self.source.arrive(100)
        with patch.object(self.bus.kv, 'set', side_effect=interrupted):
            self.worker.tick()
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 100)
        self.assertEqual(self.bus.sent, [])
        self.worker.close()
        self.worker = self.start()
        self.tick()
        self.assertEqual(sorted(target for target, _, _ in self.bus.sent), ['#first@t', '#second@t'])
        self.assertEqual(self.source.bodies, [100])
        self.tick()
        self.assertEqual(len(self.bus.sent), 2)

    def test_stop_after_acceptance_and_before_outcome_commit_pauses_after_restart(self):
        sub = self.add()
        original = self.bus.kv.set
        def interrupted(key, value, **options):
            if key.startswith('sub.') and value['ledger']['entries'].get('100', {}).get('state') == 'sent':
                raise Refusal(503, 'fixture stop after accepted send')
            return original(key, value, **options)
        self.source.arrive(100)
        with patch.object(self.bus.kv, 'set', side_effect=interrupted):
            self.worker.tick()
        self.assertEqual(len(self.bus.sent), 1)
        self.worker.close()
        self.worker = self.start()
        self.tick()
        self.assertEqual(self.worker.subs.get(sub['id'])['ledger']['pause'], 'unconfirmed')
        self.assertEqual(len(self.bus.sent), 1)

    def test_matching_slice_defers_uid_without_advancing_or_losing_a_subscription(self):
        for n in range(3):
            self.add(f'#target{n}@t', 'invoice')
        self.source.arrive(100)
        def slow(*_):
            self.clock.advance(.11)
            return True
        with patch('forwarding.matches', side_effect=slow):
            self.worker.tick()
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 100)
        self.assertEqual(len(self.bus.sent), 2)
        self.tick(2)
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 101)
        self.assertEqual(len(self.bus.sent), 3)
        self.assertEqual(self.source.bodies, [100])

    def test_refused_fetch_has_a_restart_safe_cap_and_then_other_mail_progresses(self):
        sub = self.add()
        self.source.arrive(100)
        self.source.arrive(101)
        self.source.refused.add(100)
        for n in range(4):
            self.tick()
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 102)
        self.assertEqual(len(self.bus.sent), 1)
        self.assertEqual(self.worker.subs.get(sub['id'])['ledger']['skips']['fetch']['count'], 1)
        self.assertEqual(self.source.bodies.count(100), 4)
        self.tick()
        self.assertEqual(self.source.bodies.count(100), 4)

    def test_uidnext_regression_stays_blocked_until_explicit_delete_and_add(self):
        sub = self.add()
        self.source.arrive(100)
        self.worker.tick()
        self.source.uidnext = 99
        self.tick()
        self.assertIn('forwarding state invalid', self.worker.status())
        self.source.uidnext = 150
        self.tick()
        self.assertIn('forwarding state invalid', self.worker.status())
        self.worker.delete(self.author, sub['id'])
        fresh = self.add()
        self.assertEqual(fresh['ledger']['start'], 150)
        self.source.arrive(150)
        self.worker.tick()
        self.assertEqual(len(self.bus.sent), 2)

    def test_uidvalidity_reset_counts_held_loss_and_clears_old_pause(self):
        sub = self.add()
        self.bus.fail_target = '#target@t'
        self.source.arrive(100)
        self.worker.tick()
        self.source.validity, self.source.uidnext, self.source.messages = 8, 1, {}
        self.tick()
        value = self.worker.subs.get(sub['id'])['ledger']
        self.assertIsNone(value['pause'])
        self.assertEqual(value['discarded'], 1)
        self.assertEqual(value['last_reset_pause'], 'acl')
        self.assertEqual(value['start'], 1)
        self.assertEqual(self.worker.bodies.listing(), {})


if __name__ == '__main__':
    unittest.main()
