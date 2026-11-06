"""On-demand sync (Plans/R1.1/filescan.md#on-demand-sync) against a scripted RAG: nothing paid, a fake clock, no sleeps."""
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
import threading
import time
from types import SimpleNamespace
import unittest

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE));sys.path.insert(0,str(HERE.parent/'shared'));sys.path.insert(0,str(HERE.parents[1]/'clients/python'))
import filescan as fs
from agent_bus.errors import BusTimeout, Refusal
from agent_bus.message import CallReply

NAME='#filescan/vault@n.test'
RAG='#rag@n.test'


class Clock:
    """A monotonic clock that moves only when the fake RAG is called."""
    def __init__(self):self.now=1000.0;self.step=0.0
    def __call__(self):return self.now


class FakeRag:
    """Stands in for the bus with a RAG behind it: stores records by (namespace, table, id), answers like RAG.

    `lose` lists paths whose upsert is stored but whose answer is lost (a timeout); `refuse` and `error` script failures;
    `incomplete_delete` makes a delete answer RAG's `incomplete:`; `partial` ids answer the check `complete: false`; `store` false
    stores nothing on a lost answer."""
    name=NAME
    def __init__(self,clock):
        self.clock=clock;self.records={};self.calls=[];self.lose=set();self.refuse=set();self.error=set();self.incomplete_delete=set();self.partial=set();self.check_error=False;self.store=True
        self.allow={};self.kinds={RAG:'agent'};self.bigfiles={};self.sends=[];self.ttls=[];self.reject_large=set()
    def may_send(self,who,target):return self.allow.get((who,target),True)
    def record(self,name):
        def put(sha,reader,*,size,ttl,sha256):
            data=reader.read();assert hashlib.sha256(data).hexdigest()==sha256==sha and len(data)==size
            self.bigfiles[sha]=data;self.ttls.append(ttl);return {'expires':'2026-11-06T01:00:00Z'}
        def delete(sha):self.bigfiles.pop(sha,None)
        return SimpleNamespace(lookup=lambda:{'name':name,'kind':self.kinds.get(name,'queue')},bigfile=SimpleNamespace(put=put,delete=delete))
    def send(self,to,body,**options):
        # Only a large upsert is sent without waiting: a bigfile reference with its own deadline. RAG stores it later, unless it
        # refuses it (`reject_large`), which the sender never hears about.
        call=json.loads(body);assert to==RAG and call['method']=='upsert' and 'bigfile' in call and 'data' not in call,call
        assert options.get('wait')==fs.LARGE_WAIT and options.get('ttl',0)>=fs.LARGE_WAIT,options
        self.calls.append(call);self.sends.append(options)
        reference=call['bigfile'];assert self.bigfiles.get(reference['name']) is not None,'the reference names an uploaded bigfile'
        assert hashlib.sha256(self.bigfiles[reference['name']]).hexdigest()==call['content_hash']==reference['sha256']
        if call['source'] not in self.reject_large:
            self.records[(call['namespace'],call['tableName'],call['id'])]={k:call[k] for k in ('namespace','tableName','id','source','content_hash')}
    def upserts(self,path=None):return [c for c in self.calls if c['method']=='upsert' and (path is None or c['source']==path)]
    def call(self,to,body,*,timeout,**options):
        call=json.loads(body);self.calls.append(call);self.clock.now+=self.clock.step
        assert timeout<=fs.SYNC_WAIT,timeout
        key=(call['namespace'],call['tableName'],call['id']);path=call.get('source') or call['id']
        if path in self.refuse:raise Refusal(403,'refused')
        if path in self.error:return CallReply(SimpleNamespace(body='error: not stored; prior record unchanged: scripted'))
        if call['method']=='upsert':
            record={k:call[k] for k in ('namespace','tableName','id','source','content_hash')}
            if path in self.lose:
                if self.store:self.records[key]=record
                raise BusTimeout()
            self.records[key]=record;return CallReply(SimpleNamespace(body=json.dumps(record)))
        if call['method']=='get':
            # The frozen completeness check: seven fields, never data (large-files.md#the-completeness-check).
            assert call.get('check') is True,call
            if self.check_error:return CallReply(SimpleNamespace(body='error: storage request failed'))
            if key not in self.records:return CallReply(SimpleNamespace(body='error: not_found: record is absent from this namespace'))
            record=self.records[key]
            return CallReply(SimpleNamespace(body=json.dumps({**record,'indexed_at':'2026-11-06T00:00:00Z','complete':record['id'] not in self.partial})))
        if call['method']=='delete':
            if path in self.incomplete_delete:return CallReply(SimpleNamespace(body='incomplete: repeat the same delete to continue'))
            if key not in self.records:return CallReply(SimpleNamespace(body='error: not_found: no such record'))
            del self.records[key];return CallReply(SimpleNamespace(body=json.dumps({'status':'ok'})))
        raise AssertionError(call)


