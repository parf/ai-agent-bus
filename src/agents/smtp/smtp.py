#!/usr/bin/env python3
"""Submit one bounded message; the runner supplies identity and credentials."""
from email.message import EmailMessage
from email.policy import SMTP as SMTP_POLICY
from email.utils import formatdate
import hashlib
import base64
import ipaddress
import json
import os
from pathlib import Path
import re
import signal
import smtplib
import socket
import ssl
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE / 'support/agents/shared' if (HERE / 'support').exists() else HERE.parent / 'shared'))
from metadata import dispatch
from mail import MailError, SMTP, SMTP_TLS, deadline, tls_context

MAX_BYTES = 1 << 20
# Deliberately no display names, quoted local parts, address lists or SMTPUTF8.
ADDRESS = re.compile(r"[A-Za-z0-9_+'-]+(?:\.[A-Za-z0-9_+'-]+)*@(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+(?:[A-Za-z]{2,63}|xn--[a-zA-Z0-9-]{1,59})\Z")


def address(value):
    if not isinstance(value, str) or len(value) > 254 or not ADDRESS.fullmatch(value):
        raise MailError('expected one bare ASCII email address')
    local, domain = value.rsplit('@', 1)
    if len(local) > 64:
        raise MailError('email local part exceeds 64 characters')
    return local + '@' + domain.lower()


def configuration(value):
    allowed = {'host', 'port', 'mode', 'auth', 'sender', 'recipients', 'cafile', 'allow_plaintext'}
    if not isinstance(value, dict) or set(value) - allowed:
        raise MailError('unknown SMTP Configuration key')
    result = {'host': 'smtp.gmail.com', 'port': 587, 'mode': 'starttls', 'auth': 'password', **value}
    result['sender'] = address(result.get('sender'))
    host, port, mode = result['host'], result['port'], result['mode']
    if not isinstance(host, str) or not host or len(host) > 253 or any(ord(c) <= 32 or ord(c) >= 127 for c in host):
        raise MailError('invalid SMTP host')
    if type(port) is not int or not 1 <= port <= 65535:
        raise MailError('port must be an integer in 1..65535')
    if mode not in {'starttls', 'tls', 'plain'}:
        raise MailError('mode must be starttls, tls or plain')
    if result['auth'] != 'password':
        raise MailError('auth must be password (XOAUTH2 is a pending increment)')
    if 'allow_plaintext' in result and type(result['allow_plaintext']) is not bool:
        raise MailError('allow_plaintext must be boolean')
    if mode == 'plain' and not result.get('allow_plaintext', False):
        raise MailError('plain mode requires allow_plaintext')
    if port == 25 and not result.get('allow_plaintext', False):
        raise MailError('port 25 requires allow_plaintext')
    if 'cafile' in result and (not isinstance(result['cafile'], str) or not result['cafile']):
        raise MailError('cafile must name a trusted public CA file')
    limits = result.get('recipients', [])
    if not isinstance(limits, list) or len(limits) > 100 or any(not isinstance(x, str) for x in limits):
        raise MailError('recipients must list at most 100 exact addresses or domains')
    result['recipients'] = []
    for entry in limits:
        if '@' in entry:
            result['recipients'].append(address(entry))
        else:
            domain = address('check@' + entry).rsplit('@', 1)[1]
            result['recipients'].append(domain)
    if result.get('cafile') and not Path(result['cafile']).is_file():
        raise MailError('cafile must name an existing public CA file')
    return result


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise MailError('duplicate JSON key')
        result[key] = value
    return result


