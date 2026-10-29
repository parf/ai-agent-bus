"""One IMAP instance's subscriptions, in its own KV; daemon-owned admission."""
import copy
import datetime
import json
import time
import sys

from agent_bus.errors import BusError, Refusal, UnexpectedAnswer
import ledger
from matching import MatchError, compile_filter
from publication import new_id, validate_creator, validate_id

MAX_SUBSCRIPTIONS = 100
MAX_PER_CREATOR = 20
MAX_VALUE = 120 * 1024
FIELDS = {'id', 'target', 'created_by', 'created_at', 'filter', 'ledger'}


class SubscriptionError(ValueError):
    pass


def target_text(value):
    if not isinstance(value, str) or not 1 <= len(value) <= 64 or not value.isascii() or any(ord(c) <= 32 or ord(c) == 127 for c in value):
        raise SubscriptionError('target must be one bus name of at most 64 ASCII characters')
    return value.lower()


def validate(value):
    if not isinstance(value, dict) or set(value) != FIELDS:
        raise SubscriptionError('invalid stored subscription')
    validate_id(value['id'])
    validate_creator(value['created_by'])
    target_text(value['target'])
    if not isinstance(value['filter'], str) or len(value['filter']) > 206:
        raise SubscriptionError('invalid stored subscription filter')
    if not isinstance(value['created_at'], str) or len(value['created_at']) > 64 or any(ord(c) < 32 for c in value['created_at']):
        raise SubscriptionError('invalid stored subscription creation time')
    datetime.datetime.fromisoformat(value['created_at'])
    ledger.validate(value['ledger'])
    if len(json.dumps(value, separators=(',', ':'), ensure_ascii=False).encode()) > MAX_VALUE:
        raise SubscriptionError('stored subscription exceeds its byte budget')
    return value


def may_delete(message, value, self_name):
    if message.envelope.get('from') == value['created_by']:
        return True
    original = message.envelope.get('original_to')
    if original and original.lower() != self_name.lower():
        return False  # Forwarded roles describe another record.
    return any(role in ('owner', 'maintainer') for role in message.envelope.get('roles', ()))


def own_manager(message, self_name):
    original = message.envelope.get('original_to')
    return (not original or original.lower() == self_name.lower()) and any(
        role in ('owner', 'maintainer') for role in message.envelope.get('roles', ()))


