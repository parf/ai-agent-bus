"""Private, rebuildable UID cache. No provider or bus dependencies."""
import fcntl
import json
import os
from pathlib import Path
import re
import stat
import shutil
import time
import uuid

MiB = 1 << 20
MAX_MAIL = 2 * MiB
MAX_ENTRIES = 1000
META_RESERVE = MiB
UID_FILE = re.compile(r'[1-9][0-9]{0,9}\.eml\Z')
TEMP_FILE = re.compile(r'\.(?:body|state)-[0-9a-f]{32}\Z')


class CacheError(ValueError):
    pass


def uid(value):
    if isinstance(value, bool) or not isinstance(value, (str, int)) or not re.fullmatch(r'[1-9][0-9]{0,9}', str(value)) or int(value) > 4294967295:
        raise CacheError('UID must be an integer in 1..4294967295')
    return int(value)


class Cache:
    def __init__(self, work, identity, validity, budget=64 * MiB, *, require_same=False):
        if type(budget) is not int or not 4 * MiB <= budget <= 512 * MiB:
            raise CacheError('cache_bytes must be in 4194304..536870912')
        self.budget, self.directory, self.lock = budget, None, None
        self.repaired = 0
        path = Path(work) / 'mail-cache'
        try:
            os.mkdir(path, 0o700)
        except FileExistsError:
            pass
        try:
            self.directory = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            os.fchmod(self.directory, 0o700)
            self.repair_special('.lock', None)
            self.lock = self.open('.lock', os.O_RDWR | os.O_CREAT)
            fcntl.flock(self.lock, fcntl.LOCK_EX)
            if os.fstat(self.lock).st_size:
                os.ftruncate(self.lock, 0)
                self.repaired += 1
            # A killed atomic write may leave one temporary file; never follow it.
            self.cleanup_temporary()
            self.repair_special('.state', 16384)
            try:
                handle = self.open('.state', os.O_RDONLY)
                with os.fdopen(handle, 'rb') as stream:
                    raw = stream.read(16385)
                if len(raw) > 16384:
                    raise ValueError()
                state = json.loads(raw)
                if not isinstance(state, dict) or set(state) not in ({'identity', 'validity', 'cursor'}, {'identity', 'validity', 'cursor', 'retry'}) or not isinstance(state['identity'], str) or type(state['validity']) is not int or not 1 <= state['validity'] <= 4294967295 or type(state['cursor']) is not int or not 0 <= state['cursor'] <= 4294967295:
                    raise ValueError()
                retry = state.setdefault('retry', None)
                if retry is not None and (not isinstance(retry, dict) or set(retry) != {'uid', 'count'} or type(retry['uid']) is not int or not 1 <= retry['uid'] <= 4294967295 or type(retry['count']) is not int or not 1 <= retry['count'] <= 3):
                    raise ValueError()
            except FileNotFoundError:
                state = None
            except ValueError:
                self.repaired += 1
                state = None  # Cache corruption cannot become the only copy of mail.
            key = {'identity': identity, 'validity': uid(validity)}
            mismatch = state is not None and any(state.get(k) != v for k, v in key.items())
            if require_same and (mismatch or (state is None and self.files())):
                raise CacheError('mailbox cache epoch changed or is unknown; sync or read --no-mark before marking')
            if state is None or mismatch:
                for name, _, _ in self.files():
                    os.unlink(name, dir_fd=self.directory)
                state = {**key, 'cursor': 0, 'retry': None}
            self.state = state
            self.trim(0)
            self.save_state()
        except BaseException:
            self.close()
            raise

    def open(self, name, flags):
        handle = os.open(name, flags | os.O_NOFOLLOW, 0o600, dir_fd=self.directory)
        if not stat.S_ISREG(os.fstat(handle).st_mode):
            os.close(handle)
            raise CacheError('cache entry is not a regular file')
        os.fchmod(handle, 0o600)
        return handle

    def discard(self, name, info):
        if stat.S_ISDIR(info.st_mode):
            shutil.rmtree(name, dir_fd=self.directory)  # fd-based, never follow symlinks.
        else:
            os.unlink(name, dir_fd=self.directory)
        self.repaired += 1

    def repair_special(self, name, maximum):
        try:
            info = os.stat(name, dir_fd=self.directory, follow_symlinks=False)
        except FileNotFoundError:
            return
        if not stat.S_ISREG(info.st_mode) or (maximum is not None and info.st_size > maximum):
            self.discard(name, info)

    def cleanup_temporary(self):
        with os.scandir(self.directory) as entries:
            for entry in entries:
                if TEMP_FILE.fullmatch(entry.name):
                    self.discard(entry.name, entry.stat(follow_symlinks=False))

    def files(self):
        found = []
        with os.scandir(self.directory) as entries:
            for entry in entries:
                name = entry.name
                if name in {'.lock', '.state'} or TEMP_FILE.fullmatch(name):
                    continue
                info = entry.stat(follow_symlinks=False)
                if not UID_FILE.fullmatch(name) or int(name[:-4]) > 4294967295 or not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= MAX_MAIL:
                    self.discard(name, info)
                    continue
                found.append((name, info.st_size, info.st_mtime_ns))
                # Bound enumeration memory even if a damaged directory has too many UIDs.
                if len(found) > MAX_ENTRIES:
                    oldest = min(found, key=lambda item: (item[2], item[0]))
                    os.unlink(oldest[0], dir_fd=self.directory)
                    found.remove(oldest)
                    self.repaired += 1
        return found

    def trim(self, incoming):
        entries = sorted(self.files(), key=lambda item: (item[2], item[0]))
        total = sum(item[1] for item in entries)
        while entries and (total + incoming > self.budget - META_RESERVE or len(entries) + bool(incoming) > MAX_ENTRIES):
            name, size, _ = entries.pop(0)
            os.unlink(name, dir_fd=self.directory)
            total -= size
        if total + incoming > self.budget - META_RESERVE:
            raise CacheError('mail exceeds cache byte budget')

    def atomic(self, name, raw, prefix):
        temporary = '.' + prefix + '-' + uuid.uuid4().hex
        handle = self.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL)
        try:
            with os.fdopen(handle, 'wb') as stream:
                stream.write(raw)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, name, src_dir_fd=self.directory, dst_dir_fd=self.directory)
            os.fsync(self.directory)
        finally:
            try:
                os.unlink(temporary, dir_fd=self.directory)
            except FileNotFoundError:
                pass

    def save_state(self):
        raw = json.dumps(self.state, separators=(',', ':')).encode()
        if len(raw) > 16384:
            raise CacheError('cache state exceeds 16384 bytes')
        self.atomic('.state', raw, 'state')

    def has(self, number):
        name = str(uid(number)) + '.eml'
        try:
            handle = self.open(name, os.O_RDONLY)
        except FileNotFoundError:
            return False
        try:
            if not 0 < os.fstat(handle).st_size <= MAX_MAIL:
                raise CacheError('cached mail exceeds its byte cap')
            return True
        finally:
            os.close(handle)

    def get(self, number, *, touch=True):
        name = str(uid(number)) + '.eml'
        handle = self.open(name, os.O_RDONLY)
        with os.fdopen(handle, 'rb') as stream:
            raw = stream.read(MAX_MAIL + 1)
            if not 0 < len(raw) <= MAX_MAIL:
                raise CacheError('cached mail exceeds its byte cap')
            if touch:
                now = time.time_ns()
                os.utime(stream.fileno(), ns=(now, now))
        return raw

    def touch(self, number):
        handle = self.open(str(uid(number)) + '.eml', os.O_RDONLY)
        try:
            now = time.time_ns()
            os.utime(handle, ns=(now, now))
        finally:
            os.close(handle)

    def put(self, number, raw):
        if not isinstance(raw, bytes) or not raw or len(raw) > MAX_MAIL:
            raise CacheError('mail must contain 1..2097152 raw bytes')
        if self.has(number):
            return  # Never replace a complete cached UID with another fetch.
        self.trim(len(raw))
        self.atomic(str(uid(number)) + '.eml', raw, 'body')

    def remove(self, number):
        try:
            os.unlink(str(uid(number)) + '.eml', dir_fd=self.directory)
        except FileNotFoundError:
            pass

    def close(self):
        if self.lock is not None:
            os.close(self.lock)
            self.lock = None
        if self.directory is not None:
            os.close(self.directory)
            self.directory = None

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