def build(envelope, config):
    body = envelope.get('body')
    if not isinstance(body, str) or len(body.encode()) > MAX_BYTES:
        raise MailError('send body exceeds 1048576 bytes or is not JSON text')
    value = json.loads(body, object_pairs_hook=unique_object)
    if not isinstance(value, dict) or set(value) - {'method', 'to', 'cc', 'bcc', 'subject', 'text', 'html'} or value.get('method') != 'send':
        raise MailError('expected send with to, subject, text and optional cc, bcc, html')
    recipients, groups = [], {}
    for name in ('to', 'cc', 'bcc'):
        entries = value.get(name, [])
        if not isinstance(entries, list) or (name == 'to' and not entries):
            raise MailError(name + ' must be an address list; to cannot be empty')
        groups[name] = [address(entry) for entry in entries]
        recipients.extend(groups[name])
    recipients = list(dict.fromkeys(recipients))
    if not 1 <= len(recipients) <= 20:
        raise MailError('send requires 1..20 unique recipients total')
    for recipient in recipients:
        domain = recipient.rsplit('@', 1)[1]
        if config['recipients'] and recipient not in config['recipients'] and domain not in config['recipients']:
            raise MailError('recipient is outside Configuration recipients allowlist')
    subject, text, html = value.get('subject'), value.get('text'), value.get('html')
    if not isinstance(subject, str) or len(subject.encode()) > 998 or any(ord(c) < 32 or ord(c) == 127 for c in subject):
        raise MailError('subject must be one line of at most 998 bytes')
    if not isinstance(text, str) or (html is not None and not isinstance(html, str)):
        raise MailError('text and optional html must be strings')
    if any(ord(c) < 32 and c not in '\t\r\n' or ord(c) == 127 for c in text + (html or '')):
        raise MailError('mail bodies cannot contain NUL or other control characters')
    delivered_id = envelope.get('message_id')
    if not isinstance(delivered_id, str) or not delivered_id:
        raise MailError('delivered message id is missing')
    message = EmailMessage(policy=SMTP_POLICY.clone(cte_type='7bit'))
    message['From'] = config['sender']
    message['To'] = ', '.join(groups['to'])
    if groups['cc']:
        message['Cc'] = ', '.join(groups['cc'])
    message['Subject'] = subject
    message['Date'] = formatdate(usegmt=True)
    message['Message-ID'] = '<' + hashlib.sha256(delivered_id.encode()).hexdigest() + '@' + config['sender'].rsplit('@', 1)[1] + '>'
    message.set_content(text)
    if html is not None:
        message.add_alternative(html, subtype='html')
    raw = message.as_bytes()
    if len(raw) > MAX_BYTES:
        raise MailError('encoded message exceeds 1048576 bytes')
    if re.search(rb'(?<!\r)\n|\r(?!\n)', raw):
        raise MailError('serialized mail contains a bare CR or LF')
    return recipients, raw