def msg(body,roles=('write',)):
    return SimpleNamespace(envelope={'from':'#me@n.test','roles':list(roles),'body':body},body=body,ask=None)


class SyncBase(unittest.TestCase):
    CONFIG={'mode':'demand','namespace':'vault','table':'notes'}

    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        base=os.path.realpath(self.tmp.name);self.root=os.path.join(base,'vault');self.work=os.path.join(base,'work')
        os.makedirs(self.root);os.makedirs(self.work)
        self.clock=Clock();self.wall=Clock();self.wall.now=time.time();self.bus=FakeRag(self.clock);self.agent=self.make({})

    def make(self,extra):
        return fs.FileScan(self.bus,{**self.CONFIG,**extra},self.root,self.work,clock=self.wall,monotonic=self.clock,warning=lambda *a:None)

    def put(self,rel,data='x'):
        p=Path(self.root)/rel;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(data)

    def sync(self,extra=''):
        return json.loads(self.agent.handle(msg(f'sync {RAG} {extra}'.strip())))


class Demand(SyncBase):
    def test_every_file_confirmed_is_success_and_a_rerun_sends_nothing(self):
        for n in range(3):self.put(f'n{n}.md',f'note {n}')
        r=self.sync()
        self.assertEqual((r['result'],r['confirmed'],r['remaining'],r['sent'],r['files']),('confirmed',3,0,3,[]))
        self.assertEqual(len(self.bus.upserts()),3)
        again=self.sync()
        self.assertEqual((again['result'],again['sent']),('confirmed',0));self.assertEqual(len(self.bus.upserts()),3,'an unchanged confirmed file is not sent again')

    def test_a_changed_file_is_sent_again_with_its_new_hash(self):
        self.put('a.md','one');self.sync();self.put('a.md','two')
        r=self.sync();self.assertEqual(r['result'],'confirmed')
        (first,second)=self.bus.upserts('a.md');self.assertNotEqual(first['content_hash'],second['content_hash'])

    def test_sync_runs_only_on_a_demand_instance_and_demand_takes_no_subscriptions(self):
        self.agent=fs.FileScan(self.bus,{'namespace':'vault','table':'notes'},self.root,self.work,monotonic=self.clock,warning=lambda *a:None)
        self.assertEqual(self.sync()['error']['code'],'not_on_demand')
        self.agent=self.make({})
        r=json.loads(self.agent.handle(msg(f'sub add {RAG} as rag vault notes')))
        self.assertEqual(r['error']['code'],'on_demand')

    def test_nothing_is_left_running_after_a_sync(self):
        threads=threading.active_count();self.put('a.md','one')
        self.assertFalse(self.agent.start_watch());self.assertFalse(self.agent.needs_timer())
        self.assertEqual(self.sync()['result'],'confirmed')
        self.assertIsNone(self.agent.watcher);self.assertEqual(threading.active_count(),threads)
        state=self.agent.state.load();self.assertEqual(state['subs'],{});self.assertEqual(self.agent.effective_mode(),'demand')

    def test_a_concurrent_sync_is_refused_as_busy(self):
        self.put('a.md')
        with self.agent.sync_lock:self.assertEqual(self.sync()['error']['code'],'busy')
        with self.agent.state.hold():self.assertEqual(self.sync()['error']['code'],'busy')
        self.assertEqual(self.bus.calls,[],'a refused sync sends nothing')

    def test_reset_clears_the_sync_marks_so_everything_is_sent_again(self):
        self.put('a.md','one');self.sync()
        self.assertEqual(json.loads(self.agent.handle(msg('reset confirm:true',roles=('owner',))))['reset'],True)
        self.assertEqual(self.sync()['sent'],1);self.assertEqual(len(self.bus.upserts('a.md')),2)

    def test_marks_are_per_target_namespace_and_table(self):
        self.put('a.md','one');self.sync();r=self.sync('as rag vault other')
        self.assertEqual((r['result'],r['sent']),('confirmed',1),'another table has its own marks')

    def test_a_file_changed_after_the_walk_is_the_next_runs(self):
        self.put('a.md','one');scan=self.agent.scan
        def edited(*a,**k):
            out=scan(*a,**k);self.put('a.md','two, longer');return out
        self.agent.scan=edited
        r=self.sync();self.assertEqual((r['result'],r['remaining']),('incomplete',1));self.assertEqual(self.bus.upserts(),[])
        self.agent.scan=scan;r=self.sync();self.assertEqual(r['result'],'confirmed')
        (sent,)=self.bus.upserts();self.assertEqual(sent['data'],'two, longer')


