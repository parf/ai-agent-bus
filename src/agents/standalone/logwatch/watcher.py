"""Bounded arrival-ordered log history and file/journal source monitoring."""
from collections import deque
from datetime import datetime, timezone
import fnmatch
import glob
import json
import os
from pathlib import Path
import queue
import re
import subprocess
import threading
import time

from cleaning import clean_line

DEFAULT_SOURCES = ['/rd/var/log/**/*.log', 'syslog', '/var/log/nginx/*error*log']
DEFAULT_EXCLUDES = ['/rd/var/log/realmo.com_ai-query.log', '/rd/var/log/cron/realmo.com_ai-query.log',
 '/rd/var/log/realmo.com_ai-search-query.log', '/rd/var/log/cron/realmo.com_ai-search-query.log',
 '/rd/var/log/realmo.com_ai-*.log', '/rd/var/log/cron/realmo.com_ai-*.log',
 '/rd/var/log/realmo.com_realmo-ai-property-type.log', '/rd/var/log/realmo.com_realmo-ai-*.log',
 '/rd/var/log/cron/realmo.com_realmo-ai-*.log', '/var/log/nginx/paperhouses-error.log']
UNIT = re.compile(r'[A-Za-z0-9][A-Za-z0-9_.:@-]*\.(?:service|socket|timer|target|mount|path|scope|slice|automount|swap|device)\Z')


def configuration(raw):
    raw = {} if raw is None else raw
    if not isinstance(raw, dict) or set(raw) - {'sources', 'excludes', 'ring_size', 'rescan_interval', 'journal', 'line_bytes', 'ring_bytes', 'startup_bytes'}:
        raise ValueError('Configuration takes sources, excludes, ring_size, rescan_interval, journal, line_bytes, ring_bytes and startup_bytes')
    result = {'sources': DEFAULT_SOURCES, 'excludes': DEFAULT_EXCLUDES, 'ring_size': 2000,
              'rescan_interval': 600, 'journal': [], 'line_bytes': 16384,
              'ring_bytes': 8388608, 'startup_bytes': 65536} | raw
    for key, low, high in [('line_bytes',128,1048576), ('ring_bytes',128,67108864), ('startup_bytes',128,1048576)]:
        if type(result[key]) is not int or not low <= result[key] <= high:
            raise ValueError(f'{key} must be an integer from {low} to {high}')
    for key in ['sources', 'excludes']:
        if not isinstance(result[key], list) or not all(isinstance(p, str) and p and '\x00' not in p and (p.startswith('/') or key == 'sources' and p == 'syslog') for p in result[key]):
            raise ValueError(key + ' must be a list of absolute patterns' + (' or syslog' if key == 'sources' else ''))
    if type(result['ring_size']) is not int or not 1 <= result['ring_size'] <= 100000:
        raise ValueError('ring_size must be an integer from 1 to 100000')
    if type(result['rescan_interval']) not in (int, float) or not .05 <= result['rescan_interval'] <= 86400:
        raise ValueError('rescan_interval must be seconds from 0.05 to 86400')
    journal = result['journal']
    if journal != 'all' and (not isinstance(journal, list) or not all(isinstance(u, str) and UNIT.fullmatch(u) for u in journal)):
        raise ValueError('journal must be all or a list of exact unit names')
    return result


def bounded_text(text, limit, omitted=0):
    raw = clean_line(text).encode('utf8')
    if not omitted and len(raw) <= limit:
        return raw.decode('utf8')
    keep = min(len(raw), limit)
    while True:
        head = raw[:keep].decode('utf8', 'ignore')
        marker = f'…[truncated {omitted + len(raw) - len(head.encode())} bytes]'
        available = limit - len(marker.encode())
        if keep <= available:
            return head + marker
        keep = available


class Partial:
    """Keep the head while counting discarded bytes until the next newline."""
    def __init__(self, limit):
        self.limit, self.head, self.omitted = limit, b'', 0

    def add(self, raw):
        count = min(len(raw), self.limit - len(self.head))
        self.head += raw[:count]
        self.omitted += len(raw) - count


