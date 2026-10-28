import contextlib
import datetime
import http.client
import http.server
import importlib.util
import io
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import sys
import threading
import time
import unittest
from unittest.mock import patch, Mock

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('qdrant_proxy', os.environ.get('QDRANT_PROXY_SOURCE', str(HERE / 'qdrant.py')))
proxy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(proxy)
KEY = 'fixture-secret-qdrant-abcdef012345'


class Fake(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def handle_request(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        self.server.requests.append((self.command, self.path, dict(self.headers), body))
        if self.server.delay:
            time.sleep(self.server.delay)
        status = self.server.status if self.headers.get('api-key') == KEY else 401
        self.send_response(status)
        self.send_header('Content-Type', self.server.content_type)
        self.send_header('Content-Encoding', self.server.encoding)
        self.send_header('Location', self.server.url + '/collections/stolen')
        self.send_header('X-Echo', self.server.echo_header)
        if self.server.cut:
            self.send_header('Content-Length', '100')
        self.end_headers()
        if self.command == 'HEAD' or status == 204:
            return
        if self.server.cut:
            self.wfile.write(b'{')
            self.wfile.flush()
            return
        try:
            chunks = [bytes([v]) for v in self.server.payload] if self.server.trickle else [self.server.payload]
            for chunk in chunks:
                self.wfile.write(chunk)
                self.wfile.flush()
                if self.server.trickle:
                    time.sleep(self.server.trickle)
        except (BrokenPipeError, ConnectionResetError):
            pass

    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = do_HEAD = do_OPTIONS = handle_request


class Qdrant(unittest.TestCase):
    def setUp(self):
        self.server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Fake)
        self.server.url = f'http://127.0.0.1:{self.server.server_port}'
        self.server.requests = []
        self.server.payload = b'{"result":true,"status":"ok"}'
        self.server.status = 200
        self.server.content_type = 'application/json; charset=utf-8'
        self.server.encoding = 'identity'
        self.server.echo_header = ''
        self.server.delay = self.server.trickle = 0
        self.server.cut = False
        thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': .01}, daemon=True)
        thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(thread.join)
        self.addCleanup(self.server.shutdown)
        actual = socket.create_connection

        def only_fixture(address, *args, **kwargs):
            if address[0] != '127.0.0.1' or address[1] != self.server.server_port:
                raise AssertionError('external connection forbidden in test')
            return actual(address, *args, **kwargs)
        guard = patch('socket.create_connection', only_fixture)
        guard.start()
        self.addCleanup(guard.stop)

    def call(self, body, roles=None, config=None, deadline=None, key=KEY):
        envelope = {'body': body, 'deadline': deadline}
        if roles is not None:
            envelope['roles'] = roles
        log = io.StringIO()
        with contextlib.redirect_stderr(log):
            output, code = proxy.answer(envelope, {'url': self.server.url} if config is None else config, key)
        return output, code, log.getvalue()

    def decoded(self, output):
        self.assertFalse(output.startswith('error:'), output)
        try:
            return json.loads(output)
        except ValueError:
            self.fail('expected a JSON reply, got ' + output[:100])

    def test_exact_raw_body_query_key_and_identity_encoding(self):
        raw = ' { "points" : [ { "id":1, "vector":[0.1, 0.2] } ] } '
        output, code, log = self.call('request PUT /collections/a/points?wait=true ' + raw, ['write'])
        self.assertTrue(self.decoded(output)['result'])
        self.assertEqual(code, 0)
        method, path, headers, sent = self.server.requests[-1]
        self.assertEqual((method, path, sent), ('PUT', '/collections/a/points?wait=true', raw.encode()))
        self.assertEqual(headers['api-key'], KEY)
        self.assertEqual(headers['Accept-Encoding'], 'identity')
        self.assertEqual(headers['Content-Type'], 'application/json')
        self.assertNotIn('wait=true', log)
        self.assertNotIn('vector', log)
        self.assertNotIn(KEY, output + log)
        self.call(json.dumps({'method': 'request', 'verb': 'PUT', 'path': '/collections/a/points', 'body': raw}), ['write'])
        self.assertEqual(self.server.requests[-1][3], raw.encode())

    def test_remote_url_prefix_transport_and_http_notice(self):
        seen = []

        def connect(base, remaining):
            seen.append((base.scheme, base.hostname, base.port, remaining))
            return http.client.HTTPConnection('127.0.0.1', self.server.server_port, timeout=remaining)
        with patch.object(proxy, 'connect', connect):
            output, _, log = self.call('request GET /collections?limit=1', ['read'], {'url': 'http://remote.example:8181/store/'})
            self.assertTrue(self.decoded(output)['result'])
            self.assertEqual(self.server.requests[-1][1], '/store/collections?limit=1')
            self.assertEqual(seen[-1][:3], ('http', 'remote.example', 8181))
            self.assertEqual(log.count('notice:'), 1)
            self.assertNotIn('remote.example', log)
            _, _, secure_log = self.call('request GET /collections', ['read'], {'url': 'https://secure.example/prefix'})
            self.assertEqual(seen[-1][:3], ('https', 'secure.example', None))
            self.assertEqual(self.server.requests[-1][1], '/prefix/collections')
            self.assertNotIn('notice:', secure_log)
        with patch('provider_http.http.client.HTTPConnection') as http_ctor, patch('provider_http.http.client.HTTPSConnection') as https_ctor:
            proxy.connect(proxy.urllib.parse.urlsplit('https://tls.example:444'), 1)
            https_ctor.assert_called_once_with('tls.example', 444, timeout=1)
            http_ctor.assert_not_called()
        self.assertFalse(proxy.remote_http(proxy.urllib.parse.urlsplit('http://[::1]:6333')))

    def test_configuration_and_path_rejected_before_connect(self):
        bad_urls = ['', 'ftp://host', 'http://u:p@host', 'http://host?x=1', 'http://host?',
                    'http://host#', 'http://host:0', 'http://host:99999', 'http://host:bad',
                    'http://host/a//b', 'http://host/../b', 'http://host/a\nb', 'http://host/%2froot']
        bad_paths = ['/etc/passwd', '/dashboard', 'http://evil/collections', '//evil/collections',
                     '/collections//x', '/collections/../x', '/collections/.', '/collections/%2eX',
                     '/collections/%2E', '/collections/%2Fetc', '/collections/%5c', '/collections/;',
                     '/collections/\\x', '/collections/x#fragment', '/collections/é',
                     '/collections/x q', '/collections/x?q=a b', '/collections/x?x=\r\n', '/collections/%xx', '/collections/c/%252e%252e%252fcluster', '/collections/x?x=%0a']
        with patch.object(proxy, 'connect', side_effect=AssertionError('validation opened a connection')):
            for url in bad_urls:
                with self.subTest(url=url):
                    self.assertTrue(self.call('request GET /collections', ['admin'], {'url': url})[0].startswith('error:'))
            for path in bad_paths:
                with self.subTest(path=path):
                    body = json.dumps({'method': 'request', 'verb': 'GET', 'path': path})
                    self.assertTrue(self.call(body, ['admin'])[0].startswith('error:'))
            for body in ['request get /collections', 'request GET /collections?q=a b',
                         'request GET /collections {}', 'request HEAD /collections {}',
                         'request OPTIONS /collections {}', 'request POST /collections/x/points hello',
                         json.dumps({'method': 'request', 'verb': 'GET', 'path': '/collections', 'roles': ['admin']}),
                         json.dumps({'method': 'request', 'verb': 'GET', 'path': '/collections', 'text': 'request DELETE /collections/a'})]:
                with self.subTest(body=body):
                    self.assertTrue(self.call(body, ['admin'])[0].startswith('error:'))
        self.assertEqual(self.server.requests, [])

    def test_read_and_write_route_table_and_wrong_verbs(self):
        reads = [('GET', '/collections'), ('GET', '/collections/c'), ('HEAD', '/collections/c'),
                 ('GET', '/collections/c/exists'), ('GET', '/collections/c/points/123'),
                 ('GET', '/collections/c/points/550e8400-e29b-41d4-a716-446655440000'),
                 ('POST', '/collections/c/points'), ('POST', '/collections/c/facet')]
        read_suffixes = ['points/scroll', 'points/count', 'points/query', 'points/query/batch',
                         'points/query/groups', 'points/search', 'points/search/batch', 'points/search/groups',
                         'points/search/matrix/pairs', 'points/search/matrix/offsets', 'points/recommend',
                         'points/recommend/batch', 'points/recommend/groups', 'points/discover', 'points/discover/batch']
        reads += [('POST', '/collections/c/' + suffix) for suffix in read_suffixes]
        writes = [(verb, '/collections/c/' + suffix) for verb, suffix in [
            ('PUT', 'points'), ('PUT', 'points/vectors'), ('POST', 'points/delete'), ('POST', 'points/batch'),
            ('POST', 'points/vectors/delete'), ('POST', 'points/payload'), ('PUT', 'points/payload'),
            ('POST', 'points/payload/delete'), ('POST', 'points/payload/clear')]]
        for verb, path in reads:
            with self.subTest(verb=verb, path=path):
                self.assertEqual(proxy.required_role(verb, path), 'read')
        for verb, path in writes:
            with self.subTest(verb=verb, path=path):
                self.assertEqual(proxy.required_role(verb, path), 'write')
                self.assertEqual(proxy.required_role('GET', path), 'admin')
        for _, path in reads:
            self.assertEqual(proxy.required_role('DELETE', path), 'admin')
        for suffix in ['facet'] + read_suffixes:
            self.assertEqual(proxy.required_role('PUT', '/collections/c/' + suffix), 'admin')
        self.assertEqual(proxy.required_role('POST', '/collections/points/points/search'), 'read')
        self.assertEqual(proxy.required_role('GET', '/collections/aliases'), 'read')

    def test_unknown_suffixes_and_admin_operations_need_admin(self):
        paths = ['/collections/c/points/searchX', '/collections/c/points/search/',
                 '/collections/c/points/Search', '/collections/c/points/%73earch',
                 '/collections/c/points/search/batch/x', '/collections/c/points/no-such-id',
                 '/collections/c/points/18446744073709551616', '/collections/c/new-operation',
                 '/cluster', '/snapshots', '/issues', '/telemetry', '/metrics', '/locks',
                 '/collections/c/snapshots/recover', '/collections/c/shards/1/snapshots/recover',
                 '/collections/c/snapshots/upload', '/collections/c/index', '/aliases']
        with patch.object(proxy, 'connect', side_effect=AssertionError('unauthorized connection')):
            for path in paths:
                with self.subTest(path=path):
                    self.assertEqual(proxy.required_role('POST', path), 'admin')
                    self.assertIn('requires admin role', self.call('request POST ' + path + ' {}', ['write'])[0])
        self.assertEqual(self.server.requests, [])

    def test_read_cannot_write_write_cannot_admin_and_no_role_default(self):
        with patch.object(proxy, 'connect', side_effect=AssertionError('unauthorized connection')):
            self.assertIn('requires write role', self.call('request PUT /collections/c/points {}', ['read'])[0])
            self.assertIn('requires admin role', self.call('request DELETE /collections/c', ['write'])[0])
            self.assertIn('requires admin role', self.call('request GET /collections/c/new', [])[0])
        for roles in (None, [], ['unrecognized'], ['write'], ['read', 'write']):
            output = self.call('request PUT /collections/c/points {}', roles)[0]
            self.assertTrue(self.decoded(output)['result'])
        for roles in (None, [], ['write'], ['unrecognized']):
            self.assertIn('requires admin role', self.call('request PUT /collections/c {}', roles)[0])
        for roles in (['admin'], ['owner'], ['maintainer'], ['read', 'owner']):
            self.assertTrue(self.decoded(self.call('request DELETE /collections/c', roles)[0])['result'])
        self.assertEqual(len(self.server.requests), 9)

    def test_explicit_read_wins_over_default_body_and_environment_claims(self):
        with patch.object(proxy, 'connect', side_effect=AssertionError('body/env role escalation')):
            for roles in (['read'], ['read', 'unknown']):
                with patch.dict(os.environ, {'AB_ROLE_ADMIN': '1'}):
                    output = self.call('request PUT /collections/c/points {"roles":["admin"],"header":"AB_ROLE_ADMIN=1"}', roles)[0]
                    self.assertIn('requires write role', output)
            self.assertTrue(self.call(json.dumps({'method': 'request', 'verb': 'PUT', 'path': '/collections/c/points', 'body': '{}', 'roles': ['admin']}), ['read'])[0].startswith('error:'))
        self.assertEqual(self.server.requests, [])

    def test_admin_passes_all_admin_roots_and_unknown_api_paths(self):
        for target in ['/cluster', '/issues', '/telemetry', '/metrics', '/locks', '/quotas',
                       '/snapshots', '/aliases', '/collections/c/snapshots/recover',
                       '/collections/c/snapshots/upload', '/collections/c/shards/1/snapshots/recover',
                       '/collections/c/new-operation']:
            with self.subTest(target=target):
                self.assertTrue(self.decoded(self.call('request POST ' + target + ' {}', ['admin'])[0])['result'])
        self.assertEqual(len(self.server.requests), 12)

    def test_content_type_error_status_headers_and_key_redaction(self):
        self.server.payload = json.dumps({KEY: KEY}).replace('fixture', '\\u0066ixture').encode()
        output, _, log = self.call('request GET /collections', ['read'])
        self.assertEqual(self.decoded(output), {'***': '***'})
        self.assertNotIn(KEY, output + log)
        self.server.echo_header = KEY
        for verb, status in [('HEAD', 200), ('GET', 204)]:
            self.server.status = status
            output, _, log = self.call('request ' + verb + ' /collections', ['read'])
            value = self.decoded(output)
            self.assertEqual(value['status'], status)
            self.assertEqual(value['headers']['X-Echo'], '***')
            self.assertNotIn('body', value)
            self.assertNotIn(KEY, output + log)
        self.server.status = 429
        self.server.content_type = 'text/plain'
        self.server.payload = ('busy ' + KEY).encode()
        output, _, log = self.call('request GET /collections', ['read'])
        self.assertEqual(output, 'error: 429 busy ***')
        self.assertNotIn(KEY, output + log)
        self.server.status = 200
        self.server.payload = b'{"looks":"json"}'
        self.server.content_type = 'application/octet-stream'
        self.assertIn('unsupported Content-Type', self.call('request GET /collections', ['read'])[0])
        self.server.content_type = 'application/json'
        self.server.encoding = 'gzip'
        self.assertIn('encoded Qdrant answer', self.call('request GET /collections', ['read'])[0])

    def test_no_redirect_no_key_and_no_retry_after_uncertain_write(self):
        self.server.status = 302
        output = self.call('request GET /collections', ['read'])[0]
        self.assertTrue(output.startswith('error: 302'))
        self.assertEqual(len(self.server.requests), 1)
        self.assertNotIn('/collections/stolen', [r[1] for r in self.server.requests])
        self.assertEqual(self.call('request GET /collections', ['read'], key='')[1], 78)
        self.assertEqual(len(self.server.requests), 1)
        self.server.status = 200
        self.server.delay = .2
        deadline = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=.18)).isoformat()
        output = self.call('request PUT /collections/c/points {}', ['write'], deadline=deadline)[0]
        self.assertIn('timed out', output)
        self.assertEqual(len(self.server.requests), 2)

    def test_response_request_caps_deadline_and_alarm_cleanup(self):
        self.server.payload = b'x' * (proxy.MAX_BYTES + 1)
        self.assertIn('exceeds 1048576 bytes', self.call('request GET /collections', ['read'])[0])
        self.assertEqual(len(self.server.requests), 1)
        too_large = json.dumps({'method': 'request', 'verb': 'PUT', 'path': '/collections/c/points', 'body': json.dumps({'padding':'x' * proxy.MAX_BYTES})})
        self.assertIn('exceeds 1048576 bytes', self.call(too_large, ['write'])[0])
        self.assertEqual(len(self.server.requests), 1)
        deadline = (datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(seconds=1)).isoformat()
        self.assertIn('timed out', self.call('request GET /collections', ['read'], deadline=deadline)[0])
        self.assertEqual(len(self.server.requests), 1)
        self.server.payload = b'"' + b'x' * 100 + b'"'
        self.server.trickle = .005
        deadline = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=.2)).isoformat()
        old = signal.getsignal(signal.SIGALRM)
        start = time.monotonic()
        self.assertIn('timed out', self.call('request GET /collections', ['read'], deadline=deadline)[0])
        self.assertLess(time.monotonic() - start, .4)
        self.assertEqual(signal.getsignal(signal.SIGALRM), old)
        self.assertEqual(signal.getitimer(signal.ITIMER_REAL), (0., 0.))

    def test_escaped_suffix_stays_admin_for_read_and_is_forwarded_for_admin(self):
        path = '/collections/c/points/%73earch'
        self.assertIn('requires admin role', self.call('request POST ' + path + ' {}', ['read'])[0])
        self.assertEqual(len(self.server.requests), 0)
        self.assertTrue(self.decoded(self.call('request POST ' + path + ' {}', ['admin'])[0])['result'])
        self.assertEqual(self.server.requests[-1][1], path)

    def test_mid_response_write_is_uncertain_and_connect_failure_is_not_sent(self):
        self.server.cut = True
        output = self.call('request PUT /collections/c/points {}', ['write'])[0]
        self.assertIn('write outcome unknown', output)
        self.assertEqual(len(self.server.requests), 1)
        self.assertNotIn(KEY, output)
        connection = Mock()
        connection.connect.side_effect = ConnectionRefusedError(KEY)
        with patch.object(proxy, 'connect', return_value=connection):
            output = self.call('request PUT /collections/c/points {}', ['write'])[0]
        self.assertIn('request not sent', output)
        self.assertNotIn('outcome unknown', output)
        self.assertNotIn(KEY, output)
        connection.request.assert_not_called()

    def test_stream_read_never_asks_for_more_than_cap_plus_one(self):
        class BoundedResponse:
            status = 200
            def getheader(inner, name, default=None):
                return {'Content-Type':'text/plain', 'Content-Encoding':'identity'}.get(name, default)
            def read(inner, limit=None):
                self.assertEqual(limit, proxy.MAX_BYTES + 1, 'unbounded upstream read')
                return b'ok'
        connection = Mock()
        connection.getresponse.return_value = BoundedResponse()
        with patch.object(proxy, 'connect', return_value=connection):
            self.assertEqual(self.call('request GET /collections', ['read'])[0], 'ok')
        connection.connect.assert_called_once_with()
        connection.close.assert_called_once_with()

    def test_generated_help_manifest_and_missing_key_entrypoint(self):
        env = {k: v for k, v in os.environ.items() if not k.startswith(('AGENT_BUS_', 'AB_ROLE_', 'QDRANT_', 'PYTHON'))}
        env['PYTHONDONTWRITEBYTECODE'] = '1'
        for ask in ['help', 'help-config', 'help-secret', 'help-json']:
            output = subprocess.check_output([sys.executable, str(HERE / 'qdrant.py')], env={**env, 'AGENT_BUS_ASK': ask}, text=True)
            self.assertTrue(output.strip())
            self.assertNotIn(KEY, output)
        result = subprocess.run([sys.executable, str(HERE / 'qdrant.py')], env=env, text=True, capture_output=True)
        self.assertEqual((result.returncode, result.stdout.strip()), (78, 'error: QDRANT_API_KEY is not set'))
        manifest = json.loads((HERE / 'agent.json').read_text())
        self.assertEqual(manifest['roles'], ['read', 'write', 'admin'])
        self.assertEqual(set(manifest['methods']), {'request'})
        self.assertEqual(set(manifest['config']), {'url'})
        self.assertNotIn('default', manifest['config']['url'])


if __name__ == '__main__':
    unittest.main()
