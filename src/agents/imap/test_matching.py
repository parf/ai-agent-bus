import json
from pathlib import Path
import subprocess
import sys
import unittest

from matching import MatchError, RegexTimeout, compile_filter, match_text, matches


class MatchingTests(unittest.TestCase):
    def test_shared_chat_vectors(self):
        rows = json.loads(Path(__file__).with_name('filter-vectors.json').read_text())
        for row in rows:
            with self.subTest(filter=row['filter'], text=row['text']):
                self.assertEqual(matches(compile_filter(row['filter']), row['text']), row['match'])

    def test_filter_syntax_is_a_bounded_common_subset(self):
        for value in ('/x/g', '/x/y', '/x/ii', '/x', 'x' * 201, '(?i)x', '(?P<n>x)',
                      '(?<n>x)', '(?<=x)y', '(?=x)', '(?>x)', '(x)\\1', r'\bword',
                      '[é]', r'[\d]', 'a++', 'a+?', 'a{10001}', 'a{', 'x\n', '\\'):
            with self.subTest(value=value), self.assertRaises(MatchError):
                compile_filter(value)
        self.assertIsNotNone(compile_filter('x' * 200))
        self.assertIsNotNone(compile_filter('/x/imsu'))

    def test_catastrophic_filter_times_out_and_next_filter_still_matches(self):
        result = None
        try:
            result = subprocess.run([sys.executable, '-B', '-c', '''
from matching import compile_filter, matches, RegexTimeout
timed_out = False
try:
    matches(compile_filter('(a+)+$'), 'a' * 1000 + '!')
except RegexTimeout:
    timed_out = True
print(timed_out, matches(compile_filter('healthy'), 'healthy'))
'''], cwd=Path(__file__).parent, capture_output=True, text=True, timeout=2)
        except subprocess.TimeoutExpired:
            pass  # The owning assertion kills a removed-budget mutant cleanly.
        self.assertIsNotNone(result, 'one pathological filter stalled later matching')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), 'True True')

    def test_sender_subject_are_decoded_normalized_and_not_display_name(self):
        raw = (b'From: Pay invoice <Bob@EXAMPLE.COM>\r\n'
               b'Subject: =?utf-8?b?0J/RgNC40LLQtdGC?=\r\n\tthere\r\n\r\n')
        self.assertEqual(match_text(raw), 'Bob@example.com\nПривет there')
        self.assertFalse(matches(compile_filter('Pay invoice'), match_text(raw)))
        self.assertTrue(matches(compile_filter('/привет/i'), match_text(raw)))

    def test_ambiguous_or_malformed_headers_never_become_match_all(self):
        for raw in (b'Subject: hi\r\n\r\n', b'From: a@x.test,b@x.test\r\n\r\n',
                    b'From: a@x.test\r\nFrom: b@x.test\r\n\r\n',
                    b'From: invalid\r\n\r\n', b'From: a@x.test\r\nSubject: a\r\nSubject: b\r\n\r\n',
                    b'x' * 8193):
            with self.subTest(raw=raw[:80]), self.assertRaises(MatchError):
                match_text(raw)

    def test_matching_input_is_bounded_at_a_utf8_boundary(self):
        raw = ('From: a@x.test\r\nSubject: ' + 'я' * 2000 + '\r\n\r\n').encode()
        text = match_text(raw)
        self.assertLessEqual(len(text.encode()), 1536)
        self.assertNotIn('\ufffd', text)
        with self.assertRaises(MatchError):
            matches(compile_filter(''), 'x' * 1537)


if __name__ == '__main__':
    unittest.main()
