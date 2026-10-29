"""Public HTTP destinations, checked once and then used as numeric addresses."""
from dataclasses import dataclass, replace
import ipaddress
import base64
import fnmatch
import re
import socket
from urllib.parse import urlsplit, urlunsplit

PORTS = frozenset((80, 443, 8080, 8443))
V4_DENY = tuple(ipaddress.ip_network(x) for x in (
    '0.0.0.0/8','10.0.0.0/8','100.64.0.0/10','127.0.0.0/8','169.254.0.0/16',
    '172.16.0.0/12','192.0.0.0/24','192.0.2.0/24','192.88.99.0/24',
    '192.168.0.0/16','198.18.0.0/15','198.51.100.0/24','203.0.113.0/24',
    '224.0.0.0/4','240.0.0.0/4'))
V6_DENY = tuple(ipaddress.ip_network(x) for x in (
    '::/96','::ffff:0:0/96','64:ff9b::/96','64:ff9b:1::/48','100::/64',
    '::ffff:0:0:0/96','2001::/32','2001:2::/48','2001:10::/28','2001:20::/28',
    '2001:db8::/32','2002::/16','3fff::/20','5f00::/16','fc00::/7','fe80::/10','fec0::/10','ff00::/8'))
LABEL = re.compile(r'[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?')


class Refused(ValueError):
    pass


@dataclass(frozen=True)
class URL:
    value: str
    scheme: str
    host: str
    port: int
    authority: str
    target: str
    literal: object = None


def public_address(value):
    try: ip = ipaddress.ip_address(value)
    except ValueError: raise Refused('address') from None
    denied = V4_DENY if ip.version == 4 else V6_DENY
    # ISATAP's interface identifier embeds IPv4 even under an ordinary prefix.
    isatap = ip.version == 6 and ((int(ip) >> 32) & 0xffffffff) in (0x5efe, 0x2005efe)
    if not ip.is_global or ip.is_multicast or ip.is_reserved or isatap or any(ip in net for net in denied):
        raise Refused('address')
    return ip


def url(value, *, redirect=False, exact=False):
    try:
        if not isinstance(value, str) or not value or len(value.encode()) > 4096 or any(ord(c) <= 32 or ord(c) == 127 for c in value) or '\\' in value:
            raise Refused('url')
        u = urlsplit(value)
        if u.netloc.endswith(':'): raise Refused('url')
        if u.scheme not in ('http','https') or not u.netloc or u.username is not None or u.password is not None or (u.fragment and not redirect):
            raise Refused('url')
        host = u.hostname
        if not host or '%' in host: raise Refused('url')
        literal = None
        try: literal = ipaddress.ip_address(host)
        except ValueError:
            # inet_aton/getaddrinfo accept shortened, octal, hex and integer IPv4.
            if re.fullmatch(r'(?:[0-9]+|0[xX][0-9a-fA-F]+)', host.rstrip('.').rsplit('.',1)[-1]) or ':' in host: raise Refused('url')
            host = host.encode('idna').decode('ascii').lower().rstrip('.')
            if len(host) > 253 or (not exact and (host == 'localhost' or host.endswith('.localhost') or host.endswith('.local'))) or any(not LABEL.fullmatch(label) for label in host.split('.')):
                raise Refused('url')
        else:
            if literal.version == 4 and str(literal) != host: raise Refused('url')
            if literal.version == 6 and (not u.netloc.startswith('[') or literal.ipv4_mapped is not None or literal in ipaddress.ip_network('::/96') and int(literal)>1): raise Refused('url')
            host = str(literal)
        port = u.port if u.port is not None else (443 if u.scheme == 'https' else 80)
        if not 1 <= port <= 65535 or not exact and port not in PORTS: raise Refused('port')
        authority = ('['+host+']') if literal is not None and literal.version == 6 else host
        if port != (443 if u.scheme == 'https' else 80): authority += ':'+str(port)
        target = u.path or '/'
        if not target.startswith('/') or any(ord(c) >= 127 for c in target+u.query): raise Refused('url')
        if u.query: target += '?'+u.query
        canonical = urlunsplit((u.scheme,authority,u.path or '/',u.query,''))
        return URL(canonical,u.scheme,host,port,authority,target,literal)
    except (ValueError, UnicodeError, TypeError):
        raise Refused('url') from None