class Ring:
    def __init__(self, size=2000, clock=time.time, line_bytes=16384, ring_bytes=8388608):
        self.size = size
        self.line_bytes, self.ring_bytes, self.bytes = line_bytes, ring_bytes, 0
        self.lines = deque()
        self.total = 0
        self.marks = {}
        self.clock = clock
        self.lock = threading.RLock()
        self.on_line = lambda line: None

    def add(self, source, text, omitted=0):
        with self.lock:
            self.total += 1
            # Bound input before cleanup (which can expand invalid/control bytes).
            raw = text.encode('utf8')
            entry = {'id': self.total, 'at': self.clock(), 'source': source,
                     'text': bounded_text(raw[:self.line_bytes].decode('utf8','replace'), self.line_bytes,
                                          omitted + max(0,len(raw)-self.line_bytes))}
            self.lines.append(entry)
            self.bytes += self.entry_bytes(entry)
            while len(self.lines) > self.size or self.bytes > self.ring_bytes:
                self.bytes -= self.entry_bytes(self.lines.popleft())
        self.on_line(entry)
        return entry

    @staticmethod
    def entry_bytes(entry):
        return len(entry['source'].encode()) + len(entry['text'].encode())

    def snapshot(self):
        with self.lock:
            return list(self.lines)

    def mark(self, label=None):
        with self.lock:
            label = label or f'mark-{self.total}-{time.time_ns()}'
            self.marks.pop(label, None)
            self.marks[label] = self.total
            while len(self.marks) > self.size:
                del self.marks[next(iter(self.marks))]
            return label

    def after(self, label):
        with self.lock:
            if label not in self.marks:
                raise ValueError('unknown mark ' + label)
            mark = self.marks[label]
            if self.lines and mark < self.lines[0]['id'] - 1:
                raise ValueError('mark is older than the retained ring; make a new mark')
            return [line for line in self.lines if line['id'] > mark]


