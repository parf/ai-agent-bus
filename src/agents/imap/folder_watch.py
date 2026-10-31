"""Serialize per-folder forwarding through the existing at-most-once engine."""
import contextlib
import hashlib
import json
import time

from agent_bus.errors import Refusal, BusError
import folders
import progress
from forwarding import Forwarder, PROGRESS, SCAN, BLOCK, selected
from publication import MAX_OUTPUT

REGISTRY = 'imap.folder.'
FIELDS = ('count', 'damaged', 'next_run', 'next_poll', 'period', 'arrivals', 'error',
          'state_errors', 'sent', 'dropped', 'warnings', 'fetch_tries', 'view_at', 'backlog')


def remember(bus, name, folder, identity):
    key = REGISTRY + hashlib.sha256(folder.encode()).hexdigest()
    bus.record(name).kv.set(key, {'folder': folder, 'identity': identity}, kind='json', mode='set')


def recover(bus, name, connection, cfg, user):
    """Name pre-step-5 synced folders from a read-only LIST, never re-sync."""
    kv = bus.record(name).kv
    names = [row.get('name', '') for row in kv.list().get('json', []) if isinstance(row, dict)]
    pending = {key[len(PROGRESS) + 1:] for key in names if key.startswith(PROGRESS + '.')}
    if not pending: return
    for key in names:
        if key.startswith(REGISTRY):
            try: entry = kv.get(key, kind='json')
            except Refusal as error:
                if error.status == 404: continue
                raise
            if isinstance(entry, dict) and isinstance(entry.get('identity'), str):
                pending.discard(entry['identity'])
    if not pending: return
    for row in folders.listing(connection):
        if not row['selectable']: continue
        folder = cfg['mailbox'] if folders.same(row['name'], cfg['mailbox']) else row['name']
        if folder.casefold() == 'inbox' and not folders.same(folder, cfg['mailbox']): folder = 'INBOX'
        identity = hashlib.sha256(json.dumps([cfg['host'], cfg['port'], user, folder], ensure_ascii=False).encode()).hexdigest()
        if identity in pending: remember(bus, name, folder, identity)


class FolderKV:
    def __init__(self, kv):
        self.kv, self.suffix = kv, ''

    def key(self, key):
        return key + self.suffix if key in (PROGRESS, SCAN, BLOCK) else key

    def get(self, key, **options): return self.kv.get(self.key(key), **options)
    def set(self, key, value, **options): return self.kv.set(self.key(key), value, **options)
    def delete(self, key, **options): return self.kv.delete(self.key(key), **options)
    def list(self): return self.kv.list()


