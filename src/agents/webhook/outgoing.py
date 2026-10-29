"""One configured POST, isolated DNS/trickle deadline and bounded helper pipes."""
import base64
import datetime
import http.client
import json
import os
from pathlib import Path
import selectors
import signal
import subprocess
import sys
import time
from urllib.parse import urlsplit
from rules import BODY_MAX, REPLY_MAX, Invalid, basic_bytes, loads

PIPE_MAX = 2 * 1024 * 1024


class Unknown(Invalid):
    pass


def redact(value, values):
    for secret in sorted(values, key=len, reverse=True):
        if secret:
            value = value.replace(secret, '***')
    return value


def scrub(value, values):
    if isinstance(value, str): return redact(value, values)
    if isinstance(value, list): return [scrub(x, values) for x in value]
    if isinstance(value, dict): return {redact(k, values): scrub(v, values) for k, v in value.items()}
    return value


def limit(envelope):
    seconds = 10.0
    stamp = (envelope or {}).get('deadline')
    try:
        if stamp and not stamp.startswith('0001-'):
            seconds = min(seconds, datetime.datetime.fromisoformat(stamp.replace('Z', '+00:00')).timestamp() - time.time() - .1)
    except (ValueError, TypeError, AttributeError):
        raise Invalid('invalid deadline') from None
    if seconds <= 0:
        raise Invalid('message deadline reached')
    return seconds


def perform(request):
    route, body, secrets = request['route'], request['body'], request['secrets']
    encoded = body.encode('utf-8')
    if len(encoded) > route['max_bytes']:
        raise Invalid('outbound body byte limit')
    u = urlsplit(route['url'])
    transport = http.client.HTTPSConnection if u.scheme == 'https' else http.client.HTTPConnection
    connection = transport(u.hostname, u.port, timeout=request['timeout'])
    headers = {'Content-Type': route['content_type'], 'Accept-Encoding': 'identity'}
    masks = list(secrets.values())
    if 'basic' in route:
        auth = base64.b64encode(basic_bytes(route, secrets)).decode('ascii')
        headers['Authorization'] = 'Basic ' + auth
        masks.extend([auth, headers['Authorization']])
    sent = False
    try:
        # Once request() starts writing, a network error cannot prove no send.
        sent = True
        connection.request('POST', (u.path or '/') + ('?' + u.query if u.query else ''), encoded, headers)
        response = connection.getresponse()
        if len(response.headers) > 64 or sum(len(k) + len(v) for k, v in response.getheaders()) > 16384:
            return {'error': 'upstream header limit', 'unknown': True}
        if response.getheader('Content-Encoding', 'identity').lower() != 'identity':
            return {'error': 'upstream encoding refused', 'unknown': True}
        data = response.read(REPLY_MAX + 1)
        if len(data) > REPLY_MAX:
            return {'error': 'upstream reply byte limit', 'unknown': True}
        try: text = data.decode('utf-8')
        except UnicodeError: return {'error': 'upstream reply is not UTF-8', 'unknown': True}
        # Decode/encode only for masking escaped credential echoes, never the request.
        try: parsed = json.loads(text)
        except ValueError: text = redact(text, masks)
        else:
            try: text = json.dumps(scrub(parsed, masks), ensure_ascii=False, allow_nan=False)
            except (ValueError, TypeError): return {'error':'upstream JSON refused', 'unknown':True}
        return {'status': response.status, 'body': text}
    except (OSError, http.client.HTTPException, ValueError, UnicodeError):
        return {'error': 'outbound delivery outcome unknown' if sent else 'outbound connection failed', 'unknown': sent}
    finally:
        connection.close()


def post(route, body, secrets, envelope=None, *, helper=None):
    timeout = limit(envelope)
    payload = json.dumps({'route': route, 'body': body, 'secrets': secrets, 'timeout': timeout}, ensure_ascii=False).encode()
    if len(payload) > PIPE_MAX:
        raise Invalid('helper input byte limit')
    # No identity, roles or private host variables reach the network helper.
    env = {k: os.environ[k] for k in ('PATH', 'LANG', 'LC_ALL', 'SSL_CERT_FILE', 'SSL_CERT_DIR') if k in os.environ}
    argv = [sys.executable, '-B', str(Path(__file__).resolve()), '--worker'] if helper is None else helper
    try:
        process = subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, start_new_session=True)
    except OSError:
        raise Invalid('outbound helper could not start') from None
    selector = selectors.DefaultSelector(); buffers = {'out': bytearray(), 'err': bytearray()}
    end = time.monotonic() + timeout
    answer = None
    try:
        # Input is bounded too; nonblocking writes avoid a helper that never reads stdin.
        for pipe, events, name in ((process.stdin, selectors.EVENT_WRITE, 'in'), (process.stdout, selectors.EVENT_READ, 'out'), (process.stderr, selectors.EVENT_READ, 'err')):
            os.set_blocking(pipe.fileno(), False); selector.register(pipe, events, name)
        offset = 0
        while selector.get_map():
            remaining = end - time.monotonic()
            if remaining <= 0:
                raise TimeoutError()
            events = selector.select(remaining)
            if not events:
                raise TimeoutError()
            for key, _ in events:
                pipe, name = key.fileobj, key.data
                if name == 'in':
                    try: offset += os.write(pipe.fileno(), payload[offset:offset+65536])
                    except BrokenPipeError:
                        selector.unregister(pipe); pipe.close(); continue
                    if offset == len(payload): selector.unregister(pipe); pipe.close()
                else:
                    data = os.read(pipe.fileno(), 65536)
                    if not data: selector.unregister(pipe); continue
                    buffers[name].extend(data)
                    if len(buffers['out']) + len(buffers['err']) > PIPE_MAX:
                        raise Invalid('outbound helper reply byte limit; outcome unknown')
        result = loads(bytes(buffers['out']).decode(), PIPE_MAX)
        if not isinstance(result, dict) or set(result) not in ({'status', 'body'}, {'error', 'unknown'}):
            raise Invalid('outbound helper response invalid; outcome unknown')
        answer = result
    except (TimeoutError, subprocess.TimeoutExpired):
        raise Unknown('outbound deadline reached; outcome unknown') from None
    except (OSError, UnicodeError, ValueError):
        raise Unknown('outbound helper failed or exceeded bounds; outcome unknown') from None
    finally:
        selector.close()
        # Reap the whole session even when the helper exited but left a child/pipe.
        try: os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError: pass
        process.wait()
        for pipe in (process.stdin, process.stdout, process.stderr):
            if not pipe.closed: pipe.close()
    if process.returncode:
        raise Unknown('outbound helper failed; outcome unknown')
    return answer


def worker():
    try:
        raw = sys.stdin.buffer.read(PIPE_MAX + 1)
        if len(raw) > PIPE_MAX: raise Invalid('input byte limit')
        request = loads(raw.decode(), PIPE_MAX)
        result = perform(request)
    except Exception:
        result = {'error': 'outbound helper failed', 'unknown': True}
    sys.stdout.write(json.dumps(result, ensure_ascii=False, allow_nan=False))


if __name__ == '__main__':
    if sys.argv[1:] == ['--worker']: worker()
    else: sys.exit(2)
