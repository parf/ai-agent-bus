import importlib.util
import io
from contextlib import redirect_stdout
import json
import os
from pathlib import Path
import shlex
import shutil
import socket
import subprocess
import tempfile
import time
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
spec = importlib.util.spec_from_file_location('sessions_agent', HERE/'sessions.py')
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)


class Sessions(unittest.TestCase):
    def setUp(self):
        (ROOT/'tmp').mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=ROOT/'tmp', prefix='windows-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root/'bin'; self.bin.mkdir()
        self.socket = self.root/'tmux.sock'
        self.real = shutil.which('tmux', path='/usr/bin:/bin')
        if not self.real:
            self.skipTest('tmux required for command-window checks')
        self.log = self.root/'calls'
        self.script('tmux', '#!/bin/sh\nexec '+self.real+' -S '+shlex.quote(str(self.socket))+' "$@"\n')
        self.script('systemd-run', """#!/usr/bin/python3
import json,os,sys
with open(os.environ['TEST_LOG'],'a') as f: f.write(json.dumps({'args':sys.argv[1:],'env':dict(os.environ)})+'\\n')
args=sys.argv[sys.argv.index('--')+1:]
os.execvp(args[0],args)
""")
        self.script('systemctl', '#!/usr/bin/python3\nimport os\nprint("PATH="+os.environ["PATH"])\n')
        self.env = patch.dict(os.environ, PATH=str(self.bin)+':'+os.environ['PATH'], TEST_LOG=str(self.log),
                              AGENT_BUS_NAME='#tmux-sessions/billing@demo', AGENT_BUS_TOKEN='private-sentinel',
                              AGENT_BUS_CONFIG='private-config', AGENT_BUS_ADDR='private-address', AB_ROLE_OWNER='1')
        self.env.start(); self.addCleanup(self.env.stop)
        self.addCleanup(self.clean_tmux)
        self.wait = patch.object(agent,'READY_WAIT',.05); self.wait.start(); self.addCleanup(self.wait.stop)
        self.grace = patch.object(agent,'STOP_GRACE',.5); self.grace.start(); self.addCleanup(self.grace.stop)
        self.script('fixture', """#!/usr/bin/python3
import json,os,signal,sys,time
from pathlib import Path
root=Path("""+repr(str(self.root))+""")
def stop(sig,frame):
 (root/('term-'+sys.argv[1])).touch()
 sys.exit(0)
signal.signal(signal.SIGTERM,stop)
(root/('env-'+sys.argv[1]+'.json')).write_text(json.dumps({'env':dict(os.environ),'args':sys.argv[1:],'pid':os.getpid(),'cgroup':Path('/proc/self/cgroup').read_text()}))
(root/('ready-'+sys.argv[1])).touch()
while True: time.sleep(.1)
""")

    def script(self, name, code):
        path=self.bin/name;path.write_text(code);path.chmod(0o755)

    def tmux(self, *args):
        return subprocess.run([self.real,'-S',str(self.socket),*args],capture_output=True,text=True,timeout=5)

    def clean_tmux(self):
        self.tmux('kill-server')
        until=time.monotonic()+3
        while True:
            with socket.socket(socket.AF_UNIX) as connection:
                live=connection.connect_ex(str(self.socket))==0
            if not live:
                self.socket.unlink(missing_ok=True)
                break
            if time.monotonic()>until:
                self.fail('test tmux server did not exit; socket was not removed')
            time.sleep(.01)
        self.assertFalse(self.socket.exists())

    def wait_marker(self, name):
        until = time.monotonic() + 3
        while not (self.root/name).exists():
            if time.monotonic() >= until:
                self.fail('stub did not publish ' + name)
            time.sleep(.01)

    def row(self, name='Main', **updates):
        return {'name':name,'cwd':str(self.root),'command':'exec fixture '+name,**updates}

    def config(self, rows=None, **updates):
        return agent.configuration(json.dumps({'session':'work','windows':rows if rows is not None else [self.row()],**updates}))

    def test_new_shape_default_session_and_strict_validation(self):
        self.assertEqual(agent.configuration('{}'),{'session':'billing','windows':[]})
        row=self.row()
        bad=[{'sessions':[]},{'tag':'old'},{'session':'bad name'},{'session':'x'*33},{'windows':'bad'}]
        for changes in [{'runtime':'claude'},{'args':[]},{'tags':[]},{'window':'old'}, {'name':'bad.name'},
                        {'name':''},{'name':'x'*33},{'cwd':'relative'},{'command':[]},{'command':''},
                        {'command':'x\ny'},{'command':'x\0y'},{'command':'é'*2049}]:
            bad.append({'windows':[{**row,**changes}]})
        bad.extend([{'windows':[row,row]},{'windows':[self.row(str(i)) for i in range(33)]}])
        for update in bad:
            with self.subTest(update=update),self.assertRaises(ValueError):self.config(**update)
        self.assertEqual(self.config([self.row('aZ_09-')])['windows'][0]['name'],'aZ_09-')

    def test_environment_forms_bounds_and_forbidden_values(self):
        text=" # comment\nA='a # b'\nB=\"two words\"\nPATH=/custom/bin\n"
        obj={'A':'a # b','B':'two words','PATH':'/custom/bin'}
        self.assertEqual(agent.environment(text),agent.environment(obj))
        config=self.config([self.row('One',env=text,command='exec '+shlex.quote(str(self.bin/'fixture'))+' One'),self.row('Two',env=obj,command='exec '+shlex.quote(str(self.bin/'fixture'))+' Two')])
        self.assertIn('Two: started',agent.start(config))
        first=json.loads((self.root/'env-One.json').read_text())['env']
        second=json.loads((self.root/'env-Two.json').read_text())['env']
        self.assertEqual(first,second)
        for env in [{'AGENT_BUS_TOKEN':'x'},{'AB_ROLE_OWNER':'1'},{'A':'x\0y'},{'A':'é'*2049},
                    {'bad-key':'x'},{'A':2},[],3,{f'A{i}':'x' for i in range(65)}]:
            with self.subTest(env=env),self.assertRaises(ValueError):agent.environment(env)
        with self.assertRaisesRegex(ValueError,'line 2'):
            agent.environment('A=1\nPRIVATE_SENTINEL')

    def test_dist_loads_through_real_parser(self):
        # Example paths are portable placeholders. Stub only their existence;
        # every key, bound and command still passes through the real parser.
        with patch.object(agent.Path,'is_dir',lambda p:str(p) in ('/home/me/work','/home/me/work/api')):
            config=agent.configuration((HERE/'config.json.dist').read_text())
        self.assertEqual(config['session'],'work')
        self.assertEqual(len(config['windows']),4)
        self.assertEqual(config['windows'][1]['env']['AGENT_BUS_NAME'],'#codex/work-api')
        self.assertEqual(config['windows'][2]['env'],{'A':'1','B':'2'})

    def test_shell_quoting_home_env_stripping_and_manager_path(self):
        person_path=str(self.bin)+':/only-the-persons-bin:/usr/bin'
        self.script('systemctl','#!/usr/bin/python3\nprint('+repr('PATH='+person_path)+')\n')
        row=self.row(command='exec fixture Main "two words" "$HOME"',env={'VISIBLE':'yes','AGENT_BUS_NAME':'#worker/billing'})
        self.assertEqual(agent.start(self.config([row])),'Main: started')
        saved=json.loads((self.root/'env-Main.json').read_text())
        self.assertEqual(saved['args'],['Main','two words',str(Path.home())])
        self.assertEqual(saved['env']['PATH'],person_path)
        self.assertEqual(saved['env']['VISIBLE'],'yes')
        self.assertEqual({k:v for k,v in saved['env'].items() if k.startswith(('AGENT_BUS_','AB_ROLE_'))},{'AGENT_BUS_NAME':'#worker/billing'})
        self.assertNotIn('private-sentinel',json.dumps(saved))
        self.assertEqual((self.root/'env-Main.json').stat().st_mode&0o777,0o644)
        override=self.config([self.row(env={'PATH':'/custom/bin'})])['windows'][0]
        self.assertIn('PATH=/custom/bin',agent.window_argv(override,person_path))

    def test_state_home_keeps_the_old_units_state(self):
        legacy=self.root/'var'; legacy.mkdir()
        old=legacy/'agent-bus-tmux-sessions-billing'; old.mkdir()
        bare=legacy/'agent-bus-tmux-sessions'; bare.mkdir()
        mine=self.root/'mine'; mine.mkdir()
        with patch.object(agent,'LEGACY_STATE',legacy),patch.dict(os.environ,XDG_STATE_HOME=str(self.root/'inherited')):
            # the named instance, and the bare agent, each find their own unit's directory
            self.assertEqual(self.config()['windows'][0].get('state_home'),str(old))
            with patch.dict(os.environ,AGENT_BUS_NAME='#tmux-sessions@demo'):
                self.assertEqual(self.config()['windows'][0].get('state_home'),str(bare))
            for name in ['#other/billing@demo','#tmux-sessions/a/b@demo','#tmux-sessions/..@demo']:
                with self.subTest(name=name),patch.dict(os.environ,AGENT_BUS_NAME=name):
                    self.assertIsNone(self.config()['windows'][0].get('state_home'))
            # a configured state_home wins over the legacy default
            self.assertEqual(self.config(state_home=str(mine))['windows'][0].get('state_home'),str(mine))
            for bad in ['relative',str(self.root/'missing'),3,'/x\0y']:
                with self.subTest(bad=bad),self.assertRaisesRegex(ValueError,'state_home'):self.config(state_home=bad)
            # the window gets it over the inherited value; its own env still overrides
            self.assertEqual(agent.start(self.config([self.row('One'),self.row('Two',env={'XDG_STATE_HOME':'/own'})])),'One: started\nTwo: started')
            self.assertEqual(json.loads((self.root/'env-One.json').read_text())['env'].get('XDG_STATE_HOME'),str(old))
            self.assertEqual(json.loads((self.root/'env-Two.json').read_text())['env'].get('XDG_STATE_HOME'),'/own')
            # not a real directory owned by this uid: no default, the inherited value stays
            old.rmdir(); old.symlink_to(mine)
            self.assertIsNone(self.config()['windows'][0].get('state_home'))
            old.unlink(); old.mkdir()
            with patch.object(agent.os,'getuid',lambda:os.geteuid()+1):
                self.assertIsNone(agent.legacy_state('#tmux-sessions/billing'))
            self.assertEqual(agent.legacy_state('#tmux-sessions/billing'),str(old))
            self.assertIn('XDG_STATE_HOME='+str(self.root/'inherited'),agent.window_argv(self.row(env={}),'/bin'))

    def test_one_session_idempotent_missing_windows_and_scope(self):
        config=self.config([self.row('First'),self.row('Second')])
        self.assertEqual(agent.start(config,['First']),'First: started')
        first=self.tmux('list-windows','-t','=work','-F','#{window_id} #{window_name}').stdout
        self.assertEqual(agent.start(config),'First: already running\nSecond: started')
        self.assertIn(first.strip(),self.tmux('list-windows','-t','=work','-F','#{window_id} #{window_name}').stdout)
        self.assertEqual(self.tmux('list-sessions','-F','#{session_name}').stdout.strip(),'work')
        self.assertEqual(agent.start(config),'First: already running\nSecond: already running')
        wrappers=[json.loads(line) for line in self.log.read_text().splitlines()]
        self.assertEqual(len(wrappers),1)
        self.assertEqual(wrappers[0]['args'][:5],['--user','--scope','--quiet','--collect','--unit=ab-session-work'])
        self.assertNotIn('private-sentinel',json.dumps(wrappers))

    def test_immediate_exit_failure_preserves_sibling_and_allows_retry(self):
        config=self.config([self.row('Good'),self.row('Bad',command="printf 'fixture command failed'; exit 7")])
        self.assertEqual(agent.start(config,['Good']),'Good: started')
        result=agent.start(config,['Bad'])
        self.assertIn('Bad: failed: command exited:',result)
        self.assertIn('fixture command failed',result)
        self.assertEqual(self.tmux('list-windows','-t','=work','-F','#{window_name}').stdout.strip(),'Good')
        config['windows'][1]['command']='exec fixture Bad'
        self.assertEqual(agent.start(config,['Bad']),'Bad: started')
        # A failed first/only window removes its empty session, too.
        failed=self.config([self.row('Bad',command='exit 7')],session='empty')
        self.assertIn(': failed:',agent.start(failed))
        self.assertEqual(self.tmux('has-session','-t','=empty').returncode,1)

    def test_methods_status_and_stop_never_touch_unmanaged_windows(self):
        # Keep the pane's shell alive until its background child handles TERM;
        # otherwise closing the PTY can HUP the child before it writes the marker.
        config=self.config([self.row('Main',command="trap 'wait; exit' TERM; fixture Main & wait"),self.row('Other')])
        self.assertEqual(agent.status(config),'failed: session work absent')
        self.assertEqual(agent.request('start Main',config),'Main: started')
        self.assertEqual(agent.status(config),'degraded: 1/2 windows up; missing Other')
        self.tmux('new-window','-d','-t','=work:','-n','Unmanaged','sleep','60')
        listing=agent.request('list',config)
        self.assertIn('"Main": present; pane alive=yes; pid=',listing)
        self.assertIn('uptime=',listing);self.assertIn('cwd=',listing)
        self.assertIn('"Unmanaged": unmanaged;',listing)
        for method in ('start','stop','restart'):
            with self.subTest(method=method),self.assertRaisesRegex(ValueError,'valid windows: Main, Other'):
                agent.request(method+' Main missing',config)
        self.wait_marker('ready-Main')
        self.assertEqual(agent.request('stop Main',config),'Main: stopped')
        self.wait_marker('term-Main')
        self.assertEqual(self.tmux('list-windows','-t','=work','-F','#{window_name}').stdout.strip(),'Unmanaged')
        self.assertTrue(agent.status(config).startswith('failed: 0/2'))
        self.assertIn('Main: started',agent.request('restart Main',config))
        self.assertEqual(agent.start(config,['Other']),'Other: started')
        self.assertEqual(agent.status(config),'ok: 2/2 windows up')
        output=io.StringIO()
        with patch.dict(os.environ,{'AGENT_BUS_ASK':'status','AGENT_BUS_CONFIG':json.dumps(config),'AB_ROLE_OWNER':''}), patch.object(agent.sys,'argv',['agent','stop Main']),redirect_stdout(output):
            agent.main()
        self.assertEqual(output.getvalue().strip(),'ok: 2/2 windows up')
        self.assertEqual(agent.status(config),'ok: 2/2 windows up')
        self.assertIn('Other: stopped',agent.request('{"method":"stop","windows":["Other"]}',config))
        self.assertEqual(self.tmux('has-session','-t','=work').returncode,0)

    def test_stop_last_window_ends_session_and_socket_cleanup(self):
        config=self.config()
        self.assertEqual(agent.start(config),'Main: started')
        self.assertEqual(agent.stop(config),'Main: stopped')
        self.assertEqual(self.tmux('has-session','-t','=work').returncode,1)
        self.assertEqual(agent.stop(config),'Main: already stopped')
        for body in ('{\"method\":[]}', '{\"method\":\"start\",\"windows\":3}', 'stop Main unknown'):
            with self.subTest(body=body),self.assertRaises(ValueError):agent.request(body,config)
        self.clean_tmux();self.assertFalse(self.socket.exists())

    def test_list_and_status_see_a_dead_pane(self):
        config=self.config()
        self.assertEqual(agent.start(config),'Main: started')
        pane=agent.session_windows(config)[0]['panes'][0]
        agent.signal_window(pane)
        until=time.monotonic()+2
        while time.monotonic()<until and agent.pane_info(pane['id'])[0]['alive']:time.sleep(.01)
        self.assertIn('pane alive=no',agent.list_windows(config))
        self.assertEqual(agent.status(config),'failed: 0/1 windows up; not running Main')

    @unittest.skipUnless(os.environ.get('AB_SESSIONS_SYSTEMD_CHECK')=='1','real service restart belongs to slow')
    def test_real_user_scope_survives_starter_service_restart(self):
        suffix=str(os.getpid())+'-'+self.root.name[-6:]
        session,unit='check-'+suffix,'agent-bus-windows-check-'+suffix
        self.script('systemd-run','#!/bin/sh\nexec /usr/bin/systemd-run "$@"\n')
        started=self.root/'started';parent=self.root/'parent.py'
        config={'session':session,'windows':[self.row()]}
        parent.write_text('import sys,json,time\nfrom pathlib import Path\nsys.path.insert(0,'+repr(str(HERE))+')\nimport sessions\nsessions.READY_WAIT=.05\nPath('+repr(str(started))+').write_text(sessions.start(sessions.configuration('+repr(json.dumps(config))+')))\ntime.sleep(60)\n')
        self.addCleanup(lambda:subprocess.run(['systemctl','--user','stop','ab-session-'+session+'.scope'],capture_output=True))
        self.addCleanup(lambda:subprocess.run(['sudo','-n','systemctl','stop',unit],capture_output=True))
        result=subprocess.run(['sudo','-n','systemd-run','--quiet','--collect','--unit='+unit,'--uid='+str(os.getuid()),'--property=UMask=0077','--property=KillMode=control-group',
                               '--setenv=PATH='+os.environ['PATH'],'--setenv=TEST_LOG='+str(self.log),'--','/usr/bin/python3',str(parent)],capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stderr)
        until=time.monotonic()+5
        while time.monotonic()<until and not started.exists():time.sleep(.02)
        self.assertTrue(started.exists())
        saved=json.loads((self.root/'env-Main.json').read_text())
        self.assertIn('/user.slice/',saved['cgroup']);self.assertNotIn(unit,saved['cgroup'])
        restarted=subprocess.run(['sudo','-n','systemctl','restart',unit],capture_output=True,text=True)
        self.assertEqual(restarted.returncode,0,restarted.stderr)
        self.assertTrue(Path('/proc',str(saved['pid'])).exists())
        self.assertEqual(self.tmux('has-session','-t','='+session).returncode,0)
