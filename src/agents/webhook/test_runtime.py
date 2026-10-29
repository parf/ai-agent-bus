"""Exported-package and shared real-client concurrency, against a disposable bus."""
from concurrent.futures import ThreadPoolExecutor
import contextlib
import http.client
import io
import json
import os
from pathlib import Path
import select
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from types import SimpleNamespace
from unittest.mock import patch

HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
sys.path[:0]=[str(HERE),str(ROOT/'src/clients/python'),str(ROOT/'src/agents/shared')]
from agent_bus import Bus,Refusal
from webhook import Agent
from test_webhook import RAW,SECRET,AUTH,FakeBus
from test_outgoing import Fake


class Runtime(unittest.TestCase):
    def test_all_asks_and_offline_export_do_no_normal_work(self):
        with tempfile.TemporaryDirectory() as directory:
            dest=Path(directory)/'webhook'
            subprocess.run([str(HERE/'package.sh'),str(dest)],check=True,capture_output=True)
            env={'PATH':os.environ['PATH'],'AGENT_BUS_ADDR':'http://127.0.0.1:1','AGENT_BUS_NAME':'#fixture@invalid','AGENT_BUS_TOKEN':'fixture-only','WEBHOOK_LISTEN':'invalid'}
            for ask in ('help','help-json','help-config','help-secret','mcp','status','unrecognized'):
                r=subprocess.run([str(dest/'webhook.py')],env={**env,'AGENT_BUS_ASK':ask},input='post out must-not-run',capture_output=True,text=True,timeout=2,cwd=directory)
                self.assertEqual(r.returncode,0,r.stderr)
                if ask=='help-json':
                    v=json.loads(r.stdout);self.assertEqual(set(v['methods']),{'post','routes'});self.assertEqual(v['config']['(root)']['type'],'list')
                if ask=='help-config':self.assertIn('root list',r.stdout)
                if ask=='unrecognized':self.assertEqual(r.stdout.strip(),'unsupported: unrecognized')
                if ask=='mcp':self.assertEqual(json.loads(r.stdout),{'tools':[]})
            manifest=json.loads((dest/'agent.json').read_text());self.assertEqual(manifest['serves'],'itself')
            self.assertEqual(manifest['processes'],1);self.assertTrue(manifest['needs']['network'])
            self.assertTrue((dest/'support/agents/shared/envfile.py').is_file())
            for name in ('LICENSE.md','NOTICE.md','THIRD-PARTY-NOTICES.md','support/clients/LICENSE.md'):self.assertTrue((dest/name).is_file())
    def test_asks_do_not_spend_outbound_slot_and_unsolicited_replies_stop(self):
        from rules import configuration
        bus=FakeBus();agent=Agent(bus,[{'name':'out','url':'http://127.0.0.1:1/fixed'}])
        self.assertIsNone(agent.problem)
        agent.outbound.acquire()
        try:
            message=SimpleNamespace(ask='help-json',body='post out private-body',envelope={},roles=[])
            self.assertEqual(set(json.loads(agent.handle(message))['methods']),{'post','routes'})
            message.ask='future';self.assertEqual(agent.handle(message),'unsupported: future')
            message.ask=None;self.assertEqual(agent.handle(message),'error: outbound call busy')
            message.envelope={'re':'forwarded-message'};self.assertIsNone(agent.handle(message))
        finally:agent.outbound.release();agent.close()
    def test_real_bus_concurrent_incoming_outbound_and_help_use_own_identity(self):
        fixture=Path(os.environ.get('WEBHOOK_BUS_FIXTURE',ROOT/'tmp/webhook-client-fixture'))
        self.assertTrue(fixture.is_file(),'build clients/fixture for real-client proof')
        process=subprocess.Popen([str(fixture)],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
        buses=[];stop=threading.Event();thread=None;agent=None;fake=Fake();fake.delay=.7
        log=io.StringIO()
        with contextlib.redirect_stderr(log):
            try:
                ready,_,_=select.select([process.stdout],[],[],5);self.assertTrue(ready)
                info=json.loads(process.stdout.readline())
                def bus(name):
                    b=Bus(name,addr=info['addr'],token=info['tokens'][name],timeout=3);buses.append(b);return b
                owner=bus('owner@fixture');caller=bus('#caller@fixture');worker=bus('#worker@fixture')
                owner.record('jobs@fixture').manage(allow=[worker.name])
                with self.assertRaises(Refusal):caller.send('jobs@fixture','must not forward as caller')
                cfg=[dict(RAW[0],target='jobs@fixture'),{'name':'out','url':f'http://127.0.0.1:{fake.server_port}/fixed'}]
                agent=Agent(worker,cfg,SECRET,'127.0.0.1:0');self.assertIsNone(agent.problem)
                errors=[]
                def serve():
                    try:worker.serve(agent.handle,workers=2,stop=stop,drain_timeout=3)
                    except Exception as e:errors.append(type(e).__name__)
                thread=threading.Thread(target=serve);thread.start()
                with ThreadPoolExecutor(max_workers=10) as pool:
                    future=pool.submit(caller.call,worker.name,'post out external body',timeout=3)
                    end=time.monotonic()+2
                    while not fake.calls and time.monotonic()<end:time.sleep(.005)
                    self.assertTrue(fake.calls,'outbound helper reached only local fake')
                    help=caller.call(worker.name,'',ask='help-json',timeout=1)
                    self.assertEqual(set(json.loads(help.message.body)['methods']),{'post','routes'})
                    def incoming(i):
                        c=http.client.HTTPConnection('127.0.0.1',agent.listener.server_port,timeout=3)
                        try:
                            c.request('POST','/hook','event-'+str(i),{'Authorization':AUTH});r=c.getresponse();r.read();return r.status
                        finally:c.close()
                    results=list(pool.map(incoming,range(8)))
                    self.assertTrue(all(x in (202,503) for x in results),results)
                    count=results.count(202);self.assertGreater(count,0)
                    messages=[owner.consume(inbox='jobs@fixture',wait=.1) for _ in range(count)]
                    self.assertTrue(all(m.envelope['from']==worker.name and m.body.startswith('event-') for m in messages))
                    self.assertEqual(len({m.id for m in messages}),count)
                    record=owner.record('jobs@fixture').lookup();self.assertEqual(record.get('in'),count)
                    self.assertEqual(json.loads(future.result().message.body)['status'],200)
                    self.assertEqual(len(fake.calls),1)
                self.assertEqual(errors,[])
            finally:
                stop.set()
                if thread:thread.join(4)
                if agent:agent.close()
                for b in buses:b.close()
                fake.close();process.stdin.close();process.wait(timeout=5)
                process.stdout.close();process.stderr.close()
                if thread:self.assertFalse(thread.is_alive())
        self.assertNotIn('fixture-password',log.getvalue())


if __name__=='__main__':unittest.main()
