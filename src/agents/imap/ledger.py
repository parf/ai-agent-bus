"""Pure, bounded per-subscription delivery state (RD138/RD140).

Every returned change must be persisted before acting on it. In particular,
begin_attempt's sending marker commits before bus.send. Nothing here sends,
decides authority, stores mail bodies, or resets damaged state implicitly.
"""
import copy
import json

from progress import MAX_UID, ProgressError, integer

MAX_PENDING = 64
MAX_BYTES = 16 * 1024 * 1024
MAX_VALUE = 96 * 1024
MAX_ENTRIES = 128
MAX_COUNTER = 2**63 - 1
STATES = {'pending', 'sending', 'retrying', 'sent', 'skipped', 'paused'}
REASONS = {'backlog', 'unconfirmed', 'acl', 'oversize', 'malformed', 'ambiguous',
           'fetch', 'unmatched', 'regex', 'reset'}
TERMINAL = {'sent', 'skipped'}
FIELDS = {'version', 'identity', 'validity', 'start', 'floor', 'pause', 'entries',
          'sent', 'skipped', 'skipped_first', 'skipped_last', 'discarded', 'resets'}
ENTRY_FIELDS = {'state', 'bytes', 'attempts', 'due', 'reason'}


class LedgerError(ProgressError):
    pass


def encoded(value):
    return json.dumps(value, separators=(',', ':'), ensure_ascii=False).encode()


def retained(value):
    return {int(uid): entry['bytes'] for uid, entry in value['entries'].items()
            if entry['state'] not in TERMINAL}


def validate(value):
    if not isinstance(value, dict) or set(value) != FIELDS or type(value['version']) is not int or value['version'] != 1:
        raise LedgerError('invalid subscription delivery state')
    if not isinstance(value['identity'], str) or not 1 <= len(value['identity']) <= 512:
        raise LedgerError('invalid subscription mailbox identity')
    integer(value['validity'], 1)
    integer(value['start'], 1, MAX_UID + 1)
    integer(value['floor'], value['start'], MAX_UID + 1)
    if value['pause'] is not None and (not isinstance(value['pause'], str) or value['pause'] not in {'backlog', 'acl', 'unconfirmed', 'regex'}):
        raise LedgerError('invalid subscription pause reason')
    entries = value['entries']
    if not isinstance(entries, dict) or len(entries) > MAX_ENTRIES:
        raise LedgerError('subscription delivery entry cap exceeded')
    for uid, entry in entries.items():
        if not isinstance(uid, str) or len(uid) > 10 or not uid.isascii() or not uid.isdigit() or uid != str(int(uid)):
            raise LedgerError('invalid subscription delivery UID')
        integer(int(uid), value['start'])
        if not isinstance(entry, dict) or set(entry) != ENTRY_FIELDS or not isinstance(entry['state'], str) or entry['state'] not in STATES:
            raise LedgerError('invalid subscription delivery entry')
        integer(entry['bytes'], 0, 2 * 1024 * 1024)
        integer(entry['attempts'], 0, MAX_COUNTER)
        integer(entry['due'], 0, MAX_COUNTER)
        if entry['reason'] is not None and (not isinstance(entry['reason'], str) or entry['reason'] not in REASONS):
            raise LedgerError('invalid subscription delivery reason')
        if entry['state'] in TERMINAL and entry['bytes'] != 0:
            raise LedgerError('terminal delivery retains a body reference')
        if entry['state'] in {'sending', 'retrying', 'sent', 'paused'} and entry['attempts'] < 1:
            raise LedgerError('attempted delivery lacks its sending marker')
        if entry['state'] in {'pending', 'skipped'} and entry['attempts'] != 0:
            raise LedgerError('unattempted delivery has an attempt count')
        if (entry['state'] == 'retrying') != (entry['due'] > 0):
            raise LedgerError('delivery backoff does not match its state')
        if entry['state'] in {'pending', 'sending', 'retrying', 'sent'} and entry['reason'] is not None:
            raise LedgerError('delivery state has an unexpected reason')
        if entry['state'] == 'skipped' and entry['reason'] is None:
            raise LedgerError('skipped delivery lacks its reason')
        if entry['state'] == 'paused' and (entry['reason'] not in {'acl', 'unconfirmed'} or value['pause'] is None):
            raise LedgerError('paused delivery lacks its visible reason')
    holds = retained(value)
    if len(holds) > MAX_PENDING or sum(holds.values()) > MAX_BYTES:
        raise LedgerError('subscription pending delivery budget exceeded')
    for field in ('sent', 'skipped', 'discarded', 'resets'):
        integer(value[field], 0, MAX_COUNTER)
    first, last = value['skipped_first'], value['skipped_last']
    if value['skipped']:
        integer(first, 1)
        integer(last, first)
    elif first is not None or last is not None:
        raise LedgerError('empty skipped counter has a UID range')
    if len(encoded(value)) > MAX_VALUE:
        raise LedgerError('subscription delivery value exceeds 96 KiB')
    return value


def new(identity, validity, start):
    return validate({'version': 1, 'identity': identity, 'validity': validity,
                     'start': start, 'floor': start, 'pause': None, 'entries': {},
                     'sent': 0, 'skipped': 0, 'skipped_first': None,
                     'skipped_last': None, 'discarded': 0, 'resets': 0})


