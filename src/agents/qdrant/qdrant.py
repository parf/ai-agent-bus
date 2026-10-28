#!/usr/bin/env python3
"""One bounded, role-gated request to the operator's Qdrant URL."""
import http.client
import ipaddress
import json
import os
from pathlib import Path
import re
import signal
import sys
import urllib.parse

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE / 'support/agents/shared' if (HERE / 'support').exists() else HERE.parent / 'shared'))
from metadata import dispatch
from provider_http import MAX_BYTES, ProxyError, redact, scrub, provider_error, arm_deadline, connect

VERBS = {'GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'}
API_ROOTS = {'collections', 'aliases', 'cluster', 'snapshots', 'telemetry', 'metrics',
             'healthz', 'livez', 'readyz', 'issues', 'quotas', 'locks'}
TIERS = {'read': 0, 'write': 1, 'admin': 2}
# Exact suffixes after /collections/{collection}; unknown method/path => admin.
READ_POST = {'points', 'facet', 'points/scroll', 'points/count', 'points/query',
             'points/query/batch', 'points/query/groups', 'points/search',
             'points/search/batch', 'points/search/groups', 'points/search/matrix/pairs',
             'points/search/matrix/offsets', 'points/recommend', 'points/recommend/batch',
             'points/recommend/groups', 'points/discover', 'points/discover/batch'}
WRITE = {('PUT', 'points'), ('PUT', 'points/vectors'), ('POST', 'points/delete'),
         ('POST', 'points/batch'), ('POST', 'points/vectors/delete'),
         ('POST', 'points/payload'), ('PUT', 'points/payload'),
         ('POST', 'points/payload/delete'), ('POST', 'points/payload/clear')}
POINT_ID = re.compile(r'(?:[0-9]{1,20}|[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12})\Z')


def clean_target(value):
    if not isinstance(value, str) or not value or len(value) > 8192:
        raise ProxyError('path must be an API request target of at most 8192 characters')
    if any(ord(c) < 33 or ord(c) > 126 for c in value) or any(c in value for c in '#;\\'):
        raise ProxyError('path/query must be ASCII without whitespace, controls, fragments or separators')
    if not value.startswith('/') or value.startswith('//') or '//' in value:
        raise ProxyError('path must start with a single slash and stay within the API')
    if re.search(r'%(?:2e|2f|5c|25|0[0-9a-f]|1[0-9a-f]|7f)', value, re.I) or re.search(r'%(?![0-9a-fA-F]{2})', value):
        raise ProxyError('encoded traversal or separators are not API paths')
    path = value.partition('?')[0]
    if any(p in {'.', '..'} for p in path.split('/')):
        raise ProxyError('path traversal is not allowed')
    return path


def configuration(config):
    if not isinstance(config, dict) or set(config) != {'url'} or not isinstance(config['url'], str):
        raise ProxyError('Configuration requires only url')
    url = config['url']
    if len(url) > 8192 or any(ord(c) <= 32 or ord(c) == 127 for c in url):
        raise ProxyError('invalid Configuration URL')
    base = urllib.parse.urlsplit(url)
    if base.scheme not in {'http', 'https'} or not base.hostname or base.username is not None or base.password is not None or '?' in url or '#' in url:
        raise ProxyError('url must be an HTTP(S) base URL without credentials, query or fragment')
    if base.port is not None and not 1 <= base.port <= 65535:
        raise ProxyError('invalid URL port')
    prefix = base.path
    if prefix:
        clean_target(prefix)
    prefix = prefix.rstrip('/')
    return base, prefix


def request_bytes(body):
    if not isinstance(body, str) or len(body.encode()) > MAX_BYTES:
        raise ProxyError('request exceeds 1048576 bytes or is not text')
    if body.startswith('request '):
        words = body.split(' ', 3)
        if len(words) < 3 or not words[1] or not words[2]:
            raise ProxyError('request takes VERB /path[?query] and an optional raw JSON body')
        verb, target = words[1:3]
        raw = words[3] if len(words) == 4 else ''
    else:
        value = json.loads(body)
        if not isinstance(value, dict) or set(value) - {'method', 'verb', 'path', 'body'} or value.get('method') != 'request':
            raise ProxyError('expected a request object with method, verb, path and optional body')
        verb, target, raw = value.get('verb'), value.get('path'), value.get('body', '')
    if not isinstance(verb, str) or verb not in VERBS:
        raise ProxyError('verb must be GET, POST, PUT, PATCH, DELETE, HEAD or OPTIONS')
    path = clean_target(target)
    if path != '/' and path.split('/')[1] not in API_ROOTS:
        raise ProxyError('path is outside the Qdrant API')
    if not isinstance(raw, str) or len(raw.encode()) > MAX_BYTES:
        raise ProxyError('body exceeds 1048576 bytes or is not raw JSON text')
    if raw:
        if verb in {'GET', 'HEAD', 'OPTIONS'}:
            raise ProxyError(verb + ' does not accept a body')
        json.loads(raw)  # Validate only: forward the original UTF-8 bytes.
    return verb, target, path, raw.encode()


def required_role(verb, path):
    parts = path.split('/')
    if verb in {'GET', 'HEAD'} and path == '/collections':
        return 'read'
    if len(parts) >= 3 and parts[1] == 'collections' and parts[2]:
        suffix = '/'.join(parts[3:])
        if verb in {'GET', 'HEAD'}:
            if len(parts) == 3 or suffix == 'exists':
                return 'read'
            if len(parts) == 5 and parts[3] == 'points' and POINT_ID.fullmatch(parts[4]):
                if not parts[4].isdigit() or int(parts[4]) <= (1 << 64) - 1:
                    return 'read'
        if verb == 'POST' and suffix in READ_POST:
            return 'read'
        if (verb, suffix) in WRITE:
            return 'write'
    return 'admin'


