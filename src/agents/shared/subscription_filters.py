"""Portable subscription regex subset, translated to Python with JS semantics."""
import json
import math
import re
import signal
import sys
import time
import unicodedata

MAX_PATTERN = 200
MAX_TEXT = 8192
MAX_QUANTIFIERS = 20
MAX_REPEAT = 100
JS_SPACE = '\t\n\v\f\r \u00a0\u1680' + ''.join(chr(n) for n in range(0x2000,0x200b)) + '\u2028\u2029\u202f\u205f\u3000\ufeff'
TERMINATORS = '\n\r\u2028\u2029'


class FilterError(ValueError):
    def __init__(self, code='invalid_filter'): self.code = code; super().__init__(code)


class PatternTimeout(TimeoutError): pass


def scalar(value, maximum, code):
    if not isinstance(value, str): raise FilterError(code)
    try: size = len(value.encode('utf-8'))
    except UnicodeError: raise FilterError(code) from None
    if size > maximum: raise FilterError(code)
    return value


def text(value): return scalar(value, MAX_TEXT, 'invalid_text')


def units(value):
    raw = value.encode('utf-16-le')
    return ''.join(chr(raw[i] | raw[i+1] << 8) for i in range(0, len(raw), 2))


def fold(char, unicode):
    if unicode:
        value = char.casefold()
        if len(value) == 1: return value
        lower = char.lower()
        return lower if len(lower) == 1 else char
    value = char.upper()
    if len(value) != 1 or ord(char) >= 128 and ord(value) < 128: return char
    return value


def split(value):
    scalar(value, 4096, 'invalid_filter')
    flags = ''
    if value.startswith('/'):
        slashes, escaped = [], False
        for i, char in enumerate(value[1:], 1):
            if escaped: escaped = False; continue
            if char == '\\': escaped = True
            elif char == '/': slashes.append(i)
        if not slashes: raise FilterError()
        end = slashes[-1]; value, flags = value[1:end], value[end+1:]
    if len(value) > MAX_PATTERN or any(unicodedata.category(c) == 'Cc' for c in value): raise FilterError()
    if len(set(flags)) != len(flags) or any(c not in 'imsu' for c in flags): raise FilterError()
    return value, flags


def character_class(body):
    """Only ASCII class literals/ranges, expanded before canonicalization."""
    if not body or not body.isascii() or any(c in body for c in '[\\'):
        raise FilterError()
    if any(op in body for op in ('&&','--','~~','||')): raise FilterError()
    negative = body.startswith('^'); body = body[1:] if negative else body
    if not body: raise FilterError()
    if body.count('-') > 1: raise FilterError()
    values = set(); i = 0
    while i < len(body):
        if i+2 < len(body) and body[i+1] == '-':
            if ord(body[i]) > ord(body[i+2]): raise FilterError()
            values.update(chr(n) for n in range(ord(body[i]), ord(body[i+2])+1)); i += 3
        else: values.add(body[i]); i += 1
    return negative, values


def compile_filter(value):
    pattern, flags = split(value)
    unicode = 'u' in flags; insensitive = 'i' in flags
    if not unicode: pattern = units(pattern)
    canonical = lambda c: fold(c, unicode) if insensitive else c
    literal = lambda c: re.escape(canonical(c))
    word = set('ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_')
    spaces = set(JS_SPACE)
    def klass(values, negative=False):
        chars = sorted({canonical(c) for c in values})
        return '[' + ('^' if negative else '') + ''.join(re.escape(c) for c in chars) + ']'
    out=[]; groups=[]; last=None; quantifiers=0; i=0
    while i < len(pattern):
        char=pattern[i]
        if char == '\\':
            i += 1
            if i == len(pattern): raise FilterError()
            escaped=pattern[i]
            if escaped.lower() in ('d','w','s'):
                values=set('0123456789') if escaped.lower()=='d' else word if escaped.lower()=='w' else spaces
                out.append(klass(values, escaped.isupper()));last='atom'
            elif escaped in 'nrt': out.append(literal({'n':'\n','r':'\r','t':'\t'}[escaped]));last='atom'
            elif escaped in r'\./-^$|?*+(){}[]':out.append(literal(escaped));last='atom'
            else:raise FilterError()
        elif char=='[':
            end=pattern.find(']',i+1)
            if end<0:raise FilterError()
            negative, values=character_class(pattern[i+1:end]);out.append(klass(values,negative));i=end;last='atom'
        elif char=='(':
            if pattern[i:i+2]=='(?':
                if pattern[i:i+3]!='(?:':raise FilterError()
                i += 2
            out.append('(?:');groups.append(False);last=None
        elif char==')':
            if not groups:raise FilterError()
            had_quantifier=groups.pop();out.append(')');last='quantified_group' if had_quantifier else 'atom'
        elif char in '*+?{':
            if last not in ('atom',):raise FilterError()
            if char=='{':
                match=re.match(r'\{([0-9]+)(?:,([0-9]*))?\}',pattern[i:])
                if not match:raise FilterError()
                lo=int(match[1]);hi=int(match[2]) if match[2] else MAX_REPEAT if ',' in match[0] else lo
                if lo>MAX_REPEAT or hi>MAX_REPEAT or lo>hi:raise FilterError()
                token=match[0];i += len(token)-1
            else:token=char
            quantifiers+=1
            if quantifiers>MAX_QUANTIFIERS:raise FilterError()
            out.append(token);last='quantifier'
            groups=[True for _ in groups]
        elif char in '}]':raise FilterError()
        elif char=='|':out.append('|');last=None
        elif char=='.':out.append('[\\s\\S]' if 's' in flags else '[^\n\r\u2028\u2029]');last='atom'
        elif char=='^':out.append(r'(?:\A|(?<=[\n\r\u2028\u2029]))' if 'm' in flags else r'\A');last='anchor'
        elif char=='$':out.append(r'(?=[\n\r\u2028\u2029]|\Z)' if 'm' in flags else r'\Z');last='anchor'
        else:out.append(literal(char));last='atom'
        i+=1
    if groups:raise FilterError()
    try: compiled=re.compile(''.join(out))
    except re.error:raise FilterError() from None
    return compiled, unicode, insensitive


def matches(compiled, value):
    text(value)
    regex, unicode, insensitive=compiled
    value=value if unicode else units(value)
    if insensitive:value=''.join(fold(c,unicode) for c in value)
    return regex.search(value) is not None


def worker():
    """Trusted bounded input; no credentials/network; one alarm per pattern."""
    raw=sys.stdin.buffer.read(128*1024+1)
    if len(raw)>128*1024:return 78
    try:
        request=json.loads(raw);value=text(request['text']);patterns=request['filters']
        if not isinstance(patterns,list) or len(patterns)>100:return 78
        running=False
        def alarm(*_):
            if running: raise PatternTimeout()
        signal.signal(signal.SIGALRM,alarm)
        results=[];end=time.monotonic()+.9
        for pattern in patterns:
            if time.monotonic()>=end:results.append('timeout');continue
            try:
                running=True
                signal.setitimer(signal.ITIMER_REAL,min(.05,end-time.monotonic()))
                try:result=matches(compile_filter(pattern),value)
                except FilterError:result='invalid'
                running=False  # Still inside the timeout-catching scope.
            except PatternTimeout:
                running=False  # One-shot signal has fired; no second alarm.
                result='timeout'
            finally:
                running=False  # A late signal during disarm cannot raise.
                signal.setitimer(signal.ITIMER_REAL,0)
            results.append(result)
        print(json.dumps(results,separators=(',',':')))
        return 0
    except Exception:return 78


if __name__=='__main__':sys.exit(worker())
