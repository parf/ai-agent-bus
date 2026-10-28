"""Restart-scoped own-KV read cache. Every write invalidates, even if uncertain."""
from collections import OrderedDict
from copy import deepcopy
from rules import RagError,number,integer,text,prompt_key


def validate(v):
    if not isinstance(v,dict) or set(v)-{'labels','instructions','threshold','top_k','summary'}: raise RagError('invalid prompt fields')
    labels=v.get('labels')
    if not isinstance(labels,dict) or not 1<=len(labels)<=64: raise RagError('prompts need 1..64 labels')
    for key,value in labels.items():
        text(key,'label',128,nfc=True);text(value,'label description',512)
    text(v.get('instructions'),'instructions',4096)
    result=deepcopy(v)
    result.setdefault('threshold',.5);result.setdefault('top_k',5);result.setdefault('summary','')
    number(result['threshold'],0,1,'threshold');integer(result['top_k'],1,64,'top_k')
    text(result['summary'],'summary',4096,empty=True)
    return result


class Cache:
    def __init__(self,kv,maximum=128): self.kv=kv;self.maximum=maximum;self.values=OrderedDict()
    def get(self,namespace,table):
        key=prompt_key(namespace,table)
        if key not in self.values:
            value=self.kv.get(key)
            self.values[key]=validate(value) if value is not None else None
            while len(self.values)>self.maximum:self.values.popitem(last=False)
        self.values.move_to_end(key)
        return deepcopy(self.values[key])
    def set(self,namespace,table,value):
        key=prompt_key(namespace,table);value=validate(value)
        try:self.kv.set(key,value)
        finally:self.values.pop(key,None)
    def delete(self,namespace,table):
        key=prompt_key(namespace,table)
        try:self.kv.delete(key)
        finally:self.values.pop(key,None)
