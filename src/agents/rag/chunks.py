"""Bounded, deterministic UTF-8 extraction; no network or binary document loader."""
from html.parser import HTMLParser
from rules import CHUNK_BYTES,CHUNK_MAX,DATA_MAX,RagError,dumps,loads,text


class HTML(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.depth=0; self.hidden=[]; self.output=[]
    def handle_starttag(self,tag,attrs):
        if tag not in {'area','base','br','col','embed','hr','img','input','link','meta','param','source','track','wbr'}: self.depth+=1
        if self.depth>64: raise RagError('HTML nesting exceeds 64')
        if tag in ('script','style'): self.hidden.append(tag)
        elif not self.hidden: self.output.append('\n')
    def handle_startendtag(self,tag,attrs):
        pass
    def handle_endtag(self,tag):
        self.depth=max(0,self.depth-1)
        if self.hidden and tag==self.hidden[-1]: self.hidden.pop()
        elif not self.hidden: self.output.append('\n')
    def handle_data(self,data):
        if not self.hidden: self.output.append(data)


def extract(record):
    source=record['data']; mime=record['mime-type']
    if mime=='text/html':
        parser=HTML(); parser.feed(source); parser.close(); source=''.join(parser.output)
    elif mime=='application/json': source=dumps(loads(source,DATA_MAX))
    text(source,'extracted text',DATA_MAX,empty=True)
    if not source.strip(): raise RagError('document has no extractable text')
    chunks=[]; current=[]; size=0
    for char in source:
        n=len(char.encode())
        if size+n>CHUNK_BYTES:
            chunks.append(''.join(current));current=[];size=0
        current.append(char);size+=n
    if current: chunks.append(''.join(current))
    if len(chunks)>CHUNK_MAX: raise RagError('document exceeds 8 chunks')
    return source,chunks
