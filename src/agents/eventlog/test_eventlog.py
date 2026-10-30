"""Eventlog access, grammar, subscriptions and exported runtime acceptance."""
import json
import os
from pathlib import Path
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
sys.path.insert(0,str(HERE.parent/'shared'))
import subscriptions


class PureSupport(unittest.TestCase):
    def test_manager_identity_is_case_insensitive_but_not_forwarded(self):
        for original in (None,'','#worker@fixture','#WORKER@FIXTURE'):
            with self.subTest(original=original):
                message=SimpleNamespace(envelope={'original_to':original,'roles':['maintainer']})
                self.assertTrue(subscriptions.own_manager(message,'#worker@fixture'))
        for original in ('jobs@fixture','#worker2@fixture',False,12):
            message=SimpleNamespace(envelope={'original_to':original,'roles':['owner']})
            self.assertFalse(subscriptions.own_manager(message,'#worker@fixture'))

import importlib.util
import select
import subprocess
import tempfile
import time
from datetime import datetime, timedelta, timezone
sys.path.insert(0,str(HERE))
import eventlog as e
from agent_bus import Bus

spec=importlib.util.spec_from_file_location('shared_subscription_fixture',HERE.parent/'shared/test_subscriptions.py')
shared=importlib.util.module_from_spec(spec);spec.loader.exec_module(shared)


def envelope(body='',roles=('write',),**values):
    return SimpleNamespace(body=body,ask=values.pop('ask',None),envelope={'from':'#caller@fixture','roles':list(roles),**values})


