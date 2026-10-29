"""#at on the fixture daemon with an injected clock; the engine's main suite is cron's test_cron.py."""
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
from zoneinfo import ZoneInfo

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(HERE.parent / 'cron'))
sys.path.insert(0, str(HERE))
import test_cron as engine  # noqa: E402
from test_cron import CALLER, OUTSIDER, TARGET, WORKER, Fixture  # noqa: E402
from agent_bus.errors import Refusal, UncertainOutcome  # noqa: E402
import at  # noqa: E402
import cron  # noqa: E402

NY = 'America/New_York'


def utc(*parts):
    return datetime(*parts, tzinfo=timezone.utc).timestamp()


class AtFixture(Fixture):
    def agent(self, cfg=None):
        agent = at.Agent(self.bus(WORKER), cfg or {}, clock=self.clock, log=self.logs.append)
        agent.load()
        return agent


class Grammar(AtFixture):
    def test_in_resolves_and_echoes_the_instant_and_zone(self):
        agent = self.agent({'tz': NY})
        self.clock.now = utc(2026, 10, 28, 13, 30, 15)
        for text in ('in 30m', 'in +30m'):
            answer = self.ask(agent, f'{text} {TARGET} x')
            self.assertEqual((answer.get('at'), answer.get('tz')), ('2026-10-28T10:00:15-04:00', NY), text)

    def test_hh_mm_is_the_next_occurrence_strictly_after_this_minute(self):
        agent = self.agent({'tz': NY})
        self.clock.now = utc(2026, 10, 28, 13, 30, 15)  # 09:30:15 in New York
        cases = {'09:31': '2026-10-28T09:31:00-04:00', '09:30': '2026-10-29T09:30:00-04:00',
                 '09:29': '2026-10-29T09:29:00-04:00'}
        for text, want in cases.items():
            self.assertEqual(self.ask(agent, f'at {text} {TARGET} x')['at'], want, text)

    def test_midnight_rollover(self):
        agent = self.agent({'tz': NY})
        self.clock.now = utc(2026, 10, 29, 3, 59, 30)  # 23:59:30 in New York
        self.assertEqual(self.ask(agent, f'at 00:00 {TARGET} x')['at'], '2026-10-29T00:00:00-04:00')

    def test_a_time_in_the_spring_forward_gap_is_refused(self):
        agent = self.agent({'tz': NY})
        self.clock.now = utc(2026, 3, 8, 6, 0)  # 01:00 EST, the clocks jump at 02:00
        self.assertIn('does not exist on 2026-03-08', self.ask(agent, f'at 02:30 {TARGET} x'))
        self.assertEqual(agent.later, {})
        self.assertEqual(self.ask(agent, f'at 03:30 {TARGET} x')['at'], '2026-03-08T03:30:00-04:00')  # control

    def test_a_repeated_fall_back_time_takes_the_first_occurrence(self):
        agent = self.agent({'tz': NY})
        self.clock.now = utc(2026, 11, 1, 4, 10)  # 00:10 EDT
        self.assertEqual(self.ask(agent, f'at 01:30 {TARGET} x')['at'], '2026-11-01T01:30:00-04:00')

    def test_malformed_times_durations_and_messages_are_refused(self):
        agent = self.agent({'tz': NY})
        refused = ['at 24:00', 'at 9:5', 'at 09:30:00', 'at ٠٩:٣٠', 'in 0s', 'in 30', 'in 1h30m',
                   'in 367d', 'at 2026-10-29T09:30:00']
        for text in refused:
            self.assertTrue(str(self.ask(agent, f'{text} {TARGET} x')).startswith('error:'), text)
        self.assertTrue(str(self.ask(agent, json.dumps({'method': 'in', 'in': '5m', 'target': TARGET, 'body': ''}))).startswith('error:'))
        self.assertTrue(str(self.ask(agent, f'in 5m {TARGET}')).startswith('error:'))
        self.assertEqual(agent.later, {})

    def test_the_command_word_is_the_first_token_and_the_message_is_verbatim(self):
        agent = self.agent()
        body = 'at  09:00 in 5m {"q": 1}'
        self.ask(agent, f'in 1s {TARGET} {body}')
        self.clock.now += 2
        agent.tick()
        self.assertEqual(self.received(), [body])


class Surfaces(AtFixture):
    def test_at_does_not_expose_cron_calls(self):
        agent = self.agent()
        for text in ('sub add * * * * * ' + TARGET + ' x', 'later +1m ' + TARGET + ' x', 'sub list'):
            self.assertTrue(self.ask(agent, text).startswith('error:'), text)
        self.assertTrue(self.ask(agent, json.dumps({'method': 'sub.add', 'cron': '* * * * *', 'target': TARGET, 'body': 'x'})).startswith('error:'))
        self.assertEqual((agent.subs, agent.later), ({}, {}))

    def test_cron_does_not_take_at_calls(self):
        agent = cron.Agent(self.bus(WORKER), {}, clock=self.clock, log=self.logs.append)
        for text in ('in 30m ' + TARGET + ' x', 'at 09:30 ' + TARGET + ' x', 'list'):
            self.assertTrue(self.ask(agent, text).startswith('error:'), text)


