#!/usr/bin/env python3
"""One main-thread reader: alarm-bounded IMAP calls and a serialized private cache."""
import base64
import hashlib
import imaplib
import json
import os
from pathlib import Path
import re
import signal
import sys

HERE = Path(__file__).resolve().parent
SUPPORT = HERE / 'support' if (HERE / 'support').is_dir() else HERE.parents[1]
sys.path.insert(0, str(SUPPORT / 'clients/python'))
sys.path.insert(0, str(SUPPORT / 'agents/shared'))
sys.path.insert(0, str(HERE))
from agent_bus import Bus
from agent_bus.errors import BusError, BusTimeout, Refusal, TransportError, UncertainOutcome
from cache import Cache, CacheError, MiB, uid
from mail import MailError, deadline, tls_context
from mail_oauth import google_access_token
from metadata import dispatch
import wire
from publication import MAX_OUTPUT, WARNING, PublicationError, render



def config(value):
    allowed = {'host', 'port', 'mode', 'auth', 'mailbox', 'target', 'cache_bytes', 'cafile', 'allow_plaintext'}
    if not isinstance(value, dict) or set(value) - allowed:
        raise MailError('unknown IMAP Configuration key')
    result = {'host': 'imap.gmail.com', 'port': 993, 'mode': 'tls', 'auth': 'password',
              'mailbox': 'INBOX', 'cache_bytes': 64 * MiB, **value}
    if not isinstance(result['host'], str) or not result['host'] or len(result['host']) > 253 or any(ord(c) <= 32 or ord(c) >= 127 for c in result['host']):
        raise MailError('invalid IMAP host')
    if type(result['port']) is not int or not 1 <= result['port'] <= 65535:
        raise MailError('IMAP port must be in 1..65535')
    if result['mode'] not in {'tls', 'plain'} or result['auth'] not in {'password', 'xoauth2'}:
        raise MailError('mode must be tls/plain; auth must be password/xoauth2')
    if 'allow_plaintext' in result and type(result['allow_plaintext']) is not bool:
        raise MailError('allow_plaintext must be boolean')
    if result['mode'] == 'plain' and not result.get('allow_plaintext', False):
        raise MailError('plain IMAP requires the explicit loopback opt-out')
    wire.mailbox(result['mailbox'])  # Validate before a connection or credentials leave.
    if type(result['cache_bytes']) is not int or not 4 * MiB <= result['cache_bytes'] <= 512 * MiB:
        raise MailError('cache_bytes must be in 4194304..536870912')
    if 'cafile' in result and (not isinstance(result['cafile'], str) or not Path(result['cafile']).is_file()):
        raise MailError('cafile must name an existing public CA file')
    target = result.get('target')
    if target is not None and (not isinstance(target, str) or not target or len(target.encode()) > 512 or any(ord(c) <= 32 or ord(c) == 127 for c in target)):
        raise MailError('target must be one fixed bus destination without whitespace')
    return result


