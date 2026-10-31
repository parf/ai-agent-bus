"""One read-only connection for polling; no bus, routes or delivery state."""
import base64
import hashlib
import imaplib
import json
import time
import select

from budget import alarm_budget
from cache import Cache
from mail import tls_context
from mail_oauth import google_access_token
import wire
import folders


class SessionError(ValueError):
    def __init__(self, code):
        super().__init__('IMAP ' + code)
        self.code = code


class Session:
    def __init__(self, settings, environment, work, clock=time.monotonic, *, folder=None):
        self.settings, self.environment, self.work, self.clock = settings, environment, work, clock
        self.connection, self.cfg = None, None
        self.identity = self.validity = self.uidnext = None
        self.masks, self.next_connect, self.auth_failures = [], 0, 0
        self.backoff_reason = None
        self.folder, self.default_mailbox = folder, None
        self.idle_tag, self.idle_since, self.idle_refused = None, 0, False

    def mailbox_name(self):
        return self.settings()['mailbox']

    def for_folder(self, name):
        return Session(self.settings, self.environment, self.work, self.clock, folder=name)

    def close(self):
        if self.connection is not None:
            try:
                self.connection.shutdown()
            except OSError:
                pass
        self.connection = None
        self.idle_tag = None

    def snapshot(self):
        if self.connection is None and self.clock() < self.next_connect:
            raise SessionError(self.backoff_reason or 'connection backoff')
        phase = 'configuration'
        try:
            with alarm_budget(10):
                phase = 'connection'
                self.finish_idle()
                if self.connection is None:
                    phase = 'configuration'
                    self.cfg = self.settings()
                    self.default_mailbox = self.cfg['mailbox']
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
                    if self.folder is not None:
                        chosen = folders.choose(self.connection, self.folder)
                        self.cfg = dict(self.cfg, mailbox=self.default_mailbox if folders.same(chosen, self.default_mailbox)
                                        else 'INBOX' if chosen.casefold() == 'inbox' else chosen)
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
                self.backoff_reason = 'authentication backoff'
            else:
                self.next_connect = self.clock() + 60
                self.backoff_reason = 'connection backoff'
            raise SessionError(phase) from None

    def cache(self):
        if self.identity is None:
            raise SessionError('not connected')
        return Cache(self.work, self.identity, self.validity, self.cfg['cache_bytes'],
                     folder=not folders.same(self.cfg['mailbox'], self.default_mailbox))

    def start_idle(self):
        if (self.connection is None or self.idle_tag is not None or self.idle_refused
                or 'IDLE' not in self.connection.capabilities):
            return False
        try:
            with alarm_budget(3):
                for key in ('EXISTS', 'RECENT', 'EXPUNGE'):
                    self.connection.untagged_responses.pop(key, None)
                tag = self.connection._new_tag()
                self.connection.response_bytes = 0
                self.connection.send(tag + b' IDLE\r\n')
                while self.connection.tagged_commands[tag] is None:
                    if self.connection._get_response() is None:
                        self.idle_tag, self.idle_since = tag, self.clock()
                        return True
                self.connection.tagged_commands.pop(tag)
                self.idle_refused = True
                return False
        except (OSError, ValueError, imaplib.IMAP4.error):
            self.close()
            raise SessionError('IDLE connection') from None

    def idle_ready(self):
        if self.idle_tag is None: return False
        # Periodic DONE also drains a response already read ahead into the
        # buffered line stream, without a blocking peek or a separate reader.
        return (self.clock() - self.idle_since >= 10
                or bool(getattr(self.connection.sock, 'pending', lambda: 0)())
                or bool(select.select([self.connection.sock], [], [], 0)[0]))

    def finish_idle(self):
        if self.idle_tag is None: return False
        tag, self.idle_tag = self.idle_tag, None
        try:
            with alarm_budget(3):
                self.connection.send(b'DONE\r\n')
                status, _ = self.connection._command_complete('IDLE', tag)
                if status != 'OK': self.idle_refused = True
                return any(self.connection.untagged_responses.get(key) for key in ('EXISTS', 'RECENT', 'EXPUNGE'))
        except (OSError, ValueError, imaplib.IMAP4.error):
            self.close()
            raise SessionError('IDLE connection') from None

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
                self.backoff_reason = 'authentication backoff'
                raise SessionError('authentication backoff') from None
            raise  # Only a healthy per-UID NO consumes a discovery try.
        except wire.LiteralTooLarge:
            self.close()
            self.next_connect = self.clock() + 60
            self.backoff_reason = 'connection backoff'
            raise
        except wire.Tainted:
            self.close()
            self.next_connect = self.clock() + 60
            self.backoff_reason = 'connection backoff'
            raise SessionError('connection') from None
        except (OSError, imaplib.IMAP4.error):
            self.close()
            self.next_connect = self.clock() + 60
            self.backoff_reason = 'connection backoff'
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