class Subscriptions:
    def __init__(self, bus, name, baseline, cleanup=lambda: None, clock=time.time):
        self.bus, self.name, self.baseline = bus, name, baseline
        self.kv, self.cleanup, self.clock = bus.record(name).kv, cleanup, clock
        self.compiled = {}
        self.last_authority_error = None
        self.cleanup_pending = False
        self.needs_disable = set()
        self.last_destination = None

    def save(self, value, mode='replace'):
        validate(value)
        self.kv.set('sub.' + value['id'], value, kind='json', mode=mode)

    def get(self, sub_id):
        validate_id(sub_id)
        try:
            value = self.kv.get('sub.' + sub_id, kind='json')
        except Refusal as error:
            if error.status == 404:
                return None
            raise
        validate(value)
        if value['id'] != sub_id:
            raise SubscriptionError('subscription id does not match its key')
        return value

    def all(self):
        listing = self.kv.list()
        if not isinstance(listing, dict) or not isinstance(listing.get('json', []), list):
            raise SubscriptionError('invalid subscription listing')
        keys = [row.get('name') for row in listing.get('json', []) if isinstance(row, dict)
                and isinstance(row.get('name'), str) and row['name'].startswith('sub.')]
        if len(keys) > MAX_SUBSCRIPTIONS:
            raise SubscriptionError('subscription cap exceeded; forwarding stopped')
        live = {key[4:] for key in keys}
        self.compiled = {key: value for key, value in self.compiled.items() if key in live}
        values, damaged = [], {}
        self.needs_disable = set()
        for key in sorted(keys):
            try:
                value = self.get(key[4:])
                if value is None:
                    continue  # Deleted between list and get, the only absence.
                cached = self.compiled.get(value['id'])
                if cached is None or cached[0] != value['filter']:
                    try:
                        pattern = compile_filter(value['filter'])
                    except MatchError:
                        pattern = None
                    self.compiled[value['id']] = (value['filter'], pattern)
                if self.compiled[value['id']][1] is None:
                    self.needs_disable.add(value['id'])
                values.append(value)
            except ValueError:
                damaged[key] = 'invalid subscription state'
        return values, damaged

    def for_driver(self):
        """Only the serialized driver writes compile failures, never list/add."""
        values, damaged = self.all()
        for index, value in enumerate(values):
            if value['id'] in self.needs_disable and value['ledger']['intake'] != 'regex':
                value = copy.deepcopy(value)
                value['ledger'] = ledger.disable_filter(value['ledger'])
                self.save(value)
                values[index] = value
        return values, damaged

    def author_allowed(self, who, target):
        """False is a real denial; unknown target pauses, unavailable raises."""
        self.last_authority_error = None
        self.last_destination = None
        try:
            allowed = self.bus.may_send(who, target)
            if type(allowed) is not bool:
                raise UnexpectedAnswer('invalid author admission answer')
            if not allowed:
                self.last_authority_error = 'author refused'
                return False
            destination = self.bus.record(target).lookup()
            if not isinstance(destination, dict) or destination.get('kind') not in ('queue', 'agent'):
                self.last_authority_error = 'destination is not a queue or agent'
                return False
            target_text(destination.get('name'))
            self.last_destination = destination
            return True
        except Refusal as error:
            if error.status == 404:
                self.last_authority_error = 'no such destination visible to this IMAP agent'
                return False
            self.last_authority_error = 'author admission unavailable'
            raise
        except UnexpectedAnswer:
            self.last_authority_error = 'daemon does not support the author admission query'
            raise
        except BusError:
            self.last_authority_error = 'author admission unavailable'
            raise

    def add(self, message, target, filter_text=''):
        creator = message.envelope.get('from')
        validate_creator(creator)
        target = target_text(target)
        pattern = compile_filter(filter_text)  # Validate before network or KV.
        if not pattern.pattern:
            filter_text = ''  # Slash-delimited empty patterns also mean ALL mail.
        values, damaged = self.all()
        if len(values) + len(damaged) >= MAX_SUBSCRIPTIONS:
            raise SubscriptionError('at most 100 subscriptions')
        if sum(value['created_by'] == creator for value in values) >= MAX_PER_CREATOR:
            raise SubscriptionError('at most 20 subscriptions per creator')
        try:
            if not self.author_allowed(creator, target):
                raise SubscriptionError(self.last_authority_error)
            destination = self.last_destination
        except BusError:
            raise SubscriptionError(self.last_authority_error or 'author admission unavailable') from None
        target = target_text(destination.get('name'))  # Daemon's canonical name.
        identity, validity, uidnext = self.baseline()
        state = ledger.new(identity, validity, uidnext)
        for _ in range(3):
            sub_id = new_id()
            validate_id(sub_id)
            value = {'id': sub_id, 'target': target, 'created_by': creator,
                     'created_at': datetime.datetime.fromtimestamp(self.clock(), datetime.timezone.utc).isoformat(),
                     'filter': filter_text, 'ledger': state}
            try:
                self.save(value, mode='add')
                self.compiled[sub_id] = (filter_text, pattern)
                return value
            except Refusal as error:
                if error.status != 412:
                    raise
        raise SubscriptionError('could not choose a free subscription id')

    def delete(self, message, sub_id):
        validate_id(sub_id)
        try:
            value = self.get(sub_id)
        except ValueError:
            if not own_manager(message, self.name):
                raise SubscriptionError('only this IMAP Owner/Maintainers may delete damaged subscription state') from None
            value = None
            # An explicit manager deletion is also the repair path for a bad
            # ledger; its unknown held bodies become orphans only after delete.
            if not self.kv.delete('sub.' + sub_id, kind='json'):
                raise SubscriptionError('no such subscription')
            print(json.dumps({'method': 'sub.delete', 'id': sub_id, 'reason': 'invalid_state'}), file=sys.stderr)
            self.compiled.pop(sub_id, None)
            return self.after_delete()
        if value is None:
            raise SubscriptionError('no such subscription')
        if not may_delete(message, value, self.name):
            raise SubscriptionError('only the creator or this IMAP Owner/Maintainers may delete it')
        if not self.kv.delete('sub.' + sub_id, kind='json'):
            raise SubscriptionError('no such subscription')
        self.compiled.pop(sub_id, None)
        # Deletion commits first; a failed sweep leaves only bounded orphans.
        return self.after_delete()

    def after_delete(self):
        try:
            self.cleanup_pending = self.cleanup() is False
        except (OSError, ValueError, BusError):
            self.cleanup_pending = True
        return not self.cleanup_pending

    def text(self):
        values, damaged = self.all()
        lines = []
        for sub in values:
            value = sub['ledger']
            pending = ledger.retained(value)
            span = f'{min(pending)}-{max(pending)}' if pending else 'none'
            skipped = f"{value['skipped_first']}-{value['skipped_last']}" if value['skipped'] else 'none'
            intake = 'regex' if sub['id'] in self.needs_disable else value['intake']
            lines.append(f"{sub['id']} {sub['target']} by={sub['created_by']} at={sub['created_at']} filter={sub['filter']!r} "
                         f"sent={value['sent']} held={len(pending)} held_uids={span} "
                         f"skipped={value['skipped']} skipped_uids={skipped} "
                         f"pause={value['pause'] or 'none'} intake={intake or 'none'} "
                         f"intake_cleared={value['intake_cleared']} discarded={value['discarded']} "
                         f"last_reset_pause={value['last_reset_pause'] or 'none'}" +
                         ('; regex disabled until delete+add' if intake == 'regex' else ''))
            for reason, counter in sorted(value['skips'].items()):
                lines.append(f"  skipped: {reason} count={counter['count']} uids={counter['first']}-{counter['last']}")
        lines.extend(f'{key!r}: invalid subscription state; forwarding disabled' for key in sorted(damaged))
        if self.needs_disable or any(sub['ledger']['pause'] or sub['ledger']['intake'] for sub in values):
            lines.append('delete+add discards held work and skips mail received while paused')
        return '\n'.join(lines) if lines else '(no subscriptions; manual cache only)'
