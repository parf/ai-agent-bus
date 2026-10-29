"""The read-only IMAP subset, with a budget before each literal allocation."""
import base64
import imaplib
import ipaddress
import re
import socket

from cache import MAX_MAIL, CacheError, uid

MAX_LINE = 32768
MAX_RESPONSE = MAX_MAIL + 65536
WINDOW = 1000


class WireError(ValueError):
    pass


class Refused(WireError):
    """A tagged refusal may be transient; sync must retry this UID later."""
    def __init__(self, message, throttled=False):
        super().__init__(message)
        self.throttled = throttled


class Tainted(WireError):
    """The stream cannot be reused after a bound or a truncated literal."""
    pass


class TooLarge(WireError):
    pass


class LiteralTooLarge(TooLarge, Tainted):
    pass


class Truncated(Tainted):
    pass


def mailbox(value):
    if not isinstance(value, str) or not value or len(value.encode()) > 1024 or any(ord(c) < 32 or ord(c) == 127 for c in value):
        raise WireError('mailbox must be nonempty text without controls, at most 1024 UTF-8 bytes')
    # IMAP modified UTF-7: printable ASCII passes through; '&' is '&-'.
    output, pending = [], []
    def flush():
        if pending:
            raw = ''.join(pending).encode('utf-16be')
            output.append('&' + base64.b64encode(raw).decode().rstrip('=').replace('/', ',') + '-')
            pending.clear()
    for char in value:
        if 32 <= ord(char) <= 126:
            flush()
            output.append('&-' if char == '&' else char)
        else:
            pending.append(char)
    flush()
    return '"' + ''.join(output).replace('\\', '\\\\').replace('"', '\\"') + '"'


class Bounded:
    def __init__(self, *args, **kwargs):
        self.response_bytes = 0
        super().__init__(*args, **kwargs)

    def _command(self, name, *args):
        self.response_bytes = 0
        return super()._command(name, *args)

    def buffer(self):
        # Python 3.14 changed imaplib.read to a separate socket buffer. Use one
        # stream for both lines and literals; failed operations close it, no IDLE.
        return self.__dict__.get('_file', self.__dict__.get('file'))

    def readline(self):
        raw = self.buffer().readline(MAX_LINE + 1)
        self.response_bytes += len(raw)
        if len(raw) > MAX_LINE or self.response_bytes > MAX_RESPONSE:
            raise Tainted('IMAP response exceeds its line or aggregate byte limit')
        return raw

    def read(self, size):
        if type(size) is not int or size < 0 or size > getattr(self, 'literal_limit', MAX_MAIL):
            raise LiteralTooLarge('IMAP literal exceeds its byte limit before allocation')
        if self.response_bytes + size > MAX_RESPONSE:
            raise Tainted('IMAP literal exceeds its byte limit before allocation')
        self.response_bytes += size
        raw = self.buffer().read(size)
        if len(raw) != size:
            raise Truncated('IMAP literal is truncated')
        return raw


class TLS(Bounded, imaplib.IMAP4_SSL):
    pass


class Plain(Bounded, imaplib.IMAP4):
    pass


def local_host(host, port):
    addresses = socket.getaddrinfo(host, port, type=socket.SOCK_STREAM)
    if not addresses or any(not ipaddress.ip_address(item[4][0]).is_loopback for item in addresses):
        raise WireError('plaintext IMAP is allowed only to a loopback server')
    return addresses[0][4][0]


def ok(result):
    kind, data = result
    if kind != 'OK':
        hints = b' '.join(item[:4096] for item in data[:8] if isinstance(item, bytes)).upper() if isinstance(data, list) else b''
        raise Refused('IMAP server refused the operation', any(word in hints for word in
                      (b'THROTTLED', b'TOO MANY CONNECTIONS', b'AUTHENTICATIONFAILED')))
    return data


