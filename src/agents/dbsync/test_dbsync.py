"""DBSync against a fake `#db` agent: sqlite answers the statements the Builder makes, in the DB agent's reply shape."""
import json
from pathlib import Path
import sqlite3
import sys
import tempfile
from types import SimpleNamespace
import unittest

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
sys.path.insert(0,str(HERE.parent/'shared'))
import dbsync
import settings as cfg
import statements
from agent_bus.errors import Refusal
from agent_bus.message import CallReply

DB='#db/orders@n.test'
NAME='#dbsync/orders@n.test'
QUEUE='orders-feed'
AUTHOR='#reader@n.test'


class FakeDB:
    """The DB agent: runs a SELECT on sqlite, cuts the answer at max_rows and says so, answers types from a table."""
    def __init__(self,max_rows=10,types=None):
        self.conn=sqlite3.connect(':memory:',check_same_thread=False)
        self.conn.execute('CREATE TABLE "orders" ("id" INTEGER PRIMARY KEY, "updated_at" TEXT, "note" TEXT)')
        self.max_rows=max_rows;self.queries=[];self.fail=None
        self.types=types or {'id':'bigint','updated_at':'timestamp'}

    def put(self,key,updated,note='n'):
        self.conn.execute('INSERT OR REPLACE INTO "orders" VALUES (?,?,?)',(key,updated,note));self.conn.commit()

    def drop(self,key):
        self.conn.execute('DELETE FROM "orders" WHERE "id"=?',(key,));self.conn.commit()

    def answer(self,body):
        assert body.startswith('query ');sql=body[6:];self.queries.append(sql)
        if self.fail:return self.fail
        if 'information_schema' in sql:
            rows=[[c,t] for c,t in self.types.items() if f"'{c}'" in sql]
            return json.dumps({'columns':['column_name','data_type'],'rows':rows,'truncated':False})
        cursor=self.conn.execute(sql);rows=[list(r) for r in cursor.fetchall()]
        truncated=len(rows)>self.max_rows
        return json.dumps({'columns':[d[0] for d in cursor.description],'rows':rows[:self.max_rows],'truncated':truncated})


class Bus:
    def __init__(self,db):
        self.name=NAME;self.db=db;self.sent=[];self.deny=set()
    def may_send(self,who,target):return (who,target) not in self.deny
    def record(self,name):return SimpleNamespace(lookup=lambda:{'name':name,'kind':'queue'})
    def send(self,to,body,**options):self.sent.append((to,json.loads(body)));return 'id'
    def call(self,to,body,*,timeout,**options):
        if to!=DB:raise Refusal(404,'absent')
        return CallReply(SimpleNamespace(body=self.db.answer(body)))


def msg(body,who=AUTHOR,roles=('read',)):
    return SimpleNamespace(envelope={'from':who,'roles':list(roles)},ask=None,body=body)


class Base(unittest.TestCase):
    config={'db':DB,'engine':'postgres','table':'orders','key':'id','updated':'updated_at','columns':['note']}

    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.db=FakeDB();self.bus=Bus(self.db);self.warnings=[];self.now=[1000.0]
        self.agent=self.make()

    def make(self,**extra):
        config=dict(self.config,**extra)
        return dbsync.DBSync(self.bus,config,self.tmp.name,clock=lambda:self.now[0],warning=lambda c,s=None:self.warnings.append((c,s)))

    def subscribe(self,**kw):
        reply=json.loads(self.agent.handle(msg(f'sub add {QUEUE}',**kw)))
        self.assertIn('id',reply,reply);return reply['id']

    def keys(self,change=None):
        return [m['key'] for _,m in self.bus.sent if change is None or m['change']==change]


class StatementTests(unittest.TestCase):
    def test_names_are_quoted_for_their_dialect_and_anything_else_is_refused(self):
        self.assertEqual(statements.quote('mysql','orders'),'`orders`')
        self.assertEqual(statements.quote('postgres','orders'),'"orders"')
        statements.quote('postgres','a'*63)
        for bad in ('orders;drop','a b','"x"','1abc','','a'*64):
            with self.assertRaises(statements.StatementError):statements.quote('postgres',bad)

    def test_a_cursor_value_reaches_a_statement_only_in_its_strict_form(self):
        self.assertEqual(statements.literal('numeric','42'),'42')
        self.assertEqual(statements.literal('timestamp','2026-11-05 10:00:00'),"'2026-11-05 10:00:00'")
        for kind,bad in (('numeric',"1 OR 1=1"),('timestamp',"2026-11-05' OR '1'='1"),('numeric',True),('timestamp',None)):
            with self.assertRaises(statements.StatementError):statements.literal(kind,bad)

    def test_a_page_reads_after_the_pair_in_updated_then_key_order(self):
        b=statements.Builder('postgres','orders','id','updated_at',['note']);b.kinds={'key':'numeric','updated':'timestamp'}
        self.assertEqual(b.page(('2026-11-05 10:00:00','7'),9),
            'SELECT "id", "updated_at", "note" FROM "orders" WHERE "updated_at" IS NOT NULL AND ("updated_at" > \'2026-11-05 10:00:00\''
            ' OR ("updated_at" = \'2026-11-05 10:00:00\' AND "id" > 7)) ORDER BY "updated_at", "id" LIMIT 9')


