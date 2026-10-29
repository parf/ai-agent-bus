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
    def __init__(self, message, code='invalid'):
        super().__init__(message)
        self.code = code


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
        if uidnext < previous['observe'] or uidnext < previous['history_ceiling'] + 1:
            raise ProgressError('UIDNEXT regressed without a UIDVALIDITY change; forwarding paused', 'regressed')
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
    listing; a partial crash replay must finish that snapshot first. Walk one
    UID at a time, including absent UIDs proved by a bounded explicit SEARCH.
    """
    validate(progress)
    integer(uid, progress['observe'], progress['observe'])
    if not isinstance(required_subscriptions, set) or not isinstance(recorded_subscriptions, set):
        raise ProgressError('discovery requires subscription sets')
    if not required_subscriptions <= recorded_subscriptions:
        raise ProgressError('discovery still has unrecorded subscription outcomes')
    result = copy.deepcopy(progress)
    result['observe'] = uid + 1
    return validate(result)


def backfill_step(progress, uid):
    """Complete exactly the next historical UID (cached/skipped/proved absent).

    The caller's bounded SEARCH window establishes absence; it must pass every
    UID in descending order, never jump over an unprocessed gap.
    """
    validate(progress)
    integer(uid, max(1, progress['history']), progress['history'])
    result = copy.deepcopy(progress)
    result['history'] = uid - 1
    if result['retry'] is not None and result['retry']['uid'] == uid:
        result['retry'] = None
    return validate(result)


def fast_forward(progress, start, other_starts, pending_uids):
    """Skip pre-subscription discovery only when no older work needs it.

    The caller supplies every other subscription's S and every held UID. This
    does not discard entries or bodies; it refuses rather than guess coverage.
    """
    validate(progress)
    integer(start, progress['baseline'], MAX_UID + 1)
    if not isinstance(other_starts, (list, tuple)) or not isinstance(pending_uids, (list, tuple)):
        raise ProgressError('fast-forward requires all existing subscription starts and pending UIDs')
    if len(other_starts) > 100 or len(pending_uids) > 6400:
        raise ProgressError('fast-forward inputs exceed subscription bounds')
    for value in other_starts:
        integer(value, progress['baseline'], MAX_UID + 1)
    for value in pending_uids:
        integer(value, 1)
    if any(value < start for value in [*other_starts, *pending_uids]):
        raise ProgressError('older subscriptions or pending deliveries still need discovery')
    result = copy.deepcopy(progress)
    result['observe'] = max(result['observe'], start)
    return validate(result)


def renew_history(progress, uidnext):
    validate(progress)
    integer(uidnext, 1, MAX_UID + 1)
    if progress['history']:
        raise ProgressError('manual history still has an unfinished pass')
    result = copy.deepcopy(progress)
    result['history'] = result['history_ceiling'] = uidnext - 1
    return validate(result)
