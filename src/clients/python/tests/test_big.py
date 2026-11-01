"""Big transfers use an isolated HTTP fake; no live records or mail."""
import hashlib
import http.server
import io
import json
import threading
import unittest
from unittest.mock import Mock, patch
from agent_bus import Bus, Refusal, InvalidArgument, BusError, UncertainOutcome

class BigClient(unittest.TestCase):
    def setUp(self):
        self.calls=[]; self.payload=b'\x00\xffbinary'; self.bad_hash=False; self.denied=False; self.unavailable=False
        owner=self
        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self,*args): pass
            def reply(self,body,status=200,**headers):
                self.send_response(status)
                for k,v in headers.items():self.send_header(k.replace('_','-'),str(v))
                self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
            def do_GET(self):
                owner.calls.append((self.command,self.path,self.headers.get('X-Agent-Bus-Token')))
                if self.path=='/status':return self.reply(b'{"you":"owner@fixture"}')
                if owner.unavailable:return self.reply(b'{"error":"disk path blocked"}',503)
                if owner.denied:return self.reply(b'{"error":"refused","kind":"acl"}',403)
                if self.path.startswith('/big/list'):
                    return self.reply(json.dumps({'keys':[{'name':'z'},{'name':'a'}]}).encode())
                self.reply(owner.payload,X_Big_Sha256=('0'*64 if owner.bad_hash else hashlib.sha256(owner.payload).hexdigest()),X_Big_Held='memory',X_Big_Expires='2026-11-02T00:00:00Z')
            def do_PUT(self):
                raw=self.rfile.read(int(self.headers['Content-Length']));owner.payload=raw
                if owner.unavailable:return self.reply(b'{"error":"disk path blocked"}',503)
                owner.calls.append((self.command,self.path,self.headers.get('X-Agent-Bus-Token')))
                digest='0'*64 if owner.bad_hash else hashlib.sha256(raw).hexdigest()
                self.reply(json.dumps({'size':len(raw),'sha256':digest,'held':'memory','replaced':True}).encode())
            def do_POST(self):
                self.rfile.read(int(self.headers['Content-Length']));owner.calls.append((self.command,self.path,None))
                if owner.unavailable:return self.reply(b'{"error":"disk path blocked"}',503)
                self.reply(b'{"deleted":true}')
        self.server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler)
        self.thread=threading.Thread(target=self.server.serve_forever,daemon=True);self.thread.start()
        self.bus=Bus(addr='http://127.0.0.1:'+str(self.server.server_port),token='fixture-token')
        self.rec=self.bus.record('owner@fixture')
        self.addCleanup(self.bus.close);self.addCleanup(self.server.server_close);self.addCleanup(self.server.shutdown)
    def big(self):
        self.assertTrue(hasattr(self.rec,'big'),'Record.big must expose the big-key client')
        return self.rec.big
    def test_binary_roundtrip_and_metadata(self):
        big=self.big(); raw=bytes(range(256))*4097
        info=big.put('a/b',io.BytesIO(raw),size=len(raw),ttl='30d',memory=True,sha256=hashlib.sha256(raw).hexdigest())
        self.assertTrue(info['replaced']);data,got=big.get('a/b')
        self.assertEqual(data,raw);self.assertEqual(got['size'],len(raw));self.assertEqual(got['sha256'],info['sha256'])
        self.assertIn('ttl=2592000',self.calls[1][1]);self.assertIn('memory=true',self.calls[1][1]);self.assertIn('name=a%2Fb',self.calls[1][1])
        self.assertTrue(all(c[2]=='fixture-token' for c in self.calls if c[0]!='POST'))
        self.assertEqual([row['name'] for row in big.list()],['a','z']);self.assertTrue(big.delete('a/b'))
    def test_empty_size_and_ttl_refused_before_any_request(self):
        big=self.big()
        for data,opts in [(b'',{}),(b'x',{'ttl':'0s'}),(b'x',{'ttl':'31d'}),(b'x',{'ttl':.5}),(io.BytesIO(b''),{'size':1}),(b'x',{'size':33554433})]:
            with self.subTest(opts=opts),self.assertRaises((InvalidArgument,ValueError)):big.put('n',data,**opts)
        self.assertEqual(self.calls,[])
    def test_hash_mismatch_and_refusal_are_not_success(self):
        big=self.big();self.bad_hash=True
        with self.assertRaisesRegex(ValueError,'SHA-256'):big.get('n')
        with self.assertRaisesRegex(ValueError,'SHA-256'):big.put('n',b'body')
        self.assertEqual(sum(c[0]=='PUT' for c in self.calls),1)
        self.denied=True
        with self.assertRaises(Refusal) as e:big.get('n')
        self.assertEqual(e.exception.status,403)
    def test_full_32_mib_download(self):
        big=self.big();self.payload=b'x'*(32*1024*1024)
        data,info=big.get('limit');self.assertEqual(len(data),32*1024*1024)
        self.assertEqual(info['sha256'],hashlib.sha256(data).hexdigest())
    def test_get_bound_is_checked_before_reading_any_body(self):
        big=self.big(); self.bus._name='owner@fixture'
        response=Mock(status=200);response.getheader.return_value='26214401'
        response.read.side_effect=AssertionError('oversized body was read before max_size refusal')
        connection=Mock();connection.getresponse.return_value=response
        with patch('agent_bus.transport.http.client.HTTPConnection',return_value=connection):
            with self.assertRaisesRegex(InvalidArgument,'max_size'):big.get('n',max_size=26214400)
        response.read.assert_not_called()
    def test_all_unavailable_calls_preserve_503_and_cause(self):
        big=self.big();self.unavailable=True
        for operation in (lambda:big.put('n',b'bytes'),lambda:big.get('n'),lambda:big.list(),lambda:big.delete('n')):
            with self.assertRaises(Refusal) as error:operation()
            self.assertEqual(error.exception.status,503);self.assertIn('disk path blocked',str(error.exception))

    def test_early_refusal_wins_over_large_upload_write_error(self):
        big=self.big();self.bus._name='owner@fixture'
        for status in (403,413,503):
            response=Mock(status=status,reason='fixture refusal')
            response.read.return_value=b'{"error":"early refusal cause"}'
            connection=Mock();connection.getresponse.return_value=response
            def failed_write(*args,**kwargs):
                next(iter(kwargs['body']))
                raise ConnectionResetError('peer stopped the large upload')
            connection.request.side_effect=failed_write
            with self.subTest(status=status),patch('agent_bus.transport.http.client.HTTPConnection',return_value=connection):
                try:big.put('n',b'x'*(3*1024*1024))
                except BusError as error:
                    self.assertIsInstance(error,Refusal,'an available response must win over the upload write error')
                    self.assertEqual(error.status,status);self.assertIn('early refusal cause',str(error))
                else:self.fail('the refused upload was reported as success')
                connection.request.assert_called_once();connection.getresponse.assert_called_once()
        connection=Mock();connection.request.side_effect=ConnectionResetError('write failed')
        connection.getresponse.side_effect=ConnectionResetError('no response received')
        with patch('agent_bus.transport.http.client.HTTPConnection',return_value=connection):
            with self.assertRaises(UncertainOutcome):big.put('n',b'x'*(3*1024*1024))
        connection.request.assert_called_once()
