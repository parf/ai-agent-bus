"""Step-2 operations use only a local TLS fixture or explicit protocol mocks."""
import io
import json
import unittest
from unittest.mock import Mock, patch
import test_imap as fixture
import operations
import mime_parts
import wire
from mail import MailError


class Arguments(unittest.TestCase):
    def test_uid_sets_bound_before_expansion(self):
        self.assertEqual(operations.uid_set('7,10:12'), [7, 10, 11, 12])
        self.assertEqual(len(operations.uid_set('1:1000')), 1000)
        for value in ('1:1001', '1:4294967295', '9:1', '1,1', '0', '*', '1:*', '1 EXPUNGE', True):
            with self.subTest(value=value), self.assertRaises(ValueError): operations.uid_set(value)

    def test_thousand_large_uids_fit_the_call_bound(self):
        numbers = list(range(1000000000, 1000001000))
        body = 'headers ' + ','.join(map(str, numbers))
        try: call = fixture.proxy.parse(body)
        except ValueError as error: self.fail('valid 1000-UID call refused: ' + str(error))
        self.assertEqual(call['uids'], numbers)
        with self.assertRaises(ValueError): fixture.proxy.parse(body + ' ' * 32768)

    def test_deleted_refused_before_connect(self):
        for flag in ('deleted', '\\Deleted', '\\deleted', 'DELETED'):
            with self.subTest(flag=flag), self.assertRaisesRegex(MailError, 'Deleted'):
                operations.parse_call(json.dumps({'method': 'flag', 'uids': '7', 'action': 'add', 'flags': [flag]}))
        self.assertEqual(operations.flag('flagged'), '\\Flagged')
        self.assertEqual(operations.flag('$Fixture'), '$Fixture')

    def test_text_json_folder_and_search_and(self):
        call = operations.parse_call('search local words --from "a@example.com" --to b@example.com --subject "a b" --header X-Tag:blue --since 01-Oct-2026 --before 01-Nov-2026 --flag seen --no-flag flagged --larger 20 --smaller 500 --limit 7 --folder "Other Box"')
        self.assertEqual(call['text'], 'local words')
        self.assertEqual(call['folder'], 'Other Box')
        self.assertEqual(call['limit'], 7)
        self.assertEqual(operations.search_args(call), ['FROM', '"a@example.com"', 'TO', '"b@example.com"', 'SUBJECT', '"a b"', 'HEADER', '"X-Tag"', '"blue"', 'SINCE', '"01-Oct-2026"', 'BEFORE', '"01-Nov-2026"', 'SEEN', 'UNFLAGGED', 'LARGER', '20', 'SMALLER', '500'])
        self.assertNotIn('local words', operations.search_args(call))
        self.assertEqual(operations.parse_call('search --since 2026-10-01')['since'], '01-Oct-2026')
        for body in ('flag 7 add "\\Deleted"', 'list --flag "a) STORE"', 'headers 7 --unknown x', 'part 7 1] ', 'search --from "x\r\ny"', 'search --since invalid', 'search --larger -1', 'list 101', 'headers 1:1001'):
            with self.subTest(body=body), self.assertRaises(ValueError): operations.parse_call(body)
        with self.assertRaises(ValueError):
            operations.parse_call('{"method":"mark","uid":7,"state":"read"}')
        self.assertEqual(operations.parse_call('{"method":"mark","uids":"7","state":"read"}')['uids'], [7])


