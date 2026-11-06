import base64
import contextlib
import datetime
import http.server
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import signal
import socket
from unittest.mock import Mock, patch
import tempfile
import threading
import time
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('proxy', HERE / 'ai.py')
proxy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(proxy)
KEY = 'private-credential-for-fixtures'
from agent_bus.message import Message
from agent_bus.errors import Refusal, TransportError
PNG = b'\x89PNG\r\n\x1a\n' + b'fixture-png-bytes'
JPEG = b'\xff\xd8\xff' + b'fixture-jpeg'


class Bigfile:
    """#ai's bigfile record in memory: list and put as the client library answers them."""
    def __init__(self, rows=(), refuse=None, puts_allowed=None):
        self.rows = [dict(row) for row in rows]
        self.puts, self.lists = [], 0
        self.refuse, self.puts_allowed = refuse, puts_allowed

    def list(self):
        self.lists += 1
        if self.refuse:
            raise self.refuse
        return [dict(row) for row in self.rows]

    def put(self, name, data, *, ttl):
        if self.puts_allowed is not None and len(self.puts) >= self.puts_allowed:
            raise Refusal(413, 'bigfile limit reached')
        self.puts.append((name, bytes(data), ttl))
        self.rows.append({'name': name, 'size': len(data)})


class Record:
    def __init__(self, bigfile=None):
        self.name = '#ai@t'
        self.bigfile = bigfile or Bigfile()


class FakeBus:
    """Hands out queued envelopes as real Messages, keeps every reply, and sends itself SIGTERM once the queue is empty."""
    def __init__(self, envelopes, store=None):
        self.name = '#ai@t'
        self.store = store or Record()
        self.queue = [Message.received(envelope, self) for envelope in envelopes]
        self.replies, self.consumed, self.dones = [], [], []

    def record(self, name):
        assert name == self.name
        return self.store

    def consume(self, *, wait, share):
        self.consumed.append((wait, share))
        if not self.queue:
            os.kill(os.getpid(), signal.SIGTERM)
            return None
        return self.queue.pop(0)

    def reply(self, message, body):
        self.replies.append((message.envelope.get('message_id'), body))

    def done(self, message):
        self.dones.append(message.envelope.get('message_id'))


