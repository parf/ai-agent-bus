import base64
import contextlib
import io
import datetime
import http.client
import http.server
import json
from pathlib import Path
import socket
import ssl
import threading
import time
import unittest
from unittest.mock import patch
import urllib.parse

import test_smtp as fixtures
import mail_oauth

TOKEN = 'fixture-access-token-012345'
REFRESH = 'fixture-refresh-token-012345'
SECRET = 'fixture-client-secret-012345'


class OAuthHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        body = self.rfile.read(int(self.headers['Content-Length']))
        self.server.requests.append((self.path, body, dict(self.headers)))
        if self.server.delay:
            time.sleep(self.server.delay)
        self.send_response(self.server.status)
        self.send_header('Content-Type', self.server.content_type)
        if self.server.declared is not False:
            self.send_header('Content-Length', str(self.server.declared if self.server.declared is not None else len(self.server.payload)))
        self.send_header('Location', 'https://evil.example/token')
        self.end_headers()
        try:
            self.wfile.write(self.server.payload[:1] if self.server.cut else self.server.payload)
        except (OSError, ssl.SSLError):
            pass


class OAuthTests(unittest.TestCase):
    setUpClass = classmethod(fixtures.SMTPTests.setUpClass.__func__)
    call = fixtures.SMTPTests.call

    def setUp(self):
        fixtures.SMTPTests.setUp(self)
        self.config['auth'] = 'xoauth2'
        self.server.auth_names = b'XOAUTH2'
        self.server.oauth_error = (REFRESH + SECRET + TOKEN).encode()
        self.env = {'SMTP_USERNAME': fixtures.USER, 'SMTP_CLIENT_ID': 'fixture-client-id',
                    'SMTP_CLIENT_SECRET': SECRET, 'SMTP_REFRESH_TOKEN': REFRESH}
        self.token_server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), OAuthHandler)
        self.token_server.socket = self.server.context.wrap_socket(self.token_server.socket, server_side=True)
        self.token_server.requests = []
        self.token_server.delay = 0
        self.token_server.content_type = 'application/json'
        self.token_server.declared = None
        self.token_server.cut = False
        self.token_server.status = 200
        self.token_server.payload = json.dumps({'access_token': TOKEN, 'token_type': 'Bearer', 'expires_in': 3600}).encode()
        thread = threading.Thread(target=self.token_server.serve_forever, kwargs={'poll_interval': .01}, daemon=True)
        thread.start()
        self.addCleanup(self.token_server.server_close)
        self.addCleanup(thread.join)
        self.addCleanup(self.token_server.shutdown)
        # No production token_url knob. Only this test swaps the fixed Google transport.
        # The SMTP fixture installed a single-port guard; replace it with a two-port guard.
        patch.stopall()
        actual = socket.create_connection
        ports = {self.server.server_address[1], self.token_server.server_port}
        def only_fixture(address, *args, **kwargs):
            if address[0] != '127.0.0.1' or address[1] not in ports:
                raise AssertionError('external connection forbidden')
            return actual(address, *args, **kwargs)
        patch('socket.create_connection', side_effect=only_fixture).start()
        https = http.client.HTTPSConnection
        real = mail_oauth.token_connection(1)
        self.assertEqual(real.host, 'oauth2.googleapis.com')
        self.assertTrue(real._context.check_hostname)
        self.assertEqual(real._context.verify_mode, ssl.CERT_REQUIRED)
        self.assertGreaterEqual(real._context.minimum_version, ssl.TLSVersion.TLSv1_2)
        real.close()
        def token_connection(timeout):
            return https('127.0.0.1', self.token_server.server_port, timeout=timeout, context=ssl.create_default_context(cafile=self.cert))
        patch('mail_oauth.token_connection', side_effect=token_connection).start()

    def test_xoauth2_refresh_payload_and_one_submission(self):
        result = self.call()
        self.assertEqual(result['outcome'], 'accepted')
        self.assertEqual(len(self.token_server.requests), 1)
        path, body, headers = self.token_server.requests[0]
        self.assertEqual(path, '/token')
        self.assertEqual(urllib.parse.parse_qs(body.decode()), {'grant_type': ['refresh_token'], 'client_id': ['fixture-client-id'],
                         'client_secret': [SECRET], 'refresh_token': [REFRESH]})
        self.assertEqual(headers['Accept-Encoding'], 'identity')
        self.assertEqual(base64.b64decode(self.server.auths[0].split()[2]),
                         b'user=' + fixtures.USER.encode() + b'\x01auth=Bearer ' + TOKEN.encode() + b'\x01\x01')
        self.assertEqual(len(self.server.messages), 1)
        self.assertEqual(len(self.server.auths), 1)
        for secret in (TOKEN, REFRESH, SECRET):
            self.assertNotIn(secret, json.dumps(result))

    def test_334_failure_sends_empty_line_and_redacts(self):
        self.server.password_ok = False
        stderr = io.StringIO()
        with contextlib.redirect_stderr(stderr):
            result = self.call()
        self.assertEqual(result['outcome'], 'refused')
        self.assertEqual(self.server.oauth_empty, b'\r\n')
        self.assertEqual(result['error'], 'XOAUTH2 authentication refused')
        self.assertEqual(len(self.server.messages), 0)
        for value in (REFRESH, SECRET, TOKEN):
            self.assertNotIn(value, json.dumps(result) + stderr.getvalue())

    def test_invalid_grant_and_redirect_no_retry_or_smtp(self):
        self.token_server.status = 400
        self.token_server.payload = json.dumps({'error': 'invalid_grant', 'error_description': REFRESH + SECRET}).encode()
        result = self.call()
        self.assertEqual(result['outcome'], 'not_sent')
        self.assertIn('invalid_grant', result['error'])
        for secret in (REFRESH, SECRET):
            self.assertNotIn(secret, json.dumps(result))
        self.token_server.status = 302
        self.assertEqual(self.call()['outcome'], 'not_sent')
        self.assertEqual(len(self.token_server.requests), 2)
        self.assertEqual(self.server.commands, [])

    def test_token_cap_missing_credentials_and_no_config_redirect(self):
        self.token_server.payload = b'x' * 65537
        self.assertIn('65536', self.call()['error'])
        before = len(self.token_server.requests)
        del self.env['SMTP_REFRESH_TOKEN']
        self.assertIn('SMTP_REFRESH_TOKEN', self.call()['error'])
        self.assertEqual(len(self.token_server.requests), before)
        self.config['token_url'] = 'https://evil.example/token'
        self.assertEqual(self.call()['outcome'], 'not_sent')
        self.assertEqual(self.server.commands, [])

    def test_invalid_token_shapes_fail_before_smtp(self):
        for payload in [b'not-json', b'{}', b'[]', b'{"access_token":"valid","token_type":42}',
                        b'{"access_token":"valid","token_type":"other"}']:
            self.token_server.payload = payload
            self.assertEqual(self.call()['outcome'], 'not_sent')
        self.assertEqual(self.server.commands, [])
        self.token_server.payload = b'{"access_token":"valid","token_type":42}'
        self.assertIn('OAuth response is invalid', self.call()['error'])

    def test_short_encoded_and_non_json_token_responses(self):
        self.token_server.declared = 2000
        self.token_server.cut = True
        self.assertIn('incomplete', self.call()['error'])
        self.token_server.cut = False
        self.token_server.declared = None
        self.token_server.content_type = 'text/plain'
        self.assertIn('not JSON', self.call()['error'])
        self.token_server.content_type = 'application/json'
        self.token_server.declared = False
        self.token_server.payload = b'x' * 65537
        self.assertIn('65536', self.call()['error'])
        self.assertEqual(self.server.commands, [])

    def test_password_and_oauth_inputs_never_fallback(self):
        self.config['auth'] = 'password'
        self.assertIn('SMTP_PASSWORD', self.call()['error'])
        self.assertEqual(self.token_server.requests, [])
        self.config['auth'] = 'xoauth2'
        self.env = {'SMTP_USERNAME': fixtures.USER, 'SMTP_PASSWORD': fixtures.PASSWORD}
        self.assertIn('SMTP_CLIENT_ID', self.call()['error'])
        self.assertEqual(self.server.commands, [])

    def test_echoed_access_token_is_redacted(self):
        self.server.acceptance = TOKEN.encode()
        stream = io.StringIO()
        with contextlib.redirect_stderr(stream):
            result = self.call()
        self.assertEqual(result['outcome'], 'accepted')
        self.assertEqual(result['server'], '***')
        self.assertNotIn(TOKEN, json.dumps(result) + stream.getvalue())

    def test_token_exchange_uses_operation_deadline(self):
        self.token_server.delay = .4
        self.envelope['deadline'] = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=.3)).isoformat()
        result = self.call()
        self.assertEqual(result['outcome'], 'not_sent')
        self.assertEqual(len(self.token_server.requests), 1)
        self.assertEqual(self.server.commands, [])

    def test_explicit_auth_never_falls_back(self):
        self.server.auth_names = b'PLAIN'
        self.assertEqual(self.call()['outcome'], 'not_sent')
        self.assertEqual(self.server.auths, [])
        self.assertEqual(self.server.messages, [])


if __name__ == '__main__':
    unittest.main()