class Bounds(SyncBase):
    def test_a_capped_run_is_incomplete_with_counts_never_success(self):
        self.agent=self.make({'max_per_scan':2})
        for n in range(5):self.put(f'n{n}.md',f'note {n}')
        r=self.sync()
        self.assertEqual(r['result'],'incomplete');self.assertEqual((r['confirmed'],r['remaining']),(2,3));self.assertEqual(r['sent'],2)
        r=self.sync();self.assertEqual((r['result'],r['confirmed'],r['remaining']),('incomplete',4,1))
        r=self.sync();self.assertEqual((r['result'],r['confirmed'],r['remaining']),('confirmed',5,0))
        self.assertEqual(len(self.bus.upserts()),5,'each file once: a rerun resumes, never starts over')

    def test_the_call_ends_inside_its_bound_and_resumes(self):
        self.clock.step=7.0
        for n in range(5):self.put(f'n{n}.md',f'note {n}')
        r=self.sync()
        self.assertEqual(r['result'],'incomplete');self.assertLess(r['sent'],5);self.assertEqual(r['remaining'],5-r['confirmed'])
        self.assertLessEqual(r['sent']*self.clock.step,fs.SYNC_BOUND+self.clock.step)
        self.clock.step=0.0;self.assertEqual(self.sync()['result'],'confirmed');self.assertEqual(len(self.bus.upserts()),5)

    def test_failed_outranks_capped_and_names_each_file(self):
        self.agent=self.make({'max_per_scan':1})
        self.put('a.md','one');self.put('big.md','x'*(fs.RAG_CAP+1));self.put('c.md','three');self.put('d.md','four')
        self.bus.error.add('c.md')
        r=self.sync();self.sync();r=self.sync()
        self.assertEqual(r['result'],'failed')
        self.assertIn({'path':'big.md','state':'failed'},[{k:f[k] for k in ('path','state')} for f in r['files']])
        self.assertIn('cap for RAG',[f for f in r['files'] if f['path']=='big.md'][0]['reason'])

    def test_a_refusal_fails_the_file(self):
        self.put('a.md','one');self.bus.refuse.add('a.md')
        r=self.sync();self.assertEqual(r['result'],'failed');self.assertEqual(r['files'][0]['state'],'failed')


