"""Language-neutral subscription parity cases and real bounded helper checks."""
import copy
from datetime import datetime
import importlib.util
import io
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
        with tempfile.TemporaryDirectory() as directory:
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
            finally:engine.close()

    def test_helper_environment_has_no_bus_or_role_credentials(self):
        engine=self.engine()
        with tempfile.TemporaryDirectory() as directory:
            script=Path(directory)/'env.py'
            script.write_text('import os,json\nprint(json.dumps(sorted(os.environ)))\n')
            try:
                with patch.dict(os.environ,{'AGENT_BUS_TOKEN':'SECRET','AB_ROLE_OWNER':'1','PYTHONPATH':'SECRET'}):
                    raw,complete=engine._helper(script,b'{}',time.monotonic()+1)
                self.assertTrue(complete)
                self.assertFalse(any(key.startswith(('AGENT_BUS_','AB_ROLE_')) or key=='PYTHONPATH' for key in json.loads(raw)))
            finally:engine.close()
