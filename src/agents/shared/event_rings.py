"""Independent event rings; alert snapshots and reserved ids survive restart."""
from collections.abc import Mapping
import copy
from datetime import datetime, timezone
import fcntl
import json
import math
import os
from pathlib import Path
import re
import stat
import tempfile
import time
import unicodedata

LEVELS = ('alert', 'warning', 'notice')
DEFAULTS = {'notice': {'size': 200, 'ttl': 86400}, 'warning': {'size': 200, 'ttl': 604800}, 'alert': {'size': 50, 'ttl': 2592000}}
NAME = re.compile(r'[A-Za-z0-9][A-Za-z0-9._-]{0,63}\Z')
SAFE = 2**53-1
BLOCK = 64
MAX_NAMES = 10000
MAX_STATE = 64*1024*1024
ANSI = re.compile(r'\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\))')


class RingError(ValueError):
    def __init__(self, code):
        self.code = code
        super().__init__(code)


def integer(value, low, high, code='invalid_request'):
    if type(value) is not int or not low <= value <= high:
        raise RingError(code)
    return value


def name(value):
    if not isinstance(value, str) or not NAME.fullmatch(value):
        raise RingError('invalid_log')
    return value


def level(value):
    if value not in LEVELS:
        raise RingError('invalid_level')
    return value


