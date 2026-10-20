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
        if path == '/kv': return {'value':'roundtrip'}
        return {}

class Doctor(unittest.TestCase):
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
        self.assertEqual(agent.answer('selftest', bus=bus).count('✅'),6)
        original = agent.answer
        def broken(body, *args, **kwargs):
            return '{}' if body == 'ping' else original(body, *args, **kwargs)
        with patch.object(agent,'answer',side_effect=broken):
            result = original('selftest', bus=bus)
        self.assertIn('❌ ping',result)
        self.assertNotIn('✅ ping',result)
    def test_simple_methods_and_crash(self):
        self.assertEqual(json.loads(agent.answer('echo',{'body':'echo','from':'alice'}))['from'],'alice')
        self.assertIn('answered_at',json.loads(agent.answer('ping')))
        self.assertIn('uid',json.loads(agent.answer('sandbox')))
        self.assertEqual(agent.answer('fail 7'),'error: debug failure 7')
        self.assertIn('мир 👋\n',agent.answer('unicode'))
        result = subprocess.run([sys.executable,str(HERE/'debug.py'),'crash'],capture_output=True,timeout=5)
        self.assertEqual(result.returncode,-11)
    def test_doctor_keeps_acl_refusals(self):
        from agent_bus.errors import Refusal
        bus = Bus()
        bus.call = lambda *args, **kwargs: (_ for _ in ()).throw(Refusal(403,'private detail'))
        result = agent.answer('doctor',bus=bus)
        self.assertEqual(result.count('not allowed:'),4)
        self.assertNotIn('private detail',result)
        self.assertIn('unavailable: TLS',result)

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
                text = agent.answer('doctor', bus=bus)
                self.assertIn(('✅' if days >= 30 else '❌')+' TLS certificate:', text)
                self.assertIn(('✅' if same else '❌')+' TLS pin matches', text)
                self.assertIn(('#server-info@fixture', '{"method":"tls"}'), calls)

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
