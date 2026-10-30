"""The owner's flow end to end: a real disposable bus, three real `#git` agent
processes on scratch clones of one scratch remote, a pipeline name and `#webhook`
publishing, and the records and scripts counted from outside.

Each agent reports its own host name (the process's `socket.getfqdn` is replaced
in a wrapper; the production code is unchanged), so one machine stands for three
servers. Nothing here touches a live daemon, checkout or account."""
import contextlib
import http.client
import io
import json
import os
from pathlib import Path
import select
import subprocess
import sys
import tempfile
import time
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
sys.path[:0] = [str(HERE.parents[2] / 'clients/python'), str(HERE.parents[2] / 'agents/webhook'), str(HERE.parents[2] / 'agents/shared')]
from agent_bus import Bus  # noqa: E402
from webhook import Agent as Webhook  # noqa: E402

REALM = 'fixture'
WRAPPER = ("import runpy,socket,sys;socket.getfqdn=lambda *a:%r;sys.path.insert(0,%r);"
           "sys.argv=[%r,'--serve'];runpy.run_path(%r,run_name='__main__')")
SECRET = b'SIGNING=fixture-signing-unique\n'
ROUTE = {'name': 'lab', 'path': '/lab', 'publish': 'autopull-cmd@fixture', 'event': 'gitlab-push',
         'branches': ['main'], 'signature': {'preset': 'gitlab', 'secret': 'SIGNING'}}
# (git agent name on the bus, host it reports, mode, the working copy is made dirty before the push)
SERVERS = [('#worker@fixture', 'host-a.test', 'mirror', True),
           ('#helper@fixture', 'host-b.test', 'mirror', False),
           ('#outsider@fixture', 'host-c.test', 'ff', True)]


def snapshot(root):
    """Every file under a copy but .git, byte for byte."""
    return {str(p.relative_to(root)): p.read_bytes() for p in sorted(Path(root).rglob('*'))
            if p.is_file() and '.git' not in p.relative_to(root).parts}


