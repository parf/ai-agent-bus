import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import threading
import unittest
import unittest.mock

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import fixtures
from fixtures import SENTINEL

spec = importlib.util.spec_from_file_location('db', HERE / 'db.py')
proxy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(proxy)

MAKERS = {'mysql': fixtures.mariadb, 'postgres': fixtures.postgres, 'clickhouse': fixtures.clickhouse}
CONFIG = {}


def need(*names):
    """Start the fixtures a class uses, once, together. A missing fixture fails the run, never skips it."""
    todo = [n for n in names if n not in CONFIG]
    failures = []

    def start(name):
        try:
            CONFIG[name] = MAKERS[name]()
        except BaseException as error:
            failures.append(f'{name}: {error!r}')
    threads = [threading.Thread(target=start, args=(n,)) for n in todo]
    [t.start() for t in threads]
    [t.join() for t in threads]
    if failures:
        raise RuntimeError('fixtures failed: ' + '; '.join(failures))


def tearDownModule():
    fixtures._cleanup()


def call(config, sql, roles=(), account='app', extra=None, deadline=None):
    """One call as the runner makes it: roles and the account in the environment."""
    env = {f'AB_ROLE_{r.upper()}': '1' for r in roles}
    env.update({'DB_USER': account, 'DB_PASSWORD': SENTINEL})
    env.update(extra or {})
    log = io.StringIO()
    with contextlib.redirect_stderr(log):
        out, code = proxy.answer({'body': sql, 'deadline': deadline}, config, env)
    return out, log.getvalue()


def rows(out):
    return json.loads(out)['rows']


class Roles(unittest.TestCase):
    def test_mapping(self):
        t = proxy.role_tier
        self.assertEqual(t([]), 'read')
        self.assertEqual(t(['read']), 'read')
        self.assertEqual(t(['nonsense']), 'read')
        self.assertEqual(t(['Admin', 'WRITE']), 'read')            # case is part of the name
        self.assertEqual(t(['write']), 'write')
        self.assertEqual(t(['write', 'read', 'nonsense']), 'write')  # highest wins
        for admin in ('owner', 'maintainer', 'admin'):
            self.assertEqual(t([admin]), 'admin')
            self.assertEqual(t(['read', 'write', admin]), 'admin')
        self.assertEqual(t([None, 5, ['admin']]), 'read')            # malformed never reaches admin

    def test_roles_come_from_the_runner_environment_only(self):
        self.assertEqual(proxy.delivered_roles({'AB_ROLE_ADMIN': '1', 'AB_ROLE_WRITE': '0', 'ROLE_OWNER': '1'}), {'admin'})

    def test_per_tier_accounts_never_fall_up(self):
        env = {'DB_READ_USER': 'r', 'DB_READ_PASSWORD': 'p', 'DB_ADMIN_USER': 'a', 'DB_ADMIN_PASSWORD': 'q', 'DB_USER': 'all'}
        self.assertEqual(proxy.tier_account(env, 'read'), ('r', 'p'))
        self.assertEqual(proxy.tier_account(env, 'admin'), ('a', 'q'))
        with self.assertRaises(proxy.ProxyError):
            proxy.tier_account(env, 'write')       # no write account: refused, not served by admin or DB_USER
        self.assertEqual(proxy.tier_account({'DB_USER': 'all', 'DB_PASSWORD': 'x'}, 'write'), ('all', 'x'))
        with self.assertRaises(proxy.ProxyError):
            proxy.tier_account({}, 'read')


