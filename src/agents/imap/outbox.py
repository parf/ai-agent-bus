"""Instance-private generation lock; mail forwarding retains no pending bodies."""
import fcntl
import os
from pathlib import Path
import stat
import time


class OutboxError(ValueError):
    pass


class PreviousGeneration(OutboxError):
    pass


class Outbox:
    def __init__(self, work, lock_wait=5):
        self.directory = self.lock = None
        try:
            path = Path(work) / 'mail-outbox'
            try:
                os.mkdir(path, 0o700)
            except FileExistsError:
                pass
            self.directory = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            if os.fstat(self.directory).st_uid != os.getuid():
                raise OutboxError('generation directory has a different owner')
            self.lock = os.open('.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600, dir_fd=self.directory)
            info = os.fstat(self.lock)
            if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or info.st_nlink != 1 or info.st_size:
                raise OutboxError('unsupported generation lock state')
            deadline = time.monotonic() + lock_wait
            while True:
                try:
                    fcntl.flock(self.lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                    break
                except BlockingIOError:
                    if time.monotonic() >= deadline:
                        raise PreviousGeneration('waiting for the previous IMAP generation') from None
                    time.sleep(min(.05, max(0, deadline - time.monotonic())))
            if set(os.listdir(self.directory)) != {'.lock'}:
                raise OutboxError('unsupported generation directory contents')
            os.fchmod(self.directory, 0o700)
            os.fchmod(self.lock, 0o600)
            os.fsync(self.directory)
        except BaseException:
            self.close()
            raise

    def close(self):
        for handle in (self.lock, self.directory):
            if handle is not None:
                os.close(handle)
        self.lock = self.directory = None

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