class OwnersFlow(unittest.TestCase):
    def sh(self, root, *args):
        p = subprocess.run(['git', '-C', str(root), *args], capture_output=True, text=True, timeout=20, env=self.git_env)
        self.assertEqual(p.returncode, 0, p.stderr)
        return p.stdout.strip()

    def setUp(self):
        fixture = Path(os.environ.get('AUTOPULL_BUS_FIXTURE', ROOT / 'tmp/autopull-client-fixture'))
        self.assertTrue(fixture.is_file(), 'build clients/fixture for the end-to-end proof')
        # Under the home, as the script fixtures are: a script directory is refused when any parent is group-writable.
        self.tmp = tempfile.TemporaryDirectory(prefix='.agent-bus-e2e-', dir=Path.home())
        self.addCleanup(self.tmp.cleanup)  # a crash before tearDown still removes it
        self.base = Path(self.tmp.name)
        self.git_env = {**{k: v for k, v in os.environ.items() if not k.startswith(('AB_ROLE_', 'AGENT_BUS_', 'GIT_'))},
                        'HOME': str(self.base), 'GIT_CONFIG_NOSYSTEM': '1'}
        self.procs, self.buses, self.webhooks = [], [], []
        self.bus_process = subprocess.Popen([str(fixture)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                            stderr=subprocess.PIPE, text=True)
        ready, _, _ = select.select([self.bus_process.stdout], [], [], 5)
        self.assertTrue(ready, 'the disposable bus did not start')
        self.info = json.loads(self.bus_process.stdout.readline())
        self.owner = self.bus('owner@fixture')
        self.pipeline = self.bus('#caller@fixture')

    def tearDown(self):
        for w in self.webhooks: w.close()
        for p in self.procs:
            p.terminate()
        for p in self.procs:
            with contextlib.suppress(Exception): p.wait(timeout=5)
            for s in (p.stdout, p.stderr):
                with contextlib.suppress(Exception): s.close()
        for b in self.buses: b.close()
        self.bus_process.stdin.close()
        with contextlib.suppress(Exception): self.bus_process.wait(timeout=5)
        for s in (self.bus_process.stdout, self.bus_process.stderr):
            with contextlib.suppress(Exception): s.close()

    def bus(self, name):
        b = Bus(name, addr=self.info['addr'], token=self.info['tokens'][name], timeout=3)
        self.buses.append(b)
        return b

    def remote_and_copies(self):
        up, seed = self.base / 'realty.git', self.base / 'seed'
        self.sh(self.base, 'init', '--bare', '--initial-branch=main', str(up))
        self.sh(self.base, 'clone', str(up), str(seed))
        for key, value in (('user.name', 'Server Author'), ('user.email', 'server@example.test')):
            self.sh(seed, 'config', key, value)
        (seed / 'a').write_text('one\n')
        (seed / 'b').write_text('one\n')
        self.sh(seed, 'add', '.')
        self.sh(seed, 'commit', '-m', 'first')
        self.sh(seed, 'push', '-u', 'origin', 'main')
        self.seed, self.first = seed, self.sh(seed, 'rev-parse', 'HEAD')
        self.copies = {}
        for name, host, mode, dirty in SERVERS:
            root = self.base / host / 'realty'
            self.sh(self.base, 'clone', str(up), str(root))
            self.copies[host] = root

    def advance(self, text):
        (self.seed / 'a').write_text(text)
        (self.seed / 'b').write_text(text)
        self.sh(self.seed, 'commit', '-am', 'advance ' + text.strip())
        self.sh(self.seed, 'push')
        return self.sh(self.seed, 'rev-parse', 'HEAD')

    def start_agents(self):
        self.marks = {}
        for name, host, mode, dirty in SERVERS:
            state = self.base / host / 'state'
            scripts = state / 'agent-bus-git' / 'autopull.d'
            scripts.mkdir(parents=True, mode=0o700)
            for kind in ('success', 'fail'):
                marker = self.base / host / (kind + '.log')
                self.marks[host, kind] = marker
                script = scripts / kind
                # The record arrives on stdin; one line per run is appended, so runs are countable.
                script.write_text('#!/bin/sh\ncat >> %s\necho >> %s\n' % (marker, marker))
                script.chmod(0o700)
            config = {'repos': {'realty': {'path': str(self.copies[host]), 'access': 'deploy', 'mode': mode, 'follow': True,
                                           'autopull-success-script': 'success', 'autopull-fail-script': 'fail'}},
                      'autopull-cmd': 'autopull-cmd@fixture', 'autopull-log': 'autopull-log@fixture'}  # this bus has no default realm for a bare name
            # The agent reads its Configuration from the bus, as installed: the owner saves it on the record.
            self.owner._request('POST', '/configure', {'name': name, 'config': config})
            env = {**self.git_env, 'AGENT_BUS_ADDR': self.info['addr'], 'AGENT_BUS_NAME': name,
                   'AGENT_BUS_TOKEN': self.info['tokens'][name],
                   'XDG_STATE_HOME': str(state), 'HOME': str(self.base / host)}
            program = HERE / 'git_agent.py'
            code = WRAPPER % (host, str(HERE), str(program), str(program))
            self.procs.append(subprocess.Popen([sys.executable, '-B', '-c', code], env=env, cwd=str(self.copies[host]),
                                               stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE))

    def agent_errors(self):
        """What the agents wrote to stderr, for a failure message."""
        out = []
        for p in self.procs:
            p.terminate()
            with contextlib.suppress(Exception): out.append(p.communicate(timeout=5)[1].decode()[-600:])
        return out

    def records(self, until, seconds=20):
        """Reads the log observer's inbox until `until(records)` holds or the time is out; returns what it read."""
        got, end = [], time.monotonic() + seconds
        while time.monotonic() < end:
            m = self.owner.consume(inbox='#caller@fixture', wait=.5)
            if m is not None:
                got.append(json.loads(m.body))
            if until(got):
                break
        return got

    def attempts(self, trigger, count=3, seconds=20):
        """Reads until `count` update attempts of `trigger` have arrived, each its own record: a post-attempt script's own
        failure record (action 'script') is another record and is not one of the attempts, so it cannot fill their place
        and hide the one still to come (J51). Returns what was read."""
        return self.records(lambda r: len(self.of(r, trigger)) >= count, seconds)

    @staticmethod
    def of(got, trigger):
        return [r for r in got if r['action'] == 'autopull' and r['trigger'] == trigger]

    def script_failures(self, got):
        """Each post-attempt script that did not run, by host and why: a script that could not be started is named, not guessed from a count."""
        return [(r['host'], r['script']['run'], r['script']['exit'], r['script']['stderr']) for r in got if r['action'] == 'script']

    def drain(self, seconds=2):
        """Whatever else reaches the log observer in a bounded time: an extra or doubled record shows here."""
        return self.records(lambda r: False, seconds)

    def lines(self, host, kind):
        marker = self.marks[host, kind]
        return [json.loads(x) for x in marker.read_text().splitlines() if x.strip()] if marker.exists() else []

    def settled(self, counts, seconds=10):
        """Bounded wait until the success and fail script runs equal `counts`, then a pause that a doubled run would show in."""
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if all(len(self.lines(h, k)) == n for (h, k), n in counts.items()):
                break
            time.sleep(.1)
        time.sleep(1.5)
        return {(h, k): len(self.lines(h, k)) for (h, k) in counts}

    def one_push(self, publish, text):
        """One push event from a publisher; the observed records and the script runs after it."""
        new = self.advance(text)
        for host in ('host-a.test', 'host-c.test'):  # the copies that hold local work when the push arrives
            (self.copies[host] / 'a').write_text('local edit\n')
            (self.copies[host] / 'junk').write_text('untracked\n')
        dirty_c = snapshot(self.copies['host-c.test'])
        head_c = self.sh(self.copies['host-c.test'], 'rev-parse', 'HEAD')
        publish(new)
        got = self.attempts('event')
        events = self.of(got, 'event')
        self.assertEqual(self.script_failures(got), [], 'a post-attempt script did not run')
        self.assertEqual(len(events), 3, got)
        ok = [r for r in events if r['result'] == 'ok']
        fail = [r for r in events if r['result'] == 'fail']
        self.assertEqual(sorted(r['host'] for r in ok), ['host-a.test', 'host-b.test'])
        self.assertEqual([r['discarded']['files'] for r in sorted(ok, key=lambda r: r['host'])], [2, 0], 'a mirror counts what it discards')
        for r in ok:
            self.assertEqual((r['repo'], r['mode'], r['to'], r['action'], r['remote']), ('realty', 'mirror', new, 'autopull', 'realty'))
            self.assertNotEqual(r['from'], r['to'])
        self.assertEqual([(r['host'], r['mode'], r['error']['code']) for r in fail], [('host-c.test', 'ff', 'dirty')])
        for r in events:  # who asked, what moved, whether anything was discarded
            self.assertEqual((r['requested_by'], r['id'], r['trigger']), ('#caller@fixture', None, 'event'), r)
        self.assertEqual((fail[0]['to'], fail[0]['from'], fail[0]['discarded']), (head_c, head_c, None))
        # An announced commit is looked up once the copy is cleared to move: the moved copies carry it, the
        # dirty refusal (made before the lookup) does not.
        self.assertEqual([r['requested'] for r in ok], [new, new])
        self.assertEqual(self.sh(self.copies['host-c.test'], 'rev-parse', 'HEAD'), head_c)
        self.assertEqual(snapshot(self.copies['host-c.test']), dirty_c, 'the dirty ff copy changed')
        for host in ('host-a.test', 'host-b.test'):
            self.assertEqual(self.sh(self.copies[host], 'rev-parse', 'HEAD'), new)
            self.assertEqual(self.sh(self.copies[host], 'status', '--porcelain'), '')
            self.assertEqual(snapshot(self.copies[host]), {'a': text.encode(), 'b': text.encode()}, host)
        expected = {k: n + (1 if k in (('host-a.test', 'success'), ('host-b.test', 'success'), ('host-c.test', 'fail')) else 0)
                    for k, n in self.runs.items()}
        self.assertEqual(self.settled(expected), expected,
                         'one push: each mirror copy\'s success script once, the dirty ff copy\'s fail script once, nothing else')
        self.runs = expected
        self.assertEqual(self.drain(), [], 'an extra record followed the three event records')
        for host in ('host-a.test', 'host-b.test'):
            self.assertEqual(self.lines(host, 'success')[-1]['to'], new)
        self.assertEqual(self.lines('host-c.test', 'fail')[-1]['error']['code'], 'dirty')
        return new, got

    def test_owners_flow_pipeline_then_webhook(self):
        self.remote_and_copies()
        self.owner.record('autopull-cmd@fixture').register(
            kind='pubsub', allow=['#caller@fixture'], subs=[s[0] for s in SERVERS])
        self.owner.record('autopull-log@fixture').register(
            kind='pubsub', allow=[s[0] for s in SERVERS], subs=['#caller@fixture'])
        for host in ('host-a.test', 'host-c.test'):  # local work already there when the agents start
            (self.copies[host] / 'a').write_text('local edit\n')
            (self.copies[host] / 'junk').write_text('untracked\n')
        self.start_agents()
        got = self.attempts('start', 3, 30)
        starts = self.of(got, 'start')
        self.assertEqual(self.script_failures(got), [], 'a post-attempt script did not run')
        self.assertEqual(sorted((r['host'], r['trigger'], r['result'], (r['error'] or {}).get('code')) for r in starts),
                         [('host-a.test', 'start', 'ok', None), ('host-b.test', 'start', 'skip', None),
                          ('host-c.test', 'start', 'fail', 'dirty')], starts or self.agent_errors())
        self.assertEqual([r['discarded'] for r in starts if r['host'] == 'host-a.test'], [{'files': 2, 'commits': 0}])
        self.assertEqual(self.drain(), [], 'a start attempt published more than one record')
        # The event attempts are counted from what the scripts had run after the start attempts.
        self.runs = self.settled({('host-a.test', 'success'): 0, ('host-a.test', 'fail'): 0, ('host-b.test', 'success'): 0,
                                  ('host-b.test', 'fail'): 0, ('host-c.test', 'success'): 0, ('host-c.test', 'fail'): 0}, 0)
        # A pipeline name publishes one push.
        def pipeline(commit):
            self.pipeline.send('autopull-cmd@fixture', json.dumps({'v': 1, 'type': 'push', 'remote': 'realty', 'branch': 'main', 'commit': commit}))
        self.one_push(pipeline, 'two\n')

        # The same through #webhook: a GitLab push payload on a route that publishes to the channel.
        log = io.StringIO()
        with contextlib.redirect_stderr(log):
            hook = Webhook(self.pipeline, [ROUTE], SECRET, '127.0.0.1:0')
        self.webhooks.append(hook)
        self.assertIsNone(hook.problem, hook.status())

        def webhook(commit):
            payload = {'object_kind': 'push', 'ref': 'refs/heads/main', 'before': '0' * 40, 'after': commit,
                       'project': {'path_with_namespace': 'realty', 'name': 'realty'}, 'commits': [{'message': 'fixture message'}]}
            c = http.client.HTTPConnection('127.0.0.1', hook.listener.server_port, timeout=3)
            try:
                with contextlib.redirect_stderr(log):  # the route's access line
                    c.request('POST', '/lab', json.dumps(payload), {'X-Gitlab-Token': 'fixture-signing-unique', 'X-Gitlab-Event': 'Push Hook'})
                    r = c.getresponse()
                self.assertEqual(r.status, 202, r.read())
            finally:
                c.close()
        self.one_push(webhook, 'three\n')


if __name__ == '__main__':
    unittest.main()
