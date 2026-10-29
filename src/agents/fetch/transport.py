"""Pinned HTTP GETs; standard HTTP framing with pre-allocation metadata bounds."""
import http.client
import socket
import ssl
import time
from urllib.parse import urljoin
from policy import Refused, resolve, url

BODY_MAX = 1 << 20
REDIRECTS = frozenset((301,302,303,307,308))


class ContentError(Exception):
    pass


class HeaderFile:
    def __init__(self, raw):
        self.raw=raw;self.header=True;self.total=0;self.lines=0
    def readline(self, size=-1):
        maximum=16384 if self.header else 65536
        line_max=16384 if self.header else 1024
        line_count=64 if self.header else 4096
        left=min(line_max,maximum-self.total)
        if left < 0 or self.lines >= line_count: raise ContentError('header or chunk metadata limit')
        data=self.raw.readline(min(size if size >= 0 else left+1,left+1))
        self.total+=len(data);self.lines+=1
        if data and not data.endswith(b'\r\n') or self.header and data.startswith((b' ',b'\t')) or any(c<32 and c not in (9,10,13) for c in data):raise ContentError('invalid header or chunk metadata')
        if self.total>maximum or len(data)>line_max: raise ContentError('header or chunk metadata limit')
        return data
    def body(self): self.header=False;self.total=0;self.lines=0
    def __getattr__(self, name):return getattr(self.raw,name)


class Response(http.client.HTTPResponse):
    def __init__(self,*args,**kwargs):
        super().__init__(*args,**kwargs);self.fp=HeaderFile(self.fp)
    def _read_status(self):
        version,status,reason=super()._read_status()
        if version not in ('HTTP/1.0','HTTP/1.1') or not 200 <= status <= 599:
            raise Refused('protocol')
        return version,status,reason
    def _read_and_discard_trailer(self):
        total=0
        for _ in range(65):
            line=self.fp.readline(1025);total+=len(line)
            if len(line)>1024 or total>16384:raise ContentError('trailer limit')
            if line==b'\r\n':return
            if not line.endswith(b'\r\n'):raise ContentError('invalid trailer')
        raise ContentError('trailer limit')


def dial(address, port, timeout):
    family, numeric = address
    sock=socket.socket(family,socket.SOCK_STREAM,socket.IPPROTO_TCP)
    try:
        sock.settimeout(timeout)
        # Numeric sockaddr only: never another resolver call at connect time.
        sock.connect((numeric,port) if family==socket.AF_INET else (numeric,port,0,0))
        return sock
    except BaseException:
        sock.close();raise


class Pinned(http.client.HTTPConnection):
    response_class=Response
    def __init__(self,target,address,timeout,connector=dial,context=None):
        super().__init__(target.host,target.port,timeout=timeout)
        self.target=target;self.address=address;self.connector=connector;self.context=context
    def connect(self):
        sock=self.connector(self.address,self.port,self.timeout)
        try:
            if self.target.scheme=='https':
                context=self.context if self.context is not None else ssl.create_default_context()
                sock=context.wrap_socket(sock,server_hostname=self.target.host)
            self.sock=sock
        except BaseException:
            sock.close();raise


def framing(response):
    headers=response.headers
    if len(headers.get_all('Content-Type',[]))>1:raise ContentError('ambiguous content type')
    if 'Upgrade' in headers: raise ContentError('upgrade refused')
    lengths=headers.get_all('Content-Length',[])
    encodings=headers.get_all('Content-Encoding',[])
    transfers=headers.get_all('Transfer-Encoding',[])
    if len(lengths)>1 or len(encodings)>1 or len(transfers)>1 or transfers and lengths:
        raise ContentError('ambiguous framing')
    if lengths:
        value=lengths[0]
        if not value.isascii() or not value.isdigit() or len(value)>10: raise ContentError('invalid length')
        if int(value)>BODY_MAX: raise ContentError('body too large')
    if encodings and encodings[0].strip().lower()!='identity': raise ContentError('encoding refused')
    if transfers and transfers[0].strip().lower()!='chunked': raise ContentError('transfer encoding refused')
    response.fp.body()


def get(raw, *, budget=60, resolver=socket.getaddrinfo, connector=dial, context=None):
    """Only internal fixtures inject resolver/connector/CA; never call/env fields."""
    target=url(raw);end=time.monotonic()+budget
    for hop in range(6):
        try:
            addresses=resolve(target,resolver)
            remaining=end-time.monotonic()
            if remaining<=0: raise Refused('timeout')
            connection=Pinned(target,addresses[0],min(5,remaining),connector,context)
            response=None
            try:
                connection.request('GET',target.target,headers={'Host':target.authority,'User-Agent':'agent-bus-fetch','Accept-Encoding':'identity','Connection':'close','Accept':'text/html,application/xhtml+xml,text/plain,text/markdown'})
                response=connection.getresponse();framing(response)
                if response.status in REDIRECTS:
                    locations=response.headers.get_all('Location',[])
                    if len(locations)!=1 or len(locations[0].encode())>4096: raise ContentError('redirect refused')
                    # Validate the raw Location before urljoin can erase controls.
                    location=locations[0]
                    if any(ord(c)<=32 or ord(c)==127 for c in location) or '\\' in location: raise ContentError('redirect refused')
                    following=url(urljoin(target.value,location),redirect=True)
                    if target.scheme=='https' and following.scheme!='https': raise Refused('downgrade')
                    if hop==5: raise ContentError('redirect limit')
                    target=following;continue
                if not 200<=response.status<300: raise ContentError('HTTP '+str(response.status))
                data=response.read(BODY_MAX+1)
                if len(data)>BODY_MAX: raise ContentError('body too large')
                return data,response.headers.get('Content-Type',''),target.value
            finally:
                if response is not None:response.close()
                connection.close()
        except (OSError,ssl.SSLError,http.client.HTTPException) as error:
            raise Refused('connection') from None
    raise ContentError('redirect limit')
