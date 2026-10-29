"""Bounded route-list parsing, per-route authentication and admission state."""
import base64
from collections import Counter
import hashlib
import hmac
import json
import re
import threading
import time
from urllib.parse import urlsplit

BODY_MAX = 256 * 1024
REPLY_MAX = 256 * 1024
HEADER_MAX = 16384
ROUTE_MAX = 64
NAME = re.compile(r'[A-Za-z0-9_-]{1,64}')
KEY = re.compile(r'[A-Za-z_][A-Za-z0-9_]{0,63}')
PATH = re.compile(r'/[A-Za-z0-9._~/-]*')


class Invalid(ValueError):
    pass


def pairs(items):
    out = {}
    for k, v in items:
        if k in out:
            raise Invalid('duplicate JSON key')
        out[k] = v
    return out


def loads(raw, maximum=1 << 20):
    try:
        if not isinstance(raw, str) or len(raw.encode()) > maximum:
            raise Invalid('JSON byte limit')
        return json.loads(raw, object_pairs_hook=pairs, parse_constant=lambda _: (_ for _ in ()).throw(Invalid('nonfinite JSON')))
    except (UnicodeError, ValueError, TypeError, RecursionError):
        raise Invalid('invalid JSON or byte limit') from None


def text(v, maximum, label, *, empty=False):
    try:
        if not isinstance(v, str) or len(v.encode()) > maximum or '\x00' in v or '\r' in v or '\n' in v or not empty and not v:
            raise Invalid('invalid ' + label)
    except UnicodeError:
        raise Invalid('invalid ' + label) from None
    return v


def integer(v, lo, hi):
    if type(v) is not int or not lo <= v <= hi:
        raise Invalid('integer out of range')
    return v


def path(v):
    if not isinstance(v, str) or len(v) > 256 or not PATH.fullmatch(v) or v.startswith('//') or any(x in ('.', '..') for x in v.split('/')) or '//' in v:
        raise Invalid('invalid exact route path')
    return v


def secret_key(v):
    if not isinstance(v, str) or not KEY.fullmatch(v):
        raise Invalid('invalid Secret key name')
    return v


def configuration(value):
    if not isinstance(value, list) or not 1 <= len(value) <= ROUTE_MAX:
        raise Invalid('Configuration must be a list of 1..64 routes')
    out, names, paths = [], set(), set()
    for raw in value:
        if not isinstance(raw, dict):
            raise Invalid('route must be an object')
        route = dict(raw)
        name = route.get('name')
        if not isinstance(name, str) or not NAME.fullmatch(name) or name in names:
            raise Invalid('route names must be unique ASCII identifiers')
        names.add(name)
        incoming = 'path' in route
        allowed = {'name', 'path', 'target', 'basic', 'signature', 'max_bytes', 'rate', 'burst'} if incoming else {'name', 'url', 'basic', 'content_type', 'max_bytes'}
        if set(route) - allowed:
            raise Invalid('unknown or mixed route fields')
        route['max_bytes'] = integer(route.get('max_bytes', BODY_MAX), 1, BODY_MAX)
        if incoming:
            path(route['path'])
            if route['path'] in paths:
                raise Invalid('duplicate inbound path')
            paths.add(route['path'])
            target = text(route.get('target'), 512, 'bus target')
            if target != target.strip().lower() or any(ord(c) <= 32 or ord(c) == 127 for c in target):
                raise Invalid('invalid bus target')
            route['rate'] = integer(route.get('rate', 60), 1, 600)
            route['burst'] = integer(route.get('burst', 10), 1, 100)
            if 'signature' in route:
                signature = route['signature']
                if not isinstance(signature, dict) or set(signature) != {'preset', 'secret'} or signature['preset'] not in ('github', 'stripe'):
                    raise Invalid('signature needs github/stripe preset and Secret key')
                secret_key(signature['secret'])
        else:
            url = text(route.get('url'), 4096, 'outbound URL')
            try:
                u = urlsplit(url)
                if u.scheme not in ('http', 'https') or not u.hostname or u.username is not None or u.password is not None or u.fragment or any(ord(c) <= 32 or ord(c) >= 127 for c in url) or u.port is not None and not 1 <= u.port <= 65535:
                    raise Invalid('invalid configured HTTP(S) URL')
            except ValueError:
                raise Invalid('invalid configured HTTP(S) URL') from None
            route['content_type'] = route.get('content_type', 'application/json')
            if route['content_type'] not in ('application/json', 'text/plain'):
                raise Invalid('content_type must be application/json or text/plain')
        if 'basic' in route:
            basic = route['basic']
            if not isinstance(basic, dict) or set(basic) != {'user', 'password'}:
                raise Invalid('Basic needs user and password Secret key')
            user = text(basic['user'], 128, 'Basic username')
            if ':' in user:
                raise Invalid('Basic username cannot contain colon')
            secret_key(basic['password'])
        out.append(route)
    return out


def references(routes):
    return sorted({r['basic']['password'] for r in routes if 'basic' in r} | {r['signature']['secret'] for r in routes if 'signature' in r})


