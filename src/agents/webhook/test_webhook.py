"""Pure contracts and real loopback HTTP. No external endpoint or live bus."""
import base64
from concurrent.futures import ThreadPoolExecutor
import contextlib
from copy import deepcopy
from email.message import Message
import hashlib
import hmac
import http.client
import io
import json
import os
from pathlib import Path
import socket
import sys
import threading
import time
import unittest
from unittest.mock import Mock

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path[:0] = [str(HERE), str(ROOT/'src/clients/python'), str(ROOT/'src/agents/shared')]
from rules import *
from incoming import Listener
from webhook import Agent

RAW = [{'name':'event','path':'/hook','target':'events@fixture','basic':{'user':'sender','password':'PASSWORD'}}]
SECRET = b'PASSWORD=fixture-password-unique\nSIGNING=fixture-signing-unique\n'
AUTH = 'Basic ' + base64.b64encode(b'sender:fixture-password-unique').decode()


def headers(**values):
    result=Message()
    for k,v in values.items(): result[k.replace('_','-')]=v
    return result


class Pure(unittest.TestCase):
    def test_dist_and_root_list_strictness(self):
        self.assertEqual(len(configuration(loads((HERE/'config.json.dist').read_text()))),4)
        for value in ({'routes':RAW},[],[RAW[0],RAW[0]], [dict(RAW[0], url='http://127.0.0.1/')],
                      [dict(RAW[0], extra=True)], [dict(RAW[0], max_bytes=True)], [dict(RAW[0], rate=0)],
                      [dict(RAW[0], signature={'preset':'sha1','secret':'SIGNING'})]):
            with self.subTest(value=value),self.assertRaises(Invalid):configuration(value)
        for p in ('http://x/hook','//host/hook','/x/../hook','/%68ook','/hook?q=a','/x\\hook'):
            with self.assertRaises(Invalid):configuration([dict(RAW[0],path=p)])
        self.assertNotEqual(path('/hook'),path('/hook/'))
    def test_basic_exact_constant_time_inputs_and_missing_secret(self):
        route=configuration(RAW)[0]; keys=credentials([route],SECRET)
        self.assertTrue(basic_ok(route,headers(Authorization=AUTH),keys))
        for value in ('Basic !!!!','Basic '+base64.b64encode(b'sender:wrong').decode(),'Bearer '+AUTH,AUTH+'!'):
            self.assertFalse(basic_ok(route,headers(Authorization=value),keys))
        dup=headers(Authorization=AUTH);dup['Authorization']=AUTH
        self.assertFalse(basic_ok(route,dup,keys))
        with self.assertRaises(Invalid):credentials([route],b'OTHER=unused\n')
        with self.assertRaises(Invalid):credentials([route],b'PASSWORD=\n')
        with self.assertRaises(Invalid):credentials([route],b'PASSWORD=one\nPASSWORD=two\n')
        with self.assertRaises(Invalid):credentials([route],b'PASSWORD=\x00\n')
    def test_github_official_vector_and_duplicate_headers(self):
        route={'signature':{'preset':'github','secret':'K'}}; keys={'K':"It's a Secret to Everybody"}
        signature='sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17'
        h=headers(X_Hub_Signature_256=signature)
        self.assertTrue(signature_ok(route,h,b'Hello, World!',keys))
        self.assertFalse(signature_ok(route,h,b'Hello, World?',keys))
        self.assertTrue(signature_ok(route,headers(X_Hub_Signature_256='sha256='+signature[7:].upper()),b'Hello, World!',keys))
        h['X-Hub-Signature-256']=signature;self.assertFalse(signature_ok(route,h,b'Hello, World!',keys))
    def test_stripe_signed_timestamp_raw_bytes_rotation_and_bounds(self):
        route={'signature':{'preset':'stripe','secret':'K'}};keys={'K':'whsec_fixture'};body=b'{ "x": "\xc3\xa9" }'
        # Fixed HMAC vector independent of the verifier's parsing.
        signature=hmac.new(b'whsec_fixture',b'1000.'+body,hashlib.sha256).hexdigest()
        good='t=1000,v0=ignored,v1='+('0'*64)+',v1='+signature
        self.assertTrue(signature_ok(route,headers(Stripe_Signature=good),body,keys,now=1300))
        self.assertFalse(signature_ok(route,headers(Stripe_Signature=good),body,keys,now=1301))
        self.assertFalse(signature_ok(route,headers(Stripe_Signature=good),body,keys,now=699))
        self.assertFalse(signature_ok(route,headers(Stripe_Signature=good+',t=1000'),body,keys,now=1000))
        self.assertFalse(signature_ok(route,headers(Stripe_Signature=good),body+b' ',keys,now=1000))
        self.assertFalse(signature_ok(route,headers(Stripe_Signature='t=1000,'+','.join(['v1='+signature]*9)),body,keys,now=1000))
    def test_token_bucket_is_bounded_and_uses_monotonic_time(self):
        now=[0.0];r=configuration([dict(RAW[0],burst=2,rate=60)])[0];s=State([r],clock=lambda:now[0])
        self.assertTrue(s.take(r));self.assertTrue(s.take(r));self.assertFalse(s.take(r))
        now[0]=1;self.assertTrue(s.take(r));self.assertFalse(s.take(r))
        now[0]=10000;self.assertTrue(s.take(r));self.assertTrue(s.take(r));self.assertFalse(s.take(r))
        self.assertEqual(len(s.buckets),1)
    def test_call_parser_never_accepts_a_caller_url_or_extra_field(self):
        self.assertEqual(parse_call('post notify { "x": 1 }')['body'],'{ "x": 1 }')
        self.assertEqual(parse_call('{"method":"post","route":"notify","body":"hello"}')['body'],'hello')
        for raw in ('{"method":"post","route":"a","body":"b","url":"http://evil/"}',
                    '{"method":"post","route":"a","route":"b","body":"x"}',
                    '{"method":"post","route":"a","body":true}','post https://evil/ x'):
            with self.assertRaises(Invalid):parse_call(raw)
    def test_logs_have_one_global_fixed_bound(self):
        now=[0.0];s=State([],clock=lambda:now[0]);log=io.StringIO()
        with contextlib.redirect_stderr(log):
            for _ in range(1000):s.log('-',0,'authentication')
            now[0]=61;s.log('event',1,'accepted')
        lines=log.getvalue().splitlines();self.assertEqual(len(lines),12)
        self.assertEqual(json.loads(lines[-2])['count'],990)
        self.assertNotIn('fixture-password',log.getvalue())


