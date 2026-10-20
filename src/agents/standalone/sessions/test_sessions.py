import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
spec = importlib.util.spec_from_file_location('sessions_agent', HERE / 'sessions.py')
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)


class Sessions(unittest.TestCase):
    def setUp(self):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=ROOT / 'tmp', prefix='sessions-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / 'bin'; self.bin.mkdir()
        self.log = self.root / 'calls'
        self.script('tmux', '''#!/usr/bin/python3
import json,os,sys
with open(os.environ['TEST_LOG'],'a') as f: f.write(json.dumps({'args':sys.argv[1:],'env':dict(os.environ)})+'\\n')
if sys.argv[1]=='has-session': sys.exit(0 if sys.argv[-1]=='=claude-existing' else 1)
''')
        self.script('systemd-run', """#!/usr/bin/python3
import json,os,sys
with open(os.environ['TEST_LOG'],'a') as f: f.write(json.dumps({'manager':sys.argv[1:],'env':dict(os.environ)})+'\\n')
args=sys.argv[sys.argv.index('--')+1:]
os.execvp(args[0],args)
""")
        self.script('agent-bus', '#!/usr/bin/python3\nprint(\'[{"name":"#codex/live","readers":1}]\')\n')
        self.env = patch.dict(os.environ, PATH=str(self.bin)+':'+os.environ['PATH'], TEST_LOG=str(self.log), AGENT_BUS_TOKEN='private-sentinel', AGENT_BUS_CONFIG='private-config', AGENT_BUS_ADDR='private-address', AB_ROLE_OWNER='1')
        self.env.start(); self.addCleanup(self.env.stop)

    def script(self, name, code):
        path = self.bin / name; path.write_text(code); path.chmod(0o755)

    def config(self, sessions=None):
        return {'tag':'Team Dev', 'sessions': sessions if sessions is not None else [{'name':'#claude/main','runtime':'claude','cwd':str(self.root),'args':['hello ; touch NOPE'],'tags':['review']} ]}

    def test_strict_configuration_and_sanitized_collisions(self):
        parsed = agent.configuration(json.dumps(self.config()))
        self.assertEqual(parsed['tag'], 'team dev')
        self.assertEqual(agent.tmux_name('#claude/main@host.us'), 'claude-main-host-us')
        for update in [{'unknown':1}, {'sessions':'bad'}, {'tag':'123'}, {'sessions':[{'name':'#bad','runtime':'shell','cwd':str(self.root)}]}, {'sessions':[{'name':'#bad','runtime':'claude','cwd':'relative'}]}, {'sessions':[{'name':'#bad','runtime':'claude','cwd':str(self.root),'args':[3]}]}]:
            with self.subTest(update=update), self.assertRaises(ValueError): agent.configuration(json.dumps(self.config() | update))
        rows = [{'name': name, 'runtime':'claude','cwd':str(self.root)} for name in ['#claude/main','#claude-main']]
        with self.assertRaisesRegex(ValueError,'tmux'): agent.configuration(json.dumps(self.config(rows)))

    def test_fixed_argv_and_no_agent_environment(self):
        result = agent.start(agent.configuration(json.dumps(self.config())))
        self.assertIn('#claude/main: started',result)
        calls = [json.loads(line) for line in self.log.read_text().splitlines() if 'args' in json.loads(line)]
        create = next(row for row in calls if row['args'][0]=='new-session')
        args = create['args']
        self.assertEqual(args[:7], ['new-session','-d','-s','claude-main','-c',str(self.root),'--'])
        self.assertIn('AGENT_BUS_NAME=#claude/main',args)
        self.assertEqual(args[args.index('ab-claude'):], ['ab-claude','--restart','--tag','review,team dev','hello ; touch NOPE'])
        self.assertFalse(any(key.startswith(('AGENT_BUS_','AB_ROLE_')) for key in create['env']))
        child = args[args.index('-i')+1:args.index(agent.sys.executable)]
        self.assertEqual([item for item in child if item.startswith(('AGENT_BUS_','AB_ROLE_'))], ['AGENT_BUS_NAME=#claude/main'])
        self.assertNotIn('private-sentinel',str(create)); self.assertNotIn('private-config',str(create))

    def test_user_scope_and_normal_umask(self):
        self.assertIn(': started', agent.start(agent.configuration(json.dumps(self.config()))))
        wrappers = [json.loads(line) for line in self.log.read_text().splitlines() if 'manager' in json.loads(line)]
        self.assertEqual(len(wrappers), 1, 'tmux startup bypasses the user manager')
        call = wrappers[0]
        self.assertEqual(call['manager'][:5], ['--user', '--scope', '--quiet', '--collect', '--unit=ab-session-claude-main'])
        self.assertEqual(call['manager'][5:8], ['--', 'tmux', 'new-session'])
        self.assertEqual(call['env']['XDG_RUNTIME_DIR'], '/run/user/' + str(os.getuid()))
        self.assertEqual(call['env']['DBUS_SESSION_BUS_ADDRESS'], 'unix:path=/run/user/' + str(os.getuid()) + '/bus')
        self.assertNotIn('private-sentinel', str(call))

    def test_existing_tmux_and_live_bus_fail_but_others_start(self):
        rows=[{'name':name,'runtime':runtime,'cwd':str(self.root)} for name,runtime in [('#claude/existing','claude'),('#codex/live','codex'),('#opencode/new','opencode')]]
        result=agent.start(agent.configuration(json.dumps(self.config(rows))))
        self.assertIn('#claude/existing: failed: tmux session already exists',result)
        self.assertIn('#codex/live: failed: bus name is already live',result)
        self.assertIn('#opencode/new: started',result)
        calls=[json.loads(line)['args'] for line in self.log.read_text().splitlines() if 'args' in json.loads(line)]
        self.assertEqual(sum(args[0]=='new-session' for args in calls),1)

    def test_installer_requires_a_non_root_person(self):
        for args in [[], ['--user', 'root']]:
            result = subprocess.run([str(HERE / 'install.py'), 'install', '--name', '#sessions@test', *args], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('non-root person', result.stderr)

    @unittest.skipUnless(shutil.which('tmux'), 'tmux is required for the real sleeping-stub check')
    def test_real_tmux_runs_only_a_sleeping_stub(self):
        real=shutil.which('tmux', path='/usr/bin:/bin')
        socket='ab-sessions-'+str(os.getpid())+'-'+self.root.name
        self.script('tmux', '#!/bin/sh\nexec '+real+' -L '+socket+' "$@"\n')
        self.script('ab-claude', '#!/usr/bin/python3\nimport os,json,time\nfrom pathlib import Path\nPath('+repr(str(self.root/'stub'))+').write_text(json.dumps(dict(os.environ)))\ntime.sleep(60)\n')
        self.addCleanup(lambda: subprocess.run([real,'-L',socket,'kill-server'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL))
        subprocess.run([real, '-L', socket, 'new-session', '-d', '-s', 'bootstrap', 'sleep', '60'], check=True, umask=0o077)
        result=agent.start(agent.configuration(json.dumps(self.config())))
        self.assertIn(': started',result)
        import time
        until=time.monotonic()+3
        while not (self.root/'stub').exists() and time.monotonic()<until: time.sleep(.01)
        self.assertTrue((self.root/'stub').exists())
        self.assertEqual((self.root/'stub').stat().st_mode & 0o777, 0o644, 'session inherited the server umask')
        env=json.loads((self.root/'stub').read_text())
        self.assertEqual({k:v for k,v in env.items() if k.startswith(('AGENT_BUS_','AB_ROLE_'))},{'AGENT_BUS_NAME':'#claude/main'})
        self.assertIn('already exists',agent.start(agent.configuration(json.dumps(self.config()))))

    @unittest.skipUnless(os.environ.get('AB_SESSIONS_SYSTEMD_CHECK') == '1', 'real service restart belongs to slow')
    def test_real_user_scope_survives_starter_service_restart(self):
        import time
        real = shutil.which('tmux', path='/usr/bin:/bin')
        self.assertIsNotNone(real)
        suffix = str(os.getpid()) + '-' + self.root.name
        socket, unit = 'ab-sessions-' + suffix, 'agent-bus-sessions-check-' + suffix
        session = 'claude-check-' + suffix
        scope = 'ab-session-' + session + '.scope'
        self.script('tmux', '#!/bin/sh\nexec ' + real + ' -L ' + socket + ' "$@"\n')
        self.script('systemd-run', '#!/bin/sh\nexec /usr/bin/systemd-run "$@"\n')
        marker, started = self.root/'stub-state', self.root/'started'
        self.script('ab-claude', '#!/usr/bin/python3\nimport os,json,time\nfrom pathlib import Path\nPath(' + repr(str(marker)) + ').write_text(json.dumps({"pid":os.getpid(),"cgroup":Path("/proc/self/cgroup").read_text()}))\ntime.sleep(60)\n')
        row = {'name':'#claude/check-' + suffix, 'runtime':'claude', 'cwd':str(self.root)}
        parent = self.root/'starter.py'
        parent.write_text('import sys,json,time\nfrom pathlib import Path\nsys.path.insert(0,' + repr(str(HERE)) + ')\nimport sessions\nPath(' + repr(str(started)) + ').write_text(sessions.start(sessions.configuration(' + repr(json.dumps({'sessions':[row]})) + ')))\ntime.sleep(60)\n')
        def run(*args):
            return subprocess.run(args, capture_output=True, text=True, timeout=10)
        self.addCleanup(lambda: run(real, '-L', socket, 'kill-server'))
        self.addCleanup(lambda: run('/usr/bin/systemctl', '--user', 'stop', scope))
        self.addCleanup(lambda: run('sudo', '-n', 'systemctl', 'stop', unit))
        launched = run('sudo', '-n', 'systemd-run', '--quiet', '--collect', '--unit='+unit,
                       '--uid='+str(os.getuid()), '--property=UMask=0077', '--property=KillMode=control-group',
                       '--setenv=PATH='+os.environ['PATH'], '--setenv=TEST_LOG='+str(self.log),
                       '--', '/usr/bin/python3', str(parent))
        self.assertEqual(launched.returncode, 0, launched.stderr)
        until = time.monotonic()+5
        while time.monotonic()<until and not marker.exists(): time.sleep(.02)
        self.assertTrue(marker.exists(), started.read_text() if started.exists() else 'starter did not answer')
        state = json.loads(marker.read_text())
        self.assertIn('/user.slice/', state['cgroup'])
        self.assertNotIn(unit, state['cgroup'])
        self.assertEqual(marker.stat().st_mode & 0o777, 0o644)
        restarted = run('sudo', '-n', 'systemctl', 'restart', unit)
        self.assertEqual(restarted.returncode, 0, restarted.stderr)
        self.assertTrue(Path('/proc', str(state['pid'])).exists(), 'starter restart killed the session')
        self.assertNotEqual(Path('/proc', str(state['pid']), 'stat').read_text().split()[2], 'Z')
        self.assertEqual(run(real, '-L', socket, 'has-session', '-t', '='+session).returncode, 0)
