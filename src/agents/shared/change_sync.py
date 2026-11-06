"""Shared core of FileScan and DBSync.

One state file in the agent's own work directory, subscriptions with the author's
authority, delivery that marks a change done only after the subscriber confirms
it (and sends it again otherwise), and a poll timer that never overlaps itself.
The agents decide what an item is; the core only needs a key and a fingerprint.
"""
import contextlib
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
SETTLE_WAIT=1.0  # one `settle` question's share of a round: many rows asked about never use up the round
ROUND_MAX=8.0   # a delivery round ends within this, so it always lets an add or delete through inside HOLD_WAIT
SUB_FIELDS={'id','target','selector','created_by','created_at','sent','confirmed','dropped'}
CODES={'state_full','state_corrupt','state_unwritable','scan_busy'}


class SyncError(Exception):
    def __init__(self,code,text=None):
        assert code in CODES
        self.code=code;super().__init__(text or code)


def _no_constant(name):raise ValueError('NaN and Infinity are not state')


def _fresh():return {'version':VERSION,'items':{},'subs':{},'marks':{},'maybe':{},'skipped':{},'meta':{}}


def _check_state(state):
    """A state that is not exactly ours is refused, never read as empty."""
    bad=lambda:SyncError('state_corrupt')
    if not isinstance(state,dict) or set(state)!={'version','items','subs','marks','maybe','skipped','meta'} or state['version']!=VERSION:raise bad()
    items,subs,marks,maybe,skipped,meta=state['items'],state['subs'],state['marks'],state['maybe'],state['skipped'],state['meta']
    if not all(isinstance(v,dict) for v in (items,subs,marks,maybe,skipped,meta)):raise bad()
    for key,record in items.items():
        if not isinstance(record,dict) or not isinstance(record.get('fp'),str) or record.get('gone',True) is not True:raise bad()
    for sub_id,row in subs.items():
        if not isinstance(row,dict) or set(row)!=SUB_FIELDS or row['id']!=sub_id:raise bad()
        try:base.validate_id(sub_id);base.target_text(row['target']);base.identity(row['created_by'])
        except SubscriptionError:raise bad() from None
        if not isinstance(row['selector'],dict) or not isinstance(row['created_at'],str):raise bad()
        for field in ('sent','confirmed','dropped'):
            if type(row[field]) is not int or not 0<=row[field]<=base.MAX_SAFE:raise bad()
    for table in (marks,maybe,skipped):
        for sub_id,mark in table.items():
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
        try:state=json.loads(raw,parse_constant=_no_constant)
        except (ValueError,UnicodeError,RecursionError):raise SyncError('state_corrupt') from None
        return _check_state(state)

    def save(self,state):
        """Raises before touching the old file when the new one would pass the cap."""
        _check_state(state)
        try:data=json.dumps(state,separators=(',',':'),sort_keys=True,allow_nan=False).encode()
        except (ValueError,TypeError,RecursionError):raise SyncError('state_corrupt','state holds a value that cannot be written (NaN, Infinity or not JSON)') from None
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
        """Leftovers of a write that died before its rename; the state itself is never touched.

        Takes the hold itself, so another writer's in-flight temporary is never removed:
        it raises `scan_busy` while anyone holds the state, and must not be called with the hold taken.
        """
        prefix=self.name+self.TEMP;count=0
        with self.hold():
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
    state['items']={};state['marks']={sub_id:{} for sub_id in state['subs']};state['maybe']={sub_id:{} for sub_id in state['subs']};state['skipped']={sub_id:{} for sub_id in state['subs']};state['meta']={}
    return state