class Pure(unittest.TestCase):
    def setUp(self):
        (ROOT/'tmp').mkdir(exist_ok=True)
        self.temp=tempfile.TemporaryDirectory(dir=ROOT/'tmp')
        self.warnings=[];self.bus=shared.Bus('#worker@fixture')
        self.agent=e.Agent(self.bus,{},self.temp.name,warning=self.warnings.append)
    def tearDown(self):self.agent.close();self.temp.cleanup()
    def ask(self,body,roles=('write',),**values):return json.loads(self.agent.handle(envelope(body,roles,**values)))
    def test_source_kv_budget_shares_deadline_and_restores_client(self):
        from unittest.mock import Mock
        request=Mock(return_value='ok')
        client=SimpleNamespace(timeout=2,_request=request)
        with patch.object(e.time,'monotonic',side_effect=[100.5,102]):
            with e.bus_budget(client,101):
                self.assertEqual(client._request('GET','/kv'),'ok')
                self.assertEqual(request.call_args.kwargs['timeout'],.5)
                with self.assertRaises(e.BusTimeout):client._request('GET','/kv')
        self.assertIs(client._request,request);self.assertEqual(request.call_count,1)

    def test_text_json_grammar_and_strict_bounds(self):
        cases=[('write deploys alert trailing  ',{'method':'write','log':'deploys','level':'alert','text':'trailing  '}),
               ('pull x 2',{'method':'pull','log':'x','count':2}),('clean x alert 7',{'method':'clean','log':'x','level':'alert','id':7}),
               ('sub add jobs@fixture alert',{'method':'sub.add','target':'jobs@fixture','log':'alert'}),
               ('sub add jobs@fixture x alert /a b/i',{'method':'sub.add','target':'jobs@fixture','log':'x','level':'alert','filter':'/a b/i'}),
               ('{"method":"sub.add","target":"jobs@fixture","level":"alert"}',{'method':'sub.add','target':'jobs@fixture','level':'alert'})]
        for body,want in cases:self.assertEqual(e.parse(body),want)
        for body in ('{"method":"read"}','[]','{"method":"list","method":"list"}','{"method":"list","extra":"unknown"}',
                     '{"method":"pull","log":"x","count":true}','{"method":"pull","log":"x","count":NaN}',
                     '{"method":"write","log":"x","level":"alert","text":"\\u0000"}',
                     '{"method":"write","log":"x","level":"alert","text":"\\ud800"}',
                     'pull x 0','pull x 501','clean x 0','read x alert 2','pull --sub','write bad/name notice x',
                     'write x nonsense x','write x notice '+'a'*16384,'sub add jobs@fixture x alert extra','list\x00',
                     '{"method":"clean","log":"x","id":1.0}'):
            with self.subTest(body=body[:70]):
                with self.assertRaises(e.rings.RingError):e.parse(body)
    def test_direct_roles_status_and_metadata_ignore_ambient_and_forwarded_roles(self):
        self.ask('write x notice one')
        with patch.dict(os.environ,{'AB_ROLE_OWNER':'1'}):
            for body in ('read x','list','write x alert no','pull x','clean x','sub list'):
                self.assertEqual(self.ask(body,roles=[]).get('error',{}).get('code'),'forbidden')
            for original in ('jobs@fixture','#other@fixture',False):
                self.assertEqual(self.ask('read x',roles=['owner'],original_to=original).get('error',{}).get('code'),'forbidden')
            self.assertEqual(self.ask('',roles=[],ask='status').get('error',{}).get('code'),'forbidden')
            meta=json.loads(self.agent.handle(envelope(roles=[],ask='help-json')))
            self.assertEqual(set(meta['methods']),{'help'});self.assertEqual(meta['hidden_methods']['count'],9)
        self.assertEqual(self.ask('read x',roles=['read'],original_to='#WORKER@FIXTURE')['counts']['notice'],1)
        self.assertEqual(self.agent.handle(envelope(roles=['read'],ask='status')),'eventlog: 1 active logs')
    def test_help_json_exposes_reader_subscriptions_and_status_with_distinct_examples(self):
        value=json.loads(self.agent.handle(envelope(roles=['read'],ask='help-json')))
        self.assertTrue({'sub.add','sub.list','sub.delete','status'} <= set(value['methods']))
        self.assertNotIn('write',value['methods'])
        examples=[m['example'] for name,m in value['methods'].items()]
        self.assertEqual(len(examples),len(set(examples)))
        self.assertEqual(self.agent.handle(envelope('status',roles=['read'])),'eventlog: 0 active logs')

    def test_read_only_refuses_destructive_calls_without_removal(self):
        self.ask('write x alert retained')
        for body in ('write x notice forbidden','pull x alert 1','clean x'):
            self.assertEqual(self.ask(body,roles=['read']).get('error',{}).get('code'),'forbidden')
        self.assertEqual(self.ask('read x',roles=['read'])['counts'],{'alert':1,'warning':0,'notice':0})
    def test_deadline_preflight_is_before_storage_but_no_candidates_have_no_floor(self):
        soon=(datetime.now(timezone.utc)+timedelta(seconds=1)).isoformat()
        self.assertIn('entry',self.ask('write x notice no-candidates',deadline=soon))
        added=self.ask('sub add jobs@fixture x',roles=['read']);self.assertIn('id',added)
        before=self.ask('read x')['counts']
        self.assertEqual(self.ask('write x alert withheld',deadline=soon).get('error',{}).get('code'),'deadline')
        self.assertEqual(self.ask('read x')['counts'],before)
        self.assertIn('entry',self.ask('write other alert no-selector-match',deadline=soon))
        past=(datetime.now(timezone.utc)-timedelta(seconds=1)).isoformat()
        self.assertEqual(self.ask('write z notice expired',deadline=past).get('error',{}).get('code'),'deadline')
        self.assertEqual(self.ask('read z').get('error',{}).get('code'),'not_found')
        expiring=(datetime.now(timezone.utc)+timedelta(seconds=.01)).isoformat()
        with patch.object(self.agent.subs,'has_candidates',side_effect=lambda _:time.sleep(.03) or False):
            self.assertEqual(self.ask('write late notice expired-in-preflight',deadline=expiring).get('error',{}).get('code'),'deadline')
        self.assertEqual(self.ask('read late').get('error',{}).get('code'),'not_found')
    def test_storage_uncertainty_never_forwards_or_exposes_failure_details(self):
        self.ask('sub add jobs@fixture x',roles=['read'])
        calls=[];self.agent.subs.delivery=lambda *args:calls.append(args) or {}
        with patch.object(e.rings.os,'replace',side_effect=OSError('planted-secret body SQL password')):
            answer=self.ask('write x alert private-entry')
        self.assertEqual(answer.get('error',{}).get('code'),'storage_unknown');self.assertEqual(calls,[])
        self.assertEqual(self.warnings,['storage_unknown'])
        self.assertNotIn('private-entry',json.dumps(answer));self.assertNotIn('planted-secret',str(answer))
        self.assertEqual(self.ask('write x notice later').get('error',{}).get('code'),'storage_unknown')
    def test_reply_bound_counts_newest_order_and_json_shapes(self):
        result=self.ask('write x alert one');self.assertEqual(set(result),{'entry','deliveries'})
        self.assertEqual(set(result['entry']),{'id','level','text','at','last_at','count'})
        self.ask('write x warning two')
        result=self.ask('read x alert');self.assertEqual(list(result['rings']),['alert'])
        self.assertEqual(result['counts'],{'alert':1,'warning':1,'notice':0})
        self.assertEqual(set(self.ask('list')),{'logs','now'})
        self.assertEqual(self.ask('pull x 1')['entries'][0]['id'],1)
        self.assertEqual(self.ask('clean x')['counts'],{'alert':0,'warning':0,'notice':0})
        try:payload=e.dumps({'entries':[{'text':'😀'*200} for _ in range(500)]})
        except e.rings.RingError:self.fail('valid maximum reply rejected')
        self.assertLess(len(payload.encode()),e.MAX_REPLY)


