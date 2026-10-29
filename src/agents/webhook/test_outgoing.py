"""Outbound tests use only local fake HTTP/TLS and fixed test helper children."""
import contextlib
import http.server
import json
import os
from pathlib import Path
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
sys.path[:0]=[str(HERE),str(ROOT/'src/clients/python'),str(ROOT/'src/agents/shared')]
from rules import Invalid,configuration
from outgoing import post,PIPE_MAX

KEY='private-password-sentinel-849bc'


class Fake(http.server.ThreadingHTTPServer):
    daemon_threads=True
    def __init__(self):
        self.calls=[];self.code=200;self.reply=b'hello';self.trickle=False;self.delay=0;self.headers_extra={};self.drop=False
        super().__init__(('127.0.0.1',0),Handler)
        self.thread=threading.Thread(target=self.serve_forever,daemon=True);self.thread.start()
    def close(self):self.shutdown();self.server_close();self.thread.join(2)


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_POST(self):
        body=self.rfile.read(int(self.headers.get('Content-Length','0')))
        self.server.calls.append((self.path,body,dict(self.headers)))
        if self.server.drop:
            self.connection.close();return
        if self.server.delay:time.sleep(self.server.delay)
        try:
            self.send_response(self.server.code)
            for k,v in self.server.headers_extra.items():self.send_header(k,v)
            self.send_header('Content-Length',str(len(self.server.reply)));self.end_headers()
            if self.server.trickle:
                for byte in self.server.reply:
                    self.wfile.write(bytes([byte]));self.wfile.flush();time.sleep(.05)
            else:self.wfile.write(self.server.reply)
        except (BrokenPipeError,ConnectionResetError):pass


