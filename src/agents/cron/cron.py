#!/usr/bin/env python3
"""Scheduled sends: recurring subscriptions and one-shot `later` calls kept in the agent's KV.

Every entry is authorised by management of its destination, when made and again
at each firing. One process fires at a time (the record lock `scheduler`), and
each firing is claimed in KV before its send, so it is never sent twice (RD138).
"""
from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import re
import secrets
import signal
import sys
import time

HERE = Path(__file__).resolve().parent
SUPPORT = HERE / 'support' if (HERE / 'support').is_dir() else HERE.parents[1]
sys.path.insert(0, str(SUPPORT / 'clients/python'))
sys.path.insert(0, str(SUPPORT / 'agents/shared'))
sys.path.insert(0, str(HERE))
from agent_bus import Bus
from agent_bus.errors import BusError, Refusal, UncertainOutcome
from metadata import answer as metadata_answer, dispatch
import cronexpr

BUS_NAME = re.compile(r'(?:#[a-z0-9][a-z0-9._/-]*@[a-z0-9][a-z0-9.-]*|[a-z0-9][a-z0-9._-]*(?:@[a-z0-9][a-z0-9.-]*)?)\Z')
ID = re.compile(r'[0-9a-f]{12}\Z')
IN = re.compile(r'\+([1-9][0-9]{0,6})([smhd])\Z')
UNIT = {'s': 1, 'm': 60, 'h': 3600, 'd': 86400}
MAX_BODY = 16 * 1024
MAX_SUBS, SUBS_EACH = 100, 20
MAX_LATER, LATER_EACH = 1000, 100
OUTCOMES = 100
HORIZON = 366 * 86400
SKEW = 30
DAY = 86400
LOCK_TTL = 30
BACKOFF_FIRST, BACKOFF_MAX = 10, 300
FIELDS = {'sub.add': {'cron', 'target', 'body'}, 'sub.list': set(), 'sub.delete': {'id'},
          'later': {'in', 'at', 'target', 'body'}, 'later.list': set(), 'later.cancel': {'id'}, 'help': set()}
TEXT = (
    (re.compile(r'sub +add +(\S+) +(\S+) +(\S+) +(\S+) +(\S+) +(\S+) (.*)\Z', re.S),
     lambda m: {'method': 'sub.add', 'cron': ' '.join(m.groups()[:5]), 'target': m[6], 'body': m[7]}),
    (re.compile(r'sub +list *\Z'), lambda m: {'method': 'sub.list'}),
    (re.compile(r'sub +delete +(\S+) *\Z'), lambda m: {'method': 'sub.delete', 'id': m[1]}),
    (re.compile(r'later +list *\Z'), lambda m: {'method': 'later.list'}),
    (re.compile(r'later +cancel +(\S+) *\Z'), lambda m: {'method': 'later.cancel', 'id': m[1]}),
    (re.compile(r'later +(\S+) +(\S+) (.*)\Z', re.S),
     lambda m: {'method': 'later', 'in' if m[1].startswith('+') else 'at': m[1], 'target': m[2], 'body': m[3]}),
    (re.compile(r'help *\Z'), lambda m: {'method': 'help'}),
)


class CallError(ValueError):
    pass


