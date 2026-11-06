"""Serialized RAG operations; providers prepare first, confirmed storage writes last."""
from collections import OrderedDict
from copy import deepcopy
import datetime
import hashlib
import math
import secrets
import time
import uuid
from chunks import extract
from prompts import Cache
from rules import (RagError,Unknown,Refused,Deadline,Incomplete,RECORD_MAX,SUMMARY_MAX,SCALARS_MAX,INDEXES,SUMMARY_FIELDS,BM25_VECTOR,NO_VECTOR,NO_IDF,INDEX_BYTES,INDEX_RECHECK,
                   config,dumps,identity,match,point_id,revision,scope,number,sparse,vector,summary,text,DATA_MAX,BODY_MAX)
from blocks import Budget
from namespaces import Catalogue,manager,SCHEMA
from rules import NamedRefusal,PROVENANCE_FIELDS,provenance


class Core:
    def __init__(self,cfg,backend,kv,keyword=None):
        self.cfg=config(cfg);self.backend=backend;self.cache=Cache(kv);self.keyword=keyword;self.catalogue=Catalogue(kv,backend,self.cfg);self.index_state={}
        self.budget=Budget();self.cursors=OrderedDict();self.health='unknown: collection not verified';self.bm25=False;self.keyword_state=NO_VECTOR;self.indexed=False;self.checked=0.0
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
        method=request['method']
        if method in ('keyword_index','namespaces','drop_namespace'):manager(envelope)
        if method=='drop_namespace' and request.get('confirm')!=request['namespace']:
            raise NamedRefusal('confirmation_required','confirm must equal the requested namespace')
        self.budget=Budget(envelope);self.backend.begin(self.budget)
        self.verify()
        # This count is a provisioning check, not an unscoped record read.
        legacy=self.backend.qdrant('POST','/points/count',{'filter':{'must':[{'is_empty':{'key':'namespace'}}]},'exact':True})
        if not isinstance(legacy,dict) or type(legacy.get('count')) is not int or legacy['count']<0:raise RagError('invalid legacy count')
        if legacy['count']:raise NamedRefusal('legacy_data','namespace-free rows require owner re-feed into a fresh collection')
        if method=='namespaces':return self.namespaces(request)
        if method=='drop_namespace':return self.drop_namespace(request)
        repo=self.catalogue.bind(request['namespace'],self.cache,request.get('tableName'))
        try:
            if method=='prompts':
                result=repo.prompt(request['action'],request['tableName'],request.get('data'))
                return result if request['action']=='get' else {'status':'ok'}
            if method=='upsert':return self.upsert(request,repo)
            if method=='get':
                result=self.get(request,repo)
                if result is None:raise NamedRefusal('not_found','record is absent from this namespace')
                return result
            if method=='list':return self.list(request,repo)
            if method=='search':return self.search(request,repo)
            if method=='keyword_index':return self.keyword_index(envelope,repo)
            if method=='delete':return self.delete(request,repo)
            raise RagError('unknown method')
        finally:
            if method in ('upsert','delete','keyword_index'):self.index_state.pop(repo.name,None)

    def namespaces(self,v):
        names=sorted(self.catalogue.load());signature=dumps(['namespaces',v['limit']]);after=None
        if 'cursor' in v:
            entry=self.cursors.get(v['cursor'])
            if not entry or entry[0]<time.monotonic() or entry[1]!=signature:raise RagError('stale or mismatched cursor')
            after=entry[2];self.cursors.move_to_end(v['cursor'])
        remaining=[name for name in names if after is None or name>after]
        page=remaining[:v['limit']];rows=[]
        for name in page:
            repo=self.catalogue.existing(name,self.cache)
            rows.append({'namespace':name,'count':repo.count({'must':[match('_rag_kind','head')]})})
        cursor=None
        if len(remaining)>len(page):
            cursor=secrets.token_urlsafe(24);self.cursors[cursor]=(time.monotonic()+600,signature,page[-1],None)
            while len(self.cursors)>128:self.cursors.popitem(last=False)
        return {'namespaces':rows,'cursor':cursor}

    def drop_namespace(self,v):
        name=v['namespace'];rows=self.catalogue.load();row=rows.get(name)
        repo=self.catalogue.existing(name,self.cache)
        if row is None:
            if repo.count():raise NamedRefusal('legacy_data','nonempty namespace has no model binding; owner re-feed required')
            return {'status':'absent','namespace':name}
        # This confirmed intent survives all partial/uncertain work below. Ordinary
        # data calls refuse it; only another explicit confirmed drop continues.
        row['state']='dropping';self.catalogue.save(rows)
        self.index_state.pop(name,None)
        for token,entry in list(self.cursors.items()):
            if entry[3]==name:self.cursors.pop(token,None)
        for table in row['tables']:self.cache.values.pop(self.cache.key(name,table),None)
        try:
            answer=repo.qdrant('POST','/points/delete?wait=true',{'filter':{}},write=True)
            if not isinstance(answer,dict) or answer.get('status')!='completed':raise Unknown('namespace deletion completion unknown')
            for table in row['tables']:repo.prompt('delete',table)
            rows.pop(name);self.catalogue.save(rows)
        except Exception:
            raise Unknown('namespace drop incomplete or outcome unknown; explicitly confirm continuation') from None
        return {'status':'dropped','namespace':name}
    def prepare(self,v,repo):
        source,chunks=extract(v); dimensions=self.cfg['dimensions']
        if len(chunks)*dimensions>SCALARS_MAX:raise RagError('chunks times dimensions exceeds 8192 scalars before provider work')
        settings=repo.prompt('get',v['tableName'])
        if settings is None:raise RagError('classification prompts are not configured')
        # Upper bound every possible selected label, before paying either provider.
        keys=sorted(settings['labels'],key=lambda x:len(dumps(x,ascii=True)),reverse=True)[:settings['top_k']]
        label_budget=sum(len(dumps(k,ascii=True))+26 for k in keys)+2
        base={k:v[k] for k in ('namespace','tableName','id','key1','key2','data','mime-type','source') if k in v}
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
                    # A provider answer that names a model must name the configured one. One that names none is tolerated: the
                    # provider does not always echo it, and the collection's own model binding already pins what the vectors mean.
                    if 'model' in answer and answer['model']!=self.cfg['embedding_model']:
                        raise NamedRefusal('model_mismatch','provider answered with another model')
                    rows=answer['data']
                    if not isinstance(rows,list) or len(rows)!=len(batch['input']):raise RagError('embedding batch size mismatch')
                    ordered={row['index']:row['embedding'] for row in rows if type(row.get('index')) is int}
                    if set(ordered)!=set(range(len(rows))):raise RagError('embedding indexes mismatch')
                    vectors.extend(vector(ordered[i],dimensions) for i in range(len(rows)))
                except (KeyError,TypeError,AttributeError):raise RagError('invalid embedding response') from None
        full={**base,'classification_ids':labels,'embeddings':vectors,
              'content_hash':hashlib.sha256(v['data'].encode()).hexdigest(),
              'indexed_at':datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00','Z')}
        if len(dumps(full,ascii=True).encode())>RECORD_MAX:raise RagError('prepared record exceeds byte cap')
        summary={k:full[k] for k in SUMMARY_FIELDS if k in full}
        if len(dumps(summary,ascii=True).encode())>SUMMARY_MAX:raise RagError('prepared summary exceeds byte cap')
        return full,chunks,vectors
    def upsert(self,v,repo):
        full,chunks,vectors=self.prepare(v,repo)
        rev=revision(full,vectors,self.cfg['embedding_model'],full['classification_ids'])
        fields={k:full[k] for k in ('namespace','tableName','id','key1','key2',*PROVENANCE_FIELDS) if k in full}
        ids=[repo.point_id(v,rev,i) for i in range(len(chunks))]
        points=[repo.point(v,{'':vec,BM25_VECTOR:sparse(chunk)} if self.bm25 else vec,{**fields,'_rag_kind':'chunk','_rag_index':i,'_rag_rev':rev,'_rag_labels':list(full['classification_ids']),'_rag_text':chunk},rev,i)
                for i,(pid,vec,chunk) in enumerate(zip(ids,vectors,chunks))]
        head={k:value for k,value in full.items() if k!='embeddings'}
        head.update(_rag_schema=SCHEMA,_rag_kind='head',_rag_rev=rev,_rag_chunks=ids,_rag_labels=list(full['classification_ids']))
        stage=0
        try:
            repo.qdrant('PUT','/points?wait=true',{'points':points},write=True)
            stage=1
            repo.qdrant('PUT','/points?wait=true',{'points':[repo.point(v,[1.0]+[0.0]*(self.cfg['dimensions']-1),head)]},write=True)
            old=repo.select(v,kind='chunk',exact=True);old['must_not']=[match('_rag_rev',rev)]
            repo.qdrant('POST','/points/delete?wait=true',{'filter':old},write=True)
        except (Refused,Deadline) as error:
            if stage==0:raise RagError('not stored; prior record unchanged: '+str(error)) from None
            raise Unknown('storage outcome unknown; repeating the same upsert/delete is safe') from None
        except RagError:
            raise Unknown('storage outcome unknown; repeating the same upsert/delete is safe') from None
        return full
    def get(self,v,repo):
        for attempt in range(2):
            try:head=repo.qdrant('GET','/points/'+repo.point_id(v)+'?with_vector=false')
            except RagError as error:
                # Backend adapters represent a missing point explicitly, never by accepting another record.
                if getattr(error,'missing',False):return None
                raise
            if head is None:return None
            try:
                payload=head['payload']
                if payload.get('_rag_kind')=='deleted':return None
                if head['id']!=repo.point_id(v) or payload.get('_rag_kind')!='head' or identity(payload)!=identity(v):raise RagError('invalid canonical head')
                ids=payload['_rag_chunks']
                if not isinstance(ids,list) or not 1<=len(ids)<=8 or len(ids)*self.cfg['dimensions']>SCALARS_MAX or len(set(ids))!=len(ids):raise RagError('invalid chunk references')
                if any(pid!=repo.point_id(v,payload['_rag_rev'],i) for i,pid in enumerate(ids)):raise RagError('chunk identity mismatch')
                rows=repo.qdrant('POST','/points',{'ids':ids,'with_payload':False,'with_vector':True})
                # A collection with a sparse vector answers each point's vectors by name; the dense one is the unnamed one.
                found={row['id']:row['vector'].get('') if isinstance(row['vector'],dict) else row['vector'] for row in rows}
                if set(found)!=set(ids) or len(rows)!=len(ids):
                    if attempt==0:continue
                    raise RagError('record changed or is incomplete; retry the read')
                result=dict(summary({k:payload[k] for k in SUMMARY_FIELDS if k in payload}))
                result['data']=text(payload.get('data'),'stored data',DATA_MAX,empty=True)
                if hashlib.sha256(result['data'].encode()).hexdigest()!=result['content_hash']:raise RagError('stored content hash mismatch')
                result['embeddings']=[vector(found[pid],self.cfg['dimensions']) for pid in ids]
                if len(dumps(result,ascii=True).encode())>RECORD_MAX:raise RagError('stored record exceeds read cap')
                return result
            except (KeyError,TypeError,AttributeError):raise RagError('invalid stored record') from None
        raise RagError('record changed or is incomplete')
    def list(self,v,repo):
        selected=repo.select(v,kind='head');signature=dumps([selected,v['limit']])
        offset=None
        if 'cursor' in v:
            entry=self.cursors.get(v['cursor'])
            if not entry or entry[0]<time.monotonic() or entry[1]!=signature:raise RagError('stale or mismatched cursor')
            self.cursors.move_to_end(v['cursor']);offset=entry[2]
        query={'filter':selected,'limit':v['limit'],'with_payload':{'include':list(SUMMARY_FIELDS)},'with_vector':False}
        if offset is not None:query['offset']=offset
        page=repo.qdrant('POST','/points/scroll',query)
        try:
            rows=page['points'];following=page['next_page_offset']
            if not isinstance(rows,list) or len(rows)>v['limit']:raise RagError('invalid scroll size')
            ids=[str(uuid.UUID(row['id'])) for row in rows]
            if ids!=sorted(ids) or len(ids)!=len(set(ids)) or offset is not None and any(i<offset for i in ids):raise RagError('invalid scroll order')
            if following is not None:
                if not rows or str(uuid.UUID(following))!=following or following<=ids[-1]:raise RagError('invalid scroll offset')
            records=[]
            for row in rows:
                record=summary({k:row['payload'][k] for k in SUMMARY_FIELDS if k in row['payload']})
                if record.get('namespace')!=v['namespace']:raise RagError('invalid scroll payload projection')
                if any(record.get(k)!=v[k] for k in ('tableName','key1','key2') if k in v):raise RagError('scroll filter mismatch')
                if len(dumps(record,ascii=True).encode())>SUMMARY_MAX:raise RagError('stored summary exceeds byte cap')
                records.append(record)
        except (KeyError,ValueError,TypeError,AttributeError):raise RagError('invalid scroll response') from None
        token=None
        if following is not None:
            token=secrets.token_urlsafe(24);self.cursors[token]=(time.monotonic()+600,signature,following,repo.name)
            while len(self.cursors)>128:self.cursors.popitem(last=False)
        return {'records':records,'cursor':token}
    def search(self,v,repo):
        """Records whose stored text best matches the query's words, best first, one row per record.

        A chunk answers only while its record's live head lists it: a draft whose head never landed, a
        replaced revision and a deleted record are skipped, as get() would not return them.
        """
        mode=v.get('mode','keyword')
        # Refused before any paid call: rerank needs a chosen model and the candidates this agent ranks itself.
        if v.get('rerank'):
            if not self.cfg['rerank_model']:raise NamedRefusal('rerank_unavailable','rerank_model is not configured')
            if mode=='keyword' and self.cfg['keyword_search']=='external':raise NamedRefusal('rerank_unavailable','external keyword search cannot be reranked')
        if mode=='keyword' and self.cfg['keyword_search']=='external':
            if self.keyword is None:raise RagError('keyword_search is external and no external keyword engine is connected')
            found=repo.external_search(self.keyword,v);seen=set()
            for record in found['records']:
                key=(record['tableName'],record['id'])
                if key in seen:raise RagError('duplicate external citation')
                seen.add(key)
                head=repo.qdrant('GET','/points/'+repo.point_id(record))
                if head is None or head['payload']['_rag_kind']!='head':raise RagError('external citation has no live head')
                payload=head['payload'];index=record['chunk_index']
                if index>=len(payload['_rag_chunks']):raise RagError('external citation chunk is absent')
                chunk=repo.qdrant('GET','/points/'+payload['_rag_chunks'][index])
                if chunk is None or chunk['payload']['_rag_kind']!='chunk' or chunk['payload']['_rag_rev']!=payload['_rag_rev']:
                    raise RagError('external citation is not a live chunk')
                if record['excerpt']!=chunk['payload'].get('_rag_text') or any(record[k]!=payload[k] or record[k]!=chunk['payload'][k] for k in PROVENANCE_FIELDS):
                    raise RagError('external citation provenance mismatch')
                if any(record[k]!=payload.get(k) for k in SUMMARY_FIELDS if k in record):raise RagError('external citation summary mismatch')
            return {**found,'mode':'keyword'}
        if mode in ('keyword','hybrid'):
            if self.cfg['keyword_search']=='external':raise NamedRefusal('unsupported_mode','hybrid requires the built-in BM25 index')
            if not self.bm25:raise RagError(self.keyword_state)
            left=self._unindexed(repo)
            if left:raise RagError(f'keyword index incomplete ({left} chunks left); run keyword_index with this namespace')
        dense=self.query_embedding(v,repo) if mode in ('semantic','hybrid') else None
        best={};want=min(64,v['limit']*4);exhausted=False
        for page in range(4):
            query={'filter':repo.select(v,kind='chunk'),'limit':want,'offset':page*want,
                   'with_payload':{'include':[*SUMMARY_FIELDS[:5],*PROVENANCE_FIELDS,'_rag_text']},'with_vector':False}
            if mode=='keyword':query.update(query=sparse(v['query'],query=True),using=BM25_VECTOR)
            elif mode=='semantic':query['query']=dense
            else:
                # Fixed prefetch windows keep the same RRF candidate set across result pages.
                query.update(query={'fusion':'rrf'},prefetch=[
                    {'query':dense,'filter':repo.select(v,kind='chunk'),'limit':want*4},
                    {'query':sparse(v['query'],query=True),'using':BM25_VECTOR,'filter':repo.select(v,kind='chunk'),'limit':want*4}])
            answer=repo.qdrant('POST','/points/query',query)
            try:
                rows=answer['points']
                if not isinstance(rows,list) or len(rows)>want:raise RagError('invalid query size')
                for row in rows:
                    payload=row['payload'];number(row['score'],-1 if mode=='semantic' else 0,float('inf'),'score')
                    if payload.get('namespace')!=v['namespace']:raise RagError('invalid query payload projection')
                    if any(payload.get(k)!=v[k] for k in ('tableName','key1','key2') if k in v):raise RagError('query filter mismatch')
                live=self._live(rows,repo)
                for row in rows:
                    payload=row['payload'];key=identity(payload)
                    head=live.get(repo.point_id(payload))
                    if key in best or head is None or row['id'] not in head['_rag_chunks'] or payload['_rag_rev']!=head['_rag_rev']:continue
                    if provenance(payload)!=provenance(head):raise RagError('live chunk provenance mismatch')
                    record={k:payload[k] for k in SUMMARY_FIELDS[:5] if k in payload}
                    record.update(provenance(payload))
                    record['chunk_index']=payload['_rag_index']
                    record['scores']={dict(keyword='bm25',semantic='dense',hybrid='rrf')[mode]:row['score']}
                    record['excerpt']=text(payload.get('_rag_text'),'stored text',DATA_MAX,empty=True)
                    best[key]=record
            except (KeyError,TypeError,AttributeError):raise RagError('invalid query response') from None
            if len(best)>=v['limit'] or len(rows)<want:break
            exhausted=page==3
        # An empty answer must mean no match, never a window that dead chunks filled.
        if exhausted and len(best)<v['limit']:raise Incomplete('search window exhausted before enough records were found; narrow the query, add filters or run cleanup')
        ranked=sorted(best.values(),key=lambda r:(-next(iter(r['scores'].values())),r['tableName'],r['id']))
        if not v.get('rerank'):return {'mode':mode,'records':ranked[:v['limit']]}
        # The candidate set is the same window the query asked for, never the whole namespace.
        candidates=ranked[:want]
        failure=self._rerank(v['query'],candidates)
        if failure:return {'mode':mode,'reranked':False,'rerank_failure':failure,'records':ranked[:v['limit']]}
        return {'mode':mode,'reranked':True,'records':candidates[:v['limit']]}
    def _rerank(self,query,candidates):
        """Reorder candidates in place by the rerank model's relevance; None once reordered, else a failure code.

        A failure leaves the list and every score untouched: no partial reorder. The code is ours, never provider text.
        """
        if not candidates:return None
        request={'model':self.cfg['rerank_model'],'query':query,'documents':[r['excerpt'] for r in candidates],
                 'return_documents':False,'truncation':True}
        if len(dumps({'method':'request','path':'/v1/rerank','body':dumps(request)}).encode())>BODY_MAX:return 'request_too_large'
        try:answer=self.backend.rerank(request)
        except Deadline:return 'deadline'
        except Refused:return 'provider_refused'
        except RagError:return 'provider_unavailable'
        try:
            if not isinstance(answer,dict):return 'invalid_answer'
            if 'model' in answer and answer['model']!=self.cfg['rerank_model']:return 'model_mismatch'
            rows=answer['data']
            if not isinstance(rows,list) or len(rows)!=len(candidates):return 'invalid_answer'
            scores={}
            for row in rows:
                index,score=row['index'],row['relevance_score']
                if type(index) is not int or not 0<=index<len(candidates) or index in scores:return 'invalid_answer'
                if type(score) not in (int,float) or not math.isfinite(score):return 'invalid_answer'
                scores[index]=float(score)
        except (KeyError,TypeError,AttributeError):return 'invalid_answer'
        order=sorted(scores,key=lambda i:(-scores[i],i))
        for i in order:candidates[i]['scores']['rerank']=scores[i]
        candidates[:]=[candidates[i] for i in order]
        return None
    def query_embedding(self,v,repo):
        # Catalogue and collection model checks have already completed before this one paid call.
        cfg=repo.config
        request={'model':cfg['embedding_model'],'input':[v['query']],'input_type':'query',
                 'output_dimension':cfg['dimensions'],'output_dtype':'float'}
        answer=self.backend.embed(request)
        try:
            rows=answer['data']
            if not isinstance(rows,list) or len(rows)!=1 or type(rows[0].get('index')) is not int or rows[0]['index']!=0:raise RagError('invalid query embedding response')
            # Same rule as for documents: a named model must be the configured one, and an answer naming none is tolerated.
            if 'model' in answer and answer['model']!=cfg['embedding_model']:raise NamedRefusal('model_mismatch','provider answered with another model')
            return vector(rows[0]['embedding'],cfg['dimensions'])
        except (KeyError,TypeError,AttributeError):raise RagError('invalid query embedding response') from None

    def _unindexed(self,repo):
        """How many chunks still lack the text vector; known complete once it was counted as none."""
        # Another writer, an older build or a restore can add chunks without the vector, so a zero holds for a minute only.
        complete,checked=self.index_state.get(repo.name,(False,0))
        if complete and time.monotonic()-checked<INDEX_RECHECK:return 0
        found=repo.qdrant('POST','/points/count',{'filter':{'must':[match('_rag_kind','chunk')],'must_not':[{'has_vector':BM25_VECTOR}]},'exact':True})
        try:n=found['count']
        except (KeyError,TypeError):raise RagError('invalid count response') from None
        if type(n) is not int or n<0:raise RagError('invalid count response')
        self.index_state[repo.name]=(n==0,time.monotonic())
        return n
    def _live(self,rows,repo):
        """Head id -> the chunk ids its live head lists, for the records the rows belong to; a missing or deleted head lists none."""
        heads=sorted({repo.point_id(row['payload']) for row in rows})
        if not heads:return {}
        found=repo.qdrant('POST','/points',{'ids':heads,'with_payload':{'include':['_rag_kind','_rag_chunks',*PROVENANCE_FIELDS]},'with_vector':False})
        live={}
        for head in found:
            payload=head.get('payload') or {}
            chunks=payload.get('_rag_chunks')
            if payload.get('_rag_kind')=='head' and isinstance(chunks,list):live[head['id']]=payload
        return live
    def keyword_index(self,envelope,repo):
        """Add the text sparse vector when the collection lacks it, then give every chunk its words; safe to repeat.

        Chunks already carrying the vector are not listed again, so a repeat after an interruption continues where it
        stopped. The proxy asks for its admin role to add the vector; the answer then names the owner's step.
        """
        if not {'owner','maintainer'}&set((envelope or {}).get('roles') or []):raise RagError('keyword_index is for this helper\'s Owner or Maintainers')
        if self.cfg['keyword_search']=='external':raise RagError('keyword_search is external; there is no built-in keyword index to build')
        self.verify()
        if self.keyword_state==NO_VECTOR:
            try:repo.qdrant('PUT','/vectors/'+BM25_VECTOR,{'sparse':{'modifier':'idf'}},write=True)
            except Refused as error:
                raise RagError(f'adding the text vector needs the Qdrant proxy\'s admin role ({error}); run as its owner: request PUT /collections/{self.cfg["collection"]}/vectors/{BM25_VECTOR} {{"sparse":{{"modifier":"idf"}}}} , then repeat this command') from None
            self.verify()
        if not self.bm25:raise RagError(self.keyword_state)
        done=0;seen=set()
        while True:
            try:self.budget.left(10)
            except Deadline:raise Incomplete(f'{done} chunks indexed so far; repeat the same command to continue') from None
            page=repo.qdrant('POST','/points/scroll',{'filter':{'must':[match('_rag_kind','chunk')],'must_not':[{'has_vector':BM25_VECTOR}]},
                'limit':64,'with_payload':{'include':['_rag_text']},'with_vector':False})
            try:
                rows=page['points']
                if not isinstance(rows,list) or len(rows)>64:raise RagError('invalid scroll size')
                if not rows:
                    self.index_state[repo.name]=(True,time.monotonic());return {'status':'ok','indexed':done}
                if seen&{row['id'] for row in rows}:raise RagError('the chunks just indexed still lack the vector; stopping')
                update=[];size=0
                for row in rows:
                    item={'id':row['id'],'vector':{BM25_VECTOR:sparse(text(row['payload'].get('_rag_text'),'stored text',DATA_MAX,empty=True))}}
                    size+=len(dumps(item))
                    # The proxy takes 1 MiB a call; what does not fit is listed again by the next page.
                    if update and size>INDEX_BYTES:break
                    update.append(item)
            except (KeyError,TypeError,AttributeError):raise RagError('invalid scroll response') from None
            repo.qdrant('PUT','/points/vectors?wait=true',{'points':update},write=True)
            seen.update(item['id'] for item in update);done+=len(update)
    def delete(self,v,repo):
        if 'id' in v:
            # A retained tombstone is an explicit continuation, not a new delete
            # of an absent record. It must still be able to finish its cleanup.
            head=repo.qdrant('GET','/points/'+repo.point_id(v))
            if head is None:raise NamedRefusal('not_found','record is absent from this namespace')
            if head['payload']['_rag_kind'] not in ('head','deleted'):raise RagError('invalid deletion head')
        try:return self._delete(v,repo)
        except Deadline:raise Incomplete('repeat the same delete to continue') from None
        except RagError:raise Unknown('storage outcome unknown; repeating the same upsert/delete is safe') from None
    def _delete(self,v,repo):
        # A tombstone removes the head from reads first, retaining identities for
        # repeatable filtered cleanup if a later step fails. No source is lost to an upsert failure.
        live=repo.select(v,kind='head')
        repo.qdrant('POST','/points/payload?wait=true',{'payload':{'_rag_kind':'deleted'},'filter':live},write=True)
        if 'id' in v:
            self._finish_delete(v,repo)
        else:
            while True:
                self.budget.left(10)
                page=repo.qdrant('POST','/points/scroll',{'filter':repo.select(v,kind='deleted'),'limit':16,
                    'with_payload':{'include':['namespace','tableName','id','key1','key2','_rag_kind']},'with_vector':False})
                try:rows=page['points']
                except (KeyError,TypeError):raise RagError('invalid deletion scroll') from None
                if not isinstance(rows,list) or len(rows)>16:raise RagError('invalid deletion scroll size')
                if not rows:break
                for row in rows:
                    record=row['payload']
                    if not isinstance(record,dict) or record.get('namespace')!=v['namespace']:raise RagError('invalid deletion scope')
                    if row.get('id')!=repo.point_id(record) or record.get('_rag_kind')!='deleted':raise RagError('invalid deletion head')
                    if any(record.get(k)!=v[k] for k in ('tableName','key1','key2') if k in v):raise RagError('invalid deletion filter')
                    self._finish_delete(record,repo)
            # Also remove pre-head chunks from a failed first upsert. Public key
            # filters are present on every chunk; this never broadens the selection.
            repo.qdrant('POST','/points/delete?wait=true',{'filter':repo.select(v,kind='chunk')},write=True)
        return {'status':'ok'}
    def _finish_delete(self,v,repo):
        repo.qdrant('POST','/points/delete?wait=true',{'filter':repo.select(v,kind='chunk',exact=True)},write=True)
        repo.qdrant('POST','/points/delete?wait=true',{'filter':repo.select(v,kind='deleted',exact=True)},write=True)
