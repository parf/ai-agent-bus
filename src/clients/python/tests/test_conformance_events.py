"""Native calls through the shared private-log vectors; no subscription helper."""
import json
import unittest

from agent_bus import CallReply
import test_client as fixture
from test_conformance import cases


def event_example(agent, owner, stranger):
    """Executed README example: three handles with different authority."""
    def call(bus, body):
        answer = bus.call('#bus', body, timeout=3)
        if not isinstance(answer, CallReply):
            raise AssertionError('event example expected an answer')
        return json.loads(answer.message.body)
    entry = call(agent, 'write #caller@fixture alert disk full')['entry']
    read = call(owner, 'read #caller@fixture')
    logs = call(owner, 'list')['logs']
    private = call(stranger, 'read #caller@fixture')['error']['code']
    own_read = call(agent, 'read #caller@fixture')['error']['code']
    return (f"written: {entry['level']} {entry['text']}\n"
            f"owner reads: {len(read['rings']['alert'])} alert\n"
            f"owner lists: {logs[0]['log']}\n"
            f"a third party: {private}\n"
            f"the writer reading its own log: {own_read}")


class SharedEvents(unittest.TestCase):
    setUp = fixture.LiveClient.setUp
    tearDown = fixture.LiveClient.tearDown
    bus = fixture.LiveClient.bus

    def test_shared_event_vectors(self):
        self.event_vectors('events')

    def test_shared_event_subscription_vectors(self):
        self.event_vectors('event-subscriptions')

    def event_vectors(self, file):
        for case in cases(file):
            with self.subTest(case=case['id']):
                self.assertIs(case['fresh'], True)
                self.tearDown()
                self.setUp()
                buses = {'owner': self.owner, 'agent': self.caller,
                         'third': self.bus('#outsider@fixture'), 'worker': self.worker}
                answers = {}
                subscription = ""
                for i, step in enumerate(case['steps']):
                    where = f"{case['id']} step {i} {step['op']}"
                    bus = buses[step['as']]
                    if step['op'] == 'call':
                        out = bus.call('#bus', step['body'].replace('$subscription', subscription), timeout=5)
                        self.assertIsInstance(out, CallReply, where)
                        answers[i] = out.message.body
                        decoded = json.loads(out.message.body)
                        if step['body'].startswith('sub add '):
                            subscription = decoded.get('id')
                            self.assertTrue(subscription, where + ' subscription id')
                        for path, want in step.get('expect', {}).items():
                            actual = decoded
                            for key in path.split('.'):
                                actual = actual[int(key)] if isinstance(actual, list) else actual[key]
                            if isinstance(want, str):
                                want = want.replace('$subscription', subscription)
                            self.assertEqual(actual, want, where + ' ' + path)
                        if 'same_answer_as' in step:
                            self.assertEqual(out.message.body, answers[step['same_answer_as']], where)
                    elif step['op'] == 'send':
                        bus.send('#bus', step['body'])
                    elif step['op'] == 'manage':
                        bus.record(step['name']).manage(**step['changes'])
                    elif step['op'] == 'inbox':
                        message = bus.consume(inbox=step.get('inbox'), wait=0.3)
                        if step['expect'].get('empty'):
                            self.assertIsNone(message, where + ' inbox not empty')
                            continue
                        self.assertIsNotNone(message, where + ' missing subscription copy')
                        decoded = dict(message.envelope)
                        decoded['body'] = json.loads(message.body)
                        for field in ('wait', 'receipt', 're'):
                            decoded.setdefault(field, '')
                        for path, want in step['expect'].items():
                            actual = decoded
                            for key in path.split('.'):
                                actual = actual[int(key)] if isinstance(actual, list) else actual[key]
                            self.assertEqual(actual, want, where + ' copy ' + path)
                        # Non-waiting copies are notices: no ack/done.

                    else:
                        self.fail('unsupported event vector operation: ' + step['op'])

    def test_native_event_example(self):
        output = event_example(self.caller, self.owner, self.bus('#outsider@fixture'))
        self.assertEqual(output, 'written: alert disk full\nowner reads: 1 alert\n'
                         'owner lists: #caller@fixture\na third party: forbidden\n'
                         'the writer reading its own log: forbidden')

    def test_native_event_subscription_example(self):
        self.assertEqual(subscription_example(self.caller, self.owner),
                         'listed: 1 subscription; copy: alert disk full; '
                         'receipt: none for a notice; cancelled: no further copies')


def subscription_example(agent, owner):
    """Executed native example; ordinary calls and queue consumption suffice."""
    def call(bus, body):
        out = bus.call('#bus', body, timeout=5)
        assert isinstance(out, CallReply), 'subscription example expected answer'
        reply = json.loads(out.message.body)
        assert 'error' not in reply, f'subscription example refused: {reply}'
        return reply

    row = call(owner, 'sub add jobs@fixture #caller@fixture alert')
    subscription = row['id']
    listed = call(owner, 'sub list #caller@fixture')['subscriptions']
    assert len(listed) == 1 and listed[0]['id'] == subscription, 'subscription listing'
    written = call(agent, 'write #caller@fixture alert disk full')
    assert written['deliveries']['sent'] == 1, 'one copy sent'
    copy = owner.consume(inbox='jobs@fixture', wait=0.3)
    assert copy is not None, 'missing subscription copy'
    assert copy.envelope['from'] == '#bus' and copy.topic == '#caller@fixture' and copy.tag == 'alert', 'copy route'
    entry = json.loads(copy.body)
    assert entry['level'] == 'alert' and entry['text'] == 'disk full', 'copy body'
    assert not copy.receipt and not copy.re and not copy.envelope.get('wait'), 'copy is a notice'
    # Do not ack/done a non-waiting notice.
    assert owner.consume(inbox='jobs@fixture', wait=0.3) is None, 'duplicate copy'
    assert call(owner, 'sub delete ' + subscription)['deleted'], 'subscription deletion'
    written = call(agent, 'write #caller@fixture alert after cancellation')
    assert written['deliveries']['matched'] == 0, 'cancelled subscription matched'
    assert owner.consume(inbox='jobs@fixture', wait=0.3) is None, 'copy after cancellation'
    return ('listed: 1 subscription; copy: alert disk full; '
            'receipt: none for a notice; cancelled: no further copies')
