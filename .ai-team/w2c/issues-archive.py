from pathlib import Path
import subprocess,re,os
root=Path('/usr/local/src/ai-agent-bus/tmp/ab-w2c-issues')
src=root/'Plans/issues/2026-10/2026-10-30.md';dst=root/'Plans/issues/done/2026-10/2026-10-30.md'
fixes={19:'6ece3dee',20:'4a950a75',21:'95d54d30',22:'2b376d97',23:'9fbfad79',24:'6dcbead8',25:'b17568da',27:'a92f2143',28:'1b6900d5',29:'df934c1e',30:'2fb8b3f4',31:'af70c4ad',33:'0a276d9b',34:'636e67be',35:'ae317c20',36:'d4ca8126',37:'3826b87b',38:'5850fd04',39:'f7932fc8',40:'1e3aed64',41:'5228a7ac',42:'362991a6',43:'d8f3db13',44:'01721a5e',45:'2e77d438',46:'623de7e1',47:'caae48b4',49:'dd34f63a',50:'7cb7c8da',51:'76e98244',52:'1ebff584',53:'087f1544',54:'565aafae',56:'156e3744',57:'6db2f0a5',58:'94970509',59:'368635bd',60:'2fce2f67'}
versions={}
for id,c in fixes.items():
 subprocess.run(['git','merge-base','--is-ancestor',c,'main'],cwd=root,check=True)
 versions[id]=subprocess.check_output(['git','show',c+':src/internal/version/VERSION'],cwd=root,text=True).strip()
s=src.read_text();lines=s.splitlines(keepends=True);sections=[];current=[]
for line in lines:
 if line.startswith('## '):
  if current:sections.append(current)
  current=[line]
 else:current.append(line)
if current:sections.append(current)
kept=[];moved=[];found=set()
for sec in sections:
 heading=sec[0] if sec[0].startswith('## ') else ''
 text=''.join(sec[1:] if heading else sec)
 starts=list(re.finditer(r'^- .*?\bJ(\d+)\b[^\n]*',text,re.M));openblocks=[];closedblocks=[]
 prefix=text[:starts[0].start()] if starts else text
 for i,m in enumerate(starts):
  end=starts[i+1].start() if i+1<len(starts) else len(text)
  block=text[m.start():end].rstrip()+'\n\n';id=int(m[1])
  if id in fixes:
   found.add(id);first,rest=block.split('\n',1)
   first=re.sub(r'^- (?:✅ |⏳ |❓ )?', '- ✅ ',first)
   c=fixes[id];v=versions[id]
   extra=''
   if id==58:extra='  - Acceptance follow-up: the later complete Mac run passed 21/21; see [P.3 result](../../R1.0-Release/done/p3-aarch64-mac-run.md). The earlier remaining-step note below is history.\n'
   if id==59:extra='  - Follow-up landed: daemon start-limit diagnostics and fixture spacing, `667dbe0d` (0.9.227); the complete Mac run is in [P.3 evidence](../../R1.0-Release/done/p3-aarch64-mac-run.md). Earlier blocked runs below are history.\n'
   if id==51:extra='  - Closure covers the landed diagnostic and acceptance fixes. The original transient operating-system failure was not reproduced or attributed; that historical uncertainty is retained below.\n'
   closedblocks.append(first+'\n'+f'  - Closed in main: `{c}` ({v}); commit ancestry and its VERSION verified.\n'+extra+rest)
  else:
   if id in (17,18):owner='owner (the recorded decision is still pending)'
   elif id==26:owner='unassigned; no landed fix is recorded'
   elif id==32:owner='owner for the recorded orphan-container cleanup; code fixes have landed'
   else:owner='unassigned'
   if id==32:block=block.replace('\n', '\n  - Code fixes landed: `c89b7d2c` (0.9.193); the historical orphan-container cleanup has no recorded completion, so this item stays open.\n',1)
   block=block.replace('\n','\n  - Owner: '+owner+'.\n',1)
   openblocks.append(block)
 if openblocks:kept.append(heading+prefix+''.join(openblocks))
 elif not starts:kept.append(''.join(sec))
 if closedblocks:moved.append(heading+''.join(closedblocks))
assert found==set(fixes),(found^set(fixes))
def relocate(m):
 target=m[1]
 if re.match(r'^[A-Za-z][A-Za-z0-9+.-]*:',target) or target.startswith(('/', '#')):return m[0]
 path,sep,anchor=target.partition('#');new=os.path.relpath((src.parent/path).resolve(),dst.parent)
 return ']('+new+(sep+anchor if sep else '')+')'
body=re.sub(r'\]\(([^)\s]+)\)',relocate,''.join(moved))
assert not dst.exists()
dst.write_text('📌 **TL;DR:** History: closed October 30 issues, with their original evidence and verified main commits and versions. Deployment caveats and unknown historical causes are retained. [Open issues](../../2026-10/2026-10-30.md).\n\n# Closed issues — Oct 30, 2026\n\n'+body)
open=''.join(kept)
open=open.replace('Bug reports from live use, not a range review.','Bug reports from live use, not a range review. Closed issues and their evidence are [archived](../done/2026-10/2026-10-30.md).')
src.write_text(open)
print('Archived',len(found),'items:',','.join('J'+str(i) for i in sorted(found)))
