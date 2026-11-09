import subprocess,sys
base=open('gate-lock.sh').read()
M=[('no pause','    sleep "$GATE_LOCK_PAUSE"','    :'),
   ('every refusal waits',"""    if [[ $out != *'"kind":"busy"'* && $out != *'already hold'* ]]; then""",'    if false; then'),
   ('holder not read','    GATE_LOCK_HOLDER=$(agent-bus holders','    GATE_LOCK_HOLDER=$(: holders')]
for name,a,b in M:
    assert base.count(a)==1,name
    open('mut.sh','w').write(base.replace(a,b))
    r=subprocess.run(['bash','gate-lock.test.sh','./mut.sh' if False else __import__('os').path.abspath('mut.sh')],capture_output=True,text=True)
    print(name,'rc',r.returncode,[l for l in r.stdout.splitlines() if l.startswith('FAIL')])
open('mut.sh','w').write('')
