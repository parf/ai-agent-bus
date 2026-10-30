"""The scheduling engine behind #cron and #at: one-shots and subscriptions kept in the agent's KV.

The agent sends under its own name, so each target's ordinary ACL decides whether
it receives (RD140); anyone admitted to the agent may schedule. One process fires
at a time (the record lock `scheduler`), and each firing is claimed in KV before
its send, so it is never sent twice (RD138). The entry's creator must itself be
allowed to send to the target, when it is added and before every send (RD141).
A copy that is refused or whose acceptance is unknown is dropped with one warning
and the subscription stays active; a one-shot is dropped and removed (RD161).
An agent subclasses Scheduler with
its call grammar and its methods.
"""
from datetime import datetime, timedelta, timezone
import hashlib
import json
import math
import os
import re
import secrets
import signal
import sys
import time

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
AUTHOR_CHECKS = 200  # distinct (creator, target) may_send questions per pass, above MAX_SUBS; the rest are deferred


# Why a copy was dropped, as the warning names it (the shared subscription vocabulary).
DROP_REASONS = ('admission_refused', 'target_gone', 'send_refused', 'send_unknown')


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


class Scheduler:
    """Subclasses set NAME, PACKAGE (for help), LATER_TOPIC and parse(body) -> call."""
    NAME = 'scheduler'
    PACKAGE = None
    LATER_TOPIC = 'later'
    METHODS = {}  # call method -> attribute, when the two differ
    KINDS = ('sub', 'later')  # stored entry kinds this agent loads and fires; others are ignored

    def __init__(self, bus, cfg, clock=time.time, log=None):
        self.bus, self.clock = bus, clock
        self.log = log or (lambda text: print(self.NAME + ': ' + text, file=sys.stderr))
        self.subs, self.later, self.claims = {}, {}, {}
        self.state = {}  # id -> outcome, sent, dropped, skipped and missed counts (memory)
        self.invalid = {}  # id -> (kind, creator or None): stored entries that do not parse
        self.taken = set()  # claims another process made; never this process's to clean up
        self.started = clock()
        self.author_check = None  # the last may_send outcome: 'ok' or 'unavailable'
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
        if self.migrate_ids(names):
            names = self.bus.kv.list()
        for entry in names.get('json', []):
            name = entry['name']
            kind, _, key = name.partition('.')
            if kind not in self.KINDS or not ID.fullmatch(key):
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
        self.unpause()
        self.sweep()

    def migrate_ids(self, names):
        """A stored entry whose id is not 12 hex moves to one that is. The new id is the first 12 hex of
        the SHA-256 of the old key, then of that digest again while it is taken, so a restart gives
        the same answer. The new entry is written before the old one is removed, and it names the old
        key, so a restart after the write finishes the move. True when anything moved."""
        present = {entry['name'] for entry in names.get('json', [])}
        moved = False
        for name in sorted(present):
            kind, _, key = name.partition('.')
            if kind not in self.KINDS or not name.partition('.')[2] or ID.fullmatch(key):
                continue
            candidate = hashlib.sha256(name.encode()).hexdigest()[:12]
            while True:
                taken = f'{kind}.{candidate}'
                if taken not in present:
                    break
                try:
                    if self.bus.kv.get(taken, kind='json').get('migrated_from') == key:
                        break  # the write happened, the removal did not
                except (BusError, AttributeError):
                    pass
                candidate = hashlib.sha256(candidate.encode()).hexdigest()[:12]
            try:
                if f'{kind}.{candidate}' not in present:
                    value = self.bus.kv.get(name, kind='json')
                    if not isinstance(value, dict):
                        raise ValueError('not an object')
                    self.bus.kv.set(f'{kind}.{candidate}', value | {'migrated_from': key}, kind='json', mode='add')
                self.bus.kv.delete(name, kind='json')
                self.log(f'{kind} {key} moved to {candidate}')
                moved = True
            except (BusError, ValueError, TypeError):
                self.log(f'stored {kind} {key} could not move to a 12-hex id; skipped')
        return moved

    def unpause(self):
        """An entry a release before RD161 paused is active again, and a paused one-shot is dropped:
        a copy is dropped, never held."""
        for key, sub in list(self.subs.items()):
            if sub.get('paused'):
                sub.pop('paused')
                try:
                    self.bus.kv.set('sub.' + key, self.stored(sub), kind='json', mode='replace')
                except BusError:
                    self.log(f'subscription {key} could not be reactivated; kept in memory')
        for key, entry in list(self.later.items()):
            if entry.get('paused'):
                entry.pop('paused')
                self.warn(key, 'send_unknown')
                self.count(key, 'dropped')
                self.finish(key, 'dropped')

    def sweep(self):
        """At start: a claim still `sending` is a copy whose acceptance is unknown, so dropped; orphans
        and old claims go."""
        now = self.clock()
        newest = {}
        for name, value in sorted(self.claims.items()):
            _, key, slot = name.split('.', 2)
            entry = self.subs.get(key) or self.later.get(key)
            if entry is None and key not in self.invalid:
                self.drop_claim(name, orphan=True)
                continue
            if value == 'sending':
                self.drop(key, name, 'send_unknown')
            elif value in ('unconfirmed', 'denied', 'gone') or value.startswith('retrying '):
                # What a release before RD161 left: that copy is over, and nothing is held for it.
                self.claims[name] = 'done'
                self.drop_claim(name)
            elif value in ('sent', 'refused', 'dropped') and key in self.later:
                self.finish(key, value)  # the removal after its outcome did not happen
            elif key in self.subs:
                newest.setdefault(key, []).append(name)
        for names in newest.values():
            for name in names[:-1]:
                self.drop_claim(name)

    def drop_claim(self, name, orphan=False):
        """Only settled claims are removed: `sending`, `unconfirmed` and `denied` are the evidence,
        unless their entry is gone."""
        if name in self.taken or not orphan and self.claims.get(name) == 'sending':
            return
        try:
            self.bus.kv.delete(name)
            self.claims.pop(name, None)
        except BusError:
            self.log('claim cleanup failed; kept')

    def claim(self, name, value, mode):
        self.bus.kv.set(name, value, mode=mode)
        self.claims[name] = value

    def stored(self, entry):
        return {k: v for k, v in entry.items() if k != 'spec'}

    def warn(self, key, why):
        """One warning for a dropped copy, naming the entry and why."""
        self.log(json.dumps({'id': key, 'warning': why}))

    def drop(self, key, name, why):
        """RD161: a refused copy, or one whose acceptance is unknown (it may have arrived), is dropped
        and counted with one warning. A subscription stays active for its next slot; a one-shot is
        dropped and removed. Nothing is held, retried or paused."""
        try:
            self.claim(name, 'dropped', 'set')
        except BusError:
            self.log('dropped state not stored')
        self.count(key, 'dropped')
        self.warn(key, why)
        if key in self.later:
            self.finish(key, 'dropped')
        else:
            self.outcome(key, 'dropped')

    def outcome(self, key, status):
        """The latest outcome, in memory (the one-shot record survives a restart)."""
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
        """One pass: the current local minute's slots and due one-shots."""
        if self.error or not self.scheduler():
            return
        now = self.clock()
        minute = cronexpr.wall(datetime.fromtimestamp(now, timezone.utc), self.cfg['tz'])
        slot = cronexpr.slot_key(minute)
        cache = {}  # (creator, target) -> may_send, for this pass
        for key, sub in list(self.subs.items()):
            try:
                if not sub['spec'].matches(minute):
                    continue
                name = f'fired.{key}.{slot}'
                if name in self.claims or name in self.taken or self.fired_after(key, minute):
                    continue
                self.fire(key, sub, name, now, now + self.period(sub, minute), cache)
            except BusError:
                raise
            except Exception:
                self.poison('sub', key)
        for key, entry in sorted(self.later.items(), key=lambda item: item[1]['at']):
            try:
                name = f'fired.{key}.once'
                if entry['at'] <= now and name not in self.claims and name not in self.taken:
                    self.fire(key, entry, name, now, max(entry['at'], now) + DAY, cache)
            except BusError:
                raise
            except Exception:
                self.poison('later', key)

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

    def author_verdict(self, cache, entry):
        """RD141, asked before every send: 'ok', 'lost' (the creator may not send), 'vanished'
        (404: the target is gone or hidden from this agent) or 'unavailable'. Answers are kept
        for this pass only; after the first failure, or AUTHOR_CHECKS questions, the pass asks
        no more and every later entry is 'unavailable' (a circuit breaker for a dead daemon)."""
        pair = (entry['by'], entry['target'])
        if pair not in cache:
            if cache.get('down'):
                return 'unavailable'
            if len(cache) >= AUTHOR_CHECKS:
                return 'deferred'
            try:
                cache[pair] = 'ok' if self.bus.may_send(*pair) else 'lost'
            except Refusal as e:
                cache[pair] = 'vanished' if e.status == 404 else 'unavailable'
                if cache[pair] == 'unavailable':
                    self.log(f'creator check refused with {e.status}; not sent')
            except BusError:
                cache['down'] = True
                cache[pair] = 'unavailable'
            self.author_check = 'unavailable' if cache[pair] == 'unavailable' else 'ok'
        return cache[pair]

    def author_lost(self, key):
        """The creator may no longer send to the target: that copy is dropped with a warning."""
        self.drop_unsent(key, 'admission_refused')

    def vanished(self, key):
        """The target is gone or hidden from this agent: that copy is dropped with a warning."""
        self.drop_unsent(key, 'target_gone')

    def drop_unsent(self, key, why):
        """A copy dropped before any claim or send: counted and warned, the entry stays (a one-shot goes)."""
        self.count(key, 'dropped')
        self.warn(key, why)
        if key in self.later:
            self.finish(key, 'dropped')
        else:
            self.outcome(key, 'dropped')

    def fire(self, key, entry, name, now, until, cache):
        verdict = self.author_verdict(cache, entry)
        if verdict != 'ok':
            return self.not_author(key, verdict)
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

    def not_author(self, key, verdict, name=None):
        """Nothing is sent: a lost creator or a vanished target drops the copy; unavailable waits."""
        if verdict in ('unavailable', 'deferred'):
            return self.count(key, 'missed' if verdict == 'unavailable' else 'deferred')  # the entry and any claim stay; the next pass asks again
        return self.author_lost(key) if verdict == 'lost' else self.vanished(key)

    def send(self, key, entry, name, now, until, attempt=0, claimed=False):
        """One attempt, never repeated; the tag names the entry's creator, the body goes unchanged."""
        try:
            if not claimed:
                self.claim(name, 'sending', 'set')
            self.bus.send(entry['target'], entry['body'], topic='cron' if key in self.subs else self.LATER_TOPIC,
                          tag='by ' + entry['by'], ttl=max(1, until - now))
        except Refusal as e:
            if not 400 <= e.status < 500:
                return self.drop(key, name, 'send_unknown')
            if e.status in (403, 404):
                return self.drop(key, name, 'admission_refused' if e.status == 403 else 'target_gone')
            return self.drop(key, name, 'send_refused')
        except UncertainOutcome:
            return self.drop(key, name, 'send_unknown')
        except BusError:
            # Not sent (the client raises UncertainOutcome once a request has left).
            return self.drop(key, name, 'send_refused')
        try:
            self.claim(name, 'sent', 'set')
        except BusError:
            self.log('sent but the success record failed; the claim stays sending')
        self.count(key, 'sent')
        if key in self.subs:
            self.outcome(key, 'sent')
            for old in [n for n in self.claims if n.startswith(f'fired.{key}.') and n != name]:
                self.drop_claim(old)
        else:
            self.finish(key, 'sent')

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
            result = dispatch(self.PACKAGE, message.ask)
            if result is not None:
                return result
            if self.error:
                return 'error: ' + self.error
            call = self.parse(message.body)
            if call['method'] == 'help':
                return metadata_answer(self.PACKAGE, as_json=message.body.lstrip().startswith('{'))
            method = self.METHODS.get(call['method'], call['method'].replace('.', '_'))
            return json.dumps(getattr(self, method)(call, message))
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

    def author(self, message, target):
        """RD141 at add: the caller must be allowed to send to the target itself."""
        try:
            allowed = self.bus.may_send(self.caller(message), target)
        except Refusal as e:
            if e.status == 404:
                raise CallError('unknown destination ' + target + ', or it does not allow ' + self.NAME) from None
            raise CallError('could not check that you may send to ' + target + '; nothing was stored') from None
        except BusError:
            raise CallError('could not check that you may send to ' + target + '; nothing was stored') from None
        if not allowed:
            raise CallError(f'you may not send to {target}, so {self.NAME} will not send there for you')

    def target(self, call):
        """Not cron itself, and an existing queue or agent: feedback for a typo, not authority."""
        if short(call['target']) == short(self.bus.name):
            raise CallError(self.NAME + ' does not schedule messages to itself')
        try:
            kind = self.bus.record(call['target']).lookup().get('kind')
        except Refusal as e:
            if e.status == 404:
                raise CallError('unknown destination ' + call['target'] + ', or it does not allow ' + self.NAME) from None
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
                if mine:
                    item['body'] = entry['body']
                out.append(item)
        for key, (kind, by) in sorted(self.invalid.items()):
            if kind == ('sub' if entries is self.subs else 'later') and (staff or by == self.caller(message)):
                out.append({'id': key, 'invalid': True, 'by': by})
        return out

    def remove(self, message, entries, key, kind):
        entry = entries.get(key)
        if entry is None and self.invalid.get(key, ('',))[0] == kind:
            # An invalid entry names no trustworthy target: its creator or cron's staff remove it.
            if not self.staff(message) and self.invalid[key][1] != self.caller(message):
                raise CallError(f'only its creator or {self.NAME}\'s Owner and Maintainers remove an invalid entry')
            self.bus.kv.delete(kind + '.' + key, kind='json')
            del self.invalid[key]
            for name in [n for n in self.claims if n.startswith(f'fired.{key}.')]:
                self.drop_claim(name, orphan=True)
            return {'deleted': key}
        if entry is None:
            raise CallError('unknown id')
        if not self.staff(message) and entry['by'] != self.caller(message):
            raise CallError(f'only its creator or {self.NAME}\'s Owner and Maintainers delete an entry')
        self.bus.kv.delete(kind + '.' + key, kind='json')
        entries.pop(key)
        for name in [n for n in self.claims if n.startswith(f'fired.{key}.')]:
            if self.claims[name] == 'sending':
                continue  # a send may be in flight: the claim stays as evidence until the next start's sweep
            self.claims[name] = 'done'
            self.drop_claim(name)
        return {'deleted': key}

    def later_add(self, call, message):
        now = self.clock()
        at = due(call, now)
        self.target(call)
        who = self.caller(message)
        if len(self.later) >= MAX_LATER or sum(e['by'] == who for e in self.later.values()) >= LATER_EACH:
            raise CallError(f'at most {MAX_LATER} pending one-shots, {LATER_EACH} per creator')
        self.author(message, call['target'])
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
        return {'later': rows, 'finished': finished, 'tz': str(self.cfg['tz']), **self.check_status()}

    def check_status(self):
        """Say why nothing fires when the creator check could not be made."""
        return {'author_check': 'unavailable'} if self.author_check == 'unavailable' else {}

    def later_cancel(self, call, message):
        return self.remove(message, self.later, call['id'], 'later')

    def wait(self):
        """Seconds to the next minute boundary or due one-shot or retry, at most 10."""
        now = self.clock()
        due = [60 - now % 60 + 0.2] + \
              [e['at'] - now for k, e in self.later.items() if f'fired.{k}.once' not in self.claims]
        return max(0.05, min(10, *due))


class Shutdown(Exception):
    pass


# Consecutive bus failures before the process gives up, about a minute at two
# seconds apart. A runner's sandbox mounts the bus socket as a file, so a daemon
# that restarted is out of reach until the process starts again (J30); exiting
# non-zero has the runner restart it, never 78, which reads as unconfigured.
BUS_FAILURES = 30


def run(cls):
    """The kept process: asks, then a loop of passes and calls."""
    result = dispatch(cls.PACKAGE, os.environ.get('AGENT_BUS_ASK'))
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
    failures = 0
    try:
        with Bus(timeout=5) as bus:
            agent = cls(bus, cfg)
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
                    failures += 1
                    if failures >= BUS_FAILURES:
                        print(f'{cls.NAME}: bus unavailable {failures} times in a row; exiting to be restarted', file=sys.stderr)
                        return 1
                    print(cls.NAME + ': bus unavailable; retrying', file=sys.stderr)
                    time.sleep(2)
                    continue
                failures = 0
                if message is None or message.receipt or message.re:
                    continue
                answer = agent.handle(message)
                try:
                    message.reply(answer)
                except BusError:
                    print(cls.NAME + ': answer delivery failed', file=sys.stderr)
    except Shutdown:
        return 0
