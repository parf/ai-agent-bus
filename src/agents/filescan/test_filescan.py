"""FileScan end to end against a fake bus: scan, subscribe, deliver, confirm, answer calls."""
import contextlib
import hashlib
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import time
from types import SimpleNamespace
import unittest

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE));sys.path.insert(0,str(HERE.parent/'shared'));sys.path.insert(0,str(HERE.parents[1]/'clients/python'))
import filescan as fs
import settings as cfg
import change_sync as core
import test_change_sync as fake
from agent_bus.message import CallReply
from agent_bus.errors import Refusal
from agent_bus import storage as bus_storage
sys.path.insert(0,str(HERE.parent/'rag'))
import rag as rag_agent
import test_rag

NAME='#filescan/vault@n.test'


class Bus(fake.Bus):
    name=NAME


def msg(who='#me@n.test',roles=('write',),body='',ask=None,original=None):
    env={'from':who,'roles':list(roles),'body':body}
    if original:env['original_to']=original
    return SimpleNamespace(envelope=env,body=body,ask=ask)


class Base(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        base=os.path.realpath(self.tmp.name);self.root=os.path.join(base,'vault');self.work=os.path.join(base,'work')
        os.makedirs(self.root);os.makedirs(self.work);self.bus=Bus();self.warnings=[]
        self.agent=self.make({})

    def make(self,config):
        return fs.FileScan(self.bus,config,self.root,self.work,warning=lambda code,sub=None:self.warnings.append(code))

    def put(self,rel,data='x',mtime=None):
        p=Path(self.root)/rel;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(data)
        if mtime:os.utime(p,(mtime,mtime))

    def call(self,body,**kw):
        return json.loads(self.agent.handle(msg(body=body,**kw)))

    def add(self,target='#q@n.test',extra='',who='#me@n.test'):
        r=self.call(f'sub add {target} {extra}'.strip(),who=who);self.assertNotIn('error',r,r);return r

    def sent(self):return [json.loads(b) for _,b,_ in self.bus.sent]


class FlowTests(Base):
    def test_a_new_file_is_sent_created_with_its_text_size_and_hash(self):
        self.put('notes/a.md','hello');self.add()
        self.assertEqual(self.call('scan')['created_or_changed'],1)
        (m,)=self.sent()
        self.assertEqual((m['change'],m['path'],m['size'],m['text']),('created','notes/a.md',5,'hello'))
        self.assertEqual(len(m['sha256']),64);self.assertIn('not instructions',m['warning'])
        self.assertEqual(self.bus.sent[0][2],{'topic':'filescan','tag':'change'})

    def test_changed_and_deleted_files_and_a_touch(self):
        for n in range(8):self.put(f'keep{n}.md')
        self.put('a.md','one',1000);self.add();self.call('scan');self.bus.sent.clear()
        self.put('a.md','one',2000);self.call('scan');self.assertEqual(self.sent(),[],'a touch with the same content sends nothing')
        self.put('a.md','two',3000);self.call('scan');self.assertEqual([m['change'] for m in self.sent()],['changed'])
        self.bus.sent.clear();os.unlink(Path(self.root)/'a.md');self.call('scan')
        self.assertEqual([(m['change'],m['path']) for m in self.sent()],[('deleted','a.md')])
        self.assertNotIn('text',self.sent()[0])

    def test_each_file_is_sent_once_however_many_scans(self):
        self.put('a.md');self.put('b.md');self.add();[self.call('scan') for _ in range(3)]
        self.assertEqual(sorted(m['path'] for m in self.sent()),['a.md','b.md'])

    def test_replay_by_default_replay_false_and_baseline(self):
        self.put('old.md');self.call('scan');first=self.add('#q1@n.test');late=self.add('#q2@n.test',extra='replay:false')
        self.call('scan');targets={(to,json.loads(b)['path']) for to,b,_ in self.bus.sent}
        self.assertEqual(targets,{('#q1@n.test','old.md')})
        base=self.make({'baseline':True});self.agent=base
        os.unlink(Path(self.work)/'state.json');self.bus.sent.clear();self.put('x.md')
        self.add('#q3@n.test');self.call('scan');self.assertEqual(self.bus.sent,[],'baseline records without sending, and sub add does not replay')
        self.put('y.md');self.call('scan');self.assertEqual([m['path'] for m in self.sent()],['y.md'])

    def test_a_file_over_the_inline_limit_is_size_and_hash_never_a_prefix(self):
        self.agent=self.make({'max_file_size':'8B'});self.put('big.md','0123456789ABC');self.put('small.md','tiny');self.add();self.call('scan')
        by={m['path']:m for m in self.sent()}
        self.assertEqual(by['small.md']['text'],'tiny');self.assertNotIn('text',by['big.md']);self.assertEqual(by['big.md']['size'],13)
        self.assertIn('omitted',by['big.md']['contents']);self.assertNotIn('0123',json.dumps(by['big.md']))

    def test_a_path_filter_picks_what_a_subscriber_sees(self):
        self.put('a.md');self.put('b.txt');self.put('sub/c.md');self.add(extra=r'/\.md$/');self.call('scan')
        self.assertEqual(sorted(m['path'] for m in self.sent()),['a.md','sub/c.md'])

    def test_the_cap_per_scan_spreads_a_big_first_run(self):
        self.agent=self.make({'max_per_scan':2});[self.put(f'f{n}.md') for n in range(5)];self.add()
        self.assertTrue(self.call('scan')['capped']);self.assertEqual(len(self.sent()),2)
        self.call('scan');self.call('scan');self.assertEqual(len(self.sent()),5)


class CallTests(Base):
    def test_status_names_mode_tracked_files_subscribers_and_state_size(self):
        self.put('a.md');self.add();self.call('scan');s=self.call('status')
        self.assertEqual((s['mode'],s['tracked'],len(s['subscribers']),s['state_max']),('both',1,1,64*1024*1024));self.assertGreater(s['state_bytes'],0)

    def test_roles_gate_the_calls(self):
        for body in ('status','scan','sub list'):
            self.assertEqual(self.call(body,roles=())['error']['code'],'forbidden',body)
            self.assertNotIn('error',self.call(body,roles=('read',)),body)
        self.assertEqual(self.call('reset confirm:true',roles=('write',))['error']['code'],'forbidden')
        self.assertEqual(self.call('scan confirm:true',roles=('read',))['error']['code'],'forbidden')
        self.assertEqual(self.call('reset',roles=('owner',))['error']['code'],'not_confirmed')
        self.assertEqual(self.call('reset confirm:true',roles=('owner',)),{'reset':True})

    def test_a_call_to_a_name_the_caller_did_not_use_gets_no_manager_role(self):
        r=self.call('reset confirm:true',roles=('owner',),original='#other@n.test');self.assertEqual(r['error']['code'],'forbidden')

    def test_reset_forgets_everything_and_everything_replays(self):
        self.put('a.md');self.add();self.call('scan');self.bus.sent.clear()
        self.call('reset confirm:true',roles=('owner',));self.call('scan');self.assertEqual([m['path'] for m in self.sent()],['a.md'])

    def test_sub_calls_follow_the_shared_rules(self):
        r=self.add('#q@n.test');self.assertEqual(len(self.call('sub list')['subscriptions']),1)
        self.assertEqual(self.call('sub list',who='#you@n.test')['subscriptions'],[])
        self.assertEqual(self.call(f'sub delete {r["id"]}',who='#you@n.test')['error']['code'],'not_author')
        self.assertEqual(self.call(f'sub delete {r["id"]}')['deleted'],True)
        self.bus.allow[('#me@n.test','#q@n.test')]=False
        self.assertEqual(self.call('sub add #q@n.test')['error']['code'],'admission_refused')
        self.assertEqual(self.call('sub add #q@n.test /(a/')['error']['code'],'invalid_selector')

    def test_bad_requests_get_named_errors_not_a_crash(self):
        for body in ('','nonsense','sub','sub add','scan confirm:maybe','{"method":3}','{"x":1}','sub add #q@n.test bogus-token','sub add #q@n.test bigfile:true','x'*5000):
            self.assertEqual(self.call(body)['error']['code'],'invalid_request',body[:20])

    def test_a_damaged_state_is_named_and_reset_repairs_it(self):
        self.put('a.md');self.call('scan');(Path(self.work)/'state.json').write_text('{broken')
        self.assertEqual(self.call('status')['error']['code'],'state_corrupt')
        self.call('reset confirm:true',roles=('owner',));self.assertNotIn('error',self.call('status'))


class ParseTests(unittest.TestCase):
    def test_the_text_forms(self):
        self.assertEqual(fs.parse('sub add #rag@n /\\.md$/i as rag vault notes replay:false'),
                         ('sub.add',{'target':'#rag@n','pattern':'/\\.md$/i','rag':True,'namespace':'vault','table':'notes','replay':False}))
        self.assertEqual(fs.parse('sub add #t@n'),('sub.add',{'target':'#t@n'}))
        self.assertEqual(fs.parse('sub add #t@n as rag vault'),('sub.add',{'target':'#t@n','rag':True,'namespace':'vault'}))
        self.assertEqual(fs.parse('scan confirm:true'),('scan',{'confirm':True}));self.assertEqual(fs.parse('sub delete abc'),('sub.delete',{'id':'abc'}))
        self.assertEqual(fs.parse('{"method":"status"}'),('status',{}))



class AgentTargetTests(Base):
    """An agent that does not answer {status:ok} must not be a target: a confirmation nobody can give stalls the subscription."""
    def test_the_real_eventlog_agent_never_answers_status_ok_to_a_change_message(self):
        with tempfile.TemporaryDirectory() as tmp:
            log=eventlog_agent.Agent(shared_fixture.Bus('#worker@fixture'),{},tmp)
            try:
                ask=lambda body:log.handle(SimpleNamespace(body=body,ask=None,envelope={'from':NAME,'roles':['write']}))
                raw=ask(json.dumps({'change':'created','path':'a.md','warning':fs.WARNING}))
                valid=ask('write defaults notice ready')
            finally:log.close()
        for what,answer in (('a raw change message',raw),('a valid write',valid)):
            result=CallReply(SimpleNamespace(body=answer))
            self.assertFalse(core.reply_ok(result),what+' would never confirm under status:ok: '+answer[:80])
        self.assertIn('invalid_request',raw);self.assertIn('entry',json.loads(valid));self.assertNotIn('status',json.loads(valid))

    def test_an_agent_target_without_an_answer_contract_is_refused_naming_it_and_a_queue_is_not(self):
        self.bus.kinds['#eventlog@n.test']='agent'
        r=self.call('sub add #eventlog@n.test',who='#me@n.test')
        self.assertEqual(r['error']['code'],'agent_contract_missing');self.assertIn('as rag',r['error']['text'])
        self.assertEqual(self.call('sub list')['subscriptions'],[],'nothing was stored')
        self.assertNotIn('error',self.call('sub add #q@n.test'));self.bus.kinds[RAG]='agent'
        self.assertNotIn('error',self.call('sub add #rag@n.test as rag vault notes'))
        self.assertEqual(self.call('sub add #rag@n.test /x/')['error']['code'],'agent_contract_missing','a filter is not a contract')

    def test_an_agent_row_without_a_contract_sends_nothing(self):
        self.put('a.md');self.bus.kinds['#eventlog@n.test']='agent';self.add('#q@n.test')
        row=self.agent.state.load()['subs'];sub_id=next(iter(row));data=self.agent.state.load()
        data['subs'][sub_id]['target']='#eventlog@n.test';data['subs'][sub_id]['selector']={};self.agent.state.save(data)
        self.call('scan');self.assertEqual(self.bus.calls,[]);self.assertEqual(self.bus.sent,[])
        self.assertIn('agent_contract_missing',self.warnings)


if __name__=='__main__':unittest.main()


class GuardTests(Base):
    def populate(self,n=10):
        for i in range(n):self.put(f'f{i}.md',str(i))
        self.add();self.call('scan');self.bus.sent.clear()

    def test_a_pass_deleting_more_than_the_share_pauses_and_sends_no_delete(self):
        self.populate(10)
        for i in range(3):os.unlink(Path(self.root)/f'f{i}.md')  # 30 % > the 20 % default
        r=self.call('scan');self.assertEqual((r['deleted'],'3 of 10' in r['held']),(0,True))
        self.assertEqual(self.sent(),[]);s=self.call('status');self.assertIn('3 of 10',s['paused']['reason'])
        self.assertEqual(s['tracked'],10,'the files are still tracked')
        self.assertEqual(self.call('scan')['error']['code'],'paused','a paused agent refuses further scans')
        self.assertEqual(self.sent(),[])

    def test_confirm_releases_it_and_then_the_deletes_go(self):
        self.populate(10)
        for i in range(3):os.unlink(Path(self.root)/f'f{i}.md')
        self.call('scan');self.assertEqual(self.call('scan')['error']['code'],'paused');r=self.call('scan confirm:true',roles=('owner',));self.assertEqual(r['deleted'],3)
        self.assertEqual(sorted((m['change'],m['path']) for m in self.sent()),[('deleted','f0.md'),('deleted','f1.md'),('deleted','f2.md')])
        self.assertIsNone(self.call('status')['paused']);self.assertNotIn('error',self.call('scan'))

    def test_a_pass_under_the_share_sends_its_deletes(self):
        self.populate(10);os.unlink(Path(self.root)/'f0.md');os.unlink(Path(self.root)/'f1.md')  # 20 % is not more than 20 %
        self.assertEqual(self.call('scan')['deleted'],2);self.assertEqual(len(self.sent()),2)

    def test_the_absolute_limit_holds_for_a_big_tree(self):
        self.agent=self.make({'delete_max':2,'delete_pct':100});self.populate(10);os.unlink(Path(self.root)/'f0.md');os.unlink(Path(self.root)/'f1.md');os.unlink(Path(self.root)/'f2.md')
        self.assertIsNotNone(self.call('scan')['held']);self.assertEqual(self.sent(),[])

    def test_an_emptied_or_missing_directory_never_deletes_anything(self):
        self.populate(4)
        for i in range(4):os.unlink(Path(self.root)/f'f{i}.md')
        self.assertIn('empty',self.call('scan')['held']);self.assertIn('empty',self.call('status')['paused']['reason']);self.assertEqual(self.sent(),[])
        self.setUp();self.populate(4);os.rename(self.root,self.root+'-gone')
        r=self.call('scan');self.assertEqual(r['error']['code'],'paused');self.assertIn('cannot be read',r['error']['text']);self.assertEqual(self.sent(),[])
        self.assertEqual(self.call('scan')['error']['code'],'paused')
        self.assertEqual(self.call('status')['tracked'],4)
        r=self.call('scan confirm:true',roles=('owner',));self.assertEqual(r['error']['code'],'paused','still unreadable even when confirmed')
        os.rename(self.root+'-gone',self.root);self.assertNotIn('error',self.call('scan confirm:true',roles=('owner',)))

    def test_a_changed_directory_is_a_reset_not_a_confirm(self):
        self.populate(3);moved=os.path.join(os.path.dirname(self.root),'other');os.makedirs(moved);Path(moved,'z.md').write_text('z')
        self.agent=fs.FileScan(self.bus,{},moved,self.work,warning=lambda *a:None)
        r=self.call('scan');self.assertEqual(r['error']['code'],'paused');self.assertIn('changed from',r['error']['text'])
        r=self.call('scan confirm:true',roles=('owner',));self.assertEqual(r['error']['code'],'paused','confirm does not release a changed directory')
        self.call('reset confirm:true',roles=('owner',));self.assertNotIn('error',self.call('scan'))

    def test_the_first_scan_of_an_empty_directory_is_not_a_guard_case(self):
        self.assertNotIn('error',self.call('scan'));self.assertIsNone(self.call('status')['paused'])

    def test_confirm_needs_a_manager(self):
        self.populate(10)
        for i in range(5):os.unlink(Path(self.root)/f'f{i}.md')
        self.call('scan');self.assertEqual(self.call('scan confirm:true',roles=('write',))['error']['code'],'forbidden')


RAG='#rag@n.test'
sys.path.insert(0,str(HERE.parent/'eventlog'))
import eventlog as eventlog_agent
import importlib.util as _util
_spec=_util.spec_from_file_location('shared_subscription_fixture',HERE.parent/'shared/test_subscriptions.py')
shared_fixture=_util.module_from_spec(_spec);_spec.loader.exec_module(shared_fixture)


class RagBus(Bus):
    """Stands in for #rag: an `ok` answer is the real RAG agent's own answer, from its real Core over a fixture store with fake
    providers (nothing paid). Any other `replies` value is a scripted failure, as in the base bus."""
    def __init__(self):
        super().__init__()
        self.store=test_rag.Store();self.rag=rag_agent.Agent(None,test_rag.CFG);self.rag.core=rag_agent.Core(test_rag.CFG,self.store,test_rag.KV())
        for table in ('notes','docs'):self.rag.core.cache.set('vault',table,test_rag.PROMPT)
    def call(self,to,body,*,timeout,**options):
        if to!=RAG or self.replies.get(to,'ok')!='ok':return super().call(to,body,timeout=timeout,**options)
        self.calls.append((to,body,options));self.timeouts.append(timeout)
        with contextlib.redirect_stderr(io.StringIO()):
            answer=self.rag.handle(SimpleNamespace(ask=None,body=body,envelope={'from':NAME,'roles':['write']},roles=('write',)))
        return CallReply(SimpleNamespace(body=answer))


class BigBus(Bus):
    """Own-record bigfiles through the REAL client (agent_bus.storage.Record), over a daemon stand-in that answers the documented
    shape `{record,name,size,sha256,held,expires,replaced}`, refuses bytes that do not match X-Bigfile-Sha256, and keeps what it stores."""
    def __init__(self):
        super().__init__();self.files={};self.puts=[];self.deletes=[];self.put_refused=None;self.delete_refused=None
    def record(self,name):
        if name==self.name:return bus_storage.Record(self,name)
        return super().record(name)
    def _request(self,method,path,query=None,headers=None,upload=None,total=None,**more):
        if path=='/bigfile/delete':
            body=query;key=(body['record'],body['name'])
            if self.delete_refused:raise Refusal(*self.delete_refused)
            if key not in self.files:raise Refusal(404,'not_found: no such bigfile')
            del self.files[key];self.deletes.append(body['name']);return {'record':body['record'],'name':body['name'],'deleted':True}
        assert (method,path)==('PUT','/bigfile'),(method,path)
        if self.put_refused:
            for _ in upload:pass
            raise Refusal(*self.put_refused)
        data=b''.join(upload)
        if headers.get('X-Bigfile-Sha256') not in (None,hashlib.sha256(data).hexdigest()):raise Refusal(400,'malformed: hash does not match')
        key=(query['record'],query['name']);replaced=key in self.files;self.files[key]=data;self.puts.append(dict(query))
        return {'record':query['record'],'name':query['name'],'size':len(data),'sha256':hashlib.sha256(data).hexdigest(),'held':'file',
                'expires':'2026-11-06T09:00:00Z','replaced':replaced}


class BigfileTests(Base):
    BIG='x'*40

    def setUp(self):
        super().setUp();self.bus=BigBus();self.agent=self.make({'max_file_size':'10B'})

    def messages(self,to=None):return [json.loads(b) for t,b,_ in self.bus.sent if to in (None,t)]

    def test_a_subscriber_that_opted_in_gets_a_bigfile_for_a_file_over_the_inline_limit(self):
        self.put('big.txt',self.BIG);self.put('small.txt','tiny');self.bus.kinds['#plain@n.test']='queue'
        self.add('#q@n.test',extra='bigfile');self.add('#plain@n.test');self.call('scan')
        by={m['path']:m for m in self.messages('#q@n.test')};plain={m['path']:m for m in self.messages('#plain@n.test')}
        digest=hashlib.sha256(self.BIG.encode()).hexdigest();found=by['big.txt']['bigfile']
        self.assertEqual({k:found[k] for k in ('record','name','size','sha256')},{'record':NAME,'name':digest,'size':40,'sha256':digest})
        self.assertEqual(found['expires'],'2026-11-06T09:00:00Z');self.assertNotIn('contents',by['big.txt']);self.assertNotIn('text',by['big.txt'])
        self.assertEqual(self.bus.files,{(NAME,digest):self.BIG.encode()},'the bytes are stored under the hash, once')
        self.assertEqual(by['small.txt']['text'],'tiny');self.assertNotIn('bigfile',by['small.txt'])
        self.assertEqual(plain['big.txt']['contents'],'omitted: over max_file_size or not text');self.assertNotIn('bigfile',plain['big.txt'])
        self.assertEqual(len(self.bus.puts),1)

    def test_a_binary_file_under_the_limit_is_a_bigfile_too_and_the_ttl_is_the_setting(self):
        (Path(self.root)/'img.bin').write_bytes(b'\xff\xfe\x00\x01');self.add(extra='bigfile');self.call('scan')
        self.assertEqual(self.messages()[0]['bigfile']['size'],4);self.assertEqual(self.bus.puts[0]['ttl'],3600,'the default is one hour')
        self.agent=self.make({'max_file_size':'10B','bigfile_ttl':'30m'});(Path(self.root)/'img2.bin').write_bytes(b'\xfe\xfd');self.call('scan')
        self.assertEqual(self.bus.puts[-1]['ttl'],1800)

    def test_a_file_over_the_bus_limit_is_named_omitted_and_never_uploaded(self):
        self.put('huge.txt','y'*100);self.add(extra='bigfile')
        old=fs.BIGFILE_MAX;fs.BIGFILE_MAX=64
        try:self.call('scan')
        finally:fs.BIGFILE_MAX=old
        (m,)=self.messages();self.assertIn('MiB bigfile limit',m['contents']);self.assertNotIn('bigfile',m);self.assertEqual(self.bus.puts,[])

    def test_a_refused_upload_holds_the_change_and_it_is_sent_once_when_the_store_recovers(self):
        self.put('big.txt',self.BIG);self.add(extra='bigfile');self.bus.put_refused=(503,'unavailable')
        self.call('scan');self.assertEqual(self.messages(),[]);self.assertIn('body_failed',self.warnings)
        self.assertEqual(self.call('status')['subscribers'][0]['pending'],1)
        self.bus.put_refused=None;self.call('scan');self.call('scan')
        self.assertEqual([m['path'] for m in self.messages()],['big.txt'],'exactly one message, after recovery')

    def test_a_file_that_changed_under_the_upload_is_refused_by_the_hash_and_tried_again(self):
        self.put('big.txt',self.BIG);self.add(extra='bigfile');real=fs.scanning.digest;calls=[]
        def stale(root,rel,keep=0):
            sha,size,text=real(root,rel,keep);calls.append(1)
            return ('0'*64,size,text) if len(calls)==2 else (sha,size,text)   # call 1 is the poll's hash, call 2 the body's
        fs.scanning.digest=stale
        try:
            self.call('scan');self.assertEqual(self.messages(),[],'the hash the store checks did not match the bytes')
            self.call('scan')
        finally:fs.scanning.digest=real
        (m,)=self.messages();self.assertEqual(m['bigfile']['sha256'],hashlib.sha256(self.BIG.encode()).hexdigest())

    def keys(self):return sorted(name for _,name in self.bus.files)

    def sha(self,text):return hashlib.sha256(text.encode()).hexdigest()

    def settle_all(self):
        self.call('scan');self.call('scan')

    def test_a_superseded_version_is_deleted_once_every_opted_in_subscriber_confirmed_the_newer_one(self):
        self.bus.kinds['#q2@n.test']='queue'
        self.put('big.txt',self.BIG);self.add(extra='bigfile');self.add('#q2@n.test',extra='bigfile');self.call('scan')
        self.assertEqual(self.keys(),[self.sha(self.BIG)])
        newer='y'*50;self.put('big.txt',newer,mtime=time.time()+5);self.bus.send_refused.add('#q2@n.test');self.call('scan')
        self.assertEqual(sorted(self.keys()),sorted([self.sha(self.BIG),self.sha(newer)]),'the second subscriber has not confirmed the newer one: the old key stays')
        self.bus.send_refused.clear();self.call('scan')
        self.assertEqual(self.keys(),[self.sha(newer)],'one key for the path after two changes');self.assertEqual(self.bus.deletes,[self.sha(self.BIG)])
        self.call('scan');self.assertEqual(self.bus.deletes,[self.sha(self.BIG)],'deleted once')

    def test_a_deleted_file_loses_its_bigfile_after_the_delete_is_confirmed_and_shared_content_stays_while_another_path_has_it(self):
        for i in range(10):self.put(f'k{i}.txt','k')
        self.put('one.txt',self.BIG);self.put('two.txt',self.BIG);self.add(extra='bigfile');self.call('scan');self.assertEqual(self.keys(),[self.sha(self.BIG)],'same bytes, one key')
        os.unlink(Path(self.root)/'one.txt');self.call('scan');self.assertEqual(self.keys(),[self.sha(self.BIG)],'two.txt still has these bytes')
        os.unlink(Path(self.root)/'two.txt');self.call('scan');self.assertEqual(self.keys(),[]);self.assertEqual(self.bus.deletes,[self.sha(self.BIG)])

    def test_a_failed_delete_is_tried_again_and_an_absent_key_counts_as_deleted(self):
        for i in range(10):self.put(f'k{i}.txt','k')
        self.put('big.txt',self.BIG);self.add(extra='bigfile');self.call('scan');newer='z'*45
        self.put('big.txt',newer,mtime=time.time()+5);self.bus.delete_refused=(503,'unavailable');self.call('scan')
        self.assertEqual(len(self.keys()),2);self.bus.delete_refused=None;self.call('scan');self.assertEqual(self.keys(),[self.sha(newer)])
        self.bus.files.clear();os.unlink(Path(self.root)/'big.txt')
        self.call('scan');self.call('scan');held=self.agent.state.load()['meta']['bigfiles']
        self.assertEqual(held,{},'the key had expired (404): nothing is kept to delete')

    def test_a_key_stays_until_the_delete_of_its_path_is_confirmed(self):
        for i in range(10):self.put(f'k{i}.txt','k')
        self.put('one.txt',self.BIG);self.add(extra='bigfile');self.call('scan');os.unlink(Path(self.root)/'one.txt')
        self.bus.send_refused.add('#q@n.test');self.call('scan');self.assertEqual(self.keys(),[self.sha(self.BIG)],'the delete is not confirmed yet')
        self.bus.send_refused.clear();self.call('scan');self.assertEqual(self.keys(),[])

    def test_a_key_shared_by_two_paths_waits_for_both(self):
        for i in range(10):self.put(f'k{i}.txt','k')
        self.put('one.txt',self.BIG);self.put('two.txt',self.BIG);self.agent=self.make({'max_file_size':'10B','batch':1})
        self.add(extra='bigfile')
        for _ in range(14):self.call('scan')   # one change per round
        self.assertEqual(self.keys(),[self.sha(self.BIG)])
        os.unlink(Path(self.root)/'one.txt');(Path(self.root)/'two.txt').write_text('w'*60);os.utime(Path(self.root)/'two.txt',(time.time()+9,time.time()+9))
        self.call('scan')   # round one: only one.txt's delete (sorted first) is confirmed
        self.assertIn(self.sha(self.BIG),self.keys(),'two.txt has not yet been told its new version')
        self.call('scan');self.assertEqual(self.keys(),[self.sha('w'*60)])

    def test_a_failed_state_hold_keeps_the_notes_and_a_later_round_collects_the_key_once(self):
        for i in range(10):self.put(f'k{i}.txt','k')
        self.put('big.txt',self.BIG);self.add(extra='bigfile');real=self.agent.state.hold;newer='n'*70
        self.call('scan')   # uploads and notes the first key
        self.put('big.txt',newer,mtime=time.time()+5)
        def failing(*a,**k):
            raise core.SyncError('state_unwritable')
        calls=[]
        orig=self.agent._collect_bigfiles
        def collect():
            calls.append(1)
            if len(calls)==1:self.agent.state.hold=failing
            try:return orig()
            finally:self.agent.state.hold=real
        self.agent._collect_bigfiles=collect
        self.call('scan')   # delivers the newer version and uploads it; the collection after it cannot hold the state
        self.assertEqual(sorted(self.keys()),sorted([self.sha(self.BIG),self.sha(newer)]))
        self.call('scan');self.assertEqual(self.keys(),[self.sha(newer)]);self.assertEqual(self.bus.deletes,[self.sha(self.BIG)],'collected once')
        self.assertEqual(list(self.agent.state.load()['meta']['bigfiles']),[self.sha(newer)],'the lost round did not lose the new key')

    def test_a_subscriber_without_the_option_makes_no_key_to_collect(self):
        self.put('big.txt',self.BIG);self.add();self.call('scan');self.assertEqual(self.bus.puts,[]);self.assertNotIn('bigfiles',self.agent.state.load()['meta'])

    def test_a_file_changed_between_the_scan_and_the_send_is_not_sent_under_a_hash_the_state_lacks(self):
        for i in range(10):self.put(f'k{i}.txt','k')
        self.put('big.txt',self.BIG);self.add(extra='bigfile');real=fs.scanning.digest;calls=[];newer='q'*55
        def racing(root,rel,keep=0):
            if rel=='big.txt':
                calls.append(1)
                if len(calls)==2:(Path(root)/rel).write_text(newer);os.utime(Path(root)/rel,(time.time()+9,time.time()+9))   # the second hash is the body's
            return real(root,rel,keep)
        fs.scanning.digest=racing
        try:
            self.call('scan')
            self.assertEqual([m for m in self.messages() if m['path']=='big.txt'],[],'the body saw bytes the state does not hold: nothing sent, nothing kept')
            self.assertEqual(self.bus.puts,[]);self.call('scan');self.call('scan')
        finally:fs.scanning.digest=real
        (m,)=[m for m in self.messages() if m['path']=='big.txt']
        self.assertEqual(m['bigfile']['sha256'],self.sha(newer));self.assertEqual(self.keys(),[self.sha(newer)],'the key that was sent is still there')
        self.assertEqual(self.bus.deletes,[]);self.assertEqual(self.agent.state.load()['items']['big.txt']['fp'],self.sha(newer))

    def test_a_file_rewritten_during_the_upload_is_refused_by_the_store_and_sent_once_next_round(self):
        for i in range(10):self.put(f'k{i}.txt','k')
        self.put('big.txt',self.BIG);self.add(extra='bigfile');real=fs.scanning.open_under;done=[]
        def rewriting(root,rel):
            if rel=='big.txt':done.append(1)
            if rel=='big.txt' and len(done)==3:(Path(root)/rel).write_text('w'*40)   # opens: the poll's hash, the body's hash, then the upload;os.utime(Path(root)/rel,(time.time()+9,time.time()+9))   # same size, other bytes
            return real(root,rel)
        fs.scanning.open_under=rewriting
        try:
            self.call('scan');self.assertEqual([m for m in self.messages() if m['path']=='big.txt'],[],'the store refused bytes that do not match the hash')
            self.assertEqual(self.bus.puts,[]);self.call('scan');self.call('scan')
        finally:fs.scanning.open_under=real
        (m,)=[m for m in self.messages() if m['path']=='big.txt'];self.assertEqual(m['bigfile']['sha256'],self.sha('w'*40));self.assertEqual(self.keys(),[self.sha('w'*40)])

    def test_bigfile_is_a_queue_option_and_cannot_join_as_rag(self):
        self.assertEqual(fs.parse('sub add #q@n /\\.bin$/ bigfile replay:false'),('sub.add',{'target':'#q@n','pattern':'/\\.bin$/','bigfile':True,'replay':False}))
        self.assertEqual(self.call('sub add #q@n.test bigfile as rag vault notes')['error']['code'],'invalid_selector')
        self.assertEqual(self.call('{"method":"sub.add","target":"#q@n.test","bigfile":"yes"}')['error']['code'],'invalid_selector')
        self.assertEqual(self.call('{"method":"sub.add","target":"#q@n.test","bigfile":false}')['error']['code'],'invalid_selector')
        self.assertNotIn('error',self.call('sub add #q@n.test bigfile'));self.assertEqual(self.call('sub list')['subscriptions'][0]['selector'],{'bigfile':True})

    def test_a_delete_carries_no_bigfile_and_the_setting_is_checked(self):
        self.put('big.txt',self.BIG)
        for i in range(10):self.put(f'k{i}.txt','k')
        self.add(extra='bigfile');self.call('scan');self.bus.sent.clear();os.unlink(Path(self.root)/'big.txt')
        self.call('scan');self.assertEqual([k for k in self.messages() if k['change']=='deleted'],[{'change':'deleted','path':'big.txt','warning':fs.WARNING}])
        with self.assertRaises(cfg.SettingsError):cfg.parse({'bigfile_ttl':'0s'})


class RagTests(Base):
    def setUp(self):
        super().setUp();self.bus=RagBus();self.agent=self.make({})

    def rag(self,extra='as rag vault notes'):
        self.bus.kinds[RAG]='agent';return self.add(RAG,extra=extra)

    def calls(self):return [json.loads(b) for _,b,_ in self.bus.calls]

    def test_an_unconfirmed_change_is_sent_again_and_a_second_subscriber_is_not_held_up(self):
        self.put('a.md');self.bus.replies['#rag@n.test']='timeout'
        self.rag();self.add('#q@n.test');self.call('scan')
        self.assertEqual([to for to,_,_ in self.bus.sent],['#q@n.test']);self.bus.replies['#rag@n.test']='ok'
        self.call('scan');self.assertEqual(len(self.bus.calls),2,'sent again, then confirmed');self.call('scan');self.assertEqual(len(self.bus.calls),2)
        status=self.call('status');self.assertEqual({s['target']:s['pending'] for s in status['subscribers']},{'#rag@n.test':0,'#q@n.test':0})

    def test_a_real_rag_upsert_is_confirmed_by_its_record_and_never_sent_again(self):
        self.put('a.md','hello');self.rag();self.call('scan')
        self.assertEqual(len(self.calls()),1);self.assertEqual(self.call('status')['subscribers'][0]['pending'],0)
        record=self.bus.rag.core.get({'namespace':'vault','tableName':'notes','id':'a.md'},self.bus.rag.core.catalogue.bind('vault',self.bus.rag.core.cache,'notes'))
        self.assertEqual((record['source'],record['content_hash']),('a.md',__import__('hashlib').sha256(b'hello').hexdigest()),'RAG holds what was sent')
        paid=len(self.bus.store.paid);self.call('scan');self.call('scan')
        self.assertEqual(len(self.calls()),1,'a confirmed upsert is not resent');self.assertEqual(len(self.bus.store.paid),paid,'and no provider work is paid again')

    def test_an_upsert_is_confirmed_only_by_the_record_that_was_sent(self):
        import hashlib
        self.put('a.md','hello');self.rag();digest=hashlib.sha256(b'hello').hexdigest()
        base={'namespace':'vault','tableName':'notes','id':'a.md','source':'a.md','content_hash':digest}
        self.bus.replies[RAG]=json.dumps(base|{'data':'hello'})   # the echo of the call is a confirmation
        self.call('scan');self.assertEqual(self.call('status')['subscribers'][0]['pending'],0)
        for what,reply in (('status:ok alone','{"status":"ok"}'),('another hash',json.dumps(base|{'content_hash':'0'*64})),
                           ('another id',json.dumps(base|{'id':'b.md'})),('another source',json.dumps(base|{'source':'old.md'})),
                           ('another table',json.dumps(base|{'tableName':'docs'})),('another namespace',json.dumps(base|{'namespace':'team'})),
                           ('a refusal',json.dumps(base|{'error':'x'})),('not_found text','error: not_found: record is absent from this namespace'),
                           ('not json','stored')):
            with self.subTest(what):
                self.agent=self.make({});self.call('reset confirm:true',roles=('owner',))
                self.bus.replies[RAG]=reply;self.call('scan');status=self.call('status')['subscribers'][0]
                self.assertEqual(status['pending'],1,what)

    def test_a_file_becomes_ragss_own_upsert_and_a_delete_its_own_delete(self):
        for i in range(8):self.put(f'k{i}.md','k')
        self.put('notes/a.md','hello');self.rag();self.call('scan')
        mine=[c for c in self.calls() if c['id']=='notes/a.md']
        import hashlib
        self.assertEqual(mine,[{'method':'upsert','namespace':'vault','tableName':'notes','id':'notes/a.md','data':'hello','mime-type':'text/markdown',
                                'source':'notes/a.md','content_hash':hashlib.sha256(b'hello').hexdigest()}])
        self.bus.calls.clear();os.unlink(Path(self.root)/'notes/a.md');self.call('scan')
        self.assertEqual(self.calls(),[{'method':'delete','namespace':'vault','tableName':'notes','id':'notes/a.md'}])

    def test_every_upsert_names_the_path_filescan_reports_as_the_source_and_a_hash_that_rag_will_check(self):
        import hashlib
        self.put('d/x.md','café\r\nline');self.rag();self.call('scan');(c,)=self.calls()
        self.assertEqual(c['source'],'d/x.md','the same relative path the change message reports, with no second path builder and no host layout')
        self.assertEqual(c['content_hash'],hashlib.sha256(c['data'].encode()).hexdigest(),'the hash is of the exact text sent, as RAG checks it')
        self.assertEqual(c['content_hash'],hashlib.sha256((Path(self.root)/'d/x.md').read_bytes()).hexdigest())

    def test_a_deleted_file_needs_no_source_and_a_path_rag_cannot_take_is_skipped(self):
        self.put('d/ok.md','a');self.put('e/'+'/'.join(['z'*200]*11)+'.md','b');self.put('e/nfd-e\u0301.md','c')
        self.rag();self.call('scan');ids=[c['id'] for c in self.calls() if c['method']=='upsert']
        self.assertEqual(ids,['d/ok.md'],'a 2048+ character path and a non-NFC path cannot be RAG sources')
        names=next(iter(self.call('status')['skipped_for_rag'].values()))
        self.assertTrue(any('source' in why for why in names.values()),names)

    def test_nothing_is_tracked_on_a_refusal_and_it_is_sent_again(self):
        self.put('a.md','x');self.rag();self.bus.replies['#rag@n.test']='{"error":"no"}';self.call('scan')
        self.assertEqual(len(self.bus.calls),1);self.assertEqual(self.call('status')['subscribers'][0]['pending'],1)
        self.bus.replies['#rag@n.test']='ok';self.call('scan');self.assertEqual(len(self.bus.calls),2);self.assertEqual(self.call('status')['subscribers'][0]['pending'],0)

    def test_a_long_path_is_hashed_with_the_path_kept_in_key1_and_an_odd_one_is_skipped(self):
        long='d/'+'x'*130+'.md';self.put(long,'t');self.put('line break.md','t');self.rag();self.call('scan')
        by={c['id']:c for c in self.calls()}
        h=[c for c in by.values() if c.get('key1')==long]
        self.assertEqual((len(h),len(h[0]['id'])),(1,71))
        self.assertEqual([c for c in by.values() if c.get('key1','').startswith('line')],[],'a path with a control or line separator cannot be a RAG source, so it is skipped, not sent')

    def test_a_file_over_rags_limit_or_not_text_is_skipped_for_rag_and_named_in_status(self):
        self.put('big.md','x'*(fs.RAG_TEXT+1));Path(self.root,'bin.dat').write_bytes(b'\xff\xfe\x01');self.put('ok.md','fine')
        self.agent=self.make({'max_file_size':'1MiB'});self.rag();self.call('scan')
        self.assertEqual([c['id'] for c in self.calls() if c['method']=='upsert'],['ok.md']);self.assertEqual(sorted(c['id'] for c in self.calls() if c['method']=='delete'),['big.md','bin.dat'])
        skipped=self.call('status')['skipped_for_rag'];names=set(next(iter(skipped.values())))
        self.assertEqual(names,{'big.md','bin.dat'});self.assertEqual(self.call('status')['subscribers'][0]['pending'],0)

    def test_a_file_that_grows_past_ragss_limit_has_its_stale_text_deleted_and_is_named(self):
        self.agent=self.make({'max_file_size':'1MiB'});self.put('a.md','small');self.rag();self.call('scan');self.bus.calls.clear()
        self.put('a.md','x'*(fs.RAG_TEXT+1),mtime=time.time()+5);self.call('scan')
        self.assertEqual(self.calls(),[{'method':'delete','namespace':'vault','tableName':'notes','id':'a.md'}])
        self.assertIn('was deleted from RAG',next(iter(self.call('status')['skipped_for_rag'].values()))['a.md'])
        self.bus.calls.clear();self.call('scan');self.assertEqual(self.bus.calls,[],'done, not repeated')

    def test_rag_counts_bytes_so_a_wide_character_path_is_judged_as_rag_judges_it(self):
        wide='/'.join(['\u6587'*80]*9)+'.md'   # 720 characters, over 2100 bytes: over RAG's 2048-byte source limit
        self.assertLess(len(wide),2048);self.assertGreater(len(wide.encode()),2048)
        self.assertFalse(fs.rag_source_ok(wide));self.assertTrue(fs.rag_source_ok('\u6587'*600))
        self.assertFalse(fs.rag_source_ok('   '),'RAG refuses a blank source');self.assertFalse(fs.rag_source_ok(''))
        plain=fs.rag_id('\u6587'*50+'.md')   # 150 bytes, over the 128-byte id limit though only 53 characters
        self.assertEqual(plain[0][:7],'sha256:');self.assertEqual(plain[1],'\u6587'*50+'.md')
        self.assertEqual(fs.rag_id('\u6587'*40+'.md'),('\u6587'*40+'.md',None))   # 123 bytes: still its own id
        self.put(wide,'text');self.put('\u6587'*50+'.md','b');self.rag();self.call('scan')
        sent=[c for c in self.calls() if c['method']=='upsert']
        self.assertEqual([c['source'] for c in sent],['\u6587'*50+'.md'],'the wide path is skipped, and the 150-byte name goes out under a hashed id')
        self.assertTrue(sent[0]['id'].startswith('sha256:') and sent[0]['key1']=='\u6587'*50+'.md')
        self.assertTrue(any(wide in k for v in self.call('status')['skipped_for_rag'].values() for k in v),'named in status')

    def test_key1_is_cut_by_bytes_on_a_character_boundary_and_a_blank_name_takes_a_hashed_id(self):
        name='/'.join(['\u6587'*50,'\u6587'*50])+'.md'    # 304 bytes: a valid source, too long for RAG's 256-byte key1
        ident,key1=fs.rag_id(name)
        self.assertEqual(ident[:7],'sha256:');self.assertLessEqual(len(key1.encode()),256);self.assertGreater(len(key1.encode()),250)
        self.assertTrue(name.startswith(key1));key1.encode().decode('utf-8')   # whole characters only
        for blank in ('   ','\t','\u00a0'):
            i,k=fs.rag_id(blank);self.assertEqual(i[:7],'sha256:',repr(blank));self.assertEqual(k,blank)
        self.put('   ','blank');self.put(name,'wide');self.rag();self.call('scan')
        for c in self.calls():
            self.assertTrue(c['id'].strip(),c);self.assertLessEqual(len(c.get('key1','').encode()),256)
        self.assertEqual(self.call('status')['subscribers'][0]['pending'],0,'neither file stalls the subscription')

    def test_a_file_that_became_unsendable_is_deleted_from_rag_on_create_too_after_a_reset(self):
        self.agent=self.make({'max_file_size':'1MiB'});self.put('big.md','x'*(fs.RAG_TEXT+1));self.rag()
        self.call('scan');self.assertEqual([c['method'] for c in self.calls()],['delete'],'RAG may still hold its old text, so a replay deletes it')
        self.assertEqual(self.calls()[0]['id'],'big.md');self.bus.calls.clear();self.call('scan');self.assertEqual(self.bus.calls,[],'once, not every scan')

    def test_rag_refusing_to_delete_an_id_it_does_not_hold_counts_as_the_delete_done(self):
        self.agent=self.make({'max_file_size':'1MiB'})
        for i in range(8):self.put(f'k{i}.md','k')
        self.put('a.md','small');self.rag();self.call('scan');self.bus.calls.clear()
        self.put('a.md','x'*(fs.RAG_TEXT+1),mtime=time.time()+5);self.call('scan')   # RAG deletes the stale text
        self.assertEqual([c['method'] for c in self.calls()],['delete']);self.bus.calls.clear()
        os.unlink(Path(self.root)/'a.md');self.bus.replies['#rag@n.test']='error: not_found: record is absent from this namespace'
        self.call('scan');self.assertEqual([c['method'] for c in self.calls()],['delete'])
        self.assertEqual(self.call('status')['subscribers'][0]['pending'],0,'the delete for an id already gone is settled, not retried forever')
        self.bus.calls.clear();self.call('scan');self.assertEqual(self.bus.calls,[])

    def test_the_not_found_answer_settles_only_a_delete_never_an_upsert(self):
        self.put('a.md','x');self.rag();self.bus.replies['#rag@n.test']='error: not_found: record is absent from this namespace'
        self.call('scan');self.assertEqual(self.call('status')['subscribers'][0]['pending'],1,'an upsert that is refused stays pending')

    def test_a_path_that_looks_like_a_hash_cannot_collide_with_a_hashed_one(self):
        import hashlib
        long='d/'+'y'*130;h=hashlib.sha256(long.encode()).hexdigest()
        self.assertEqual(fs.rag_id(h),(h,None));self.assertEqual(fs.rag_id(long)[0],'sha256:'+h);self.assertNotEqual(fs.rag_id(h)[0],fs.rag_id(long)[0])
        self.assertEqual(fs.rag_id('sha256:'+h)[0],'sha256:'+hashlib.sha256(('sha256:'+h).encode()).hexdigest(),'a plain path in the hashed form is hashed too')

    def test_without_a_namespace_and_table_rag_gets_nothing_and_status_says_so(self):
        self.put('a.md');self.rag(extra='as rag');self.call('scan');self.assertEqual(self.bus.calls,[])
        self.assertIn('namespace',next(iter(self.call('status')['skipped_for_rag'].values()))['*'])
        self.agent=self.make({'namespace':'vault','table':'notes'});self.call('scan')
