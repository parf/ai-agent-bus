#!/usr/bin/env python3
"""Exercise the actual shard wrappers with a real failing Bun assertion."""
from pathlib import Path
import os
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent


class BunOutput(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        scratch = ROOT.parent / 'tmp'
        scratch.mkdir(exist_ok=True)
        cls.temp = tempfile.TemporaryDirectory(dir=scratch, prefix='bun-log-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.fixture = Path(cls.temp.name) / 'failure.test.ts'
        cls.fixture.write_text('import {test,expect} from "bun:test";\n'
                               'test("deliberate diff",()=>{\n'
                               '  expect({answer:"actual-value"}).toEqual({answer:"expected-value"});\n'
                               '});\n')
        result = subprocess.run(['bun', 'test', '--no-color', str(cls.fixture)],
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                text=True, timeout=10)
        if result.returncode != 1:
            raise AssertionError(f'fixture did not fail an assertion: {result.stdout}')
        cls.output = result.stdout
        cls.log = Path(cls.temp.name) / 'bun.log'
        cls.log.write_text(cls.output)
        cls.source = (ROOT / 'smoke.sh').read_text()

    def wrapper(self, name, shared_only=False):
        match = re.search(r'^' + name + r'\(\) \{\n.*?^\}', self.source, re.M | re.S)
        self.assertIsNotNone(match, name)
        # Only the expensive command is replayed; the shard's actual output
        # filter, count handling and failure accounting run without a daemon.
        script = '''set -u
source ./smoke-bun.sh
normal() { return 0; }
level() { echo normal; }
sec() { :; }
ok_exit() { [ "$2" = 0 ] || fail=$((fail+1)); }
timeout() {
  if [ "$SHARED_ONLY" = 1 ] && [ "$1" = 240 ]; then printf " 1 pass\n 0 fail\n"; return 0; fi
  cat "$BUN_LOG"; return 1;
}
SELF="$PWD/smoke.sh"; D="$PWD"; pass=0; fail=0; skipped=0
''' + match.group() + '\n' + name + '\nprintf "SHARD_FAILURES=%s\\n" "$fail"\n'
        env = dict(os.environ, BUN_LOG=str(self.log), SHARED_ONLY=str(int(shared_only)))
        result = subprocess.run(['bash', '-c', script], cwd=ROOT, env=env,
                                text=True, stdout=subprocess.PIPE,
                                stderr=subprocess.STDOUT, timeout=10)
        self.assertEqual(result.returncode, 0, result.stdout)
        return result.stdout

    def test_failures_keep_the_diff_and_location_in_every_wrapper(self):
        self.assertIn('error: expect(received).toEqual(expected)', self.output)
        for wrapper in ('web_ts', 'agent_slack', 'agent_telegram'):
            with self.subTest(wrapper=wrapper):
                log = self.wrapper(wrapper)
                self.assertRegex(log, r'(?m)^  \+\s+"answer": "actual-value",$')
                self.assertRegex(log, r'(?m)^  -\s+"answer": "expected-value",$')
                self.assertRegex(log, r'failure\.test\.ts:3:\d+')
                self.assertIn('(fail) deliberate diff', log)
                self.assertRegex(log, r'SHARD_FAILURES=[1-9]')

    def test_shared_subscription_failure_keeps_its_own_diff(self):
        log = self.wrapper("agent_slack", shared_only=True)
        self.assertRegex(log, r'(?m)^  \+\s+"answer": "actual-value",$')
        self.assertRegex(log, r'(?m)^  -\s+"answer": "expected-value",$')
        self.assertRegex(log, r"failure\.test\.ts:3:\d+")
        self.assertIn("SHARD_FAILURES=1", log)

    def test_failure_output_is_not_truncated(self):
        self.log.write_text(self.output + '\n' + '\n'.join(f'context {i}' for i in range(100)) + '\nlast detail\n')
        try:
            self.assertIn('last detail', self.wrapper('agent_slack'))
        finally:
            self.log.write_text(self.output)

    def test_passing_output_stays_terse(self):
        result = subprocess.run(['bash', '-c', 'source ./smoke-bun.sh; bun_test_text 0'],
                                input='(pass) detail\n 7 pass\n 0 fail\nFILE_TIMING 0.1s a\n',
                                cwd=ROOT, text=True, capture_output=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('(pass)', result.stdout)
        self.assertIn('7 pass', result.stdout)
        self.assertIn('FILE_TIMING', result.stdout)


if __name__ == '__main__':
    unittest.main()