class SettingsTests(unittest.TestCase):
    def test_an_unknown_or_missing_setting_is_refused_by_name(self):
        base={'db':DB,'engine':'mysql','table':'t','key':'id','updated':'u'}
        with self.assertRaisesRegex(cfg.SettingsError,'^password: unknown'):cfg.parse(dict(base,password='x'))
        with self.assertRaisesRegex(cfg.SettingsError,'^updated: required'):cfg.parse({k:v for k,v in base.items() if k!='updated'})
        self.assertEqual(cfg.parse(base)['max_rows'],1000)

    def test_a_page_leaves_a_tenth_of_max_rows_so_the_db_agent_cut_is_never_the_page(self):
        self.assertEqual((cfg.page_size(1000),cfg.page_size(10),cfg.page_size(2),cfg.page_size(1)),(900,9,1,0))


class TypeGuardTests(Base):
    def test_a_cursor_column_that_is_not_numeric_or_timestamp_is_named(self):
        self.db.types={'id':'text','updated_at':'timestamp'}
        with self.assertRaisesRegex(dbsync.CannotSync,r'^cannot sync: 1 of 2 cursor columns .*key "id" is text'):self.agent.check_types()

    def test_a_numeric_updated_column_needs_a_unit_and_a_missing_column_is_named(self):
        self.db.types={'updated_at':'bigint'}
        with self.assertRaisesRegex(dbsync.CannotSync,r'2 of 2 .*key "id" is missing; updated "updated_at" is bigint with no updated_unit'):
            self.agent.check_types()
        self.db.types={'id':'int','updated_at':'bigint'}
        self.assertEqual(self.make(updated_unit='ms').check_types(),{'key':'numeric','updated':'numeric'})


