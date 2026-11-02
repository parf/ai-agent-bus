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
        for case in cases('events'):
            with self.subTest(case=case['id']):
                self.assertIs(case['fresh'], True)
                self.tearDown()
                self.setUp()
                buses = {'owner': self.owner, 'agent': self.caller,
                         'third': self.bus('#outsider@fixture')}
                answers = {}
                for i, step in enumerate(case['steps']):
                    where = f"{case['id']} step {i} {step['op']}"
                    bus = buses[step['as']]
                    if step['op'] == 'call':
                        out = bus.call('#bus', step['body'], timeout=3)
                        self.assertIsInstance(out, CallReply, where)
                        answers[i] = out.message.body
                        decoded = json.loads(out.message.body)
                        for path, want in step.get('expect', {}).items():
                            actual = decoded
                            for key in path.split('.'):
                                actual = actual[int(key)] if isinstance(actual, list) else actual[key]
                            self.assertEqual(actual, want, where + ' ' + path)
                        if 'same_answer_as' in step:
                            self.assertEqual(out.message.body, answers[step['same_answer_as']], where)
                    elif step['op'] == 'send':
                        bus.send('#bus', step['body'])
                    elif step['op'] == 'inbox':
                        self.assertIs(step['expect']['empty'], True)
                        self.assertIsNone(bus.consume(wait=0.3), where)
                    else:
                        self.fail('unsupported event vector operation: ' + step['op'])

    def test_native_event_example(self):
        output = event_example(self.caller, self.owner, self.bus('#outsider@fixture'))
        self.assertEqual(output, 'written: alert disk full\nowner reads: 1 alert\n'
                         'owner lists: #caller@fixture\na third party: forbidden\n'
                         'the writer reading its own log: forbidden')
