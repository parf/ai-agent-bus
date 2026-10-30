"""The fixture's start-up, without podman: a container that vanishes is started again once, a second failure explains itself."""
import io
import contextlib
import socket
import subprocess
import sys
import threading
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import fixtures


class FakePodman:
    """Stands in for podman: records each call, fails the exec calls a scenario names."""
    def __init__(self, fail_exec_on=(), fail_setup_on=()):
        self.calls, self.fail_exec_on, self.fail_setup_on = [], set(fail_exec_on), set(fail_setup_on)

    def __call__(self, *args, check=True):
        self.calls.append(args)
        if args[0] == 'exec' and args[1] in self.fail_exec_on:
            if check:
                raise subprocess.CalledProcessError(255, ['podman', *args])
            return subprocess.CompletedProcess(args, 255, '', '')
        if args[0] == 'exec' and args[1] in self.fail_setup_on and any('CREATE ROLE rw' in a for a in args):
            raise subprocess.CalledProcessError(255, ['podman', *args])   # ready, then gone mid-setup
        if args[0] == 'ps':
            text = 'ab-db-fixture-1-2\n' if args[-1] == '{{.Names}}' else 'ab-db-fixture-1-2 Exited (137)\n'
            return subprocess.CompletedProcess(args, 0, text, '')
        if args[0] == 'logs':
            return subprocess.CompletedProcess(args, 0, 'database system was shut down\n', '')
        return subprocess.CompletedProcess(args, 0, '', '')

    def verbs(self, verb):
        return [a for a in self.calls if a[0] == verb]


class Patched(unittest.TestCase):
    def setUp(self):
        self.saved = (fixtures.podman, fixtures.container, fixtures.speaks_postgres)
        fixtures.START_SECONDS = 0.05
        self.addCleanup(lambda: setattr(fixtures, 'START_SECONDS', 45))
        self.addCleanup(lambda: setattr(fixtures, 'podman', self.saved[0]) or setattr(fixtures, 'container', self.saved[1])
                        or setattr(fixtures, 'speaks_postgres', self.saved[2]))
        self.made = []

    def containers(self, *names):
        it = iter(names)

        def container(image, port, env):
            name = next(it)
            self.made.append(name)
            return name, 5000 + len(self.made)
        fixtures.container = container

    def quiet(self, fn, *a, **kw):
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            try:
                return fn(*a, **kw), err.getvalue()
            except Exception as error:
                return error, err.getvalue()


