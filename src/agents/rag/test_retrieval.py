"""Query construction and live retrieval with deterministic fake dense/BM25 providers."""
from copy import deepcopy
import math
from pathlib import Path
import sys
import unittest
HERE=Path(__file__).resolve().parent
sys.path[:0]=[str(HERE),str(HERE.parents[1]/'clients/python'),str(HERE.parent/'shared')]
from rules import parse,dumps,match,RagError,NamedRefusal,Refused,Deadline
from core import Core
from test_rag import Store,KV,CFG,PROMPT

class RetrievalStore(Store):
    rerank_answer=None   # None: the fake reverses the candidates; else a function of the request, or an exception to raise
    def rerank(self,request):
        if hasattr(self,'budget'):self.budget.left(30)
        self.paid.append(('rerank',deepcopy(request)))
        if isinstance(self.rerank_answer,Exception):raise self.rerank_answer
        if self.rerank_answer is not None:return self.rerank_answer(request)
        n=len(request['documents'])
        return {'object':'list','data':[{'index':i,'relevance_score':(i+1)/n} for i in reversed(range(n))],'model':request['model']}
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

RERANK='rerank-2.5'
SECRET='provider says: key sk-live-123 rejected'

class Rerank(unittest.TestCase):
    """Rerank reorders the query's own candidate window once, through one paid call, or keeps the fused order and says why."""
    def setUp(self):
        self.store=RetrievalStore();self.store.sparse=True;self.kv=KV();self.app=Core({**CFG,'rerank_model':RERANK},self.store,self.kv)
        for table in ('t','u'):
            self.app.run(parse(dumps({'method':'prompts','namespace':'a','tableName':table,'action':'set','data':PROMPT})))
        for id,data,vec in (('meaning','automobile repair',[1,0]),('word','carpet car car',[0,1]),('third','a car and more car words',[1,1])):
            self.upsert('t',id,data,vec)
        self.upsert('u','other','car car car car',[0,1])
        self.store.paid.clear();self.store.calls.clear()
    def upsert(self,table,id,data,vec,chunks=1):
        self.app.run(parse(dumps({'method':'upsert','namespace':'a','tableName':table,'id':id,'data':data,'source':id+'.md','embeddings':[vec]*chunks})))
    def search(self,mode='hybrid',**more):
        return self.app.run(parse(dumps({'method':'search','namespace':'a','query':'car','mode':mode,'tableName':'t',**more})))
    def reranks(self):return [request for target,request in self.store.paid if target=='rerank']
    def base(self,mode='hybrid',**more):
        got=self.search(mode,**more);self.store.paid.clear();self.store.calls.clear();return got
    def test_rerank_reverses_the_fused_order_in_one_call_and_keeps_every_field(self):
        base=self.base()
        got=self.search(rerank=True)
        self.assertIs(got['reranked'],True);self.assertNotIn('rerank_failure',got);self.assertEqual(got['mode'],'hybrid')
        ids=[r['id'] for r in base['records']]
        self.assertEqual(len(ids),3);self.assertEqual([r['id'] for r in got['records']],ids[::-1])
        self.assertEqual({'meaning','word'}-set(ids),set())   # a meaning-only and a keyword-only match both reach rerank
        calls=self.reranks();self.assertEqual(len(calls),1)
        self.assertEqual(calls[0]['model'],RERANK);self.assertEqual(calls[0]['query'],'car')
        self.assertEqual(calls[0]['documents'],[r['excerpt'] for r in base['records']])
        self.assertEqual([t for t,_ in self.store.paid],['voyage','rerank'])
        for before,after in zip(base['records'][::-1],got['records']):
            self.assertEqual(set(after),set(before))
            self.assertEqual({k:v for k,v in after.items() if k!='scores'},{k:v for k,v in before.items() if k!='scores'})
            self.assertEqual(after['scores']['rrf'],before['scores']['rrf']);self.assertEqual(set(after['scores']),{'rrf','rerank'})
        self.assertEqual([r['scores']['rerank'] for r in got['records']],sorted((r['scores']['rerank'] for r in got['records']),reverse=True))
    def test_filters_hold_before_rerank(self):
        got=self.search(rerank=True)
        self.assertEqual({r['tableName'] for r in got['records']},{'t'})
        self.assertNotIn('car car car car',self.reranks()[0]['documents'])
        self.assertEqual(len(self.reranks()[0]['documents']),3)
    def test_keyword_and_semantic_rerank_too(self):
        for mode in ('keyword','semantic'):
            base=self.base(mode)
            got=self.search(mode,rerank=True)
            self.assertIs(got['reranked'],True);self.assertEqual(got['mode'],mode)
            self.assertEqual([r['id'] for r in got['records']],[r['id'] for r in base['records']][::-1])
            self.store.paid.clear()
    def test_the_candidates_are_the_query_window_of_four_times_the_limit(self):
        for i in range(9):self.upsert('t',f'n{i}','car '+'filler '*i,[0,1])
        for limit,want in ((1,4),(2,8),(16,11)):   # 11 records hold the word: a window wider than the matches sends them all
            self.store.paid.clear()
            got=self.search('keyword',rerank=True,limit=limit)
            self.assertEqual(len(self.reranks()[0]['documents']),want,limit);self.assertEqual(len(got['records']),min(limit,want))
    def test_a_second_page_cannot_widen_the_candidates(self):
        # One record's eight chunks fill the first page with a single record, so the second page adds a full window more.
        self.upsert('t','dense','car '*7600,[0,1],chunks=8)
        for i in range(9):self.upsert('t',f'n{i}','car '+'filler '*i,[0,1])
        self.store.paid.clear()
        self.search('keyword',rerank=True,limit=2)
        pages=[body for _,path,body,_ in self.store.calls if path=='/points/query']
        self.assertEqual(len(pages),2,'the fixture must reach the second page')
        self.assertEqual(len(self.reranks()[0]['documents']),8)
    def test_rerank_off_makes_no_rerank_call(self):
        for more in ({},{'rerank':False}):
            self.store.paid.clear()
            got=self.search(**more)
            self.assertEqual(self.reranks(),[]);self.assertNotIn('reranked',got);self.assertNotIn('rerank_failure',got)
            self.assertEqual(set(got['records'][0]['scores']),{'rrf'})
    def test_each_failure_keeps_the_fused_order_and_names_its_own_code(self):
        def answer(rows,**more):return lambda request:{'data':rows(len(request['documents'])),**more}
        good=lambda n:[{'index':i,'relevance_score':i/n} for i in range(n)]
        cases={
            'provider_unavailable':RagError(SECRET),
            'provider_refused':Refused(SECRET),
            'deadline':Deadline(SECRET),
            'model_mismatch':answer(good,model='rerank-lite'),
            'invalid_answer':[
                lambda request:SECRET,
                lambda request:{'model':RERANK},
                answer(lambda n:good(n)[:-1]),
                answer(lambda n:good(n)+[{'index':0,'relevance_score':1}]),
                answer(lambda n:[{'index':0,'relevance_score':.1}]*n),
                answer(lambda n:good(n)[:-1]+[{'index':n,'relevance_score':.9}]),
                answer(lambda n:good(n)[:-1]+[{'index':-1,'relevance_score':.9}]),
                answer(lambda n:good(n)[:-1]+[{'index':True,'relevance_score':.9}]),
                answer(lambda n:good(n)[:-1]+[{'index':n-1,'relevance_score':'0.9'}]),
                answer(lambda n:good(n)[:-1]+[{'index':n-1,'relevance_score':True}]),
                answer(lambda n:good(n)[:-1]+[{'index':n-1,'relevance_score':float('nan')}]),
                answer(lambda n:good(n)[:-1]+[{'index':n-1}]),
                answer(lambda n:SECRET),
            ]}
        base=self.base()
        for code,shapes in cases.items():
            for shape in shapes if isinstance(shapes,list) else [shapes]:
                with self.subTest(code=code,shape=shape):
                    self.store.rerank_answer=shape;self.store.paid.clear()
                    got=self.search(rerank=True)
                    self.assertIs(got['reranked'],False);self.assertEqual(got['rerank_failure'],code)
                    self.assertEqual(got['records'],base['records'])
                    self.assertNotIn('sk-live',dumps(got));self.assertEqual(len(self.reranks()),1)
    def test_an_unrepresentable_score_is_an_invalid_answer_not_a_crash(self):
        # JSON integers have no size limit; 10**400 has no float and must not escape as an exception.
        base=self.base()
        for score in (10**400,-10**400):
            self.store.rerank_answer=lambda request,score=score:{'data':[{'index':i,'relevance_score':score if i==0 else .5} for i in range(len(request['documents']))]}
            try:got=self.search(rerank=True)
            except Exception as error:self.fail(f'score {str(score)[:8]}... escaped as {type(error).__name__}')
            self.assertEqual((got['reranked'],got['rerank_failure'],got['records']),(False,'invalid_answer',base['records']))
        self.store.rerank_answer=lambda request:{'data':[{'index':i,'relevance_score':10**300 if i==0 else i} for i in range(len(request['documents']))]}
        got=self.search(rerank=True)
        self.assertIs(got['reranked'],True);self.assertEqual(got['records'][0]['id'],base['records'][0]['id'])
        self.assertEqual(got['records'][0]['scores']['rerank'],1e300)
    def test_an_oversized_request_is_not_sent(self):
        import core
        saved=core.BODY_MAX;core.BODY_MAX=200
        try:got=self.search(rerank=True)
        finally:core.BODY_MAX=saved
        self.assertEqual((got['reranked'],got['rerank_failure']),(False,'request_too_large'));self.assertEqual(self.reranks(),[])
    def test_no_candidates_pay_nothing(self):
        got=self.app.run(parse(dumps({'method':'search','namespace':'a','query':'zebra','mode':'keyword','rerank':True})))
        self.assertEqual((got['records'],got['reranked']),([],True));self.assertEqual(self.store.paid,[])
    def test_rerank_without_a_model_or_on_external_keywords_is_refused_before_any_paid_call(self):
        self.app.cfg['rerank_model']=''
        with self.assertRaises(NamedRefusal) as c:self.search('semantic',rerank=True)
        self.assertEqual(c.exception.code,'rerank_unavailable');self.assertEqual(self.store.paid,[])
        self.app.cfg.update(rerank_model=RERANK,keyword_search='external')
        with self.assertRaises(NamedRefusal) as c:self.search('keyword',rerank=True)
        self.assertEqual(c.exception.code,'rerank_unavailable');self.assertEqual(self.store.paid,[])
    def test_rerank_must_be_boolean_and_the_model_valid(self):
        for bad in (1,'true',None):
            with self.assertRaisesRegex(RagError,'rerank must be boolean'):parse(dumps({'method':'search','namespace':'a','query':'car','rerank':bad}))
        for bad in ('has space',7,'x'*129):
            with self.assertRaisesRegex(RagError,'rerank model'):Core({**CFG,'rerank_model':bad},self.store,self.kv)
        self.assertEqual(Core(CFG,self.store,self.kv).cfg['rerank_model'],'')

if __name__=='__main__':unittest.main()
