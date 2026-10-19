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
from watcher import Monitor, Ring, configuration


class FakeKV:
    def __init__(self): self.values = {}
    def list(self): return {'json': [{'name': key} for key in self.values]}
    def get(self, key, **_): return self.values[key]
    def set(self, key, value, **_): self.values[key] = dict(value)
    def delete(self, key, **_): self.values.pop(key); return True


class FakeBus:
    name = '#logwatch@t'
    def __init__(self): self.kv = FakeKV(); self.sent = []; self.manages = True
    def send(self, target, body, **options): self.sent.append((target, body, options))
    def _request(self, *_args, **_kwargs): return {'manages': self.manages}


def message(body, roles=(), original_to=None):
    return SimpleNamespace(body=body, roles=roles, original_to=original_to, ask=None, **{'from':'#caller@t'})


class WatcherTests(unittest.TestCase):
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
                            if ring.snapshot() or watch.journal_error:break
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

    def test_parser_config_and_filters_refuse_unknowns_and_bound_regex(self):
        for body in ['tail --all','tail 0','since nope','sub edit x','{"method":"tail","roles":["owner"]}','{"method":"tail","n":true}']:
            with self.subTest(body=body),self.assertRaises(ValueError):parse(body)
        self.assertEqual(match('err',['ERROR','fine']),[True,False]);self.assertEqual(match('/^error/i',['ERROR','fine']),[True,False])
        with self.assertRaisesRegex(ValueError,'took too long'):match('/(a+)+$/',['a'*10000+'!'])
        for config in [{'ring_size':0},{'journal':['--evil']},{'sources':['relative']},{'unknown':1}]:
            with self.assertRaises(ValueError):configuration(config)


if __name__=='__main__':unittest.main()
