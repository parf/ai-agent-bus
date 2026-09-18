#!/usr/bin/env python3
"""G.1.2: actual generated-unit policy, real web, disposable state only.
Usage: web-resources.py BUILT_PROGRAM_DIR NEW_EVIDENCE_DIR
Requires systemd >=254, cgroup2 delegation, installed accounts, sudo and Go.
Pressure is a separate bounded helper placed in the real web cgroup, not an
HTTP exploit or substitute renderer. No live installation is mutated.
"""
import http.client
import os
from pathlib import Path
import pwd
import socket
import subprocess as sp
import sys
import threading
import time
import urllib.request

source = Path(__file__).resolve().parent.parent
binary, out = (Path(p).resolve() for p in sys.argv[1:])
out.mkdir(mode=0o755)
name = f"agent-bus-g12-{os.getpid()}"
state, runtime = Path('/var/lib')/name, Path('/run')/name
unit_path = Path('/run/systemd/system')/(name+'.service')
account = pwd.getpwnam('agent-busd')
checks = 0

def run(*a, **kw): return sp.check_output(list(map(str,a)), text=True, **kw)
def root(*a, **kw): return run('sudo','-n',*a,**kw)
def check(ok, label):
    global checks
    print(('ok ' if ok else 'FAIL ')+label, flush=True)
    if not ok: raise AssertionError(label)
    checks += 1

def wait(fn, label, seconds=15):
    until=time.monotonic()+seconds
    while time.monotonic()<until:
        try:
            v=fn()
            if v: return v
        except (OSError, ValueError, http.client.HTTPException): pass
        time.sleep(.05)
    raise AssertionError('timed out: '+label)

def api(path='/identity', token=''):
    conn=http.client.HTTPConnection('bus',timeout=2)
    conn.sock=socket.socket(socket.AF_UNIX);conn.sock.settimeout(2);conn.sock.connect(str(runtime/'bus.sock'))
    conn.request('GET',path,headers={'X-Agent-Bus-Token':token})
    r=conn.getresponse();body=r.read();status=r.status;conn.close()
    return status,body

def page():
    with urllib.request.urlopen(f'http://127.0.0.1:{port}/signin',timeout=2) as r:
        return r.status==200 and b'<form' in r.read()

def cgroup(pid):
    return Path('/sys/fs/cgroup')/Path(Path(f'/proc/{pid}/cgroup').read_text().strip().removeprefix('0::')).relative_to('/')

def descendants(pid):
    values=run('ps','-eo','pid=,ppid=,comm=').splitlines();todo=[pid];found=[]
    while todo:
        parent=todo.pop()
        for line in values:
            p,pp,comm=line.split(maxsplit=2)
            if int(pp)==parent: found.append((int(p),comm));todo.append(int(p))
    return found

def web_pid():
    found=[p for p,c in descendants(supervisor) if c=='agent-bus-web']
    return found[0] if len(found)==1 else None

def read_root(path): return root('cat',path)
def values(path): return dict(line.split() for line in read_root(path).splitlines())
def unit_text(delegated=True):
    text=run(binary/'agent-bus-setup','--print-unit','--owner','owner@fixture','--addr','127.0.0.1:0','--exec',binary/'agent-busd')
    text=text.replace('/var/lib/agent-bus/daemon',str(state)).replace('StateDirectory=agent-bus/daemon','StateDirectory='+name).replace('/run/agent-bus',str(runtime)).replace('RuntimeDirectory=agent-bus\n','RuntimeDirectory='+name+'\n')
    lines=[]
    for line in text.splitlines():
        if line.startswith('ExecStart='): line+=' -dump-every 0'
        if not delegated and line.startswith(('Delegate=','DelegateSubgroup=')): continue
        lines.append(line)
    # Outer safety cap remains above the helper's finite allocation, so it
    # cannot masquerade as the web limit when that limit is mutated away.
    return '\n'.join(lines).replace('[Install]',f'Environment=AGENT_BUS_WEB_ADDR=127.0.0.1:{port}\nMemoryMax=768M\n[Install]')+'\n'

def install(text):
    (out/'unit.service').write_text(text)
    root('cp',out/'unit.service',unit_path);root('systemctl','daemon-reload');root('systemctl','start',name)

def stop(): root('systemctl','stop',name)
def journal(): return root('journalctl','-u',name,'--no-pager','-o','cat')

