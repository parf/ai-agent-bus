"""Cron expressions: strict fields, Vixie's day rule, DST slots, next-slot search."""
from datetime import datetime, timedelta, timezone
from pathlib import Path
import sys
import unittest
from zoneinfo import ZoneInfo

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'shared'))
import cronexpr  # noqa: E402

NY = ZoneInfo('America/New_York')


def brute(spec, start, minutes):
    """Minute-by-minute evaluation, independent of next_slot's day/hour/minute walk."""
    return [start + timedelta(minutes=i) for i in range(1, minutes + 1) if spec.matches(start + timedelta(minutes=i))]


def ok(case, text):
    """Parse, turning a refusal into a test failure rather than an error."""
    try:
        return cronexpr.parse(text)
    except cronexpr.CronError as e:
        case.fail(f'{text!r} refused: {e}')


class Parse(unittest.TestCase):
    def test_valid_forms(self):
        cases = {
            '* * * * *': (60, 24, 31, 12, 7),
            '*/15 0-6/2 1,15 1-12 1-5': (4, 4, 2, 12, 5),
            '0 0 * * 7': (1, 1, 31, 12, 1),
            '5-7 1 1 1 5-7': (3, 1, 1, 1, 3),
        }
        for text, sizes in cases.items():
            spec = cronexpr.parse(text)
            self.assertEqual(tuple(map(len, (spec.minute, spec.hour, spec.dom, spec.month, spec.dow))), sizes, text)
        self.assertEqual(cronexpr.parse('0 0 * * 7').dow, {0})
        self.assertEqual(cronexpr.parse('0 0 * * 5-7').dow, {5, 6, 0})

    def test_refusals_name_the_field(self):
        cases = {
            '60 * * * *': 'field 1', '* 24 * * *': 'field 2', '* * 0 * *': 'field 3', '* * * 13 *': 'field 4',
            '* * * * 8': 'field 5', '5-1 * * * *': 'backwards', '*/0 * * * *': 'step 0', '1,,2 * * * *': 'field 1',
            '1_0 * * * *': 'field 1', '+5 * * * *': 'field 1', '٣ * * * *': 'field 1', '5/2 * * * *': 'step',
            '* * * *': 'five fields', '* * * * * *': 'five fields', '*  * * * *': 'five fields',
            ' * * * * *': 'five fields', '* * * * * ': 'five fields', '*\t* * * *': 'five fields',
            '@daily': 'five fields', '0 0 31 4 *': 'never', '0 0 30 2 *': 'never', '': 'five fields',
        }
        for text, words in cases.items():
            with self.assertRaises(cronexpr.CronError, msg=text) as caught:
                cronexpr.parse(text)
            self.assertIn(words, str(caught.exception), text)

    def test_positive_controls_for_the_never_rule(self):
        ok(self, '0 0 29 2 *')  # leap years only
        ok(self, '0 0 30 2 1')  # both restricted: OR, so every Monday in February
        ok(self, '0 0 31 * *')
        ok(self, '0 0 29 2 */7')  # a Sunday February 29: 2032, outside a seven-year sample


class Days(unittest.TestCase):
    def test_vixie_or_when_both_restricted(self):
        spec = cronexpr.parse('0 0 13 * 5')
        self.assertTrue(spec.matches(datetime(2026, 10, 13)))  # a Tuesday the 13th
        self.assertTrue(spec.matches(datetime(2026, 10, 16)))  # a Friday
        self.assertFalse(spec.matches(datetime(2026, 10, 15)))

    def test_a_star_step_counts_as_star(self):
        spec = cronexpr.parse('0 0 */2 * 1')  # dom starts with '*': AND
        self.assertTrue(spec.matches(datetime(2026, 10, 5)))   # Monday, odd day
        self.assertFalse(spec.matches(datetime(2026, 10, 7)))  # odd day, Wednesday
        self.assertFalse(spec.matches(datetime(2026, 10, 12)))  # Monday, even day

    def test_sunday_is_0_and_7(self):
        self.assertTrue(ok(self, '0 0 * * 7').matches(datetime(2026, 11, 1)))
        self.assertTrue(ok(self, '0 0 * * 0').matches(datetime(2026, 11, 1)))
        self.assertFalse(ok(self, '0 0 * * 7').matches(datetime(2026, 11, 2)))


class Slots(unittest.TestCase):
    def test_next_slot_agrees_with_brute_force_over_a_year(self):
        start = datetime(2026, 1, 1, 0, 0)
        for text in ('*/7 * * * *', '0 9 * * 1-5', '30 2 1,15 * *', '0 0 13 * 5', '0 0 */2 * 1', '15 */5 * 2-3 0'):
            spec = cronexpr.parse(text)
            hits = brute(spec, start, 366 * 1440 if '*/7' not in text else 20000)
            walked, at = [], start
            while len(walked) < len(hits):
                at = cronexpr.next_slot(spec, at, timezone.utc)
                walked.append(at)
            self.assertEqual(walked, hits, text)

    def test_february_29_within_eight_years(self):
        self.assertEqual(cronexpr.next_slot(ok(self, '0 0 29 2 *'), datetime(2025, 3, 1), timezone.utc),
                         datetime(2028, 2, 29))

    def test_spring_forward_minutes_do_not_exist(self):
        spec = cronexpr.parse('30 2 * * *')
        self.assertFalse(cronexpr.exists(datetime(2026, 3, 8, 2, 30), NY))
        self.assertEqual(cronexpr.next_slot(spec, datetime(2026, 3, 7, 3, 0), NY), datetime(2026, 3, 9, 2, 30))

    def test_fall_back_repeats_one_slot_key(self):
        first = datetime(2026, 11, 1, 5, 30, tzinfo=timezone.utc)   # 01:30 EDT
        second = datetime(2026, 11, 1, 6, 30, tzinfo=timezone.utc)  # 01:30 EST
        self.assertEqual(cronexpr.slot_key(cronexpr.wall(first, NY)), cronexpr.slot_key(cronexpr.wall(second, NY)))


if __name__ == '__main__':
    unittest.main()
