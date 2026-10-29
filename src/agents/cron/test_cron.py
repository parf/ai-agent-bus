"""Cron on the fixture daemon with an injected clock: authority, single firing, restarts, RD138 delivery."""
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import select
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(ROOT / 'src/clients/python'))
sys.path.insert(0, str(HERE))
from agent_bus import Bus  # noqa: E402
from agent_bus.errors import Refusal, UncertainOutcome  # noqa: E402
import cron  # noqa: E402

WORKER, CALLER, TARGET, OUTSIDER = '#worker@fixture', '#caller@fixture', '#helper@fixture', '#outsider@fixture'
T0 = datetime(2026, 10, 28, 12, 0, 5, tzinfo=timezone.utc).timestamp()


class Stop(BaseException):
    """A process stopping at an exact point; BaseException so no handler catches it."""


class Clock:
    def __init__(self):
        self.now = T0

    def __call__(self):
        return self.now


class Fixture(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        fixture = Path(os.environ.get('CRON_BUS_FIXTURE', ROOT / 'tmp/cron-client-fixture'))
        if not fixture.is_file():
            raise AssertionError('build src/clients/fixture to ' + str(fixture))
        cls.fixture = fixture

    def setUp(self):
        self.process = subprocess.Popen([str(self.fixture)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.DEVNULL, text=True)
        ready, _, _ = select.select([self.process.stdout], [], [], 5)
        self.assertTrue(ready, 'fixture bus did not start')
        self.info = json.loads(self.process.stdout.readline())
        self.buses = []
        self.owner, self.caller, self.target, self.outsider = map(self.bus, ('owner@fixture', CALLER, TARGET, OUTSIDER))
        self.owner.record(TARGET).manage(maintainers=[CALLER])
        self.clock = Clock()
        self.logs = []

    def tearDown(self):
        for bus in self.buses:
            bus.close()
        self.process.stdin.close()
        self.process.wait(timeout=5)
        self.process.stdout.close()

    def bus(self, name):
        bus = Bus(name, addr=self.info['addr'], token=self.info['tokens'][name], timeout=2)
        self.buses.append(bus)
        return bus

    def agent(self, cfg=None):
        """A cron process: its own connection, loaded from the agent's KV."""
        agent = cron.Agent(self.bus(WORKER), cfg or {}, clock=self.clock, log=self.logs.append)
        agent.load()
        return agent

    def die(self, agent):
        """The process is gone: its lock lapses as the TTL would."""
        agent.lease = 0
        try:
            self.owner.record(WORKER).locks.force_release('scheduler')
        except Exception:
            pass

    def ask(self, agent, body, sender=None):
        (sender or self.caller).send(WORKER, body)
        message = agent.bus.consume(wait=1)
        answer = agent.handle(message)
        return json.loads(answer) if answer.startswith('{') else answer

    def received(self, bus=None):
        out = []
        while True:
            message = (bus or self.target).consume(wait=0.05)
            if message is None:
                return out
            out.append(message.body)

    def at(self, hh, mm, ss=5):
        self.clock.now = datetime(2026, 10, 28, hh, mm, ss, tzinfo=timezone.utc).timestamp()


class Authority(Fixture):
    def test_entries_need_management_of_the_destination_and_store_nothing_otherwise(self):
        agent = self.agent()
        refused = self.ask(agent, 'sub add * * * * * ' + TARGET + ' hi', self.outsider)
        self.assertEqual(refused, 'error: scheduling requires management of destination ' + TARGET)
        refused = self.ask(agent, 'later +1m ' + TARGET + ' hi', self.outsider)
        self.assertIn('requires management', refused)
        self.assertEqual(self.owner.record(WORKER).kv.list().get('json', []), [])
        made = self.ask(agent, 'sub add * * * * * ' + TARGET + ' hi')  # positive control: a maintainer of the target
        self.assertRegex(made['id'], '^[0-9a-f]{12}$')
        self.assertEqual(len(self.owner.record(WORKER).kv.list()['json']), 1)

    def test_cron_owner_without_managing_the_target_is_refused(self):
        self.owner.record(WORKER).manage(maintainers=[OUTSIDER])
        agent = self.agent()
        self.assertIn('requires management', self.ask(agent, 'sub add * * * * * ' + TARGET + ' hi', self.outsider))
        self.assertIn('requires management', self.ask(agent, 'later +1m ' + TARGET + ' hi', self.outsider))

    def test_not_to_itself_and_per_creator_caps(self):
        agent = self.agent()
        self.owner.record(WORKER).manage(maintainers=[CALLER])
        self.assertEqual(self.ask(agent, 'later +1m #worker@fixture hi'), 'error: cron does not schedule messages to itself')
        for i in range(cron.SUBS_EACH):
            self.assertIn('id', self.ask(agent, f'sub add {i} * * * * {TARGET} n{i}'))
        self.assertIn('per creator', self.ask(agent, f'sub add 59 * * * * {TARGET} over'))
        self.assertEqual(len(agent.subs), cron.SUBS_EACH)

    def test_delete_and_cancel_need_management_too(self):
        agent = self.agent()
        sub = self.ask(agent, 'sub add * * * * * ' + TARGET + ' hi')['id']
        shot = self.ask(agent, 'later +5m ' + TARGET + ' hi')['id']
        self.assertIn('requires management', self.ask(agent, 'sub delete ' + sub, self.outsider))
        self.assertIn('requires management', self.ask(agent, 'later cancel ' + shot, self.outsider))
        self.assertEqual(self.ask(agent, 'sub delete ' + sub), {'deleted': sub})
        self.assertEqual(self.ask(agent, 'later cancel ' + shot), {'deleted': shot})

    def test_lists_show_bodies_only_to_managers_of_the_target(self):
        self.owner.record(WORKER).manage(maintainers=[OUTSIDER])
        agent = self.agent()
        self.ask(agent, 'sub add 0 9 * * * ' + TARGET + ' secret command')
        mine = self.ask(agent, 'sub list')['subs']
        self.assertEqual([s['body'] for s in mine], ['secret command'])
        staff = self.ask(agent, 'sub list', self.outsider)['subs']
        self.assertEqual(len(staff), 1)
        self.assertNotIn('body', staff[0])
        self.owner.record(WORKER).manage(maintainers=[])
        self.assertEqual(self.ask(agent, 'sub list', self.outsider)['subs'], [])

    def test_authority_is_checked_again_at_each_firing(self):
        agent = self.agent()
        self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')
        self.ask(agent, 'later +30s ' + TARGET + ' once')
        self.owner.record(TARGET).manage(maintainers=[])
        self.at(12, 1)
        agent.tick()
        self.assertEqual(self.received(), [])
        [sub] = agent.subs.values()
        self.assertTrue(sub['paused'].startswith('revoked'))
        self.assertEqual(agent.later, {})
        self.assertEqual([v['status'] for v in self.owner.record(WORKER).kv.get('outcomes', kind='json').values()],
                         ['revoked'])


class Text(Fixture):
    def test_the_message_is_the_rest_of_the_line_verbatim(self):
        agent = self.agent()
        body = '{  "two  spaces" : \'quoted\' } trailing  '
        self.ask(agent, 'sub add * * * * * ' + TARGET + ' ' + body)
        self.ask(agent, json.dumps({'method': 'later', 'in': '+1s', 'target': TARGET, 'body': 'line one\nline two'}))
        self.at(12, 1)
        agent.tick()
        self.assertEqual(sorted(self.received()), sorted([body, 'line one\nline two']))

    def test_short_or_macro_forms_are_refused(self):
        agent = self.agent()
        for text in ('sub add * * * * * ' + TARGET, 'sub add @daily ' + TARGET + ' x', 'later +1m', 'later +0s ' + TARGET + ' x'):
            self.assertTrue(self.ask(agent, text).startswith('error:'), text)
        self.assertEqual(agent.subs, {})
        self.assertEqual(agent.later, {})


class Firing(Fixture):
    def test_a_schedule_fires_once_in_its_minute(self):
        agent = self.agent()
        self.ask(agent, 'sub add 1 12 * * * ' + TARGET + ' at 12:01')
        self.at(12, 0, 50)
        agent.tick()
        self.at(12, 1, 1)
        agent.tick()
        self.at(12, 1, 40)
        agent.tick()
        self.assertEqual(self.received(), ['at 12:01'])

    def test_two_schedulers_send_once(self):
        first = self.agent()
        self.ask(first, 'sub add * * * * * ' + TARGET + ' tick')
        second = self.agent()
        self.at(12, 1)
        second.tick()
        first.tick()
        self.assertEqual(self.received(), ['tick'])

    def test_the_slot_claim_alone_stops_a_second_sender(self):
        first = self.agent()
        self.ask(first, 'sub add * * * * * ' + TARGET + ' tick')
        second = self.agent()
        self.at(12, 1)
        first.tick()
        with patch.object(second, 'scheduler', return_value=True):
            second.claims.clear()
            second.tick()
        self.assertEqual(self.received(), ['tick'])

    def test_a_missed_slot_is_not_replayed(self):
        agent = self.agent()
        self.ask(agent, 'sub add 2 12 * * * ' + TARGET + ' missed')
        self.ask(agent, 'sub add 7 12 * * * ' + TARGET + ' current')
        self.die(agent)
        self.at(12, 7)
        restarted = self.agent()
        restarted.tick()
        self.assertEqual(self.received(), ['current'])

    def test_subscriptions_and_one_shots_survive_a_restart(self):
        agent = self.agent()
        self.ask(agent, 'sub add 3 12 * * * ' + TARGET + ' recurring')
        self.ask(agent, 'later +2m ' + TARGET + ' once')
        self.die(agent)
        self.at(12, 3)
        restarted = self.agent()
        restarted.tick()
        self.assertEqual(sorted(self.received()), ['once', 'recurring'])
        self.at(12, 3, 30)
        restarted.tick()
        self.die(restarted)
        third = self.agent()
        third.tick()
        self.assertEqual(self.received(), [])

    def test_an_overdue_one_shot_fires_once_at_start(self):
        agent = self.agent()
        self.ask(agent, 'later +1m ' + TARGET + ' overdue')
        self.die(agent)
        self.at(15, 0)
        restarted = self.agent()
        restarted.tick()
        restarted.tick()
        self.assertEqual(self.received(), ['overdue'])
        [done] = self.owner.record(WORKER).kv.get('outcomes', kind='json').values()
        self.assertEqual(done['status'], 'sent')

    def test_a_paused_subscription_skips_its_slots(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        agent.pause(key, 'unconfirmed: test')
        self.at(12, 1)
        agent.tick()
        self.assertEqual(self.received(), [])
        self.assertEqual(agent.state[key]['skipped'], 1)

    def test_invalid_configuration_fires_nothing_and_touches_nothing(self):
        agent = self.agent()
        self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')
        before = self.owner.record(WORKER).kv.list()
        broken = self.agent({'tz': 'Mars/Olympus'})
        self.at(12, 1)
        broken.tick()
        self.assertEqual(self.received(), [])
        self.assertEqual(self.owner.record(WORKER).kv.list(), before)
        self.assertIn('Configuration', self.ask(broken, 'sub list'))

    def test_one_bad_stored_entry_leaves_the_others_firing(self):
        agent = self.agent()
        self.ask(agent, 'sub add * * * * * ' + TARGET + ' good')
        self.owner.record(WORKER).kv.set('sub.0123456789ab', {'cron': '61 * * * *', 'target': TARGET, 'body': 'x', 'by': CALLER},
                                         kind='json')
        self.die(agent)
        try:
            restarted = self.agent()
        except Exception as e:
            self.fail('the loader stopped on one bad entry: ' + repr(e))
        self.at(12, 1)
        restarted.tick()
        self.assertEqual(self.received(), ['good'])
        self.assertTrue(restarted.state['0123456789ab']['invalid'])


class Delivery(Fixture):
    """RD138: unconfirmed pauses, confirmed is never resent, a definite refusal retries."""

    def test_an_unconfirmed_send_pauses_and_is_not_resent(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=UncertainOutcome('lost')):
            agent.tick()
        self.assertTrue(agent.subs[key].get('paused', '').startswith('unconfirmed'))
        self.at(12, 2)
        agent.tick()
        self.assertEqual(self.received(), [])
        self.die(agent)
        self.assertTrue(self.agent().subs[key].get('paused', '').startswith('unconfirmed'))

    def test_an_unconfirmed_one_shot_stays_paused_after_restart(self):
        agent = self.agent()
        key = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=UncertainOutcome('lost')):
            agent.tick()
        self.die(agent)
        restarted = self.agent()
        restarted.tick()
        self.assertEqual(self.received(), [])
        self.assertEqual(restarted.later.get(key, {}).get('paused'), 'unconfirmed')
        self.assertEqual([r.get('paused') for r in self.ask(restarted, 'later list')['later']], ['unconfirmed'])
        self.assertGreater(restarted.wait(), 1)  # a paused one-shot does not keep the loop busy

    def test_a_server_error_is_unconfirmed_too(self):
        agent = self.agent()
        key = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(500, 'internal')):
            agent.tick()
        self.assertEqual(agent.later.get(key, {}).get('paused'), 'unconfirmed')
        self.at(12, 2)
        agent.tick()
        self.assertEqual(self.received(), [])

    def test_a_stop_between_send_and_success_record_pauses_after_restart(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        real = agent.claim

        def stop_on_success(name, value, mode):
            if value == 'sent':
                raise Stop()
            return real(name, value, mode)
        self.at(12, 1)
        with patch.object(agent, 'claim', side_effect=stop_on_success), self.assertRaises(Stop):
            agent.tick()
        self.assertEqual(self.received(), ['tick'])
        self.die(agent)
        restarted = self.agent()
        self.assertTrue(restarted.subs[key].get('paused', '').startswith('unconfirmed'))
        self.at(12, 2)
        restarted.tick()
        self.assertEqual(self.received(), [])

    def test_a_refusal_retries_across_a_restart_and_delivers_once(self):
        agent = self.agent()
        key = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(403, 'not allowed')):
            agent.tick()
        self.assertEqual(self.received(), [])
        self.assertIn(key, agent.later)
        self.assertNotIn('paused', agent.later[key])
        self.die(agent)
        restarted = self.agent()
        self.assertIn(key, restarted.later)
        self.assertNotIn('paused', restarted.later[key])
        restarted.tick()
        restarted.tick()
        self.assertEqual(self.received(), ['once'])

    def test_a_refusal_backs_off_then_records_refused(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add 1 12 * * * ' + TARGET + ' tick')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(409, 'full')) as send:
            agent.tick()
            self.clock.now += 5
            agent.tick()
            self.assertEqual(send.call_count, 1)  # backoff, not a hot loop
            self.clock.now += 6
            agent.tick()
            self.assertEqual(send.call_count, 2)
            self.clock.now += 86400 + 3600  # past the 24 h TTL, away from the next 12:01 slot
            agent.tick()
        self.assertEqual(agent.state[key]['outcome'], 'refused')
        self.assertNotIn('paused', agent.subs[key])

    def test_sends_carry_a_ttl(self):
        agent = self.agent()
        self.ask(agent, 'sub add */5 * * * * ' + TARGET + ' tick')
        self.at(12, 5)
        with patch.object(agent.bus, 'send', return_value='m1') as send:
            agent.tick()
        self.assertEqual(send.call_args.kwargs.get('ttl'), 300)

    def test_the_claim_cleanup_keeps_unconfirmed_evidence(self):
        agent = self.agent()
        name = 'fired.0123456789ab.2026-10-28T12:01'
        agent.claim(name, 'unconfirmed', 'set')
        agent.drop_claim(name)
        self.assertIn(name, [e['name'] for e in self.owner.record(WORKER).kv.list().get('string', [])])


class Entrypoint(Fixture):
    def test_the_packaged_process_serves_and_fires(self):
        with tempfile.TemporaryDirectory(dir=ROOT / 'tmp') as work:
            subprocess.run([str(HERE / 'package.sh'), work + '/cron'], check=True, capture_output=True)
            env = {'PATH': os.environ['PATH'], 'PYTHONDONTWRITEBYTECODE': '1', 'AGENT_BUS_NAME': WORKER,
                   'AGENT_BUS_TOKEN': self.info['tokens'][WORKER], 'AGENT_BUS_ADDR': self.info['addr'],
                   'AGENT_BUS_CONFIG': '{"tz":"UTC"}'}
            agent = subprocess.Popen([work + '/cron/cron.py'], env=env, cwd=work, stdin=subprocess.DEVNULL,
                                     stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True)
            try:
                answer = self.caller.call(WORKER, 'later +1s ' + TARGET + ' from the process', timeout=5)
                self.assertRegex(answer.message.body, '"id": "[0-9a-f]{12}"')
                delivered = self.target.consume(wait=5)
                self.assertIsNotNone(delivered, 'the one-shot did not fire')
                self.assertEqual((delivered.body, delivered.envelope['from'], delivered.topic),
                                 ('from the process', WORKER, 'later'))
                self.assertIsNone(self.target.consume(wait=1.5))
            finally:
                agent.terminate()
                self.assertEqual(agent.wait(timeout=5), 0)
                errors = agent.stderr.read()
                agent.stderr.close()
            self.assertNotIn('from the process', errors)


class Package(unittest.TestCase):
    def test_exported_help_without_a_bus(self):
        with tempfile.TemporaryDirectory(dir=ROOT / 'tmp') as work:
            subprocess.run([str(HERE / 'package.sh'), work + '/cron'], check=True, capture_output=True)
            env = {'PATH': os.environ['PATH'], 'PYTHONDONTWRITEBYTECODE': '1', 'AGENT_BUS_ASK': 'help-json'}
            result = subprocess.run([work + '/cron/cron.py'], env=env, capture_output=True, text=True, timeout=5)
            self.assertEqual(result.returncode, 0, result.stderr)
            metadata = json.loads(result.stdout)
            self.assertEqual(sorted(metadata['methods']), ['help', 'later', 'later.cancel', 'later.list', 'sub.add', 'sub.delete', 'sub.list'])
            self.assertTrue(metadata['docs'].endswith('/cron.md'))


if __name__ == '__main__':
    unittest.main()
