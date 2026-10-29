"""Scoped curl and allowlist contracts; every server is a disposable loopback fixture."""
import base64
import contextlib
import http.server
import io
import json
from pathlib import Path
import socket
import ssl
import subprocess
import os
import tempfile
import time
import datetime
from socketserver import TCPServer
import sys
import threading
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2];sys.path.insert(0,str(HERE))
from policy import Policy,Refused,curl_fields,BODY_CURL,resolve
from transport import curl,get,Uncertain,Reply,ContentError
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
        for raw in ('https://bücher.example/v1/other?x=a','https://bücher.example:8443/v1/item?x=a','http://bücher.example/v1/item?x=a','http://bücher.example:443/v1/item?x=a','https://bücher.example/v1/item?x=b'):
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
        for raw in ('https://example.org/item1?x=a','https://example.org/item[1]?x=a','https://example.org/item[1]ax=[a]'):
            with self.assertRaises(Refused):policy.authorize(raw)
    def test_ambiguous_numeric_and_mapped_hosts_never_inherit_an_exception(self):
        policy=Policy({'allow':['127.0.0.1','*']})
        for host in ('2130706433','0177.0.0.1','127.1','[::ffff:127.0.0.1]','[::127.0.0.1]'):
            with self.subTest(host=host),self.assertRaises(Refused):policy.authorize('http://'+host+'/')
    def test_exact_dns_trusts_all_private_but_refuses_mixed_answers(self):
        target,private=Policy({'allow':['api.example.org']}).authorize('https://api.example.org/')
        self.assertTrue(private)
        self.assertEqual(resolve(target,lambda *a,**k:answers('127.0.0.1'),private=private),[(socket.AF_INET,'127.0.0.1')])
        with self.assertRaises(Refused):resolve(target,lambda *a,**k:answers('8.8.8.8','127.0.0.1'),private=private)
        target,private=Policy({'allow':['0.1.2.3']}).authorize('http://0.1.2.3/')
        with self.assertRaises(Refused):resolve(target,private=private)
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
        for headers in ({'Host':'evil'},{'hOsT':'evil'},{'Connection':'close'},{'TE':'trailers'},{'Trailer':'X'},{'Keep-Alive':'x'},{'Proxy-Authorization':'private'},{'Proxy-Connection':'keep-alive'},{'Transfer-Encoding':'chunked'},{'Content-Length':'1'},{'Expect':'100-continue'},{'Sec-WebSocket-Key':'x'},{'X':'a\r\nb'},{'X':'\x00'},{'X':'x','x':'y'},{'bad name':'x'},{'X':'é'}):
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
    def test_binary_with_near_maximum_headers_still_fits_answer(self):
        reply=Reply(200,[('X-A','ÿ'*7000),('X-B','\t'*8000)],b'\xff'*BODY_CURL,'https://example.org/','application/octet-stream')
        output=worker.raw_answer(reply);self.assertLessEqual(len(output.encode()),OUTPUT_MAX)
        value=json.loads(output[len(WARNING):]);self.assertEqual(base64.b64decode(value['body']),reply.data);self.assertEqual(value['headers'][0][1],'ÿ'*7000)
    def test_helper_preflight_and_popen_failures_are_definitely_not_sent(self):
        with self.assertRaises(fetch.NotSent):fetch.run({'url':'x'*(2*1024*1024),'format':'text'},1)
        with patch('fetch.subprocess.Popen',side_effect=OSError()):
            with self.assertRaises(fetch.NotSent):fetch.run({'url':'https://example.org/','format':'text'},1)
        def not_sent(*args):raise fetch.NotSent()
        with contextlib.redirect_stderr(io.StringIO()):
            result=fetch.answer({'body':'curl POST https://example.org/ x'},{},execute=not_sent)
        self.assertTrue(result.startswith('error:'));self.assertIn('not sent',result)
    def test_empty_allow_is_sent_to_worker_and_caller_cannot_override_scope(self):
        seen=[]
        def execute(request,timeout):seen.append(request);return {'body':fetch.FAILURE,'class':'allowlist','bytes':0}
        with contextlib.redirect_stderr(io.StringIO()):
            fetch.answer({'body':'curl GET https://example.org/'},{'allow':[]},execute=execute)
            self.assertIn('allow',seen[0]);self.assertEqual(seen[0]['allow'],[])
            seen.clear();fetch.answer({'body':'{"method":"curl","verb":"GET","url":"https://example.org/","allow":["*"]}'},{'allow':[]},execute=execute)
        self.assertEqual(seen,[])
    def test_worker_preserves_unknown_outcome_class(self):
        request=fetch.parse('curl POST https://example.org/ x');request['timeout']=1
        with patch.object(worker,'curl',side_effect=Uncertain('request was sent; response unavailable')):
            result=worker.perform(request)
        self.assertEqual(result['class'],'unknown');self.assertTrue(result['body'].startswith('unknown:'))
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
        response=self.server.response(event)
        if response is None:self.close_connection=True;return
        status,headers,data=response
        try:
            self.send_response(status)
            for name,value in headers:self.send_header(name,value)
            if not getattr(self.server,'close_framed',False) and not any(k.lower() in ('content-length','transfer-encoding') for k,v in headers):self.send_header('Content-Length',str(sum(len(piece) for piece in data) if isinstance(data,list) else len(data)))
            self.send_header('Connection','close');self.end_headers()
            if self.command!='HEAD':
                if isinstance(data,list):
                    for piece in data:self.wfile.write(piece);self.wfile.flush();time.sleep(.05)
                else:self.wfile.write(data)
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
        try:reply=self.call()
        except Refused:self.fail('an exact private grant was refused before the fixture')
        self.assertEqual(reply.status,201);self.assertEqual(reply.data,b'raw\x00\xffbody')
        self.assertEqual([v for k,v in reply.headers if k=='Set-Cookie'],['a=1','b=2'])
        self.assertEqual(len(self.server.calls),1);event=self.server.calls[0]
        self.assertEqual((event['verb'],event['path'],event['body']),('POST','/items',b' { "x": 1 }\n'));self.assertEqual(event['headers'].get('X-Test'),'exact')
    def test_wildcard_and_no_list_never_reach_same_private_fixture(self):
        for policy in (Policy({}),Policy({'allow':['*']}),Policy({'allow':['127.*']}),Policy({'allow':[]}),Policy({'allow':[self.url+'*']})):
            with self.subTest(policy=policy.allow),self.assertRaises(Refused):curl(self.url,verb='GET',headers={},body=b'',policy=policy)
        self.assertEqual(self.server.calls,[])
        def no_connect(*args):self.fail('public-only wildcard connected to private address')
        with self.assertRaises(Refused):curl('http://127.0.0.1/',verb='GET',headers={},body=b'',policy=Policy({'allow':['127.*']}),connector=no_connect)
    def test_definite_first_connect_failure_is_not_unknown(self):
        with socket.socket() as closed:
            closed.bind(('127.0.0.1',0));port=closed.getsockname()[1]
            try:curl('http://127.0.0.1:'+str(port)+'/',verb='POST',headers={},body=b'x',policy=self.policy)
            except (Refused,Uncertain) as error:self.assertIsInstance(error,Refused)
            else:self.fail('unlistening reserved port accepted the request')
        self.assertEqual(self.server.calls,[])
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
        try:self.call()
        except (Uncertain,ContentError) as caught:
            self.assertIsInstance(caught,Uncertain);self.assertIn('request was sent',str(caught));self.assertIn('answer exceeds limits',str(caught))
        else:self.fail('oversized raw response was accepted')
        self.assertEqual(len(self.server.calls),1)
    def test_actual_close_framed_and_chunked_curl_reply_caps(self):
        self.server.close_framed=True
        for headers,data in (([],b'x'*(BODY_CURL+1)),([('Transfer-Encoding','chunked')],b'40001\r\n'+b'x'*(BODY_CURL+1)+b'\r\n0\r\n\r\n')):
            self.server.calls.clear();self.server.response=lambda event:(200,headers,data)
            try:self.call()
            except (Uncertain,ContentError) as error:self.assertIsInstance(error,Uncertain)
            else:self.fail('oversized raw reply was silently accepted or truncated')
            self.assertEqual(len(self.server.calls),1)
        self.server.response=lambda event:(200,[('Transfer-Encoding','chunked')],b'3\r\nraw\r\n0\r\n\r\n')
        self.assertEqual(self.call().data,b'raw')
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
    def test_same_host_port_change_drops_caller_credentials(self):
        other=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);other.daemon_threads=True;other.calls=[];other.response=lambda event:(200,[],b'ok')
        thread=threading.Thread(target=other.serve_forever,kwargs={'poll_interval':.01});thread.start()
        try:
            self.server.response=lambda event:(302,[('Location','http://127.0.0.1:'+str(other.server_port)+'/done')],b'')
            curl(self.url,verb='GET',headers={'Authorization':'Bearer fixture','X-Key':'fixture'},body=b'',policy=self.policy)
            self.assertEqual(len(other.calls),1);self.assertFalse(set(other.calls[0]['headers'])&{'Authorization','X-Key'})
        finally:other.shutdown();other.server_close();thread.join(2)
    def test_put_delete_and_custom_301_302_stay_same_origin(self):
        for status,verb in ((301,'PUT'),(302,'DELETE'),(302,'CUSTOM')):
            self.server.calls.clear();self.server.response=lambda event:(status,[('Location','/done')],b'') if event['path']=='/items' else (200,[],b'ok')
            curl(self.url,verb=verb,headers={},body=b'kept',policy=self.policy)
            self.assertEqual([e['verb'] for e in self.server.calls],[verb,verb]);self.assertEqual([e['body'] for e in self.server.calls],[b'kept',b'kept'])
    def test_cross_origin_preserved_body_or_verb_is_not_replayed(self):
        port=self.server.server_port
        self.server.response=lambda event:(307,[('Location',f'http://two.example:{port}/done')],b'')
        with self.assertRaises(Uncertain):curl(f'http://one.example:{port}/items',verb='POST',headers={},body=b'private',policy=Policy({'allow':['one.example','two.example']}),resolver=lambda *a,**k:answers('127.0.0.1'))
        self.assertEqual(len(self.server.calls),1)
    def test_dropped_post_reply_and_deadline_after_write_are_uncertain_once(self):
        self.server.response=lambda event:None
        with self.assertRaises(Uncertain):self.call()
        self.assertEqual(len(self.server.calls),1)
        self.server.calls.clear();self.server.response=lambda event:(200,[],[b'x']*100)
        stamp=(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=1.5)).isoformat()
        with contextlib.redirect_stderr(io.StringIO()):
            result=fetch.answer({'body':'curl POST '+self.url+' payload','deadline':stamp},{'allow':['127.0.0.1']})
        self.assertTrue(result.startswith('unknown:'));self.assertEqual(len(self.server.calls),1)
    def test_real_exported_entrypoint_policy_body_and_credential_boundaries(self):
        (ROOT/'tmp').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=ROOT/'tmp',prefix='curl-export-') as directory:
            dest=Path(directory)/'fetch';subprocess.run([str(HERE/'package.sh'),str(dest)],check=True,capture_output=True)
            secret='fixture-caller-authorization';host_secret='fixture-host-token-never-in-helper'
            self.server.response=lambda event:(202,[('X-Upstream','yes')],event['body'])
            config=json.dumps({'allow':['127.0.0.1']})
            env={'PATH':os.environ['PATH'],'AGENT_BUS_CONFIG':config,'AGENT_BUS_TOKEN':host_secret,'AB_ROLE_OWNER':'1'}
            body=json.dumps({'method':'curl','verb':'POST','url':self.url,'headers':{'Authorization':'Bearer '+secret},'body':' { "x": 1 }\n'})
            process=subprocess.run([str(dest/'fetch.py')],input=json.dumps({'body':body}),env=env,capture_output=True,text=True,timeout=3)
            self.assertEqual(process.returncode,0);self.assertTrue(process.stdout.startswith(WARNING),process.stdout)
            value=json.loads(process.stdout[len(WARNING):]);self.assertEqual(value['body'],' { "x": 1 }\n');self.assertEqual(value['status'],202)
            self.assertEqual(len(self.server.calls),1);self.assertEqual(self.server.calls[0]['headers']['Authorization'],'Bearer '+secret)
            self.assertNotIn(secret,process.stderr);self.assertNotIn(host_secret,process.stdout+process.stderr);self.assertNotIn(self.url,process.stderr)
            for config in ({},{'allow':[]}):
                result=subprocess.run([str(dest/'fetch.py')],input=json.dumps({'body':'curl POST '+self.url+' payload'}),env={**env,'AGENT_BUS_CONFIG':json.dumps(config)},capture_output=True,text=True,timeout=3)
                self.assertEqual(result.stdout.strip(),fetch.FAILURE)
            self.assertEqual(len(self.server.calls),1)
            result=subprocess.run([str(dest/'fetch.py')],input=json.dumps({'ask':'future','body':body}),env={**env,'AGENT_BUS_CONFIG':'invalid'},capture_output=True,text=True,timeout=3)
            self.assertEqual(result.stdout.strip(),'unsupported: future');self.assertEqual(len(self.server.calls),1)
    def test_private_https_requires_normal_ca_and_hostname_verification(self):
        self.server.shutdown();self.thread.join(2);(ROOT/'tmp').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=ROOT/'tmp',prefix='curl-tls-') as directory:
            key=Path(directory)/'key';cert=Path(directory)/'cert'
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',str(key),'-out',str(cert),'-days','1','-subj','/CN=localhost','-addext','subjectAltName=DNS:localhost,IP:127.0.0.1'],check=True,capture_output=True)
            context=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER);context.load_cert_chain(cert,key)
            self.server.socket=context.wrap_socket(self.server.socket,server_side=True)
            self.thread=threading.Thread(target=self.server.serve_forever,kwargs={'poll_interval':.01});self.thread.start()
            target=self.url.replace('http:','https:')
            with self.assertRaises(Refused):curl(target,verb='GET',headers={},body=b'',policy=self.policy)
            self.assertEqual(self.server.calls,[])
            reply=curl(target,verb='GET',headers={},body=b'',policy=self.policy,context=ssl.create_default_context(cafile=str(cert)))
            self.assertEqual(reply.status,201);self.assertEqual(len(self.server.calls),1)
    def test_head_body_and_trace_extension_methods_are_supported(self):
        self.server.response=lambda event:(200,[('Content-Length','614400')],b'')
        reply=curl(self.url,verb='HEAD',headers={},body=b'head-data',policy=self.policy)
        self.assertEqual(reply.data,b'');self.assertEqual(self.server.calls[-1]['body'],b'head-data')
        self.server.response=lambda event:(200,[],b'ok')
        for verb in ('TRACE','M-SEARCH','CuStOm'):
            reply=curl(self.url,verb=verb,headers={},body=b'',policy=self.policy);self.assertEqual(reply.data,b'ok');self.assertEqual(self.server.calls[-1]['verb'],verb)


if __name__=='__main__':unittest.main()
