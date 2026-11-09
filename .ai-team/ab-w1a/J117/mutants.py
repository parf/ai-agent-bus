# J117 mutants of gate.sh.new: each must make gate.test.sh fail on an assertion.
import subprocess, os, shutil
env = {k: v for k, v in os.environ.items() if k not in ('AGENT_BUS_SESSION_FILE', 'AGENT_BUS_TOKEN')}
env['TREE'] = '/usr/local/src/ai-agent-bus'
M = [('agent answer never read', "grep -qx '  agent pack only: checks that own no changed path are skipped'", "grep -qx 'never'"),
     ('selector exit ignored', '--out "$log.normal" 2>&1) \\\n      && grep', '--out "$log.normal" 2>&1; true) \\\n      && grep'),
     ('warm-up never skipped', '((agents_only)) || (cd', '((0)) || (cd'),
     ('skipped checks named', '''$2 != "skip" { print $1 }''', '''{ print $1 }''')]
base = open('gate.sh.new').read()
for name, a, b in M:
    assert base.count(a) == 1, name
    os.makedirs('mut', exist_ok=True); shutil.copy('gate-lock.sh', 'mut/gate-lock.sh')
    open('mut/gate.sh.new', 'w').write(base.replace(a, b))
    r = subprocess.run(['bash', 'gate.test.sh', os.path.abspath('mut')], capture_output=True, text=True, env=env, timeout=250)
    fails = [l for l in r.stdout.splitlines() if l.startswith('FAIL')]
    print(f'{name}: rc={r.returncode} fails={len(fails)}', [f[:120] for f in fails[:2]])
shutil.rmtree('mut')
