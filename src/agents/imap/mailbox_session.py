"""One read-only connection for polling; no bus, routes or delivery state."""
import base64
import hashlib
import imaplib
import json
import time

from budget import alarm_budget
from cache import Cache
from mail import tls_context
from mail_oauth import google_access_token
import wire


class SessionError(ValueError):
    def __init__(self, code):
        super().__init__('IMAP ' + code)
        self.code = code


class Session:
    def __init__(self, settings, environment, work, clock=time.monotonic):
        self.settings, self.environment, self.work, self.clock = settings, environment, work, clock
        self.connection, self.cfg = None, None
        self.identity = self.validity = self.uidnext = None
        self.masks, self.next_connect, self.auth_failures = [], 0, 0

    def close(self):
        if self.connection is not None:
            try:
                self.connection.shutdown()
            except OSError:
                pass
        self.connection = None

    def snapshot(self):
        if self.connection is None and self.clock() < self.next_connect:
            raise SessionError('authentication backoff')
        phase = 'configuration'
        try:
            with alarm_budget(10):
                if self.connection is None:
                    self.cfg = self.settings()
                    user = self.environment.get('IMAP_USERNAME', '')
                    password = self.environment.get('IMAP_PASSWORD', '')
                    if not isinstance(user, str) or not user or not user.isascii() or any(ord(c) < 32 or ord(c) == 127 for c in user):
                        raise SessionError('configuration')
                    if self.cfg['auth'] == 'password' and (not isinstance(password, str) or not password or not password.isascii() or any(ord(c) < 32 or ord(c) == 127 for c in password)):
                        raise SessionError('configuration')
                    self.masks = [self.environment.get(key, '') for key in ('IMAP_PASSWORD', 'IMAP_CLIENT_SECRET', 'IMAP_REFRESH_TOKEN')]
                    phase = 'authentication'
                    token = google_access_token(self.environment, 10, prefix='IMAP') if self.cfg['auth'] == 'xoauth2' else None
                    if self.cfg['auth'] == 'xoauth2' and not token:
                        raise SessionError('authentication')
                    phase = 'connection'
                    if self.cfg['mode'] == 'tls':
                        self.connection = wire.TLS(self.cfg['host'], self.cfg['port'],
                                                   ssl_context=tls_context(self.cfg.get('cafile')), timeout=10)
                    else:
                        self.connection = wire.Plain(wire.local_host(self.cfg['host'], self.cfg['port']), self.cfg['port'], timeout=10)
                    phase = 'authentication'
                    if token:
                        if 'AUTH=XOAUTH2' not in self.connection.capabilities:
                            raise SessionError('authentication')
                        initial = ('user=' + user + '\x01auth=Bearer ' + token + '\x01\x01').encode()
                        self.masks.extend([token, base64.b64encode(initial).decode()])
                        sent = False
                        def authenticate(_challenge):
                            nonlocal sent
                            if sent:
                                return b''
                            sent = True
                            return initial
                        self.connection.authenticate('XOAUTH2', authenticate)
                    else:
                        self.connection.login('"' + user.replace('\\', '\\\\').replace('"', '\\"') + '"', password)
                    self.identity = hashlib.sha256(json.dumps([self.cfg['host'], self.cfg['port'], user,
                                                              self.cfg['mailbox']], ensure_ascii=False).encode()).hexdigest()
                phase = 'mailbox'
                self.validity, self.uidnext = wire.selected(self.connection, self.cfg['mailbox'])
                self.auth_failures = 0
            return self.identity, self.validity, self.uidnext
        except (OSError, ValueError, imaplib.IMAP4.error) as error:
            self.close()
            if phase in {'configuration', 'authentication'} or getattr(error, 'throttled', False):
                self.auth_failures += 1
                self.next_connect = self.clock() + min(900 * 2**min(self.auth_failures - 1, 2), 3600)
            else:
                self.next_connect = self.clock() + 60
            raise SessionError(phase) from None

    def cache(self):
        if self.identity is None:
            raise SessionError('not connected')
        return Cache(self.work, self.identity, self.validity, self.cfg['cache_bytes'])

    def search(self, start, end):
        return self.operation(lambda: wire.search(self.connection, start, end))

    def operation(self, fn):
        if self.connection is None:
            raise SessionError('not connected')
        try:
            with alarm_budget(5):
                return fn()
        except wire.Refused as error:
            if error.throttled:
                self.close()
                self.auth_failures += 1
                self.next_connect = self.clock() + min(900 * 2**min(self.auth_failures - 1, 2), 3600)
            raise  # A per-UID NO is bounded and retried by discovery.
        except wire.Tainted:
            self.close()
            self.next_connect = self.clock() + 60
            raise
        except (OSError, imaplib.IMAP4.error):
            self.close()
            self.next_connect = self.clock() + 60
            raise SessionError('connection') from None

    def headers(self, number):
        def read():
            with self.cache() as cache:
                if cache.has(number):
                    raw = cache.get(number, touch=False)
                    return raw.partition(b'\r\n\r\n')[0].partition(b'\n\n')[0] + b'\r\n\r\n'
                return wire.headers(self.connection, number)
        return self.operation(read)

    def body(self, number):
        def read():
            with self.cache() as cache:
                if not cache.has(number):
                    raw, _ = wire.fetch(self.connection, number)
                    cache.put(number, raw)
                return cache.get(number)
        return self.operation(read)

    def legacy(self):
        with self.cache() as cache:
            return dict(cache.state)