def record(value, uid, size=0, skip=None):
    """Idempotently record one discovery; caller persists before advancing O.

    floor compacts terminal knowledge only after O was durably advanced. A
    rediscovery below floor or with any existing state is therefore a no-op.
    Pauses hold existing pending work but count and skip later matching mail.
    """
    validate(value)
    integer(uid, 1)
    integer(size, 0, 2 * 1024 * 1024)
    if skip is not None and (not isinstance(skip, str) or skip not in REASONS):
        raise LedgerError('invalid discovery skip reason')
    result = copy.deepcopy(value)
    if uid < result['floor'] or str(uid) in result['entries']:
        return result
    if len(result['entries']) >= MAX_ENTRIES:
        raise LedgerError('compact durable discovery outcomes before adding more')
    holds = retained(result)
    if skip is None and result['pause'] is not None:
        skip = result['pause']
    if skip is None and (len(holds) >= MAX_PENDING or sum(holds.values()) + size > MAX_BYTES):
        result['pause'], skip = 'backlog', 'backlog'
    state = 'pending' if skip is None else 'skipped'
    result['entries'][str(uid)] = {'state': state, 'bytes': size if skip is None else 0,
                                 'attempts': 0, 'due': 0, 'reason': skip}
    if skip is not None and skip != 'unmatched':
        result['skipped'] += 1
        result['skipped_first'] = min(result['skipped_first'] or uid, uid)
        result['skipped_last'] = max(result['skipped_last'] or uid, uid)
    return validate(result)


def ready(value, now):
    validate(value)
    integer(now, 0, MAX_COUNTER)
    if value['pause'] is not None or any(entry['state'] == 'sending' for entry in value['entries'].values()):
        return []
    return sorted(int(uid) for uid, entry in value['entries'].items()
                  if entry['state'] in {'pending', 'retrying'} and entry['due'] <= now)


def begin_attempt(value, uid, now):
    """Return the sending marker that MUST commit before the external send."""
    integer(uid, 1)
    if uid not in ready(value, now):
        raise LedgerError('delivery is paused, terminal, already sending or not due')
    result = copy.deepcopy(value)
    entry = result['entries'][str(uid)]
    entry.update(state='sending', attempts=entry['attempts'] + 1, due=0, reason=None)
    return validate(result)


def finish(value, uid, outcome, now):
    """Classify an accepted send, definite temporary refusal, ACL or unknown.

    Only the bus adapter may choose an outcome; arbitrary exception text never
    enters this state. ACL refusal is a permanent pause under RD140, not retry.
    """
    validate(value)
    integer(uid, 1)
    integer(now, 0, MAX_COUNTER)
    if not isinstance(outcome, str) or outcome not in {'accepted', 'refused', 'acl', 'unconfirmed'}:
        raise LedgerError('invalid delivery outcome')
    result = copy.deepcopy(value)
    entry = result['entries'].get(str(uid))
    if entry is None or entry['state'] != 'sending':
        raise LedgerError('delivery completion needs a committed sending marker')
    if outcome == 'accepted':
        entry.update(state='sent', bytes=0)
        result['sent'] += 1
    elif outcome == 'refused':
        entry.update(state='retrying', due=now + min(10 * 2**min(entry['attempts'] - 1, 5), 300))
    else:
        entry.update(state='paused', reason=outcome)
        result['pause'] = outcome
    return validate(result)


def recover(value):
    """A crash in sending is uncertain; retrying is explicitly NOT uncertain."""
    validate(value)
    result = copy.deepcopy(value)
    for entry in result['entries'].values():
        if entry['state'] == 'sending':
            entry.update(state='paused', reason='unconfirmed')
            result['pause'] = 'unconfirmed'
    return validate(result)


def compact(value, durable_observe):
    """Caller commits global O first, then removes only older terminal entries.

    Recorded pending/sending/retrying/paused entries below O remain retained.
    A rollback of O after this commit must fail closed in the parent loader.
    """
    validate(value)
    integer(durable_observe, 1, MAX_UID + 1)
    if durable_observe < value['start']:
        return copy.deepcopy(value)  # A newly added sub can start ahead of O.
    integer(durable_observe, value['floor'], MAX_UID + 1)
    result = copy.deepcopy(value)
    result['floor'] = durable_observe
    result['entries'] = {uid: entry for uid, entry in result['entries'].items()
                         if int(uid) >= durable_observe or entry['state'] not in TERMINAL}
    return validate(result)


def mailbox_reset(value, identity, validity, start):
    """Explicit real mailbox reset, with visible loss counts; not error repair."""
    validate(value)
    if value['identity'] == identity and value['validity'] == validity:
        raise LedgerError('mailbox reset requires a changed identity or UIDVALIDITY')
    result = new(identity, validity, start)
    # Prior skipped UIDs become historical loss counts, not ranges belonging
    # to the new mailbox's UID namespace. Do not silently erase those losses.
    result['discarded'] = value['discarded'] + len(retained(value)) + value['skipped']
    result['sent'] = value['sent']
    result['resets'] = value['resets'] + 1
    return validate(result)