class Calls(unittest.TestCase):
    setUpClass = classmethod(fixture.IMAPTests.setUpClass.__func__)
    setUp = fixture.IMAPTests.setUp
    call = fixture.IMAPTests.call

    def test_multi_mark_and_flag_per_uid_continue(self):
        self.server.messages[8] = fixture.RAW
        self.server.permanent = [b'\\Seen', b'\\Flagged', b'\\Answered', b'\\Draft', b'\\*']
        answer = self.call('mark 7,99,8 read')
        self.assertIn('UID 7 observed read', answer)
        self.assertIn('UID 99: error:', answer)
        self.assertIn('UID 8 observed read', answer)
        answer = self.call('flag 7,99,8 add flagged project-blue')
        self.assertIn('UID 99: error:', answer)
        for n in (7, 8): self.assertEqual(self.server.flags[n], {b'\\Seen', b'\\Flagged', b'project-blue'})
        self.assertIn('observed flags', answer)
        self.call('flag 7 remove flagged')
        self.assertEqual(self.server.flags[7], {b'\\Seen', b'project-blue'})
        self.assertTrue(all(b'Deleted' not in args for _, args in self.server.stores))

    def test_unknown_write_not_retried(self):
        self.server.permanent = [b'\\Seen', b'\\Flagged']
        self.server.drop_after_store = True
        self.assertIn('outcome unknown', self.call('flag 7 add flagged'))
        self.assertEqual(len(self.server.stores), 1)

    def test_unknown_uid_write_does_not_stop_later_uid_or_repeat_it(self):
        self.server.messages[8] = fixture.RAW
        self.server.permanent = [b'\\Seen', b'\\Flagged']
        self.server.drop_after_store = True
        answer = self.call('flag 7,8 add flagged')
        self.assertIn('UID 7: error: flag outcome unknown', answer)
        self.assertIn('UID 8: error: flag outcome unknown', answer)
        self.assertEqual([n for n, _ in self.server.stores], [7, 8])

    def test_search_and_list_live_flags_and_folder(self):
        self.server.messages[8] = fixture.RAW.replace(b'Subject:', b'Subject: LocalNeedle ')
        self.call('sync')
        self.server.flags[7] = {b'\\Flagged'}
        answer = self.call('list --flag flagged')
        self.assertIn('UID 7 ', answer)
        self.assertNotIn('UID 8 ', answer)
        self.assertIn('Flags: \\Flagged', answer)
        self.server.criteria_matches = [8]
        answer = self.call('search LocalNeedle --from a@example.com --limit 1')
        self.assertIn('UID 8 ', answer)
        self.assertNotIn('UID 7 ', answer)
        search = next(c for c in reversed(self.server.commands) if b'UID SEARCH FROM' in c)
        self.assertIn(b'"a@example.com"', search)
        self.assertNotIn(b'LocalNeedle', search)
        self.assertIn('UID 8 ', self.call('{"method":"search","from":"a@example.com"}'))

    def test_per_uid_errors_count_toward_answer_budget(self):
        def peek(_connection, number, _section, _cap):
            if number != 500: raise wire.WireError('missing UID')
            return b'Subject: ' + b'x' * 256000
        with patch.object(wire, 'peek', side_effect=peek):
            answer = self.call('headers 1:500')
        self.assertFalse(answer.startswith('error:'), answer[:100])
        self.assertLessEqual(len(answer.encode()), 262144)
        self.assertIn('UID 1: error: missing UID', answer)
        self.assertIn('UID 500: result exceeds remaining', answer)

    def test_utf8_search_is_enabled_before_selection(self):
        self.server.utf8 = True
        self.server.messages[7] = b'Subject: ' + 'Письмо'.encode() + b'\r\n\r\nbody'
        self.call('sync')
        self.server.commands.clear()
        answer = self.call('search --subject "Письмо"')
        self.assertIn('UID 7 ', answer)
        commands = self.server.commands
        enable = next(i for i, c in enumerate(commands) if b' ENABLE ' in c)
        select = next(i for i, c in enumerate(commands) if b' EXAMINE ' in c)
        self.assertLess(enable, select)
        self.assertTrue(any(b'UID SEARCH SUBJECT' in c and 'Письмо'.encode() in c for c in commands))
        self.server.utf8 = False
        self.server.commands.clear()
        self.assertIn('UTF8=ACCEPT', self.call('search --subject "Письмо"'))
        self.assertFalse(any(b' EXAMINE ' in c for c in self.server.commands))
        self.assertIn('UID 7 ', self.call('search Письмо'))

    def test_headers_and_part_never_seen(self):
        self.server.messages[8] = fixture.RAW
        self.server.part_bodies = {'1': b'hello'}
        answer = self.call('headers 7,99,8')
        self.assertIn('UID 7 headers', answer)
        self.assertIn('UID 99: error:', answer)
        self.assertIn('UID 8 headers', answer)
        self.assertNotIn(fixture.RAW.split(b'\r\n\r\n', 1)[1].decode(), answer)
        self.assertIn('hello', self.call('part 7 1'))
        self.assertIn('text/plain', self.call('parts 7'))
        self.assertEqual(self.server.stores, [])
        self.assertTrue(all(not fs for fs in self.server.flags.values()))
        fetches = [args for _, args in self.server.fetches if b'BODYSTRUCTURE' not in args]
        self.assertTrue(fetches)
        self.assertTrue(all(b'BODY.PEEK[' in args for args in fetches))
        self.assertFalse(any(b' SELECT ' in c for c in self.server.commands))

    def test_new_calls_route_other_folder_and_no_cached_uid_bleed(self):
        self.server.folder_rows.append(('Other Box', b'\\Archive'))
        self.server.folders['Other Box'] = {'messages': {7: b'Subject: OtherFolder\r\n\r\nother'}, 'flags': {}, 'validity': 2}
        self.assertIn('OtherFolder', self.call('headers 7 --folder "Other Box"'))
        self.assertIn('observed read', self.call('mark 7 read --folder "Other Box"'))
        self.assertEqual(self.server.flags, {})
        self.assertEqual(self.server.folders['Other Box']['flags'][7], {b'\\Seen'})
        self.assertIn('text/plain', self.call('parts 7 --folder "Other Box"'))
        self.assertTrue(all(b'"Other Box"' == m for m in self.server.mailboxes))

    def test_decoded_text_and_binary_parts_and_missing_id(self):
        self.server.bodystructure = b'("TEXT" "PLAIN" ("CHARSET" "utf-8") NIL NIL "BASE64" 8 1)'
        self.server.part_bodies = {'1': b'aGVsbG8='}
        answer = self.call('part 7 1')
        self.assertIn('hello', answer)
        self.assertNotIn('aGVsbG8=', answer)
        self.server.bodystructure = b'("APPLICATION" "OCTET-STREAM" NIL NIL NIL "QUOTED-PRINTABLE" 9 NIL NIL)'
        self.server.part_bodies = {'1': b'=00=01=02'}
        self.assertIn('AAEC', self.call('part 7 1'))
        self.assertIn('base64', self.call('part 7 1'))
        self.assertIn('part-id', self.call('part 7 2'))
        self.assertEqual(self.server.stores, [])

    def test_part_preflight_25mb_and_bus_answer_cap(self):
        self.server.bodystructure = b'("APPLICATION" "OCTET-STREAM" NIL NIL NIL "BASE64" 26214401 NIL NIL)'
        self.assertIn('25 MB', self.call('part 7 1'))
        self.assertFalse(any(b'BODY.PEEK[1]' in a for _, a in self.server.fetches))
        self.server.bodystructure = b'("TEXT" "PLAIN" NIL NIL NIL "7BIT" 300000 1)'
        self.server.part_bodies = {'1': b'a' * 300000}
        answer = self.call('part 7 1')
        self.assertFalse(answer.startswith('error:'), answer[:100])
        self.assertLessEqual(len(answer.encode()), 262144)
        self.assertIn('truncated', answer)
        self.env['IMAP_CLIENT_SECRET'] = 'a'
        answer = self.call('part 7 1')
        self.assertFalse(answer.startswith('error:'), answer[:100])
        self.assertLessEqual(len(answer.encode()), 262144)
        self.assertIn('***', answer)
        self.assertIn('truncated', answer)
        self.assertEqual(self.server.stores, [])
        self.server.literal_override = 26214401
        self.assertIn('25 MB', self.call('part 7 1'))


