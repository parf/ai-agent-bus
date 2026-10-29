#!/usr/bin/env python3
"""One main-thread reader: alarm-bounded IMAP calls and a serialized private cache."""
from email import policy
from email.parser import BytesHeaderParser
import unicodedata
import base64
import hashlib
import imaplib
import json
import os
from pathlib import Path
import re
import signal
import sys
import time

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
import progress
from publication import MAX_OUTPUT, WARNING, PublicationError, render
from publication import validate_id
from subscriptions import SubscriptionError, target_text
from outbox import PreviousGeneration
from mailbox_session import SessionError



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


def summary(raw, masks):
    """Only bounded decoded presentation headers; never transport metadata."""
    header = raw.partition(b'\r\n\r\n')[0].partition(b'\n\n')[0][:65536]
    try:
        message = BytesHeaderParser(policy=policy.default).parsebytes(header + b'\r\n\r\n')
    except Exception:
        message = {}
    rows = []
    for name, limit in (('Date', 80), ('From', 200), ('Subject', 240)):
        try:
            text = unicodedata.normalize('NFC', str(message.get(name, '(unknown)')))
            text = ''.join(' ' if unicodedata.category(c).startswith('C') else c for c in text)
            text = ' '.join(text.split())
        except Exception:
            text = '(malformed)'
        rows.append(name + ': ' + text[:limit] + ('…' if len(text) > limit else ''))
    return render('\n'.join(rows).encode('utf-8'), masks)


