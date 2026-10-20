#!/usr/bin/env python3
"""Bounded bus diagnostics. No-argument startup serves with debug's own credential."""
from datetime import datetime, timezone
import json
import math
import os
from pathlib import Path
import resource
import signal
import sys
import threading
import time

HERE = Path(__file__).resolve().parent
SUPPORT = HERE / 'support' if (HERE / 'support').is_dir() else HERE.parents[1]
sys.path.insert(0, str(SUPPORT / 'clients/python'))
sys.path.insert(0, str(SUPPORT / 'agents/shared'))
from agent_bus import Bus
from agent_bus.errors import BusError
from metadata import answer as metadata_answer, ASKS, VERSION

MAX_BODY = 16384
MAX_SLEEP = 30
MAX_BIG = 64
METHODS = {'hello', 'env', 'help', 'echo', 'roles', 'ping', 'sleep', 'fail', 'crash',
           'sandbox', 'selftest', 'doctor', 'big', 'unicode'}
_SELFTEST = threading.Lock()


def dumps(value):
    return json.dumps(value, ensure_ascii=False, separators=(',', ':'))


def bounded_number(value, low, high, name):
    try: number = float(value)
    except (ValueError, TypeError): raise ValueError(name + ' must be a number')
    if not math.isfinite(number) or not low <= number <= high:
        raise ValueError(f'{name} must be {low}..{high}')
    return number


class Budget:
    def __init__(self, envelope=None, maximum=30):
        self.end = time.monotonic() + maximum
        deadline = (envelope or {}).get('deadline')
        if deadline and not deadline.startswith('0001-'):
            parsed = datetime.fromisoformat(deadline.replace('Z', '+00:00'))
            self.end = min(self.end, time.monotonic() + (parsed-datetime.now(timezone.utc)).total_seconds())

    def left(self, maximum=30):
        remaining = min(maximum, self.end-time.monotonic())
        if remaining <= 0: raise TimeoutError('message deadline reached')
        return remaining


def environment(envelope):
    # Never display the credential, even though its name uses the bus prefix.
    values = dict(os.environ)
    if envelope:
        for key in list(values):
            if key.startswith('AB_ROLE_'): del values[key]
        for key in ('from', 'to', 'topic', 'tag', 'ask', 'deadline', 'original_to'):
            values['AGENT_BUS_' + key.upper()] = str(envelope.get(key) or '')
        for role in envelope.get('roles', ()):
            values['AB_ROLE_' + role.upper()] = '1'
    return '\n'.join(f'{key}={value if key != "AGENT_BUS_TOKEN" and key.startswith(("AGENT_BUS_", "AB_ROLE_")) else "***"}'
                     for key, value in sorted(values.items()))


def sandbox():
    # Observations only, no privilege or network probe.
    status = {}
    try:
        for line in Path('/proc/self/status').read_text().splitlines():
            key, _, value = line.partition(':')
            if key in ('NoNewPrivs', 'Seccomp', 'CapEff', 'NSpid'): status[key] = value.strip()
    except OSError: status['process'] = 'unavailable: /proc/self/status'
    return {'uid': os.getuid(), 'gid': os.getgid(), 'process': status,
            'limits': {name: resource.getrlimit(getattr(resource, name)) for name in ('RLIMIT_AS','RLIMIT_NOFILE','RLIMIT_FSIZE')},
            'writable': {name: os.access(name, os.W_OK) for name in ('.','/tmp','/')},
            'network': 'unavailable: no external network probe; doctor uses the bus'}


def selftest(bus, budget):
    if not _SELFTEST.acquire(blocking=False): return 'error: selftest already running'
    results = []
    def check(name, fn):
        try:
            if not fn(): raise ValueError('unexpected answer')
            results.append('✅ ' + name)
        except Exception as error:
            results.append('❌ ' + name + ': ' + type(error).__name__)
    def call(body):
        deadline = datetime.fromtimestamp(time.time()+budget.left(2),timezone.utc).isoformat()
        return answer(body, {'body':body,'deadline':deadline})
    key = 'debug-selftest'
    def kv():
        bus._request('POST','/kv/set',{'record':bus.name,'name':key,'kind':'string','value':'roundtrip','how':'set'},timeout=budget.left(2))
        try:
            return bus._request('GET','/kv',query={'record':bus.name,'name':key,'kind':'string'},timeout=budget.left(2))['value'] == 'roundtrip'
        finally:
            bus._request('POST','/kv/delete',{'record':bus.name,'name':key,'kind':'string'},timeout=budget.left(2))
    def lock():
        bus._request('POST','/try-lock',{'record':bus.name,'name':key,'ttl':'2s'},timeout=budget.left(2))
        bus._request('POST','/release',{'record':bus.name,'name':key},timeout=budget.left(2))
        return True
    try:
        check('ping', lambda: 'answered_at' in json.loads(call('ping')))
        check('echo', lambda: json.loads(call('echo'))['body'] == 'echo')
        check('slow answer', lambda: json.loads(call('sleep 0.05'))['slept'] >= .05)
        check('error answer', lambda: call('fail 7') == 'error: debug failure 7')
        check('KV round trip', kv)
        check('lock acquire/release', lock)
        return 'Local handler checks; KV and lock use the bus:\n' + '\n'.join(results)
    finally: _SELFTEST.release()


