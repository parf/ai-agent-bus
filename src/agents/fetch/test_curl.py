"""Scoped curl and allowlist contracts; every server is a disposable loopback fixture."""
import base64
import contextlib
import http.server
import io
import json
from pathlib import Path
import socket
from socketserver import TCPServer
import sys
import threading
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent;sys.path.insert(0,str(HERE))
from policy import Policy,Refused,curl_fields,BODY_CURL,resolve
from transport import curl,get,Uncertain,Reply
from extract import WARNING,OUTPUT_MAX
import worker
import fetch
from test_policy import answers


class Allowlist(unittest.TestCase):
    def test_omitted_empty_and_invalid_lists_fail_closed(self):
        self.assertFalse(Policy({}).authorize('https://example.org/')[1])
        with self.assertRaises(Refused):Policy({'allow':[]}).authorize('https://example.org/')
        for config in ({'allow':None},{'allow':'*'},{'allow':['']},{'allow':[1]},{'allow':['*']*65},{'other':[]},{'allow':['https://example.org/'+('x'*4096)]},{'allow':['https://example.org/'+('x'*2000)+str(i) for i in range(64)]}):
            with self.subTest(config=str(config)[:80]),self.assertRaises(ValueError):Policy(config)
    def test_exact_host_is_explicit_all_ports_paths_private_grant(self):
        policy=Policy({'allow':['127.0.0.1','localhost','[::1]']})
        for raw in ('http://127.0.0.1:8123/any/path?x=1','https://LOCALHOST.:444/a','http://[::1]:65535/a'):
            self.assertTrue(policy.authorize(raw)[1])
        for raw in ('http://0.0.0.0/','http://[::]/','http://224.0.0.1/'):
            candidate=Policy({'allow':['0.0.0.0','[::]','224.0.0.1']})
            target,private=candidate.authorize(raw)
            with self.assertRaises(Refused):resolve(target,private=private)
    def test_exact_url_scope_and_default_port_case_idna(self):
        policy=Policy({'allow':['HTTPS://BÜCHER.example:443/v1/item?x=a']})
        target,private=policy.authorize('https://XN--BCHER-KVA.EXAMPLE./v1/item?x=%61')
        self.assertTrue(private);self.assertEqual(target.value,'https://xn--bcher-kva.example/v1/item?x=a')
        for raw in ('https://bücher.example/v1/other?x=a','https://bücher.example:8443/v1/item?x=a','http://bücher.example/v1/item?x=a','https://bücher.example/v1/item?x=b'):
            with self.subTest(raw=raw),self.assertRaises(Refused):policy.authorize(raw)
    def test_wildcards_are_anchored_and_never_private(self):
        policy=Policy({'allow':['https://*.example.org/v1/*']})
        target,private=policy.authorize('https://one.two.example.org/v1/item?q=x');self.assertFalse(private)
        for raw in ('https://example.org/v1/a','https://one.example.org.evil/v1/a','https://one.example.org/admin','https://one.example.org:8123/v1/a'):
            with self.subTest(raw=raw),self.assertRaises(Refused):policy.authorize(raw)
        with self.assertRaises(Refused):Policy({'allow':['127.*']}).authorize('http://127.0.0.1:8123/')
        self.assertFalse(Policy({'allow':['127.*']}).authorize('http://127.0.0.1/')[1])
    def test_overlap_grants_only_the_matched_exact_entry(self):
        policy=Policy({'allow':['*.example.org','https://one.example.org:8123/private']})
        self.assertTrue(policy.authorize('https://one.example.org:8123/private')[1])
        self.assertFalse(policy.authorize('https://two.example.org/')[1])
        with self.assertRaises(Refused):policy.authorize('https://two.example.org:8123/private')
    def test_path_scope_decodes_normalizes_then_sends_same_target(self):
        policy=Policy({'allow':['https://example.org/allowed/*']})
        target,_=policy.authorize('https://example.org/allowed/%61/./b?x=%62');self.assertEqual(target.target,'/allowed/a/b?x=b')
        for path in ('/allowed/../admin','/allowed/%2e%2e/admin','/allowed/%252e%252e/admin','/allowed/x%2Fy','/allowed/x%5cy','/allowed/%GG','/allowed//x','/allowed/%00'):
            with self.subTest(path=path),self.assertRaises(Refused):policy.authorize('https://example.org'+path)
    def test_only_star_is_wildcard_and_query_is_literal(self):
        policy=Policy({'allow':['https://example.org/item[1]?x=[a]']})
        self.assertTrue(policy.authorize('https://example.org/item[1]?x=[a]')[1])
        for raw in ('https://example.org/item1?x=a','https://example.org/item[1]?x=a'):
            with self.assertRaises(Refused):policy.authorize(raw)
    def test_ambiguous_numeric_and_mapped_hosts_never_inherit_an_exception(self):
        policy=Policy({'allow':['127.0.0.1','*']})
        for host in ('2130706433','0177.0.0.1','127.1','[::ffff:127.0.0.1]','[::127.0.0.1]'):
            with self.subTest(host=host),self.assertRaises(Refused):policy.authorize('http://'+host+'/')
    def test_invalid_patterns_and_literal_limits(self):
        for value in ('http*://example.org/','https://example.org:*/','https://user@example.org/','https://example.org/#x','example.org:80','http://[::1]:/','http://example.org/%2f','*'*17+'.example.org'):
            with self.subTest(value=value),self.assertRaises(ValueError):Policy({'allow':[value]})


