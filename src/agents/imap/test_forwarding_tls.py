import unittest
from types import SimpleNamespace

import test_imap as fixture
from forwarding import Forwarder
from mailbox_session import Session
from test_forwarding import Bus, Clock, deliver, raw


class ForwardingTLS(unittest.TestCase):
    setUpClass = classmethod(fixture.IMAPTests.setUpClass.__func__)

    def setUp(self):
        fixture.IMAPTests.setUp(self)
        self.clock, self.forward_bus = Clock(), Bus()
        self.session = Session(lambda: fixture.proxy.config(self.cfg), self.env, self.work.name, self.clock)
        self.worker = Forwarder(self.forward_bus, self.forward_bus.name, self.work.name,
                                self.session, self.clock, self.clock, send_one=deliver(self.forward_bus))
        self.addCleanup(lambda: self.worker.close())
        self.author = SimpleNamespace(envelope={'from': '#author@t', 'roles': [], 'original_to': self.forward_bus.name})

    def test_real_tls_poll_is_read_only_new_only_and_cached_after_restart(self):
        self.worker.add(self.author, '#destination@t', 'invoice')
        self.assertEqual(self.server.fetches, [])
        self.worker.tick()
        self.assertEqual(self.forward_bus.sent, [])
        self.server.messages[8] = raw()
        self.clock.advance(60)
        self.worker.tick()
        self.assertEqual(len(self.forward_bus.sent), 1)
        bodies = [item for item in self.server.fetches if item[1] == b'(UID BODY.PEEK[])']
        self.assertEqual(bodies, [(8, b'(UID BODY.PEEK[])')])
        self.assertTrue(any(b'BODY.PEEK[HEADER.FIELDS' in args for _, args in self.server.fetches))
        self.worker.close()
        self.session = Session(lambda: fixture.proxy.config(self.cfg), self.env, self.work.name, self.clock)
        self.worker = Forwarder(self.forward_bus, self.forward_bus.name, self.work.name,
                                self.session, self.clock, self.clock, send_one=deliver(self.forward_bus))
        self.clock.advance(60)
        self.worker.tick()
        self.assertEqual(len(self.forward_bus.sent), 1)
        self.assertEqual([item for item in self.server.fetches if item[1] == b'(UID BODY.PEEK[])'], bodies)
        for command in self.server.commands:
            verb = command.strip().split(b' ', 2)[1]
            self.assertIn(verb, (b'CAPABILITY', b'LOGIN', b'EXAMINE', b'UID', b'STATUS'))
            self.assertNotIn(b'STORE', command)
            if b'BODY' in command:
                self.assertIn(b'BODY.PEEK', command)

    def test_an_already_cached_uid_needs_no_remote_header_or_body_fetch(self):
        self.worker.add(self.author, '#destination@t')
        self.server.messages[8] = raw()
        with self.session.cache() as cache:
            cache.put(8, raw())
        self.clock.advance(60)
        self.worker.tick()
        self.assertEqual(len(self.forward_bus.sent), 1)
        self.assertEqual(self.server.fetches, [])
