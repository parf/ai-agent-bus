"""The fixture's start-up, without podman: a container that vanishes is started again once, a second failure explains itself."""
import io
import contextlib
import os
import signal
import socket
import subprocess
import sys
import threading
import time
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
        if args[0] == 'logs':   # podman logs passes the container's stderr on as its own
            return subprocess.CompletedProcess(args, 0, '', 'database system was shut down\n')
        return subprocess.CompletedProcess(args, 0, '', '')

    def verbs(self, verb):
        return [a for a in self.calls if a[0] == verb]


class Patched(unittest.TestCase):
    def setUp(self):
        self.saved = (fixtures.podman, fixtures.container, fixtures.speaks_postgres)
        self.watched = []
        saved_watch = fixtures.watch
        fixtures.watch = lambda *what: self.watched.append(what)   # no real watcher for a fake container
        self.addCleanup(lambda: setattr(fixtures, 'watch', saved_watch))
        saved_held = fixtures.gate_held
        fixtures.gate_held = lambda: None   # a fake container needs no gate; Lock tests the real check
        self.addCleanup(lambda: setattr(fixtures, 'gate_held', saved_held))
        fixtures.START_SECONDS = 0.05
        self.addCleanup(lambda: setattr(fixtures, 'START_SECONDS', 45))
        self.addCleanup(lambda: setattr(fixtures, 'podman', self.saved[0]) or setattr(fixtures, 'container', self.saved[1])
                        or setattr(fixtures, 'speaks_postgres', self.saved[2]))
        self.made = []

    def containers(self, *names):
        it = iter(names)

        def container(image, port, env, files=()):
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
            self.assertIn('label=ab-db-fixture=' + fixtures.RUN, call)

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
        self.assertIn('ab-db-fixture=' + fixtures.RUN, run)
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


class Leaks(Patched):
    def setUp(self):
        super().setUp()
        global saved_leaked
        saved_leaked = fixtures.leaked

    def test_a_failed_starts_logs_are_copied_out_before_the_container_is_removed(self):
        fake = fixtures.podman = FakePodman()
        fixtures.speaks_postgres = lambda port: False
        self.containers('c1', 'c2')
        self.quiet(fixtures.bring_up, 'PostgreSQL', lambda: fixtures.container(None, 0, {}),
                   lambda name, port: False, lambda name, port: None, 0.05)
        copy = Path(fixtures.private()) / 'ab-db-fixture-1-2.log'
        self.assertIn('database system was shut down', copy.read_text())
        order = [c[0] for c in fake.calls]
        self.assertLess(order.index('logs'), order.index('rm'))   # the lines are kept, then the container goes

    def test_the_private_dir_goes_with_the_run(self):
        path = fixtures.private()
        self.assertTrue(os.path.isdir(path))
        self.assertTrue(path.startswith(str(Path(__file__).resolve().parents[3] / 'tmp')))
        fixtures._cleanup()
        self.assertFalse(os.path.exists(path))

    def test_every_container_has_a_watcher_and_a_capped_log(self):
        fake = fixtures.podman = FakePodman()
        saved = fixtures.STOP[:]
        try:
            name, _ = self.saved[1]('image', 5432, {})
        finally:
            fixtures.STOP[:] = saved
        starting, *cleanup = [w for w in self.watched if w[1] != 'rm'][0]
        self.assertEqual(cleanup, ['podman', '--events-backend=none', 'rm', '-f', '-t', '1', name])
        run = fake.verbs('run')[0]
        self.assertIn('max-size=10mb', run)
        self.assertIn('path=%s/%s.ctr.log' % (fixtures.private(), name), run)   # in the run's dir, never journald
        command = ' '.join([*fixtures.PODMAN, *run, 'image'])
        self.assertRegex(command, starting)   # the watcher recognises this start while it is in flight
        self.assertNotRegex(' '.join(['sh', '-c', fixtures.WATCH, 'sh', fixtures.RUN, starting, *cleanup]), starting)   # not itself
        self.assertNotRegex(command.replace(name, name + '9'), starting)   # nor a neighbour's start

    def test_nothing_podman_does_for_a_fixture_reaches_the_journal(self):
        self.assertEqual(fixtures.PODMAN, ('podman', '--events-backend=none'))
        saved, fixtures.PRIVATE[:] = fixtures.PRIVATE[:], []
        try:
            path = fixtures.private()
        finally:
            __import__('shutil').rmtree(fixtures.PRIVATE[0], ignore_errors=True)
            fixtures.PRIVATE[:] = saved
        starting = '^podman .* run .*--name ab-db-fixture-%s-' % fixtures.RUN
        self.assertIn((starting, 'rm', '-rf', path), self.watched)   # the private dir has its watcher too, after any start

    def test_clickhouse_logs_to_the_console_only(self):
        self.assertIn('<console>1</console>', fixtures.LOGGING)
        self.assertIn('<log remove="remove"/>', fixtures.LOGGING)
        self.assertIn('<errorlog remove="remove"/>', fixtures.LOGGING)

    def test_a_container_left_by_a_dead_run_is_removed_when_the_next_one_starts(self):
        fake = fixtures.podman = FakePodman()
        fixtures.leaked = lambda: ['ab-db-fixture-999999999-2']
        self.addCleanup(lambda: setattr(fixtures, 'leaked', saved_leaked))
        saved = fixtures.STOP[:]
        try:
            self.saved[1]('image', 5432, {})
        finally:
            fixtures.STOP[:] = saved
        self.assertIn(('rm', '-f', '-t', '1', 'ab-db-fixture-999999999-2'), fake.calls)

    def test_a_store_that_cannot_be_listed_is_not_an_empty_one(self):
        fixtures.podman = lambda *a, check=True: subprocess.CompletedProcess(a, 125, '', 'storage broke')
        with self.assertRaises(RuntimeError) as caught:
            fixtures.leaked()
        self.assertIn('storage broke', str(caught.exception))

    def test_leaked_names_only_containers_whose_test_is_gone(self):
        reused = '%d-0' % os.getpid()   # this pid, an earlier start: a dead run whose pid came back
        fixtures.podman = lambda *a, check=True: subprocess.CompletedProcess(
            a, 0, 'mine %s\ngone 999999999-5\nreused %s\nold\n' % (fixtures.RUN, reused), '')
        self.assertEqual(fixtures.leaked(), ['gone', 'reused', 'old'])

    def test_a_dead_runs_private_dirs_are_swept_and_a_live_ones_kept(self):
        made = [Path(fixtures.SCRATCH) / n for n in ('ab-db-fixture-999999999-5-a', 'ab-db-maria-%d-0-b' % os.getpid(),
                                                      'ab-db-fixture-%s-c' % fixtures.RUN)]
        for path in made:
            path.mkdir(parents=True)
            self.addCleanup(lambda p=path: __import__('shutil').rmtree(p, ignore_errors=True))
        fixtures.sweep()
        self.assertEqual([p.exists() for p in made], [False, False, True])


