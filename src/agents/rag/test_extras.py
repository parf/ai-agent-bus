"""R.8 extras: count, similar and classify on the fixture store; no live provider is ever called."""
from copy import deepcopy
import hashlib
import math
from pathlib import Path
import sys
import unittest

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
from test_rag import Store,KV,CFG,PROMPT
from rules import RagError,NamedRefusal,parse,dumps,point_id,FACET_MAX,config,tokens,PREFIX_INDEX,COMPLETE_CHUNKS
from core import Core


class Extras(Store):
    """The fixture store, plus Qdrant's facet, a recommend query over the stored dense vectors and a prefix text match."""
    facet_extra=0
    def matches(self,point,condition):
        text=[c for c in condition.get('must',[]) if 'text' in c.get('match',{})]
        rest={**condition,'must':[c for c in condition.get('must',[]) if c not in text]}
        words=tokens(str(point['payload'].get('_rag_text','')))
        return super().matches(point,rest) and all(any(w.startswith(c['match']['text']) for w in words) for c in text)
    def qdrant(self,verb,path,body=None,write=False):
        if verb=='POST' and path=='/facet':
            if hasattr(self,'budget'):self.budget.left(10)
            self.calls.append((verb,path,deepcopy(body),write))
            counts={}
            for p in self.points.values():
                if self.matches(p,body['filter']):
                    for label in p['payload'].get('_rag_labels',[]):counts[label]=counts.get(label,0)+1
            counts.update({f'extra{n:03}':1 for n in range(self.facet_extra)})
            hits=sorted(counts.items(),key=lambda x:(-x[1],x[0]))[:body['limit']]
            return {'hits':[{'value':k,'count':n} for k,n in hits]}
        if verb=='POST' and path=='/points/query' and 'recommend' in body.get('query',{}):
            if hasattr(self,'budget'):self.budget.left(10)
            self.calls.append((verb,path,deepcopy(body),write))
            dense=lambda p:p['vector'] if isinstance(p['vector'],list) else p['vector']['']
            positive=[dense(self.points[i]) for i in body['query']['recommend']['positive']]
            target=[sum(c)/len(positive) for c in zip(*positive)]
            cosine=lambda a,b:sum(x*y for x,y in zip(a,b))/(math.hypot(*a)*math.hypot(*b))
            scored=sorted(((round(cosine(target,dense(p)),6),p) for p in self.points.values() if self.matches(p,body['filter'])),key=lambda x:(-x[0],x[1]['id']))
            keep=body['with_payload']['include']
            start=body.get('offset',0)
            return {'points':[{'id':p['id'],'score':s,'payload':{k:v for k,v in p['payload'].items() if k in keep}} for s,p in scored[start:start+body['limit']]]}
        return super().qdrant(verb,path,body,write)


class Base(unittest.TestCase):
    cfg=CFG
    def setUp(self):
        self.store=Extras();self.kv=KV();self.app=Core(self.cfg,self.store,self.kv)
        self.app.cache.set('team','notes',PROMPT)
    def run_(self,method,**fields):
        return self.app.run(parse(dumps({'method':method,'namespace':'team',**fields})))
    def put(self,id,vector,labels=None,namespace='team',table='notes',**more):
        self.store.probabilities=labels or {'drink':.8,'other':.2}
        if (namespace,table)!=('team','notes'):self.app.cache.set(namespace,table,PROMPT)
        v={'method':'upsert','namespace':namespace,'tableName':table,'id':id,'data':id+' text','source':'notes/'+id,'embeddings':[vector],**more}
        return self.app.run(parse(dumps(v)))
    def fresh(self):
        """Forget the setup's calls: what follows is the method under test alone."""
        self.store.calls.clear();self.store.paid.clear()


