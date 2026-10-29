"""Persistence ordering for one private IMAP delivery; no authority adapter.

execute requires the author's fresh RD141 admission check as a callback. A
single main-thread driver serialises put/ledger commit and never sweeps between
them. The receiver's normal ACL still applies to the actual IMAP bus send.
"""
import copy

from agent_bus.errors import BusError, BusTimeout, Refusal, TransportError, UncertainOutcome
import ledger
from outbox import CapacityError, MissingBody, filename
from publication import PublicationError, forwarded


def queue(value, uid, raw, store, creator, sub_id, secrets, save):
    ledger.validate(value)
    ledger.integer(uid, 1)
    if uid < value['floor'] or str(uid) in value['entries']:
        return copy.deepcopy(value)
    try:
        forwarded(raw, creator, sub_id, secrets)
    except PublicationError as error:
        result = ledger.record(value, uid, skip='oversize' if error.code == 'oversize' else 'malformed')
        save(result)
        return result
    result = ledger.record(value, uid, len(raw))
    if result['entries'][str(uid)]['state'] == 'pending':
        try:
            store.put(filename(value['identity'], value['validity'], uid), raw)
        except CapacityError:
            # Only this subscription is intake-paused; don't stop discovery.
            blocked = copy.deepcopy(value)
            blocked['intake'] = 'backlog'
            result = ledger.record(blocked, uid, skip='backlog')
    save(result)  # Body publish+fsync precede this commit, never the reverse.
    return result


def execute(value, uid, store, creator, sub_id, secrets, save, send, now, *, target, authorize):
    """Persist marker -> one send -> persist outcome, never retry in this call.

    A save failure propagates. In particular an accepted send followed by a
    failed outcome commit leaves sending on disk; recover then pauses it.
    Control exceptions (SIGTERM's Shutdown) propagate with the marker intact.
    authorize(creator,target) must return a bool; errors propagate unsent and
    unchanged. send(body) must return Bus.send's nonempty message-id string.
    """
    if uid not in ledger.ready(value, now):
        raise ledger.LedgerError('delivery is not ready')
    try:
        raw = store.get(filename(value['identity'], value['validity'], uid))
    except MissingBody:
        result = ledger.abandon(value, uid, 'fetch')
        save(result)
        return result
    try:
        body = forwarded(raw, creator, sub_id, secrets)
    except PublicationError as error:
        result = ledger.abandon(value, uid, 'oversize' if error.code == 'oversize' else 'malformed')
        save(result)
        return result
    allowed = authorize(creator, target)
    if type(allowed) is not bool:
        raise ValueError('author admission check did not return a boolean')
    if not allowed:
        result = ledger.deny(value, uid)
        save(result)
        return result
    marker = ledger.begin_attempt(value, uid, now)
    save(marker)
    try:
        answer = send(body)
        outcome = 'accepted' if isinstance(answer, str) and answer else 'unconfirmed'
    except Refusal as error:
        outcome = ('acl' if error.status in (401, 403) else 'missing' if error.status == 404
                   else 'refused' if 400 <= error.status < 500 else 'unconfirmed')
    except UncertainOutcome:
        outcome = 'unconfirmed'
    except (BusTimeout, TransportError):
        outcome = 'refused'  # SDK asserts these failed before transmitting.
    except (BusError, TimeoutError, OSError, ValueError, KeyError, TypeError):
        outcome = 'unconfirmed'
    result = ledger.finish(marker, uid, outcome, now)
    save(result)
    return result
