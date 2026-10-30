"""#at on the fixture daemon with an injected clock; the engine's main suite is cron's test_cron.py."""
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import unittest.mock
from unittest.mock import patch
from zoneinfo import ZoneInfo

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(HERE.parent / 'cron'))
sys.path.insert(0, str(HERE))
import test_cron as engine  # noqa: E402
from test_cron import CALLER, OUTSIDER, T0 as engine_T0, TARGET, WORKER, Fixture  # noqa: E402
from agent_bus.errors import Refusal, TransportError, UncertainOutcome  # noqa: E402
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

    def test_fall_back_takes_the_next_real_instant(self):
        agent = self.agent({'tz': NY})
        cases = [(utc(2026, 11, 1, 5, 45), '01:50', '2026-11-01T01:50:00-04:00'),  # first pass, EDT
                 (utc(2026, 11, 1, 5, 45), '01:30', '2026-11-01T01:30:00-05:00'),  # the repeat, an hour on
                 (utc(2026, 11, 1, 6, 45), '01:50', '2026-11-01T01:50:00-05:00'),  # second pass, EST
                 (utc(2026, 11, 1, 6, 45), '01:30', '2026-11-02T01:30:00-05:00')]
        for now, text, want in cases:
            self.clock.now = now
            self.assertEqual(self.ask(agent, f'at {text} {TARGET} x').get('at'), want, (now, text))

    def test_the_gap_names_the_nearest_valid_time_and_the_next_day_is_fine(self):
        agent = self.agent({'tz': NY})
        self.clock.now = utc(2026, 3, 8, 6, 0)
        self.assertIn('nearest valid time is 03:00 EDT', self.ask(agent, f'at 02:30 {TARGET} x'))
        self.clock.now = utc(2026, 3, 9, 5, 0)  # 01:00 EDT the next day
        self.assertEqual(self.ask(agent, f'at 02:30 {TARGET} x').get('at'), '2026-03-09T02:30:00-04:00')

    def test_the_default_zone_is_utc_and_echoed(self):
        agent = self.agent()
        self.clock.now = utc(2026, 10, 28, 13, 30, 15)
        self.assertEqual(self.ask(agent, f'at 14:00 {TARGET} x'), {'id': unittest.mock.ANY, 'at': '2026-10-28T14:00:00+00:00', 'tz': 'UTC'})

    def test_words_are_refused_with_the_two_forms(self):
        agent = self.agent()
        for word in ('tomorrow', 'noon', 'now'):
            self.assertIn('HH:MM or an RFC 3339 instant', str(self.ask(agent, f'at {word} {TARGET} x')), word)
        self.assertEqual(agent.later, {})

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
    def test_unavailable_authority_refuses_add_and_defers_without_sending(self):
        agent = self.agent()
        with patch.object(agent.bus, 'may_send', side_effect=TransportError('down')):
            self.assertIn('could not check', self.ask(agent, f'in 1s {TARGET} blocked'))
        self.assertEqual(agent.later, {})
        key = self.ask(agent, f'in 1s {TARGET} once')['id']
        self.clock.now += 2
        with patch.object(agent.bus, 'may_send', side_effect=TransportError('down')):
            agent.tick()
        self.assertEqual(self.received(), [])
        self.assertIn(key, agent.later)
        self.assertEqual(self.ask(agent, 'list')['author_check'], 'unavailable')
        self.clock.now += 2
        agent.tick()
        self.assertEqual(self.received(), ['once'])
        self.assertNotIn(key, agent.later)

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

    def test_an_acl_refusal_drops_the_one_shot_and_is_not_retried(self):
        agent = self.agent()
        key = self.ask(agent, f'in 1s {TARGET} closed')['id']
        self.clock.now += 2
        with patch.object(agent.bus, 'send', side_effect=Refusal(403, 'not allowed')) as send:
            agent.tick()
            self.clock.now += 3600
            agent.tick()
        self.assertEqual(send.call_count, 1)
        self.assertNotIn(key, agent.later)
        rows = self.ask(agent, 'list')
        self.assertEqual((rows['later'], [r['status'] for r in rows['finished']], rows['tz']), ([], ['dropped'], 'UTC'))

    def test_the_creator_must_be_allowed_to_send(self):
        agent = self.agent()
        self.owner.record(TARGET).manage(allow=[CALLER, WORKER])
        self.assertEqual(self.ask(agent, f'in 5m {TARGET} x', self.outsider),
                         f'error: you may not send to {TARGET}, so at will not send there for you')
        key = self.ask(agent, f'in 1s {TARGET} x')['id']
        self.owner.record(TARGET).manage(allow=[WORKER])
        self.clock.now += 2
        agent.tick()
        self.assertEqual(self.received(), [])
        self.assertNotIn(key, agent.later)  # that copy is dropped and the one-shot removed

    def test_cancel_while_sending_keeps_the_evidence(self):
        agent = self.agent()
        key = self.ask(agent, f'in 1m {TARGET} x')['id']
        agent.claim(f'fired.{key}.once', 'sending', 'set')
        self.assertEqual(self.ask(agent, 'cancel ' + key), {'deleted': key})
        self.assertIn(f'fired.{key}.once', [e['name'] for e in self.owner.record(WORKER).kv.list().get('string', [])])

    def test_a_legacy_gone_one_shot_is_dropped_once_at_restart_and_never_sent(self):
        agent = self.agent()
        key = self.ask(agent, f'in 1s {TARGET} once')['id']
        self.owner.record(WORKER).kv.set(f'fired.{key}.once', 'gone')
        self.clock.now += 2
        self.logs.clear()
        self.die(agent)
        restarted = self.agent()
        self.assertNotIn(key, restarted.later)
        outcomes = self.owner.record(WORKER).kv.get('outcomes', kind='json')
        self.assertEqual(outcomes[key]['status'], 'dropped')
        restarted.tick()
        self.assertEqual(self.received(), [])
        self.die(restarted)
        again = self.agent()
        self.assertNotIn(key, again.later)
        self.clock.now += 120
        again.tick()
        self.assertEqual(self.received(), [])
        warnings = [json.loads(line) for line in self.logs if line.startswith('{') and 'warning' in json.loads(line)]
        self.assertEqual(warnings, [{'id': key, 'warning': 'target_gone'}])

    def test_an_unconfirmed_one_shot_is_dropped_and_leaves_no_entry_or_claim(self):
        agent = self.agent()
        key = self.ask(agent, f'in 1s {TARGET} x')['id']
        self.clock.now += 2
        with patch.object(agent.bus, 'send', side_effect=UncertainOutcome('lost')):
            agent.tick()
        self.assertNotIn(key, agent.later)
        self.assertEqual(self.ask(agent, 'cancel ' + key), 'error: unknown id')
        names = [e['name'] for e in self.owner.record(WORKER).kv.list().get('string', [])]
        self.assertNotIn(f'fired.{key}.once', names)


class Kinds(AtFixture):
    def test_a_stored_subscription_never_fires_in_at(self):
        self.owner.record(WORKER).kv.set('sub.0123456789ab', {'cron': '* * * * *', 'target': TARGET, 'body': 'stray',
                                                               'by': CALLER, 'created': engine_T0}, kind='json')
        agent = self.agent()
        self.clock.now += 120
        agent.tick()
        self.assertEqual((agent.subs, self.received()), ({}, []))


class Package(unittest.TestCase):
    def test_both_packages_ship_the_engine_unchanged(self):
        with tempfile.TemporaryDirectory(dir=ROOT / 'tmp') as work:
            for agent in ('at', 'cron'):
                built = subprocess.run([str(ROOT / f'src/agents/{agent}/package.sh'), f'{work}/{agent}'], capture_output=True, text=True)
                self.assertEqual(built.returncode, 0, built.stderr)
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
