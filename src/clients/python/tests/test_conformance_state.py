"""Native Python adapter for the shared state vectors: kv, locks and bigfile."""
import base64
import hashlib
import http.server
import json
import threading
import unittest

from agent_bus import Bus, BusError, Refusal
import test_client as fixture
from test_conformance import cases


class SharedState(unittest.TestCase):
    # Each vector case gets a daemon of its own: the fixture is cheap and no case depends on another.
    setUp = fixture.LiveClient.setUp
    tearDown = fixture.LiveClient.tearDown
    bus = fixture.LiveClient.bus
    serve = fixture.LiveClient.serve

    def each_case(self, name):
        for case in cases(name):
            self.tearDown()
            self.setUp()
            yield case

    def step_value(self, step):
        if 'repeat' in step:
            return step['repeat']['text'] * step['repeat']['count']
        if 'value_b64' in step:
            return base64.b64decode(step['value_b64'])
        return step.get('value', '')

    def refusal(self, where, step, call):
        """Run call; True when the step expected a refusal and got it, else its result via self.result."""
        want = (step.get('expect') or {}).get('refused')
        self.result = None
        if not want:
            self.result = call()
            return False
        with self.assertRaises(BusError, msg=where) as raised:
            call()
        if isinstance(want, dict):
            error = raised.exception
            self.assertIsInstance(error, Refusal, where)
            self.assertEqual(error.status, want['status'], where + ' ' + str(error))
            if 'message_has' in want:
                self.assertIn(want['message_has'], str(error), where)
        return True

    def test_shared_kv_vectors(self):
        for case in self.each_case('kv'):
            kv = self.owner.record('jobs@fixture').kv
            for i, step in enumerate(case['steps']):
                where = f"{case['id']} step {i} {step['op']} {step.get('name', '')}"
                expect = step.get('expect') or {}
                kind = step.get('kind', 'string')
                op = step['op']
                if op == 'set':
                    value = self.step_value(step)
                    if kind == 'int':
                        value = int(value)
                    elif kind == 'json':
                        value = json.loads(value)
                    self.refusal(where, step, lambda: kv.set(step['name'], value, kind=kind, mode=step.get('how', 'set')))
                elif op == 'get':
                    if self.refusal(where, step, lambda: kv.get(step['name'], kind=kind)):
                        continue
                    got = self.result
                    if 'value_b64' in expect:
                        raw = got if isinstance(got, bytes) else got.encode()
                        self.assertEqual(base64.b64encode(raw).decode(), expect['value_b64'], where)
                    elif 'length' in expect:
                        self.assertEqual(len(got), expect['length'], where)
                    elif 'json' in expect:
                        self.assertEqual(got, expect['json'], where)
                    elif kind == 'int':
                        self.assertEqual(str(got), expect['value'], where)
                    else:
                        self.assertEqual(got, expect['value'], where)
                elif op == 'fill':
                    for n in range(step['count']):
                        kv.set(step['prefix'] + str(n), 'v', kind=kind, mode='add')
                elif op == 'delete':
                    if not self.refusal(where, step, lambda: kv.delete(step['name'], kind=kind)):
                        self.assertEqual(self.result, expect['deleted'], where)
                elif op == 'inc':
                    if not self.refusal(where, step, lambda: kv.inc(step['name'], by=int(step['by']))) and 'value' in expect:
                        self.assertEqual(str(self.result), expect['value'], where)
                elif op == 'json':
                    if not self.refusal(where, step, lambda: kv.json(step['name'], *step['ops'])) and 'changed' in expect:
                        self.assertEqual([r['changed'] for r in self.result], expect['changed'], where)
                else:
                    self.fail('unknown vector op ' + op)

    def test_shared_lock_vectors(self):
        for case in self.each_case('locks'):
            buses = {'owner': self.owner, 'worker': self.worker}
            for setup in case.get('setup', []):
                self.assertEqual(setup, 'worker-maintains-jobs')
                self.owner.record('jobs@fixture').manage(maintainers=['#worker@fixture'])
            for i, step in enumerate(case['steps']):
                where = f"{case['id']} step {i} {step['as']} {step['op']} {step.get('name', '')}"
                locks = buses[step['as']].record('jobs@fixture').locks
                expect = step.get('expect') or {}
                ttl, op, name = step.get('ttl_s', 0), step['op'], step.get('name')
                calls = {
                    'try': lambda: locks.try_acquire(name, ttl=ttl),
                    'acquire': lambda: locks.acquire(name, ttl=ttl, wait=step.get('wait_s', 0)),
                    'extend': lambda: locks.extend(name, ttl=ttl),
                    'release': lambda: locks.release(name),
                    'force': lambda: locks.force_release(name),
                    'holders': lambda: locks.holders(),
                }
                self.assertIn(op, calls, where)
                if self.refusal(where, step, calls[op]):
                    continue
                if op == 'force':
                    self.assertEqual(self.result, expect['displaced'], where)
                elif op == 'holders':
                    self.assertEqual({k: v['holder'] for k, v in self.result.items()}, expect['holders'], where)

    def test_shared_bigfile_vectors(self):
        for case in self.each_case('bigfile'):
            big = self.owner.record('jobs@fixture').bigfile
            for i, step in enumerate(case['steps']):
                where = f"{case['id']} step {i} {step['op']} {step.get('name', '')}"
                expect = step.get('expect') or {}
                op = step['op']
                if op == 'put':
                    value = self.step_value(step)
                    value = value.encode() if isinstance(value, str) else value
                    options = {'ttl': step['ttl_s'], 'memory': bool(step.get('memory'))}
                    if step.get('sha256'):
                        options['sha256'] = step['sha256']
                    if self.refusal(where, step, lambda: big.put(step['name'], value, **options)):
                        continue
                    got = self.result
                    for key in ('size', 'sha256', 'held', 'replaced'):
                        if key in expect:
                            self.assertEqual(got[key], expect[key], where + ' ' + key)
                elif op == 'get':
                    if self.refusal(where, step, lambda: big.get(step['name'])):
                        continue
                    data, info = self.result
                    self.assertEqual(data.decode(), expect['value'], where)
                    self.assertEqual(info['sha256'], hashlib.sha256(data).hexdigest(), where)
                    if 'sha256' in expect:
                        self.assertEqual(info['sha256'], expect['sha256'], where)
                elif op == 'list':
                    self.refusal(where, step, lambda: big.list())
                    self.assertEqual([row['name'] for row in self.result], expect['names'], where)
                elif op == 'delete':
                    if not self.refusal(where, step, lambda: big.delete(step['name'])):
                        self.assertEqual(self.result, expect['deleted'], where)
                else:
                    self.fail('unknown vector op ' + op)

    def fake_daemon(self, identity, refuse=None):
        """A daemon whose /identity answers the vector's raw body; /ls answers refuse when given, any other path a status."""
        calls = []

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                calls.append(self.path)
                code, headers, body = 200, {}, '{"you":"owner@fixture"}'
                if self.path == '/identity':
                    body = identity
                elif refuse and self.path.split('?')[0] == '/ls':
                    code, headers, body = refuse['status'], ({'Allow': refuse['allow']} if refuse.get('allow') else {}), refuse['body']
                data = body.encode()
                self.send_response(code)
                self.send_header('Content-Length', str(len(data)))
                for key, value in headers.items():
                    self.send_header(key, value)
                self.end_headers()
                self.wfile.write(data)

        server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
        thread.start()
        self.addCleanup(thread.join, 2)
        self.addCleanup(server.shutdown)
        self.addCleanup(server.server_close)
        bus = Bus(addr='http://127.0.0.1:' + str(server.server_port), token='fixture', timeout=2)
        self.addCleanup(bus.close)
        return bus, calls

    def test_shared_identity_vectors(self):
        for case in cases('identity'):
            with self.subTest(case=case['id']):
                bus, calls = self.fake_daemon(case['identity'])
                expect = case['expect']
                if 'fatal' in expect:
                    with self.assertRaises(BusError) as raised:
                        bus.status()
                    self.assertIn(expect['fatal'], str(raised.exception))
                    self.assertEqual(calls, ['/identity'], 'an application request followed a fatal handshake')
                elif 'warns' in expect:
                    with self.assertLogs('agent_bus', level='WARNING') as logged:
                        bus.status()
                    self.assertIn(expect['warns'], ' '.join(logged.output))
                    self.assertEqual(calls[0], '/identity')
                else:
                    with self.assertNoLogs('agent_bus', level='WARNING'):
                        bus.status()
                    self.assertEqual(calls[0], '/identity')

    def test_shared_refusal_vectors(self):
        for case in cases('refusals'):
            with self.subTest(case=case['id']):
                identity = '{"protocol":1,"protocol_min":1}' if case['identity'] == 'current' else '{}'
                bus, _ = self.fake_daemon(identity, case['daemon'])
                with self.assertRaises(Refusal) as raised:
                    with self.assertLogs('agent_bus', level='WARNING') if case['identity'] == 'old' else self.assertNoLogs('agent_bus', level='WARNING'):
                        bus.records()
                want = case['expect']
                error = raised.exception
                self.assertEqual((error.status, error.kind or '', str(error)), (want['status'], want['kind'], want['message']))


if __name__ == '__main__':
    unittest.main()
