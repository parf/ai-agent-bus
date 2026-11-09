import subprocess,os,shutil
base=open('gate.sh.new').read()
M=[('docs path never taken','  if ((docs_only)); then\n    check="docs check"','  if false; then\n    check="docs check"'),
   ('any .md is docs',"    [[ $f =~ ^Plans/.+\\.md$ || ( $f =~ ^[^/]+\\.md$ && $f != LICENSE.md ) ]]","    [[ $f == *.md ]]"),
   ('docs/ pages are docs again',"    [[ $f =~ ^Plans/.+\\.md$ ||","    [[ $f =~ ^(Plans|docs)/.+\\.md$ ||"),
   ('LICENSE counts as docs',"$f != LICENSE.md","$f != NOTLICENSE.md"),
   ('selector answer ignored',"<<<\"$sel\" || docs_only=0","<<<\"$sel\" || true"),
   ('selector error ignored','--out "$log.select" 2>&1) || docs_only=0','--out "$log.select" 2>&1) || true'),
   ('normal selection asked','smoke-select.py --mode fast','smoke-select.py --mode normal'),
   ('markers not checked','python3 src/check-links.py && python3 src/check-markers.py}','python3 src/check-links.py}'),
   ('docs tests skipped','bash -c "$DOCS_CHECK" && bash -c "$DOCS_TESTS")','bash -c "$DOCS_CHECK")'),
   ('red docs check lands','    ((rc == 0)) || receipt red "docs-only check','    true || receipt red "docs-only check')]
os.makedirs('mut',exist_ok=True);shutil.copy('gate-lock.sh','mut/gate-lock.sh')
for name,a,b in M:
    assert base.count(a)==1,name
    open('mut/gate.sh','w').write(base.replace(a,b))
    r=subprocess.run(['bash','gate.test.sh',os.path.abspath('mut')],capture_output=True,text=True)
    fails=[l for l in r.stdout.splitlines() if l.startswith('FAIL')]
    print(f'{name}: rc={r.returncode} fails={len(fails)}',fails[:2])
