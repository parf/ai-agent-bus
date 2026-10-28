import io
import contextlib
import unittest
from unittest.mock import Mock, patch
from test_imap import proxy, RAW


class PureIMAP(unittest.TestCase):
    def test_parse_and_configuration_are_strict_before_io(self):
        self.assertEqual(proxy.parse('read 7'), {'method': 'read', 'uid': 7})
        self.assertEqual(proxy.parse('{"method":"sync","limit":2}'), {'method': 'sync', 'limit': 2})
        for body in ('read 0', 'read 7\r\nLOGOUT', 'search x\r\nLOGIN', '{"method":"read","uid":7,"target":"#other"}',
                     '{"method":"read","method":"read","uid":7}', '{"method":"sync","limit":true}'):
            with self.assertRaises(ValueError):
                proxy.parse(body)
        self.assertEqual(proxy.config({})['host'], 'imap.gmail.com')
        for cfg in ({'mode': 'plain'}, {'port': True}, {'mailbox': 'INBOX\r\nLOGOUT'}, {'cache_bytes': 3}, {'token_url': 'https://evil.example'}):
            with self.assertRaises(ValueError):
                proxy.config(cfg)

    def test_external_publication_warning_bound_and_redaction(self):
        self.assertEqual(proxy.render(RAW, []), proxy.WARNING + RAW.decode())
        raw = b'x' * (proxy.MAX_OUTPUT - len(proxy.WARNING.encode()))
        self.assertEqual(len(proxy.render(raw, []).encode()), proxy.MAX_OUTPUT)
        with self.assertRaises(proxy.MailError):
            proxy.render(raw + b'x', [])
        self.assertEqual(proxy.render(b'fixture-private', ['fixture-private']), proxy.WARNING + '***')
        self.assertEqual(proxy.render(b'\xff', []), proxy.WARNING + r'\xff')

    def test_modified_utf7_quotes_and_controls(self):
        self.assertEqual(proxy.wire.mailbox('A&B'), '"A&-B"')
        self.assertEqual(proxy.wire.mailbox('台北'), '"&U,BTFw-"')
        self.assertEqual(proxy.wire.mailbox('a"b\\c'), '"a\\"b\\\\c"')
        with self.assertRaises(ValueError):
            proxy.wire.mailbox('a\x00b')

    def test_main_reader_ignores_delivery_failure_and_continues(self):
        first = Mock(receipt=None, re=None)
        first.reply.side_effect = proxy.BusError('fixture transport closed')
        second = Mock(receipt=None, re=None)
        bus = Mock()
        bus.consume.side_effect = [first, second, proxy.Shutdown()]
        context = Mock()
        context.__enter__ = Mock(return_value=bus)
        context.__exit__ = Mock(return_value=False)
        agent = Mock()
        agent.handle.return_value = 'private fixture answer'
        log = io.StringIO()
        with patch.object(proxy, 'Bus', return_value=context), patch.object(proxy, 'Agent', return_value=agent), \
             patch.object(proxy.signal, 'signal'), patch.dict(proxy.os.environ, {'AGENT_BUS_CONFIG': '{}'}), \
             contextlib.redirect_stderr(log):
            self.assertEqual(proxy.main(), 0)
        first.reply.assert_called_once_with('private fixture answer')
        second.reply.assert_called_once_with('private fixture answer')
        self.assertEqual(agent.handle.call_count, 2)
        self.assertTrue(all(call.kwargs == {'wait': 10} for call in bus.consume.call_args_list))
        self.assertIn('answer delivery failed', log.getvalue())
        self.assertNotIn('private fixture answer', log.getvalue())

    def test_wire_line_and_literal_limits_precede_allocation(self):
        stream = Mock()
        obj = object.__new__(proxy.wire.TLS)
        obj.response_bytes = 0
        obj.__dict__['_file'] = stream
        with self.assertRaises(proxy.wire.WireError):
            obj.read(proxy.wire.MAX_MAIL + 1)
        stream.read.assert_not_called()
        obj.__dict__['_file'] = io.BytesIO(b'x' * (proxy.wire.MAX_LINE + 1))
        with self.assertRaises(proxy.wire.WireError):
            obj.readline()
        obj.__dict__['_file'] = io.BytesIO(b'hello\r\n')
        obj.response_bytes = proxy.wire.MAX_RESPONSE
        with self.assertRaises(proxy.wire.WireError):
            obj.readline()
        obj.__dict__['_file'] = io.BytesIO(b'x')
        obj.response_bytes = 0
        with self.assertRaises(proxy.wire.Tainted):
            obj.read(5)
        connection = Mock()
        connection.uid.return_value = ('OK', [b'1001'])
        with self.assertRaises(ValueError):
            proxy.wire.search(connection, 1, 1000)


if __name__ == '__main__':
    unittest.main()
