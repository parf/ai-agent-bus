"""Native Python adapter for the shared concrete protocol examples."""
import concurrent.futures
import json
from pathlib import Path
import threading
import time
import unittest
from unittest.mock import patch

from agent_bus import BusTimeout, CallReply
import test_client as fixture

CASES = Path(__file__).resolve().parents[3] / 'testdata/client-conformance'


def cases(name):
    rows = json.loads((CASES / (name + '.json')).read_text())
    if not isinstance(rows, list) or not rows:
        raise AssertionError('shared case file must contain concrete examples: ' + name)
    return rows


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
            actual = message.envelope.get(key, '') if key == 'from' else getattr(message, key)
            self.assertEqual(actual, value, key)

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
        request_ids = {}
        def handler(message):
            handled.append(message.body)
            request_ids[message.body] = message.id
            return answers[message.body]
        self.serve(self.worker, handler)
        for case in rows:
            with self.subTest(case=case['id']):
                request = dict(case['request'])
                result = self.caller.call(request.pop('to'), request.pop('body'), timeout=2, **request)
                self.assertIsInstance(result, CallReply)
                self.envelope(result.message, case['expected'])
                self.assertEqual(result.message.re, request_ids[case['request']['body']], 'handler answer lost its request marker')
        self.assertEqual(handled, [case['request']['body'] for case in rows])

    def test_shared_self_call(self):
        for case in cases('self-call'):
            with self.subTest(case=case['id']):
                handled = threading.Event()
                seen = []
                request_ids = []
                def handler(message):
                    seen.append(message.body)
                    request_ids.append(message.id)
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
                    self.assertEqual(answer.message.re, request_ids[0], 'self-call returned its unmarked request')
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
        progressed = threading.Event()
        original_consume = self.caller._consume
        def after_routing(**options):
            with self.caller._condition:
                if (any(p.request is not None for p in self.caller._pending.values())
                        or len(self.caller._serving.futures) > 1):
                    progressed.set()
            return original_consume(**options)
        with patch.object(self.caller, '_consume', after_routing), concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
            future = pool.submit(self.caller.call, **case['request'], timeout=3)
            try:
                self.assertTrue(progressed.wait(1), 'dispatcher did not make progress after receiving the self request')
                with self.caller._condition:
                    self.assertEqual(len(self.caller._serving.futures), 1, 'self-call bypassed worker capacity')
                    self.assertTrue(any(p.request is not None for p in self.caller._pending.values()),
                                    'self request was not retained while the worker was full')
                self.assertEqual(seen, ['hold slot'], 'self-call bypassed worker capacity')
                self.assertFalse(future.done(), 'unmarked self request completed its own call')
            finally:
                release.set()
            answer = future.result(timeout=4)
        self.envelope(answer.message, case['expected'])
        self.assertEqual(seen, ['hold slot', case['request']['body']])

    def test_self_request_arriving_before_send_returns(self):
        case = cases('self-call')[0]
        original = self.caller._request
        early = threading.Event()
        def delay_send(method, path, body=None, **options):
            response = original(method, path, body, **options)
            if path == '/send' and body.get('body') == case['request']['body']:
                until = time.monotonic() + 1
                while time.monotonic() < until:
                    with self.caller._condition:
                        if any(p.early for p in self.caller._pending.values()):
                            early.set()
                            break
                    time.sleep(0.005)
                self.assertTrue(early.is_set(), 'fixture did not deliver before the send response')
            return response
        self.serve(self.caller, lambda message: case['answer'])
        with patch.object(self.caller, '_request', delay_send):
            answer = self.caller.call(**case['request'], timeout=2)
        self.envelope(answer.message, case['expected'])
        self.assertTrue(answer.message.re)

    def test_self_call_with_an_alternate_reply_inbox(self):
        case = cases('self-call')[0]
        handled = []
        def handler(message):
            handled.append(message.id)
            return case['answer']
        self.serve(self.caller, handler)
        try:
            answer = self.caller.call(**case['request'], timeout=2,
                                      reply_to={'name': 'jobs@fixture', 'topic': 'self-return'})
        except BusTimeout:
            self.fail('self-call with alternate reply route failed to run its handler')
        self.envelope(answer.message, {**case['expected'], 'to': 'jobs@fixture', 'topic': 'self-return'})
        self.assertEqual(len(handled), 1)
        self.assertEqual(answer.message.re, handled[0])

    def test_stop_drains_an_already_taken_self_request(self):
        case = cases('self-call')[0]
        entered, release = threading.Event(), threading.Event()
        def handler(message):
            if message.body == 'hold slot':
                entered.set()
                self.assertTrue(release.wait(3))
                return 'released'
            return case['answer']
        stop, thread, serving_result = self.serve(self.caller, handler)
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
                self.assertTrue(retained, 'fixture did not take its self request before stopping')
                stop.set()
                until = time.monotonic() + 1
                while time.monotonic() < until:
                    with self.caller._condition:
                        accepting = self.caller._serving.accepting
                    if not accepting:
                        break
                    time.sleep(0.005)
                self.assertFalse(accepting, 'serve did not enter its drain')
                self.assertTrue(thread.is_alive(), 'serve finished before draining taken work')
            finally:
                release.set()
            try:
                answer = future.result(timeout=4)
            except BusTimeout:
                self.fail('stop discarded an already-taken self request')
        self.envelope(answer.message, case['expected'])
        thread.join(3)
        serving_result.result(timeout=1)
