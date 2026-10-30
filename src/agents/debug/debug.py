#!/usr/bin/env python3
"""Bounded bus diagnostics. No-argument startup serves with debug's own credential."""
from datetime import datetime, timezone
from collections import Counter
import json
import math
import os
from pathlib import Path
import resource
import re
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
from metadata import answer as metadata_answer, ASKS, VERSION, dispatch, normalize_tags

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


# Only names the runner or the delivery sets for every agent show a value; the bus prefix alone
# proves nothing, since AGENT_BUS_CONFIG and a record's secret keys carry it too.
SHOWN = {'AGENT_BUS_' + key for key in ('FROM', 'TO', 'TOPIC', 'TAG', 'ASK', 'DEADLINE', 'ORIGINAL_TO',
                                       'NAME', 'ADDR', 'WORK', 'TLS_FINGERPRINT', 'MESSAGE_ID')}


def environment(envelope):
    values = dict(os.environ)
    if envelope:
        for key in list(values):
            if key.startswith('AB_ROLE_'): del values[key]
        for key in ('from', 'to', 'topic', 'tag', 'ask', 'deadline', 'original_to'):
            values['AGENT_BUS_' + key.upper()] = str(envelope.get(key) or '')
        for role in envelope.get('roles', ()):
            values['AB_ROLE_' + role.upper()] = '1'
    return '\n'.join(f'{key}={value if key in SHOWN or key.startswith("AB_ROLE_") else "***"}'
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
        return 'Local handler checks; KV and lock use the bus:\n' + '\n'.join(results) + f'\nChecks: {len(results)}'
    finally: _SELFTEST.release()


def tls_check(bus, budget, target, results):
    try:
        reply = bus.call(target, '{"method":"tls"}', timeout=budget.left(2)).message.body
        rows = response_json(reply)['tls']
        if any(row['status'] for row in rows):
            raise ValueError('certificate or fingerprint helper failed')
        facts = rows[0]['data']
        days = facts['days_left']
        if not isinstance(days, (int, float)) or not math.isfinite(days):
            raise ValueError('invalid days left')
        results.append(('✅' if days >= 30 else '❌') + f' TLS certificate: {days:.1f} days left; expires ' + facts['expires_at'] + ' (' + target + ')')
        results.append(('✅' if facts['pin'] == facts['token_pin'] else '❌') + ' TLS pin matches agent-bus token --fingerprint (' + target + ')')
    except (BusError, TimeoutError, ValueError, KeyError, TypeError, IndexError):
        results.append('unavailable: TLS certificate facts from ' + target)



def response_json(reply):
    if len(reply.encode()) > 256 * 1024:
        raise ValueError('response exceeds 256 KiB')
    try:
        return json.loads(reply)
    except (ValueError, RecursionError) as error:
        raise ValueError('invalid JSON response') from error


def one_line(value, limit=160):
    return ' '.join(''.join(c for c in str(value) if c.isprintable() or c.isspace()).split())[:limit]


def disk_summary(reply):
    rows = response_json(reply)['disks']
    df = next(row for row in rows if row.get('command') == ['df', '-h'])
    if df.get('status') != 0:
        raise ValueError('df failed')
    candidates = []
    lines = df['stdout'].splitlines()
    if len(lines) > 1001:
        raise ValueError('too many filesystems')
    for line in lines:
        fields = line.split(None, 5)
        if len(fields) != 6 or not re.fullmatch(r'[0-9]+%', fields[4]):
            continue
        used = int(fields[4][:-1])
        free = fields[3]
        size = re.fullmatch(r'([0-9.]+)([KMGTPE])', free)
        if size:
            free = size[1] + ' ' + size[2] + 'iB'
        elif free.isdigit():
            free += ' B'
        else:
            raise ValueError('invalid free space')
        candidates.append((used, fields[5], free))
    if not candidates:
        raise ValueError('no disk usage facts')
    used, mount, free = max(candidates, key=lambda row: row[0])
    mark = '❌' if used >= 95 else '⚠' if used >= 85 else '✅'
    return f'{mark} disk {one_line(mount)} {used}% used ({free} free)'


def failed_summary(reply):
    result = response_json(reply)
    units = result['units']
    if result.get('status') != 0 or not isinstance(units, list) or len(units) > 1000:
        raise ValueError('failed units unavailable or over limit')
    if not units:
        return '✅ no failed units'
    groups = Counter()
    for row in units:
        unit = row['unit']
        if not isinstance(unit, str) or not unit:
            raise ValueError('invalid unit')
        groups[one_line(unit.split('@', 1)[0], 100)] += 1
    names = sorted(groups.items(), key=lambda item: (-item[1], item[0]))
    text = ', '.join(name + (' ×' + str(count) if count > 1 else '') for name, count in names[:12])
    if len(names) > 12:
        text += f', +{len(names)-12} other templates'
    return f'⚠ {len(units)} failed unit' + ('s' if len(units) != 1 else '') + ': ' + text + ' (grouped by template)'


def check_summary(capability, reply):
    if capability == 'server-info':
        return disk_summary(reply)
    if capability == 'systemd':
        return failed_summary(reply)
    if reply.lstrip().startswith('error:'):
        return '❌ ' + capability + ' check returned an error'
    if capability == 'logwatch':
        lines = reply.strip().splitlines()
        if not lines or reply.strip() == '(no lines)':
            return '✅ no log matches for 409'
        return '⚠ ' + str(len(lines)) + ' log matches for 409'
    return '✅ ' + capability + ' responds'


def doctor(bus, budget, mode=None):
    host = bus.name.rsplit('@', 1)[-1]
    results = []
    versions = {'debug': VERSION}
    remaining = 32
    # Visible templates in this realm; only systemd has a shared fan-out capability tag.
    records = None
    checks = [('server-info','disks'),('systemd','failed'),('git','list'),('logwatch','grep 409 10'),('telegram','whoami')]
    for capability, body in checks:
        if mode == 'disks' and capability != 'server-info':
            continue
        try:
            if records is None:
                records = bus._request('GET', '/ls', query={'kind':'agent'}, timeout=budget.left(2))
            targets = sorted({r['name'] for r in records if r.get('kind') == 'agent'
                              and r.get('name', '').rsplit('@',1)[-1] == host
                              and (('systemd' in normalize_tags(r.get('tags', []))) if capability == 'systemd'
                                   else (r['name'].split('@',1)[0] == '#' + capability
                                         or r['name'].split('@',1)[0].startswith('#' + capability + '/')))})
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
                request = dumps({'method':body}) if mode is None and capability in ('server-info','systemd') else body
                reply = bus.call(target, request, timeout=budget.left(2)).message.body
                if mode in ('full', 'disks'):
                    results.append(target + ': ' + reply[:4096] + ('\n[output truncated]' if len(reply) > 4096 else ''))
                else:
                    try:
                        verdict = check_summary(capability, reply)
                        results.append(verdict + ' (' + target + ')')
                    except (ValueError, KeyError, TypeError, IndexError, StopIteration, AttributeError):
                        results.append('unavailable: ' + capability + ' facts from ' + target)
                if mode == 'disks':
                    continue
                try:
                    description = bus.call(target, '', ask='help-json', timeout=budget.left(2)).message.body
                    version = response_json(description).get('version')
                    if isinstance(version, str): versions[target] = one_line(version, 80)
                    else: results.append('unavailable: version for ' + target)
                except (BusError, ValueError, TimeoutError, AttributeError): results.append('unavailable: version for ' + target)
                if capability == 'server-info':
                    tls_check(bus, budget, target, results)
            except BusError as error:
                kind = type(error).__name__
                if getattr(error, 'status', None) in (401,403,404): results.append('not allowed: ' + target)
                else: results.append('unavailable: ' + target + ' (' + kind + ')')
            except TimeoutError: results.append('unavailable: ' + target + ' (deadline)')
    if mode != 'disks':
        if len(set(versions.values())) == 1:
            results.append('✅ versions agree (' + VERSION + ')')
        else:
            results.append('❌ versions disagree: ' + '; '.join(name + '=' + value for name, value in versions.items()))
    return '\n'.join(results)


def answer(body, envelope=None, bus=None):
    try:
        if len(body.encode()) > MAX_BODY: raise ValueError('body exceeds 16384 bytes')
        words = body.split()
        command, rest = (words[0], words[1:]) if words else ('help', [])
        if command not in METHODS: raise ValueError('unknown method; ask help')
        if command == 'doctor':
            delivered = envelope or {}
            addressed = delivered.get('original_to') or delivered.get('to')
            if (bus is None or addressed != bus.name or
                    not set(delivered.get('roles', ())) & {'owner','maintainer'}):
                return 'error: ' + command + ' requires a delivered Owner or Maintainer role addressed directly to debug'
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
            # real SIGSEGV creates host core dumps
            # os.kill(os.getpid(), signal.SIGSEGV)
            os.kill(os.getpid(), signal.SIGKILL)
            raise RuntimeError('SIGKILL did not terminate debug')
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
        if command == 'doctor':
            if len(rest) > 1 or rest and rest[0] not in ('full', 'disks'):
                raise ValueError('doctor takes full or disks, or no argument')
            if bus is None: return 'unavailable: doctor needs debug serving on the bus'
            return doctor(bus, budget, rest[0] if rest else None)
        if rest: raise ValueError(command + ' takes no arguments')
        if bus is None: return 'unavailable: ' + command + ' needs debug serving on the bus'
        if command == 'selftest': return selftest(bus, budget)
    except (ValueError, TimeoutError) as error: return 'error: ' + str(error)


def handle(bus, message):
    result = dispatch(HERE, message.ask, roles=message.roles)
    if result is not None: return result
    envelope = dict(message.envelope)
    envelope['_received_at'] = time.time()
    return answer(message.body, envelope, bus)


def main():
    if len(sys.argv) > 1:
        # Explicit args retain fixture use; the installed package starts without args.
        envelope = {key: os.environ.get('AGENT_BUS_' + key.upper()) for key in ('from','to','topic','tag','deadline','ask')}
        envelope['roles'] = [key[8:].lower() for key, value in os.environ.items() if key.startswith('AB_ROLE_') and value == '1']
        result = dispatch(HERE, envelope['ask'], roles=envelope['roles'])
        print(result if result is not None else answer(' '.join(sys.argv[1:]), envelope))
        return
    stop = threading.Event()
    signal.signal(signal.SIGTERM, lambda *_: stop.set())
    signal.signal(signal.SIGINT, lambda *_: stop.set())
    with Bus(timeout=2) as bus:
        description = json.loads((HERE / 'agent.json').read_text())['description']
        bus.record(bus.name).register(create_only=False, kind='agent', descr=description)
        bus.serve(lambda message: handle(bus, message), workers=4, stop=stop, drain_timeout=3)


if __name__ == '__main__': main()
