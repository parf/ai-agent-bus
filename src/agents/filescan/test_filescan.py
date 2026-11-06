"""FileScan end to end against a fake bus: scan, subscribe, deliver, confirm, answer calls."""
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
import change_sync as core
import test_change_sync as fake
from agent_bus.message import CallReply

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

    def test_an_unconfirmed_change_is_sent_again_and_a_second_subscriber_is_not_held_up(self):
        self.put('a.md');self.bus.kinds['#rag@n.test']='agent';self.bus.replies['#rag@n.test']='timeout'
        self.add('#rag@n.test');self.add('#q@n.test');self.call('scan')
        self.assertEqual([to for to,_,_ in self.bus.sent],['#q@n.test']);self.bus.replies['#rag@n.test']='ok'
        self.call('scan');self.assertEqual(len(self.bus.calls),2,'sent again, then confirmed');self.call('scan');self.assertEqual(len(self.bus.calls),2)
        status=self.call('status');self.assertEqual({s['target']:s['pending'] for s in status['subscribers']},{'#rag@n.test':0,'#q@n.test':0})

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
        for body in ('','nonsense','sub','sub add','scan confirm:maybe','{"method":3}','{"x":1}','sub add #q@n.test bogus-token','x'*5000):
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


if __name__=='__main__':unittest.main()