def parse(body):
    """A text call (the message is the rest of the line, verbatim) or its JSON form."""
    if body.lstrip().startswith('{'):
        try:
            call = json.loads(body)
        except ValueError:
            raise CallError('invalid JSON call') from None
    else:
        for pattern, build in TEXT:
            m = pattern.match(body.lstrip(' '))
            if m:
                call = build(m)
                break
        else:
            raise CallError('use sub add|list|delete, later, later list|cancel or help; a message needs all its arguments')
    if not isinstance(call, dict) or call.get('method') not in FIELDS:
        raise CallError('unknown method; ask help')
    method = call['method']
    if set(call) - FIELDS[method] - {'method'}:
        raise CallError('unknown method argument')
    if any(not isinstance(v, str) for k, v in call.items()):
        raise CallError('arguments are text')
    need = {'sub.add': ('cron', 'target', 'body'), 'sub.delete': ('id',), 'later': ('target', 'body'),
            'later.cancel': ('id',)}.get(method, ())
    for key in need:
        if key not in call or key != 'body' and not call[key]:
            raise CallError(key + ' is required')
    if method == 'later' and ('in' in call) == ('at' in call):
        raise CallError('later takes one of in (+90s, +15m, +2h, +3d) or at (RFC 3339 with an offset)')
    if 'body' in call and len(call['body'].encode()) > MAX_BODY:
        raise CallError(f'a message is at most {MAX_BODY} bytes')
    if 'target' in call and not BUS_NAME.fullmatch(call['target']):
        raise CallError('target must be a bus name: a queue or an agent')
    if 'id' in call and not ID.fullmatch(call['id']):
        raise CallError('unknown id')
    return call


def due(call, now):
    """The epoch second a later call is due, refusing the past and anything past the horizon."""
    if 'in' in call:
        m = IN.fullmatch(call['in'])
        if not m:
            raise CallError('in is +<n>s, +<n>m, +<n>h or +<n>d')
        at = now + int(m[1]) * UNIT[m[2]]
    else:
        try:
            instant = datetime.fromisoformat(call['at'])
        except ValueError:
            raise CallError('at is an RFC 3339 instant with an offset') from None
        if instant.tzinfo is None:
            raise CallError('at needs an explicit offset, such as Z or -04:00')
        at = instant.timestamp()
        if at < now - SKEW:
            raise CallError('at is in the past')
    if at > now + HORIZON:
        raise CallError('at most 366 days ahead')
    return max(at, now)


def short(name):
    return name.split('@', 1)[0]


def config(value):
    """Configuration: `tz`, an IANA zone available in the sandbox; UTC by default."""
    from zoneinfo import ZoneInfo, ZoneInfoNotFoundError
    if not isinstance(value, dict) or set(value) - {'tz'}:
        raise CallError('Configuration takes only tz')
    name = value.get('tz', 'UTC')
    if not isinstance(name, str) or not name:
        raise CallError('tz is an IANA time zone name')
    if name == 'UTC':
        return {'tz': timezone.utc}
    try:
        return {'tz': ZoneInfo(name)}
    except (ZoneInfoNotFoundError, ValueError):
        raise CallError('tz ' + name + ' is not in this host\'s time zone data') from None


