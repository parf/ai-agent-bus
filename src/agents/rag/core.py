"""Serialized RAG operations; providers prepare first, confirmed storage writes last."""
from collections import OrderedDict
from copy import deepcopy
import secrets
import time
import uuid
from chunks import extract
from prompts import Cache
from rules import (RagError,Unknown,Refused,Deadline,Incomplete,RECORD_MAX,SUMMARY_MAX,SCALARS_MAX,INDEXES,SUMMARY_FIELDS,BM25_VECTOR,NO_VECTOR,NO_IDF,INDEX_BYTES,
                   config,dumps,identity,match,point_id,revision,scope,number,sparse,vector,summary,text,DATA_MAX)
from blocks import Budget


class Core:
    def __init__(self,cfg,backend,kv,keyword=None):
        self.cfg=config(cfg);self.backend=backend;self.cache=Cache(kv);self.keyword=keyword
        self.budget=Budget();self.cursors=OrderedDict();self.health='unknown: collection not verified';self.bm25=False;self.keyword_state=NO_VECTOR;self.indexed=False
    def verify(self):
        try:
            info=self.backend.qdrant('GET','')
            if info.get('status') not in ('green','yellow'):raise RagError('collection is not healthy')
            params=info['config']['params']; vectors=params['vectors']
            if type(vectors.get('size')) is not int or vectors.get('size')!=self.cfg['dimensions'] or vectors.get('distance')!='Cosine' or vectors.get('on_disk',False) or info['config'].get('quantization_config') is not None:raise RagError('collection vector settings mismatch')
            # The owner creates the collection; keyword search needs its sparse vector, and a collection without one still serves everything else.
            word=(params.get('sparse_vectors') or {}).get(BM25_VECTOR)
            self.bm25=self.cfg['keyword_search']=='builtin' and isinstance(word,dict) and word.get('modifier')=='idf'
            self.keyword_state=NO_VECTOR if not isinstance(word,dict) else NO_IDF if word.get('modifier')!='idf' else None
            if self.keyword_state is not None:self.indexed=False
            metadata=info['config'].get('metadata',{})
            if metadata.get('embedding_model')!=self.cfg['embedding_model']:raise RagError('collection embedding model mismatch')
            indexes=info['payload_schema']
            if any(indexes.get(k,{}).get('data_type')!='keyword' for k in INDEXES):raise RagError('collection needs owner-created keyword indexes')
            self.health='ok: collection verified' if info['status']=='green' else 'degraded: collection optimizing'
        except (KeyError,TypeError,AttributeError):
            self.health='failed: invalid collection settings'
            raise RagError('invalid collection settings') from None
        except RagError:
            self.health='failed: collection verification failed';raise
    def run(self,request,envelope=None):
        self.budget=Budget(envelope);self.backend.begin(self.budget)
        method=request['method']
        if method=='prompts':
            action=request['action'];n=request['namespace'];t=request['tableName']
            if action=='get':return self.cache.get(n,t)
            if action=='set':self.cache.set(n,t,request['data'])
            else:self.cache.delete(n,t)
            return {'status':'ok'}
        self.verify()
        if method=='upsert':return self.upsert(request)
        if method=='get':return self.get(request)
        if method=='list':return self.list(request)
        if method=='search':return self.search(request)
        if method=='keyword_index':return self.keyword_index(envelope)
        if method=='delete':return self.delete(request)
        raise RagError('unknown method')
    def prepare(self,v):
        source,chunks=extract(v); dimensions=self.cfg['dimensions']
        if len(chunks)*dimensions>SCALARS_MAX:raise RagError('chunks times dimensions exceeds 8192 scalars before provider work')
        settings=self.cache.get(v['namespace'],v['tableName'])
        if settings is None:raise RagError('classification prompts are not configured')
        # Upper bound every possible selected label, before paying either provider.
        keys=sorted(settings['labels'],key=lambda x:len(dumps(x,ascii=True)),reverse=True)[:settings['top_k']]
        label_budget=sum(len(dumps(k,ascii=True))+26 for k in keys)+2
        base={k:v[k] for k in ('namespace','tableName','id','key1','key2','data','mime-type') if k in v}
        estimate=len(dumps(base,ascii=True).encode())+len(chunks)*dimensions*25+label_budget+4096
        if estimate>RECORD_MAX:raise RagError(f'worst-case record {estimate} bytes exceeds {RECORD_MAX} before provider work')
        summary_estimate=len(dumps({k:base[k] for k in SUMMARY_FIELDS if k in base},ascii=True).encode())+label_budget+1024
        if summary_estimate>SUMMARY_MAX:raise RagError('worst-case summary exceeds 32768 bytes before provider work')
        supplied=None
        if 'embeddings' in v:
            if not isinstance(v['embeddings'],list) or len(v['embeddings'])!=len(chunks):raise RagError('one supplied vector required per chunk')
            supplied=[vector(row,dimensions) for row in v['embeddings']]
        classification={'model':self.cfg['decision_model'],'state':source,'questions':{'labels':{'type':'choice','instructions':settings['instructions'],'criteria':settings['labels']}}}
        batches=[{'model':self.cfg['embedding_model'],'input':chunks[i:i+4],'input_type':'document','output_dimension':dimensions,'output_dtype':'float'} for i in range(0,len(chunks),4)]
        if any(len(dumps(x).encode())>(1<<20)-1024 for x in [classification,*batches]):raise RagError('provider request exceeds 1 MiB before provider work')
        result=self.backend.decide(classification)
        try:
            if result['answers']['labels']['type']!='choice':raise RagError('expected Jev Choice probabilities')
            probabilities=result['answers']['labels']['probabilities']
            if not isinstance(probabilities,dict) or set(probabilities)!=set(settings['labels']):raise RagError('Jev labels mismatch')
            for p in probabilities.values():number(p,0,1,'probability')
        except (KeyError,TypeError):raise RagError('invalid Jev probabilities') from None
        labels=dict(sorted(((k,p) for k,p in probabilities.items() if p>=settings['threshold']),key=lambda x:(-x[1],x[0]))[:settings['top_k']])
        vectors=supplied
        if vectors is None:
            vectors=[]
            for batch in batches:
                answer=self.backend.embed(batch)
                try:
                    rows=answer['data']
                    if not isinstance(rows,list) or len(rows)!=len(batch['input']):raise RagError('embedding batch size mismatch')
                    ordered={row['index']:row['embedding'] for row in rows if type(row.get('index')) is int}
                    if set(ordered)!=set(range(len(rows))):raise RagError('embedding indexes mismatch')
                    vectors.extend(vector(ordered[i],dimensions) for i in range(len(rows)))
                except (KeyError,TypeError,AttributeError):raise RagError('invalid embedding response') from None
        full={**base,'classification_ids':labels,'embeddings':vectors}
        if len(dumps(full,ascii=True).encode())>RECORD_MAX:raise RagError('prepared record exceeds byte cap')
        summary={k:full[k] for k in SUMMARY_FIELDS if k in full}
        if len(dumps(summary,ascii=True).encode())>SUMMARY_MAX:raise RagError('prepared summary exceeds byte cap')
        return full,chunks,vectors
    def upsert(self,v):
        full,chunks,vectors=self.prepare(v)
        rev=revision(v,vectors,self.cfg['embedding_model'],full['classification_ids'])
        fields={k:full[k] for k in ('namespace','tableName','id','key1','key2') if k in full}
        ids=[point_id(v,rev,i) for i in range(len(chunks))]
        points=[{'id':pid,'vector':{'':vec,BM25_VECTOR:sparse(chunk)} if self.bm25 else vec,'payload':{**fields,'_rag_kind':'chunk','_rag_rev':rev,'_rag_labels':list(full['classification_ids']),'_rag_text':chunk}}
                for pid,vec,chunk in zip(ids,vectors,chunks)]
        head={k:value for k,value in full.items() if k!='embeddings'}
        head.update(_rag_kind='head',_rag_rev=rev,_rag_chunks=ids,_rag_labels=list(full['classification_ids']))
        stage=0
        try:
            self.backend.qdrant('PUT','/points?wait=true',{'points':points},write=True)
            stage=1
            self.backend.qdrant('PUT','/points?wait=true',{'points':[{'id':point_id(v),'vector':[1.0]+[0.0]*(self.cfg['dimensions']-1),'payload':head}]},write=True)
            old=scope(v,kind='chunk',exact=True);old['must_not']=[match('_rag_rev',rev)]
            self.backend.qdrant('POST','/points/delete?wait=true',{'filter':old},write=True)
        except (Refused,Deadline) as error:
            if stage==0:raise RagError('not stored; prior record unchanged: '+str(error)) from None
            raise Unknown('storage outcome unknown; repeating the same upsert/delete is safe') from None
        except RagError:
            raise Unknown('storage outcome unknown; repeating the same upsert/delete is safe') from None
        return full
    def get(self,v):
        for attempt in range(2):
            try:head=self.backend.qdrant('GET','/points/'+point_id(v)+'?with_vector=false')
            except RagError as error:
                # Backend adapters represent a missing point explicitly, never by accepting another record.
                if getattr(error,'missing',False):return None
                raise
            if head is None:return None
            try:
                payload=head['payload']
                if payload.get('_rag_kind')=='deleted':return None
                if head['id']!=point_id(v) or payload.get('_rag_kind')!='head' or identity(payload)!=identity(v):raise RagError('invalid canonical head')
                ids=payload['_rag_chunks']
                if not isinstance(ids,list) or not 1<=len(ids)<=8 or len(ids)*self.cfg['dimensions']>SCALARS_MAX or len(set(ids))!=len(ids):raise RagError('invalid chunk references')
                if any(pid!=point_id(v,payload['_rag_rev'],i) for i,pid in enumerate(ids)):raise RagError('chunk identity mismatch')
                rows=self.backend.qdrant('POST','/points',{'ids':ids,'with_payload':False,'with_vector':True})
                # A collection with a sparse vector answers each point's vectors by name; the dense one is the unnamed one.
                found={row['id']:row['vector'].get('') if isinstance(row['vector'],dict) else row['vector'] for row in rows}
                if set(found)!=set(ids) or len(rows)!=len(ids):
                    if attempt==0:continue
                    raise RagError('record changed or is incomplete; retry the read')
                result=dict(summary({k:payload[k] for k in SUMMARY_FIELDS if k in payload}))
                result['data']=text(payload.get('data'),'stored data',DATA_MAX,empty=True)
                result['embeddings']=[vector(found[pid],self.cfg['dimensions']) for pid in ids]
                if len(dumps(result,ascii=True).encode())>RECORD_MAX:raise RagError('stored record exceeds read cap')
                return result
            except (KeyError,TypeError,AttributeError):raise RagError('invalid stored record') from None
        raise RagError('record changed or is incomplete')
    def list(self,v):
        selected=scope(v,kind='head');signature=dumps([selected,v['limit']])
        offset=None
        if 'cursor' in v:
            entry=self.cursors.get(v['cursor'])
            if not entry or entry[0]<time.monotonic() or entry[1]!=signature:raise RagError('stale or mismatched cursor')
            self.cursors.move_to_end(v['cursor']);offset=entry[2]
        query={'filter':selected,'limit':v['limit'],'with_payload':{'include':list(SUMMARY_FIELDS)},'with_vector':False}
        if offset is not None:query['offset']=offset
        page=self.backend.qdrant('POST','/points/scroll',query)
        try:
            rows=page['points'];following=page['next_page_offset']
            if not isinstance(rows,list) or len(rows)>v['limit']:raise RagError('invalid scroll size')
            ids=[str(uuid.UUID(row['id'])) for row in rows]
            if ids!=sorted(ids) or len(ids)!=len(set(ids)) or offset is not None and any(i<offset for i in ids):raise RagError('invalid scroll order')
            if following is not None:
                if not rows or str(uuid.UUID(following))!=following or following<=ids[-1]:raise RagError('invalid scroll offset')
            records=[]
            for row in rows:
                record=summary(row['payload'])
                if record.get('namespace')!=v['namespace']:raise RagError('invalid scroll payload projection')
                if any(record.get(k)!=v[k] for k in ('tableName','key1','key2') if k in v):raise RagError('scroll filter mismatch')
                if len(dumps(record,ascii=True).encode())>SUMMARY_MAX:raise RagError('stored summary exceeds byte cap')
                records.append(record)
        except (KeyError,ValueError,TypeError,AttributeError):raise RagError('invalid scroll response') from None
        token=None
        if following is not None:
            token=secrets.token_urlsafe(24);self.cursors[token]=(time.monotonic()+600,signature,following)
            while len(self.cursors)>128:self.cursors.popitem(last=False)
        return {'records':records,'cursor':token}
    def search(self,v):
        """Records whose stored text best matches the query's words, best first, one row per record.

        A chunk answers only while its record's live head lists it: a draft whose head never landed, a
        replaced revision and a deleted record are skipped, as get() would not return them.
        """
        if self.cfg['keyword_search']=='external':
            if self.keyword is None:raise RagError('keyword_search is external and no external keyword engine is connected')
            found=self.keyword(deepcopy(v))
            if not isinstance(found,dict) or not isinstance(found.get('records'),list):raise RagError('invalid external keyword answer')
            return found
        if not self.bm25:raise RagError(self.keyword_state)
        left=self._unindexed()
        if left:raise RagError(f'keyword index incomplete ({left} chunks left); run {{"method":"keyword_index"}}')
        best={};want=min(64,v['limit']*4);exhausted=False
        for page in range(4):
            query={'query':sparse(v['query'],query=True),'using':BM25_VECTOR,'filter':scope(v,kind='chunk'),'limit':want,'offset':page*want,
                   'with_payload':{'include':[*SUMMARY_FIELDS[:5],'_rag_text']},'with_vector':False}
            answer=self.backend.qdrant('POST','/points/query',query)
            try:
                rows=answer['points']
                if not isinstance(rows,list) or len(rows)>want:raise RagError('invalid query size')
                for row in rows:
                    payload=row['payload'];number(row['score'],0,float('inf'),'score')
                    if payload.get('namespace')!=v['namespace']:raise RagError('invalid query payload projection')
                    if any(payload.get(k)!=v[k] for k in ('tableName','key1','key2') if k in v):raise RagError('query filter mismatch')
                live=self._live(rows)
                for row in rows:
                    payload=row['payload'];key=identity(payload)
                    if key in best or row['id'] not in live.get(point_id(payload),()):continue
                    record={k:payload[k] for k in SUMMARY_FIELDS[:5] if k in payload}
                    record['score']=row['score'];record['text']=text(payload.get('_rag_text'),'stored text',DATA_MAX,empty=True)
                    best[key]=record
            except (KeyError,TypeError,AttributeError):raise RagError('invalid query response') from None
            if len(best)>=v['limit'] or len(rows)<want:break
            exhausted=page==3
        # An empty answer must mean no match, never a window that dead chunks filled.
        if exhausted and len(best)<v['limit']:raise Incomplete('search window exhausted by chunks no record head lists; narrow the query, add filters or run cleanup')
        return {'records':sorted(best.values(),key=lambda r:(-r['score'],r['id']))[:v['limit']]}
    def _unindexed(self):
        """How many chunks still lack the text vector; known complete once it was counted as none."""
        if self.indexed:return 0
        found=self.backend.qdrant('POST','/points/count',{'filter':{'must':[match('_rag_kind','chunk')],'must_not':[{'has_vector':BM25_VECTOR}]},'exact':True})
        try:n=found['count']
        except (KeyError,TypeError):raise RagError('invalid count response') from None
        if type(n) is not int or n<0:raise RagError('invalid count response')
        self.indexed=n==0
        return n
    def _live(self,rows):
        """Head id -> the chunk ids its live head lists, for the records the rows belong to; a missing or deleted head lists none."""
        heads=sorted({point_id(row['payload']) for row in rows})
        if not heads:return {}
        found=self.backend.qdrant('POST','/points',{'ids':heads,'with_payload':{'include':['_rag_kind','_rag_chunks']},'with_vector':False})
        live={}
        for head in found:
            payload=head.get('payload') or {}
            chunks=payload.get('_rag_chunks')
            if payload.get('_rag_kind')=='head' and isinstance(chunks,list):live[head['id']]=set(chunks)
        return live
    def keyword_index(self,envelope):
        """Add the text sparse vector when the collection lacks it, then give every chunk its words; safe to repeat.

        Chunks already carrying the vector are not listed again, so a repeat after an interruption continues where it
        stopped. The proxy asks for its admin role to add the vector; the answer then names the owner's step.
        """
        if not {'owner','maintainer'}&set((envelope or {}).get('roles') or []):raise RagError('keyword_index is for this helper\'s Owner or Maintainers')
        if self.cfg['keyword_search']=='external':raise RagError('keyword_search is external; there is no built-in keyword index to build')
        self.verify()
        if self.keyword_state==NO_VECTOR:
            try:self.backend.qdrant('PUT','/vectors/'+BM25_VECTOR,{'sparse':{'modifier':'idf'}},write=True)
            except Refused as error:
                raise RagError(f'adding the text vector needs the Qdrant proxy\'s admin role ({error}); run as its owner: request PUT /collections/{self.cfg["collection"]}/vectors/{BM25_VECTOR} {{"sparse":{{"modifier":"idf"}}}} , then repeat this command') from None
            self.verify()
        if not self.bm25:raise RagError(self.keyword_state)
        done=0;seen=set()
        while True:
            try:self.budget.left(10)
            except Deadline:raise Incomplete(f'{done} chunks indexed so far; repeat the same command to continue') from None
            page=self.backend.qdrant('POST','/points/scroll',{'filter':{'must':[match('_rag_kind','chunk')],'must_not':[{'has_vector':BM25_VECTOR}]},
                'limit':64,'with_payload':{'include':['_rag_text']},'with_vector':False})
            try:
                rows=page['points']
                if not isinstance(rows,list) or len(rows)>64:raise RagError('invalid scroll size')
                if not rows:
                    self.indexed=True;return {'status':'ok','indexed':done}
                if seen&{row['id'] for row in rows}:raise RagError('the chunks just indexed still lack the vector; stopping')
                update=[];size=0
                for row in rows:
                    item={'id':row['id'],'vector':{BM25_VECTOR:sparse(text(row['payload'].get('_rag_text'),'stored text',DATA_MAX,empty=True))}}
                    size+=len(dumps(item))
                    # The proxy takes 1 MiB a call; what does not fit is listed again by the next page.
                    if update and size>INDEX_BYTES:break
                    update.append(item)
            except (KeyError,TypeError,AttributeError):raise RagError('invalid scroll response') from None
            self.backend.qdrant('PUT','/points/vectors?wait=true',{'points':update},write=True)
            seen.update(item['id'] for item in update);done+=len(update)
    def delete(self,v):
        try:return self._delete(v)
        except Deadline:raise Incomplete('repeat the same delete to continue') from None
        except RagError:raise Unknown('storage outcome unknown; repeating the same upsert/delete is safe') from None
    def _delete(self,v):
        # A tombstone removes the head from reads first, retaining identities for
        # repeatable filtered cleanup if a later step fails. No source is lost to an upsert failure.
        live=scope(v,kind='head')
        self.backend.qdrant('POST','/points/payload?wait=true',{'payload':{'_rag_kind':'deleted'},'filter':live},write=True)
        if 'id' in v:
            self._finish_delete(v)
        else:
            while True:
                self.budget.left(10)
                page=self.backend.qdrant('POST','/points/scroll',{'filter':scope(v,kind='deleted'),'limit':16,
                    'with_payload':{'include':['namespace','tableName','id','key1','key2','_rag_kind']},'with_vector':False})
                try:rows=page['points']
                except (KeyError,TypeError):raise RagError('invalid deletion scroll') from None
                if not isinstance(rows,list) or len(rows)>16:raise RagError('invalid deletion scroll size')
                if not rows:break
                for row in rows:
                    record=row['payload']
                    if not isinstance(record,dict) or record.get('namespace')!=v['namespace']:raise RagError('invalid deletion scope')
                    if row.get('id')!=point_id(record) or record.get('_rag_kind')!='deleted':raise RagError('invalid deletion head')
                    if any(record.get(k)!=v[k] for k in ('tableName','key1','key2') if k in v):raise RagError('invalid deletion filter')
                    self._finish_delete(record)
            # Also remove pre-head chunks from a failed first upsert. Public key
            # filters are present on every chunk; this never broadens the selection.
            self.backend.qdrant('POST','/points/delete?wait=true',{'filter':scope(v,kind='chunk')},write=True)
        return {'status':'ok'}
    def _finish_delete(self,v):
        self.backend.qdrant('POST','/points/delete?wait=true',{'filter':scope(v,kind='chunk',exact=True)},write=True)
        self.backend.qdrant('POST','/points/delete?wait=true',{'filter':scope(v,kind='deleted',exact=True)},write=True)
