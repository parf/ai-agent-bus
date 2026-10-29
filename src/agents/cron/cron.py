#!/usr/bin/env python3
"""Scheduled sends: recurring subscriptions and one-shot `later` calls kept in the agent's KV.

Cron sends under its own name, so each target's ordinary ACL decides whether it
receives (RD140); anyone admitted to cron may schedule. One process fires at a
time (the record lock `scheduler`), and each firing is claimed in KV before its
send, so it is never sent twice (RD138).
"""
from datetime import datetime, timedelta, timezone
import json
import math
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


UNCONFIRMED = 'unconfirmed: delivery could not be confirmed; delete and add again to restart'
DENIED = 'denied: the target\'s ACL does not admit cron; allow it there, then delete and add again'
UNKNOWN = 'unknown destination: the target no longer exists; delete this entry'


class CallError(ValueError):
    pass


def valid(kind, value):
    """A stored entry, checked explicitly (no assert: it vanishes under python -O)."""
    if not isinstance(value, dict):
        raise ValueError('not an object')
    if not isinstance(value.get('target'), str) or not BUS_NAME.fullmatch(value['target']):
        raise ValueError('target')
    if not isinstance(value.get('body'), str) or not isinstance(value.get('by'), str) or not value['by']:
        raise ValueError('body or creator')
    if 'paused' in value and not isinstance(value['paused'], str):
        raise ValueError('paused')
    for number in ('created',) + (('at',) if kind == 'later' else ()):
        if type(value.get(number)) not in (int, float) or not math.isfinite(value[number]):
            raise ValueError(number)
    if kind == 'sub':
        value['spec'] = cronexpr.parse(value['cron'])
    return value


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
        self.invalid = {}  # id -> (kind, creator or None): stored entries that do not parse
        self.taken = set()  # claims another process made; never this process's to clean up
        self.unstored = set()  # subscriptions whose pause could not be stored yet
        self.started = clock()
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
            value = None
            try:
                value = self.bus.kv.get(name, kind='json')
                (self.subs if kind == 'sub' else self.later)[key] = valid(kind, value)
            except (BusError, KeyError, TypeError, ValueError):
                self.log(f'stored {kind} {key} is invalid; skipped')
                by = value.get('by') if isinstance(value, dict) and isinstance(value.get('by'), str) else None
                self.invalid[key] = (kind, by)
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
            if entry is None and key not in self.invalid:
                self.drop_claim(name, orphan=True)
                continue
            if value == 'sending':
                self.unconfirmed(key, name)
            elif value == 'unconfirmed' and key in self.later:
                self.later[key]['paused'] = 'unconfirmed'
            elif value == 'unconfirmed' and key in self.subs:
                self.pause(key, UNCONFIRMED)
            elif value == 'denied' and key in self.later:
                self.later[key]['paused'] = 'denied'
            elif value == 'denied' and key in self.subs:
                self.pause(key, DENIED)
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

    def drop_claim(self, name, orphan=False):
        """Only settled claims are removed: `sending`, `unconfirmed` and `denied` are the evidence,
        unless their entry is gone."""
        if name in self.taken or not orphan and self.claims.get(name) in ('sending', 'unconfirmed', 'denied'):
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
            self.store_pause(key)
        self.outcome(key, reason.split(':')[0])

    def store_pause(self, key):
        try:
            self.bus.kv.set('sub.' + key, self.stored(self.subs[key]), kind='json', mode='replace')
            self.unstored.discard(key)
        except BusError:
            self.unstored.add(key)
            self.log(f'subscription {key} pause not stored; retried next pass')

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

    def tick(self):
        """One pass: the current local minute's slots, due one-shots and due retries."""
        if self.error or not self.scheduler():
            return
        now = self.clock()
        minute = cronexpr.wall(datetime.fromtimestamp(now, timezone.utc), self.cfg['tz'])
        slot = cronexpr.slot_key(minute)
        for key in list(self.unstored):
            if key in self.subs:
                self.store_pause(key)
            else:
                self.unstored.discard(key)
        for key, sub in list(self.subs.items()):
            try:
                if not sub['spec'].matches(minute):
                    continue
                if sub.get('paused'):
                    self.count(key, 'skipped')
                    continue
                name = f'fired.{key}.{slot}'
                if name in self.claims or name in self.taken or self.fired_after(key, minute):
                    continue
                self.fire(key, sub, name, now, now + self.period(sub, minute))
            except BusError:
                raise
            except Exception:
                self.poison('sub', key)
        for key, entry in list(self.later.items()):
            try:
                name = f'fired.{key}.once'
                if entry['at'] <= now and name not in self.claims and name not in self.taken:
                    self.fire(key, entry, name, now, max(entry['at'], now) + DAY)
            except BusError:
                raise
            except Exception:
                self.poison('later', key)
        for name, (key, slot, attempt, at, until) in list(self.retries.items()):
            if at <= now:
                entry = self.subs.get(key) or self.later.get(key)
                if entry is None or entry.get('paused'):
                    self.retries.pop(name)
                    continue
                self.send(key, entry, name, now, until, attempt)

    def fired_after(self, key, minute):
        """A claim for this or a later slot within two hours: the clocks went back, so the
        repeated local minutes already fired."""
        for name in self.claims:
            if name.startswith(f'fired.{key}.'):
                try:
                    claimed = datetime.strptime(name.rsplit('.', 1)[1], '%Y-%m-%dT%H:%M')
                except ValueError:
                    continue
                if timedelta(0) < claimed - minute <= timedelta(hours=2):
                    return True
        return False

    def poison(self, kind, key):
        """An entry that breaks a pass is set aside, so it cannot stop every other schedule."""
        entry = (self.subs if kind == 'sub' else self.later).pop(key, None) or {}
        self.invalid[key] = (kind, entry.get('by'))
        self.log(f'{kind} {key} is invalid; set aside')

    def count(self, key, what):
        state = self.state.setdefault(key, {})
        state[what] = state.get(what, 0) + 1

    def period(self, sub, minute):
        later = cronexpr.next_slot(sub['spec'], minute, self.cfg['tz'])
        seconds = (later - minute).total_seconds() if later else DAY
        return max(60, min(DAY, seconds))

    def fire(self, key, entry, name, now, until):
        try:
            self.claim(name, 'sending', 'add')
        except Refusal as e:
            if e.status == 412:  # another process claimed this firing
                self.taken.add(name)
                return
            return self.count(key, 'missed')
        except BusError:
            return self.count(key, 'missed')
        self.send(key, entry, name, now, until, 0, claimed=True)

    def send(self, key, entry, name, now, until, attempt, claimed=False):
        """One attempt; the tag names the entry's creator, the body goes unchanged."""
        try:
            if not claimed:
                self.claim(name, 'sending', 'set')
            self.bus.send(entry['target'], entry['body'], topic='cron' if key in self.subs else 'later',
                          tag='by ' + entry['by'], ttl=max(1, until - now))
        except Refusal as e:
            if not 400 <= e.status < 500:
                return self.unsure(key, name)
            if e.status in (403, 404):
                return self.denied(key, name, e.status)
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

    def denied(self, key, name, status=403):
        """RD140: the target's ACL refused cron, or the target is gone; the entry pauses
        visibly and is not retried."""
        self.retries.pop(name, None)
        try:
            self.claim(name, 'denied', 'set')
        except BusError:
            self.log('denied state not stored')
        reason = DENIED if status == 403 else UNKNOWN
        if key in self.subs:
            self.pause(key, reason)
        elif key in self.later:
            self.later[key]['paused'] = reason.split(':')[0]
            self.outcome(key, reason.split(':')[0])

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
            self.pause(key, UNCONFIRMED)
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
        call = {}
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
            if call.get('method', '').endswith('list'):
                return 'error: the bus could not check this; try again'
            return 'error: the bus could not check or store this; nothing was changed'

    def caller(self, message):
        return getattr(message, 'from')

    def staff(self, message):
        return message.original_to in (None, '', self.bus.name) and bool({'owner', 'maintainer'} & set(message.roles))

    def target(self, call):
        """Not cron itself, and an existing queue or agent: feedback for a typo, not authority."""
        if short(call['target']) == short(self.bus.name):
            raise CallError('cron does not schedule messages to itself')
        try:
            kind = self.bus.record(call['target']).lookup().get('kind')
        except Refusal as e:
            if e.status == 404:
                raise CallError('unknown destination ' + call['target'] + ', or it does not allow cron') from None
            raise
        if kind not in ('agent', 'queue'):
            raise CallError('the target must be a queue or an agent, not a ' + str(kind))

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
        sub = {'cron': call['cron'], 'target': call['target'], 'body': call['body'], 'by': who, 'created': self.clock()}
        key = self.store('sub', sub)
        self.subs[key] = sub | {'spec': spec}
        return {'id': key, 'next': self.next(spec)}

    def next(self, spec):
        minute = cronexpr.wall(datetime.fromtimestamp(self.clock(), timezone.utc), self.cfg['tz'])
        slot = cronexpr.next_slot(spec, minute, self.cfg['tz'])
        return cronexpr.slot_key(slot) if slot else None

    def listing(self, message, entries, row):
        """The caller's own entries, with bodies; cron's staff see the others' without."""
        staff, out = self.staff(message), []
        for key, entry in sorted(entries.items()):
            mine = entry['by'] == self.caller(message)
            if mine or staff:
                item = {'id': key, 'target': entry['target'], 'by': entry['by'], **row(key, entry),
                        **{k: v for k, v in self.state.get(key, {}).items()}}
                if entry.get('paused'):
                    item['paused'] = entry['paused']
                if mine:
                    item['body'] = entry['body']
                out.append(item)
        for key, (kind, by) in sorted(self.invalid.items()):
            if kind == ('sub' if entries is self.subs else 'later') and (staff or by == self.caller(message)):
                out.append({'id': key, 'invalid': True, 'by': by})
        return out

    def sub_list(self, call, message):
        return {'subs': self.listing(message, self.subs, lambda k, s: {'cron': s['cron'], 'next': self.next(s['spec'])}),
                'tz': str(self.cfg['tz']),
                'outcomes_since': datetime.fromtimestamp(self.started, timezone.utc).isoformat()}

    def remove(self, message, entries, key, kind):
        entry = entries.get(key)
        if entry is None and self.invalid.get(key, ('',))[0] == kind:
            # An invalid entry names no trustworthy target: its creator or cron's staff remove it.
            if not self.staff(message) and self.invalid[key][1] != self.caller(message):
                raise CallError('only its creator or cron\'s Owner and Maintainers remove an invalid entry')
            self.bus.kv.delete(kind + '.' + key, kind='json')
            del self.invalid[key]
            for name in [n for n in self.claims if n.startswith(f'fired.{key}.')]:
                self.drop_claim(name, orphan=True)
            return {'deleted': key}
        if entry is None:
            raise CallError('unknown id')
        if not self.staff(message) and entry['by'] != self.caller(message):
            raise CallError('only its creator or cron\'s Owner and Maintainers delete an entry')
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
        finished = [{'id': k, **v} for k, v in sorted(done.items())
                    if self.staff(message) or v.get('by') == self.caller(message)]
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
