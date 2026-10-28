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
from unittest.mock import Mock

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
sys.path[:0]=[str(HERE),str(ROOT/'src/clients/python'),str(ROOT/'src/agents/shared')]
from rules import *
from chunks import extract
from prompts import Cache
from core import Core

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
        self.dimensions=dimensions;self.indexes={k:{'data_type':'keyword'} for k in INDEXES}
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
            if 'has_id' in c:return point['id'] in c['has_id']
            return point['payload'].get(c['key'])==c['match']['value']
        return all(check(c) for c in condition.get('must',[])) and not any(check(c) for c in condition.get('must_not',[]))
    def qdrant(self,verb,path,body=None,write=False):
        if hasattr(self,'budget'):self.budget.left(10)
        body=deepcopy(body);self.calls.append((verb,path,body,write))
        failure=self.fail(verb,path,body) if self.fail else False
        if isinstance(failure,Exception):raise failure
        if failure:raise Unknown('fake uncertain storage failure')
        if path=='':return {'status':'green','config':{'params':{'vectors':{'size':self.dimensions,'distance':'Cosine'}},
                            'metadata':{'embedding_model':self.model}},'payload_schema':deepcopy(self.indexes)}
        if path.startswith('/points/') and verb=='GET':
            point=self.points.get(path.split('?')[0].split('/')[-1])
            return deepcopy(point) if point else None
        if verb=='PUT':
            if '?wait=true' not in path:raise AssertionError('wait=true required')
            self.points.update({p['id']:deepcopy(p) for p in body['points']})
        elif '/payload?' in path:
            for p in self.points.values():
                if self.matches(p,body['filter']):p['payload'].update(body['payload'])
        elif '/delete?' in path:
            if 'points' in body:self.points={pid:p for pid,p in self.points.items() if pid not in body['points']}
            else:self.points={pid:p for pid,p in self.points.items() if not self.matches(p,body['filter'])}
        elif path=='/points':
            return [{'id':p,'vector':deepcopy(self.points[p]['vector'])} for p in body['ids'] if p in self.points]
        elif path=='/points/scroll':
            if self.enforce_projection:
                assert body['with_payload']=={'include':list(SUMMARY_FIELDS)}, 'scroll must project summaries before proxy reply cap'
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
                projected.append({'id':p['id'],'payload':deepcopy(payload)})
            return {'points':projected,'next_page_offset':following}
        else:raise AssertionError('unexpected fake Qdrant operation')
        return {'status':'completed'}


class Rag(unittest.TestCase):
    def setUp(self):
        self.store=Store();self.kv=KV();self.app=Core(CFG,self.store,self.kv)
        self.app.cache.set('team','notes',PROMPT)
    def req(self,method='upsert',**changes):
        v={'method':method,'namespace':'team'}
        if method in ('upsert','get'):v.update(tableName='notes',id='tea')
        if method=='upsert':v['data']='Tea is a drink.'
        v.update(changes);return parse(dumps(v))
    def call(self,method='upsert',**changes):return self.app.run(self.req(method,**changes))
    def test_upsert_get_idempotency_and_shrinking_chunks(self):
        first=self.call(data='x'*5000)
        self.assertEqual(len(first['embeddings']),2)
        self.assertEqual(len(self.store.points),3)
        ids=set(self.store.points)
        self.assertEqual(self.call(data='x'*5000),first)
        self.assertEqual(set(self.store.points),ids)
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
        self.assertEqual(cleanup['must'][:3],[match('namespace','team'),match('tableName','notes'),match('id','tea')])
        self.assertEqual(cleanup['must_not'][0]['key'],'_rag_rev')
        self.assertNotIn('has_id',dumps(cleanup))
    def test_delete_hides_head_before_failed_chunk_cleanup_and_repeats(self):
        self.call(data='old')
        self.store.fail=lambda verb,path,body:'/delete?' in path and any(c.get('match',{}).get('value')=='chunk' for c in body['filter']['must'])
        with self.assertRaises(Unknown):self.call('delete',tableName='notes',id='tea')
        self.assertIsNone(self.call('get'))
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
        self.assertEqual(query,{'filter':{'must':[match('namespace','team'),match('_rag_kind','head')]},'limit':16,
            'with_payload':{'include':list(SUMMARY_FIELDS)},'with_vector':False})
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
        app=Core(CFG|{'dimensions':1024},Store(1024),self.kv)
        with self.assertRaisesRegex(RagError,'before provider work'):app.run(self.req(data='界'*10000))
        self.assertEqual(app.backend.paid,[])
        app=Core(CFG|{'dimensions':2731},Store(2731),self.kv)
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
        self.assertEqual(self.store.calls[-1][2],{'filter':{'must':[match('namespace','team'),match('_rag_kind','chunk')]}})

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
        v=self.req();rev='a'*64
        ids=[point_id(v,rev,i) for i in range(8)]
        vec=vector([1.1754943508222875e-38]*1024,1024)
        payload={'namespace':'team','tableName':'notes','id':'tea','mime-type':'text/plain',
                 'data':'x'*32768,'classification_ids':{'drink':.8}}
        full={**payload,'embeddings':[vec]*8}
        extra=RECORD_MAX-len(dumps(full,ascii=True).encode())
        self.assertGreater(extra,0)
        five,one=divmod(extra,5)
        self.assertLess(five+one,32768)
        payload['data']='\x01'*five+'\n'*one+'x'*(32768-five-one)
        payload.update(_rag_kind='head',_rag_rev=rev,_rag_chunks=ids)
        store.points[point_id(v)]={'id':point_id(v),'payload':payload,'vector':vec}
        for pid in ids:store.points[pid]={'id':pid,'vector':vec,'payload':{'_rag_text':'must not retrieve this'}}
        result=app.run(self.req('get'))
        self.assertEqual(len(dumps(result,ascii=True).encode()),RECORD_MAX)
        self.assertEqual(store.calls[-1][2],{'ids':ids,'with_payload':False,'with_vector':True})
        payload['data']=payload['data'][:-1]+'\n'
        with self.assertRaisesRegex(RagError,'cap'):app.run(self.req('get'))

    def test_get_missing_chunk_reloads_head_once_then_refuses(self):
        self.call()
        pid=self.store.points[point_id(self.req())]['payload']['_rag_chunks'][0]
        del self.store.points[pid]
        before=len(self.store.calls)
        with self.assertRaisesRegex(RagError,'incomplete'):self.call('get')
        reads=[c for c in self.store.calls[before:] if c[0]=='GET' and '/points/' in c[1]]
        self.assertEqual(len(reads),2)

    def test_permanent_storage_identity_vectors(self):
        v=self.req()
        rev=revision(v,[[.5,.25]],"voyage-3.5",{"drink":.8})
        self.assertEqual(point_id(v),'d73f39e4-7cd4-535d-9805-49fcd10ffe08')
        self.assertEqual(rev,'fcd6026ef966f0d659a2f063e6f7c3bde30c682330bb7d5471227e4e4c6e5526')
        self.assertEqual(point_id(v,rev,0),'66ae5bd7-42f9-5b35-bff8-3112f340df0c')

    def test_ids_are_typed_revision_specific_and_keyword_is_not_claimed(self):
        v=self.req()
        self.assertNotEqual(point_id(v),point_id(v|{'id':'tea/'}))
        a=self.call(data='old');oldchunks=set(self.store.points)-{point_id(v)}
        self.call(data='new');self.assertTrue(oldchunks.isdisjoint(set(self.store.points)-{point_id(v)}))
        keyword=Mock();self.app.keyword=keyword;self.call(data='same');keyword.assert_not_called()


if __name__=='__main__':unittest.main()
