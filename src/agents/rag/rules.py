"""The RAG wire and storage bounds; changes to IDs/chunking require migration."""
import hashlib
import json
import math
import re
import struct
import unicodedata
import uuid

BODY_MAX = 1 << 20
RECORD_MAX = 256 << 10
SUMMARY_MAX = 32 << 10
DATA_MAX = 32 << 10
CHUNK_BYTES = 4096
CHUNK_MAX = 8
SCALARS_MAX = 8192
INDEXES = ('namespace','tableName','id','key1','key2','_rag_kind','_rag_labels','_rag_rev')
SUMMARY_FIELDS = ('namespace','tableName','id','key1','key2','mime-type','classification_ids')
ID_NAMESPACE = uuid.UUID('14c03739-20d3-5372-b739-a919d36eeb4f')
CHUNK_FORMAT = 'utf8-ws4096-half-v1'


class RagError(ValueError):
    pass


class Unknown(RagError):
    pass


class Refused(RagError):
    pass


class Deadline(RagError):
    pass


class Incomplete(RagError):
    pass


def dumps(value, *, ascii=False):
    return json.dumps(value,ensure_ascii=ascii,allow_nan=False,separators=(',',':'),sort_keys=True)


def reject(_):
    raise RagError('non-standard JSON constant')


def pairs(items):
    value={}
    for k,v in items:
        if k in value: raise RagError('duplicate JSON key')
        value[k]=v
    return value


def loads(raw, maximum=BODY_MAX):
    try:
        if not isinstance(raw,str) or len(raw.encode('utf-8'))>maximum: raise RagError('JSON body exceeds byte limit')
        return json.loads(raw,object_pairs_hook=pairs,parse_constant=reject)
    except (ValueError,UnicodeError,TypeError):
        raise RagError('invalid JSON, encoding, duplicate key or byte limit') from None


def text(value, name, maximum, *, empty=False, nfc=False):
    try:
        if not isinstance(value,str) or len(value.encode('utf-8'))>maximum: raise RagError(name+' exceeds text limit')
    except UnicodeError: raise RagError(name+' must be UTF-8') from None
    if '\x00' in value or '\ufeff' in value: raise RagError(name+' contains NUL or BOM')
    if not empty and not value.strip(): raise RagError(name+' is required')
    if nfc and (unicodedata.normalize('NFC',value)!=value or '\n' in value or '\r' in value):
        raise RagError(name+' must be NFC without line breaks')
    return value


def integer(value,low,high,name):
    if type(value) is not int or not low<=value<=high: raise RagError(name+' integer out of range')
    return value


def finite(value):
    try:return type(value) in (int,float) and math.isfinite(value)
    except OverflowError:return False


def number(value,low,high,name):
    if not finite(value) or not low<=value<=high:
        raise RagError(name+' must be a finite number in range')
    return value


def vector(values,dimensions):
    if not isinstance(values,list) or len(values)!=dimensions: raise RagError('vector dimension mismatch')
    result=[]
    for v in values:
        if not finite(v): raise RagError('vector requires finite numbers, not booleans')
        try: f=struct.unpack('!f',struct.pack('!f',v))[0]
        except (OverflowError,struct.error): raise RagError('vector exceeds float32 range') from None
        if not math.isfinite(f): raise RagError('vector exceeds float32 range')
        result.append(f)
    return result


