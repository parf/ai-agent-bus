"""Folder/APPEND authority and wire commands against the local TLS fake only."""
import json
import hashlib
import base64
import socket
import ipaddress
from urllib.parse import urlparse
import unittest
from unittest.mock import patch, Mock
import test_imap as fixture
import test_destructive
import folder_admin
import wire
from agent_bus.errors import InvalidArgument, Refusal, UncertainOutcome
from mail import MailError

CONNECT = socket.create_connection


class Calls(unittest.TestCase):
    setUpClass = classmethod(fixture.IMAPTests.setUpClass.__func__)
    setUp = fixture.IMAPTests.setUp
    call = fixture.IMAPTests.call
    folder = test_destructive.Calls.folder

    def writes(self):
        return [c for c in self.server.commands if c.split()[1] in {b'CREATE', b'RENAME', b'DELETE', b'APPEND', b'EXPUNGE', b'CLOSE'}]

    def enable(self):
        self.cfg['allow_destructive'] = True
        self.other = self.folder('Other', '')

    def big_store(self, raw=b'Subject: big\r\n\r\nmail'):
        info = {'size': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}
        big = Mock()
        big.get.return_value = (raw, info)
        big.put.side_effect = lambda name, body: {'size': len(body), 'sha256': hashlib.sha256(body).hexdigest()}
        original = self.bus.record
        def record(name):
            rec = original(name)
            rec.big = big
            return rec
        self.bus.record = Mock(side_effect=record)
        return big

    def test_big_reference_last_slash_and_exclusive_inline_text_json(self):
        for body in ['append Other --big "#imap/source@fixture/mail"',
                     json.dumps({'method': 'append', 'folder': 'Other', 'big': '#imap/source@fixture/mail'})]:
            self.assertEqual(fixture.proxy.parse(body)['big'], ('#imap/source@fixture', 'mail'))
        for ref in ('missing', '/key', '#record/', '', 1):
            with self.subTest(ref=ref), self.assertRaisesRegex(MailError, 'last /'):
                fixture.proxy.parse(json.dumps({'method': 'append', 'folder': 'Other', 'big': ref}))
        for body in ['append Other --big user/key\nraw',
                     json.dumps({'method': 'append', 'folder': 'Other', 'big': 'user/key', 'raw': 'raw'})]:
            with self.assertRaisesRegex(MailError, 'never both'): fixture.proxy.parse(body)
        for name in ('', 'x' * 257, 'é' * 129, 1):
            with self.assertRaisesRegex(MailError, 'key name'):
                fixture.proxy.parse(json.dumps({'method': 'part', 'uid': 7, 'part_id': '1', 'big': name}))
        self.assertEqual(fixture.proxy.parse('part 7 1 --big archive')['big'], 'archive')

    def test_big_part_exact_decoded_bytes_own_record_no_seen_and_no_answer_body(self):
        big = self.big_store()
        for kind, raw in [('TEXT', b'not-utf8:\xff secret-bytes'), ('APPLICATION', b'\x00\xffbinary')]:
            encoded = base64.b64encode(raw)
            self.server.bodystructure = ('("' + kind + '" "PLAIN" ("CHARSET" "utf-8") NIL NIL "BASE64" ' + str(len(encoded)) + ' 1)').encode()
            self.server.part_bodies = {'1': encoded}
            self.env['IMAP_CLIENT_SECRET'] = 'secret-bytes'
            answer = self.call('part 7 1 --big saved' if kind == 'TEXT' else json.dumps({'method': 'part', 'uid': 7, 'part_id': '1', 'big': 'saved'}))
            self.assertFalse(answer.startswith('error:'), answer)
            big.put.assert_called_with('saved', raw)
            self.assertIn('size ' + str(len(raw)), answer)
            self.assertIn(hashlib.sha256(raw).hexdigest(), answer)
            self.assertNotIn('secret-bytes', answer)
            self.assertNotIn(encoded.decode(), answer)
        self.assertEqual(big.put.call_count, 2)
        self.assertTrue(all(args[0] == self.bus.name for args, _ in self.bus.record.call_args_list))
        self.assertEqual(self.server.stores, [])
        self.assertFalse(any(b' SELECT ' in c for c in self.server.commands))
        self.assertTrue(all(b'BODY.PEEK[' in args for _, args in self.server.fetches if b'BODYSTRUCTURE' not in args))

    def test_big_part_empty_refused_without_upload(self):
        big = self.big_store()
        self.server.bodystructure = b'("TEXT" "PLAIN" NIL NIL NIL "7BIT" 0 1)'
        self.server.part_bodies = {'1': b''}
        self.assertIn('empty', self.call('part 7 1 --big empty'))
        big.put.assert_not_called()

    def test_big_part_predecode_cap_stores_nothing(self):
        big = self.big_store()
        self.server.bodystructure = b'("APPLICATION" "OCTET-STREAM" NIL NIL NIL "BASE64" 26214401 NIL NIL)'
        self.server.part_bodies = {'1': b'YQ=='}
        self.assertIn('25 MB', self.call('part 7 1 --big large'))
        big.put.assert_not_called()
        self.assertFalse(any(b'BODY.PEEK[1]' in a for _, a in self.server.fetches))

    def test_big_part_bus_refusal_and_unknown_upload_not_retried(self):
        big = self.big_store()
        self.server.part_bodies = {'1': b'hello'}
        for error, answer in [(Refusal(403, 'fixture'), 'access refused'),
                              (UncertainOutcome('fixture'), 'outcome unknown')]:
            big.put.reset_mock()
            big.put.side_effect = error
            self.assertIn(answer, self.call('part 7 1 --big saved'))
            self.assertEqual(big.put.call_count, 1)

    def test_big_append_exact_size_boundary(self):
        self.enable()
        raw = b'Subject: limit\r\n\r\nx'
        big = self.big_store(raw)
        with patch.object(folder_admin, 'MAIL_CAP', len(raw)):
            self.assertIn('destination UID', self.call('append Other --big source/mail'))
            big.get.assert_called_with('mail', max_size=len(raw))
        self.assertEqual(self.server.appended[0][1], raw)

    def test_big_append_exact_binary_bounded_source_record_uid_and_missing_uid(self):
        self.enable()
        raw = b'Subject: raw bytes\r\n\r\n\xff\x00unchanged'
        big = self.big_store(raw)
        for mode, expected in [('valid', 'destination UID'), ('missing', 'UID unavailable')]:
            self.server.appenduid_mode = mode
            answer = self.call('append Other --big "user/folder@fixture/mail" --flags answered --date 2026-10-31T01:02:03Z' if mode == 'valid' else json.dumps({'method': 'append', 'folder': 'Other', 'big': 'user/folder@fixture/mail', 'flags': ['answered']}))
            self.assertIn(expected, answer)
        self.assertEqual(len(self.server.appended), 2)
        self.assertTrue(all(row[1] == raw for row in self.server.appended))
        big.get.assert_called_with('mail', max_size=26214400)
        self.assertEqual(big.get.call_count, 2)
        self.assertTrue(any(c.args == ('user/folder@fixture',) for c in self.bus.record.call_args_list))
        self.assertEqual(self.server.mailboxes, [])

    def test_big_append_switch_and_admission_refuse_before_any_write(self):
        self.folder('Other', '')
        big = self.big_store()
        self.assertIn('allow_destructive', self.call('append Other --big source/mail'))
        big.get.assert_not_called()
        self.assertEqual(self.server.commands, [])
        self.cfg['allow_destructive'] = True
        big.get.side_effect = Refusal(403, 'fixture denied')
        self.assertIn('big key access refused', self.call('append Other --big source/mail'))
        self.assertEqual(self.server.commands, [])
        self.assertEqual(self.writes(), [])
        self.assertEqual(big.get.call_count, 1)

    def test_big_append_cap_before_connection_and_no_retry_unknown_write(self):
        self.enable()
        big = self.big_store()
        with patch.object(folder_admin, 'MAIL_CAP', 10):
            self.assertIn('25 MB', self.call('append Other --big source/mail'))
        self.assertEqual(self.server.commands, [])
        for error, expected in [(InvalidArgument('oversize'), '25 MB'), (ValueError('bad hash'), 'integrity verification')]:
            big.get.side_effect = error
            self.assertIn(expected, self.call('append Other --big source/mail'))
            self.assertEqual(self.server.commands, [])
            self.assertEqual(self.writes(), [])
        big.get.side_effect = None
        self.server.drop_after_admin = True
        answer = self.call('append Other --big source/mail')
        self.assertIn('outcome unknown', answer)
        self.assertEqual(len(self.server.appended), 1)
        self.assertEqual(len(self.writes()), 1)

    def test_big_transfer_through_real_client_core_api_and_instance_admission(self):
        from test_subscriptions_bus import Fixture
        live = Fixture(self)
        self.addCleanup(live.close)
        address = urlparse(live.info['addr'])
        self.assertTrue(ipaddress.ip_address(address.hostname).is_loopback)
        allowed = {('127.0.0.1', self.server.server_address[1]), (address.hostname, address.port)}
        def only_fixtures(address, *args, **kwargs):
            if address not in allowed: raise AssertionError('external connection forbidden')
            return CONNECT(address, *args, **kwargs)
        patch('socket.create_connection', side_effect=only_fixtures).start()
        owner, caller, imap, stranger = [live.client(name) for name in
            ('owner@fixture', '#caller@fixture', '#worker@fixture', '#outsider@fixture')]
        owner.record(imap.name).manage(allow=[caller.name])
        owner.record(caller.name).manage(allow=[])
        agent = fixture.proxy.Agent(imap, self.cfg, self.env, self.work.name)
        self.addCleanup(agent.close)
        def request(body):
            caller.send(imap.name, body)
            message = imap.consume(wait=.2)
            self.assertIsNotNone(message)
            self.assertEqual(message.envelope['from'], caller.name)
            return agent.handle(message)
        # Larger than a bus message, with binary and non-UTF-8 bytes throughout.
        raw = b'Subject: real big transfer\r\n\r\n' + bytes(range(256)) * 4097
        self.assertGreater(len(raw), 1048576)
        self.server.bodystructure = ('("APPLICATION" "OCTET-STREAM" NIL NIL NIL "BINARY" ' + str(len(raw)) + ' NIL NIL)').encode()
        self.server.part_bodies = {'1': raw}
        answer = request('part 7 1 --big decoded')
        self.assertIn('size ' + str(len(raw)), answer)
        self.assertIn(hashlib.sha256(raw).hexdigest(), answer)
        downloaded, info = caller.record(imap.name).big.get('decoded')
        self.assertEqual(downloaded, raw)
        self.assertEqual(info['sha256'], hashlib.sha256(raw).hexdigest())
        self.assertEqual(self.server.stores, [])
        with self.assertRaises(Refusal) as denied:
            stranger.record(imap.name).big.get('decoded')
        self.assertEqual(denied.exception.status, 403)
        caller.record(caller.name).big.put('mail', raw)
        self.folder('Other', '')
        command = 'append Other --big ' + caller.name + '/mail'
        before = list(self.server.commands)
        self.assertIn('allow_destructive', request(command))
        self.cfg['allow_destructive'] = True
        self.assertIn('big key access refused', request(command))
        self.assertEqual(self.server.commands, before)
        owner.record(caller.name).manage(allow=[imap.name])
        self.assertIn('destination UID', request(command))
        self.assertEqual(len(self.server.appended), 1)
        self.assertTrue(self.server.appended[0][1] == raw, 'APPEND changed verified big bytes')
        self.assertEqual(self.server.mailboxes, [b'"INBOX"'])  # part EXAMINE only
        owner.record(caller.name).manage(allow=[])
        before = list(self.server.commands)
        self.assertIn('big key access refused', request(command))
        self.assertEqual(self.server.commands, before)
        self.assertEqual(len(self.server.appended), 1)

    def test_create_ungated_and_utf7_quoted_without_selection(self):
        name = '邮件 & "box"'
        result = self.call('folder create ' + json.dumps(name, ensure_ascii=False))
        self.assertIn('completed', result)
        self.assertIn(name, self.server.folders)
        self.assertIn(b' CREATE ' + wire.mailbox(name).encode(), self.writes()[0])
        self.assertEqual(self.server.mailboxes, [])
        self.assertEqual(self.server.messages, {7: fixture.RAW})
        self.assertIn('completed', self.call('{"method":"folder create","name":"JSON"}'))

    def test_off_gate_rename_delete_append_before_connection(self):
        self.folder('Other', '')  # A real subject; only the switch may refuse it.
        for text in ['folder rename Other New', 'folder delete Other --force', 'append Other\n' + fixture.RAW.decode()]:
            answer = self.call(text)
            self.assertIn('requires Configuration allow_destructive=true', answer)
            self.assertEqual(self.server.commands, [])
        self.assertTrue(self.call('{"method":"folder.delete","name":"Other","allow_destructive":true}').startswith('error:'))
        self.assertEqual(self.server.commands, [])

    def test_rename_quoted_subtree_preserves_other_mail_and_no_select(self):
        self.enable()
        child = self.folder('Other/Child', '')
        child['messages'][4] = b'child'
        self.other['messages'][6] = b'parent'
        answer = self.call('folder rename Other "档案 &"')
        self.assertIn('completed', answer)
        self.assertIn('档案 &', self.server.folders)
        self.assertIn('档案 &/Child', self.server.folders)
        self.assertEqual(self.server.folders['档案 &']['messages'], {6: b'parent'})
        self.assertEqual(self.server.folders['档案 &/Child']['messages'], {4: b'child'})
        self.assertEqual(self.server.messages, {7: fixture.RAW})
        self.assertEqual(len(self.writes()), 1)

        self.assertEqual(self.server.mailboxes, [])

    def test_inbox_never_renamed_deleted_or_rename_destination(self):
        self.enable()
        for text in ['folder rename inBox New', 'folder delete INbox --force', 'folder rename Other INBOX']:
            self.assertIn('INBOX', self.call(text))
        self.assertEqual(self.writes(), [])
        self.assertIn('Other', self.server.folders)

    def test_inbox_delete_guard_independent_of_configuration_and_role_metadata(self):
        self.enable()
        self.cfg['mailbox'] = 'Other'
        # row() normally synthesizes Inbox even without a SPECIAL-USE flag.
        # With that independent protection absent, the explicit guard must
        # still refuse before STATUS/DELETE, including when --force is true.
        rows = [{'name':'INBOX', 'role':'', 'special_use':False,
                 'delimiter':'/', 'selectable':True}]
        with patch.object(folder_admin.folders, 'listing', return_value=rows):
            self.assertIn('INBOX cannot be renamed or deleted', self.call('folder delete INBOX --force'))
        self.assertEqual(self.writes(), [])
        self.assertFalse(any(b' STATUS ' in c for c in self.server.commands))
        self.assertEqual(self.server.messages, {7: fixture.RAW})

    def test_configured_mailbox_and_ancestor_protect_current_subscriptions(self):
        self.enable()
        self.folder('Other/Live', '')
        self.cfg['mailbox'] = 'Other/Live'
        for name in ['Other', 'Other/Live']:
            for text in [f'folder rename {name} New', f'folder delete {name} --force']:
                self.assertIn('configured mailbox', self.call(text))
        self.assertEqual(self.writes(), [])
        # A shared textual prefix without the hierarchy separator is not an ancestor.
        self.folder('Oth', '')
        self.assertIn('completed', self.call('folder delete Oth'))

    def test_special_use_role_and_ancestors_force_cannot_override(self):
        self.enable()
        for index, role in enumerate(['\\Sent', '\\Drafts', '\\Trash', '\\Junk', '\\Archive', '\\All', '\\Flagged', '\\Important']):
            name = 'Role' + str(index)
            self.folder(name, role)
            self.assertIn('role folders', self.call('folder delete ' + name + ' --force'))
        self.folder('Other/Bin', '\\Trash')
        self.assertIn('role folders', self.call('folder delete Other --force'))
        self.assertEqual(self.writes(), [])
        # A folder named Trash without SPECIAL-USE is ordinary, never guessed.
        self.folder('Trash', '')
        self.assertIn('completed', self.call('folder delete Trash'))

    def test_hierarchy_uses_server_delimiter_and_nil_is_flat(self):
        self.enable()
        self.server.delimiter = '.'
        self.folder('Other.Live', '')
        self.cfg['mailbox'] = 'Other.Live'
        self.assertIn('configured mailbox', self.call('folder rename Other New'))
        self.assertEqual(self.writes(), [])
        self.server.delimiter = None
        self.assertIn('completed', self.call('folder delete Other'))

    def test_nonempty_requires_force_and_force_deletes_only_that_folder(self):
        self.enable()
        self.other['messages'][7] = fixture.RAW
        keep = self.folder('Keep', '')
        keep['messages'][7] = b'keep'
        self.assertIn('requires --force', self.call('folder delete Other'))
        self.assertEqual(self.writes(), [])
        self.assertIn('completed', self.call('folder delete Other --force'))
        self.assertNotIn('Other', self.server.folders)
        self.assertEqual(keep['messages'], {7: b'keep'})
        self.assertEqual(self.server.messages, {7: fixture.RAW})
        self.assertEqual([c.split()[1] for c in self.writes()], [b'DELETE'])
        self.assertEqual(self.server.mailboxes, [])

    def test_missing_folder_refused_without_write(self):
        self.enable()
        for text in ['folder rename Missing New', 'folder delete Missing --force', 'append Missing\n' + fixture.RAW.decode()]:
            self.assertIn('folder not found', self.call(text))
        self.assertEqual(self.writes(), [])

    def test_existing_rename_destination_and_noselect_append_refused(self):
        self.enable()
        self.folder('Existing', '')
        self.folder('Parent', '\\Noselect')
        self.assertIn('already exists', self.call('folder rename Other Existing'))
        self.assertIn('not selectable', self.call('append Parent\n' + fixture.RAW.decode()))
        self.assertEqual(self.writes(), [])

    def test_append_exact_mail_flags_date_uid_and_utf7_without_select(self):
        self.enable()
        name = '邮件 &'
        data = self.folder(name, '')
        raw = 'Subject: café\r\n\r\nexternal text\r\n'
        result = self.call('append ' + json.dumps(name, ensure_ascii=False) + ' --flags seen,$Tag --date 2026-10-31T01:02:03+02:00\n' + raw)
        self.assertIn('destination UID 100, UIDVALIDITY 3', result)
        self.assertEqual(data['messages'], {100: raw.encode()})
        self.assertEqual(data['flags'][100], {b'\\Seen', b'$Tag'})
        self.assertEqual(self.server.appended, [(name, raw.encode(), b'(\\Seen $Tag)', b'31-Oct-2026 01:02:03 +0200')])
        self.assertEqual(self.server.messages, {7: fixture.RAW})
        self.assertEqual(self.server.mailboxes, [])
        self.assertEqual(len(self.writes()), 1)
        data['next_uid'] = 1234
        self.assertIn('destination UID 1234, UIDVALIDITY 3', self.call(json.dumps({'method':'append', 'folder':name, 'raw':raw})))

    def test_append_json_and_missing_or_invalid_appenduid_never_retry(self):
        self.enable()
        for mode, expected in [('valid', 'destination UID 100'), ('missing', 'UID unavailable'), ('bad', 'invalid APPENDUID'), ('overflow', 'invalid APPENDUID')]:
            self.server.appenduid_mode = mode
            before = len(self.writes())
            self.assertIn(expected, self.call(json.dumps({'method':'append','folder':'Other','raw':fixture.RAW.decode(),'flags':['answered']})))
            self.assertEqual(len(self.writes()) - before, 1)
        self.assertEqual(len(self.server.appended), 4)

    def test_append_has_its_own_limit_above_ordinary_call_limit(self):
        self.enable()
        raw = 'Subject: large\r\n\r\n' + 'x' * 40000
        self.assertIn('destination UID 100', self.call('append Other\n' + raw))
        self.assertEqual(self.other['messages'][100], raw.encode())
        self.assertIn('destination UID 101', self.call(json.dumps({'method':'append','folder':'Other','raw':raw})))
        self.assertEqual(self.other['messages'][101], raw.encode())

    def test_refused_or_unknown_writes_not_repeated(self):
        self.enable()
        for body in ['folder create New', 'folder rename Other New', 'folder delete Other', 'append Other\n' + fixture.RAW.decode()]:
            self.server.admin_refused = True
            before = len(self.writes())
            self.assertIn('refused; no retry', self.call(body))
            self.assertEqual(len(self.writes()) - before, 1)
        self.server.admin_refused = False
        self.server.drop_after_admin = True
        for body in ['folder create New', 'folder rename Other Renamed', 'folder delete Renamed', 'append New\n' + fixture.RAW.decode()]:
            before = len(self.writes())
            self.assertIn('outcome unknown', self.call(body))
            self.assertEqual(len(self.writes()) - before, 1)
        self.assertEqual(len(self.server.appended), 1)

    def test_request_mail_caps_and_bad_date_flags_refuse_before_connection(self):
        self.enable()
        too_big = 'append Other\nSubject: cap\r\n\r\n' + 'x' * folder_admin.REQUEST_CAP
        self.assertIn('1048576-byte bus request cap', self.call(too_big))
        self.assertIn('1048576-byte bus request cap', self.call(json.dumps({'method':'append','folder':'Other','raw':'x' * folder_admin.REQUEST_CAP})))
        with patch.object(folder_admin, 'MAIL_CAP', 10):
            self.assertIn('25 MB', self.call('append Other\n' + fixture.RAW.decode()))
        for options in ['--flags deleted', '--flags "\\\\Deleted"', '--date 2026-02-30T01:02:03Z', '--date 2026-01-01', '--date "2026-10-31T01:02:03Z\r\nX"']:
            self.assertTrue(self.call('append Other ' + options + '\n' + fixture.RAW.decode()).startswith('error:'))
        self.assertEqual(self.server.commands, [])


