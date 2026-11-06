"""The RAG wire and storage bounds; IDs and chunking define the persisted format."""
import datetime
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
PROVENANCE_FIELDS = ('source','content_hash','indexed_at')
SUMMARY_FIELDS = ('namespace','tableName','id','key1','key2','mime-type','classification_ids',*PROVENANCE_FIELDS)
ID_NAMESPACE = uuid.UUID('14c03739-20d3-5372-b739-a919d36eeb4f')
CHUNK_FORMAT = 'utf8-ws4096-half-v1'
KEYWORD_MODES = ('builtin','external')
QUERY_MAX = 1024
SEARCH_MODES = ('keyword','semantic','hybrid')
METHODS = ('upsert','get','list','search','keyword_index','delete','prompts','capabilities','namespaces','drop_namespace')
# Below own-KV's 512 KiB and the proxy body cap, including worst-case JSON escaping.
CATALOGUE_MAX = 256 << 10
NAMESPACE_MAX = 128
TABLES_MAX = 256
SOURCE_MAX = 2048
BM25_VECTOR = 'text'
# Qdrant's own BM25: the client sends term weights, the collection's idf modifier supplies the rest.
BM25_K1 = 1.2
BM25_B = 0.75
BM25_AVG_TOKENS = 128
TOKEN_MAX = 64
INDEX_BYTES = 512 << 10
INDEX_RECHECK = 60   # seconds a count of zero unindexed chunks is trusted
NO_VECTOR = 'keyword index not built yet; run {"method":"keyword_index"}'
NO_IDF = 'the collection\'s text vector lacks the idf modifier, so search is unavailable; recreate that vector with {"sparse":{"modifier":"idf"}}'


class RagError(ValueError):
    pass


class Unknown(RagError):
    pass


class Refused(RagError):
    pass


class NamedRefusal(Refused):
    """Machine-readable refusal name, also retained in the plain-text answer."""
    def __init__(self,code,detail):
        self.code=code
        super().__init__(code+': '+detail)


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
    except (ValueError,UnicodeError,TypeError,RecursionError):
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
    if method=='capabilities':
        if set(v)!={'method'}: raise RagError('unknown method field')
        return v
    if method=='namespaces':
        if set(v)-{'method','limit','cursor'}: raise RagError('unknown method field')
        integer(v.setdefault('limit',16),1,16,'limit')
        if 'cursor' in v: text(v['cursor'],'cursor',128)
        return v
    allowed={'upsert':{'namespace','tableName','id','data','mime-type','key1','key2','embeddings','source','content_hash'},
             'get':{'namespace','tableName','id'},'list':{'namespace','tableName','key1','key2','limit','cursor'},
             'search':{'namespace','tableName','key1','key2','query','limit','mode','rerank'},
             'keyword_index':{'namespace'},'drop_namespace':{'namespace','confirm'},
             'delete':{'namespace','tableName','id','key1','key2','all'},'prompts':{'namespace','tableName','action','data'}}
    if not isinstance(method,str) or method not in allowed: raise NamedRefusal('unsupported_method','unknown method')
    if set(v)-allowed[method]-{'method'}: raise RagError('unknown method field')
    text(v.get('namespace'),'namespace',128,nfc=True)
    if v['namespace']=='*': raise NamedRefusal('invalid_namespace','wildcard namespace is not allowed')
    if method=='drop_namespace':
        if 'confirm' in v: text(v['confirm'],'confirm',128,nfc=True)
        return v
    required=('tableName','id') if method in ('upsert','get') else ('tableName',) if method=='prompts' else ()
    for k in required: text(v.get(k),k,128,nfc=True)
    for k in ('tableName','id'):
        if k in v: text(v[k],k,128,nfc=True)
    for k in ('key1','key2'):
        if k in v: text(v[k],k,256,empty=True,nfc=True)
    if method=='upsert':
        text(v.get('source'),'source',SOURCE_MAX,nfc=True)
        text(v.get('data'),'data',DATA_MAX,empty=True)
        if 'content_hash' in v:
            if not isinstance(v['content_hash'],str) or not re.fullmatch('[0-9a-f]{64}',v['content_hash']): raise RagError('content_hash must be lower-case SHA256')
            if v['content_hash']!=hashlib.sha256(v.get('data','').encode()).hexdigest(): raise RagError('content_hash does not match source bytes')
        mime=v.setdefault('mime-type','text/plain')
        if mime not in ('text/plain','text/markdown','text/html','application/json'): raise RagError('unsupported mime-type')
    if method=='list':
        integer(v.setdefault('limit',16),1,16,'limit')
        if 'cursor' in v: text(v['cursor'],'cursor',128)
    if method=='search':
        text(v.get('query'),'query',QUERY_MAX)
        if v.setdefault('mode','keyword') not in SEARCH_MODES: raise NamedRefusal('unsupported_mode','expected keyword, semantic or hybrid')
        if v['mode'] in ('keyword','hybrid') and not tokens(v['query']): raise RagError('query has no searchable words')
        integer(v.setdefault('limit',16),1,16,'limit')
        if 'rerank' in v and type(v['rerank']) is not bool: raise RagError('rerank must be boolean')
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
    if not isinstance(v,dict) or set(v)-required-{'decision_model','keyword_search','rerank_model'} or not required<=set(v): raise RagError('configure block names, collection, embedding_model and dimensions')
    v=dict(v)
    if v.setdefault('keyword_search','builtin') not in KEYWORD_MODES: raise RagError('keyword_search is builtin or external')
    for k in ('qdrant','voyage','ai'):
        text(v[k],k,256)
        if not v[k].startswith('#'): raise RagError(k+' must be a complete agent name')
    if not isinstance(v['collection'],str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,64}',v['collection']): raise RagError('invalid collection name')
    for k in ('embedding_model','decision_model'):
        if k=='decision_model': v.setdefault(k,'typesafe/jev-1.14')
        if not isinstance(v[k],str) or not re.fullmatch(r'[A-Za-z0-9._/~:@+-]{1,128}',v[k]): raise RagError('invalid model ID')
    # Empty means rerank is off: search refuses rerank:true rather than calling a model nobody chose.
    if v.setdefault('rerank_model','')!='' and (not isinstance(v['rerank_model'],str) or not re.fullmatch(r'[A-Za-z0-9._/~:@+-]{1,128}',v['rerank_model'])): raise RagError('invalid rerank model ID')
    integer(v['dimensions'],1,8192,'dimensions')
    return v