class Unknown(SyncBase):
    def test_a_lost_answer_is_unknown_never_success(self):
        self.put('a.md','one');self.bus.lose.add('a.md')
        r=self.sync()
        self.assertEqual(r['result'],'unknown');self.assertEqual([(f['path'],f['state']) for f in r['files']],[('a.md','unknown')])

    def test_a_complete_check_settles_an_unknown_file_with_no_second_upsert(self):
        self.put('a.md','one');self.bus.lose.add('a.md');self.sync();self.bus.lose.clear()
        r=self.sync();self.assertEqual(r['result'],'confirmed')
        self.assertEqual(len(self.bus.upserts('a.md')),1,'RAG holds it whole: no paid resend')
        self.assertEqual([c['method'] for c in self.bus.calls],['upsert','get'])

    def test_an_unknown_file_is_never_resent_without_resend_true(self):
        self.bus.store=False;self.put('a.md','one');self.bus.lose.add('a.md');self.sync();self.bus.lose.clear()
        for _ in range(2):
            r=self.sync();self.assertEqual(r['result'],'unknown');self.assertIn('resend:true',r['files'][0]['reason'])
        self.assertEqual(len(self.bus.upserts('a.md')),1)
        r=self.sync('resend:true');self.assertEqual(r['result'],'confirmed');self.assertEqual(len(self.bus.upserts('a.md')),2)


class Removal(SyncBase):
    def test_a_file_gone_from_the_folder_is_removed_and_done_only_when_rag_confirms(self):
        for n in range(9):self.put(f'keep{n}.md',f'k{n}')
        self.put('a.md','one');self.sync();os.unlink(Path(self.root)/'a.md')
        self.bus.incomplete_delete.add('a.md')
        r=self.sync();self.assertEqual((r['result'],r['remaining']),('incomplete',1))
        self.assertEqual(self.sync()['result'],'incomplete','an unconfirmed removal is never done')
        self.bus.incomplete_delete.clear();r=self.sync()
        self.assertEqual((r['result'],r['remaining']),('confirmed',0));self.assertNotIn(('vault','notes','a.md'),self.bus.records)
        self.assertEqual(self.sync()['sent'],0,'a confirmed removal is not sent again')

    def test_not_found_counts_as_removed(self):
        for n in range(9):self.put(f'keep{n}.md',f'k{n}')
        self.put('a.md','one');self.sync();del self.bus.records[('vault','notes','a.md')];os.unlink(Path(self.root)/'a.md')
        self.assertEqual(self.sync()['result'],'confirmed')

    def test_the_mass_delete_guard_holds_a_large_removal(self):
        for n in range(5):self.put(f'n{n}.md',f'n{n}')
        self.sync();calls=len(self.bus.calls)
        for n in range(5):os.unlink(Path(self.root)/f'n{n}.md')
        r=self.sync();self.assertEqual(r['error']['code'],'paused')
        self.assertEqual(len(self.bus.calls),calls,'no removal is sent while the guard holds');self.assertEqual(len(self.bus.records),5)


