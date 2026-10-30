"""Real bus fixture and exported package; no provider HTTP and no live data."""
import concurrent.futures
import contextlib
import io
import json
import os
from pathlib import Path
import select
import socket
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from types import SimpleNamespace

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
sys.path[:0]=[str(HERE),str(ROOT/'src/clients/python'),str(ROOT/'src/agents/shared')]
from agent_bus import Bus,Refusal,CallDone,CallReply
from blocks import Blocks,Budget
from rag import Agent
from rules import dumps,RagError
from test_rag import Store,PROMPT
from test_rag import CFG


class Runtime(unittest.TestCase):
    def test_asks_bypass_core_and_secrets_and_exported_package(self):
        with tempfile.TemporaryDirectory() as directory:
            package=Path(directory)/'rag'
            subprocess.run([str(HERE/'package.sh'),str(package)],check=True,capture_output=True)
            for ask in ['help','help-json','help-config','help-secret','mcp','status','unknown-ask']:
                p=subprocess.run([str(package/'rag.py')],env={'PATH':os.environ['PATH'],'AGENT_BUS_ASK':ask,'AGENT_BUS_ADDR':'http://127.0.0.1:1','AGENT_BUS_NAME':'#fixture@invalid','AGENT_BUS_TOKEN':'fixture-only'},
                    input='invalid normal work',text=True,capture_output=True,timeout=2,cwd=directory)
                self.assertEqual(p.returncode,0,p.stderr)
                if ask=='help-json':
                    value=json.loads(p.stdout);self.assertEqual(value['version'],json.loads((HERE/'agent.json').read_text())['version']);self.assertIn('upsert',value['methods'])
                if ask=='unknown-ask':self.assertEqual(p.stdout.strip(),'unsupported: unknown-ask')
            agent=Agent(None,None)
            for ask in ['help','help-json','status','not-supported']:
                answer=agent.handle(SimpleNamespace(ask=ask,roles=[],body='normal-work-sentinel-7a68',envelope={}))
                self.assertNotIn('normal-work-sentinel-7a68',answer)
                if ask=='help-json':self.assertTrue(answer.startswith('{'),'help-json must be JSON')
                if ask=='help':self.assertIn('upsert',answer)
                if ask=='not-supported':self.assertEqual(answer,'unsupported: not-supported')
            self.assertEqual(json.loads((package/'agent.json').read_text())['version'],json.loads((HERE/'agent.json').read_text())['version'])
            self.assertTrue((package/'support/clients/python/agent_bus/client.py').is_file())
            self.assertTrue((package/'LICENSE.md').is_file())
            self.assertTrue((package/'licenses/NOTICE.md').is_file())
            self.assertTrue((package/'support/clients/LICENSE.md').is_file())
    def test_logs_never_include_source_or_invalid_method_text(self):
        from core import Core
        from test_rag import KV
        kv=KV();agent=Agent(None,CFG);agent.core=Core(CFG,Store(),kv)
        agent.core.cache.set('team','notes',PROMPT)
        log=io.StringIO();marker='private-source-sentinel-xyz'
        with contextlib.redirect_stderr(log):
            answer=agent.handle(SimpleNamespace(ask=None,roles=[],envelope={},body=dumps({'method':'upsert','namespace':'team','tableName':'notes','id':'a','data':marker})))
            refused=agent.handle(SimpleNamespace(ask=None,roles=[],envelope={},body=dumps({'method':marker})))
        self.assertIn(marker,answer);self.assertTrue(refused.startswith('error:'))
        self.assertNotIn(marker,log.getvalue())
        self.assertTrue(all(set(json.loads(line))=={'method','status'} for line in log.getvalue().splitlines()))

    def test_terminal_done_is_not_a_provider_answer(self):
        class DoneBus:
            def call(self,*args,**kwargs):return CallDone(SimpleNamespace(body='{"status":"ok","result":{}}'))
        backend=Blocks(DoneBus(),CFG)
        with self.assertRaisesRegex(RagError,'did not answer'):
            backend.qdrant('GET','')

    def test_only_definite_first_write_refusals_claim_not_stored(self):
        from core import Core
        from rules import Unknown,parse
        from test_rag import KV
        for text,unknown in [('error: 499 proxy closed',True),('error: 429 retry later',True),
                             ('error: 408 timeout',True),('error: 409 conflict',True),
                             ('error: 422 invalid request',False),('error: requires write role',False)]:
            with self.subTest(reply=text):
                class ReplyBus:
                    def call(self,*args,**kwargs):return CallReply(SimpleNamespace(body=text))
                store=Store();kv=KV();app=Core(CFG,store,kv);app.cache.set('team','notes',PROMPT)
                original=store.qdrant;proxy=Blocks(ReplyBus(),CFG)
                def call(verb,path,body=None,write=False):
                    return proxy.qdrant(verb,path,body,write=write) if write else original(verb,path,body)
                store.qdrant=call
                with self.assertRaises(RagError) as error:
                    app.run(parse(dumps({'method':'upsert','namespace':'team','tableName':'notes','id':'a','data':'Tea'})))
                self.assertEqual(isinstance(error.exception,Unknown),unknown)
                if not unknown:self.assertIn('not stored; prior record unchanged',str(error.exception))
                if 'requires' in text:self.assertIn('grant missing',str(error.exception))
                self.assertEqual(store.points,{})

    def test_real_bus_uses_app_identity_and_grant_not_original_caller(self):
        fixture=Path(os.environ.get('RAG_BUS_FIXTURE',ROOT/'tmp/rag-client-fixture'))
        self.assertTrue(fixture.is_file(),'build clients/fixture for the isolated bus check')
        process=subprocess.Popen([str(fixture)],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
        buses=[];servers=[]
        try:
            ready,_,_=select.select([process.stdout],[],[],5)
            self.assertTrue(ready,'bus fixture ready')
            info=json.loads(process.stdout.readline())
            def bus(name):
                b=Bus(name,addr=info['addr'],token=info['tokens'][name],timeout=2);buses.append(b);return b
            owner=bus('owner@fixture');caller=bus('#caller@fixture');app=bus('#worker@fixture');helper=bus('#helper@fixture')
            owner.record(helper.name).manage(allow=['#worker@fixture(write)'],roles=['read','write','admin'])
            seen=[];handler_errors=[]
            def serve(b,handler):
                stop=threading.Event();future=concurrent.futures.Future()
                def target():
                    try:b.serve(handler,stop=stop,workers=1,drain_timeout=2);future.set_result(None)
                    except BaseException as error:future.set_exception(error)
                thread=threading.Thread(target=target,daemon=True);thread.start();servers.append((stop,thread,future))
                end=time.monotonic()+2
                while not b._serving and time.monotonic()<end:time.sleep(.005)
                self.assertIsNotNone(b._serving)
            def block(message):
                seen.append((message.envelope['from'],tuple(message.roles),json.loads(message.body)))
                return '{"status":"ok","result":{"status":"completed"}}'
            serve(helper,block)
            backend=Blocks(app,CFG|{'qdrant':helper.name});backend.begin(Budget())
            def forward(message):
                # An application request's roles/envelope never become call options.
                try:backend.qdrant('PUT','/points?wait=true',{'points':[]},write=True)
                except Exception as error:
                    handler_errors.append(type(error).__name__);return 'handler failure'
                return 'completed'
            serve(app,forward)
            self.assertEqual(caller.call(app.name,'test original caller',timeout=3).message.body,'completed',str(handler_errors))
            self.assertEqual(len(seen),1)
            self.assertEqual(seen[0][0],app.name)
            self.assertEqual(seen[0][1],('write',))
            self.assertEqual(seen[0][2],{'method':'request','verb':'PUT','path':'/collections/notes/points?wait=true','body':'{"points":[]}'})
            with self.assertRaises(Refusal):caller.call(helper.name,'must not reach downstream',timeout=1)
            self.assertEqual(len(seen),1)
            # Replace the helper handler without changing its actual bus identity/grant.
            store=Store()
            def fake_blocks(message):
                seen.append((message.envelope['from'],tuple(message.roles),json.loads(message.body)))
                request=json.loads(message.body)
                if request['method']=='decide':return dumps(store.decide(json.loads(request['body'])))
                if request['path']=='/v1/embeddings':return dumps(store.embed(json.loads(request['body'])))
                body=json.loads(request['body']) if 'body' in request else None
                relative=request['path'].removeprefix('/collections/notes')
                write=request['verb'] in ('PUT','PATCH','DELETE') or '?wait=true' in relative
                result=store.qdrant(request['verb'],relative,body,write=write)
                return dumps({'status':'ok','result':result})
            helper._serving.handler=fake_blocks
            actual=Agent(app,CFG|{'qdrant':helper.name,'voyage':helper.name,'ai':helper.name})
            app._serving.handler=actual.handle
            def call(value):return json.loads(caller.call(app.name,dumps(value),timeout=3).message.body)
            self.assertEqual(call({'method':'prompts','action':'set','namespace':'team','tableName':'notes','data':PROMPT}),{'status':'ok'})
            record=call({'method':'upsert','namespace':'team','tableName':'notes','id':'tea','data':'Tea'})
            self.assertEqual(record['classification_ids'],{'drink':.8})
            self.assertEqual(call({'method':'get','namespace':'team','tableName':'notes','id':'tea'}),record)
            self.assertEqual(len(call({'method':'list','namespace':'team'})['records']),1)
            self.assertEqual(call({'method':'delete','namespace':'team','tableName':'notes','id':'tea'}),{'status':'ok'})
            self.assertTrue(all(item[0]==app.name and item[1]==('write',) for item in seen))
            self.assertEqual(store.points,{})
        finally:
            for stop,_,_ in servers:stop.set()
            for _,thread,_ in servers:thread.join(3)
            for b in buses:b.close()
            process.stdin.close();process.wait(timeout=5)
            process.stdout.close();process.stderr.close()
            for _,thread,future in servers:
                self.assertFalse(thread.is_alive(),'bus worker leaked')
                if future.done():future.result()
