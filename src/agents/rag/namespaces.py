"""The only data-plane Qdrant and prompt-KV access: one immutable namespace per call."""
from copy import deepcopy
from dataclasses import dataclass
import re
from types import MappingProxyType
from rules import (RagError,NamedRefusal,Unknown,CATALOGUE_MAX,NAMESPACE_MAX,TABLES_MAX,
                   dumps,text,integer,match,point_id,scope)

CATALOGUE_KEY='rag.namespaces'
SCHEMA=3
IDENTITY_FIELDS=['namespace','tableName','id','_rag_kind','_rag_rev','_rag_index','_rag_schema','_rag_chunks']


def manager(envelope):
    if not {'owner','maintainer'} & set((envelope or {}).get('roles') or []):
        raise NamedRefusal('forbidden','requires this agent\'s Owner or Maintainers')


def namespace(value):
    text(value,'namespace',128,nfc=True)
    if value=='*':raise NamedRefusal('invalid_namespace','wildcard namespace is not allowed')
    return value


class Catalogue:
    """One bounded own-KV value. Always re-read; an uncertain write never seeds a cache."""
    def __init__(self,kv,backend,cfg):self.kv,self.backend,self.cfg=kv,backend,cfg

    def load(self):
        value=self.kv.get(CATALOGUE_KEY)
        if value is None:return {}
        try:
            if not isinstance(value,dict) or set(value)!={'schema','namespaces'} or type(value['schema']) is not int or value['schema']!=SCHEMA:raise ValueError()
            rows=value['namespaces']
            if not isinstance(rows,dict) or len(rows)>NAMESPACE_MAX:raise ValueError()
            if len(dumps(value,ascii=True).encode())>CATALOGUE_MAX:raise ValueError()
            for name,row in rows.items():
                namespace(name)
                if not isinstance(row,dict) or set(row)!={'model','dimensions','tables','state'}:raise ValueError()
                if not isinstance(row['model'],str) or not re.fullmatch(r'[A-Za-z0-9._/~:@+-]{1,128}',row['model']):raise ValueError()
                integer(row['dimensions'],1,8192,'dimensions')
                if row['state'] not in ('active','dropping'):raise ValueError()
                if not isinstance(row['tables'],list) or len(row['tables'])>TABLES_MAX or len(set(row['tables']))!=len(row['tables']):raise ValueError()
                for table in row['tables']:text(table,'tableName',128,nfc=True)
            return deepcopy(rows)
        except (ValueError,TypeError,KeyError):raise NamedRefusal('state_invalid','namespace catalogue is invalid; owner inspection required') from None

    def save(self,rows):
        value={'schema':SCHEMA,'namespaces':deepcopy(rows)}
        if len(rows)>NAMESPACE_MAX or len(dumps(value,ascii=True).encode())>CATALOGUE_MAX:
            raise NamedRefusal('limit','namespace catalogue is full')
        try:self.kv.set(CATALOGUE_KEY,value)
        except Exception:raise Unknown('namespace metadata outcome unknown; inspect before repeating') from None

    def bind(self,name,cache,table=None,*,dropping=False):
        name=namespace(name);rows=self.load();row=rows.get(name)
        if row is None:
            # Count only this namespace. A binding cannot be inferred from existing records.
            found=self.backend.qdrant('POST','/points/count',{'filter':{'must':[match('namespace',name)]},'exact':True})
            if not isinstance(found,dict) or type(found.get('count')) is not int or found['count']<0:raise RagError('invalid namespace count')
            if found['count']:raise NamedRefusal('legacy_data','nonempty namespace has no model binding; re-feed into a fresh collection')
            row={'model':self.cfg['embedding_model'],'dimensions':self.cfg['dimensions'],'tables':[],'state':'active'}
            rows[name]=row;changed=True
        else:changed=False
        if row['model']!=self.cfg['embedding_model'] or row['dimensions']!=self.cfg['dimensions']:
            raise NamedRefusal('model_mismatch','namespace model and dimensions are immutable; provision a matching instance')
        if row['state']=='dropping' and not dropping:raise NamedRefusal('busy','namespace drop is incomplete; explicitly confirm continuation')
        if table is not None and table not in row['tables']:
            text(table,'tableName',128,nfc=True)
            if len(row['tables'])>=TABLES_MAX:raise NamedRefusal('limit','namespace table catalogue is full')
            row['tables'].append(table);row['tables'].sort();changed=True
        if changed:self.save(rows) # Confirm before a prompt write, classifier or embedding call.
        return NamespaceAccess(name,self.backend,cache,MappingProxyType(deepcopy(self.cfg)))


