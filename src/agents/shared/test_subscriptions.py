"""Language-neutral subscription parity cases and real bounded helper checks."""
import copy
from datetime import datetime
import importlib.util
import io
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit, parse_qs
import json
import os
from pathlib import Path
import signal
import sys
import tempfile
import threading
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
import subscriptions as s
import subscription_filters as f
from agent_bus.errors import Refusal, UnexpectedAnswer, UncertainOutcome, BusTimeout
VECTORS=HERE.parents[1]/'testdata/subscriptions.json'


class Store:
    def __init__(self,initial=()):self.rows={'sub.'+r['id']:copy.deepcopy(r) for r in initial};self.step={};self.writes=0
    def list(self):return {'json':[{'name':key} for key in sorted(self.rows)]}
    def get(self,key,*,kind):
        if key not in self.rows:raise Refusal(404,'absent')
        return copy.deepcopy(self.rows[key])
    def set(self,key,value,*,kind,mode):
        self.writes+=1
        if mode=='add' and key in self.rows:raise Refusal(412,'exists')
        self.rows[key]=copy.deepcopy(value)
        if self.step.get('store')=='unknown_after':raise UncertainOutcome('not confirmed')
    def delete(self,key,*,kind):return self.rows.pop(key,None) is not None
    def json(self,key,*ops):
        self.writes+=1
        if self.step.get('counter_store')=='unknown':raise UncertainOutcome('not confirmed')
        if key not in self.rows:raise Refusal(404,'absent')
        for op in ops:self.rows[key][op['key']]=copy.deepcopy(op['value'])
        return []


class Bus:
    def __init__(self,source,initial=()):
        self.name=source;self.kv=Store(initial);self.step={};self.queries=[];self.attempts=[];self.answers=[]
    def record(self,name):
        def lookup():return {'name':name if '@' in name else name+'@fixture.test','kind':self.step.get('kind') or 'queue'}
        return SimpleNamespace(kv=self.kv,lookup=lookup)
    def may_send(self,who,target):
        self.queries.append({'who':who,'target':target})
        value=self.answers.pop(0) if self.answers else True
        if value=='404':raise Refusal(404,'absent')
        if value=='unexpected':raise UnexpectedAnswer('foreign')
        if value=='timeout':raise BusTimeout()
        return value
    def send(self,target,body,**options):
        self.attempts.append((target,body,options))
        value=self.step.get('send','ok')
        if value=='refused':raise Refusal(429,'full')
        if value=='unknown':raise UncertainOutcome('not confirmed')
        if value=='must-not-send':raise AssertionError('unexpected send')
        return 'accepted'


def selector(value):
    if not isinstance(value,dict) or set(value)-{'log','level'} or any(not isinstance(v,str) for v in value.values()):raise ValueError()
    return value


def selected(value,event):return all(event.get(key)==wanted for key,wanted in value.items())


def message(step):
    e={'from':step['author'],'roles':step.get('roles',[])}
    if 'original_to' in step:e['original_to']=step['original_to']
    return SimpleNamespace(envelope=e)


class Parity(unittest.TestCase):
    def test_filter_vectors(self):
        for row in json.loads(VECTORS.read_text())['filters']:
            with self.subTest(filter=repr(row['filter']),text=repr(row.get('text'))[:60]):
                try:
                    compiled=f.compile_filter(row['filter'])
                    result=f.matches(compiled,row['text']) if 'text' in row else None
                except f.FilterError as error:
                    self.assertEqual(error.code,row.get('error'),row)
                else:
                    self.assertNotIn('error',row,row)
                    self.assertEqual(result,row['match'],row)

    def test_operation_vectors(self):
        for case in json.loads(VECTORS.read_text())['cases']:
            with self.subTest(case=case['name']):self.case(case)

    def case(self,case):
        bus=Bus(case['source'],case.get('initial',[]));warnings=[];ids=list(case.get('ids',[]));serial=0
        def next_id():
            nonlocal serial
            serial+=1
            if 'ids' in case:
                self.assertTrue(ids,'id factory exhausted')
                return ids.pop(0)
            return f'{serial:012x}'
        now=datetime.fromisoformat(case.get('now','2026-10-29T00:00:00.000Z').replace('Z','+00:00')).timestamp()
        def delivery(rows,body,topic,tag,deadline):
            cache={};return {row['id']:s.deliver_one(bus,bus.name,row,body,topic,tag,deadline,cache) for row in rows}
        engine=s.Subscriptions(bus,bus.name,validate_selector=selector,selector_matches=selected,warning=warnings.append,clock=lambda:now,id_factory=next_id,delivery=delivery,**case.get('limits',{}))
        try:
            for step in case['steps']:
                bus.step=bus.kv.step=step;bus.answers=list(step.get('checks',[]));bus.queries=[];bus.attempts=[];bus.kv.writes=0;warnings.clear()
                expected=step['expect'];actual={}
                try:
                    if step['op']=='add':actual['row']=engine.add(message(step),step['target'],step['selector'],step.get('filter',''))
                    elif step['op']=='list':actual['rows']=engine.list(message(step))
                    elif step['op']=='delete':actual['deleted']=engine.delete(message(step),step['id'])
                    elif step['op']=='deliver':
                        event=step['event'];deadline=time.monotonic()+5 if step.get('deadline')!='expired' else time.monotonic()-1
                        actual.update(engine.deliver(event,event['text'],json.dumps(event),topic=event['log'],tag=event['level'],deadline=deadline))
                    else:self.fail('unknown vector operation')
                except s.SubscriptionError as error:actual['error']=error.code
                actual.update(queries=bus.queries,attempts=len(bus.attempts),warnings=warnings,writes=bus.kv.writes)
                for field,value in expected.items():self.assertEqual(actual.get(field),value,(case['name'],step['op'],field,actual))
        finally:engine.close()


