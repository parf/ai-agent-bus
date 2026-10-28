import base64
import contextlib
import datetime
from email import policy
from email.parser import BytesParser
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import socket
import socketserver
import ssl
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('mail_smtp', os.environ.get('SMTP_SOURCE', str(HERE / 'smtp.py')))
smtp = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smtp)
USER, PASSWORD = 'fixture-account', 'fixture-password-012345'


class Handler(socketserver.BaseRequestHandler):
    def handle(self):
        sock = self.request
        sock.settimeout(2)
        secured = self.server.mode == 'tls'
        try:
            if secured:
                sock = self.server.context.wrap_socket(sock, server_side=True)
            reader = sock.makefile('rb')
            def send(line):
                sock.sendall(line + b'\r\n')
            send(b'220 fixture ESMTP')
            while True:
                line = reader.readline(4097)
                if not line:
                    break
                self.server.commands.append((secured, line))
                verb = line.split(b' ', 1)[0].strip().upper()
                if verb == b'EHLO':
                    send(b'250-fixture')
                    if self.server.starttls and not secured:
                        send(b'250-STARTTLS')
                    send(b'250 AUTH ' + (b'PLAIN' if self.server.auth else b'OTHER'))
                elif verb == b'STARTTLS':
                    if self.server.fail_tls:
                        send(b'454 TLS unavailable')
                    else:
                        send(b'220 upgrade')
                        reader.close()
                        sock = self.server.context.wrap_socket(sock, server_side=True)
                        reader = sock.makefile('rb')
                        secured = True
                elif verb == b'AUTH':
                    self.server.auths.append(line)
                    if not self.server.password_ok:
                        send(b'535 ' + PASSWORD.encode())
                    elif base64.b64decode(line.split()[2]) != b'\x00' + USER.encode() + b'\x00' + PASSWORD.encode():
                        send(b'535 bad credentials')
                    else:
                        send(b'235 authenticated')
                elif verb == b'MAIL':
                    self.server.senders.append(line)
                    send(b'250 sender ok')
                elif verb == b'RCPT':
                    self.server.recipients.append(line)
                    send(b'550 refused ' + PASSWORD.encode() if b'blocked@' in line else b'250 recipient ok')
                elif verb == b'DATA':
                    send(b'354 send data')
                    raw = bytearray()
                    while True:
                        chunk = reader.readline(2 * smtp.MAX_BYTES)
                        if chunk in (b'.\r\n', b''):
                            break
                        raw.extend(chunk[1:] if chunk.startswith(b'..') else chunk)
                    self.server.messages.append(bytes(raw))
                    if self.server.cut:
                        break
                    if self.server.delay:
                        time.sleep(self.server.delay)
                    send(str(self.server.data_code).encode() + b' fixture-queue-id')
                elif verb == b'RSET':
                    send(b'250 reset')
                else:
                    send(b'221 bye')
                    break
            reader.close()
        except (ssl.SSLError, OSError):
            pass
        finally:
            sock.close()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


class SMTPTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        scratch = HERE.parents[2] / 'tmp'
        scratch.mkdir(exist_ok=True)
        cls.temp = tempfile.TemporaryDirectory(dir=scratch, prefix='smtp-fixture-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.cert, cls.key = [str(Path(cls.temp.name) / name) for name in ('cert.pem', 'key.pem')]
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', cls.key,
                        '-out', cls.cert, '-days', '1', '-subj', '/CN=fixture', '-addext', 'subjectAltName=IP:127.0.0.1'],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    def setUp(self):
        self.server = Server(('127.0.0.1', 0), Handler)
        self.server.context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        self.server.context.load_cert_chain(self.cert, self.key)
        self.server.mode = 'starttls'
        self.server.starttls = self.server.auth = self.server.password_ok = True
        self.server.fail_tls = self.server.cut = False
        self.server.delay = 0
        self.server.data_code = 250
        self.server.commands, self.server.messages = [], []
        self.server.auths, self.server.senders, self.server.recipients = [], [], []
        thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': .01}, daemon=True)
        thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(thread.join)
        self.addCleanup(self.server.shutdown)
        actual = socket.create_connection
        def only_fixture(address, *args, **kwargs):
            if address != ('127.0.0.1', self.server.server_address[1]):
                raise AssertionError('external connection forbidden')
            return actual(address, *args, **kwargs)
        self.addCleanup(patch.stopall)
        patch('socket.create_connection', side_effect=only_fixture).start()
        self.config = {'host': '127.0.0.1', 'port': self.server.server_address[1], 'mode': 'starttls',
                       'sender': 'sender@example.com', 'cafile': self.cert}
        self.env = {'SMTP_USERNAME': USER, 'SMTP_PASSWORD': PASSWORD}
        self.body = {'method': 'send', 'to': ['Bob@Example.COM'], 'subject': 'Fixture', 'text': 'first\n.\nlast'}
        self.envelope = {'message_id': 'daemon-delivered-id', 'body': json.dumps(self.body)}

    def call(self, **kwargs):
        if 'body' in kwargs:
            self.envelope['body'] = json.dumps(kwargs['body'])
        return json.loads(smtp.answer(self.envelope, self.config, self.env)[0])

    def test_starttls_once_recipient_bcc_and_crlf(self):
        self.body.update(cc=['copy@example.com'], bcc=['blind@example.com'])
        result = self.call(body=self.body)
        self.assertEqual(result['outcome'], 'accepted')
        self.assertEqual(len(self.server.messages), 1)
        self.assertEqual(len(self.server.auths), 1)
        self.assertEqual(self.server.senders, [b'mail from:<sender@example.com>\r\n'])
        self.assertEqual(self.server.recipients, [b'rcpt to:<Bob@example.com>\r\n', b'rcpt to:<copy@example.com>\r\n', b'rcpt to:<blind@example.com>\r\n'])
        raw = self.server.messages[0]
        self.assertIsNone(re.search(rb'(?<!\r)\n|\r(?!\n)', raw))
        parsed = BytesParser(policy=policy.default).parsebytes(raw)
        self.assertIsNone(parsed['Bcc'])
        self.assertEqual(str(parsed['From']), 'sender@example.com')
        self.assertEqual(parsed.get_content(), 'first\r\n.\r\nlast\r\n')
        commands = self.server.commands
        auth_at = next(i for i, (_, line) in enumerate(commands) if line.startswith(b'AUTH'))
        self.assertTrue(commands[auth_at][0])
        self.assertEqual([secure for secure, line in commands[:auth_at] if line.startswith(b'ehlo')], [False, True])

    def test_implicit_tls(self):
        self.server.mode = self.config['mode'] = 'tls'
        self.assertEqual(self.call()['outcome'], 'accepted')
        self.assertTrue(all(secure for secure, _ in self.server.commands))
        self.assertFalse(any(line.startswith(b'STARTTLS') for _, line in self.server.commands))

    def test_missing_starttls_and_failed_upgrade_never_auth(self):
        self.server.starttls = False
        self.assertEqual(self.call()['outcome'], 'not sent')
        self.server.starttls = True
        self.server.fail_tls = True
        self.assertEqual(self.call()['outcome'], 'refused')
        self.assertEqual(self.server.auths, [])
        self.assertEqual(self.server.messages, [])

    def test_certificate_verification_and_hostname(self):
        self.config.pop('cafile')
        self.assertEqual(self.call()['outcome'], 'not sent')
        self.assertEqual(self.server.auths, [])
        context = smtp.tls_context(self.cert)
        self.assertTrue(context.check_hostname)
        self.assertEqual(context.verify_mode, ssl.CERT_REQUIRED)
        self.assertGreaterEqual(context.minimum_version, ssl.TLSVersion.TLSv1_2)
        # Same trusted CA but wrong name, on a local socket, never DNS/provider I/O.
        self.server.mode = 'tls'
        with socket.create_connection(('127.0.0.1', self.server.server_address[1])) as conn:
            with self.assertRaises(ssl.SSLCertVerificationError):
                context.wrap_socket(conn, server_hostname='wrong.example')

    def test_recipient_allowlist_exact_domain_and_local_case(self):
        self.config['recipients'] = ['example.com']
        self.assertEqual(self.call()['outcome'], 'accepted')
        self.config['recipients'] = ['Bob@example.com']
        self.assertEqual(self.call()['outcome'], 'accepted')
        count = len(self.server.commands)
        for target in ['Bob@example.com.evil.test', 'bob@example.com']:
            self.body['to'] = [target]
            self.assertEqual(self.call(body=self.body)['outcome'], 'not sent')
        self.assertEqual(len(self.server.commands), count)

    def test_invalid_headers_sender_and_recipients_never_connect(self):
        for delta in [{'subject': 'hi\r\nBcc: other@example.com'}, {'from': 'other@example.com'},
                      {'to': ['Name <bob@example.com>']}, {'to': ['a@example.com,b@example.com']},
                      {'to': ['a\r\n@example.com']}, {'to': ['a@éxample.com']},
                      {'to': ['a@example.com'] * 21}, {'subject': 'x' * 999}]:
            self.assertEqual(self.call(body={**self.body, **delta})['outcome'], 'not sent')
        self.assertEqual(self.server.commands, [])

    def test_message_id_from_envelope_only(self):
        cfg = smtp.configuration(self.config)
        _, a = smtp.build(self.envelope, cfg)
        _, b = smtp.build(self.envelope, cfg)
        parse = lambda raw: BytesParser(policy=policy.default).parsebytes(raw)['Message-ID']
        self.assertEqual(parse(a), parse(b))
        self.assertTrue(str(parse(a)).endswith('@example.com>'))
        self.envelope['message_id'] = 'different-daemon-id'
        self.assertNotEqual(parse(a), parse(smtp.build(self.envelope, cfg)[1]))
        self.assertEqual(self.call(body={**self.body, 'message_id': 'forged'})['outcome'], 'not sent')

    def test_refusals_and_secret_redaction(self):
        self.server.password_ok = False
        stream = io.StringIO()
        with contextlib.redirect_stderr(stream):
            result = self.call()
        self.assertEqual(result['outcome'], 'refused')
        self.assertNotIn(PASSWORD, json.dumps(result) + stream.getvalue())
        self.assertEqual(self.server.messages, [])
        self.server.password_ok = True
        self.server.data_code = 451
        self.assertEqual(self.call()['outcome'], 'refused')
        self.assertEqual(len(self.server.messages), 1)

    def test_partial_and_all_recipient_refusal(self):
        result = self.call(body={**self.body, 'cc': ['blocked@example.com']})
        self.assertEqual(result['outcome'], 'partial')
        self.assertEqual(result['accepted'], ['Bob@example.com'])
        self.assertEqual(set(result['refused']), {'blocked@example.com'})
        self.assertNotIn(PASSWORD, json.dumps(result))
        self.assertEqual(len(self.server.messages), 1)
        self.assertEqual(self.call(body={**self.body, 'to': ['blocked@example.com']})['outcome'], 'refused')
        self.assertEqual(len(self.server.messages), 1)

    def test_unknown_disconnect_no_retry_and_overall_deadline(self):
        self.server.cut = True
        self.assertIn('unknown', self.call()['outcome'])
        self.assertEqual(len(self.server.messages), 1)
        self.server.cut = False
        self.server.delay = .4
        self.envelope['deadline'] = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=.3)).isoformat()
        started = time.monotonic()
        self.assertIn('unknown', self.call()['outcome'])
        self.assertLess(time.monotonic() - started, .5)
        self.assertEqual(len(self.server.messages), 2)

    def test_bounds_credentials_and_plaintext_optout(self):
        self.body['text'] = 'a' * smtp.MAX_BYTES
        self.assertEqual(self.call(body=self.body)['outcome'], 'not sent')
        self.env = {}
        self.assertEqual(self.call(body={**self.body, 'text': 'ok'})['outcome'], 'not sent')
        self.assertEqual(self.server.commands, [])
        self.env = {'SMTP_USERNAME': USER, 'SMTP_PASSWORD': PASSWORD}
        self.config['mode'] = 'plain'
        self.assertEqual(self.call(body={**self.body, 'text': 'ok'})['outcome'], 'not sent')
        self.config['allow_plaintext'] = True
        self.assertEqual(self.call()['outcome'], 'accepted')
        self.assertFalse(any(secure for secure, _ in self.server.commands))

    def test_unsupported_auth_and_bounded_multiline_reply(self):
        self.server.auth = False
        self.assertEqual(self.call()['outcome'], 'not sent')
        self.assertEqual(self.server.auths, [])
        client = smtp.SMTP()
        client.file = io.BytesIO(b'250-' + b'x' * 4000 + b'\r\n' + (b'250-' + b'x' * 4000 + b'\r\n') * 20)
        with self.assertRaises(smtp.MailError):
            client.getreply()
        client.file = None

    def test_metadata_without_credentials(self):
        env = {key: value for key, value in os.environ.items() if not key.startswith(('SMTP_', 'AGENT_BUS_'))}
        env['AGENT_BUS_ASK'] = 'help'
        result = subprocess.run([sys.executable, '-B', str(HERE / 'smtp.py')], env=env, capture_output=True, timeout=5, text=True)
        self.assertEqual(result.returncode, 0)
        self.assertIn('send', result.stdout)
        self.assertNotIn(PASSWORD, result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
