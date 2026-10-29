import copy
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock

import test_imap  # Sets the supported SDK path, without opening a connection.
from agent_bus.errors import Refusal, TransportError, UncertainOutcome
import delivery
import ledger
import outbox
from publication import WARNING, MAX_OUTPUT


class DeliveryTests(unittest.TestCase):
    def setUp(self):
        root = Path(__file__).resolve().parents[2] / 'tmp'
        root.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix='imap-delivery-', dir=root)
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.value = ledger.new('account', 7, 100)
        self.saved = copy.deepcopy(self.value)
        self.events = []

    def save(self, value):
        self.saved = copy.deepcopy(value)
        self.events.append('commit:' + value['entries']['100']['state'])

    def queue(self, store, raw=b'From: a@x.test\r\nSubject: hello\r\n\r\nmail'):
        return delivery.queue(self.value, 100, raw, store, '#author@t', 'aabbccdd', [], self.save)

    def execute(self, value, store, send):
        return delivery.execute(value, 100, store, '#author@t', 'aabbccdd', [], self.save, send, 1000)

    def test_body_precedes_ledger_and_marker_commit_precedes_one_send(self):
        with outbox.Outbox(self.work) as store:
            def save(value):
                self.assertEqual(store.get(outbox.filename('account', 7, 100)).split(b'\r\n\r\n')[1], b'mail')
                self.save(value)
            value = delivery.queue(self.value, 100, b'From: a@x.test\r\n\r\nmail', store,
                                   '#author@t', 'aabbccdd', [], save)
            def send(body):
                self.assertEqual(self.saved['entries']['100']['state'], 'sending')
                self.events.append('send')
                self.assertTrue(body.startswith(WARNING.rstrip('\n') + ' Subscription aabbccdd created by #author@t.\n'))
                return 'fixture-message-id'
            result = self.execute(value, store, send)
            self.assertEqual(self.events, ['commit:pending', 'commit:sending', 'send', 'commit:sent'])
            self.assertEqual(result['sent'], 1)

    def test_marker_commit_failure_never_sends(self):
        with outbox.Outbox(self.work) as store:
            value = self.queue(store)
            send = Mock(return_value='fixture')
            with self.assertRaises(OSError):
                delivery.execute(value, 100, store, '#author@t', 'aabbccdd', [],
                                 Mock(side_effect=OSError('fixture storage')), send, 1000)
            send.assert_not_called()
            self.assertEqual(self.saved['entries']['100']['state'], 'pending')

    def test_accepted_send_then_failed_outcome_commit_pauses_on_restart(self):
        with outbox.Outbox(self.work) as store:
            value = self.queue(store)
            def save(value):
                if value['entries']['100']['state'] == 'sent':
                    raise OSError('fixture stop after acceptance')
                self.save(value)
            send = Mock(return_value='fixture')
            with self.assertRaises(OSError):
                delivery.execute(value, 100, store, '#author@t', 'aabbccdd', [], save, send, 1000)
            send.assert_called_once()
            recovered = ledger.recover(self.saved)
            self.assertEqual(recovered['pause'], 'unconfirmed')
            self.assertEqual(ledger.ready(recovered, 100000), [])

    def test_acl_refusal_never_retries_but_temporary_refusal_does(self):
        for status, expected in ((401, 'acl'), (403, 'acl'), (404, None), (429, None), (503, 'unconfirmed')):
            with self.subTest(status=status), outbox.Outbox(self.work) as store:
                value = self.queue(store)
                send = Mock(side_effect=Refusal(status, 'fixture-private'))
                result = self.execute(value, store, send)
                send.assert_called_once()
                self.assertEqual(result['pause'], expected)
                if status == 429:
                    self.assertEqual(ledger.ready(ledger.recover(result), 1010), [100])
                else:
                    self.assertEqual(ledger.ready(ledger.recover(result), 100000), [])

    def test_uncertain_response_is_never_retried_and_preconnect_failure_is_retryable(self):
        for error, expected in ((UncertainOutcome('fixture-private'), 'unconfirmed'),
                                (TransportError('fixture-private'), None)):
            with outbox.Outbox(self.work) as store:
                value = self.queue(store)
                result = self.execute(value, store, Mock(side_effect=error))
                self.assertEqual(result['pause'], expected)

    def test_missing_referenced_body_is_counted_and_never_sent(self):
        with outbox.Outbox(self.work) as store:
            value = self.queue(store)
            store.sweep(set())
            send = Mock()
            result = self.execute(value, store, send)
            send.assert_not_called()
            self.assertEqual(result['skipped'], 1)
            self.assertEqual(result['entries']['100']['reason'], 'fetch')

    def test_global_capacity_skips_only_this_subscription_without_orphan(self):
        with outbox.Outbox(self.work) as store:
            store.put = Mock(side_effect=outbox.CapacityError('fixture capacity'))
            result = self.queue(store)
            self.assertEqual(result['intake'], 'backlog')
            self.assertEqual(result['skipped'], 1)
            self.assertEqual(ledger.retained(result), {})
            self.assertEqual(store.listing(), {})
            refreshed = ledger.refresh_intake(result)
            self.assertIsNone(refreshed['intake'])
            self.assertEqual(refreshed['intake_cleared'], 1)
            self.assertEqual(refreshed['skipped'], 1)

    def test_body_missing_after_a_definite_refusal_keeps_the_attempt_count(self):
        with outbox.Outbox(self.work) as store:
            value = self.queue(store)
            value = self.execute(value, store, Mock(side_effect=Refusal(429, 'full')))
            store.sweep(set())
            send = Mock()
            result = delivery.execute(value, 100, store, '#author@t', 'aabbccdd', [], self.save, send, 1010)
            send.assert_not_called()
            self.assertEqual(result['entries']['100']['reason'], 'fetch')
            self.assertEqual(result['entries']['100']['attempts'], 1)

    def test_size_includes_creator_line_and_failed_ledger_commit_leaves_only_orphan(self):
        with outbox.Outbox(self.work) as store:
            result = self.queue(store, b'x' * (MAX_OUTPUT - len(WARNING.encode())))
            self.assertEqual(result['skipped'], 1)
            self.assertEqual(store.listing(), {})
            with self.assertRaises(OSError):
                delivery.queue(self.value, 100, b'raw mail', store, '#author@t', 'aabbccdd', [],
                               Mock(side_effect=OSError('fixture stop')))
            self.assertEqual(len(store.listing()), 1)
        with outbox.Outbox(self.work) as store:
            self.assertEqual(store.sweep(set()), 1)

    def test_queue_replay_never_publishes_a_second_body_or_commits_again(self):
        with outbox.Outbox(self.work) as store:
            value = self.queue(store)
            save = Mock()
            store.put = Mock(side_effect=AssertionError('body published twice'))
            again = delivery.queue(value, 100, b'changed', store, '#author@t', 'aabbccdd', [], save)
            self.assertEqual(again, value)
            save.assert_not_called()


if __name__ == '__main__':
    unittest.main()
