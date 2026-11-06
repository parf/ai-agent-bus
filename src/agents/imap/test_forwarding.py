import copy
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock

import test_imap  # noqa: F401  (sets the import path)
from agent_bus.errors import Refusal, TransportError
from forwarding import BLOCK, Forwarder, PROGRESS, shared
import wire

SELF = '#imap/account@t'


def raw(subject='invoice'):
    return ('From: sender@example.test\r\nSubject: ' + subject + '\r\n\r\nexternal mail').encode()


def deliver(bus, creation_only=False):
    """The engine's own decision path, in process: author checks to target and to this agent, then one send."""
    return lambda row, body, deadline: shared.deliver_one(bus, SELF, row, body, 'imap', row['id'], deadline, {},
                                                          source_at_creation_only=creation_only)


class KV:
    """The bus KV as the shared engine uses it: add/replace/set modes and JSON field updates."""
    def __init__(self):
        self.data, self.writes, self.deletes = {}, [], []

    def get(self, key, *, kind):
        if key not in self.data:
            raise Refusal(404, 'not found')
        return copy.deepcopy(self.data[key])

    def set(self, key, value, *, kind, mode):
        if mode == 'add' and key in self.data:
            raise Refusal(412, 'exists')
        if mode == 'replace' and key not in self.data:
            raise Refusal(404, 'not found')
        self.data[key] = copy.deepcopy(value)
        self.writes.append(key)

    def json(self, key, *ops):
        for op in ops:
            self.data[key][op['key']] = op['value']
        self.writes.append(key)

    def delete(self, key, *, kind):
        self.deletes.append(key)
        return self.data.pop(key, None) is not None

    def list(self):
        return {'json': [{'name': key} for key in sorted(self.data)]}


class Clock:
    def __init__(self):
        self.value = 1000.0
    def __call__(self):
        return self.value
    def advance(self, seconds=60):
        self.value += seconds


class Stop(BaseException):
    """A kill between two steps: nothing after it runs."""


class Source:
    def __init__(self):
        self.identity, self.validity, self.uidnext = 'account', 7, 100
        self.messages = {99: raw('backlog')}
        self.masks, self.bodies, self.header_reads = [], [], []
        self.snapshots, self.closed, self.refused, self.refused_headers = 0, 0, set(), set()
    def snapshot(self):
        self.snapshots += 1
        return self.identity, self.validity, self.uidnext
    def close(self):
        self.closed += 1
    def search(self, start, end):
        return [uid for uid in self.messages if start <= uid <= end]
    def headers(self, uid):
        self.header_reads.append(uid)
        if uid in self.refused_headers:
            raise wire.Refused('fixture unavailable')
        return self.messages[uid].split(b'\r\n\r\n')[0] + b'\r\n\r\n'
    def body(self, uid):
        self.bodies.append(uid)
        if uid in self.refused:
            raise wire.Refused('fixture unavailable')
        return self.messages[uid]
    def arrive(self, uid, subject='invoice'):
        self.messages[uid] = raw(subject)
        self.uidnext = max(self.uidnext, uid + 1)


