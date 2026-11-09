from pathlib import Path
import ctypes,datetime,json,os,signal,subprocess,time
repo=Path('/usr/local/src/ai-agent-bus');wt=repo/'tmp/ab-w2c-r1-final-slow';base=repo/'tmp/gate-logs/20261102-R1-final-slow'
ctypes.CDLL(None).prctl(36,1,0,0,0) # adopt/reap our orphan descendants only
started=time.monotonic();begin=datetime.datetime.now(datetime.timezone.utc).isoformat()
seen={};interrupted=False

def processes():
 out={}
 for d in Path('/proc').iterdir():
  if not d.name.isdigit():continue
  try:
   raw=(d/'stat').read_text(); fields=raw[raw.rfind(')')+2:].split()
   out[int(d.name)]=(int(fields[1]),fields[0],fields[19])
  except (OSError,ValueError,IndexError):pass
 return out

def track():
 ps=processes();family={os.getpid()};changed=True
 while changed:
  changed=False
  for pid,(parent,state,born) in ps.items():
   if parent in family and pid not in family:family.add(pid);seen[pid]=born;changed=True
 return ps

def living():
 ps=track()
 return [p for p,b in seen.items() if p in ps and ps[p][2]==b and ps[p][1]!='Z']

def stop(sig,frame):
 global interrupted
 interrupted=True
 try:os.killpg(int(base.with_suffix('.pid').read_text()),signal.SIGTERM)
 except (OSError,ValueError):pass
signal.signal(signal.SIGTERM,stop);signal.signal(signal.SIGINT,stop)
with base.with_suffix('.log').open('x') as log:
 log.write('TIP c815ac3f1c19d2668e2e9a5c0f2e09de3a0ee9c7\nTREE c7de8b7d9b1969f1fce3652991107fadef9bd280\nCOMMAND setsid --wait bash -c "exec src/smoke.sh --slow all"\nSTART '+begin+'\n');log.flush()
 command='printf "%s\\n" "$$" > '+str(base.with_suffix('.pid'))+'; exec src/smoke.sh --slow all'
 child=subprocess.Popen(['setsid','--wait','bash','-c',command],cwd=wt,stdout=log,stderr=subprocess.STDOUT)
 while child.poll() is None:
  track();time.sleep(.5)
 rc=child.wait();left=living();initial_left=left.copy()
 if left:
  for pid in left:
   try:os.kill(pid,signal.SIGTERM)
   except ProcessLookupError:pass
  until=time.monotonic()+3
  while living() and time.monotonic()<until:time.sleep(.1)
  for pid in living():
   try:os.kill(pid,signal.SIGKILL)
   except ProcessLookupError:pass
  until=time.monotonic()+3
  while living() and time.monotonic()<until:time.sleep(.1)
 while True:
  try:
   if os.waitpid(-1,os.WNOHANG)[0]==0:break
  except ChildProcessError:break
 result={'tip':'c815ac3f1c19d2668e2e9a5c0f2e09de3a0ee9c7','tree':'c7de8b7d9b1969f1fce3652991107fadef9bd280','command':'setsid --wait bash -c "exec src/smoke.sh --slow all"','started':begin,'ended':datetime.datetime.now(datetime.timezone.utc).isoformat(),'wall_seconds':round(time.monotonic()-started,3),'exit_status':rc,'interrupted':interrupted,'descendants_left_at_exit':initial_left,'own_background_left':living()}
 base.with_suffix('.status.json').write_text(json.dumps(result,indent=2)+'\n')
 log.write('\nFINAL '+json.dumps(result)+'\n');log.flush()
 print(json.dumps(result),flush=True)
