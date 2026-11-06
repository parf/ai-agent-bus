"""DBSync against real engines, through the DB agent's own code under its read tier: the db fixture's MariaDB and PostgreSQL.

Normal-set lines of the agent_dbsync shard: they start the shared db fixture (rootless podman for PostgreSQL), so a
hand run takes the smoke-gate lock like a gate. What they prove that the sqlite fake cannot: each dialect's quoting and case, current_schema(),
the time query against a zoned and a naive column, microsecond ties, and the read tier refusing nothing DBSync sends.
"""
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE));sys.path.insert(0,str(HERE.parent/'db'));sys.path.insert(0,str(HERE.parent/'shared'))
import dbsync
import fixtures
from fixtures import SENTINEL
from test_dbsync import Bus, msg, DB, QUEUE

spec=importlib.util.spec_from_file_location('db_agent',HERE.parent/'db'/'db.py')
proxy=importlib.util.module_from_spec(spec);spec.loader.exec_module(proxy)


def tearDownModule():fixtures._cleanup()


class Agent:
    """The DB agent as a caller with its role sees it: `tier` picks the account, max_rows cuts every answer."""
    def __init__(self,config):self.config=config;self.queries=[];self.errors=[]
    def run(self,sql,tier='admin'):
        env={'DB_USER':'app','DB_PASSWORD':SENTINEL}
        if tier=='admin':env['AB_ROLE_ADMIN']='1'
        out,_=proxy.answer({'body':'query '+sql,'deadline':None},self.config,env)
        if out.startswith('error:'):raise AssertionError(f'{sql}: {out}')
        return out
    def answer(self,body):
        # The read tier on the app account, as test_db.py reads: db.py redacts DB_USER from every answer, and the
        # fixture's "ro" turns the answer's own "rows" key into "***ws".
        self.queries.append(body);out,_=proxy.answer({'body':body,'deadline':None},self.config,{'DB_USER':'app','DB_PASSWORD':SENTINEL})
        if out.startswith('error:'):self.errors.append((body,out))
        return out


class Engine:
    make=None;setup=();config={}
    late=None;tie=None;stale=None;remove=None

    @classmethod
    def setUpClass(cls):
        cls.db_config=dict(cls.make(),max_rows=4)   # pages of 3, so a page boundary falls inside the tie
        cls.agent=Agent(cls.db_config)
        for sql in cls.setup:cls.agent.run(sql)

    def test_a_table_is_paged_late_commits_are_sent_once_and_a_soft_delete_goes(self):
        tmp=tempfile.TemporaryDirectory(dir=HERE.parents[2]/'tmp');self.addCleanup(tmp.cleanup)
        bus=Bus(self.agent);sync=dbsync.DBSync(bus,dict(self.config,db=DB,max_rows=4),tmp.name,warning=lambda *a:None)
        self.assertEqual(sync.check_types(),{'key':'numeric','updated':'timestamp'})
        reply=json.loads(sync.handle(msg(f'sub add {QUEUE}')));self.assertIn('id',reply,reply)
        sync.poll()
        keys=lambda:[m['key'] for _,m in bus.sent]
        self.assertEqual(sorted(keys(),key=int),['1','2','3','4','5'],self.agent.errors)
        first=bus.sent[0][1]
        self.assertEqual(set(first['row']),set(self.columns),first)
        self.agent.run(self.late)                   # committed now, stamped two minutes back: below the position
        sync.poll()
        self.assertEqual(keys()[5:],['6'],self.agent.errors)
        sync.poll();sync.poll()
        self.assertEqual(len(bus.sent),6)           # the lookback reads them all again; none goes twice
        self.agent.run(self.stale)                  # stamped ten minutes back: older than the lookback, not seen
        self.agent.run(self.remove)
        sync.poll()
        self.assertEqual([(m['change'],m['key']) for _,m in bus.sent[6:]],[('deleted','1')],self.agent.errors)
        self.assertEqual(self.agent.errors,[])
        self.assertTrue(any(' >= ' in q for q in self.agent.queries),'no poll read again from the cutoff')


class PostgreSQL(Engine,unittest.TestCase):
    make=staticmethod(fixtures.postgres)
    # Mixed-case names exist only quoted: an unquoted statement would ask for "orders" and "updatedat".
    setup=('CREATE TABLE "Orders" ("Id" bigint PRIMARY KEY, "updatedAt" timestamptz(6) NOT NULL, "Note" text, "Gone" boolean NOT NULL DEFAULT false)',
           # five rows: 2 and 3 share one microsecond stamp, 4 is one microsecond later, all well inside the lookback
           "INSERT INTO \"Orders\" VALUES (1,now()-interval '60 seconds','a',false),(2,now()-interval '50 seconds','b',false),"
           "(3,now()-interval '50 seconds','c',false),(4,now()-interval '50 seconds'+interval '1 microsecond','d',false),(5,now()-interval '40 seconds','e',false)",
           "UPDATE \"Orders\" SET \"updatedAt\"=(SELECT \"updatedAt\" FROM \"Orders\" WHERE \"Id\"=2) WHERE \"Id\"=3")
    config={'engine':'postgres','table':'Orders','key':'Id','updated':'updatedAt','columns':['Note','Gone'],'deleted':'Gone','deleted_value':True}
    columns=('Id','updatedAt','Note','Gone')
    late="INSERT INTO \"Orders\" VALUES (6,now()-interval '55 seconds','late',false)"
    stale="INSERT INTO \"Orders\" VALUES (7,now()-interval '10 minutes','stale',false)"
    remove="UPDATE \"Orders\" SET \"Gone\"=true,\"updatedAt\"=now() WHERE \"Id\"=1"


class MySQL(Engine,unittest.TestCase):
    make=staticmethod(fixtures.mariadb)
    setup=('CREATE TABLE orders (id BIGINT PRIMARY KEY, updated_at DATETIME(6) NOT NULL, note TEXT, gone TINYINT NOT NULL DEFAULT 0)',
           "INSERT INTO orders VALUES (1,NOW(6)-INTERVAL 60 SECOND,'a',0),(2,NOW(6)-INTERVAL 50 SECOND,'b',0),(3,NOW(6)-INTERVAL 50 SECOND,'c',0),"
           "(4,NOW(6)-INTERVAL 50 SECOND+INTERVAL 1 MICROSECOND,'d',0),(5,NOW(6)-INTERVAL 40 SECOND,'e',0)",
           'UPDATE orders o JOIN (SELECT updated_at u FROM orders WHERE id=2) x SET o.updated_at=x.u WHERE o.id=3')
    config={'engine':'mysql','table':'orders','key':'id','updated':'updated_at','columns':['note','gone'],'deleted':'gone','deleted_value':1}
    columns=('id','updated_at','note','gone')
    late="INSERT INTO orders VALUES (6,NOW(6)-INTERVAL 55 SECOND,'late',0)"
    stale="INSERT INTO orders VALUES (7,NOW(6)-INTERVAL 10 MINUTE,'stale',0)"
    remove='UPDATE orders SET gone=1,updated_at=NOW(6) WHERE id=1'


if __name__=='__main__':unittest.main()