class Firing(AtFixture):
    def test_fires_once_survives_a_restart_and_names_its_creator(self):
        agent = self.agent()
        self.ask(agent, f'in 2m {TARGET} later on')
        self.die(agent)
        self.clock.now += 121
        restarted = self.agent()
        restarted.tick()
        restarted.tick()
        message = self.target.consume(wait=0.5)
        self.assertEqual((message.body, message.topic, message.tag), ('later on', 'at', 'by ' + CALLER))
        self.assertEqual(tuple(message.roles), ())
        self.die(restarted)
        self.agent().tick()
        self.assertEqual(self.received(), [])

    def test_an_overdue_call_fires_once_at_start(self):
        agent = self.agent()
        self.ask(agent, f'in 1m {TARGET} overdue')
        self.die(agent)
        self.clock.now += 3 * 3600
        restarted = self.agent()
        restarted.tick()
        restarted.tick()
        self.assertEqual(self.received(), ['overdue'])

    def test_cancel_stops_it_and_needs_the_creator_or_staff(self):
        agent = self.agent()
        key = self.ask(agent, f'in 1m {TARGET} never')['id']
        self.assertIn('creator', self.ask(agent, 'cancel ' + key, self.outsider))
        self.assertEqual(self.ask(agent, 'cancel ' + key), {'deleted': key})
        self.clock.now += 120
        agent.tick()
        self.assertEqual(self.received(), [])

    def test_an_acl_refusal_pauses_and_is_not_retried(self):
        agent = self.agent()
        key = self.ask(agent, f'in 1s {TARGET} closed')['id']
        self.owner.record(TARGET).manage(allow=[CALLER])
        self.clock.now += 2
        agent.tick()
        self.clock.now += 3600
        agent.tick()
        self.assertEqual(self.received(), [])
        self.assertEqual(agent.later.get(key, {}).get('paused'), 'denied')
        rows = self.ask(agent, 'list')
        self.assertEqual(([r.get('paused') for r in rows['later']], rows['tz']), (['denied'], 'UTC'))

    def test_cancel_while_sending_keeps_the_evidence(self):
        agent = self.agent()
        key = self.ask(agent, f'in 1m {TARGET} x')['id']
        agent.claim(f'fired.{key}.once', 'sending', 'set')
        self.assertEqual(self.ask(agent, 'cancel ' + key), {'deleted': key})
        self.assertIn(f'fired.{key}.once', [e['name'] for e in self.owner.record(WORKER).kv.list().get('string', [])])

    def test_cancel_after_unconfirmed_removes_the_entry_and_its_claim(self):
        agent = self.agent()
        key = self.ask(agent, f'in 1s {TARGET} x')['id']
        self.clock.now += 2
        with patch.object(agent.bus, 'send', side_effect=UncertainOutcome('lost')):
            agent.tick()
        self.assertEqual(agent.later.get(key, {}).get('paused'), 'unconfirmed')
        self.assertEqual(self.ask(agent, 'cancel ' + key), {'deleted': key})
        names = [e['name'] for e in self.owner.record(WORKER).kv.list().get('string', [])]
        self.assertNotIn(f'fired.{key}.once', names)


class Package(unittest.TestCase):
    def test_both_packages_ship_the_engine_unchanged(self):
        with tempfile.TemporaryDirectory(dir=ROOT / 'tmp') as work:
            for agent in ('at', 'cron'):
                subprocess.run([str(ROOT / f'src/agents/{agent}/package.sh'), f'{work}/{agent}'], check=True, capture_output=True)
                for name in ('scheduler.py', 'cronexpr.py'):
                    self.assertTrue((Path(work) / agent / 'support/agents/shared' / name).is_file(), f'{agent} {name}')
                    self.assertEqual((Path(work) / agent / 'support/agents/shared' / name).read_bytes(),
                                     (ROOT / 'src/agents/shared' / name).read_bytes(), f'{agent} {name}')
            env = {'PATH': os.environ['PATH'], 'PYTHONDONTWRITEBYTECODE': '1', 'AGENT_BUS_ASK': 'help-json'}
            result = subprocess.run([f'{work}/at/at.py'], env=env, capture_output=True, text=True, timeout=5)
            self.assertEqual(result.returncode, 0, result.stderr)
            metadata = json.loads(result.stdout)
            self.assertEqual(sorted(metadata['methods']), ['at', 'cancel', 'help', 'in', 'list'])
            self.assertTrue(metadata['docs'].endswith('/at.md'))


class Entrypoint(AtFixture):
    def test_the_packaged_process_serves_and_fires(self):
        with tempfile.TemporaryDirectory(dir=ROOT / 'tmp') as work:
            subprocess.run([str(HERE / 'package.sh'), work + '/at'], check=True, capture_output=True)
            env = {'PATH': os.environ['PATH'], 'PYTHONDONTWRITEBYTECODE': '1', 'AGENT_BUS_NAME': WORKER,
                   'AGENT_BUS_TOKEN': self.info['tokens'][WORKER], 'AGENT_BUS_ADDR': self.info['addr']}
            agent = subprocess.Popen([work + '/at/at.py'], env=env, cwd=work, stdin=subprocess.DEVNULL,
                                     stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True)
            try:
                answer = self.caller.call(WORKER, 'in 1s ' + TARGET + ' from the process', timeout=5)
                self.assertIn('"tz": "UTC"', answer.message.body)
                delivered = self.target.consume(wait=5)
                self.assertIsNotNone(delivered, 'the call did not fire')
                self.assertEqual((delivered.body, delivered.topic, delivered.tag), ('from the process', 'at', 'by ' + CALLER))
                self.assertIsNone(self.target.consume(wait=1.5))
            finally:
                agent.terminate()
                self.assertEqual(agent.wait(timeout=5), 0)
                errors = agent.stderr.read()
                agent.stderr.close()
            self.assertNotIn('from the process', errors)


del Fixture, engine  # only the subclasses above are this module's tests

if __name__ == '__main__':
    unittest.main()
