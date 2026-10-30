"""Single-process 60-second polling of one mailbox into the shared subscriptions (B.29).

One UID at a time: the forwarding frontier commits past a UID before its copies
are sent, so a stop can lose that UID's copies but never sends them twice. Each
copy is checked and sent once by the shared engine; a refused or unknown copy is
dropped, counted and warned, and the subscription stays active.
"""
import datetime
import json
import sys
import time

from agent_bus.errors import BusError, Refusal
from matching import MatchError, match_text
from mailbox_session import SessionError
from outbox import Outbox
import progress
from publication import PublicationError, forwarded
import wire

import subscriptions as shared   # src/agents/shared, on the path imap.py sets

PROGRESS = 'imap.progress'
SCAN = 'imap.discovery'   # the per-subscription discovery snapshot of before B.29; removed at start
BLOCK = 'imap.progress_error'
LEGACY = {'id', 'target', 'created_by', 'created_at', 'filter', 'ledger'}
FETCH_TRIES = 3
MIGRATE_TRIES = 16
JOURNAL = 'imap.migrate.'
LEDGER = {'version', 'identity', 'validity', 'start', 'floor', 'pause', 'entries', 'intake', 'intake_cleared',
          'last_reset_pause', 'sent', 'skipped', 'skipped_first', 'skipped_last', 'skips', 'discarded', 'resets'}
ENTRY = {'state', 'bytes', 'attempts', 'due', 'reason'}
STATES = {'pending', 'sending', 'retrying', 'sent', 'skipped', 'paused'}
MAX_COUNTER = 2**63 - 1
MAX_PENDING, MAX_BYTES, MAX_VALUE, MAX_ENTRIES = 64, 16 * 1024 * 1024, 96 * 1024, 128
REASONS = {'backlog', 'unconfirmed', 'acl', 'oversize', 'malformed', 'ambiguous',
           'fetch', 'unmatched', 'regex', 'reset', 'refused'}
TERMINAL = {'sent', 'skipped'}
MESSAGES = {
    'invalid_target': 'target must be one queue or agent name of at most 64 ASCII characters',
    'invalid_selector': 'invalid subscription selector',
    'invalid_filter': 'filter is not a supported /regex/: at most 200 characters, flags imsu',
    'invalid_id': 'subscription id must be 12 lower-case hex characters',
    'limit': 'at most 100 subscriptions, 20 per author',
    'not_found': 'no such subscription',
    'not_author': 'only its author or this IMAP Owner/Maintainers may delete it',
    'admission_refused': 'the author may not send to that target or to this IMAP agent',
    'admission_unavailable': 'author admission unavailable; nothing stored',
    'store_unknown': 'subscription store unavailable',
}


class RetryScan(ValueError):
    pass


def message(error):
    """A shared engine code as the sentence this agent answers with."""
    return MESSAGES.get(getattr(error, 'code', ''), str(error))


def selector(value):
    """IMAP watches its one mailbox; the selector is where a subscription starts in it."""
    if not isinstance(value, dict) or set(value) != {'validity', 'from_uid'}:
        raise ValueError('selector is the mailbox validity and first UID')
    progress.integer(value['validity'], 1)
    progress.integer(value['from_uid'], 1, progress.MAX_UID + 1)
    return value


def selected(value, event):
    # A new UIDVALIDITY is a new mailbox: its mail is all after the subscription.
    return event['validity'] != value['validity'] or event['uid'] >= value['from_uid']