class Count(Base):
    def setUp(self):
        super().setUp()
        self.put('tea',[1,0]);self.put('coffee',[1,0],key1='hot')
        self.put('rock',[0,1],{'drink':.1,'other':.9})
        self.put('elsewhere',[1,0],namespace='other')
        self.fresh()
    def test_live_records_and_their_labels_are_counted_free(self):
        self.assertEqual(self.run_('count'),{'count':3,'labels':{'drink':2,'other':1},'labels_truncated':False})
        self.assertEqual(self.run_('count',key1='hot'),{'count':1,'labels':{'drink':1},'labels_truncated':False})
        self.assertEqual(self.store.paid,[])
    def test_the_facet_asks_only_live_heads_inside_the_namespace(self):
        self.run_('count',tableName='notes')
        verb,path,body,write=next(c for c in self.store.calls if c[1]=='/facet')
        must=body['filter']['must']
        for condition in ({'key':'namespace','match':{'value':'team'}},{'key':'tableName','match':{'value':'notes'}},{'key':'_rag_kind','match':{'value':'head'}}):
            self.assertIn(condition,must)
        self.assertEqual((verb,write,body['key'],body['exact'],body['limit']),('POST',False,'_rag_labels',True,FACET_MAX+1))
    def test_a_deleted_record_is_not_counted(self):
        self.run_('delete',tableName='notes',id='tea')
        self.assertEqual(self.run_('count'),{'count':2,'labels':{'drink':1,'other':1},'labels_truncated':False})
    def test_more_labels_than_one_answer_holds_are_said_never_cut_silently(self):
        self.store.facet_extra=FACET_MAX
        answer=self.run_('count')
        self.assertEqual((len(answer['labels']),answer['labels_truncated']),(FACET_MAX,True))
        self.store.facet_extra=FACET_MAX-2   # drink, other and 62 more: exactly the cap
        answer=self.run_('count')
        self.assertEqual((len(answer['labels']),answer['labels_truncated']),(FACET_MAX,False))
    def test_a_malformed_facet_answer_is_refused(self):
        real=self.store.qdrant
        for hits in ([{'value':'drink','count':-1}],[{'value':'drink','count':True}],[{'value':'drink','count':1}]*2,[{'count':1}],None):
            with self.subTest(hits=hits):
                self.store.qdrant=lambda verb,path,body=None,write=False,h=hits:{'hits':deepcopy(h)} if path=='/facet' else real(verb,path,body,write)
                with self.assertRaisesRegex(RagError,'facet'):self.run_('count')
        self.store.qdrant=real


class Similar(Base):
    def setUp(self):
        super().setUp()
        self.put('tea',[1,0]);self.put('green',[.9,.1]);self.put('rock',[0,1]);self.put('far',[-1,.1])
        self.put('twin',[1,0],namespace='other')            # out of scope: another namespace
        self.put('twin',[1,0],table='drafts')               # out of scope: another table
        self.put('keyed',[1,.05],key1='k')
        self.fresh()
    def ids(self,**more):
        return [r['id'] for r in self.run_('similar',tableName='notes',id='tea',**more)['records']]
    def test_nearest_records_come_first_never_the_subject_and_nothing_is_embedded(self):
        self.assertEqual(self.ids(),['keyed','green','rock','far'])
        self.assertEqual(self.store.paid,[])
        record=self.run_('similar',tableName='notes',id='tea',limit=1)['records'][0]
        self.assertEqual((record['id'],set(record['scores'])),('keyed',{'similarity'}))
        self.assertEqual(record['excerpt'],'keyed text')
    def test_an_out_of_scope_near_duplicate_never_answers(self):
        self.assertNotIn('twin',self.ids())
        query=next(body for _,path,body,_ in self.store.calls if path=='/points/query')
        must=query['filter']['must']
        for condition in ({'key':'namespace','match':{'value':'team'}},{'key':'tableName','match':{'value':'notes'}},{'key':'_rag_kind','match':{'value':'chunk'}}):
            self.assertIn(condition,must)
        head=self.store.points[point_id({'namespace':'team','tableName':'notes','id':'tea'})]
        self.assertEqual(query['query'],{'recommend':{'positive':head['payload']['_rag_chunks']}})
    def test_the_key_filters_narrow_like_search(self):
        self.assertEqual(self.ids(key1='k'),['keyed'])
    def test_limit_is_capped_at_sixteen(self):
        self.assertEqual(len(self.ids(limit=2)),2)
        for limit in (0,17,True,'2'):
            with self.subTest(limit=limit),self.assertRaises(RagError):self.ids(limit=limit)
    def test_a_missing_or_deleted_subject_is_not_found(self):
        with self.assertRaises(NamedRefusal) as c:self.run_('similar',tableName='notes',id='nobody')
        self.assertEqual(c.exception.code,'not_found')
        # A delete whose chunk removal failed leaves the head marked deleted.
        self.store.fail=lambda verb,path,body:verb=='POST' and '/delete?' in path
        with self.assertRaises(RagError):self.run_('delete',tableName='notes',id='tea')
        self.store.fail=None
        self.assertEqual(self.store.points[point_id({'namespace':'team','tableName':'notes','id':'tea'})]['payload']['_rag_kind'],'deleted')
        with self.assertRaises(NamedRefusal) as c:self.ids()
        self.assertEqual(c.exception.code,'not_found')
        self.run_('delete',tableName='notes',id='tea')
        with self.assertRaises(NamedRefusal) as c:self.ids()
        self.assertEqual(c.exception.code,'not_found')
    def test_a_draft_or_replaced_revision_never_answers(self):
        # A chunk whose head never landed, nearest of all, and a stale revision of a live record.
        template=next(p for p in self.store.points.values() if p['payload'].get('_rag_kind')=='chunk' and p['payload']['id']=='rock')
        for id,rev in (('draft','d'*64),('rock','e'*64)):
            payload=deepcopy(template['payload']);payload.update(id=id,_rag_rev=rev)
            pid=point_id(payload,rev,0)
            self.store.points[pid]={'id':pid,'vector':[1,0],'payload':payload}
        self.assertEqual(self.ids(),['keyed','green','rock','far'])
        record=next(r for r in self.run_('similar',tableName='notes',id='tea')['records'] if r['id']=='rock')
        self.assertEqual(record['scores']['similarity'],0.0)   # its live chunk, not the stale one


