"""Direct port of internal/envfile: dotenv bytes, without evaluation.

Adapted from motdotla/dotenv; copyright Scott Motte, BSD-2-Clause (NOTICE.md).
The Go/Python suites share internal/envfile/testdata/common.json.
"""
import re

SPACE = '\t\n\v\f\r \u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff'
WS = '[' + re.escape(SPACE) + ']'
LINE = re.compile((r"\s*(?:export\s+)?([A-Za-z0-9_.-]+)(?:\s*=\s*?|:\s+?)(\s*'(?:\\'|[^'])*'|\s*\"(?:\\\"|[^\"])*\"|\s*`(?:\\`|[^`])*`|[^#\r\n]+)?\s*(?:#([^\n\u2028\u2029]*))?(?:\Z|[\n\u2028\u2029])").replace(r'\s', WS))
REFERENCE = re.compile(r'\$[A-Za-z_{(]')
END = re.compile('[\n\u2028\u2029]')
FIELDS = re.compile('[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+')


def unquote(value):
    last = {q: i for i, q in enumerate(value) if q in "'\"`" and (i + 1 == len(value) or value[i + 1] in '\n\u2028\u2029')}
    out, copied, i = [], 0, 0
    while i < len(value):
        if i == 0 or value[i - 1] in '\n\u2028\u2029':
            close = last.get(value[i], -1)
            if close > i:
                out.extend((value[copied:i], value[i + 1:close]))
                copied, i = close + 1, close
        i += 1
    return ''.join(out) + value[copied:]


def parse(source):
    text = source.replace('\r\n', '\n').replace('\r', '\n')
    def line(at):
        return 1 + len(END.findall(text, 0, at))
    if '\0' in text:
        raise ValueError(f'line {line(text.index(chr(0)))}: NUL cannot be delivered in an environment')
    result, offset = [], 0
    while offset < len(text):
        match = LINE.match(text, offset)
        if match is None:
            end = END.search(text, offset)
            stop = end.start() if end else len(text)
            value = text[offset:stop].strip(SPACE)
            if value and not value.startswith('#'):
                raise ValueError(f'line {line(offset)}: expected a dotenv assignment')
            offset = stop + 1
            continue
        key, raw, comment = match.groups()
        value = (raw or '').strip(SPACE)
        double = value.startswith('"')
        if not value.startswith(("'", '`')) and REFERENCE.search(value):
            raise ValueError(f'line {line(match.start(1))}: a reference is never expanded; single-quote a literal dollar')
        value = unquote(value)
        if double:
            value = value.replace(r'\n', '\n').replace(r'\r', '\r')
        required = False
        if comment is not None:
            end = match.end(1)
            if raw and raw.strip(SPACE):
                trimmed = raw.strip(SPACE)
                end = match.start(2) + raw.rfind(trimmed) + len(trimmed)
            if not END.search(text, end, match.start(3) - 1):
                required = [word for word in FIELDS.split(comment) if word][:1] == ['required']
        result.append({'Key': key, 'Value': value, 'Required': required})
        offset = match.end()
    return result
