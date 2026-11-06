"""Shared core of FileScan and DBSync.

One state file in the agent's own work directory, subscriptions with the author's
authority, delivery that marks a change done only after the subscriber confirms
it (and sends it again otherwise), and a poll timer that never overlaps itself.
The agents decide what an item is; the core only needs a key and a fingerprint.
"""
import copy
import errno
import fcntl
import json
import math
import os
from pathlib import Path
import re
import sys
import threading
import time
import uuid

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
sys.path.insert(0,str(HERE.parents[1]/'clients/python'))
from agent_bus.errors import Refusal
from agent_bus.message import CallDone, CallReply
import subscriptions as base
from subscriptions import SubscriptionError

VERSION=1
DEFAULT_STATE_MAX=64*1024*1024
HOLD_WAIT=10.0  # `sub add` and `sub delete` wait this long for a running scan or delivery round
SUB_FIELDS={'id','target','selector','created_by','created_at','sent','confirmed','dropped'}
CODES={'state_full','state_corrupt','state_unwritable','scan_busy'}


class SyncError(Exception):
    def __init__(self,code,text=None):
        assert code in CODES
        self.code=code;super().__init__(text or code)


def _fresh():return {'version':VERSION,'items':{},'subs':{},'marks':{},'meta':{}}


def _check_state(state):
    """A state that is not exactly ours is refused, never read as empty."""
    bad=lambda:SyncError('state_corrupt')
    if not isinstance(state,dict) or set(state)!={'version','items','subs','marks','meta'} or state['version']!=VERSION:raise bad()
    items,subs,marks,meta=state['items'],state['subs'],state['marks'],state['meta']
    if not all(isinstance(v,dict) for v in (items,subs,marks,meta)):raise bad()
    for key,record in items.items():
        if not isinstance(record,dict) or not isinstance(record.get('fp'),str):raise bad()
    for sub_id,row in subs.items():
        if not isinstance(row,dict) or set(row)!=SUB_FIELDS or row['id']!=sub_id:raise bad()
        try:base.validate_id(sub_id);base.target_text(row['target']);base.identity(row['created_by'])
        except SubscriptionError:raise bad() from None
        if not isinstance(row['selector'],dict) or not isinstance(row['created_at'],str):raise bad()
        for field in ('sent','confirmed','dropped'):
            if type(row[field]) is not int or not 0<=row[field]<=base.MAX_SAFE:raise bad()
    for sub_id,mark in marks.items():
        if sub_id not in subs or not isinstance(mark,dict) or not all(isinstance(v,str) for v in mark.values()):raise bad()
    return state


