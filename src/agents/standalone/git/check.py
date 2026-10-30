#!/usr/bin/env python3
"""Private starter/daemon fixture: no host units or working repositories touched."""
import json
import os
from pathlib import Path
import subprocess
import shutil
import sys
import time

HERE=Path(__file__).resolve().parent;BIN=Path(os.environ['CLIENT_BIN'])
ROOT=Path(os.environ['AB_TEST_DIR'])/'git-installed';ROOT.mkdir();(ROOT/'bin').mkdir()
ENV={k:v for k,v in os.environ.items() if not k.startswith(('AGENT_BUS_','AB_ROLE_','GIT_'))}
ENV.update(AGENT_BUS_ADDR=os.environ['AB_TEST_TCP'],AGENT_BUS_TOKEN=os.environ['AB_TEST_OWNER_TOKEN'],
           B2_UNIT_ROOT=str(ROOT),XDG_STATE_HOME=str(ROOT/'admin-state'))
NAME='#git@localhost';unit='agent-bus-git.service'


def run(args,env=ENV):
    p=subprocess.run(args,env=env,capture_output=True,text=True,timeout=60)
    assert p.returncode==0,p.stderr
    return p.stdout


def cli(*args,token=None):return run([str(BIN/'agent-bus'),*args],{**ENV,**({'AGENT_BUS_TOKEN':token} if token else {})})


def install(action):
    return run([sys.executable,str(HERE/'install.py'),action,'--name',NAME,'--user','root',
        '--allow','#plain@localhost,#operator@localhost(commit)',
        '--bus-binary',str(BIN/'agent-bus'),'--systemctl',str(HERE.parent/'systemd/fixture-unit.py'),
        '--unit-dir',str(ROOT/'units'),'--config-dir',str(ROOT/'config')])


def ready():
    for _ in range(200):
        if any(r['name']==NAME and r.get('readers',0)>0 for r in json.loads(cli('ls','--all'))):return
        time.sleep(.025)
    raise AssertionError('installed git reader did not start')


def autopull_check(tokens, ask):
    sys.path.insert(0,str(HERE.parents[2]/'clients/python'))
    from agent_bus import Bus
    repo=ROOT/'repo';seed=ROOT/'seed'
    run(['git','clone',str(ROOT/'remote.git'),str(seed)])
    for k,v in [('user.name','Fixture'),('user.email','fixture@example.test')]:run(['git','-C',str(seed),'config',k,v])
    def advance(text):
        (seed/'file').write_text(text)
        run(['git','-C',str(seed),'commit','-am','PRIVATE_COMMIT_MESSAGE_SENTINEL'])
        run(['git','-C',str(seed),'push'])
        return run(['git','-C',str(seed),'rev-parse','HEAD']).strip()
    command='autopull-cmd@localhost';log='autopull-log@localhost';queue='autopull-results@localhost'
    for name,kind in [(command,'pubsub'),(log,'pubsub'),(queue,'queue')]:cli('channel','create',name,'--kind',kind)
    cli('manage',command,'--add-allow','#plain@localhost','--deliver-to',NAME)
    cli('manage',log,'--add-allow',NAME,'--deliver-to',queue)
    config={'repos':{'ab':{'path':str(repo),'access':'deploy','follow':True,'mode':'ff'}},'autopull-cmd':command,'autopull-log':log}
    cli('agent-template',NAME,json.dumps(config))
    wanted=advance('second')
    body={'v':1,'type':'push','remote':'remote','branch':'main','commit':wanted,'id':'fixture-1'}
    # Same sender/body: direct call has no channel authority.
    direct=ask('plain',body)
    assert direct['error']=='unknown method','direct event bypassed channel ACL'
    assert run(['git','-C',str(repo),'rev-parse','HEAD']).strip()!=wanted
    with Bus(addr=ENV['AGENT_BUS_ADDR'],token=ENV['AGENT_BUS_TOKEN'],timeout=3) as bus:
        def record():
            message=bus.consume(inbox=queue,wait=3)
            assert message is not None,'expected exactly one result publication'
            value=json.loads(message.body)
            assert getattr(message,'from')==NAME,'publication did not use git identity'
            assert message.topic=='ab@'+value['host'] and message.tag==value['result']
            for secret in [str(repo),str(ROOT/'remote.git'),'PRIVATE_COMMIT_MESSAGE_SENTINEL']:
                assert secret not in message.body,'result leaked checkout, origin or subject'
            return value
        cli('publish','--channel',command,json.dumps(body),token=tokens['plain'])
        value=record()
        assert value['result']=='ok' and value['to']==wanted and value['from']!=value['to']
        assert value['id']=='fixture-1' and value['requested_by']=='#plain@localhost'
        assert bus.consume(inbox=queue,wait=0) is None,'duplicate attempt publication'
        (repo/'file').write_text('keep dirty')
        cli('publish','--channel',command,json.dumps(body),token=tokens['plain'])
        value=record();assert value['error']['code']=='dirty' and (repo/'file').read_text()=='keep dirty'
        # Explicit mirror via current Configuration, no restart required.
        config['repos']['ab']['mode']='mirror';cli('agent-template',NAME,json.dumps(config))
        (repo/'untracked').write_text('discard')
        cli('publish','--channel',command,json.dumps(body),token=tokens['plain'])
        value=record();assert value['result']=='ok' and value['discarded']['files']==2
        assert not (repo/'untracked').exists()
        # Refused log delivery is not a Git failure; the direct answer is pure JSON.
        cli('manage',log,'--remove-allow',NAME)
        value=ask('owner',{'method':'deploy','repo':'ab','branch':'main','rev':wanted[:8],'id':'direct-1'})
        assert value['result']=='skip' and value['id']=='direct-1'
        status=ask('owner',{'method':'autopull','repo':'ab'})
        assert 'log publish failed' in status['stdout']
        assert bus.consume(inbox=queue,wait=0) is None,'refused publish was retried or delivered'
        cli('manage',log,'--add-allow',NAME)
        # A restart catches exactly one missed update without an event.
        new=advance('third')
        run([str(HERE.parent/'systemd/fixture-unit.py'),'restart',unit]);ready()
        value=record();assert value['trigger']=='start' and value['to']==new and value['result']=='ok'
        assert bus.consume(inbox=queue,wait=0) is None,'startup duplicated publication'
        assert not list((ROOT/'units').glob('*-autopull-*')),'reinstall retained old timers'
    print('private installed startup, channel authority, exact records, mirror and log failure verified')


