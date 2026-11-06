import contextlib
from copy import deepcopy
import io
import json
import os
from pathlib import Path
import struct
import sys
import tempfile
import unittest
import unittest.mock
from unittest.mock import Mock

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
sys.path[:0]=[str(HERE),str(ROOT/'src/clients/python'),str(ROOT/'src/agents/shared')]
from rules import *
from chunks import extract
from prompts import Cache
from core import Core
from namespaces import IDENTITY_FIELDS,SCHEMA

CFG={'qdrant':'#qdrant@fixture','voyage':'#voyage@fixture','ai':'#ai@fixture',
     'collection':'notes','embedding_model':'voyage-3.5','dimensions':2}
PROMPT={'labels':{'drink':'Beverages','other':'Other'},'instructions':'Choose the topic.'}


class KV:
    def __init__(self):self.values={};self.reads=0;self.fail=False
    def get(self,k):self.reads+=1;return deepcopy(self.values.get(k))
    def set(self,k,v):
        self.values[k]=deepcopy(v)
        if self.fail:raise RagError('uncertain KV write')
    def delete(self,k):
        self.values.pop(k,None)
        if self.fail:raise RagError('uncertain KV write')


class Store:
    def __init__(self,dimensions=2):
        self.points={};self.calls=[];self.paid=[];self.fail=None;self.fail_paid=None;self.fail_get=False
        self.enforce_projection=False
        self.sparse=False;self.refuse_vector=False;self.modifier='idf';self.dimensions=dimensions;self.indexes={k:{'data_type':'keyword'} for k in INDEXES}
        self.model='voyage-3.5';self.probabilities={'drink':.8,'other':.2}
    def begin(self,budget):self.budget=budget
    def decide(self,request):
        if hasattr(self,'budget'):self.budget.left(30)
        self.paid.append(('ai',deepcopy(request)))
        if self.fail_paid=='ai':raise RagError('fake AI unavailable')
        return {'answers':{'labels':{'type':'choice','probabilities':deepcopy(self.probabilities)}}}
    def embed(self,request):
        if hasattr(self,'budget'):self.budget.left(30)
        self.paid.append(('voyage',deepcopy(request)))
        if self.fail_paid=='voyage':raise RagError('fake Voyage unavailable')
        return {'data':[{'index':i,'embedding':[.5]*self.dimensions} for i in range(len(request['input']))]}
    def matches(self,point,condition):
        def check(c):
            if 'is_empty' in c:return point['payload'].get(c['is_empty']['key']) is None
            if 'has_vector' in c:return isinstance(point['vector'],dict) and c['has_vector'] in point['vector']
            if 'has_id' in c:return point['id'] in c['has_id']
            return point['payload'].get(c['key'])==c['match']['value']
        return all(check(c) for c in condition.get('must',[])) and not any(check(c) for c in condition.get('must_not',[]))
    def qdrant(self,verb,path,body=None,write=False):
        if hasattr(self,'budget'):self.budget.left(10)
        body=deepcopy(body);self.calls.append((verb,path,body,write))
        failure=self.fail(verb,path,body) if self.fail else False
        if isinstance(failure,Exception):raise failure
        if failure:raise Unknown('fake uncertain storage failure')
        if path=='':
            params={'vectors':{'size':self.dimensions,'distance':'Cosine'}}
            if self.sparse:params['sparse_vectors']={'text':{'modifier':self.modifier} if self.modifier else {}}
            return {'status':'green','config':{'params':params,'metadata':{'embedding_model':self.model}},'payload_schema':deepcopy(self.indexes)}
        if path.startswith('/points/') and verb=='GET':
            point=self.points.get(path.split('?')[0].split('/')[-1])
            return deepcopy(point) if point else None
        if verb=='POST' and path=='/points/query':
            # A sparse dot product over the stored `text` vectors, as the idf-less core of Qdrant's own.
            want=dict(zip(body['query']['indices'],body['query']['values']))
            scored=[]
            for p in self.points.values():
                if not self.matches(p,body['filter']) or not isinstance(p['vector'],dict) or body['using'] not in p['vector']:continue
                have=p['vector'][body['using']]
                score=sum(want[i]*w for i,w in zip(have['indices'],have['values']) if i in want)
                if score>0:scored.append((score,p))
            scored.sort(key=lambda x:(-x[0],x[1]['id']))
            keep=body['with_payload']['include']
            start=body.get('offset',0)
            return {'points':[{'id':p['id'],'score':s,'payload':{k:v for k,v in p['payload'].items() if k in keep}} for s,p in scored[start:start+body['limit']]]}
        if verb=='POST' and path=='/points/count':
            return {'count':sum(1 for p in self.points.values() if self.matches(p,body['filter']))}
        if verb=='PUT' and path=='/vectors/text':
            if self.refuse_vector:raise Refused('Qdrant grant missing: requires admin role')
            self.sparse=True;self.modifier=body['sparse']['modifier'];return {'status':'completed'}
        if verb=='PUT' and path.startswith('/points/vectors'):
            for update in body['points']:
                point=self.points[update['id']]
                dense=point['vector'] if isinstance(point['vector'],list) else point['vector']['']
                point['vector']={'':dense,**update['vector']}
            return {'status':'completed'}
        if verb=='PUT':
            if '?wait=true' not in path:raise AssertionError('wait=true required')
            for point in body['points']:
                if not self.sparse and isinstance(point['vector'],dict):raise AssertionError('no named vectors in a dense-only collection')
            self.points.update({p['id']:deepcopy(p) for p in body['points']})
        elif '/payload?' in path:
            for p in self.points.values():
                if self.matches(p,body['filter']):p['payload'].update(body['payload'])
        elif '/delete?' in path:
            if 'points' in body:self.points={pid:p for pid,p in self.points.items() if pid not in body['points']}
            else:self.points={pid:p for pid,p in self.points.items() if not self.matches(p,body['filter'])}
        elif path=='/points':
            rows=[]
            for p in body['ids']:
                if p not in self.points:continue
                row={'id':p,'vector':deepcopy(self.points[p]['vector'])}
                if body.get('with_payload'):
                    keep=body['with_payload']['include'] if isinstance(body['with_payload'],dict) else None
                    row['payload']={k:deepcopy(v) for k,v in self.points[p]['payload'].items() if keep is None or k in keep}
                rows.append(row)
            return rows
        elif path=='/points/scroll':
            if self.enforce_projection:
                assert body['with_payload']=={'include':list(dict.fromkeys([*SUMMARY_FIELDS,*IDENTITY_FIELDS]))}, 'scroll must project summaries before proxy reply cap'
                assert body['with_vector'] is False, 'scroll cannot retrieve vectors'
            values=[p for p in self.points.values() if self.matches(p,body['filter'])]
            values.sort(key=lambda p:p['id'])
            if 'offset' in body:values=[p for p in values if p['id']>=body['offset']]
            following=values[body['limit']]['id'] if len(values)>body['limit'] else None
            values=values[:body['limit']]
            projected=[]
            for p in values:
                payload=p['payload']
                if isinstance(body['with_payload'],dict):payload={k:payload[k] for k in body['with_payload']['include'] if k in payload}
                projected.append({'id':p['id'],'payload':deepcopy(payload),**({'vector':deepcopy(p['vector'])} if body.get('with_vector') else {})})
            return {'points':projected,'next_page_offset':following}
        else:raise AssertionError('unexpected fake Qdrant operation')
        return {'status':'completed'}