class WatchResend(SyncBase):
    """The standing feed (`sub add … as rag`): an upsert whose answer was lost goes again only after RAG's check shows the file is not
    stored whole as this version; a refusal goes again without asking."""
    CONFIG={'mode':'poll','namespace':'vault','table':'notes'}

    def setUp(self):
        super().setUp()
        r=json.loads(self.agent.handle(msg(f'sub add {RAG} as rag vault notes')));self.assertNotIn('error',r,r)

    def scan(self):self.assertNotIn('error',json.loads(self.agent.handle(msg('scan'))))

    def test_a_lost_answer_then_a_complete_check_sends_no_second_upsert(self):
        self.put('a.md','one');self.bus.lose.add('a.md');self.scan();self.bus.lose.clear()
        for _ in range(3):self.scan()
        self.assertEqual(len(self.bus.upserts('a.md')),1,'RAG holds it whole: no paid resend')
        self.assertEqual([c['method'] for c in self.bus.calls],['upsert','get'])
        self.assertEqual(json.loads(self.agent.handle(msg('status')))['subscribers'][0]['pending'],0)

    def test_a_file_edited_after_a_lost_answer_goes_once_more_with_its_new_hash(self):
        self.put('a.md','one');self.bus.lose.add('a.md');self.scan();self.bus.lose.clear()
        self.put('a.md','two, edited');self.scan();self.scan()
        (old,new)=self.bus.upserts('a.md')
        self.assertNotEqual(old['content_hash'],new['content_hash']);self.assertEqual(new['data'],'two, edited')

    def test_a_check_that_proves_nothing_waits_and_the_other_files_go_on(self):
        self.put('a.md','one');self.bus.lose.add('a.md');self.scan();self.bus.lose.clear();self.bus.check_error=True
        self.put('b.md','two');self.scan();self.scan()
        self.assertEqual(len(self.bus.upserts('a.md')),1,'an unknown check is never grounds for a paid resend')
        self.assertEqual(len(self.bus.upserts('b.md')),1,'one unknown file never holds up the others')
        self.bus.check_error=False;self.scan();self.assertEqual(len(self.bus.upserts('a.md')),1)

    def test_a_check_showing_it_absent_or_partial_allows_one_resend(self):
        self.bus.store=False;self.put('a.md','one');self.bus.lose.add('a.md');self.scan();self.bus.lose.clear()
        self.scan();self.assertEqual(len(self.bus.upserts('a.md')),2)
        self.bus.store=True;self.put('b.md','two');self.bus.lose.add('b.md');self.scan();self.bus.lose.clear();self.bus.partial.add('b.md')
        self.scan();self.assertEqual(len(self.bus.upserts('b.md')),2,'complete:false confirms nothing: sent again')

    def test_check_driven_resends_of_one_version_stop_at_the_bound_and_status_names_the_file(self):
        self.bus.store=False;self.bus.lose.add('a.md');self.put('a.md','one');self.put('b.md','two')
        for _ in range(fs.RESEND_MAX+4):self.scan()
        self.assertEqual(len(self.bus.upserts('a.md')),1+fs.RESEND_MAX,'each resend pays RAG again: bounded per file')
        self.assertEqual(len(self.bus.upserts('b.md')),1,'the bound is per file: b was stored')
        named=json.loads(self.agent.handle(msg('status')))['skipped_for_rag']
        self.assertIn('never stored it whole',next(iter(named.values()))['a.md'])
        self.put('a.md','one, edited');self.scan();self.assertEqual(len(self.bus.upserts('a.md')),2+fs.RESEND_MAX,'a new version starts again')

    def test_a_refusal_goes_again_without_a_check(self):
        self.put('a.md','one');self.bus.refuse.add('a.md');self.scan();self.bus.refuse.clear();self.scan()
        self.assertEqual([c['method'] for c in self.bus.calls],['upsert','upsert'])


if __name__=='__main__':
    unittest.main()


LARGE='# big\n'+'word '*8000   # over RAG's 32 KiB inline limit, within its 1 MiB cap


