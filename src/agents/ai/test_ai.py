import contextlib
import datetime
import http.server
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import socket
from unittest.mock import patch
import tempfile
import threading
import time
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('proxy', HERE / 'ai.py')
proxy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(proxy)
KEY = 'private-credential-for-fixtures'


class Fake(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        raw = self.rfile.read(int(self.headers['Content-Length']))
        self.server.raw_requests.append(raw)
        self.server.requests.append((self.path, self.headers['Authorization'], json.loads(raw)))
        if self.server.delay:
            time.sleep(self.server.delay)
        self.send_response(self.server.status)
        if self.server.status == 302:
            self.send_header('Location', self.server.url + '/stolen')
        self.end_headers()
        try:
            if self.server.trickle:
                for byte in self.server.payload:
                    self.wfile.write(bytes([byte]))
                    self.wfile.flush()
                    time.sleep(self.server.trickle)
            else:
                self.wfile.write(self.server.payload)
        except (BrokenPipeError, ConnectionResetError):
            pass


class Proxy(unittest.TestCase):
    def setUp(self):
        self.server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Fake)
        self.server.url = f'http://127.0.0.1:{self.server.server_port}/v1'
        self.server.requests = []
        self.server.raw_requests = []
        create_connection = socket.create_connection
        def local_only(address, *args, **kwargs):
            if address[0] != '127.0.0.1':
                raise AssertionError('non-loopback test connection refused')
            return create_connection(address, *args, **kwargs)
        guard = patch('socket.create_connection', local_only)
        guard.start()
        self.addCleanup(guard.stop)
        self.server.status, self.server.delay, self.server.trickle = 200, 0, 0
        self.server.payload = json.dumps({'choices': [{'message': {'content': 'Hello from the model'}}]}).encode()
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': .01}, daemon=True)
        self.thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.thread.join)
        self.addCleanup(self.server.shutdown)
        self.config = {'model': 'fixture/default', 'base_url': self.server.url}

    def call(self, body, deadline=None):
        log = io.StringIO()
        envelope = {'from': '#fixture@h', 'body': body}
        if deadline:
            envelope['deadline'] = deadline
        with contextlib.redirect_stderr(log):
            answer, code = proxy.answer(envelope, self.config, KEY)
        return answer, code, log.getvalue()

    def test_declared_ask_method_and_bare_forms_share_provider_handler(self):
        env = dict(os.environ, AGENT_BUS_ASK='help-json')
        help_result = subprocess.run([str(HERE / 'ai.py')], env=env, text=True, capture_output=True, check=True)
        self.assertEqual(set(json.loads(help_result.stdout)['methods']), {'ask','decide'})
        for body in ['hello', 'ask hello', json.dumps({'method':'ask', 'prompt':'hello'})]:
            self.assertEqual(self.call(body)[:2], ('Hello from the model', 0))
            self.assertEqual(self.server.requests[-1][2], {'model':'fixture/default','messages':[{'role':'user','content':'hello'}]})
        request = {'messages':[{'role':'user','content':'hello'}], 'temperature': .2}
        for body in [request, {'method':'ask','prompt':request}]:
            self.assertEqual(json.loads(self.call(json.dumps(body))[0]), json.loads(self.server.payload))
            self.assertEqual(self.server.requests[-1][2], request | {'model':'fixture/default'})
        self.assertEqual(len(self.server.requests), 5)

    def test_english_decide_prompts_remain_chat(self):
        body = 'decide whether tea beats coffee'
        self.assertEqual(self.call(body)[:2], ('Hello from the model', 0))
        self.assertEqual(self.server.requests[-1], ('/v1/chat/completions', 'Bearer '+KEY,
            {'model':'fixture/default','messages':[{'role':'user','content':body}]}))

    def test_model_log_supports_presets_and_variants(self):
        for model in ['@preset/demo', 'provider/model+variant']:
            for body in [json.dumps({'model':model,'messages':[]}), 'decide '+json.dumps({'model':model})]:
                self.assertEqual(json.loads(self.call(body)[2])['model'],model)

    def test_decide_exact_raw_bytes_fixed_path_and_no_default_injection(self):
        raw = ' { "model":"typesafe/jev-1.14", "duplicate":1, "duplicate":2, "state":"café \\u0061", "n":1e-7, "big":123456789012345678901234567890, "questions":{} }  '
        self.config['base_url'] += '/discard/this/path'
        for body in ['decide '+raw, json.dumps({'method':'decide','body':raw})]:
            answer, code, _ = self.call(body)
            self.assertEqual(code, 0)
            self.assertEqual(self.server.raw_requests[-1], raw.encode())
            self.assertEqual(self.server.requests[-1][:2], ('/api/alpha/decisions', 'Bearer '+KEY))
            self.assertEqual(json.loads(answer), json.loads(self.server.payload))
        # A missing model is sent unchanged so the provider can refuse it.
        self.server.status, self.server.payload = 400, b'{"error":{"message":"model missing"}}'
        answer, _, _ = self.call('decide { "questions":{} }')
        self.assertEqual(self.server.raw_requests[-1], b'{ "questions":{} }')
        self.assertEqual(answer, 'error: 400 model missing')

    def test_decide_strict_shape_and_unknown_methods_keep_chat_behavior(self):
        for value in [{'method':'decide','body':{}}, {'method':'decide','body':'{}','extra':1},
                      {'method':'decide'}, {'method':'decide','body':'[]'}]:
            self.assertTrue(self.call(json.dumps(value))[0].startswith('error:'))
        for raw in ['{"stream":true}','{"x":NaN}','{"x":Infinity}','{"x":-Infinity}']:
            self.assertTrue(self.call('decide '+raw)[0].startswith('error:'))
        self.assertEqual(self.server.requests, [])
        for request in [{'method':'future','messages':[]}, {'method':'decide','body':'{}','model':'fixture/explicit'}]:
            body = json.dumps(request) if request['method']=='future' else json.dumps({'method':'ask','prompt':request})
            self.call(body)
            self.assertEqual(self.server.requests[-1], ('/v1/chat/completions', 'Bearer '+KEY,
                request if 'model' in request else request | {'model':'fixture/default'}))

    def test_decide_returns_every_probability_as_json_values(self):
        raw = b'{"answers":{"labels":{"type":"choice","probabilities":{"zero":0,"one":1,"fraction":0.3,"tiny":1e-7,"long":0.12345678901234567890}}},"usage":{"cost":0}}'
        self.server.payload = raw
        answer, code, _ = self.call('decide {"model":"typesafe/jev-1.14"}')
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(answer), json.loads(raw))
        self.assertEqual(set(json.loads(answer)['answers']['labels']['probabilities']), {'zero','one','fraction','tiny','long'})
        for bad in [b'{"p":NaN}',b'{"p":Infinity}',b'{"p":1e999}']:
            self.server.payload = bad
            self.assertTrue(self.call('decide {}')[0].startswith('error:'))

    def test_decide_errors_and_key_never_appear_in_answer_or_log(self):
        encoded = ''.join('\\u%04x' % ord(c) for c in KEY)
        for status, payload in [(429, ('{"error":{"message":"quota '+encoded+'"}}').encode()),
                                (200, ('{"'+KEY+'":"'+encoded+'"}').encode()),
                                (400, ('raw '+KEY).encode())]:
            self.server.status, self.server.payload = status, payload
            answer, code, log = self.call('decide '+json.dumps({'model':KEY,'state':'sensitive prompt'}))
            self.assertEqual(code,0)
            self.assertNotIn(KEY, answer+log)
            self.assertNotIn('sensitive prompt',log)
            if status>=300: self.assertTrue(answer.startswith(f'error: {status} '))

    def test_model_logging_is_bounded_without_changing_chat_or_decide_payload(self):
        for model in ['secret prompt with spaces', 'x'*129, {'untrusted':'object'}]:
            for body in [json.dumps({'messages':[],'model':model}), 'decide '+json.dumps({'model':model})]:
                answer, code, log = self.call(body)
                self.assertEqual(json.loads(log)['model'], 'invalid')
                self.assertLess(len(log),256)
                if isinstance(model,str): self.assertNotIn(model,log)

    def test_decide_caps_deadline_single_attempt_and_no_redirect(self):
        self.server.status = 302
        self.assertTrue(self.call('decide {}')[0].startswith('error: 302 '))
        self.assertEqual(len(self.server.requests),1)
        self.server.status = 200
        self.server.trickle, self.server.payload = .04, b'{"p":0.5}'
        deadline = (datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=.2)).isoformat()
        self.assertEqual(self.call('decide {}',deadline)[0], 'error: provider request timed out')
        self.assertEqual(len(self.server.requests),2)
        self.server.trickle = 0
        self.server.payload = b'{}'+b' '*(proxy.MAX_BYTES-1)
        self.assertEqual(self.call('decide {}')[0], 'error: provider answer exceeds 1048576 bytes')
        before=len(self.server.requests)
        self.assertTrue(self.call('decide '+('x'*(proxy.MAX_BYTES+1)))[0].startswith('error:'))
        self.assertEqual(len(self.server.requests),before)

    def test_json_pass_through_and_default_model(self):
        for model in [None, 'fixture/override']:
            request = {'messages': [{'role': 'user', 'content': 'sensitive prompt'}], 'temperature': .12,
                       'tools': [{'type': 'function', 'function': {'name': 'demo'}}], 'unknown_extension': [1, 2]}
            if model:
                request['model'] = model
            answer, code, log = self.call(json.dumps(request))
            expected = request | {'model': model or 'fixture/default'}
            self.assertEqual(self.server.requests[-1], ('/v1/chat/completions', 'Bearer ' + KEY, expected))
            self.assertEqual(json.loads(answer), json.loads(self.server.payload))
            self.assertEqual(self.server.raw_requests[-1], json.dumps(expected, ensure_ascii=False).encode())
            self.assertEqual(answer.encode(), json.dumps(json.loads(self.server.payload), ensure_ascii=False).encode())
            self.assertEqual(code, 0)
            self.assertEqual(json.loads(log)['caller'], '#fixture@h')
            self.assertNotIn('sensitive prompt', log)
            self.assertNotIn('Hello from the model', log)
            self.assertEqual(set(json.loads(log)), {'caller', 'model', 'request_bytes', 'answer_bytes', 'status'})

    def test_text_is_one_user_message_and_assistant_text_only(self):
        answer, code, _ = self.call('What is two plus two?')
        self.assertEqual(answer, 'Hello from the model')
        self.assertEqual(code, 0)
        self.assertEqual(self.server.requests[0][2], {'model': 'fixture/default', 'messages': [
            {'role': 'user', 'content': 'What is two plus two?'}]})

    def test_no_key_exits_78_without_contacting_provider(self):
        env = os.environ.copy()
        env.pop('OPENROUTER_API_KEY', None)
        env['AGENT_BUS_CONFIG'] = json.dumps(self.config)
        result = subprocess.run([str(HERE / 'ai.py')], input='{"body":"hello"}', text=True,
                                capture_output=True, env=env, timeout=2)
        self.assertEqual(result.returncode, 78)
        self.assertEqual(result.stdout.strip(), 'error: OPENROUTER_API_KEY is not set')
        self.assertEqual(self.server.requests, [])

    def test_provider_error_and_key_never_appears_in_answers_or_logs(self):
        self.config['model'] = 'fixture/' + KEY
        for status, payload in [(429, {'error': {'message': 'no quota ' + KEY}}),
                                (200, {'choices': [{'message': {'content': KEY}}], KEY: KEY}),
                                (400, 'raw failure ' + KEY)]:
            self.server.status = status
            self.server.payload = (json.dumps(payload).encode() if isinstance(payload, dict) else payload.encode())
            for body in ['hello', '{"messages":[]}']:
                answer, _, log = self.call(body)
                self.assertNotIn(KEY, answer + log)
                if status >= 300:
                    self.assertTrue(answer.startswith(f'error: {status} '), answer)
                self.assertNotIn('no quota', log)
        # A provider can encode the credential as JSON escapes too.
        self.server.status = 200
        encoded = ''.join('\\u%04x' % ord(c) for c in KEY)
        self.server.payload = ('{"choices":[{"message":{"content":"' + encoded + '"}}]}').encode()
        answer, _, log = self.call('hello')
        self.assertNotIn(KEY, answer + log)
        self.assertIn('***', answer)

    def test_redirect_is_not_followed(self):
        self.server.status = 302
        answer, _, _ = self.call('hello')
        self.assertTrue(answer.startswith('error: 302 '))
        self.assertEqual(len(self.server.requests), 1)

    def test_deadline_and_size_bounds(self):
        self.server.delay = .3
        deadline = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=.2)).isoformat()
        start = time.monotonic()
        answer, _, _ = self.call('hello', deadline)
        self.assertEqual(answer, 'error: provider request timed out')
        self.assertLess(time.monotonic() - start, .25)
        self.server.delay = 0
        small = b'{"choices":[{"message":{"content":"ok"}}]}'
        self.server.payload = small + b' ' * (proxy.MAX_BYTES + 1 - len(small))
        answer, _, _ = self.call('hello')
        self.assertEqual(answer, 'error: provider answer exceeds 1048576 bytes')
        before = len(self.server.requests)
        answer, _, _ = self.call('x' * (proxy.MAX_BYTES + 1))
        self.assertTrue(answer.startswith('error:'))
        self.assertEqual(len(self.server.requests), before)

    def test_deadline_covers_trickling_answer(self):
        self.server.trickle = .04
        self.server.payload = b'{"ok":true}'
        deadline = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=.2)).isoformat()
        start = time.monotonic()
        answer, _, _ = self.call('{"messages":[]}', deadline)
        self.assertEqual(answer, 'error: provider request timed out')
        self.assertLess(time.monotonic() - start, .25)

    def test_package_runs_without_checkout(self):
        scratch = HERE.parents[2] / 'tmp'
        scratch.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=scratch) as directory:
            package = Path(directory) / 'ai'
            subprocess.run([str(HERE / 'package.sh'), str(package)], check=True, capture_output=True)
            result = subprocess.run([str(package / 'ai.py')], input=json.dumps({'body': 'hello', 'deadline': '0001-01-01T00:00:00Z'}),
                                    text=True, capture_output=True, timeout=2, cwd=directory,
                                    env={'PATH': os.environ['PATH'], 'OPENROUTER_API_KEY': KEY,
                                         'AGENT_BUS_CONFIG': json.dumps(self.config)})
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), 'Hello from the model')
            self.assertNotIn(KEY, result.stdout + result.stderr)
            raw = '{ "model":"typesafe/jev-1.14", "questions":{} } '
            result = subprocess.run([str(package / 'ai.py')], input=json.dumps({'body':'decide '+raw}),
                                    text=True,capture_output=True,timeout=2,cwd=directory,
                                    env={'PATH':os.environ['PATH'],'OPENROUTER_API_KEY':KEY,
                                         'AGENT_BUS_CONFIG':json.dumps(self.config)})
            self.assertEqual(result.returncode,0,result.stderr)
            self.assertEqual(self.server.raw_requests[-1],raw.encode())
            self.assertEqual(self.server.requests[-1][0],'/api/alpha/decisions')
            self.assertEqual(json.loads(result.stdout),json.loads(self.server.payload))
            manifest = json.loads((package / 'agent.json').read_text())
            self.assertEqual(manifest['form'], 'json')
            self.assertNotIn('serves', manifest)
            self.assertTrue(manifest['needs']['network'])
            self.assertEqual(set(manifest['config']), {'model', 'base_url'})


if __name__ == '__main__':
    unittest.main()

class AskDispatch(unittest.TestCase):
    def test_asks_never_reach_provider_or_normal_body(self):
        from unittest.mock import patch
        import sys
        program = HERE / ("ai.py" if HERE.name == "ai" else "sms.py")
        for ask in ['help','help-config','help-secret','help-json','mcp','status','not-a-known-ask']:
            process = subprocess.run([sys.executable,str(program)],input='invalid normal input',text=True,capture_output=True,
                env={**os.environ,'AGENT_BUS_ASK':ask,'AGENT_BUS_CONFIG':'not json','OPENROUTER_API_KEY':'','TELNYX_API_KEY':'','TELNYX_API_URL':'http://127.0.0.1:1'},timeout=3)
            self.assertEqual(process.returncode,0,process.stderr)
            if ask in ('status','not-a-known-ask'): self.assertEqual(process.stdout.strip(),'unsupported: '+ask)
            else:
                self.assertTrue(process.stdout.strip())
                self.assertNotIn('invalid normal input',process.stdout)
                if ask=='help-json': self.assertIn('methods',json.loads(process.stdout))