def pending(state,sub_id,matches=lambda key:True):
    """What this subscriber has not confirmed: (key, 'create'|'change'|'delete', record), sorted by key.
    A key the subscriber has no mark for is `create`, one whose mark differs is `change`, one it has a mark for that is gone is `delete`.
    A key whose send was started but never confirmed (`maybe`) is owed a delete once it is gone, and a change while it is
    present with a different fingerprint than the send had (even when the mark equals the present one): the target may
    hold what was sent. `?` stands for several unknown sends, or a delete whose outcome is unknown, and differs from every
    fingerprint: a key that comes back after such a delete is owed a change even at the fingerprint its mark holds.
    A tombstone (an item with `gone`, a row the source still sees as deleted) is a `delete` with its record for a subscriber
    that holds a mark or an unknown send of the key, and nothing for one that never got it: there is nothing to take back.

    Derived from the items and the subscriber's marks, so a send that was never
    confirmed is pending again without any separate bookkeeping.
    """
    mark=state['marks'].get(sub_id,{});maybe=state['maybe'].get(sub_id,{});skipped=state['skipped'].get(sub_id,{});result=[]
    for key in sorted(set(state['items'])|set(mark)|set(maybe)):
        if not matches(key):continue
        record=state['items'].get(key)
        if record is None:result.append((key,'delete',None))  # a mark, or a send whose outcome is unknown, still earns its delete
        elif record.get('gone'):
            if key in mark or key in maybe:result.append((key,'delete',record))
        elif skipped.get(key)==record['fp']:continue  # the agent had nothing to send for this very version
        elif key not in mark:result.append((key,'create',record))
        elif mark[key]!=record['fp'] or maybe.get(key,record['fp'])!=record['fp']:result.append((key,'change',record))
    return result


class SyncSubscriptions:
    """`sub add`, `sub list`, `sub delete` over the state file, with the shared authority rules."""

    def __init__(self,bus,source,state,*,validate_selector,agent_contract=None,max_subscriptions=100,max_per_creator=20,clock=time.time,id_factory=lambda:uuid.uuid4().hex[:12]):
        self.bus,self.source,self.state=bus,base.target_text(source),state
        self.validate_selector,self.clock,self.id_factory=validate_selector,clock,id_factory
        # An agent target is accepted only for a selector the owner names a tested answer contract for: `agent_contract(selector)` is true.
        self.agent_contract=agent_contract
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

    # Errors are SubscriptionError codes. Beyond the shared ones: `busy` when a running scan or round did
    # not end within HOLD_WAIT (nothing was changed), `store_unknown` when the state file is damaged or unwritable.
    @contextlib.contextmanager
    def _held(self):
        hold=self.state.hold(HOLD_WAIT)
        try:hold.__enter__()
        except SyncError as error:raise SubscriptionError('busy' if error.code=='scan_busy' else 'store_unknown') from error
        try:yield
        finally:hold.__exit__()

    def _loaded(self):
        try:return self.state.load()
        except SyncError as error:raise SubscriptionError('store_unknown') from error

    def add(self,message,target,selector,*,replay=True):
        who=base.actor(message);target=base.target_text(target);selector=self._selector(selector)
        with self._held():
            state=self._loaded()
            if len(state['subs'])>=self.maximum or sum(r['created_by']==who for r in state['subs'].values())>=self.per_creator:raise SubscriptionError('limit')
            cache={}
            for wanted in (target,self.source):
                code=base.checked_admission(self.bus,who,wanted,cache)
                if code:raise SubscriptionError(code)
            try:
                target=base.destination(self.bus,target)
                kind=self.bus.record(target).lookup()['kind']
            except SubscriptionError:raise
            except Exception:raise SubscriptionError('admission_unavailable') from None
            if kind=='agent' and not (self.agent_contract and self.agent_contract(selector)):raise SubscriptionError('agent_contract_missing')
            sub_id=base.validate_id(self.id_factory())
            if sub_id in state['subs']:raise SubscriptionError('limit')
            row={'id':sub_id,'target':target,'selector':selector,'created_by':who,'created_at':base.stamp(self.clock),'sent':0,'confirmed':0,'dropped':0}
            state['subs'][sub_id]=row
            # Replay by default: no marks, so every item is pending. replay:false marks what exists as already done (not a tombstone).
            state['marks'][sub_id]={} if replay else {k:r['fp'] for k,r in state['items'].items() if not r.get('gone')};state['maybe'][sub_id]={};state['skipped'][sub_id]={}
            try:self.state.save(state)
            except SyncError as error:raise SubscriptionError('store_unknown') from error
            return copy.deepcopy(row)

    def list(self,message,matches=None):
        who=base.actor(message)
        state=self._loaded();manager=base.own_manager(message,self.source);result=[]
        for sub_id in sorted(state['subs']):
            row=state['subs'][sub_id]
            if not manager and row['created_by']!=who:continue
            shown=copy.deepcopy(row)
            shown['pending']=len(pending(state,sub_id,(lambda key,row=row:matches(row,key)) if matches else (lambda key:True)))
            result.append(shown)
        return result

    def delete(self,message,sub_id):
        base.validate_id(sub_id);who=base.actor(message)
        with self._held():
            state=self._loaded();row=state['subs'].get(sub_id)
            if row is None:raise SubscriptionError('not_found')
            if not base.own_manager(message,self.source) and row['created_by']!=who:raise SubscriptionError('not_author')
            del state['subs'][sub_id];state['marks'].pop(sub_id,None);state['maybe'].pop(sub_id,None);state['skipped'].pop(sub_id,None)
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


