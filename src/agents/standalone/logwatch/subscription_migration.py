"""One-time legacy KV conversion, completed before the agent starts serving.

A checkpoint outside sub.* reserves an absent deterministic ID and retains the
exact old value and the intended new value. After an interrupted add/delete,
restart recognizes the committed row and finishes deletion before any match is
served. Existing IDs are collisions even when their content happens to match.
The agent is the sole subscription writer during startup; external KV edits
that disagree with a checkpoint fail closed without deleting the old row.
"""
import copy
import hashlib

from agent_bus.errors import BusError, Refusal
import subscriptions as shared

PREFIX = 'logwatch-migration.'
MISSING = object()
LEGACY = {'id', 'target', 'filter', 'created_by', 'created_at', 'forwarded',
          'sent', 'dropped', 'paused', 'disabled', 'last_error'}


def migration_id(source, old_key, attempt=0):
    return hashlib.sha256(f'{source}\0{old_key}\0{attempt}'.encode()).hexdigest()[:12]


def read(kv, key):
    try: return kv.get(key, kind='json')
    except Refusal as error:
        if error.status == 404: return MISSING
        raise


def keys(kv):
    listing = kv.list()
    if not isinstance(listing, dict) or not isinstance(listing.get('json', []), list):
        raise shared.SubscriptionError('store_unknown')
    return sorted(row['name'] for row in listing.get('json', [])
                  if isinstance(row, dict) and isinstance(row.get('name'), str))


def project(engine, key, value, new_id, clock):
    """Preserve damaged rows for manager repair, never fill in a missing author."""
    raw = copy.deepcopy(value)
    if not isinstance(value, dict) or value.get('id') != key[4:]: return raw
    raw['id'] = new_id
    if 'selector' in value or set(value) - LEGACY: return raw
    try:
        query = value['filter']
        if isinstance(query, str) and query and not query.startswith('/'):
            query = '/' + ''.join('\\'+c if c in r'\./-^$|?*+(){}[]' else c for c in query) + '/iu'
        target = shared.target_text(value['target'])
        if '@' not in target: target = shared.destination(engine.bus, target)
        row = {'id': new_id, 'target': target, 'selector': {}, 'filter': query,
               'created_by': shared.identity(value['created_by']),
               'created_at': value.get('created_at', shared.stamp(clock)),
               'sent': value.get('sent', value.get('forwarded', 0)), 'dropped': value.get('dropped', 0)}
        return engine._row(row, new_id)
    except (KeyError, ValueError, TypeError, UnicodeError, BusError, OSError): return raw


def checkpoint_key(old_key):
    return PREFIX + hashlib.sha256(old_key.encode()).hexdigest()[:24]


def reserve(engine, old_key, old_value, clock):
    kv = engine.kv
    for attempt in range(1024):
        new_id = migration_id(engine.source, old_key, attempt)
        if read(kv, 'sub.'+new_id) is not MISSING: continue
        checkpoint = {'old_key': old_key, 'old_value': old_value, 'new_id': new_id, 'attempt': attempt,
                      'new_value': project(engine, old_key, old_value, new_id, clock)}
        kv.set(checkpoint_key(old_key), checkpoint, kind='json', mode='add')
        return checkpoint
    raise shared.SubscriptionError('limit')


def finish(engine, marker_key, marker):
    kv = engine.kv
    if not isinstance(marker, dict) or set(marker) != {'old_key','old_value','new_id','attempt','new_value'}:
        raise shared.SubscriptionError('store_unknown')
    old_key, new_id = marker['old_key'], marker['new_id']
    if (not isinstance(old_key, str) or not old_key.startswith('sub.') or marker_key != checkpoint_key(old_key)
            or type(marker['attempt']) is not int or not 0 <= marker['attempt'] < 1024
            or new_id != migration_id(engine.source, old_key, marker['attempt']) or not shared.ID.fullmatch(new_id)):
        raise shared.SubscriptionError('store_unknown')
    old = read(kv, old_key)
    if old is not MISSING and old != marker['old_value']:
        raise shared.SubscriptionError('store_unknown')
    new_key = 'sub.'+new_id
    current = read(kv, new_key)
    if current is MISSING:
        if old is MISSING: raise shared.SubscriptionError('store_unknown')
        kv.set(new_key, marker['new_value'], kind='json', mode='add')
        current = read(kv, new_key)
    if current != marker['new_value']:
        # A changed reserved row is not ours to overwrite or coalesce.
        raise shared.SubscriptionError('store_unknown')
    # Never delete the old key before a confirmed new commit. Unknown outcomes
    # propagate: the checkpoint is retained and the agent does not serve yet.
    if old is not MISSING: kv.delete(old_key, kind='json')
    if read(kv, old_key) is not MISSING: raise shared.SubscriptionError('store_unknown')
    kv.delete(marker_key, kind='json')
    if read(kv, marker_key) is not MISSING: raise shared.SubscriptionError('store_unknown')


def migrate(engine, clock):
    kv = engine.kv
    # Resume before scanning definitions: both copies may exist after SIGKILL.
    for key in keys(kv):
        if key.startswith(PREFIX): finish(engine, key, kv.get(key, kind='json'))
    for key in keys(kv):
        if not key.startswith('sub.'): continue
        value = kv.get(key, kind='json')
        if shared.ID.fullmatch(key[4:]):
            row = project(engine, key, value, key[4:], clock)
            if row != value: kv.set(key, row, kind='json', mode='replace')
        else:
            marker = reserve(engine, key, value, clock)
            finish(engine, checkpoint_key(key), marker)
