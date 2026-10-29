"""Pinned HTTP GETs; standard HTTP framing with pre-allocation metadata bounds."""
import http.client
from dataclasses import dataclass
import socket
import ssl
import time
from urllib.parse import urljoin
from policy import Refused, resolve, url, Policy, BODY_CURL, method, request_headers

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
        line_count=64 if self.header else 20000
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


def framing(response, *, maximum=BODY_MAX, raw=False, head=False):
    headers=response.headers
    if not raw and len(headers.get_all('Content-Type',[]))>1:raise ContentError('ambiguous content type')
    if 'Upgrade' in headers: raise ContentError('upgrade refused')
    lengths=headers.get_all('Content-Length',[])
    encodings=headers.get_all('Content-Encoding',[])
    transfers=headers.get_all('Transfer-Encoding',[])
    if len(lengths)>1 or (not raw and len(encodings)>1) or len(transfers)>1 or transfers and lengths:
        raise ContentError('ambiguous framing')
    if lengths:
        value=lengths[0]
        if not value.isascii() or not value.isdigit() or len(value)>10: raise ContentError('invalid length')
        if not head and int(value)>maximum: raise ContentError('body too large')
    if not raw and encodings and encodings[0].strip().lower()!='identity': raise ContentError('encoding refused')
    if transfers and transfers[0].strip().lower()!='chunked': raise ContentError('transfer encoding refused')
    response.fp.body()


class Uncertain(Exception):
    """An HTTP request may have executed; no automatic retry is safe."""
    pass


@dataclass(frozen=True)
class Reply:
    status: int
    headers: list
    data: bytes
    final: str
    content_type: str


def origin(target):return target.scheme,target.host,target.port


def headers_for(target,caller,raw):
    values={name.lower():(name,value) for name,value in caller.items()}
    defaults={'Host':target.authority,'User-Agent':'agent-bus-fetch','Accept-Encoding':'identity','Connection':'close','Accept':'*/*' if raw else 'text/html,application/xhtml+xml,text/plain,text/markdown'}
    for name,value in defaults.items():
        if name.lower() not in values:values[name.lower()]=(name,value)
    return dict(values.values())


def request(raw, *, verb='GET', headers=None, body=None, policy=None, raw_reply=False, budget=60, resolver=socket.getaddrinfo, connector=dial, context=None):
    """One HTTP path. Socket waits are advisory; the parent is the hard deadline."""
    verb=method(verb);caller=request_headers(headers or {})
    if body is not None and (not isinstance(body,bytes) or len(body)>BODY_CURL):raise ValueError('request body limit')
    policy=policy if policy is not None else Policy({})
    target,private=policy.authorize(raw);end=time.monotonic()+budget;sent=False
    maximum=BODY_CURL if raw_reply else BODY_MAX
    for hop in range(6):
        connection=None;response=None
        try:
            addresses=resolve(target,resolver,private=private)
            remaining=end-time.monotonic()
            if remaining<=0:raise Refused('timeout')
            connection=Pinned(target,addresses[0],min(5,remaining),connector,context)
            # DNS/TCP/TLS failure here is definitely before HTTP bytes.
            connection.connect()
            sent=True
            connection.request(verb,target.target,body,headers_for(target,caller,raw_reply))
            response=connection.getresponse();framing(response,maximum=maximum,raw=raw_reply,head=verb=='HEAD')
            if response.status in REDIRECTS:
                locations=response.headers.get_all('Location',[])
                if len(locations)!=1 or len(locations[0].encode())>4096:raise ContentError('redirect refused')
                location=locations[0]
                if any(ord(c)<=32 or ord(c)==127 for c in location) or '\\' in location:raise ContentError('redirect refused')
                following,next_private=policy.authorize(urljoin(target.value,location),redirect=True)
                if target.scheme=='https' and following.scheme!='https':raise Refused('downgrade')
                if hop==5:raise ContentError('redirect limit')
                if raw_reply:
                    if response.status==303 and verb!='HEAD' or response.status in (301,302) and verb=='POST':
                        verb='GET';body=None
                        caller={k:v for k,v in caller.items() if not k.lower().startswith('content-') and k.lower()!='digest'}
                    if origin(following)!=origin(target):
                        if body or verb not in ('GET','HEAD'):raise Refused('cross-origin request replay refused')
                        caller={}
                target,private=following,next_private;continue
            if not raw_reply and not 200<=response.status<300:raise ContentError('HTTP '+str(response.status))
            data=response.read(maximum+1)
            if len(data)>maximum:raise ContentError('body too large')
            return Reply(response.status,response.getheaders(),data,target.value,response.headers.get('Content-Type',''))
        except (Refused,ContentError) as error:
            if raw_reply and sent:
                # All messages originate here, not from URL/body/HTTP exceptions.
                reason='answer exceeds limits' if 'limit' in str(error) or 'large' in str(error) else 'answer or redirect refused'
                raise Uncertain('request was sent; '+reason+'; do not retry automatically') from None
            raise
        except (OSError,ssl.SSLError,http.client.HTTPException):
            if raw_reply and sent:raise Uncertain('request was sent; response unavailable; do not retry automatically') from None
            raise Refused('connection') from None
        finally:
            if response is not None:response.close()
            if connection is not None:connection.close()
    raise ContentError('redirect limit')


def get(raw, *, budget=60, resolver=socket.getaddrinfo, connector=dial, context=None, policy=None):
    reply=request(raw,budget=budget,resolver=resolver,connector=connector,context=context,policy=policy)
    return reply.data,reply.content_type,reply.final


def curl(raw, *, verb, headers, body, policy=None, budget=60, resolver=socket.getaddrinfo, connector=dial, context=None):
    return request(raw,verb=verb,headers=headers,body=body,policy=policy,raw_reply=True,budget=budget,resolver=resolver,connector=connector,context=context)
