"""Destructive operations on a local fake mailbox only, never a provider."""
import datetime
import json
from pathlib import Path
import unittest
from unittest.mock import patch
import test_imap as fixture
import destructive
import operations
import wire


class Calls(unittest.TestCase):
    setUpClass = classmethod(fixture.IMAPTests.setUpClass.__func__)
    setUp = fixture.IMAPTests.setUp
    call = fixture.IMAPTests.call

    def enable(self, *capabilities):
        self.cfg['allow_destructive'] = True
        self.server.extra_capabilities = list(capabilities)
        self.server.permanent.append(b'\\Deleted')
        self.server.messages.update({8: fixture.RAW + b'\r\nOnly UID 8', 9: fixture.RAW + b'\r\nDo not copy UID 9'})
        self.server.flags[9] = {b'\\Deleted', b'\\Seen'}
        self.destination = self.folder('Destination', '')
        self.trash = self.folder('Bin', '\\Trash')

    def folder(self, name, role):
        self.server.folder_rows.append((name, role.encode()))
        value = {'messages': {}, 'flags': {}, 'validity': 3, 'next_uid': 100}
        self.server.folders[name] = value
        return value

    def writes(self):
        return [c for c in self.server.commands if any(b' ' + verb + b' ' in c for verb in (b'COPY', b'MOVE', b'STORE', b'EXPUNGE', b'CLOSE'))]

    def assert_exact(self, verb, numbers):
        commands = [c.split() for c in self.server.commands if b' UID ' + verb + b' ' in c]
        self.assertEqual([int(c[3]) for c in commands], numbers)
        self.assertFalse(any(c.split()[1] in (b'EXPUNGE', b'CLOSE') for c in self.server.commands))

    def test_switch_refuses_each_method_before_connection_and_body_cannot_enable(self):
        for body in ('copy 7 Destination', 'move 7 Destination', 'trash 7', 'delete 7', 'compact_trash'):
            with self.subTest(body=body):
                answer = self.call(body)
                self.assertIn(body.split()[0] + ' requires Configuration allow_destructive=true', answer)
                self.assertEqual(self.server.commands, [])
        self.assertTrue(self.call('{"method":"delete","uids":"7","allow_destructive":true}').startswith('error:'))
        self.assertEqual(self.server.commands, [])
        for value in ('true', 1, None, []):
            with self.assertRaises(ValueError): fixture.proxy.config({'allow_destructive': value})

    def test_copy_preserves_source_and_flags_and_reports_new_uid(self):
        self.enable('UIDPLUS')
        self.call('read 7 --no-mark')
        answer = self.call('copy 7,999,8 Destination')
        self.assertIn('UID 999: error: UID is no longer present', answer)
        self.assertIn('UID 7 copied, source kept; destination UID 100', answer)
        self.assertIn('UID 8 copied, source kept; destination UID 101', answer)
        self.assertEqual(set(self.server.messages), {7, 8, 9})
        self.assertEqual(set(self.destination['messages']), {100, 101})
        self.assertEqual(self.destination['messages'][100], fixture.RAW)
        self.assertEqual(self.destination['messages'][101], fixture.RAW + b'\r\nOnly UID 8')
        self.assertTrue((Path(self.work.name) / 'mail-cache/7.eml').is_file())
        self.assertEqual(self.server.flags[9], {b'\\Deleted', b'\\Seen'})
        self.assert_exact(b'COPY', [7, 8])
        self.assertEqual(self.server.stores, [])

    def test_native_move_removes_only_named_source_cache_and_preserves_other_folder(self):
        self.enable('MOVE')
        self.destination['messages'][7] = b'Subject: other folder\r\n\r\nother'
        self.call('read 7 --no-mark')
        self.call('read 7 --no-mark --folder Destination')
        answer = self.call('move 7 Destination')
        self.assertIn('UID 7 moved; source removed; destination UID 100', answer)
        self.assertEqual(set(self.server.messages), {8, 9})
        self.assertEqual(self.server.flags.get(8, set()), set())
        self.assertEqual(set(self.destination['messages']), {7, 100})
        self.assertFalse((Path(self.work.name) / 'mail-cache/7.eml').exists())
        self.assertIn('(cached)', self.call('read 7 --no-mark --folder Destination'))
        self.assert_exact(b'MOVE', [7])
        self.assertEqual(self.server.stores, [])

    def test_fallback_move_has_exact_copy_store_expunge_and_keeps_other_deleted_mail(self):
        self.enable('UIDPLUS')
        self.call('read 7 --no-mark')
        answer = self.call('move 7 Destination')
        self.assertIn('UID 7 moved; destination UID 100', answer)
        self.assertEqual(set(self.server.messages), {8, 9})
        self.assertEqual(self.server.flags.get(8, set()), set())
        self.assertEqual(set(self.destination['messages']), {100})
        self.assertEqual(self.server.stores, [(7, b'+FLAGS.SILENT (\\Deleted)')])
        self.assertFalse((Path(self.work.name) / 'mail-cache/7.eml').exists())
        self.assert_exact(b'COPY', [7])
        self.assert_exact(b'EXPUNGE', [7])

    def test_capability_refusals_make_no_copy_or_store(self):
        self.enable()
        for body in ('move 7 Destination', 'trash 7', 'delete 7', 'compact_trash'):
            with self.subTest(body=body):
                self.server.commands.clear()
                answer = self.call(body)
                self.assertIn('requires', answer)
                self.assertIn('UIDPLUS', answer)
                self.assertEqual(self.writes(), [])

    def test_missing_deleted_permission_refuses_before_copy_or_flag(self):
        self.enable('UIDPLUS')
        self.server.permanent.remove(b'\\Deleted')
        for body in ('move 7 Destination', 'delete 7', 'compact_trash'):
            self.server.commands.clear()
            self.assertIn('does not permit persistent \\Deleted', self.call(body))
            self.assertEqual(self.writes(), [])

    def test_destination_and_trash_role_refusals_precede_selection(self):
        self.enable('MOVE', 'UIDPLUS')
        self.server.folder_rows.append(('Parent', b'\\Noselect'))
        for body in ('copy 7 Missing', 'move 7 Missing', 'copy 7 Parent', 'move 7 Parent'):
            self.server.mailboxes.clear()
            self.assertIn('folder', self.call(body))
            self.assertEqual(self.server.mailboxes, [])
            self.assertEqual(self.writes(), [])
        self.server.folder_rows = [row for row in self.server.folder_rows if row[0] != 'Bin']
        for body in ('trash 7', 'compact_trash'):
            self.assertIn('no selectable Trash role folder', self.call(body))
            self.assertEqual(self.writes(), [])
        self.server.folder_rows.append(('BlockedTrash', b'\\Trash \\Noselect'))
        self.assertIn('no selectable Trash role folder', self.call('trash 7'))
        self.assertEqual(self.writes(), [])
        self.server.folder_rows = [row for row in self.server.folder_rows if row[0] != 'BlockedTrash']
        self.server.folder_rows.append(('Bin', b'\\Trash'))
        self.folder('Second Bin', '\\Trash')
        self.assertIn('ambiguous Trash role', self.call('trash 7'))
        self.assertIn('ambiguous Trash role', self.call('compact_trash'))
        self.assertEqual(self.writes(), [])

    def test_trash_uses_role_not_name_and_only_requested_uid(self):
        self.enable('MOVE')
        wrong = self.folder('Trash', '')
        answer = self.call('trash 7')
        self.assertIn('moved', answer)
        self.assertEqual(set(self.trash['messages']), {100})
        self.assertEqual(wrong['messages'], {})
        self.assertEqual(set(self.server.messages), {8, 9})
        self.assertEqual(self.server.flags.get(8, set()), set())
        self.assert_exact(b'MOVE', [7])

    def test_delete_exact_uid_and_no_close_leaves_other_deleted_mail(self):
        self.enable('UIDPLUS')
        self.call('read 7 --no-mark')
        answer = self.call('delete 7')
        self.assertIn('UID 7 deleted; source removed', answer)
        self.assertEqual(set(self.server.messages), {8, 9})
        self.assertEqual(self.server.flags.get(8, set()), set())
        self.assertEqual(self.server.flags[9], {b'\\Deleted', b'\\Seen'})
        self.assertEqual(self.server.stores, [(7, b'+FLAGS.SILENT (\\Deleted)')])
        self.assert_exact(b'EXPUNGE', [7])
        self.assertFalse((Path(self.work.name) / 'mail-cache/7.eml').exists())

    def test_compact_only_trash_internaldate_and_matching_uids(self):
        self.enable('UIDPLUS')
        self.trash['messages'].update({7: b'Date: Thu, 01 Jan 2099 00:00:00 +0000\r\n\r\nold internally',
                                       8: b'Date: Mon, 01 Jan 2001 00:00:00 +0000\r\n\r\nnew internally', 9: fixture.RAW})
        self.trash['flags'][9] = {b'\\Deleted'}
        today = datetime.datetime.now(datetime.timezone.utc).date()
        self.server.internal_dates = {('Bin', 7): today - datetime.timedelta(days=30), ('Bin', 8): today, ('Bin', 9): today}
        self.server.mailboxes.clear()
        self.assertIn('restricted to the Trash role folder', self.call('compact_trash --folder INBOX'))
        self.assertEqual(self.server.mailboxes, [])
        self.assertEqual(self.writes(), [])
        answer = self.call('compact_trash --older 7')
        self.assertIn('UID 7 deleted', answer)
        self.assertEqual(set(self.trash['messages']), {8, 9})
        self.assertEqual(set(self.server.messages), {7, 8, 9})
        self.assertEqual(self.server.mailboxes, [b'"Bin"'])
        self.assert_exact(b'EXPUNGE', [7])
        self.assertEqual(self.server.stores, [(7, b'+FLAGS.SILENT (\\Deleted)')])
        self.assertTrue(any(b'UID SEARCH BEFORE' in c for c in self.server.commands))

    def test_compact_all_and_over_limit_refuses_before_writes(self):
        self.enable('UIDPLUS')
        self.trash['messages'].update({7: fixture.RAW, 8: fixture.RAW})
        self.assertIn('UID 8 deleted', self.call('compact_trash'))
        self.assertEqual(self.trash['messages'], {})
        self.assertEqual(set(self.server.messages), {7, 8, 9})
        self.assert_exact(b'EXPUNGE', [7, 8])
        self.trash['messages'].update({n: fixture.RAW for n in range(1, 1002)})
        self.server.commands.clear()
        self.envelope = {'deadline': (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=5)).isoformat()}
        self.assertIn('1000 UIDs', self.call('compact_trash'))
        self.assertEqual(self.writes(), [])
        self.assertEqual(len(self.trash['messages']), 1001)

    def test_bad_or_missing_copyuid_never_removes_source(self):
        self.enable('UIDPLUS')
        for mode in ('missing', 'wrong'):
            self.server.copyuid_mode = mode
            answer = self.call('move 7 Destination')
            self.assertIn('copied, source kept', answer)
            self.assertIn('COPYUID', answer)
            self.assertIn(7, self.server.messages)
            self.assertEqual(self.server.stores, [])
        self.assertEqual(len(self.destination['messages']), 2)
        self.assertFalse(any(b'UID EXPUNGE' in c for c in self.server.commands))

    def test_copy_and_advertised_move_refusal_are_not_retried(self):
        self.enable('MOVE', 'UIDPLUS')
        self.server.copy_refused = True
        self.assertIn('refused', self.call('move 7 Destination'))
        self.assertEqual(self.server.stores, [])
        self.assertEqual(self.destination['messages'], {})
        self.assert_exact(b'MOVE', [7])
        self.assertFalse(any(b'UID COPY' in c for c in self.server.commands))
        self.server.commands.clear()
        self.assertIn('refused', self.call('copy 7 Destination'))
        self.assert_exact(b'COPY', [7])
        self.assertEqual(self.server.stores, [])
        self.server.extra_capabilities = ['UIDPLUS']
        self.server.commands.clear()
        self.assertIn('refused', self.call('move 7 Destination'))
        self.assert_exact(b'COPY', [7])
        self.assertEqual(self.server.stores, [])
        self.assertIn(7, self.server.messages)
        self.assertEqual(self.destination['messages'], {})

    def test_partial_fallback_preserves_new_uid_and_no_retry(self):
        self.enable('UIDPLUS')
        self.server.store_refused = True
        answer = self.call('move 7 Destination')
        self.assertIn('copied, source kept; destination UID 100', answer)
        self.assertIn('Deleted flag refused', answer)
        self.assertIn(7, self.server.messages)
        self.assertFalse(any(b'UID EXPUNGE' in c for c in self.server.commands))
        self.server.store_refused = False
        self.server.expunge_refused = True
        answer = self.call('move 8 Destination')
        self.assertIn('copied, source kept; destination UID 101', answer)
        self.assertIn('UID EXPUNGE refused', answer)
        self.assertIn(8, self.server.messages)
        self.assertIn(b'\\Deleted', self.server.flags[8])
        self.assert_exact(b'COPY', [7, 8])
        self.assert_exact(b'EXPUNGE', [8])

    def test_unknown_copy_does_not_repeat_and_later_uid_continues(self):
        self.enable('UIDPLUS')
        self.server.drop_after_copy = {7}
        answer = self.call('copy 7,8 Destination')
        self.assertIn('UID 7: error: COPY outcome unknown', answer)
        self.assertIn('UID 8 copied', answer)
        self.assertEqual(len(self.destination['messages']), 2)
        self.assertEqual(set(self.server.messages), {7, 8, 9})
        self.assert_exact(b'COPY', [7, 8])

    def test_unknown_expunge_keeps_mapping_and_does_not_repeat(self):
        self.enable('UIDPLUS')
        self.server.drop_after_expunge = True
        answer = self.call('move 7 Destination')
        self.assertIn('copied; destination UID 100', answer)
        self.assertIn('source removal outcome unknown', answer)
        self.assertNotIn(7, self.server.messages)
        self.assertEqual(len(self.destination['messages']), 1)
        self.assert_exact(b'COPY', [7])
        self.assert_exact(b'EXPUNGE', [7])

    def test_compact_reconnect_refuses_changed_trash_role(self):
        self.enable('UIDPLUS')
        self.trash['messages'].update({7: fixture.RAW, 8: fixture.RAW})
        self.server.drop_after_expunge = True
        self.server.change_trash_role_after_drop = True
        answer = self.call('compact_trash')
        self.assertIn('UID 7: error: source removal outcome unknown', answer)
        self.assertIn('UID 8: error: Trash role changed; UID not attempted', answer)
        self.assertEqual(set(self.trash['messages']), {8})
        self.assert_exact(b'EXPUNGE', [7])

    def test_trash_reconnect_refuses_changed_destination_role(self):
        self.enable('MOVE')
        self.server.drop_after_move = {7}
        self.server.change_trash_role_after_drop = True
        answer = self.call('trash 7,8')
        self.assertIn('UID 7: error: MOVE outcome unknown', answer)
        self.assertIn('UID 8: error: Trash role changed; UID not attempted', answer)
        self.assertEqual(set(self.trash['messages']), {100})
        self.assertIn(8, self.server.messages)
        self.assert_exact(b'MOVE', [7])

    def test_epoch_change_refuses_writes_and_reconnect_checks_original_epoch(self):
        self.enable('UIDPLUS')
        self.call('read 7 --no-mark')
        self.server.validity += 1
        self.server.commands.clear()
        self.assertIn('epoch changed', self.call('copy 7 Destination'))
        self.assertEqual(self.writes(), [])
        self.call('read 7 --no-mark')
        self.server.drop_after_copy = {7}
        self.server.change_epoch_after_copy = True
        answer = self.call('copy 7,8 Destination')
        self.assertIn('UID 8: error: UIDVALIDITY changed', answer)
        self.assert_exact(b'COPY', [7])


class Arguments(unittest.TestCase):
    def test_new_forms_and_uid_order_message(self):
        self.assertEqual(operations.parse_call('copy 130,123 "Other Box" --folder INBOX'),
                         {'method': 'copy', 'uids': [130, 123], 'dest': 'Other Box', 'folder': 'INBOX'})
        self.assertEqual(operations.parse_call('{"method":"move","uids":"1:3","dest":"Other"}')['uids'], [1, 2, 3])
        self.assertEqual(operations.parse_call('compact_trash --older 5')['older'], 5)
        with self.assertRaisesRegex(ValueError, 'each UID range must run low:high'): operations.uid_set('130:123')
        for body in ('copy 7 "Bad\r\nfolder"', 'delete 1:1001', 'compact_trash --older -1', 'compact_trash --older 365001',
                     '{"method":"compact_trash","older":true}', 'compact_trash 7', 'trash 7 Trash', 'copy 7 X --allow-destructive true', '{"method":"copy","uids":"7","dest":"bad\\r\\nfolder"}'):
            with self.subTest(body=body), self.assertRaises(ValueError): fixture.proxy.parse(body)