def answer(envelope, config, env):
    transport, timer = None, None
    secrets = [env.get('SMTP_PASSWORD', '')]
    user = env.get('SMTP_USERNAME', '')
    if secrets[0] and user:
        secrets.append(base64.b64encode(('\x00' + user + '\x00' + secrets[0]).encode()).decode())
    outcome, count, size = 'not_sent', 0, 0

    def safe(value):
        result = str(value)
        for secret in secrets:
            if secret:
                result = result.replace(secret, '***')
        return result

    def reply(result):
        nonlocal outcome
        outcome = result['outcome']
        return safe(json.dumps(result, ensure_ascii=False)), 0

    def failure(detail):
        state = 'unknown' if transport and transport.data_started else 'not_sent'
        return reply({'outcome': state, 'error': detail, **({'note': 'Delivery outcome unknown; do not automatically retry'} if state == 'unknown' else {})})

    try:
        config = configuration(config)
        recipients, raw = build(envelope, config)
        count, size = len(recipients), len(raw)
        user, password = env.get('SMTP_USERNAME', ''), env.get('SMTP_PASSWORD', '')
        if not user or not password:
            raise MailError('SMTP_USERNAME and SMTP_PASSWORD are required in this instance host env')
        if not password.isascii() or not user.isascii():
            raise MailError('SMTP_USERNAME and SMTP_PASSWORD must be ASCII')
        if any(ord(c) < 32 or ord(c) == 127 for c in user + password):
            raise MailError('SMTP credentials contain invalid control characters')
        timer, remaining = deadline(envelope)
        if config['mode'] == 'plain':
            peers = socket.getaddrinfo(config['host'], config['port'], type=socket.SOCK_STREAM)
            if not peers or any(not ipaddress.ip_address(peer[4][0]).is_loopback for peer in peers):
                raise MailError('plaintext is allowed only for a loopback server')
            # Connect to a validated numeric address, avoiding a second DNS resolution.
            host = peers[0][4][0]
        else:
            host = config['host']
        context = tls_context(config.get('cafile'))
        transport = SMTP_TLS(timeout=remaining, context=context, local_hostname=config['sender'].rsplit('@', 1)[1]) if config['mode'] == 'tls' else SMTP(timeout=remaining, local_hostname=config['sender'].rsplit('@', 1)[1])
        transport._host = host  # smtplib uses this for STARTTLS hostname verification.
        code, _ = transport.connect(host, config['port'])
        if code != 220:
            raise MailError('SMTP greeting refused')
        if transport.ehlo()[0] != 250:
            raise MailError('SMTP EHLO refused')
        if config['mode'] == 'starttls':
            if not transport.has_extn('starttls'):
                raise MailError('SMTP server does not advertise required STARTTLS')
            transport.starttls(context=context)
            if transport.ehlo()[0] != 250:
                raise MailError('SMTP EHLO after STARTTLS refused')
        if 'PLAIN' not in transport.esmtp_features.get('auth', '').upper().split():
            raise MailError('SMTP server does not offer selected AUTH PLAIN')
        transport.auth('PLAIN', lambda challenge=None: '\x00' + user + '\x00' + password)
        refused = transport.sendmail(config['sender'], recipients, raw)
        accepted = [recipient for recipient in recipients if recipient not in refused]
        result = {'outcome': 'partial' if refused else 'accepted', 'accepted': accepted,
                  'refused': {recipient: {'code': code, 'message': safe(message.decode('utf-8', 'replace'))[:1024]}
                              for recipient, (code, message) in refused.items()},
                  'server': safe(transport.acceptance.decode('utf-8', 'replace'))[:1024]}
        return reply(result)
    except (smtplib.SMTPRecipientsRefused, smtplib.SMTPResponseException) as error:
        if isinstance(error, smtplib.SMTPRecipientsRefused):
            return reply({'outcome': 'refused', 'refused': {recipient: {'code': code, 'message': safe(message.decode('utf-8', 'replace'))[:1024]} for recipient, (code, message) in error.recipients.items()}})
        return reply({'outcome': 'refused', 'code': error.smtp_code,
                      'error': safe(error.smtp_error.decode('utf-8', 'replace'))[:1024]})
    except (ssl.SSLError, OSError, TimeoutError, smtplib.SMTPException):
        return failure('SMTP transport/TLS operation failed')
    except (MailError, ValueError, TypeError, KeyError, AttributeError, UnicodeError) as error:
        # Never surface parser/library exception text, which may contain private input.
        detail = safe(error) if isinstance(error, MailError) else 'invalid message or Configuration'
        return failure(detail)
    finally:
        print(json.dumps({'outcome': outcome, 'recipients': count, 'message_bytes': size}), file=sys.stderr)
        if transport is not None:
            transport.close()
        if timer is not None:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, timer)


def main():
    result = dispatch(HERE, os.environ.get('AGENT_BUS_ASK'))
    if result is not None:
        print(result)
        return 0
    try:
        raw = sys.stdin.buffer.read(2 * MAX_BYTES + 1)
        if len(raw) > 2 * MAX_BYTES:
            raise ValueError()
        envelope = json.loads(raw, object_pairs_hook=unique_object)
        output, code = answer(envelope, json.loads(os.environ.get('AGENT_BUS_CONFIG', '{}')), os.environ)
    except (ValueError, TypeError):
        output, code = '{"outcome":"not_sent","error":"invalid message or Configuration"}', 0
    print(output)
    return code


if __name__ == '__main__':
    sys.exit(main())