class StateFile:
    """JSON state written whole to a temporary file, flushed, then renamed over the old one."""
    TEMP='.tmp.'

    def __init__(self,directory,name='state.json',max_bytes=DEFAULT_STATE_MAX):
        if '/' in name or name in ('','.','..') or type(max_bytes) is not int or max_bytes<=0:raise ValueError('state name or cap')
        self.dir=Path(directory);self.name=name;self.max_bytes=max_bytes
        if not self.dir.is_dir():raise SyncError('state_unwritable',f'work directory missing: {self.dir}')
        self.path=self.dir/name;self.lock_path=self.dir/(name+'.lock')
        self._thread_lock=threading.Lock()

    def load(self):
        try:raw=self.path.read_bytes()
        except FileNotFoundError:return _fresh()
        except OSError:raise SyncError('state_unwritable') from None
        try:state=json.loads(raw)
        except (ValueError,UnicodeError):raise SyncError('state_corrupt') from None
        return _check_state(state)

    def save(self,state):
        """Raises before touching the old file when the new one would pass the cap."""
        _check_state(state)
        data=json.dumps(state,separators=(',',':'),sort_keys=True,allow_nan=False).encode()
        if len(data)>self.max_bytes:
            raise SyncError('state_full',f'state full: {len(data)} of {self.max_bytes} bytes; raise state_max, exclude more, or split the folder')
        temp=self.dir/f'{self.name}{self.TEMP}{os.getpid()}.{uuid.uuid4().hex[:8]}'
        try:
            fd=os.open(temp,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
            try:
                view=memoryview(data)
                while view:view=view[os.write(fd,view):]
                os.fsync(fd)
            finally:os.close(fd)
            os.replace(temp,self.path)
        except OSError as error:
            try:os.unlink(temp)
            except OSError:pass
            raise SyncError('state_unwritable',f'cannot write state: {errno.errorcode.get(error.errno,error.errno)}') from None
        except BaseException:
            try:os.unlink(temp)
            except OSError:pass
            raise
        try:
            dfd=os.open(self.dir,os.O_RDONLY)
            try:os.fsync(dfd)
            finally:os.close(dfd)
        except OSError:pass  # The rename is done; a directory flush that fails cannot undo it.

    def clean_temporaries(self):
        """Leftovers of a write that died before its rename; the state itself is never touched."""
        prefix=self.name+self.TEMP;count=0
        for entry in self.dir.iterdir():
            if entry.name.startswith(prefix) and entry.is_file() and not entry.is_symlink():
                try:entry.unlink();count+=1
                except OSError:pass
        return count

    def hold(self,wait=0.0):
        """One writer at a time, across threads and processes: `with state.hold():`.

        Refuses with `scan_busy` at once, or after waiting up to `wait` seconds.
        Reading (`load`) needs no hold: a saved file is always whole.
        """
        return _Hold(self,wait)


class _Hold:
    def __init__(self,state,wait):self.state=state;self.wait=wait;self.fd=None
    def __enter__(self):
        end=time.monotonic()+self.wait
        while True:
            if self._try():return self
            if time.monotonic()>=end:raise SyncError('scan_busy','a scan is already running')
            time.sleep(0.02)
    def _try(self):
        if not self.state._thread_lock.acquire(blocking=False):return False
        try:
            self.fd=os.open(self.state.lock_path,os.O_WRONLY|os.O_CREAT|os.O_NOFOLLOW,0o600)
            fcntl.flock(self.fd,fcntl.LOCK_EX|fcntl.LOCK_NB)
            return True
        except BaseException as error:
            if self.fd is not None:os.close(self.fd);self.fd=None
            self.state._thread_lock.release()
            if isinstance(error,OSError) and error.errno in (errno.EWOULDBLOCK,errno.EAGAIN):return False
            raise
    def __exit__(self,*_):
        try:os.close(self.fd)
        finally:self.state._thread_lock.release()


def reset(state):
    """Drops items and marks; subscriptions stay, so everything replays to each of them."""
    state['items']={};state['marks']={sub_id:{} for sub_id in state['subs']};state['meta']={}
    return state


def pending(state,sub_id,matches=lambda key:True):
    """What this subscriber has not confirmed: (key, 'upsert'|'delete', record), sorted by key.

    Derived from the items and the subscriber's marks, so a send that was never
    confirmed is pending again without any separate bookkeeping.
    """
    mark=state['marks'].get(sub_id,{});result=[]
    for key in sorted(set(state['items'])|set(mark)):
        if not matches(key):continue
        record=state['items'].get(key)
        if record is None:result.append((key,'delete',None))
        elif mark.get(key)!=record['fp']:result.append((key,'upsert',record))
    return result


class SyncSubscriptions:
    """`sub add`, `sub list`, `sub delete` over the state file, with the shared authority rules."""

    def __init__(self,bus,source,state,*,validate_selector,max_subscriptions=100,max_per_creator=20,clock=time.time,id_factory=lambda:uuid.uuid4().hex[:12]):
        self.bus,self.source,self.state=bus,base.target_text(source),state
        self.validate_selector,self.clock,self.id_factory=validate_selector,clock,id_factory
        if type(max_subscriptions) is not int or not 1<=max_subscriptions<=100 or type(max_per_creator) is not int or not 1<=max_per_creator<=20:
            raise SubscriptionError('limit')
        self.maximum,self.per_creator=max_subscriptions,max_per_creator

    def _selector(self,value):
        try:
            if not isinstance(value,dict) or len(json.dumps(value,allow_nan=False).encode())>2048:raise ValueError()
            result=self.validate_selector(copy.deepcopy(value))
            if not isinstance(result,dict):raise ValueError()
            return result
        except Exception:raise SubscriptionError('invalid_selector') from None

    def add(self,message,target,selector,*,replay=True):
        who=base.actor(message);target=base.target_text(target);selector=self._selector(selector)
        with self.state.hold(HOLD_WAIT):
            state=self.state.load()
            if len(state['subs'])>=self.maximum or sum(r['created_by']==who for r in state['subs'].values())>=self.per_creator:raise SubscriptionError('limit')
            cache={}
            for wanted in (target,self.source):
                code=base.checked_admission(self.bus,who,wanted,cache)
                if code:raise SubscriptionError(code)
            try:target=base.destination(self.bus,target)
            except SubscriptionError:raise
            except Exception:raise SubscriptionError('admission_unavailable') from None
            sub_id=base.validate_id(self.id_factory())
            if sub_id in state['subs']:raise SubscriptionError('limit')
            row={'id':sub_id,'target':target,'selector':selector,'created_by':who,'created_at':base.stamp(self.clock),'sent':0,'confirmed':0,'dropped':0}
            state['subs'][sub_id]=row
            # Replay by default: no marks, so every item is pending. replay:false marks what exists as already done.
            state['marks'][sub_id]={} if replay else {k:r['fp'] for k,r in state['items'].items()}
            try:self.state.save(state)
            except SyncError as error:raise SubscriptionError('store_unknown') from error
            return copy.deepcopy(row)

    def list(self,message,matches=None):
        who=base.actor(message)
        state=self.state.load();manager=base.own_manager(message,self.source);result=[]
        for sub_id in sorted(state['subs']):
            row=state['subs'][sub_id]
            if not manager and row['created_by']!=who:continue
            shown=copy.deepcopy(row)
            shown['pending']=len(pending(state,sub_id,(lambda key,row=row:matches(row,key)) if matches else (lambda key:True)))
            result.append(shown)
        return result

    def delete(self,message,sub_id):
        base.validate_id(sub_id);who=base.actor(message)
        with self.state.hold(HOLD_WAIT):
            state=self.state.load();row=state['subs'].get(sub_id)
            if row is None:raise SubscriptionError('not_found')
            if not base.own_manager(message,self.source) and row['created_by']!=who:raise SubscriptionError('not_author')
            del state['subs'][sub_id];state['marks'].pop(sub_id,None)
            try:self.state.save(state)
            except SyncError as error:raise SubscriptionError('store_unknown') from error
            return True


def reply_ok(result):
    """A confirmed answer: a done receipt, or a reply that says `status: ok`. Anything else is not a confirmation."""
    if isinstance(result,CallDone):return True
    if not isinstance(result,CallReply):return False
    try:
        body=json.loads(result.message.body)
    except (ValueError,TypeError,AttributeError):return False
    return isinstance(body,dict) and body.get('status')=='ok' and 'error' not in body


def deliver(bus,source,state,*,matches,body,topic,tag,batch=20,confirm_wait=15.0,deadline=None,warning=lambda code,sub_id:None,ok=reply_ok):
    """One delivery round for every subscriber; returns {sub_id: counts}.

    - Per subscriber: pending changes in key order, at most `batch`; stops at the first one not confirmed, so order holds.
    - A change is marked done for that subscriber only after the target confirms it: an agent answers ok, a queue accepts the send.
    - The author's may-send to the target and to the source, and the target's record, are checked again before every send.
    - A refusal, a timeout or no answer leaves the change pending; one stuck subscriber never holds up the others.
    - The state is saved once per subscriber, so a crash between a confirmation and the save sends that batch again (at least once).
    """
    results={}
    with state.hold():
        data=state.load()
        for sub_id in sorted(data['subs']):
            row=data['subs'][sub_id];count={'sent':0,'confirmed':0,'dropped':0,'pending':0};results[sub_id]=count
            todo=pending(data,sub_id,lambda key,row=row:matches(row,key))
            try:
                for key,action,record in todo[:batch]:
                    if deadline is not None and time.monotonic()>=deadline:break
                    code,kind=_admit(bus,source,row)
                    if code:
                        count['dropped']+=1;row['dropped']=min(base.MAX_SAFE,row['dropped']+1);warning(code,sub_id);break
                    try:
                        payload=body(row,key,action,record)
                        count['sent']+=1;row['sent']=min(base.MAX_SAFE,row['sent']+1)
                        if kind=='queue':
                            bus.send(row['target'],payload,topic=topic,tag=tag);confirmed=True
                        else:
                            confirmed=ok(bus.call(row['target'],payload,timeout=confirm_wait,topic=topic))
                    except Refusal:confirmed=False;code='send_refused'
                    except Exception:confirmed=False;code='send_unknown'
                    if not confirmed:
                        count['dropped']+=1;row['dropped']=min(base.MAX_SAFE,row['dropped']+1);warning(code if code else 'unconfirmed',sub_id);break
                    count['confirmed']+=1;row['confirmed']=min(base.MAX_SAFE,row['confirmed']+1)
                    if action=='delete':data['marks'][sub_id].pop(key,None)
                    else:data['marks'][sub_id][key]=record['fp']
            finally:
                count['pending']=len(pending(data,sub_id,lambda key,row=row:matches(row,key)))
        if any(c['sent'] or c['dropped'] for c in results.values()):state.save(data)
    return results


def _admit(bus,source,row):
    """(refusal code or None, 'queue' or 'agent'), asked afresh for each forward."""
    who,target=row['created_by'],row['target'];cache={}
    for wanted in (target,source):
        code=base.checked_admission(bus,who,wanted,cache)
        if code:return code,None
    try:
        value=bus.record(target).lookup()
        if not isinstance(value,dict) or value.get('kind') not in ('agent','queue') or base.target_text(value.get('name'))!=target:return 'admission_refused',None
    except Refusal as error:return ('admission_refused' if error.status==404 else 'admission_unavailable'),None
    except Exception:return 'admission_unavailable',None
    return None,value['kind']


class PollTimer:
    """Runs `run` at start and then every `interval` seconds, one run at a time.

    A run that raises is reported to `on_error` and the timer keeps going. `trigger()` asks for a run now.
    The next wait starts when a run ends, so a slow run never makes the timer catch up with a burst.
    """

    def __init__(self,interval,run,*,on_error=lambda error:None,immediate=True):
        if not isinstance(interval,(int,float)) or isinstance(interval,bool) or not math.isfinite(interval) or interval<=0:raise ValueError('interval must be positive')
        self.interval,self.run,self.on_error,self.immediate=interval,run,on_error,immediate
        self._wake=threading.Event();self._stop=threading.Event();self._thread=None;self.runs=0

    def start(self):
        if self._thread is not None:raise RuntimeError('timer already started')
        self._thread=threading.Thread(target=self._loop,name='poll-timer',daemon=True);self._thread.start()

    def trigger(self):self._wake.set()

    def stop(self,timeout=5.0):
        """True when the loop has ended; False when a run is still going after `timeout`."""
        self._stop.set();self._wake.set()
        if self._thread is None:return True
        self._thread.join(timeout)
        return not self._thread.is_alive()

    def _loop(self):
        first=self.immediate
        while not self._stop.is_set():
            if not first:
                self._wake.wait(self.interval)
                if self._stop.is_set():break
            first=False;self._wake.clear()
            try:self.run()
            except Exception as error:
                try:self.on_error(error)
                except Exception:pass
            self.runs+=1