class Monitor:
    def __init__(self, config, ring, log=print):
        self.config, self.ring, self.log = config, ring, log
        self.files = {}
        self.info = {}
        self.excluded = []
        self.started = False
        self.scanned = 0
        self.journal = None
        self.journal_thread = None
        self.journal_started = 0
        self.journal_error = ''
        self.journal_lines = queue.Queue(maxsize=1000)
        self.lock = threading.RLock()

    @staticmethod
    def syslog():
        return [p for p in ['/var/log/messages', '/var/log/syslog'] if Path(p).is_file()]

    def rescan(self):
        with self.lock:
            found = set()
            for pattern in self.config['sources']:
                found.update(self.syslog() if pattern == 'syslog' else glob.glob(pattern, recursive=True))
            self.excluded = sorted(p for p in found if any(fnmatch.fnmatchcase(p, x) for x in self.config['excludes']))
            effective = found - set(self.excluded)
            for name in list(self.files):
                if name not in effective:
                    self.files.pop(name)['file'].close()
                    self.info[name]['state'] = 'removed'
            for name in sorted(effective):
                if name in self.files:
                    continue
                row = self.info.setdefault(name, {'source': name, 'lines': 0, 'last': ''})
                try:
                    stat = os.stat(name)
                    if not stat.st_size or not Path(name).is_file():
                        row['state'] = 'empty'
                        continue
                    file = open(name, 'rb')
                    if not self.started:
                        # tail-all initially reads ten lines; later discoveries start at EOF.
                        file.seek(0, 2)
                        start = max(0, file.tell() - self.config['startup_bytes'])
                        file.seek(start)
                        data = file.read(self.config['startup_bytes'])
                        if start: data = data.partition(b'\n')[2]  # omit a cut first line
                        lines = data.split(b'\n')
                        if lines and lines[-1] == b'': lines.pop()
                        for line in lines[-10:]: self.record(name, line.decode('utf8', 'replace'))
                    file.seek(0, 2)
                    self.files[name] = {'file': file, 'inode': stat.st_ino, 'partial': Partial(self.config['line_bytes'])}
                    row.update(state='reading', error='')
                except OSError as error:
                    row.update(state='unreadable', error=error.strerror or type(error).__name__)
            self.scanned = time.monotonic()
            self.started = True

    def record(self, source, text, omitted=0):
        entry = self.ring.add(source, text, omitted)
        with self.lock:
            row = self.info.setdefault(source, {'source': source, 'lines': 0, 'last': ''})
            row.update(lines=row['lines'] + 1, last=entry['text'], state='reading', error='')

    def journal_reader(self):
        process = self.journal
        try:
            partial = Partial(self.config['line_bytes'])
            while raw := process.stdout.read(4096):
                pieces = raw.split(b'\n')
                for index, piece in enumerate(pieces):
                    partial.add(piece)
                    if index < len(pieces)-1:
                        try: self.journal_lines.put_nowait((partial.head, partial.omitted))
                        except queue.Full: self.log('journal reader queue full; line dropped')
                        partial = Partial(self.config['line_bytes'])
            if partial.head:
                try: self.journal_lines.put_nowait((partial.head, partial.omitted))
                except queue.Full: self.log('journal reader queue full; line dropped')
        finally:
            process.stdout.close()
            process.wait()

    def start_journal(self):
        units = self.config['journal']
        if not units: return
        if self.journal and self.journal.poll() is None: return
        if self.journal and time.monotonic() - self.journal_started < 5: return
        argv = ['journalctl', '-f', '-o', 'json', '-n', '0', '--no-pager']
        if units != 'all':
            for unit in units: argv += ['-u', unit]
        try:
            # stderr joins the reader so permission failures are visible, never silently absent.
            self.journal = subprocess.Popen(argv, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
            self.journal_started = time.monotonic()
            self.journal_thread = threading.Thread(target=self.journal_reader, daemon=True)
            self.journal_thread.start()
        except OSError as error:
            self.journal_error = str(error)

    def tick(self):
        if not self.started or time.monotonic() - self.scanned >= self.config['rescan_interval']:
            self.rescan()
        with self.lock:
            for name, state in list(self.files.items()):
                try:
                    file = state['file']; stat = os.stat(name)
                    if stat.st_ino != state['inode']:
                        file.close(); del self.files[name]; self.scanned = 0
                        continue
                    if stat.st_size < file.tell(): file.seek(0); state['partial'] = Partial(self.config['line_bytes'])
                    pieces = file.read(65536).split(b'\n')
                    for index, piece in enumerate(pieces):
                        state['partial'].add(piece)
                        if index < len(pieces)-1:
                            partial = state['partial']
                            self.record(name, partial.head.decode('utf8','replace'), partial.omitted)
                            state['partial'] = Partial(self.config['line_bytes'])
                except OSError as error:
                    self.info[name].update(state='unreadable', error=error.strerror)
            self.start_journal()
            for _ in range(1000):
                try: raw, omitted = self.journal_lines.get_nowait()
                except queue.Empty: break
                raw = raw.decode('utf8','replace')
                if omitted:
                    self.record('journal:unknown', raw, omitted)
                    continue
                try:
                    entry = json.loads(raw)
                    unit = entry.get('_SYSTEMD_UNIT') or entry.get('SYSLOG_IDENTIFIER') or 'unknown'
                    text = entry.get('MESSAGE', '')
                    if isinstance(text, list): text = bytes(text).decode('utf8', 'replace')
                    stamp = datetime.fromtimestamp(int(entry.get('__REALTIME_TIMESTAMP', time.time()*1e6))/1e6, timezone.utc).isoformat(timespec='seconds')
                    line = f"{stamp} {entry.get('_HOSTNAME', '')} {entry.get('SYSLOG_IDENTIFIER', unit)}: {text}"
                    self.record('journal:' + unit, line)
                except (ValueError, TypeError):
                    self.journal_error = clean_line(raw).strip()[:300]
            if self.journal and self.journal.poll() not in (None, 0) and not self.journal_error:
                self.journal_error = 'journalctl exited ' + str(self.journal.returncode)

    def sources(self):
        with self.lock:
            rows = [dict(value) for value in self.info.values()]
            for source in self.excluded: rows.append({'source': source, 'state': 'excluded', 'lines': 0, 'last': ''})
            journal = self.config['journal']
            for unit in (['all'] if journal == 'all' else journal):
                name = 'journal:' + unit
                if not any(row['source'] == name for row in rows):
                    rows.append({'source': name, 'state': 'unreadable' if self.journal_error else 'reading', 'lines': 0, 'last': '', 'error': self.journal_error})
            if self.journal_error:
                for row in rows:
                    if row['source'].startswith('journal:'): row.update(state='unreadable', error=self.journal_error)
            return sorted(rows, key=lambda row: row['source'])

    def close(self):
        if self.journal:
            if self.journal.poll() is None: self.journal.terminate()
            try: self.journal.wait(timeout=2)
            except subprocess.TimeoutExpired: self.journal.kill(); self.journal.wait()
        if self.journal_thread: self.journal_thread.join(timeout=2)
        for state in self.files.values(): state['file'].close()
