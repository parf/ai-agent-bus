#!/usr/bin/env python3
"""One bounded Voyage POST per runner call, without provider credentials on the bus."""
import http.client
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
from provider_http import MAX_BYTES, ProxyError, redact, scrub, provider_error, arm_deadline, connect, post

ORIGIN = 'https://api.voyageai.com'


def request_bytes(body, config):
    if not isinstance(body, str) or len(body.encode()) > MAX_BYTES:
        raise ProxyError('request exceeds 1048576 bytes or is not text')
    if not isinstance(config, dict) or set(config) - {'model'}:
        raise ProxyError('Configuration accepts only model')
    if body.startswith('request '):
        path, separator, raw = body[8:].partition(' ')
        if not separator:
            raise ProxyError('request takes a /v1/ path and a JSON body')
        model = ''
    else:
        value = json.loads(body) if body.lstrip().startswith('{') else {'method':'embed', 'text':body.removeprefix('embed ')}
        if not isinstance(value, dict):
            raise ProxyError('expected an embed or request object')
        if value.get('method') == 'request' and set(value) == {'method','path','body'}:
            path, raw, model = value['path'], value['body'], ''
        elif value.get('method') == 'embed' and not set(value) - {'method','text','model','input_type'}:
            text = value.get('text')
            model = value.get('model', config.get('model', 'voyage-3.5'))
            if not isinstance(text, str) or not text or not isinstance(model, str) or not re.fullmatch(r'[A-Za-z0-9._/-]{1,128}', model):
                raise ProxyError('embed needs non-empty text and a model ID (1..128 letters, digits, . _ / -)')
            payload = {'input':text, 'model':model}
            if 'input_type' in value:
                if value['input_type'] not in (None,'query','document'):
                    raise ProxyError('input_type must be query, document or null')
                payload['input_type'] = value['input_type']
            path, raw = '/v1/embeddings', json.dumps(payload, ensure_ascii=False)
        else:
            raise ProxyError('expected an embed or request object')
    # Paths are API names, never URLs or encoded/traversing filesystem paths.
    if not isinstance(path, str) or not re.fullmatch(r'/v1/[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*', path):
        raise ProxyError('path must stay within /v1/')
    if not isinstance(raw, str) or len(raw.encode()) > MAX_BYTES:
        raise ProxyError('request exceeds 1048576 bytes or is not text')
    parsed = json.loads(raw)
    if not isinstance(parsed, dict) or parsed.get('stream'):
        raise ProxyError('expected a non-streaming JSON object')
    return path, raw.encode(), model


def answer(envelope, config, key):
    if not key:
        return 'error: VOYAGE_API_KEY is not set', 78
    timer = connection = None
    status, model, sent, received = 'invalid', '', 0, 0
    try:
        path, payload, model = request_bytes(envelope['body'], config)
        sent = len(payload)
        timer, remaining = arm_deadline(envelope)
        connection = connect(urllib.parse.urlsplit(ORIGIN), remaining)
        response = post(connection, path, payload, key)
        status = response.status
        raw = response.read(MAX_BYTES + 1)
        received = len(raw)
        if received > MAX_BYTES:
            raise ProxyError('provider answer exceeds 1048576 bytes')
        try:
            result = scrub(json.loads(raw), key)
        except (ValueError, UnicodeError):
            if status >= 300:
                return provider_error(status, redact(raw.decode('utf-8', errors='replace'), key)), 0
            raise ProxyError('provider did not return JSON') from None
        if status >= 300:
            error = result.get('error', result) if isinstance(result, dict) else result
            message = error.get('message', error) if isinstance(error, dict) else error
            return provider_error(status, message), 0
        output = json.dumps(result, ensure_ascii=False)
        if len(output.encode()) > MAX_BYTES:
            raise ProxyError('provider answer exceeds 1048576 bytes')
        return output, 0
    except TimeoutError:
        status = 'timeout'
        return 'error: provider request timed out', 0
    except ProxyError as error:
        return 'error: ' + redact(str(error), key), 0
    except (ValueError, TypeError, KeyError, AttributeError):
        return 'error: invalid request, Configuration or provider response', 0
    except (OSError, http.client.HTTPException):
        status = 'unavailable'
        return 'error: provider connection failed', 0
    finally:
        if timer is not None:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, timer)
        if connection is not None:
            connection.close()
        print(json.dumps({'model':redact(model, key), 'request_bytes':sent,
                          'answer_bytes':received, 'status':status}), file=sys.stderr)


def main():
    result = dispatch(HERE, os.environ.get('AGENT_BUS_ASK'))
    if result is not None:
        print(result)
        return 0
    key = os.environ.get('VOYAGE_API_KEY', '')
    if not key:
        print('VOYAGE_API_KEY is not set', file=sys.stderr)
        print('error: VOYAGE_API_KEY is not set')
        return 78
    try:
        raw = sys.stdin.buffer.read(8 * MAX_BYTES + 1)
        if len(raw) > 8 * MAX_BYTES:
            raise ValueError()
        output, code = answer(json.loads(raw), json.loads(os.environ.get('AGENT_BUS_CONFIG','{}')), key)
    except (ValueError, TypeError):
        output, code = 'error: invalid message or Configuration', 0
    print(output)
    return code


if __name__ == '__main__':
    sys.exit(main())