@dataclass(frozen=True)
class NamespaceAccess:
    name:str
    _backend:object
    _cache:object
    config:object

    def external_search(self,engine,request):
        if request.get('namespace')!=self.name:raise NamedRefusal('scope_mismatch','search is outside the bound namespace')
        found=engine(deepcopy(request))
        if not isinstance(found,dict) or not isinstance(found.get('records'),list) or len(found['records'])>request['limit']:raise RagError('invalid external keyword answer')
        for record in found['records']:
            if not isinstance(record,dict) or record.get('namespace')!=self.name:raise NamedRefusal('scope_mismatch','external engine returned a foreign namespace')
            for key in ('tableName','id'):text(record.get(key),key,128,nfc=True)
            if any(record.get(key)!=request[key] for key in ('tableName','key1','key2') if key in request):raise RagError('external search filter mismatch')
        return found

    def point_id(self,value,revision=None,index=None):
        if value.get('namespace')!=self.name:raise NamedRefusal('scope_mismatch','identity is outside the bound namespace')
        return point_id(value,revision,index)

    def select(self,value,**options):
        if value.get('namespace')!=self.name:raise NamedRefusal('scope_mismatch','selection is outside the bound namespace')
        return scope(value,**options)

    def point(self,value,vector,payload,revision=None,index=None):
        # The boundary owns the persisted namespace, schema and deterministic identity.
        payload={**deepcopy(payload),'namespace':self.name,'_rag_schema':SCHEMA}
        return {'id':self.point_id(value,revision,index),'vector':vector,'payload':payload}

    def filter(self,selected=None):
        result=deepcopy(selected or {})
        # Top-level AND cannot be replaced by an inner OR or caller filter.
        result['must']=[match('namespace',self.name),*result.get('must',[])]
        return result

    def prompt(self,action,table,value=None):
        text(table,'tableName',128,nfc=True)
        if action=='get':return self._cache.get(self.name,table)
        if action=='set':return self._cache.set(self.name,table,value)
        if action=='delete':return self._cache.delete(self.name,table)
        raise RagError('invalid prompt operation')

    def _validate(self,row):
        try:
            payload=row['payload']
            if payload.get('namespace')!=self.name:raise NamedRefusal('scope_mismatch','backend returned a foreign namespace')
            for key in ('tableName','id'):text(payload.get(key),key,128,nfc=True)
            if type(payload.get('_rag_schema')) is not int or payload['_rag_schema']!=SCHEMA:
                raise NamedRefusal('legacy_data','record has no current namespace/provenance contract; re-feed required')
            kind=payload.get('_rag_kind')
            if kind in ('head','deleted'):expected=point_id(payload)
            elif kind=='chunk':
                index=integer(payload.get('_rag_index'),0,7,'chunk index')
                rev=payload.get('_rag_rev')
                if not isinstance(rev,str) or not re.fullmatch('[0-9a-f]{64}',rev):raise RagError('invalid chunk revision')
                expected=point_id(payload,rev,index)
            else:raise RagError('invalid point kind')
            if row['id']!=expected:raise RagError('point identity mismatch')
        except (KeyError,TypeError,AttributeError):raise RagError('invalid scoped point') from None
        return row

    def _projection(self,body):
        # Identity validation applies even when the handler only needs vectors or a summary.
        keep=body.get('with_payload')
        if isinstance(keep,dict) and isinstance(keep.get('include'),list):
            body['with_payload']={'include':list(dict.fromkeys([*keep['include'],*IDENTITY_FIELDS]))}
        else:body['with_payload']=True

    def qdrant(self,verb,path,body=None,*,write=False):
        body=deepcopy(body)
        single=False;ids=None
        if verb=='GET' and path.startswith('/points/'):
            ids=[path.split('?',1)[0].rsplit('/',1)[1]];single=True
            body={'filter':{'must':[{'has_id':ids}]},'limit':1,'with_payload':True,'with_vector':False}
            verb,path='POST','/points/scroll'
        elif path=='/points' and verb=='POST':
            ids=body['ids']
            if not isinstance(ids,list) or not 1<=len(ids)<=64:raise RagError('invalid scoped id batch')
            body={'filter':{'must':[{'has_id':ids}]},'limit':len(ids),'with_payload':body.get('with_payload',True),'with_vector':body.get('with_vector',False)}
            path='/points/scroll'
        if path in ('/points/scroll','/points/query','/points/count') or path.startswith('/points/delete?') or path.startswith('/points/payload?'):
            if 'points' in (body or {}):raise RagError('unfiltered point deletion is not allowed')
            body['filter']=self.filter(body.get('filter'))
            for prefetch in body.get('prefetch',[]):prefetch['filter']=self.filter(prefetch.get('filter'))
            if path in ('/points/scroll','/points/query'):self._projection(body)
        elif verb=='PUT' and path.startswith('/points/vectors?'):
            wanted=[p['id'] for p in body['points']]
            found=self.qdrant('POST','/points',{'ids':wanted,'with_payload':True,'with_vector':False})
            if len(found)!=len(wanted) or {p['id'] for p in found}!=set(wanted):raise RagError('vector update includes a missing or foreign point')
        elif verb=='PUT' and path.startswith('/points?'):
            for row in body['points']:self._validate(row)
        elif path.startswith('/vectors/'):
            # Shared schema definition, never a data scan/backfill.
            pass
        else:raise RagError('unbound data operation is not allowed')
        answer=self._backend.qdrant(verb,path,body,write=write)
        if path in ('/points/scroll','/points/query'):
            try:rows=answer['points']
            except (TypeError,KeyError):raise RagError('invalid scoped response') from None
            if not isinstance(rows,list) or len(rows)>body['limit']:raise RagError('invalid scoped response size')
            for row in rows:
                self._validate(row)
                if ids is not None and row['id'] not in ids:raise RagError('backend returned an unrequested id')
            if single:return rows[0] if rows else None
            if ids is not None:
                if len({row['id'] for row in rows})!=len(rows):raise RagError('duplicate scoped point')
                return rows
        return answer
