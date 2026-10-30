"""Common subscription state, authority, matching and once-only delivery."""
from collections.abc import Mapping
import copy
from concurrent.futures import ThreadPoolExecutor
import selectors
from datetime import datetime, timezone
import json
import math
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import threading
import time
import uuid

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE.parents[1]/'clients/python'))
from agent_bus.errors import BusError, Refusal, UnexpectedAnswer, UncertainOutcome
import subscription_filters as filters

MAX_SAFE=2**53-1
FIELDS={'id','target','selector','filter','created_by','created_at','sent','dropped'}
ID=re.compile(r'[0-9a-f]{12}\Z')
NAME=re.compile(r'#?[a-z0-9][a-z0-9._/-]*(?:@[a-z0-9][a-z0-9.-]*)?\Z')
ERRORS={'invalid_target','invalid_selector','invalid_filter','invalid_id','limit','not_found','not_author','admission_refused','admission_unavailable','store_unknown'}
WARNINGS={'admission_refused','admission_unavailable','filter_invalid','filter_timeout','invalid_text','send_refused','send_unknown','deadline','store_unknown'}


class SubscriptionError(ValueError):
    def __init__(self,code):self.code=code;super().__init__(code)


def target_text(value):
    if not isinstance(value,str) or not 1<=len(value)<=64 or not value.isascii() or not NAME.fullmatch(value.lower()):
        raise SubscriptionError('invalid_target')
    return value.lower()


def identity(value):
    try:return target_text(value)
    except SubscriptionError:raise SubscriptionError('not_author') from None


def validate_id(value):
    if not isinstance(value,str) or not ID.fullmatch(value):raise SubscriptionError('invalid_id')
    return value


def actor(message):
    envelope=getattr(message,'envelope',None)
    if not isinstance(envelope,Mapping):raise SubscriptionError('not_author')
    return identity(envelope.get('from'))


def own_manager(message,source):
    e=getattr(message,'envelope',{})
    original=e.get('original_to')
    direct=original in (None,'') or isinstance(original,str) and original.lower()==source
    return direct and bool(set(e.get('roles',())) & {'owner','maintainer'})