with socket.socket() as s: s.bind(('127.0.0.1',0));port=s.getsockname()[1]
run('go','build','-o',out/'pressure','./acceptance/resource-pressure',cwd=source)
try:
    install(unit_text())
    wait(lambda:api()[0]==200,'bus ready')
    supervisor=int(run('systemctl','show',name,'-p','MainPID','--value').strip())
    bus=wait(lambda: next((p for p,c in descendants(supervisor) if c=='agent-busd'),None),'bus child')
    first=wait(web_pid,'web child');wait(page,'real sign-in renders')
    web_group=cgroup(first);bus_group=cgroup(bus)
    check(web_group!=bus_group and cgroup(supervisor)==bus_group,'web limit excludes bus and supervisor')
    expected={'memory.max':'268435456','memory.swap.max':'0','memory.oom.group':'1','pids.max':'64','cpu.max':'100000 100000'}
    check(all(read_root(web_group/k).strip()==v for k,v in expected.items()),'all configured web limits installed')
    check(page(),'actual web serves under the limits')
    token=root('cat',state/'token').split()[1]
    for kind,event,key in [('cpu','cpu.stat','nr_throttled'),('pids','pids.events','max'),('memory','memory.events','oom_kill')]:
        before=int(values(web_group/event)[key]);failures=[];successes=[];done=threading.Event()
        def positive():
            while not done.is_set():
                try:
                    status,body=api('/status',token)
                    if status!=200: failures.append(status)
                    else: successes.append(time.monotonic())
                except Exception as e: failures.append(str(e))
                done.wait(.05)
        monitor=threading.Thread(target=positive);monitor.start()
        try:
            p=sp.run(['sudo','-n',str(out/'pressure'),str(web_group),str(account.pw_uid),str(account.pw_gid),kind],stdout=sp.PIPE,stderr=sp.STDOUT,text=True,timeout=20)
            (out/(kind+'.log')).write_text(p.stdout)
            check('pressure member ' in p.stdout and str(web_group).removeprefix('/sys/fs/cgroup') in p.stdout,
                  kind+': pressure helper starts in the measured web cgroup')
        finally: done.set();monitor.join(timeout=3)
        check(successes and not failures,kind+': authenticated bus calls continue during pressure')
        check(int(values(web_group/event)[key])>before,kind+': kernel records the web limit taking effect')
        check(cgroup(bus)==bus_group and int(run('systemctl','show',name,'-p','MainPID','--value'))==supervisor,kind+': same bus and supervisor survive')
        wait(page,kind+': real web serves afterward')
        check(page(),kind+': real renderer positive control after pressure')
        if kind=='memory':
            replacement=wait(lambda:web_pid() if web_pid()!=first else None,'OOM web restart')
            check(replacement!=first and cgroup(replacement)==web_group,'OOM replaces real web inside the same limited cgroup')
            logs=journal();check('web died' in logs and 'restarting in 100ms' in logs,'web OOM restart and backoff are observable')
    groups=root('find',web_group.parent,'-maxdepth','1','-type','d','-name','web-*').splitlines()
    check(len(groups)==1,'restarts reuse one cgroup without leaking new directories')
    stop();check(not web_group.exists(),'supervisor shutdown removes the web cgroup')
    # Missing delegation is a separate failure path: bus up, web never execs,
    # no fallback group search and a concrete operator diagnostic.
    install(unit_text(False));wait(lambda:api()[0]==200,'bus without delegation')
    supervisor=int(run('systemctl','show',name,'-p','MainPID','--value').strip())
    def delegation_failure():
        # Observe the whole wait: an unlimited renderer must not appear even
        # briefly while the supervisor is retrying and writing its diagnostic.
        if web_pid():
            raise AssertionError('missing delegation never starts an unlimited renderer')
        return 'web limits unavailable; refusing unlimited web' in journal()
    wait(delegation_failure,'delegation failure diagnostic')
    check(not web_pid(),'missing delegation never starts an unlimited renderer')
    check(api('/status',token)[0]==200,'bus stays available without web delegation')
    check('DelegateSubgroup=supervisor' in journal(),'missing-delegation diagnostic names recovery configuration')
    print(f'checks {checks}, failed 0',flush=True)
finally:
    (out/'journal.log').write_text(journal())
    stop()
    root('rm','-f',unit_path);root('systemctl','daemon-reload')
    root('rm','-rf',state)
