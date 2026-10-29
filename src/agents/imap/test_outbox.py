import os
import errno
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import patch

import outbox


class OutboxTests(unittest.TestCase):
    def setUp(self):
        root = Path(__file__).resolve().parents[2] / 'tmp'
        root.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix='imap-outbox-', dir=root)
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.first = outbox.filename('account', 7, 100)
        self.second = outbox.filename('account', 7, 101)

    def test_one_immutable_body_is_shared_by_references_and_survives_restart(self):
        with outbox.Outbox(self.work) as store:
            self.assertTrue(store.put(self.first, b'original'))
            self.assertFalse(store.put(self.first, b'different'))
            self.assertEqual(store.listing(), {self.first: 8})
        with outbox.Outbox(self.work) as store:
            self.assertEqual(store.get(self.first), b'original')
            self.assertEqual(store.sweep({self.first}), 0)
            self.assertEqual(store.sweep(set()), 1)
            self.assertEqual(store.listing(), {})

    def test_account_and_validity_have_distinct_body_names(self):
        self.assertNotEqual(self.first, outbox.filename('other', 7, 100))
        self.assertNotEqual(self.first, outbox.filename('account', 8, 100))
        for name in ('../outside', '/etc/passwd', self.first + '/x', '0' * 64 + '.4294967296.eml'):
            with self.assertRaises(outbox.OutboxError):
                outbox.check_name(name)

    def test_capacity_refuses_without_evicting_referenced_mail(self):
        with patch.object(outbox, 'MAX_BYTES', 8), outbox.Outbox(self.work) as store:
            store.put(self.first, b'12345678')
            with self.assertRaises(outbox.CapacityError):
                store.put(self.second, b'x')
            self.assertEqual(store.get(self.first), b'12345678')
            self.assertEqual(store.listing(), {self.first: 8})
        with patch.object(outbox, 'MAX_FILES', 1), outbox.Outbox(self.work) as store:
            with self.assertRaises(outbox.CapacityError):
                store.put(self.second, b'x')

    def test_private_modes_and_root_and_body_symlinks_never_followed(self):
        outside = self.work / 'outside'
        outside.write_bytes(b'outside sentinel')
        with outbox.Outbox(self.work) as store:
            store.put(self.first, b'original')
            directory = self.work / 'mail-outbox'
            self.assertEqual(stat.S_IMODE(directory.stat().st_mode), 0o700)
            self.assertEqual(stat.S_IMODE((directory / self.first).stat().st_mode), 0o600)
            os.symlink(outside, directory / self.second)
            with self.assertRaises(outbox.MissingBody):
                store.get(self.second)
            self.assertEqual(store.sweep({self.first}), 0)
            self.assertEqual(store.repaired, 1)
            self.assertEqual(outside.read_bytes(), b'outside sentinel')
        other = self.work / 'other'
        other.mkdir()
        os.symlink(directory, other / 'mail-outbox')
        with self.assertRaises(OSError):
            outbox.Outbox(other)

    def test_partial_file_is_never_a_body_and_is_cleaned_at_restart(self):
        with outbox.Outbox(self.work) as store:
            with patch('outbox.os.link', side_effect=OSError('fixture interruption')):
                with self.assertRaises(OSError):
                    store.put(self.first, b'raw mail')
            self.assertEqual(store.listing(), {})

    def test_private_junk_and_nonempty_lock_are_repaired_without_following_links(self):
        with outbox.Outbox(self.work) as store:
            directory = self.work / 'mail-outbox'
        outside = self.work / 'outside'
        outside.write_bytes(b'outside sentinel')
        (directory / 'notes.txt').write_text('stray')
        (directory / '.lock').write_text('old data')
        (directory / self.first).write_bytes(b'x' * (outbox.MAX_BODY + 1))
        os.symlink(outside, directory / self.second)
        junk = directory / 'old-backup'
        junk.mkdir()
        os.symlink(outside, junk / 'link')
        with outbox.Outbox(self.work) as store:
            self.assertEqual(store.listing(), {})
            self.assertEqual(store.repaired, 5)
            self.assertTrue(store.put(self.first, b'recovered'))
            self.assertEqual(store.sweep({self.first}), 0)
            self.assertEqual(outside.read_bytes(), b'outside sentinel')

    def test_link_unsupported_uses_fsynced_rename_under_the_lock(self):
        for code in (errno.EPERM, errno.EMLINK, errno.ENOTSUP):
            with outbox.Outbox(self.work) as store:
                store.sweep(set())
                failure = None
                with patch('outbox.os.link', side_effect=OSError(code, 'fixture')):
                    try:
                        created = store.put(self.first, b'raw mail')
                    except OSError as error:
                        failure = type(error)
                self.assertIsNone(failure, 'unsupported hardlinks must use the serialized rename fallback')
                self.assertTrue(created)
                self.assertEqual(store.get(self.first), b'raw mail')

    def test_missing_reference_is_distinct_and_orphan_is_swept_after_restart(self):
        with outbox.Outbox(self.work) as store:
            store.put(self.first, b'not committed into a ledger yet')
        with outbox.Outbox(self.work) as store:
            self.assertEqual(store.sweep(set()), 1)
            with self.assertRaises(outbox.MissingBody):
                store.get(self.first)
        temp = self.work / 'mail-outbox' / ('.tmp-' + 'a' * 32)
        temp.write_bytes(b'partial')
        with outbox.Outbox(self.work) as store:
            self.assertFalse(temp.exists())
            self.assertEqual(store.listing(), {})

    def test_delete_or_mailbox_reset_sweeps_only_unreferenced_bodies(self):
        with outbox.Outbox(self.work) as store:
            store.put(self.first, b'one')
            store.put(self.second, b'two')
            self.assertEqual(store.sweep({self.second}), 1)
            self.assertEqual(store.get(self.second), b'two')
            self.assertEqual(store.sweep(set()), 1)
            self.assertEqual(store.listing(), {})

    def test_oversize_and_zero_body_are_refused_before_file_creation(self):
        with outbox.Outbox(self.work) as store, patch.object(outbox, 'MAX_BODY', 8):
            for raw in (b'', b'x' * 9):
                with self.assertRaises(outbox.OutboxError):
                    store.put(self.first, raw)
            self.assertEqual(store.listing(), {})

    def test_content_and_directory_are_fsynced_around_publish(self):
        events = []
        real_sync, real_link = os.fsync, os.link
        def sync(handle):
            events.append('directory' if stat.S_ISDIR(os.fstat(handle).st_mode) else 'content')
            return real_sync(handle)
        def link(*args, **kwargs):
            self.assertIn('content', events, 'publication preceded content fsync')
            events.append('publish')
            return real_link(*args, **kwargs)
        with outbox.Outbox(self.work) as store:
            with patch('outbox.os.fsync', side_effect=sync), patch('outbox.os.link', side_effect=link):
                store.put(self.first, b'raw mail')
            self.assertEqual(events, ['content', 'publish', 'directory'])

    def test_unknown_reference_set_refuses_sweep_without_removing_any_body(self):
        with outbox.Outbox(self.work) as store:
            store.put(self.first, b'raw mail')
            for keep in (None, [], {}):
                error = None
                try:
                    store.sweep(keep)
                except Exception as caught:
                    error = caught
                self.assertIs(type(error), outbox.OutboxError)
                self.assertEqual(store.get(self.first), b'raw mail')

    def test_mode_repair_applies_to_existing_directory_and_file(self):
        with outbox.Outbox(self.work) as store:
            store.put(self.first, b'raw mail')
        directory = self.work / 'mail-outbox'
        os.chmod(directory, 0o755)
        os.chmod(directory / self.first, 0o644)
        with outbox.Outbox(self.work) as store:
            store.listing()
            self.assertEqual(stat.S_IMODE(directory.stat().st_mode), 0o700)
            self.assertEqual(stat.S_IMODE((directory / self.first).stat().st_mode), 0o600)

    def test_transient_open_error_is_not_a_permanent_missing_body(self):
        with outbox.Outbox(self.work) as store:
            with patch.object(store, 'open', side_effect=OSError(errno.EMFILE, 'fixture')):
                with self.assertRaises(outbox.OutboxError) as caught:
                    store.get(self.first)
                self.assertNotIsInstance(caught.exception, outbox.MissingBody)

    def test_another_generation_has_a_bounded_wait_and_changes_no_body(self):
        with outbox.Outbox(self.work) as store:
            store.put(self.first, b'held body')
            with self.assertRaises(outbox.PreviousGeneration):
                outbox.Outbox(self.work, lock_wait=.01)
            self.assertEqual(store.get(self.first), b'held body')


if __name__ == '__main__':
    unittest.main()
