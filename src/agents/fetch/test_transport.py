"""Only disposable loopback servers, reached through a recording test connector."""
import http.server
from pathlib import Path
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
sys.path.insert(0,str(HERE))
from policy import Refused
from transport import get,ContentError,BODY_MAX,dial
from test_policy import answers


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_GET(self):
        self.server.calls.append((self.path,dict(self.headers)))
        data=self.server.raw(self.path) if callable(self.server.raw) else self.server.raw
        try:
            if isinstance(data,list):
                import time
                for piece in data:self.connection.sendall(piece);time.sleep(.05)
            else:self.connection.sendall(data)
        except OSError:pass
        self.close_connection=True


def reply(body=b'<p>Hello world</p>',headers=b'Content-Type: text/html\r\n',status=b'200 OK'):
    return b'HTTP/1.1 '+status+b'\r\nConnection: close\r\n'+headers+b'\r\n'+body


class Transport(unittest.TestCase):
    def setUp(self):
        self.server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);self.server.daemon_threads=True
        self.server.calls=[];self.server.raw=reply();self.connects=[];self.resolves=[]
        self.thread=threading.Thread(target=self.server.serve_forever,kwargs={'poll_interval':.01});self.thread.start()
    def tearDown(self):self.server.shutdown();self.server.server_close();self.thread.join(2)
    def resolver(self,host,port,**kwargs):self.resolves.append((host,port));return answers('8.8.8.8')
    def connect(self,address,port,timeout):
        self.connects.append((address,port));self.assertEqual(address,(socket.AF_INET,'8.8.8.8'))
        return socket.create_connection(('127.0.0.1',self.server.server_port),timeout)
    def fetch(self,raw='http://public.example/a?q=1',**extra):return get(raw,resolver=self.resolver,connector=self.connect,**extra)
    def test_public_dns_is_resolved_once_then_pinned_with_original_host(self):
        data,mime,final=self.fetch()
        self.assertEqual(data,b'<p>Hello world</p>');self.assertEqual(mime,'text/html');self.assertEqual(final,'http://public.example/a?q=1')
        self.assertEqual(self.resolves,[('public.example',80)]);self.assertEqual(self.connects,[((socket.AF_INET,'8.8.8.8'),80)])
        path,headers=self.server.calls[0];self.assertEqual(path,'/a?q=1');self.assertEqual(headers['Host'],'public.example')
        self.assertEqual(headers['Accept-Encoding'],'identity');self.assertEqual(headers['Connection'],'close')
        self.assertFalse(set(headers)&{'Authorization','Cookie','Referer','Accept-Language'})
    def test_actual_numeric_dial_never_resolves_a_second_time(self):
        calls=[]
        class Sock:
            def settimeout(self,value):pass
            def connect(self,value):calls.append(value)
            def close(self):pass
        def no_dns(*args,**kwargs):self.fail('dial resolved after validation')
        with patch('transport.socket.socket',side_effect=lambda *a:Sock()),patch('transport.socket.getaddrinfo',side_effect=no_dns):
            dial((socket.AF_INET,'8.8.8.8'),80,1)
            dial((socket.AF_INET6,'2606:4700:4700::1111'),443,1)
        self.assertEqual(calls,[('8.8.8.8',80),('2606:4700:4700::1111',443,0,0)])
    def test_forbidden_initial_answer_never_connects(self):
        for resolved in (answers('127.0.0.1'),answers('8.8.8.8','10.0.0.1'),answers('::ffff:7f00:1')):
            with self.subTest(resolved=resolved),self.assertRaises(Refused):get('http://public.example',resolver=lambda *a,**k:resolved,connector=self.connect)
        self.assertEqual(self.connects,[]);self.assertEqual(self.server.calls,[])
    def test_redirect_to_private_never_hits_private_destination(self):
        self.server.raw=reply(b'',b'Location: http://127.0.0.1/secret\r\n',b'302 Found')
        with self.assertRaises(Refused):self.fetch()
        self.assertEqual(len(self.connects),1);self.assertEqual(len(self.server.calls),1)
    def test_same_host_redirect_rechecks_all_dns_answers(self):
        self.server.raw=reply(b'',b'Location: /next\r\n',b'302 Found');count=[0]
        def rebinding(*args,**kwargs):count[0]+=1;return answers('8.8.8.8') if count[0]==1 else answers('127.0.0.1')
        with self.assertRaises(Refused):get('http://public.example',resolver=rebinding,connector=self.connect)
        self.assertEqual(count[0],2);self.assertEqual(len(self.server.calls),1);self.assertEqual(len(self.connects),1)
    def test_relative_redirects_and_hop_bound_include_same_url_loops(self):
        self.server.raw=lambda path:reply(b'',b'Location: /done#fragment\r\n',b'302 Found') if path.startswith('/a') else reply(b'done',b'Content-Type: text/plain\r\n')
        self.assertEqual(self.fetch()[0],b'done');self.assertEqual([x[0] for x in self.server.calls],['/a?q=1','/done'])
        self.server.calls.clear();self.server.raw=reply(b'',b'Location: /loop\r\n',b'302 Found')
        with self.assertRaises(ContentError):self.fetch()
        self.assertEqual(len(self.server.calls),6)
    def test_redirect_ports_and_location_ambiguity_refuse(self):
        for fields in (b'Location: http://public.example:22/\r\n',b'Location: /a\r\nLocation: /b\r\n',b'Location: http://user@public.example/a\r\n',b'',b'Location: /'+b'a'*4096+b'\r\n'):
            self.server.raw=reply(b'',fields,b'302 Found')
            with self.subTest(fields=fields[:60]),self.assertRaises((Refused,ContentError)):self.fetch()
    def test_header_and_chunk_metadata_bounds_precede_accumulation(self):
        for raw in (reply(b'',b'X-A: '+b'a'*16384+b'\r\n'),reply(b'',b'X: a\r\n'*64),reply(b'1\r\nx\r\n0\r\n'+b'X: a\r\n'*4097+b'\r\n',b'Transfer-Encoding: chunked\r\n')):
            self.server.raw=raw
            with self.subTest(bytes=len(raw)),self.assertRaises(ContentError):self.fetch()
    def test_body_declared_and_chunked_actual_caps(self):
        for raw in (reply(b'',b'Content-Length: 1048577\r\n'),reply(b'x'*(BODY_MAX+1)),reply(b'100001\r\n'+b'x'*(BODY_MAX+1)+b'\r\n0\r\n\r\n',b'Transfer-Encoding: chunked\r\n')):
            self.server.raw=raw
            with self.subTest(bytes=len(raw)),self.assertRaises(ContentError):self.fetch()
    def test_ambiguous_encoding_and_framing_refuse(self):
        for fields in (b'Content-Length: 0\r\nContent-Length: 1\r\n',b'Transfer-Encoding: chunked\r\nContent-Length: 0\r\n',b'Transfer-Encoding: gzip\r\n',b'Content-Encoding: gzip\r\n',b'Upgrade: websocket\r\n'):
            self.server.raw=reply(b'',fields)
            with self.subTest(fields=fields),self.assertRaises(ContentError):self.fetch()
        for raw in (b'HTTP/1.1 100 Continue\r\n\r\n'+reply(),b'HTTP/1.1 101 Switching Protocols\r\n\r\n',b'HTTP/2.0 200 OK\r\n\r\n'):
            self.server.raw=raw
            with self.subTest(raw=raw[:20]),self.assertRaises(Refused):self.fetch()
    def test_tls_checks_ca_hostname_sni_and_refuses_downgrade(self):
        self.server.shutdown();self.thread.join(2)
        (ROOT/'tmp').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=ROOT/'tmp',prefix='fetch-tls-') as d:
            cert=Path(d)/'cert.pem';key=Path(d)/'key.pem'
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',str(key),'-out',str(cert),'-days','1','-subj','/CN=public.example','-addext','subjectAltName=DNS:public.example'],check=True,capture_output=True)
            context=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER);context.load_cert_chain(cert,key);names=[]
            context.set_servername_callback(lambda sock,name,ctx:names.append(name))
            self.server.socket=context.wrap_socket(self.server.socket,server_side=True)
            self.thread=threading.Thread(target=self.server.serve_forever,kwargs={'poll_interval':.01});self.thread.start()
            with self.assertRaises(Refused):self.fetch('https://public.example/')
            self.assertEqual(self.server.calls,[])
            trusted=ssl.create_default_context(cafile=str(cert))
            self.assertEqual(self.fetch('https://public.example/',context=trusted)[0],b'<p>Hello world</p>')
            self.assertEqual(names[-1],'public.example');self.assertEqual(self.connects[-1][0],(socket.AF_INET,'8.8.8.8'))
            with self.assertRaises(Refused):self.fetch('https://wrong.example/',context=trusted)
            self.server.raw=reply(b'',b'Location: http://public.example/a\r\n',b'302 Found')
            with self.assertRaises(Refused):self.fetch('https://public.example/',context=trusted)


if __name__=='__main__':unittest.main()
