"""One focused check of built-in keyword search against a real Qdrant, in a throwaway rootless container (T-25).

The fake in test_rag.py proves our requests; only the server proves that an unnamed dense vector, the `text`
sparse vector with the idf modifier, the mixed upsert, the query and the read-back behave as the code assumes.
The container belongs to a gate or a mutant run: it needs the smoke-gate slow lock, as the database fixtures do.
"""
import json
import secrets
import sys
import urllib.error
import urllib.request
import unittest
from pathlib import Path

HERE=Path(__file__).resolve().parent
sys.path[:0]=[str(HERE),str(HERE.parent/'db'),str(HERE.parents[2]/'src/clients/python'),str(HERE.parents[2]/'src/agents/shared')]
import fixtures
from rules import *
from core import Core
from test_rag import CFG,KV,PROMPT

QDRANT='docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2'   # docs/agents/qdrant.md pins it
KEY=secrets.token_hex(16)   # this run's own API key; never a host credential
DIM=4


class Wire:
    """The Blocks adapter's qdrant() over plain HTTP to the throwaway server; providers are deterministic stand-ins."""
    def __init__(self,port,collection):self.port=port;self.collection=collection
    def begin(self,budget):pass
    def http(self,verb,path,body=None):
        request=urllib.request.Request(f'http://127.0.0.1:{self.port}{path}',method=verb,headers={'api-key':KEY,'content-type':'application/json'},
                                       data=None if body is None else json.dumps(body).encode())
        try:
            with urllib.request.urlopen(request,timeout=20) as answer:return json.loads(answer.read())
        except urllib.error.HTTPError as error:
            raise RagError(f'Qdrant said {error.code}: {error.read().decode()[:200]}') from None
    def qdrant(self,verb,path,body=None,write=False):
        return self.http(verb,'/collections/'+self.collection+path,body)['result']
    def decide(self,request):return {'answers':{'labels':{'type':'choice','probabilities':{'drink':.8,'other':.2}}}}
    def embed(self,request):return {'data':[{'index':i,'embedding':[.5]*DIM} for i in range(len(request['input']))]}


class RealKeyword(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        def ready(name,port):
            request=urllib.request.Request(f'http://127.0.0.1:{port}/',headers={'api-key':KEY})
            try:
                with urllib.request.urlopen(request,timeout=3) as answer:return answer.status==200
            except (OSError,urllib.error.URLError):return False
        _,cls.port=fixtures.bring_up('Qdrant',lambda:fixtures.container(QDRANT,6333,{'QDRANT__SERVICE__API_KEY':KEY}),ready,lambda n,p:None,seconds=120)
    def setUp(self):
        self.collection='rag_'+secrets.token_hex(4)
        self.wire=Wire(self.port,self.collection)
        self.wire.http('PUT','/collections/'+self.collection,{'vectors':{'size':DIM,'distance':'Cosine'},
            'sparse_vectors':{'text':{'modifier':'idf'}},'metadata':{'embedding_model':'voyage-3.5'}})
        for field in INDEXES:
            self.wire.http('PUT',f'/collections/{self.collection}/index?wait=true',{'field_name':field,'field_schema':'keyword'})
        self.app=Core({**CFG,'collection':self.collection,'dimensions':DIM},self.wire,KV())
        self.app.cache.set('team','notes',PROMPT)
        self.addCleanup(self.wire.http,'DELETE','/collections/'+self.collection)
    def put(self,id,data):return self.app.run(parse(dumps({'method':'upsert','namespace':'team','tableName':'notes','id':id,'data':data})))
    def find(self,query,**more):
        try:return self.app.run(parse(dumps({'method':'search','namespace':'team','query':query,**more})))
        except RagError as error:self.fail(f'the real server refused search {query!r}: {error}')
    def test_ingest_search_get_and_delete_against_the_real_server(self):
        self.put('tea','Tea is a drink made from leaves.')
        self.put('saffron','Saffron is a rare spice.')
        self.put('cats','Cats sleep all day. Tea is not for cats.')
        found=self.find('tea')
        self.assertTrue(self.app.bm25)   # verify() saw the sparse vector with the idf modifier
        self.assertEqual({r['id'] for r in found['records']},{'tea','cats'})
        self.assertEqual(found['records'][0]['id'],'tea')   # the shorter text weighs the term more
        self.assertTrue(all(r['score']>0 and r['text'] for r in found['records']))
        # idf: a rare term outweighs a common one in the same query
        both=self.find('tea saffron')
        self.assertEqual(both['records'][0]['id'],'saffron')
        self.assertEqual([r['id'] for r in self.find('tea',key1='nothing')['records']],[])
        self.assertEqual(self.find('zebra')['records'],[])
        # the dense side still reads back: a collection with a sparse vector answers vectors by name
        got=self.app.run(parse(dumps({'method':'get','namespace':'team','tableName':'notes','id':'tea'})))
        self.assertEqual(got['embeddings'],[[.5]*DIM])
        self.assertEqual(got['data'],'Tea is a drink made from leaves.')
        # an upsert that changes the text replaces the terms; a delete removes the record from search
        self.put('tea','Coffee is a drink.')
        self.assertEqual({r['id'] for r in self.find('tea')['records']},{'cats'})
        self.assertEqual([r['id'] for r in self.find('coffee')['records']],['tea'])
        self.app.run(parse(dumps({'method':'delete','namespace':'team','tableName':'notes','id':'tea'})))
        self.assertEqual(self.find('coffee')['records'],[])


if __name__=='__main__':unittest.main()
