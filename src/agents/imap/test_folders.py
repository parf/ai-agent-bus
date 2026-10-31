"""Folder routing against a local TLS IMAP server; never a live mailbox."""
import hashlib
import json
import os
from pathlib import Path
import unittest
from unittest.mock import Mock, patch

import test_imap as fixture
RAW, USER, proxy = fixture.RAW, fixture.USER, fixture.proxy
from cache import Cache, MiB
import folders
import wire


class FolderCalls(unittest.TestCase):
    setUpClass = classmethod(fixture.IMAPTests.setUpClass.__func__)
    setUp = fixture.IMAPTests.setUp
    call = fixture.IMAPTests.call

    def add(self, name='Other Box', validity=2, raw=b'Subject: Login code\r\n\r\n123456'):
        self.server.folder_rows.append((name, b'\\Archive'))
        data = {'messages': {7: raw}, 'flags': {}, 'validity': validity}
        self.server.folders[name] = data
        return data

    def test_folders_roles_counts_and_no_selection(self):
        self.add()
        for name, flag in [('Sent', b'\\Sent'), ('Drafts', b'\\Drafts'), ('Trash', b'\\Trash'), ('Junk', b'\\Junk')]:
            self.server.folder_rows.append((name, flag))
        self.server.folder_rows.append(('Parent', b'\\Noselect'))
        self.server.flags[7] = {b'\\Seen'}
        answer = self.call('folders')
        for role in ('Inbox', 'Sent', 'Drafts', 'Trash', 'Junk', 'Archive'):
            self.assertIn(' | ' + role, answer)
        self.assertEqual(self.server.mailboxes, [])
        self.assertFalse(any(cmd.split()[1] == b'STATUS' for cmd in self.server.commands))
        answer = self.call('{"method":"folders","counts":true}')
        self.assertIn('"INBOX" | Inbox | total 1 | unseen 0', answer)
        self.assertIn('"Other Box" | Archive | total 1 | unseen 1', answer)
        self.assertIn('"Parent" | - | not selectable', answer)
        self.assertEqual(self.server.fetches, [])
        self.assertEqual(self.server.mailboxes, [])

    def test_live_list_status_and_special_use_return(self):
        self.add()
        self.server.list_status = True
        self.assertIn('"Other Box" | Archive | total 1 | unseen 1', self.call('folders --counts'))
        commands = [cmd for cmd in self.server.commands if b' LIST ' in cmd]
        self.assertEqual(len(commands), 1)
        self.assertIn(b'RETURN (SPECIAL-USE STATUS (MESSAGES UNSEEN))', commands[0])
        self.assertFalse(any(cmd.split()[1] == b'STATUS' for cmd in self.server.commands))
        self.assertEqual(self.server.mailboxes, [])
        self.server.commands.clear()
        self.assertIn('"INBOX" | Inbox', self.call('folders'))
        self.assertIn(b'RETURN (SPECIAL-USE)', next(cmd for cmd in self.server.commands if b' LIST ' in cmd))
        self.assertFalse(any(cmd.split()[1] == b'STATUS' for cmd in self.server.commands))

    def test_fallback_count_cap(self):
        for n in range(53):
            self.server.folder_rows.append((f'Box{n}', b''))
        answer = self.call('folders --counts')
        self.assertIn('counts skipped for 4 folders (limit 50)', answer)
        self.assertEqual(sum(cmd.split()[1] == b'STATUS' for cmd in self.server.commands), 50)

    def test_all_six_calls_are_folder_scoped(self):
        other = self.add()
        self.assertIn('fetched 1;', self.call('sync --folder "Other Box"'))
        self.assertIn('Login code', self.call('list --folder "Other Box"'))
        self.assertIn('Login code', self.call('search Login --folder "Other Box"'))
        self.assertNotIn('External report', self.call('list --folder "Other Box"'))
        self.assertIn('123456', self.call('read 7 --no-mark --folder "Other Box"'))
        self.assertIn('observed read', self.call('mark 7 read --folder "Other Box"'))
        self.assertEqual(other['flags'][7], {b'\\Seen'})
        self.assertEqual(self.server.flags, {})
        self.assertIn('published UID', self.call('publish 7 --folder "Other Box"'))
        self.assertIn('123456', self.bus.sent[-1][1])
        self.assertNotIn('External report', self.bus.sent[-1][1])
        self.assertTrue(all(name == b'"Other Box"' for name in self.server.mailboxes))
        self.assertIn('External report', self.call('read 7 --no-mark'))
        self.assertIn('123456', self.call('read 7 --no-mark --folder "Other Box"'))

    def test_epoch_is_independent_and_default_cache_is_adopted(self):
        identity = hashlib.sha256(json.dumps([self.cfg['host'], self.cfg['port'], USER, 'INBOX'], ensure_ascii=False).encode()).hexdigest()
        with Cache(self.work.name, identity, 1) as old:
            old.put(7, RAW)
        original = Path(self.work.name, 'mail-cache', '7.eml').stat().st_ino
        self.assertIn('(cached)', self.call('read 7 --no-mark --folder inbox'))
        self.assertEqual(self.server.fetches, [])
        other = self.add()
        self.assertIn('no cached matches', self.call('list --folder "Other Box"'))
        self.assertIn('123456', self.call('read 7 --no-mark --folder "Other Box"'))
        other['validity'] = 3
        other['messages'][7] = b'Subject: Replacement\r\n\r\nnew epoch'
        self.assertIn('epoch changed', self.call('read 7 --folder "Other Box"'))
        self.assertIn('new epoch', self.call('read 7 --no-mark --folder "Other Box"'))
        self.assertIn('(cached)', self.call('read 7 --no-mark'))
        self.assertEqual(Path(self.work.name, 'mail-cache', '7.eml').stat().st_ino, original)
        self.assertEqual(len(list(Path(self.work.name).glob('mail-cache-*'))), 1)

    def test_progress_does_not_reset_or_depend_on_forwarding(self):
        self.add()
        self.assertIn('fetched 1;', self.call('sync'))
        before = self.bus.kv.get('imap.progress', kind='json')
        self.bus.kv.set('imap.progress_error', {'error': 'fixture'}, kind='json', mode='set')
        self.assertIn('fetched 1;', self.call('sync --folder "Other Box"'))
        self.assertEqual(self.bus.kv.get('imap.progress', kind='json'), before)
        self.assertIn('forwarding state invalid', self.call('sync'))
        keys = self.bus.kv.list()['json']
        self.assertEqual(sum(row['name'].startswith('imap.progress.') for row in keys), 1)
        self.assertIn('fetched 0;', self.call('sync --folder "Other Box"'))

    def test_unknown_and_noselect_refused_before_select(self):
        self.server.folder_rows += [('Parent', b'\\Noselect')]
        for name, reason in [('Missing', 'folder not found'), ('Parent', 'not selectable'), ('inBoxx', 'folder not found')]:
            with self.subTest(name=name):
                self.assertIn(reason, self.call(json.dumps({'method': 'read', 'uid': 7, 'folder': name})))
        self.assertEqual(self.server.mailboxes, [])
        self.assertEqual(self.server.fetches, [])
        self.assertEqual(self.server.stores, [])

    def test_unicode_quotes_and_ampersand_roundtrip(self):
        name = 'Письма & "login"'
        self.add(name)
        self.assertIn(json.dumps(name, ensure_ascii=False), self.call('folders'))
        self.assertIn('123456', self.call(json.dumps({'method': 'read', 'uid': 7, 'folder': name, 'mark_read': False})))
        self.assertEqual(self.server.mailboxes, [wire.mailbox(name).encode()])