class LargeSync(SyncBase):
    """A file over 32 KiB goes to RAG as a bigfile reference with its own deadline. Inside that deadline only RAG's free check is
    asked, whatever it answers; after it, a check that does not prove the file stored is unknown, and only `resend:true` sends it."""
    def test_a_large_file_is_in_flight_then_confirmed_by_the_check_alone(self):
        self.put('big.md',LARGE)
        r=self.sync()
        self.assertEqual((r['result'],r['sent'],[f['state'] for f in r['files']]),('incomplete',1,['in_flight']))
        (up,)=self.bus.upserts('big.md');self.assertEqual(up['content_hash'],hashlib.sha256(LARGE.encode()).hexdigest())
        self.assertEqual(up['bigfile']['size'],len(LARGE));self.assertEqual(up['mime-type'],'text/markdown')
        self.assertGreater(self.bus.ttls[0],fs.LARGE_WAIT,'the key outlives the upsert\'s own deadline')
        again=self.sync()
        self.assertEqual((again['result'],again['confirmed'],again['sent']),('confirmed',1,0))
        self.assertEqual(len(self.bus.upserts('big.md')),1,'a complete check confirms it with no second upsert')

    def test_inside_its_deadline_a_refused_large_file_is_never_sent_again_even_with_resend(self):
        self.put('big.md',LARGE);self.bus.reject_large.add('big.md');self.sync()
        for extra in ('','resend:true'):
            r=self.sync(extra)
            self.assertEqual((r['result'],[f['state'] for f in r['files']]),('incomplete',['in_flight']))
        self.assertEqual(len(self.bus.upserts('big.md')),1)
        self.wall.now+=fs.LARGE_WAIT+1
        r=self.sync();self.assertEqual((r['result'],[f['state'] for f in r['files']]),('unknown',['unknown']))
        self.assertEqual(len(self.bus.upserts('big.md')),1,'after the deadline only the caller resends')
        self.bus.reject_large.clear();self.sync('resend:true');self.assertEqual(len(self.bus.upserts('big.md')),2)
        self.assertEqual(self.sync()['result'],'confirmed')

    def test_a_file_over_rags_cap_or_a_large_file_that_is_not_markdown_fails(self):
        for name,data in (('huge.md','x'*(fs.RAG_CAP+1)),('big.txt',LARGE)):
            with self.subTest(name):
                self.setUp();self.put(name,data)
                r=self.sync();self.assertEqual((r['result'],r['sent'],r['files'][0]['state']),('failed',0,'failed'))
                self.assertEqual(self.bus.calls,[],'nothing is sent')


class LargeWatch(SyncBase):
    CONFIG={'mode':'poll','namespace':'vault','table':'notes'}

    def setUp(self):
        super().setUp()
        r=json.loads(self.agent.handle(msg(f'sub add {RAG} as rag vault notes')));self.assertNotIn('error',r,r)

    def scan(self):self.assertNotIn('error',json.loads(self.agent.handle(msg('scan'))))
    def pending(self):return json.loads(self.agent.handle(msg('status')))['subscribers'][0]['pending']

    def test_a_large_file_stored_by_rag_is_confirmed_by_the_check_with_no_second_upsert(self):
        self.put('big.md',LARGE);self.scan();self.assertEqual(self.pending(),1,'in flight shows as pending')
        self.scan();self.assertEqual(self.pending(),0)
        self.assertEqual([c['method'] for c in self.bus.calls],['upsert','get'])

    def test_a_refused_large_file_is_only_checked_until_its_deadline_then_sent_again(self):
        self.put('big.md',LARGE);self.bus.reject_large.add('big.md');self.scan()
        for _ in range(3):self.scan()
        self.assertEqual(len(self.bus.upserts('big.md')),1,'never sent again inside its deadline');self.assertEqual(self.pending(),1)
        self.bus.reject_large.clear();self.wall.now+=fs.LARGE_WAIT+1;self.scan()
        self.assertEqual(len(self.bus.upserts('big.md')),2,'after it, a check that does not prove it stored allows the resend')
        self.scan();self.assertEqual(self.pending(),0)

    def test_a_large_file_that_is_not_markdown_is_skipped_for_rag_and_named(self):
        self.put('big.txt',LARGE);self.scan()
        self.assertEqual(self.bus.upserts(),[]);self.assertEqual(self.bus.sends,[])
        named=json.loads(self.agent.handle(msg('status')))['skipped_for_rag']
        self.assertIn('not Markdown',next(iter(named.values()))['big.txt'])

    def test_a_large_file_edited_in_flight_goes_at_once_with_its_new_hash(self):
        self.put('big.md',LARGE);self.bus.reject_large.add('big.md');self.scan()
        self.put('big.md',LARGE+'more');self.scan()
        (old,new)=self.bus.upserts('big.md');self.assertNotEqual(old['content_hash'],new['content_hash'])