class Helpers(unittest.TestCase):
    def engine(self):return s.Subscriptions(Bus('#eventlog@fixture.test'),'#eventlog@fixture.test',validate_selector=selector,selector_matches=selected,warning=lambda _:None)

    def test_regex_helper_timeout_is_per_pattern_and_group_reaped(self):
        engine=self.engine()
        try:
            request=json.dumps({'text':'a'*180+'b','filters':['/(a|aa)+$/','/b$/']}).encode()
            started=time.monotonic();raw,complete=engine._helper(HERE/'subscription_filters.py',request,time.monotonic()+1)
            self.assertTrue(complete);self.assertEqual(json.loads(raw),['timeout',True]);self.assertLess(time.monotonic()-started,1)
            self.assertFalse(engine.processes)
        finally:engine.close()

    def test_js_whitespace_is_not_a_range_string_or_hyphen(self):
        for text,want in [('-',False),('\u2003',True),('\u200a',True),('\u180e',False)]:
            self.assertEqual(f.matches(f.compile_filter('/^\\s$/'),text),want)

    def test_alarm_during_disarm_does_not_drop_the_batch(self):
        request=SimpleNamespace(buffer=io.BytesIO(json.dumps({'text':'x','filters':['/x/','/x/']}).encode()))
        out=io.StringIO();handler=None;real_signal=signal.signal
        def install(sig,value):
            nonlocal handler
            handler=value
            return signal.SIG_DFL
        def timer(kind,value):
            if value==0 and handler is not None:handler(signal.SIGALRM,None)
        with patch.object(f.sys,'stdin',request),patch.object(f.sys,'stdout',out),patch.object(f.signal,'signal',side_effect=install),patch.object(f.signal,'setitimer',side_effect=timer):
            self.assertEqual(f.worker(),0)
        self.assertEqual(json.loads(out.getvalue()),[True,True])

    def test_helper_kills_leader_and_descendant_holding_stdout(self):
        engine=self.engine()
        with tempfile.TemporaryDirectory(dir=HERE.parents[2]/"tmp") as directory:
            root=Path(directory);script=root/'held.py';pidfile=root/'child.pid'
            script.write_text('import subprocess,sys,time\np=subprocess.Popen([sys.executable,"-c","import time;time.sleep(30)"])\nopen('+repr(str(pidfile))+',"w").write(str(p.pid))\nprint("started",flush=True)\n')
            try:
                raw,complete=engine._helper(script,b'{}',time.monotonic()+.3)
                self.assertFalse(complete);self.assertIn(b'started',raw)
                self.assertTrue(pidfile.is_file(),'fixture child did not start')
                pid=int(pidfile.read_text());end=time.monotonic()+1
                while time.monotonic()<end:
                    try:state=Path('/proc',str(pid),'stat').read_text().rsplit(')',1)[1].split()[0]
                    except (FileNotFoundError,ProcessLookupError):break
                    if state=='Z':break
                    time.sleep(.01)
                else:self.fail('helper left a descendant alive')
                self.assertFalse(engine.processes)
            finally:
                engine.close()
                if pidfile.is_file():
                    try:os.kill(int(pidfile.read_text()),signal.SIGKILL)
                    except ProcessLookupError:pass

    def test_helper_environment_has_no_bus_or_role_credentials(self):
        engine=self.engine()
        with tempfile.TemporaryDirectory(dir=HERE.parents[2]/"tmp") as directory:
            script=Path(directory)/'env.py'
            script.write_text('import os,json\nprint(json.dumps(sorted(os.environ)))\n')
            try:
                with patch.dict(os.environ,{'AGENT_BUS_TOKEN':'SECRET','AB_ROLE_OWNER':'1','PYTHONPATH':'SECRET'}):
                    raw,complete=engine._helper(script,b'{}',time.monotonic()+1)
                self.assertTrue(complete)
                self.assertFalse(any(key.startswith(('AGENT_BUS_','AB_ROLE_')) or key=='PYTHONPATH' for key in json.loads(raw)))
            finally:engine.close()


