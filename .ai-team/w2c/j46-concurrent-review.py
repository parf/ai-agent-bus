from pathlib import Path
import os,sys,tempfile,subprocess,shutil
root=Path.cwd()
with tempfile.TemporaryDirectory(dir=root/'tmp') as d:
 p=Path(d);tools=p/'bin';tools.mkdir();marks=p/'marks';marks.mkdir()
 for name,real,arg in [('mv',shutil.which('mv'),'$1'),('python3',sys.executable,'$3')]:
  (tools/name).write_text('''#!/bin/sh
source='''+arg+'''
case "$source" in *.new.*)
 if [ "$LABEL" = A ]; then
  i=0; while [ ! -e "$MARKS/B-ready" ]; do i=$((i+1)); [ "$i" -lt 200 ] || exit 91; sleep .01; done
  "'''+real+'''" "$@"; rc=$?; touch "$MARKS/A-done"; exit "$rc"
 else
  touch "$MARKS/B-ready"
  i=0; while [ ! -e "$MARKS/A-done" ]; do i=$((i+1)); [ "$i" -lt 200 ] || exit 92; sleep .01; done
 fi ;;
esac
exec "'''+real+'''" "$@"
''');(tools/name).chmod(0o755)
 script='set -eu; . "$HELPER"; package_begin "$TARGET"; echo "$LABEL" > "$dest/code"; package_commit'
 procs=[]
 for label in ['A','B']:
  env=dict(os.environ,PATH=str(tools)+':'+os.environ['PATH'],HELPER=str(root/'src/agents/package-tree.sh'),TARGET=str(p/'agent'),LABEL=label,MARKS=str(marks))
  procs.append(subprocess.Popen(['/bin/sh','-c',script],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True))
 result=[]
 for q in procs:
  out,err=q.communicate(timeout=5);result.append((q.returncode,err))
 assert result[0][0]==0 and result[1][0]!=0,result
 assert 'FileExistsError' in result[1][1] or 'Directory not empty' in result[1][1],result
 files=sorted(str(x.relative_to(p/'agent')) for x in (p/'agent').rglob('*'))
 assert files==['code'] and (p/'agent/code').read_text()=='A\n',files
 assert not list(p.glob('agent.new.*')) and not list(p.glob('agent.old.*'))
 print(f'A rc={result[0][0]}, B rc={result[1][0]}; B loudly refused: '+result[1][1].splitlines()[-1]+'; target contains only A code; no nesting or temporary remnants')