def resolve(target, resolver=socket.getaddrinfo, *, private=False):
    if target.literal is not None:
        ip = unicast_address(target.literal) if private else public_address(target.literal)
        return [(socket.AF_INET if ip.version == 4 else socket.AF_INET6, str(ip))]
    answers = resolver(target.host,target.port,type=socket.SOCK_STREAM,proto=socket.IPPROTO_TCP)
    if not answers or len(answers) > 16: raise Refused('dns')
    checked = []
    for family,kind,proto,_,address in answers:
        if family not in (socket.AF_INET,socket.AF_INET6) or kind != socket.SOCK_STREAM or proto != socket.IPPROTO_TCP or family == socket.AF_INET6 and (len(address) != 4 or address[3] != 0):
            raise Refused('dns')
        ip = unicast_address(address[0]) if private else public_address(address[0])
        if (family == socket.AF_INET) != (ip.version == 4): raise Refused('dns')
        item=(family,str(ip))
        if item not in checked: checked.append(item)
    if private:
        kinds=set()
        for _,numeric in checked:
            try:public_address(numeric);kinds.add('public')
            except Refused:kinds.add('private')
        if len(kinds)>1:raise Refused('address')
    return checked


BODY_CURL=256*1024
TOKEN=re.compile(r"[!#$%&'*+.^_`|~0-9A-Za-z-]+")
FORBIDDEN_HEADERS=frozenset(('host','connection','keep-alive','proxy-authenticate','proxy-authorization','te','trailer','transfer-encoding','upgrade','content-length','expect'))


def unicast_address(value):
    try:ip=ipaddress.ip_address(value)
    except ValueError:raise Refused('address') from None
    if ip.is_unspecified or ip.is_multicast or ip.version==4 and ip in ipaddress.ip_network('0.0.0.0/8') or ip==ipaddress.ip_address('255.255.255.255'):
        raise Refused('address')
    return ip


def method(value):
    if not isinstance(value,str) or not 1<=len(value)<=32 or not TOKEN.fullmatch(value) or value.upper()=='CONNECT':
        raise ValueError('invalid HTTP method')
    return value


def request_headers(value):
    if not isinstance(value,dict) or len(value)>64:raise ValueError('invalid request headers')
    total=0;seen=set();result={}
    for name,text in value.items():
        if not isinstance(name,str) or not TOKEN.fullmatch(name) or name.lower() in seen or name.lower() in FORBIDDEN_HEADERS or name.lower().startswith(('sec-websocket-','proxy-')):
            raise ValueError('invalid request header')
        if not isinstance(text,str) or len(text)>8192 or any((ord(c)<32 and c!='\t') or ord(c)>=127 for c in text):
            raise ValueError('invalid request header')
        total+=len(name)+len(text)+4
        if total>16384:raise ValueError('request header limit')
        seen.add(name.lower());result[name]=text
    return result