def tls_check(bus, budget, target, results):
    try:
        reply = bus.call(target, '{"method":"tls"}', timeout=budget.left(2)).message.body
        rows = json.loads(reply)['tls']
        if any(row['status'] for row in rows):
            raise ValueError('certificate or fingerprint helper failed')
        facts = rows[0]['data']
        days = facts['days_left']
        if not isinstance(days, (int, float)) or not math.isfinite(days):
            raise ValueError('invalid days left')
        results.append(('✅' if days >= 30 else '❌') + f' TLS certificate: {days:.1f} days left; expires ' + facts['expires_at'] + ' (' + target + ')')
        results.append(('✅' if facts['pin'] == facts['token_pin'] else '❌') + ' TLS pin matches agent-bus-token --fingerprint (' + target + ')')
    except (BusError, TimeoutError, ValueError, KeyError, TypeError, IndexError):
        results.append('unavailable: TLS certificate facts from ' + target)


def normalize_tags(tags):
    # Keep identical to internal/protocol/tags.go NormalizeTags: discovery
    # compares authored labels with the daemon's stored canonical labels.
    normalized = set()
    for raw in tags:
        for word in raw.lower().split(','):
            word = word.strip()
            if word == 'no-tags':
                continue
            word = ''.join(' ' if char in '\t\r\n\f\v' else char
                           for char in word)
            word = ''.join(char for char in word if 'a' <= char <= 'z' or char in '_ ')
            word = ' '.join(word.split())
            if word:
                normalized.add(word)
    return sorted(normalized)


def doctor(bus, budget):
    host = bus.name.rsplit('@', 1)[-1]
    results = ['debug version: ' + VERSION]
    versions = {'debug': VERSION}
    remaining = 32
    # Visible tagged instances in this realm; fixed queries as debug itself.
    for capability, body in [('server-info','disks'),('systemd','failed'),('git','list'),('logwatch','grep 409 10'),('telegram','whoami')]:
        try:
            tag = normalize_tags([capability])[0]
            records = bus._request('GET', '/ls', query={'kind':'agent', 'tag':tag}, timeout=budget.left(2))
            targets = sorted({r['name'] for r in records if r.get('kind') == 'agent'
                              and tag in normalize_tags(r.get('tags', [])) and r.get('name', '').rsplit('@',1)[-1] == host})
        except (BusError, TimeoutError, ValueError, KeyError, TypeError):
            results.append('unavailable: discovery for ' + capability)
            continue
        if not targets:
            results.append(capability + ': none installed')
        for target in targets:
            if remaining == 0:
                results.append('unavailable: doctor target limit (32)')
                return '\n'.join(results)
            remaining -= 1
            try:
                reply = bus.call(target, body, timeout=budget.left(2)).message.body
                results.append(target + ': ' + reply[:4096])
                try:
                    description = bus.call(target, '', ask='help-json', timeout=budget.left(2)).message.body
                    version = json.loads(description).get('version')
                    if isinstance(version, str): versions[target] = version[:80]
                    else: results.append('unavailable: version for ' + target)
                except (BusError, ValueError, TimeoutError): results.append('unavailable: version for ' + target)
                if capability == 'server-info':
                    tls_check(bus, budget, target, results)
            except BusError as error:
                kind = type(error).__name__
                if getattr(error, 'status', None) in (401,403,404): results.append('not allowed: ' + target)
                else: results.append('unavailable: ' + target + ' (' + kind + ')')
            except TimeoutError: results.append('unavailable: ' + target + ' (deadline)')
    results.append(('✅' if len(set(versions.values())) == 1 else '❌') + ' reported versions: ' + dumps(versions))
    return '\n'.join(results)