class Arguments(unittest.TestCase):
    def test_folder_subcalls_keep_ordinary_32768_byte_cap(self):
        for action, fields in [('create', {'name':'Other'}),
                               ('rename', {'name':'Other','new':'New'}),
                               ('delete', {'name':'Other'})]:
            text = 'folder ' + action + ' Other' + (' New' if action == 'rename' else '')
            encoded = json.dumps({'method':'folder ' + action, **fields})
            for body in [text, encoded[:-1]]:
                # Exactly the ordinary byte cap remains valid, with whitespace
                # before the JSON brace so method/name themselves stay small.
                suffix = '}' if body.startswith('{') else ''
                boundary = body + ' ' * (32768 - len((body + suffix).encode())) + suffix
                self.assertEqual(fixture.proxy.parse(boundary)['method'], 'folder.' + action)
                for oversized in [boundary[:-len(suffix)] + ' ' + suffix if suffix else boundary + ' ',
                                  body + ' ' * 40000 + suffix]:
                    with self.subTest(action=action, json=bool(suffix), size=len(oversized.encode())):
                        with self.assertRaisesRegex(ValueError, '32768'):
                            fixture.proxy.parse(oversized)

    def test_list_delimiter_and_role_metadata(self):
        import folders
        self.assertEqual(folders.row(b'() NIL "Box"')['delimiter'], '')
        self.assertEqual(folders.row(b'() "." "Box"')['delimiter'], '.')
        self.assertTrue(folders.row(b'(\\All) "/" "Everything"')['special_use'])
        for raw in [b'() "xx" "Box"', b'() "" "Box"']:
            with self.assertRaises(wire.WireError): folders.row(raw)

    def test_date_requires_timezone_and_flags_are_validated(self):
        for date in ['2026-01-01', '2026-01-01T01:02:03', '2026-02-30T01:02:03Z']:
            with self.assertRaises(ValueError):
                fixture.proxy.parse(json.dumps({'method':'append','folder':'Other','raw':'Subject: x\r\n\r\ny','date':date}))
        for flag in ['deleted', '\\Deleted', '\\Recent', 'bad flag']:
            with self.assertRaises(ValueError):
                fixture.proxy.parse(json.dumps({'method':'append','folder':'Other','raw':'Subject: x\r\n\r\ny','flags':[flag]}))

    def test_strict_text_json_shapes_and_injection(self):
        for body in ['folder create "bad\r\nDELETE INBOX"', 'folder delete Other --force=false',
                     'folder create Other\nraw', 'folder rename Other', 'folder unknown Other',
                     'append Other', 'append Other --flags a --flags b\nSubject: x\r\n\r\ny',
                     '{"method":"folder.delete","name":"Other","force":"true"}',
                     '{"method":"folder.rename","name":"Other","new":"New","force":true}',
                     '{"method":"append","folder":"Other","raw":"x","flags":["\\\\Deleted"]}']:
            with self.subTest(body=body), self.assertRaises((ValueError, wire.WireError)):
                fixture.proxy.parse(body)
