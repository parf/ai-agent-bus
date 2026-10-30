import copy
import json
import os
from pathlib import Path
import queue
import tempfile
import time
import unittest
from types import SimpleNamespace
from unittest.mock import patch

from cleaning import clean_line
from filters import match
import logwatch
import subscription_migration as migration
from logwatch import Agent, parse
from agent_bus.errors import BusError, Refusal, UnexpectedAnswer
from watcher import Monitor, Ring, configuration


class FakeKV:
    def __init__(self): self.values = {}
    def list(self): return {'json': [{'name': key} for key in self.values]}
    def get(self, key, **_):
        if key not in self.values: raise Refusal(404, 'absent')
        return copy.deepcopy(self.values[key])
    def set(self, key, value, mode='set', **_):
        if mode == 'replace' and key not in self.values: raise Refusal(404, 'absent')
        if mode == 'add' and key in self.values: raise Refusal(412, 'exists')
        self.values[key] = copy.deepcopy(value)
    def delete(self, key, **_): return self.values.pop(key, None) is not None
    def json(self, key, *operations):
        for op in operations:
            assert op['op'] == 'set'
            self.values[key][op['key']] = op['value']


class FakeBus:
    name = '#logwatch@t'
    def __init__(self):
        self.kv = FakeKV(); self.sent = []; self.attempts = []; self.manages = True
        self.may = True; self.asked = []; self.send_failures = []; self.kinds = {}; self.canonical = {}
    def may_send(self, who, name):
        self.asked.append((who, name))
        value = self.may.get((who, name), True) if isinstance(self.may, dict) else self.may
        if isinstance(value, Exception): raise value
        return value
    def record(self, name):
        return SimpleNamespace(kv=self.kv, lookup=lambda: {'name':self.canonical.get(name,name), 'kind':self.kinds.get(name, 'agent')})
    def send(self, target, body, **options):
        self.attempts.append((target, body, options))
        if self.send_failures:
            failure = self.send_failures.pop(0)
            if failure: raise failure
        self.sent.append((target, body, options))
    def _request(self, *_args, **_kwargs): return {'manages': self.manages}


def message(body, roles=(), original_to=None, who='#caller@t'):
    return SimpleNamespace(body=body, roles=roles, original_to=original_to, ask=None,
        envelope={'from':who, 'roles':list(roles), 'original_to':original_to}, **{'from':who})