def curl_fields(value):
    if not isinstance(value,dict) or set(value)-{'method','verb','url','headers','body','body_base64'} or value.get('method')!='curl' or not isinstance(value.get('url'),str) or not 1<=len(value['url'].encode())<=4096:
        raise ValueError('invalid curl call')
    verb=method(value.get('verb'));headers=request_headers(value.get('headers',{}))
    if 'body' in value and 'body_base64' in value:raise ValueError('choose one body encoding')
    if 'body_base64' in value:
        encoded=value['body_base64']
        if not isinstance(encoded,str) or len(encoded)>4*((BODY_CURL+2)//3):raise ValueError('request body limit')
        try:data=base64.b64decode(encoded,validate=True)
        except (ValueError,UnicodeError):raise ValueError('invalid base64') from None
        if base64.b64encode(data).decode()!=encoded:raise ValueError('noncanonical base64')
    else:
        text=value.get('body','')
        if not isinstance(text,str):raise ValueError('body must be text')
        data=text.encode('utf-8')
    if len(data)>BODY_CURL:raise ValueError('request body limit')
    return {'method':'curl','verb':verb,'url':value['url'],'headers':headers,'body_base64':base64.b64encode(data).decode()}


def glob_pattern(value):
    # fnmatch's optimized translator handles stars; quote its other operators.
    literal=''.join('[[]' if c=='[' else '[]]' if c==']' else '[?]' if c=='?' else c for c in value)
    return re.compile(fnmatch.translate(literal))


def canonical_part(value, *, path=False, pattern=False):
    if path and '//' in value:raise ValueError('ambiguous path')
    output=[];index=0
    while index<len(value):
        char=value[index]
        if char=='%':
            token=value[index+1:index+3]
            if len(token)!=2 or not re.fullmatch('[0-9a-fA-F]{2}',token):raise ValueError('invalid URL escape')
            byte=int(token,16);decoded=chr(byte)
            if byte<32 or byte==127 or decoded in '%\\' or path and decoded=='/':raise ValueError('ambiguous URL escape')
            output.append(decoded if decoded in 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~' else '%'+token.upper());index+=3
        else:output.append(char);index+=1
    result=''.join(output)
    if not path:return result
    parts=[];trailing=result.endswith(('/', '/.', '/..'))
    for segment in result.split('/'):
        if segment=='.':continue
        if segment=='..':
            if pattern and '*' in result:raise ValueError('ambiguous pattern path')
            if parts:parts.pop()
        elif segment:parts.append(segment)
    result='/'+'/'.join(parts)
    if trailing and result!='/':result+='/'
    return result


def canonical_target(target, *, pattern=False):
    parsed=urlsplit(target.value)
    path=canonical_part(parsed.path or '/',path=True,pattern=pattern)
    query=canonical_part(parsed.query,pattern=pattern)
    value=urlunsplit((target.scheme,target.authority,path,query,''))
    return replace(target,value=value,target=path+('?' +query if query else ''))


def pattern_host(value):
    if '%' in value or not value:raise ValueError('invalid allow host')
    if ':' in value:
        try:ip=ipaddress.IPv6Address(value.replace('*','0'))
        except ValueError:raise ValueError('invalid allow host') from None
        if '*' not in value:
            if ip.ipv4_mapped is not None or ip in ipaddress.ip_network('::/96') and int(ip)>1:raise ValueError('ambiguous literal')
            return str(ip)
        return value.lower()
    if '*' not in value:
        return url('http://'+value+'/',exact=True).host
    labels=[]
    for part in value.rstrip('.').split('.'):
        if '*' in part:
            if not part.isascii() or not LABEL.fullmatch(part.replace('*','a')):raise ValueError('invalid wildcard host')
            labels.append(part.lower())
        else:
            label=part.encode('idna').decode('ascii').lower()
            if not LABEL.fullmatch(label):raise ValueError('invalid allow host')
            labels.append(label)
    host='.'.join(labels)
    if len(host)>253:raise ValueError('allow host limit')
    return host


@dataclass(frozen=True)
class Entry:
    host: object
    scheme: str = ''
    port: int = 0
    target: object = None
    exact: bool = False
    def matches(self,target):
        return bool(self.host.fullmatch(target.host) and (not self.scheme or self.scheme==target.scheme and self.port==target.port and self.target.fullmatch(target.target)))


def entry(value):
    if not isinstance(value,str) or not 1<=len(value.encode())<=4096 or value.count('*')>16 or any(ord(c)<=32 or ord(c)==127 for c in value) or '\\' in value or '#' in value:raise ValueError('invalid allow entry')
    exact='*' not in value
    if '://' not in value:
        if value.startswith('[') and value.endswith(']'):host=pattern_host(value[1:-1])
        elif any(c in value for c in '/:?@[]'):raise ValueError('bare allow host has no port or path')
        else:host=pattern_host(value)
        return Entry(glob_pattern(host),exact=exact)
    match=re.fullmatch(r'(https?)://([^/?#]+)([^#]*)',value,re.IGNORECASE)
    if not match:raise ValueError('invalid allow URL')
    scheme,authority,tail=match.groups();scheme=scheme.lower()
    if any(ord(c)>=127 for c in tail):raise ValueError('encode non-ASCII URL paths')
    if any(c in authority for c in '@%'):raise ValueError('invalid allow authority')
    if authority.startswith('['):
        close=authority.find(']')
        if close<0:raise ValueError('invalid allow authority')
        host=pattern_host(authority[1:close]);port_text=authority[close+1:]
        if port_text==':' or port_text and not port_text.startswith(':'):raise ValueError('invalid allow port')
        port_text=port_text[1:] if port_text else ''
    else:
        if authority.count(':')>1:raise ValueError('invalid allow authority')
        host_text,sep,port_text=authority.partition(':');host=pattern_host(host_text)
        if sep and not port_text:raise ValueError('invalid allow port')
    if port_text and not re.fullmatch('[0-9]{1,5}',port_text):raise ValueError('invalid allow port')
    port=int(port_text) if port_text else (443 if scheme=='https' else 80)
    if not 1<=port<=65535:raise ValueError('invalid allow port')
    path,sep,query=tail.partition('?');path=canonical_part(path or '/',path=True,pattern=True);query=canonical_part(query,pattern=True)
    return Entry(glob_pattern(host),scheme,port,glob_pattern(path+('?' +query if sep and query else '')),exact)


class Policy:
    def __init__(self,config):
        if not isinstance(config,dict) or set(config)-{'allow'}:raise ValueError('invalid fetch Configuration')
        self.allow=config.get('allow')
        if 'allow' in config:
            if not isinstance(self.allow,list) or len(self.allow)>64 or sum(len(v.encode()) if isinstance(v,str) else 65537 for v in self.allow)>65536:raise ValueError('invalid allow list')
            self.entries=[entry(value) for value in self.allow]
        else:self.entries=None
    def authorize(self,raw,*,redirect=False):
        if self.entries is None:return url(raw,redirect=redirect),False
        try:
            target=canonical_target(url(raw,redirect=redirect,exact=True))
            matches=[item for item in self.entries if item.matches(target)]
            if not matches:raise Refused('allowlist')
            private=any(item.exact for item in matches)
            if not private:url(target.value)
            return target,private
        except (ValueError,UnicodeError):raise Refused('allowlist') from None