class Bus:
    def __init__(self):
        self.name, self.kv = SELF, KV()
        self.author_allowed, self.sent = True, []
        self.fail_target = None
        self.may_send = Mock(side_effect=lambda *_: self.author_allowed)
    def record(self, name):
        return SimpleNamespace(kv=self.kv) if name == SELF else SimpleNamespace(lookup=lambda: {'name': name, 'kind': 'queue'})
    def send(self, target, body, **options):
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
        self.stop_before_send = False
        self.worker = self.start()
        self.addCleanup(lambda: self.worker.close())
        self.author = self.message('#author@t')

    def message(self, who, roles=(), original_to=SELF):
        return SimpleNamespace(envelope={'from': who, 'roles': list(roles), 'original_to': original_to})

    def start(self):
        def send_one(row, body, deadline):
            if self.stop_before_send:
                raise Stop()
            # With the real Forwarder's own source rule, as its private send helper uses it.
            return deliver(self.bus, self.worker.subs.source_at_creation_only)(row, body, deadline)
        return Forwarder(self.bus, SELF, self.temp.name, self.source, self.clock, self.clock, send_one=send_one)

    def restart(self):
        self.worker.close()
        self.worker = self.start()

    def add(self, target='#target@t', pattern=''):
        return self.worker.add(self.author, target, pattern)

    def row(self, sub):
        self.assertIn('sub.' + sub['id'], self.bus.kv.data)
        return self.bus.kv.data['sub.' + sub['id']]

    def tick(self, seconds=60):
        self.clock.advance(seconds)
        self.worker.tick()

    def test_zero_subscriptions_performs_no_background_mailbox_fetch(self):
        self.worker.tick()
        self.assertEqual((self.source.snapshots, self.source.bodies, self.source.header_reads), (0, [], []))
        self.assertIn('manual cache only', self.worker.status())

    def test_only_new_mail_is_forwarded_once_even_after_restart(self):
        sub = self.add(pattern='/invoice/i')
        self.assertEqual(sub['selector'], {'validity': 7, 'from_uid': 100})
        self.worker.tick()
        self.assertEqual(self.bus.sent, [])   # UID 99 was there before the subscription
        self.source.arrive(100)
        self.tick()
        self.assertEqual(len(self.bus.sent), 1)
        target, body, options = self.bus.sent[0]
        self.assertEqual((target, options), ('#target@t', {'topic': 'imap', 'tag': sub['id']}))
        self.assertIn(f"Subscription {sub['id']} created by #author@t.", body)
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 101)
        self.restart()
        self.tick()
        self.assertEqual(len(self.bus.sent), 1)
        self.assertEqual(self.source.bodies, [100])
        self.assertEqual((self.row(sub)['sent'], self.row(sub)['dropped']), (1, 0))

    def test_every_matching_subscription_gets_its_own_copy_and_the_body_is_fetched_once(self):
        first, second = self.add(pattern='invoice'), self.add(pattern='/sender@example.test/')
        self.source.arrive(100)
        self.worker.tick()
        self.assertEqual(sorted(options['tag'] for _, _, options in self.bus.sent), sorted([first['id'], second['id']]))
        for _, body, options in self.bus.sent:
            self.assertIn(f"Subscription {options['tag']} created by", body)
        self.assertEqual(self.source.bodies, [100])

    def test_unmatched_mail_reads_only_its_header_and_counts_nothing(self):
        sub = self.add(pattern='^never$')
        self.source.arrive(100)
        self.worker.tick()
        self.assertEqual((self.bus.sent, self.source.bodies, self.source.header_reads), ([], [], [100]))
        self.assertEqual((self.row(sub)['sent'], self.row(sub)['dropped']), (0, 0))
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 101)

    def test_a_later_subscription_starts_at_its_own_uidnext(self):
        early = self.add()
        self.source.arrive(100)
        self.source.arrive(101)
        late = self.add()   # added while UIDs 100 and 101 are still undiscovered
        self.assertEqual(late['selector']['from_uid'], 102)
        self.worker.tick()
        self.assertEqual([options['tag'] for _, _, options in self.bus.sent], [early['id'], early['id']])
        self.source.arrive(102)
        self.tick()
        self.assertEqual(sorted(options['tag'] for _, _, options in self.bus.sent[2:]), sorted([early['id'], late['id']]))

    def test_a_refused_copy_is_dropped_counted_and_the_subscription_stays_active(self):
        denied, healthy = self.add('#denied@t'), self.add('#healthy@t')
        self.bus.fail_target = '#denied@t'
        self.source.arrive(100)
        self.worker.tick()
        self.assertEqual([target for target, _, _ in self.bus.sent], ['#healthy@t'])
        self.assertEqual((self.row(denied)['sent'], self.row(denied)['dropped']), (0, 1))
        self.assertEqual(self.worker.warnings, {'send_refused': 1})
        self.bus.fail_target = None
        self.source.arrive(101)
        self.tick()
        self.assertEqual(sorted(target for target, _, _ in self.bus.sent), ['#denied@t', '#healthy@t', '#healthy@t'])
        self.assertEqual((self.row(denied)['sent'], self.row(denied)['dropped']), (1, 1))
        self.assertEqual(self.row(healthy)['sent'], 2)

    def test_unavailable_authority_refuses_add_and_drops_only_that_copy(self):
        self.bus.may_send.side_effect = TransportError('down')
        with self.assertRaisesRegex(shared.SubscriptionError, 'admission_unavailable'):
            self.add()
        self.assertFalse(any(key.startswith('sub.') for key in self.bus.kv.data))
        self.bus.may_send.side_effect = lambda *_: True
        sub = self.add()
        self.bus.may_send.side_effect = TransportError('down')
        self.source.arrive(100)
        self.worker.tick()
        self.assertEqual(self.bus.sent, [])
        self.assertEqual(self.row(sub)['dropped'], 1)
        self.assertEqual(self.worker.warnings, {'admission_unavailable': 1})
        self.bus.may_send.side_effect = lambda *_: True
        self.source.arrive(101)
        self.tick()
        self.assertEqual(len(self.bus.sent), 1)
        self.assertEqual((self.row(sub)['sent'], self.row(sub)['dropped']), (1, 1))

    def test_the_source_is_checked_again_before_every_forward(self):
        """Only the author's reach to #imap is gone; the target still admits it (Q326, Q339)."""
        self.assertIs(self.worker.subs.source_at_creation_only, False)
        sub = self.add()
        self.bus.may_send.side_effect = lambda who, name: name != SELF
        self.source.arrive(100)
        self.worker.tick()
        self.assertEqual(self.bus.sent, [])
        self.assertEqual(self.row(sub)['dropped'], 1)
        self.assertEqual(self.worker.warnings, {'admission_refused': 1})

    def test_a_revoked_author_is_checked_before_every_send(self):
        sub = self.add()
        self.bus.author_allowed = False
        self.source.arrive(100)
        self.worker.tick()
        self.assertEqual(self.bus.sent, [])
        self.assertEqual(self.row(sub)['dropped'], 1)
        self.assertEqual(self.worker.warnings, {'admission_refused': 1})
        self.bus.author_allowed = True
        self.source.arrive(101)
        self.tick()
        self.assertEqual(len(self.bus.sent), 1)

    def test_a_stop_after_the_frontier_moves_never_sends_that_uid_again(self):
        self.add()
        self.source.arrive(100)
        self.stop_before_send = True
        with self.assertRaises(Stop):
            self.worker.tick()
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 101)
        self.stop_before_send = False
        self.restart()
        self.tick()
        self.assertEqual(self.bus.sent, [])
        self.source.arrive(101)
        self.tick()
        self.assertEqual(len(self.bus.sent), 1)

    def test_a_refused_header_fetch_is_tried_three_times_then_skipped(self):
        self.add()
        self.source.arrive(100)
        self.source.arrive(101)
        self.source.refused_headers.add(100)
        for _ in range(2):
            self.tick()
            self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 100)
            self.assertIn('mail fetch retry pending', self.worker.status())
        self.tick()
        self.assertEqual(self.source.header_reads.count(100), 3)
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 102)
        self.assertEqual(len(self.bus.sent), 1)   # UID 101 only

    def test_a_refused_body_fetch_drops_the_matched_copies(self):
        sub = self.add()
        self.source.arrive(100)
        self.source.refused.add(100)
        self.worker.tick()
        self.assertEqual(self.bus.sent, [])
        self.assertEqual(self.row(sub)['dropped'], 1)
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 101)

    def test_uidnext_regression_stays_blocked_until_delete_and_add(self):
        sub = self.add()
        self.source.arrive(100)
        self.worker.tick()
        self.source.uidnext = 50
        self.tick()
        self.assertEqual(self.bus.kv.data[BLOCK], {'reason': 'regressed'})
        self.assertIn('forwarding state invalid', self.worker.status())
        self.worker.delete(self.author, sub['id'])
        self.assertNotIn(PROGRESS, self.bus.kv.data)
        self.assertNotIn(BLOCK, self.bus.kv.data)
        self.add()
        self.source.arrive(50)
        self.tick()
        self.assertEqual(len(self.bus.sent), 2)

    def test_a_new_uidvalidity_rebaselines_and_its_mail_reaches_old_subscriptions(self):
        self.add()
        self.source.validity, self.source.uidnext, self.source.messages = 8, 10, {}
        self.tick()
        self.assertEqual(self.bus.kv.data[PROGRESS]['observe'], 10)
        self.source.arrive(10)
        self.tick()
        self.assertEqual(len(self.bus.sent), 1)

    def test_listing_shows_each_author_their_own_and_managers_all(self):
        mine = self.add()
        other = self.worker.add(self.message('#other@t'), '#target@t', '')
        listed = self.worker.text(self.author)
        self.assertIn(mine['id'], listed)
        self.assertNotIn(other['id'], listed)
        manager = self.worker.text(self.message('#boss@t', ['owner']))
        self.assertIn(mine['id'], manager)
        self.assertIn(other['id'], manager)
        forwarded = self.worker.text(self.message('#boss@t', ['owner'], '#elsewhere@t'))
        self.assertNotIn(other['id'], forwarded)
        self.assertIn(f"{mine['id']} #target@t by=#author@t", listed)
        self.assertIn('from_uid=100 sent=0 dropped=0', listed)

    def test_deleting_the_last_subscription_clears_forwarding_progress(self):
        sub = self.add()
        self.assertIn(PROGRESS, self.bus.kv.data)
        with self.assertRaises(shared.SubscriptionError):
            self.worker.delete(self.message('#other@t'), sub['id'])
        self.worker.delete(self.author, sub['id'])
        self.assertNotIn(PROGRESS, self.bus.kv.data)
        self.worker.tick()
        self.assertIn('manual cache only', self.worker.status())


