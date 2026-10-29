"""Bounded child protocol, offline exported metadata and no real network calls."""
import contextlib
import datetime
import io
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
sys.path[:0]=[str(HERE),str(ROOT/'src/agents/shared')]
import fetch
import worker
import test_transport as fixtures
from extract import WARNING,OUTPUT_MAX
from policy import Refused
from transport import ContentError


class Runtime(unittest.TestCase):
    def test_strict_text_and_json_parser(self):
        for raw in ('https://example.org/','get https://example.org/','{"method":"get","url":"https://example.org/"}'):
            self.assertEqual(fetch.parse(raw),{'url':'https://example.org/','format':'text'})
        self.assertEqual(fetch.parse('{"method":"get","url":"https://example.org/","format":"markdown"}')['format'],'markdown')
        for raw in ('{"method":"get","url":"x","url":"y"}','{"method":"get","url":"x","format":"html"}','{"method":"post","url":"x"}','{"method":"get","url":"x","headers":{}}','{"method":"get","url":true}','{"method":"get","url":"x","format":NaN}','get '+'x'*4097):
            with self.subTest(raw=raw[:70]),self.assertRaises(ValueError):fetch.parse(raw)
    def test_uniform_failures_do_not_echo_urls_or_operator_details(self):
        results=[]
        for reason in ('dns','address','connection','timeout'):
            with patch.object(worker,'get',side_effect=Refused(reason)):
                result=worker.perform({'url':'http://private-sensitive.example/secret-query','format':'text','timeout':1})
            self.assertFalse(result['ok']);results.append(result['body']);self.assertNotIn('sensitive',json.dumps(result))
        self.assertEqual(set(results),{fetch.FAILURE})
        with patch.object(worker,'get',side_effect=ContentError('HTTP 404')):
            self.assertEqual(worker.perform({'url':'x','format':'text','timeout':1})['body'],'error: HTTP 404')
    def test_worker_extraction_always_carries_warning_in_both_formats(self):
        with patch.object(worker,'get',return_value=(b'<p>hello</p>','text/html','https://example.org/')):
            for format in ('text','markdown'):
                result=worker.perform({'url':'x','format':format,'timeout':1})
                self.assertTrue(result['ok']);self.assertEqual(result['body'],WARNING+'hello')
    def test_real_child_refuses_literal_private_without_network(self):
        result=fetch.run({'url':'http://127.0.0.1/','format':'text'},1)
        self.assertFalse(result['ok']);self.assertEqual(result['body'],fetch.FAILURE);self.assertEqual(result['class'],'address')
    def test_expired_deadline_and_config_refusal_do_not_launch_child(self):
        calls=[]
        def execute(*args):calls.append(args);self.fail('refused request launched child')
        with contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(fetch.answer({'body':'http://example.org','deadline':'2000-01-01T00:00:00Z'},{},execute=execute),fetch.FAILURE)
            self.assertEqual(fetch.answer({'body':'http://example.org'},{'allow_private':True},execute=execute),'error: Configuration accepts only a valid allow list')
        self.assertEqual(calls,[])
    def test_helper_environment_and_argv_do_not_contain_identity_or_credentials(self):
        code='import json,os,sys;print(json.dumps({"ok":True,"body":'+repr(WARNING)+'+json.dumps({"env":dict(os.environ),"argv":sys.argv}),"class":"ok","bytes":0}))'
        with patch.dict(os.environ,{'AGENT_BUS_TOKEN':'fixture-private-token','AGENT_BUS_NAME':'#private','AB_ROLE_OWNER':'1','OTHER_SECRET':'fixture-private-token'}):
            result=fetch.run({'url':'http://example.org/','format':'text'},1,helper=[sys.executable,'-c',code])
        value=json.loads(result['body'][len(WARNING):]);self.assertFalse(any(k.startswith(('AGENT_BUS_','AB_ROLE_')) for k in value['env']))
        self.assertNotIn('OTHER_SECRET',value['env']);self.assertNotIn('fixture-private-token',json.dumps(value));self.assertNotIn('http://example.org/',json.dumps(value['argv']))
    def test_stuck_dns_has_a_hard_parent_deadline(self):
        (ROOT/'tmp').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=ROOT/'tmp',prefix='fetch-dns-') as d:
            ready=Path(d)/'ready'
            code=f'import sys,socket,time,pathlib;sys.path.insert(0,{str(HERE)!r});socket.getaddrinfo=lambda *a,**k:(pathlib.Path({str(ready)!r}).write_text("ready"),time.sleep(30))[1];import worker;worker.main()'
            start=time.monotonic()
            with self.assertRaises(TimeoutError):fetch.run({'url':'http://public.example/','format':'text'},1,helper=[sys.executable,'-B','-c',code])
            self.assertTrue(ready.exists(),'DNS fake actually entered, rather than a child that never started')
            self.assertLess(time.monotonic()-start,1.7)
    def test_header_and_body_trickle_have_one_hard_parent_deadline(self):
        fixture=fixtures.Transport();fixture.setUp()
        try:
            for response in ([b'H']+[b'T']*50,[b'HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 100\r\n\r\n']+[b'x']*100):
                fixture.server.raw=response;fixture.server.calls.clear()
                code=f'import sys,socket;sys.path.insert(0,{str(HERE)!r});import transport,worker;from test_policy import answers;worker.get=lambda raw,budget:transport.get(raw,budget=budget,resolver=lambda *a,**k:answers("8.8.8.8"),connector=lambda address,port,timeout:socket.create_connection(("127.0.0.1",{fixture.server.server_port}),timeout));worker.main()'
                start=time.monotonic()
                with self.assertRaises(TimeoutError):fetch.run({'url':'http://public.example/','format':'text'},.5,helper=[sys.executable,'-B','-c',code])
                self.assertLess(time.monotonic()-start,1.2);self.assertEqual(len(fixture.server.calls),1)
        finally:fixture.tearDown()
    def test_helper_pipe_output_cap_is_not_unbounded_communicate(self):
        code='import sys,time;sys.stdout.write("x"*'+str(fetch.PIPE_MAX+65536)+');sys.stdout.flush();time.sleep(5)'
        start=time.monotonic()
        try:fetch.run({'url':'http://example.org/','format':'text'},2,helper=[sys.executable,'-c',code])
        except (ValueError,TimeoutError):pass
        else:self.fail('oversized helper output was accepted')
        self.assertLess(time.monotonic()-start,1)
    def test_deadline_kills_the_ready_descendant_before_reaping(self):
        (ROOT/'tmp').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=ROOT/'tmp',prefix='fetch-child-') as d:
            ready=Path(d)/'ready';survived=Path(d)/'survived'
            child=f'import pathlib,time;pathlib.Path({str(ready)!r}).write_text("ready");time.sleep(1.3);pathlib.Path({str(survived)!r}).write_text("bad")'
            code=f'import subprocess,sys,time;subprocess.Popen([sys.executable,"-c",{child!r}]);time.sleep(30)'
            with self.assertRaises(TimeoutError):fetch.run({'url':'http://example.org/','format':'text'},1,helper=[sys.executable,'-c',code])
            self.assertTrue(ready.exists());time.sleep(1.4);self.assertFalse(survived.exists())
    def test_helper_success_schema_and_output_warning_cannot_be_skipped(self):
        for value in ({'ok':True,'body':'missing warning','class':'ok','bytes':0},{'ok':True,'body':WARNING+'x'*OUTPUT_MAX,'class':'ok','bytes':0},{'ok':True,'body':WARNING,'class':'injected private URL','bytes':0}):
            code='import json;v='+repr({**value,'body':''})+';v["body"]='+ (repr(WARNING)+'+"x"*'+str(OUTPUT_MAX) if len(value['body'])>OUTPUT_MAX else repr(value['body']))+';print(json.dumps(v))'
            with self.subTest(value={k:v for k,v in value.items() if k!='body'}),self.assertRaises(ValueError):fetch.run({'url':'http://example.org/','format':'text'},1,helper=[sys.executable,'-c',code])
    def test_exported_offline_asks_do_not_launch_network_or_read_config(self):
        (ROOT/'tmp').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=ROOT/'tmp',prefix='fetch-package-') as d:
            dest=Path(d)/'fetch';subprocess.run([str(HERE/'package.sh'),str(dest)],check=True,capture_output=True)
            env={'PATH':os.environ['PATH'],'AGENT_BUS_CONFIG':'invalid config','AGENT_BUS_TOKEN':'fixture-private-token'}
            for ask in ('help','help-json','help-config','help-secret','mcp','status','future'):
                process=subprocess.run([str(dest/'fetch.py')],input='get http://127.0.0.1',env={**env,'AGENT_BUS_ASK':ask},capture_output=True,text=True,timeout=2,cwd=d)
                self.assertEqual(process.returncode,0,process.stderr);self.assertEqual(process.stderr,'')
                if ask in ('status','future'):self.assertEqual(process.stdout.strip(),'unsupported: '+ask)
                if ask=='help-json':
                    self.assertTrue(process.stdout.startswith('{'),process.stdout)
                    self.assertEqual(set(json.loads(process.stdout)['methods']),{'get','curl'})
                if ask=='mcp':self.assertEqual(json.loads(process.stdout),{'tools':[]})
                self.assertNotIn('fixture-private-token',process.stdout)
            # Envelope asks take precedence over invalid Configuration/body too.
            request=json.dumps({'ask':'future','body':'get http://127.0.0.1'})
            process=subprocess.run([str(dest/'fetch.py')],input=request,env=env,capture_output=True,text=True,timeout=2)
            self.assertEqual(process.stdout.strip(),'unsupported: future');self.assertEqual(process.stderr,'')


if __name__=='__main__':unittest.main()
