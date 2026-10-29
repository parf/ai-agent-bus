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
from logwatch import Agent, parse
from agent_bus.errors import BusError, Refusal, UnexpectedAnswer
from watcher import Monitor, Ring, configuration


class FakeKV:
    def __init__(self): self.values = {}
    def list(self): return {'json': [{'name': key} for key in self.values]}
    def get(self, key, **_): return self.values[key]
    def set(self, key, value, mode='set', **_):
        if mode == 'replace' and key not in self.values: raise Refusal(404, 'absent')
        self.values[key] = dict(value)
    def delete(self, key, **_): self.values.pop(key); return True


class FakeBus:
    name = '#logwatch@t'
    def __init__(self): self.kv = FakeKV(); self.sent = []; self.manages = True; self.may = True; self.asked = []
    def may_send(self, who, name):
        self.asked.append((who, name))
        if isinstance(self.may, Exception): raise self.may
        return self.may
    def send(self, target, body, **options): self.sent.append((target, body, options))
    def _request(self, *_args, **_kwargs): return {'manages': self.manages}


def message(body, roles=(), original_to=None):
    return SimpleNamespace(body=body, roles=roles, original_to=original_to, ask=None, **{'from':'#caller@t'})


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
        return Agent(FakeBus(), configuration({'sources': [], 'excludes': [], **overrides}), log=lambda _:None)

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

    def test_subscriptions_match_fold_persist_and_enforce_destination_authority(self):
        now=[1000.];agent=self.agent();agent.clock=lambda:now[0]
        caller=message('sub add #target@t /error/i')
        added=json.loads(agent.handle(caller));self.assertIn('id',added)
        lines=[agent.ring.add('file','ERROR repeated') for _ in range(37)] + [agent.ring.add('file','ordinary')]
        agent.forward(lines)
        self.assertEqual(len(agent.bus.sent),1);self.assertIn('ERROR repeated',agent.bus.sent[0][1])
        now[0]+=300;agent.forward([])
        self.assertEqual(len(agent.bus.sent),2);self.assertIn('×37 in 5 min',agent.bus.sent[1][1])
        restored=Agent(agent.bus,agent.config,log=lambda _:None);restored.load();self.assertIn(added['id'],restored.subs)
        agent.bus.manages=False
        self.assertIn('management of destination',agent.handle(message('sub add #other@t')))
        self.assertIn('management of destination',agent.handle(message('sub delete '+added['id'])))
        self.assertIn('management of destination',agent.handle(message('sub add #other@t',roles=('owner',),original_to='#forwarder@t')))
        self.assertIn('id',json.loads(agent.handle(message('sub add #other@t',roles=('maintainer',)))))
        self.assertIn('deleted',agent.handle(message('sub delete '+added['id'],roles=('owner',))))

    def test_author_may_send_is_checked_at_add_and_before_every_send_and_fails_closed(self):
        agent = self.agent()
        agent.bus.may = False
        self.assertIn('may not send', agent.handle(message('sub add #target@t /error/i')))
        self.assertEqual(agent.bus.kv.values, {})  # nothing stored
        agent.bus.may = Refusal(404, 'no such name')
        self.assertIn('no such destination', agent.handle(message('sub add #target@t /error/i')))
        for failure in (BusError('down'), UnexpectedAnswer('old daemon'), OSError('socket')):
            agent.bus.may = failure
            self.assertIn('error:', agent.handle(message('sub add #target@t /error/i')))
        self.assertEqual(agent.bus.kv.values, {})
        agent.bus.may = True
        added = json.loads(agent.handle(message('sub add #target@t /error/i')))
        self.assertEqual(agent.bus.asked[-1], ('#caller@t', '#target@t'))
        agent.forward([agent.ring.add('file', 'ERROR one')])
        self.assertEqual(len(agent.bus.sent), 1)
        # An unanswerable check sends nothing, and recovers when it answers.
        agent.bus.may = BusError('down')
        agent.forward([agent.ring.add('file', 'ERROR two')])
        self.assertEqual(len(agent.bus.sent), 1)
        self.assertNotIn('paused', agent.subs[added['id']])
        agent.bus.may = True
        agent.forward([agent.ring.add('file', 'ERROR three')])
        self.assertEqual(len(agent.bus.sent), 2)
        # The author's right revoked: the next send is the last, and the entry stops visibly.
        agent.bus.may = False
        agent.forward([agent.ring.add('file', 'ERROR four')])
        self.assertEqual(len(agent.bus.sent), 2)
        self.assertEqual(agent.subs[added['id']]['paused'], 'denied')
        self.assertIn('"paused": "denied"', agent.handle(message('sub list')))
        agent.bus.may = True
        agent.forward([agent.ring.add('file', 'ERROR five')])
        self.assertEqual(len(agent.bus.sent), 2)  # paused until deleted
        restored = Agent(agent.bus, agent.config, log=lambda _: None); restored.load()
        self.assertEqual(restored.subs[added['id']]['paused'], 'denied')
        # A subscription deleted by another thread meanwhile is not written back by its pause.
        gone = json.loads(agent.handle(message('sub add #target@t /error/i')))
        agent.bus.kv.values.pop('sub.' + gone['id'])
        agent.bus.may = False
        agent.forward([agent.ring.add('file', 'ERROR seven')])
        self.assertNotIn('sub.' + gone['id'], agent.bus.kv.values)
        agent.bus.may = True
        # A destination that is gone pauses it too.
        other = json.loads(agent.handle(message('sub add #target@t /error/i')))
        agent.bus.may = Refusal(404, 'no such name')
        agent.forward([agent.ring.add('file', 'ERROR six')])
        self.assertEqual(agent.subs[other['id']]['paused'], 'destination gone')

    def test_outage_asks_once_per_batch_and_a_subscription_without_an_author_is_paused(self):
        agent = self.agent()
        for n in range(6):
            agent.bus.kv.set('sub.s%d' % n, {'id': 's%d' % n, 'target': '#t@t', 'filter': 'ERROR', 'created_by': '#caller@t'})
        agent.bus.kv.set('sub.orphan', {'id': 'orphan', 'target': '#t@t', 'filter': 'ERROR'})
        agent.load()
        agent.bus.may = BusError('down')
        logged = []; agent.log = logged.append
        agent.forward([agent.ring.add('file', 'ERROR a'), agent.ring.add('file', 'ERROR b')])
        self.assertEqual(agent.bus.sent, [])
        self.assertEqual(len(agent.bus.asked), 1)  # one failing call for the whole batch
        self.assertEqual(agent.subs['orphan'].get('paused'), 'author unknown')
        self.assertEqual(len([l for l in logged if 'unavailable' in l]), 6)  # once per subscription, not per line
        agent.forward([agent.ring.add('file', 'ERROR c')])
        self.assertEqual(len([l for l in logged if 'unavailable' in l]), 6)  # not again while it stays down
        self.assertEqual(len(agent.bus.asked), 2)  # one more call: a new batch asks afresh
        agent.bus.may = UnexpectedAnswer('old')
        self.assertIn('does not answer may-send', agent.handle(message('sub add #target@t /error/i')))

    def test_idle_subscription_does_not_launch_a_regex_worker(self):
        agent = self.agent()
        agent.handle(message('sub add #target@t /error/i'))
        with patch('logwatch.match', side_effect=AssertionError('idle subscription matched')):
            agent.forward([])
        with patch('filters.subprocess.run', side_effect=AssertionError('empty match started a worker')):
            self.assertEqual(match('/error/i', []), [])

    def test_parser_config_and_filters_refuse_unknowns_and_bound_regex(self):
        for body in ['tail --all','tail 0','since nope','sub edit x','{"method":"tail","roles":["owner"]}','{"method":"tail","n":true}']:
            with self.subTest(body=body),self.assertRaises(ValueError):parse(body)
        self.assertEqual(match('err',['ERROR','fine']),[True,False]);self.assertEqual(match('/^error/i',['ERROR','fine']),[True,False])
        with self.assertRaisesRegex(ValueError,'took too long'):match('/(a+)+$/',['a'*10000+'!'])
        for config in [{'ring_size':0},{'journal':['--evil']},{'sources':['relative']},{'unknown':1}]:
            with self.assertRaises(ValueError):configuration(config)


if __name__=='__main__':unittest.main()