class EngineTests(unittest.TestCase):
    def test_forwarding_runs_on_the_shared_engine_not_a_local_copy(self):
        # imap.py puts this directory first on sys.path, so a local subscriptions.py would shadow the engine.
        support = Path(test_imap.proxy.SUPPORT).resolve()
        self.assertEqual(Path(shared.__file__).resolve(), support / 'agents/shared/subscriptions.py')
        self.assertFalse((Path(__file__).resolve().parent / 'subscriptions.py').exists())


class UnsupportedStateTests(unittest.TestCase):
    def test_unsupported_state_is_unchanged_and_never_delivered(self):
        values = [
            {'sub.aabbccdd': {'id': 'aabbccdd', 'ledger': {}}},
            {'sub.aabbccdd0000': {'id': 'aabbccdd0000', 'ledger': {}}},
            {'imap.migrate.aabbccdd': {'to': 'sub.aabbccdd0000'}},
            {'imap.discovery': {'uid': 95}},
            {PROGRESS: {'identity': 'account', 'validity': 7, 'cursor': 0}},
        ]
        for data in values:
            with self.subTest(data=data), tempfile.TemporaryDirectory() as work:
                bus, source = Bus(), Source()
                bus.kv.data = copy.deepcopy(data)
                with self.assertRaises(ValueError):
                    Forwarder(bus, SELF, work, source, Clock(), Clock(), deliver(bus))
                self.assertEqual(bus.kv.data, data)
                self.assertEqual(bus.kv.writes, [])
                self.assertEqual(bus.kv.deletes, [])
                self.assertEqual(bus.sent, [])
                self.assertEqual(source.snapshots, 0)


if __name__ == '__main__': unittest.main()
