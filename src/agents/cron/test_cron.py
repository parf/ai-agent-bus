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
from agent_bus.errors import BusTimeout, Refusal, TransportError, UncertainOutcome, UnexpectedAnswer  # noqa: E402
import cron  # noqa: E402
import scheduler  # noqa: E402

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
    """RD140: cron sends under its own name; the target's ordinary ACL decides."""

    def test_anyone_admitted_to_cron_may_schedule_and_the_target_acl_decides(self):
        agent = self.agent()
        refused = self.ask(agent, 'sub add * * * * * ' + TARGET + ' closed', self.outsider)
        self.assertRegex(refused['id'], '^[0-9a-f]{12}$')  # adding needs only admission to cron
        shot = self.ask(agent, 'later +1s ' + TARGET + ' closed once', self.outsider)['id']
        self.owner.record(TARGET).manage(allow=[CALLER, OUTSIDER])  # the target stops admitting cron
        self.at(12, 1)
        agent.tick()
        self.assertEqual(self.received(), [])
        self.assertEqual(agent.state[refused['id']]['dropped'], 1)  # that copy is dropped, the subscription stays active
        self.assertNotIn('paused', agent.subs[refused['id']])
        self.assertNotIn(shot, agent.later)  # a one-shot is dropped and removed
        self.assertIn({'warning': 'target_gone', 'id': refused['id']}, [json.loads(line) for line in self.logs if line.startswith('{')])
        self.owner.record(TARGET).manage(allow=[CALLER, OUTSIDER, WORKER])  # positive control: the target admits cron
        self.ask(agent, 'later +1s ' + TARGET + ' open', self.outsider)
        self.at(12, 2)
        agent.tick()
        self.assertEqual(sorted(self.received()), ['closed', 'open'])  # the next slot of the subscription delivers too

    def test_the_creator_must_be_allowed_to_send_at_add(self):
        agent = self.agent()
        self.owner.record(TARGET).manage(allow=[CALLER, WORKER])
        refused = self.ask(agent, 'sub add * * * * * ' + TARGET + ' x', self.outsider)
        self.assertEqual(refused, f'error: you may not send to {TARGET}, so cron will not send there for you')
        self.assertIn('you may not send', self.ask(agent, 'later +1m ' + TARGET + ' x', self.outsider))
        self.assertEqual(self.owner.record(WORKER).kv.list().get('json', []), [])
        self.assertIn('id', self.ask(agent, 'later +1m ' + TARGET + ' x'))  # positive control: the creator may send

    def test_the_creator_losing_access_drops_the_copy_before_any_send(self):
        agent = self.agent()
        sub = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        shot = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.owner.record(TARGET).manage(allow=[WORKER, OUTSIDER])  # cron still admitted, the creator not
        self.at(12, 1)
        agent.tick()
        self.assertEqual(self.received(), [])
        self.assertEqual(agent.state[sub]['dropped'], 1)
        self.assertNotIn('paused', agent.subs[sub])
        self.assertNotIn(shot, agent.later)
        self.assertEqual({n: v for n, v in agent.claims.items() if n.startswith('fired.')},
                         {f'fired.{sub}.2026-10-28T12:01': 'dropped'})  # no attempt was made; the subscription's slot is settled

    def test_a_dropped_slot_is_dropped_once_and_stays_dropped_when_the_creator_is_restored(self):
        agent = self.agent()
        sub = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        self.owner.record(TARGET).manage(allow=[WORKER, OUTSIDER])
        self.at(12, 1)
        agent.tick()
        self.clock.now += 20
        agent.tick()  # the same minute again
        self.assertEqual(agent.state[sub]['dropped'], 1)
        self.owner.record(TARGET).manage(allow=[CALLER, WORKER])  # restored within the minute
        self.clock.now += 20
        agent.tick()
        self.assertEqual(self.received(), [])  # the dropped slot is never delivered
        self.assertEqual(agent.state[sub]['dropped'], 1)
        self.at(12, 2)
        agent.tick()
        self.assertEqual(self.received(), ['tick'])  # the next slot is

    def test_a_refused_one_shot_is_dropped_and_never_retried(self):
        agent = self.agent()
        shot = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(429, 'full')) as send:
            agent.tick()
            self.clock.now += 400
            agent.tick()
        self.assertEqual(send.call_count, 1)
        self.assertNotIn(shot, agent.later)
        self.assertEqual(self.received(), [])
        [done] = self.owner.record(WORKER).kv.get('outcomes', kind='json').values()
        self.assertEqual(done['status'], 'dropped')

    def test_a_dead_daemon_costs_one_question_per_pass(self):
        agent = self.agent()
        for sender, target in ((None, TARGET), (None, 'jobs@fixture'), (self.outsider, TARGET), (self.outsider, 'jobs@fixture')):
            self.ask(agent, f'later +1s {target} x', sender)
        self.at(12, 1)
        with patch.object(agent.bus, 'may_send', side_effect=BusTimeout()) as asked:
            agent.tick()
        self.assertEqual(asked.call_count, 1)
        self.assertEqual(self.received(), [])
        self.assertTrue(all('paused' not in e for e in agent.later.values()))
        self.assertEqual(self.ask(agent, 'later list').get('author_check'), 'unavailable')

    def test_the_check_cap_covers_every_subscription_and_defers_visibly(self):
        self.assertGreaterEqual(scheduler.AUTHOR_CHECKS, cron.MAX_SUBS + 50)
        agent = self.agent()
        for sender, target in ((None, TARGET), (None, 'jobs@fixture'), (self.outsider, TARGET)):
            self.ask(agent, f'later +1s {target} x', sender)
        self.at(12, 1)
        with patch.object(scheduler, 'AUTHOR_CHECKS', 2):
            agent.tick()
        self.assertEqual(len(agent.later), 1)  # two sent, one deferred
        [left] = agent.later
        self.assertEqual(agent.state[left].get('deferred'), 1)
        self.assertNotIn('author_check', self.ask(agent, 'later list'))  # deferral is not a failed check
        self.at(12, 2)
        agent.tick()
        self.assertEqual(agent.later, {})

    def test_each_pass_asks_again(self):
        agent = self.agent()
        self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')
        real = agent.bus.may_send
        with patch.object(agent.bus, 'may_send', side_effect=real) as asked:
            self.at(12, 1)
            agent.tick()
            self.at(12, 2)
            agent.tick()
        self.assertEqual(asked.call_count, 2)


    def test_a_dropped_one_shot_stays_removed_after_a_restart(self):
        agent = self.agent()
        shot = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.owner.record(TARGET).manage(allow=[WORKER])
        self.at(12, 1)
        agent.tick()
        self.die(agent)
        restarted = self.agent()
        self.assertNotIn(shot, restarted.later)
        self.assertEqual([r['status'] for r in self.ask(restarted, 'later list')['finished']], ['dropped'])

    def test_cron_staff_get_no_bypass_at_add(self):
        agent = self.agent()
        self.owner.record(WORKER).manage(maintainers=[OUTSIDER])
        self.owner.record(TARGET).manage(allow=[CALLER, WORKER])
        self.assertIn('you may not send', self.ask(agent, 'later +1m ' + TARGET + ' x', self.outsider))

    def test_an_unavailable_check_refuses_and_sends_nothing(self):
        agent = self.agent()
        with patch.object(agent.bus, 'may_send', side_effect=TransportError('down')):
            self.assertIn('could not check', self.ask(agent, 'later +1m ' + TARGET + ' x'))
        self.assertEqual(agent.later, {})
        shot = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'may_send', side_effect=TransportError('down')):
            agent.tick()
        self.assertEqual(self.received(), [])
        self.assertNotIn('paused', agent.later[shot])
        self.at(12, 2)
        agent.tick()  # the check is back: sent once
        self.assertEqual(self.received(), ['once'])

    def test_an_acl_refusal_drops_one_copy_per_slot_and_the_subscription_survives_a_restart(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(403, 'not allowed')) as send:
            agent.tick()
            agent.tick()  # the same slot: never retried
            self.assertEqual(send.call_count, 1)
            self.clock.now += 3600
            agent.tick()  # the next slot is its own copy
        self.assertEqual(send.call_count, 2)
        self.assertEqual(agent.state[key]['dropped'], 2)
        self.assertNotIn('paused', agent.subs[key])
        self.die(agent)
        self.at(14, 0)
        restarted = self.agent()
        restarted.tick()
        self.assertEqual(self.received(), ['tick'])  # active again: nothing was paused or held

    def test_a_legacy_denied_claim_is_settled_and_the_subscription_stays_active(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        self.owner.record(WORKER).kv.set(f'fired.{key}.2026-10-28T12:00', 'denied')
        self.die(agent)
        restarted = self.agent()
        self.at(12, 1)
        restarted.tick()
        self.assertEqual(self.received(), ['tick'])
        self.assertNotIn('paused', restarted.subs[key])

    def test_a_typo_or_a_non_inbox_target_is_refused_at_add(self):
        agent = self.agent()
        self.assertIn('unknown destination #nope@fixture', self.ask(agent, 'later +1m #nope@fixture x'))
        self.owner.record('news@fixture').register(kind='pubsub', allow=['*'])
        self.assertIn('queue or an agent', self.ask(agent, 'sub add * * * * * news@fixture x'))
        self.assertIn('id', self.ask(agent, 'later +1m jobs@fixture x'))  # positive control: a queue
        self.assertEqual(len(agent.later), 1)

    def test_a_vanished_target_drops_the_copy_instead_of_retrying(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(404, 'no such receiver')) as send:
            agent.tick()
            self.clock.now += 30
            agent.tick()
        self.assertEqual(send.call_count, 1)
        self.assertEqual(agent.state[key]['dropped'], 1)
        self.assertNotIn('paused', agent.subs[key])

    def test_a_vanished_target_removes_a_one_shot_for_good(self):
        agent = self.agent()
        key = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(404, 'no such receiver')):
            agent.tick()
        self.die(agent)
        restarted = self.agent()
        self.assertNotIn(key, restarted.later)

    def test_messages_carry_only_roles_granted_to_cron_itself(self):
        agent = self.agent()
        self.ask(agent, 'later +1s ' + TARGET + ' plain', self.outsider)
        self.at(12, 1)
        agent.tick()
        message = self.target.consume(wait=0.5)
        self.assertEqual(tuple(message.roles), ())  # same Owner as the target, yet no role
        self.owner.record(TARGET).manage(maintainers=[WORKER])  # positive control: a grant to cron itself
        self.ask(agent, 'later +1s ' + TARGET + ' granted', self.outsider)
        self.at(12, 2)
        agent.tick()
        self.assertEqual(tuple(self.target.consume(wait=0.5).roles), ('maintainer',))

    def test_every_firing_names_its_creator(self):
        agent = self.agent()
        self.ask(agent, 'later +1s ' + TARGET + ' {"method":"x"}')
        self.at(12, 1)
        agent.tick()
        message = self.target.consume(wait=0.5)
        self.assertIsNotNone(message)
        self.assertEqual((message.body, message.envelope['from'], message.tag), ('{"method":"x"}', WORKER, 'by ' + CALLER))

    def test_not_to_itself_and_per_creator_caps(self):
        agent = self.agent()
        self.assertEqual(self.ask(agent, 'later +1m #worker@fixture hi'), 'error: cron does not schedule messages to itself')
        for i in range(cron.SUBS_EACH):
            self.assertIn('id', self.ask(agent, f'sub add {i} * * * * {TARGET} n{i}'))
        self.assertIn('per creator', self.ask(agent, f'sub add 59 * * * * {TARGET} over'))
        self.assertEqual(len(agent.subs), cron.SUBS_EACH)

    def test_delete_and_cancel_need_the_creator_or_cron_staff(self):
        agent = self.agent()
        sub = self.ask(agent, 'sub add * * * * * ' + TARGET + ' hi')['id']
        shot = self.ask(agent, 'later +5m ' + TARGET + ' hi')['id']
        self.assertIn('creator', self.ask(agent, 'sub delete ' + sub, self.outsider))
        self.assertIn('creator', self.ask(agent, 'later cancel ' + shot, self.outsider))
        self.assertIn(sub, agent.subs)
        self.assertEqual(self.ask(agent, 'sub delete ' + sub), {'deleted': sub})  # the creator
        self.owner.record(WORKER).manage(maintainers=[OUTSIDER])
        self.assertEqual(self.ask(agent, 'later cancel ' + shot, self.outsider), {'deleted': shot})  # cron's staff

    def test_lists_show_bodies_only_to_the_creator(self):
        agent = self.agent()
        self.ask(agent, 'sub add 0 9 * * * ' + TARGET + ' secret command')
        mine = self.ask(agent, 'sub list')['subs']
        self.assertEqual([s['body'] for s in mine], ['secret command'])
        self.assertEqual(self.ask(agent, 'sub list', self.outsider)['subs'], [])
        self.owner.record(WORKER).manage(maintainers=[OUTSIDER])
        staff = self.ask(agent, 'sub list', self.outsider)['subs']
        self.assertEqual(len(staff), 1)
        self.assertNotIn('body', staff[0])


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
        self.assertEqual(restarted.invalid.get('0123456789ab'), ('sub', CALLER))

    def test_invalid_entries_are_listed_and_deletable_by_their_creator(self):
        self.owner.record(WORKER).kv.set('sub.0123456789ab', {'cron': '61 * * * *', 'target': TARGET, 'body': 'x', 'by': CALLER,
                                                               'created': T0}, kind='json')
        self.owner.record(WORKER).kv.set('later.0123456789ac', {'at': 'soon', 'target': TARGET, 'body': 'x',
                                                                 'by': CALLER, 'created': T0}, kind='json')
        agent = self.agent()
        self.assertEqual(sorted(agent.invalid), ['0123456789ab', '0123456789ac'])  # 61 minutes; a non-number at
        rows = self.ask(agent, 'sub list')['subs']
        self.assertEqual(rows, [{'id': '0123456789ab', 'invalid': True, 'by': CALLER}])
        self.assertEqual(self.ask(agent, 'later list')['later'], [{'id': '0123456789ac', 'invalid': True, 'by': CALLER}])
        self.assertIn('creator', self.ask(agent, 'sub delete 0123456789ab', self.outsider))
        self.assertEqual(self.ask(agent, 'sub delete 0123456789ab'), {'deleted': '0123456789ab'})
        self.assertNotIn('sub.0123456789ab', [e['name'] for e in self.owner.record(WORKER).kv.list().get('json', [])])

    def test_a_stored_entry_that_breaks_a_pass_is_set_aside(self):
        agent = self.agent()
        self.ask(agent, 'sub add * * * * * ' + TARGET + ' good')
        bad = self.ask(agent, 'sub add * * * * * ' + TARGET + ' bad')['id']
        agent.subs[bad]['spec'] = None
        self.at(12, 1)
        try:
            agent.tick()
        except Exception as e:
            self.fail('one entry stopped the pass: ' + repr(e))
        self.assertEqual(self.received(), ['good'])
        self.assertEqual(agent.invalid.get(bad, ('',))[0], 'sub')


