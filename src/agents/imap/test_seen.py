"""Synthetic TLS Seen flags; no host account or mail contents."""
import contextlib
import io
import os
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock

import test_imap as fixture
from test_imap import RAW, proxy
import wire


class SeenTests(unittest.TestCase):
    setUpClass = classmethod(fixture.IMAPTests.setUpClass.__func__)
    setUp = fixture.IMAPTests.setUp
    call = fixture.IMAPTests.call

    def test_live_flags_change_between_list_and_search_without_body_refetch(self):
        self.server.messages = {1: RAW, 2: RAW, 3: RAW}
        self.assertIn('fetched 3', self.call('sync'))
        self.server.flags[3] = {b'\\Seen', b'\\Answered', b'$Fixture'}
        self.assertIn('UID 3 (cached; read)', self.call('list'))
        self.assertIn('UID 2 (cached; unread)', self.call('list'))
        self.server.flags[3].remove(b'\\Seen')
        self.server.flags[2] = {b'\\Seen'}
        self.assertIn('UID 3 (cached; unread)', self.call('search report'))
        self.assertIn('UID 2 (cached; read)', self.call('search report'))
        self.assertEqual(self.server.stores, [])
        self.assertEqual(len([x for x in self.server.fetches if b'BODY.PEEK[]' in x[1]]), 3)
        self.assertTrue(all(b' SELECT ' not in c for c in self.server.commands))

    def test_unread_filter_precedes_limit_and_omits_expunged_uid(self):
        self.server.messages = {1: RAW, 2: RAW, 3: RAW}
        self.call('sync')
        self.server.flags[3] = {b'\\Seen'}
        self.server.messages.pop(2)
        answer = self.call('list 1 --unread')
        self.assertIn('UID 1 (cached; unread)', answer)
        self.assertNotIn('UID 2', answer)
        self.assertNotIn('UID 3', answer)
        self.assertEqual(self.server.stores, [])

    def test_default_read_cached_read_and_mark_touch_only_seen(self):
        self.server.flags[7] = {b'\\Answered', b'$Fixture'}
        self.assertIn('; observed read', self.call('read 7'))
        self.assertEqual(self.server.flags[7], {b'\\Seen', b'\\Answered', b'$Fixture'})
        self.assertEqual(len(self.server.stores), 1)
        fetched = len(self.server.fetches)
        self.server.flags[7].discard(b'\\Seen')  # Another client changes the cached UID.
        self.assertIn('; observed read', self.call('read 7'))
        self.assertEqual(len(self.server.stores), 2)
        self.assertIn('; observed read', self.call('read 7'))
        self.assertEqual(len(self.server.stores), 2)
        self.assertEqual(len(self.server.fetches), fetched)
        self.assertIn('observed unread', self.call('mark 7 unread'))
        self.assertEqual(self.server.flags[7], {b'\\Answered', b'$Fixture'})
        self.assertIn('observed read', self.call('mark 7 read'))
        self.assertEqual(self.server.stores, [(7, b'+FLAGS.SILENT (\\Seen)'), (7, b'+FLAGS.SILENT (\\Seen)'), (7, b'-FLAGS.SILENT (\\Seen)'), (7, b'+FLAGS.SILENT (\\Seen)')])
        self.assertTrue(all(b'BODY.PEEK' in args for _, args in self.server.fetches if b'BODY' in args))
        self.assertFalse(any(any(word in c for word in (b' EXPUNGE', b' CLOSE', b' COPY', b' MOVE', b' APPEND', b'\\Deleted')) for c in self.server.commands))

    def test_no_mark_and_publish_remain_readonly_and_peek(self):
        self.assertIn('not marked', self.call('read 7 --no-mark'))
        self.assertNotIn(b'\\Seen', self.server.flags.get(7, set()))
        self.assertIn('published', self.call('publish 7'))
        self.assertEqual(self.server.stores, [])
        self.assertTrue(all(b' SELECT ' not in c for c in self.server.commands))

    def test_read_refuses_cached_epoch_change_before_any_store(self):
        self.call('read 7 --no-mark')
        self.server.validity += 1
        self.assertIn('epoch changed', self.call('read 7'))
        self.assertIn('epoch changed', self.call('mark 7 read'))
        self.assertEqual(self.server.stores, [])

    def test_unknown_cached_epoch_refuses_marking_but_no_mark_can_refresh(self):
        self.call('read 7 --no-mark')
        state = Path(self.work.name) / 'mail-cache/.state'
        state.write_bytes(b'{damaged fixture state')
        self.assertIn('epoch changed or is unknown', self.call('read 7'))
        self.assertEqual(self.server.stores, [])
        self.assertIn('not marked', self.call('read 7 --no-mark'))
        self.assertNotIn(b'\\Seen', self.server.flags.get(7, set()))

    def test_no_permanent_seen_or_writable_selection_refuses(self):
        for permanent in (None, [], [b'\\Answered']):
            self.server.permanent = permanent
            self.assertIn('cannot mark', self.call('read 7'))
            self.assertEqual(self.server.stores, [])
        self.server.permanent = [b'\\*']
        self.assertIn('observed read', self.call('read 7'))
        self.server.select_refused = True
        self.assertTrue(self.call('mark 7 unread').startswith('error:'))
        self.assertEqual(len(self.server.stores), 1)

    def test_store_refusal_is_definite_disconnect_unknown_and_external_change_observed(self):
        for refusal in (True, 'BAD'):
            self.server.store_refused = refusal
            answer = self.call('read 7')
            self.assertIn('flag unchanged', answer)
            self.assertNotIn('unknown', answer)
        self.server.store_refused = False
        self.server.drop_after_store = True
        self.assertIn('outcome unknown', self.call('read 7'))
        self.assertEqual(len(self.server.stores), 1)
        self.server.drop_after_store = False
        self.server.flags[7] = set()
        self.server.external_after_store = True
        self.assertIn('observed unread', self.call('read 7'))
        self.assertEqual(len(self.server.stores), 2)

    def test_flags_batches_and_malformed_store_answer_have_bounded_safe_outcomes(self):
        connection = Mock()
        def flags(_verb, numbers, _fields):
            return 'OK', [(b'1 (UID ' + n.encode() + b' FLAGS ())') for n in numbers.split(',')]
        connection.uid.side_effect = flags
        result = wire.seen(connection, list(range(1, 1001)))
        self.assertEqual(len(result), 1000)
        self.assertEqual(connection.uid.call_count, 10)
        self.assertTrue(all(len(call.args[1].split(',')) <= 100 for call in connection.uid.call_args_list))
        connection = Mock(seen_writable=True)
        connection.uid.side_effect = [('OK', [b'1 (UID 7 FLAGS ())']), ('BROKEN', [])]
        caught = None
        try:
            wire.mark_seen(connection, 7, True)
        except Exception as error:
            caught = error
        self.assertIsInstance(caught, wire.FlagOutcomeUnknown)
        self.assertEqual(connection.uid.call_count, 2)

    def test_cache_repairs_over_count_before_live_flags_query(self):
        self.server.messages = {n: RAW for n in range(1, 1002)}
        self.call('sync 1')
        directory = Path(self.work.name) / 'mail-cache'
        for number in range(1, 1001):
            path = directory / f'{number}.eml'
            path.write_bytes(RAW)
            os.utime(path, (1, 1))
        answer = self.call('list 1')
        self.assertIn('UID 1001 (cached; unread)', answer)
        self.assertEqual(len(list(directory.glob('*.eml'))), 1000)
        self.assertEqual(len(self.server.flag_fetches), 10)
        self.assertTrue(all(len(batch) <= 100 for batch in self.server.flag_fetches))

    def test_deadline_gap_after_store_is_unknown_and_never_retried(self):
        class Interrupted(tuple):
            def __len__(self):
                raise TimeoutError('fixture alarm after STORE response')
        connection = Mock(seen_writable=True)
        connection.uid.side_effect = [('OK', [b'1 (UID 7 FLAGS ())']), Interrupted(('OK', []))]
        caught = None
        try:
            wire.mark_seen(connection, 7, True)
        except Exception as error:
            caught = error
        self.assertIsInstance(caught, wire.FlagOutcomeUnknown)
        self.assertEqual(connection.uid.call_count, 2)
        self.assertFalse(connection.seen_store_active)

    def test_strict_call_shapes_and_flags_bounds_before_io(self):
        self.assertEqual(proxy.parse('read 7 --no-mark'), {'method': 'read', 'uid': 7, 'mark_read': False})
        self.assertEqual(proxy.parse('mark 7 unread'), {'method': 'mark', 'uid': 7, 'state': 'unread'})
        self.assertEqual(proxy.parse('list --unread 2'), {'method': 'list', 'unread': True, 'limit': 2})
        for body in ('read 07', 'mark +7 read', 'mark 7 deleted', 'read 7 extra', 'list --unread --unread', '{"method":"read","uid":7,"mark_read":1}', '{"method":"list","unread":"true"}'):
            with self.subTest(body=body), self.assertRaises(ValueError):
                proxy.parse(body)
        connection = Mock()
        with self.assertRaises(wire.WireError):
            wire.seen(connection, list(range(1, 1002)))
        connection.uid.assert_not_called()
        connection.uid.return_value = ('OK', [b'1 (UID 999 FLAGS (\\Seen))', b'2 (UID 7 FLAGS ())'])
        self.assertEqual(wire.seen(connection, [7]), {7: False})
        for response in (b'1 (FLAGS ())', b'1 (UID 7 FLAGS broken)', b'1 (UID 7 FLAGS (bad\x00atom))'):
            connection.uid.return_value = ('OK', [response])
            with self.subTest(response=response), self.assertRaises(ValueError):
                wire.seen(connection, [7])


if __name__ == '__main__':
    unittest.main()
