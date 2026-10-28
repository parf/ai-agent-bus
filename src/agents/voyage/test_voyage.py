import contextlib
import datetime
import http.server
import importlib.util
import io
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('voyage', HERE / 'voyage.py')
proxy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(proxy)
KEY = 'fixture-private-voyage-key'


class Fake(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        self.server.requests.append((self.path, self.headers.get('Authorization'),
                                     self.rfile.read(int(self.headers['Content-Length']))))
        if self.server.delay:
            time.sleep(self.server.delay)
        self.send_response(self.server.status)
        self.send_header('Location', self.server.url + '/stolen')
        self.end_headers()
        try:
            for chunk in ([bytes([b]) for b in self.server.payload] if self.server.trickle else [self.server.payload]):
                self.wfile.write(chunk)
                self.wfile.flush()
                if self.server.trickle:
                    time.sleep(self.server.trickle)
        except (BrokenPipeError, ConnectionResetError):
            pass


class Voyage(unittest.TestCase):
    def setUp(self):
        self.server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Fake)
        self.server.url = f'http://127.0.0.1:{self.server.server_port}'
        self.server.requests = []
        self.server.status, self.server.delay, self.server.trickle = 200, 0, 0
        self.server.payload = b'{"data":[{"embedding":[0.1,0.2]}]}'
        thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval':.01}, daemon=True)
        thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(thread.join)
        self.addCleanup(self.server.shutdown)
        origin = patch.object(proxy, 'ORIGIN', self.server.url)
        origin.start()
        self.addCleanup(origin.stop)
        # Fail closed: even if an origin override regresses, no test can use a provider.
        create_connection = socket.create_connection
        def local_only(address, *args, **kwargs):
            if address[0] != '127.0.0.1':
                raise AssertionError('non-loopback test connection refused')
            return create_connection(address, *args, **kwargs)
        guard = patch('socket.create_connection', local_only)
        guard.start()
        self.addCleanup(guard.stop)

    def call(self, body, config=None, deadline=None):
        log = io.StringIO()
        with contextlib.redirect_stderr(log):
            answer, code = proxy.answer({'body':body, 'deadline':deadline}, config or {}, KEY)
        return answer, code, log.getvalue()

    def test_embed_defaults_and_optional_arguments(self):
        for body, expected in [
            ('embed tea', {'input':'tea','model':'voyage-3.5'}),
            ('tea', {'input':'tea','model':'voyage-3.5'}),
            (json.dumps({'method':'embed','text':'tea','model':'custom','input_type':'query'}),
             {'input':'tea','model':'custom','input_type':'query'}),
        ]:
            answer, code, log = self.call(body)
            path, auth, payload = self.server.requests[-1]
            self.assertEqual((path, auth), ('/v1/embeddings', 'Bearer '+KEY))
            self.assertEqual(json.loads(payload), expected)
            self.assertEqual(json.loads(answer), json.loads(self.server.payload))
            self.assertEqual(code, 0)
            self.assertEqual(set(json.loads(log)), {'model','request_bytes','answer_bytes','status'})
            self.assertNotIn('tea', log)
        self.call('embed tea', {'model':'configured'})
        self.assertEqual(json.loads(self.server.requests[-1][2])['model'], 'configured')

    def test_pass_through_exact_bytes_path_and_auth(self):
        raw = '{ "model" : "fixture", "unknown" : [1,2], "input":"café" }  '
        for path in ['/v1/rerank', '/v1/multimodalembeddings', '/v1/contextualizedembeddings']:
            for body in ['request '+path+' '+raw, json.dumps({'method':'request','path':path,'body':raw})]:
                self.call(body)
                self.assertEqual(self.server.requests[-1], (path, 'Bearer '+KEY, raw.encode()))

    def test_errors_429_and_escaped_keys_never_leak(self):
        escaped = ''.join('\\u%04x'%ord(c) for c in KEY)
        for status, payload in [(429, b'{"error":{"message":"quota"}}'),
            (400, ('bad '+KEY).encode()), (200, ('{"'+KEY+'":"'+escaped+'"}').encode()),
            (400, ('{"error":{"message":"'+escaped+'"}}').encode())]:
            self.server.status, self.server.payload = status, payload
            answer, code, log = self.call('embed private prompt', {'model':KEY})
            self.assertNotIn(KEY, answer + log)
            self.assertNotIn('private prompt', log)
            self.assertNotIn('quota', log)
            self.assertEqual(code, 0)
            if status >= 300:
                self.assertTrue(answer.startswith(f'error: {status} '), answer)

    def test_model_is_a_bounded_id_not_prompt_log_content(self):
        for model in ['private prompt', 'x'*129, '<script>', 'line\nother']:
            answer, _, log = self.call(json.dumps({'method':'embed','text':'tea','model':model}))
            self.assertTrue(answer.startswith('error:'))
            self.assertNotIn(model, log)
            self.assertLess(len(log), 200)
        self.assertEqual(self.server.requests, [])

    def test_paths_and_streaming_refused_before_post(self):
        for path in ['/v2/embed','/v1/../admin','/v1/%2e%2e/admin','//evil/v1/a','https://evil/v1/a','/v1/a?next=x','/v1/a\\b']:
            self.assertTrue(self.call('request '+path+' {}')[0].startswith('error:'))
        self.assertTrue(self.call('request /v1/embed {"stream":true}')[0].startswith('error:'))
        self.assertEqual(self.server.requests, [])

    def test_no_retry_or_redirect(self):
        self.server.status = 302
        self.assertTrue(self.call('embed tea')[0].startswith('error: 302 '))
        self.assertEqual(len(self.server.requests), 1)
        self.server.delay = .3
        deadline = (datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=.2)).isoformat()
        start = time.monotonic()
        self.assertEqual(self.call('embed tea', deadline=deadline)[0], 'error: provider request timed out')
        self.assertLess(time.monotonic()-start, .5)
        self.assertEqual(len(self.server.requests), 2)

    def test_request_answer_and_trickle_bounds(self):
        self.assertTrue(self.call('x'*(proxy.MAX_BYTES+1))[0].startswith('error:'))
        self.assertEqual(self.server.requests, [])
        self.server.payload = b'{}'+b' '*(proxy.MAX_BYTES-1)
        self.assertEqual(self.call('embed tea')[0], 'error: provider answer exceeds 1048576 bytes')
        self.server.payload, self.server.trickle = b'{"ok":true}', .04
        deadline = (datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=.2)).isoformat()
        self.assertEqual(self.call('embed tea', deadline=deadline)[0], 'error: provider request timed out')

    def test_metadata_missing_key_and_exported_package(self):
        with tempfile.TemporaryDirectory() as directory:
            package = Path(directory)/'voyage'
            subprocess.run([str(HERE/'package.sh'), str(package)],check=True,capture_output=True)
            self.assertTrue((package/'support/agents/shared/provider_http.py').is_file())
            for ask in ['help','help-json','help-config','help-secret','mcp','status','other']:
                result = subprocess.run([str(package/'voyage.py')],input='not a message',text=True,capture_output=True,
                    cwd=directory, env={'PATH':os.environ['PATH'],'AGENT_BUS_ASK':ask},timeout=2)
                self.assertEqual(result.returncode,0,result.stderr)
                if ask in ('status','other'):
                    self.assertEqual(result.stdout.strip(),'unsupported: '+ask)
                elif ask == 'help-json':
                    self.assertEqual(set(json.loads(result.stdout)['methods']), {'embed','request'})
            result = subprocess.run([str(package/'voyage.py')],input='{}',text=True,capture_output=True,
                cwd=directory,env={'PATH':os.environ['PATH']},timeout=2)
            self.assertEqual(result.returncode,78)
            self.assertEqual(result.stdout.strip(),'error: VOYAGE_API_KEY is not set')
            self.assertEqual(self.server.requests, [])
