from pathlib import Path
import subprocess
base=Path('/usr/local/src/ai-agent-bus/tmp')
for task,args,want in [
 ('J46',['python3','-B','src/agents/package_tree_test.py','PackageTrees.test_every_runner_package_replaces_its_tree_exactly'],'deploying over an old tree retained an unshipped file'),
 ('J49',['python3','-B','src/agents/standalone/tmux-sessions/test_package.py','Package.test_shipped_repackager_replaces_tree_from_release'],'shipped repackager retained stale files')]:
 root=base/('ab-w2c-'+task+'-read');p=root/'src/agents/package-tree.sh';src=p.read_text()
 offset=src.index('package_commit() {')
 mutant=src[:offset]+'''package_commit() {
 mkdir -p "$package_target"
 cp -a "$package_stage/." "$package_target/"
 rm -rf "$package_stage"
 package_stage=
 trap - EXIT HUP INT TERM
}
'''
 try:
  p.write_text(mutant)
  syntax=subprocess.run(['sh','-n',str(p)],capture_output=True,text=True);assert syntax.returncode==0,syntax.stderr
  r=subprocess.run(args,cwd=root,capture_output=True,text=True,timeout=40);out=r.stdout+r.stderr
  assert r.returncode!=0 and want in out and 'AssertionError:' in out and 'FAILED (failures=' in out,(task,r.returncode,out)
  print(task+': '+next(l.strip() for l in out.splitlines() if 'AssertionError:' in l)[:180]+'; '+next(l.strip() for l in out.splitlines() if l.startswith('FAILED (failures=')),flush=True)
 finally:p.write_text(src)