class FolderProtocol(unittest.TestCase):
    def test_text_json_parser_and_refusals(self):
        for method, body in [('sync', 'sync 2'), ('list', 'list --unread'), ('search', 'search login code'), ('read', 'read 7 --no-mark'), ('mark', 'mark 7 unread'), ('publish', 'publish 7')]:
            with self.subTest(method=method):
                value = proxy.parse(body + ' --folder "Login Mail"')
                self.assertEqual(value['folder'], 'Login Mail')
                self.assertEqual(proxy.parse(json.dumps(value)), value)
        for body in ('sync --folder', 'sync --folder ""', 'list --folder X --folder Y', 'folders --counts --counts', '{"method":"folders","counts":1}', '{"method":"sync","folder":null}', 'search x\r\nLOGIN --folder INBOX', '{"method":"read","uid":7,"folder":"X\\r\\nSELECT INBOX"}', 'read 7 --folder "unclosed'):
            with self.subTest(body=body), self.assertRaises((ValueError, wire.WireError)):
                proxy.parse(body)

    def test_literal_and_invalid_list_names(self):
        raw = wire.mailbox('邮件 &').encode()[1:-1]
        self.assertEqual(folders.row((b'(\\Sent) "/" {' + str(len(raw)).encode() + b'}', raw))['name'], '邮件 &')
        for raw in (b'&broken', b'&AA-', b'&YQ-', b'&!!!!-', b'bad\r\nname'):
            with self.subTest(raw=raw), self.assertRaises(wire.WireError):
                folders.decode(raw)
        for item in (b'no flags', b'() "/" "unclosed', b'() "/" "x" junk', (b'() NIL {2}', b'x')):
            with self.subTest(item=item), self.assertRaises(wire.WireError):
                folders.row(item)

    def test_list_status_extension_and_count_validation(self):
        connection = Mock(capabilities=('LIST-STATUS',))
        connection._simple_command.return_value = ('OK', [b'done'])
        connection._untagged_response.side_effect = [('OK', [b'() "/" "INBOX"']), ('OK', [b'"INBOX" (UNSEEN 2 MESSAGES 3)'])]
        self.assertIn('total 3 | unseen 2', folders.describe(connection, True))
        connection.list.assert_not_called()
        connection.status.assert_not_called()
        self.assertIn('RETURN', connection._simple_command.call_args.args)
        for raw in (b'INBOX (MESSAGES 1 UNSEEN 2)', b'INBOX (MESSAGES 1 MESSAGES 1)', b'INBOX (MESSAGES -1 UNSEEN 0)'):
            with self.subTest(raw=raw), self.assertRaises(wire.WireError):
                folders.parse_counts(raw)

    def test_one_byte_budget_and_lru_across_folders(self):
        import tempfile
        root = Path(__file__).resolve().parents[3] / 'tmp'
        with tempfile.TemporaryDirectory(dir=root) as work:
            with Cache(work, 'default', 1, 4 * MiB) as old:
                old.put(1, b'a' * MiB)
                old.put(2, b'b' * MiB)
                old.put(3, b'c' * MiB)
                os.utime(Path(work, 'mail-cache', '1.eml'), ns=(1, 1))
                os.utime(Path(work, 'mail-cache', '2.eml'), ns=(2, 2))
                os.utime(Path(work, 'mail-cache', '3.eml'), ns=(3, 3))
            with Cache(work, 'a' * 64, 1, 4 * MiB, folder=True) as other:
                other.put(1, b'd' * MiB)
            self.assertFalse(Path(work, 'mail-cache', '1.eml').exists())
            self.assertTrue(Path(work, 'mail-cache', '2.eml').exists())
            self.assertTrue(Path(work, 'mail-cache-' + 'a' * 64, '1.eml').exists())
            self.assertEqual(sum(path.stat().st_size for path in Path(work).glob('mail-cache*/*.eml')), 3 * MiB)