class Engines:
    sneaky = None
    """Every engine, the same promises. Subclasses give the engine's own statements."""
    name = ''
    # statement for a table-changing attempt, and a way to count rows afterwards
    count = 'SELECT count(*) FROM t'
    attempts = {}          # label -> statement a read caller must not get through
    smuggles = {}          # label -> a SELECT followed by something that writes

    @classmethod
    def setUpClass(cls):
        need(cls.name)

    @property
    def config(self):
        return CONFIG[self.name]

    def total(self):
        return rows(call(self.config, self.count, ['admin'])[0])[0][0]

    def test_read_selects(self):
        out, log = call(self.config, 'SELECT id, v FROM t ORDER BY id')
        self.assertEqual([[int(r[0]), r[1]] for r in rows(out)], [[1, 'one'], [2, 'two']], out)
        self.assertEqual(json.loads(out)['columns'], ['id', 'v'])
        self.assertEqual(set(json.loads(log)), {'engine', 'tier', 'request_bytes', 'status'})
        self.assertNotIn('SELECT', log)

    def test_read_cannot_write_and_the_same_account_can_as_admin(self):
        before = self.total()
        for label, (statement, undo) in self.attempts.items():
            out, _ = call(self.config, statement)                      # no role: read
            self.assertTrue(out.startswith('error:'), (label, out))
            self.assertEqual(self.total(), before, label)
            self.assertFalse(self.exists(label), label)
            out, _ = call(self.config, statement, ['admin'])             # positive control: it was a valid statement
            self.assertFalse(out.startswith('error:'), (label, out))
            call(self.config, undo, ['admin'])
        self.assertEqual(self.total(), before)

    def exists(self, label):
        return False

    def test_read_smuggle_fails_and_changes_nothing(self):
        before = self.total()
        for label, statement in self.smuggles.items():
            out, _ = call(self.config, statement)
            self.assertTrue(out.startswith('error:'), (label, out))
            self.assertEqual(self.total(), before, label)

    def test_write_function_inside_a_select_fails(self):
        if self.sneaky is None:
            return   # ClickHouse has no user function that writes; INSERT ... SELECT is covered by the attempts
        before = self.total()
        out, _ = call(self.config, self.sneaky)
        self.assertTrue(out.startswith('error:'), out)
        self.assertEqual(self.total(), before)
        out, _ = call(self.config, self.sneaky, ['admin'])           # positive control
        self.assertFalse(out.startswith('error:'), out)
        call(self.config, 'DELETE FROM t WHERE id = 99', ['admin'])

    def test_row_cap_is_applied_by_the_engine_path(self):
        out, _ = call({**self.config, 'max_rows': 1}, 'SELECT id FROM t ORDER BY id')
        self.assertEqual(len(rows(out)), 1, out)
        self.assertTrue(json.loads(out)['truncated'])
        out, _ = call({**self.config, 'max_rows': 5}, 'SELECT id FROM t ORDER BY id')
        self.assertFalse(json.loads(out)['truncated'])

    def test_no_secret_in_any_answer_or_log(self):
        for statement in ('SELECT 1', 'SELEC nonsense', 'SELECT * FROM missing_table_zz'):
            out, log = call(self.config, statement)
            self.assertNotIn(SENTINEL, out + log)
        out, log = call(self.config, 'SELECT 1', account='nobody')
        self.assertTrue(out.startswith('error:'), out)
        self.assertNotIn(SENTINEL, out + log)
        self.assertNotIn('nobody', out + log)


class Redaction(unittest.TestCase):
    def test_exceptions_results_and_logs_never_carry_a_password(self):
        env = {'DB_USER': 'u', 'DB_PASSWORD': SENTINEL}
        for raised in (RuntimeError('driver says ' + SENTINEL), ConnectionRefusedError(SENTINEL), proxy.DatabaseError('db says ' + SENTINEL)):
            def runner(*_args, _raised=raised):
                raise _raised
            log = io.StringIO()
            with unittest.mock.patch.dict(proxy.RUNNERS, {'mysql': runner}), contextlib.redirect_stderr(log):
                out, _ = proxy.answer({'body': 'SELECT 1'}, {'engine': 'mysql', 'host': 'h'}, env)
            self.assertTrue(out.startswith('error:'), out)
            self.assertNotIn(SENTINEL, out + log.getvalue())
        with unittest.mock.patch.dict(proxy.RUNNERS, {'mysql': lambda *a: {'columns': ['c'], 'rows': [[SENTINEL]], 'truncated': False}}), \
                contextlib.redirect_stderr(io.StringIO()):
            out, _ = proxy.answer({'body': 'SELECT 1'}, {'engine': 'mysql', 'host': 'h'}, env)
        self.assertNotIn(SENTINEL, out)

    def test_refused_connection_and_timeout_are_generic(self):
        log = io.StringIO()
        with contextlib.redirect_stderr(log):
            out, _ = proxy.answer({'body': 'SELECT 1'}, {'engine': 'postgres', 'host': '127.0.0.1', 'port': 1},
                                  {'DB_USER': 'u', 'DB_PASSWORD': SENTINEL})
        self.assertEqual(out, 'error: database connection or authentication failed')
        self.assertNotIn(SENTINEL, log.getvalue())


