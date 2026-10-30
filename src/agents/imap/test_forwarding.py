import copy
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock

import test_imap  # noqa: F401  (sets the import path)
from agent_bus.errors import Refusal
from forwarding import BLOCK, Forwarder, JOURNAL, PROGRESS, SCAN, shared
import wire

SELF = '#imap/account@t'


def raw(subject='invoice'):
    return ('From: sender@example.test\r\nSubject: ' + subject + '\r\n\r\nexternal mail').encode()


def deliver(bus):
    """The engine's own decision path, in process: author checks to target and to this agent, then one send."""
    return lambda row, body, deadline: shared.deliver_one(bus, SELF, row, body, 'imap', row['id'], deadline, {})


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
    def legacy(self):
        return {'identity': self.identity, 'validity': self.validity, 'cursor': 0, 'retry': None}
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
            return deliver(self.bus)(row, body, deadline)
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


def ledger(entries=None, skips=None, sent=4, discarded=1):
    """A pre-B.29 ledger in the shape its validator accepted."""
    if entries is None:
        entries = {'95': {'state': 'pending', 'bytes': 10, 'attempts': 0, 'due': 0, 'reason': None},
                   '96': {'state': 'retrying', 'bytes': 10, 'attempts': 1, 'due': 5, 'reason': None},
                   '97': {'state': 'skipped', 'bytes': 0, 'attempts': 0, 'due': 0, 'reason': 'unmatched'}}
    if skips is None:
        skips = {'fetch': {'count': 2, 'first': 91, 'last': 92}}
    counted = sum(counter['count'] for counter in skips.values())
    return {'version': 1, 'identity': 'account', 'validity': 7, 'start': 90, 'floor': 90, 'pause': None,
            'entries': entries, 'intake': None, 'intake_cleared': 0, 'last_reset_pause': None, 'sent': sent,
            'skipped': counted, 'skipped_first': min((c['first'] for c in skips.values()), default=None),
            'skipped_last': max((c['last'] for c in skips.values()), default=None), 'skips': skips,
            'discarded': discarded, 'resets': 0}