class Fake(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_GET(self):
        self.server.requests.append((self.path, self.headers['Authorization'], None))
        self.reply(*self.server.routes.get(self.path, (self.server.status, self.server.payload)))

    def reply(self, status, payload):
        self.send_response(status)
        self.end_headers()
        self.wfile.write(payload)

    def do_POST(self):
        raw = self.rfile.read(int(self.headers['Content-Length']))
        self.server.raw_requests.append(raw)
        self.server.requests.append((self.path, self.headers['Authorization'], json.loads(raw)))
        if self.path in self.server.routes:
            return self.reply(*self.server.routes[self.path])
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
        self.server.routes = {}
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
        self.store = Record()

    def call(self, body, deadline=None, roles=(), work=None, **route):
        """One message through the kept process's entry: roles come from the envelope the daemon wrote."""
        log = io.StringIO()
        envelope = {'from': '#fixture@h', 'body': body, 'roles': list(roles), **route}
        if deadline:
            envelope['deadline'] = deadline
        with contextlib.redirect_stderr(log):
            answer = proxy.handle(Message.received(envelope, None), self.config, KEY, self.store, work)
        return answer, 0, log.getvalue()

    def route(self, path, payload, status=200):
        self.server.routes['/v1' + path] = (status, json.dumps(payload).encode() if not isinstance(payload, bytes) else payload)

    def work(self):
        scratch = HERE.parents[2] / 'tmp'
        scratch.mkdir(exist_ok=True)
        directory = tempfile.TemporaryDirectory(dir=scratch, prefix='ai-work-')
        self.addCleanup(directory.cleanup)
        return directory.name

    def test_declared_ask_method_and_bare_forms_share_provider_handler(self):
        env = dict(os.environ, AGENT_BUS_ASK='help-json')
        help_result = subprocess.run([str(HERE / 'ai.py')], env=env, text=True, capture_output=True, check=True)
        self.assertEqual(set(json.loads(help_result.stdout)['methods']), {'ask','decide','models','model','embed'})
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

    def test_english_image_prompts_remain_chat_for_every_caller(self):
        self.config['image_model'] = 'openai/gpt-image-2'
        body = 'image recognition models are getting better; summarise why'
        for roles in [(), ('owner',), ('maintainer',)]:
            self.assertEqual(self.call(body, roles=roles)[:2], ('Hello from the model', 0))
            self.assertEqual(self.server.requests[-1], ('/v1/chat/completions', 'Bearer '+KEY,
                {'model':'fixture/default','messages':[{'role':'user','content':body}]}))
        # The method is a JSON object, after "image " or as {"method":"image"}.
        self.route('/images', self.IMAGES)
        for request in ['image {"prompt":"a red panda"}', '{"method":"image","prompt":"a red panda"}']:
            self.assertRegex(self.call(request, roles=('owner',))[0], r'^#ai@t/image-[0-9a-f]{16}\.png ')
            self.assertEqual(self.call(request)[0], 'error: image needs the owner or maintainer role')
        self.assertTrue(self.call('image {"prompt":', roles=('owner',))[0].startswith('error:'))
        self.assertEqual([r[0] for r in self.server.requests], ['/v1/chat/completions'] * 3 + ['/v1/images'] * 2)

    def test_english_embed_prompts_remain_chat(self):
        self.config['embed_model'] = 'fixture/embed'
        for body in ['embed this video in my page', 'embed hello', 'embed 3 photos in a grid']:
            self.assertEqual(self.call(body)[:2], ('Hello from the model', 0))
            self.assertEqual(self.server.requests[-1], ('/v1/chat/completions', 'Bearer '+KEY,
                {'model':'fixture/default','messages':[{'role':'user','content':body}]}))
        self.route('/embeddings', {'data': [{'index': 0, 'embedding': [0.5]}]})
        for body, sent in [('embed "a text"', 'a text'), ('embed ["a text"]', ['a text']),
                           ('embed {"input":"a text","model":"fixture/other"}', 'a text')]:
            self.assertEqual(json.loads(self.call(body)[0])['vectors'], [[0.5]])
            self.assertEqual(self.server.requests[-1][2]['input'], sent)
        self.assertEqual(self.server.requests[-1][2]['model'], 'fixture/other')
        for bad in ['embed "unterminated', 'embed {"text":"x"}', 'embed {"input":"x","extra":1}']:
            self.assertTrue(self.call(bad)[0].startswith('error:'), bad)
        self.assertEqual(len(self.server.requests), 6)

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
        # Before any bus connection: the address below would refuse it, and the exit would be 1.
        env.update(AGENT_BUS_ADDR='http://127.0.0.1:1', AGENT_BUS_NAME='#ai@t', AGENT_BUS_TOKEN='fixture-only')
        result = subprocess.run([str(HERE / 'ai.py')], text=True, capture_output=True, env=env, timeout=5)
        self.assertEqual((result.returncode, result.stdout), (78, ''))
        self.assertEqual(result.stderr.strip(), 'OPENROUTER_API_KEY is not set')
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
            env = {'PATH': os.environ['PATH'], 'OPENROUTER_API_KEY': KEY, 'AGENT_BUS_CONFIG': json.dumps(self.config),
                   'AGENT_BUS_ADDR': 'http://127.0.0.1:1', 'AGENT_BUS_NAME': '#ai@t', 'AGENT_BUS_TOKEN': 'fixture-only'}
            # The packaged program finds its client library: it gets as far as the bus, which is not there.
            result = subprocess.run([str(package / 'ai.py')], text=True, capture_output=True, timeout=10, cwd=directory, env=env)
            self.assertEqual((result.returncode, result.stdout), (1, ''), result.stderr)
            self.assertIn('ai: bus unavailable', result.stderr)
            self.assertNotIn(KEY, result.stderr)
            result = subprocess.run([str(package / 'ai.py')], text=True, capture_output=True, timeout=5, cwd=directory,
                                    env={**env, 'AGENT_BUS_ASK': 'help-json'})
            self.assertEqual(set(json.loads(result.stdout)['methods']), {'ask', 'decide', 'models', 'model', 'embed'})
            self.assertTrue((package / 'support/clients/python/agent_bus/client.py').is_file())
            self.assertTrue((package / 'support/clients/LICENSE.md').is_file())
            manifest = json.loads((package / 'agent.json').read_text())
            self.assertEqual(manifest['form'], 'json')
            self.assertEqual((manifest['serves'], manifest['processes']), ('itself', 2))
            self.assertTrue(manifest['needs']['network'])
            self.assertEqual(set(manifest['config']), {'model', 'embed_model', 'image_model', 'base_url'})

    # Recorded provider shapes (OpenRouter API reference, 2026-11): replayed locally, never fetched live.
    MODELS = {'data': [{'id': 'openai/gpt-4.1-mini', 'canonical_slug': 'openai/gpt-4.1-mini-2025-04-14', 'name': 'GPT-4.1 Mini', 'context_length': 1047576,
                        'architecture': {'input_modalities': ['text', 'image']}, 'pricing': {'prompt': '0.0000004', 'completion': '0.0000016'}},
                       {'id': 'fixture/default', 'name': 'Default', 'context_length': 8192,
                        'architecture': {'input_modalities': ['text']}, 'pricing': {'prompt': '0.000003', 'completion': '0.000015'}},
                       {'id': 'google/gemma-4-31b-it:free', 'name': 'Gemma 4 31B (free)', 'context_length': 262144,
                        'architecture': {'input_modalities': ['text']}, 'pricing': {'prompt': '0', 'completion': '0'}},
                       {'id': 'fixture/zero', 'name': 'Zero', 'context_length': 4096,
                        'architecture': {'input_modalities': ['text']}, 'pricing': {'prompt': '0', 'completion': '0.0'}},
                       {'id': 'openrouter/auto', 'name': 'Auto Router', 'context_length': 2000000, 'pricing': {'prompt': '-1', 'completion': '-1'}}]}
    TABLE = ['  id                          context  $/M in  $/M out  input',
             '  openai/gpt-4.1-mini         1047576  0.4     1.6      text,image',
             '* fixture/default             8192     3       15       text',
             '  google/gemma-4-31b-it:free  262144   0       0        text',
             '  fixture/zero                4096     0       0        text',
             '  openrouter/auto             2000000  varies  varies   -']
    ENDPOINTS = {'data': {'id': 'openai/gpt-4', 'name': 'GPT-4', 'architecture': {'input_modalities': ['text', 'image']},
                          'endpoints': [{'name': 'OpenAI: GPT-4', 'provider_name': 'OpenAI', 'context_length': 8192, 'max_completion_tokens': 4096,
                                         'pricing': {'prompt': '0.00003', 'completion': '0.00006', 'request': '0', 'image': '0'}, 'uptime_last_1d': 99.8},
                                        {'name': 'Azure: GPT-4', 'provider_name': 'Azure', 'context_length': 32768, 'max_completion_tokens': 8192,
                                         'pricing': {'prompt': '0.00006', 'completion': '0.00012', 'request': '0', 'image': '0'}, 'uptime_last_1d': 97.1}]}}
    CREDITS = {'data': {'total_credits': 20, 'total_usage': 7.6544}}
    KEY_INFO = {'data': {'label': 'sk-or-v1-abc...xyz', 'limit': 50, 'limit_remaining': 42.5, 'limit_reset': 'monthly', 'usage': 7.5,
                         'is_free_tier': False, 'rate_limit': {'requests': 230, 'interval': '10s'},
                         'free_model_daily_requests': {'used': 1, 'limit': 1000, 'remaining': 999}, 'include_byok_in_limit': False}}

    def test_models_lists_the_provider_catalogue_with_the_default_marked(self):
        self.route('/models', self.MODELS)
        for body in ['models', ' models\n', '{"method":"models"}']:
            answer, code, _ = self.call(body)
            self.assertEqual((answer, code), ('\n'.join(self.TABLE + ['default: fixture/default']), 0))
            self.assertEqual(self.server.requests[-1], ('/v1/models', 'Bearer ' + KEY, None))
        self.assertEqual(len(self.server.requests), 3)

    def test_models_filters_by_word_and_free(self):
        self.route('/models', self.MODELS)
        cells = lambda lines: [line.split() for line in lines]
        for body in ['models GPT', '{"method":"models","filter":"gpt"}']:
            self.assertEqual(cells(self.call(body)[0].split('\n')), cells([self.TABLE[0], self.TABLE[1], 'default: fixture/default']))
        for body in ['models free', '{"method":"models","filter":"free"}']:
            self.assertEqual(cells(self.call(body)[0].split('\n')), cells([self.TABLE[0], self.TABLE[3], self.TABLE[4], 'default: fixture/default']))
        # A filter that hides the default still reports it as listed.
        self.assertEqual(self.call('models gemma')[0].split('\n')[-1], 'default: fixture/default')
        for bad in ['{"method":"models","filter":"two words"}', '{"method":"models","json":1}', '{"method":"models","x":1}']:
            self.assertTrue(self.call(bad)[0].startswith('error:'), bad)
        # More than one word after models is an English prompt, not the method.
        self.call('models are getting cheaper, why?')
        self.assertEqual(self.server.requests[-1][0], '/v1/chat/completions')

    def test_models_default_matches_variant_and_canonical_slug_and_says_when_unset(self):
        self.route('/models', self.MODELS)
        for default, mark, footer in [('fixture/default:nitro', 'fixture/default', 'default: fixture/default:nitro'),
                                      ('openai/gpt-4.1-mini-2025-04-14', 'openai/gpt-4.1-mini', 'default: openai/gpt-4.1-mini-2025-04-14'),
                                      ('google/gemma-4-31b-it:free', 'google/gemma-4-31b-it:free', 'default: google/gemma-4-31b-it:free'),
                                      ('fixture/gone', None, 'default: fixture/gone (not listed)'),
                                      (None, None, 'default: unset')]:
            self.config['model'] = default
            if default is None:
                del self.config['model']
            lines = self.call('models')[0].split('\n')
            self.assertEqual([line.split()[1] for line in lines[1:-1] if line.startswith('* ')], [mark] if mark else [], default)
            self.assertEqual(lines[-1], footer)

    def test_models_json_passes_the_provider_records_through(self):
        self.route('/models', self.MODELS)
        for body in ['models --json', '{"method":"models","json":true}']:
            self.assertEqual(json.loads(self.call(body)[0]), self.MODELS['data'])
        self.assertEqual(json.loads(self.call('models --json free')[0]), self.MODELS['data'][2:4])

    def test_models_prices_are_exact_finite_and_one_unit_and_cells_stay_on_one_line(self):
        # A price under 0.00005 USD/M is not free, a non-finite price is no price, and per-image never meets per-megapixel.
        rows = {'data': [{'id': 'fixture/tiny', 'pricing': {'prompt': '0.00000000001', 'completion': '0'}},
                         {'id': 'fixture/odd', 'pricing': {'prompt': 'NaN', 'completion': 'inf'}},
                         {'id': 'fixture/line\nbreak', 'context_length': 'a\rb', 'architecture': {'input_modalities': ['te\nxt']}}]}
        self.route('/models', rows)
        self.assertEqual([line.split() for line in self.call('models fixture')[0].split('\n')[1:-1]],
                         [['fixture/tiny', '-', '0', '0', '-'], ['fixture/odd', '-', '-', '-', '-'],
                          ['fixture/line\\x0abreak', 'a\\x0db', '-', '-', 'te\\x0axt']])
        self.assertEqual(self.call('models free')[0].split('\n')[1:-1], [])
        self.route('/images/models', {'data': [{'id': 'fixture/both', 'pricing': [{'unit': 'megapixel', 'cost_usd': 0.001},
                                                                                {'unit': 'image', 'cost_usd': 0.05}, {'unit': 'image', 'cost_usd': 0.04}]},
                                               {'id': 'fixture/mp', 'pricing': [{'unit': 'megapixel', 'cost_usd': 0.02}, {'unit': 'megapixel', 'cost_usd': 'inf'}]}]})
        self.assertEqual([line.split()[:2] for line in self.call('models --images')[0].split('\n')[1:-1]],
                         [['fixture/both', '0.04/image'], ['fixture/mp', '0.02/megapixel']])

    def test_model_keeps_each_endpoints_own_context_and_prices(self):
        self.route('/models/openai/gpt-4/endpoints', self.ENDPOINTS)
        for body in ['model openai/gpt-4', '{"method":"model","id":"openai/gpt-4"}']:
            value = json.loads(self.call(body)[0])
            self.assertEqual(self.server.requests[-1][0], '/v1/models/openai/gpt-4/endpoints')
            self.assertEqual((value['id'], value['input']), ('openai/gpt-4', ['text', 'image']))
            self.assertEqual([(e['provider'], e['context_length'], e['pricing']['prompt']) for e in value['endpoints']],
                             [('OpenAI', 8192, '0.00003'), ('Azure', 32768, '0.00006')])
        # A prompt that merely starts with "model" is still a prompt.
        self.assertEqual(self.call('model this as a tree')[0], 'Hello from the model')
        self.assertEqual(self.server.requests[-1][0], '/v1/chat/completions')

    def test_balance_and_key_are_for_owner_and_maintainers_only(self):
        self.route('/credits', self.CREDITS)
        self.route('/key', self.KEY_INFO)
        for method in ['balance', 'key']:
            for roles in [(), ('guest',), ('admin', 'write')]:
                self.assertEqual(self.call(method, roles=roles)[:2], (f'error: {method} needs the owner or maintainer role', 0))
                self.assertEqual(self.call(json.dumps({'method': method}), roles=roles)[0], f'error: {method} needs the owner or maintainer role')
        self.assertEqual(self.server.requests, [], 'a refused role must not reach the provider')
        for roles in [('owner',), ('maintainer',)]:
            self.assertEqual(self.call('balance', roles=roles)[0], '12.3456 USD left (20 USD bought, 7.6544 USD used)')
            self.assertEqual(json.loads(self.call('key', roles=roles)[0]), {
                'limit': 50, 'limit_remaining': 42.5, 'limit_reset': 'monthly', 'usage': 7.5, 'is_free_tier': False,
                'rate_limit': {'requests': 230, 'interval': '10s'}, 'free_model_daily_requests': {'used': 1, 'limit': 1000, 'remaining': 999}})
        self.assertEqual([r[0] for r in self.server.requests], ['/v1/credits', '/v1/key'] * 2)

    def serve(self, envelopes, store=None):
        """Run the kept loop over envelopes; it ends on the SIGTERM the fake bus sends once they are served."""
        handlers = {number: signal.getsignal(number) for number in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP)}
        self.addCleanup(lambda: [signal.signal(number, handler) for number, handler in handlers.items()])
        bus = FakeBus(envelopes, store)
        with contextlib.redirect_stderr(io.StringIO()):
            code = proxy.serve(bus, self.config, KEY, None)
        return code, bus

    def test_kept_plain_answers_trim_only_trailing_newlines(self):
        with patch.object(proxy, 'handle', return_value='answer\n\n'):
            code, bus = self.serve([{'message_id': 'a', 'body': 'hello'}])
        self.assertEqual((code, bus.replies, bus.dones), (0, [('a', 'answer')], []))

    def test_kept_empty_answer_sends_done_without_a_reply(self):
        with patch.object(proxy, 'handle', return_value='\n\n'):
            code, bus = self.serve([{'message_id': 'a', 'body': 'hello'}])
        self.assertEqual((code, bus.replies, bus.dones), (0, [], ['a']))

    def test_kept_handler_failure_is_generic_and_next_message_still_answers(self):
        for error in (RuntimeError('private payload ' + KEY), MemoryError('private payload ' + KEY)):
            with self.subTest(error=type(error).__name__):
                escaped = None
                with patch.object(proxy, 'handle', side_effect=[error, 'second']), patch('builtins.print') as logged:
                    try:
                        code, bus = self.serve([{'message_id': 'a', 'body': 'poison'}, {'message_id': 'b', 'body': 'hello'}])
                    except Exception as failure:
                        escaped = type(failure).__name__
                self.assertIsNone(escaped, 'an unexpected handler error ended the kept reader')
                self.assertEqual((code, bus.replies), (0, [('a', 'error: message handling failed'), ('b', 'second')]))
                self.assertEqual(logged.call_count, 1)
                self.assertEqual(logged.call_args.args, ('ai: message handling failed',))

    def test_roles_come_from_the_daemon_set_envelope_only(self):
        self.route('/credits', self.CREDITS)
        left, refused = '12.3456 USD left (20 USD bought, 7.6544 USD used)', 'error: balance needs the owner or maintainer role'
        # Only the daemon-set roles count, and roles on a message first sent to another name are not #ai's.
        cases = [('1', 'balance', ['maintainer'], None, left), ('2', 'balance', ['owner'], '#ai@t', left),
                 ('3', 'balance', ['owner', 'admin'], '#AI@t', left), ('4', 'balance', [], None, refused),
                 ('5', 'balance', ['admin', 'write'], None, refused), ('6', 'balance', ['owner'], '#other@t', refused),
                 ('7', 'balance', 'owner', None, refused)]
        envelopes = [{'message_id': mid, 'from': 'owner@h', 'body': body, 'roles': roles, **({'original_to': to} if to else {})}
                     for mid, body, roles, to, _ in cases]
        envelopes.insert(0, {'message_id': 'r', 'from': 'x@h', 'body': 'balance', 'roles': ['owner'], 'receipt': 'read'})
        envelopes.insert(1, {'message_id': 'h', 'from': 'x@h', 'body': 'invalid normal input', 'ask': 'help-json', 'roles': ['owner']})
        code, bus = self.serve(envelopes)
        self.assertEqual(code, 0)
        self.assertEqual(set(json.loads(bus.replies[0][1])['methods']), {'ask', 'decide', 'models', 'model', 'embed', 'balance', 'key', 'image'})
        self.assertEqual(bus.replies[1:], [(mid, want) for mid, _, _, _, want in cases])
        self.assertEqual(len(self.server.requests), 3, 'only the three served balance calls reach the provider')
        self.assertTrue(all(share for _, share in bus.consumed))

    def test_sigterm_and_sighup_end_the_loop_after_the_answer(self):
        for number in (signal.SIGTERM, signal.SIGHUP):
            stop = lambda *_: os.kill(os.getpid(), number)
            # The signal arrives while a message is being answered: that answer is still sent, then the loop ends.
            with patch.object(proxy, 'answer', side_effect=lambda *a, **k: (stop(), ('answered', 0))[1]):
                code, bus = self.serve([{'message_id': 'a', 'body': 'hello'}, {'message_id': 'b', 'body': 'never read'}])
            self.assertEqual((code, bus.replies, len(bus.queue)), (0, [('a', 'answered')], 1), number)

    def test_a_method_the_provider_lacks_is_refused_by_name(self):
        for method, body in [('models', 'models'), ('models', 'models --images'), ('model', 'model openai/gpt-4'), ('balance', 'balance'),
                             ('key', 'key'), ('embed', 'embed "hello"')]:
            self.server.status, self.server.payload = 404, b'{"error":{"message":"Not Found"}}'
            self.config['embed_model'] = 'fixture/embed'
            self.assertEqual(self.call(body, roles=('owner',))[:2], (f'error: provider has no {method}', 0))
        # model asks the chat catalogue, then the Image API, before it says the provider has none.
        self.assertEqual([r[0] for r in self.server.requests], ['/v1/models', '/v1/images/models', '/v1/models/openai/gpt-4/endpoints',
                         '/v1/images/models/openai/gpt-4/endpoints', '/v1/credits', '/v1/key', '/v1/embeddings'])

    def test_text_answers_report_tokens_and_cost_from_usage(self):
        self.server.payload = json.dumps({'choices': [{'message': {'content': 'Four.'}}],
                                          'usage': {'prompt_tokens': 12, 'completion_tokens': 3, 'total_tokens': 15, 'cost': 0.000123}}).encode()
        for body in ['What is two plus two?', 'ask What is two plus two?']:
            self.assertEqual(self.call(body)[0], 'Four.\n\n[tokens 12 in, 3 out; cost 0.000123 USD]')
        # A chat JSON request already carries the provider's usage object unchanged.
        self.assertEqual(json.loads(self.call('{"messages":[]}')[0])['usage']['cost'], 0.000123)

    def embeddings(self, count, dims, value=-1.7976931348623157e+308):
        return {'object': 'list', 'model': 'fixture/embed', 'usage': {'prompt_tokens': count, 'total_tokens': count},
                'data': [{'object': 'embedding', 'index': i, 'embedding': [value] * dims} for i in reversed(range(count))]}

    def test_embed_sends_text_or_list_and_answers_vectors_in_order(self):
        self.config['embed_model'] = 'fixture/embed'
        data = {'object': 'list', 'data': [{'object': 'embedding', 'index': 1, 'embedding': [0.5, -0.25]},
                                           {'object': 'embedding', 'index': 0, 'embedding': [0.125, 1]}]}
        self.route('/embeddings', data)
        value = json.loads(self.call('embed ["first","second"]')[0])
        self.assertEqual(value, {'model': 'fixture/embed', 'vectors': [[0.125, 1], [0.5, -0.25]]})
        self.assertEqual(self.server.requests[-1], ('/v1/embeddings', 'Bearer ' + KEY, {'model': 'fixture/embed', 'input': ['first', 'second']}))
        self.route('/embeddings', {'data': [{'index': 0, 'embedding': [0.5]}]})
        self.assertEqual(json.loads(self.call('embed "one text"')[0])['vectors'], [[0.5]])
        self.assertEqual(self.server.requests[-1][2], {'model': 'fixture/embed', 'input': 'one text'})
        self.call(json.dumps({'method': 'embed', 'input': 'x', 'model': 'fixture/other'}))
        self.assertEqual(self.server.requests[-1][2], {'model': 'fixture/other', 'input': 'x'})
        before = len(self.server.requests)
        del self.config['embed_model']
        for body in ['embed "hello"', 'embed []', 'embed [1]', json.dumps({'method': 'embed', 'input': ''}),
                     json.dumps({'method': 'embed', 'input': 'x', 'extra': 1})]:
            self.assertTrue(self.call(body)[0].startswith('error:'), body)
        self.assertEqual(len(self.server.requests), before)

    def send_request(self, envelope, body):
        # What answerAs posts to /send, spelled as Go's encoding/json writes protocol.Envelope (checked against
        # json.Marshal on 2026-11-05: 273 bytes for the probe below), independent of ai.py's own size helper.
        back = envelope.get('reply_to') or {'name': envelope['from'], 'topic': envelope.get('topic'), 'tag': envelope.get('tag')}
        fields = [('message_id', ''), ('from', ''), ('to', back['name'])] + [(k, back[k]) for k in ('topic', 'tag') if back.get(k)]
        fields += [('body', body), ('at', '0001-01-01T00:00:00Z'), ('expires', '0001-01-01T00:00:00Z'), ('deadline', '0001-01-01T00:00:00Z')]
        text = json.dumps(dict(fields), separators=(',', ':'), ensure_ascii=False)
        for raw, go in (('<', '\\u003c'), ('>', '\\u003e'), ('&', '\\u0026'), ('\u2028', '\\u2028'), ('\u2029', '\\u2029')):
            text = text.replace(raw, go)
        return len(text.encode())

    def test_reply_size_matches_the_go_envelope_probe(self):
        envelope = {'from': '#caller@h', 'topic': 't<o>&p\u2028\u00e9', 'tag': 'tag'}
        body = '{"model":"fixture/embed","vectors":[[-1.7976931348623157e+308,1]]}'
        self.assertEqual(proxy.reply_size(envelope, body), 273)
        self.assertEqual(self.send_request(envelope, body), 273)

    def test_embed_cap_is_exact_in_the_reply_envelope_and_remembers_dims(self):
        self.config['embed_model'] = 'fixture/embed'
        dims, work = 3, self.work()
        texts = lambda n: 'embed ' + json.dumps(['t'] * n)
        # Long, escaping route fields cost envelope bytes; reply_to replaces the sender's own route.
        for route in [{}, {'topic': '<&>' * 200 + '\u2028\u00e9' * 50, 'tag': '"q"' * 100},
                      {'reply_to': {'name': '#elsewhere@h', 'topic': 'x' * 900, 'tag': '&' * 300}}]:
            envelope = {'from': '#fixture@h', **route}
            cap = proxy.max_items('fixture/embed', dims, envelope)
            self.route('/embeddings', self.embeddings(cap, dims))
            answer, code, _ = self.call(texts(cap), work=work, **route)
            self.assertEqual(code, 0, answer[:200])
            self.assertEqual(len(json.loads(answer)['vectors']), cap)
            self.assertIn('-1.7976931348623157e+308', answer)
            size = self.send_request(envelope, answer)
            self.assertLessEqual(size, proxy.ANSWER_LIMIT, 'M items fit the reply the runner posts')
            self.assertGreater(size + dims * 25 + 2, proxy.ANSWER_LIMIT, 'the cap is tight: one more widest item would not fit')
            self.assertLessEqual(len(answer.encode()) + 1, proxy.ANSWER_LIMIT, 'and the stdout bound')
            self.route('/embeddings', self.embeddings(cap + 1, dims))
            self.assertEqual(self.call(texts(cap + 1), work=work, **route)[0], f'error: {cap + 1} items × {dims} dims > max {cap}')
        # A model whose size is not yet known costs one provider call; then its size is kept.
        envelope = {'from': '#fixture@h'}
        other = proxy.max_items('fixture/new', dims, envelope)
        self.config['embed_model'] = 'fixture/new'
        self.route('/embeddings', self.embeddings(other + 1, dims))
        before = len(self.server.requests)
        self.assertEqual(self.call(texts(other + 1), work=work)[0], f'error: {other + 1} items × {dims} dims > max {other}')
        self.assertEqual(len(self.server.requests), before + 1)
        self.assertEqual(self.call(texts(other + 1), work=work)[0], f'error: {other + 1} items × {dims} dims > max {other}')
        self.assertEqual(len(self.server.requests), before + 1, 'a known size is refused before any provider call')
        self.assertEqual(json.loads((Path(work) / proxy.DIMS_FILE).read_text()), {'fixture/embed': dims, 'fixture/new': dims})

    def test_kept_dims_are_bounded_and_written_whole(self):
        work = self.work()
        for i in range(proxy.DIMS_KEPT + 5):
            proxy.remember_dims(work, f'fixture/m{i}', 3)
        kept = json.loads((Path(work) / proxy.DIMS_FILE).read_text())
        self.assertEqual(len(kept), proxy.DIMS_KEPT)
        self.assertNotIn('fixture/m0', kept)
        self.assertIn(f'fixture/m{proxy.DIMS_KEPT + 4}', kept)
        self.assertEqual(sorted(p.name for p in Path(work).iterdir()), [proxy.DIMS_FILE])

    def test_model_ids_cannot_steer_the_provider_path(self):
        for bad in ['../../x', 'a//b', '/a/b', 'a/..', './b', 'a/b/c', 'openai/gpt-4:../x', 'a/b?x', 'a/b#x', 'a/%2e%2e']:
            for body in ['model ' + bad, json.dumps({'method': 'model', 'id': bad})]:
                self.assertTrue(self.call(body)[0].startswith('error:'), body)
        self.assertEqual(self.server.requests, [])
        self.route('/models/openai/gpt-4:free/endpoints', self.ENDPOINTS)
        self.assertEqual(json.loads(self.call('model openai/gpt-4:free')[0])['id'], 'openai/gpt-4')
        self.assertEqual(self.call('model railways')[0], 'Hello from the model')

    def test_deeply_nested_json_is_the_generic_error(self):
        deep = '{"method":"embed","input":' + '[' * 100000 + ']' * 100000 + '}'
        try:
            answer = self.call(deep)[0]
        except RecursionError:
            answer = 'RecursionError escaped answer()'
        self.assertEqual(answer, 'error: invalid request, Configuration or provider response')
        self.assertEqual(self.server.requests, [])

    def test_key_never_appears_in_any_new_method_answer_or_error(self):
        self.config['embed_model'] = 'fixture/embed'
        encoded = ''.join('\\u%04x' % ord(c) for c in KEY)
        echoes = [(500, ('{"error":{"message":"bad key ' + KEY + '"}}').encode()), (401, ('raw ' + KEY).encode()),
                  (429, ('{"error":{"message":{"detail":"' + encoded + '"}}}').encode())]
        bodies = ['models', 'models --images', 'model openai/gpt-4', 'balance', 'key', 'embed "hello"']
        for status, payload in echoes:
            self.server.status, self.server.payload = status, payload
            for body in bodies:
                answer, _, log = self.call(body, roles=('owner',))
                prefix = f'error: the provider refused the configured key ({status}: ' if status in (401, 403) else f'error: {status} '
                self.assertTrue(answer.startswith(prefix), (body, answer))
                self.assertNotIn(KEY, answer + log)
        self.route('/models', {'data': [{'id': 'fixture/' + KEY}]})
        self.route('/models/openai/gpt-4/endpoints', {'data': {'id': 'openai/gpt-4', 'name': encoded, 'endpoints': [{'name': KEY, 'pricing': {}}]}})
        self.route('/key', {'data': {'label': KEY, 'limit': None, 'usage': 0, 'rate_limit': {'note': KEY}}})
        self.route('/embeddings', {'data': [{'index': 0, 'embedding': [1]}], 'model': KEY})
        self.server.routes['/v1/models/openai/gpt-4/endpoints'] = (200, self.server.routes['/v1/models/openai/gpt-4/endpoints'][1].replace(b'\\\\u', b'\\u'))
        for body in bodies:
            answer, _, log = self.call(body, roles=('owner',))
            self.assertNotIn(KEY, answer + log, body)
        self.assertNotIn('label', self.call('key', roles=('owner',))[0])
    IMAGE_MODELS = {'data': [{'id': 'bytedance-seed/seedream-4.5', 'name': 'Seedream 4.5', 'architecture': {'input_modalities': ['text', 'image'], 'output_modalities': ['image']},
                              'supported_parameters': {'resolution': {'type': 'enum', 'values': ['1K', '2K', '4K']}, 'seed': {'type': 'boolean'}},
                              'pricing': [{'billable': 'output_image', 'unit': 'image', 'cost_usd': 0.06}, {'billable': 'output_image', 'unit': 'image', 'cost_usd': 0.03}],
                              'endpoints': '/api/v1/images/models/bytedance-seed/seedream-4.5/endpoints'},
                             {'id': 'openai/gpt-image-2', 'name': 'GPT Image 2'}]}
    IMAGE_ENDPOINTS = {'data': {'id': 'bytedance-seed/seedream-4.5', 'name': 'Seedream 4.5', 'architecture': {'input_modalities': ['text', 'image']},
                                'supported_parameters': {'resolution': {'type': 'enum', 'values': ['1K', '2K', '4K']}},
                                'endpoints': [{'provider_name': 'BytePlus', 'name': 'BytePlus: Seedream 4.5',
                                               'pricing': [{'billable': 'output_image', 'unit': 'image', 'cost_usd': 0.03},
                                                           {'billable': 'output_image', 'unit': 'image', 'cost_usd': 0.06, 'variant': '4k'}],
                                               'supported_parameters': {'seed': {'type': 'boolean'}}},
                                              {'provider_name': 'fal', 'name': 'fal: Seedream 4.5',
                                               'pricing': [{'billable': 'output_image', 'unit': 'megapixel', 'cost_usd': 0.01}]}]}}

    def test_models_images_lists_the_image_catalogue_with_its_default_marked(self):
        self.route('/images/models', self.IMAGE_MODELS)
        self.config['image_model'] = 'openai/gpt-image-2'
        for body in ['models --images', '{"method":"models","images":true}']:
            self.assertEqual(self.call(body)[0].split('\n'), ['  id                           price USD   input',
                                                               '  bytedance-seed/seedream-4.5  0.03/image  text,image',
                                                               '* openai/gpt-image-2           -           -',
                                                               'default: openai/gpt-image-2'])
            self.assertEqual(self.server.requests[-1], ('/v1/images/models', 'Bearer ' + KEY, None))
        self.assertEqual(json.loads(self.call('models --images --json seedream')[0]), self.IMAGE_MODELS['data'][:1])
        for bad in ['{"method":"models","images":1}', '{"method":"models","images":true,"x":1}']:
            self.assertTrue(self.call(bad)[0].startswith('error:'), bad)

    def test_model_folds_in_image_endpoint_parameters_and_prices(self):
        self.route('/models/bytedance-seed/seedream-4.5/endpoints', {'error': {'message': 'Not Found'}}, 404)
        self.route('/images/models/bytedance-seed/seedream-4.5/endpoints', self.IMAGE_ENDPOINTS)
        value = json.loads(self.call('model bytedance-seed/seedream-4.5')[0])
        self.assertEqual((value['kind'], value['parameters']), ('image', {'resolution': {'type': 'enum', 'values': ['1K', '2K', '4K']}}))
        self.assertEqual([(e['provider'], [p['cost_usd'] for p in e['pricing']]) for e in value['endpoints']], [('BytePlus', [0.03, 0.06]), ('fal', [0.01])])
        self.assertEqual(value['endpoints'][0]['supported_parameters'], {'seed': {'type': 'boolean'}})
        self.assertEqual([r[0] for r in self.server.requests], ['/v1/models/bytedance-seed/seedream-4.5/endpoints',
                                                                 '/v1/images/models/bytedance-seed/seedream-4.5/endpoints'])
        # A chat model answers from its own catalogue alone.
        self.route('/models/openai/gpt-4/endpoints', self.ENDPOINTS)
        self.assertEqual(json.loads(self.call('model openai/gpt-4')[0])['kind'], 'text')
        self.assertEqual(len(self.server.requests), 3)

    IMAGES = {'created': 1748372400, 'data': [{'b64_json': base64.b64encode(PNG).decode(), 'media_type': 'image/png'}],
              'usage': {'prompt_tokens': 0, 'completion_tokens': 4175, 'cost': 0.04}}
    TWO_IMAGES = {'data': [{'b64_json': base64.b64encode(PNG).decode(), 'media_type': 'image/png'},
                           {'b64_json': base64.b64encode(JPEG).decode()}], 'usage': {'cost': 0.08}}
    KEPT = r'#ai@t/image-[0-9a-f]{16}\.%s image/%s %d bytes, kept 24 h'

    def image(self, roles=('owner',), **fields):
        return self.call(json.dumps({'method': 'image', 'prompt': 'a red panda', **fields}), roles=roles)[0]

    def test_image_puts_each_picture_into_the_bigfile_record_and_names_it(self):
        self.config['image_model'] = 'openai/gpt-image-2'
        self.route('/images', self.TWO_IMAGES)
        lines = self.image(n=2, aspect_ratio='16:9').split('\n')
        self.assertEqual(len(lines), 4, lines)
        self.assertRegex(lines[0], '^' + self.KEPT % ('png', 'png', len(PNG)) + '$')
        # The second item has no media type: its bytes say JPEG.
        self.assertRegex(lines[1], '^' + self.KEPT % ('jpeg', 'jpeg', len(JPEG)) + '$')
        self.assertEqual(lines[2:], ['', '[cost 0.08 USD]'])
        puts = self.store.bigfile.puts
        self.assertEqual([(lines[i].split()[0], data, ttl) for i, (_, data, ttl) in enumerate(puts)],
                         [('#ai@t/' + name, data, 86400) for name, data, _ in puts])
        self.assertEqual([data for _, data, _ in puts], [PNG, JPEG])
        self.assertEqual(self.server.requests[-1], ('/v1/images', 'Bearer ' + KEY,
                                                     {'model': 'openai/gpt-image-2', 'prompt': 'a red panda', 'n': 2, 'aspect_ratio': '16:9'}))
        self.assertNotEqual(puts[0][0], puts[1][0])
        self.image(n=2)
        self.assertEqual(len({name for name, _, _ in self.store.bigfile.puts}), 4, 'every image gets a new random name')

    def test_image_is_for_owner_and_maintainers_and_never_guesses_a_model(self):
        self.route('/images', self.IMAGES)
        self.config['image_model'] = 'openai/gpt-image-2'
        for body in ['image {"prompt":"a red panda"}', json.dumps({'method': 'image', 'prompt': 'a red panda', 'n': 2})]:
            for roles in [(), ('guest',), ('admin',)]:
                self.assertEqual(self.call(body, roles=roles)[:2], ('error: image needs the owner or maintainer role', 0))
        self.assertEqual(self.server.requests, [], 'a refused role must not reach the provider')
        for roles in [('owner',), ('maintainer',)]:
            self.assertRegex(self.image(roles=roles), '^' + self.KEPT % ('png', 'png', len(PNG)) + '\n')
        del self.config['image_model']
        before = len(self.server.requests)
        self.assertEqual(self.image(), 'error: no image model configured: set Configuration image_model or give a request model')
        self.assertEqual((len(self.server.requests), self.store.bigfile.lists), (before, 4), 'no provider call and no bigfile look without a model')
        self.assertRegex(self.image(model='openai/gpt-image-2'), '^#ai@t/image-')

    def test_a_full_record_is_refused_by_name_before_the_paid_call(self):
        self.config['image_model'] = 'openai/gpt-image-2'
        self.route('/images', self.IMAGES)
        mib = 1024 * 1024
        full = lambda keys, used: f'error: #ai@t bigfile record is full ({keys} keys, {used} bytes of 100 keys / 256 MiB); try again after old images expire (24 h)'
        for rows, n, want in [([{'name': f'k{i}', 'size': 1} for i in range(100)], 1, full(100, 100)),
                              ([{'name': f'k{i}', 'size': 1} for i in range(97)], 4, full(97, 97)),
                              ([{'name': 'big', 'size': 256 * mib - 32 * mib + 1}], 1, full(1, 256 * mib - 32 * mib + 1))]:
            self.store = Record(Bigfile(rows))
            self.assertEqual(self.image(n=n), want)
            self.assertEqual(self.store.bigfile.puts, [])
        self.assertEqual(self.server.requests, [], 'a full record refuses before any paid call')
        self.store = Record(Bigfile([{'name': 'big', 'size': 256 * mib - 32 * mib}]))
        self.assertRegex(self.image(), '^#ai@t/image-', 'exactly room for one image at the largest size is room')

    def test_a_record_filled_during_the_call_names_what_was_kept_and_the_cost(self):
        self.config['image_model'] = 'openai/gpt-image-2'
        self.route('/images', self.TWO_IMAGES)
        # The other process filled the record between this one's check and its second put.
        self.store = Record(Bigfile(puts_allowed=1))
        lines = self.image(n=2).split('\n')
        self.assertRegex(lines[0], '^' + self.KEPT % ('png', 'png', len(PNG)) + '$')
        self.assertEqual(lines[1:], ['error: #ai@t bigfile record is full (100 keys / 256 MiB); try again after old images expire (24 h)', '', '[cost 0.08 USD]'])
        self.assertEqual(len(self.store.bigfile.puts), 1)

    def test_a_store_that_refuses_or_an_answer_without_an_image_is_named(self):
        self.config['image_model'] = 'openai/gpt-image-2'
        self.route('/images', self.IMAGES)
        for error, want in [(Refusal(403, 'forbidden'), 'error: #ai@t cannot store images (403: forbidden)'),
                            (Refusal(503, 'bigfile store unavailable'), 'error: #ai@t cannot store images (503: bigfile store unavailable)'),
                            (TransportError('closed'), 'error: #ai@t cannot store images (the bus did not answer)')]:
            self.store = Record(Bigfile(refuse=error))
            self.assertEqual(self.image(), want)
        self.assertEqual(self.server.requests, [])
        self.store = Record()
        for data in [[], [{'b64_json': 'not base64!'}], [{'b64_json': base64.b64encode(b'plain text').decode()}],
                     [{'b64_json': base64.b64encode(PNG).decode(), 'media_type': 'text/html'}], [{'url': 'https://example.com/a.png'}],
                     [{'b64_json': base64.b64encode(PNG).decode()}] * (proxy.IMAGE_N_MAX + 1)]:
            self.route('/images', {'data': data})
            self.assertEqual(self.image(), 'error: provider returned no image', data)
        # A paid answer without a usable image still says what it cost.
        self.route('/images', {'data': [{'b64_json': 'not base64!'}], 'usage': {'cost': 0.04}})
        self.assertEqual(self.image(), 'error: provider returned no image\n\n[cost 0.04 USD]')
        self.route('/images', {'data': [{'url': 'data:image/png;base64,' + base64.b64encode(PNG).decode()}]})
        self.assertRegex(self.image(), '^' + self.KEPT % ('png', 'png', len(PNG)) + '$')
        self.assertEqual(len(self.store.bigfile.puts), 1)
        self.route('/images', {'error': {'message': 'model not found'}}, 400)
        self.assertEqual(self.image(), 'error: 400 model not found')
        # One image over what a bigfile key holds is named and skipped; the others are kept.
        self.route('/images', self.TWO_IMAGES)
        with patch.object(proxy, 'IMAGE_MAX', len(JPEG)):
            lines = self.image(n=2).split('\n')
        self.assertEqual(lines[0], f'error: image 1 is {len(PNG)} bytes, over the 32 MiB a bigfile holds; not kept')
        self.assertRegex(lines[1], '^' + self.KEPT % ('jpeg', 'jpeg', len(JPEG)) + '$')

    def test_image_caps_n_options_references_and_request_size(self):
        self.config['image_model'] = 'openai/gpt-image-2'
        self.route('/images', self.IMAGES)
        image = lambda **fields: self.image(**fields)
        self.assertRegex(image(n=proxy.IMAGE_N_MAX), '^#ai@t/image-')
        for n in [0, proxy.IMAGE_N_MAX + 1, 10, '2', True, 1.0]:
            self.assertEqual(image(n=n), f'error: n is 1 to {proxy.IMAGE_N_MAX} images per call', n)
        self.assertTrue(image(output_format='bmp').startswith('error: output_format is one of'))
        self.assertTrue(image(stream=True).startswith('error: image options are'))
        good = ['https://example.com/a.png', 'data:image/png;base64,iVBORw0KGgo=', {'type': 'image_url', 'image_url': {'url': 'https://example.com/b.jpg'}}]
        self.assertRegex(image(input_references=good), '^#ai@t/image-')
        self.assertEqual(self.server.requests[-1][2]['input_references'], [{'type': 'image_url', 'image_url': {'url': u}} for u in
                                                                        ['https://example.com/a.png', 'data:image/png;base64,iVBORw0KGgo=', 'https://example.com/b.jpg']])
        for bad in [['http://example.com/a.png'], ['file:///etc/passwd'], ['my-bigfile-name'], [{'url': 'https://x'}],
                    ['https://example.com/x'] * (proxy.IMAGE_REFERENCES_MAX + 1), 'https://example.com/a.png']:
            self.assertRegex(image(input_references=bad), r'^error: (an )?input', bad)
        big = 'data:image/png;base64,' + 'A' * proxy.MAX_BYTES
        self.assertEqual(proxy.image_request({'prompt': 'p', 'input_references': [big[:proxy.MAX_BYTES - 200]]}, self.config)[0][:1], b'{')
        with self.assertRaisesRegex(proxy.ProxyError, 'request exceeds 1048576 bytes'):
            proxy.image_request({'prompt': 'p', 'input_references': [big]}, self.config)
        sent = len(self.server.requests)
        self.assertEqual(image(model='../../x'), 'error: image model is an author/slug model id')
        self.assertEqual(self.call(json.dumps({'method': 'image', 'model': 'a/b'}), roles=('owner',))[0], 'error: image takes a prompt')
        self.assertEqual(len(self.server.requests), sent)

    def test_refused_key_and_credit_are_named_for_every_method(self):
        self.config['embed_model'] = 'fixture/embed'
        bodies = ['hello', '{"messages":[]}', 'decide {}', 'models', 'models --images', 'model openai/gpt-4', 'balance', 'key', 'embed "hello"']
        cases = [(401, b'{"error":{"message":"User not found."}}', 'the provider refused the configured key (401: User not found.)'),
                 (403, b'{"error":{"message":"Key disabled"}}', 'the provider refused the configured key (403: Key disabled)'),
                 (401, ('{"error":{"message":"bad key ' + KEY + '"}}').encode(), 'the provider refused the configured key (401: bad key ***)'),
                 (403, ('denied ' + KEY).encode(), 'the provider refused the configured key (403: denied ***)'),
                 (402, b'{"error":{"message":"Insufficient credits"}}', 'the provider refused for lack of credit (402: Insufficient credits)')]
        encoded = ''.join('\\u%04x' % ord(c) for c in KEY)
        for status, named in [(401, 'the configured key'), (402, 'for lack of credit'), (403, 'the configured key')]:
            cases.append((status, ('{"error":{"message":"echo ' + encoded + '"}}').encode(), f'the provider refused {named} ({status}: echo ***)'))
        for status, payload, expected in cases:
            self.server.status, self.server.payload = status, payload
            for body in bodies:
                answer, _, log = self.call(body, roles=('owner',))
                self.assertEqual(answer, 'error: ' + expected, (status, body))
                self.assertNotIn(KEY, answer + log)

    def test_refused_key_answers_stay_within_one_mebibyte(self):
        for status, named in [(401, 'the configured key'), (402, 'for lack of credit'), (403, 'the configured key')]:
            self.server.status = status
            for payload in [b'x' * (proxy.MAX_BYTES - 10), json.dumps({'error': {'message': 'y' * (proxy.MAX_BYTES - 40)}}).encode()]:
                self.server.payload = payload
                for body in ['hello', '{"messages":[]}', 'decide {}', 'models']:
                    answer = self.call(body)[0]
                    self.assertEqual(answer, f'error: the provider refused {named} ({status}: provider error exceeds 1048576 bytes)', (status, body))

    def test_a_failed_dims_write_leaves_no_partial_file(self):
        work = self.work()
        real = os.replace
        with patch('os.replace', side_effect=OSError('disk full')):
            proxy.remember_dims(work, 'fixture/m', 3)
        self.assertEqual(list(Path(work).iterdir()), [])
        self.assertIs(os.replace, real)