class PurePackage(unittest.TestCase):
    def test_exported_helpers_config_and_offline_metadata_need_no_checkout_or_credentials(self):
        with tempfile.TemporaryDirectory(dir=ROOT/'tmp') as directory:
            root=Path(directory);package=root/'package'
            result=subprocess.run([str(HERE/'package.sh'),str(package)],capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
            for name in ('metadata.py','event_rings.py','subscriptions.py','subscription_filters.py'):
                copied=package/'support/agents/shared'/name
                self.assertTrue(copied.is_file(),name+' missing runtime artifact')
                self.assertEqual(copied.read_bytes(),(HERE.parent/'shared'/name).read_bytes())
            self.assertEqual(json.loads((package/'config.json.dist').read_text()),{'rings':{},'logs':{},'max_logs':100})
            env={key:value for key,value in os.environ.items() if not key.startswith(('AGENT_BUS_','AB_ROLE_','PYTHON'))}
            env.update(AGENT_BUS_ASK='help-json',AGENT_BUS_CONFIG='planted-secret',AGENT_BUS_TOKEN='planted-secret',AB_ROLE_OWNER='1')
            result=subprocess.run([sys.executable,'-I',str(package/'eventlog.py')],cwd=root,env=env,capture_output=True,text=True,timeout=5)
            self.assertEqual(result.returncode,0,result.stderr);self.assertEqual(result.stderr,'')
            value=json.loads(result.stdout)
            self.assertEqual(set(value['methods']),{'help'});self.assertEqual(value['hidden_methods']['count'],9)
            self.assertEqual(value['secrets'],[]);self.assertEqual(value['body_formats'],['text','json'])
            self.assertNotIn('planted-secret',result.stdout);self.assertFalse((root/'events.json').exists())
            self.assertEqual(json.loads((package/'agent.json').read_text())['serves'],'itself')


class Daemon(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.fixture=Path(os.environ.get('EVENTLOG_BUS_FIXTURE',ROOT/'tmp/eventlog-client-fixture'))
        if not cls.fixture.is_file():raise AssertionError('build the isolated clients/fixture binary')
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(dir=ROOT/'tmp');self.warnings=[]
        self.addCleanup(self.temp.cleanup)
        self.process=subprocess.Popen([str(self.fixture)],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,text=True)
        self.addCleanup(self.stop_fixture)
        self.assertTrue(select.select([self.process.stdout],[],[],5)[0],'fixture did not start')
        self.info=json.loads(self.process.stdout.readline());self.clients=[]
        self.owner,self.reader,self.worker,self.target,self.stranger=[self.client(name) for name in ('owner@fixture','#caller@fixture','#worker@fixture','#helper@fixture','#outsider@fixture')]
        self.owner.record(self.worker.name).manage(roles=['read','write'],allow=[self.reader.name+'(read)',self.target.name+'(write)',self.stranger.name])
        self.owner._request('POST','/configure',{'name':self.worker.name,'config':{}})
        self.agent=e.Agent(self.worker,{},self.temp.name,warning=self.warnings.append)
        self.addCleanup(self.agent.close)
    def client(self,name):
        result=Bus(name,addr=self.info['addr'],token=self.info['tokens'][name],timeout=2);self.clients.append(result);self.addCleanup(result.close);return result
    def stop_fixture(self):
        self.process.stdin.close();self.process.wait(timeout=5);self.process.stdout.close()
    def ask(self,body,caller=None,**options):
        (caller or self.owner).send(self.worker.name,body,**options)
        message=self.worker.consume(wait=1);self.assertIsNotNone(message)
        self.last=message
        return json.loads(self.agent.handle(message))
    def inbox(self,bus):
        result=[]
        while True:
            message=bus.consume(wait=.001)
            if message is None:return result
            result.append(message)
    def test_authenticated_reader_roles_and_foreign_subscription_delete(self):
        first=self.ask('sub add '+self.target.name+' deploys alert')
        self.assertIn('id',first)
        forbidden=self.ask('sub delete '+first['id'],self.reader)
        self.assertIn('read',self.last.roles);self.assertNotIn('owner',self.last.roles)
        self.assertEqual(forbidden.get('error',{}).get('code'),'not_author')
        mine=self.ask('sub add '+self.target.name+' deploys alert',self.reader);self.assertIn('id',mine)
        self.assertEqual([row['id'] for row in self.ask('sub list',self.reader)['subscriptions']],[mine['id']])
        self.assertEqual(self.ask('sub delete '+mine['id'],self.reader),{'deleted':True})
        self.assertEqual([row['id'] for row in self.ask('sub list')['subscriptions']],[first['id']])
        self.assertEqual(self.ask('sub delete '+first['id']),{'deleted':True})
    def test_subscriptions_arrive_before_pull_once_per_fold_and_selector(self):
        self.ask('sub add '+self.target.name+' deploys alert',self.reader)
        self.ask('sub add jobs@fixture deploys alert',self.reader)
        for _ in range(2):
            result=self.ask('write deploys alert failed');self.assertEqual(result['deliveries'],{'matched':2,'sent':2,'dropped':0})
        self.ask('write deploys notice routine');self.ask('write other alert elsewhere')
        pulled=self.ask('pull deploys alert 1');self.assertEqual(pulled['entries'][0]['count'],2)
        for recipient in (self.target,self.client('owner@fixture')):
            # The owner can consume a queue explicitly, without acting as it.
            messages=self.inbox(recipient) if recipient is self.target else []
            if recipient is not self.target:
                while True:
                    message=recipient.consume(inbox='jobs@fixture',wait=.001)
                    if message is None:break
                    messages.append(message)
            self.assertEqual(len(messages),2)
            payloads=[json.loads(message.body) for message in messages]
            self.assertEqual([p['count'] for p in payloads],[1,2])
            self.assertEqual([p['id'] for p in payloads],[pulled['entries'][0]['id']]*2)
            self.assertTrue(all(message.envelope['from']==self.worker.name for message in messages))
            self.assertTrue(all(message.topic=='deploys' and message.tag=='alert' for message in messages))
        self.assertEqual(self.warnings,[])
    def test_full_target_drops_warns_once_then_next_delivers_and_revocation_blocks(self):
        self.owner.record(self.target.name).manage(bound=1,overflow='strict',allow=[self.reader.name,self.worker.name])
        sub=self.ask('sub add '+self.target.name+' deploys alert',self.reader);self.assertIn('id',sub)
        self.owner.send(self.target.name,'fill')
        result=self.ask('write deploys alert failed');self.assertEqual(result['deliveries'],{'matched':1,'sent':0,'dropped':1})
        self.assertEqual(self.warnings,['send_refused']);self.assertEqual([m.body for m in self.inbox(self.target)],['fill'])
        result=self.ask('write deploys alert recovered');self.assertEqual(result['deliveries'],{'matched':1,'sent':1,'dropped':0})
        self.assertEqual([json.loads(m.body)['text'] for m in self.inbox(self.target)],['recovered'])
        self.owner.record(self.target.name).manage(allow=[self.worker.name])
        result=self.ask('write deploys alert revoked');self.assertEqual(result['deliveries']['dropped'],1)
        self.assertEqual(self.inbox(self.target),[]);self.assertEqual(self.warnings,['send_refused','admission_refused'])
        self.owner.record(self.target.name).manage(allow=[self.reader.name,self.worker.name])
        self.owner.record(self.worker.name).manage(allow=[self.target.name+'(write)',self.stranger.name])
        result=self.ask('write deploys alert source-revoked');self.assertEqual(result['deliveries']['dropped'],1)
        self.assertEqual(self.inbox(self.target),[])
        rows=self.ask('sub list')['subscriptions'];self.assertEqual((rows[0]['sent'],rows[0]['dropped']),(1,3))
        self.assertEqual(len(self.ask('pull deploys')['entries']),4)
    def test_real_forwarded_owner_and_bare_acl_cannot_read_or_destroy(self):
        self.ask('write deploys alert retained')
        for body in ('read deploys','pull deploys','clean deploys','write deploys notice no'):
            self.assertEqual(self.ask(body,self.reader).get('error',{}).get('code'),'forbidden') if body!='read deploys' else self.assertIn('rings',self.ask(body,self.reader))
        self.assertEqual(self.ask('read deploys',self.stranger).get('error',{}).get('code'),'forbidden')
        self.owner.record(self.worker.name).manage(allow=[self.reader.name+'(read)',self.target.name+'(write)',self.stranger.name,'jobs@fixture'])
        self.owner.record('jobs@fixture').manage(subs=[self.worker.name])
        self.owner.send('jobs@fixture','clean deploys')
        message=self.worker.consume(wait=1)
        self.assertEqual(message.original_to,'jobs@fixture');self.assertIn('owner',message.roles)
        self.assertEqual(json.loads(self.agent.handle(message)).get('error',{}).get('code'),'forbidden')
        self.assertEqual(self.ask('read deploys')['counts']['alert'],1)
    def test_kept_entry_uses_bus_config_work_directory_and_clean_shutdown(self):
        self.agent.close()
        env={key:value for key,value in os.environ.items() if not key.startswith(('AGENT_BUS_','AB_ROLE_','PYTHON'))}
        env.update(AGENT_BUS_ADDR=self.info['addr'],AGENT_BUS_NAME=self.worker.name,AGENT_BUS_TOKEN=self.info['tokens'][self.worker.name],AGENT_BUS_WORK=self.temp.name,AGENT_BUS_CONFIG='not used; fetch private Configuration')
        launch=Path(self.temp.name)/'launch';launch.mkdir()
        child=subprocess.Popen([sys.executable,'-B',str(HERE/'eventlog.py')],env=env,cwd=launch,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
        try:
            answer=self.owner.call(self.worker.name,'write deploys alert live',timeout=5)
            self.assertEqual(json.loads(answer.message.body)['entry']['text'],'live')
            self.assertTrue((Path(self.temp.name)/'events.json').is_file())
            child.terminate();out,err=child.communicate(timeout=5)
            self.assertEqual(child.returncode,0,err);self.assertEqual((out,err),('',''))
        finally:
            if child.poll() is None:child.kill();child.communicate(timeout=5)