def legacy_ledger(value):
    """The removed pre-B.29 ledger validator, kept verbatim for the migration: a ledger it
    refused stays raw and inert as invalid_state, never an active row."""
    if not isinstance(value, dict) or set(value) != LEDGER or type(value['version']) is not int or value['version'] != 1:
        raise progress.ProgressError('invalid subscription delivery state')
    if not isinstance(value['identity'], str) or not 1 <= len(value['identity']) <= 512:
        raise progress.ProgressError('invalid subscription mailbox identity')
    progress.integer(value['validity'], 1)
    progress.integer(value['start'], 1, progress.MAX_UID + 1)
    progress.integer(value['floor'], value['start'], progress.MAX_UID + 1)
    if value['pause'] is not None and (not isinstance(value['pause'], str) or value['pause'] not in {'acl', 'unconfirmed'}):
        raise progress.ProgressError('invalid subscription pause reason')
    if value['intake'] is not None and (not isinstance(value['intake'], str) or value['intake'] not in {'backlog', 'regex'}):
        raise progress.ProgressError('invalid subscription intake reason')
    if value['last_reset_pause'] is not None and (not isinstance(value['last_reset_pause'], str) or value['last_reset_pause'] not in {'acl', 'unconfirmed'}):
        raise progress.ProgressError('invalid historical reset reason')
    entries = value['entries']
    if not isinstance(entries, dict) or len(entries) > MAX_ENTRIES:
        raise progress.ProgressError('subscription delivery entry cap exceeded')
    for uid, entry in entries.items():
        if not isinstance(uid, str) or len(uid) > 10 or not uid.isascii() or not uid.isdigit() or uid != str(int(uid)):
            raise progress.ProgressError('invalid subscription delivery UID')
        progress.integer(int(uid), value['start'])
        if not isinstance(entry, dict) or set(entry) != ENTRY or not isinstance(entry['state'], str) or entry['state'] not in STATES:
            raise progress.ProgressError('invalid subscription delivery entry')
        progress.integer(entry['bytes'], 0, 2 * 1024 * 1024)
        progress.integer(entry['attempts'], 0, MAX_COUNTER)
        progress.integer(entry['due'], 0, MAX_COUNTER)
        if entry['reason'] is not None and (not isinstance(entry['reason'], str) or entry['reason'] not in REASONS):
            raise progress.ProgressError('invalid subscription delivery reason')
        if entry['state'] in TERMINAL and entry['bytes'] != 0:
            raise progress.ProgressError('terminal delivery retains a body reference')
        if entry['state'] in {'sending', 'retrying', 'sent'} and entry['attempts'] < 1:
            raise progress.ProgressError('attempted delivery lacks its sending marker')
        if (entry['state'] == 'pending' or (entry['state'] == 'skipped' and entry['reason'] not in {'refused', 'fetch', 'oversize', 'malformed'})) and entry['attempts'] != 0:
            raise progress.ProgressError('unattempted delivery has an attempt count')
        if (entry['state'] == 'retrying') != (entry['due'] > 0):
            raise progress.ProgressError('delivery backoff does not match its state')
        if entry['state'] in {'pending', 'sending', 'retrying', 'sent'} and entry['reason'] is not None:
            raise progress.ProgressError('delivery state has an unexpected reason')
        if entry['state'] == 'skipped' and entry['reason'] is None:
            raise progress.ProgressError('skipped delivery lacks its reason')
        if entry['state'] == 'skipped' and entry['reason'] == 'refused' and entry['attempts'] < 1:
            raise progress.ProgressError('refused delivery lacks its attempt')
        if entry['state'] == 'paused' and (entry['reason'] not in {'acl', 'unconfirmed'} or value['pause'] is None):
            raise progress.ProgressError('paused delivery lacks its visible reason')
        if entry['state'] == 'paused' and entry['reason'] == 'unconfirmed' and entry['attempts'] < 1:
            raise progress.ProgressError('uncertain delivery lacks its sending marker')
    holds = {int(uid): entry['bytes'] for uid, entry in value['entries'].items() if entry['state'] not in TERMINAL}
    if len(holds) > MAX_PENDING or sum(holds.values()) > MAX_BYTES:
        raise progress.ProgressError('subscription pending delivery budget exceeded')
    for field in ('sent', 'skipped', 'discarded', 'resets', 'intake_cleared'):
        progress.integer(value[field], 0, MAX_COUNTER)
    first, last = value['skipped_first'], value['skipped_last']
    if value['skipped']:
        progress.integer(first, 1)
        progress.integer(last, first)
    elif first is not None or last is not None:
        raise progress.ProgressError('empty skipped counter has a UID range')
    if not isinstance(value['skips'], dict) or len(value['skips']) > len(REASONS):
        raise progress.ProgressError('invalid per-reason skip counters')
    for reason, counter in value['skips'].items():
        if reason not in REASONS - {'unmatched'} or not isinstance(counter, dict) or set(counter) != {'count', 'first', 'last'}:
            raise progress.ProgressError('invalid per-reason skip counter')
        progress.integer(counter['count'], 1, MAX_COUNTER)
        progress.integer(counter['first'], 1)
        progress.integer(counter['last'], counter['first'])
    if sum(counter['count'] for counter in value['skips'].values()) != value['skipped']:
        raise progress.ProgressError('skip totals do not agree')
    if len(json.dumps(value, separators=(',', ':'), ensure_ascii=False).encode()) > MAX_VALUE:
        raise progress.ProgressError('subscription delivery value exceeds 96 KiB')
    return value


