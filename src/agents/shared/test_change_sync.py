"""The shared core of FileScan and DBSync: state file, subscriptions, confirmed delivery, poll timer."""
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import threading
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
import change_sync as cs
from subscriptions import SubscriptionError
from agent_bus.errors import Refusal, BusTimeout
from agent_bus.message import CallDone, CallReply

SOURCE='#filescan/vault@n.test'


class Bus:
    def __init__(self):
        self.allow={};self.kinds={};self.sent=[];self.calls=[];self.timeouts=[];self.delay=0;self.queries=[];self.replies={};self.send_refused=set()
    def may_send(self,who,target):
        self.queries.append((who,target));value=self.allow.get((who,target),True)
        if value=='404':raise Refusal(404,'absent')
        return value
    def record(self,name):
        return SimpleNamespace(lookup=lambda:{'name':name,'kind':self.kinds.get(name,'queue')})
    def send(self,to,body,**options):
        if to in self.send_refused:raise Refusal(403,'refused')
        self.sent.append((to,body,options));return 'id'
    def call(self,to,body,*,timeout,**options):
        self.calls.append((to,body,options));self.timeouts.append(timeout);value=self.replies.get(to,'ok')
        if self.delay:time.sleep(self.delay)
        if value=='timeout':raise BusTimeout()
        if value=='refused':raise Refusal(403,'refused')
        if value=='done':return CallDone(SimpleNamespace(body=''))
        return CallReply(SimpleNamespace(body=json.dumps({'status':'ok'}) if value=='ok' else value))


def msg(who,roles=()):
    return SimpleNamespace(envelope={'from':who,'roles':list(roles)})


def item(fp):return {'fp':fp}