class WatcherTests(unittest.TestCase):
    def test_byte_caps_bound_partial_ring_and_startup(self):
        agent = self.agent(ring_size=100, line_bytes=16384, ring_bytes=32768, startup_bytes=65536)
        with tempfile.TemporaryDirectory(dir=Path(__file__).resolve().parents[4]/'tmp') as directory:
            file = Path(directory)/'large.log'; file.write_bytes(b'initial\n')
            agent.config['sources'] = [str(file)]
            watch = agent.monitor
            try:
                watch.tick()
                with file.open('ab') as out: out.write(b'x' * (7 * 1024 * 1024))
                for _ in range(112):
                    watch.tick()
                    self.assertLessEqual(len(watch.files[str(file)]['partial'].head), 16384)
                with file.open('ab') as out: out.write(b'\n')
                watch.tick()
                line = agent.ring.snapshot()[-1]['text']
                self.assertLessEqual(len(line.encode()), 16384)
                self.assertTrue(line.startswith('x' * 100))
                kept, marker = line.split('…[truncated ')
                self.assertEqual(marker, str(7*1024*1024-len(kept))+' bytes]')
                for n in range(5): agent.ring.add('file', str(n) + 'é'*10000)
                self.assertLessEqual(agent.ring.bytes, 32768)
                self.assertLess(len(agent.ring.snapshot()), 5)
                self.assertTrue(agent.ring.snapshot()[-1]['text'].startswith('4'))
                self.assertTrue(all(len(row['text'].encode()) <= 16384 for row in agent.ring.snapshot()))
            finally: watch.close()
            # Startup reads only a bounded window, not backwards through the huge line.
            import io
            reads = []
            class TrackingFile(io.BytesIO):
                def read(self, size=-1):
                    reads.append(size)
                    return super().read(size)
            with patch('watcher.open', lambda *_: TrackingFile(file.read_bytes())):
                startup = Monitor(agent.config, Ring())
                try: startup.rescan()
                finally: startup.close()
            self.assertEqual(reads, [65536])

    def test_byte_cap_configuration_is_closed_and_bounded(self):
        defaults = configuration({})
        self.assertEqual([defaults[k] for k in ('line_bytes','ring_bytes','startup_bytes')], [16384,8388608,65536])
        for key in ('line_bytes','ring_bytes','startup_bytes'):
            for value in (0, True, 1.5, 2**40):
                with self.subTest(key=key,value=value), self.assertRaises(ValueError): configuration({key:value})

    def test_oversized_journal_record_has_a_bounded_unknown_head(self):
        import io
        agent = self.agent()
        raw = json.dumps({'MESSAGE':'x'*(2*1024*1024)}).encode()
        agent.monitor.journal = SimpleNamespace(stdout=io.BytesIO(raw+b'\n'),wait=lambda:None)
        agent.monitor.journal_reader()
        agent.monitor.journal = None
        head, omitted = agent.monitor.journal_lines.queue[0]
        self.assertEqual(len(head),16384)
        self.assertEqual(omitted,len(raw)-len(head))
        agent.monitor.tick()
        line=agent.ring.snapshot()[0]
        self.assertEqual(line['source'],'journal:unknown')
        self.assertLessEqual(len(line['text'].encode()),16384)
        self.assertIn('…[truncated ',line['text'])

    def agent(self, **overrides):
        bus = FakeBus()
        def delivery(rows, body, topic, tag, deadline):
            cache = {}
            return {r['id']: logwatch.shared_subscriptions.deliver_one(bus, bus.name, r, body, topic, tag, deadline, cache) for r in rows}
        agent = Agent(bus, configuration({'sources': [], 'excludes': [], **overrides}), log=lambda _:None, delivery=delivery)
        self.addCleanup(agent.close)
        return agent

    def test_j21_invalid_filter_is_refused_at_add_without_storing(self):
        agent = self.agent()
        self.assertEqual(agent.handle(message('sub add #target@t /(/')), 'error: invalid_filter')
        self.assertEqual(agent.bus.kv.values, {})

    def test_tail_is_unfiltered_oldest_first_and_ring_is_bounded(self):
        agent = self.agent(ring_size=7)
        for number in range(40): agent.ring.add('fixture', f'ordinary {number}')
        result = json.loads(agent.handle(message('{"method":"tail","n":5}')))
        self.assertEqual([line['text'] for line in result['lines']], [f'ordinary {n}' for n in range(35,40)])
        self.assertEqual(len(agent.ring.snapshot()), 7)
        stats = json.loads(agent.handle(message('{"method":"stats"}')))
        self.assertEqual((stats['ring_size'],stats['fill'],stats['lines_seen'],stats['dropped']), (7,7,40,33))
        self.assertIn('[fixture] ordinary 39', agent.handle(message('tail 1')))

    def test_mark_excludes_old_lines_and_since_uses_arrival_time(self):
        now = [1000.]
        agent = self.agent(); agent.clock = lambda:now[0]; agent.ring.clock = agent.clock
        agent.ring.add('file','before'); self.assertEqual(agent.handle(message('mark change')), 'change')
        now[0] += 100; agent.ring.add('file','after')
        result = json.loads(agent.handle(message('{"method":"after","label":"change"}')))
        self.assertEqual([line['text'] for line in result['lines']], ['after'])
        self.assertEqual(agent.handle(message('since 1m')), '[file] after')
        self.assertIn('unknown mark', agent.handle(message('after missing')))

    def test_cleaning_preserves_plain_text_and_removes_exact_legacy_forms(self):
        text = '\x1b[31mred\x1b[0m #033[32mgreen [0;38;05;145mplain \x00\x01\x02end\tkeep\r\n'
        self.assertEqual(clean_line(text), 'red green plain ...end\tkeep\r')
        self.assertEqual(clean_line('\x1b]title\x07hello\x1bM'), 'hello')
        ring = Ring(); ring.add('file',text)
        self.assertNotIn('\x1b', ring.snapshot()[0]['text'])
        self.assertNotIn('#033', ring.snapshot()[0]['text'])

    def test_rescan_picks_up_new_file_at_end_excludes_and_follows_rotation(self):
        with tempfile.TemporaryDirectory(dir=Path(__file__).resolve().parents[4]/"tmp") as directory:
            root=Path(directory); old=root/'old.log';old.write_text('initial\n')
            (root/'excluded.log').write_text('never\n'); (root/'empty.log').touch()
            ring=Ring(); watch=Monitor(configuration({'sources':[str(root/'*.log')], 'excludes':[str(root/'excluded*')], 'rescan_interval':.05}), ring)
            try:
                watch.tick(); self.assertEqual([l['text'] for l in ring.snapshot()],['initial'])
                new=root/'new.log';new.write_text('old content\n');time.sleep(.06);watch.tick()
                with new.open('a') as file:file.write('new arrival\n')
                watch.tick();self.assertEqual([l['text'] for l in ring.snapshot()],['initial','new arrival'])
                old.rename(root/'old.previous');old.write_text('replacement existing\n');watch.tick();watch.tick()
                with old.open('a') as file:file.write('after rotation\n')
                watch.tick();self.assertEqual(ring.snapshot()[-1]['text'],'after rotation')
                self.assertNotIn('never','\n'.join(l['text'] for l in ring.snapshot()))
                self.assertEqual(next(row for row in watch.sources() if row['source'].endswith('excluded.log'))['state'],'excluded')
            finally:watch.close()

    def test_syslog_selects_each_platform_file(self):
        with patch('watcher.Path.is_file', lambda p:str(p)=='/var/log/syslog'):
            self.assertEqual(Monitor.syslog(), ['/var/log/syslog'])
        with patch('watcher.Path.is_file', lambda p:str(p)=='/var/log/messages'):
            self.assertEqual(Monitor.syslog(), ['/var/log/messages'])

    def test_journal_single_reader_attributes_units_all_and_permission_errors(self):
        with tempfile.TemporaryDirectory(dir=Path(__file__).resolve().parents[4]/"tmp") as directory:
            root=Path(directory);program=root/'journalctl';record=root/'argv.json'
            program.write_text('''#!/usr/bin/env python3
import sys,json,time,os
from pathlib import Path
Path(os.environ['JOURNAL_ARGV']).write_text(json.dumps(sys.argv[1:]))
if 'denied.service' in sys.argv:
 print('cannot read denied.service: permission denied',flush=True);sys.exit(1)
print(json.dumps({'_SYSTEMD_UNIT':'web.service','SYSLOG_IDENTIFIER':'python3','_HOSTNAME':'fixture','__REALTIME_TIMESTAMP':'1700000000000000','MESSAGE':'\\u001b[31mjournal arrival\\u001b[0m'}),flush=True)
print(json.dumps({'SYSLOG_IDENTIFIER':'kernel','MESSAGE':'kernel message'}),flush=True)
time.sleep(30)
''');program.chmod(0o755)
            with patch.dict(os.environ,{'PATH':directory+':'+os.environ['PATH'],'JOURNAL_ARGV':str(record)}):
                for units in [['web.service'],'all',['denied.service']]:
                    ring=Ring();watch=Monitor(configuration({'sources':[],'excludes':[],'journal':units}),ring)
                    try:
                        end=time.monotonic()+3
                        while time.monotonic()<end:
                            watch.tick()
                            if len(ring.snapshot()) >= 2 or watch.journal_error:break
                            time.sleep(.01)
                        args=json.loads(record.read_text());self.assertEqual(args.count('-f'),1);self.assertEqual(args[args.index('-o')+1],'json')
                        if units=='all':self.assertNotIn('-u',args)
                        else:self.assertEqual(args[args.index('-u')+1],units[0])
                        if units==['denied.service']:
                            row=next(row for row in watch.sources() if row['source']=='journal:denied.service')
                            self.assertEqual(row['state'],'unreadable');self.assertIn('permission denied',row['error'])
                        else:
                            self.assertTrue(ring.snapshot(), 'journal lines were dropped')
                            self.assertEqual(ring.snapshot()[0]['source'],'journal:web.service')
                            self.assertIn('journal arrival',ring.snapshot()[0]['text']);self.assertNotIn('\x1b',ring.snapshot()[0]['text'])
                            self.assertIn('journal:kernel',[line['source'] for line in ring.snapshot()])
                    finally:watch.close()

    def test_shared_subscription_engine_is_not_a_local_shadow(self):
        self.assertEqual(Path(logwatch.shared_subscriptions.__file__).resolve(),
                         Path(__file__).resolve().parents[2] / 'shared/subscriptions.py')
        self.assertIs(type(self.agent().subscriptions), logwatch.shared_subscriptions.Subscriptions)

    def test_subscriptions_send_every_repeated_match_and_persist_counts(self):
        agent = self.agent()
        added = json.loads(agent.handle(message('sub add #target@t /error/i')))
        lines = [agent.ring.add('file', 'ERROR repeated') for _ in range(37)] + [agent.ring.add('file', 'ordinary')]
        agent.forward(lines)
        self.assertEqual(len(agent.bus.sent), 37)
        self.assertTrue(all(body == '[file] ERROR repeated' for _, body, _ in agent.bus.sent))
        self.assertTrue(all('wait' not in options for _, _, options in agent.bus.sent))
        agent.forward([])
        self.assertEqual(len(agent.bus.sent), 37)
        restored = Agent(agent.bus, agent.config, log=lambda _:None)
        self.addCleanup(restored.close); restored.load()
        rows = json.loads(restored.handle(message('sub list')))['subs']
        self.assertEqual([(r['id'], r['sent'], r['dropped']) for r in rows], [(added['id'], 37, 0)])
        self.assertEqual(set(rows[0]), logwatch.shared_subscriptions.FIELDS)

    def test_refused_copy_warns_once_and_the_next_identical_match_is_delivered(self):
        agent = self.agent(); warnings = []; agent.log = warnings.append
        agent.handle(message('sub add #target@t /error/i'))
        agent.bus.send_failures = [Refusal(429, 'full'), None]
        agent.forward([agent.ring.add('file', 'ERROR repeat'), agent.ring.add('file', 'ERROR repeat')])
        self.assertEqual(len(agent.bus.attempts), 2)
        self.assertEqual([body for _, body, _ in agent.bus.sent], ['[file] ERROR repeat'])
        self.assertEqual(warnings, ['subscription: send_refused'])
        rows = json.loads(agent.handle(message('sub list')))['subs']
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual((row['sent'], row['dropped']), (1, 1))
        self.assertNotIn('paused', row)

    def test_author_is_checked_for_target_and_source_at_add_and_each_copy(self):
        agent = self.agent(); warnings = []; agent.log = warnings.append
        for target in ('#target@t', agent.bus.name):
            agent.bus.may = {('#caller@t', target):False}
            self.assertEqual(agent.handle(message('sub add #target@t')), 'error: admission_refused')
            self.assertEqual(agent.bus.kv.values, {})
        agent.bus.may = True
        agent.handle(message('sub add #target@t /error/i'))
        for target in ('#target@t', agent.bus.name):
            agent.bus.may = {('#caller@t', target):False}
            before = len(agent.bus.sent)
            agent.forward([agent.ring.add('file', 'ERROR revoked')])
            self.assertEqual(len(agent.bus.sent), before)
            self.assertEqual(warnings[-1], 'subscription: admission_refused')
            agent.bus.may = True
            agent.forward([agent.ring.add('file', 'ERROR recovered')])
            self.assertEqual(len(agent.bus.sent), before + 1)
        for failure in (BusError('down'), UnexpectedAnswer('old daemon'), OSError('socket')):
            agent.bus.may = failure
            before = len(agent.bus.sent)
            agent.forward([agent.ring.add('file', 'ERROR unavailable')])
            self.assertEqual(len(agent.bus.sent), before)
            self.assertEqual(warnings[-1], 'subscription: admission_unavailable')
            self.assertEqual(agent.handle(message('sub add #other@t')), 'error: admission_unavailable')
        agent.bus.may = True
        agent.forward([agent.ring.add('file', 'ERROR recovered')])
        rows = json.loads(agent.handle(message('sub list')))['subs']
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual((row['sent'], row['dropped']), (3, 5))
        self.assertNotIn('paused', row)

    def test_creators_manage_only_their_rows_and_forwarded_manager_roles_do_not_expand_visibility(self):
        agent = self.agent(); agent.bus.manages = False
        mine = json.loads(agent.handle(message('sub add #target@t')))['id']
        other = json.loads(agent.handle(message('sub add #other@t', who='#other-author@t')))['id']
        self.assertEqual([r['id'] for r in json.loads(agent.handle(message('sub list')))['subs']], [mine])
        self.assertEqual(agent.handle(message('sub delete '+other)), 'error: not_author')
        forwarded = message('sub list', roles=('owner',), original_to='#forwarder@t')
        self.assertEqual([r['id'] for r in json.loads(agent.handle(forwarded))['subs']], [mine])
        self.assertEqual(len(json.loads(agent.handle(message('sub list', roles=('maintainer',))))['subs']), 2)
        self.assertEqual(json.loads(agent.handle(message('sub delete '+other, roles=('owner',))))['deleted'], other)

    def test_legacy_definitions_keep_author_counts_and_inert_filters_without_pauses(self):
        agent = self.agent(); agent.clock = lambda:1000.
        base = {'target':'#target@t', 'filter':'/error/i', 'created_by':'#caller@t'}
        agent.bus.kv.set('sub.000000000001', dict(base, id='000000000001', forwarded=7, dropped=2, paused='denied'))
        agent.bus.kv.set('sub.000000000002', dict(base, id='000000000002', filter='/(/', disabled=True))
        agent.bus.kv.set('sub.000000000003', {'id':'000000000003', 'target':'#target@t', 'filter':''})
        agent.load(); once = copy.deepcopy(agent.bus.kv.values); agent.load()
        self.assertEqual(agent.bus.kv.values, once, 'migration rewrote a shared row')
        rows = json.loads(agent.handle(message('sub list')))['subs']
        self.assertEqual([r['id'] for r in rows], ['000000000001','000000000002'])
        self.assertEqual((rows[0]['created_by'], rows[0]['created_at'], rows[0]['sent'], rows[0]['dropped']),
                         ('#caller@t','1970-01-01T00:16:40.000Z',7,2))
        self.assertEqual(rows[1]['reason'], 'invalid_filter')
        managers = json.loads(agent.handle(message('sub list', roles=('owner',))))['subs']
        self.assertEqual(managers[-1], {'id':'000000000003','reason':'invalid_state'})
        warnings = []; agent.log = warnings.append
        agent.forward([agent.ring.add('file', 'ERROR arrival')])
        self.assertEqual(len(agent.bus.sent), 1)
        self.assertEqual(warnings, ['subscription: filter_invalid'])
        rows = json.loads(agent.handle(message('sub list')))['subs']
        self.assertEqual((rows[0]['sent'], rows[0]['dropped'], rows[1]['dropped']), (8,2,1))
        self.assertFalse(any('paused' in r or 'disabled' in r for r in rows))

    def test_legacy_unqualified_destination_uses_the_registry_canonical_name(self):
        agent = self.agent(); agent.bus.canonical['events'] = 'events@t'
        agent.bus.kv.set('sub.000000000001', {'id':'000000000001','target':'events',
                        'filter':'','created_by':'#caller@t'})
        agent.load()
        rows = json.loads(agent.handle(message('sub list')))['subs']
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(row['target'], 'events@t')
        agent.forward([agent.ring.add('file', 'arrival')])
        self.assertEqual(agent.bus.sent[0][0], 'events@t')

    def test_legacy_literal_filter_is_escaped_in_the_shared_grammar(self):
        agent = self.agent()
        agent.bus.kv.set('sub.000000000001', {'id':'000000000001','target':'#target@t',
                        'filter':'error.dot','created_by':'#caller@t'})
        agent.load()
        rows = json.loads(agent.handle(message('sub list')))['subs']
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(row['filter'], r'/error\.dot/iu')
        agent.forward([agent.ring.add('file', 'errorXdot'), agent.ring.add('file', 'ERROR.dot')])
        self.assertEqual([body for _, body, _ in agent.bus.sent], ['[file] ERROR.dot'])

    def test_non_hex_ids_are_deterministic_collision_checked_and_idempotent(self):
        agent = self.agent(); agent.clock = lambda:1000.
        key = 'sub.old-id'; legacy = {'id':'old-id','target':'#target@t','filter':'','created_by':'#caller@t'}
        collision = migration.migration_id(agent.bus.name, key)
        self.assertEqual(collision, migration.migration_id(agent.bus.name, key), 'legacy ID mapping is not deterministic')
        native = migration.project(agent.subscriptions, key, legacy, collision, agent.clock)
        agent.bus.kv.set('sub.'+collision, native)
        agent.bus.kv.set(key, legacy)
        agent.load()
        self.assertNotIn(key, agent.bus.kv.values)
        self.assertEqual(agent.bus.kv.get('sub.'+collision), native, 'migration overwrote/coalesced a real collision')
        expected = migration.migration_id(agent.bus.name, key, 1)
        rows = json.loads(agent.handle(message('sub list')))['subs']
        self.assertEqual({r['id'] for r in rows}, {collision, expected})
        self.assertFalse(any(k.startswith(migration.PREFIX) for k in agent.bus.kv.values))
        once = copy.deepcopy(agent.bus.kv.values); agent.clock = lambda:2000.; agent.load()
        self.assertEqual(agent.bus.kv.values, once)
        agent.forward([agent.ring.add('file', 'one match')])
        self.assertEqual(len(agent.bus.sent), 2, 'a real subscription was lost to an ID collision')

    def test_non_hex_unconfirmed_new_commit_keeps_old_until_restart_deduplicates(self):
        agent = self.agent(); agent.clock = lambda:1000.
        key = 'sub.short'; legacy = {'id':'short','target':'#target@t','filter':'','created_by':'#caller@t'}
        agent.bus.kv.set(key, legacy)
        new_key = 'sub.'+migration.migration_id(agent.bus.name, key)
        real_set = agent.bus.kv.set
        def lost_reply(name, value, **options):
            result = real_set(name, value, **options)
            if name == new_key: raise BusError('commit reply lost')
            return result
        agent.bus.kv.set = lost_reply
        try:
            with self.assertRaises(BusError): agent.load()
        finally: agent.bus.kv.set = real_set
        self.assertIn(key, agent.bus.kv.values, 'legacy row disappeared before a confirmed new commit')
        self.assertEqual(agent.bus.kv.get(key), legacy)
        self.assertIn(new_key, agent.bus.kv.values)
        first = agent.bus.kv.get(new_key)
        agent.clock = lambda:2000.; agent.load()
        self.assertNotIn(key, agent.bus.kv.values)
        self.assertEqual(agent.bus.kv.get(new_key), first, 'restart recreated the committed row')
        rows = json.loads(agent.handle(message('sub list')))['subs']
        self.assertEqual(len(rows), 1)
        agent.forward([agent.ring.add('file', 'after restart')])
        self.assertEqual(len(agent.bus.sent), 1)

    def test_non_hex_failed_new_write_does_not_delete_the_legacy_row(self):
        agent = self.agent()
        key = 'sub.short'; legacy = {'id':'short','target':'#target@t','filter':'','created_by':'#caller@t'}
        agent.bus.kv.set(key, legacy)
        new_key = 'sub.'+migration.migration_id(agent.bus.name, key)
        real_set = agent.bus.kv.set
        def failed_write(name, value, **options):
            if name == new_key: raise BusError('write failed')
            return real_set(name, value, **options)
        agent.bus.kv.set = failed_write
        try:
            with self.assertRaises(BusError): agent.load()
        finally: agent.bus.kv.set = real_set
        self.assertIn(key, agent.bus.kv.values, 'legacy row disappeared before a confirmed new commit')
        self.assertEqual(agent.bus.kv.get(key), legacy)
        self.assertNotIn(new_key, agent.bus.kv.values)
        agent.load()
        self.assertNotIn(key, agent.bus.kv.values)
        self.assertIn(new_key, agent.bus.kv.values)

    def test_non_hex_delete_reply_lost_finishes_checkpoint_without_recreating_old_row(self):
        agent = self.agent()
        key = 'sub.short'; legacy = {'id':'short','target':'#target@t','filter':'','created_by':'#caller@t'}
        agent.bus.kv.set(key, legacy)
        real_delete = agent.bus.kv.delete
        def lost_reply(name, **options):
            result = real_delete(name, **options)
            if name == key: raise BusError('delete reply lost')
            return result
        agent.bus.kv.delete = lost_reply
        try:
            with self.assertRaises(BusError): agent.load()
        finally: agent.bus.kv.delete = real_delete
        self.assertNotIn(key, agent.bus.kv.values)
        self.assertTrue(any(k.startswith(migration.PREFIX) for k in agent.bus.kv.values))
        agent.load()
        rows = json.loads(agent.handle(message('sub list')))['subs']
        self.assertEqual(len(rows), 1)
        self.assertFalse(any(k.startswith(migration.PREFIX) for k in agent.bus.kv.values))

    def test_non_hex_damaged_definition_stays_manager_only_repair(self):
        agent = self.agent()
        key = 'sub.orphan'
        agent.bus.kv.set(key, {'id':'orphan','target':'#target@t','filter':''})
        agent.load()
        new_id = migration.migration_id(agent.bus.name, key)
        self.assertNotIn(key, agent.bus.kv.values)
        self.assertEqual(json.loads(agent.handle(message('sub list')))['subs'], [])
        managers = json.loads(agent.handle(message('sub list', roles=('owner',))))['subs']
        self.assertEqual(managers, [{'id':new_id, 'reason':'invalid_state'}])
        self.assertEqual(agent.handle(message('sub delete '+new_id)), 'error: not_author')
        self.assertEqual(json.loads(agent.handle(message('sub delete '+new_id, roles=('owner',))))['deleted'], new_id)

    def test_sub_edit_is_refused_with_delete_and_add_guidance_without_mutation(self):
        agent = self.agent()
        agent.handle(message('sub add #target@t'))
        before = copy.deepcopy(agent.bus.kv.values)
        for body in ['sub edit old #other@t /error/', 'sub.edit old #other@t', '{"method":"sub.edit","id":"old","target":"#other@t"}']:
            self.assertEqual(agent.handle(message(body)), 'error: delete it and add it again')
            self.assertEqual(agent.bus.kv.values, before)
        self.assertNotIn('sub.edit', logwatch.FIELDS)
        self.assertIn('delete it and add it again', agent.handle(message('help')))

    def test_idle_subscription_does_not_launch_a_regex_worker(self):
        agent = self.agent(); agent.handle(message('sub add #target@t /error/i'))
        with patch.object(agent.subscriptions, '_helper', side_effect=AssertionError('idle subscription matched')):
            agent.forward([])
        with patch('filters.subprocess.run', side_effect=AssertionError('empty grep match started a worker')):
            self.assertEqual(match('/error/i', []), [])

    def test_parser_config_and_filters_refuse_unknowns_and_bound_regex(self):
        for body in ['tail --all','tail 0','since nope','sub edit x','{"method":"tail","roles":["owner"]}','{"method":"tail","n":true}']:
            with self.subTest(body=body),self.assertRaises(ValueError):parse(body)
        self.assertEqual(match('err',['ERROR','fine']),[True,False]);self.assertEqual(match('/^error/i',['ERROR','fine']),[True,False])
        with self.assertRaisesRegex(ValueError,'took too long'):match('/(a+)+$/',['a'*10000+'!'])
        for config in [{'ring_size':0},{'journal':['--evil']},{'sources':['relative']},{'unknown':1}]:
            with self.assertRaises(ValueError):configuration(config)


if __name__=='__main__':unittest.main()