class PollTests(Base):
    def test_every_row_goes_once_then_a_row_that_changes_goes_again_at_its_new_state(self):
        for k in range(1,6):self.db.put(k,f'2026-11-05 10:00:0{k}')
        self.subscribe()
        self.agent.poll()
        self.assertEqual(sorted(self.keys('changed'),key=int),['1','2','3','4','5'])
        self.assertEqual(self.bus.sent[0][1]['row'],{'id':1,'updated_at':'2026-11-05 10:00:01','note':'n'})
        self.agent.poll()
        self.assertEqual(len(self.bus.sent),5)   # nothing new: nothing sent again
        self.db.put(3,'2026-11-05 11:00:00','new')
        self.agent.poll()
        self.assertEqual(self.keys()[5:],['3'])
        self.assertEqual((self.bus.sent[-1][1]['row']['note'],self.bus.sent[-1][1]['updated']),('new','2026-11-05 11:00:00'))

    def test_a_table_larger_than_a_page_is_read_page_by_page_in_order(self):
        self.db.max_rows=4;self.agent=self.make(max_rows=4)   # a page of 3, under the DB agent's cut
        for k in range(1,11):self.db.put(k,'2026-11-05 10:00:00')   # one updated value: the key breaks the tie
        self.subscribe()
        self.agent.poll()
        self.assertEqual(sorted(self.keys(),key=int),[str(k) for k in range(1,11)])
        pages=[q for q in self.db.queries if ' LIMIT ' in q]
        self.assertEqual((len(pages),{q.rsplit(' ',1)[1] for q in pages}),(4,{'3'}))

    def test_rows_every_subscriber_is_past_leave_the_window(self):
        for k in range(1,4):self.db.put(k,f'2026-11-05 10:00:0{k}')
        self.subscribe();self.agent.poll()
        status=self.agent.status()
        self.assertEqual(status['window'],0,status)
        self.assertEqual(status['subscribers'][0]['position'],['2026-11-05 10:00:03','3'])

    def test_a_cut_page_with_no_row_after_the_position_refuses_by_name_instead_of_looping(self):
        self.db.put(1,'2026-11-05 10:00:00');self.subscribe()
        self.db.fail=json.dumps({'columns':['id','updated_at','note'],'rows':[],'truncated':True})
        self.agent.check_types=lambda:self.agent.sql.kinds.update(key='numeric',updated='timestamp')
        with self.assertRaisesRegex(dbsync.CannotSync,'larger than the DB agent returns'):self.agent.poll()
        self.assertIn(('no_progress',None),self.warnings)
        self.assertIsNotNone(self.agent.status()['stuck'])

    def test_a_row_gone_before_its_body_is_read_is_dropped_without_a_delete(self):
        for k in (1,2):self.db.put(k,f'2026-11-05 10:00:0{k}')
        self.subscribe();self.agent._read()
        self.agent.rows.clear();self.db.drop(1)   # a new process: the body is read again, and the row is gone
        self.agent._deliver()   # the vanished row ends this subscriber's round, sending nothing
        self.assertNotIn('1',self.agent.state.load()['items'])
        self.agent._deliver()
        self.assertEqual(self.keys(),['2'])

    def test_an_author_no_longer_admitted_to_the_db_agent_gets_nothing_and_is_parked(self):
        self.db.put(1,'2026-11-05 10:00:01');sub=self.subscribe()
        self.bus.deny.add((AUTHOR,DB))
        self.agent.poll()
        self.assertEqual(self.bus.sent,[])
        self.now[0]+=601;self.agent.poll()
        self.assertEqual(self.agent.status()['subscribers'][0]['parked']['reason'],'admission_refused')
        self.assertIn(('parked',sub),self.warnings)
        self.bus.deny.clear();self.agent.poll()
        self.assertIn(('unparked',sub),self.warnings)
        self.agent.poll()
        self.assertEqual(self.keys(),['1'])

    def test_a_refused_subscriber_is_parked_and_stops_holding_the_window_for_the_others(self):
        self.agent=self.make(window_max=3)
        for k in range(1,11):self.db.put(k,f'2026-11-05 10:00:{k:02}')
        self.subscribe(who='#gone@n.test');self.subscribe()
        self.bus.deny.add(('#gone@n.test',QUEUE))
        for _ in range(3):self.agent.poll()
        self.assertEqual(len(self.keys()),3)   # the full window waits for the refused one
        self.now[0]+=601
        for _ in range(6):self.agent.poll()
        status={s['position'] is None and 'gone' or 'ok':s for s in self.agent.status()['subscribers']}
        self.assertEqual(status['gone']['parked']['reason'],'admission_refused')
        self.assertEqual(sorted(self.keys(),key=int),[str(k) for k in range(1,11)])
        self.bus.deny.clear()
        for _ in range(8):self.agent.poll()
        self.assertEqual(len(self.keys()),20)   # back in: it gets all ten from where it stood

    def test_a_subscriber_added_after_the_window_was_pruned_still_gets_every_row(self):
        for k in range(1,4):self.db.put(k,f'2026-11-05 10:00:0{k}')
        self.subscribe();self.agent.poll()
        self.assertEqual(self.agent.status()['window'],0)
        self.subscribe(who='#late@n.test');self.agent.poll()
        self.assertEqual(sorted(self.keys(),key=int),['1','1','2','2','3','3'])


    def test_a_subscriber_that_asks_for_later_changes_only_starts_at_the_newest_row(self):
        for k in range(1,4):self.db.put(k,f'2026-11-05 10:00:0{k}')
        reply=json.loads(self.agent.handle(msg(f'sub add {QUEUE} replay:false')))
        self.assertEqual(self.agent.status()['subscribers'][0]['position'],['2026-11-05 10:00:03','3'],reply)
        self.agent.poll()
        self.assertEqual(self.bus.sent,[])
        self.db.put(4,'2026-11-05 10:00:04');self.db.put(2,'2026-11-05 10:00:05','x');self.agent.poll()
        self.assertEqual(sorted(self.keys(),key=int),['2','4'])

    def test_later_changes_only_on_an_empty_table_sends_every_row_that_comes(self):
        self.agent.handle(msg(json.dumps({'method':'sub.add','target':QUEUE,'replay':False})))
        self.db.put(1,'2026-11-05 10:00:01');self.agent.poll()
        self.assertEqual(self.keys(),['1'])
        reply=json.loads(self.agent.handle(msg(json.dumps({'method':'sub.add','target':QUEUE,'replay':'no'}))))
        self.assertEqual(reply.get('error',{}).get('code'),'invalid_request',reply)

    def test_a_subscriber_added_while_a_page_is_read_still_gets_the_pruned_rows(self):
        for k in range(1,4):self.db.put(k,f'2026-11-05 10:00:0{k}')
        self.subscribe();self.agent.poll()
        self.assertEqual((self.agent.status()['window'],len(self.bus.sent)),(0,3))
        answer=self.db.answer;added=[]
        def during(body):
            result=answer(body)
            if ' LIMIT ' in body and not added:added.append(self.subscribe(who='#late@n.test'))
            return result
        self.db.answer=during
        self.agent.poll();self.agent.poll()
        self.assertEqual(len(added),1)
        self.assertEqual(sorted(self.keys(),key=int),['1','1','2','2','3','3'])