class ClassifyOff(Base):
    def test_off_by_default_and_refused_before_any_call(self):
        self.assertIs(config(CFG)['classify'],False)
        self.fresh()
        with self.assertRaises(NamedRefusal) as c:self.run_('classify',tableName='notes',data='Green tea.')
        self.assertEqual(c.exception.code,'classify_unavailable')
        self.assertEqual((self.store.calls,self.store.paid),([],[]))
    def test_the_setting_is_a_boolean(self):
        for bad in ('true',1,None):
            with self.subTest(value=bad),self.assertRaisesRegex(RagError,'classify must be true or false'):config({**CFG,'classify':bad})


class Classify(Base):
    cfg={**CFG,'classify':True}
    def setUp(self):
        super().setUp()
        self.store.probabilities={'drink':.8,'other':.6}
        self.fresh()
    def test_one_paid_call_labels_the_text_and_nothing_is_stored(self):
        points=deepcopy(self.store.points)
        answer=self.run_('classify',tableName='notes',data='Green tea.')
        self.assertEqual(answer,{'labels':{'drink':.8,'other':.6},'probabilities':{'drink':.8,'other':.6}})
        self.assertEqual([kind for kind,_ in self.store.paid],['ai'])
        request=self.store.paid[0][1]
        self.assertEqual((request['model'],request['state'],request['questions']['labels']['criteria']),('typesafe/jev-1.14','Green tea.',PROMPT['labels']))
        self.assertEqual(self.store.points,points)
        self.assertFalse(any(write for *_,write in self.store.calls))
    def test_threshold_and_top_k_are_the_tables_as_on_upsert(self):
        self.app.cache.set('team','notes',{**PROMPT,'threshold':.7,'top_k':1})
        self.store.probabilities={'drink':.75,'other':.9}
        self.assertEqual(self.run_('classify',tableName='notes',data='x'),{'labels':{'other':.9},'probabilities':{'other':.9,'drink':.75}})
    def test_input_and_prompts_are_checked_before_the_paid_call(self):
        for fields,error in (({'tableName':'unset','data':'x'},'prompts are not configured'),
                             ({'tableName':'notes','data':'{not json','mime-type':'application/json'},''),
                             ({'tableName':'notes','data':''},'')):
            with self.subTest(fields=fields),self.assertRaisesRegex(RagError,error):self.run_('classify',**fields)
        self.assertEqual(self.store.paid,[])
    def test_a_bad_answer_fails_the_call(self):
        for probabilities in ({'drink':.8},{'drink':.8,'other':2},{'drink':.8,'other':.1,'more':.1}):
            with self.subTest(probabilities=probabilities):
                self.store.probabilities=probabilities
                with self.assertRaisesRegex(RagError,'Jev|probability'):self.run_('classify',tableName='notes',data='x')
    def test_upsert_keeps_the_same_labels_through_the_shared_check(self):
        record=self.put('tea',[1,0],{'drink':.8,'other':.6})
        self.assertEqual(record['classification_ids'],{'drink':.8,'other':.6})


