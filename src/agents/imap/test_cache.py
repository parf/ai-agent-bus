import json
import os
from pathlib import Path
import stat
import tempfile
import subprocess
import sys
import unittest
from unittest.mock import patch

import cache

HERE = Path(__file__).resolve().parent


class CacheTests(unittest.TestCase):
    def setUp(self):
        root = HERE.parents[2] / 'tmp'
        root.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix='imap-cache-', dir=root)
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)

    def open(self, identity='account-a', validity=1, budget=4 * cache.MiB):
        return cache.Cache(self.work, identity, validity, budget)

    def test_restart_reuses_uid_and_invalidity_or_account_drops_it(self):
        with self.open() as store:
            store.put(7, b'Subject: A\r\n\r\noriginal')
            store.state['cursor'] = 7
            store.save_state()
        with self.open() as store:
            self.assertTrue(store.has(7))
            self.assertEqual(store.state['cursor'], 7)
            store.put(7, b'replacement must never overwrite a cached UID')
            self.assertEqual(store.get(7), b'Subject: A\r\n\r\noriginal')
        with self.open(validity=2) as store:
            self.assertFalse(store.has(7))
            self.assertEqual(store.state['cursor'], 0)
            store.put(7, b'new incarnation')
        with self.open(identity='account-b', validity=2) as store:
            self.assertFalse(store.has(7))

    def test_byte_eviction_reserves_space_before_temporary_write(self):
        with self.open() as store:
            store.put(1, b'a' * cache.MAX_MAIL)
            original = store.atomic
            def checked(name, raw, prefix):
                current = sum(p.stat().st_size for p in (self.work / 'mail-cache').iterdir())
                self.assertLessEqual(current + len(raw), store.budget)
                return original(name, raw, prefix)
            with patch.object(store, 'atomic', side_effect=checked):
                store.put(2, b'b' * cache.MAX_MAIL)
            self.assertFalse(store.has(1))
            self.assertTrue(store.has(2))
            self.assertLessEqual(sum(p.stat().st_size for p in (self.work / 'mail-cache').iterdir()), store.budget)

    def test_entry_count_evicts_oldest_access(self):
        with patch.object(cache, 'MAX_ENTRIES', 3), self.open() as store:
            store.put(1, b'a')
            store.put(2, b'b')
            store.put(3, b'c')
            for number in (1, 2, 3):
                os.utime(self.work / 'mail-cache' / f'{number}.eml', ns=(number, number))
            self.assertEqual(store.get(1), b'a')
            store.put(4, b'd')
            self.assertLessEqual(len(list((self.work / 'mail-cache').glob('*.eml'))), 3)
            self.assertTrue(store.has(1))
            self.assertFalse(store.has(2))
            self.assertTrue(store.has(4))

    def test_private_modes_and_symlinks_never_followed(self):
        outside = self.work / 'outside'
        outside.write_bytes(b'private fixture sentinel')
        with self.open() as store:
            store.put(1, b'cached')
            directory = self.work / 'mail-cache'
            self.assertEqual(stat.S_IMODE(directory.stat().st_mode), 0o700)
            for name in ('.lock', '.state', '1.eml'):
                self.assertEqual(stat.S_IMODE((directory / name).stat().st_mode), 0o600)
            os.chmod(directory / '1.eml', 0o644)
            store.get(1)
            self.assertEqual(stat.S_IMODE((directory / '1.eml').stat().st_mode), 0o600)
            os.symlink(outside, directory / '2.eml')
            with self.assertRaises((OSError, cache.CacheError)):
                store.get(2)
            self.assertEqual(outside.read_bytes(), b'private fixture sentinel')

    def test_corrupt_state_rebuilds_without_trusting_a_partial_mail(self):
        with self.open() as store:
            store.put(1, b'old cache')
        directory = self.work / 'mail-cache'
        (directory / '.state').write_bytes(b'{invalid')
        (directory / ('.body-' + 'a' * 32)).write_bytes(b'partial')
        with self.open() as store:
            self.assertFalse(store.has(1))
            self.assertEqual(store.state['cursor'], 0)
            self.assertEqual(sorted(p.name for p in directory.iterdir()), ['.lock', '.state'])

    def test_interrupted_atomic_write_never_becomes_cached(self):
        with self.open() as store:
            store.put(1, b'complete')
            with patch('cache.os.replace', side_effect=OSError('fixture interruption')):
                with self.assertRaises(OSError):
                    store.put(2, b'partial')
            self.assertFalse(store.has(2))
            self.assertEqual(store.get(1), b'complete')
            self.assertEqual(len(list((self.work / 'mail-cache').iterdir())), 3)

    def test_corrupt_entries_are_repaired_without_following_links(self):
        with self.open() as store:
            store.put(1, b'valid mail')
        directory = self.work / 'mail-cache'
        outside = self.work / 'outside'
        outside.write_bytes(b'private outside fixture')
        (directory / 'stray').write_bytes(b'junk')
        (directory / '2.eml').write_bytes(b'')
        with open(directory / '3.eml', 'wb') as stream:
            stream.truncate(3 * cache.MiB)
        os.symlink(outside, directory / '4.eml')
        (directory / 'stray-directory').mkdir()
        (directory / 'stray-directory' / 'file').write_bytes(b'junk')
        with self.open() as store:
            self.assertEqual(store.get(1), b'valid mail')
            self.assertGreaterEqual(store.repaired, 5)
            self.assertEqual([name for name, _, _ in store.files()], ['1.eml'])
            self.assertEqual(sorted(p.name for p in directory.iterdir()), ['.lock', '.state', '1.eml'])
        self.assertEqual(outside.read_bytes(), b'private outside fixture')
        (directory / '.state').unlink()
        os.symlink(outside, directory / '.state')
        with self.open() as store:
            self.assertFalse(store.has(1))
            self.assertGreaterEqual(store.repaired, 1)
        self.assertEqual(outside.read_bytes(), b'private outside fixture')

    def test_root_directory_symlink_is_refused(self):
        outside = self.work / 'outside-directory'
        outside.mkdir()
        (outside / 'sentinel').write_bytes(b'outside stays untouched')
        os.symlink(outside, self.work / 'mail-cache')
        with self.assertRaises(OSError):
            with self.open():
                pass
        self.assertEqual((outside / 'sentinel').read_bytes(), b'outside stays untouched')

    def test_flock_serializes_process_generations(self):
        code = 'import os,fcntl,sys\nf=os.open(sys.argv[1],os.O_RDWR)\ntry: fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB)\nexcept BlockingIOError: sys.exit(23)\n'
        lock = str(self.work / 'mail-cache' / '.lock')
        with self.open():
            result = subprocess.run([sys.executable, '-B', '-c', code, lock], capture_output=True, timeout=2)
            self.assertEqual(result.returncode, 23)
        result = subprocess.run([sys.executable, '-B', '-c', code, lock], capture_output=True, timeout=2)
        self.assertEqual(result.returncode, 0)

    def test_uid_and_message_caps(self):
        self.assertEqual(cache.MAX_MAIL, 2097152)
        with self.open() as store:
            for invalid in (0, -1, True, 1.5, '1\r\nLOGOUT', '01', 4294967296):
                with self.assertRaises(cache.CacheError):
                    store.has(invalid)
            for raw in (b'', b'a' * (cache.MAX_MAIL + 1)):
                with self.assertRaises(cache.CacheError):
                    store.put(1, raw)
            self.assertEqual(store.files(), [])
            with open(self.work / 'mail-cache' / '1.eml', 'wb') as stream:
                stream.truncate(cache.MAX_MAIL + 1)
            with self.assertRaises(cache.CacheError):
                store.has(1)
            with self.assertRaises(cache.CacheError):
                store.get(1)


if __name__ == '__main__':
    unittest.main()