class Clocks(Fixture):
    def test_a_dense_schedule_fires_once_per_local_minute_across_fall_back(self):
        agent = self.agent({'tz': 'America/New_York'})
        self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')
        start = datetime(2026, 11, 1, 5, 0, 5, tzinfo=timezone.utc).timestamp()  # 01:00 EDT
        for i in range(120):  # 01:00 EDT .. 01:59 EST: 120 instants, 60 local minutes
            self.clock.now = start + 60 * i
            agent.tick()
        self.assertEqual(len(self.received()), 60)
        self.clock.now = start + 60 * 120  # 02:00 EST fires again
        agent.tick()
        self.assertEqual(len(self.received()), 1)

    def test_later_in_the_past_refused_beyond_the_skew(self):
        agent = self.agent()
        late = datetime.fromtimestamp(T0 - 60, timezone.utc).isoformat()
        self.assertEqual(self.ask(agent, f'later {late} {TARGET} x'), 'error: at is in the past')
        near = datetime.fromtimestamp(T0 - 10, timezone.utc).isoformat()
        self.assertIn('id', self.ask(agent, f'later {near} {TARGET} near'))
        agent.tick()
        agent.tick()
        self.assertEqual(self.received(), ['near'])
        naive = datetime.fromtimestamp(T0 + 60, timezone.utc).replace(tzinfo=None).isoformat()
        self.assertIn('offset', self.ask(agent, f'later {naive} {TARGET} x'))

    def test_another_process_claim_is_never_cleaned_up_here(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        other = f'fired.{key}.2026-10-28T12:00'
        self.owner.record(WORKER).kv.set(other, 'sending')
        self.at(12, 0)
        with patch.object(agent, 'scheduler', return_value=True):
            agent.tick()  # 412 on the claim: another process's
            self.at(12, 1)
            agent.tick()  # this process sends and cleans up its older claims
        self.assertEqual(self.received(), ['tick'])
        self.assertIn(other, [e['name'] for e in self.owner.record(WORKER).kv.list().get('string', [])])


class Delivery(Fixture):
    """RD161: a refused or unconfirmed copy is dropped with one warning, never resent; the subscription stays active."""

    def test_an_unconfirmed_send_is_dropped_and_not_resent(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=UncertainOutcome('lost')):
            agent.tick()
        self.assertEqual(agent.state[key]['dropped'], 1)
        self.assertNotIn('paused', agent.subs[key])
        agent.tick()  # the same slot is never repeated
        self.assertEqual(self.received(), [])
        self.at(12, 2)
        agent.tick()  # the next slot is delivered: the subscription stayed active
        self.assertEqual(self.received(), ['tick'])
        self.die(agent)
        self.assertNotIn('paused', self.agent().subs[key])

    def test_an_unconfirmed_one_shot_is_dropped_and_removed(self):
        agent = self.agent()
        key = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=UncertainOutcome('lost')):
            agent.tick()
        self.assertNotIn(key, agent.later)
        self.die(agent)
        restarted = self.agent()
        restarted.tick()
        self.assertEqual(self.received(), [])
        self.assertEqual(restarted.later, {})
        self.assertEqual([r['status'] for r in self.ask(restarted, 'later list')['finished']], ['dropped'])

    def test_a_legacy_unconfirmed_claim_is_settled_and_the_subscription_stays_active(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        self.owner.record(WORKER).kv.set(f'fired.{key}.2026-10-28T12:00', 'unconfirmed')
        self.die(agent)
        restarted = self.agent()
        self.at(12, 1)
        restarted.tick()
        self.assertEqual(self.received(), ['tick'])
        self.assertNotIn('paused', restarted.subs[key])

    def test_a_legacy_unconfirmed_claim_of_the_current_slot_is_dropped_and_not_sent_on_restart(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        self.owner.record(WORKER).kv.set(f'fired.{key}.2026-10-28T12:01', 'unconfirmed')
        self.die(agent)
        restarted = self.agent()
        self.at(12, 1)
        restarted.tick()
        restarted.tick()
        self.assertEqual(self.received(), [])
        self.assertEqual(restarted.claims[f'fired.{key}.2026-10-28T12:01'], 'dropped')
        self.assertEqual(restarted.state[key]['dropped'], 1)

    def test_a_legacy_retrying_one_shot_is_dropped_on_restart_instead_of_sent(self):
        agent = self.agent()
        key = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.owner.record(WORKER).kv.set(f'fired.{key}.once', 'retrying 1 ' + str(int(T0) + 60))
        self.die(agent)
        self.at(12, 1)
        restarted = self.agent()
        restarted.tick()
        self.assertEqual(self.received(), [])
        self.assertNotIn(key, restarted.later)

    def test_an_orphan_in_flight_claim_is_swept(self):
        name = 'fired.0123456789ab.2026-10-28T12:00'
        self.owner.record(WORKER).kv.set(name, 'sending')
        self.agent()
        self.assertNotIn(name, [e['name'] for e in self.owner.record(WORKER).kv.list().get('string', [])])

    def test_a_server_error_is_an_unknown_outcome_and_drops_the_copy(self):
        agent = self.agent()
        key = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(500, 'internal')):
            agent.tick()
        self.assertNotIn(key, agent.later)
        self.at(12, 2)
        agent.tick()
        self.assertEqual(self.received(), [])

    def test_a_stop_between_send_and_success_record_never_resends_and_loses_no_later_slot(self):
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
        delivered = self.received()
        self.die(agent)
        restarted = self.agent()  # the claim is still `sending`: that copy may have arrived, so it is dropped, never sent again
        self.assertNotIn('paused', restarted.subs[key])
        self.at(12, 1)
        restarted.tick()
        delivered += self.received()
        self.at(12, 2)
        restarted.tick()
        delivered += self.received()
        self.assertEqual(delivered, ['tick', 'tick'])  # slot 12:01 once, slot 12:02 once: none twice, none missing

    def test_a_stop_between_the_claim_and_the_send_loses_only_that_copy(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']

        def stop_before_send(*args, **kwargs):
            raise Stop()
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=stop_before_send), self.assertRaises(Stop):
            agent.tick()
        self.die(agent)
        restarted = self.agent()
        self.at(12, 1)
        restarted.tick()
        self.assertEqual(self.received(), [])  # that slot's copy is dropped, not retried
        self.assertEqual(restarted.state[key]['dropped'], 1)
        self.at(12, 2)
        restarted.tick()
        self.assertEqual(self.received(), ['tick'])  # the next slot is not missing

    def test_a_refused_one_shot_is_not_held_across_a_restart(self):
        agent = self.agent()
        key = self.ask(agent, 'later +1s ' + TARGET + ' once')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(429, 'full')):
            agent.tick()
        self.assertNotIn(key, agent.later)
        self.die(agent)
        restarted = self.agent()
        restarted.tick()
        self.assertEqual(self.received(), [])
        self.assertEqual(restarted.later, {})

    def test_a_refusal_drops_one_copy_per_slot_with_no_backoff(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add 1 12 * * * ' + TARGET + ' tick')['id']
        self.at(12, 1)
        with patch.object(agent.bus, 'send', side_effect=Refusal(409, 'full')) as send:
            agent.tick()
            self.clock.now += 5
            agent.tick()
            self.clock.now += 6
            agent.tick()
            self.assertEqual(send.call_count, 1)  # one attempt for the slot, however long it takes
        self.assertEqual(agent.state[key]['outcome'], 'dropped')
        self.assertEqual(agent.state[key]['dropped'], 1)
        self.assertNotIn('paused', agent.subs[key])

    def test_sends_carry_a_ttl(self):
        agent = self.agent()
        self.ask(agent, 'sub add */5 * * * * ' + TARGET + ' tick')
        self.at(12, 5)
        with patch.object(agent.bus, 'send', return_value='m1') as send:
            agent.tick()
        self.assertEqual(send.call_args.kwargs.get('ttl'), 300)

    def test_the_claim_cleanup_keeps_an_in_flight_claim_as_evidence(self):
        agent = self.agent()
        name = 'fired.0123456789ab.2026-10-28T12:01'
        agent.claim(name, 'sending', 'set')
        agent.drop_claim(name)
        self.assertIn(name, [e['name'] for e in self.owner.record(WORKER).kv.list().get('string', [])])

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


class BusLoss(unittest.TestCase):
    """J30: the loop gives up after a bounded run of bus failures, so the runner restarts it."""

    def run_loop(self, script):
        consumed = []

        class Bus:
            def __init__(self, timeout):
                pass

            def __enter__(self):
                return self

            def __exit__(self, *exc):
                return False

            def consume(self, wait=None):
                consumed.append(wait)
                if len(consumed) > len(script):
                    raise scheduler.Shutdown()  # past the script: end the loop the way SIGTERM does
                if script[len(consumed) - 1]:
                    return None
                raise TransportError('no daemon at the socket')

        class Agent:
            NAME, PACKAGE = 'fixture', None

            def __init__(self, bus, cfg):
                self.loaded = True

            def tick(self):
                pass

            def wait(self):
                return 0

        env = {k: v for k, v in os.environ.items() if k != 'AGENT_BUS_ASK'}
        with patch.object(scheduler, 'Bus', Bus), patch.object(scheduler.time, 'sleep', lambda _: None), \
                patch.object(scheduler.signal, 'signal', lambda *_: None), patch.dict(os.environ, env, clear=True), \
                patch('sys.stderr'):
            return scheduler.run(Agent), len(consumed)

    def test_it_exits_non_zero_after_bounded_consecutive_failures(self):
        n = scheduler.BUS_FAILURES
        self.assertGreater(n, 1)
        rc, calls = self.run_loop([False] * n)
        self.assertEqual((rc, calls), (1, n))
        self.assertNotEqual(rc, 78, '78 tells the runner the agent is unconfigured, which it does not restart')

    def test_a_good_call_starts_the_count_again(self):
        n = scheduler.BUS_FAILURES
        rc, calls = self.run_loop([False] * (n - 1) + [True] + [False] * n)
        self.assertEqual((rc, calls), (1, 2 * n))


if __name__ == '__main__':
    unittest.main()


class Migration(Fixture):
    """B.29: an id that is not 12 hex moves to one that is, deterministically, and a paused entry is active again."""

    def put(self, name, value):
        self.owner.record(WORKER).kv.set(name, value, kind='json')

    def keys(self):
        return sorted(e['name'] for e in self.owner.record(WORKER).kv.list().get('json', []))

    def entry(self, by=CALLER):
        return {'cron': '* * * * *', 'target': TARGET, 'body': 'tick', 'by': by, 'created': T0}

    def test_a_legacy_id_moves_to_the_same_12_hex_id_every_time_and_keeps_firing(self):
        import hashlib
        self.put('sub.Legacy-1', self.entry())
        agent = self.agent()
        new = hashlib.sha256(b'sub.Legacy-1').hexdigest()[:12]
        self.assertEqual(self.keys(), ['sub.' + new])
        self.assertIn(new, agent.subs)
        self.at(12, 1)
        agent.tick()
        self.assertEqual(self.received(), ['tick'])
        self.die(agent)
        self.assertEqual(self.keys(), ['sub.' + new])  # a restart changes nothing

    def test_a_claim_under_the_old_id_moves_with_its_entry_and_the_slot_is_not_sent_again(self):
        import hashlib
        new = hashlib.sha256(b'sub.Legacy-1').hexdigest()[:12]
        self.put('sub.Legacy-1', self.entry())
        kv = self.owner.record(WORKER).kv
        kv.set('fired.Legacy-1.2026-10-28T12:01', 'sending')
        agent = self.agent()
        names = {e['name'] for e in kv.list().get('string', [])}
        self.assertEqual({n for n in names if n.startswith('fired.')} - {f'fired.{new}.2026-10-28T12:01'}, set())
        self.at(12, 1)
        agent.tick()
        agent.tick()
        self.assertEqual(self.received(), [])  # the claimed copy of that slot was dropped, not sent again
        self.assertEqual(agent.claims[f'fired.{new}.2026-10-28T12:01'], 'dropped')

    def test_a_taken_id_moves_the_entry_to_the_next_digest(self):
        import hashlib
        first = hashlib.sha256(b'sub.Legacy-2').hexdigest()[:12]
        second = hashlib.sha256(first.encode()).hexdigest()[:12]
        self.put('sub.' + first, self.entry(by=OUTSIDER))  # someone else's entry already holds the first candidate
        self.put('sub.Legacy-2', self.entry())
        agent = self.agent()
        self.assertEqual(self.keys(), sorted(['sub.' + first, 'sub.' + second]))
        self.assertEqual(agent.subs[first]['by'], OUTSIDER)
        self.assertEqual(agent.subs[second]['by'], CALLER)

    def test_a_move_interrupted_after_the_write_is_finished_on_restart(self):
        import hashlib
        new = hashlib.sha256(b'later.Old').hexdigest()[:12]
        shot = {'at': T0 + 600, 'target': TARGET, 'body': 'once', 'by': CALLER, 'created': T0}
        self.put('later.Old', shot)
        self.put('later.' + new, shot | {'migrated_from': 'Old'})  # written, the old key not yet removed
        agent = self.agent()
        self.assertEqual(self.keys(), ['later.' + new])
        self.assertEqual(list(agent.later), [new])

    def test_a_paused_entry_from_an_earlier_release_is_active_again(self):
        key = '0123456789ab'
        self.put('sub.' + key, self.entry() | {'paused': 'denied: test'})
        self.put('later.ba9876543210', {'at': T0 + 1, 'target': TARGET, 'body': 'once', 'by': CALLER, 'created': T0, 'paused': 'unconfirmed'})
        agent = self.agent()
        self.assertNotIn('paused', agent.subs[key])
        self.assertNotIn('paused', self.owner.record(WORKER).kv.get('sub.' + key, kind='json'))
        self.assertEqual(agent.later, {})  # a held one-shot is dropped
        self.at(12, 1)
        agent.tick()
        self.assertEqual(self.received(), ['tick'])

    def test_sub_edit_is_refused_and_changes_nothing(self):
        agent = self.agent()
        key = self.ask(agent, 'sub add * * * * * ' + TARGET + ' tick')['id']
        for body in ('sub edit ' + key + ' * * * * * ' + TARGET + ' other', 'sub edit', '{"method": "sub.edit", "id": "' + key + '"}'):
            self.assertEqual(self.ask(agent, body), 'error: sub edit is not supported; delete it and add it again')
        self.assertEqual(agent.subs[key]['body'], 'tick')
