"""Only bus calls. Never borrow caller credentials, forward roles or use HTTP."""
import datetime
import re
import time
from agent_bus import CallReply
from agent_bus.errors import BusError,Refusal
from rules import BODY_MAX,RagError,Unknown,Refused,Deadline,dumps,loads


class Budget:
    def __init__(self,envelope=None):
        self.end=time.monotonic()+120
        value=(envelope or {}).get('deadline')
        if value and not value.startswith('0001-'):
            try: stamp=datetime.datetime.fromisoformat(value.replace('Z','+00:00')).timestamp()
            except (ValueError,TypeError):raise RagError('invalid message deadline') from None
            self.end=min(self.end,time.monotonic()+stamp-time.time())
    def left(self,cap):
        remaining=min(cap,self.end-time.monotonic()-.1)
        if remaining<=0:raise Deadline('message deadline reached')
        return remaining


class Blocks:
    def __init__(self,bus,cfg): self.bus=bus;self.cfg=cfg;self.budget=Budget()
    def begin(self,budget): self.budget=budget
    def invoke(self,target,request, *, write=False):
        body=dumps(request)
        if len(body.encode())>BODY_MAX:raise RagError('block request exceeds 1 MiB')
        try:
            message=self.bus.call(self.cfg[target],body,timeout=self.budget.left(10 if target=='qdrant' else 30),topic='rag')
            if not isinstance(message,CallReply):raise RagError(target+' did not answer')
            raw=message.message.body
            if target=='qdrant' and request.get('verb')=='GET' and '/points/' in request.get('path','') and isinstance(raw,str) and raw.startswith('error: 404 '):
                return {'status':'ok','result':None}
            if isinstance(raw,str) and re.match(r'error: 4[0-9]{2}(?: |$)',raw):raise Refused(target+' refused the request')
            if not isinstance(raw,str) or raw.startswith(('error:','unsupported:')):raise RagError(target+' refused the request')
            return loads(raw)
        except BusError:
            if write:raise Unknown('storage outcome unknown; repeating the same upsert/delete is safe') from None
            raise RagError(target+' call failed') from None
    def qdrant(self,verb,path,body=None, *, write=False):
        request={'method':'request','verb':verb,'path':'/collections/'+self.cfg['collection']+path}
        if body is not None:request['body']=dumps(body)
        result=self.invoke('qdrant',request,write=write)
        if not isinstance(result,dict) or result.get('status')!='ok' or 'result' not in result:raise RagError('invalid Qdrant response')
        if write and (not isinstance(result['result'],dict) or result['result'].get('status')!='completed'):
            raise Unknown('storage completion unknown; repeating the same upsert/delete is safe')
        return result['result']
    def decide(self,request):return self.invoke('ai',{'method':'decide','body':dumps(request)})
    def embed(self,request):return self.invoke('voyage',{'method':'request','path':'/v1/embeddings','body':dumps(request)})


class OwnKV:
    def __init__(self,bus,budget): self.bus=bus;self.budget=budget
    def get(self,key):
        try:return self.bus._request('GET','/kv',query={'record':self.bus.name,'name':key,'kind':'json'},timeout=self.budget().left(10))['value']
        except Refusal as error:
            if error.status==404:return None
            raise
    def set(self,key,value):
        self.bus._request('POST','/kv/set',{'record':self.bus.name,'name':key,'kind':'json','how':'set','value':value},timeout=self.budget().left(10))
    def delete(self,key):
        self.bus._request('POST','/kv/delete',{'record':self.bus.name,'name':key,'kind':'json'},timeout=self.budget().left(10))
