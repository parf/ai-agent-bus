import contextlib
import datetime
import http.client
import http.server
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import socket
import threading
import time
import unittest
from unittest.mock import patch, MagicMock

spec = importlib.util.spec_from_file_location('sms', Path(__file__).with_name('sms.py'))
sms = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sms)
KEY = 'fixture-private-telnyx-key'
ID = '11111111-2222-4333-8444-555555555555'


class Provider(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        self.server.requests.append((self.path, self.headers.get('Authorization'),
                                    json.loads(self.rfile.read(int(self.headers['Content-Length'])))))
        time.sleep(self.server.delay)
        self.send_response(self.server.status)
        self.send_header('Location', '/second-send')
        self.end_headers()
        try:
            self.wfile.write(self.server.payload)
        except (BrokenPipeError, ConnectionResetError):
            pass


class Sms(unittest.TestCase):
    def setUp(self):
        self.server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Provider)
        original_connect = socket.create_connection
        def fake_only(address, *args, **kwargs):
            if address[0] != '127.0.0.1':
                raise AssertionError('SMS tests may contact only the loopback fake provider')
            return original_connect(address, *args, **kwargs)
        guard = patch('socket.create_connection', side_effect=fake_only)
        guard.start()
        self.addCleanup(guard.stop)
        env = patch.dict(os.environ, {'TELNYX_API_URL': f'http://127.0.0.1:{self.server.server_port}/v2/messages', 'AGENT_BUS_TO':'', 'AGENT_BUS_MESSAGE_ID':''})
        env.start()
        self.addCleanup(env.stop)
        self.server.requests = []
        self.server.delay, self.server.status = 0, 200
        self.server.payload = json.dumps({'data': {'id': ID}}).encode()
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': .005}, daemon=True)
        self.thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.thread.join)
        self.addCleanup(self.server.shutdown)
        self.config = {'from': '+12025550100', 'aliases': {'oncall': '+12025550142'}}

    def call(self, body, seconds=2):
        log = io.StringIO()
        deadline = datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=seconds)
        with contextlib.redirect_stderr(log):
            result, code = sms.answer({'from': '#fixture@h', 'body': body, 'deadline': deadline.isoformat()}, self.config, KEY)
        return result, code, log.getvalue()

    def test_json_and_text_acceptance_and_caller_audit(self):
        for body in [json.dumps({'to': '(202) 555-0101', 'text': 'private message'}), '+12025550101 private message']:
            result, code, log = self.call(body)
            self.assertEqual(result, {'status': 'accepted', 'to': '+12025550101', 'from': '+12025550100', 'provider_message_id': ID})
            self.assertEqual(code, 0)
            self.assertEqual(self.server.requests[-1], ('/v2/messages', 'Bearer '+KEY, {'to': '+12025550101', 'from': '+12025550100', 'text': 'private message'}))
            self.assertEqual(json.loads(log), {'caller': '#fixture@h', 'last4': '0101', 'status': 'accepted', 'id': ID})
            self.assertNotIn('private message', log)
            self.assertNotIn('+12025550101', log)

    def test_normalization_aliases_and_validation_before_send(self):
        self.assertEqual(sms.phone('oncall', self.config['aliases']), '+12025550142')
        self.assertEqual(sms.phone('1 (202) 555-0101'), '+12025550101')
        self.assertEqual(sms.phone('(202) 555-0101'), '+12025550101')
        self.assertEqual(sms.phone('+1 202 555 0102'), '+12025550102')
        for value in ['+1+2025550101', '911', 'abc', '+0123456789', '2025550101 ext2']:
            with self.assertRaises(ValueError):
                sms.phone(value)
        for body in ['oncall ', json.dumps({'to':'oncall','text':''}), json.dumps({'to':'oncall','text':'x'*1601}), json.dumps({'to':['oncall'],'text':'hello'})]:
            self.assertEqual(self.call(body)[0]['status'], 'invalid')
        self.assertEqual(self.server.requests, [])
        self.assertEqual(self.call('oncall hello')[0]['to'], '+12025550142')

    def test_timeout_is_unknown_and_exactly_one_post(self):
        self.server.delay = 1
        result, _, _ = self.call('oncall once only', seconds=.5)
        self.assertEqual(result['status'], 'unknown')
        time.sleep(.02)
        self.assertEqual(len(self.server.requests), 1, 'unknown send must never be retried')

    def test_redirect_error_and_malformed_success_never_retry_or_leak_key(self):
        for status in [302, 429, 500, 200]:
            self.server.status = status
            self.server.payload = json.dumps({'data': {'id': KEY}, 'error': KEY, 'text': 'private message'}).encode()
            before = len(self.server.requests)
            result, _, log = self.call('oncall private message')
            self.assertNotIn(KEY, json.dumps(result)+log)
            self.assertEqual(result['status'], 'failed' if status == 429 else 'unknown')
            self.assertEqual(len(self.server.requests), before+1)
            self.assertNotIn('private message', log)

    def test_uuid_shaped_key_is_not_accepted_as_provider_id(self):
        with patch.dict(globals(), {'KEY': ID}):
            result, _, log = self.call('oncall private message')
            self.assertNotIn(ID, json.dumps(result)+log)
            self.assertEqual(result['status'], 'unknown')

    def test_real_host_requires_live_runner_and_never_connects_in_tests(self):
        connect = MagicMock()
        with patch.dict(os.environ, {'TELNYX_API_URL': sms.DEFAULT_URL}), contextlib.redirect_stderr(io.StringIO()):
            result, _ = sms.answer({'from':'#fixture@h','body':'oncall never send'}, self.config, KEY, connect)
        self.assertEqual(result['status'], 'invalid')
        connect.assert_not_called()
        self.assertEqual(self.server.requests, [])

    def test_missing_key_exits_78_before_network(self):
        env = {**os.environ, 'TELNYX_API_KEY': ''}
        p = subprocess.run(['python3', str(Path(__file__).with_name('sms.py'))], input='{}', capture_output=True, text=True, env=env)
        self.assertEqual(p.returncode, 78)
        self.assertEqual(p.stdout.strip(), 'error: TELNYX_API_KEY is not set')
        self.assertEqual(self.server.requests, [])


if __name__ == '__main__':
    unittest.main()