class SoftDeleteTests(Base):
    config=dict(Base.config,deleted='note',deleted_value='del')

    def test_a_row_marked_deleted_after_it_was_delivered_goes_as_a_delete_without_its_columns(self):
        self.db.put(1,'2026-11-05 10:00:01');self.subscribe();self.agent.poll()
        self.assertEqual(self.agent.status()['window'],0)   # delivered and pruned: the window no longer holds it
        self.db.put(1,'2026-11-05 10:05:00','del');self.agent.poll()
        sent=self.bus.sent[-1][1]
        self.assertEqual((sent['change'],sent['key'],sent['updated'],'row' in sent),('deleted','1','2026-11-05 10:05:00',False))
        self.agent.poll()
        self.assertEqual(len(self.bus.sent),2)   # confirmed once, never again

    def test_a_replaying_subscriber_gets_no_delete_for_a_row_it_never_had(self):
        self.db.put(1,'2026-11-05 10:00:01','del');self.db.put(2,'2026-11-05 10:00:02')
        self.subscribe();self.agent.poll()
        self.assertEqual([(m['change'],m['key']) for _,m in self.bus.sent],[('changed','2')])

    def test_a_row_deleted_before_a_lagging_subscriber_got_it_sends_that_one_nothing(self):
        self.db.put(1,'2026-11-05 10:00:01');self.subscribe();self.agent.poll()   # receiving: it has a position
        self.bus.deny.add((AUTHOR,QUEUE))
        self.db.put(2,'2026-11-05 10:00:02');self.agent.poll()   # refused: row 2 stays in the window with no mark for it
        self.db.put(2,'2026-11-05 10:05:00','del');self.bus.deny.clear();self.agent.poll()
        self.assertEqual([(m['change'],m['key']) for _,m in self.bus.sent],[('changed','1')])

    def test_the_marker_matches_only_as_the_database_returned_it(self):
        self.assertTrue(dbsync.DBSync._is(1,True));self.assertTrue(dbsync.DBSync._is('del','del'))
        self.assertFalse(dbsync.DBSync._is('1',1));self.assertFalse(dbsync.DBSync._is(1,'1'));self.assertFalse(dbsync.DBSync._is(2,True))
        self.assertFalse(dbsync.DBSync._is(True,1))   # a boolean column does not match a configured number

    def test_a_marker_value_needs_its_column_and_a_plain_value(self):
        base={'db':DB,'engine':'mysql','table':'t','key':'id','updated':'u'}
        with self.assertRaisesRegex(cfg.SettingsError,'^deleted_value: only with deleted'):cfg.parse(dict(base,deleted_value=1))
        with self.assertRaisesRegex(cfg.SettingsError,'^deleted_value: the value'):cfg.parse(dict(base,deleted='d',deleted_value=1.5))


class CallTests(Base):
    def test_a_caller_without_a_role_is_refused_and_reset_needs_a_manager_and_confirm(self):
        self.assertEqual(json.loads(self.agent.handle(msg('status',roles=())))['error']['code'],'forbidden')
        self.assertEqual(json.loads(self.agent.handle(msg('reset')))['error']['code'],'forbidden')
        self.assertEqual(json.loads(self.agent.handle(msg('reset',roles=('owner',))))['error']['code'],'not_confirmed')
        self.assertEqual(json.loads(self.agent.handle(msg('reset confirm:true',roles=('owner',)))),{'reset':True})

    def test_a_subscriber_must_be_able_to_reach_the_db_agent_itself(self):
        self.bus.deny.add((AUTHOR,DB))
        self.assertEqual(json.loads(self.agent.handle(msg(f'sub add {QUEUE}')))['error']['code'],'admission_refused')

    def test_a_db_agent_error_is_named_and_its_text_kept(self):
        self.db.fail='error: relation "orders" does not exist'
        reply=json.loads(self.agent.handle(msg(f'sub add {QUEUE}')))
        self.assertEqual(reply['error'],{'code':'db_unavailable','text':'error: relation "orders" does not exist'})


class ManifestTests(unittest.TestCase):
    def test_every_method_in_the_manifest_is_one_the_agent_parses_and_no_other(self):
        manifest=json.loads((HERE/'agent.json').read_text())
        forms={'help':'help','status':'status','poll':'poll','reset':'reset confirm:true','sub.add':'sub add q','sub.list':'sub list','sub.delete':'sub delete x'}
        self.assertEqual(set(manifest['methods']),set(forms))
        for method,text in forms.items():self.assertEqual(dbsync.parse(text)[0],method)
        self.assertEqual(set(manifest['config']),cfg.KEYS)
        with self.assertRaises(ValueError):dbsync.parse('query SELECT 1')


if __name__=='__main__':unittest.main()