def stamp(clock):
    value=clock()
    if type(value) not in (int,float) or not math.isfinite(value) or not 0<=value<253402300799:raise SubscriptionError('store_unknown')
    ms=math.floor(value*1000)
    return datetime.fromtimestamp(ms//1000,timezone.utc).strftime('%Y-%m-%dT%H:%M:%S.')+f'{ms%1000:03d}Z'


def checked_admission(bus,who,target,cache):
    pair=(who,target)
    if pair in cache:return cache[pair]
    try:
        answer=bus.may_send(who,target)
        code=None if answer is True else 'admission_refused' if answer is False else 'admission_unavailable'
    except Refusal as error:code='admission_refused' if error.status==404 else 'admission_unavailable'
    except Exception:code='admission_unavailable'
    cache[pair]=code
    return code


def destination(bus,target):
    value=bus.record(target).lookup()
    if not isinstance(value,dict) or value.get('kind') not in ('agent','queue'):
        raise SubscriptionError('invalid_target')
    return target_text(value.get('name'))


def deliver_one(bus,source,row,body,topic,tag,deadline,cache,on_send=None):
    """Pure decision path shared by injected fixtures and private send helpers."""
    if time.monotonic()>=deadline:return 'dropped','deadline'
    who,target=row['created_by'],row['target']
    code=checked_admission(bus,who,target,cache)
    if code:return 'dropped',code
    code=checked_admission(bus,who,source,cache)
    if code:return 'dropped',code
    try:
        if destination(bus,target)!=target:return 'dropped','admission_refused'
    except SubscriptionError:return 'dropped','admission_refused'
    except Refusal as error:return 'dropped','admission_refused' if error.status==404 else 'admission_unavailable'
    except Exception:return 'dropped','admission_unavailable'
    if time.monotonic()>=deadline:return 'dropped','deadline'
    try:
        if on_send:on_send()
        bus.send(target,body,topic=topic,tag=tag)
        return 'sent',None
    except Refusal:return 'dropped','send_refused'
    except Exception:return 'dropped','send_unknown'


class Subscriptions:
    def __init__(self,bus,source,*,validate_selector,selector_matches,warning,
                 clock=time.time,id_factory=lambda:uuid.uuid4().hex[:12],
                 max_subscriptions=100,max_per_creator=20,delivery=None):
        self.bus,self.source=bus,target_text(source)
        self.kv=bus.record(source).kv
        self.validate_selector,self.selector_matches=validate_selector,selector_matches
        self.warning,self.clock,self.id_factory=warning,clock,id_factory
        if type(max_subscriptions) is not int or not 1<=max_subscriptions<=100 or type(max_per_creator) is not int or not 1<=max_per_creator<=20:
            raise SubscriptionError('limit')
        self.maximum,self.per_creator=max_subscriptions,max_per_creator
        self.delivery=delivery
        self.lock=threading.RLock()
        self.process_lock=threading.Lock();self.process_condition=threading.Condition(self.process_lock);self.processes=set();self.closed=False

    def _warn(self,code):
        assert code in WARNINGS
        try:self.warning(code)
        except Exception:pass  # Diagnostics cannot cause a message to be sent again.

    def _selector(self,value):
        try:
            if not isinstance(value,dict) or len(json.dumps(value,allow_nan=False).encode())>2048:raise ValueError()
            result=self.validate_selector(copy.deepcopy(value))
            if not isinstance(result,dict):raise ValueError()
            return result
        except Exception:raise SubscriptionError('invalid_selector') from None

    def _row(self,value,key):
        if not isinstance(value,dict) or set(value)!=FIELDS or value.get('id')!=key:raise ValueError()
        validate_id(key);target_text(value['target']);identity(value['created_by'])
        if not isinstance(value['filter'],str):raise ValueError()
        self._selector(value['selector'])
        if not isinstance(value['created_at'],str) or not re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z',value['created_at']):raise ValueError()
        datetime.fromisoformat(value['created_at'].replace('Z','+00:00'))
        for field in ('sent','dropped'):
            if type(value[field]) is not int or not 0<=value[field]<=MAX_SAFE:raise ValueError()
        if len(json.dumps(value,allow_nan=False).encode())>4096:raise ValueError()
        return copy.deepcopy(value)

    def _load(self):
        try:
            listing=self.kv.list()
            if not isinstance(listing,dict) or not isinstance(listing.get('json',[]),list):raise ValueError()
            keys=sorted(row['name'] for row in listing.get('json',[]) if isinstance(row,dict) and isinstance(row.get('name'),str) and row['name'].startswith('sub.'))
            if len(keys)>self.maximum:raise SubscriptionError('limit')
            rows={};damaged={}
            for key in keys:
                sub_id=validate_id(key[4:])
                try:value=self.kv.get(key,kind='json')
                except Refusal as error:
                    if error.status==404:continue
                    raise
                try:rows[sub_id]=self._row(value,sub_id)
                except (ValueError,TypeError,UnicodeError):damaged[sub_id]={'id':sub_id,'reason':'invalid_state'}
            return rows,damaged
        except SubscriptionError:raise
        except Exception:raise SubscriptionError('store_unknown') from None

    def add(self,message,target,selector,filter=''):
        who=actor(message);target=target_text(target);selector=self._selector(selector)
        try:filters.compile_filter(filter)
        except filters.FilterError:raise SubscriptionError('invalid_filter') from None
        with self.lock:
            rows,damaged=self._load()
            if len(rows)+len(damaged)>=self.maximum or sum(r['created_by']==who for r in rows.values())>=self.per_creator:raise SubscriptionError('limit')
            cache={}
            for wanted in (target,self.source):
                code=checked_admission(self.bus,who,wanted,cache)
                if code:raise SubscriptionError(code)
            try:target=destination(self.bus,target)
            except SubscriptionError:raise
            except Exception:raise SubscriptionError('admission_unavailable') from None
            created=stamp(self.clock)
            for _ in range(3):
                sub_id=validate_id(self.id_factory())
                row={'id':sub_id,'target':target,'selector':selector,'filter':filter,'created_by':who,'created_at':created,'sent':0,'dropped':0}
                try:self.kv.set('sub.'+sub_id,row,kind='json',mode='add')
                except Refusal as error:
                    if error.status==412:continue
                    raise SubscriptionError('store_unknown') from None
                except Exception:raise SubscriptionError('store_unknown') from None
                return copy.deepcopy(row)
            raise SubscriptionError('limit')

    def list(self,message):
        who=actor(message)
        with self.lock:
            rows,damaged=self._load();manager=own_manager(message,self.source);result=[]
            for key in sorted(set(rows)|set(damaged)):
                if key in damaged:
                    if manager:result.append(damaged[key])
                    continue
                row=rows[key]
                if not manager and row['created_by']!=who:continue
                try:filters.compile_filter(row['filter'])
                except filters.FilterError:row['reason']='invalid_filter'
                result.append(row)
            return result

    def delete(self,message,sub_id):
        validate_id(sub_id);who=actor(message)
        with self.lock:
            rows,damaged=self._load();row=rows.get(sub_id)
            if row is None and sub_id not in damaged:raise SubscriptionError('not_found')
            if not own_manager(message,self.source) and (row is None or row['created_by']!=who):raise SubscriptionError('not_author')
            try:deleted=self.kv.delete('sub.'+sub_id,kind='json')
            except Exception:raise SubscriptionError('store_unknown') from None
            if deleted is not True:raise SubscriptionError('not_found')
            return True

    def _counter(self,row,field):
        row[field]=min(MAX_SAFE,row[field]+1)
        self.kv.json('sub.'+row['id'],{'op':'set','key':'sent','value':row['sent']},
                     {'op':'set','key':'dropped','value':row['dropped']})

    def _match(self,rows,text,deadline):
        result={};patterns=[]
        for row in rows:
            if not row['filter']:result[row['id']]=True
            else:patterns.append(row['filter'])
        if not patterns:return result
        # The private helper has no credentials and emits only a bounded bool/code list.
        wanted=list(dict.fromkeys(patterns))
        request=json.dumps({'text':text,'filters':wanted},ensure_ascii=True).encode()
        raw,complete=self._helper(HERE/'subscription_filters.py',request,min(deadline,time.monotonic()+1))
        try:
            answers=json.loads(raw)
            if not complete or not isinstance(answers,list) or len(answers)!=len(wanted) or any(type(v) is not bool and v not in ('timeout','invalid') for v in answers):raise ValueError()
        except (ValueError,UnicodeError):answers=['timeout']*len(wanted)
        selected=dict(zip(wanted,answers))
        for row in rows:
            if row['filter']:result[row['id']]=selected[row['filter']]
        return result

    def has_candidates(self,event):
        """Selector-only preflight, before an adapter commits an entry."""
        with self.lock:
            rows,_=self._load()
            for row in rows.values():
                try:
                    if self.selector_matches(row['selector'],event):return True
                except Exception:pass
            return False

    def deliver(self,event,text,body,*,topic,tag,deadline):
        with self.lock:
            if self.closed:raise SubscriptionError('store_unknown')
            rows,_=self._load();candidates=[]
            for row in rows.values():
                try:yes=self.selector_matches(row['selector'],event)
                except Exception:yes=False
                if yes:candidates.append(row)
            candidates.sort(key=lambda r:r['id'])
            outcome={};active=[]
            if time.monotonic()>=deadline:
                outcome={r['id']:('dropped','deadline') for r in candidates}
            else:
                try:filters.text(text)
                except filters.FilterError:outcome={r['id']:('dropped','invalid_text') for r in candidates}
                if not outcome:
                    matching=self._match(candidates,text,deadline)
                    for row in candidates:
                        status=matching[row['id']]
                        if status is True:active.append(row)
                        elif status=='invalid':outcome[row['id']]=('dropped','filter_invalid')
                        elif status=='timeout':outcome[row['id']]=('dropped','filter_timeout')
            if active:
                if self.delivery is not None:
                    outcome.update(self.delivery(active,body,topic,tag,deadline))
                else:outcome.update(self._deliver(active,body,topic,tag,deadline))
            result={'matched':len(outcome),'sent':0,'dropped':0};store_failed=False
            # Every worker is finished before the first KV/counter mutation.
            for row in candidates:
                if row['id'] not in outcome:continue
                status,code=outcome[row['id']]
                assert status in ('sent','dropped') and (code is None if status=='sent' else code in WARNINGS)
                result[status]+=1
                if code:self._warn(code)
                try:self._counter(row,status)
                except Exception:store_failed=True
            if store_failed:self._warn('store_unknown')
            return result

    def _helper(self,script,payload,deadline,args=()):
        """Private pipes and group kill before wait, including a held child pipe."""
        if len(payload)>2*1024*1024 or deadline<=time.monotonic():return b'',False
        env={k:v for k,v in os.environ.items() if k in ('PATH','LANG','LC_ALL','TZ','SSL_CERT_FILE','SSL_CERT_DIR')}
        env.update(PYTHONNOUSERSITE='1',PYTHONDONTWRITEBYTECODE='1')
        with self.process_lock:
            if self.closed:return b'',False
            try:
                process=subprocess.Popen([sys.executable,str(script),*args],stdin=subprocess.PIPE,stdout=subprocess.PIPE,
                                         stderr=subprocess.DEVNULL,env=env,start_new_session=True)
            except OSError:return b'',False
            self.processes.add(process)
        result=bytearray();offset=0;complete=False
        try:
            os.set_blocking(process.stdin.fileno(),False);os.set_blocking(process.stdout.fileno(),False)
            with selectors.DefaultSelector() as poll:
                poll.register(process.stdin,selectors.EVENT_WRITE,'in');poll.register(process.stdout,selectors.EVENT_READ,'out')
                while poll.get_map():
                    remaining=deadline-time.monotonic()
                    if remaining<=0:break
                    events=poll.select(remaining)
                    if not events:break
                    for key,_ in events:
                        if key.data=='in':
                            try:n=os.write(key.fileobj.fileno(),payload[offset:offset+65536])
                            except BlockingIOError:continue
                            except BrokenPipeError:n=0;offset=len(payload)
                            offset+=n
                            if offset>=len(payload):poll.unregister(key.fileobj);key.fileobj.close()
                        else:
                            try:value=os.read(key.fileobj.fileno(),65536)
                            except BlockingIOError:continue
                            if not value:poll.unregister(key.fileobj);key.fileobj.close()
                            else:
                                result.extend(value)
                                if len(result)>64*1024:return b'',False
                complete=not poll.get_map()
        except (OSError,ValueError):complete=False
        finally:
            # Keep registration and reap under one guard: close cannot see a
            # reaped leader whose numeric pgid could already have been reused.
            with self.process_lock:
                try:os.killpg(process.pid,signal.SIGKILL)
                except ProcessLookupError:pass
                process.wait()
                self.processes.discard(process)
                self.process_condition.notify_all()
            for stream in (process.stdin,process.stdout):
                if not stream.closed:stream.close()
        return bytes(result),complete

    def _deliver(self,rows,body,topic,tag,deadline):
        end=min(deadline,time.monotonic()+15)
        try:
            transport=self.bus._transport
            connection={'addr':transport.addr,'token':transport.token,'fingerprint':transport.pin}
            groups={}
            for row in rows:groups.setdefault(row['created_by'],[]).append(row)
            def group(wanted):
                request={'source':self.source,'connection':connection,'rows':wanted,'body':body,'topic':topic,'tag':tag,'deadline':end}
                payload=json.dumps(request,ensure_ascii=True,allow_nan=False).encode()
                raw,complete=self._helper(Path(__file__).resolve(),payload,end,('--deliver',))
                known={r['id'] for r in wanted};result={};started=set();invalid=False
                for line in raw.splitlines():
                    try:
                        value=json.loads(line)
                        if not isinstance(value,dict) or value.get('id') not in known:raise ValueError()
                        if set(value)=={'id','phase'} and value['phase']=='sending':started.add(value['id']);continue
                        if set(value)!={'id','status','code'} or value['id'] in result:raise ValueError()
                        status,code=value['status'],value['code']
                        if status not in ('sent','dropped') or (code is not None if status=='sent' else code not in WARNINGS):raise ValueError()
                        result[value['id']]=(status,code)
                    except (ValueError,TypeError):invalid=True;break
                if invalid:return {r['id']:('dropped','send_unknown') for r in wanted}
                for row in wanted:
                    if row['id'] not in result:result[row['id']]=('dropped','send_unknown' if row['id'] in started else 'deadline' if not complete else 'admission_unavailable')
                return result
            with ThreadPoolExecutor(max_workers=8,thread_name_prefix='subscription-send') as pool:
                futures=[pool.submit(group,wanted) for _,wanted in sorted(groups.items())]
                result={}
                for future in futures:result.update(future.result())
            return result  # Every group is joined/killed/reaped before returning.
        except Exception:return {r['id']:('dropped','admission_unavailable') for r in rows}

    def close(self):
        with self.process_condition:
            self.closed=True
            for process in list(self.processes):
                try:os.killpg(process.pid,signal.SIGKILL)
                except ProcessLookupError:pass
            end=time.monotonic()+2
            while self.processes:
                left=end-time.monotonic()
                if left<=0:raise SubscriptionError('store_unknown')
                self.process_condition.wait(left)


def delivery_worker():
    """Send-only Bus never registers or consumes; connection data is private stdin."""
    from agent_bus import Bus
    try:
        raw=sys.stdin.buffer.read(2*1024*1024+1)
        if len(raw)>2*1024*1024:return 78
        request=json.loads(raw);rows=request['rows'];source=request['source'];connection=request['connection']
        if not isinstance(rows,list) or len(rows)>20:return 78
        deadline=float(request['deadline'])
        if not math.isfinite(deadline):return 78
        running=False
        def alarm(*_):
            if running:raise TimeoutError()
        signal.signal(signal.SIGALRM,alarm)
        with Bus(name=source,addr=connection['addr'],token=connection['token'],fingerprint=connection['fingerprint'],timeout=.2) as bus:
            cache={}
            for row in rows:
                sub_id=validate_id(row['id'])
                running=False
                try:
                    left=min(1,deadline-time.monotonic())
                    if left<=0:status,code='dropped','deadline'
                    else:
                        running=True
                        signal.setitimer(signal.ITIMER_REAL,left)
                        def sending():print(json.dumps({'id':sub_id,'phase':'sending'}),flush=True)
                        status,code=deliver_one(bus,source,row,request['body'],request['topic'],request['tag'],min(deadline,time.monotonic()+1),cache,sending)
                        running=False
                except TimeoutError:
                    running=False;status,code='dropped','send_unknown'
                except Exception:
                    running=False;status,code='dropped','admission_unavailable'
                finally:
                    running=False;signal.setitimer(signal.ITIMER_REAL,0)
                print(json.dumps({'id':sub_id,'status':status,'code':code},separators=(',',':')),flush=True)
        return 0
    except Exception:return 78


if __name__=='__main__':sys.exit(delivery_worker() if sys.argv[1:]==['--deliver'] else 78)
