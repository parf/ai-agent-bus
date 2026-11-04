"""Current generation locking and refusal of unsupported state, without deletion."""
from pathlib import Path
import tempfile
import unittest
import outbox


class GenerationTests(unittest.TestCase):
    def test_current_lock_reopens_and_excludes_overlapping_generation(self):
        with tempfile.TemporaryDirectory() as work:
            with outbox.Outbox(work):
                with self.assertRaises(outbox.PreviousGeneration):
                    outbox.Outbox(work, lock_wait=0)
                root = Path(work) / 'mail-outbox'
                self.assertEqual(root.stat().st_mode & 0o777, 0o700)
                self.assertEqual((root/'.lock').stat().st_mode & 0o777, 0o600)
            with outbox.Outbox(work):
                self.assertEqual(set(p.name for p in root.iterdir()), {'.lock'})

    def test_obsolete_body_or_journal_is_preserved_and_refused(self):
        for name in ('body.eml', '.tmp-old', 'ledger.json'):
            with self.subTest(name=name), tempfile.TemporaryDirectory() as work:
                with outbox.Outbox(work): pass
                item = Path(work)/'mail-outbox'/name
                item.write_bytes(b'preserve')
                with self.assertRaisesRegex(outbox.OutboxError, 'unsupported generation directory contents'):
                    outbox.Outbox(work)
                self.assertEqual(item.read_bytes(), b'preserve')

    def test_foreign_lock_and_symlinks_are_never_replaced(self):
        for kind in ('content', 'link', 'directory'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as work:
                root = Path(work)/'mail-outbox'; root.mkdir()
                target = Path(work)/'foreign'; target.write_bytes(b'preserve')
                lock = root/'.lock'
                if kind == 'content': lock.write_bytes(b'preserve')
                elif kind == 'link': lock.symlink_to(target)
                else: lock.mkdir()
                with self.assertRaises((OSError, outbox.OutboxError)):
                    outbox.Outbox(work, lock_wait=0)
                self.assertEqual(target.read_bytes(), b'preserve')
                self.assertTrue(lock.exists())
                if kind == 'content': self.assertEqual(lock.read_bytes(), b'preserve')
                if kind == 'link': self.assertTrue(lock.is_symlink())
                if kind == 'directory': self.assertTrue(lock.is_dir())
