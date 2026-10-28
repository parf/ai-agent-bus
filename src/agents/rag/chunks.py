"""Bounded deterministic extraction and whitespace-aware UTF-8 chunk format v1."""
from html.parser import HTMLParser
from rules import CHUNK_BYTES,CHUNK_MAX,DATA_MAX,RagError,dumps,loads,text

VOID={'area','base','br','col','embed','hr','img','input','link','meta','param','source','track','wbr'}
HIDDEN={'script','style','noscript','template'}
BLOCK={'p','div','li','br','hr','pre','section','article','tr','h1','h2','h3','h4','h5','h6'}
OPTIONAL={'p','li','dt','dd','option','tr','td','th'}


class HTML(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.stack=[];self.output=[]
    def hidden(self):return any(tag in HIDDEN for tag in self.stack)
    def handle_starttag(self,tag,attrs):
        # HTML permits omitted end tags for these common blocks.
        if tag in OPTIONAL and tag in self.stack:
            index=len(self.stack)-1-self.stack[::-1].index(tag);del self.stack[index:]
        if tag in BLOCK and tag!='p' and self.stack and self.stack[-1]=='p':self.stack.pop()
        if tag not in VOID:self.stack.append(tag)
        if len(self.stack)>64:raise RagError('HTML nesting exceeds 64')
        if tag in BLOCK and not self.hidden():self.output.append('\n')
    def handle_startendtag(self,tag,attrs):
        if tag in BLOCK and not self.hidden():self.output.append('\n')
    def handle_endtag(self,tag):
        hidden=self.hidden()
        if tag in self.stack:
            index=len(self.stack)-1-self.stack[::-1].index(tag);del self.stack[index:]
        if tag in BLOCK and not hidden:self.output.append('\n')
    def handle_data(self,data):
        if not self.hidden():self.output.append(data)


def extract(record):
    source=record['data'];mime=record['mime-type']
    if mime=='text/html':
        parser=HTML();parser.feed(source);parser.close();source=''.join(parser.output)
    elif mime=='application/json':source=dumps(loads(source,DATA_MAX))
    text(source,'extracted text',DATA_MAX,empty=True)
    if not source.strip():raise RagError('document has no extractable text')
    remaining=source;chunks=[]
    while remaining:
        encoded=remaining.encode()
        prefix=encoded[:CHUNK_BYTES].decode('utf-8',errors='ignore')
        cut=len(prefix)
        if len(encoded)>CHUNK_BYTES:
            # Prefer paragraph, then line, then space, only in the latter half;
            # otherwise hard-cut at a complete UTF-8 character. Keep all bytes.
            for separator in ('\n\n','\n',' '):
                at=prefix.rfind(separator)
                candidate=at+len(separator)
                if at>=0 and len(prefix[:candidate].encode())>=CHUNK_BYTES//2:
                    cut=candidate;break
        chunks.append(remaining[:cut]);remaining=remaining[cut:]
        if len(chunks)>CHUNK_MAX:raise RagError('document exceeds 8 chunks')
    return source,chunks