class Agent:
    def __init__(self, bus, cfg, clock=time.time, log=lambda text: print('cron: ' + text, file=sys.stderr)):
        self.bus, self.clock, self.log = bus, clock, log
        self.subs, self.later, self.claims = {}, {}, {}
        self.state = {}  # id -> outcome, skipped and missed counts (memory; pauses are stored)
        self.retries = {}  # claim name -> (entry id, slot, attempt, next try, until)
        self.lease = 0
        self.loaded = False
        self.error = None
        try:
            self.cfg = config(cfg)
        except CallError as e:
            self.cfg, self.error = None, 'Configuration: ' + str(e)

    # Storage ----------------------------------------------------------
    def load(self):
        """Read every entry; one that does not parse is shown as invalid, never fatal."""
        if self.error:
            return
        names = self.bus.kv.list()
        for entry in names.get('json', []):
            name = entry['name']
            kind, _, key = name.partition('.')
            if kind not in ('sub', 'later') or not ID.fullmatch(key):
                continue
            try:
                value = self.bus.kv.get(name, kind='json')
                if kind == 'sub':
                    value['spec'] = cronexpr.parse(value['cron'])
                    assert BUS_NAME.fullmatch(value['target']) and isinstance(value['body'], str) and value['by']
                    self.subs[key] = value
                else:
                    assert BUS_NAME.fullmatch(value['target']) and isinstance(value['body'], str) and value['by']
                    float(value['at'])
                    self.later[key] = value
            except (BusError, KeyError, TypeError, ValueError, AssertionError):
                self.log(f'stored {kind} {key} is invalid; skipped')
                self.state.setdefault(key, {})['invalid'] = True
        for entry in names.get('string', []):
            name = entry['name']
            if name.startswith('fired.'):
                self.claims[name] = self.bus.kv.get(name)
        self.sweep()

    def sweep(self):
        """At start: an in-flight claim means an unconfirmed send; orphans and old claims go."""
        now = self.clock()
        newest = {}
        for name, value in sorted(self.claims.items()):
            _, key, slot = name.split('.', 2)
            entry = self.subs.get(key) or self.later.get(key)
            if entry is None and key not in self.state:
                self.drop_claim(name)
                continue
            if value == 'sending':
                self.unconfirmed(key, name)
            elif value == 'unconfirmed' and key in self.later:
                self.later[key]['paused'] = 'unconfirmed'
            elif value in ('sent', 'refused') and key in self.later:
                self.finish(key, value)  # the removal after its outcome did not happen
            elif value.startswith('retrying '):
                attempt, until = value.split(' ')[1:3]
                self.retries[name] = (key, slot, int(attempt), now, float(until))
            elif key in self.subs:
                newest.setdefault(key, []).append(name)
        for names in newest.values():
            for name in names[:-1]:
                self.drop_claim(name)

    def drop_claim(self, name):
        """Only settled claims are removed: `sending` and `unconfirmed` are the evidence."""
        if self.claims.get(name) in ('sending', 'unconfirmed'):
            return
        try:
            self.bus.kv.delete(name)
            self.claims.pop(name, None)
        except BusError:
            self.log('claim cleanup failed; kept')

    def claim(self, name, value, mode):
        self.bus.kv.set(name, value, mode=mode)
        self.claims[name] = value

    def pause(self, key, reason):
        sub = self.subs.get(key)
        if sub is not None and not sub.get('paused'):
            sub['paused'] = reason
            try:
                self.bus.kv.set('sub.' + key, self.stored(sub), kind='json')
            except BusError:
                self.log(f'subscription {key} pause not stored')
        self.outcome(key, reason.split(':')[0])

    def stored(self, entry):
        return {k: v for k, v in entry.items() if k != 'spec'}

    def outcome(self, key, status):
        """The latest outcome, in memory (a stored pause and the one-shot record survive a restart)."""
        self.state.setdefault(key, {})['outcome'] = status
        self.log(json.dumps({'id': key, 'status': status}))

    # Firing ------------------------------------------------------------
    def scheduler(self):
        """Hold the record lock `scheduler`; a second process stays idle without it."""
        now = time.monotonic()
        try:
            if self.lease > now:
                self.bus.locks.extend('scheduler', ttl=LOCK_TTL)
            else:
                self.bus.locks.try_acquire('scheduler', ttl=LOCK_TTL)
            self.lease = now + LOCK_TTL
            return True
        except BusError:
            self.lease = 0
            return False

    def manages(self, cache, who, target):
        if (who, target) not in cache:
            answer = self.bus._request('GET', '/manages', query={'who': who, 'name': target})
            cache[(who, target)] = answer.get('manages') is True
        return cache[(who, target)]

    def tick(self):
        """One pass: the current local minute's slots, due one-shots and due retries."""
        if self.error or not self.scheduler():
            return
        now = self.clock()
        minute = cronexpr.wall(datetime.fromtimestamp(now, timezone.utc), self.cfg['tz'])
        slot = cronexpr.slot_key(minute)
        cache = {}
        for key, sub in list(self.subs.items()):
            if not sub['spec'].matches(minute):
                continue
            if sub.get('paused'):
                self.count(key, 'skipped')
                continue
            name = f'fired.{key}.{slot}'
            if name in self.claims:
                continue
            ttl = self.period(sub, minute)
            self.fire(cache, key, sub, name, now, now + ttl)
        for key, entry in list(self.later.items()):
            name = f'fired.{key}.once'
            if entry['at'] <= now and name not in self.claims:
                self.fire(cache, key, entry, name, now, max(entry['at'], now) + DAY)
        for name, (key, slot, attempt, at, until) in list(self.retries.items()):
            if at <= now:
                entry = self.subs.get(key) or self.later.get(key)
                if entry is None or entry.get('paused'):
                    self.retries.pop(name)
                    continue
                self.send(cache, key, entry, name, now, until, attempt)

    def count(self, key, what):
        state = self.state.setdefault(key, {})
        state[what] = state.get(what, 0) + 1

    def period(self, sub, minute):
        later = cronexpr.next_slot(sub['spec'], minute, self.cfg['tz'])
        seconds = (later - minute).total_seconds() if later else DAY
        return max(60, min(DAY, seconds))

    def fire(self, cache, key, entry, name, now, until):
        try:
            if not self.manages(cache, entry['by'], entry['target']):
                return self.revoked(key)
            self.claim(name, 'sending', 'add')
        except Refusal as e:
            if e.status == 412:  # another process claimed this firing
                self.claims[name] = 'taken'
                return
            return self.count(key, 'missed')
        except BusError:
            return self.count(key, 'missed')
        self.send(cache, key, entry, name, now, until, 0, claimed=True)

    def revoked(self, key):
        if key in self.subs:
            self.pause(key, 'revoked: the creator no longer manages the target')
        else:
            self.finish(key, 'revoked')

    def send(self, cache, key, entry, name, now, until, attempt, claimed=False):
        try:
            if not claimed:
                if not self.manages(cache, entry['by'], entry['target']):
                    self.retries.pop(name, None)
                    self.drop_claim(name)
                    return self.revoked(key)
                self.claim(name, 'sending', 'set')
            self.bus.send(entry['target'], entry['body'], topic='cron' if key in self.subs else 'later',
                          ttl=max(1, until - now))
        except Refusal as e:
            if not 400 <= e.status < 500:
                return self.unsure(key, name)
            return self.refused(key, name, now, until, attempt)
        except UncertainOutcome:
            return self.unsure(key, name)
        except BusError:
            # Not sent (the client raises UncertainOutcome once a request has left).
            return self.refused(key, name, now, until, attempt)
        self.retries.pop(name, None)
        try:
            self.claim(name, 'sent', 'set')
        except BusError:
            self.log('sent but the success record failed; the claim stays sending')
        if key in self.subs:
            self.outcome(key, 'sent')
            for old in [n for n in self.claims if n.startswith(f'fired.{key}.') and n != name]:
                self.drop_claim(old)
        else:
            self.finish(key, 'sent')

    def refused(self, key, name, now, until, attempt):
        """A definite refusal is retried with backoff until the send TTL ends."""
        wait = min(BACKOFF_MAX, BACKOFF_FIRST * 2 ** attempt)
        if now + wait >= until:
            self.retries.pop(name, None)
            try:
                self.claim(name, 'refused', 'set')
            except BusError:
                pass
            return self.outcome(key, 'refused') if key in self.subs else self.finish(key, 'refused')
        try:
            self.claim(name, f'retrying {attempt + 1} {until}', 'set')
        except BusError:
            self.log('retry state not stored')
        self.retries[name] = (key, name.rsplit('.', 1)[1], attempt + 1, now + wait, until)
        self.outcome(key, 'retrying')

    def unsure(self, key, name):
        """RD138: an unconfirmed send is never repeated; its entry pauses, visibly."""
        self.retries.pop(name, None)
        self.unconfirmed(key, name)

    def unconfirmed(self, key, name):
        try:
            self.claim(name, 'unconfirmed', 'set')
        except BusError:
            self.log('unconfirmed state not stored; the claim stays sending')
        if key in self.subs:
            self.pause(key, 'unconfirmed: delivery could not be confirmed; delete and add again to restart')
        elif key in self.later:
            self.later[key]['paused'] = 'unconfirmed'
            self.outcome(key, 'unconfirmed')

    def finish(self, key, status):
        """A one-shot is done: its outcome is kept (last 100), the entry and claim removed."""
        entry = self.later.get(key)
        if entry is None:
            return
        self.record(key, entry, status)
        try:
            self.bus.kv.delete('later.' + key, kind='json')
        except BusError:
            self.log(f'one-shot {key} removal failed')
            return
        self.later.pop(key, None)
        self.claims[f'fired.{key}.once'] = 'done'
        self.drop_claim(f'fired.{key}.once')
        self.outcome(key, status)

    def record(self, key, entry, status):
        try:
            try:
                done = self.bus.kv.get('outcomes', kind='json')
            except Refusal:
                done = {}
            done[key] = {'status': status, 'at': self.clock(), 'target': entry['target'], 'by': entry['by']}
            for old in sorted(done, key=lambda k: done[k]['at'])[:-OUTCOMES]:
                del done[old]
            self.bus.kv.set('outcomes', done, kind='json')
        except BusError:
            self.log(f'one-shot {key} outcome not stored')

    # Calls -------------------------------------------------------------
    def handle(self, message):
        try:
            result = dispatch(HERE, message.ask)
            if result is not None:
                return result
            if self.error:
                return 'error: ' + self.error
            call = parse(message.body)
            if call['method'] == 'help':
                return metadata_answer(HERE, as_json=message.body.lstrip().startswith('{'))
            return json.dumps(getattr(self, {'later': 'later_add'}.get(call['method'], call['method'].replace('.', '_')))(call, message))
        except CallError as e:
            return 'error: ' + str(e)
        except cronexpr.CronError as e:
            return 'error: ' + str(e)
        except BusError:
            return 'error: the bus could not check or store this; nothing was changed'

    def caller(self, message):
        return getattr(message, 'from')

    def staff(self, message):
        return message.original_to in (None, '', self.bus.name) and bool({'owner', 'maintainer'} & set(message.roles))

    def authorized(self, message, target):
        """Management of the destination, for everybody: cron's Owner gets no bypass here."""
        answer = self.bus._request('GET', '/manages', query={'who': self.caller(message), 'name': target})
        if answer.get('manages') is not True:
            raise CallError('scheduling requires management of destination ' + target)

    def target(self, call):
        if short(call['target']) == short(self.bus.name):
            raise CallError('cron does not schedule messages to itself')

    def store(self, kind, value):
        for _ in range(3):
            key = secrets.token_hex(6)
            try:
                self.bus.kv.set(kind + '.' + key, value, kind='json', mode='add')
                return key
            except Refusal as e:
                if e.status != 412:
                    raise
        raise CallError('could not allocate an id; try again')

    def sub_add(self, call, message):
        spec = cronexpr.parse(call['cron'])
        self.target(call)
        who = self.caller(message)
        if len(self.subs) >= MAX_SUBS or sum(s['by'] == who for s in self.subs.values()) >= SUBS_EACH:
            raise CallError(f'at most {MAX_SUBS} subscriptions, {SUBS_EACH} per creator')
        self.authorized(message, call['target'])
        sub = {'cron': call['cron'], 'target': call['target'], 'body': call['body'], 'by': who, 'created': self.clock()}
        key = self.store('sub', sub)
        self.subs[key] = sub | {'spec': spec}
        return {'id': key, 'next': self.next(spec)}

    def next(self, spec):
        minute = cronexpr.wall(datetime.fromtimestamp(self.clock(), timezone.utc), self.cfg['tz'])
        slot = cronexpr.next_slot(spec, minute, self.cfg['tz'])
        return cronexpr.slot_key(slot) if slot else None

    def visible(self, message, cache, target):
        if (self.caller(message), target) not in cache:
            try:
                cache[(self.caller(message), target)] = self.bus._request(
                    'GET', '/manages', query={'who': self.caller(message), 'name': target}).get('manages') is True
            except BusError:
                cache[(self.caller(message), target)] = False
        return cache[(self.caller(message), target)]

    def listing(self, message, entries, row):
        """Entries whose target the caller manages, with bodies; cron's staff see the rest without."""
        cache, staff, out = {}, self.staff(message), []
        for key, entry in sorted(entries.items()):
            mine = self.visible(message, cache, entry['target'])
            if mine or staff:
                item = {'id': key, 'target': entry['target'], 'by': entry['by'], **row(key, entry),
                        **{k: v for k, v in self.state.get(key, {}).items()}}
                if entry.get('paused'):
                    item['paused'] = entry['paused']
                if mine:
                    item['body'] = entry['body']
                out.append(item)
        return out

    def sub_list(self, call, message):
        return {'subs': self.listing(message, self.subs, lambda k, s: {'cron': s['cron'], 'next': self.next(s['spec'])}),
                'tz': str(self.cfg['tz'])}

    def remove(self, message, entries, key, kind):
        entry = entries.get(key)
        if entry is None:
            raise CallError('unknown id')
        if not self.staff(message):
            self.authorized(message, entry['target'])
        self.bus.kv.delete(kind + '.' + key, kind='json')
        entries.pop(key)
        for name in [n for n in self.claims if n.startswith(f'fired.{key}.')]:
            self.retries.pop(name, None)
            self.claims[name] = 'done'
            self.drop_claim(name)
        return {'deleted': key}

    def sub_delete(self, call, message):
        return self.remove(message, self.subs, call['id'], 'sub')

    def later_add(self, call, message):
        now = self.clock()
        at = due(call, now)
        self.target(call)
        who = self.caller(message)
        if len(self.later) >= MAX_LATER or sum(e['by'] == who for e in self.later.values()) >= LATER_EACH:
            raise CallError(f'at most {MAX_LATER} pending one-shots, {LATER_EACH} per creator')
        self.authorized(message, call['target'])
        entry = {'at': at, 'target': call['target'], 'body': call['body'], 'by': who, 'created': now}
        key = self.store('later', entry)
        self.later[key] = entry
        return {'id': key, 'at': datetime.fromtimestamp(at, timezone.utc).isoformat()}

    def later_list(self, call, message):
        rows = self.listing(message, self.later, lambda k, e: {'at': datetime.fromtimestamp(e['at'], timezone.utc).isoformat()})
        try:
            done = self.bus.kv.get('outcomes', kind='json')
        except BusError:
            done = {}
        cache = {}
        finished = [{'id': k, **v} for k, v in sorted(done.items())
                    if self.staff(message) or self.visible(message, cache, v['target'])]
        return {'later': rows, 'finished': finished}

    def later_cancel(self, call, message):
        return self.remove(message, self.later, call['id'], 'later')

    def wait(self):
        """Seconds to the next minute boundary or due one-shot or retry, at most 10."""
        now = self.clock()
        due = [60 - now % 60 + 0.2] + [r[3] - now for r in self.retries.values()] + \
              [e['at'] - now for k, e in self.later.items() if f'fired.{k}.once' not in self.claims]
        return max(0.05, min(10, *due))


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
        cfg = json.loads(os.environ.get('AGENT_BUS_CONFIG') or '{}')
    except ValueError:
        cfg = None
    try:
        with Bus(timeout=5) as bus:
            agent = Agent(bus, cfg)
            while True:
                try:
                    if not agent.loaded:
                        agent.load()
                        agent.loaded = True
                    agent.tick()
                    message = bus.consume(wait=agent.wait())
                except Shutdown:
                    raise
                except BusError:
                    print('cron: bus unavailable; retrying', file=sys.stderr)
                    time.sleep(2)
                    continue
                if message is None or message.receipt or message.re:
                    continue
                answer = agent.handle(message)
                try:
                    message.reply(answer)
                except BusError:
                    print('cron: answer delivery failed', file=sys.stderr)
    except Shutdown:
        return 0


if __name__ == '__main__':
    sys.exit(main())