def deliver(bus,source,state,*,matches,body,topic,tag,batch=20,confirm_wait=5.0,deadline=None,warning=lambda code,sub_id:None,ok=None,agent_contract=None,rename=None,settle=None):
    """One delivery round for every subscriber; returns {sub_id: counts}.

    - Per subscriber: pending changes in key order, at most `batch`; stops at the first one not confirmed, so order holds (a row
      `settle` leaves unknown is passed over, never a stop).
    - A change is marked done for that subscriber only after the target confirms it: an agent answers ok, a queue accepts the send.
      Before the sends, every create and change of the batch is recorded as possibly delivered (and saved), so a send whose
      outcome is never learned still earns its delete if the source later removes the key; every delete is recorded as `?`,
      so a key that comes back after a delete of unknown outcome is sent again. A change the loop never reached,
      or whose send was refused outright, is taken back out; a timeout, an unknown result or a reply that is not ok keeps it.
      A `body` that returns None has nothing to send that subscriber: the change is counted as `skipped` and remembered
      (per subscriber, by fingerprint) so it is not skipped again, without writing a mark: only a confirmation does.
      A second unknown send of the same key turns its record into `?`, which stays owed until a confirmation.
    - `rename(row)` true lets a pending create whose record names `from` (the key it was renamed from) travel as ONE `rename`
      change with the delete of that key, when the subscriber has both pending (it holds, or may hold, the old key). Both
      keys are recorded as possibly delivered before the send and both marks move on the confirmation; a crash or an
      unconfirmed send leaves both pending, so the same rename is sent again, never a bare create. A `body` for a rename
      must return a message; None is a failure of the body. Without `rename` a rename is a delete and a create, as ever.
    - `body` may return (text, options): the options go with that one send or call. Options with `wait` send to an agent without
      waiting for its answer (RAG works to that deadline): the change stays possibly delivered for `settle` to resolve later.
    - `settle(row, key, record, wait)` is asked before sending again a create or change whose earlier send of this very version (same
      fingerprint) had an unknown outcome: 'confirmed' marks it done with no send, 'resend' sends it as without the hook, anything else
      leaves it pending and untouched for this round while the rest of the batch goes on. A key edited since that send has another
      fingerprint and is sent without asking; a send refused outright was never possibly delivered, so it is not asked either.
      Such rows take no place in the batch (up to `batch` of them are asked after it), and each question gets at most
      SETTLE_WAIT. Without `settle` nothing changes.
    - After a crash a subscriber may get a delete for a key it never received (the record is written for the whole batch before
      the sends): deletes must be idempotent. Never the other way: a delete is not lost.
    - The author's may-send to the target and to the source, and the target's record, are checked again before every send.
    - A `body` that raises sends nothing: the key's possibly-delivered record is taken back, `body_failed` is warned and the
      subscriber's batch stops there, so order holds.
    - A refusal, a timeout or no answer leaves the change pending; one stuck subscriber never holds up the others.
    - The round ends by `deadline` (default ROUND_MAX seconds from now) and no wait on a target runs past it, so a silent
      agent cannot keep the state held longer than a `sub add` or `sub delete` waits for it. Only the waits are
      bounded: a send already started is not interrupted, and the deadline is checked again after the authority check.
    - Marks are saved after each subscriber that confirmed something, so a later failure keeps the earlier subscribers'
      confirmations; the sent and dropped counters wait in memory for the round's last save (so a round of refusals writes once). A crash
      between a confirmation and its save sends that subscriber's batch again (at least once). A save that fails (`state_full`,
      a write error) is reported through `warning` and ends the round: nothing more is sent whose confirmation cannot be kept.
    """
    if deadline is None:deadline=time.monotonic()+ROUND_MAX
    results={};dirty=False
    with state.hold():
        data=state.load()
        for sub_id in sorted(data['subs']):
            row=data['subs'][sub_id];count={'sent':0,'confirmed':0,'dropped':0,'pending':0,'skipped':0};results[sub_id]=count
            entries=_fuse(pending(data,sub_id,lambda key,row=row:matches(row,key)),bool(rename and rename(row)))
            if settle is None:todo=entries[:batch]
            else:
                # Rows whose earlier send of this version is unknown are only asked about, so they take no place in the batch:
                # a run of them (large files in flight) never holds up the files behind them. Each kind is capped at `batch`.
                unsettled=lambda e:e[1] in ('create','change') and data['maybe'][sub_id].get(e[0])==e[2]['fp']
                todo=[e for e in entries if not unsettled(e)][:batch]+[e for e in entries if unsettled(e)][:batch]
            # Written down before any send: a send whose outcome is never learned (a timeout, a crash, a failed save) may
            # have been applied, so a later delete of that key must still be sent. Cleared by the confirmation.
            # A delete is written as `?`: the target may still hold the key, or may not, so a key that comes back is owed again.
            had=data['maybe'][sub_id];owed={}
            earlier={k:had.get(k) for k,_,_ in todo}   # before this batch is written down: what an earlier round left unknown
            for k,a,r in todo:
                if a=='rename' and had.get(r['from'])!='?':owed[r['from']]='?'
                if a=='delete':
                    if had.get(k)!='?':owed[k]='?'
                elif had.get(k)!=r['fp']:owed[k]=r['fp'] if k not in had else '?'
            prior={k:data['maybe'][sub_id].get(k) for k in owed};attempted=set();restored=False
            gone=[k for k in data['skipped'][sub_id] if k not in data['items']]
            for k in gone:del data['skipped'][sub_id][k]  # a removed item's skip memory would otherwise live forever
            restored=bool(gone)
            if owed:
                data['maybe'][sub_id].update(owed)
                try:state.save(data)
                except SyncError as error:
                    count['unsaved']=True;warning(error.code,sub_id);break
            try:
                for key,action,record in todo:
                    left=deadline-time.monotonic()
                    if left<=0:break
                    code,kind=_admit(bus,source,row)
                    if not code and time.monotonic()>=deadline:break  # the check itself may have used the time left
                    if code:
                        count['dropped']+=1;row['dropped']=min(base.MAX_SAFE,row['dropped']+1);warning(code,sub_id);break
                    if kind=='agent' and (ok is None or (agent_contract is not None and not agent_contract(row))):
                        # No answer contract for an agent: nothing is sent, because nothing could say it was done.
                        count['dropped']+=1;row['dropped']=min(base.MAX_SAFE,row['dropped']+1);warning('agent_contract_missing',sub_id);break
                    if settle is not None and action in ('create','change') and earlier.get(key)==record['fp']:
                        verdict=settle(row,key,record,max(0.05,min(SETTLE_WAIT,deadline-time.monotonic())))
                        if verdict=='confirmed':
                            attempted.add(key);count['confirmed']+=1;row['confirmed']=min(base.MAX_SAFE,row['confirmed']+1)
                            data['maybe'][sub_id].pop(key,None);data['skipped'][sub_id].pop(key,None);data['marks'][sub_id][key]=record['fp']
                            continue
                        if verdict!='resend':
                            count['unsettled']=count.get('unsettled',0)+1;continue   # still unknown: this row waits, the others go on
                    try:payload=body(row,key,action,record)
                    except Exception:
                        # Built before the key counts as attempted: nothing was sent, so its record is taken back below.
                        warning('body_failed',sub_id);break
                    if payload is None and action=='rename':
                        warning('body_failed',sub_id);break   # as a body that raised: nothing sent, so the pair is taken back
                    attempted.update(_keys(key,action,record))
                    options={}
                    if isinstance(payload,tuple):payload,options=payload   # (text, call options such as a longer `ttl`)
                    try:
                        if payload is None:
                            # Nothing to send this subscriber for it (the agent says why): done, never blocking the rest.
                            count['skipped']+=1
                            # Nothing was sent. An earlier unknown send of this key stays owed a delete, but is no longer owed a change.
                            if action=='delete' or (prior[key] if key in prior else data['maybe'][sub_id].get(key)) is None:data['maybe'][sub_id].pop(key,None)
                            else:data['maybe'][sub_id][key]=record['fp']
                            restored=True
                            # Skip bookkeeping stays out of the marks: only a real confirmation writes one, and only a mark or a
                            # possibly-delivered record earns a delete.
                            if action=='delete':data['marks'][sub_id].pop(key,None);data['skipped'][sub_id].pop(key,None)
                            else:data['skipped'][sub_id][key]=record['fp']
                            continue
                        count['sent']+=1;row['sent']=min(base.MAX_SAFE,row['sent']+1)
                        if kind=='queue':
                            bus.send(row['target'],payload,topic=topic,tag=tag,**options);confirmed=True
                        elif 'wait' in options:
                            # Sent with its own deadline and not waited on: in flight, its outcome unknown until `settle` learns it.
                            # It stays possibly delivered (never resent while `settle` says wait) and the rest of the batch goes on.
                            bus.send(row['target'],payload,topic=topic,**options);count['in_flight']=count.get('in_flight',0)+1;continue
                        else:
                            confirmed=ok(bus.call(row['target'],payload,timeout=max(0.05,min(confirm_wait,left)),topic=topic,**options))
                    except Refusal:
                        # Refused outright, so never queued: it is not possibly delivered (a timeout or a bad reply is).
                        confirmed=False;code='send_refused'
                        for k in _keys(key,action,record):
                            if k in prior:_restore(data['maybe'][sub_id],k,prior);restored=True
                    except Exception:confirmed=False;code='send_unknown'
                    if not confirmed:
                        count['dropped']+=1;row['dropped']=min(base.MAX_SAFE,row['dropped']+1);warning(code if code else 'unconfirmed',sub_id);break
                    count['confirmed']+=1;row['confirmed']=min(base.MAX_SAFE,row['confirmed']+1)
                    for k in _keys(key,action,record):data['maybe'][sub_id].pop(k,None);data['skipped'][sub_id].pop(k,None)
                    if action=='rename':data['marks'][sub_id].pop(record['from'],None)
                    if action=='delete':data['marks'][sub_id].pop(key,None)
                    else:data['marks'][sub_id][key]=record['fp']
            finally:
                # What was written down for the batch but never attempted (the loop stopped first) was not sent.
                for k in owed:
                    if k not in attempted:_restore(data['maybe'][sub_id],k,prior);restored=True
                count['pending']=len(pending(data,sub_id,lambda key,row=row:matches(row,key)))
            # Marks (and a taken-back record) are saved as soon as a subscriber has any; the counters wait in memory for the round's last save.
            if count['confirmed'] or count['skipped'] or restored:
                try:state.save(data);dirty=False
                except SyncError as error:
                    count['unsaved']=True;warning(error.code,sub_id);break
            elif count['sent'] or count['dropped']:dirty=True
        else:
            if dirty:
                try:state.save(data)
                except SyncError as error:warning(error.code,None)
    return results


def _keys(key,action,record):
    """The keys one change settles: a rename settles the new key and the one it came from."""
    return (key,record['from']) if action=='rename' else (key,)


def _fuse(entries,allowed):
    """Join each pending create that names `from` with the pending delete of that key into one `rename` at the new key."""
    if not allowed:return entries
    deleted={k for k,a,_ in entries if a=='delete'};used=set();out=[]
    for k,a,r in entries:
        # `from` is the key renamed from, or the keys it came from through several renames, nearest first: the first the
        # subscriber has pending as a delete is the one it still holds.
        origins=(r['from'] if isinstance(r.get('from'),list) else [r.get('from')]) if a=='create' else []
        chosen=next((o for o in origins if isinstance(o,str) and o!=k and o in deleted and o not in used),None)
        if chosen is not None:out.append((k,'rename',{**r,'from':chosen}));used.add(chosen)
        else:out.append((k,a,r))
    return [e for e in out if not (e[1]=='delete' and e[0] in used)]


def _restore(maybe,key,prior):
    """Put back what `maybe` held for `key` before the batch was recorded."""
    if prior.get(key) is None:maybe.pop(key,None)
    else:maybe[key]=prior[key]


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