def parse(body):
    if not isinstance(body, str) or len(body.encode()) > 8192:
        raise MailError('call must be text or JSON of at most 8192 bytes')
    if body.startswith('{'):
        value = json.loads(body, object_pairs_hook=object_pairs)
    elif body.startswith('sub add '):
        target, _, pattern = body[len('sub add '):].partition(' ')
        value = {'method': 'sub.add', 'target': target, 'filter': pattern}
    elif body.rstrip() == 'sub list':
        value = {'method': 'sub.list'}
    elif body.startswith('sub delete '):
        value = {'method': 'sub.delete', 'id': body[len('sub delete '):].strip()}
    else:
        method, _, rest = body.partition(' ')
        value = {'method': method}
        if rest:
            value['text' if method == 'search' else 'uid' if method in {'read', 'publish'} else 'limit'] = rest
    if not isinstance(value, dict):
        raise MailError('expected an IMAP call')
    method = value.get('method')
    if not isinstance(method, str):
        raise MailError('method must be text')
    method = {'sub add': 'sub.add', 'sub list': 'sub.list', 'sub delete': 'sub.delete'}.get(method, method)
    value['method'] = method
    fields = {'sync': {'limit'}, 'list': {'limit'}, 'search': {'text'}, 'read': {'uid'}, 'publish': {'uid'},
              'sub.add': {'target', 'filter'}, 'sub.list': set(), 'sub.delete': {'id'}, 'status': set(), 'help': set()}
    if method not in fields or set(value) - {'method'} - fields[method]:
        raise MailError('use help, status, sync, list, search, read, publish or sub add/list/delete')
    if method in {'help', 'status', 'sub.list'}:
        return value
    if method == 'sub.add':
        value['target'] = target_text(value.get('target'))
        value.setdefault('filter', '')
        if not isinstance(value['filter'], str):
            raise MailError('subscription filter must be text')
        return value
    if method == 'sub.delete':
        validate_id(value.get('id'))
        return value
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
        self.forwarder = None
        self.waiting_previous, self.retry_construct, self.start_error = False, 0, None

    def forwarding(self):
        if self.forwarder is None:
            if time.monotonic() < self.retry_construct:
                raise PreviousGeneration('waiting for the previous IMAP generation')
            from forwarding import Forwarder
            from mailbox_session import Session
            source = Session(lambda: config(self.configuration), self.environment, self.work)
            try:
                self.forwarder = Forwarder(self.bus, self.bus.name, self.work, source)
            except PreviousGeneration:
                source.close()
                self.waiting_previous = True
                self.retry_construct = time.monotonic() + 1
                raise
            self.waiting_previous, self.start_error = False, None
        return self.forwarder

    def poll(self):
        try:
            forwarder = self.forwarding()
            forwarder.tick()
            return forwarder.wait()
        except PreviousGeneration:
            return 1
        except (BusError, OSError, ValueError):
            self.start_error = 'forwarding state unavailable'
            return 10

    def status(self):
        if self.forwarder is not None:
            return self.forwarder.status()
        if self.waiting_previous:
            return 'starting: waiting for the previous IMAP generation'
        return ('failed: ' + self.start_error) if self.start_error else 'starting: forwarding not initialized'

    def close(self):
        if self.forwarder is not None:
            self.forwarder.close()

    def handle(self, message):
        result = dispatch(HERE, message.ask, handlers={'status': self.status})
        if result is not None:
            return result
        connection = timer = None
        method, status, error_class, phase = 'invalid', 'error', 'invalid', 'validation'
        try:
            call = parse(message.body)
            method = call['method']
            if method in {'help', 'status'}:
                status = 'ok'
                return dispatch(HERE, 'help') if method == 'help' else self.status()
            if method.startswith('sub.'):
                phase = 'deadline'
                timer, remaining = deadline(message.envelope)
                phase = 'operation'
                forwarder = self.forwarding()
                if method == 'sub.add':
                    sub = forwarder.add(message, call['target'], call['filter'])
                    result = f"added {sub['id']} -> {sub['target']}; by {sub['created_by']}; from UID {sub['ledger']['start']}; receiver must admit this instance"
                    if not sub.get('filter', call['filter']):
                        result += '; matches ALL mail from this account'
                elif method == 'sub.delete':
                    clean = forwarder.delete(message, call['id'])
                    result = 'deleted' + ('' if clean else '; outbox cleanup pending')
                else:
                    result = forwarder.subs.text() + '\n\n' + forwarder.status()
                status = 'ok'
                return result
            if self.waiting_previous:
                self.forwarding()  # No cache mutation while the old reader owns its state.
            cfg = config(self.configuration)
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
                    kv = self.bus.record(self.bus.name).kv
                    try:
                        block = kv.get('imap.progress_error', kind='json')
                    except Refusal as error:
                        if error.status != 404:
                            raise
                        block = None
                    if block is not None:
                        raise progress.ProgressError('forwarding state invalid; delete all subscriptions then add to rebaseline')
                    try:
                        previous = kv.get('imap.progress', kind='json')
                    except Refusal as error:
                        if error.status != 404:
                            raise
                        listing = kv.list()
                        if not isinstance(listing, dict) or not isinstance(listing.get('json', []), list):
                            raise progress.ProgressError('invalid subscription listing')
                        if any(isinstance(row, dict) and str(row.get('name', '')).startswith('sub.')
                               for row in listing.get('json', [])):
                            raise progress.ProgressError('forwarding progress is missing; explicit rebaseline required')
                        previous = cache.state
                    state, _reset = progress.load(previous, identity, validity, next_uid)
                    if state['history'] == 0 and next_uid - 1 > state['history_ceiling']:
                        state = progress.renew_history(state, next_uid)
                    fetched, attempted, pages, skipped = 0, 0, 0, 0
                    details, tainted, retry = [], False, None
                    while state['history'] > 0 and attempted < call['limit'] and pages < 10 and not tainted and retry is None:
                        end = state['history']
                        start = max(1, end - wire.WINDOW + 1)
                        numbers = set(wire.search(connection, start, end))
                        pages += 1
                        for number in range(end, start - 1, -1):
                            if number in numbers and not cache.has(number):
                                attempted += 1
                                try:
                                    body, note = wire.fetch(connection, number)
                                    cache.put(number, body)
                                    fetched += 1
                                    if note and len(details) < 100:
                                        details.append(f'UID {number}: ' + note)
                                except wire.Refused:
                                    pending = state['retry']
                                    count = pending['count'] + 1 if pending and pending['uid'] == number else 1
                                    if count <= 3:
                                        state['retry'] = {'uid': number, 'count': count}
                                        retry = number
                                        break
                                    skipped += 1
                                    if len(details) < 100:
                                        details.append(f'UID {number}: server refused FETCH repeatedly; use read <uid> to retry')
                                except wire.WireError as error:
                                    skipped += 1
                                    if len(details) < 100:
                                        details.append(f'UID {number}: ' + str(error))
                                    tainted = isinstance(error, wire.Tainted)
                            state = progress.backfill_step(state, number)
                            if attempted >= call['limit'] or tainted:
                                break
                        # Bodies commit first. A failed progress commit may repeat SEARCH,
                        # but the durable cache prevents another body FETCH after restart.
                        kv.set('imap.progress', state, kind='json', mode='set')
                    # Even an empty mailbox persists the migration and baseline.
                    kv.set('imap.progress', state, kind='json', mode='set')
                    result = f"fetched {fetched}; history {state['history']}; UIDNEXT {next_uid}; more {'yes' if state['history'] else 'no'}; skipped {skipped}"
                    if details:
                        result += '\n' + '\n'.join(details)
                    if retry is not None:
                        result += f"\nretry UID {retry} on the next sync (server refused FETCH, retry {state['retry']['count']}/3)"
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
                        row = 'UID ' + str(number) + ' (cached)\n' + summary(raw, secrets)
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
        except PreviousGeneration:
            error_class = 'waiting'
            return 'error: waiting for the previous IMAP generation; try again'
        except (MailError, PublicationError, SubscriptionError, CacheError, wire.WireError) as error:
            error_class = 'cache' if isinstance(error, CacheError) else 'protocol' if isinstance(error, wire.WireError) else 'invalid'
            return 'error: ' + str(error)
        except SessionError as error:
            error_class = 'connection'
            return 'error: IMAP ' + error.code + '; nothing was stored'
        except imaplib.IMAP4.error:
            error_class = 'server_refused'
            return 'error: IMAP server refused the operation'
        except Refusal:
            error_class = 'bus_refused'
            return ('error: the bus could not store this; check sub list before repeating'
                    if method.startswith('sub.') else 'error: publication refused by the bus')
        except UncertainOutcome:
            error_class = 'unconfirmed'
            return ('error: outcome unknown; check sub list before repeating'
                    if method.startswith('sub.') else 'error: publication outcome unknown; inspect the destination before repeating')
        except (BusTimeout, TransportError):
            error_class = 'bus_unavailable'
            return ('error: the bus could not store this; check sub list before repeating'
                    if method.startswith('sub.') else 'error: bus unavailable; publication not sent')
        except BusError:
            error_class = 'unconfirmed'
            return ('error: the bus could not store this; check sub list before repeating'
                    if method.startswith('sub.') else 'error: publication outcome unknown; inspect the destination before repeating')
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
    agent = None
    try:
        cfg = json.loads(os.environ.get('AGENT_BUS_CONFIG', '{}'))
        with Bus(timeout=5) as bus:
            agent = Agent(bus, cfg, os.environ, os.environ.get('AGENT_BUS_WORK', '.'))
            # A raw main-thread reader keeps SIGALRM effective during DNS/TLS/flock.
            while True:
                message = bus.consume(wait=agent.poll())
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
    finally:
        if agent is not None:
            agent.close()


if __name__ == '__main__':
    sys.exit(main())