class FakeBus:
    def __init__(self):self.sent=[];self.published=[];self.kind='queue';self.may=True;self.problem=None;self.lock=threading.Lock()
    def may_send(self,who,name):return self.may
    def publish(self,channel,body,**options):
        with self.lock:self.published.append((channel,body))
    def record(self,name):
        self.name=name;return self
    def lookup(self):return {'kind':self.kind}
    def send(self,to,body,**options):
        if self.problem:raise self.problem
        with self.lock:self.sent.append((to,body,options))
        return {'id':'fixture'}


class HTTP(unittest.TestCase):
    def setUp(self):
        self.log=io.StringIO();self.redirect=contextlib.redirect_stderr(self.log);self.redirect.__enter__()
        self.bus=FakeBus();self.agent=Agent(self.bus,deepcopy(RAW),SECRET,'127.0.0.1:0')
        self.assertIsNone(self.agent.problem);self.port=self.agent.listener.server_port
    def tearDown(self):
        self.agent.close();self.redirect.__exit__(None,None,None)
    def request(self,path='/hook',body=b'{ "id": 1 }',auth=AUTH,extra=None):
        c=http.client.HTTPConnection('127.0.0.1',self.port,timeout=4)
        h={'Authorization':auth,**(extra or {})}
        try:
            c.request('POST',path,body,h);r=c.getresponse();return r.status,r.read(),r.getheaders()
        finally:c.close()
    def raw(self,data):
        with socket.create_connection(('127.0.0.1',self.port),timeout=4) as s:
            s.sendall(data);chunks=[]
            while True:
                try:part=s.recv(4096)
                except ConnectionResetError:break
                if not part:break
                chunks.append(part)
            return b''.join(chunks)
    def test_exact_body_target_and_one_send_per_request(self):
        body=b'{  "x": "\xc3\xa9" }\n'
        status,result,h=self.request(body=body)
        self.assertEqual(status,202,result);self.assertEqual(self.bus.sent,[('events@fixture',body.decode(),{})])
        self.assertIn(('Connection','close'),h)
        self.assertEqual(self.agent.state.snapshot()['accepted'],1)
    def test_thousand_bad_auth_does_not_consume_a_delivery_token(self):
        for _ in range(1000):self.assertEqual(self.request(auth='Basic bad')[0],404)
        self.assertEqual(self.request()[0],202)
        self.assertEqual(len(self.bus.sent),1)
        self.assertLessEqual(len(self.log.getvalue().splitlines()),10)
    def test_unknown_path_and_bad_basic_have_identical_response(self):
        unknown=self.request(path='/missing');bad=self.request(auth='Basic bad')
        self.assertEqual(unknown,bad);self.assertEqual(self.bus.sent,[])
    def test_signature_and_basic_are_both_required_before_bucket(self):
        self.agent.close();route=deepcopy(RAW[0]);route['signature']={'preset':'github','secret':'SIGNING'};route['burst']=1
        self.agent=Agent(self.bus,[route],SECRET,'127.0.0.1:0');self.port=self.agent.listener.server_port
        body=b'payload';sig='sha256='+hmac.digest(b'fixture-signing-unique',body,'sha256').hex()
        for _ in range(20):self.assertEqual(self.request(body=body,extra={'X-Hub-Signature-256':'sha256='+'0'*64})[0],404)
        self.assertEqual(self.request(body=body,auth='Basic bad',extra={'X-Hub-Signature-256':sig})[0],404)
        self.assertEqual(self.request(body=body,extra={'X-Hub-Signature-256':sig})[0],202)
        self.assertEqual(self.request(body=body,extra={'X-Hub-Signature-256':sig})[0],429)
        self.assertEqual(len(self.bus.sent),1)
    def test_framing_and_pipelining_never_deliver(self):
        for fields,body in [(b'Content-Length: 0\r\nContent-Length: 0\r\n',b''),
                            (b'Content-Length: 0\r\nTransfer-Encoding: chunked\r\n',b''),
                            (b'Content-Length: 0\r\nExpect: 100-continue\r\n',b''),
                            (b'Content-Length: 0\r\n X-Fold: yes\r\n',b''),
                            (b'Content-Length: 1\r\n',b'\xff'),
                            (b'Content-Length: 999999\r\n',b''),
                            (b'Content-Length: 0\r\n',b'POST /hook HTTP/1.1\r\nHost: x\r\nContent-Length: 0\r\n\r\n')]:
            with self.subTest(fields=fields):
                result=self.raw(b'POST /hook HTTP/1.1\r\nHost: x\r\nAuthorization: '+AUTH.encode()+b'\r\n'+fields+b'\r\n'+body)
                self.assertNotIn(b' 202 ',result)
        self.assertEqual(self.bus.sent,[])
    def test_wrong_target_kind_and_unknown_send_are_not_success_or_retry(self):
        from agent_bus.errors import UncertainOutcome
        self.bus.kind='pubsub';self.assertEqual(self.request()[0],503);self.assertEqual(self.bus.sent,[])
        self.bus.kind='agent';self.bus.problem=UncertainOutcome('fixture-secret-in-exception')
        status,body,_=self.request();self.assertEqual(status,503);self.assertIn(b'unknown',body)
        self.assertEqual(self.bus.sent,[]);self.assertNotIn('fixture-secret',self.log.getvalue())
        self.assertEqual(self.agent.state.snapshot()['unknown'],1)
    def test_idle_connections_are_shed_and_valid_request_progresses(self):
        sockets=[socket.create_connection(('127.0.0.1',self.port),timeout=4) for _ in range(6)]
        try:
            time.sleep(.03)
            try: status=self.request()[0]
            except OSError: self.fail('new fast request was refused instead of shedding oldest unauthenticated socket')
            self.assertEqual(status,202)
            self.assertLessEqual(len(self.agent.listener.unauth),6)
            sockets[0].settimeout(1);self.assertEqual(sockets[0].recv(4096),b'')
        finally:
            for sock in sockets:sock.close()
    def test_header_trickle_has_an_absolute_deadline(self):
        with socket.create_connection(('127.0.0.1',self.port),timeout=3) as sock:
            sock.sendall(b'P');start=time.monotonic()
            for _ in range(5):
                time.sleep(.42)
                try:sock.sendall(b'O')
                except OSError:break
            sock.settimeout(1)
            try: received=sock.recv(4096)
            except TimeoutError:self.fail('header deadline left the connection open')
            self.assertEqual(received,b'')
            self.assertLess(time.monotonic()-start,2.8)
    def test_unauthenticated_oversize_signature_route_does_not_reveal_path(self):
        self.agent.close();route={'name':'sign','path':'/hook','target':'events@fixture','signature':{'preset':'github','secret':'SIGNING'},'max_bytes':8}
        self.agent=Agent(self.bus,[route],SECRET,'127.0.0.1:0');self.port=self.agent.listener.server_port
        self.assertEqual(self.request(body=b'x'*9),self.request(path='/unknown',body=b'x'*9))
        self.assertEqual(self.bus.sent,[])
    def test_global_body_cap_is_uniform_before_route_authentication(self):
        prefix=b'POST /hook HTTP/1.1\r\nHost: x\r\nAuthorization: '+AUTH.encode()+b'\r\nContent-Length: 262145\r\n\r\n'
        result=self.raw(prefix)
        self.assertIn(b' 404 ',result);self.assertEqual(self.bus.sent,[])
    def test_empty_authenticated_body_is_forwarded_once(self):
        self.assertEqual(self.request(body=b'')[0],202)
        self.assertEqual(self.bus.sent,[('events@fixture','',{})])
    def test_stop_drains_an_inflight_send_and_rejects_new_acceptance(self):
        entered=threading.Event();release=threading.Event();original=self.bus.send
        def send(*args,**kwargs):
            entered.set();self.assertTrue(release.wait(1));return original(*args,**kwargs)
        self.bus.send=send
        with ThreadPoolExecutor(max_workers=2) as pool:
            incoming=pool.submit(self.request)
            self.assertTrue(entered.wait(1))
            self.agent.begin_stop()
            closing=pool.submit(self.agent.close)
            self.assertFalse(closing.done())
            release.set()
            self.assertEqual(incoming.result()[0],202)
            closing.result(timeout=2)
        self.assertEqual(len(self.bus.sent),1)
    def test_missing_secret_binds_nothing_and_duplicate_bind_stays_alive(self):
        bad=Agent(self.bus,RAW,b'','127.0.0.1:0')
        self.assertIsNone(bad.listener);self.assertTrue(bad.status().startswith('failed:'))
        overlap=Agent(self.bus,RAW,SECRET,'127.0.0.1:'+str(self.port))
        self.assertIsNone(overlap.listener);self.assertTrue(overlap.status().startswith('failed:'))
        self.assertEqual(self.request()[0],202);bad.close();overlap.close()