def main():
    assert os.geteuid()==0,'isolated unit fixture requires sudo'
    run(['git','init','--initial-branch=main',str(ROOT/'repo')])
    for k,v in [('user.name','Server'),('user.email','server@example.test')]:run(['git','-C',str(ROOT/'repo'),'config',k,v])
    (ROOT/'repo/file').write_text('fixture')
    run(['git','-C',str(ROOT/'repo'),'add','file']);run(['git','-C',str(ROOT/'repo'),'commit','-m','first'])
    tokens={'owner':ENV['AGENT_BUS_TOKEN']}
    for who in ['plain','operator']:
        cli('register','#'+who+'@localhost','--kind','agent','--allow',NAME)
        tokens[who]=run([str(BIN/'agent-bus'),'token' ,'#'+who+'@localhost']).strip()
    def ask(who,body):return json.loads(json.loads(cli('call',NAME,'--wait','10s',json.dumps(body),token=tokens[who]))['body'])
    try:
        assert install('install').strip()==unit;ready()
        pid=(ROOT/(unit+'.pid')).read_text();install('install');ready()
        assert (ROOT/(unit+'.pid')).read_text()==pid,'unchanged install restarted keeper'
        assert len(list((ROOT/'units').glob('*.service')))==1
        assert sum(r['name']==NAME for r in json.loads(cli('ls','--all')))==1
        cli('agent-template', NAME, json.dumps({'repos':{'ab':{'path':str(ROOT/'repo'),'access':'commit'}}}))
        assert next(r for r in json.loads(cli('ls','--all')) if r['name']==NAME)['roles']==['deploy','commit']
        assert ask('plain',{'method':'list'})['repos']==['ab']
        assert ask('plain',{'method':'rev','repo':'ab'})['branch']=='main'
        (ROOT/'repo/junk').write_text('keep')
        assert 'error' in ask('plain',{'method':'clean','repo':'ab'})
        assert 'error' in ask('operator',{'method':'clean','repo':'ab','roles':['admin']})
        assert (ROOT/'repo/junk').exists(),'inherited or body role granted clean'
        assert ask('operator',{'method':'add','repo':'ab','paths':['junk']})['status']==0
        assert ask('operator',{'method':'commit','repo':'ab','message':'delivered commit'})['status']==0
        assert 'error' in ask('plain',{'method':'status','repo':str(ROOT/'repo')})
        run(['git','init','--bare','--initial-branch=main',str(ROOT/'remote.git')])
        run(['git','-C',str(ROOT/'repo'),'remote','add','origin',str(ROOT/'remote.git')])
        run(['git','-C',str(ROOT/'repo'),'push','-u','origin','main'])
        assert ask('owner',{'method':'push','repo':'ab'})['status']==0
        cli('agent-template', NAME, json.dumps({'repos':{'ab':{'path':str(ROOT/'repo'),'access':'read'}}}))
        assert 'access cap read' in ask('owner',{'method':'push','repo':'ab'})['error']
        assert 'access cap read' in ask('operator',{'method':'clean','repo':'ab'})['error']
        assert ask('plain',{'method':'rev','repo':'ab'})['branch']=='main'
        autopull_check(tokens,ask)
        cli('agent-template', NAME, json.dumps({'repos':{}}))
        assert 'undeclared repository' in ask('operator',{'method':'status','repo':'ab'})['error']
    finally:
        install('remove')
    assert not (ROOT/'units'/unit).exists()
    assert not (ROOT/'config/agent-bus-git.env').exists()
    assert not any(r['name']==NAME for r in json.loads(cli('ls','--all')))
    print('starter idempotence/removal and real delivered roles verified')


if __name__=='__main__':
    try: main()
    finally:
        if not (ROOT/(unit+'.pid')).exists(): shutil.rmtree(ROOT)
