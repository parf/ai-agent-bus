#!/usr/bin/env python3
"""One bus call, at most one Telnyx POST. No delivery callbacks or retries."""
import datetime
import http.client
import json
import os
import re
import pwd
import signal
import sys
import time
import uuid
import urllib.parse

HOST = 'api.telnyx.com'
DEFAULT_URL = 'https://api.telnyx.com/v2/messages'
MAX_BYTES = 65536


def phone(value, aliases=None):
    if not isinstance(value, str):
        raise ValueError()
    value = (aliases or {}).get(value.lower(), value).strip()
    if not re.fullmatch(r'\+?[0-9().\s-]+', value):
        raise ValueError()
    digits = re.sub(r'[^0-9]', '', value)
    if value.startswith('+'):
        normalized = '+' + digits
    elif len(digits) == 10:
        normalized = '+1' + digits
    elif len(digits) == 11 and digits.startswith('1'):
        normalized = '+' + digits
    else:
        raise ValueError()
    if not re.fullmatch(r'\+[1-9][0-9]{7,14}', normalized):
        raise ValueError()
    return normalized


def expired(*_):
    raise TimeoutError()


def endpoint(envelope):
    url = urllib.parse.urlsplit(os.environ.get('TELNYX_API_URL', DEFAULT_URL))
    live = (pwd.getpwuid(os.geteuid()).pw_name.endswith('-ab-runner')
            and os.environ.get('AGENT_BUS_TO', '').startswith('#sms-telnyx@')
            and os.environ.get('AGENT_BUS_MESSAGE_ID') == envelope.get('message_id')
            and bool(envelope.get('message_id')))
    if url.username or url.password or url.query or url.fragment:
        raise ValueError()
    if url.scheme == 'https' and url.hostname == HOST and url.port in (None, 443):
        if not live:
            raise ValueError()
    elif url.scheme == 'http' and url.hostname == '127.0.0.1' and url.port:
        if live:
            raise ValueError()
    else:
        raise ValueError()
    return url


def answer(envelope, config, key, connect=None):
    result = {'status': 'invalid', 'to': None, 'from': None, 'provider_message_id': None}
    connection, timer = None, None
    caller = str(envelope.get('from', 'unknown')).replace(key, '***')[:512] if key else 'unknown'
    try:
        if not key:
            return result, 78
        if not isinstance(config, dict) or set(config) - {'from', 'aliases'}:
            raise ValueError()
        aliases = config.get('aliases', {})
        if not isinstance(aliases, dict) or any(not isinstance(k, str) or not isinstance(v, str) for k, v in aliases.items()):
            raise ValueError()
        result['from'] = phone(config.get('from'))
        body = envelope.get('body')
        if not isinstance(body, str) or len(body.encode()) > MAX_BYTES:
            raise ValueError()
        if body.lstrip().startswith('{'):
            request = json.loads(body)
        else:
            to, text = body.split(None, 1)
            request = {'to': to, 'text': text}
        if not isinstance(request, dict) or set(request) != {'to', 'text'}:
            raise ValueError()
        result['to'] = phone(request['to'], aliases)
        text = request['text']
        if not isinstance(text, str) or not 1 <= len(text) <= 1600:
            raise ValueError()
        url = endpoint(envelope)
        remaining = 20.0
        deadline = envelope.get('deadline')
        if deadline and not deadline.startswith('0001-'):
            remaining = min(remaining, datetime.datetime.fromisoformat(deadline.replace('Z', '+00:00')).timestamp() - time.time() - .1)
        result['status'] = 'unknown'
        if remaining <= 0:
            return result, 0
        timer = signal.signal(signal.SIGALRM, expired)
        signal.setitimer(signal.ITIMER_REAL, remaining)
        factory = connect or (http.client.HTTPSConnection if url.scheme == 'https' else http.client.HTTPConnection)
        connection = factory(url.netloc, timeout=remaining)
        payload = json.dumps({'to': result['to'], 'from': result['from'], 'text': text}).encode()
        # Exactly one attempt. A transport error may follow an accepted SMS.
        connection.request('POST', url.path, payload,
                           {'Authorization': 'Bearer ' + key, 'Content-Type': 'application/json'})
        response = connection.getresponse()
        if 400 <= response.status < 500:
            result['status'] = 'failed'
        elif 200 <= response.status < 300:
            raw = response.read(MAX_BYTES + 1)
            if len(raw) <= MAX_BYTES:
                data = json.loads(raw)
                # Only a canonical provider id leaves this process, never arbitrary echoed data.
                provider_id = str(uuid.UUID(data['data']['id']))
                if key.lower() in provider_id.lower():
                    raise ValueError()
                result['provider_message_id'] = provider_id
                result['status'] = 'accepted'
        # Redirects, 5xx, malformed success, timeouts: unknown, never a second POST.
    except (ValueError, TypeError, KeyError, AttributeError):
        pass
    except (OSError, http.client.HTTPException):
        pass
    finally:
        if timer is not None:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, timer)
        if connection is not None:
            connection.close()
        print(json.dumps({'caller': caller, 'last4': (result['to'] or '')[-4:],
                          'status': result['status'], 'id': result['provider_message_id']}), file=sys.stderr)
    return result, 0


def main():
    key = os.environ.get('TELNYX_API_KEY', '')
    if not key:
        print('error: TELNYX_API_KEY is not set')
        return 78
    try:
        raw = sys.stdin.buffer.read(MAX_BYTES + 1)
        if len(raw) > MAX_BYTES:
            raise ValueError()
        envelope = json.loads(raw)
        config = json.loads(os.environ.get('AGENT_BUS_CONFIG', '{}'))
        result, code = answer(envelope, config, key)
    except (ValueError, TypeError, AttributeError):
        result, code = {'status': 'invalid', 'to': None, 'from': None, 'provider_message_id': None}, 0
    print(json.dumps(result))
    return code


if __name__ == '__main__':
    sys.exit(main())
