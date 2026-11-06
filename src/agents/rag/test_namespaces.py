"""Namespace isolation/model proofs, fake blocks only; no real Qdrant imports."""
from copy import deepcopy
from dataclasses import FrozenInstanceError
from pathlib import Path
import sys
import unittest
HERE=Path(__file__).resolve().parent
sys.path[:0]=[str(HERE),str(HERE.parents[1]/'clients/python'),str(HERE.parent/'shared')]
from core import Core
from prompts import validate
from rules import parse,dumps,NamedRefusal,Unknown,match
from namespaces import Catalogue,CATALOGUE_KEY
from test_rag import Store,KV,CFG,PROMPT

class Boundary(unittest.TestCase):
    def setUp(self):
        self.store=Store();self.store.sparse=True;self.kv=KV();self.app=Core(CFG,self.store,self.kv)
    def call(self,method='list',namespace='a',**fields):
        return self.app.run(parse(dumps({'method':method,'namespace':namespace,**fields})),{'roles':['owner']})
    def put(self,namespace,id='same',data='Tea'):
        self.call('prompts',namespace,tableName='t',action='set',data=PROMPT)
        return self.call('upsert',namespace,tableName='t',id=id,data=data,source='notes/'+id)
    def test_every_data_read_write_and_prompt_is_bound(self):
        self.put('a',data='Tea A');self.put('b',data='Tea B')
        self.assertEqual(self.call('get','a',tableName='t',id='same')['data'],'Tea A')
        self.assertEqual([r['namespace'] for r in self.call('list','a')['records']],['a'])
        self.assertEqual([r['namespace'] for r in self.call('search','a',query='tea')['records']],['a'])
        other=PROMPT|{'instructions':'B secret'}
        self.call('prompts','b',tableName='t',action='set',data=other)
        self.assertEqual(self.call('prompts','a',tableName='t',action='get'),validate(PROMPT))
        self.call('prompts','a',tableName='t',action='delete')
        self.assertEqual(self.call('prompts','b',tableName='t',action='get'),validate(other))
        self.call('delete','a',tableName='t',id='same')
        self.assertEqual(self.call('get','b',tableName='t',id='same')['data'],'Tea B')
        for method in ('get','delete'):
            with self.assertRaises(NamedRefusal) as c:self.call(method,'a',tableName='t',id='same')
            self.assertEqual(c.exception.code,'not_found')
        self.assertTrue(all(p['payload']['namespace']=='b' for p in self.store.points.values()))
    def test_external_engine_cannot_return_foreign_evidence(self):
        self.app=Core(CFG|{'keyword_search':'external'},self.store,self.kv,
                      keyword=lambda request:{'records':[{'namespace':'b','tableName':'t','id':'secret'}]})
        with self.assertRaises(NamedRefusal) as c:self.call('search','a',query='tea')
        self.assertEqual(c.exception.code,'scope_mismatch');self.assertEqual(self.store.paid,[])

    def test_real_queries_include_namespace_even_for_id_reads(self):
        self.put('a');self.store.calls.clear()
        self.call('get','a',tableName='t',id='same');self.call('list','a');self.call('keyword_index','a')
        for verb,path,body,write in self.store.calls:
            if path in ('/points/scroll','/points/query') or path=='/points/count' and not any('is_empty' in c for c in body['filter'].get('must',[])):
                self.assertIn(match('namespace','a'),body['filter']['must'],'every actual data request has its bound namespace')
        self.assertFalse(any(v=='GET' and p.startswith('/points/') for v,p,_,_ in self.store.calls))
    def test_namespace_predicate_never_selects_a_foreign_sentinel(self):
        self.put('a');self.put('b',id='b-only-secret')
        repo=self.app.catalogue.bind('a',self.app.cache);original=self.store.qdrant;selected=[]
        def traced(verb,path,body=None,**kwargs):
            result=original(verb,path,body,**kwargs)
            if path=='/points/scroll':selected.extend(row['payload']['namespace'] for row in result['points'])
            return result
        self.store.qdrant=traced
        # A second response guard must not mask a missing selection predicate in this proof.
        try:repo.qdrant('POST','/points/scroll',{'limit':16,'with_payload':True,'with_vector':False})
        except NamedRefusal:pass
        self.assertEqual(set(selected),{'a'},'the backend must never select the B-only sentinel for A')

    def test_foreign_backend_reply_is_refused_at_boundary(self):
        self.put('b');foreign=deepcopy(next(p for p in self.store.points.values() if p['payload']['_rag_kind']=='head'))
        repo=self.app.catalogue.bind('a',self.app.cache);original=self.store.qdrant
        self.store.qdrant=lambda verb,path,body=None,**kw:{'points':[foreign],'next_page_offset':None} if path=='/points/scroll' else original(verb,path,body,**kw)
        with self.assertRaises(NamedRefusal) as c:repo.qdrant('POST','/points/scroll',{'limit':1,'with_payload':True,'with_vector':False})
        self.assertEqual(c.exception.code,'scope_mismatch')
    def test_immutable_namespace_and_cursor_scope(self):
        repo=self.app.catalogue.bind('a',self.app.cache)
        with self.assertRaises(FrozenInstanceError):repo.name='b'
        with self.assertRaises(TypeError):repo.config['dimensions']=4
        for i in range(3):self.put('a',str(i))
        page=self.call('list','a',limit=1);self.assertIsNotNone(page['cursor'])
        with self.assertRaisesRegex(ValueError,'cursor'):self.call('list','b',limit=1,cursor=page['cursor'])
    def test_binding_restart_and_same_dimension_model_drift(self):
        self.put('a');saved=deepcopy(self.kv.values[CATALOGUE_KEY]);paid=len(self.store.paid)
        app=Core(CFG,self.store,self.kv)
        self.assertEqual(app.run(parse('{"method":"get","namespace":"a","tableName":"t","id":"same"}'))['data'],'Tea')
        with self.assertRaises(NamedRefusal) as c:Catalogue(self.kv,self.store,CFG|{'embedding_model':'other'}).bind('a',self.app.cache)
        self.assertEqual(c.exception.code,'model_mismatch');self.assertEqual(len(self.store.paid),paid)
        self.assertEqual(self.kv.values[CATALOGUE_KEY],saved)
    def test_uncertain_binding_precedes_paid_calls(self):
        self.kv.fail=True
        with self.assertRaises(Unknown):self.call('upsert','a',tableName='t',id='same',data='Tea',source='tea.md')
        self.assertEqual(self.store.paid,[]);self.assertEqual(self.store.points,{})
    def test_bindingless_and_namespace_free_legacy_refused(self):
        self.put('a');self.kv.values.clear();paid=len(self.store.paid)
        with self.assertRaises(NamedRefusal) as c:self.call('get','a',tableName='t',id='same')
        self.assertEqual(c.exception.code,'legacy_data');self.assertEqual(len(self.store.paid),paid)
        self.store.points={'old':{'id':'old','vector':[1,0],'payload':{'data':'old secret'}}}
        with self.assertRaises(NamedRefusal) as c:self.call('list','a')
        self.assertEqual(c.exception.code,'legacy_data');self.assertEqual(self.kv.values,{})
    def test_corrupt_catalogue_and_dropping_namespace_refused(self):
        self.kv.values[CATALOGUE_KEY]={'schema':True,'namespaces':{}}
        with self.assertRaises(NamedRefusal) as c:self.call('list','a')
        self.assertEqual(c.exception.code,'state_invalid')
        self.kv.values.clear();self.call('list','a');self.kv.values[CATALOGUE_KEY]['namespaces']['a']['state']='dropping'
        with self.assertRaises(NamedRefusal) as c:self.call('list','a')
        self.assertEqual(c.exception.code,'busy');self.assertEqual(self.store.paid,[])
    def test_backfill_and_completeness_scoped(self):
        self.put('a');self.put('b')
        for p in self.store.points.values():
            if p['payload']['_rag_kind']=='chunk':p['vector'].pop('text',None)
        self.call('keyword_index','a')
        self.assertEqual(self.call('search','a',query='tea')['records'][0]['namespace'],'a')
        with self.assertRaisesRegex(ValueError,'incomplete'):self.call('search','b',query='tea')
        for p in self.store.points.values():
            if p['payload']['namespace']=='b' and p['payload']['_rag_kind']=='chunk':self.assertNotIn('text',p['vector'])

if __name__=='__main__':unittest.main()
