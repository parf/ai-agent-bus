"""Decode LIST names without treating provider text as an IMAP command."""
import base64
import binascii
import json
import re
import wire

ROLES = {b'\\inbox': 'Inbox', b'\\sent': 'Sent', b'\\drafts': 'Drafts',
         b'\\trash': 'Trash', b'\\junk': 'Junk', b'\\archive': 'Archive'}
COUNT_CAP = 50


def decode(raw):
    try:
        text = raw.decode('ascii')
        result, position = [], 0
        for match in re.finditer(r'&([^-]*)-', text):
            plain = text[position:match.start()]
            if '&' in plain:
                raise ValueError()
            result.append(plain)
            encoded = match[1]
            if not encoded:
                result.append('&')
            else:
                if not re.fullmatch(r'[A-Za-z0-9+,]+', encoded):
                    raise ValueError()
                value = base64.b64decode(encoded.replace(',', '/') + '=' * (-len(encoded) % 4), validate=True).decode('utf-16be')
                if not value or any(32 <= ord(c) <= 126 for c in value):
                    raise ValueError()
                result.append(value)
            position = match.end()
        if '&' in text[position:]:
            raise ValueError()
        result.append(text[position:])
        name = ''.join(result)
        wire.mailbox(name)
        return name
    except (ValueError, UnicodeError, binascii.Error) as error:
        raise wire.WireError('invalid modified UTF-7 folder name') from error


def token(raw):
    """One quoted string or atom, returning its unescaped bytes and remainder."""
    raw = raw.lstrip()
    if raw.startswith(b'"'):
        value, i = bytearray(), 1
        while i < len(raw):
            char = raw[i]
            if char == 34:
                return bytes(value), raw[i + 1:]
            if char == 92:
                i += 1
                if i == len(raw) or raw[i] not in (34, 92):
                    break
                char = raw[i]
            value.append(char)
            i += 1
        raise wire.WireError('malformed quoted folder name')
    match = re.match(rb'([^ ()\r\n]+)(.*)', raw, re.S)
    if not match:
        raise wire.WireError('malformed folder response')
    return match[1], match[2]


def row(item):
    literal = None
    if isinstance(item, tuple) and len(item) == 2:
        item, literal = item
    if not isinstance(item, bytes):
        raise wire.WireError('malformed LIST response')
    match = re.match(rb'^\(([^()]*)\) +', item)
    if not match:
        raise wire.WireError('malformed LIST flags')
    flags = match[1].lower().split()
    _delimiter, rest = token(item[match.end():])
    if literal is not None:
        if not isinstance(literal, bytes) or rest.strip() != b'{' + str(len(literal)).encode() + b'}':
            raise wire.WireError('malformed LIST literal')
        name = decode(literal)
    else:
        raw, rest = token(rest)
        if rest.strip():
            raise wire.WireError('malformed LIST name')
        name = decode(raw)
    return {'name': name, 'role': 'Inbox' if name.casefold() == 'inbox' else next((v for k, v in ROLES.items() if k in flags), ''),
            'selectable': b'\\noselect' not in flags}


def same(a, b):
    return a == b or (a.casefold() == 'inbox' and b.casefold() == 'inbox')


def status_rows(data):
    """Join imaplib's literal-name tuple and trailing STATUS attributes."""
    items = iter(data or [])
    for item in items:
        if item in (None, b''):
            continue
        if isinstance(item, tuple) and len(item) == 2:
            header, name = item
            tail = next(items, None)
            if (not isinstance(header, bytes) or not isinstance(name, bytes) or not isinstance(tail, bytes)
                    or header.strip() != b'{' + str(len(name)).encode() + b'}'):
                raise wire.WireError('malformed STATUS literal')
            item = b'"' + name.replace(b'\\', b'\\\\').replace(b'"', b'\\"') + b'"' + tail
        yield item


