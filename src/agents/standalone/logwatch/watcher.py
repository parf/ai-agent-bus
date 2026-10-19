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
    if not isinstance(raw, dict) or set(raw) - {'sources', 'excludes', 'ring_size', 'rescan_interval', 'journal'}:
        raise ValueError('Configuration takes sources, excludes, ring_size, rescan_interval and journal')
    result = {'sources': DEFAULT_SOURCES, 'excludes': DEFAULT_EXCLUDES, 'ring_size': 2000,
              'rescan_interval': 600, 'journal': []} | raw
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


class Ring:
    def __init__(self, size=2000, clock=time.time):
        self.size = size
        self.lines = deque(maxlen=size)
        self.total = 0
        self.marks = {}
        self.clock = clock
        self.lock = threading.RLock()
        self.on_line = lambda line: None

    def add(self, source, text):
        with self.lock:
            self.total += 1
            entry = {'id': self.total, 'at': self.clock(), 'source': source, 'text': clean_line(text)}
            self.lines.append(entry)
        self.on_line(entry)
        return entry

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
                        end = file.tell(); start = end; data = b''
                        while start and data.count(b'\n') <= 10:
                            amount = min(start, 65536); start -= amount; file.seek(start)
                            data = file.read(amount) + data
                        lines = data.split(b'\n')
                        if lines and lines[-1] == b'': lines.pop()
                        for line in lines[-10:]: self.record(name, line.decode('utf8', 'replace'))
                    file.seek(0, 2)
                    self.files[name] = {'file': file, 'inode': stat.st_ino, 'partial': b''}
                    row.update(state='reading', error='')
                except OSError as error:
                    row.update(state='unreadable', error=error.strerror or type(error).__name__)
            self.scanned = time.monotonic()
            self.started = True

    def record(self, source, text):
        entry = self.ring.add(source, text)
        with self.lock:
            row = self.info.setdefault(source, {'source': source, 'lines': 0, 'last': ''})
            row.update(lines=row['lines'] + 1, last=entry['text'], state='reading', error='')

    def journal_reader(self):
        process = self.journal
        try:
            for raw in process.stdout:
                if raw.strip():
                    try: self.journal_lines.put_nowait(raw)
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
            self.journal = subprocess.Popen(argv, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
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
                    if stat.st_size < file.tell(): file.seek(0); state['partial'] = b''
                    data = state['partial'] + file.read(65536)
                    lines = data.split(b'\n'); state['partial'] = lines.pop()
                    for line in lines: self.record(name, line.decode('utf8', 'replace'))
                except OSError as error:
                    self.info[name].update(state='unreadable', error=error.strerror)
            self.start_journal()
            for _ in range(1000):
                try: raw = self.journal_lines.get_nowait()
                except queue.Empty: break
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