class StartUp(Patched):
    def test_a_container_that_starts_is_used_once(self):
        fixtures.podman = FakePodman()
        fixtures.speaks_postgres = lambda port: True
        self.containers('c1')
        config, err = self.quiet(fixtures.postgres)
        self.assertEqual(config['port'], 5001)
        self.assertEqual(self.made, ['c1'])
        self.assertEqual(err, '')

    def test_a_container_that_vanishes_during_setup_is_started_again_once(self):
        # The P.2 gate failure: `podman exec ... CREATE ROLE rw` exited 255 because the container was gone.
        fake = fixtures.podman = FakePodman(fail_exec_on={'c1'})
        fixtures.speaks_postgres = lambda port: True
        self.containers('c1', 'c2')
        # c1 answers the readiness probe's select 1 with a failure exit, so it never becomes ready; c2 is used.
        config, err = self.quiet(fixtures.postgres)
        self.assertEqual(self.made, ['c1', 'c2'])
        self.assertEqual(config['port'], 5002)
        self.assertIn(('rm', '-f', '-t', '1', 'c1'), fake.calls)
        self.assertIn('fixture PostgreSQL failed', err)

    def test_a_container_that_is_ready_and_then_gone_during_setup_is_started_again(self):
        # P.2 exactly: the probe passed, then `podman exec ... CREATE ROLE rw` exited 255.
        fake = fixtures.podman = FakePodman(fail_setup_on={'c1'})
        fixtures.speaks_postgres = lambda port: True
        self.containers('c1', 'c2')
        config, err = self.quiet(fixtures.postgres)
        self.assertEqual(self.made, ['c1', 'c2'])
        self.assertEqual(config['port'], 5002)
        self.assertIn(('rm', '-f', '-t', '1', 'c1'), fake.calls)

    def test_the_wait_is_the_modules_budget_unless_a_caller_gives_one(self):
        seen = []
        saved = fixtures.wait
        fixtures.wait = lambda probe, what, seconds=60: seen.append(seconds)
        self.addCleanup(lambda: setattr(fixtures, 'wait', saved))
        fixtures.podman = FakePodman()
        self.containers('c1')
        fixtures.START_SECONDS = 7
        fixtures.bring_up('X', lambda: fixtures.container(None, 0, {}), lambda n, p: True, lambda n, p: None)
        self.containers('c2')
        fixtures.bring_up('X', lambda: fixtures.container(None, 0, {}), lambda n, p: True, lambda n, p: None, 3)
        self.assertEqual(seen, [7, 3])

    def test_a_container_that_never_answers_is_replaced_and_then_the_failure_explains_itself(self):
        # The B.27 gate failure: 90 s of probes that never succeeded.
        fake = FakePodman()
        fixtures.podman = fake
        fixtures.speaks_postgres = lambda port: False
        self.containers('c1', 'c2')
        error, err = self.quiet(fixtures.bring_up, 'PostgreSQL', lambda: fixtures.container(None, 0, {}),
                                lambda name, port: fixtures.speaks_postgres(port), lambda name, port: None, 0.05)
        self.assertIsInstance(error, RuntimeError)
        self.assertIn('did not start in two attempts', str(error))
        self.assertEqual(self.made, ['c1', 'c2'])
        self.assertEqual(len(fake.verbs('logs')), 2)   # one container named per failed attempt
        self.assertIn('database system was shut down', err)
        self.assertIn('Exited (137)', err)

    def test_the_failure_names_the_first_error_as_well_as_the_last(self):
        fixtures.podman = FakePodman()
        self.containers('c1', 'c2')
        errors = iter([RuntimeError('first-cause'), RuntimeError('last-cause')])

        def setup(name, port):
            raise next(errors)
        error, _ = self.quiet(fixtures.bring_up, 'X', lambda: fixtures.container(None, 0, {}),
                              lambda name, port: True, setup, 1)
        self.assertIn('first-cause', str(error))
        self.assertIn('last-cause', str(error))

    def test_the_diagnostics_name_only_this_runs_containers(self):
        fake = fixtures.podman = FakePodman()
        fixtures.speaks_postgres = lambda port: False
        self.containers('c1', 'c2')
        self.quiet(fixtures.bring_up, 'PostgreSQL', lambda: fixtures.container(None, 0, {}),
                   lambda name, port: False, lambda name, port: None, 0.05)
        for call in fake.verbs('ps'):
            self.assertIn('label=ab-db-fixture=%d' % __import__('os').getpid(), call)

    def test_a_container_is_started_with_this_runs_label(self):
        fake = fixtures.podman = FakePodman()
        fixtures.podman = fake
        fixtures.container.__globals__['podman'] = fake
        saved = fixtures.STOP[:]
        try:
            name, port = self.saved[1]('image', 5432, {'A': 'b'})
        finally:
            fixtures.STOP[:] = saved
        run = fake.verbs('run')[0]
        self.assertIn('--label', run)
        self.assertNotIn('--rm', run)   # an exited container must stay for diagnose to list and read
        self.assertIn('ab-db-fixture=%d' % __import__('os').getpid(), run)
        self.assertTrue(name.startswith('ab-db-fixture-'))


    def test_a_run_that_fails_after_creating_the_container_still_has_it_removed(self):
        class Failing(FakePodman):
            def __call__(self, *args, check=True):
                super().__call__(*args, check=check)
                if args[0] == 'run':
                    raise subprocess.CalledProcessError(126, ['podman', *args])   # created, then the start failed
                return subprocess.CompletedProcess(args, 0, '', '')
        fake = fixtures.podman = Failing()
        saved = fixtures.STOP[:]
        del fixtures.STOP[:]
        try:
            with self.assertRaises(subprocess.CalledProcessError):
                self.saved[1]('image', 5432, {})
            name = fake.verbs('run')[0][fake.verbs('run')[0].index('--name') + 1]
            for stop in fixtures.STOP:
                stop()
        finally:
            fixtures.STOP[:] = saved
        self.assertIn(('rm', '-f', '-t', '1', name), fake.calls)


class Wire(unittest.TestCase):
    def serve(self, reply):
        server = socket.socket()
        server.bind(('127.0.0.1', 0))
        server.listen(1)
        self.addCleanup(server.close)

        def run():
            conn, _ = server.accept()
            conn.recv(8)
            if reply:
                conn.sendall(reply)
            conn.close()
        threading.Thread(target=run, daemon=True).start()
        return server.getsockname()[1]

    def test_a_server_that_answers_the_ssl_request_is_postgres(self):
        self.assertTrue(fixtures.speaks_postgres(self.serve(b'N')))

    def test_a_listener_that_closes_without_answering_is_not(self):
        self.assertFalse(fixtures.speaks_postgres(self.serve(b'')))

    def test_a_closed_port_is_not(self):
        s = socket.socket()
        s.bind(('127.0.0.1', 0))
        port = s.getsockname()[1]
        s.close()
        with self.assertRaises(OSError):
            fixtures.speaks_postgres(port)


if __name__ == '__main__':
    unittest.main()
