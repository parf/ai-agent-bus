import imaplib
import unittest
from unittest.mock import Mock, patch

import test_imap
from mailbox_session import Session, SessionError
from test_forwarding import Clock
import wire


class SessionTests(unittest.TestCase):
    def setUp(self):
        self.clock = Clock()
        self.connection = Mock(capabilities=('AUTH=XOAUTH2',))
        self.session = Session(lambda: test_imap.proxy.config({}),
                               {'IMAP_USERNAME': 'fixture@example.test', 'IMAP_PASSWORD': 'fixture-private'},
                               '.', self.clock)
        self.addCleanup(self.session.close)

    def test_connection_is_kept_without_resolving_again_on_each_poll(self):
        with patch('mailbox_session.wire.TLS', return_value=self.connection) as connect, \
             patch('mailbox_session.wire.selected', return_value=(7, 100)) as examine:
            for _ in range(3):
                self.session.snapshot()
                self.clock.advance(60)
            self.assertEqual(connect.call_count, 1)
            self.assertEqual(examine.call_count, 3)

    def test_authentication_backoff_is_900_1800_3600_and_success_resets_it(self):
        self.connection.login.side_effect = imaplib.IMAP4.error('fixture-private')
        with patch('mailbox_session.wire.TLS', return_value=self.connection) as connect, \
             patch('mailbox_session.wire.selected', return_value=(7, 100)):
            for delay in (900, 1800, 3600, 3600):
                now = self.clock()
                with self.assertRaises(SessionError):
                    self.session.snapshot()
                self.assertEqual(self.session.next_connect - now, delay)
                calls = connect.call_count
                self.clock.advance(delay - 1)
                with self.assertRaises(SessionError):
                    self.session.snapshot()
                self.assertEqual(connect.call_count, calls)
                self.clock.advance(1)
            self.connection.login.side_effect = None
            self.session.snapshot()
            self.assertEqual(self.session.auth_failures, 0)

    def test_throttle_and_truncation_are_session_failures_not_uid_refusals(self):
        self.session.connection = self.connection
        def fail(error):
            raise error
        for error in (wire.Refused('throttled', throttled=True), wire.Truncated('short read')):
            self.session.connection = self.connection
            with self.assertRaises(SessionError):
                self.session.operation(lambda: fail(error))
            self.assertIsNone(self.session.connection)
        self.session.connection = self.connection
        with self.assertRaises(wire.LiteralTooLarge):
            self.session.operation(lambda: fail(wire.LiteralTooLarge('oversized literal')))

    def test_header_literal_cap_applies_before_reading_any_literal_bytes(self):
        connection = object.__new__(wire.TLS)
        stream = Mock()
        connection.__dict__['_file'] = stream
        connection.response_bytes = 0
        connection.uid = Mock(side_effect=lambda *_: connection.read(8193))
        with self.assertRaises(wire.LiteralTooLarge):
            wire.headers(connection, 100)
        stream.read.assert_not_called()
        self.assertEqual(connection.literal_limit, wire.MAX_MAIL)

    def test_xoauth2_refreshes_once_per_actual_reconnect_only(self):
        self.session.settings = lambda: test_imap.proxy.config({'auth': 'xoauth2'})
        with patch('mailbox_session.google_access_token', return_value='fixture-access-token') as token, \
             patch('mailbox_session.wire.TLS', return_value=self.connection), \
             patch('mailbox_session.wire.selected', return_value=(7, 100)):
            self.session.snapshot()
            self.session.snapshot()
            self.assertEqual(token.call_count, 1)
            self.session.close()
            self.session.snapshot()
            self.assertEqual(token.call_count, 2)
            self.assertEqual(self.connection.login.call_count, 0)


if __name__ == '__main__':
    unittest.main()