def timestamp(value):
    if type(value) not in (int, float) or not math.isfinite(value) or not 0 <= value < 253402300799:
        raise RingError('invalid_clock')
    ms = math.floor(value*1000)
    return datetime.fromtimestamp(ms//1000, timezone.utc).strftime('%Y-%m-%dT%H:%M:%S.')+f'{ms%1000:03d}Z'


def seconds(value):
    if not isinstance(value, str) or not re.fullmatch(r'[1-9][0-9]{0,7}[smhd]', value):
        raise RingError('invalid_config')
    result = int(value[:-1])*{'s': 1, 'm': 60, 'h': 3600, 'd': 86400}[value[-1]]
    if result > 366*86400:
        raise RingError('invalid_config')
    return result


def config(value):
    if not isinstance(value, dict) or set(value)-{'rings', 'logs', 'max_logs'}:
        raise RingError('invalid_config')
    maximum = integer(value.get('max_logs', 100), 1, 100, 'invalid_config')
    def override(base, values):
        if not isinstance(values, dict) or set(values)-set(LEVELS):
            raise RingError('invalid_config')
        result = copy.deepcopy(base)
        for key, fields in values.items():
            if not isinstance(fields, dict) or set(fields)-{'size', 'ttl'}:
                raise RingError('invalid_config')
            if 'size' in fields:
                result[key]['size'] = integer(fields['size'], 1, 500, 'invalid_config')
            if 'ttl' in fields:
                result[key]['ttl'] = seconds(fields['ttl'])
        if sum(row['size'] for row in result.values()) > 500:
            raise RingError('invalid_config')
        return result
    defaults = override(DEFAULTS, value.get('rings', {}))
    logs = value.get('logs', {})
    if not isinstance(logs, dict) or len(logs) > 100:
        raise RingError('invalid_config')
    overrides = {}
    for key, fields in logs.items():
        try: name(key)
        except RingError: raise RingError('invalid_config') from None
        overrides[key] = override(defaults, fields)
    return maximum, defaults, overrides


def line(value):
    if not isinstance(value, str):
        raise RingError('invalid_text')
    try: value.encode('utf-8')
    except UnicodeError: raise RingError('invalid_text') from None
    value = ANSI.sub('', value)
    return ''.join(c for c in value if unicodedata.category(c) not in ('Cc', 'Cf', 'Cs', 'Zl', 'Zp'))[:200]


def strict_json(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result: raise ValueError('duplicate')
            result[key] = value
        return result
    def invalid(_): raise ValueError('nonfinite')
    return json.loads(raw, object_pairs_hook=pairs, parse_constant=invalid)


def epoch(value):
    if not isinstance(value, str) or not re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z', value):
        raise ValueError('timestamp')
    return datetime.fromisoformat(value.replace('Z', '+00:00')).timestamp()


class Rings:
    def __init__(self, directory, configuration=None, *, clock=time.time, lock_timeout=2):
        self.maximum, self.defaults, self.overrides = config({} if configuration is None else configuration)
        self.clock = clock
        self.directory = Path(directory)
        self.directory.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.path = self.directory/'events.json'
        self.lockfile = None
        self.poisoned = False
        self.logs = {}
        self.next_ids = {}
        self.last_clock = 0
        self.backwards = False
        try:
            fd = os.open(self.directory/'events.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
            self.lockfile = os.fdopen(fd, 'r+b')
            if not stat.S_ISREG(os.fstat(fd).st_mode): raise RingError('storage_invalid')
            until = time.monotonic()+lock_timeout
            while True:
                try: fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB); break
                except BlockingIOError:
                    if time.monotonic() >= until: raise RingError('storage_locked') from None
                    time.sleep(.01)
            self._load()
        except Exception as error:
            self.close()
            if isinstance(error, RingError): raise
            raise RingError('storage_invalid') from None

    def _load(self):
        try:
            fd = os.open(self.path, os.O_RDONLY | os.O_NOFOLLOW)
        except FileNotFoundError:
            return
        try:
            with os.fdopen(fd, 'rb') as source:
                info = os.fstat(source.fileno())
                if not stat.S_ISREG(info.st_mode) or info.st_size > MAX_STATE: raise ValueError()
                value = strict_json(source.read(MAX_STATE+1))
            if not isinstance(value, dict) or set(value) != {'v', 'logs'} or type(value['v']) is not int or value['v'] != 1: raise ValueError()
            records = value['logs']
            if not isinstance(records, dict) or len(records) > MAX_NAMES: raise ValueError()
            for key, record in records.items():
                name(key)
                if not isinstance(record, dict) or set(record) != {'high', 'active', 'alerts'}: raise ValueError()
                high = integer(record['high'], BLOCK, SAFE, 'storage_invalid')
                if high % BLOCK or type(record['active']) is not bool: raise ValueError()
                alerts = record['alerts']
                if not isinstance(alerts, list) or len(alerts) > 500 or (not record['active'] and alerts): raise ValueError()
                ids = set()
                for entry in alerts:
                    if not isinstance(entry, dict) or set(entry) != {'id', 'level', 'text', 'at', 'last_at', 'count'}: raise ValueError()
                    integer(entry['id'], 1, high, 'storage_invalid');integer(entry['count'], 1, SAFE, 'storage_invalid')
                    if entry['id'] in ids or entry['level'] != 'alert' or line(entry['text']) != entry['text']: raise ValueError()
                    ids.add(entry['id'])
                    if epoch(entry['last_at']) < epoch(entry['at']): raise ValueError()
                self.logs[key] = {'high': high, 'active': record['active'], 'rings': {'alert': sorted(alerts, key=lambda e: e['id']), 'warning': [], 'notice': []}}
                self.next_ids[key] = high+1
            self.last_clock = max((epoch(e['last_at']) for row in self.logs.values() for e in row['rings']['alert']), default=0)
            if sum(r['active'] for r in self.logs.values()) > 100: raise ValueError()
        except Exception:
            raise RingError('storage_invalid') from None

    def _snapshot(self, logs):
        return {'v': 1, 'logs': {key: {'high': row['high'], 'active': row['active'], 'alerts': row['rings']['alert']} for key, row in logs.items()}}

    def _commit(self, logs):
        """Never publish state until replace AND directory fsync finish."""
        self._check()
        if self._snapshot(logs) == self._snapshot(self.logs):
            self.logs = logs
            return
        pending = None
        try:
            raw = json.dumps(self._snapshot(logs), ensure_ascii=False, allow_nan=False, separators=(',', ':')).encode()
            if len(raw) > MAX_STATE: raise ValueError()
            fd, pending = tempfile.mkstemp(prefix='.events-', dir=self.directory)
            with os.fdopen(fd, 'wb') as output:
                os.fchmod(output.fileno(), 0o600)
                output.write(raw);output.flush();os.fsync(output.fileno())
            os.replace(pending, self.path)
            pending = None
            fd = os.open(self.directory, os.O_RDONLY | os.O_DIRECTORY)
            try: os.fsync(fd)
            finally: os.close(fd)
        except Exception:
            self.poisoned = True
            raise RingError('storage_unknown') from None
        finally:
            if pending is not None:
                try: os.unlink(pending)
                except FileNotFoundError: pass
        self.logs = logs

    def _check(self):
        if self.lockfile is None or self.poisoned: raise RingError('storage_unknown')

    def _policy(self, key):
        return self.overrides.get(key, self.defaults)

    def _sweep(self):
        self._check()
        now = self.clock();timestamp(now)
        self.backwards = now < self.last_clock
        self.last_clock = max(self.last_clock, now)
        logs = copy.deepcopy(self.logs)
        for key, record in logs.items():
            for severity in LEVELS:
                rule = self._policy(key)[severity]
                record['rings'][severity] = [e for e in record['rings'][severity] if now-epoch(e['last_at']) < rule['ttl']][-rule['size']:]
        self._commit(logs)
        return now

    def _get(self, key):
        name(key)
        if key not in self.logs or not self.logs[key]['active']: raise RingError('not_found')
        return self.logs[key]

    def write(self, key, severity, text):
        name(key);level(severity);text = line(text)
        now = self._sweep();at = timestamp(now)
        logs = copy.deepcopy(self.logs)
        old = logs.get(key)
        if old is None:
            if len(logs) >= MAX_NAMES: raise RingError('name_limit')
            old = {'high': 0, 'active': False, 'rings': {s: [] for s in LEVELS}}
            logs[key] = old
        if not old['active']:
            if sum(r['active'] for r in logs.values()) >= self.maximum: raise RingError('log_limit')
            old['active'] = True
        entries = old['rings'][severity]
        folded = next((e for e in reversed(entries) if not self.backwards and e['text'] == text and epoch(e['last_at']) <= now and 0 <= now-epoch(e['at']) < 60), None)
        if folded is not None:
            folded['count'] = min(SAFE, folded['count']+1);folded['last_at'] = at
            entry = folded
        else:
            next_id = self.next_ids.get(key, 1)
            if next_id > SAFE-BLOCK: raise RingError('id_limit')
            if next_id > old['high']: old['high'] = next_id+BLOCK-1
            entry = {'id': next_id, 'level': severity, 'text': text, 'at': at, 'last_at': at, 'count': 1}
            entries.append(entry)
            del entries[:-self._policy(key)[severity]['size']]
        self._commit(logs)
        if folded is None: self.next_ids[key] = entry['id']+1
        return copy.deepcopy(entry)

    def _counts(self, row): return {s: len(row['rings'][s]) for s in LEVELS}

    def _summary(self, key, row):
        return {'log': key, 'counts': self._counts(row), 'latest_alert_at': max((e['last_at'] for e in row['rings']['alert']), default=None)}

    def read(self, key, severity=None):
        if severity is not None: level(severity)
        now = self._sweep();row = self._get(key)
        return {**self._summary(key, row), 'rings': {s: copy.deepcopy(list(reversed(row['rings'][s]))) for s in LEVELS if severity is None or s == severity}, 'now': timestamp(now)}

    def list(self):
        now = self._sweep()
        return {'logs': [self._summary(key, row) for key, row in sorted(self.logs.items()) if row['active']], 'now': timestamp(now)}

    def pull(self, key, severity=None, count=100):
        if severity is not None: level(severity)
        integer(count, 1, 500)
        now = self._sweep();self._get(key)
        logs = copy.deepcopy(self.logs);row = logs[key]
        entries = sorted((e for s in LEVELS if severity is None or s == severity for e in row['rings'][s]), key=lambda e: e['id'])[:count]
        ids = {e['id'] for e in entries}
        for s in LEVELS: row['rings'][s] = [e for e in row['rings'][s] if e['id'] not in ids]
        self._commit(logs)
        return {'log': key, 'entries': copy.deepcopy(entries), 'remaining': sum(self._counts(row).values()), 'now': timestamp(now)}

    def clean(self, key, severity=None, upper=None):
        if severity is not None: level(severity)
        if upper is not None: integer(upper, 1, SAFE)
        self._sweep();self._get(key)
        logs = copy.deepcopy(self.logs);row = logs[key];before = sum(self._counts(row).values())
        for s in LEVELS:
            if severity is None or severity == s:
                row['rings'][s] = [e for e in row['rings'][s] if upper is not None and e['id'] > upper]
        if severity is None and upper is None: row['active'] = False
        self._commit(logs)
        return {'log': key, 'removed': before-sum(self._counts(row).values()), 'counts': self._counts(row)}

    def close(self):
        if self.lockfile is not None:
            self.lockfile.close();self.lockfile = None
