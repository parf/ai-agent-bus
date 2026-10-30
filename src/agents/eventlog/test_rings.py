"""Deterministic ring and persistence acceptance; no daemon or wall-clock waits."""
import copy
import json
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(HERE.parent/'shared'))
import event_rings as r


class Rings(unittest.TestCase):
    def setUp(self):
        (ROOT/'tmp').mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=ROOT/'tmp')
        self.now = 1793318400.0
        self.directory = Path(self.temp.name)
        self.rings = r.Rings(self.directory, clock=lambda: self.now)
    def tearDown(self):
        self.rings.close();self.temp.cleanup()
    def restart(self, cfg=None):
        self.rings.close();self.rings = r.Rings(self.directory, cfg, clock=lambda: self.now)
    def test_sizes_ttl_and_severity_are_independent(self):
        self.rings.write('deploys','alert','failure')
        for i in range(201): self.rings.write('deploys','notice',str(i))
        self.assertEqual(len(self.rings.logs['deploys']['rings']['notice']),200)
        for i in range(300): self.rings.write('deploys','warning',str(i))
        result=self.rings.read('deploys')
        self.assertEqual(result['counts'],{'alert':1,'warning':200,'notice':200})
        self.assertEqual([e['text'] for e in result['rings']['notice']],list(map(str,range(200,0,-1))))
        self.now += 86400
        self.assertEqual(self.rings.read('deploys')['counts'],{'alert':1,'warning':200,'notice':0})
        self.now += 6*86400
        self.assertEqual(self.rings.read('deploys')['counts'],{'alert':1,'warning':0,'notice':0})
        self.now += 23*86400
        self.assertEqual(self.rings.read('deploys')['counts'],{'alert':0,'warning':0,'notice':0})
        self.restart();self.assertEqual(self.rings.read('deploys')['counts']['alert'],0)
    def test_fold_uses_first_instant_and_latest_live_matching_text(self):
        first=self.rings.write('x','alert','A');self.rings.write('x','alert','B')
        self.now+=59
        again=self.rings.write('x','alert','A')
        self.assertEqual((again['id'],again['count'],again['at']),(first['id'],2,first['at']))
        self.assertEqual(again['last_at'],r.timestamp(self.now))
        self.now+=1
        later=self.rings.write('x','alert','A')
        self.assertGreater(later['id'],first['id']);self.assertEqual(later['count'],1)
        self.now-=1
        backwards=self.rings.write('x','alert','A')
        self.assertGreater(backwards['id'],later['id'])
        self.restart()
        self.assertGreater(self.rings.write('x','alert','B')['id'],backwards['id'])
    def test_single_line_sanitization_precedes_scalar_truncation(self):
        text='\x1b[31m'+'😀'*201+'\x1b[0m\r\n\x00\x85\u202e\u2028'
        entry=self.rings.write('x','notice',text)
        self.assertEqual(entry['text'],'😀'*200)
        self.assertEqual(r.line('a\x1b]0;title\x07b\n\u200dc'),'abc')
    def test_shared_sequence_restart_gap_and_alert_fold_survive(self):
        ids=[self.rings.write('x',severity,severity)['id'] for severity in ('notice','warning','alert')]
        self.assertEqual(ids,[1,2,3]);self.restart()
        self.assertEqual(self.rings.read('x')['counts'],{'alert':1,'warning':0,'notice':0})
        self.assertEqual(self.rings.write('x','alert','alert')['count'],2)
        next_entry=self.rings.write('x','notice','next')
        self.assertEqual(next_entry['id'],65)
        self.assertEqual(stat.S_IMODE((self.directory/'events.json').stat().st_mode),0o600)
    def test_pull_oldest_once_closes_fold_and_persists_alert_removal(self):
        for text in ('a','b','c','d','e'): self.rings.write('x','notice',text)
        pulled=self.rings.pull('x','notice',2)
        self.assertEqual([e['id'] for e in pulled['entries']],[1,2]);self.assertEqual(pulled['remaining'],3)
        self.assertEqual([e['id'] for e in self.rings.read('x')['rings']['notice']],[5,4,3])
        self.assertEqual([e['id'] for e in self.rings.pull('x','notice',2)['entries']],[3,4])
        self.assertEqual(self.rings.write('x','notice','a')['count'],1)
        self.rings.write('x','alert','persisted');self.rings.pull('x','alert')
        self.restart();self.assertEqual(self.rings.read('x')['counts']['alert'],0)
    def test_partial_clean_id_boundary_and_full_clean_tombstone(self):
        for i in range(1,10): self.rings.write('x','alert' if i in (7,9) else 'warning',str(i))
        self.assertEqual(self.rings.clean('x','alert',7)['removed'],1)
        result=self.rings.read('x');self.assertEqual([e['id'] for e in result['rings']['alert']],[9]);self.assertEqual(result['counts']['warning'],7)
        self.assertEqual(self.rings.clean('x','warning')['removed'],7)
        self.restart({'max_logs':1})
        self.assertEqual([e['id'] for e in self.rings.read('x')['rings']['alert']],[9])
        self.rings.clean('x')
        self.assertEqual(self.rings.list()['logs'],[])
        self.rings.write('other','notice','slot freed');self.rings.clean('other')
        self.assertGreater(self.rings.write('x','notice','reborn')['id'],9)
        self.restart();self.assertGreater(self.rings.write('x','warning','again')['id'],65)
    def test_invalid_names_log_and_lifetime_caps_preserve_existing(self):
        for value in ('','/bad','two words','a'*65,'é'):
            with self.assertRaises(r.RingError):self.rings.write(value,'notice','x')
        self.restart({'max_logs':1});first=self.rings.write('x','notice','a')
        with self.assertRaisesRegex(r.RingError,'log_limit'): self.rings.write('y','notice','b')
        self.assertEqual(self.rings.read('x')['rings']['notice'],[first])
        self.rings.clean('x')
        with patch.object(r,'MAX_NAMES',1):
            with self.assertRaisesRegex(r.RingError,'name_limit'):self.rings.write('new','notice','c')
            self.assertGreater(self.rings.write('x','notice','d')['id'],first['id'])
    def test_corrupt_oversize_and_symlink_state_fail_closed(self):
        self.rings.write('x','alert','secret');self.rings.close()
        path=self.directory/'events.json';original=path.read_bytes()
        for raw in (b'{broken',b'{"v":1,"v":1,"logs":{}}',b'{"v":true,"logs":{}}',b'{"v":1,"logs":{"x":{"active":true,"high":64,"alerts":[{}]}}}'):
            path.write_bytes(raw)
            with self.assertRaisesRegex(r.RingError,'storage_invalid'):r.Rings(self.directory)
            self.assertEqual(path.read_bytes(),raw)
        path.write_bytes(original)
        with patch.object(r,'MAX_STATE',10):
            with self.assertRaisesRegex(r.RingError,'storage_invalid'):r.Rings(self.directory)
        (self.directory/'foreign').write_bytes(original)
        path.unlink();path.symlink_to(self.directory/'foreign')
        with self.assertRaisesRegex(r.RingError,'storage_invalid'):r.Rings(self.directory)
    def test_atomic_failures_poison_until_reopen_and_never_reuse_ids(self):
        self.rings.write('x','alert','one')
        for where in ('fsync','replace'):
            with patch.object(r.os,where,side_effect=OSError('planted-secret')):
                with self.assertRaisesRegex(r.RingError,'storage_unknown'):self.rings.write('x','alert','two')
            with self.assertRaisesRegex(r.RingError,'storage_unknown'):self.rings.write('x','notice','unsafe')
            self.restart()
            self.assertEqual([e['text'] for e in self.rings.read('x')['rings']['alert']],['one'])
        real_fsync=r.os.fsync;calls=0
        def fsync(fd):
            nonlocal calls
            calls+=1
            if calls==2:raise OSError('directory outcome unknown')
            return real_fsync(fd)
        with patch.object(r.os,'fsync',side_effect=fsync):
            with self.assertRaisesRegex(r.RingError,'storage_unknown'):self.rings.write('x','alert','committed')
        self.restart()
        rows=self.rings.read('x')['rings']['alert'];self.assertEqual([e['text'] for e in rows],['committed','one'])
        self.assertGreater(self.rings.write('x','notice','next')['id'],max(e['id'] for e in rows))
        self.assertEqual(list(self.directory.glob('.events-*')),[])
    def test_lifetime_lock_refuses_overlapping_generation(self):
        with self.assertRaisesRegex(r.RingError,'storage_locked'):r.Rings(self.directory,lock_timeout=.03)
        self.rings.close();self.rings=r.Rings(self.directory)
        self.assertEqual(self.rings.write('x','notice','one')['id'],1)
    def test_configuration_overrides_bounds_and_strict_types(self):
        for cfg in ([],{'oops':1},{'max_logs':True},{'max_logs':101},{'rings':{'alert':{'size':501}}},{'rings':{'alert':{'ttl':'0s'}}},{'rings':{'notice':{'size':500}}},{'logs':{'bad/name':{}}}):
            with self.assertRaises(r.RingError,msg=repr(cfg)):r.config(cfg)
        self.restart({'rings':{'notice':{'size':2,'ttl':'2s'}},'logs':{'x':{'notice':{'size':1,'ttl':'1s'}}}})
        self.rings.write('x','notice','a');self.rings.write('x','notice','b');self.rings.write('y','notice','c')
        self.assertEqual(self.rings.read('x')['counts']['notice'],1)
        self.now+=1
        self.assertEqual(self.rings.read('x')['counts']['notice'],0);self.assertEqual(self.rings.read('y')['counts']['notice'],1)
    def test_json_ring_vectors(self):
        vectors=json.loads((ROOT/'src/testdata/event-rings.json').read_text())
        self.assertEqual(vectors['v'],1)
        for case in vectors['cases']:
            with self.subTest(case=case['name']):
                self.rings.close()
                path=self.directory/'events.json'
                if path.exists():path.unlink()
                self.now=case['now'];self.rings=r.Rings(self.directory,case.get('config',{}),clock=lambda:self.now)
                for step in case['steps']:
                    method=step['op']
                    if method=='advance':self.now+=step['seconds'];continue
                    if method=='restart':self.restart(case.get('config',{}));continue
                    try:
                        actual=getattr(self.rings,method)(*step.get('args',[]))
                    except r.RingError as error:actual={'error':error.code}
                    for path,wanted in step['expect'].items():
                        value=actual
                        for key in path.split('.'):
                            value=value[int(key)] if isinstance(value,list) else value[key]
                        self.assertEqual(value,wanted,(case['name'],step,path))
