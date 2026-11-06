"""Manager-only inventory and explicitly resumed namespace deletion, fake blocks only."""
from copy import deepcopy
import unittest
from unittest.mock import patch
from test_namespaces import Boundary
from test_rag import Core,CFG,Store,KV,PROMPT
from rules import parse,dumps,NamedRefusal,Unknown,RagError,prompt_key
from namespaces import CATALOGUE_KEY


class Administration(unittest.TestCase):
    def setUp(self):
        self.store=Store();self.kv=KV();self.app=Core(CFG,self.store,self.kv)
    def call(self,method,roles=('owner',),**fields):
        return self.app.run(parse(dumps({'method':method,**fields})),{'roles':list(roles)})
    def put(self,ns,id='same'):
        self.call('prompts',namespace=ns,tableName='t',action='set',data=PROMPT)
        self.call('upsert',namespace=ns,tableName='t',id=id,data='Secret '+ns,source=ns+'.md')
    def drop(self,ns='a',**fields):return self.call('drop_namespace',namespace=ns,confirm=ns,**fields)

    def test_inventory_is_manager_only_before_any_read(self):
        self.put('a');before=deepcopy(self.store.points);reads=self.kv.reads;self.store.calls.clear()
        for method,fields in (('namespaces',{}),('drop_namespace',{'namespace':'a','confirm':'a'})):
            with self.assertRaises(NamedRefusal) as c:self.call(method,roles=(),**fields)
            self.assertEqual(c.exception.code,'forbidden')
            self.assertEqual(self.store.points,before,'an admitted reader must not remove a sentinel')
        self.assertEqual(self.store.calls,[]);self.assertEqual(self.kv.reads,reads)

    def test_missing_or_wrong_confirmation_preserves_the_sentinel(self):
        self.put('a');before=deepcopy(self.store.points);metadata=deepcopy(self.kv.values);self.store.calls.clear()
        for fields in ({},{'confirm':'b'}):
            with self.assertRaises(NamedRefusal) as c:self.call('drop_namespace',namespace='a',**fields)
            self.assertEqual(c.exception.code,'confirmation_required')
            self.assertEqual(self.store.points,before,'missing confirmation must never remove data')
            self.assertEqual(self.kv.values,metadata)
        self.assertEqual(self.store.calls,[])

    def test_inventory_counts_heads_not_chunks_drafts_or_tombstones(self):
        self.put('a');self.put('a','second');self.put('b')
        self.call('list',namespace='empty')
        # Duplicate a staged chunk, which is not a record head.
        chunk=deepcopy(next(p for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk'))
        self.store.points['draft']=chunk
        page=self.call('namespaces',roles=('maintainer',),limit=2)
        self.assertEqual(page['namespaces'],[{'namespace':'a','count':2},{'namespace':'b','count':1}])
        self.assertIsNotNone(page['cursor'])
        self.assertEqual(self.call('namespaces',limit=2,cursor=page['cursor']),{'namespaces':[{'namespace':'empty','count':0}],'cursor':None})
        with self.assertRaisesRegex(RagError,'cursor'):self.call('list',namespace='a',cursor=page['cursor'],limit=2)
        self.store.points.pop('draft')
        self.call('delete',namespace='a',tableName='t',id='second')
        self.assertEqual(self.call('namespaces')['namespaces'][0],{'namespace':'a','count':1})

    def test_drop_is_scoped_and_cleans_only_its_prompts_cursors_and_caches(self):
        for ns in ('a','b'):
            for id in ('first','second'):self.put(ns,id)
        cursors={ns:self.call('list',namespace=ns,limit=1)['cursor'] for ns in ('a','b')}
        self.app.index_state={'a':True,'b':True};b_points={k:deepcopy(p) for k,p in self.store.points.items() if p['payload']['namespace']=='b'}
        b_prompt=deepcopy(self.kv.values[prompt_key('b','t')]);paid=len(self.store.paid)
        self.assertEqual(self.drop(),{'status':'dropped','namespace':'a'})
        self.assertEqual(self.store.points,b_points,'the B sentinel must survive an A drop')
        self.assertNotIn(prompt_key('a','t'),self.kv.values);self.assertEqual(self.kv.values[prompt_key('b','t')],b_prompt)
        self.assertNotIn(prompt_key('a','t'),self.app.cache.values)
        self.assertNotIn(cursors['a'],self.app.cursors);self.assertIn(cursors['b'],self.app.cursors)
        self.assertEqual(self.app.index_state,{'b':True});self.assertEqual(len(self.store.paid),paid)
        self.assertEqual(self.drop(),{'status':'absent','namespace':'a'})
        self.assertEqual(self.call('namespaces')['namespaces'],[{'namespace':'b','count':2}])

    def test_partial_drop_blocks_data_after_restart_until_confirmed_continuation(self):
        for stage in ('points','prompt','catalogue'):
            with self.subTest(stage=stage):
                self.setUp();self.put('a');self.put('b');original_delete=self.kv.delete;original_set=self.kv.set
                if stage=='points':self.store.fail=lambda verb,path,body:'/delete?' in path
                if stage=='prompt':
                    def fail_delete(key):raise RagError('unavailable')
                    self.kv.delete=fail_delete
                if stage=='catalogue':
                    def fail_set(key,value):
                        if key==CATALOGUE_KEY and 'a' not in value['namespaces']:raise RagError('unavailable')
                        return original_set(key,value)
                    self.kv.set=fail_set
                with self.assertRaises(Unknown):self.drop()
                self.assertEqual(self.kv.values[CATALOGUE_KEY]['namespaces']['a']['state'],'dropping')
                self.app=Core(CFG,self.store,self.kv)
                for method,fields in (('list',{}),('prompts',{'tableName':'t','action':'get'}),('keyword_index',{})):
                    with self.assertRaises(NamedRefusal) as c:self.call(method,namespace='a',**fields)
                    self.assertEqual(c.exception.code,'busy')
                self.store.fail=None;self.kv.delete=original_delete;self.kv.set=original_set
                self.assertEqual(self.drop()['status'],'dropped')
                self.assertTrue(all(p['payload']['namespace']=='b' for p in self.store.points.values()))
                self.assertNotIn('a',self.kv.values[CATALOGUE_KEY]['namespaces'])

    def test_uncertain_intent_stops_before_any_point_deletion(self):
        self.put('a');before=deepcopy(self.store.points);self.kv.fail=True
        with self.assertRaises(Unknown):self.drop()
        self.assertEqual(self.store.points,before)
        self.assertEqual(self.kv.values[CATALOGUE_KEY]['namespaces']['a']['state'],'dropping')

    def test_failed_count_is_not_reported_as_zero(self):
        self.put('a');original=self.store.qdrant
        def wrong(verb,path,body=None,**kw):
            if path=='/points/count' and any(c.get('key')=='_rag_kind' for c in body['filter']['must']):return {'count':False}
            return original(verb,path,body,**kw)
        self.store.qdrant=wrong
        with self.assertRaisesRegex(RagError,'count'):self.call('namespaces')

    def test_catalogue_reserves_space_to_start_drop_when_full(self):
        self.call('list',namespace='a');rows=self.app.catalogue.load()
        actual=len(dumps({'schema':3,'namespaces':rows},ascii=True).encode())
        with patch('namespaces.CATALOGUE_MAX',actual):
            with self.assertRaisesRegex(NamedRefusal,'full'):self.app.catalogue.save(rows)


if __name__=='__main__':unittest.main()
