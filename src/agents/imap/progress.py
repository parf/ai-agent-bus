"""Pure mailbox frontiers; persistence and authority stay with the agent.

Nothing here sends, fetches, discards cached mail or treats a cache cursor as a
delivery confirmation. Invalid forwarding state is refused, never reset into
an accidental replay. The caller persists the returned snapshot atomically.
"""
import copy

MAX_UID = 4294967295
FIELDS = {'version', 'identity', 'validity', 'cursor', 'retry', 'baseline',
          'observe', 'history', 'history_ceiling'}
LEGACY = {'identity', 'validity', 'cursor'}


class ProgressError(ValueError):
    pass


def integer(value, low=0, high=MAX_UID):
    if type(value) is not int or not low <= value <= high:
        raise ProgressError('invalid mailbox progress integer')
    return value


def validate_retry(value):
    if value is None:
        return
    if not isinstance(value, dict) or set(value) != {'uid', 'count'}:
        raise ProgressError('invalid manual FETCH retry state')
    integer(value['uid'], 1)
    integer(value['count'], 1, 3)


def validate(value):
    if not isinstance(value, dict) or set(value) != FIELDS or type(value['version']) is not int or value['version'] != 1:
        raise ProgressError('unsupported mailbox progress state')
    if not isinstance(value['identity'], str) or not 1 <= len(value['identity']) <= 512:
        raise ProgressError('invalid mailbox identity')
    integer(value['validity'], 1)
    integer(value['cursor'])
    validate_retry(value['retry'])
    integer(value['baseline'], 1, MAX_UID + 1)
    integer(value['observe'], value['baseline'], MAX_UID + 1)
    integer(value['history_ceiling'])
    integer(value['history'], 0, value['history_ceiling'])
    return value


def load(previous, identity, validity, uidnext):
    """Return (new snapshot, reset), preserving old cache retry/cursor.

    First observation and a real mailbox reset establish B=O=UIDNEXT. A
    matching restored mailbox retains O even if UIDNEXT has moved. H covers
    uncached manual history, independently; old cache bodies are not touched.
    """
    if not isinstance(identity, str) or not 1 <= len(identity) <= 512:
        raise ProgressError('invalid mailbox identity')
    integer(validity, 1)
    integer(uidnext, 1, MAX_UID + 1)
    legacy = False
    if previous is not None:
        if not isinstance(previous, dict):
            raise ProgressError('invalid mailbox progress state')
        legacy = set(previous) in (LEGACY, LEGACY | {'retry'})
        if legacy:
            if not isinstance(previous['identity'], str) or not 1 <= len(previous['identity']) <= 512:
                raise ProgressError('invalid legacy mailbox identity')
            integer(previous['validity'], 1)
            integer(previous['cursor'])
            validate_retry(previous.get('retry'))
        else:
            validate(previous)
    same = previous is not None and previous['identity'] == identity and previous['validity'] == validity
    if same and not legacy:
        return copy.deepcopy(previous), False
    value = {'version': 1, 'identity': identity, 'validity': validity,
             'cursor': previous['cursor'] if same else 0,
             'retry': copy.deepcopy(previous.get('retry')) if same else None,
             'baseline': uidnext, 'observe': uidnext,
             'history': uidnext - 1, 'history_ceiling': uidnext - 1}
    return validate(value), previous is not None and not same


def subscription_start(progress, uidnext):
    validate(progress)
    integer(uidnext, 1, MAX_UID + 1)
    return max(progress['baseline'], uidnext)


def eligible(progress, start, uid):
    validate(progress)
    integer(start, progress['baseline'], MAX_UID + 1)
    integer(uid, 1)
    return uid >= start and uid >= progress['observe']


def advance_observation(progress, uid, required_subscriptions, recorded_subscriptions):
    """Only advance after every discovered subscription outcome is durable.

    The caller passes the fixed discovery snapshot, not a fresh subscription
    listing; a partial crash replay must finish that snapshot first. Gaps before
    uid must already be proved absent by a bounded explicit UID SEARCH range.
    """
    validate(progress)
    integer(uid, progress['observe'])
    if not isinstance(required_subscriptions, set) or not isinstance(recorded_subscriptions, set):
        raise ProgressError('discovery requires subscription sets')
    if not required_subscriptions <= recorded_subscriptions:
        raise ProgressError('discovery still has unrecorded subscription outcomes')
    result = copy.deepcopy(progress)
    result['observe'] = uid + 1
    return validate(result)


def backfill_step(progress, uid):
    validate(progress)
    integer(uid, 1, progress['history'])
    result = copy.deepcopy(progress)
    result['history'] = uid - 1
    if result['retry'] is not None and result['retry']['uid'] == uid:
        result['retry'] = None
    return validate(result)


def renew_history(progress, uidnext):
    validate(progress)
    integer(uidnext, 1, MAX_UID + 1)
    if progress['history']:
        raise ProgressError('manual history still has an unfinished pass')
    result = copy.deepcopy(progress)
    result['history'] = result['history_ceiling'] = uidnext - 1
    return validate(result)
