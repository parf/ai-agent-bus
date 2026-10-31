"""Bounded BODYSTRUCTURE metadata and PEEK-only MIME payloads."""
import base64
import quopri
import re
import wire

PART_CAP = 25 * 1024 * 1024


def structure(raw):
    """IMAP parenthesised values, including quoted strings; bounded recursion."""
    tokens = re.findall(rb'\(|\)|"(?:[^"\\]|\\.)*"|[^()\s]+', raw)
    at = 0
    def read(depth=0):
        nonlocal at
        if depth > 32 or at >= len(tokens): raise wire.WireError('invalid or deeply nested BODYSTRUCTURE')
        token = tokens[at]; at += 1
        if token == b'(':
            result = []
            while at < len(tokens) and tokens[at] != b')':
                result.append(read(depth + 1))
                if len(result) > 1000: raise wire.WireError('BODYSTRUCTURE exceeds 1000 values')
            if at == len(tokens): raise wire.WireError('unclosed BODYSTRUCTURE')
            at += 1
            return result
        if token == b')': raise wire.WireError('unexpected BODYSTRUCTURE close')
        if token == b'NIL': return None
        if token.startswith(b'"'): return re.sub(rb'\\(.)', rb'\1', token[1:-1]).decode('utf-8', 'replace')
        if token.isdigit(): return int(token)
        return token.decode('utf-8', 'replace')
    result = read()
    if at != len(tokens): raise wire.WireError('extra BODYSTRUCTURE values')
    return result


def tree(connection, number):
    raw = wire.ok(connection.uid('FETCH', str(number), '(UID BODYSTRUCTURE)'))
    chunks = []
    for item in raw:
        if item is None: continue
        if isinstance(item, tuple) and len(item) == 2 and all(isinstance(v, bytes) for v in item):
            header, literal = item
            count = re.search(rb'\{([0-9]+)\}\Z', header)
            if not count or int(count[1]) != len(literal): raise wire.WireError('invalid BODYSTRUCTURE literal')
            chunks.append(header[:count.start()] + b'"' + literal.replace(b'\\', b'\\\\').replace(b'"', b'\\"') + b'"')
        elif isinstance(item, bytes): chunks.append(item)
        else: raise wire.WireError('invalid BODYSTRUCTURE response')
    joined = b''.join(chunks)
    if len(joined) > wire.MAX_LINE: raise wire.WireError('BODYSTRUCTURE exceeds metadata limit')
    match = re.fullmatch(rb'[0-9]+ (\(.*\))', joined)
    if not match: raise wire.WireError('invalid BODYSTRUCTURE FETCH')
    fields = structure(match[1])
    if not isinstance(fields, list) or len(fields) != 4:
        raise wire.WireError('invalid BODYSTRUCTURE FETCH fields')
    fields = dict(zip(fields[::2], fields[1::2]))
    if fields.get('UID') != number or 'BODYSTRUCTURE' not in fields:
        raise wire.WireError('BODYSTRUCTURE UID mismatch')
    root, rows = fields['BODYSTRUCTURE'], []
    def walk(node, prefix=''):
        if not isinstance(node, list) or not node: raise wire.WireError('invalid MIME node')
        if isinstance(node[0], list):
            index = 0
            while index < len(node) and isinstance(node[index], list):
                walk(node[index], (prefix + '.' if prefix else '') + str(index + 1))
                index += 1
            if index == len(node) or not isinstance(node[index], str): raise wire.WireError('missing multipart subtype')
            rows.append({'id': prefix or 'root', 'type': 'multipart/' + node[index].lower(), 'size': None, 'filename': None, 'disposition': 'inline', 'encoding': None})
            return
        if len(node) < 7 or not all(isinstance(node[i], str) for i in (0, 1, 5)) or type(node[6]) is not int or node[6] < 0:
            raise wire.WireError('invalid MIME leaf')
        kind = node[0].lower() + '/' + node[1].lower()
        params = node[2] or []
        if not isinstance(params, list) or len(params) % 2: raise wire.WireError('invalid MIME parameters')
        params = {str(params[i]).lower(): params[i + 1] for i in range(0, len(params), 2)}
        disposition_at = 9 if node[0].lower() == 'text' else 11 if kind == 'message/rfc822' else 8
        disp = node[disposition_at] if len(node) > disposition_at else None
        filename, disposition = params.get('name'), 'inline'
        if isinstance(disp, list) and disp:
            disposition = str(disp[0]).lower()
            dparams = disp[1] if len(disp) > 1 else None
            if isinstance(dparams, list) and len(dparams) % 2 == 0:
                filename = dict(zip([str(x).lower() for x in dparams[::2]], dparams[1::2])).get('filename', filename)
        row = {'id': prefix or '1', 'type': kind, 'size': node[6], 'filename': filename,
               'disposition': 'attachment' if disposition == 'attachment' else 'inline', 'encoding': node[5].lower(), 'charset': params.get('charset', 'utf-8')}
        rows.append(row)
        if len(rows) > 1000: raise wire.WireError('MIME tree exceeds 1000 parts')
        if kind == 'message/rfc822' and len(node) > 8 and isinstance(node[8], list):
            walk(node[8], row['id'] if isinstance(node[8][0], list) else row['id'] + '.1')
    walk(root)
    return rows


def payload(connection, number, part_id):
    rows = tree(connection, number)
    row = next((r for r in rows if r['id'] == part_id and r['size'] is not None), None)
    if row is None: raise wire.WireError('part-id is not a MIME leaf; use parts')
    if row['size'] > PART_CAP: raise wire.TooLarge('part exceeds the 25 MB (26214400-byte) cap before decoding')
    try:
        raw = wire.peek(connection, number, part_id, PART_CAP)
    except wire.LiteralTooLarge:
        raise wire.LiteralTooLarge('part exceeds the 25 MB (26214400-byte) cap before decoding') from None
    if len(raw) > PART_CAP: raise wire.TooLarge('part exceeds the 25 MB (26214400-byte) cap before decoding')
    encoding = row['encoding']
    if encoding == 'base64':
        try: body = base64.b64decode(re.sub(rb'\s', b'', raw), validate=True)
        except ValueError: raise wire.WireError('invalid base64 MIME part') from None
    elif encoding == 'quoted-printable': body = quopri.decodestring(raw)
    elif encoding in {'7bit', '8bit', 'binary'}: body = raw
    else: raise wire.WireError('unsupported MIME transfer encoding')
    if row['type'].startswith('text/'):
        try: text = body.decode(row['charset'], 'replace')
        except LookupError: text = body.decode('utf-8', 'replace')
        return row, text, False
    return row, base64.b64encode(body).decode('ascii'), True