class Lock(Patched):
    """No container starts unless someone holds the smoke-gate lock (J32): a stub agent-bus answers holders."""
    HELD = 'smoke-gate@parf.us slow #ab-gate 14m left'

    def setUp(self):
        super().setUp()
        fixtures.gate_held = Lock.real_held   # Patched stubbed it; this class checks the real one
        self.dir = Path(fixtures.SCRATCH) / f'ab-db-lock-{fixtures.RUN}-{self.id().rsplit(".", 1)[1]}'
        self.dir.mkdir(parents=True, exist_ok=True)
        self.addCleanup(lambda: __import__('shutil').rmtree(self.dir, ignore_errors=True))
        self.asked = self.dir / 'asked'
        stub = self.dir / 'agent-bus'
        stub.write_text('#!/bin/sh\necho "$*" >>"$ASKED"\nprintf "%s" "$SAY"\nexit "${RC:-0}"\n')
        stub.chmod(0o755)
        self.env = {'PATH': f'{self.dir}:{os.environ["PATH"]}', 'ASKED': str(self.asked)}
        self.fake = fixtures.podman = FakePodman()

    def start(self, say, rc=0):
        saved = dict(os.environ)
        os.environ.update(self.env, SAY=say, RC=str(rc))
        try:
            return self.quiet(fixtures.container, 'image', 5432, {})
        finally:
            os.environ.clear()
            os.environ.update(saved)

    def refused(self, say, rc=0):
        result, _ = self.start(say, rc)
        self.assertIsInstance(result, RuntimeError)
        self.assertIn('refused to start a container', str(result))
        self.assertEqual(self.fake.calls, [], 'podman was called before the lock was checked')
        self.assertEqual(self.asked.read_text(), 'holders smoke-gate@parf.us\n')   # it asked, once, about the right record
        return str(result)

    def test_nobody_holding_the_lock_refuses_before_any_podman_call(self):
        self.assertIn('nobody holds the slow lock on smoke-gate@parf.us', self.refused('smoke-gate@parf.us holds no locks\n'))

    def test_another_lock_on_the_record_is_not_the_gate(self):
        self.refused('smoke-gate@parf.us fast #ab-w1a 3m left\n')

    def test_a_failing_holders_answer_refuses(self):
        self.assertIn('failed (exit 1)', self.refused(self.HELD + '\n', rc=1))

    def test_a_held_lock_lets_the_run_proceed(self):
        result, _ = self.start(self.HELD + '\n')
        self.assertIsInstance(result, tuple, result)
        self.assertTrue(result[0].startswith(f'ab-db-fixture-{fixtures.RUN}-'))
        self.assertEqual(len(self.fake.verbs('run')), 1)