COMMIT='9f2c1a7e5b3d4c6e8f0a1b2c3d4e5f6a7b8c9d0e'
GITLAB={'object_kind':'push','ref':'refs/heads/main','before':'0'*40,'after':COMMIT,
        'project':{'path_with_namespace':'realmo/realty','name':'realty'},'commits':[{'message':'fixture message'}]}
GITHUB={'ref':'refs/heads/main','after':COMMIT,'deleted':False,'repository':{'full_name':'realmo/realty'}}
PUSH='{"v":1,"type":"push","remote":"realmo/realty","branch":"main","commit":"'+COMMIT+'"}'
LAB={'name':'lab','path':'/lab','publish':'autopull-cmd@fixture','event':'gitlab-push','branches':['main'],'signature':{'preset':'gitlab','secret':'SIGNING'}}
CI={'name':'ci','path':'/ci','publish':'autopull-cmd@fixture','event':'deploy','branches':['main'],'basic':{'user':'sender','password':'PASSWORD'}}


class Publish(unittest.TestCase):
    def test_push_payloads_map_to_the_exact_event(self):
        lab=configuration([LAB])[0];hub=configuration([dict(LAB,event='github-push')])[0]
        self.assertEqual(new_code(lab,headers(X_Gitlab_Event='Push Hook'),json.dumps(GITLAB)),PUSH)
        self.assertEqual(new_code(hub,headers(X_GitHub_Event='push'),json.dumps(GITHUB)),PUSH)
        upper=dict(GITLAB,after=COMMIT.upper())
        self.assertEqual(new_code(lab,headers(X_Gitlab_Event='Push Hook'),json.dumps(upper)),PUSH)
    def test_other_branch_tag_ping_and_deletion_publish_nothing(self):
        lab=configuration([LAB])[0];push=headers(X_Gitlab_Event='Push Hook')
        for value,h in ((dict(GITLAB,ref='refs/heads/feature'),push),(dict(GITLAB,ref='refs/tags/v1'),push),
                        (dict(GITLAB,after='0'*40),push),(GITLAB,headers(X_Gitlab_Event='Tag Push Hook')),(GITLAB,headers())):
            with self.subTest(value=value.get('ref'),h=h.items()):self.assertIsNone(new_code(lab,h,json.dumps(value)))
        hub=configuration([dict(LAB,event='github-push')])[0]
        self.assertIsNone(new_code(hub,headers(X_GitHub_Event='ping'),json.dumps(GITHUB)))
    def test_malformed_push_payloads_are_refused(self):
        lab=configuration([LAB])[0];push=headers(X_Gitlab_Event='Push Hook')
        for value in ([GITLAB],dict(GITLAB,after=COMMIT[:39]),dict(GITLAB,after='g'*40),dict(GITLAB,project={}),
                      dict(GITLAB,project={'path_with_namespace':'realmo/../etc'}),dict(GITLAB,ref='refs/heads/main/.x')):
            with self.subTest(value=value),self.assertRaises(Invalid):new_code(lab,push,json.dumps(value))
        with self.assertRaises(Invalid):new_code(lab,push,'{"ref":"a","ref":"b"}')
    def test_deploy_request_keeps_its_id_and_refuses_extras(self):
        ci=configuration([CI])[0]
        body={'remote':'realmo/realty','branch':'main','commit':'ABCDEF1','id':'pipeline-81234'}
        self.assertEqual(json.loads(new_code(ci,headers(),json.dumps(body))),
                         {'v':1,'type':'deploy','remote':'realmo/realty','branch':'main','commit':'abcdef1','id':'pipeline-81234'})
        self.assertNotIn('id',json.loads(new_code(ci,headers(),json.dumps({k:v for k,v in body.items() if k!='id'}))))
        self.assertIsNone(new_code(ci,headers(),json.dumps(dict(body,branch='other'))))
        for bad in (dict(body,v=1),dict(body,type='push'),dict(body,commit='abcdef'),dict(body,commit='a'*41),
                    dict(body,id='has space'),dict(body,id='x'*65),{'remote':'realmo/realty','branch':'main'}):
            with self.subTest(bad=bad),self.assertRaises(Invalid):new_code(ci,headers(),json.dumps(bad))
    def test_publish_route_fields_are_strict(self):
        for route in (dict(LAB,target='events@fixture'),{k:v for k,v in LAB.items() if k!='event'},
                      {k:v for k,v in LAB.items() if k!='branches'},dict(LAB,event='push'),dict(LAB,branches=[]),
                      dict(LAB,branches=['main','main']),dict(LAB,branches=['../x']),dict(RAW[0],event='deploy'),
                      dict(RAW[0],branches=['main']),{'name':'out','url':'http://127.0.0.1/','publish':'c@fixture'}):
            with self.subTest(route=route),self.assertRaises(Invalid):configuration([route])
    def test_gitlab_token_is_exact_and_single(self):
        route=configuration([LAB])[0];keys={'SIGNING':'fixture-signing-unique'}
        self.assertTrue(signature_ok(route,headers(X_Gitlab_Token='fixture-signing-unique'),b'',keys))
        for value in ('fixture-signing-uniqu','fixture-signing-unique ','x'):
            self.assertFalse(signature_ok(route,headers(X_Gitlab_Token=value),b'',keys))
        dup=headers(X_Gitlab_Token='fixture-signing-unique');dup['X-Gitlab-Token']='fixture-signing-unique'
        self.assertFalse(signature_ok(route,dup,b'',keys))
        self.assertFalse(signature_ok(route,headers(),b'',keys))


