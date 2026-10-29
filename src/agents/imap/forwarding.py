"""Single-process 60-second polling; one UID's durable outcomes before O moves."""
import copy
import json
import sys
import time

from agent_bus.errors import BusError, Refusal, UnexpectedAnswer
import delivery
import ledger
from matching import MatchError, RegexTimeout, match_text, matches
from mailbox_session import SessionError
from outbox import MissingBody, Outbox, OutboxError, filename
import progress
from subscriptions import Subscriptions, SubscriptionError
import wire

PROGRESS = 'imap.progress'
SCAN = 'imap.discovery'
BLOCK = 'imap.progress_error'
SCAN_FIELDS = {'identity', 'validity', 'uid', 'required', 'fetch_tries'}


class RetryScan(ValueError):
    pass


class Forwarder:
    def __init__(self, bus, name, work, source, clock=time.monotonic, wall=time.time):
        self.bus, self.name, self.source, self.clock, self.wall = bus, name, source, clock, wall
        # Held for this kept process's lifetime: another generation cannot
        # inspect/update the same ledgers until this process has stopped.
        self.bodies = Outbox(work)
        self.subs = Subscriptions(bus, name, self.baseline, self.cleanup, wall)
        self.kv = self.subs.kv
        self.values, self.damaged = [], {}
        self.references_known = False
        self.next_run, self.next_poll = 0, 0
        self.period, self.arrivals, self.attempts = self.clock(), 0, 0
        self.error, self.authority_errors, self.state_errors = None, 0, 0
        self.outbox_failures = 0
        self.notified = set()
        self.view_at = None
        self.missing_bodies, self.backlog = 0, 0
        self.header_cache = None

    def get(self, key):
        try:
            return self.kv.get(key, kind='json')
        except Refusal as error:
            if error.status == 404:
                return None
            raise

    def set(self, key, value):
        self.kv.set(key, value, kind='json', mode='set')

    def save_sub(self, sub, state):
        updated = copy.deepcopy(sub)
        updated['ledger'] = state
        try:
            self.subs.save(updated)
        except BusError:
            # The write may have committed. Don't sweep using a stale view.
            self.references_known = False
            raise
        sub.clear()
        sub.update(updated)

    def load_subs(self):
        self.references_known = False
        self.values, self.damaged = self.subs.for_driver()
        self.references_known = not self.damaged
        self.view_at = self.clock()
        for sub in self.values:
            # At every pass, not just startup: failed commits may have left a
            # sending marker in this same process. No send is active here.
            state = ledger.recover(sub['ledger'])
            state = ledger.refresh_intake(state)
            if state != sub['ledger']:
                self.save_sub(sub, state)

    def sweep(self):
        if not self.references_known:
            return False
        keep = {filename(sub['ledger']['identity'], sub['ledger']['validity'], uid)
                for sub in self.values for uid in ledger.retained(sub['ledger'])}
        self.bodies.sweep(keep)
        self.missing_bodies = len(keep - self.bodies.listing().keys())
        return True

    def cleanup(self):
        # Explicit delete commits before this callback. Reload the complete
        # reference set, never sweep against a partially deleted snapshot.
        self.values, self.damaged = self.subs.all()
        self.references_known = not self.damaged
        if not self.values and not self.damaged:
            for key in (PROGRESS, SCAN, BLOCK):
                self.kv.delete(key, kind='json')
            self.source.close()
            self.header_cache = None
        self.next_run = 0
        return self.sweep()

    def current_progress(self, snapshot, *, first_add=False):
        identity, validity, uidnext = snapshot
        previous = self.get(PROGRESS)
        block = self.get(BLOCK)
        if block is not None and not first_add:
            raise progress.ProgressError('forwarding state invalid; delete all subscriptions then add to rebaseline')
        existing = previous
        if first_add and (previous is None or block is not None):
            previous = self.source.legacy()
        elif previous is None and (self.values or self.damaged):
            raise progress.ProgressError('forwarding progress is missing; explicit rebaseline required')
        try:
            value, reset = progress.load(previous, identity, validity, uidnext)
        except progress.ProgressError as error:
            if first_add:
                value, reset = progress.load(self.source.legacy(), identity, validity, uidnext)
            else:
                self.set(BLOCK, {'reason': error.code})
                raise
        if first_add:
            value = progress.fast_forward(value, uidnext, [], [])
        if first_add or value != existing:
            # Commit the new namespace before resetting sub rows. A stop in
            # the reset loop reuses this B and completes the remaining rows.
            self.set(PROGRESS, value)
        if first_add:
            self.kv.delete(SCAN, kind='json')
            self.kv.delete(BLOCK, kind='json')
        for sub in self.values:
            state = sub['ledger']
            if state['identity'] != identity or state['validity'] != validity:
                self.save_sub(sub, ledger.mailbox_reset(state, identity, validity, value['baseline']))
            elif state['floor'] > max(value['observe'], state['start']):
                self.set(BLOCK, {'reason': 'rollback'})
                raise progress.ProgressError('forwarding progress rolled back behind a compacted delivery')
        scan = self.get(SCAN)
        if scan is not None and not isinstance(scan, dict):
            raise progress.ProgressError('invalid discovery state')
        if scan is not None and (scan.get('identity') != identity or scan.get('validity') != validity):
            self.kv.delete(SCAN, kind='json')
            self.header_cache = None
        return value

    def baseline(self):
        self.values, self.damaged = self.subs.all()  # Read-only add preflight.
        snapshot = self.source.snapshot()
        self.current_progress(snapshot, first_add=not self.values and not self.damaged)
        return snapshot

    def add(self, message, target, filter_text=''):
        value = self.subs.add(message, target, filter_text)
        self.next_run = 0
        self.next_poll = 0
        return value

    def delete(self, message, sub_id):
        return self.subs.delete(message, sub_id)

    def load_scan(self, state):
        scan = self.get(SCAN)
        if scan is None:
            return None
        if not isinstance(scan, dict) or set(scan) != SCAN_FIELDS:
            raise progress.ProgressError('invalid discovery state')
        progress.integer(scan['uid'], 1)
        progress.integer(scan['fetch_tries'], 0, 4)
        if not isinstance(scan['required'], list) or len(scan['required']) > 100 or any(not isinstance(sid, str) for sid in scan['required']) or len(set(scan['required'])) != len(scan['required']):
            raise progress.ProgressError('invalid discovery subscription snapshot')
        from publication import validate_id
        for sub_id in scan['required']:
            validate_id(sub_id)
        if scan['identity'] != state['identity'] or scan['validity'] != state['validity'] or scan['uid'] < state['observe']:
            self.kv.delete(SCAN, kind='json')
            return None
        if scan['uid'] != state['observe']:
            raise progress.ProgressError('discovery would skip an unrecorded UID')
        return scan

    def scan_for(self, state):
        scan = self.load_scan(state)
        if scan is None:
            scan = {'identity': state['identity'], 'validity': state['validity'], 'uid': state['observe'],
                    'required': [sub['id'] for sub in self.values if sub['ledger']['start'] <= state['observe']],
                    'fetch_tries': 0}
            self.set(SCAN, scan)
            self.arrivals += 1
        return scan

    def retry_fetch(self, scan):
        scan['fetch_tries'] += 1
        self.set(SCAN, scan)
        if scan['fetch_tries'] <= 3:
            raise RetryScan('mail fetch will retry on the next poll')

    def discover_uid(self, state, deadline=float('inf')):
        scan = self.scan_for(state)
        number = scan['uid']
        by_id = {sub['id']: sub for sub in self.values}
        # Deleted subs cancel their outstanding discovery; damaged rows send
        # nothing and keep reference sets unknown rather than erase mail.
        current = [by_id[sid] for sid in scan['required'] if sid in by_id]
        todo = [sub for sub in current if str(number) not in sub['ledger']['entries']
                and number >= sub['ledger']['floor']]
        raw, text, body_error = None, None, None
        header_error = 'fetch' if scan['fetch_tries'] >= 4 else None
        if todo and header_error is None:
            key = filename(state['identity'], state['validity'], number)
            try:
                raw = self.bodies.get(key)
                header = raw.partition(b'\r\n\r\n')[0].partition(b'\n\n')[0] + b'\r\n\r\n'
            except MissingBody:
                try:
                    header = self.source.headers(number)
                except wire.Refused as error:
                    if error.throttled:
                        raise SessionError('authentication backoff') from None
                    self.retry_fetch(scan)
                    header_error = 'fetch'
                except wire.LiteralTooLarge:
                    header_error = 'oversize'
                except wire.WireError:
                    header_error = 'malformed'
            if header_error is None:
                try:
                    text = match_text(header)
                except MatchError as error:
                    header_error = error.code
        spent = 0.0
        for sub in todo:
            if spent >= .2 or self.clock() >= deadline:
                return state, False  # O stays here; deferral is NOT no-match.
            value = ledger.compact(sub['ledger'], state['observe'])
            skip = header_error
            if skip is None:
                if value['intake'] == 'regex':
                    skip = 'regex'
                else:
                    pattern = self.subs.compiled[sub['id']][1]
                    began = self.clock()
                    try:
                        selected = matches(pattern, text)
                    except RegexTimeout:
                        value = ledger.disable_filter(value)
                        skip = 'regex'
                    finally:
                        spent += self.clock() - began
                    if skip is None:
                        skip = 'unmatched' if not selected else value['pause'] or value['intake']
            if skip is not None:
                self.save_sub(sub, ledger.record(value, number, skip=skip))
                continue
            if raw is None and body_error is None:
                try:
                    raw = self.source.body(number)
                except wire.Refused as error:
                    if error.throttled:
                        raise SessionError('authentication backoff') from None
                    self.retry_fetch(scan)
                    body_error = 'fetch'
                except wire.TooLarge:
                    body_error = 'oversize'
                except wire.WireError:
                    body_error = 'fetch'
            if body_error:
                self.save_sub(sub, ledger.record(value, number, skip=body_error))
                continue
            delivery.queue(value, number, raw, self.bodies, sub['created_by'], sub['id'], self.source.masks,
                           lambda updated, target=sub: self.save_sub(target, updated))
        required = set(scan['required'])
        recorded = {sub['id'] for sub in current if str(number) in sub['ledger']['entries'] or number < sub['ledger']['floor']}
        recorded |= required - set(by_id)  # Explicit deletions/damaged entries.
        advanced = progress.advance_observation(state, number, required, recorded)
        self.set(PROGRESS, advanced)
        self.kv.delete(SCAN, kind='json')
        self.header_cache = None
        return advanced, True

    def deliver_ready(self, deadline):
        # One ready item per sub per round, never let one dead target consume
        # the whole pass. The loop is resumed next tick if its soft slice ends.
        unavailable = set()
        while self.attempts < 100 and self.clock() < deadline:
            changed = False
            for sub in self.values:
                if sub['id'] in unavailable:
                    continue
                ready = ledger.ready(sub['ledger'], int(self.wall()))
                if not ready or self.attempts >= 100 or self.clock() >= deadline:
                    continue
                self.attempts += 1
                authority_failed = False
                def authorize(who, target):
                    nonlocal authority_failed
                    try:
                        return self.subs.author_allowed(who, target)
                    except BusError:
                        authority_failed = True
                        raise
                try:
                    delivery.execute(sub['ledger'], ready[0], self.bodies, sub['created_by'], sub['id'], self.source.masks,
                                     lambda value, target=sub: self.save_sub(target, value),
                                     lambda body, target=sub: self.bus.send(target['target'], body, topic='imap', tag=target['id']),
                                     int(self.wall()), target=sub['target'], authorize=authorize)
                except BusError as error:
                    if authority_failed:
                        self.authority_errors += 1
                        self.error = self.subs.last_authority_error
                        if isinstance(error, UnexpectedAnswer) and 'daemon_too_old' not in self.notified:
                            print(json.dumps({'method': 'forward', 'error_class': 'daemon_too_old'}), file=sys.stderr)
                            self.notified.add('daemon_too_old')
                        unavailable.add(sub['id'])
                        continue
                    self.state_errors += 1
                    self.references_known = False
                    raise
                changed = True
                self.sweep()
            if not changed:
                break

    def tick(self):
        now = self.clock()
        if now < self.next_run:
            return
        self.next_run = now + 60
        if now - self.period >= 60:
            self.period, self.arrivals, self.attempts = now, 0, 0
        deadline = self.clock() + 2  # Soft slice; each wire op has its own alarm.
        try:
            self.load_subs()
            if not self.values:
                self.source.close()
                self.sweep()
                self.error = 'invalid subscription state' if self.damaged else None
                return
            snapshot = self.source.snapshot()
            state = self.current_progress(snapshot)
            self.sweep()
            self.error = 'invalid subscription state' if self.damaged else None
            self.deliver_ready(deadline)
            if now < self.next_poll and self.load_scan(state) is None:
                return
            self.next_poll = now + 60
            windows = 0
            uidnext = snapshot[2]
            if self.load_scan(state) is None:
                earliest = min(sub['ledger']['start'] for sub in self.values)
                if state['observe'] < min(earliest, uidnext):
                    state = progress.fast_forward(state, min(earliest, uidnext),
                                                  [sub['ledger']['start'] for sub in self.values], [])
                    self.set(PROGRESS, state)
            while state['observe'] < uidnext and self.arrivals < 100 and windows < 10 and self.clock() < deadline:
                scan = self.load_scan(state)
                if scan is not None:
                    state, complete = self.discover_uid(state, deadline)
                    if not complete:
                        self.next_run = self.clock() + 1
                        break
                    continue
                start = state['observe']
                end = min(start + wire.WINDOW - 1, uidnext - 1)
                found = set(self.source.search(start, end))
                windows += 1
                while state['observe'] <= end and self.clock() < deadline:
                    number = state['observe']
                    if number in found:
                        if self.arrivals >= 100:
                            break
                        state, complete = self.discover_uid(state, deadline)
                        if not complete:
                            self.next_run = self.clock() + 1
                            break
                    else:
                        next_uid = min((uid for uid in found if uid > number), default=end + 1)
                        state = progress.advance_absent(state, next_uid, start=start, end=end,
                                                        uidnext=uidnext, found=found)
                        self.set(PROGRESS, state)
                if self.load_scan(state) is not None:
                    break
            self.backlog = max(0, uidnext - state['observe'])
            self.deliver_ready(deadline)
            self.sweep()
            if self.backlog and self.arrivals < 100:
                self.next_run = self.clock() + 1
            due = [entry['due'] for sub in self.values if sub['ledger']['pause'] is None
                   for entry in sub['ledger']['entries'].values() if entry['state'] == 'retrying']
            if due and self.attempts < 100:
                self.next_run = min(self.next_run, self.clock() + max(1, min(due) - self.wall()))
        except RetryScan:
            self.error = 'mail fetch retry pending'
        except (BusError, OSError, ValueError, KeyError, TypeError, AttributeError) as error:
            if isinstance(error, OutboxError):
                self.outbox_failures += 1
            self.error = ('forwarding state invalid' if isinstance(error, progress.ProgressError)
                          else str(error) if isinstance(error, SubscriptionError)
                          else 'bus unavailable' if isinstance(error, BusError)
                          else 'outbox unavailable' if isinstance(error, OutboxError)
                          else 'IMAP ' + error.code if isinstance(error, SessionError)
                          else 'mailbox unavailable')
            self.source.close()

    def status(self):
        values, damaged = self.values, self.damaged
        paused = sum(bool(sub['ledger']['pause']) for sub in values)
        held = sum(len(ledger.retained(sub['ledger'])) for sub in values)
        skipped = sum(sub['ledger']['skipped'] for sub in values)
        discarded = sum(sub['ledger']['discarded'] for sub in values)
        word = 'degraded' if self.error or damaged or paused or self.subs.needs_disable or self.missing_bodies else 'ok'
        return (f"{word}: {self.error or ('manual cache only' if not values else '60 s polling')}; "
                f"subscriptions={len(values)} paused={paused} held={held} backlog={self.backlog} "
                f"skipped={skipped} discarded={discarded} "
                f"repaired={self.bodies.repaired} missing_bodies={self.missing_bodies} "
                f"authority_unavailable={self.authority_errors} outbox_errors={self.outbox_failures} "
                f"view_age_s={max(0, int(self.clock() - self.view_at)) if self.view_at is not None else 'unknown'}")

    def wait(self):
        return max(.05, min(10, self.next_run - self.clock()))

    def close(self):
        self.source.close()
        self.bodies.close()