def listing(connection, with_counts=False):
    use_status = with_counts and 'LIST-STATUS' in connection.capabilities
    returns = ['SPECIAL-USE'] if 'SPECIAL-USE' in connection.capabilities else []
    if use_status:
        returns.append('STATUS (MESSAGES UNSEEN)')
    if returns:
        status, tagged = connection._simple_command('LIST', '""', '"*"', 'RETURN', '(' + ' '.join(returns) + ')')
        status, data = connection._untagged_response(status, tagged, 'LIST')
        _, statuses = connection._untagged_response('OK', [], 'STATUS')
    else:
        statuses = []
        status, data = connection.list('""', '"*"')
    if status != 'OK':
        raise wire.WireError('folder LIST refused')
    rows, names = [], set()
    for item in data or []:
        if item in (None, b''):  # imaplib's trailing fragment after a literal.
            continue
        value = row(item)
        key = 'INBOX' if value['name'].casefold() == 'inbox' else value['name']
        if key in names:
            raise wire.WireError('duplicate folder in LIST')
        names.add(key)
        rows.append(value)
    if use_status:
        observed = {}
        for raw in status_rows(statuses):
            if raw is None:
                continue
            name, total, unseen = parse_counts(raw)
            if name in observed:
                raise wire.WireError('duplicate folder counts')
            observed[name] = (total, unseen)
        for item in rows:
            if item['selectable']:
                value = next((value for name, value in observed.items() if same(name, item['name'])), None)
                if value is None:
                    raise wire.WireError('LIST-STATUS omitted folder counts')
                item['counts'] = value
    return rows


def choose(connection, name):
    for item in listing(connection):
        if same(name, item['name']):
            if not item['selectable']:
                raise wire.WireError('folder is not selectable: ' + json.dumps(name, ensure_ascii=False))
            return item['name']
    raise wire.WireError('folder not found: ' + json.dumps(name, ensure_ascii=False))


def counts(connection, name):
    status, data = connection.status(wire.mailbox(name), '(MESSAGES UNSEEN)')
    data = list(status_rows(data))
    if status != 'OK' or len(data) != 1:
        raise wire.WireError('folder STATUS refused')
    actual, total, unseen = parse_counts(data[0])
    if not same(actual, name):
        raise wire.WireError('STATUS named a different folder')
    return total, unseen


def parse_counts(data):
    if not isinstance(data, bytes):
        raise wire.WireError('malformed folder STATUS')
    raw, rest = token(data)
    name = decode(raw)
    match = re.fullmatch(rb'\s*\(([^()]*)\)', rest)
    if not match:
        raise wire.WireError('malformed folder counts')
    words = match[1].upper().split()
    if len(words) != 4 or set(words[::2]) != {b'MESSAGES', b'UNSEEN'} or any(not re.fullmatch(rb'[0-9]{1,10}', n) for n in words[1::2]):
        raise wire.WireError('invalid folder counts')
    values = dict(zip(words[::2], map(int, words[1::2])))
    if values[b'UNSEEN'] > values[b'MESSAGES']:
        raise wire.WireError('invalid folder unseen count')
    return name, values[b'MESSAGES'], values[b'UNSEEN']


def describe(connection, with_counts):
    rows, skipped, queried = [], 0, 0
    for item in listing(connection, with_counts):
        line = json.dumps(item['name'], ensure_ascii=False) + ' | ' + (item['role'] or '-')
        if not item['selectable']:
            line += ' | not selectable'
        elif with_counts:
            if 'counts' in item or queried < COUNT_CAP:
                total, unseen = item['counts'] if 'counts' in item else counts(connection, item['name'])
                queried += 1
                line += f' | total {total} | unseen {unseen}'
            else:
                skipped += 1
                line += ' | counts skipped'
        rows.append(line)
    if skipped:
        rows.append(f'counts skipped for {skipped} folders (limit {COUNT_CAP})')
    return '\n'.join(rows) or 'no folders'