class PublishHTTP(unittest.TestCase):
    def setUp(self):
        self.log=io.StringIO();self.redirect=contextlib.redirect_stderr(self.log);self.redirect.__enter__()
        self.bus=FakeBus();self.bus.kind='pubsub'
        self.agent=Agent(self.bus,deepcopy([LAB,CI]),SECRET,'127.0.0.1:0')
        self.assertIsNone(self.agent.problem);self.port=self.agent.listener.server_port
    def tearDown(self):
        self.agent.close();self.redirect.__exit__(None,None,None)
    def post(self,path,value,**h):
        c=http.client.HTTPConnection('127.0.0.1',self.port,timeout=4)
        try:
            c.request('POST',path,json.dumps(value),{k.replace('_','-'):v for k,v in h.items()});r=c.getresponse();return r.status,r.read()
        finally:c.close()
    def test_gitlab_push_publishes_one_event_and_bad_token_or_branch_nothing(self):
        good={'X_Gitlab_Token':'fixture-signing-unique','X_Gitlab_Event':'Push Hook'}
        self.assertEqual(self.post('/lab',GITLAB,**good)[0],202)
        self.assertEqual(self.bus.published,[('autopull-cmd@fixture',PUSH)]);self.assertEqual(self.bus.sent,[])
        self.assertEqual(self.post('/lab',GITLAB,**dict(good,X_Gitlab_Token='wrong'))[0],404)
        self.assertEqual(self.post('/lab',GITLAB,X_Gitlab_Event='Push Hook')[0],404)
        self.assertEqual(self.post('/lab',dict(GITLAB,ref='refs/heads/other'),**good),(202,b'Ignored\n'))
        self.assertEqual(self.post('/lab',dict(GITLAB,after='x'),**good)[0],400)
        self.assertEqual(len(self.bus.published),1)
        self.assertNotIn('fixture message',self.log.getvalue())
    def test_deploy_request_publishes_its_id(self):
        body={'remote':'realmo/realty','branch':'main','commit':COMMIT[:12],'id':'pipeline-81234'}
        self.assertEqual(self.post('/ci',body,Authorization=AUTH)[0],202)
        self.assertEqual(json.loads(self.bus.published[0][1])['id'],'pipeline-81234')
        self.assertEqual(self.post('/ci',body,Authorization='Basic bad')[0],404);self.assertEqual(len(self.bus.published),1)
    def test_a_target_that_is_no_longer_pubsub_publishes_nothing(self):
        self.bus.kind='queue'
        self.assertEqual(self.post('/ci',{'remote':'r/x','branch':'main','commit':COMMIT},Authorization=AUTH)[0],503)
        self.assertEqual(self.bus.published,[])
    def test_unpublishable_channel_is_refused_at_start_and_binds_nothing(self):
        for kind,may in (('queue',True),('agent',True),('pubsub',False)):
            bus=FakeBus();bus.kind=kind;bus.may=may
            agent=Agent(bus,deepcopy([LAB]),SECRET,'127.0.0.1:0')
            with self.subTest(kind=kind,may=may):
                self.assertIsNone(agent.listener);self.assertEqual(agent.status(),'failed: route lab may not publish to its pubsub record')
            agent.close()


if __name__=='__main__':unittest.main()
