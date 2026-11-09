import re,sys,subprocess
files=subprocess.run(['git','diff','--name-only','--diff-filter=U'],capture_output=True,text=True).stdout.split()
pat=re.compile(r'<<<<<<< [^\n]*\n(.*?)(?:\|\|\|\|\|\|\| [^\n]*\n.*?)?=======\n(.*?)>>>>>>> [^\n]*\n',re.S)
for f in files:
    s=open(f).read()
    s=pat.sub(lambda m: m.group(1)+('' if m.group(1).endswith('\n\n') or not m.group(1) else '\n')+m.group(2),s)
    assert not re.search(r'^(<<<<<<<|=======$|>>>>>>>|\|\|\|\|\|\|\|)',s,re.M),f
    open(f,'w').write(s); print('resolved',f)