def tokens(value):
    """Lower-cased word tokens: Unicode letters, digits and underscore, each at most 64 characters."""
    return [t for t in re.findall(r'\w+',unicodedata.normalize('NFKC',value).casefold()) if len(t)<=TOKEN_MAX]


def term_index(token):
    return int.from_bytes(hashlib.blake2s(token.encode(),digest_size=4).digest(),'big')


def sparse(value,*,query=False):
    """A Qdrant sparse vector {indices,values} of value's terms; indices ascend and never repeat.

    A document term weighs its saturated BM25 term frequency; a query term weighs 1.
    """
    counts={}
    words=tokens(value)
    for word in words:
        index=term_index(word);counts[index]=counts.get(index,0)+1
    length=len(words) or 1
    weights={i:1.0 if query else n*(BM25_K1+1)/(n+BM25_K1*(1-BM25_B+BM25_B*length/BM25_AVG_TOKENS)) for i,n in counts.items()}
    order=sorted(weights)
    return {'indices':order,'values':[round(weights[i],6) for i in order]}


def identity(v):
    return tuple(v[k] for k in ('namespace','tableName','id'))


def point_id(v,revision=None,index=None):
    parts=[*identity(v),'head'] if revision is None else [*identity(v),'chunk',revision,index]
    return str(uuid.uuid5(ID_NAMESPACE,dumps(parts)))


def revision(v,vectors,model,labels):
    value=[v['data'],v['mime-type'],[v.get(k) for k in ('key1','key2')],sorted(labels),model,CHUNK_FORMAT,vectors,v.get('source'),v.get('content_hash'),v.get('indexed_at')]
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


def provenance(value):
    try:
        text(value.get('source'),'source',SOURCE_MAX,nfc=True)
        digest=value.get('content_hash')
        if not isinstance(digest,str) or not re.fullmatch('[0-9a-f]{64}',digest):raise ValueError()
        stamp=value.get('indexed_at')
        if not isinstance(stamp,str) or len(stamp)>40 or not stamp.endswith('Z'):raise ValueError()
        parsed=datetime.datetime.fromisoformat(stamp[:-1]+'+00:00')
        if parsed.utcoffset()!=datetime.timedelta():raise ValueError()
    except (ValueError,TypeError,AttributeError):raise NamedRefusal('legacy_data','stored provenance is absent or invalid; re-feed required') from None
    return {key:value[key] for key in PROVENANCE_FIELDS}


def summary(value):
    if not isinstance(value,dict) or set(value)-set(SUMMARY_FIELDS):raise RagError('invalid record summary fields')
    for key in ('namespace','tableName','id'):text(value.get(key),key,128,nfc=True)
    provenance(value)
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