class Complete(Base):
    def setUp(self):
        super().setUp()
        self.store.sparse=True;self.app=Core(self.cfg,self.store,self.kv)
        for id,data,more in (('tea','Green tea is great.',{}),('pot','A teapot for tea.',{'key1':'k'}),('team','The team drinks coffee.',{}),
                             ('gone','Tea, teal and tears.',{})):
            self.put(id,[1,0],data=data,**more)
        self.put('far',[1,0],data='tea tea tea',namespace='other')
        self.run_('delete',tableName='notes',id='gone')
    def build(self,roles=('owner',)):
        return self.app.run({'method':'keyword_index','namespace':'team'},{'roles':list(roles)})
    def complete(self,prefix,**more):
        return self.run_('complete',prefix=prefix,**more)
    def text_reads(self):
        return [c for c in self.store.calls if c[1]=='/points/scroll' and any('text' in m.get('match',{}) for m in c[2]['filter']['must'])]
    def test_without_the_prefix_index_it_is_refused_and_reads_nothing(self):
        self.fresh()
        with self.assertRaises(NamedRefusal) as c:self.complete('te')
        self.assertEqual(c.exception.code,'complete_unavailable')
        self.assertIn('keyword_index',str(c.exception))
        self.assertEqual(self.text_reads(),[])
    def test_keyword_index_creates_the_index_once_and_only_for_a_manager(self):
        for roles in ((),('write',)):
            with self.subTest(roles=roles),self.assertRaisesRegex(RagError,'Owner or Maintainers'):self.build(roles)
        self.assertNotIn('_rag_text',self.store.indexes)
        self.fresh();self.build()
        puts=[c for c in self.store.calls if c[1]=='/index?wait=true']
        self.assertEqual([(verb,body,write) for verb,_,body,write in puts],[('PUT',{'field_name':'_rag_text','field_schema':PREFIX_INDEX},True)])
        self.assertEqual(self.store.indexes['_rag_text']['params'],{'type':'text','tokenizer':'prefix','lowercase':True,'min_token_len':1,'max_token_len':64})
        self.fresh();self.build()
        self.assertFalse(any(c[1]=='/index?wait=true' for c in self.store.calls))   # a repeat is a no-op for the index
        self.assertEqual(self.complete('te')['suggestions'][0],{'text':'tea','records':2})
    def test_a_proxy_without_the_admin_role_names_the_owners_step(self):
        self.store.refuse_index=True
        with self.assertRaisesRegex(RagError,'admin role.*request PUT /collections/notes/index\\?wait=true .*"tokenizer":"prefix".*repeat this command'):self.build()
        self.assertNotIn('_rag_text',self.store.indexes)
    def test_external_keyword_search_still_gets_the_prefix_index(self):
        self.app=Core({**self.cfg,'keyword_search':'external'},self.store,self.kv)
        with self.assertRaisesRegex(RagError,'external'):self.build()
        self.assertIn('_rag_text',self.store.indexes)
    def test_the_last_word_is_completed_from_live_records_most_first(self):
        self.build();self.fresh()
        self.assertEqual(self.complete('te'),{'suggestions':[{'text':'tea','records':2},{'text':'team','records':1},{'text':'teapot','records':1}],'sampled':False})
        self.assertEqual(self.complete('TE')['suggestions'],self.complete('te')['suggestions'])   # case-folded
        self.assertEqual(self.complete('green te')['suggestions'],[{'text':'green tea','records':1}])
        self.assertEqual(self.complete('te',key1='k')['suggestions'],[{'text':'tea','records':1},{'text':'teapot','records':1}])
        self.assertEqual(self.complete('te',limit=1)['suggestions'],[{'text':'tea','records':1+1}])
        self.assertEqual(self.complete('tea ')['suggestions'],[])   # ends between words
        self.assertEqual(self.store.paid,[])
    def test_the_read_stays_in_scope_and_skips_deleted_records(self):
        self.build();self.fresh()
        words={s['text'] for s in self.complete('te')['suggestions']}
        self.assertFalse(words&{'teal','tears'})   # only the deleted record holds them
        read=self.text_reads()[0][2]
        for condition in ({'key':'namespace','match':{'value':'team'}},{'key':'_rag_kind','match':{'value':'chunk'}},{'key':'_rag_text','match':{'text':'te'}}):
            self.assertIn(condition,read['filter']['must'])
        self.assertEqual((read['limit'],read['with_vector']),(COMPLETE_CHUNKS,False))
    def test_more_matching_chunks_than_one_read_says_sampled(self):
        for n in range(COMPLETE_CHUNKS):self.put(f'n{n:02}',[1,0],data='tea')
        self.build()
        answer=self.complete('te')
        self.assertTrue(answer['sampled'])
    def test_a_prefix_needs_a_word_and_a_bounded_limit(self):
        for prefix in ('','  ','?!'):
            with self.subTest(prefix=prefix),self.assertRaises(RagError):self.complete(prefix)
        for limit in (0,17):
            with self.subTest(limit=limit),self.assertRaises(RagError):self.complete('te',limit=limit)


if __name__=='__main__':unittest.main()