class Forwarder:
    def __init__(self, bus, name, work, source, clock=time.monotonic, wall=time.time, send_one=None):
        self.bus, self.name, self.source, self.clock, self.wall = bus, name, source, clock, wall
        # Held for this kept process's lifetime: another generation cannot
        # forward the same mailbox until this process has stopped.
        self.generation = Outbox(work, lock_wait=0)
        self.subs = shared.Subscriptions(bus, name, validate_selector=selector, selector_matches=selected,
                                         warning=self.warn, clock=wall, delivery=self.send_rows)
        self.send_one = send_one or self.send_shared
        self.kv = self.subs.kv
        self.count, self.damaged = 0, 0
        self.next_run, self.next_poll = 0, 0
        self.period, self.arrivals = self.clock(), 0
        self.error, self.state_errors = None, 0
        self.sent = self.dropped = 0
        self.warnings = {}
        self.fetch_tries = {}
        self.view_at = None
        self.backlog = 0
        try:
            self.migrate()
        except BaseException:
            self.generation.close()   # as a stopped process would release it
            raise

    def warn(self, code):
        self.warnings[code] = self.warnings.get(code, 0) + 1
        print(json.dumps({'method': 'forward', 'warning': code}), file=sys.stderr)

    def get(self, key):
        try:
            return self.kv.get(key, kind='json')
        except Refusal as error:
            if error.status == 404:
                return None
            raise

    def set(self, key, value):
        self.kv.set(key, value, kind='json', mode='set')

    def migrate(self):
        """Move pre-B.29 rows onto the shared shape before anything is served; held copies are dropped and counted.

        A row keeps its target, author, filter and start; its 8-hex id gains the first suffix
        "0000", "0001", ... whose key is free. The journal names that key before the row is
        created, so the next start finishes an interrupted move into it and an occupied key is
        never taken for one of ours. The old key goes only after the new row is stored. One that
        cannot be read is kept as it was under the new id, for the Owner and Maintainers to delete.
        """
        listing = self.kv.list()
        names = [row.get('name') for row in listing.get('json', []) if isinstance(row, dict)]
        legacy = [n for n in names if isinstance(n, str) and n.startswith('sub.') and len(n) == 12]
        for key in legacy:
            value = self.get(key)
            if value is None:
                continue
            journal = JOURNAL + key[4:]
            mark = self.get(journal)
            new = mark.get('to') if isinstance(mark, dict) else None
            if not (isinstance(new, str) and len(new) == 16 and new.startswith(key) and all(c in '0123456789abcdef' for c in new[12:])):
                new = next((f'{key}{suffix:04x}' for suffix in range(MIGRATE_TRIES) if self.get(f'{key}{suffix:04x}') is None), None)
                if new is None:
                    raise ValueError(f'no free subscription id for {key[4:]}')
                self.kv.set(journal, {'to': new}, kind='json', mode='set')
            try:
                self.kv.set(new, self.migrated(value, new[4:]), kind='json', mode='add')
            except Refusal as error:
                if error.status != 412:
                    raise   # 412: the journal reserved this key, so it is our create from before a stop
            self.kv.delete(key, kind='json')
            self.kv.delete(journal, kind='json')
        for name in names:
            if isinstance(name, str) and name.startswith(JOURNAL) and 'sub.' + name[len(JOURNAL):] not in legacy:
                self.kv.delete(name, kind='json')   # stopped after the old key went
        if SCAN in names:
            self.kv.delete(SCAN, kind='json')
        self.generation.sweep(set())   # the held copies of before B.29

    def migrated(self, value, sub_id):
        try:
            if not isinstance(value, dict) or set(value) != LEGACY:
                raise ValueError()
            state = legacy_ledger(value['ledger'])
            # As the old mailbox reset counted its losses: earlier discards, counted skips
            # (compacted entries included) and the copies still held; unmatched mail is no loss.
            held = sum(1 for entry in state['entries'].values() if entry['state'] not in ('sent', 'skipped'))
            row = {'id': sub_id, 'target': shared.target_text(value['target']),
                   'selector': selector({'validity': state['validity'], 'from_uid': state['start']}),
                   'filter': value['filter'], 'created_by': shared.identity(value['created_by']),
                   'created_at': shared.stamp(lambda: datetime.datetime.fromisoformat(value['created_at']).timestamp()),
                   'sent': state['sent'], 'dropped': state['discarded'] + state['skipped'] + held}
            return self.subs._row(row, sub_id)
        except (KeyError, ValueError, TypeError, AttributeError, shared.SubscriptionError, progress.ProgressError):
            return value

    def rows(self):
        try:
            rows, damaged = self.subs._load()
        except shared.SubscriptionError as error:
            raise ValueError(message(error)) from None
        self.count, self.damaged = len(rows), len(damaged)
        self.view_at = self.clock()
        return rows, damaged

    def current_progress(self, snapshot, *, first_add=False):
        identity, validity, uidnext = snapshot
        previous = self.get(PROGRESS)
        if self.get(BLOCK) is not None and not first_add:
            raise progress.ProgressError('forwarding state invalid; delete all subscriptions then add to rebaseline')
        existing = previous
        if first_add and previous is None:
            previous = self.source.legacy()
        elif previous is None:
            raise progress.ProgressError('forwarding progress is missing; explicit rebaseline required')
        try:
            value, _reset = progress.load(previous, identity, validity, uidnext)
        except progress.ProgressError as error:
            if not first_add:
                self.set(BLOCK, {'reason': error.code})
                raise
            value, _reset = progress.load(self.source.legacy(), identity, validity, uidnext)
        if first_add:
            value = progress.fast_forward(value, uidnext, [], [])
        if first_add or value != existing:
            self.set(PROGRESS, value)
        if first_add:
            self.kv.delete(BLOCK, kind='json')
        return value

    def add(self, message, target, filter_text=''):
        rows, damaged = self.rows()   # read-only add preflight
        identity, validity, uidnext = self.source.snapshot()
        state = self.current_progress((identity, validity, uidnext), first_add=not rows and not damaged)
        start = {'validity': validity, 'from_uid': progress.subscription_start(state, uidnext)}
        try:
            value = self.subs.add(message, target, start, filter_text)
        except shared.SubscriptionError as error:
            raise shared.SubscriptionError(error.code) from None
        self.next_run = self.next_poll = 0
        return value

    def delete(self, message, sub_id):
        self.subs.delete(message, sub_id)
        rows, damaged = self.rows()
        if not rows and not damaged:
            for key in (PROGRESS, SCAN, BLOCK):
                self.kv.delete(key, kind='json')
            self.source.close()
        self.next_run = 0
        return True

    def text(self, message):
        lines = []
        for row in self.subs.list(message):
            if row.get('reason') == 'invalid_state':
                lines.append(f"{row['id']} invalid subscription state; only this IMAP Owner/Maintainers may delete it")
                continue
            lines.append(f"{row['id']} {row['target']} by={row['created_by']} at={row['created_at']} "
                         f"filter={row['filter']!r} from_uid={row['selector']['from_uid']} "
                         f"sent={row['sent']} dropped={row['dropped']}" +
                         ('; filter no longer supported: matches nothing' if row.get('reason') == 'invalid_filter' else ''))
        return '\n'.join(lines) if lines else '(no subscriptions you may see; manual cache only if none at all)'

    def send_rows(self, rows, body, topic, tag, deadline):
        """Each row's copy names its own subscription and author, so it is sent on its own.

        The body is fetched here, once, only when a filter matched; a fetch that fails drops these copies.
        """
        raw, skip = body()
        if skip is not None:
            print(json.dumps({'method': 'forward', 'skipped': skip}), file=sys.stderr)
            return {row['id']: ('dropped', 'invalid_text') for row in rows}
        result = {}
        for row in rows:
            try:
                body = forwarded(raw, row['created_by'], row['id'], self.source.masks)
            except PublicationError:
                result[row['id']] = ('dropped', 'invalid_text')
                continue
            result[row['id']] = self.send_one(row, body, deadline)
        return result

    def send_shared(self, row, body, deadline):
        # The engine's private send helper: author checks to target and to this agent, then one send.
        return self.subs._deliver([row], body, 'imap', row['id'], deadline)[row['id']]

    def fetch(self, read, number):
        """One fetch; a refusal is tried again on later polls, FETCH_TRIES times in all."""
        try:
            return read(number), None
        except wire.Refused as error:
            if error.throttled:
                raise SessionError('authentication backoff') from None
            self.fetch_tries[number] = self.fetch_tries.get(number, 0) + 1
            if self.fetch_tries[number] < FETCH_TRIES:
                raise RetryScan('mail fetch will retry on the next poll') from None
            return None, 'fetch'
        except (wire.LiteralTooLarge, wire.TooLarge):
            return None, 'oversize'
        except wire.WireError:
            return None, 'malformed'

    def discover_uid(self, state, number):
        event = {'validity': state['validity'], 'uid': number}
        text = skip = None
        if self.subs.has_candidates(event):
            self.arrivals += 1
            header, skip = self.fetch(self.source.headers, number)
            if skip is None:
                try:
                    text = match_text(header)
                except MatchError as error:
                    skip = error.code
        # Past this UID before any copy is sent: a stop now loses its copies, never repeats them.
        state = progress.advance_observation(state, number, set(), set())
        self.set(PROGRESS, state)
        self.fetch_tries.pop(number, None)
        if skip is not None:
            self.warn('invalid_text')
            print(json.dumps({'method': 'forward', 'uid': number, 'skipped': skip}), file=sys.stderr)
        elif text is not None:
            fetched = []
            def body():
                if not fetched:
                    try:
                        fetched.append((self.source.body(number), None))
                    except (wire.TooLarge, wire.LiteralTooLarge):
                        fetched.append((None, 'oversize'))
                    except (wire.WireError, SessionError, OSError):
                        fetched.append((None, 'fetch'))
                return fetched[0]
            outcome = self.subs.deliver(event, text, body, topic='imap', tag='', deadline=time.monotonic() + 15)
            self.sent += outcome['sent']
            self.dropped += outcome['dropped']
        return state

    def tick(self):
        now = self.clock()
        if now < self.next_run:
            return
        self.next_run = now + 60
        if now - self.period >= 60:
            self.period, self.arrivals = now, 0
        deadline = self.clock() + 2  # Soft slice; each wire op has its own alarm.
        try:
            rows, damaged = self.rows()
            self.error = 'invalid subscription state' if damaged else None
            if not rows:
                self.source.close()
                return
            if now < self.next_poll:
                return
            self.next_poll = now + 60
            snapshot = self.source.snapshot()
            state = self.current_progress(snapshot)
            uidnext, windows = snapshot[2], 0
            while state['observe'] < uidnext and self.arrivals < 100 and windows < 10 and self.clock() < deadline:
                start = state['observe']
                end = min(start + wire.WINDOW - 1, uidnext - 1)
                found = set(self.source.search(start, end))
                windows += 1
                while state['observe'] <= end and self.arrivals < 100 and self.clock() < deadline:
                    number = state['observe']
                    if number in found:
                        state = self.discover_uid(state, number)
                    else:
                        upto = min((uid for uid in found if uid > number), default=end + 1)
                        state = progress.advance_absent(state, upto, start=start, end=end, uidnext=uidnext, found=found)
                        self.set(PROGRESS, state)
            self.backlog = max(0, uidnext - state['observe'])
            if self.backlog and self.arrivals < 100:
                self.next_run = self.next_poll = self.clock() + 1
        except RetryScan:
            self.error = 'mail fetch retry pending'
            self.next_poll = 0
        except (BusError, OSError, ValueError, KeyError, TypeError, AttributeError) as error:
            self.state_errors += 1
            self.error = ('forwarding state invalid' if isinstance(error, progress.ProgressError)
                          else 'bus unavailable' if isinstance(error, BusError)
                          else 'IMAP ' + error.code if isinstance(error, SessionError)
                          else str(error) if isinstance(error, ValueError) and str(error) in MESSAGES.values()
                          else 'mailbox unavailable')
            self.source.close()

    def status(self):
        word = 'degraded' if self.error or self.damaged else 'ok'
        warned = ' '.join(f'{code}={count}' for code, count in sorted(self.warnings.items())) or 'none'
        return (f"{word}: {self.error or ('manual cache only' if not self.count else '60 s polling')}; "
                f"subscriptions={self.count} damaged={self.damaged} backlog={self.backlog} "
                f"sent={self.sent} dropped={self.dropped} warnings={warned} "
                f"view_age_s={max(0, int(self.clock() - self.view_at)) if self.view_at is not None else 'unknown'}")

    def wait(self):
        return max(.05, min(10, self.next_run - self.clock()))

    def close(self):
        try:
            self.subs.close()
        except shared.SubscriptionError:
            pass
        self.source.close()
        self.generation.close()