class Outgoing(unittest.TestCase):
    def setUp(self):self.server=Fake()
    def tearDown(self):self.server.close()
    def route(self,**extra):
        return configuration([{'name':'out','url':f'http://127.0.0.1:{self.server.server_port}/fixed?q=public',**extra}])[0]
    def test_body_path_auth_and_non2xx_are_exact(self):
        self.server.code=429;self.server.reply=b'{"error":"rate limited"}'
        route=self.route(basic={'user':'sender','password':'KEY'})
        body='{ "prompt": "\u043f\u0440\u0438\u0432\u0435\u0442" }'
        answer=post(route,body,{'KEY':KEY})
        self.assertEqual(answer['status'],429);self.assertEqual(json.loads(answer['body']),{'error':'rate limited'})
        self.assertEqual(len(self.server.calls),1)
        path,actual,h=self.server.calls[0];self.assertEqual(path,'/fixed?q=public');self.assertEqual(actual,body.encode())
        import base64
        self.assertEqual(h['Authorization'],'Basic '+base64.b64encode(('sender:'+KEY).encode()).decode())
        self.assertEqual(h['Content-Type'],'application/json')
    def test_redirect_is_an_answer_not_a_second_request(self):
        self.server.code=302;self.server.headers_extra={'Location':'http://127.0.0.1:1/forbidden'}
        self.assertEqual(post(self.route(),'body',{})['status'],302)
        self.assertEqual(len(self.server.calls),1)
    def test_encoded_credentials_and_header_echo_are_masked(self):
        import base64
        auth='Basic '+base64.b64encode(('sender:'+KEY).encode()).decode()
        self.server.reply=json.dumps({'key':KEY,'auth':auth}).replace('private','\\u0070rivate').encode()
        result=post(self.route(basic={'user':'sender','password':'K'}),'body',{'K':KEY})
        self.assertNotIn(KEY,json.dumps(result));self.assertNotIn(auth,json.dumps(result))
        self.assertEqual(json.loads(result['body']),{'key':'***','auth':'***'})
    def test_body_and_reply_caps_stop_before_or_after_exactly_one_post(self):
        result=post(self.route(max_bytes=3),'four',{})
        self.assertIn('error',result);self.assertEqual(len(self.server.calls),0)
        self.server.reply=b'x'*(256*1024+1)
        answer=post(self.route(),'body',{})
        self.assertIn('error',answer);self.assertEqual(len(self.server.calls),1)
    def test_trickle_and_preexpired_message_are_bounded_not_retried(self):
        import datetime
        with self.assertRaises(Invalid):post(self.route(),'body',{}, {'deadline':'2000-01-01T00:00:00Z'})
        self.assertEqual(self.server.calls,[])
        self.server.trickle=True;self.server.reply=b'x'*100
        deadline=(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=.4)).isoformat()
        start=time.monotonic()
        with self.assertRaises(Invalid):post(self.route(),'body',{}, {'deadline':deadline})
        self.assertLess(time.monotonic()-start,1.2);self.assertEqual(len(self.server.calls),1)
    def test_helper_stdout_and_stderr_are_bounded_and_process_reaped(self):
        start=time.monotonic()
        with self.assertRaises(Invalid):post(self.route(),'body',{},helper=[sys.executable,'-c',f'import sys;sys.stderr.write("x"*{PIPE_MAX+65536});sys.stderr.flush();__import__("time").sleep(30)'])
        self.assertLess(time.monotonic()-start,1.2)
    def test_helper_environment_and_argv_have_no_bus_credentials_or_secret(self):
        code='import os,json,sys; print(json.dumps({"status":200,"body":json.dumps({"env":dict(os.environ),"argv":sys.argv})}))'
        with patch.dict(os.environ,{'AGENT_BUS_TOKEN':KEY,'AGENT_BUS_NAME':'#private','AB_ROLE_OWNER':'1','PRIVATE_OTHER':KEY}):
            result=post(self.route(),'body',{'K':KEY},helper=[sys.executable,'-c',code])
        value=json.loads(result['body'])
        self.assertFalse(any(k.startswith(('AGENT_BUS_','AB_ROLE_')) for k in value['env']))
        self.assertNotIn(KEY,json.dumps(value));self.assertNotIn('PRIVATE_OTHER',value['env'])
    def test_helper_deadline_kills_its_descendant_process_group(self):
        import datetime
        with tempfile.TemporaryDirectory() as directory:
            marker=Path(directory)/'child-survived'
            child=f'import time,pathlib;time.sleep(.8);pathlib.Path({str(marker)!r}).write_text("bad")'
            code=f'import subprocess,sys,time;subprocess.Popen([sys.executable,"-c",{child!r}]);time.sleep(30)'
            deadline=(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=.35)).isoformat()
            with self.assertRaises(Invalid):post(self.route(),'body',{}, {'deadline':deadline},helper=[sys.executable,'-c',code])
            time.sleep(.85);self.assertFalse(marker.exists())
    def test_closed_port_is_not_sent_but_dropped_reply_is_unknown(self):
        with socket.socket() as reserve:
            reserve.bind(('127.0.0.1',0));closed=reserve.getsockname()[1]
        route=configuration([{'name':'closed','url':f'http://127.0.0.1:{closed}/fixed'}])[0]
        answer=post(route,'body',{});self.assertFalse(answer.get('unknown'),answer)
        self.server.drop=True
        answer=post(self.route(),'body',{});self.assertTrue(answer.get('unknown'),answer)
        self.assertEqual(len(self.server.calls),1)
    def test_helper_stdin_contains_only_selected_route_credentials(self):
        code='import json,sys; raw=sys.stdin.read(); v=json.loads(raw); print(json.dumps({"status":200,"body":json.dumps({"keys":list(v["secrets"]),"other_seen":"never-send-inbound-key-53a9" in raw})}))'
        route=self.route(basic={'user':'sender','password':'OUT'})
        result=post(route,'body',{'OUT':KEY,'INBOUND':'never-send-inbound-key-53a9'},helper=[sys.executable,'-c',code])
        self.assertEqual(json.loads(result['body']),{'keys':['OUT'],'other_seen':False})
    def test_nonstandard_json_does_not_turn_a_confirmed_status_into_unknown(self):
        self.server.reply=b'{"x":NaN,"x":Infinity,"echo":"'+KEY.encode()+b'"}'
        answer=post(self.route(),'body',{'K':KEY})
        self.assertEqual(answer['status'],200);self.assertNotIn(KEY,answer['body'])
        self.assertIn('NaN',answer['body']);self.assertIn('Infinity',answer['body'])
    def test_tls_wrong_certificate_is_refused_and_trusted_cert_works(self):
        self.server.close()
        with tempfile.TemporaryDirectory() as directory:
            key=Path(directory)/'key.pem';cert=Path(directory)/'cert.pem'
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',str(key),'-out',str(cert),'-days','1','-subj','/CN=localhost','-addext','subjectAltName=DNS:localhost'],check=True,capture_output=True)
            server=Fake();server.shutdown();server.thread.join(2)
            context=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER);context.load_cert_chain(cert,key)
            server.socket=context.wrap_socket(server.socket,server_side=True)
            server.thread=threading.Thread(target=server.serve_forever,daemon=True);server.thread.start();self.server=server
            route=configuration([{'name':'tls','url':f'https://localhost:{server.server_port}/fixed'}])[0]
            result=post(route,'body',{});self.assertIn('error',result);self.assertFalse(result.get('unknown'),result);self.assertEqual(server.calls,[])
            with patch.dict(os.environ,{'SSL_CERT_FILE':str(cert)}):result=post(route,'body',{})
            self.assertEqual(result['status'],200);self.assertEqual(len(server.calls),1)


if __name__=='__main__':unittest.main()