Lock.real_held = fixtures.gate_held


class Identity(unittest.TestCase):
    def test_a_run_is_its_pid_and_start_time(self):
        self.assertEqual(fixtures.RUN, '%d-%s' % (os.getpid(), fixtures.started(os.getpid())))
        self.assertFalse(fixtures.dead(fixtures.RUN))
        self.assertTrue(fixtures.dead('%d-0' % os.getpid()))   # the same pid, another start
        self.assertTrue(fixtures.dead('999999999-5'))
        self.assertTrue(fixtures.dead(''))


CHILD_PATH = str(Path(__file__).resolve().parent)


class Watcher(unittest.TestCase):
    """The real watcher on real processes, no podman: it acts once its run is over, and not before a start finishes."""
    def setUp(self):
        self.dir = Path(fixtures.SCRATCH) / ('watcher-test-%d' % os.getpid())
        self.dir.mkdir(parents=True, exist_ok=True)
        self.addCleanup(lambda: __import__('shutil').rmtree(self.dir, ignore_errors=True))

    def wait_for(self, path, seconds):
        end = time.time() + seconds
        while time.time() < end and not path.exists():
            time.sleep(0.2)
        return path.exists()

    def test_a_start_in_flight_when_the_run_is_killed_finishes_before_the_cleanup(self):
        created, removed = self.dir / 'created', self.dir / 'removed'
        # The child stands in for a test: a `podman run` that takes 2 s (an image pull), then the watcher for it.
        start = ('import os, sys, time; time.sleep(2); open(sys.argv[-1], "w").close()')
        child = subprocess.Popen([sys.executable, '-c', (
            'import sys, subprocess, time; sys.path.insert(0, %r); import fixtures\n'
            'name = "ab-db-fixture-%%s-1" %% fixtures.RUN\n'
            'subprocess.Popen(["podman", "-c", %r, "--events-backend=none", "run", "-d", "--name", name, %r], executable=sys.executable,'
            ' start_new_session=True)\n'
            'fixtures.watch("^podman .* run .*--name %%s( |$)" %% name, "sh", "-c", \'[ -e "$1" ] && touch "$2" || touch "$2.early"\', "sh", %r, %r)\n'
            'print(1, flush=True); time.sleep(600)\n') % (CHILD_PATH, start, str(created), str(created), str(removed))],
            stdout=subprocess.PIPE, text=True)
        self.addCleanup(child.wait)
        child.stdout.readline()
        child.kill()   # SIGKILL: no handler, no atexit; the start is still in flight
        self.assertTrue(self.wait_for(removed, 15) or self.wait_for(Path(str(removed) + '.early'), 1), 'no cleanup at all')
        self.assertFalse(Path(str(removed) + '.early').exists(), 'the cleanup ran before the in-flight start had made its container')

    def test_a_pgrep_error_is_not_none_in_flight(self):
        # Review regression (#ab-w1c): pgrep exiting 2 must keep the watcher waiting; only 1 (none) lets it clean up.
        fake, seen, allowed, cleaned = [self.dir / p for p in ('pgrep', 'seen', 'allowed', 'cleaned')]
        fake.write_text('#!/bin/sh\ntouch "$SEEN"\n[ ! -f "$ALLOWED" ] || exit 1\nexit 2\n')
        fake.chmod(0o755)
        child = subprocess.Popen(['sh', '-c', fixtures.WATCH, 'sh', '999999999-0', 'fixture-probe', 'touch', str(cleaned)],
                                 env={**os.environ, 'PATH': '%s:%s' % (self.dir, os.environ['PATH']), 'SEEN': str(seen),
                                      'ALLOWED': str(allowed)}, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.addCleanup(lambda: child.poll() is None and child.kill())
        self.assertTrue(self.wait_for(seen, 3), 'the fake pgrep never ran')
        time.sleep(1.2)   # at least one more turn of the watcher's loop
        self.assertFalse(cleaned.exists(), 'the watcher cleaned up after pgrep failed')
        allowed.touch()
        self.assertEqual(child.wait(10), 0)
        self.assertTrue(cleaned.exists(), 'the watcher never cleaned up once pgrep said none')

    def test_the_cleanup_runs_after_a_sigkill(self):
        done = self.dir / 'done'
        child = subprocess.Popen([sys.executable, '-c', (
            'import sys, time; sys.path.insert(0, %r); import fixtures\n'
            'fixtures.watch("", "touch", %r); print(1, flush=True); time.sleep(600)\n') % (CHILD_PATH, str(done))],
            stdout=subprocess.PIPE, text=True)
        self.addCleanup(child.wait)
        child.stdout.readline()
        time.sleep(1)
        self.assertFalse(done.exists(), 'the cleanup ran while its run was alive')
        child.kill()
        self.assertTrue(self.wait_for(done, 10), 'no cleanup after SIGKILL')


class SignalExit(unittest.TestCase):
    """SIGTERM ends a unittest run at once, cleaned up: unittest catches SystemExit as one test's error and goes on."""
    def test_sigterm_under_unittest_cleans_up_and_stops_the_run(self):
        work = Path(fixtures.SCRATCH) / ('signal-test-%d' % os.getpid())
        work.mkdir(parents=True, exist_ok=True)
        self.addCleanup(lambda: __import__('shutil').rmtree(work, ignore_errors=True))
        (work / 'test_victim.py').write_text(
            'import sys, time, unittest; sys.path.insert(0, %r); import fixtures\n'
            'class V(unittest.TestCase):\n'
            '    def test_a(self):\n'
            '        fixtures.STOP.append(lambda: open(%r, "w").close())\n'
            '        print("started", flush=True); time.sleep(30)\n'
            '    def test_b(self):\n'
            '        open(%r, "w").close()\n' % (CHILD_PATH, str(work / 'cleaned'), str(work / 'went-on')))
        child = subprocess.Popen([sys.executable, '-m', 'unittest', 'test_victim'], cwd=work, stdout=subprocess.PIPE,
                                 stderr=subprocess.DEVNULL, text=True)
        self.addCleanup(lambda: child.poll() is None and child.kill())
        self.assertEqual(child.stdout.readline().strip(), 'started')
        child.send_signal(signal.SIGTERM)
        self.assertEqual(child.wait(20), 128 + signal.SIGTERM)
        self.assertTrue((work / 'cleaned').exists(), 'SIGTERM did not run the cleanup')
        self.assertFalse((work / 'went-on').exists(), 'the run went on to the next test after SIGTERM')


@unittest.skipUnless(subprocess.run(['which', 'podman'], capture_output=True).returncode == 0, 'podman is not installed')
class KilledRun(unittest.TestCase):
    """A real container, a real kill: nothing of the fixture may be left. Needs the smoke-gate lock (rootless podman)."""
    CHILD = ('import sys; sys.path.insert(0, %r); import fixtures, time\n'
             'name, port = fixtures.container(fixtures.POSTGRES, 5432, {"POSTGRES_PASSWORD": "x"})\n'
             'print(name, flush=True); time.sleep(600)\n') % str(Path(__file__).resolve().parent)

    def run_and_kill(self, number):
        child = subprocess.Popen([sys.executable, '-c', self.CHILD], stdout=subprocess.PIPE, text=True)
        self.addCleanup(child.wait)
        name = child.stdout.readline().strip()
        self.assertTrue(name.startswith('ab-db-fixture-%d-' % child.pid), name)
        self.assertEqual(fixtures.podman('ps', '-a', '--filter', 'name=' + name, '-q').stdout.count('\n'), 1)   # it exists
        self.addCleanup(lambda: fixtures.podman('rm', '-f', '-t', '1', name, check=False))
        os.kill(child.pid, number)
        child.wait(30)
        end = __import__('time').time() + 30
        while __import__('time').time() < end and fixtures.podman('ps', '-a', '--filter', 'name=' + name, '-q').stdout.strip():
            __import__('time').sleep(0.5)
        left = fixtures.podman('ps', '-a', '--filter', 'name=' + name, '-q').stdout.strip()
        self.assertEqual(left, '', 'the container outlived its killed test')
        self.assertEqual([n for n in fixtures.leaked() if n.startswith('ab-db-fixture-%d-' % child.pid)], [])   # a neighbour's is not this test's

    # The run is still in flight when the test is killed, as during an image pull: a process named podman, with the
    # run's own arguments, that waits 4 s and then becomes the real podman run.
    SLOW_START = ('import sys, subprocess; sys.path.insert(0, %r); import fixtures, time\n'
                  'real = fixtures.podman\n'
                  'SHIM = "import os, sys, time; time.sleep(4); os.execvp(\\"podman\\", [\\"podman\\"] + sys.argv[1:])"\n'
                  'def slow(*args, check=True):\n'
                  '    if args[0] == "run":\n'
                  '        return subprocess.run(["podman", "-c", SHIM, *fixtures.PODMAN[1:], *args], executable=sys.executable,'
                  ' check=check, capture_output=True, text=True)\n'
                  '    return real(*args, check=check)\n'
                  'fixtures.podman = slow\n'
                  'print(1, flush=True); fixtures.container(fixtures.POSTGRES, 5432, {"POSTGRES_PASSWORD": "x"})\n') % str(Path(__file__).resolve().parent)

    def test_a_kill_during_startup_leaves_no_container(self):
        child = subprocess.Popen([sys.executable, '-c', self.SLOW_START], stdout=subprocess.PIPE, text=True)
        self.addCleanup(child.wait)
        child.stdout.readline()
        mine = 'name=ab-db-fixture-%d-' % child.pid
        self.addCleanup(lambda: [fixtures.podman('rm', '-f', '-t', '1', n, check=False)
                                 for n in fixtures.podman('ps', '-a', '--filter', mine, '--format', '{{.Names}}').stdout.split()])
        __import__('time').sleep(1)
        os.kill(child.pid, signal.SIGKILL)   # the first removal fires now, before `podman run` has made the container
        child.wait(30)
        end = __import__('time').time() + 40
        seen = False
        while __import__('time').time() < end:
            if fixtures.podman('ps', '-a', '--filter', mine, '-q').stdout.strip():
                seen = True   # created late, after the first removal
            elif seen:
                break
            __import__('time').sleep(0.5)
        self.assertTrue(seen, 'the slow start never made its container: the test measured nothing')
        self.assertEqual(fixtures.podman('ps', '-a', '--filter', mine, '-q').stdout.strip(), '', 'a late container survived')

    def test_sigterm_leaves_no_container(self):
        self.run_and_kill(signal.SIGTERM)

    def test_sigkill_leaves_no_container(self):
        self.run_and_kill(signal.SIGKILL)


@unittest.skipUnless(subprocess.run(['which', 'podman'], capture_output=True).returncode == 0, 'podman is not installed')
class Journal(unittest.TestCase):
    """A real fixture container writes nothing to the host journal (CLAUDE.md: normal runs write nothing there; J32)."""
    def count(self, *match):
        out = subprocess.run(['journalctl', '--user', '-q', '-o', 'cat', *match], capture_output=True, text=True)
        return len(out.stdout.splitlines())

    def test_a_fixture_container_writes_nothing_to_the_journal(self):
        probe = subprocess.run(['journalctl', '--user', '-n0', '--show-cursor'], capture_output=True, text=True)
        cursor = probe.stdout.rpartition('-- cursor: ')[2].strip()
        self.assertTrue(cursor, 'the user journal cannot be read here, so silence would prove nothing: %r' % probe.stderr)
        saved = fixtures.STOP[:]
        try:
            name, port = fixtures.container(fixtures.POSTGRES, 5432, {'POSTGRES_PASSWORD': 'x'})
            before = (self.count('CONTAINER_NAME=' + name), self.count('PODMAN_NAME=' + name))
            fixtures.wait(lambda: fixtures.speaks_postgres(port), 'PostgreSQL', 60)   # it has logged its start by now
            log = Path(fixtures.private()) / (name + '.ctr.log')
            said = log.stat().st_size if log.exists() else 0
        finally:
            for stop in fixtures.STOP[len(saved):]:
                stop()
            fixtures.STOP[:] = saved
        time.sleep(1)   # journald takes what it is sent asynchronously
        after = (self.count('CONTAINER_NAME=' + name), self.count('PODMAN_NAME=' + name))
        self.assertEqual(before, (0, 0))
        self.assertEqual(after, before, 'the container or podman wrote to the host journal')
        since = subprocess.run(['journalctl', '--user', '-q', '-o', 'verbose', '--after-cursor', cursor],
                               capture_output=True, text=True).stdout
        self.assertNotIn(name, since, 'something wrote this container to the journal')
        self.assertGreater(said, 0, 'the container said nothing to its own log, so the silence above measured nothing')


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
