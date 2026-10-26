import importlib.util
import json
import os
from pathlib import Path
import tempfile
import subprocess
import sys
import time
import unittest
from datetime import datetime, timezone, timedelta
from types import SimpleNamespace
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('debug_agent', HERE / 'debug.py')
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)

class Bus:
    name = '#debug@fixture'
    broken = False
    def call(self, name, body, timeout):
        answer = agent.answer(body, {'body':body})
        if self.broken and body == 'ping': answer = '{}'
        return SimpleNamespace(message=SimpleNamespace(body=answer))
    def _request(self, method, path, body=None, **kwargs):
        if path == '/ls':
            return [{'name':'#'+template+'@fixture', 'kind':'agent', 'tags':['systemd'] if template=='systemd' else []}
                    for template in ['server-info','systemd','git','logwatch','telegram']]
        if path == '/kv': return {'value':'roundtrip'}
        return {}

AUTH = {"roles":["owner"], "to":Bus.name}

class Doctor(unittest.TestCase):
    def test_doctor_requires_direct_delivered_management(self):
        bus = Bus()
        bus.call = lambda *a, **kw: self.fail('unauthorized debug relay')
        bus._request = lambda *a, **kw: self.fail('unauthorized debug bus operation')
        for body in ('doctor','doctor full','doctor disks'):
            for envelope in ({}, {'roles':['operate'],'to':bus.name},
                             {'roles':['owner'],'to':bus.name,'original_to':'#other@fixture'},
                             {'roles':['owner'],'to':'#other@fixture'}):
                with self.subTest(body=body,envelope=envelope):
                    self.assertIn('requires a delivered Owner or Maintainer', agent.answer(body,envelope,bus))
        for role in ('owner','maintainer'):
            for original in (None,bus.name):
                with patch.object(agent,'doctor',return_value='allowed'), patch.object(agent,'selftest',return_value='allowed'):
                    envelope={'roles':[role],'to':bus.name,'original_to':original}
                    self.assertEqual(agent.answer('doctor',envelope,bus),'allowed')
                    self.assertEqual(agent.answer('doctor full',envelope,bus),'allowed')
        with patch.dict(os.environ, {'AB_ROLE_OWNER':'1'}):
            self.assertIn('requires a delivered',agent.answer('doctor',{'to':bus.name},bus))

    def test_caps_and_deadline(self):
        for body in ('sleep 31','sleep nan','sleep inf','sleep -1','big 65','big 1.5','fail 256','fail 1.2'):
            self.assertTrue(agent.answer(body).startswith('error:'), body)
        self.assertEqual(len(agent.answer('big 64')), 65536)
        self.assertTrue(agent.answer('hello ' + 'x'*16384).startswith('error:'))
        until = (datetime.now(timezone.utc)+timedelta(seconds=.05)).isoformat()
        start = time.monotonic()
        self.assertIn('deadline', agent.answer('sleep 1', {'deadline':until}))
        self.assertLess(time.monotonic()-start,.5)
    def test_private_env_and_delivered_roles(self):
        with patch.dict(os.environ, {'AGENT_BUS_TOKEN':'credential-sentinel','SECRET':'secret-sentinel','AB_ROLE_ADMIN':'1'}, clear=True):
            result = agent.answer('env', {'roles':['reader'],'from':'alice'})
        self.assertNotIn('credential-sentinel',result)
        self.assertNotIn('secret-sentinel',result)
        self.assertNotIn('AB_ROLE_ADMIN',result)
        self.assertIn('AGENT_BUS_TOKEN=***',result)
        self.assertIn('AB_ROLE_READER=1',result)
        self.assertEqual(json.loads(agent.answer('roles', {'roles':['reader']}))['roles'],['reader'])
    def test_selftest_detects_failure(self):
        bus = Bus()
        self.assertEqual(agent.answer('selftest', {'roles':[], 'to':bus.name}, bus=bus).count('✅'),6)
        self.assertIn('Checks: 6', agent.answer('selftest', {'roles':[], 'to':bus.name}, bus=bus))
        original = agent.answer
        def broken(body, *args, **kwargs):
            return '{}' if body == 'ping' else original(body, *args, **kwargs)
        with patch.object(agent,'answer',side_effect=broken):
            result = original('selftest', AUTH, bus=bus)
        self.assertIn('❌ ping',result)
        self.assertNotIn('✅ ping',result)
    def test_simple_methods_and_crash(self):
        self.assertEqual(json.loads(agent.answer('echo',{'body':'echo','from':'alice'}))['from'],'alice')
        self.assertIn('answered_at',json.loads(agent.answer('ping')))
        self.assertIn('uid',json.loads(agent.answer('sandbox')))
        self.assertEqual(agent.answer('fail 7'),'error: debug failure 7')
        self.assertIn('мир 👋\n',agent.answer('unicode'))
        result = subprocess.run([sys.executable,str(HERE/'debug.py'),'crash'],capture_output=True,timeout=5)
        # real SIGSEGV creates host core dumps
        # self.assertEqual(result.returncode,-11)
        self.assertEqual(result.returncode,-9)
    def test_doctor_keeps_acl_refusals(self):
        from agent_bus.errors import Refusal
        bus = Bus()
        bus.call = lambda *args, **kwargs: (_ for _ in ()).throw(Refusal(403,'private detail'))
        result = agent.answer('doctor', AUTH, bus=bus)
        self.assertEqual(result.count('not allowed:'),5)
        self.assertNotIn('private detail',result)

    def test_doctor_tls_expiry_and_pin(self):
        for days, same in [(45, True), (30, True), (29.9, True), (-1, True), (45, False)]:
            with self.subTest(days=days, same=same):
                facts = {'days_left':days, 'expires_at':'2030-01-01T00:00:00+00:00',
                         'pin':'sha256:'+'a'*64, 'token_pin':'sha256:'+('a' if same else 'b')*64}
                bus = Bus()
                calls = []
                def call(name, body, **kwargs):
                    calls.append((name, body))
                    result = {'tls':[{'status':0,'data':facts},{'status':0}]} if body == '{"method":"tls"}' else {'version':agent.VERSION}
                    return SimpleNamespace(message=SimpleNamespace(body=json.dumps(result)))
                bus.call = call
                text = agent.answer('doctor', AUTH, bus=bus)
                self.assertIn(('✅' if days >= 30 else '❌')+' TLS certificate:', text)
                self.assertIn(('✅' if same else '❌')+' TLS pin matches', text)
                self.assertIn(('#server-info@fixture', '{"method":"tls"}'), calls)

    def test_doctor_discovers_local_templates_without_name_tags(self):
        bus = Bus()
        calls = []
        def request(method, path, body=None, **kwargs):
            self.assertEqual(path, '/ls')
            self.assertEqual(kwargs['query']['kind'], 'agent')
            self.assertEqual(kwargs['query'], {'kind':'agent'})
            return [{'name':name,'kind':kind,'tags':tags} for name,kind,tags in [
                ('#telegram/alice@fixture','agent',['messaging']),
                ('#telegram/bob@fixture','agent',[]),
                ('#telegram/alice@elsewhere','agent',['telegram']),
                ('#unrelated@fixture','agent',['telegram','messaging']),
                ('ordinary@fixture','resource',['telegram']),
                ('#telegram-fake@fixture','agent',['telegram'])]]
        def call(name, body, **kwargs):
            calls.append((name, body))
            return SimpleNamespace(message=SimpleNamespace(body=json.dumps({'version':agent.VERSION})))
        bus._request = request
        bus.call = call
        report = agent.answer('doctor', AUTH, bus=bus)
        self.assertEqual([name for name,body in calls if body=='whoami'],['#telegram/alice@fixture','#telegram/bob@fixture'])
        self.assertIn('systemd: none installed',report)
        self.assertNotIn('#telegram@fixture',report)

    def test_shared_tag_vectors(self):
        path = Path(__file__).resolve().parents[2] / 'internal/protocol/testdata/tags.json'
        for case in json.loads(path.read_text()):
            with self.subTest(case=case['input']):
                if case['invalid']:
                    with self.assertRaisesRegex(ValueError, 'invalid tag'):
                        agent.normalize_tags(case['input'])
                else:
                    self.assertEqual(agent.normalize_tags(case['input']), case['expected'])

    def test_doctor_keeps_systemd_shared_capability_tag(self):
        bus = Bus()
        calls = []
        bus._request = lambda *a, **kw: [
            {'name':'#units/custom@fixture','kind':'agent','tags':['systemd','ops']},
            {'name':'#units/foreign@elsewhere','kind':'agent','tags':['systemd']},
            {'name':'#systemd-untagged@fixture','kind':'agent','tags':['ops']},
        ]
        bus.call = lambda name, body, **kw: (calls.append((name,body)) or SimpleNamespace(message=SimpleNamespace(body='{}')))
        agent.answer('doctor', AUTH, bus=bus)
        self.assertIn(('#units/custom@fixture','{"method":"failed"}'),calls)
        self.assertNotIn(('#units/foreign@elsewhere','{"method":"failed"}'),calls)
        self.assertNotIn(('#systemd-untagged@fixture','{"method":"failed"}'),calls)

    def summary_bus(self, percent=38, failed=25):
        bus = Bus()
        def call(name, body, **kwargs):
            if kwargs.get('ask') == 'help-json': value = {'version':agent.VERSION}
            elif body in ('disks', '{"method":"disks"}'):
                value = {'disks':[{'command':['lsblk'], 'status':0, 'stdout':'RAW-DISK-SENTINEL'},
                                  {'command':['df','-h'], 'status':0, 'stdout':f'Filesystem Size Used Avail Use% Mounted on\n/dev/root 280G 105G 175G {percent}% /\n'}]}
            elif body in ('failed', '{"method":"failed"}'):
                value = {'status':0, 'stdout':'RAW-UNIT-SENTINEL', 'units':[
                    {'unit':f'drkonqi-coredump-processor@{i}.service'} for i in range(failed)]}
            elif body == '{"method":"tls"}':
                value = {'tls':[{'status':0, 'data':{'days_left':45, 'expires_at':'2030-01-01', 'pin':'same', 'token_pin':'same'}}]}
            else: return SimpleNamespace(message=SimpleNamespace(body='(no lines)' if 'logwatch' in name else 'reachable'))
            return SimpleNamespace(message=SimpleNamespace(body=json.dumps(value)))
        bus.call = call
        return bus

    def test_doctor_disk_thresholds(self):
        for percent, mark in [(38,'✅'), (84,'✅'), (85,'⚠'), (94,'⚠'), (95,'❌'), (100,'❌')]:
            with self.subTest(percent=percent):
                report = agent.answer('doctor', AUTH, bus=self.summary_bus(percent=percent, failed=0))
                self.assertIn(f'{mark} disk / {percent}% used (175 GiB free)', report)
                self.assertNotIn('RAW-DISK-SENTINEL',report)

    def test_doctor_groups_failed_units_and_keeps_summary_short(self):
        report = agent.answer('doctor', AUTH, bus=self.summary_bus())
        self.assertIn('⚠ 25 failed units: drkonqi-coredump-processor ×25 (grouped by template)',report)
        self.assertIn('✅ versions agree ('+agent.VERSION+')',report)
        self.assertNotIn('RAW-UNIT-SENTINEL',report)
        self.assertNotIn('"status"',report)
        self.assertLess(len(report),2000)
        clean = agent.answer('doctor', AUTH, bus=self.summary_bus(failed=0))
        self.assertIn('✅ no failed units',clean)
        single = agent.answer('doctor', AUTH, bus=self.summary_bus(failed=1))
        self.assertIn('⚠ 1 failed unit:',single)

    def test_doctor_facts_and_output_stay_bounded(self):
        with self.assertRaises(ValueError): agent.response_json(' ' * (256 * 1024 + 1))
        with self.assertRaises(ValueError): agent.failed_summary(json.dumps({'status':0,'units':[{}]*1001}))
        value = {'disks':[{'command':['df','-h'],'status':0,'stdout':'Filesystem Size Used Avail Use% Mounted on\n/dev/root 280G 105G 175G 38% /\n/dev/data 20G 19G 1G 95% /var\n'}]}
        self.assertIn('❌ disk /var 95% used',agent.disk_summary(json.dumps(value)))
        bus = self.summary_bus()
        original = bus.call
        def call(name,body,**kwargs):
            if body == 'disks': return SimpleNamespace(message=SimpleNamespace(body='x'*5000+'TAIL-SENTINEL'))
            return original(name,body,**kwargs)
        bus.call = call
        raw = agent.answer('doctor disks', AUTH, bus=bus)
        self.assertIn('[output truncated]',raw)
        self.assertNotIn('TAIL-SENTINEL',raw)
        self.assertLess(len(raw),4200)

    def test_doctor_raw_is_explicit_and_scoped(self):
        full = agent.answer('doctor full', AUTH, bus=self.summary_bus())
        self.assertIn('RAW-DISK-SENTINEL',full)
        self.assertIn('RAW-UNIT-SENTINEL',full)
        disks = agent.answer('doctor disks', AUTH, bus=self.summary_bus())
        self.assertIn('RAW-DISK-SENTINEL',disks)
        self.assertNotIn('RAW-UNIT-SENTINEL',disks)
        self.assertNotIn('TLS',disks)
        self.assertNotIn('versions',disks)
        for bad in ['doctor other','doctor full extra']:
            self.assertTrue(agent.answer(bad,AUTH,bus=self.summary_bus()).startswith('error:'),bad)

    def test_package_without_checkout(self):
        # Package has everything imported by both the handler and server.
        scratch = HERE.parents[2] / 'tmp'
        scratch.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=scratch) as directory:
            dest = Path(directory)/'debug'
            subprocess.run([str(HERE/'package.sh'),str(dest)],check=True)
            result = subprocess.run([sys.executable,str(dest/'debug.py'),'help'],capture_output=True,text=True,check=True,env={'PATH':os.environ['PATH']})
            self.assertIn('selftest',result.stdout)
            self.assertIn('may-send',result.stdout)
