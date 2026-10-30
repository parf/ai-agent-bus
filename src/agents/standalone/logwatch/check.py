#!/usr/bin/env python3
"""Disposable daemon + real Python client + recorded shared-starter lifecycle."""
import json
import os
from pathlib import Path
import subprocess
import sys
import time

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE.parents[2]/'clients/python'))
from agent_bus import Bus, CallReply, Refusal

ROOT=Path(os.environ['AB_TEST_DIR'])/'logwatch';ROOT.mkdir()
BIN=Path(os.environ['CLIENT_BIN']);NAME='#logwatch@localhost'
source=ROOT/'fixture.log';source.write_text('initial ordinary\n')
config={'sources':[str(source)],'excludes':[],'ring_size':20,'rescan_interval':.05}
ENV={k:v for k,v in os.environ.items() if not k.startswith('AGENT_BUS_')}
ENV.update(AGENT_BUS_ADDR=os.environ['AB_TEST_TCP'],AGENT_BUS_TOKEN=os.environ['AB_TEST_OWNER_TOKEN'],STANDALONE_TEST_ROOT=str(ROOT))
controller=ROOT/'control.py'
controller.write_text(f'''#!/usr/bin/env python3
import os,subprocess,sys
if sys.argv[1] in ('start','restart'):
 result=subprocess.run([{str(BIN/'agent-bus')!r},'agent-template',{NAME!r},{json.dumps(config)!r}],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 if result.returncode:sys.exit(result.returncode)
os.execv(sys.executable,[sys.executable,{str(HERE.parent/'systemctl-fixture.py')!r},*sys.argv[1:]])
''');controller.chmod(0o755)

def starter(action):
    return subprocess.run([sys.executable,str(HERE/'install.py'),action,'--name',NAME,'--user',str(__import__('pwd').getpwuid(os.getuid()).pw_name),'--allow','@owner',
      '--bus-binary',str(BIN/'agent-bus'),'--systemctl',str(controller),
      '--unit-dir',str(ROOT/'units'),'--config-dir',str(ROOT/'config')],env=ENV,capture_output=True,text=True,timeout=50)

def wait(check):
    end=time.monotonic()+8
    while time.monotonic()<end:
        if check():return
        time.sleep(.02)
    assert check(),'fixture barrier not reached'

with Bus(addr=ENV['AGENT_BUS_ADDR'],token=ENV['AGENT_BUS_TOKEN']) as owner:
    unit=None
    def call(body,bus=owner):
        result=bus.call(NAME,body,timeout=8)
        assert isinstance(result,CallReply),repr(result)
        return result.message.body
    try:
        result=starter('install');assert result.returncode==0,result.stderr
        unit=result.stdout.strip();pidfile=ROOT/(unit+'.pid');pid=pidfile.read_text()
        wait(lambda:owner.record(NAME).lookup().get('readers',0)>0)
        assert 'initial ordinary' in call('tail 5')
        assert 'fixture.log' in call('sources')
        envfile=ROOT/'config'/unit.replace('.service','.env')
        values=dict(line.split('=',1) for line in envfile.read_text().splitlines() if '=' in line and not line.startswith('#'))
        assert values['AGENT_BUS_TOKEN']!=ENV['AGENT_BUS_TOKEN']
        with Bus(addr=ENV['AGENT_BUS_ADDR'],token=values['AGENT_BUS_TOKEN']) as own:
            assert own.name==NAME,'only agent token is stored'
        again=starter('install');assert again.returncode==0,again.stderr
        assert pidfile.read_text()==pid,'unchanged install restarted'
        assert call('mark action')=='action'
        with source.open('a') as file:file.write('after action\n')
        wait(lambda:'after action' in call('tail 5'))
        assert 'initial ordinary' not in call('after action')
        manager='#manager@localhost';outside='#outside@localhost';target='#log-target@localhost'
        for name in [manager,outside,target]:owner.record(name).register(kind='agent',allow=[NAME])
        owner.record(target).manage(maintainers=[manager])
        owner.record(NAME).manage(allow=[owner.name,manager])
        tokens={name:owner._request('POST','/token',{'name':name})['token'] for name in [manager,outside,target]}
        with Bus(addr=ENV['AGENT_BUS_ADDR'],token=tokens[outside]) as stranger:
            try:stranger.send(NAME,'tail 5')
            except Refusal as error:assert error.status==403
            else:raise AssertionError('ACL outsider was accepted')
        with Bus(addr=ENV['AGENT_BUS_ADDR'],token=tokens[manager]) as person,Bus(addr=ENV['AGENT_BUS_ADDR'],token=tokens[target]) as destination:
            before=json.loads(call('{"method":"sub.list"}'))['subs']
            assert 'admission_refused' in call('sub add '+outside,bus=person)
            assert json.loads(call('{"method":"sub.list"}'))['subs']==before
            assert call('sub add '+target+' /(/',bus=person)=='error: invalid_filter'
            assert json.loads(call('{"method":"sub.list"}',bus=person))['subs']==[], 'bad filter was stored'
            sub=json.loads(call('sub add '+target+' /ERROR/i',bus=person))['id']
            with source.open('a') as file:file.write('ordinary nonmatch\nERROR matching\n')
            received=destination.consume(wait=3)
            assert received is not None and 'ERROR matching' in received.body,'subscribed line missing'
            assert received.topic=='logwatch' and received.tag==str(source)
            assert destination.consume(wait=.2) is None,'nonmatching line forwarded'
            assert 'deleted' in call('sub delete '+sub,bus=person)
        restart=subprocess.run([str(controller),'restart',unit],env=ENV,capture_output=True,text=True,timeout=30);assert restart.returncode==0,restart.stderr
        wait(lambda:owner.record(NAME).lookup().get('readers',0)>0)
        assert 'version' in call('stats'),'restart must serve with only agent credential'
        removed=starter('remove');assert removed.returncode==0,removed.stderr
        assert not envfile.exists() and not (ROOT/'units'/unit).exists(),'removal left owned files'
        assert not any(row['name']==NAME for row in owner.records()),'removal left registration'
        print('logwatch install/idempotence/own-token/restart/ACL/destination-subscriptions/remove: green')
    finally:
        if unit:
            result=starter('remove')
            if result.returncode:
                subprocess.run([str(controller),'disable',unit],env=ENV,capture_output=True,timeout=30)