class ImageReadBounds(unittest.TestCase):
    def transport(self, size):
        response = Mock(status=200)
        # Valid JSON padded to the exact byte boundary, without a paid provider.
        response.read.side_effect = lambda _limit: b'{}' + b' ' * (size - 2)
        connection = Mock()
        return response, connection

    def test_image_answer_exact_cap_and_cap_plus_one(self):
        self.assertEqual(proxy.IMAGE_READ, 48 * 1024 * 1024)
        manifest = json.loads((HERE / 'agent.json').read_text())
        self.assertEqual(manifest['processes'], 2)
        self.assertLessEqual(2 * (3 * proxy.IMAGE_READ + 64 * 1024 * 1024) + 64 * 1024 * 1024, 512 * 1024 * 1024)
        base = proxy.trusted_base({'base_url': 'https://fixture.invalid/v1'})
        for extra in (0, 1):
            response, connection = self.transport(proxy.IMAGE_READ + extra)
            with self.subTest(extra=extra), patch.object(proxy, 'arm_deadline', return_value=(signal.SIG_DFL, 60)), patch.object(proxy, 'connect', return_value=connection), patch.object(proxy, 'post', return_value=response), patch.object(signal, 'setitimer'), patch.object(signal, 'signal'):
                if extra:
                    with self.assertRaisesRegex(proxy.ProxyError, '^image answer exceeds ' + str(proxy.IMAGE_READ) + ' bytes$'):
                        proxy.provider_call('image', {}, base, 'POST', '/images', b'{}', KEY, proxy.IMAGE_READ)
                else:
                    try:
                        answer = proxy.provider_call('image', {}, base, 'POST', '/images', b'{}', KEY, proxy.IMAGE_READ)
                    except proxy.ProxyError as failure:
                        answer = str(failure)
                    self.assertEqual(answer, (200, {}), 'an image answer exactly at the cap must be accepted')
                response.read.assert_called_once_with(proxy.IMAGE_READ + 1)
                connection.close.assert_called_once()

    def test_image_and_other_methods_use_their_own_alarm_ceiling(self):
        base = proxy.trusted_base({'base_url': 'https://fixture.invalid/v1'})
        deadline = datetime.datetime.fromtimestamp(1060.1, datetime.timezone.utc).isoformat()
        longer = datetime.datetime.fromtimestamp(1090.1, datetime.timezone.utc).isoformat()
        for method, envelope, expected in (('image', {}, 75), ('image', {'deadline': longer}, 75), ('image', {'deadline': deadline}, 45), ('models', {}, 60)):
            response, connection = self.transport(2)
            # Use the real arm_deadline: patching it would hide a fixed 60s ceiling.
            with self.subTest(method=method, envelope=envelope), patch('provider_http.time.time', return_value=1000), patch.object(proxy, 'connect', return_value=connection) as connect, patch.object(proxy, 'post', return_value=response), patch.object(signal, 'setitimer') as alarm, patch.object(signal, 'signal', return_value=signal.SIG_DFL):
                self.assertEqual(proxy.provider_call(method, envelope, base, 'POST', '/fixture', b'{}', KEY, proxy.IMAGE_READ), (200, {}))
                self.assertEqual(connect.call_count, 1)
                self.assertAlmostEqual(connect.call_args.args[1], expected)
                self.assertEqual(alarm.call_args_list[-1].args, (signal.ITIMER_REAL, 0))

    def test_image_reserves_fifteen_seconds_for_storage_and_reply(self):
        base = proxy.trusted_base({'base_url': 'https://fixture.invalid/v1'})
        for remaining, expected in ((90, 75), (60, 45), (45, 30), (44.9, None), (29.9, None), (10, None)):
            response, connection = self.transport(2)
            with self.subTest(remaining=remaining), patch.object(proxy, 'arm_deadline', return_value=(signal.SIG_DFL, remaining)), patch.object(proxy, 'connect', return_value=connection) as connect, patch.object(proxy, 'post', return_value=response) as post, patch.object(signal, 'setitimer') as alarm, patch.object(signal, 'signal'):
                if expected is None:
                    with self.assertRaisesRegex(proxy.ProxyError, r'^image needs at least 30 s for generation: use --wait=60s or more \(90s recommended\)$'):
                        proxy.provider_call('image', {}, base, 'POST', '/images', b'{}', KEY, proxy.IMAGE_READ)
                    connect.assert_not_called()
                    post.assert_not_called()
                else:
                    self.assertEqual(proxy.provider_call('image', {}, base, 'POST', '/images', b'{}', KEY, proxy.IMAGE_READ), (200, {}))
                    connect.assert_called_once_with(base, expected)
                    self.assertEqual(alarm.call_args_list[0].args, (signal.ITIMER_REAL, expected))
                self.assertEqual(alarm.call_args_list[-1].args, (signal.ITIMER_REAL, 0))
        self.assertEqual(json.loads((HERE / 'agent.json').read_text())['deadline'], '90s')


if __name__ == '__main__':
    unittest.main()

class AskDispatch(unittest.TestCase):
    def test_asks_never_reach_provider_or_normal_body(self):
        from unittest.mock import Mock, patch
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
