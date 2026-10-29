"""Instance-private, reference-counted-by-the-caller mail bodies for pending sends.

One immutable file per mailbox identity/UIDVALIDITY/UID. The driver derives
keep names from all valid ledgers and sweeps at start, after deletion/reset,
and after finishing deliveries. Never sweep if a damaged ledger makes that
reference set unknown. This store never evicts a referenced body for capacity.
"""
import fcntl
import hashlib
import os
from pathlib import Path
import re
import stat
import uuid

from progress import MAX_UID, integer

MAX_BODY = 2 * 1024 * 1024
MAX_BYTES = 64 * 1024 * 1024
MAX_FILES = 1024
BODY = re.compile(r'[0-9a-f]{64}\.[1-9][0-9]{0,9}\.eml\Z')
TEMP = re.compile(r'\.tmp-[0-9a-f]{32}\Z')


class OutboxError(ValueError):
    pass


class CapacityError(OutboxError):
    pass


def filename(identity, validity, uid):
    if not isinstance(identity, str) or not 1 <= len(identity) <= 512:
        raise OutboxError('invalid outbox mailbox identity')
    integer(validity, 1)
    integer(uid, 1)
    epoch = hashlib.sha256((identity + '\0' + str(validity)).encode()).hexdigest()
    return epoch + '.' + str(uid) + '.eml'


def check_name(name):
    if not isinstance(name, str) or not BODY.fullmatch(name) or int(name.split('.')[1]) > MAX_UID:
        raise OutboxError('invalid outbox body reference')


class Outbox:
    def __init__(self, work):
        self.directory = self.lock = None
        try:
            path = Path(work) / 'mail-outbox'
            try:
                os.mkdir(path, 0o700)
            except FileExistsError:
                pass
            self.directory = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            if os.fstat(self.directory).st_uid != os.getuid():
                raise OutboxError('outbox directory has a different owner')
            os.fchmod(self.directory, 0o700)
            self.lock = self.open('.lock', os.O_RDWR | os.O_CREAT)
            fcntl.flock(self.lock, fcntl.LOCK_EX)
            if os.fstat(self.lock).st_size:
                raise OutboxError('invalid outbox lock file')
            with os.scandir(self.directory) as entries:
                for entry in entries:
                    if TEMP.fullmatch(entry.name):
                        os.unlink(entry.name, dir_fd=self.directory)
            os.fsync(self.directory)
        except BaseException:
            self.close()
            raise

    def open(self, name, flags):
        handle = os.open(name, flags | os.O_NOFOLLOW, 0o600, dir_fd=self.directory)
        info = os.fstat(handle)
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid():
            os.close(handle)
            raise OutboxError('outbox entry is not an own regular file')
        try:
            os.fchmod(handle, 0o600)
        except BaseException:
            os.close(handle)
            raise
        return handle

    def listing(self):
        found, total = {}, 0
        with os.scandir(self.directory) as entries:
            for entry in entries:
                if entry.name == '.lock':
                    continue
                check_name(entry.name)
                handle = self.open(entry.name, os.O_RDONLY)
                try:
                    size = os.fstat(handle).st_size
                finally:
                    os.close(handle)
                if not 0 < size <= MAX_BODY:
                    raise OutboxError('invalid outbox body size')
                found[entry.name] = size
                total += size
                if len(found) > MAX_FILES or total > MAX_BYTES:
                    raise CapacityError('outbox is over its body/count budget')
        return found

    def get(self, name):
        check_name(name)
        handle = self.open(name, os.O_RDONLY)
        with os.fdopen(handle, 'rb') as stream:
            size = os.fstat(stream.fileno()).st_size
            if not 0 < size <= MAX_BODY:
                raise OutboxError('invalid outbox body size')
            raw = stream.read(MAX_BODY + 1)
        if len(raw) != size or len(raw) > MAX_BODY:
            raise OutboxError('outbox body changed or was truncated')
        return raw

    def put(self, name, raw):
        check_name(name)
        if not isinstance(raw, bytes) or not 0 < len(raw) <= MAX_BODY:
            raise OutboxError('invalid outbox body size')
        existing = self.listing()
        if name in existing:
            return False  # UID bodies are immutable within this mailbox epoch.
        if len(existing) >= MAX_FILES or sum(existing.values()) + len(raw) > MAX_BYTES:
            raise CapacityError('outbox has no capacity for this new body')
        temporary = '.tmp-' + uuid.uuid4().hex
        handle = self.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL)
        try:
            with os.fdopen(handle, 'wb') as stream:
                stream.write(raw)
                stream.flush()
                os.fsync(stream.fileno())
            # Atomic exclusive publish: unlike rename, never overwrites a UID
            # that appeared after the capacity check. It cannot follow a link.
            os.link(temporary, name, src_dir_fd=self.directory,
                    dst_dir_fd=self.directory, follow_symlinks=False)
            os.unlink(temporary, dir_fd=self.directory)
            os.fsync(self.directory)
            return True
        finally:
            try:
                os.unlink(temporary, dir_fd=self.directory)
            except FileNotFoundError:
                pass

    def sweep(self, keep):
        """Remove only unreferenced bodies; reject an unknown reference set."""
        if not isinstance(keep, set):
            raise OutboxError('outbox sweep requires the complete reference set')
        for name in keep:
            check_name(name)
        removed = 0
        with os.scandir(self.directory) as entries:
            for entry in entries:
                if entry.name == '.lock':
                    continue
                check_name(entry.name)
                if entry.name not in keep:
                    # unlink never follows a symlink, even for a damaged orphan.
                    os.unlink(entry.name, dir_fd=self.directory)
                    removed += 1
        os.fsync(self.directory)
        return removed

    def close(self):
        for handle in (self.lock, self.directory):
            if handle is not None:
                os.close(handle)
        self.lock = self.directory = None

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
