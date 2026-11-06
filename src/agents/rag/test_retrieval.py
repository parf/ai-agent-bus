"""Query construction and live retrieval with deterministic fake dense/BM25 providers."""
from copy import deepcopy
import math
from pathlib import Path
import sys
import unittest
HERE=Path(__file__).resolve().parent
sys.path[:0]=[str(HERE),str(HERE.parents[1]/'clients/python'),str(HERE.parent/'shared')]
from rules import parse,dumps,match,RagError,NamedRefusal
from core import Core
from test_rag import Store,KV,CFG,PROMPT

class RetrievalStore(Store):
    def embed(self,request):
        result=super().embed(request)
        if request.get('input_type')=='query':return {'data':[{'index':0,'embedding':[1.0,0.0]}]}
        return result
    def qdrant(self,verb,path,body=None,write=False):
        if path!='/points/query':return super().qdrant(verb,path,body,write)
        self.calls.append((verb,path,deepcopy(body),write))
        def ranked(q):
            result=[]
            for row in self.points.values():
                if not self.matches(row,q['filter']):continue
                vec=row['vector']
                if q.get('using')=='text':
                    if not isinstance(vec,dict) or 'text' not in vec:continue
                    weights=dict(zip(q['query']['indices'],q['query']['values']));doc=vec['text']
                    score=sum(weights.get(i,0)*v for i,v in zip(doc['indices'],doc['values']))
                    if not score:continue
                else:
                    dense=vec[''] if isinstance(vec,dict) else vec
                    denom=math.sqrt(sum(x*x for x in dense))*math.sqrt(sum(x*x for x in q['query']))
                    score=sum(a*b for a,b in zip(dense,q['query']))/denom if denom else 0
                result.append((score,row))
            return sorted(result,key=lambda x:(-x[0],x[1]['id']))[:q['limit']]
        if body['query']=={'fusion':'rrf'}:
            scores={};points={}
            for branch in body['prefetch']:
                for rank,(_,p) in enumerate(ranked(branch)):
                    scores[p['id']]=scores.get(p['id'],0)+1/(rank+2);points[p['id']]=p
            result=sorted(((score,points[i]) for i,score in scores.items()),key=lambda x:(-x[0],x[1]['id']))
        else:result=ranked(body|{'limit':body['limit']+body.get('offset',0)})
        keep=body['with_payload']['include'];offset=body.get('offset',0)
        return {'points':[{'id':p['id'],'score':s,'payload':{k:v for k,v in p['payload'].items() if k in keep}} for s,p in result[offset:offset+body['limit']]]}

class Retrieval(unittest.TestCase):
    def setUp(self):
        self.store=RetrievalStore();self.store.sparse=True;self.kv=KV();self.app=Core(CFG,self.store,self.kv)
        self.app.run(parse(dumps({'method':'prompts','namespace':'a','tableName':'t','action':'set','data':PROMPT})))
        for id,data,vec in (('meaning','automobile repair',[1,0]),('word','carpet car car',[0,1])):
            self.app.run(parse(dumps({'method':'upsert','namespace':'a','tableName':'t','id':id,'data':data,'source':id+'.md','embeddings':[vec]})))
        self.store.paid.clear();self.store.calls.clear()
    def search(self,mode='keyword',**more):
        return self.app.run(parse(dumps({'method':'search','namespace':'a','query':'car','mode':mode,**more})))
    def test_semantic_finds_meaning_without_a_keyword_match_and_embeds_once(self):
        got=self.search('semantic')
        self.assertEqual(got['mode'],'semantic');self.assertEqual(got['records'][0]['id'],'meaning')
        self.assertEqual(self.store.paid,[('voyage',{'model':CFG['embedding_model'],'input':['car'],'input_type':'query','output_dimension':2,'output_dtype':'float'})])
        self.assertFalse(any(target=='ai' for target,_ in self.store.paid))
    def test_keyword_is_free_and_keeps_the_word_hit(self):
        got=self.search();self.assertEqual(got['mode'],'keyword')
        self.assertEqual([r['id'] for r in got['records']],['word']);self.assertEqual(self.store.paid,[])
    def test_hybrid_uses_both_real_branches_and_namespace_filters(self):
        got=self.search('hybrid');self.assertEqual(got['mode'],'hybrid')
        self.assertEqual({r['id'] for r in got['records']},{'meaning','word'})
        query=next(body for _,path,body,_ in self.store.calls if path=='/points/query')
        self.assertEqual(query['query'],{'fusion':'rrf'});self.assertEqual(len(query['prefetch']),2)
        self.assertEqual(query['prefetch'][0]['query'],[1,0]);self.assertEqual(query['prefetch'][1]['using'],'text')
        for branch in query['prefetch']:self.assertIn(match('namespace','a'),branch['filter']['must'])
        self.assertEqual(len(self.store.paid),1)
    def test_semantic_needs_no_sparse_index_and_never_falls_back(self):
        self.store.sparse=False
        self.assertEqual(self.search('semantic')['mode'],'semantic')
        paid=len(self.store.paid)
        with self.assertRaisesRegex(RagError,'keyword index'):self.search('hybrid')
        self.assertEqual(len(self.store.paid),paid)
        self.store.fail_paid='voyage'
        with self.assertRaisesRegex(RagError,'Voyage'):self.search('semantic')
        self.assertFalse(any(path=='/points/query' and body.get('using')=='text' for _,path,body,_ in self.store.calls))
    def test_same_dimension_model_drift_precedes_query_embedding(self):
        self.app.cfg['embedding_model']='changed';self.store.model='changed'
        with self.assertRaises(NamedRefusal) as c:self.search('semantic')
        self.assertEqual(c.exception.code,'model_mismatch');self.assertEqual(self.store.paid,[])
    def test_invalid_query_vectors_are_refused_without_search(self):
        for bad in ([True,0],[float('nan'),0],[1],[1e50,0]):
            self.store.calls.clear();self.store.embed=lambda request:{'data':[{'index':0,'embedding':bad}]}
            with self.assertRaises(RagError):self.search('semantic')
            self.assertFalse(any(path=='/points/query' for _,path,_,_ in self.store.calls))

if __name__=='__main__':unittest.main()