class Fields(unittest.TestCase):
    def test_plain_and_json_request_bodies_are_unchanged(self):
        value=fetch.parse('curl POST https://example.org/api  { "x": 1 }\n')
        self.assertEqual(value['headers'],{});self.assertEqual(base64.b64decode(value['body_base64']),b' { "x": 1 }\n')
        value=fetch.parse('{"method":"curl","verb":"PATCH","url":"https://example.org/","headers":{"X-Test":"value"},"body":"é"}')
        self.assertEqual(value['verb'],'PATCH');self.assertEqual(base64.b64decode(value['body_base64']),'é'.encode())
        self.assertEqual(base64.b64decode(fetch.parse('curl GET https://example.org/')['body_base64']),b'')
    def test_request_header_smuggling_and_case_duplicates_refuse(self):
        for headers in ({'Host':'evil'},{'hOsT':'evil'},{'Connection':'close'},{'TE':'trailers'},{'Trailer':'X'},{'Keep-Alive':'x'},{'Proxy-Authorization':'private'},{'Transfer-Encoding':'chunked'},{'Content-Length':'1'},{'Expect':'100-continue'},{'Sec-WebSocket-Key':'x'},{'X':'a\r\nb'},{'X':'\x00'},{'X':'x','x':'y'},{'bad name':'x'},{'X':'é'}):
            with self.subTest(headers=headers),self.assertRaises(ValueError):curl_fields({'method':'curl','verb':'POST','url':'https://example.org/','headers':headers})
    def test_request_method_body_and_header_bounds(self):
        for value in ({'verb':'CONNECT'},{'verb':'coNnect'},{'verb':'G\nET'},{'verb':'x'*33},{'headers':{str(i):'x' for i in range(65)}},{'headers':{'X':'x'*8193}},{'headers':{'X':'x'*8192,'Y':'y'*8192}},{'body':'x'*(BODY_CURL+1)},{'body_base64':'!!!!'},{'body':'','body_base64':''},{'body_base64':'Zh=='},{'body':True},{'allow':['*']}):
            with self.subTest(value=str(value)[:90]),self.assertRaises(ValueError):curl_fields({'method':'curl','verb':'GET','url':'https://example.org/',**value})
        value=curl_fields({'method':'curl','verb':'TRACE','url':'https://example.org/','body_base64':base64.b64encode(b'\x00'*BODY_CURL).decode()});self.assertEqual(len(base64.b64decode(value['body_base64'])),BODY_CURL)
    def test_binary_and_escaped_text_answers_round_trip_with_warning_and_cap(self):
        for body in (b'\xff\x00binary',b'\x00'*BODY_CURL,b'<script>untrusted</script>',b'{ "x": 1 }\n'):
            reply=Reply(429,[('Set-Cookie','a=1'),('Set-Cookie','b=2')],body,'https://example.org/','application/octet-stream')
            raw=worker.raw_answer(reply);self.assertTrue(raw.startswith(WARNING));self.assertLessEqual(len(raw.encode()),OUTPUT_MAX)
            value=json.loads(raw[len(WARNING):]);self.assertEqual(value['status'],429);self.assertEqual(value['headers'],[['Set-Cookie','a=1'],['Set-Cookie','b=2']])
            actual=base64.b64decode(value['body']) if value['body_encoding']=='base64' else value['body'].encode()
            self.assertEqual(actual,body)
    def test_curl_helper_timeout_is_uncertain_but_preflight_is_not_sent(self):
        def timeout(*args):raise TimeoutError()
        with contextlib.redirect_stderr(io.StringIO()):
            result=fetch.answer({'body':'curl POST https://example.org/ payload'},{},execute=timeout)
            self.assertTrue(result.startswith('unknown:'));self.assertIn('do not retry',result)
            def no_run(*args):self.fail('invalid policy called helper')
            self.assertTrue(fetch.answer({'body':'curl POST https://example.org/ payload'},{'allow':None},execute=no_run).startswith('error:'))


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version='HTTP/1.1'
    def log_message(self,*args):pass
    def handle_call(self):
        body=self.rfile.read(int(self.headers.get('Content-Length','0')))
        event={'verb':self.command,'path':self.path,'headers':dict(self.headers),'body':body};self.server.calls.append(event)
        status,headers,data=self.server.response(event)
        try:
            self.send_response(status)
            for name,value in headers:self.send_header(name,value)
            if not any(k.lower()=='content-length' for k,v in headers):self.send_header('Content-Length',str(len(data)))
            self.send_header('Connection','close');self.end_headers()
            if self.command!='HEAD':self.wfile.write(data)
        except OSError:pass
        self.close_connection=True
    def __getattr__(self,name):
        if name.startswith('do_'):return self.handle_call
        raise AttributeError(name)


