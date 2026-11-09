from pathlib import Path
import subprocess, time
root=Path('/usr/local/src/ai-agent-bus/tmp/ab-w1c-J80')
p=root/'src/launchers/runtime-auth.test.ts'
fixed=p.read_text()
old=subprocess.check_output(['git','show','main:src/launchers/runtime-auth.test.ts'],cwd=root,text=True)
load=[]
def run(label,source,expect):
 p.write_text(source)
 r=subprocess.run(['bun','test','src/launchers/runtime-auth.test.ts','--test-name-pattern','foreign listeners|a port collision'],cwd=root,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=40)
 print(label,'exit',r.returncode,flush=True)
 print(r.stdout,flush=True)
 assert (r.returncode==0)==expect,label
 return r.stdout
try:
 for _ in range(4):
  load.append(subprocess.Popen(['python3','-c','import time\ne=time.monotonic()+160\nwhile time.monotonic()<e: pass'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL))
 # A bounded injected start delay makes the load-dependent deadline deterministic.
 before=run('OLD with 2.6s per cleanup delayed fixture cleanup under load',old.replace('async function stopRuntime(child: ServerChild) {','async function stopRuntime(child: ServerChild) { await Bun.sleep(2600);'),False)
 assert 'timed out after 5000ms' in before
 run('FIXED same delayed fixture cleanup under load',fixed.replace('async function stopRuntime(child: ServerChild) {','async function stopRuntime(child: ServerChild) { await Bun.sleep(2600);'),True)
 for i in range(20):run(f'FIXED load repetition {i+1}/20',fixed,True)
 mutant=run('BOUND SIGNAL OMITTED MUTANT',fixed.replace('const ready = pending.then', 'const ready = new Promise<never>(() => {}); const ignored = pending.then'),False)
 assert 'runtime listener never signalled owned readiness or foreign refusal within 15s' in mutant
 assert 'timed out after' not in mutant
finally:
 p.write_text(fixed)
 for proc in load:
  if proc.poll() is None:proc.terminate()
 for proc in load:proc.wait(timeout=5)