class MigrationTests(unittest.TestCase):
    """Rows of before B.29 move once onto the shared shape; nothing is sent twice."""
    def setUp(self):
        root = Path(__file__).resolve().parents[2] / 'tmp'
        root.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix='imap-migrate-', dir=root)
        self.addCleanup(self.temp.cleanup)
        self.clock, self.source, self.bus = Clock(), Source(), Bus()

    def legacy(self, sub_id, **change):
        value = {'id': sub_id, 'target': '#target@t', 'created_by': '#author@t',
                 'created_at': '2026-10-29T12:00:00.250000+00:00', 'filter': '/invoice/i', 'ledger': ledger()}
        value.update(change)
        self.bus.kv.data['sub.' + sub_id] = value

    def start(self):
        worker = Forwarder(self.bus, SELF, self.temp.name, self.source, self.clock, self.clock,
                           send_one=deliver(self.bus))
        self.addCleanup(worker.close)
        return worker

    def test_a_legacy_row_keeps_target_author_filter_start_and_counts(self):
        self.legacy('aabbccdd')
        self.bus.kv.data[SCAN] = {'uid': 95}
        self.start()
        self.assertNotIn('sub.aabbccdd', self.bus.kv.data)
        self.assertNotIn(SCAN, self.bus.kv.data)
        self.assertEqual(self.bus.kv.data['sub.aabbccdd0000'], {
            'id': 'aabbccdd0000', 'target': '#target@t', 'selector': {'validity': 7, 'from_uid': 90},
            'filter': '/invoice/i', 'created_by': '#author@t', 'created_at': '2026-10-29T12:00:00.250Z',
            'sent': 4, 'dropped': 5})   # one discarded, two counted skips, two held copies; unmatched is no loss

    def test_compacted_skips_count_as_dropped_and_unmatched_mail_does_not(self):
        self.legacy('aabbccdd', ledger=ledger(entries={}, skips={'fetch': {'count': 1, 'first': 95, 'last': 95}}, discarded=0))
        unmatched = {'96': {'state': 'skipped', 'bytes': 0, 'attempts': 0, 'due': 0, 'reason': 'unmatched'}}
        self.legacy('11223344', ledger=ledger(entries=unmatched, skips={}, discarded=0))
        self.start()
        self.assertIn('sub.aabbccdd0000', self.bus.kv.data)
        self.assertIn('sub.112233440000', self.bus.kv.data)
        self.assertEqual(self.bus.kv.data['sub.aabbccdd0000']['dropped'], 1)
        self.assertEqual(self.bus.kv.data['sub.112233440000']['dropped'], 0)

    def test_a_ledger_whose_skip_totals_disagree_is_kept_as_invalid_state(self):
        broken = ledger()
        broken['skipped'] += 1
        self.legacy('aabbccdd', ledger=broken)
        self.start()
        self.assertIn('sub.aabbccdd0000', self.bus.kv.data)
        self.assertIn('ledger', self.bus.kv.data['sub.aabbccdd0000'])   # kept raw, not converted
        self.assertEqual(self.bus.kv.data['sub.aabbccdd0000']['ledger'], broken)

    def test_a_stop_between_the_add_and_the_delete_serves_the_subscription_once(self):
        self.legacy('aabbccdd')
        self.bus.kv.data[PROGRESS] = {'version': 1, 'identity': 'account', 'validity': 7, 'cursor': 0, 'retry': None,
                                      'baseline': 90, 'observe': 100, 'history': 89, 'history_ceiling': 89}
        delete = self.bus.kv.delete
        def killed(key, *, kind):
            if key == 'sub.aabbccdd':
                raise Stop()
            return delete(key, kind=kind)
        self.bus.kv.delete = killed
        with self.assertRaises(Stop):
            self.start().close()
        self.assertIn('sub.aabbccdd', self.bus.kv.data)        # both rows are stored now
        self.assertIn('sub.aabbccdd0000', self.bus.kv.data)
        self.assertEqual(self.bus.kv.data[JOURNAL + 'aabbccdd'], {'to': 'sub.aabbccdd0000'})
        self.bus.kv.delete = delete
        worker = self.start()
        self.assertEqual(sorted(k for k in self.bus.kv.data if k.startswith('sub.')), ['sub.aabbccdd0000'])
        self.assertNotIn(JOURNAL + 'aabbccdd', self.bus.kv.data)
        self.source.arrive(100)
        worker.tick()
        self.assertEqual([(target, options['tag']) for target, _, options in self.bus.sent], [('#target@t', 'aabbccdd0000')])
        self.assertEqual(self.bus.kv.data['sub.aabbccdd0000']['sent'], 5)
        author = SimpleNamespace(envelope={'from': '#author@t', 'roles': [], 'original_to': SELF})
        self.assertEqual(worker.text(author).count('#target@t by=#author@t'), 1)

    def test_a_taken_new_id_moves_the_row_to_the_next_free_suffix(self):
        self.legacy('aabbccdd')
        other = {'id': 'aabbccdd0000', 'target': '#other@t', 'selector': {'validity': 7, 'from_uid': 100}, 'filter': '',
                 'created_by': '#other@t', 'created_at': '2026-10-30T00:00:00.000Z', 'sent': 0, 'dropped': 0}
        self.bus.kv.data['sub.aabbccdd0000'] = copy.deepcopy(other)
        self.start().close()
        self.assertEqual(self.bus.kv.data['sub.aabbccdd0000'], other)
        self.assertIn('sub.aabbccdd0001', self.bus.kv.data)
        self.assertEqual(self.bus.kv.data['sub.aabbccdd0001']['created_by'], '#author@t')
        self.assertNotIn('sub.aabbccdd', self.bus.kv.data)

    def test_an_identical_row_already_at_the_new_id_is_not_taken_for_the_moved_one(self):
        self.legacy('aabbccdd')
        twin = {'id': 'aabbccdd0000', 'target': '#target@t', 'selector': {'validity': 7, 'from_uid': 90},
                'filter': '/invoice/i', 'created_by': '#author@t', 'created_at': '2026-10-29T12:00:00.250Z',
                'sent': 4, 'dropped': 5}
        self.bus.kv.data['sub.aabbccdd0000'] = copy.deepcopy(twin)
        self.start()
        self.assertEqual(sorted(k for k in self.bus.kv.data if k.startswith('sub.')), ['sub.aabbccdd0000', 'sub.aabbccdd0001'])
        self.assertEqual(self.bus.kv.data['sub.aabbccdd0000'], twin)

    def test_a_stop_before_the_create_finishes_into_the_journalled_id(self):
        self.legacy('aabbccdd')
        set_ = self.bus.kv.set
        def killed(key, value, *, kind, mode):
            if key == 'sub.aabbccdd0000':
                raise Stop()
            return set_(key, value, kind=kind, mode=mode)
        self.bus.kv.set = killed
        with self.assertRaises(Stop):
            self.start().close()
        self.assertEqual(self.bus.kv.data[JOURNAL + 'aabbccdd'], {'to': 'sub.aabbccdd0000'})
        self.bus.kv.set = set_
        self.start()
        self.assertEqual(sorted(k for k in self.bus.kv.data if k.startswith('sub.')), ['sub.aabbccdd0000'])
        self.assertNotIn(JOURNAL + 'aabbccdd', self.bus.kv.data)

    def test_a_journal_left_after_the_old_row_went_is_removed(self):
        self.bus.kv.data[JOURNAL + 'aabbccdd'] = {'to': 'sub.aabbccdd0000'}
        self.start()
        self.assertNotIn(JOURNAL + 'aabbccdd', self.bus.kv.data)

    def test_a_damaged_legacy_row_is_kept_for_the_managers_to_delete(self):
        self.legacy('aabbccdd', ledger='broken')
        worker = self.start()
        self.assertEqual(self.bus.kv.data['sub.aabbccdd0000']['ledger'], 'broken')
        author = SimpleNamespace(envelope={'from': '#author@t', 'roles': [], 'original_to': SELF})
        owner = SimpleNamespace(envelope={'from': '#boss@t', 'roles': ['owner'], 'original_to': SELF})
        self.assertNotIn('aabbccdd0000', worker.text(author))
        self.assertIn('aabbccdd0000 invalid subscription state', worker.text(owner))
        with self.assertRaises(shared.SubscriptionError):
            worker.delete(author, 'aabbccdd0000')
        worker.delete(owner, 'aabbccdd0000')
        self.assertNotIn('sub.aabbccdd0000', self.bus.kv.data)

    def test_a_migrated_row_receives_new_mail_once(self):
        self.legacy('aabbccdd')
        self.bus.kv.data[PROGRESS] = {'version': 1, 'identity': 'account', 'validity': 7, 'cursor': 0, 'retry': None,
                                      'baseline': 90, 'observe': 100, 'history': 89, 'history_ceiling': 89}
        worker = self.start()
        self.source.arrive(100)
        worker.tick()
        self.assertEqual([options['tag'] for _, _, options in self.bus.sent], ['aabbccdd0000'])
        self.assertEqual(self.bus.kv.data['sub.aabbccdd0000']['sent'], 5)


if __name__ == '__main__':
    unittest.main()
