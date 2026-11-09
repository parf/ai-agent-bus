from pathlib import Path
import tempfile,subprocess,os
root=Path.cwd()
with tempfile.TemporaryDirectory(dir=root/'tmp') as d:
 p=Path(d);tools=p/'bin';tools.mkdir();marks=p/'marks';marks.mkdir()
 (tools/'mv').write_text('''#!/bin/sh
case "$1" in *.new.*)
 if [ "$LABEL" = A ]; then
  i=0; while [ ! -e "$MARKS/B-ready" ]; do i=$((i+1)); [ "$i" -lt 200 ] || exit 91; sleep .01; done
  /usr/bin/mv "$@" || exit $?
  touch "$MARKS/A-done"
  exit 0
 else
  touch "$MARKS/B-ready"
  i=0; while [ ! -e "$MARKS/A-done" ]; do i=$((i+1)); [ "$i" -lt 200 ] || exit 92; sleep .01; done
 fi ;;
esac
exec /usr/bin/mv "$@"
''');(tools/'mv').chmod(0o755)
 script='set -eu; . "$HELPER"; package_begin "$TARGET"; echo "$LABEL" > "$dest/code"; package_commit'
 procs=[]
 for label in ['A','B']:
  env=dict(os.environ,PATH=str(tools)+':'+os.environ['PATH'],HELPER=str(root/'src/agents/package-tree.sh'),TARGET=str(p/'agent'),LABEL=label,MARKS=str(marks))
  procs.append(subprocess.Popen(['/bin/sh','-c',script],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True))
 for q in procs:
  out,err=q.communicate(timeout=5);assert q.returncode==0,(q.returncode,out,err)
 files=sorted(str(x.relative_to(p/'agent')) for x in (p/'agent').rglob('*'))
 assert files==['code'] and (p/'agent/code').read_text()=='B\n',f'both packagers returned0, target={files}, root code={(p/"agent/code").read_text()!r}; expected only final B code'
