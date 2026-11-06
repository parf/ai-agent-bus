"""Citations refer to the live published revision, not draft chunks or invented provenance."""
from copy import deepcopy
import hashlib
from pathlib import Path
import sys
import unittest
HERE=Path(__file__).resolve().parent
sys.path[:0]=[str(HERE),str(HERE.parents[1]/'clients/python'),str(HERE.parent/'shared')]
from rules import parse,dumps,Unknown,NamedRefusal,point_id
from core import Core
from test_rag import Store,KV,CFG,PROMPT

class Evidence(unittest.TestCase):
    def setUp(self):
        self.store=Store();self.store.sparse=True;self.kv=KV();self.app=Core(CFG,self.store,self.kv)
        self.app.run(parse(dumps({'method':'prompts','namespace':'a','tableName':'t','action':'set','data':PROMPT})))
    def put(self,data='Tea is a drink.',source='notes/tea.md',**extra):
        return self.app.run(parse(dumps({'method':'upsert','namespace':'a','tableName':'t','id':'tea','data':data,'source':source,**extra})))
    def find(self):return self.app.run(parse('{"method":"search","namespace":"a","query":"tea"}'))
    def test_exact_source_hash_and_live_chunk_citation(self):
        raw='<p>Tea is a drink.</p><script>not indexed</script>'
        full=self.put(raw,**{'mime-type':'text/html'})
        hit=self.find()['records'][0]
        self.assertEqual(hit['namespace'],'a');self.assertEqual(hit['tableName'],'t');self.assertEqual(hit['id'],'tea')
        self.assertEqual(hit['chunk_index'],0);self.assertEqual(hit['source'],'notes/tea.md')
        self.assertEqual(hit['content_hash'],hashlib.sha256(raw.encode()).hexdigest())
        self.assertNotIn('not indexed',hit['excerpt']);self.assertIn('Tea is a drink.',hit['excerpt'])
        self.assertEqual(set(hit['scores']),{'bm25'});self.assertGreater(hit['scores']['bm25'],0)
        self.assertEqual(hit['indexed_at'],full['indexed_at']);self.assertTrue(hit['indexed_at'].endswith('Z'))
        self.assertNotIn('score',hit);self.assertNotIn('text',hit)
        got=self.app.run(parse('{"method":"get","namespace":"a","tableName":"t","id":"tea"}'))
        self.assertEqual(got['source'],hit['source']);self.assertEqual(got['content_hash'],hit['content_hash']);self.assertEqual(got['data'],raw)
    def test_failed_replacement_draft_never_supplies_citations(self):
        old=self.put();head=point_id({'namespace':'a','tableName':'t','id':'tea'})
        def fail(verb,path,body):return verb=='PUT' and '/points?' in path and any(p['id']==head for p in body['points'])
        self.store.fail=fail
        with self.assertRaises(Unknown):self.put('Tea replacement draft','new/path.md')
        self.store.fail=None
        hit=self.find()['records'][0]
        self.assertEqual(hit['source'],'notes/tea.md');self.assertEqual(hit['indexed_at'],old['indexed_at'])
        self.assertNotIn('replacement',hit['excerpt'])
    def test_a_source_only_replacement_gets_distinct_chunk_ids(self):
        self.put();old={p['id'] for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk'}
        full=self.put(source='renamed/tea.md')
        new={p['id'] for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk'}
        self.assertFalse(old&new);hit=self.find()['records'][0]
        self.assertEqual(hit['source'],'renamed/tea.md');self.assertEqual(hit['indexed_at'],full['indexed_at'])
    def test_missing_provenance_is_refused_at_boundary(self):
        self.put();repo=self.app.catalogue.bind('a',self.app.cache)
        point=deepcopy(next(p for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk'))
        for field in ('source','content_hash','indexed_at'):
            bad=deepcopy(point);del bad['payload'][field]
            with self.assertRaises(NamedRefusal) as c:repo._validate(bad)
            self.assertEqual(c.exception.code,'legacy_data')
    def test_wrong_supplied_hash_refuses_before_provider_work(self):
        self.store.paid.clear()
        with self.assertRaises(ValueError):self.put(content_hash='0'*64)
        self.assertEqual(self.store.paid,[]);self.assertEqual(self.store.points,{})

if __name__=='__main__':unittest.main()
