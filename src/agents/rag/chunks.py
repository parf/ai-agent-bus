"""Bounded deterministic extraction and whitespace-aware UTF-8 chunk format v1, and the heading chunk format of a large Markdown file."""
from html.parser import HTMLParser
import hashlib
import re
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
        if len(chunks)>CHUNK_MAX:raise RagError('document needs at least '+str(len(chunks))+' chunks; limit 8')
    return source,chunks


# Markdown heading chunk format v1 for a file that arrives by bigfile reference.
# 
# A chunk's identity is its heading path, each heading told apart from same-titled
# siblings by its ordinal, plus its part number inside that section: never a running
# index over the file, so an edit in one section leaves every other chunk's identity
# and hash as they were. Headings follow CommonMark ATX rules; fenced code and YAML
# front matter never start a section.
LARGE_FORMAT='md-heading-v1'
LARGE_CHUNKS=256
ATX=re.compile(r' {0,3}(#{1,6})(?:[ \t]+(.*?))?[ \t]*$')
FENCE=re.compile(r' {0,3}(`{3,}|~{3,})(.*)$')


def _title(raw):
    # A closing run of # preceded by a space is not part of the title.
    title=re.sub(r'(?:^|[ \t]+)#+[ \t]*$','',raw or '').strip()
    return title


def sections(source):
    """Sections in file order: (path, display, first line, last line, text); path entries are (level, title, ordinal)."""
    lines=source.split('\n')
    found=[];stack=[];counts={};fence=None
    start=0;current=[];first=1
    # YAML front matter: a --- block on the first line, closed by --- or ...
    front=0
    if lines and lines[0].rstrip()=='---':
        for i in range(1,len(lines)):
            if lines[i].rstrip() in ('---','...'):front=i+1;break
    def close(end):
        body='\n'.join(lines[start:end])
        if body.strip():found.append((tuple(stack),first,end,body))
    for i,line in enumerate(lines):
        if i<front:continue
        if fence:
            m=FENCE.match(line)
            if m and m[1][0]==fence[0] and len(m[1])>=len(fence) and not m[2].strip():fence=None
            continue
        m=FENCE.match(line)
        if m and not (m[1][0]=='`' and '`' in m[2]):
            fence=m[1];continue
        m=ATX.fullmatch(line)
        if not m:continue
        close(i)
        level=len(m[1]);title=_title(m[2])
        while stack and stack[-1][0]>=level:stack.pop()
        parent=tuple(stack)
        key=(parent,title)
        counts[key]=counts.get(key,0)+1
        stack.append((level,title,counts[key]))
        start=i;first=i+1
    close(len(lines))
    # Display a sibling ordinal only where same-titled siblings exist.
    many={k for k,n in counts.items() if n>1}
    result=[]
    for path,a,b,body in found:
        display=[]
        for depth,(level,title,ordinal) in enumerate(path):
            parent=tuple(path[:depth])
            display.append(f'{title} #{ordinal}' if (parent,title) in many else title)
        result.append((path,display,a,b,body))
    return result


def _parts(body):
    """A section's text in parts of at most CHUNK_BYTES, cut at a paragraph, then a line, then a space; all bytes kept."""
    remaining=body;parts=[]
    while remaining:
        encoded=remaining.encode()
        prefix=encoded[:CHUNK_BYTES].decode('utf-8',errors='ignore')
        cut=len(prefix)
        if len(encoded)>CHUNK_BYTES:
            for separator in ('\n\n','\n',' '):
                at=prefix.rfind(separator)
                if at>=0 and len(prefix[:at+len(separator)].encode())>=CHUNK_BYTES//2:
                    cut=at+len(separator);break
        parts.append(remaining[:cut]);remaining=remaining[cut:]
    return parts


def chunks(source):
    """Every chunk of a large Markdown file: key, heading path for display, first and last line, text and its SHA-256."""
    result=[]
    for path,display,first,last,body in sections(source):
        line=first
        for number,part in enumerate(_parts(body)):
            if not part.strip():
                line+=part.count('\n');continue
            end=line+part.rstrip('\n').count('\n')
            key=dumps([[level,title,ordinal] for level,title,ordinal in path]+[number])
            result.append({'key':key,'heading':display,'lines':[line,end],'text':part,
                           'hash':hashlib.sha256(part.encode()).hexdigest()})
            line+=part.count('\n')
            if len(result)>LARGE_CHUNKS:raise RagError(f'document needs more than {LARGE_CHUNKS} chunks; limit {LARGE_CHUNKS}')
    if not result:raise RagError('document has no extractable text')
    return result


def chunk_bound(payload):
    """How many chunks a stored head may name: 256 for a large file's format, eight for every other record."""
    format_=payload.get('_rag_format')
    if format_ is None:return 8
    if format_!=LARGE_FORMAT:raise RagError('unknown chunk format')
    return LARGE_CHUNKS
