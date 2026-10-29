"""Public HTTP destinations, checked once and then used as numeric addresses."""
from dataclasses import dataclass
import ipaddress
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


def url(value, *, redirect=False):
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
            if len(host) > 253 or host == 'localhost' or host.endswith('.localhost') or host.endswith('.local') or any(not LABEL.fullmatch(label) for label in host.split('.')):
                raise Refused('url')
        else:
            if literal.version == 4 and str(literal) != host: raise Refused('url')
            if literal.version == 6 and not u.netloc.startswith('['): raise Refused('url')
            host = str(literal)
        port = u.port if u.port is not None else (443 if u.scheme == 'https' else 80)
        if port not in PORTS: raise Refused('port')
        authority = ('['+host+']') if literal is not None and literal.version == 6 else host
        if port != (443 if u.scheme == 'https' else 80): authority += ':'+str(port)
        target = u.path or '/'
        if not target.startswith('/') or any(ord(c) >= 127 for c in target+u.query): raise Refused('url')
        if u.query: target += '?'+u.query
        canonical = urlunsplit((u.scheme,authority,u.path or '/',u.query,''))
        return URL(canonical,u.scheme,host,port,authority,target,literal)
    except (ValueError, UnicodeError, TypeError):
        raise Refused('url') from None


def resolve(target, resolver=socket.getaddrinfo):
    if target.literal is not None:
        ip = public_address(target.literal)
        return [(socket.AF_INET if ip.version == 4 else socket.AF_INET6, str(ip))]
    answers = resolver(target.host,target.port,type=socket.SOCK_STREAM,proto=socket.IPPROTO_TCP)
    if not answers or len(answers) > 16: raise Refused('dns')
    checked = []
    for family,kind,proto,_,address in answers:
        if family not in (socket.AF_INET,socket.AF_INET6) or kind != socket.SOCK_STREAM or proto != socket.IPPROTO_TCP or family == socket.AF_INET6 and (len(address) != 4 or address[3] != 0):
            raise Refused('dns')
        ip = public_address(address[0])
        if (family == socket.AF_INET) != (ip.version == 4): raise Refused('dns')
        item=(family,str(ip))
        if item not in checked: checked.append(item)
    return checked