def parse(raw):
    v=loads(raw)
    if not isinstance(v,dict): raise RagError('expected a JSON method object')
    method=v.get('method')
    allowed={'upsert':{'namespace','tableName','id','data','mime-type','key1','key2','embeddings'},
             'get':{'namespace','tableName','id'},'list':{'namespace','tableName','key1','key2','limit','cursor'},
             'delete':{'namespace','tableName','id','key1','key2','all'},'prompts':{'namespace','tableName','action','data'}}
    if not isinstance(method,str) or method not in allowed: raise RagError('unknown method')
    if set(v)-allowed[method]-{'method'}: raise RagError('unknown method field')
    text(v.get('namespace'),'namespace',128,nfc=True)
    required=('tableName','id') if method in ('upsert','get') else ('tableName',) if method=='prompts' else ()
    for k in required: text(v.get(k),k,128,nfc=True)
    for k in ('tableName','id'):
        if k in v: text(v[k],k,128,nfc=True)
    for k in ('key1','key2'):
        if k in v: text(v[k],k,256,empty=True,nfc=True)
    if method=='upsert':
        text(v.get('data'),'data',DATA_MAX,empty=True)
        mime=v.setdefault('mime-type','text/plain')
        if mime not in ('text/plain','text/markdown','text/html','application/json'): raise RagError('unsupported mime-type')
    if method=='list':
        integer(v.setdefault('limit',16),1,16,'limit')
        if 'cursor' in v: text(v['cursor'],'cursor',128)
    if method=='delete':
        if 'all' in v and type(v['all']) is not bool: raise RagError('all must be boolean')
        if 'id' in v:
            if 'tableName' not in v or any(k in v for k in ('key1','key2','all')): raise RagError('id delete needs tableName and forbids key filters/all')
        elif v.get('all') is not True: raise RagError('filtered delete requires all:true')
    if method=='prompts':
        if v.get('action') not in ('get','set','delete'): raise RagError('prompts action must be get, set or delete')
        if (v['action']=='set')!=('data' in v): raise RagError('only prompts set requires data')
    return v


def config(v):
    required={'qdrant','voyage','ai','collection','embedding_model','dimensions'}
    if not isinstance(v,dict) or set(v)-required-{'decision_model'} or not required<=set(v): raise RagError('configure block names, collection, embedding_model and dimensions')
    v=dict(v)
    for k in ('qdrant','voyage','ai'):
        text(v[k],k,256)
        if not v[k].startswith('#'): raise RagError(k+' must be a complete agent name')
    if not isinstance(v['collection'],str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,64}',v['collection']): raise RagError('invalid collection name')
    for k in ('embedding_model','decision_model'):
        if k=='decision_model': v.setdefault(k,'typesafe/jev-1.14')
        if not isinstance(v[k],str) or not re.fullmatch(r'[A-Za-z0-9._/~:@+-]{1,128}',v[k]): raise RagError('invalid model ID')
    integer(v['dimensions'],1,8192,'dimensions')
    return v


def identity(v):
    return tuple(v[k] for k in ('namespace','tableName','id'))


def point_id(v,revision=None,index=None):
    parts=[*identity(v),'head'] if revision is None else [*identity(v),'chunk',revision,index]
    return str(uuid.uuid5(ID_NAMESPACE,dumps(parts)))


def revision(v,vectors,model,labels):
    value=[v['data'],v['mime-type'],[v.get(k) for k in ('key1','key2')],sorted(labels),model,CHUNK_FORMAT,vectors]
    return hashlib.sha256(dumps(value).encode()).hexdigest()


def match(key,value):
    return {'key':key,'match':{'value':value}}


def scope(v, *, kind=None, exact=False):
    # Callers only pass parsed requests; missing namespace is still fail-closed.
    text(v.get('namespace'),'namespace',128,nfc=True)
    fields=('namespace','tableName','id') if exact else ('namespace','tableName','id','key1','key2')
    must=[]
    for k in fields:
        if exact or k in v:
            text(v.get(k),k,256 if k.startswith('key') else 128,empty=k.startswith('key'),nfc=True)
            must.append(match(k,v[k]))
    if kind: must.append(match('_rag_kind',kind))
    return {'must':must}


def summary(value):
    if not isinstance(value,dict) or set(value)-set(SUMMARY_FIELDS):raise RagError('invalid record summary fields')
    for key in ('namespace','tableName','id'):text(value.get(key),key,128,nfc=True)
    for key in ('key1','key2'):
        if key in value:text(value[key],key,256,empty=True,nfc=True)
    if value.get('mime-type') not in ('text/plain','text/markdown','text/html','application/json'):raise RagError('invalid stored mime-type')
    labels=value.get('classification_ids')
    if not isinstance(labels,dict) or len(labels)>64:raise RagError('invalid stored classifications')
    for key,probability in labels.items():
        text(key,'label',128,nfc=True);number(probability,0,1,'probability')
    if len(dumps(value,ascii=True).encode())>SUMMARY_MAX:raise RagError('stored summary exceeds byte cap')
    return value


def prompt_key(namespace,table):
    return 'rag.prompts.'+hashlib.sha256(dumps([namespace,table]).encode()).hexdigest()