def object_pairs(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise MailError('duplicate request key')
        result[key] = value
    return result


def parse(body):
    if not isinstance(body, str) or len(body.encode()) > 8192:
        raise MailError('call must be text or JSON of at most 8192 bytes')
    if body.startswith('{'):
        value = json.loads(body, object_pairs_hook=object_pairs)
    else:
        method, _, rest = body.partition(' ')
        value = {'method': method}
        if rest:
            value['text' if method == 'search' else 'uid' if method in {'read', 'publish'} else 'limit'] = rest
    if not isinstance(value, dict):
        raise MailError('expected an IMAP call')
    method = value.get('method')
    fields = {'sync': {'limit'}, 'list': {'limit'}, 'search': {'text'}, 'read': {'uid'}, 'publish': {'uid'}}
    if method not in fields or set(value) - {'method'} - fields[method]:
        raise MailError('use sync, list, search, read or publish with their documented arguments')
    if method in {'read', 'publish'}:
        value['uid'] = uid(value.get('uid'))
    elif method in {'sync', 'list'}:
        limit = value.get('limit', 100 if method == 'sync' else 20)
        if isinstance(limit, str) and re.fullmatch(r'[1-9][0-9]{0,2}', limit):
            limit = int(limit)
        if type(limit) is not int or not 1 <= limit <= 100:
            raise MailError('limit must be in 1..100')
        value['limit'] = limit
    else:
        text = value.get('text')
        if not isinstance(text, str) or not text or len(text.encode()) > 4096 or any(ord(c) < 32 or ord(c) == 127 for c in text):
            raise MailError('search requires 1..4096 UTF-8 bytes of local text without controls')
    return value


class Agent:
    def __init__(self, bus, configuration, environment, work):
        self.bus, self.configuration, self.environment, self.work = bus, configuration, environment, work

    def handle(self, message):
        result = dispatch(HERE, message.ask)
        if result is not None:
            return result
        connection = timer = None
        method, status, error_class, phase = 'invalid', 'error', 'invalid', 'validation'
        try:
            cfg, call = config(self.configuration), parse(message.body)
            method = call['method']
            if method == 'publish' and not cfg.get('target'):
                raise MailError('publish requires Configuration target')
            user, password = self.environment.get('IMAP_USERNAME', ''), self.environment.get('IMAP_PASSWORD', '')
            if not user or not user.isascii() or any(ord(c) < 32 or ord(c) == 127 for c in user):
                raise MailError('IMAP_USERNAME must be a nonempty ASCII account name without controls')
            if cfg['auth'] == 'password' and (not password or not password.isascii() or any(ord(c) < 32 or ord(c) == 127 for c in password)):
                raise MailError('IMAP_PASSWORD must be a nonempty ASCII app password without controls')
            secrets = [self.environment.get(name, '') for name in ('IMAP_PASSWORD', 'IMAP_CLIENT_SECRET', 'IMAP_REFRESH_TOKEN')]
            phase = 'deadline'
            timer, remaining = deadline(message.envelope)
            phase = 'operation'
            token = google_access_token(self.environment, remaining, prefix='IMAP') if cfg['auth'] == 'xoauth2' else None
            if token:
                secrets.extend([token, base64.b64encode(token.encode()).decode()])
            if cfg['mode'] == 'tls':
                connection = wire.TLS(cfg['host'], cfg['port'], ssl_context=tls_context(cfg.get('cafile')), timeout=remaining)
            else:
                connection = wire.Plain(wire.local_host(cfg['host'], cfg['port']), cfg['port'], timeout=remaining)
            if token:
                if 'AUTH=XOAUTH2' not in connection.capabilities:
                    raise MailError('IMAP server does not offer selected XOAUTH2')
                initial = ('user=' + user + '\x01auth=Bearer ' + token + '\x01\x01').encode()
                secrets.append(base64.b64encode(initial).decode())
                sent = False
                def authenticate(_challenge):
                    nonlocal sent
                    if sent:
                        return b''
                    sent = True
                    return initial
                connection.authenticate('XOAUTH2', authenticate)
            else:
                connection.login('"' + user.replace('\\', '\\\\').replace('"', '\\"') + '"', password)
            validity, next_uid = wire.selected(connection, cfg['mailbox'])
            identity = hashlib.sha256(json.dumps([cfg['host'], cfg['port'], user, cfg['mailbox']], ensure_ascii=False).encode()).hexdigest()
            with Cache(self.work, identity, validity, cfg['cache_bytes']) as cache:
                if method == 'sync':
                    fetched, attempted, pages, skipped = 0, 0, 0, 0
                    details, tainted, retry = [], False, None
                    while cache.state['cursor'] < next_uid - 1 and attempted < call['limit'] and pages < 10 and not tainted and retry is None:
                        start = cache.state['cursor'] + 1
                        end = min(start + wire.WINDOW - 1, next_uid - 1)
                        numbers = wire.search(connection, start, end)
                        pages += 1
                        for number in numbers:
                            if not cache.has(number):
                                attempted += 1
                                try:
                                    body, note = wire.fetch(connection, number)
                                    cache.put(number, body)
                                    fetched += 1
                                    if note and len(details) < 100:
                                        details.append(f'UID {number}: ' + note)
                                except wire.Refused:
                                    pending = cache.state['retry']
                                    count = pending['count'] + 1 if pending and pending['uid'] == number else 1
                                    if count <= 3:
                                        cache.state['retry'] = {'uid': number, 'count': count}
                                        retry = number
                                        break  # Persist one pending UID across explicit sync calls.
                                    skipped += 1
                                    if len(details) < 100:
                                        details.append(f'UID {number}: server refused FETCH repeatedly; use read <uid> to retry')
                                except wire.WireError as error:
                                    skipped += 1
                                    if len(details) < 100:
                                        details.append(f'UID {number}: ' + str(error))
                                    tainted = isinstance(error, wire.Tainted)
                            cache.state['cursor'] = number
                            cache.state['retry'] = None
                            if attempted >= call['limit'] or tainted:
                                break
                        else:
                            cache.state['cursor'] = end
                            cache.state['retry'] = None
                        cache.save_state()
                    result = f"fetched {fetched}; cursor {cache.state['cursor']}; UIDNEXT {next_uid}; more {'yes' if cache.state['cursor'] < next_uid - 1 else 'no'}; skipped {skipped}"
                    if details:
                        result += '\n' + '\n'.join(details)
                    if retry is not None:
                        result += f"\nretry UID {retry} on the next sync (server refused FETCH, retry {cache.state['retry']['count']}/3)"
                    if skipped:
                        result += '\nuse read <uid> to retry a skipped UID'
                elif method in {'list', 'search'}:
                    limit, rows, used, omitted, full = call.get('limit', 100), [], 0, 0, False
                    files = sorted(cache.files(), key=lambda item: int(item[0][:-4]), reverse=True)
                    for index, (name, _, _) in enumerate(files):
                        if method == 'list' and (full or len(rows) >= limit):
                            omitted += len(files) - index
                            break
                        number = int(name[:-4])
                        raw = cache.get(number, touch=False)
                        if method == 'search' and call['text'].casefold() not in raw.decode('utf-8', 'backslashreplace').casefold():
                            continue
                        if full or len(rows) >= limit:
                            omitted += 1
                            continue  # Search counts matches without rendering or touching them.
                        header = raw.partition(b'\r\n\r\n')[0].partition(b'\n\n')[0][:65536]
                        row = 'UID ' + str(number) + ' (cached)\n' + render(header, secrets)
                        size = len(row.encode()) + 2
                        if used + size <= MAX_OUTPUT - 128:
                            rows.append(row)
                            used += size
                            cache.touch(number)
                        else:
                            full = True
                            omitted += 1
                    result = '\n\n'.join(rows) if rows else 'no cached matches; sync to discover mail'
                    if omitted and not rows:
                        result = 'cached matches exceed the answer limit'
                    if omitted:
                        result += f'\ntruncated; {omitted} more cached matches'
                else:
                    number = call['uid']
                    if not wire.exists(connection, number):
                        cache.remove(number)
                        raise MailError('UID is no longer present in the mailbox')
                    cached = cache.has(number)
                    note = None
                    if not cached:
                        body, note = wire.fetch(connection, number)
                        cache.put(number, body)
                    result = render(cache.get(number), secrets)
                    if method == 'publish':
                        self.bus.send(cfg['target'], result)
                        result = f'published UID {number} to the configured destination'
                    else:
                        result = f"UID {number} ({'cached' if cached else 'fetched'})" + ('; ' + note if note else '') + '\n' + result
                if cache.repaired:
                    result = f'cache repairs: {cache.repaired}\n' + result
                if len(result.encode()) > MAX_OUTPUT:
                    raise MailError('answer exceeds the 262144-byte limit; narrow the result')
                status = 'ok'
                return result
        except (MailError, PublicationError, CacheError, wire.WireError) as error:
            error_class = 'cache' if isinstance(error, CacheError) else 'protocol' if isinstance(error, wire.WireError) else 'invalid'
            return 'error: ' + str(error)
        except imaplib.IMAP4.error:
            error_class = 'server_refused'
            return 'error: IMAP server refused the operation'
        except Refusal:
            error_class = 'bus_refused'
            return 'error: publication refused by the bus'
        except UncertainOutcome:
            error_class = 'unconfirmed'
            return 'error: publication outcome unknown; inspect the destination before repeating'
        except (BusTimeout, TransportError):
            error_class = 'bus_unavailable'
            return 'error: bus unavailable; publication not sent'
        except BusError:
            error_class = 'unconfirmed'
            return 'error: publication outcome unknown; inspect the destination before repeating'
        except TimeoutError:
            error_class = 'expired' if phase == 'deadline' else 'timeout'
            if error_class == 'expired':
                return 'error: request deadline expired before IMAP execution'
            return 'error: IMAP operation timed out'
        except OSError:
            error_class = 'connection'
            return 'error: IMAP connection, TLS or private cache operation failed'
        except (ValueError, KeyError, TypeError, AttributeError, UnicodeError):
            error_class = 'invalid_response'
            return 'error: invalid call, Configuration or IMAP response'
        finally:
            if connection is not None:
                try:
                    connection.shutdown()
                except OSError:
                    pass
            if timer is not None:
                signal.setitimer(signal.ITIMER_REAL, 0)
                signal.signal(signal.SIGALRM, timer)
            print(json.dumps({'method': method, 'status': status,
                              **({'error_class': error_class} if status != 'ok' else {})}), file=sys.stderr)


class Shutdown(Exception):
    pass


def main():
    result = dispatch(HERE, os.environ.get('AGENT_BUS_ASK'))
    if result is not None:
        print(result)
        return 0
    def stop(*_):
        raise Shutdown()
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    try:
        cfg = json.loads(os.environ.get('AGENT_BUS_CONFIG', '{}'))
        with Bus(timeout=5) as bus:
            agent = Agent(bus, cfg, os.environ, os.environ.get('AGENT_BUS_WORK', '.'))
            # A raw main-thread reader keeps SIGALRM effective during DNS/TLS/flock.
            while True:
                message = bus.consume(wait=10)
                if message is None or message.receipt or message.re:
                    continue
                answer = agent.handle(message)
                try:
                    message.reply(answer)
                except BusError:
                    print('imap: answer delivery failed', file=sys.stderr)
    except Shutdown:
        return 0
    except (BusError, ValueError):
        print('imap: bus or Configuration unavailable', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
