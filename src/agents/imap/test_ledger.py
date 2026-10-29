import copy
import unittest
from unittest.mock import patch

import ledger
from progress import ProgressError, advance_observation, load


class LedgerTests(unittest.TestCase):
    def value(self):
        return ledger.new('account', 7, 100)

    def sending(self):
        return ledger.begin_attempt(ledger.record(self.value(), 100, 123), 100, 1000)

    def test_every_discovery_state_is_idempotent_including_skips(self):
        pending = ledger.record(self.value(), 100, 123)
        sending = ledger.begin_attempt(pending, 100, 1000)
        examples = [pending, sending, ledger.record(self.value(), 100, skip='oversize')]
        examples += [ledger.finish(sending, 100, result, 1000)
                     for result in ('accepted', 'refused', 'acl', 'unconfirmed')]
        for value in examples:
            with self.subTest(state=value['entries']['100']['state']):
                self.assertEqual(ledger.record(value, 100, 456), value)
                self.assertEqual(ledger.record(value, 100, skip='malformed'), value)

    def test_partial_two_subscription_discovery_replays_only_missing_work(self):
        progress, _ = load(None, 'account', 7, 100)
        first = ledger.record(self.value(), 100, 123)
        # Stop after first persisted sub but before the second or global O.
        restored_first = ledger.record(copy.deepcopy(first), 100, 123)
        restored_second = ledger.record(self.value(), 100, 123)
        self.assertEqual(restored_first, first)
        self.assertEqual(list(restored_second['entries']), ['100'])
        progress = advance_observation(progress, 100, {'first', 'second'}, {'first', 'second'})
        self.assertEqual(progress['observe'], 101)
        self.assertEqual(len(ledger.retained(restored_first)), 1)
        self.assertEqual(len(ledger.retained(restored_second)), 1)

    def test_sending_marker_is_required_and_sent_uid_never_becomes_ready(self):
        pending = ledger.record(self.value(), 100, 123)
        with self.assertRaises(ledger.LedgerError):
            ledger.finish(pending, 100, 'accepted', 1000)
        sending = ledger.begin_attempt(pending, 100, 1000)
        self.assertEqual((sending['entries']['100']['state'], sending['entries']['100']['attempts']), ('sending', 1))
        self.assertEqual(pending['entries']['100']['state'], 'pending')
        self.assertEqual(ledger.ready(sending, 9999), [])
        sent = ledger.finish(sending, 100, 'accepted', 1000)
        self.assertEqual(sent['sent'], 1)
        self.assertEqual(ledger.retained(sent), {})
        self.assertEqual(ledger.ready(ledger.recover(sent), 9999), [])
        with self.assertRaises(ledger.LedgerError):
            ledger.begin_attempt(sent, 100, 9999)
        with self.assertRaises(ledger.LedgerError):
            ledger.finish(sent, 100, 'accepted', 9999)
        self.assertEqual(sent['sent'], 1)

    def test_only_one_send_can_be_in_flight_per_subscription(self):
        value = ledger.record(ledger.record(self.value(), 100, 123), 101, 456)
        sending = ledger.begin_attempt(value, 100, 1000)
        self.assertEqual(ledger.ready(sending, 1000), [])
        with self.assertRaises(ledger.LedgerError):
            ledger.begin_attempt(sending, 101, 1000)
        sent = ledger.finish(sending, 100, 'accepted', 1000)
        self.assertEqual(ledger.ready(sent, 1000), [101])

    def test_definite_refusal_backoff_survives_restart_without_pause(self):
        retrying = ledger.finish(self.sending(), 100, 'refused', 1000)
        restored = ledger.recover(retrying)
        self.assertIsNone(restored['pause'])
        self.assertEqual(ledger.ready(restored, 1009), [])
        self.assertEqual(ledger.ready(restored, 1010), [100])
        sending = ledger.begin_attempt(restored, 100, 1010)
        self.assertEqual(sending['entries']['100']['attempts'], 2)
        retrying = ledger.finish(sending, 100, 'refused', 1010)
        self.assertEqual(retrying['entries']['100']['due'], 1030)
        for _ in range(4):
            due = retrying['entries']['100']['due']
            sending = ledger.begin_attempt(retrying, 100, due)
            retrying = ledger.finish(sending, 100, 'refused', due)
        self.assertEqual(retrying['entries']['100']['due'] - due, 300)

    def test_temporary_refusals_give_up_on_attempt_twelve_with_counted_loss(self):
        value = ledger.record(self.value(), 100, 123)
        now = 1000
        for attempt in range(1, 13):
            value = ledger.begin_attempt(value, 100, now)
            value = ledger.finish(value, 100, 'refused', now)
            if attempt < 12:
                self.assertEqual(value['entries']['100']['state'], 'retrying')
                now = value['entries']['100']['due']
        self.assertEqual(value['entries']['100']['state'], 'skipped')
        self.assertEqual(value['entries']['100']['reason'], 'refused')
        self.assertEqual((value['skipped'], value['skipped_first'], value['skipped_last']), (1, 100, 100))
        self.assertEqual(ledger.retained(value), {})
        self.assertEqual(ledger.ready(value, now + 86400), [])

    def test_missing_destination_is_a_counted_permanent_skip(self):
        value = ledger.finish(self.sending(), 100, 'missing', 1000)
        self.assertEqual(value['entries']['100']['state'], 'skipped')
        self.assertEqual(value['skipped'], 1)
        self.assertEqual(ledger.ready(value, 999999), [])
        self.assertIsNone(value['pause'])

    def test_unknown_or_recovered_sending_pauses_and_never_retries(self):
        sending = self.sending()
        for paused in (ledger.finish(sending, 100, 'unconfirmed', 1000), ledger.recover(sending)):
            self.assertEqual(paused['pause'], 'unconfirmed')
            self.assertEqual(paused['entries']['100']['state'], 'paused')
            self.assertEqual(ledger.ready(paused, 999999), [])
            self.assertEqual(ledger.retained(paused), {100: 123})
            self.assertEqual(ledger.recover(paused), paused)

    def test_acl_refusal_pauses_instead_of_retrying(self):
        paused = ledger.finish(self.sending(), 100, 'acl', 1000)
        self.assertEqual(paused['pause'], 'acl')
        self.assertEqual(paused['entries']['100']['reason'], 'acl')
        self.assertEqual(ledger.ready(ledger.recover(paused), 999999), [])
        with self.assertRaises(ledger.LedgerError):
            ledger.begin_attempt(paused, 100, 999999)

    def test_paused_subscription_counts_matching_skips_without_retaining_bodies(self):
        paused = ledger.finish(self.sending(), 100, 'unconfirmed', 1000)
        skipped = ledger.record(paused, 101, 5000)
        skipped = ledger.record(skipped, 102, 5000)
        self.assertEqual((skipped['skipped'], skipped['skipped_first'], skipped['skipped_last']), (2, 101, 102))
        self.assertEqual(ledger.retained(skipped), {100: 123})
        self.assertEqual(ledger.record(skipped, 101, 5000), skipped)
        unmatched = ledger.record(skipped, 103, skip='unmatched')
        self.assertEqual(unmatched['skipped'], 2)

    def test_one_full_subscription_does_not_mutate_healthy_subscription(self):
        full, healthy = self.value(), self.value()
        for uid in range(100, 164):
            full = ledger.record(full, uid, 1)
        self.assertEqual(len(ledger.retained(full)), 64)
        full = ledger.record(full, 164, 1)
        healthy = ledger.record(healthy, 164, 1)
        self.assertEqual(full['intake'], 'backlog')
        self.assertIsNone(full['pause'])
        self.assertEqual(full['skipped'], 1)
        self.assertEqual(full['entries']['164']['bytes'], 0)
        self.assertEqual(ledger.ready(healthy, 1000), [164])
        self.assertIsNone(healthy['pause'])

    def test_reference_byte_cap_counts_each_referencing_subscription(self):
        value = self.value()
        for uid in range(100, 108):
            value = ledger.record(value, uid, 2 * 1024 * 1024)
        self.assertEqual(sum(ledger.retained(value).values()), 16 * 1024 * 1024)
        refused = ledger.record(value, 108, 1)
        self.assertEqual(refused['intake'], 'backlog')
        self.assertEqual(refused['skipped'], 1)
        self.assertEqual(len(ledger.retained(refused)), 8)

    def test_compaction_is_after_global_observe_and_keeps_pending_references(self):
        value = ledger.record(self.value(), 100, 123)
        value = ledger.record(value, 101, skip='oversize')
        before = ledger.compact(value, 101)
        self.assertIn('101', before['entries'])
        after = ledger.compact(before, 102)
        self.assertNotIn('101', after['entries'])
        self.assertIn('100', after['entries'])
        self.assertEqual(ledger.record(after, 101, skip='oversize'), after)
        self.assertEqual(after['skipped'], 1)
        with self.assertRaises(ProgressError):
            ledger.compact(after, 101)
        later = ledger.new('account', 7, 200)
        self.assertEqual(ledger.compact(later, 150), later)

    def test_reset_counts_old_pending_and_skipped_losses_and_keeps_sub_not_pause(self):
        value = ledger.finish(self.sending(), 100, 'unconfirmed', 1000)
        value = ledger.record(value, 101, 123)
        reset = ledger.mailbox_reset(value, 'account', 8, 200)
        self.assertEqual((reset['discarded'], reset['resets']), (2, 1))
        self.assertEqual(reset['entries'], {})
        self.assertIsNone(reset['pause'])
        self.assertEqual(reset['last_reset_pause'], 'unconfirmed')
        self.assertEqual(reset['start'], 200)
        with self.assertRaises(ledger.LedgerError):
            ledger.mailbox_reset(value, 'account', 7, 200)

    def test_invalid_state_types_and_key_growth_fail_closed(self):
        value = self.sending()
        for field, bad in (('pause', {}), ('version', True), ('sent', -1), ('floor', 99)):
            with self.subTest(field=field), self.assertRaises(ProgressError):
                ledger.validate({**value, field: bad})
        for field, bad in (('state', {}), ('bytes', True), ('due', -1), ('attempts', 0), ('reason', [])):
            broken = copy.deepcopy(value)
            broken['entries']['100'][field] = bad
            with self.subTest(field=field), self.assertRaises(ProgressError):
                ledger.validate(broken)
        broken = {**value, 'entries': {'1' * 10000: value['entries']['100']}}
        with self.assertRaises(ledger.LedgerError):
            ledger.validate(broken)
        with self.assertRaises(ProgressError):
            ledger.begin_attempt(value, True, 1000)

    def test_value_byte_cap_is_checked_before_the_new_snapshot_can_commit(self):
        value = self.value()
        length = len(ledger.encoded(value))
        with patch.object(ledger, 'MAX_VALUE', length):
            self.assertEqual(ledger.validate(value), value)
        with patch.object(ledger, 'MAX_VALUE', length - 1):
            with self.assertRaises(ledger.LedgerError):
                ledger.validate(value)
        self.assertEqual(value['entries'], {})

    def test_backlog_intake_pause_still_drains_and_clears_when_empty(self):
        value = self.value()
        for uid in range(100, 164):
            value = ledger.record(value, uid, 1)
        value = ledger.record(value, 164, 1)
        self.assertEqual(value['intake'], 'backlog')
        for uid in range(100, 164):
            self.assertEqual(ledger.ready(value, 1000)[0], uid)
            value = ledger.begin_attempt(value, uid, 1000)
            value = ledger.finish(value, uid, 'accepted', 1000)
        self.assertIsNone(value['intake'])
        self.assertEqual(value['intake_cleared'], 1)
        self.assertEqual(value['sent'], 64)
        value = ledger.record(value, 165, 1)
        self.assertEqual(ledger.ready(value, 1000), [165])

    def test_regex_disable_only_blocks_new_intake(self):
        value = ledger.disable_filter(ledger.record(self.value(), 100, 123))
        self.assertEqual(value['intake'], 'regex')
        self.assertEqual(ledger.ready(value, 1000), [100])
        value = ledger.begin_attempt(value, 100, 1000)
        value = ledger.finish(value, 100, 'accepted', 1000)
        self.assertEqual(value['sent'], 1)
        self.assertEqual(value['intake'], 'regex')
        value = ledger.record(value, 101, 123)
        self.assertEqual(value['skipped'], 1)
        self.assertEqual(ledger.retained(value), {})

    def test_terminal_compaction_per_uid_never_wedges_a_held_backlog(self):
        value = self.value()
        for uid in range(100, 164):
            value = ledger.record(value, uid, 1)
        value = ledger.compact(value, 164)
        for uid in range(164, 364):
            value = ledger.record(value, uid, skip='oversize')
            value = ledger.compact(value, uid + 1)
            self.assertEqual(len(value['entries']), 64)
        self.assertEqual(value['skipped'], 200)
        self.assertEqual((value['skipped_first'], value['skipped_last']), (164, 363))

    def test_ready_uids_are_ordered_with_due_lower_retry_first(self):
        value = ledger.record(self.value(), 102, 1)
        value = ledger.record(value, 100, 1)
        value = ledger.record(value, 101, 1)
        self.assertEqual(ledger.ready(value, 1000), [100, 101, 102])
        value = ledger.begin_attempt(value, 100, 1000)
        value = ledger.finish(value, 100, 'refused', 1000)
        self.assertEqual(ledger.ready(value, 1009), [101, 102])
        self.assertEqual(ledger.ready(value, 1010), [100, 101, 102])


if __name__ == '__main__':
    unittest.main()