class Base(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.dir=Path(self.tmp.name);self.state=cs.StateFile(self.dir);self.ids=iter(f'{n:012x}' for n in range(1,1000))

    def put(self,items):
        data=self.state.load();data['items'].update(items);self.state.save(data)

    def subs(self,bus=None,**kw):
        self.bus=bus or Bus()
        return cs.SyncSubscriptions(self.bus,SOURCE,self.state,validate_selector=lambda v:v,id_factory=lambda:next(self.ids),**kw)

    def round(self,**kw):
        args=dict(matches=lambda row,key:True,body=lambda row,key,action,record:f'{action} {key}',topic='t',tag='x')
        args.update(kw);return cs.deliver(self.bus,SOURCE,self.state,**args)


class StateFileTests(Base):
    def test_a_missing_file_is_a_fresh_state_and_a_save_round_trips(self):
        self.assertEqual(self.state.load()['items'],{})
        self.put({'a/b':item('1')})
        self.assertEqual(cs.StateFile(self.dir).load()['items'],{'a/b':{'fp':'1'}})
        self.assertEqual(stat.S_IMODE(self.state.path.stat().st_mode),0o600)

    def test_a_damaged_file_is_refused_never_read_as_empty(self):
        for text in ('{not json','[]','{"version":2,"items":{},"subs":{},"marks":{},"maybe":{},"meta":{}}',
                     '{"version":1,"items":{"a":{"nofp":1}},"subs":{},"marks":{},"maybe":{},"meta":{}}',
                     '{"version":1,"items":{},"subs":{},"marks":{"zz":{}},"maybe":{},"meta":{}}',
                     '{"version":1,"items":{},"subs":{"a":{}},"marks":{},"maybe":{},"meta":{}}'):
            self.state.path.write_text(text)
            with self.assertRaises(cs.SyncError,msg=text) as c:self.state.load()
            self.assertEqual(c.exception.code,'state_corrupt')
            self.assertEqual(self.state.path.read_text(),text)

    def test_a_save_past_the_cap_is_refused_and_the_old_file_stays(self):
        small=cs.StateFile(self.dir,max_bytes=400);data=small.load();data['items']['k']=item('x');small.save(data)
        before=small.path.read_bytes()
        data['items']['big']=item('y'*500)
        with self.assertRaises(cs.SyncError) as c:small.save(data)
        self.assertEqual(c.exception.code,'state_full');self.assertIn('state full:',str(c.exception));self.assertIn('of 400 bytes',str(c.exception))
        self.assertEqual(small.path.read_bytes(),before)
        self.assertEqual([p.name for p in self.dir.iterdir() if cs.StateFile.TEMP in p.name],[])

    def test_a_write_that_fails_leaves_the_old_state_and_no_temporary(self):
        self.put({'k':item('old')});before=self.state.path.read_bytes()
        data=self.state.load();data['items']['k']=item('new')
        for target in ('os.fsync','os.replace'):
            with patch(target,side_effect=OSError(5,'io')):
                with self.assertRaises(cs.SyncError) as c:self.state.save(data)
            self.assertEqual(c.exception.code,'state_unwritable')
            self.assertEqual(self.state.path.read_bytes(),before,target)
            self.assertEqual([p.name for p in self.dir.iterdir() if cs.StateFile.TEMP in p.name],[],target)

    def test_a_state_is_replaced_whole_by_a_rename(self):
        self.put({'k':item('old')});inode=self.state.path.stat().st_ino;seen=[]
        real=os.replace
        def spy(a,b):
            seen.append((Path(a).name.startswith(cs.StateFile.TEMP.join(('state.json',''))),json.loads(Path(a).read_text())['items']['k']['fp'],json.loads(Path(b).read_text())['items']['k']['fp']))
            return real(a,b)
        data=self.state.load();data['items']['k']=item('new')
        with patch('os.replace',spy):self.state.save(data)
        self.assertEqual(seen,[(True,'new','old')])
        self.assertNotEqual(self.state.path.stat().st_ino,inode)

    def test_a_dead_write_leaves_a_temporary_that_is_cleaned_without_touching_the_state(self):
        self.put({'k':item('1')});before=self.state.path.read_bytes()
        (self.dir/'state.json.tmp.99.dead').write_text('half');(self.dir/'other.txt').write_text('x')
        self.assertEqual(self.state.clean_temporaries(),1)
        self.assertEqual(self.state.path.read_bytes(),before);self.assertTrue((self.dir/'other.txt').exists())

    def test_non_finite_numbers_are_not_state_on_save_or_on_load(self):
        self.put({'k':item('old')});before=self.state.path.read_bytes()
        for bad in (float('nan'),float('inf'),float('-inf')):
            data=self.state.load();data['items']['k']={'fp':'x','n':bad}
            with self.assertRaises(cs.SyncError,msg=repr(bad)) as c:self.state.save(data)
            self.assertEqual(c.exception.code,'state_corrupt');self.assertEqual(self.state.path.read_bytes(),before)
        for token in ('NaN','Infinity','-Infinity'):
            self.state.path.write_text('{"version":1,"items":{"k":{"fp":"x","n":%s}},"subs":{},"marks":{},"maybe":{},"meta":{}}'%token)
            with self.assertRaises(cs.SyncError,msg=token) as c:self.state.load()
            self.assertEqual(c.exception.code,'state_corrupt')
        self.state.path.write_bytes(before)
        data=self.state.load();data['meta']['o']=object()
        with self.assertRaises(cs.SyncError) as c:self.state.save(data)
        self.assertEqual(c.exception.code,'state_corrupt')

    def test_cleaning_temporaries_never_runs_beside_another_writer(self):
        live=self.dir/'state.json.tmp.1.live';live.write_text('in flight')
        with self.state.hold():
            other=cs.StateFile(self.dir)
            with self.assertRaises(cs.SyncError) as c:other.clean_temporaries()
            self.assertEqual(c.exception.code,'scan_busy')
        self.assertTrue(live.exists())
        self.assertEqual(self.state.clean_temporaries(),1);self.assertFalse(live.exists())

    def test_a_missing_work_directory_is_refused(self):
        with self.assertRaises(cs.SyncError) as c:cs.StateFile(self.dir/'nope')
        self.assertEqual(c.exception.code,'state_unwritable')

    def test_one_holder_at_a_time_in_a_thread_and_in_another_process(self):
        with self.state.hold():
            with self.assertRaises(cs.SyncError) as c:self.state.hold().__enter__()
            self.assertEqual(c.exception.code,'scan_busy')
            code=("import sys;sys.path.insert(0,%r);import change_sync as cs\n"
                  "try:\n cs.StateFile(%r).hold().__enter__();print('held')\nexcept cs.SyncError as e:print(e.code)")%(str(HERE),str(self.dir))
            out=subprocess.run([sys.executable,'-c',code],capture_output=True,text=True,timeout=20).stdout.strip()
            self.assertEqual(out,'scan_busy')
        with self.state.hold():pass
        out=subprocess.run([sys.executable,'-c',code],capture_output=True,text=True,timeout=20).stdout.strip()
        self.assertEqual(out,'held')

    def test_a_waiting_hold_gets_the_lock_when_the_holder_finishes(self):
        got=[]
        def other():
            with self.state.hold(5):got.append(time.monotonic())
        with self.state.hold():
            thread=threading.Thread(target=other);thread.start();time.sleep(0.15);self.assertEqual(got,[])
            released=time.monotonic()
        thread.join(5);self.assertEqual(len(got),1);self.assertGreaterEqual(got[0],released)


class PendingTests(Base):
    def test_what_a_subscriber_has_not_confirmed(self):
        data=self.state.load();data['subs']['s']={}
        data['items']={'a':item('1'),'b':item('2'),'c':item('3')}
        data['marks']={'s':{'a':'1','b':'old','gone':'9'}}
        got=cs.pending({**data,'subs':{}} if False else data,'s')
        self.assertEqual([(k,a) for k,a,_ in got],[('b','change'),('c','create'),('gone','delete')])
        self.assertEqual([k for k,_,_ in cs.pending(data,'s',lambda k:k!='gone')],['b','c'])
        self.assertEqual(len(cs.pending(data,'nobody')),3)

    def test_reset_forgets_items_and_marks_but_keeps_subscribers_so_all_replays(self):
        data=self.state.load();data['items']={'a':item('1')};data['subs']['s']={};data['marks']={'s':{'a':'1'}};data['meta']={'x':1}
        cs.reset(data)
        self.assertEqual((data['items'],data['marks'],data['meta']),({},{'s':{}},{}))
        data['items']={'a':item('1')};self.assertEqual(len(cs.pending(data,'s')),1)


class SubscriptionTests(Base):
    def test_add_stores_a_row_and_replays_everything_by_default(self):
        self.put({'a':item('1'),'b':item('2')});subs=self.subs()
        row=subs.add(msg('#me@n.test'),'#rag@n.test',{})
        self.assertEqual((row['target'],row['created_by'],row['sent'],row['confirmed'],row['dropped']),('#rag@n.test','#me@n.test',0,0,0))
        data=self.state.load();self.assertEqual(data['marks'][row['id']],{})
        self.assertEqual(len(cs.pending(data,row['id'])),2)

    def test_replay_false_marks_what_exists_so_only_later_changes_go(self):
        self.put({'a':item('1')});subs=self.subs()
        row=subs.add(msg('#me@n.test'),'#rag@n.test',{},replay=False)
        data=self.state.load();self.assertEqual(cs.pending(data,row['id']),[])
        data['items']['b']=item('2');self.assertEqual([k for k,_,_ in cs.pending(data,row['id'])],['b'])

    def test_the_author_must_be_allowed_to_send_to_the_target_and_to_the_source(self):
        for denied,code in (('#rag@n.test','admission_refused'),(SOURCE,'admission_refused'),('#rag@n.test','admission_unavailable')):
            bus=Bus();bus.allow[('#me@n.test',denied)]=False if code=='admission_refused' else None
            if code=='admission_unavailable':bus.may_send=lambda who,target:(_ for _ in ()).throw(RuntimeError('down'))
            subs=self.subs(bus)
            with self.assertRaises(SubscriptionError,msg=denied) as c:subs.add(msg('#me@n.test'),'#rag@n.test',{})
            self.assertEqual(c.exception.code,code);self.assertEqual(self.state.load()['subs'],{})

    def test_a_target_that_is_not_a_queue_or_an_agent_is_refused(self):
        bus=Bus();bus.kinds['#rag@n.test']='service';subs=self.subs(bus)
        with self.assertRaises(SubscriptionError) as c:subs.add(msg('#me@n.test'),'#rag@n.test',{})
        self.assertEqual(c.exception.code,'invalid_target')

    def test_limits_per_creator_and_in_total(self):
        subs=self.subs(max_subscriptions=3,max_per_creator=2)
        subs.add(msg('#a@n.test'),'#t1@n.test',{});subs.add(msg('#a@n.test'),'#t2@n.test',{})
        with self.assertRaises(SubscriptionError) as c:subs.add(msg('#a@n.test'),'#t3@n.test',{})
        self.assertEqual(c.exception.code,'limit')
        subs.add(msg('#b@n.test'),'#t3@n.test',{})
        with self.assertRaises(SubscriptionError) as c:subs.add(msg('#c@n.test'),'#t4@n.test',{})
        self.assertEqual(c.exception.code,'limit');self.assertEqual(len(self.state.load()['subs']),3)

    def test_an_invalid_selector_is_refused(self):
        subs=cs.SyncSubscriptions(Bus(),SOURCE,self.state,validate_selector=lambda v:(_ for _ in ()).throw(ValueError()))
        with self.assertRaises(SubscriptionError) as c:subs.add(msg('#me@n.test'),'#rag@n.test',{})
        self.assertEqual(c.exception.code,'invalid_selector')

    def test_list_shows_own_rows_and_the_owner_sees_all_with_a_pending_count(self):
        self.put({'a':item('1')});subs=self.subs()
        mine=subs.add(msg('#me@n.test'),'#t1@n.test',{});other=subs.add(msg('#you@n.test'),'#t2@n.test',{})
        got=subs.list(msg('#me@n.test'))
        self.assertEqual([(r['id'],r['pending']) for r in got],[(mine['id'],1)])
        self.assertEqual({r['id'] for r in subs.list(msg('#boss@n.test',['owner'],))},{mine['id'],other['id']})
        self.assertEqual({r['id'] for r in subs.list(msg('#boss@n.test',['maintainer']))},{mine['id'],other['id']})
        self.assertEqual([r['id'] for r in subs.list(msg('#boss@n.test'))],[])

    def test_delete_removes_the_row_and_its_marks_for_the_author_or_a_manager_only(self):
        subs=self.subs();mine=subs.add(msg('#me@n.test'),'#t1@n.test',{});other=subs.add(msg('#you@n.test'),'#t2@n.test',{})
        with self.assertRaises(SubscriptionError) as c:subs.delete(msg('#me@n.test'),other['id'])
        self.assertEqual(c.exception.code,'not_author')
        with self.assertRaises(SubscriptionError) as c:subs.delete(msg('#me@n.test'),'0'*12)
        self.assertEqual(c.exception.code,'not_found')
        self.assertTrue(subs.delete(msg('#me@n.test'),mine['id']))
        self.assertTrue(subs.delete(msg('#boss@n.test',['owner']),other['id']))
        data=self.state.load();self.assertEqual((data['subs'],data['marks']),({},{}))

    def test_an_add_that_meets_a_running_round_waits_for_it_to_finish(self):
        subs=self.subs();got=[]
        def add():got.append(subs.add(msg('#me@n.test'),'#t1@n.test',{}))
        with self.state.hold():
            thread=threading.Thread(target=add);thread.start();time.sleep(0.15);self.assertEqual(got,[])
        thread.join(5);self.assertEqual(len(got),1)

    def test_an_add_waits_for_a_running_round_and_refuses_when_it_never_ends(self):
        subs=self.subs()
        with patch.object(cs,'HOLD_WAIT',0.1),self.state.hold():
            with self.assertRaises(SubscriptionError) as c:subs.add(msg('#me@n.test'),'#t1@n.test',{})
        self.assertEqual(c.exception.code,'busy')
        with self.state.hold():self.assertEqual(len(subs.list(msg('#me@n.test'))),0)  # a list reads, so it needs no hold

    def test_a_damaged_state_is_a_store_error_for_add_list_and_delete(self):
        subs=self.subs();self.state.path.write_text('{broken')
        for call in (lambda:subs.add(msg('#me@n.test'),'#t1@n.test',{}),lambda:subs.list(msg('#me@n.test')),lambda:subs.delete(msg('#me@n.test'),'0'*12)):
            with self.assertRaises(SubscriptionError) as c:call()
            self.assertEqual(c.exception.code,'store_unknown')


class DeliveryTests(Base):
    def setUp(self):
        super().setUp();self.put({'a':item('1'),'b':item('2')})

    def add(self,target='#q@n.test',who='#me@n.test',kind='queue',**kw):
        subs=self.subs(self.bus if hasattr(self,'bus') else None);self.bus.kinds[target]=kind
        return subs,subs.add(msg(who),target,{},**kw)

    def test_a_queue_accepting_the_send_confirms_and_nothing_is_sent_twice(self):
        subs,row=self.add()
        result=self.round()
        self.assertEqual(result[row['id']],{'sent':2,'confirmed':2,'dropped':0,'pending':0,'skipped':0})
        self.assertEqual([b for _,b,_ in self.bus.sent],['create a','create b']);self.assertEqual(self.bus.sent[0][2],{'topic':'t','tag':'x'})
        self.assertEqual(self.round()[row['id']]['sent'],0);self.assertEqual(len(self.bus.sent),2)
        stored=self.state.load()['subs'][row['id']];self.assertEqual((stored['sent'],stored['confirmed'],stored['dropped']),(2,2,0))

    def test_an_agent_must_answer_ok_before_a_change_is_done(self):
        subs,row=self.add('#rag@n.test',kind='agent')
        self.bus.replies['#rag@n.test']='{"error":{"code":"x"}}'
        r=self.round()[row['id']]
        self.assertEqual((r['sent'],r['confirmed'],r['dropped'],r['pending']),(1,0,1,2))
        self.assertEqual(len(self.bus.calls),1,'stops at the first unconfirmed change')
        self.bus.replies['#rag@n.test']='ok'
        r=self.round()[row['id']];self.assertEqual((r['confirmed'],r['pending']),(2,0))
        self.assertEqual([b for _,b,_ in self.bus.calls],['create a','create a','create b'],'the unconfirmed one was sent again')

    def test_no_answer_a_refusal_and_a_send_refusal_leave_the_change_pending(self):
        for reply,warn in (('timeout','send_unknown'),('refused','send_refused'),('not json','unconfirmed')):
            with self.subTest(reply):
                self.setUp();warnings=[];subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']=reply
                r=self.round(warning=lambda code,sub:warnings.append(code))[row['id']]
                self.assertEqual((r['confirmed'],r['pending']),(0,2));self.assertEqual(warnings,[warn])
                self.assertEqual(len(self.state.load()['marks'][row['id']]),0)
        self.setUp();subs,row=self.add();self.bus.send_refused.add('#q@n.test')
        r=self.round()[row['id']];self.assertEqual((r['confirmed'],r['pending']),(0,2))

    def test_a_done_receipt_confirms(self):
        subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']='done'
        self.assertEqual(self.round()[row['id']]['confirmed'],2)

    def test_the_author_is_checked_again_before_every_forward(self):
        subs,row=self.add();self.bus.queries.clear();self.round()
        self.assertEqual(len([q for q in self.bus.queries if q==('#me@n.test','#q@n.test')]),2,'once per change, not once per round')
        self.assertEqual(len([q for q in self.bus.queries if q==('#me@n.test',SOURCE)]),2)

    def test_a_revoked_author_sends_nothing_and_it_resumes_when_allowed_again(self):
        subs,row=self.add();self.bus.allow[('#me@n.test','#q@n.test')]=False;warnings=[]
        r=self.round(warning=lambda code,sub:warnings.append(code))[row['id']]
        self.assertEqual((r['sent'],r['dropped'],r['pending']),(0,1,2));self.assertEqual((self.bus.sent,warnings),([],['admission_refused']))
        self.bus.allow.clear();self.assertEqual(self.round()[row['id']]['confirmed'],2)
        self.bus.allow[('#me@n.test',SOURCE)]=False;self.put({'c':item('3')})
        r=self.round()[row['id']];self.assertEqual((r['sent'],r['pending']),(0,1))

    def test_an_unavailable_authority_check_never_falls_back_to_sending(self):
        subs,row=self.add();self.bus.may_send=lambda who,target:(_ for _ in ()).throw(RuntimeError('down'))
        r=self.round()[row['id']];self.assertEqual((r['sent'],r['pending'],self.bus.sent),(0,2,[]))

    def test_a_target_that_is_no_longer_what_it_was_is_not_sent_to(self):
        subs,row=self.add();self.bus.kinds['#q@n.test']='service'
        self.assertEqual((self.round()[row['id']]['sent'],self.bus.sent),(0,[]))

    def test_one_stuck_subscriber_does_not_hold_up_the_others(self):
        subs,stuck=self.add('#rag@n.test',kind='agent');_,fine=self.add('#q@n.test')
        self.bus.replies['#rag@n.test']='timeout'
        got=self.round()
        self.assertEqual((got[stuck['id']]['confirmed'],got[stuck['id']]['pending']),(0,2));self.assertEqual((got[fine['id']]['confirmed'],got[fine['id']]['pending']),(2,0))

    def test_marks_are_per_subscriber_and_a_late_subscriber_is_replayed(self):
        subs,first=self.add();self.round();_,late=self.add('#q2@n.test')
        got=self.round();self.assertEqual((got[first['id']]['sent'],got[late['id']]['confirmed']),(0,2))

    def test_at_most_batch_changes_go_per_round_in_key_order(self):
        self.put({f'k{n}':item(str(n)) for n in range(5)});subs,row=self.add()
        r=self.round(batch=3)[row['id']];self.assertEqual(([b for _,b,_ in self.bus.sent],r['pending']),(['create a','create b','create k0'],4))
        self.assertEqual(self.round(batch=3)[row['id']]['pending'],1);self.assertEqual(self.round(batch=3)[row['id']]['pending'],0)

    def test_a_changed_item_is_sent_again_and_a_removed_one_is_a_delete(self):
        subs,row=self.add();self.round();self.bus.sent.clear()
        data=self.state.load();data['items']['a']=item('1b');del data['items']['b'];self.state.save(data)
        r=self.round()[row['id']]
        self.assertEqual(([b for _,b,_ in self.bus.sent],r['pending']),(['change a','delete b'],0))
        self.assertEqual(self.state.load()['marks'][row['id']],{'a':'1b'})

    def test_a_timed_out_applied_create_must_still_get_its_delete(self):
        subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']='timeout';self.round()
        self.assertEqual(self.state.load()['marks'][row['id']],{},'never confirmed')
        data=self.state.load();del data['items']['a'];self.state.save(data);self.bus.replies['#rag@n.test']='ok';self.bus.calls.clear()
        r=self.round()[row['id']]
        self.assertIn('delete a',[b for _,b,_ in self.bus.calls]);self.assertEqual(r['pending'],0)
        self.assertEqual(self.state.load()['maybe'][row['id']].get('a'),None,'the delete cleared it')

    def test_a_batch_that_stopped_early_keeps_a_record_only_for_what_it_tried(self):
        self.put({'c':item('3')});subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']='timeout'
        self.round(batch=3);self.assertEqual(len(self.bus.calls),1)
        self.assertEqual(self.state.load()['maybe'][row['id']],{'a':'1'},'only the first was attempted: b and c were never sent')

    def test_only_the_attempted_key_gets_a_delete_when_the_batch_stopped_early(self):
        self.put({'c':item('3')});subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']='timeout';self.round(batch=3)
        data=self.state.load();[data['items'].pop(k) for k in ('a','b','c')];self.state.save(data)
        self.bus.replies['#rag@n.test']='ok';self.bus.calls.clear();self.round()
        self.assertEqual([b for _,b,_ in self.bus.calls],['delete a'])
        self.assertEqual((self.state.load()['maybe'][row['id']],self.state.load()['marks'][row['id']]),({},{}))

    def test_review_unknown_changed_then_original_fingerprint_still_pending(self):
        subs,row=self.add('#rag@n.test',kind='agent');self.round()
        data=self.state.load();data['items']['a']=item('NEW');self.state.save(data);self.bus.replies['#rag@n.test']='timeout';self.round()
        data=self.state.load();data['items']['a']=item('1');self.state.save(data)  # the source reverts to what was confirmed
        self.assertEqual(self.state.load()['marks'][row['id']]['a'],'1','the mark equals the present fingerprint')
        self.assertIn(('a','change'),[(k,a) for k,a,_ in cs.pending(self.state.load(),row['id'])],'but the target may hold NEW')
        self.bus.replies['#rag@n.test']='ok';self.bus.calls.clear();self.round()
        self.assertIn('change a',[b for _,b,_ in self.bus.calls]);self.assertEqual(self.state.load()['maybe'][row['id']],{})

    def test_two_unknown_sends_of_one_key_stay_owed_until_a_confirmation(self):
        subs,row=self.add('#rag@n.test',kind='agent');self.round()
        self.bus.replies['#rag@n.test']='timeout'
        for fp in ('X','1'):
            data=self.state.load();data['items']['a']=item(fp);self.state.save(data);self.round()
        self.assertEqual(self.state.load()['maybe'][row['id']]['a'],'?','the second unknown send does not hide the first')
        self.assertIn('a',[k for k,_,_ in cs.pending(self.state.load(),row['id'])])
        self.bus.replies['#rag@n.test']='ok';self.round();self.assertEqual(self.state.load()['maybe'][row['id']],{})

    def test_after_a_crash_mid_batch_an_extra_delete_may_go_but_never_a_lost_one(self):
        subs,row=self.add();real=self.bus.send
        def die(to,body,**o):raise SystemExit('killed')
        self.bus.send=die
        with self.assertRaises(SystemExit):self.round()
        self.assertEqual(self.state.load()['maybe'][row['id']],{'a':'1','b':'2'},'the whole batch was recorded before the first send')
        data=self.state.load();del data['items']['a'];del data['items']['b'];self.state.save(data)
        self.bus.send=real;self.round()
        self.assertEqual(sorted(b for _,b,_ in self.bus.sent),['delete a','delete b'],'b was never attempted, so its delete is the documented extra; a is not lost')

    def test_a_round_that_sent_nothing_still_saves_the_taken_back_record(self):
        subs,row=self.add();r=self.round(deadline=time.monotonic()-1)[row['id']]
        self.assertEqual((r['sent'],r['dropped']),(0,0));self.assertEqual(self.state.load()['maybe'][row['id']],{},'what was recorded for the batch was taken back on disk')

    def test_a_refusal_outright_is_not_possibly_delivered_but_a_timeout_or_a_bad_reply_is(self):
        subs,row=self.add();self.bus.send_refused.add('#q@n.test');self.round()
        self.assertEqual(self.state.load()['maybe'][row['id']],{},'refused outright: never queued')
        for reply in ('timeout','{"error":"no"}'):
            with self.subTest(reply):
                self.setUp();self.put({'a':item('1')});subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']=reply;self.round()
                self.assertEqual(self.state.load()['maybe'][row['id']],{'a':'1'})
        self.setUp();subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']='refused';self.round()
        self.assertEqual(self.state.load()['maybe'][row['id']],{},'a Refusal from the send of a call is definite too')

    def test_an_earlier_unknown_outcome_survives_a_later_refusal_of_the_same_change(self):
        subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']='timeout';self.round()
        self.assertEqual(self.state.load()['maybe'][row['id']],{'a':'1'})
        data=self.state.load();data['items']['a']=item('1b');self.state.save(data)
        self.bus.replies['#rag@n.test']='refused';self.round()
        self.assertEqual(self.state.load()['maybe'][row['id']],{'a':'1'},'the earlier send may still have been applied')

    def test_a_confirmed_create_whose_save_failed_still_gets_its_delete(self):
        subs,row=self.add();real=self.state.save;calls=[]
        def flaky(d):
            calls.append(1)
            if len(calls)==2:raise cs.SyncError('state_unwritable')
            return real(d)
        with patch.object(self.state,'save',flaky):self.round()
        self.assertEqual(len(self.bus.sent),2,'both were sent and accepted')
        data=self.state.load();self.assertEqual(data['marks'][row['id']],{},'the marks were lost with the failed save');del data['items']['a'];self.state.save(data)
        self.bus.sent.clear();self.round()
        self.assertIn('delete a',[b for _,b,_ in self.bus.sent])

    def test_the_possibly_delivered_record_is_written_before_the_send(self):
        subs,row=self.add();seen=[]
        real=self.bus.send
        def spy(to,body,**o):seen.append(dict(self.state.load()['maybe'][row['id']]));return real(to,body,**o)
        self.bus.send=spy;self.round()
        self.assertEqual(seen[0],{'a':'1','b':'2'});self.assertEqual(self.state.load()['maybe'][row['id']],{})

    def test_an_unconfirmed_delete_stays_pending(self):
        subs,row=self.add('#rag@n.test',kind='agent');self.round()
        data=self.state.load();del data['items']['a'];self.state.save(data);self.bus.replies['#rag@n.test']='timeout'
        self.assertEqual(self.round()[row['id']]['pending'],1);self.assertIn('a',self.state.load()['marks'][row['id']])

    def test_a_skip_after_an_unknown_send_does_not_loop_and_a_delete_stays_owed(self):
        subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']='timeout';self.round()
        self.assertEqual(self.state.load()['maybe'][row['id']]['a'],'1')
        data=self.state.load();data['items']['a']=item('2');self.state.save(data)
        skip=lambda sub,key,action,record:None if key=='a' else f'{action} {key}'
        self.assertEqual(self.round(body=skip)[row['id']]['skipped'],1)
        self.assertEqual(self.state.load()['maybe'][row['id']]['a'],'2','no longer owed a change, still owed a delete')
        self.assertEqual(self.round(body=skip)[row['id']]['skipped'],0,'it does not skip again on every round')
        data=self.state.load();del data['items']['a'];self.state.save(data);self.bus.replies['#rag@n.test']='ok';self.bus.calls.clear()
        self.round(body=lambda sub,key,action,record:f'{action} {key}');self.assertIn('delete a',[b for _,b,_ in self.bus.calls])

    def test_a_create_skipped_on_first_sight_leaves_no_possibly_delivered_record(self):
        subs,row=self.add();skip=lambda sub,key,action,record:None if key=='a' else f'{action} {key}'
        self.round(body=skip);self.assertNotIn('a',self.state.load()['maybe'][row['id']],'nothing was ever sent for a, so it is not possibly delivered')
        self.assertEqual(self.state.load()['marks'][row['id']]['a'],'1','but it is done: it is not skipped again every round')
        self.assertEqual(self.round(body=skip)[row['id']]['skipped'],0)

    def test_a_skipped_delete_clears_its_record_and_does_not_loop(self):
        subs,row=self.add('#rag@n.test',kind='agent');self.bus.replies['#rag@n.test']='timeout';self.round()
        data=self.state.load();del data['items']['a'];self.state.save(data)
        never=lambda sub,key,action,record:None
        self.round(body=never);self.assertEqual(self.round(body=never)[row['id']]['skipped'],0)
        self.assertNotIn('a',self.state.load()['maybe'][row['id']])

    def test_a_body_with_nothing_to_send_is_skipped_marked_done_and_does_not_block_the_rest(self):
        subs,row=self.add();body=lambda sub,key,action,record:None if key=='a' else f'{action} {key}'
        r=self.round(body=body)[row['id']]
        self.assertEqual((r['sent'],r['confirmed'],r['skipped'],r['pending']),(1,1,1,0));self.assertEqual([b for _,b,_ in self.bus.sent],['create b'])
        self.assertEqual(self.round(body=body)[row['id']]['skipped'],0,'a skipped change is done, not retried')
        self.assertEqual(self.state.load()['marks'][row['id']],{'a':'1','b':'2'})

    def test_the_matcher_decides_what_a_subscriber_sees(self):
        subs,row=self.add();self.round(matches=lambda sub,key:key=='b')
        self.assertEqual([b for _,b,_ in self.bus.sent],['create b'])

    def test_a_save_that_fails_before_the_sends_sends_nothing_and_the_next_round_does(self):
        subs,row=self.add()
        warnings=[]
        with patch.object(self.state,'save',side_effect=cs.SyncError('state_unwritable')):
            r=self.round(warning=lambda code,sub:warnings.append(code))[row['id']]
        self.assertEqual((r['unsaved'],warnings,self.bus.sent),(True,['state_unwritable'],[]),'nothing is sent whose outcome cannot be recorded')
        self.assertEqual(len(cs.pending(self.state.load(),row['id'])),2)
        self.round();self.assertEqual(len(self.bus.sent),2)

    def test_each_subscribers_confirmations_are_saved_before_the_next_is_tried(self):
        subs,first=self.add();_,second=self.add('#q2@n.test');real=self.state.save;calls=[];warnings=[]
        def flaky(data):
            calls.append(1)
            if len(calls)==3:raise cs.SyncError('state_full','state full: 1 of 1 bytes')  # the second subscriber's first save
            return real(data)
        with patch.object(self.state,'save',flaky):
            got=self.round(warning=lambda code,sub:warnings.append((code,sub)))
        self.assertEqual(len(calls),3);self.assertEqual(warnings,[('state_full',second['id'])])
        self.assertTrue(got[second['id']]['unsaved']);self.assertNotIn('unsaved',got[first['id']])
        marks=self.state.load()['marks'];self.assertEqual(len(marks[first['id']]),2,'the earlier subscriber kept its marks')
        self.assertEqual(len(marks[second['id']]),0)

    def test_a_full_state_ends_the_round_so_nothing_unkeepable_is_sent(self):
        subs,first=self.add();_,second=self.add('#q2@n.test');_,third=self.add('#q3@n.test')
        with patch.object(self.state,'save',side_effect=cs.SyncError('state_full')):
            got=self.round()
        self.assertEqual(sorted(got),[first['id']],'ended after the first failed save')
        self.assertEqual(self.bus.sent,[],'nothing was sent past a save that failed')

    def test_a_save_that_fails_after_the_sends_ends_the_round_too(self):
        subs,first=self.add();_,second=self.add('#q2@n.test');_,third=self.add('#q3@n.test');real=self.state.save;calls=[]
        def flaky(d):
            calls.append(1)
            if len(calls)==2:raise cs.SyncError('state_full')  # the first subscriber's save after its sends
            return real(d)
        with patch.object(self.state,'save',flaky):got=self.round()
        self.assertEqual((sorted(got),got[first['id']].get('unsaved')),([first['id']],True))
        self.assertEqual({to for to,_,_ in self.bus.sent},{'#q@n.test'},'the others were not tried')

    def test_a_silent_agent_cannot_hold_the_state_past_the_round_deadline(self):
        self.put({f'k{n}':item(str(n)) for n in range(6)});subs,row=self.add('#rag@n.test',kind='agent');self.bus.delay=0.2;self.bus.replies['#rag@n.test']='timeout'
        started=time.monotonic();self.round(deadline=started+0.5,confirm_wait=30)
        self.assertLess(time.monotonic()-started,1.0)
        self.assertTrue(all(t<=0.5 for t in self.bus.timeouts),self.bus.timeouts)

    def test_an_add_during_a_slow_round_gets_through(self):
        subs,row=self.add('#rag@n.test',kind='agent');self.bus.delay=0.4;self.bus.replies['#rag@n.test']='timeout'
        thread=threading.Thread(target=lambda:self.round(deadline=time.monotonic()+0.8));thread.start();time.sleep(0.1)
        added=subs.add(msg('#me@n.test'),'#q9@n.test',{});thread.join(10)
        self.assertIn(added['id'],self.state.load()['subs'])

    def test_an_expired_deadline_sends_nothing(self):
        subs,row=self.add();r=self.round(deadline=time.monotonic()-1)[row['id']]
        self.assertEqual((r['sent'],r['pending'],self.bus.sent),(0,2,[]))

    def test_a_deadline_that_passes_during_the_authority_check_sends_nothing(self):
        subs,row=self.add();real=self.bus.may_send
        def slow(who,target):time.sleep(0.15);return real(who,target)
        self.bus.may_send=slow
        r=self.round(deadline=time.monotonic()+0.2)[row['id']];self.assertEqual((r['sent'],self.bus.sent),(0,[]))

    def test_a_round_of_refusals_writes_twice_not_per_attempt_and_keeps_the_counters(self):
        subs,row=self.add();self.bus.send_refused.add('#q@n.test');real=self.state.save;saves=[]
        with patch.object(self.state,'save',lambda d:(saves.append(1),real(d))[1]):self.round()
        self.assertEqual(len(saves),2,'the record written before the sends, and its taking back with the counters');stored=self.state.load()['subs'][row['id']];self.assertEqual((stored['sent'],stored['dropped'],stored['confirmed']),(1,1,0))

    def test_marks_are_saved_per_confirming_subscriber_and_a_later_subscribers_counters_in_the_last_save(self):
        subs,first=self.add();_,second=self.add('#q2@n.test');self.bus.send_refused.add('#q2@n.test');real=self.state.save;saves=[]
        with patch.object(self.state,'save',lambda d:(saves.append(1),real(d))[1]):self.round()
        self.assertEqual(len(saves),4,'two per subscriber: before the sends and after');stored=self.state.load()['subs'];self.assertEqual((stored[first['id']]['confirmed'],stored[second['id']]['dropped']),(2,1))

    def test_nothing_is_written_when_nothing_was_tried(self):
        subs,row=self.add();self.round();before=self.state.path.stat().st_mtime_ns
        with patch.object(self.state,'save',side_effect=AssertionError('saved')):self.round()
        self.assertEqual(self.state.path.stat().st_mtime_ns,before)

    def test_a_round_refuses_while_another_holds_the_state(self):
        self.add()
        with self.state.hold():
            with self.assertRaises(cs.SyncError) as c:self.round()
        self.assertEqual(c.exception.code,'scan_busy');self.assertEqual(self.bus.sent,[])

    def test_the_reply_that_counts_as_ok(self):
        r=lambda body:CallReply(SimpleNamespace(body=body))
        self.assertTrue(cs.reply_ok(CallDone(SimpleNamespace(body=''))));self.assertTrue(cs.reply_ok(r('{"status":"ok","indexed":1}')))
        for bad in ('{"status":"error"}','{"status":"ok","error":{}}','{}','[]','ok','','null'):self.assertFalse(cs.reply_ok(r(bad)),bad)
        self.assertFalse(cs.reply_ok(None));self.assertFalse(cs.reply_ok('ok'))


class TimerTests(unittest.TestCase):
    def test_it_runs_at_start_and_then_on_the_interval(self):
        hits=[];timer=cs.PollTimer(0.03,lambda:hits.append(time.monotonic()));timer.start()
        time.sleep(0.25);self.assertTrue(timer.stop());self.assertGreaterEqual(len(hits),3)
        stamp=len(hits);time.sleep(0.1);self.assertEqual(len(hits),stamp,'stopped')

    def test_the_first_run_is_at_once_not_after_an_interval(self):
        hits=[];timer=cs.PollTimer(30,lambda:hits.append(1));timer.start();time.sleep(0.2)
        self.assertEqual(hits,[1]);self.assertTrue(timer.stop())

    def test_no_run_at_start_when_not_asked_for(self):
        hits=[];timer=cs.PollTimer(30,lambda:hits.append(1),immediate=False);timer.start();time.sleep(0.1)
        self.assertEqual(hits,[]);timer.trigger();time.sleep(0.15);self.assertEqual(hits,[1]);self.assertTrue(timer.stop())

    def test_runs_never_overlap_even_when_triggered_while_one_is_going(self):
        live=[];peak=[0];gate=threading.Event()
        def run():
            live.append(1);peak[0]=max(peak[0],len(live));time.sleep(0.05);live.pop()
        timer=cs.PollTimer(0.01,run);timer.start()
        for _ in range(20):timer.trigger();time.sleep(0.01)
        self.assertTrue(timer.stop());self.assertEqual(peak[0],1)

    def test_the_wait_starts_after_a_run_ends_so_there_is_no_catch_up_burst(self):
        spans=[]
        def run():
            start=time.monotonic();time.sleep(0.08);spans.append((start,time.monotonic()))
        timer=cs.PollTimer(0.05,run);timer.start();time.sleep(0.45);self.assertTrue(timer.stop())
        self.assertGreaterEqual(len(spans),2)
        for (_,end),(start,_) in zip(spans,spans[1:]):self.assertGreaterEqual(start-end,0.04)

    def test_a_failing_run_is_reported_and_the_timer_keeps_going(self):
        errors=[];n=[0]
        def run():
            n[0]+=1
            if n[0]==1:raise RuntimeError('boom')
        timer=cs.PollTimer(0.02,run,on_error=errors.append);timer.start();time.sleep(0.2);self.assertTrue(timer.stop())
        self.assertEqual([str(e) for e in errors],['boom']);self.assertGreaterEqual(n[0],3)

    def test_a_failing_error_handler_does_not_end_the_timer(self):
        n=[0]
        def run():n[0]+=1;raise RuntimeError('x')
        timer=cs.PollTimer(0.02,run,on_error=lambda e:1/0);timer.start();time.sleep(0.15);self.assertTrue(timer.stop());self.assertGreaterEqual(n[0],3)

    def test_stop_says_when_a_run_would_not_end(self):
        release=threading.Event();timer=cs.PollTimer(0.01,lambda:release.wait(5));timer.start();time.sleep(0.05)
        self.assertFalse(timer.stop(timeout=0.1));release.set();self.assertTrue(timer.stop())

    def test_a_bad_interval_and_a_second_start_are_refused(self):
        for bad in (0,-1,float('nan'),float('inf'),True,'5',None):
            with self.assertRaises(ValueError,msg=repr(bad)):cs.PollTimer(bad,lambda:None)
        timer=cs.PollTimer(30,lambda:None);timer.start()
        with self.assertRaises(RuntimeError):timer.start()
        self.assertTrue(timer.stop())


if __name__=='__main__':unittest.main()
