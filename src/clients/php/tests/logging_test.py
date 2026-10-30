"""Journal guard failures and a safe native-syslog surrogate for mutations."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]


class Logging(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(dir=ROOT / 'tmp', prefix='php-logging-')
        self.addCleanup(self.tmp.cleanup)
        self.work = Path(self.tmp.name)
        self.env = dict(os.environ, PATH=f'{self.work}:' + os.environ['PATH'],
                        PROBE=str(self.work))
        self.env.pop('SASL_PATH', None)
        self.script('journalctl', '''#!/bin/bash
if [ -f "$PROBE/read" ]; then
  [ "${JOURNAL_FAIL:-}" = after ] && exit 7
  [ ! -f "$PROBE/line" ] || cat "$PROBE/line"
else
  touch "$PROBE/read"
  [ "${JOURNAL_FAIL:-}" != before ] || exit 7
fi
exit 0
''')

    def script(self, name, content):
        path = self.work / name
        path.write_text(content)
        path.chmod(0o700)
        return path

    def run_guard(self, *command):
        return subprocess.run(['bash', str(HERE / 'journal-quiet.sh'), *command],
                              env=self.env, text=True, capture_output=True, timeout=15)

    def test_silence(self):
        p = self.run_guard('true')
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn('journal gained 0 lines', p.stdout)

    def test_counts_native_syslog(self):
        p = self.run_guard('bash', '-c', '''printf '{"SYSLOG_IDENTIFIER":"php","_CMDLINE":"php -d syslog.ident=%s","MESSAGE":"DIGEST-MD5 common mech free"}\n' "$AB_PHP_JOURNAL_ID" > "$PROBE/line"''')
        self.assertEqual(p.returncode, 3, p.stdout + p.stderr)
        self.assertIn('journal gained 1 lines', p.stderr)

    def test_external_php_is_not_counted(self):
        p = self.run_guard('bash', '-c', '''printf '%s\n' '{"SYSLOG_IDENTIFIER":"php","_CMDLINE":"php -d syslog.ident=another-shard","MESSAGE":"DIGEST-MD5 common mech free"}' > "$PROBE/line"''')
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn('journal gained 0 lines', p.stdout)

    def test_php_identifier_is_counted(self):
        p = self.run_guard('bash', '-c', '''printf '{"SYSLOG_IDENTIFIER":"%s","MESSAGE":"PHP error"}\n' "$AB_PHP_JOURNAL_ID" > "$PROBE/line"''')
        self.assertEqual(p.returncode, 3, p.stdout + p.stderr)
        self.assertIn('journal gained 1 lines', p.stderr)

    def test_unreadable_before(self):
        self.env['JOURNAL_FAIL'] = 'before'
        p = self.run_guard('touch', str(self.work / 'ran'))
        self.assertEqual(p.returncode, 2, p.stdout + p.stderr)
        self.assertFalse((self.work / 'ran').exists())

    def test_unreadable_after(self):
        self.env['JOURNAL_FAIL'] = 'after'
        p = self.run_guard('true')
        self.assertEqual(p.returncode, 2, p.stdout + p.stderr)

    def test_preserves_test_failure(self):
        self.assertEqual(self.run_guard('bash', '-c', 'exit 9').returncode, 9)

    def fake_php(self):
        # Execute the real runner, replacing only PHP. An unsafe launch emits
        # a fake journal entry, never a real syslog write, even in a mutant.
        self.script('php', '''#!/bin/bash
case " $* " in
  *" -d syslog.ident=$AB_PHP_JOURNAL_ID "*) ;;
  *) echo 'FAIL PHP process lacks its shard journal marker' >&2; exit 1 ;;
esac
if [ -z "${SASL_PATH:-}" ] || [ ! -d "$SASL_PATH" ] ||
   [ -n "$(ls -A "$SASL_PATH")" ]; then
  printf '{"_CMDLINE":"php -d syslog.ident=%s","MESSAGE":"DIGEST-MD5 common mech free"}\n' "$AB_PHP_JOURNAL_ID" > "$PROBE/line"
fi
case " $* " in
  *' -d log_errors=0 -d display_errors=stderr '*) ;;
  *) printf '{"_CMDLINE":"php -d syslog.ident=%s","MESSAGE":"PHP error logging enabled"}\n' "$AB_PHP_JOURNAL_ID" > "$PROBE/line" ;;
esac
exit 0
''')

    def test_runner_suppresses_native_syslog(self):
        self.fake_php()
        p = self.run_guard('bash', str(ROOT / 'src/clients/check.sh'), 'php')
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn('journal gained 0 lines', p.stdout)

    def test_example_suppresses_native_syslog(self):
        self.fake_php()
        self.script('java', '#!/bin/bash\nexit 0\n')
        self.script('sha256sum', '#!/bin/bash\ncat >/dev/null\nexit 0\n')
        jar = self.work / 'fake.jar'
        jar.write_text('compiler stub input')
        tokens = self.work / 'tokens'
        tokens.mkdir()
        for language in ['php', 'java']:
            (tokens / f'_stats-{language}_localhost').write_text('fixture-token')
        self.env.update(EXAMPLES_ROOT=str(ROOT), EXAMPLES_TOKENS=str(tokens),
                        JACKSON_CORE_JAR=str(jar))
        # Reproduce the gate's scheduling: a checksum reader may run before
        # its writer. The stub must consume stdin, as real sha256sum does.
        self.env['BASH_FUNC_echo%%'] = '() { sleep 0.02; builtin echo "$@"; }'
        p = self.run_guard('bash', str(ROOT / 'src/clients/examples/codestats/run.sh'))
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn('journal gained 0 lines', p.stdout)


if __name__ == '__main__':
    unittest.main()
