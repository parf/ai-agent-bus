import copy
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import test_imap
from agent_bus.errors import Refusal, TransportError, UnexpectedAnswer
from publication import forwarded, validate_id
from subscriptions import SubscriptionError, Subscriptions
import ledger

SELF = '#imap/account@t'


class KV:
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

    def delete(self, key, *, kind):
        self.deletes.append(key)
        return self.data.pop(key, None) is not None

    def list(self):
        return {'json': [{'name': key} for key in self.data]}


class SubscriptionTests(unittest.TestCase):
    def setUp(self):
        self.kv = KV()
        self.target = {'name': '#target@t', 'kind': 'queue'}
        self.bus = Mock()
        self.bus.record.side_effect = lambda name: SimpleNamespace(kv=self.kv) if name == SELF else SimpleNamespace(lookup=lambda: self.target)
        self.bus.may_send.return_value = True
        self.baseline = Mock(return_value=('account', 7, 100))
        self.cleanup = Mock()
        self.store = Subscriptions(self.bus, SELF, self.baseline, self.cleanup, clock=lambda: 1000)
        self.author = self.message('#author@t')

    def message(self, who, roles=(), original_to=None):
        return SimpleNamespace(envelope={'from': who, 'roles': roles, 'original_to': original_to})

    def test_add_checks_envelope_author_and_generates_provenance_safe_id(self):
        value = self.store.add(self.author, '#TARGET@t', '/invoice/i')
        self.bus.may_send.assert_called_once_with('#author@t', '#target@t')
        self.assertEqual(value['created_by'], '#author@t')
        self.assertEqual(value['ledger']['start'], 100)
        validate_id(value['id'])
        self.assertIn(f"created by #author@t.\n", forwarded(b'raw mail', value['created_by'], value['id'], []))
        self.assertEqual(self.kv.data['sub.' + value['id']], value)
        self.assertEqual(self.bus.record.call_count, 2)  # Own KV, one target lookup.

    def test_add_denial_never_stores_even_for_own_owner(self):
        self.bus.may_send.return_value = False
        for message in (self.author, self.message('owner@t', ('owner',))):
            with self.assertRaises(SubscriptionError):
                self.store.add(message, '#target@t')
        self.assertEqual(self.kv.writes, [])
        self.baseline.assert_not_called()

    def test_unavailable_old_or_unknown_admission_refuses_without_secret_error_text(self):
        for error in (TransportError('fixture-private'), UnexpectedAnswer('fixture-private'), Refusal(404, 'fixture-private')):
            self.bus.may_send.side_effect = error
            with self.assertRaises(SubscriptionError) as caught:
                self.store.add(self.author, '#target@t')
            self.assertNotIn('fixture-private', str(caught.exception))
        self.assertEqual(self.kv.writes, [])
        self.baseline.assert_not_called()

    def test_bad_target_creator_filter_and_kind_store_nothing(self):
        for message, target, pattern in ((self.author, '#target\nforge', ''),
                                         (self.message('bad creator'), '#target@t', ''),
                                         (self.author, '#target@t', '(?i)bad')):
            with self.assertRaises(ValueError):
                self.store.add(message, target, pattern)
        self.target['kind'] = 'pubsub'
        with self.assertRaises(SubscriptionError):
            self.store.add(self.author, '#target@t')
        self.assertEqual(self.kv.writes, [])

    def test_delete_creator_or_own_manager_not_a_stranger_or_forwarded_role(self):
        value = self.store.add(self.author, '#target@t')
        for message in (self.message('#stranger@t'), self.message('#stranger@t', ('owner',), '#other@t')):
            with self.assertRaises(SubscriptionError):
                self.store.delete(message, value['id'])
        self.assertEqual(self.kv.deletes, [])
        self.store.delete(self.author, value['id'])
        self.assertNotIn('sub.' + value['id'], self.kv.data)
        self.cleanup.assert_called_once()
        for role in ('owner', 'maintainer'):
            value = self.store.add(self.author, '#target@t')
            self.store.delete(self.message('manager@t', (role,)), value['id'])
            self.assertNotIn('sub.' + value['id'], self.kv.data)

    def test_corrupt_ledger_is_visible_and_only_an_own_manager_can_delete_it(self):
        value = self.store.add(self.author, '#target@t')
        self.kv.data['sub.' + value['id']]['ledger'] = {'broken': True}
        values, damaged = self.store.all()
        self.assertEqual(values, [])
        self.assertEqual(len(damaged), 1)
        with self.assertRaises(SubscriptionError):
            self.store.delete(self.author, value['id'])
        self.store.delete(self.message('manager@t', ('owner',)), value['id'])
        self.assertEqual(self.kv.data, {})

    def test_list_shows_delivery_pause_intake_and_loss_ranges(self):
        value = self.store.add(self.author, '#target@t')
        state = ledger.record(value['ledger'], 100, 123)
        state = ledger.deny(state, 100)
        value['ledger'] = ledger.record(state, 101, 123)
        self.store.save(value)
        text = self.store.text()
        for expected in ('by=#author@t', 'pause=acl', 'held=1', 'skipped=1', 'skipped_uids=101-101', 'delete+add discards held work'):
            self.assertIn(expected, text)

    def test_reloaded_invalid_regex_disables_intake_without_dropping_held_work(self):
        value = self.store.add(self.author, '#target@t')
        value['ledger'] = ledger.record(value['ledger'], 100, 123)
        value['filter'] = '(?i)bad'
        self.store.save(value)
        before = len(self.kv.writes)
        values, damaged = self.store.all()
        self.assertEqual(damaged, {})
        self.assertIsNone(values[0]['ledger']['intake'])
        self.assertIn(value['id'], self.store.needs_disable)
        self.assertEqual(len(self.kv.writes), before)
        self.assertIn('intake=regex', self.store.text())
        self.assertEqual(len(self.kv.writes), before)
        values, damaged = self.store.for_driver()
        self.assertEqual(values[0]['ledger']['intake'], 'regex')
        self.assertEqual(len(self.kv.writes), before + 1)
        self.store.for_driver()
        self.assertEqual(len(self.kv.writes), before + 1)
        self.assertEqual(ledger.ready(values[0]['ledger'], 1000), [100])

    def test_transport_failure_is_not_an_empty_subscription_store(self):
        self.kv.list = Mock(side_effect=TransportError('fixture-private'))
        with self.assertRaises(TransportError):
            self.store.all()

    def test_delete_reports_cleanup_pending_after_committed_deletion(self):
        value = self.store.add(self.author, '#target@t')
        self.cleanup.side_effect = OSError('fixture-private')
        self.assertFalse(self.store.delete(self.author, value['id']))
        self.assertNotIn('sub.' + value['id'], self.kv.data)
        self.assertTrue(self.store.cleanup_pending)

    def test_per_creator_cap_is_checked_before_admission_but_other_creator_can_add(self):
        for _ in range(20):
            self.store.add(self.author, '#target@t')
        checks = self.bus.may_send.call_count
        with self.assertRaisesRegex(SubscriptionError, '20 subscriptions per creator'):
            self.store.add(self.author, '#target@t')
        self.assertEqual(self.bus.may_send.call_count, checks)
        self.store.add(self.message('#other@t'), '#target@t')
        self.assertEqual(len(self.store.all()[0]), 21)

    def test_over_cap_store_can_be_repaired_by_own_manager_deletion(self):
        value = self.store.add(self.author, '#target@t')
        for number in range(101):
            sub = copy.deepcopy(value)
            sub['id'] = f'{number:08x}'
            self.kv.data['sub.' + sub['id']] = sub
        with self.assertRaises(SubscriptionError):
            self.store.all()
        self.assertTrue(self.store.delete(self.message('manager@t', ('owner',)), '00000000'))
        self.assertNotIn('sub.00000000', self.kv.data)

    def test_list_line_shape_is_stable_and_read_only(self):
        with patch('subscriptions.new_id', return_value='aabbccdd'):
            self.store.add(self.author, '#target@t')
        before = len(self.kv.writes)
        self.assertEqual(self.store.text(),
                         "aabbccdd #target@t by=#author@t at=1970-01-01T00:16:40+00:00 filter='' "
                         'sent=0 held=0 held_uids=none skipped=0 skipped_uids=none pause=none intake=none '
                         'intake_cleared=0 discarded=0 last_reset_pause=none')
        self.assertEqual(len(self.kv.writes), before)


if __name__ == '__main__':
    unittest.main()