def credentials(routes, raw):
    from envfile import parse
    try:
        if not isinstance(raw, bytes) or len(raw) > 64 * 1024:
            raise Invalid('Secret byte limit')
        entries = parse(raw.decode('utf-8'))
        result = {}
        for entry in entries:
            if entry['Key'] in result:
                raise Invalid('duplicate Secret key')
            result[entry['Key']] = entry['Value']
        for key in references(routes):
            if key not in result or not result[key] or len(result[key].encode()) > 4096:
                raise Invalid('missing or invalid required Secret entry')
        return {key: result[key] for key in references(routes)}
    except (ValueError, UnicodeError, TypeError):
        raise Invalid('missing or invalid required Secret entry') from None


def basic_bytes(route, secrets):
    return (route['basic']['user'] + ':' + secrets[route['basic']['password']]).encode('utf-8')


def basic_ok(route, headers, secrets):
    if 'basic' not in route:
        return True
    values = headers.get_all('Authorization', [])
    if len(values) != 1 or len(values[0]) > 8192:
        return False
    try:
        scheme, encoded = values[0].split(' ', 1)
        supplied = base64.b64decode(encoded, validate=True)
        return scheme.lower() == 'basic' and hmac.compare_digest(supplied, basic_bytes(route, secrets))
    except (ValueError, UnicodeError):
        return False


def signature_ok(route, headers, body, secrets, now=None):
    if 'signature' not in route:
        return True
    cfg = route['signature']; key = secrets[cfg['secret']].encode('utf-8')
    name = 'X-Hub-Signature-256' if cfg['preset'] == 'github' else 'Stripe-Signature'
    values = headers.get_all(name, [])
    if len(values) != 1 or len(values[0]) > 2048:
        return False
    if cfg['preset'] == 'github':
        value = values[0]
        if not re.fullmatch(r'sha256=[0-9A-Fa-f]{64}', value):
            return False
        return hmac.compare_digest(bytes.fromhex(value[7:]), hmac.digest(key, body, 'sha256'))
    timestamp, candidates = None, []
    for part in values[0].split(','):
        pair = part.strip().split('=', 1)
        if len(pair) != 2:
            return False
        kind, value = pair
        if kind == 't':
            if timestamp is not None or not re.fullmatch(r'[0-9]{1,12}', value):
                return False
            timestamp = value
        elif kind == 'v1':
            if not re.fullmatch(r'[0-9A-Fa-f]{64}', value) or len(candidates) >= 8:
                return False
            candidates.append(bytes.fromhex(value))
    if timestamp is None or not candidates or abs((time.time() if now is None else now) - int(timestamp)) > 300:
        return False
    expected = hmac.digest(key, timestamp.encode() + b'.' + body, 'sha256')
    return any(hmac.compare_digest(value, expected) for value in candidates)


def parse_call(body):
    if not isinstance(body, str) or len(body.encode('utf-8')) > BODY_MAX + 2048:
        raise Invalid('call byte limit')
    if body.startswith('{'):
        value = loads(body, BODY_MAX + 2048)
    elif body == 'routes':
        value = {'method': 'routes'}
    else:
        method, sep, rest = body.partition(' ')
        route, sep2, payload = rest.partition(' ')
        if method != 'post' or not sep or not sep2:
            raise Invalid('use post <route> <body> or routes')
        value = {'method': method, 'route': route, 'body': payload}
    if value == {'method': 'routes'}:
        return value
    if not isinstance(value, dict) or set(value) != {'method', 'route', 'body'} or value['method'] != 'post' or not isinstance(value['route'], str) or not NAME.fullmatch(value['route']) or not isinstance(value['body'], str):
        raise Invalid('expected post with route and text body')
    try:
        if len(value['body'].encode()) > BODY_MAX:
            raise Invalid('outbound body limit')
    except UnicodeError:
        raise Invalid('body must be UTF-8') from None
    return value


class State:
    def __init__(self, routes, clock=time.monotonic):
        self.lock = threading.Lock(); self.clock = clock
        self.buckets = {r['name']: [r.get('burst', 10), clock()] for r in routes}
        self.counts = Counter(); self.by_route = {r['name']:Counter() for r in routes}; self.log_at = clock(); self.log_count = 0; self.suppressed = 0
    def take(self, route):
        with self.lock:
            now = self.clock(); bucket = self.buckets[route['name']]
            bucket[0] = min(route['burst'], bucket[0] + max(0, now - bucket[1]) * route['rate'] / 60)
            bucket[1] = now
            if bucket[0] < 1:
                return False
            bucket[0] -= 1
            return True
    def count(self, reason, route=None):
        with self.lock:
            self.counts[reason] = min(self.counts[reason] + 1, (1 << 63) - 1)
            if route in self.by_route:
                counts = self.by_route[route]
                counts[reason] = min(counts[reason] + 1, (1 << 63) - 1)
    def snapshot(self):
        with self.lock:
            return dict(self.counts)
    def route_snapshot(self, name):
        with self.lock: return dict(self.by_route.get(name, {}))
    def log(self, route, size, status):
        # One global bound also covers unknown paths; never allocate attacker keys.
        import sys
        with self.lock:
            now = self.clock()
            if now - self.log_at >= 60:
                if self.suppressed:
                    print(json.dumps({'route': '-', 'bytes': 0, 'status': 'suppressed', 'count': self.suppressed}), file=sys.stderr)
                self.log_at = now; self.log_count = 0; self.suppressed = 0
            if self.log_count < 10:
                print(json.dumps({'route': route, 'bytes': size, 'status': status}), file=sys.stderr)
                self.log_count += 1
            else:
                self.suppressed = min(self.suppressed + 1, (1 << 63) - 1)
