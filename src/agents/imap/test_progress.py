import copy
import unittest

from progress import (ProgressError, advance_observation, backfill_step, eligible,
                      fast_forward, load, renew_history, subscription_start, validate)


class ProgressTests(unittest.TestCase):
    def test_baseline_excludes_backlog_but_preserves_manual_history(self):
        value, reset = load(None, 'account', 7, 100)
        self.assertFalse(reset)
        self.assertEqual((value['baseline'], value['observe'], value['history']), (100, 100, 99))
        self.assertFalse(eligible(value, 100, 99))
        self.assertTrue(eligible(value, 100, 100))
        self.assertEqual(subscription_start(value, 123), 123)
        self.assertFalse(eligible(value, 123, 122))
        self.assertTrue(eligible(value, 123, 123))

    def test_old_state_preserves_retry_and_cursor_without_reinterpreting_delivery(self):
        old = {'identity': 'account', 'validity': 7, 'cursor': 8, 'retry': {'uid': 9, 'count': 3}}
        value, reset = load(old, 'account', 7, 100)
        self.assertFalse(reset)
        self.assertEqual((value['cursor'], value['retry']), (8, {'uid': 9, 'count': 3}))
        self.assertEqual(value['observe'], 100)
        self.assertEqual(value['history'], 99)
        value['retry']['count'] = 1
        self.assertEqual(old['retry']['count'], 3)
        no_retry = {key: val for key, val in old.items() if key != 'retry'}
        self.assertIsNone(load(no_retry, 'account', 7, 100)[0]['retry'])

    def test_restart_retains_observation_frontier_even_when_mail_arrives(self):
        value, _ = load(None, 'account', 7, 100)
        for uid in range(100, 105):
            value = advance_observation(value, uid, {'one', 'two'}, {'one', 'two'})
        restored, reset = load(value, 'account', 7, 200)
        self.assertFalse(reset)
        self.assertEqual(restored['observe'], 105)
        self.assertEqual(restored['baseline'], 100)

    def test_partial_discovery_cannot_advance_or_mutate_frontier(self):
        value, _ = load(None, 'account', 7, 100)
        before = copy.deepcopy(value)
        with self.assertRaises(ProgressError):
            advance_observation(value, 100, {'one', 'two'}, {'one'})
        self.assertEqual(value, before)
        result = advance_observation(value, 100, {'one', 'two'}, {'one', 'two'})
        self.assertEqual(result['observe'], 101)
        self.assertEqual(value, before)

    def test_observation_cannot_jump_over_an_unrecorded_uid(self):
        value, _ = load(None, 'account', 7, 100)
        with self.assertRaises(ProgressError):
            advance_observation(value, 102, {'one'}, {'one'})
        self.assertEqual(value['observe'], 100)
        result = advance_observation(value, 100, set(), set())
        self.assertEqual(result['observe'], 101)

    def test_regressed_uidnext_is_not_silently_rebaselined(self):
        value, _ = load(None, 'account', 7, 100)
        for uid in range(100, 105):
            value = advance_observation(value, uid, set(), set())
        before = copy.deepcopy(value)
        with self.assertRaises(ProgressError) as caught:
            load(value, 'account', 7, 104)
        self.assertEqual(caught.exception.code, 'regressed')
        self.assertEqual(value, before)
        self.assertEqual(load(value, 'account', 7, 105)[0], value)
        # Even when O is lower, an already-seen history ceiling cannot shrink.
        value['history_ceiling'] = 199
        with self.assertRaises(ProgressError):
            load(value, 'account', 7, 150)

    def test_first_subscription_fast_forward_preserves_history_and_held_work(self):
        value, _ = load(None, 'account', 7, 100)
        result = fast_forward(value, 200, [], [])
        self.assertEqual((result['observe'], result['baseline'], result['history']), (200, 100, 99))
        self.assertEqual(value['observe'], 100)
        for starts, pending in (([150], []), ([], [150])):
            with self.assertRaises(ProgressError):
                fast_forward(value, 200, starts, pending)
        self.assertEqual(fast_forward(result, 150, [], [])['observe'], 200)

    def test_validity_or_account_change_is_an_explicit_reset(self):
        old, _ = load(None, 'account', 7, 100)
        old['retry'] = {'uid': 9, 'count': 2}
        for identity, validity in (('account', 8), ('other-account', 7)):
            value, reset = load(old, identity, validity, 150)
            self.assertTrue(reset)
            self.assertEqual((value['observe'], value['baseline'], value['history']), (150, 150, 149))
            self.assertIsNone(value['retry'])

    def test_history_is_descending_and_independent_of_observation(self):
        value, _ = load(None, 'account', 7, 100)
        value['retry'] = {'uid': 90, 'count': 3}
        result = value
        for uid in range(99, 89, -1):
            result = backfill_step(result, uid)
        self.assertEqual((result['history'], result['observe']), (89, 100))
        self.assertIsNone(result['retry'])
        with self.assertRaises(ProgressError):
            renew_history(result, 200)
        for uid in range(89, 0, -1):
            result = backfill_step(result, uid)
        renewed = renew_history(result, 200)
        self.assertEqual((renewed['history'], renewed['observe']), (199, 100))

    def test_history_cannot_jump_over_unprocessed_uids(self):
        value, _ = load(None, 'account', 7, 100)
        with self.assertRaises(ProgressError):
            backfill_step(value, 90)
        self.assertEqual(value['history'], 99)

    def test_progress_never_accepts_boolean_bounds_unknown_keys_or_corrupt_retry(self):
        valid, _ = load(None, 'account', 7, 100)
        for field, bad in (('observe', 99), ('observe', True), ('history', 100),
                           ('validity', 0), ('version', True), ('retry', {'uid': 9, 'count': 4})):
            value = {**valid, field: bad}
            with self.subTest(field=field, value=bad), self.assertRaises(ProgressError):
                validate(value)
        with self.assertRaises(ProgressError):
            load({**valid, 'unknown': 1}, 'account', 7, 200)


if __name__ == '__main__':
    unittest.main()