class FolderForwarder(Forwarder):
    def __init__(self, bus, name, work, source, clock=time.monotonic, wall=time.time, send_one=None):
        super().__init__(bus, name, work, source, clock, wall, send_one)
        try:
            self.default_folder = source.mailbox_name()
        except BaseException:
            super().close()
            raise
        self.active = self.default_folder
        self.kv = FolderKV(self.kv)
        self.slots = {self.active: {'source': source, 'state': self.saved(), 'identity': None}}
        self.cursor = 0
        self.watching = set()
        self.subs.selector_matches = self.matches

    def saved(self):
        return {key: getattr(self, key) for key in FIELDS}

    def row_folder(self, row):
        return row['selector'].get('folder', self.default_folder)

    def matches(self, value, event):
        return (folders.same(value.get('folder', self.default_folder), self.active)
                and selected(value, dict(event, folder=self.active)))

    def all_rows(self):
        return self.subs._load()

    def rows(self):
        rows, damaged = self.all_rows()
        rows = {key: row for key, row in rows.items() if folders.same(self.row_folder(row), self.active)}
        self.count, self.damaged = len(rows), len(damaged)
        self.view_at = self.clock()
        return rows, damaged

    def slot(self, folder):
        folder = self.default_folder if folders.same(folder, self.default_folder) else folder
        if folder.casefold() == 'inbox' and not folders.same(folder, self.default_folder): folder = 'INBOX'
        if folder not in self.slots:
            source = self.slots[self.default_folder]['source'].for_folder(folder)
            state = dict(self.slots[self.default_folder]['state'])
            state.update(count=0, damaged=0, next_run=0, next_poll=0, period=self.clock(), arrivals=0,
                         error=None, state_errors=0, sent=0, dropped=0, warnings={}, fetch_tries={},
                         view_at=None, backlog=0)
            self.slots[folder] = {'source': source, 'state': state, 'identity': None}
        return folder, self.slots[folder]

    @contextlib.contextmanager
    def using(self, folder):
        old, suffix = self.active, self.kv.suffix
        self.slots[old]['state'] = self.saved()
        name, slot = self.slot(folder)
        self.active, self.source = name, slot['source']
        self.kv.suffix = '' if name == self.default_folder else '.' + (slot['identity'] or 'uninitialized')
        for key, value in slot['state'].items(): setattr(self, key, value)
        try:
            yield slot
        finally:
            slot['state'] = self.saved()
            previous = self.slots[old]
            self.active, self.source, self.kv.suffix = old, previous['source'], suffix
            for key, value in previous['state'].items(): setattr(self, key, value)

    def snapshot(self):
        snapshot = self.source.snapshot()
        self.slots[self.active]['identity'] = snapshot[0]
        self.kv.suffix = '' if self.active == self.default_folder else '.' + snapshot[0]
        return snapshot

    def current_progress(self, snapshot, **options):
        # Parent tick/add call source.snapshot themselves; establish the exact
        # same per-folder key as manual sync before any progress read/write.
        self.slots[self.active]['identity'] = snapshot[0]
        self.kv.suffix = '' if self.active == self.default_folder else '.' + snapshot[0]
        recover(self.bus, self.name, self.source.connection, self.source.settings(), self.source.environment.get('IMAP_USERNAME', ''))
        return super().current_progress(snapshot, **options)

    def add(self, message, target, filter_text='', folder=None):
        with self.using(folder or self.default_folder):
            # The Session resolves an explicit name with LIST and rejects a
            # missing/Noselect folder before baseline or subscription writes.
            snapshot = self.snapshot()
            rows, damaged = self.rows()
            state = self.current_progress(snapshot, first_add=not rows and not damaged)
            start = {'validity': snapshot[1], 'from_uid': progress.subscription_start(state, snapshot[2])}
            if folder is not None: start['folder'] = self.source.cfg['mailbox']
            value = self.subs.add(message, target, start, filter_text)
            remember(self.bus, self.name, self.active, snapshot[0])
            self.next_run = self.next_poll = 0
            return value

    def delete(self, message, sub_id):
        rows, _ = self.all_rows()
        row = rows.get(sub_id)
        self.subs.delete(message, sub_id)
        if row is not None:
            with self.using(self.row_folder(row)):
                remaining, damaged = self.rows()
                if not remaining and not damaged:
                    if self.slots[self.active]['identity'] is None:
                        # Resolve the persisted descriptor, never open a mailbox
                        # merely to remove a subscription during an outage.
                        for entry in self.registered():
                            if folders.same(entry['folder'], self.active):
                                self.kv.suffix = '' if self.active == self.default_folder else '.' + entry['identity']
                    for key in (PROGRESS, SCAN, BLOCK): self.kv.delete(key, kind='json')
                    self.source.close()
                self.next_run = 0
        return True

    def text(self, message):
        lines = super().text(message).splitlines()
        visible = self.subs.list(message)
        by_id = {row['id']: row for row in visible if isinstance(row.get('selector'), dict)}
        return '\n'.join(line + (' folder=' + json.dumps(self.row_folder(by_id[line.split()[0]]), ensure_ascii=False)
                                 if line.split()[0] in by_id else '') for line in lines)

    def registered(self):
        entries = []
        listing = self.kv.list()
        for item in listing.get('json', []):
            key = item.get('name', '') if isinstance(item, dict) else ''
            if not key.startswith(REGISTRY): continue
            try: value = self.kv.get(key, kind='json')
            except Refusal as error:
                if error.status == 404: continue
                raise
            if (not isinstance(value, dict) or set(value) != {'folder', 'identity'}
                    or not isinstance(value['identity'], str) or len(value['identity']) != 64):
                raise ValueError('invalid folder registry')
            # Validation is shared with explicit calls, including byte bounds.
            from wire import mailbox
            mailbox(value['folder'])
            entries.append(value)
        return entries

    def watched(self):
        rows, damaged = self.all_rows()
        if damaged: raise ValueError('invalid subscription state; folder changes refused')
        return {self.row_folder(row) for row in rows.values()}

    def tick(self):
        rows, _ = self.all_rows()
        watching = sorted({self.row_folder(row) for row in rows.values()})
        self.watching = set(watching)
        if not watching:
            for slot in self.slots.values(): slot['source'].close()
            return
        deadline = self.clock() + 2
        ordered = watching[self.cursor % len(watching):] + watching[:self.cursor % len(watching)]
        for name in ordered:
            with self.using(name):
                try:
                    if self.source.idle_ready():
                        if self.source.finish_idle(): self.next_run = self.next_poll = 0
                    super().tick()
                    if not self.error and not self.backlog: self.source.start_idle()
                except (OSError, ValueError):
                    self.error = 'mailbox unavailable'
                    self.source.close()
                    self.next_run = self.clock() + 60
            self.cursor = (watching.index(name) + 1) % len(watching)
            if self.clock() >= deadline: break

    def status(self):
        names = {self.default_folder}
        try:
            names.update(row['folder'] for row in self.registered())
            names.update(self.watched())
        except (BusError, ValueError):
            return 'degraded: folder state unavailable'
        lines = []
        for name in sorted(names):
            with self.using(name):
                mode = 'IDLE' if self.source.idle_tag is not None else 'polling'
                lines.append('folder=' + json.dumps(name, ensure_ascii=False) + ' ' + super().status() + '; ' + mode)
        answer = '\n'.join(lines)
        if len(answer.encode()) > MAX_OUTPUT:
            return 'degraded: folder status exceeds the 262144-byte answer cap'
        return answer

    def wait(self):
        if not self.watching: return 10
        slots = [self.slot(name)[1] for name in self.watching]
        if any(slot['source'].idle_tag is not None for slot in slots): return 1
        return max(.05, min(10, min(slot['state']['next_run'] for slot in slots) - self.clock()))

    def close(self):
        for slot in self.slots.values(): slot['source'].close()
        super().close()
