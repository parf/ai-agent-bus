from pathlib import Path
import subprocess,os
p=Path('src/cmd/agent-bus-setup/person_files.go'); original=p.read_bytes()
hooks=Path('src/cmd/agent-bus-setup/review_hooks.local.go')
test=Path('src/cmd/agent-bus-setup/review_swap.local_test.go')
hooks.write_text('''package main
import "golang.org/x/sys/unix"
var reviewOpenat = unix.Openat
var reviewFstat = unix.Fstat
var reviewFchown = unix.Fchown
''')
test.write_text('''package main
import("os";"path/filepath";"testing";"golang.org/x/sys/unix")
func TestReviewRejectsForeignOwnerSwappedAfterMkdir(t *testing.T){
 root,uid,gid:=personFilesFixture(t)
 parent:=filepath.Join(root,"users","parf")
 mustMkdir(t,parent,0700)
 mustMkdir(t,filepath.Join(root,"users"),0755)
 existing:=filepath.Join(parent,"existing-private")
 mustMkdir(t,existing,0755)
 target:=filepath.Join(parent,"files")
 displaced:=filepath.Join(parent,"created-by-this-call")
 fdSwapped:=-1
 chowned:=false
 reviewOpenat=func(parentfd int,name string,flags int,mode uint32)(int,error){
  if name=="files" {
   if err:=os.Rename(target,displaced);err!=nil {t.Fatal(err)}
   if err:=os.Rename(existing,target);err!=nil {t.Fatal(err)}
  }
  fd,err:=unix.Openat(parentfd,name,flags,mode)
  if name=="files" {fdSwapped=fd}
  return fd,err
 }
 reviewFstat=func(fd int,st *unix.Stat_t)error{
  err:=unix.Fstat(fd,st)
  // Model the existing directory's foreign UID without host root privileges.
  if fd==fdSwapped {st.Uid=uint32(uid+1)}
  return err
 }
 reviewFchown=func(fd,owner,group int)error{
  if fd==fdSwapped {chowned=true}
  return unix.Fchown(fd,owner,group)
 }
 defer func(){reviewOpenat=unix.Openat;reviewFstat=unix.Fstat;reviewFchown=unix.Fchown}()
 _,err:=ensurePersonFiles(root,"parf",uid,gid)
 if err==nil || chowned {t.Fatalf("foreign-owned swapped directory accepted/changed: error=%v Fchown=%t mode=%#o; observed foreign UID=%d",err,chowned,modeOf(t,target).Perm(),uid+1)}
}
''')
try:
 s=original.decode().replace('unix.Openat(', 'reviewOpenat(').replace('unix.Fstat(', 'reviewFstat(').replace('unix.Fchown(', 'reviewFchown(')
 p.write_text(s)
 r=subprocess.run(['go','-C','src','test','./cmd/agent-bus-setup','-run','^TestReviewRejectsForeignOwnerSwappedAfterMkdir$','-count=1','-timeout','10s'],env=dict(os.environ,GOCACHE='/usr/local/src/ai-agent-bus/tmp/gocache'),capture_output=True,text=True,timeout=30)
 out=r.stdout+r.stderr;Path('tmp/t27-swap-probe.log').write_text(out);print(out)
 assert r.returncode!=0 and 'foreign-owned swapped directory accepted/changed' in out and '[build failed]' not in out
finally:
 p.write_bytes(original);hooks.unlink();test.unlink()