class CurlHTTP(unittest.TestCase):
    def setUp(self):
        self.server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);self.server.daemon_threads=True;self.server.calls=[]
        self.server.response=lambda event:(201,[('Content-Type','application/octet-stream'),('Set-Cookie','a=1'),('Set-Cookie','b=2')],b'raw\x00\xffbody')
        self.thread=threading.Thread(target=self.server.serve_forever,kwargs={'poll_interval':.01});self.thread.start()
        self.url='http://127.0.0.1:'+str(self.server.server_port)+'/items'
        self.policy=Policy({'allow':['127.0.0.1']})
    def tearDown(self):self.server.shutdown();self.server.server_close();self.thread.join(2)
    def call(self,**kwargs):return curl(self.url,**{'verb':'POST','headers':{'X-Test':'exact'},'body':b' { "x": 1 }\n','policy':self.policy,**kwargs})
    def test_real_exact_private_post_headers_body_status_and_raw_response(self):
        reply=self.call();self.assertEqual(reply.status,201);self.assertEqual(reply.data,b'raw\x00\xffbody')
        self.assertEqual([v for k,v in reply.headers if k=='Set-Cookie'],['a=1','b=2'])
        self.assertEqual(len(self.server.calls),1);event=self.server.calls[0]
        self.assertEqual((event['verb'],event['path'],event['body']),('POST','/items',b' { "x": 1 }\n'));self.assertEqual(event['headers']['X-Test'],'exact')
    def test_wildcard_and_no_list_never_reach_same_private_fixture(self):
        for policy in (Policy({}),Policy({'allow':['*']}),Policy({'allow':['127.*']}),Policy({'allow':[]})):
            with self.subTest(policy=policy.allow),self.assertRaises(Refused):curl(self.url,verb='GET',headers={},body=b'',policy=policy)
        self.assertEqual(self.server.calls,[])
        def no_connect(*args):self.fail('public-only wildcard connected to private address')
        with self.assertRaises(Refused):curl('http://127.0.0.1/',verb='GET',headers={},body=b'',policy=Policy({'allow':['127.*']}),connector=no_connect)
    def test_get_uses_same_exact_scope_but_remains_public_by_default(self):
        self.server.response=lambda event:(200,[('Content-Type','text/plain')],b'readable')
        self.assertEqual(get(self.url,policy=self.policy)[0],b'readable')
        with self.assertRaises(Refused):get(self.url)
        self.assertEqual(len(self.server.calls),1)
    def test_unlisted_same_host_redirect_never_makes_second_request(self):
        self.server.response=lambda event:(302,[('Location','/outside')],b'')
        with self.assertRaises(Uncertain):self.call(policy=Policy({'allow':[self.url]}))
        self.assertEqual(len(self.server.calls),1)
    def test_oversize_post_answer_is_sent_unknown_exactly_once(self):
        self.server.response=lambda event:(200,[],b'x'*(600*1024))
        with self.assertRaises(Uncertain) as caught:self.call()
        self.assertIn('request was sent',str(caught.exception));self.assertIn('answer exceeds limits',str(caught.exception));self.assertEqual(len(self.server.calls),1)
    def test_non2xx_and_compressed_body_are_returned_without_extraction(self):
        self.server.response=lambda event:(429,[('Content-Encoding','gzip')],b'\x1f\x8b\x00binary')
        reply=self.call();self.assertEqual(reply.status,429);self.assertEqual(reply.data,b'\x1f\x8b\x00binary')
    def test_same_origin_307_preserves_method_body_and_headers(self):
        self.server.response=lambda event:(307,[('Location','/done')],b'') if event['path']=='/items' else (200,[],b'ok')
        reply=self.call();self.assertEqual(reply.data,b'ok');self.assertEqual(len(self.server.calls),2)
        self.assertEqual([e['verb'] for e in self.server.calls],['POST','POST']);self.assertEqual([e['body'] for e in self.server.calls],[b' { "x": 1 }\n']*2)
        self.assertTrue(all(e['headers']['X-Test']=='exact' for e in self.server.calls))
    def test_post_303_becomes_get_and_removes_body_content_headers(self):
        self.server.response=lambda event:(303,[('Location','/done')],b'') if event['path']=='/items' else (200,[],b'ok')
        curl(self.url,verb='POST',headers={'Content-Type':'application/json'},body=b'private',policy=self.policy)
        last=self.server.calls[-1];self.assertEqual((last['verb'],last['body']),('GET',b''));self.assertNotIn('Content-Type',last['headers'])
    def test_origin_change_strips_every_caller_header(self):
        port=self.server.server_port
        self.server.response=lambda event:(302,[('Location',f'http://two.example:{port}/done')],b'') if event['path']=='/items' else (200,[],b'ok')
        policy=Policy({'allow':['one.example','two.example']})
        def resolver(*args,**kwargs):return answers('127.0.0.1')
        curl(f'http://one.example:{port}/items',verb='GET',headers={'Authorization':'Bearer fixture-private','Cookie':'private','X-API-Key':'fixture-private'},body=b'',policy=policy,resolver=resolver)
        self.assertEqual(len(self.server.calls),2);last=self.server.calls[-1]
        self.assertEqual(last['headers']['Host'],f'two.example:{port}')
        self.assertFalse(set(last['headers'])&{'Authorization','Cookie','X-API-Key'})
    def test_cross_origin_preserved_body_or_verb_is_not_replayed(self):
        port=self.server.server_port
        self.server.response=lambda event:(307,[('Location',f'http://two.example:{port}/done')],b'')
        with self.assertRaises(Uncertain):curl(f'http://one.example:{port}/items',verb='POST',headers={},body=b'private',policy=Policy({'allow':['one.example','two.example']}),resolver=lambda *a,**k:answers('127.0.0.1'))
        self.assertEqual(len(self.server.calls),1)
    def test_head_body_and_trace_extension_methods_are_supported(self):
        self.server.response=lambda event:(200,[('Content-Length','614400')],b'')
        reply=curl(self.url,verb='HEAD',headers={},body=b'head-data',policy=self.policy)
        self.assertEqual(reply.data,b'');self.assertEqual(self.server.calls[-1]['body'],b'head-data')
        self.server.response=lambda event:(200,[],b'ok')
        for verb in ('TRACE','M-SEARCH','CuStOm'):
            reply=curl(self.url,verb=verb,headers={},body=b'',policy=self.policy);self.assertEqual(reply.data,b'ok');self.assertEqual(self.server.calls[-1]['verb'],verb)


if __name__=='__main__':unittest.main()