def authorize(envelope, required):
    roles = envelope.get('roles', [])
    if not isinstance(roles, list) or any(not isinstance(r, str) for r in roles):
        raise ProxyError('invalid delivered roles')
    held = set(roles)
    if held & {'owner', 'maintainer', 'admin'}:
        tier = 2
    elif 'write' in held:
        tier = 1
    elif 'read' in held:
        tier = 0
    else:
        tier = 1  # RD133: ACL-admitted caller without an application role gets write.
    if tier < TIERS[required]:
        raise ProxyError('requires ' + required + ' role')


def remote_http(base):
    if base.scheme != 'http' or base.hostname == 'localhost':
        return False
    try:
        return not ipaddress.ip_address(base.hostname).is_loopback
    except ValueError:
        return True


def answer(envelope, config, key):
    if not key:
        return 'error: QDRANT_API_KEY is not set', 78
    timer = connection = None
    request_started = False

    def uncertain():
        return '; write outcome unknown; do not automatically retry' if request_started and verb not in {'GET', 'HEAD'} else ''

    status, verb, path, sent, received = 'invalid', '', '', 0, 0
    try:
        if any(ord(c) < 32 or ord(c) == 127 for c in key):
            raise ProxyError('QDRANT_API_KEY is not a valid HTTP header value')
        base, prefix = configuration(config)
        verb, target, path, payload = request_bytes(envelope['body'])
        authorize(envelope, required_role(verb, path))
        if remote_http(base):
            print('notice: non-loopback HTTP sends QDRANT_API_KEY unencrypted', file=sys.stderr)
        sent = len(payload)
        timer, remaining = arm_deadline(envelope)
        connection = connect(base, remaining)
        connection.connect()
        headers = {'api-key': key, 'Accept-Encoding': 'identity'}
        if payload:
            headers['Content-Type'] = 'application/json'
        request_started = True
        connection.request(verb, prefix + target, payload if payload else None, headers)
        response = connection.getresponse()
        status = response.status
        if verb == 'HEAD' or status == 204:
            output = json.dumps({'status': status, 'headers': scrub(dict(response.getheaders()), key)}, ensure_ascii=False)
        else:
            if response.getheader('Content-Encoding', 'identity').lower() != 'identity':
                raise ProxyError('encoded Qdrant answer is not supported')
            declared = response.getheader('Content-Length')
            if declared is not None and int(declared) > MAX_BYTES:
                raise ProxyError('Qdrant answer exceeds 1048576 bytes')
            raw = response.read(MAX_BYTES + 1)
            received = len(raw)
            if received > MAX_BYTES:
                raise ProxyError('Qdrant answer exceeds 1048576 bytes')
            if declared is not None and len(raw) != int(declared):
                raise http.client.IncompleteRead(raw)
            content_type = response.getheader('Content-Type', '').split(';')[0].strip().lower()
            if content_type == 'application/json' or content_type.endswith('+json'):
                output = json.dumps(scrub(json.loads(raw), key), ensure_ascii=False)
            elif content_type.startswith('text/'):
                output = redact(raw.decode('utf-8'), key)
            else:
                raise ProxyError('Qdrant answer has unsupported Content-Type')
            if status >= 300:
                output = provider_error(status, output)
        if len(output.encode()) > MAX_BYTES:
            raise ProxyError('Qdrant answer exceeds 1048576 bytes')
        return output, 0
    except TimeoutError:
        status = 'timeout'
        return 'error: Qdrant request timed out' + (uncertain() if request_started else '; request not sent'), 0
    except ProxyError as error:
        return 'error: ' + (str(status) + ' ' if isinstance(status, int) else '') + redact(str(error), key) + uncertain(), 0
    except (ValueError, TypeError, KeyError, AttributeError):
        return 'error: ' + (str(status) + ' ' if isinstance(status, int) else '') + 'invalid request, Configuration or Qdrant response' + uncertain(), 0
    except (OSError, http.client.HTTPException):
        status = 'unavailable'
        return 'error: Qdrant connection failed' + (uncertain() if request_started else '; request not sent'), 0
    finally:
        if timer is not None:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, timer)
        if connection is not None:
            connection.close()
        print(json.dumps({'verb': verb, 'path': redact(path, key), 'request_bytes': sent,
                          'answer_bytes': received, 'status': status}), file=sys.stderr)


def main():
    result = dispatch(HERE, os.environ.get('AGENT_BUS_ASK'))
    if result is not None:
        print(result)
        return 0
    key = os.environ.get('QDRANT_API_KEY', '')
    if not key:
        print('error: QDRANT_API_KEY is not set')
        return 78
    try:
        raw = sys.stdin.buffer.read(8 * MAX_BYTES + 1)
        if len(raw) > 8 * MAX_BYTES:
            raise ValueError()
        envelope = json.loads(raw)
        if not isinstance(envelope, dict):
            raise ValueError()
        output, code = answer(envelope, json.loads(os.environ.get('AGENT_BUS_CONFIG', '{}')), key)
    except (ValueError, TypeError):
        output, code = 'error: invalid message or Configuration', 0
    print(output)
    return code


if __name__ == '__main__':
    sys.exit(main())