def killed_client(config, sql, active, extra_env=None):
    """Run the agent as the runner does, kill it mid-query, and count what the server still runs."""
    import datetime, time
    deadline = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=2)).isoformat()
    env = {**os.environ, 'DB_USER': 'app', 'DB_PASSWORD': SENTINEL, 'AGENT_BUS_CONFIG': json.dumps(config)}
    child = subprocess.Popen([sys.executable, '-B', str(HERE / 'db.py')], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, env=env)
    child.stdin.write(json.dumps({'body': sql, 'deadline': deadline}).encode())
    child.stdin.close()
    time.sleep(0.5)
    child.kill()
    child.wait()
    child.stdout.close()
    child.stderr.close()
    time.sleep(3.5)   # the statement limit is 2 s; the sleep is 30 s
    return active()


class ServerSideLimit(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        need('postgres', 'mysql')

    def test_postgres_query_stops_when_the_client_is_killed(self):
        probe = lambda: rows(call(CONFIG['postgres'], "SELECT count(*) FROM pg_stat_activity WHERE query LIKE 'SELECT pg_sleep(30)%' "
                                  "AND state = 'active' AND pid <> pg_backend_pid()", ['admin'])[0])[0][0]
        self.assertEqual(killed_client(CONFIG['postgres'], 'SELECT pg_sleep(30)', probe), 0)

    def test_mysql_query_stops_when_the_client_is_killed(self):
        probe = lambda: rows(call(CONFIG['mysql'], "SELECT count(*) FROM information_schema.processlist WHERE info LIKE 'SELECT SLEEP(30)%'", ['admin'])[0])[0][0]
        self.assertEqual(killed_client(CONFIG['mysql'], 'SELECT SLEEP(30)', probe), 0)


class Cells(unittest.TestCase):
    def test_deterministic_bounded_values(self):
        import datetime, decimal
        self.assertEqual(proxy.cell(decimal.Decimal('1.50')), '1.50')
        self.assertEqual(proxy.cell(datetime.datetime(2026, 1, 2, 3, 4, 5)), '2026-01-02T03:04:05')
        self.assertEqual(proxy.cell(float('nan')), 'nan')
        self.assertEqual(proxy.cell(b'\x00\xff'), 'base64:AP8=')
        self.assertEqual(len(proxy.cell('x' * (1 << 17))), 1 << 16)

    def test_row_cap_cuts_and_says_so(self):
        c = proxy.Collect(2)
        self.assertEqual([c.add([i]) for i in range(3)], [True, True, False])
        self.assertTrue(c.cut)


class MySQL(Engines, unittest.TestCase):
    name = 'mysql'
    sneaky = 'SELECT sneaky()'
    attempts = {
        'insert': ("INSERT INTO t VALUES (50,'x')", 'DELETE FROM t WHERE id = 50'),
        'update': ("UPDATE t SET v='zzz' WHERE id = 1", "UPDATE t SET v='one' WHERE id = 1"),
        'delete': ("DELETE FROM t WHERE id = 2", "INSERT INTO t VALUES (2,'two')"),
        'create': ('CREATE TABLE made (a INT)', 'DROP TABLE made'),
        'temporary': ('CREATE TEMPORARY TABLE tmpmade (a INT)', 'SELECT 1'),
        'lock': ('LOCK TABLES t WRITE', 'UNLOCK TABLES'),
    }
    smuggles = {
        'insert': "SELECT 1; INSERT INTO t VALUES (51,'x')",
        'commit then insert': "SELECT 1; COMMIT; INSERT INTO t VALUES (52,'x')",
        'unset read only': "SELECT 1; SET SESSION transaction_read_only=OFF; INSERT INTO t VALUES (53,'x')",
        'drop': 'SELECT 1; DROP TABLE t',
    }

    def exists(self, label):
        if label == 'create':
            return rows(call(self.config, "SELECT count(*) FROM information_schema.tables WHERE table_name='made'", ['admin'])[0])[0][0] > 0
        return False

    def test_multi_statements_are_off(self):
        out, _ = call(self.config, 'SELECT 1; SELECT 2')
        self.assertTrue(out.startswith('error:'), out)
        self.assertIn('syntax', out.lower())


class PostgreSQL(Engines, unittest.TestCase):
    name = 'postgres'
    sneaky = 'SELECT sneaky()'
    attempts = {
        'insert': ("INSERT INTO t VALUES (50,'x')", 'DELETE FROM t WHERE id = 50'),
        'update': ("UPDATE t SET v='zzz' WHERE id = 1", "UPDATE t SET v='one' WHERE id = 1"),
        'delete': ("DELETE FROM t WHERE id = 2", "INSERT INTO t VALUES (2,'two')"),
        'create': ('CREATE TABLE made (a INT)', 'DROP TABLE made'),
        'cte write': ("WITH w AS (INSERT INTO t VALUES (54,'x') RETURNING 1) SELECT * FROM w", 'DELETE FROM t WHERE id = 54'),
    }
    smuggles = {
        'insert': "SELECT 1; INSERT INTO t VALUES (51,'x')",
        'commit then insert': "SELECT 1; COMMIT; INSERT INTO t VALUES (52,'x')",
        'unset read only': "SELECT 1; SET default_transaction_read_only=off; COMMIT; INSERT INTO t VALUES (53,'x')",
        'drop': 'SELECT 1; DROP TABLE t',
    }

    def exists(self, label):
        if label == 'create':
            return rows(call(self.config, "SELECT count(*) FROM pg_tables WHERE tablename='made'", ['admin'])[0])[0][0] > 0
        return False

    def test_smuggle_is_refused_by_the_protocol(self):
        out, _ = call(self.config, 'SELECT 1; SELECT 2')
        self.assertIn('multiple commands', out)
        out, _ = call(self.config, 'SELECT 1; SELECT 2', ['admin'])    # positive control: the server says it, not the role
        self.assertIn('multiple commands', out)

    def test_privileged_account_limit(self):
        # The read-only transaction is not a boundary against a privileged account: dblink_exec runs on its own
        # connection. The guide says a read caller needs a read account without such privileges.
        out, _ = call(self.config, "SELECT dblink_exec('dbname=app user=postgres password=%s host=127.0.0.1', "
                                   "'CREATE TABLE leaked (a int)')" % SENTINEL, account='postgres')
        made = rows(call(self.config, "SELECT count(*) FROM pg_tables WHERE tablename='leaked'", ['admin'], account='postgres')[0])[0][0]
        call(self.config, 'DROP TABLE IF EXISTS leaked', ['admin'], account='postgres')
        self.assertEqual(made, 1, out)
        out, _ = call(self.config, "SELECT dblink_exec('dbname=app user=ro password=%s host=127.0.0.1', "
                                   "'CREATE TABLE leaked2 (a int)')" % SENTINEL, account='ro')
        self.assertTrue(out.startswith('error:'), out)        # a plain read account has no dblink


class ClickHouse(Engines, unittest.TestCase):
    name = 'clickhouse'
    count = 'SELECT count() FROM t'
    attempts = {
        'insert': ("INSERT INTO t VALUES (50,'x')", 'ALTER TABLE t DELETE WHERE id = 50 SETTINGS mutations_sync = 2'),
        'create': ('CREATE TABLE made (a UInt8) ENGINE = Memory', 'DROP TABLE made'),
        'alter': ('ALTER TABLE t ADD COLUMN extra UInt8', 'ALTER TABLE t DROP COLUMN extra'),
        'drop table': ('CREATE TABLE d (a UInt8) ENGINE = Memory', 'DROP TABLE d'),
    }
    smuggles = {
        'second statement': "SELECT 1; INSERT INTO t VALUES (51,'x')",
        'readonly off in settings': "SELECT 1 SETTINGS readonly=0",
        'set readonly': "SET readonly=0",
    }

    def test_readonly_cannot_be_turned_off_by_the_caller(self):
        for statement in ("INSERT INTO t SETTINGS readonly=0 VALUES (55,'x')", "SELECT 1 SETTINGS readonly=0"):
            before = self.total()
            out, _ = call(self.config, statement)
            self.assertTrue(out.startswith('error:'), (statement, out))
            self.assertEqual(self.total(), before)


if __name__ == '__main__':
    unittest.main()
