"""Native Python adapter for the shared concrete protocol examples."""
import concurrent.futures
import json
from pathlib import Path
import threading
import time
import unittest

from agent_bus import CallReply
import test_client as fixture

CASES = Path(__file__).resolve().parents[3] / 'testdata/client-conformance'


def cases(name):
    return json.loads((CASES / (name + '.json')).read_text())


class SharedConformance(unittest.TestCase):
    # Reuse the disposable real daemon lifecycle, without inheriting and
    # rerunning all of LiveClient's independent language-specific tests.
    setUp = fixture.LiveClient.setUp
    tearDown = fixture.LiveClient.tearDown
    bus = fixture.LiveClient.bus
    serve = fixture.LiveClient.serve

    def envelope(self, message, expected):
        self.assertIsNotNone(message, 'accepted request was not delivered')
        for key, value in expected.items():
            self.assertEqual(message.envelope.get(key, ''), value, key)

    def test_shared_send_cases(self):
        for case in cases('send'):
            with self.subTest(case=case['id']):
                request = dict(case['request'])
                sent = self.caller.send(request.pop('to'), request.pop('body'), **request)
                message = self.worker.consume(wait=0)
                self.envelope(message, case['expected'])
                self.assertEqual(message.id, sent)

    def test_shared_call_cases(self):
        rows = cases('call')
        answers = {case['request']['body']: case['answer'] for case in rows}
        handled = []
        def handler(message):
            handled.append(message.body)
            return answers[message.body]
        self.serve(self.worker, handler)
        for case in rows:
            with self.subTest(case=case['id']):
                request = dict(case['request'])
                result = self.caller.call(request.pop('to'), request.pop('body'), timeout=2, **request)
                self.assertIsInstance(result, CallReply)
                self.envelope(result.message, case['expected'])
                self.assertTrue(result.message.re, 'handler answer lost its reply marker')
        self.assertEqual(handled, [case['request']['body'] for case in rows])

    def test_shared_self_call(self):
        for case in cases('self-call'):
            with self.subTest(case=case['id']):
                handled = threading.Event()
                seen = []
                def handler(message):
                    seen.append(message.body)
                    handled.set()
                    return case['answer']
                stop, thread, result = self.serve(self.caller, handler)
                try:
                    request = dict(case['request'])
                    answer = self.caller.call(request.pop('to'), request.pop('body'), timeout=2, **request)
                    self.assertIsInstance(answer, CallReply)
                    self.envelope(answer.message, case['expected'])
                    self.assertTrue(handled.wait(0.5), 'self-call returned without running its handler')
                    self.assertEqual(seen, [case['request']['body']])
                    self.assertTrue(answer.message.re, 'self-call returned its unmarked request')
                finally:
                    stop.set()
                    thread.join(3)
                    result.result(timeout=1)

    def test_self_call_waits_for_a_free_worker(self):
        case = cases('self-call')[0]
        entered, release = threading.Event(), threading.Event()
        seen = []
        def handler(message):
            seen.append(message.body)
            if message.body == 'hold slot':
                entered.set()
                self.assertTrue(release.wait(3))
                return 'released'
            return case['answer']
        self.serve(self.caller, handler, workers=1)
        self.owner.send(self.caller.name, 'hold slot')
        self.assertTrue(entered.wait(1))
        with concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
            future = pool.submit(self.caller.call, **case['request'], timeout=3)
            try:
                until = time.monotonic() + 1
                retained = False
                while time.monotonic() < until:
                    with self.caller._condition:
                        retained = any(p.request is not None for p in self.caller._pending.values())
                    if retained:
                        break
                    time.sleep(0.005)
                self.assertTrue(retained, 'self request was not retained while the worker was full')
                self.assertEqual(seen, ['hold slot'], 'self-call bypassed worker capacity')
                self.assertFalse(future.done(), 'unmarked self request completed its own call')
            finally:
                release.set()
            answer = future.result(timeout=4)
        self.envelope(answer.message, case['expected'])
        self.assertEqual(seen, ['hold slot', case['request']['body']])