class Protocol(unittest.TestCase):
    def test_utf8_server_search_and_literal_filename(self):
        call = operations.parse_call('search --subject "Письмо"')
        c = Mock()
        c.capabilities = ('UTF8=ACCEPT', 'ENABLE')
        c.enable.return_value = ('OK', [b'enabled'])
        wire.prepare_search(c, operations.search_args(call))
        c.enable.assert_called_once_with('UTF8=ACCEPT')
        c.utf8_enabled = True
        c.uid.return_value = ('OK', [b'7'])
        self.assertEqual(wire.criteria_search(c, operations.search_args(call)), {7})
        self.assertEqual(c.uid.call_args.args[:3], ('SEARCH', None, b'SUBJECT'))
        self.assertIn('Письмо'.encode(), c.uid.call_args.args[-1])
        c.uid.return_value = ('OK', [(b'1 (UID 7 BODYSTRUCTURE ("APPLICATION" "PDF" NIL NIL NIL "BASE64" 4 NIL ("ATTACHMENT" ("FILENAME" {5}', b'a.pdf'), b'))))'])
        self.assertEqual(mime_parts.tree(c, 7)[0]['filename'], 'a.pdf')

    def test_multipart_tree_binary_and_predecode_cap(self):
        raw = b'1 (UID 7 BODYSTRUCTURE (("TEXT" "PLAIN" ("CHARSET" "utf-8") NIL NIL "7BIT" 5 1)("APPLICATION" "OCTET-STREAM" NIL NIL NIL "BASE64" 4 NIL ("ATTACHMENT" ("FILENAME" "a.bin"))) "MIXED"))'
        c = Mock()
        c.uid.return_value = ('OK', [raw])
        rows = mime_parts.tree(c, 7)
        self.assertEqual([r['id'] for r in rows], ['1', '2', 'root'])
        self.assertEqual(rows[1]['filename'], 'a.bin')
        self.assertEqual(rows[1]['disposition'], 'attachment')
        c.uid.return_value = ('OK', [raw.replace(b'UID 7 BODYSTRUCTURE ', b'BODYSTRUCTURE ')[:-1] + b' UID 7)'])
        self.assertEqual(mime_parts.tree(c, 7), rows)
        c.uid.return_value = ('OK', [raw])
        with patch.object(wire, 'peek', return_value=b'AAEC'):
            row, text, binary = mime_parts.payload(c, 7, '2')
            self.assertEqual(text, 'AAEC')
            self.assertTrue(binary)
        c.uid.return_value = ('OK', [raw.replace(b'"BASE64" 4', b'"BASE64" 26214401')])
        with patch.object(wire, 'peek') as peek, patch.object(mime_parts.base64, 'b64decode') as decode:
            with self.assertRaisesRegex(wire.TooLarge, '25 MB'): mime_parts.payload(c, 7, '2')
            peek.assert_not_called()
            decode.assert_not_called()

    def test_peek_checks_literal_cap_before_reading_and_restores_limits(self):
        c = object.__new__(wire.Plain)
        c.response_bytes = 0
        c.literal_limit = 25 * 1024 * 1024
        c.response_limit = c.literal_limit + 65536
        c._file = io.BytesIO(b'not read')
        with self.assertRaises(wire.WireError) as caught: c.read(c.literal_limit + 1)
        self.assertIsInstance(caught.exception, wire.LiteralTooLarge)
        self.assertEqual(c._file.tell(), 0)
        c = Mock(literal_limit=123, response_limit=456)
        c.uid.return_value = ('NO', [b'no'])
        with self.assertRaises(wire.Refused): wire.peek(c, 7, 'HEADER', 8192)
        self.assertEqual((c.literal_limit, c.response_limit), (123, 456))

    def test_flag_permission_and_unknown_outcome(self):
        c = Mock(seen_writable=True, permanent_flags={'\\seen'})
        c.uid.return_value = ('OK', [b'1 (UID 7 FLAGS (\\Flagged))'])
        with self.assertRaises(wire.WireError): wire.change_flags(c, 7, ['\\Flagged'], True)
        c.uid.assert_not_called()
        c.permanent_flags = {'\\flagged'}
        c.uid.side_effect = [('OK', [b'1 (UID 7 FLAGS ())']), ('OK', [b'done']), OSError('lost')]
        with self.assertRaises(wire.FlagOutcomeUnknown): wire.change_flags(c, 7, ['\\Flagged'], True)
        self.assertEqual(sum(call.args[0] == 'STORE' for call in c.uid.call_args_list), 1)