def answer(body, envelope=None, bus=None):
    try:
        if len(body.encode()) > MAX_BODY: raise ValueError('body exceeds 16384 bytes')
        words = body.split()
        command, rest = (words[0], words[1:]) if words else ('help', [])
        if command not in METHODS: raise ValueError('unknown method; ask help')
        budget = Budget(envelope)
        budget.left()
        if command == 'hello': return 'hello, ' + (' '.join(rest) if rest else 'world')
        if command == 'help': return metadata_answer(HERE, roles=(envelope or {}).get('roles', ()))
        if command == 'env':
            if rest: raise ValueError('env takes no arguments')
            return environment(envelope)
        if command == 'echo':
            if rest: raise ValueError('echo takes no arguments')
            return dumps({key: (envelope or {}).get(key) for key in ('message_id','from','to','body','topic','tag','ask','deadline','roles','original_to')})
        if command == 'roles':
            if rest: raise ValueError('roles takes no arguments')
            return dumps({'roles': list((envelope or {}).get('roles', ())), 'source': 'delivered by the daemon for this addressed record; never taken from the body'})
        if command == 'ping':
            if rest: raise ValueError('ping takes no arguments')
            return dumps({'received_at': (envelope or {}).get('_received_at', time.time()), 'answered_at': time.time(), 'version': VERSION})
        if command == 'sleep':
            if len(rest) != 1: raise ValueError('sleep takes seconds')
            seconds = bounded_number(rest[0], 0, MAX_SLEEP, 'seconds')
            left = budget.left()
            time.sleep(min(seconds, left))
            budget.left()
            return dumps({'slept':seconds})
        if command == 'fail':
            if len(rest) > 1: raise ValueError('fail takes one optional code')
            code = bounded_number(rest[0] if rest else '1',1,255,'code')
            if not code.is_integer(): raise ValueError('code must be an integer')
            print('debug requested failure ' + str(int(code)), file=sys.stderr)
            return 'error: debug failure ' + str(int(code))
        if command == 'crash':
            if rest: raise ValueError('crash takes no arguments')
            resource.setrlimit(resource.RLIMIT_CORE, (0,0))
            os.kill(os.getpid(), signal.SIGSEGV)
            raise RuntimeError('SIGSEGV did not terminate debug')
        if command == 'sandbox':
            if rest: raise ValueError('sandbox takes no arguments')
            return dumps(sandbox())
        if command == 'big':
            if len(rest) != 1: raise ValueError('big takes KB')
            size = bounded_number(rest[0],1,MAX_BIG,'KB')
            if not size.is_integer(): raise ValueError('KB must be an integer')
            return 'x' * (int(size)*1024)
        if command == 'unicode':
            if rest: raise ValueError('unicode takes no arguments')
            return 'Hello, мир 👋\nsecond line\tTAB\x01'
        if rest: raise ValueError(command + ' takes no arguments')
        if bus is None: return 'unavailable: ' + command + ' needs debug serving on the bus'
        if command == 'selftest': return selftest(bus, budget)
        if command == 'doctor': return doctor(bus, budget)
    except (ValueError, TimeoutError) as error: return 'error: ' + str(error)


def handle(bus, message):
    if message.ask in ASKS: return metadata_answer(HERE, message.ask, roles=message.roles)
    if message.ask == 'mcp': return '{"tools":[]}'
    envelope = dict(message.envelope)
    envelope['_received_at'] = time.time()
    return answer(message.body, envelope, bus)


def main():
    if len(sys.argv) > 1:
        # Explicit args retain fixture use; the installed package starts without args.
        envelope = {key: os.environ.get('AGENT_BUS_' + key.upper()) for key in ('from','to','topic','tag','deadline','ask')}
        envelope['roles'] = [key[8:].lower() for key, value in os.environ.items() if key.startswith('AB_ROLE_') and value == '1']
        if envelope['ask'] in ASKS: print(metadata_answer(HERE, envelope['ask']))
        elif envelope['ask'] == 'mcp': print('{"tools":[]}')
        else: print(answer(' '.join(sys.argv[1:]), envelope))
        return
    stop = threading.Event()
    signal.signal(signal.SIGTERM, lambda *_: stop.set())
    signal.signal(signal.SIGINT, lambda *_: stop.set())
    with Bus(timeout=2) as bus:
        description = json.loads((HERE / 'agent.json').read_text())['description']
        bus.record(bus.name).register(create_only=False, kind='agent', descr=description)
        bus.serve(lambda message: handle(bus, message), workers=4, stop=stop, drain_timeout=3)


if __name__ == '__main__': main()
