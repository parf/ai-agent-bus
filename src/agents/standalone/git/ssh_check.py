#!/usr/bin/env python3
"""Isolated sshd and bare upstream; proves key selection with real authentication."""
import importlib.util
import json
import os
from pathlib import Path
import shlex
import shutil
import socket
import subprocess
import sys
import tempfile
import time

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('git_agent',HERE/'git_agent.py')
g=importlib.util.module_from_spec(spec);spec.loader.exec_module(g)


def main():
    assert os.geteuid()==0, 'private sshd fixture requires sudo, never changes host sshd'
    with tempfile.TemporaryDirectory(dir=os.environ['AB_TEST_DIR'],prefix='git-ssh-') as directory:
        base=Path(directory);server=None
        def cmd(*args):
            p=subprocess.run(args,capture_output=True,text=True,timeout=15)
            assert p.returncode==0,(args[0],p.stderr)
            return p.stdout
        try:
            for key in ['host','configured','default']:
                cmd('ssh-keygen','-q','-t','ed25519','-N','','-f',str(base/key))
            upstream=base/'up.git';repo=base/'repo'
            cmd('git','init','--bare','--initial-branch=main',str(upstream))
            cmd('git','clone',str(upstream),str(repo))
            cmd('git','-C',str(repo),'config','user.name','Fixture Server');cmd('git','-C',str(repo),'config','user.email','fixture@example.test')
            (repo/'file').write_text('first');cmd('git','-C',str(repo),'add','file');cmd('git','-C',str(repo),'commit','-m','first')
            cmd('git','-C',str(repo),'push','-u','origin','main')
            with socket.socket() as listener:
                listener.bind(('127.0.0.1',0));port=listener.getsockname()[1]
            (base/'authorized').write_text((base/'configured.pub').read_text())
            conf=base/'sshd.conf'
            conf.write_text(f'Port {port}\nListenAddress 127.0.0.1\nHostKey {base}/host\nPidFile {base}/pid\nAuthorizedKeysFile {base}/authorized\nStrictModes no\nPermitRootLogin yes\nPasswordAuthentication no\nKbdInteractiveAuthentication no\nUsePAM no\nLogLevel VERBOSE\n')
            sshd=shutil.which('sshd');assert sshd
            server=subprocess.Popen([sshd,'-D','-f',str(conf),'-E',str(base/'sshd.log')])
            for _ in range(100):
                assert server.poll() is None,'private sshd exited'
                try:
                    with socket.create_connection(('127.0.0.1',port),.1):break
                except OSError:time.sleep(.02)
            else:raise AssertionError('private sshd not ready')
            host=(base/'host.pub').read_text().split()
            (base/'known_hosts').write_text(f'[127.0.0.1]:{port} {host[0]} {host[1]}\n')
            (base/'client.conf').write_text(f'Host *\n IdentityFile {base}/default\n IdentitiesOnly yes\n IdentityAgent none\n BatchMode yes\n')
            (base/'bin').mkdir();ssh=shutil.which('ssh')
            wrapper=base/'bin/ssh';wrapper.write_text('#!/bin/sh\nexec '+shlex.join([ssh,'-o','UserKnownHostsFile='+str(base/'known_hosts'),'-o','StrictHostKeyChecking=yes'])+' "$@"\n');wrapper.chmod(0o755)
            os.environ.update(PATH=str(base/'bin')+':'+os.environ['PATH'],XDG_STATE_HOME=str(base/'state'),AB_ROLE_COMMIT='1',GIT_SSH_COMMAND='ssh -F '+shlex.quote(str(base/'client.conf')))
            os.environ.pop('AB_ROLE_DEPLOY',None)
            cmd('git','-C',str(repo),'remote','set-url','origin',f'ssh://root@127.0.0.1:{port}{upstream}')
            (repo/'file').write_text('second');cmd('git','-C',str(repo),'commit','-am','second')
            repos={'ab':{'path':str(repo),'access':'commit'}}
            os.environ['AGENT_BUS_CONFIG']=json.dumps({'repos':repos})
            denied=json.loads(g.answer('{"method":"push","repo":"ab"}'))
            assert denied['status']!=0,'account default key must be refused by configured-only upstream'
            os.environ['AGENT_BUS_CONFIG']=json.dumps({'repos':repos,'ssh_key':str(base/'configured')})
            allowed=json.loads(g.answer('{"method":"push","repo":"ab"}'))
            assert allowed['status']==0,'configured key must authenticate and push: '+str(allowed)
            assert cmd('git','-C',str(upstream),'rev-parse','main')==cmd('git','-C',str(repo),'rev-parse','HEAD')
            # Server now accepts only the account's default. No config must use it.
            (base/'authorized').write_text((base/'default.pub').read_text())
            still_configured=json.loads(g.answer('{"method":"push","repo":"ab"}'))
            assert still_configured['status']!=0,'configured identity must not fall back to account key'
            os.environ['AGENT_BUS_CONFIG']=json.dumps({'repos':repos})
            default=json.loads(g.answer('{"method":"push","repo":"ab"}'))
            assert default['status']==0,'unconfigured request must use account key'
            print('real SSH: configured-only key, no fallback, and account default all verified')
        finally:
            if server:
                server.terminate()
                try:server.wait(5)
                except subprocess.TimeoutExpired:server.kill();server.wait()


if __name__=='__main__':main()
