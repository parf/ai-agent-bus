import importlib.util
import json
import os
import signal
import sys
import time
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('git_agent', HERE / 'git_agent.py')
g = importlib.util.module_from_spec(spec); spec.loader.exec_module(g)
spec = importlib.util.spec_from_file_location('git_install', HERE / 'install.py')
installer = importlib.util.module_from_spec(spec); spec.loader.exec_module(installer)


class GitAgent(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(dir=os.environ.get('AGENT_BUS_TMP', str(HERE.parents[3] / 'tmp')))
        self.base = Path(self.tmp.name)
        self.env = {k: v for k, v in os.environ.items() if not k.startswith(('AB_ROLE_', 'AGENT_BUS_', 'GIT_'))}
        self.env.update(HOME=str(self.base), XDG_STATE_HOME=str(self.base / 'state'), GIT_CONFIG_NOSYSTEM='1')
        self.patcher = patch.dict(os.environ, self.env, clear=True); self.patcher.start()
        self.up = self.base / 'up.git'; self.seed = self.base / 'seed'; self.repo = self.base / 'work'
        self.run_git(self.base, 'init', '--bare', '--initial-branch=main', str(self.up))
        self.run_git(self.base, 'clone', str(self.up), str(self.seed))
        self.run_git(self.seed, 'config', 'user.name', 'Server Author')
        self.run_git(self.seed, 'config', 'user.email', 'server@example.test')
        (self.seed / 'a').write_text('one\n'); (self.seed / 'b').write_text('one\n')
        self.run_git(self.seed, 'add', '.'); self.run_git(self.seed, 'commit', '-m', 'first')
        self.first = self.run_git(self.seed, 'rev-parse', 'HEAD').strip()
        self.run_git(self.seed, 'push', '-u', 'origin', 'main')
        self.run_git(self.base, 'clone', str(self.up), str(self.repo))
        self.run_git(self.repo, 'config', 'user.name', 'Server Author'); self.run_git(self.repo, 'config', 'user.email', 'server@example.test')
        self.repos = {'ab': {'path':str(self.repo), 'access':'commit'}}
        os.environ['AGENT_BUS_CONFIG'] = json.dumps({'repos':self.repos})

    def tearDown(self):
        self.patcher.stop(); self.tmp.cleanup()

    def run_git(self, root, *args):
        p = subprocess.run(['git', '-C', str(root), *args], capture_output=True, text=True, timeout=20)
        self.assertEqual(p.returncode, 0, p.stderr)
        return p.stdout

    def ask(self, method, role='', **kw):
        with patch.dict(os.environ, {'AB_ROLE_DEPLOY': '1' if role == 'deploy' else '', 'AB_ROLE_COMMIT': '1' if role == 'commit' else ''}):
            return json.loads(g.answer( json.dumps({'method': method, 'repo': 'ab', **kw})))

    def ok(self, result): self.assertNotIn('error', result, result)

    def advance(self):
        (self.seed / 'a').write_text('two\n'); (self.seed / 'b').write_text('two\n')
        self.run_git(self.seed, 'commit', '-am', 'second'); self.run_git(self.seed, 'push')
        return self.run_git(self.seed, 'rev-parse', 'HEAD').strip()

    def test_deploy_discards_edits_and_returns_from_detached(self):
        new = self.advance()
        (self.repo / 'a').write_text('local edit\n'); (self.repo / 'junk').write_text('untracked\n')
        result = self.ask('deploy', 'deploy'); self.ok(result)
        self.assertEqual((result['old_head'], result['new_head']), (self.first, new))
        self.assertEqual((self.repo / 'a').read_text(), 'two\n'); self.assertFalse((self.repo / 'junk').exists())
        self.assertEqual(self.run_git(self.repo, 'status', '--porcelain'), '')
        self.ok(self.ask('pull', 'deploy', rev=self.first))
        self.assertIsNone(self.ask('rev')['branch'])
        self.assertEqual(self.ask('rev')['head'], self.first)
        self.ok(self.ask('deploy', 'deploy'))
        self.assertEqual(self.ask('rev')['branch'], 'main'); self.assertEqual(self.ask('rev')['head'], new)

    def test_pull_file_changes_only_named_file(self):
        new = self.advance(); self.ok(self.ask('pull', 'deploy'))
        before = self.run_git(self.repo, 'rev-parse', 'HEAD')
        self.ok(self.ask('pull', 'deploy', rev=self.first, file='a'))
        self.assertEqual((self.repo / 'a').read_text(), 'one\n')
        self.assertEqual((self.repo / 'b').read_text(), 'two\n')
        self.assertEqual(self.run_git(self.repo, 'rev-parse', 'HEAD'), before)
        self.assertEqual(self.run_git(self.repo, 'diff', '--name-only').strip(), 'a')

    def test_refuses_options_unknown_revisions_and_outside_paths_without_changes(self):
        for kw in [{'rev':'--upload-pack=evil'}, {'rev':'no-such-revision'}, {'rev':self.first,'file':'../outside'},
                   {'rev':self.first,'file':'missing'}, {'rev':self.first,'file':str(self.seed / 'a')}, {'rev':self.first,'file':'--work-tree=x'}]:
            result = self.ask('pull','deploy',**kw); self.assertIn('error',result,kw)
            self.assertEqual(self.run_git(self.repo,'status','--porcelain'),'')
            self.assertEqual(self.ask('rev')['head'],self.first)
        (self.repo / 'escape').symlink_to(self.seed, target_is_directory=True)
        self.assertIn('error',self.ask('add','commit',paths=['escape/a']))
        for name in ['missing', str(self.repo), '../seed']:
            self.assertIn('error',json.loads(g.answer(json.dumps({'method':'status','repo':name}))))
        with patch.object(g, 'git', wraps=g.git) as spy:
            self.assertIn('error', self.ask('pull','deploy',rev='--upload-pack=evil'))
            self.assertEqual(spy.call_count,0, 'option refusal must precede every git command')

    def test_roles_body_forgery_and_server_identity(self):
        (self.repo / 'junk').write_text('keep')
        for method, kw in [('clean',{}),('reset',{}),('deploy',{}),('pull',{}),('add',{'paths':['junk']}),('commit',{'message':'test'}),('push',{})]:
            before=self.run_git(self.repo,'status','--porcelain')
            self.assertIn('error',self.ask(method,**kw))
            self.assertEqual(self.run_git(self.repo,'status','--porcelain'),before)
        self.assertIn('error',self.ask('commit','deploy',message='denied'))
        self.assertIn('error',self.ask('clean',role_claim='commit'))
        self.assertIn('error',self.ask('clean',roles=['commit']))
        self.assertTrue((self.repo/'junk').exists())
        self.ok(self.ask('add','commit',paths=['junk']))
        self.assertIn('error',self.ask('commit','commit',message='wrong author',author='Caller <caller@example.test>'))
        self.ok(self.ask('commit','commit',message='server commit'))
        self.assertEqual(self.run_git(self.repo,'log','-1','--format=%an <%ae>').strip(),'Server Author <server@example.test>')
        self.ok(self.ask('push','commit'))
        self.assertEqual(self.run_git(self.up,'rev-parse','main'),self.run_git(self.repo,'rev-parse','HEAD'))
        self.assertEqual(self.run_git(self.repo,'rev-list','--count','HEAD').strip(),'2')
        (self.repo/'trash').write_text('remove'); self.ok(self.ask('clean','commit')); self.assertFalse((self.repo/'trash').exists())

    def test_read_methods_shapes_and_help(self):
        self.advance()
        for method in ['status','log','rev','fetch','diff']: self.ok(self.ask(method))
        self.assertEqual(self.ask('fetch')['behind'],1)
        self.assertEqual(json.loads(g.answer('{"method":"list"}'))['repos'],['ab'])
        self.assertIn('error', json.loads(g.answer( '{"method":[]}')))
        for body in ['log ab 0','log ab 1001','commit ab --amend','add ab','status ab extra']:
            self.assertTrue(g.answer(body).startswith('error:'),body)
        self.assertEqual(g.answer('help'),g.answer('--help'))
        self.assertNotIn('deploy <repo>',g.describe(self.repos))
        with patch.dict(os.environ,{'AB_ROLE_COMMIT':'1'}):
            for method in g.TIERS: self.assertIn(method, g.describe(self.repos))
        self.assertEqual(g.parse('commit ab "two words"')['message'],'two words')

    def test_repository_cap_and_role_are_both_required(self):
        for cap in ['read', 'deploy', 'commit']:
            self.repos['ab']['access'] = cap
            for role in ['', 'deploy', 'commit']:
                with patch.dict(os.environ, {'AB_ROLE_DEPLOY': str(int(role == 'deploy')), 'AB_ROLE_COMMIT': str(int(role == 'commit'))}), patch.object(g, 'operation', return_value={'status':0}) as operation:
                    for method, required in [('status',0), ('pull',1), ('reset',1), ('clean',1), ('deploy',1), ('add',2), ('commit',2), ('push',2)]:
                        allowed = min(['read','deploy','commit'].index(cap), ['', 'deploy','commit'].index(role)) >= required
                        operation.reset_mock()
                        try:
                            g.execute(self.repos, {'method':method,'repo':'ab'})
                            success = True
                        except ValueError:
                            success = False
                        self.assertEqual(success, allowed, (cap,role,method))
                        self.assertEqual(bool(operation.call_count), allowed)

    def test_configuration_changes_caps_roots_and_help_on_next_call(self):
        with patch.dict(os.environ, {'AB_ROLE_COMMIT':'1'}):
            for access in ['read', 'deploy', 'commit']:
                os.environ['AGENT_BUS_CONFIG'] = json.dumps({'repos':{'ab':{'path':str(self.repo),'access':access}}})
                self.assertIn('ab ('+access+')', g.answer('help'))
                self.assertEqual('push <repo>' in g.answer('help'), access == 'commit')
                with patch.object(g, 'operation', return_value={'status':0}):
                    self.assertEqual('error' in json.loads(g.answer('{"method":"pull","repo":"ab"}')), access == 'read')
                    self.assertEqual('error' in json.loads(g.answer('{"method":"push","repo":"ab"}')), access != 'commit')
            os.environ['AGENT_BUS_CONFIG'] = json.dumps({'repos':{'other':{'path':str(self.seed),'access':'read'}}})
            with patch.object(g, 'git', wraps=g.git) as run:
                self.assertIn('undeclared repository: ab', g.answer('status ab'))
                run.assert_not_called()
            self.assertEqual(json.loads(g.answer('{"method":"list"}'))['repos'], ['other'])
            self.assertNotIn('error', g.answer('status other'))

    def test_malformed_repos_answer_including_help_without_git(self):
        configs = [{}, {'repos':None}, {'repos':[]}, {'repos':{'ab':None}},
                   {'repos':{'ab':{'path':'relative','access':'read'}}},
                   {'repos':{'ab':{'path':str(self.repo),'access':[]}}},
                   {'repos':{'ab':{'path':str(self.repo),'access':'admin'}}},
                   {'repos':{'ab':{'path':str(self.repo),'access':'read','branch':'--all'}}}]
        for config in configs:
            with patch.dict(os.environ, {'AGENT_BUS_CONFIG':json.dumps(config)}), patch.object(g,'git') as run:
                for body in ['help','status ab','{"method":"status","repo":"ab"}']:
                    self.assertIn('Configuration repos', g.answer(body))
                run.assert_not_called()
                process=subprocess.run([sys.executable,str(HERE/'git_agent.py'),'help'],capture_output=True,text=True)
                self.assertEqual(process.returncode,0)
                self.assertIn('error: Configuration repos',process.stdout)

    def test_autopull_rechecks_cap_and_missing_repository(self):
        import autopull, timers
        new=self.advance()
        readonly={'repos':{'ab':{'path':str(self.repo),'access':'read'}}}
        with self.assertRaisesRegex(ValueError,'requires access'):
            timers.intervals(['ab=5m'],readonly['repos'])
        result=autopull.attempt('ab', readonly)
        self.assertIn('access cap read',result.get('error',''))
        self.assertEqual(self.run_git(self.repo,'rev-parse','HEAD').strip(),self.first)
        self.assertIn('undeclared repository',autopull.attempt('ab',{'repos':{}})['error'])
        result=autopull.attempt('ab',{'repos':self.repos})
        self.ok(result)
        self.assertEqual(self.run_git(self.repo,'rev-parse','HEAD').strip(),new)

    def test_detached_branch_configuration_and_missing_fallback(self):
        self.run_git(self.repo,'checkout','--detach',self.first)
        self.run_git(self.repo,'symbolic-ref','--delete','refs/remotes/origin/HEAD')
        self.assertIn('branch is required',self.ask('deploy','deploy')['error'])
        os.environ['AGENT_BUS_CONFIG']=json.dumps({'repos':{'ab':{**self.repos['ab'],'branch':'main'}}})
        self.ok(self.ask('deploy','deploy'))
        self.assertEqual(self.ask('rev')['branch'],'main')

    def test_old_roles_do_not_grant_and_paths_must_be_work_trees(self):
        with patch.dict(os.environ, {'AB_ROLE_OPERATE':'1','AB_ROLE_ADMIN':'1'}):
            self.assertIn('requires deploy',g.answer('clean ab'))
        with patch.dict(os.environ, {'AGENT_BUS_CONFIG':json.dumps({'repos':{'ab':{'path':str(self.base),'access':'read'}}})}):
            self.assertIn('error',g.answer('status ab'))

    def test_command_timeout_stops_its_transport_child(self):
        fake = self.base / 'bin'; fake.mkdir()
        pidfile = self.base / 'transport.pid'
        script = fake / 'git'
        script.write_text('#!' + sys.executable + '\nimport subprocess,sys,time,os\np=subprocess.Popen([sys.executable,"-c","import time;time.sleep(30)"])\nopen(os.environ["TRANSPORT_PID"],"w").write(str(p.pid))\ntime.sleep(30)\n')
        script.chmod(0o700)
        env = {**os.environ, 'PATH': str(fake) + ':' + os.environ['PATH'], 'TRANSPORT_PID': str(pidfile)}
        child = None
        try:
            with patch.object(g, 'COMMAND_TIMEOUT', .2):
                with self.assertRaises(g.GitError) as error:
                    g.git(self.repo, ['fetch'], env)
            self.assertEqual(error.exception.result['status'], 124)
            child = int(pidfile.read_text())
            deadline = time.monotonic() + 1
            while time.monotonic() < deadline:
                stat = Path('/proc') / str(child) / 'stat'
                try:
                    state = stat.read_text().split(') ')[-1].split()[0]
                except (FileNotFoundError, ProcessLookupError):
                    break  # reaped between observations is dead too
                if state == 'Z':
                    break
                time.sleep(.01)
            else:
                self.fail('git timeout left its SSH/transport descendant running')
        finally:
            if child is None and pidfile.exists(): child = int(pidfile.read_text())
            if child is not None:
                try: os.kill(child, signal.SIGKILL)
                except ProcessLookupError: pass


    def test_autopull_keeps_dirty_and_diverged_trees_and_records_results(self):
        import autopull
        new = self.advance()
        result = autopull.attempt('ab', {'repos': self.repos})
        self.ok(result)
        self.assertEqual(self.run_git(self.repo, 'rev-parse', 'HEAD').strip(), new)
        (self.repo / 'a').write_text('local edit\n')
        (self.repo / 'junk').write_text('keep\n')
        result = autopull.attempt('ab', {'repos': self.repos})
        self.assertEqual((self.repo/'a').read_text(), 'local edit\n', 'autopull discarded dirty changes')
        self.assertIn('dirty', result.get('error',''))
        self.assertEqual((self.repo/'junk').read_text(), 'keep\n')
        self.assertEqual(self.ask('autopull')['error'], result['error'])
        self.assertIn('at', self.ask('autopull'))
        (self.repo/'junk').unlink()
        self.run_git(self.repo,'commit','-am','local branch')
        local=self.run_git(self.repo,'rev-parse','HEAD').strip()
        (self.seed/'a').write_text('remote third\n')
        self.run_git(self.seed,'commit','-am','upstream diverges');self.run_git(self.seed,'push')
        result=autopull.attempt('ab', {'repos': self.repos})
        self.assertNotEqual(result['status'],0,'autopull forced a diverged branch')
        self.assertEqual(self.run_git(self.repo,'rev-parse','HEAD').strip(),local)

    def test_autopull_timer_arrives_and_removal_stops_it(self):
        import http.server
        import threading
        import pwd
        import shlex
        import timers
        key='agent-bus-auto-test-'+str(os.getpid())+'-'+self.base.name[-6:].replace('_','-')
        name='#'+key.removeprefix('agent-bus-')+'@localhost'
        class Config(http.server.BaseHTTPRequestHandler):
            def do_GET(handler):
                value = {'you': name} if handler.path.startswith('/status') else {'repos': self.repos}
                raw = json.dumps(value).encode()
                handler.send_response(200);handler.send_header('Content-Length',str(len(raw)));handler.end_headers();handler.wfile.write(raw)
            def log_message(*args): pass
        server = http.server.ThreadingHTTPServer(('127.0.0.1',0), Config)
        thread = threading.Thread(target=server.serve_forever,daemon=True);thread.start()
        units=self.base/'units';units.mkdir()
        unit=units/(key+'.service')
        marker='# agent-bus standalone: '+name+'\n'
        envfile=self.base/'agent.env'
        envfile.write_text('AGENT_BUS_ADDR=http://127.0.0.1:'+str(server.server_port)+'\nAGENT_BUS_TOKEN=fixture-agent-only\n')
        state=self.base/'timer-state'
        user=pwd.getpwuid(os.getuid())
        unit.write_text(marker+'[Service]\nUser='+user.pw_name+'\nEnvironmentFile='+str(envfile)+'\nEnvironment=XDG_STATE_HOME='+str(state)+'\n')
        ctl=self.base/'unit-controller'
        ctl.write_text('#!/usr/bin/env python3\nimport sys,subprocess,pathlib,os\nunits=pathlib.Path('+repr(str(units))+')\nif sys.argv[1]=="daemon-reload":\n for p in units.glob("*-autopull-*"):\n  v=subprocess.run(["systemd-analyze","--user","verify","--man=no",str(p)],capture_output=True,text=True)\n  if v.returncode or v.stderr: raise RuntimeError(v.stderr)\n  subprocess.run(["systemctl","--user","link","--runtime",str(p)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)\nos.execvp("systemctl",["systemctl","--user",*sys.argv[1:]])\n')
        ctl.chmod(0o700)
        timer=key+'-autopull-ab.timer'
        try:
            new=self.advance()
            timers.sync(unit, {'ab':'1s'}, self.repos, name, str(ctl))
            deadline=time.monotonic()+15
            while time.monotonic()<deadline:
                if self.run_git(self.repo,'rev-parse','HEAD').strip()==new:break
                time.sleep(.05)
            self.assertEqual(self.run_git(self.repo,'rev-parse','HEAD').strip(),new,'installed autopull timer never pulled the upstream commit')
            (self.repo/'a').write_text('timer must keep this\n')
            (self.repo/'junk').write_text('timer must keep junk\n')
            with patch.dict(os.environ, {'XDG_STATE_HOME':str(state)}):
                deadline=time.monotonic()+15
                while time.monotonic()<deadline:
                    last=self.ask('autopull')
                    if 'dirty' in last.get('error',''):break
                    time.sleep(.05)
                self.assertIn('dirty',last.get('error',''),'timer failure not reported by autopull status')
            self.assertEqual((self.repo/'a').read_text(),'timer must keep this\n')
            self.assertEqual((self.repo/'junk').read_text(),'timer must keep junk\n')
        finally:
            try:
                # Exercise the Git installer's remove branch; the shared
                # starter's own removal is already checked by check.py.
                real_run=subprocess.run
                def controlled(argv,*a,**kw):
                    if argv[:2]==[sys.executable,str(Path(installer.__file__).resolve().parent.parent/'install.py')]:
                        return subprocess.CompletedProcess(argv,0)
                    return real_run(argv,*a,**kw)
                with patch.object(sys,'argv',['install.py','remove','--name',name,'--unit-dir',str(units),'--systemctl',str(ctl)]), patch.object(subprocess,'run',side_effect=controlled):
                    self.assertIsNone(installer.main())
            finally:
                server.shutdown();server.server_close();thread.join()
                # StateDirectory is empty: attempts use the private state above.
                manager_state=Path(user.pw_dir)/'.local/state'/key
                if manager_state.exists():manager_state.rmdir()
        self.assertFalse(list(units.glob('*-autopull-*')),'removal left timer files')
        self.assertNotEqual(subprocess.run(['systemctl','--user','is-active','--quiet',timer]).returncode,0,'removal left an active timer')
        for bad in ['ab=0s','ab=wrong','outside=5m','ab=5m\nExecStart=evil']:
            with self.assertRaises(ValueError):timers.intervals([bad],self.repos)

    def test_configured_key_is_only_identity_and_absence_keeps_account_defaults(self):
        key=self.base/'key with spaces';key.write_text('test fixture')
        with patch.dict(os.environ,{'AGENT_BUS_CONFIG':json.dumps({'repos':self.repos,'ssh_key':str(key)})}):
            cmd=__import__('shlex').split(g.git_environment()['GIT_SSH_COMMAND'])
            self.assertIn('IdentitiesOnly=yes',cmd);self.assertIn('IdentityAgent=none',cmd)
            self.assertEqual(cmd[cmd.index('-i')+1],str(key));self.assertEqual(cmd[cmd.index('-F')+1],'/dev/null')
        self.assertNotIn('GIT_SSH_COMMAND',g.git_environment())


if __name__ == '__main__': unittest.main()
