import contextlib
import io
import tempfile
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

from test_imap import proxy
from test_forwarding import Bus
from outbox import Outbox


class ControlTests(unittest.TestCase):
    def test_text_and_json_subscription_forms_and_verbatim_filter(self):
        self.assertEqual(proxy.parse('sub add #TARGET@t /^a  b$/i'),
                         {'method': 'sub.add', 'target': '#target@t', 'filter': '/^a  b$/i'})
        self.assertEqual(proxy.parse('{"method":"sub.add","target":"#target@t","filter":"/^a  b$/i"}'),
                         proxy.parse('sub add #target@t /^a  b$/i'))
        self.assertEqual(proxy.parse('sub list'), {'method': 'sub.list'})
        self.assertEqual(proxy.parse('sub delete aabbccdd'), {'method': 'sub.delete', 'id': 'aabbccdd'})
        self.assertEqual(proxy.parse('{"method":"sub delete","id":"aabbccdd"}'), proxy.parse('sub delete aabbccdd'))
        self.assertEqual(proxy.parse('sub add #target@t')['filter'], '')
        for body in ('sub add', 'sub delete', 'sub delete ../x', 'sub list extra',
                     '{"method":"sub.add","target":"#target@t","created_by":"owner@t"}',
                     '{"method":"sub.add","target":"#target@t","filter":true}',
                     '{"method":"sub.list","id":"aabbccdd"}', '{"method":{}}',
                     '{"method":"sub.list","method":"sub.list"}'):
            with self.subTest(body=body), self.assertRaises(ValueError):
                proxy.parse(body)

    def test_add_reply_warns_match_all_and_uses_only_authenticated_envelope(self):
        agent = proxy.Agent(None, {'invalid': True}, {}, '.')
        agent.forwarder = Mock()
        agent.forwarder.add.return_value = {'id': 'aabbccdd', 'target': '#target@t', 'created_by': '#author@t'}
        message = SimpleNamespace(ask=None, body='sub add #target@t', envelope={'from': '#author@t'})
        with contextlib.redirect_stderr(io.StringIO()):
            answer = agent.handle(message)
        self.assertEqual(answer, 'added aabbccdd -> #target@t; by #author@t; matches ALL mail from this account')
        agent.forwarder.add.assert_called_once_with(message, '#target@t', '')

    def test_held_generation_keeps_help_and_status_responsive_then_starts(self):
        root = Path(__file__).resolve().parents[2] / 'tmp'
        root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='imap-control-', dir=root) as work:
            agent = proxy.Agent(Bus(), {}, {}, work)
            try:
                with Outbox(work):
                    with patch.object(proxy.wire, 'TLS') as connect:
                        self.assertEqual(agent.poll(), 1)
                        self.assertEqual(agent.status(), 'starting: waiting for the previous IMAP generation')
                        help_message = SimpleNamespace(ask='help', body='', envelope={})
                        status_message = SimpleNamespace(ask='status', body='', envelope={})
                        self.assertIn('sync', agent.handle(help_message))
                        self.assertEqual(agent.handle(status_message), agent.status())
                        connect.assert_not_called()
                agent.retry_construct = 0
                self.assertGreater(agent.poll(), 0)
                self.assertIsNotNone(agent.forwarder)
                self.assertIn('manual cache only', agent.status())
            finally:
                agent.close()

    def test_status_and_sub_list_work_without_imap_credentials_or_valid_account_config(self):
        agent = proxy.Agent(None, {'bad': True}, {}, '.')
        agent.forwarder = Mock()
        agent.forwarder.status.return_value = 'ok: manual cache only'
        agent.forwarder.subs.text.return_value = '(no subscriptions; manual cache only)'
        with contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(agent.handle(SimpleNamespace(ask=None, body='status', envelope={})), 'ok: manual cache only')
            answer = agent.handle(SimpleNamespace(ask=None, body='sub list', envelope={}))
        self.assertIn('manual cache only', answer)


if __name__ == '__main__':
    unittest.main()