def selected(connection, name):
    ok(connection.select(mailbox(name), readonly=True))
    values = []
    for key in ('UIDVALIDITY', 'UIDNEXT'):
        _, raw = connection.response(key)
        if key == 'UIDNEXT' and (not raw or raw == [None]):
            status = ok(connection.status(mailbox(name), '(UIDNEXT)'))
            if len(status) != 1 or not isinstance(status[0], bytes):
                raise WireError('IMAP STATUS omitted valid UIDNEXT')
            matches = re.findall(rb'\bUIDNEXT ([1-9][0-9]{0,9})\b', status[0])
            raw = matches
        if not raw or len(raw) != 1 or not isinstance(raw[0], bytes) or not re.fullmatch(rb'[1-9][0-9]{0,9}', raw[0]):
            raise WireError('IMAP select/status omitted valid UIDVALIDITY or UIDNEXT')
        values.append(uid(raw[0].decode('ascii')))
    return tuple(values)


def search(connection, start, end):
    start, end = uid(start), uid(end)
    if end < start or end - start >= WINDOW:
        raise WireError('invalid bounded UID search range')
    raw = ok(connection.uid('SEARCH', None, 'UID', f'{start}:{end}'))
    if len(raw) != 1 or not isinstance(raw[0], bytes) or len(raw[0]) > WINDOW * 11:
        raise WireError('IMAP UID list exceeds its byte limit')
    words = raw[0].split()
    if len(words) > WINDOW:
        raise WireError('IMAP UID list exceeds its count limit')
    numbers = [uid(word.decode('ascii')) for word in words]
    if len(set(numbers)) != len(numbers) or any(not start <= number <= end for number in numbers):
        raise WireError('IMAP server returned UIDs outside the requested range')
    return sorted(numbers)


def exists(connection, number):
    return search(connection, number, number) == [number]


def headers(connection, number):
    """FROM/SUBJECT only, with the header literal capped before allocation."""
    number = uid(number)
    previous = getattr(connection, 'literal_limit', MAX_MAIL)
    connection.literal_limit = 8192
    try:
        raw = ok(connection.uid('FETCH', str(number), '(UID BODY.PEEK[HEADER.FIELDS (FROM SUBJECT)])'))
    finally:
        connection.literal_limit = previous
    parts = [item for item in raw if isinstance(item, tuple)]
    if len(parts) != 1 or len(parts[0]) != 2 or not isinstance(parts[0][0], bytes) or not isinstance(parts[0][1], bytes):
        raise WireError('invalid sender/subject header metadata')
    header, body = parts[0]
    matched = re.search(rb'\bUID ([1-9][0-9]*)\b', header)
    declared = re.search(rb'\{([0-9]+)\}\Z', header)
    if not matched or int(matched[1]) != number or not declared or len(body) != int(declared[1]) or len(body) > 8192:
        raise WireError('mismatched or oversized sender/subject headers')
    return body


def fetch(connection, number):
    number = uid(number)
    raw = ok(connection.uid('FETCH', str(number), '(UID RFC822.SIZE)'))
    if len(raw) != 1 or not isinstance(raw[0], bytes):
        raise WireError('IMAP server returned invalid size metadata')
    matched_uid = re.search(rb'\bUID ([1-9][0-9]*)\b', raw[0])
    size = re.search(rb'\bRFC822\.SIZE ([0-9]+)\b', raw[0])
    if not matched_uid or int(matched_uid[1]) != number or not size:
        raise WireError('IMAP server returned mismatched UID/size metadata')
    size = int(size[1])
    if size == 0:
        raise WireError('mail has an invalid zero size')
    if size > MAX_MAIL:
        raise TooLarge('mail exceeds the 2097152-byte raw message limit')
    raw = ok(connection.uid('FETCH', str(number), '(UID BODY.PEEK[])'))
    parts = [item for item in raw if isinstance(item, tuple)]
    if len(parts) != 1 or len(parts[0]) != 2 or not isinstance(parts[0][0], bytes) or not isinstance(parts[0][1], bytes):
        raise WireError('IMAP server returned invalid body metadata')
    matched_uid = re.search(rb'\bUID ([1-9][0-9]*)\b', parts[0][0])
    body = parts[0][1]
    declared = re.search(rb'\{([0-9]+)\}\Z', parts[0][0])
    if not matched_uid or int(matched_uid[1]) != number or not declared or len(body) != int(declared[1]) or not 0 < len(body) <= MAX_MAIL:
        raise WireError('IMAP body UID or literal length is invalid')
    note = f'RFC822.SIZE {size} differs from literal {len(body)}' if len(body) != size else None
    return body, note