class Delivery(unittest.TestCase):
    """Exercise the actual send-only child and Python HTTP client, not an injected sender."""
    def setUp(self):
        self.calls=[];self.accepted=[];self.refuse=True;self.revoked=None;self.slow=False
        fixture=self
        class Handler(BaseHTTPRequestHandler):
            def log_message(self,*_):pass
            def answer(self,status,value):
                raw=json.dumps(value).encode()
                try:
                    self.send_response(status);self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
                except (BrokenPipeError,ConnectionResetError):pass
            def do_GET(self):
                path=urlsplit(self.path);q=parse_qs(path.query)
                fixture.calls.append((path.path,q,self.headers.get('X-Agent-Bus-Token')))
                if path.path=='/status':self.answer(200,{'you':'#eventlog@fixture.test'})
                elif path.path=='/may-send':
                    if fixture.slow:time.sleep(.4)
                    self.answer(200,{'who':q['who'][0],'may_send':q['name'][0]!=fixture.revoked})
                elif path.path=='/lookup':self.answer(200,{'name':q['name'][0],'kind':'queue'})
                else:self.answer(404,{'error':'unexpected request'})
            def do_POST(self):
                value=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                fixture.calls.append((self.path,value,self.headers.get('X-Agent-Bus-Token')))
                if self.path!='/send':self.answer(404,{'error':'unexpected mutation'})
                elif fixture.refuse:self.answer(429,{'error':'full'})
                else:
                    fixture.accepted.append(value);self.answer(200,{'message_id':'copy'})
        self.server=ThreadingHTTPServer(('127.0.0.1',0),Handler)
        self.thread=threading.Thread(target=self.server.serve_forever,daemon=True);self.thread.start()
        self.bus=Bus('#eventlog@fixture.test')
        self.bus._transport=SimpleNamespace(addr='http://127.0.0.1:'+str(self.server.server_port),token='fixture-secret',pin='')
        self.warnings=[]
        self.engine=s.Subscriptions(self.bus,self.bus.name,validate_selector=selector,selector_matches=selected,warning=self.warnings.append,id_factory=lambda:'000000000001')
        self.engine.add(message({'author':'alice@fixture.test'}),'dest@fixture.test',{})
    def tearDown(self):
        self.engine.close();self.server.shutdown();self.server.server_close();self.thread.join()
    def deliver(self):
        return self.engine.deliver({'log':'deploys'},'entry','payload',topic='deploys',tag='alert',deadline=time.monotonic()+2)
    def test_refused_copy_warns_once_then_next_entry_is_delivered_once(self):
        self.assertEqual(self.deliver(),{'matched':1,'sent':0,'dropped':1})
        self.assertEqual(self.warnings,['send_refused']);self.assertEqual(self.accepted,[])
        self.refuse=False;self.warnings.clear()
        self.assertEqual(self.deliver(),{'matched':1,'sent':1,'dropped':0})
        self.assertEqual(self.warnings,[])
        self.assertEqual(self.accepted,[{'to':'dest@fixture.test','body':'payload','topic':'deploys','tag':'alert','wait':'0.000000000s'}])
        self.assertEqual(sum(path=='/send' for path,_,_ in self.calls),2)
        row=self.engine.list(message({'author':'alice@fixture.test'}))[0]
        self.assertEqual((row['sent'],row['dropped']),(1,1))
        self.assertFalse(self.engine.processes)
        self.assertTrue(all(token=='fixture-secret' for _,_,token in self.calls))
        self.assertTrue(all(path in ('/status','/may-send','/lookup','/send') for path,_,_ in self.calls),'helper must not register, unregister or consume')
    def test_revoked_creator_to_target_or_source_prevents_delivery(self):
        self.refuse=False
        for target in ('dest@fixture.test','#eventlog@fixture.test'):
            with self.subTest(target=target):
                self.calls.clear();self.warnings.clear();self.revoked=target
                self.assertEqual(self.deliver(),{'matched':1,'sent':0,'dropped':1})
                self.assertEqual(self.warnings,['admission_refused']);self.assertEqual(self.accepted,[])
                wanted=['dest@fixture.test'] if target=='dest@fixture.test' else ['dest@fixture.test','#eventlog@fixture.test']
                self.assertEqual([q for path,q,_ in self.calls if path=='/may-send'],[{'who':['alice@fixture.test'],'name':[name]} for name in wanted])
                self.assertFalse(any(path=='/send' for path,_,_ in self.calls))
    def test_stalled_authority_is_bounded_and_leaves_no_worker(self):
        self.slow=True;self.refuse=False
        start=time.monotonic()
        self.assertEqual(self.deliver(),{'matched':1,'sent':0,'dropped':1})
        self.assertLess(time.monotonic()-start,1.5)
        self.assertEqual(self.warnings,['admission_unavailable']);self.assertEqual(self.accepted,[])
        self.assertFalse(self.engine.processes)