class Keyword(unittest.TestCase):
    """Built-in keyword search: Qdrant's idf sparse vector named text, filled at upsert and queried by word."""
    def setUp(self):
        self.store=Store();self.store.sparse=True;self.kv=KV();self.app=Core(CFG,self.store,self.kv)
        self.app.cache.set('team','notes',PROMPT)
    def put(self,id,data,**more):
        v={'method':'upsert','namespace':'team','tableName':'notes','id':id,'data':data,'source':'notes/'+id,**more}
        return self.app.run(parse(dumps(v)))
    def find(self,query,**more):
        return self.app.run(parse(dumps({'method':'search','namespace':'team','query':query,**more})))
    def test_the_setting_defaults_to_builtin_and_only_builtin_or_external_pass(self):
        self.assertEqual(config(CFG)['keyword_search'],'builtin')
        self.assertEqual(config({**CFG,'keyword_search':'external'})['keyword_search'],'external')
        for bad in ('Builtin','bm25','',None,True,['builtin']):
            with self.assertRaisesRegex(RagError,'builtin or external'):config({**CFG,'keyword_search':bad})
    def test_a_chunk_is_stored_with_its_terms_beside_its_dense_vector(self):
        self.put('tea','Tea is a drink. Tea!')
        chunk=next(p for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk')
        self.assertEqual(set(chunk['vector']),{'','text'})
        self.assertEqual(chunk['vector'][''],[.5,.5])
        self.assertEqual(chunk['vector']['text'],sparse('Tea is a drink. Tea!'))
        indices=chunk['vector']['text']['indices']
        self.assertEqual(indices,sorted(set(indices)))
    def test_a_collection_without_the_sparse_vector_keeps_plain_vectors_and_refuses_search(self):
        self.store.sparse=False
        self.put('tea','Tea is a drink.')
        chunk=next(p for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk')
        self.assertEqual(chunk['vector'],[.5,.5])
        with self.assertRaisesRegex(RagError,'keyword index not built yet'):self.find('tea')
    def test_get_still_returns_the_dense_vectors_of_a_collection_with_named_ones(self):
        self.put('tea','Tea is a drink.')
        try:got=self.app.run(parse(dumps({'method':'get','namespace':'team','tableName':'notes','id':'tea'})))
        except RagError as error:self.fail(f'get refused a collection whose vectors come back by name: {error}')
        self.assertEqual(got['embeddings'],[[.5,.5]])
    def test_the_terms_are_lower_cased_words_hashed_to_ascending_unique_indices(self):
        a,b=term_index('tea'),term_index('drink')
        self.assertEqual(tokens('Tea, TEA! drink_2 x'),['tea','tea','drink_2','x'])
        self.assertEqual(term_index('tea'),term_index('TEA'.casefold()))
        query=sparse('Tea, TEA drink',query=True)
        self.assertEqual(query,{'indices':sorted([a,b]),'values':[1.0,1.0]})
        doc=sparse('tea tea')   # tf 2, two tokens: 2*2.2/(2+1.2*(0.25+0.75*2/128))
        self.assertEqual(doc['indices'],[a]);self.assertAlmostEqual(doc["values"][0],1.901418,places=5)
        mixed=sparse('tea drink tea')
        self.assertEqual(dict(zip(mixed['indices'],mixed['values'])).keys(),{a,b})
        self.assertGreater(dict(zip(mixed['indices'],mixed['values']))[a],dict(zip(mixed['indices'],mixed['values']))[b])
        self.assertEqual(sparse('',query=True),{'indices':[],'values':[]})
    def test_a_sparse_vector_without_the_idf_modifier_is_not_bm25(self):
        for modifier in (None,'none'):
            self.store.modifier=modifier
            self.app=Core(CFG,self.store,self.kv);self.app.cache.set('team','notes',PROMPT)
            self.put('tea','Tea is a drink.')
            chunk=next(p for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk')
            self.assertEqual(chunk['vector'],[.5,.5])
            with self.assertRaisesRegex(RagError,'lacks the idf modifier'):self.find('tea')
    def test_an_answer_outside_the_asked_scope_or_shape_is_refused(self):
        self.put('tea','Tea is a drink.')
        real=self.store.qdrant
        def lying(change):
            def qdrant(verb,path,body=None,write=False):
                answer=real(verb,path,body,write)
                if path=='/points/query':
                    for row in answer['points']:row['payload'].update(change)
                return answer
            return qdrant
        self.put('keyed','Tea with a key.',key1='y')
        for change,more in (({'namespace':'other'},{}),({'tableName':'elsewhere'},{'tableName':'notes'}),({'key1':'x'},{'key1':'y'})):
            self.store.qdrant=lying(change)
            with self.assertRaisesRegex(RagError,'scope_mismatch|identity mismatch|projection|filter mismatch'):self.find('tea',**more)
        self.store.qdrant=real
    def test_a_draft_whose_head_never_landed_is_never_searched(self):
        self.put('tea','Tea is a drink.')
        calls=[]
        def fail(verb,path,body):
            if verb=='PUT' and path=='/points?wait=true':
                calls.append(1)
                return len(calls)==2   # the head, after its chunks
            return False
        self.store.fail=fail
        with self.assertRaisesRegex(Unknown,'unknown'):self.put('saffron','Saffron is a rare spice.')
        self.store.fail=None
        with self.assertRaisesRegex(NamedRefusal,'not_found'):self.app.run(parse(dumps({'method':'get','namespace':'team','tableName':'notes','id':'saffron'})))
        self.assertTrue(any(p['payload'].get('_rag_kind')=='chunk' and p['payload'].get('id')=='saffron' for p in self.store.points.values()))   # the draft is stored
        self.assertEqual(self.find('saffron')['records'],[])
        self.assertEqual([r['id'] for r in self.find('tea')['records']],['tea'])
    def test_a_replaced_revision_whose_chunks_were_not_removed_is_hidden(self):
        self.put('tea','Tea is a drink.')
        self.store.fail=lambda verb,path,body:verb=='POST' and '/delete?' in path   # the old revision's removal fails
        with self.assertRaisesRegex(Unknown,'unknown'):self.put('tea','Coffee is a drink.')
        self.store.fail=None
        chunks=[p for p in self.store.points.values() if p['payload'].get('_rag_kind')=='chunk']
        self.assertEqual(len(chunks),2)   # both revisions are stored
        self.assertEqual(self.find('tea')['records'],[])
        found=self.find('coffee')['records']
        self.assertEqual([(r['id'],r['excerpt']) for r in found],[('tea','Coffee is a drink.')])
        self.assertEqual(len(self.find('drink')['records']),1)
    def test_a_deleted_record_whose_chunks_remain_is_hidden(self):
        self.put('tea','Tea is a drink.')
        self.store.fail=lambda verb,path,body:verb=='POST' and '/delete?' in path
        with self.assertRaises(RagError):self.app.run(parse(dumps({'method':'delete','namespace':'team','tableName':'notes','id':'tea'})))
        self.store.fail=None
        self.assertEqual(self.find('tea')['records'],[])
    def test_dead_chunks_that_outrank_live_ones_do_not_crowd_them_out(self):
        self.put('live','tea')
        self.drafts(6)   # drafts score higher than the live record
        self.assertEqual([r['id'] for r in self.find('tea',limit=1)['records']],['live'])
    def build(self,roles):
        try:return self.app.run({'method':'keyword_index','namespace':'team'},roles)
        except RagError as error:self.fail(f'keyword_index refused: {error}')
    def drafts(self,count):
        template=deepcopy(next(p for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk'))
        for n in range(count):
            payload=deepcopy(template['payload']);payload.update(id=f'draft{n}',_rag_rev='d'*64,_rag_index=0,_rag_text='tea tea tea tea')
            payload.update(source=f'draft{n}.md',content_hash=hashlib.sha256(payload['_rag_text'].encode()).hexdigest())
            pid=point_id(payload,payload['_rag_rev'],0)
            self.store.points[pid]={'id':pid,'vector':{'':[.5,.5],'text':sparse(payload['_rag_text'])},'payload':payload}
    def test_a_window_filled_by_dead_chunks_is_an_incomplete_answer_never_an_empty_one(self):
        self.put('live','tea')
        self.drafts(15)   # the live chunk is the 16th of the 16 rows the four pages read
        self.assertEqual([r['id'] for r in self.find('tea',limit=1)['records']],['live'])
        self.drafts(16)   # now it is the 17th: beyond the window
        with self.assertRaisesRegex(Incomplete,'search window exhausted before enough records were found'):self.find('tea',limit=1)
        self.assertEqual(self.find('zebra')['records'],[])   # a real "no match" is still an empty answer
    def test_a_half_built_index_is_refused_until_it_is_whole(self):
        self.store.sparse=False
        self.app=Core(CFG,self.store,self.kv);self.app.cache.set('team','notes',PROMPT)
        for n in range(70):self.put(f'doc{n:02d}',f'Tea number {n}.')
        owner={'roles':['owner']};updates=[]
        def fail(verb,path,body):
            if verb=='PUT' and path.startswith('/points/vectors'):
                updates.append(1);return len(updates)==2
            return False
        self.store.fail=fail
        with self.assertRaises(RagError):self.app.run({'method':'keyword_index','namespace':'team'},owner)
        self.store.fail=None
        with self.assertRaisesRegex(RagError,r'keyword index incomplete \(6 chunks left\); run keyword_index with this namespace'):self.find('tea')
        with self.assertRaisesRegex(RagError,'keyword index incomplete'):self.find('tea')   # still refused on the next search, not just the first
        self.assertEqual(self.build(owner),{'status':'ok','indexed':6})
        self.assertEqual(len(self.find('tea',limit=16)['records']),16)
        counts=len([c for c in self.store.calls if c[1]=='/points/count' and any('has_vector' in x for x in c[2]['filter'].get('must_not',[]))]);self.find('tea');self.find('number')
        self.assertEqual(len([c for c in self.store.calls if c[1]=='/points/count' and any('has_vector' in x for x in c[2]['filter'].get('must_not',[]))]),counts)   # a whole index is not counted again
    def test_a_zero_count_is_trusted_for_a_minute_and_then_counted_again(self):
        self.put('tea','Tea is a drink.')
        now=[1000.0]
        with unittest.mock.patch('core.time.monotonic',lambda:now[0]):
            self.assertEqual(len(self.find('tea')['records']),1)
            counts=lambda:len([c for c in self.store.calls if c[1]=='/points/count' and any('has_vector' in x for x in c[2]['filter'].get('must_not',[]))])
            first=counts()
            for n in range(2):   # chunks another writer (an older build, a restore) stored without the vector
                pid=f'00000000-0000-0000-0000-0000000000a{n}'
                self.store.points[pid]={'id':pid,'vector':[.5,.5],'payload':{'namespace':'team','tableName':'notes','id':f'late{n}','_rag_kind':'chunk','_rag_text':'late'}}
            now[0]+=30
            try:self.find('tea')
            except RagError as error:self.fail(f'search recounted within the minute: {error}')
            self.assertEqual(counts(),first)   # within the minute: not counted again
            now[0]+=31
            with self.assertRaisesRegex(RagError,r'keyword index incomplete \(2 chunks left\)'):self.find('tea')
            self.assertEqual(counts(),first+1)
    def test_keyword_index_adds_the_vector_and_gives_every_chunk_its_words_and_is_safe_to_repeat(self):
        self.store.sparse=False
        self.app=Core(CFG,self.store,self.kv);self.app.cache.set('team','notes',PROMPT)
        for n in range(3):self.put(f'doc{n}',f'Tea number {n}. Saffron is a spice.' if n==1 else f'Tea number {n}.')
        with self.assertRaisesRegex(RagError,'keyword index not built yet; run \\{"method":"keyword_index"\\}'):self.find('tea')
        owner={'roles':['owner']}
        first=self.build(owner)
        self.assertEqual(first,{'status':'ok','indexed':3})
        self.assertTrue(self.store.sparse and self.store.modifier=='idf')
        self.assertEqual(sorted(r['id'] for r in self.find('tea')['records']),['doc0','doc1','doc2'])
        self.assertEqual([r['id'] for r in self.find('saffron')['records']],['doc1'])
        self.assertEqual(self.build(owner),{'status':'ok','indexed':0})
        # the dense vectors came through the update untouched
        chunk=next(p for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk')
        self.assertEqual(chunk['vector'][''],[.5,.5])
    def test_keyword_index_resumes_after_an_interruption_without_redoing_chunks(self):
        self.store.sparse=False
        self.app=Core(CFG,self.store,self.kv);self.app.cache.set('team','notes',PROMPT)
        for n in range(70):self.put(f'doc{n:02d}',f'Tea number {n}.')
        owner={'roles':['maintainer']};updates=[]
        def fail(verb,path,body):
            if verb=='PUT' and path.startswith('/points/vectors'):
                updates.append(len(body['points']))
                return len(updates)==2
            return False
        self.store.fail=fail
        with self.assertRaises(RagError):self.app.run({'method':'keyword_index','namespace':'team'},owner)
        self.store.fail=None
        done=sum(1 for p in self.store.points.values() if isinstance(p['vector'],dict) and 'text' in p['vector'])
        self.assertEqual(done,64)   # the first batch stayed
        updates.clear()
        again=self.build(owner)
        self.assertEqual(again,{'status':'ok','indexed':6})   # only the chunks still without the vector
        self.assertEqual(len(self.find('tea',limit=16)['records']),16)
    def test_keyword_index_keeps_every_update_under_the_proxys_body_cap(self):
        self.store.sparse=False
        self.app=Core(CFG,self.store,self.kv);self.app.cache.set('team','notes',PROMPT)
        words=' '.join(f'{n:03x}' for n in range(1000))   # one 4 KB chunk of 1,000 distinct terms: about 22 KB of vector
        for n in range(70):self.put(f'doc{n:02d}',words)
        sizes=[]
        real=self.store.qdrant
        def watch(verb,path,body=None,write=False):
            if verb=='PUT' and path.startswith('/points/vectors'):sizes.append(len(dumps(body)))
            return real(verb,path,body,write)
        self.store.qdrant=watch
        self.assertEqual(self.build({'roles':['owner']}),{'status':'ok','indexed':70})
        self.assertGreater(len(sizes),2)   # one page of 64 would be about 1.4 MB, so it was split
        self.assertLess(max(sizes),(1<<20)-4096)
    def test_keyword_index_is_for_the_owner_and_maintainers_and_only_in_builtin_mode(self):
        for roles in ({'roles':[]},{'roles':['write']},{}):
            with self.assertRaisesRegex(RagError,'Owner or Maintainers'):self.app.run({'method':'keyword_index','namespace':'team'},roles)
        self.assertEqual(self.store.points,{})
        self.app=Core({**CFG,'keyword_search':'external'},self.store,self.kv)
        with self.assertRaisesRegex(RagError,'external'):self.app.run({'method':'keyword_index','namespace':'team'},{'roles':['owner']})
    def test_a_proxy_without_the_admin_role_names_the_owners_step(self):
        self.store.sparse=False;self.store.refuse_vector=True
        self.app=Core(CFG,self.store,self.kv)
        with self.assertRaisesRegex(RagError,'admin role.*request PUT /collections/notes/vectors/text .*repeat this command'):self.app.run({'method':'keyword_index','namespace':'team'},{'roles':['owner']})
        self.assertFalse(self.store.sparse)
    def test_keyword_index_requires_only_its_namespace(self):
        parse(dumps({'method':'keyword_index','namespace':'team'}))
        for extra in ({'limit':1},{'all':True}):
            with self.assertRaises(RagError):parse(dumps({'method':'keyword_index','namespace':'team',**extra}))
        with self.assertRaises(RagError):parse(dumps({'method':'keyword_index'}))
    def test_search_ranks_by_the_words_and_returns_one_row_per_record(self):
        self.put('tea','Tea is a drink made from leaves.')
        self.put('cats','Cats sleep. Tea is not for cats.')
        self.put('long','tea '*1500+'x'*100)   # more than one chunk, all matching
        answer=self.find('tea leaves')
        ids=[r['id'] for r in answer['records']]
        self.assertEqual(ids[0],'tea')
        self.assertEqual(sorted(ids),['cats','long','tea'])
        self.assertEqual(len(ids),len(set(ids)))
        self.assertTrue(all(set(r)>={'namespace','tableName','id','scores','excerpt','source','content_hash','indexed_at','chunk_index'} for r in answer['records']))
        self.assertEqual(answer['records'][0]['excerpt'],'Tea is a drink made from leaves.')
        one=self.find('tea leaves',limit=1)['records']
        self.assertEqual([r['id'] for r in one],['tea'])
        self.assertEqual(len(self.find('tea leaves',limit=2)['records']),2)
        self.assertEqual(self.find('zebra')['records'],[])
    def test_the_query_is_sent_as_query_terms_over_chunks_in_the_callers_scope(self):
        self.put('tea','Tea is a drink.',key1='hot');self.store.calls.clear()
        self.find('Tea, TEA drink',tableName='notes',key1='hot',limit=3)
        verb,path,body,write=next(c for c in self.store.calls if c[1]=='/points/query')
        self.assertEqual((verb,path,write),('POST','/points/query',False))
        self.assertEqual(body['using'],'text');self.assertEqual(body['limit'],12)
        self.assertEqual(body['query'],sparse('tea drink',query=True))
        self.assertEqual(body['filter'],self.app.catalogue.bind('team',self.app.cache).filter(scope({'namespace':'team','tableName':'notes','key1':'hot'},kind='chunk')))
        self.assertEqual(self.find('tea',key1='other')['records'],[])
    def test_another_namespace_never_matches(self):
        self.put('tea','Tea is a drink.')
        self.assertEqual(self.app.run(parse(dumps({'method':'search','namespace':'other','query':'tea'})))['records'],[])
    def test_search_requests_are_parsed_strictly(self):
        base={'method':'search','namespace':'team','query':'tea'}
        parse(dumps(base))
        for change in ({'query':''},{'query':'  !!! '},{'query':'x'*1025},{'query':5},{'limit':0},{'limit':17},{'limit':True},{'id':'tea'},{'data':'x'}):
            with self.assertRaises(RagError):parse(dumps({**base,**change}))
        del base['query']
        with self.assertRaises(RagError):parse(dumps(base))
    def test_external_mode_asks_the_connected_engine_and_never_builtin(self):
        self.app=Core({**CFG,'keyword_search':'external'},self.store,self.kv)
        self.app.cache.set('team','notes',PROMPT);self.put('tea','Tea is a drink.')
        chunk=next(p for p in self.store.points.values() if p['payload']['_rag_kind']=='chunk')
        self.assertEqual(chunk['vector'],[.5,.5])   # nothing built-in is written
        with self.assertRaisesRegex(RagError,'no external keyword engine'):self.find('tea')
        self.assertFalse(any(c[1]=='/points/query' for c in self.store.calls))
        seen=[]
        head=next(p['payload'] for p in self.store.points.values() if p['payload']['_rag_kind']=='head')
        evidence={k:head[k] for k in SUMMARY_FIELDS if k in head}
        evidence.update(chunk_index=0,excerpt=chunk['payload']['_rag_text'],scores={'bm25':1.0})
        self.app.keyword=lambda request:seen.append(request) or {'records':[evidence]}
        self.assertEqual(self.find('tea')['records'],[evidence])
        self.assertEqual(seen[0]['query'],'tea')
        self.app.keyword=lambda request:'nonsense'
        with self.assertRaisesRegex(RagError,'invalid external'):self.find('tea')
        self.app.keyword=lambda request:{'records':[evidence|{'excerpt':'fabricated'}]}
        with self.assertRaisesRegex(RagError,'provenance mismatch'):self.find('tea')



class Rag(unittest.TestCase):
    def setUp(self):
        self.store=Store();self.kv=KV();self.app=Core(CFG,self.store,self.kv)
        self.app.cache.set('team','notes',PROMPT)
    def req(self,method='upsert',**changes):
        v={'method':method,'namespace':'team'}
        if method in ('upsert','get'):v.update(tableName='notes',id='tea')
        if method=='upsert':v.update(data='Tea is a drink.',source='notes/tea.md')
        v.update(changes);return parse(dumps(v))
    def call(self,method='upsert',**changes):return self.app.run(self.req(method,**changes))
    def test_repeated_upsert_get_and_shrinking_chunks(self):
        first=self.call(data='x'*5000)
        self.assertEqual(len(first['embeddings']),2)
        self.assertEqual(len(self.store.points),3)
        ids=set(self.store.points)
        repeated=self.call(data='x'*5000)
        self.assertEqual({k:v for k,v in repeated.items() if k!='indexed_at'},{k:v for k,v in first.items() if k!='indexed_at'})
        self.assertEqual(len(self.store.points),3)
        self.assertEqual(self.call('get'),repeated)
        self.call(data='short',key1='new')
        self.assertEqual(len(self.store.points),2)
        self.assertEqual(self.call('get')['data'],'short')
        self.call(data='short')
        self.assertNotIn('key1',self.call('get'))
        self.assertEqual([x[0] for x in self.store.paid].count('ai'),4)
        head=self.store.points[point_id(self.req())]['payload']
        self.assertNotIn('embeddings',head)
    def test_failed_new_upsert_and_provider_failures_keep_previous_record(self):
        old=self.call(data='old')
        saved=deepcopy(self.store.points)
        for provider in ['ai','voyage']:
            self.store.fail_paid=provider
            with self.assertRaises(RagError):self.call(data='new')
            self.assertEqual(self.store.points,saved)
        self.store.fail_paid=None
        self.store.fail=lambda verb,path,body:verb=='PUT'
        with self.assertRaisesRegex(Unknown,'unknown'):self.call(data='new')
        self.assertEqual(self.store.points,saved)
        self.store.fail=None
        self.assertEqual(self.call('get'),old)
    def test_retry_repairs_orphan_revisions_after_stale_delete_failure(self):
        self.call(data='old')
        self.store.fail=lambda verb,path,body:'/delete?' in path
        with self.assertRaises(Unknown):self.call(data='new')
        self.assertEqual(len(self.store.points),3)
        self.store.fail=None
        self.call(data='new')
        self.assertEqual(len(self.store.points),2)
        head=self.store.points[point_id(self.req())]['payload']
        self.assertTrue(all(pid in self.store.points for pid in head['_rag_chunks']))
        body=[c[2] for c in self.store.calls if '/delete?' in c[1]][-1]
        self.assertIn('filter',body)
        cleanup=body['filter']
        self.assertEqual(cleanup['must'][:4],[match('namespace','team'),match('namespace','team'),match('tableName','notes'),match('id','tea')])
        self.assertEqual(cleanup['must_not'][0]['key'],'_rag_rev')
        self.assertNotIn('has_id',dumps(cleanup))
    def test_delete_hides_head_before_failed_chunk_cleanup_and_repeats(self):
        self.call(data='old')
        self.store.fail=lambda verb,path,body:'/delete?' in path and any(c.get('match',{}).get('value')=='chunk' for c in body['filter']['must'])
        with self.assertRaises(Unknown):self.call('delete',tableName='notes',id='tea')
        with self.assertRaisesRegex(NamedRefusal,'not_found'):self.call('get')
        self.assertTrue(any(p['payload']['_rag_kind']=='chunk' for p in self.store.points.values()))
        self.store.fail=None
        self.call('delete',tableName='notes',id='tea')
        self.assertEqual(self.store.points,{})
        self.assertIsNotNone(self.app.cache.get('team','notes'))
    def test_scoped_filtered_delete_and_old_key_revisions(self):
        self.call(data='old',key1='old')
        self.store.fail=lambda verb,path,body:'/delete?' in path
        with self.assertRaises(Unknown):self.call(data='new',key1='new')
        self.store.fail=None
        self.app.cache.set('other','notes',PROMPT)
        self.call(namespace='other',id='untouched')
        self.call('delete',key1='new',all=True)
        remaining=list(self.store.points.values())
        self.assertEqual(len(remaining),2)
        self.assertTrue(all(p['payload']['namespace']=='other' for p in remaining))
    def test_missing_namespace_and_scope_never_match_all(self):
        for v in [{'method':'delete','all':True},{'method':'delete','namespace':'','all':True},
                  {'method':'delete','tableName':'notes','all':True},{'method':'list','namespace':None},
                  {'method':'delete','namespace':'team'},{'method':'delete','namespace':'team','tableName':'notes','id':'x','key1':'x'}]:
            with self.assertRaises(RagError):parse(dumps(v))
        with self.assertRaises(RagError):scope({'tableName':'notes'})
        self.assertEqual(self.store.calls,[])
    def test_projection_page_size_and_opaque_cursor(self):
        for i in range(17):self.call(id=str(i),data='x'*32768)
        # Full heads exceed the proxy cap; projected summaries must fit upstream.
        for point in self.store.points.values():
            if point['payload']['_rag_kind']=='head':
                point['payload']['data']='界'*10000
                point['payload']['classification_ids']={str(i)+'я'*60:.5 for i in range(64)}
        heads=[p for p in self.store.points.values() if p['payload']['_rag_kind']=='head']
        self.assertGreater(len(dumps(heads[:16],ascii=True)),1<<20)
        self.store.enforce_projection=True
        result=self.call('list')
        self.assertEqual(len(result['records']),16)
        self.assertTrue(result['cursor'])
        query=self.store.calls[-1][2]
        self.assertEqual(query,{'filter':{'must':[match('namespace','team'),match('namespace','team'),match('_rag_kind','head')]},'limit':16,
            'with_payload':{'include':list(dict.fromkeys([*SUMMARY_FIELDS,*IDENTITY_FIELDS]))},'with_vector':False})
        self.assertNotIn('data',dumps(result));self.assertNotIn('embeddings',dumps(result))
        self.assertLess(len(dumps(result,ascii=True)),1<<20)
        last=self.call('list',cursor=result['cursor'])
        self.assertEqual(len(last['records']),1)
        for token in ['fake',str(__import__('uuid').uuid4())]:
            with self.assertRaises(RagError):self.call('list',cursor=token)
        with self.assertRaises(RagError):self.call('list',tableName='notes',cursor=result['cursor'])
        self.app.cursors.clear()
        with self.assertRaises(RagError):self.call('list',cursor=result['cursor'])
    def test_indexes_model_dimensions_checked_without_admin_or_paid_work(self):
        for change in ['index','model','dimensions']:
            store=Store()
            if change=='index':store.indexes.pop('_rag_rev')
            if change=='model':store.model='wrong'
            if change=='dimensions':store.dimensions=4
            app=Core(CFG,store,self.kv)
            with self.assertRaises(RagError):app.run(self.req())
            self.assertEqual(store.paid,[]);self.assertEqual(store.points,{})
            self.assertTrue(all(c[0]=='GET' for c in store.calls))
    def test_prepaid_worst_case_and_vector_bounds(self):
        app=Core(CFG|{'dimensions':1024},Store(1024),KV());app.cache.set('team','notes',PROMPT)
        with self.assertRaisesRegex(RagError,'before provider work'):app.run(self.req(data='界'*10000))
        self.assertEqual(app.backend.paid,[])
        app=Core(CFG|{'dimensions':2731},Store(2731),KV());app.cache.set('team','notes',PROMPT)
        with self.assertRaisesRegex(RagError,'8192'):app.run(self.req(data='x'*9000))
        self.assertEqual(app.backend.paid,[])
        for vectors in [[[True,1]],[[float('nan'),0]],[[1e39,0]],[[1]]]:
            v=self.req();v['embeddings']=vectors
            with self.assertRaises(RagError):self.app.run(v)
        self.assertEqual(self.store.paid,[])
    def test_supplied_vectors_labels_and_jev_validation(self):
        self.call(embeddings=[[.5,.25]])
        self.assertEqual([x[0] for x in self.store.paid],['ai'])
        self.assertEqual(self.call('get')['embeddings'],[[.5,.25]])
        self.assertEqual(self.call('get')['classification_ids'],{'drink':.8})
        for probabilities in [{'drink':True,'other':0},{'drink':float('inf'),'other':0},{'alien':.8,'other':.2}]:
            self.store.probabilities=probabilities;saved=deepcopy(self.store.points)
            with self.assertRaises(RagError):self.call(data='new')
            self.assertEqual(self.store.points,saved)
    def test_mime_encoding_and_duplicate_numeric_validation(self):
        v=self.req(data='<style>hide</style><script>secret()</script><p>Tea &amp; coffee</p>',**{'mime-type':'text/html'})
        source,_=extract(v);self.assertNotIn('secret',source);self.assertNotIn('hide',source);self.assertIn('Tea & coffee',source)
        for body in ['{"method":"get","namespace":"a","namespace":"b","tableName":"notes","id":"a"}','{"method":"list","namespace":"a","limit":true}',
                     '{"method":"upsert","namespace":"a","tableName":"b","id":"x","data":"\\u0000"}',
                     '{"method":"list","namespace":"e\\u0301"}','{"method":"list","namespace":"a","limit":NaN}']:
            with self.assertRaises(RagError):parse(body)
        with self.assertRaises(RagError):extract(self.req(data='<a>'*65,**{'mime-type':'text/html'}))
        with self.assertRaises(RagError):parse('{"method":"list","namespace":"\ud800"}')
    def test_prompt_cache_unknown_write_invalidation_and_restart(self):
        a=self.app.cache.get('team','notes');reads=self.kv.reads
        self.app.cache.get('team','notes');self.assertEqual(self.kv.reads,reads)
        self.kv.fail=True
        with self.assertRaises(RagError):self.app.cache.set('team','notes',PROMPT|{'instructions':'Changed'})
        self.assertEqual(self.app.cache.get('team','notes')['instructions'],'Changed')
        with self.assertRaises(RagError):self.app.cache.delete('team','notes')
        self.assertIsNone(self.app.cache.get('team','notes'))
        self.kv.fail=False;self.kv.set(prompt_key('team','notes'),a)
        self.assertIsNone(self.app.cache.get('team','notes'))
        self.assertEqual(Cache(self.kv).get('team','notes')['instructions'],a['instructions'])
        self.assertEqual(len(prompt_key('a','b')),76)
        self.assertNotEqual(prompt_key('a/b','c'),prompt_key('a','b/c'))
    def test_expired_message_does_no_block_or_paid_work(self):
        with self.assertRaises(RagError):self.app.run(self.req(),{'deadline':'2000-01-01T00:00:00Z'})
        self.assertEqual(self.store.calls,[]);self.assertEqual(self.store.paid,[]);self.assertEqual(self.store.points,{})

    def test_threshold_topk_ties_and_manifest_examples(self):
        import shlex
        for probabilities,expected in [({'drink':.5,'other':.5},{'drink':.5}),({'drink':.6,'other':.8},{'other':.8})]:
            self.app.cache.set('team','notes',PROMPT|{'threshold':.5,'top_k':1})
            self.store.probabilities=probabilities
            self.assertEqual(self.call()['classification_ids'],expected)
        manifest=json.loads((HERE/'agent.json').read_text())
        for method,info in manifest['methods'].items():
            value=parse(shlex.split(info['example'])[-1]);self.assertEqual(value['method'],method)
            with self.assertRaises(RagError):parse(dumps(value|{'unknown_record_field':True}))

    def test_same_table_record_ids_never_widen_cleanup(self):
        self.call(id='a');self.call(id='b')
        self.assertEqual(len(self.store.points),4)
        self.call(id='a',data='changed')
        self.assertEqual(len(self.store.points),4)
        self.assertEqual(self.call('get',id='b')['data'],'Tea is a drink.')

    def test_strict_json_parser_rejects_nonfinite_constants(self):
        for raw in ['{"p":NaN}','{"p":Infinity}','{"p":-Infinity}']:
            with self.assertRaises(RagError):loads(raw)

    def test_corrupt_stored_fields_are_refused(self):
        self.call()
        head=self.store.points[point_id(self.req())]['payload']
        original=deepcopy(head)
        for bad in [{'id':7},{'classification_ids':{'drink':True}},{'data':None},{'mime-type':'binary/unknown'}]:
            head.clear();head.update(original);head.update(bad)
            with self.assertRaises(RagError):self.call('get')
        head.clear();head.update(original)
        head['classification_ids']={'drink':False}
        with self.assertRaises(RagError):self.call('list')

    def test_namespace_delete_sweeps_failed_first_publish_orphans(self):
        self.store.fail=lambda verb,path,body:verb=='PUT' and body['points'][0]['payload']['_rag_kind']=='head'
        with self.assertRaises(Unknown):self.call()
        self.assertEqual(len(self.store.points),1)
        self.store.fail=None
        self.call('delete',all=True)
        self.assertEqual(self.store.points,{})
        self.assertEqual(self.store.calls[-1][2],{'filter':{'must':[match('namespace','team'),match('namespace','team'),match('_rag_kind','chunk')]}})

    def test_embedding_dimension_dtype_and_one_classification_per_record(self):
        self.call(data='x'*32768)
        self.assertEqual(len([p for p in self.store.paid if p[0]=='ai']),1)
        batches=[p[1] for p in self.store.paid if p[0]=='voyage']
        self.assertEqual(len(batches),2)
        self.assertTrue(all(b.get('output_dimension')==2 and b.get('output_dtype')=='float' and len(b['input'])==4 for b in batches))
        self.assertTrue(all(len(dumps(b).encode())<1<<20 for b in batches))
        self.assertEqual(self.store.points[point_id(self.req())]['vector'],[1.0,0.0])

    def test_clean_first_refusal_is_not_unknown(self):
        self.call(data='old');old=deepcopy(self.store.points)
        self.store.fail=lambda verb,path,body:Refused('clean 400') if verb=='PUT' else False
        with self.assertRaisesRegex(RagError,'not stored; prior record unchanged') as error:self.call(data='new')
        self.assertNotIsInstance(error.exception,Unknown)
        self.assertEqual(self.store.points,old)

    def test_each_storage_step_failure_converges_on_explicit_repeat(self):
        for operation in ['upsert','delete']:
            for failed in (1,2,3):
                with self.subTest(operation=operation,failed=failed):
                    self.store=Store();self.app=Core(CFG,self.store,self.kv);self.call(data='old')
                    count=0
                    def fail(verb,path,body):
                        nonlocal count
                        if verb in ('POST','PUT') and '?wait=true' in path:
                            count+=1;return count==failed
                        return False
                    self.store.fail=fail
                    with self.assertRaises(Unknown):
                        self.call(data='new') if operation=='upsert' else self.call('delete',tableName='notes',id='tea')
                    self.store.fail=None
                    if operation=='upsert':
                        self.call(data='new');self.assertEqual(len(self.store.points),2)
                        self.assertEqual(self.call('get')['data'],'new')
                    else:
                        self.call('delete',tableName='notes',id='tea');self.assertEqual(self.store.points,{})

    def test_pathological_breaks_refuse_before_paid_calls_but_eight_hard_chunks_fit(self):
        pathological=('x'*2198+'\n\n')*14+'x'*1968
        self.assertEqual(len(pathological.encode()),32768)
        with self.assertRaisesRegex(RagError,'at least 9 chunks; limit 8'):self.call(data=pathological)
        self.assertEqual(self.store.paid,[])
        self.call(data='x'*32768)
        self.assertEqual(len(self.call('get')['embeddings']),8)

    def test_chunk_format_whitespace_utf8_and_html_implicit_tags(self):
        source='a'*3000+'\n\n'+'b'*2000
        _,parts=extract(self.req(data=source))
        self.assertEqual(parts[0],'a'*3000+'\n\n');self.assertEqual(''.join(parts),source)
        source='я'*3000
        _,parts=extract(self.req(data=source))
        self.assertEqual(''.join(parts),source);self.assertTrue(all(len(p.encode())<=4096 for p in parts))
        source='<p>Tea'*100+'<noscript>hide-one</noscript><template>hide-two</template>'
        try:extracted,_=extract(self.req(data=source,**{'mime-type':'text/html'}))
        except RagError:self.fail('valid implicit HTML end tags must not overflow the stack')
        self.assertEqual(extracted.count('Tea'),100);self.assertNotIn('hide-',extracted)

    def test_cursor_expiry_eviction_and_changed_classification_revision(self):
        for i in range(17):self.call(id=str(i))
        first=self.call('list')['cursor']
        entry=self.app.cursors[first];self.app.cursors[first]=(0,*entry[1:])
        with self.assertRaises(RagError):self.call('list',cursor=first)
        first=self.call('list')['cursor']
        for _ in range(129):self.call('list')
        self.assertEqual(len(self.app.cursors),128)
        with self.assertRaises(RagError):self.call('list',cursor=first)
        self.call(id='tea');old=self.store.points[point_id(self.req())]['payload']['_rag_rev']
        self.store.probabilities={'drink':.2,'other':.8};self.call(id='tea')
        self.assertNotEqual(self.store.points[point_id(self.req())]['payload']['_rag_rev'],old)

    def test_get_exact_byte_cap_without_chunk_text_or_vector_duplication(self):
        cfg=CFG|{'dimensions':1024};store=Store(1024);app=Core(cfg,store,self.kv)
        app.catalogue.bind('team',app.cache,'notes')
        v=self.req();rev='a'*64
        ids=[point_id(v,rev,i) for i in range(8)]
        vec=vector([1.1754943508222875e-38]*1024,1024)
        payload={'namespace':'team','tableName':'notes','id':'tea','mime-type':'text/plain',
                 'data':'x'*32768,'classification_ids':{'drink':.8},'source':'notes/tea.md',
                 'content_hash':'0'*64,'indexed_at':'2026-11-05T00:00:00Z'}
        full={**payload,'embeddings':[vec]*8}
        extra=RECORD_MAX-len(dumps(full,ascii=True).encode())
        self.assertGreater(extra,0)
        five,one=divmod(extra,5)
        self.assertLess(five+one,32768)
        payload['data']='\x01'*five+'\n'*one+'x'*(32768-five-one)
        payload['content_hash']=hashlib.sha256(payload['data'].encode()).hexdigest()
        payload.update(_rag_schema=SCHEMA,_rag_kind='head',_rag_rev=rev,_rag_chunks=ids)
        store.points[point_id(v)]={'id':point_id(v),'payload':payload,'vector':vec}
        for i,pid in enumerate(ids):
            chunk={k:payload[k] for k in ('namespace','tableName','id','source','content_hash','indexed_at')}
            chunk.update(_rag_schema=SCHEMA,_rag_kind='chunk',_rag_rev=rev,_rag_index=i,_rag_text='must not retrieve this')
            store.points[pid]={'id':pid,'vector':vec,'payload':chunk}
        result=app.run(self.req('get'))
        self.assertEqual(len(dumps(result,ascii=True).encode()),RECORD_MAX)
        query=store.calls[-1][2]
        self.assertEqual(query['with_payload'],{'include':list(IDENTITY_FIELDS)})
        self.assertTrue(query['with_vector']);self.assertIn({'has_id':ids},query['filter']['must'])
        payload['data']=payload['data'][:-1]+'\n'
        payload['content_hash']=hashlib.sha256(payload['data'].encode()).hexdigest()
        with self.assertRaisesRegex(RagError,'cap'):app.run(self.req('get'))

    def test_get_missing_chunk_reloads_head_once_then_refuses(self):
        self.call()
        pid=self.store.points[point_id(self.req())]['payload']['_rag_chunks'][0]
        del self.store.points[pid]
        before=len(self.store.calls)
        with self.assertRaisesRegex(RagError,'incomplete'):self.call('get')
        reads=[c for c in self.store.calls[before:] if c[1]=='/points/scroll' and c[2]['filter']['must'][-1].get('has_id')==[point_id(self.req())]]
        self.assertEqual(len(reads),2)

    def test_storage_identity_and_citation_revision_vectors(self):
        v=self.req();v.update(content_hash=hashlib.sha256(v['data'].encode()).hexdigest(),indexed_at='2026-11-05T00:00:00Z')
        rev=revision(v,[[.5,.25]],"voyage-3.5",{"drink":.8})
        self.assertEqual(point_id(v),'d73f39e4-7cd4-535d-9805-49fcd10ffe08')
        self.assertEqual(rev,'c9ede491b865fd7b4d15930940eebb765afc4b34a11e7609847e89ea7dd82000')
        self.assertEqual(point_id(v,rev,0),'86c60096-73bd-51ae-9f78-0714e589ad2b')

    def test_ids_are_typed_revision_specific_and_keyword_is_not_claimed(self):
        v=self.req()
        self.assertNotEqual(point_id(v),point_id(v|{'id':'tea/'}))
        a=self.call(data='old');oldchunks=set(self.store.points)-{point_id(v)}
        self.call(data='new');self.assertTrue(oldchunks.isdisjoint(set(self.store.points)-{point_id(v)}))
        keyword=Mock();self.app.keyword=keyword;self.call(data='same');keyword.assert_not_called()


if __name__=='__main__':unittest.main()
