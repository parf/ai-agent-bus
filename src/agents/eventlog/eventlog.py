#!/usr/bin/env python3
"""Kept, serialized bus-only named event logs; subscription copies precede pull."""
from collections.abc import Mapping
from contextlib import contextmanager
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import signal
import sys
import time

HERE = Path(__file__).resolve().parent
SUPPORT = HERE/'support' if (HERE/'support').is_dir() else HERE.parents[1]
sys.path.insert(0, str(SUPPORT/'clients/python'))
sys.path.insert(0, str(SUPPORT/'agents/shared'))
from agent_bus import Bus
from agent_bus.errors import BusError, BusTimeout
import event_rings as rings
from metadata import dispatch, answer as metadata_answer
from subscriptions import Subscriptions, SubscriptionError

# Read grants own-subscription management; direct instance managers manage all.
SUB_ROLES = frozenset({'read', 'write'})
MAX_REQUEST = 16384
MAX_REPLY = 1024*1024
FIELDS = {'write': {'log','level','text'}, 'read': {'log','level'}, 'list': set(),
          'pull': {'log','level','count'}, 'clean': {'log','level','id'},
          'sub.add': {'target','log','level','filter'}, 'sub.list': set(),
          'sub.delete': {'id'}, 'help': set(), 'status': set()}
REQUIRED = {'write': {'log','level','text'}, 'read': {'log'}, 'list': set(),
            'pull': {'log'}, 'clean': {'log'}, 'sub.add': {'target'},
            'sub.list': set(), 'sub.delete': {'id'}, 'help': set(), 'status': set()}
ERROR_TEXT = {
    'invalid_request': 'Invalid call; ask help.', 'invalid_log': 'Invalid log name.',
    'invalid_level': 'Use notice, warning or alert.', 'invalid_text': 'Invalid entry text.',
    'invalid_config': 'Invalid instance Configuration.', 'invalid_clock': 'Clock unavailable.',
    'invalid_target': 'Destination must be an agent or queue.', 'invalid_selector': 'Invalid subscription selector.',
    'invalid_filter': 'Unsupported subscription filter.', 'invalid_id': 'Invalid subscription id.',
    'forbidden': 'This call requires a direct instance role.', 'not_found': 'No such log or subscription.',
    'not_author': 'Only the creator or direct instance managers may do this.',
    'limit': 'Subscription limit reached.', 'log_limit': 'Active log limit reached.',
    'name_limit': 'Lifetime log-name limit reached.', 'id_limit': 'Entry id limit reached.',
    'admission_refused': 'Creator is not admitted to source or destination.',
    'admission_unavailable': 'Creator admission could not be checked.',
    'store_unknown': 'Subscription storage outcome is unknown.',
    'storage_invalid': 'Persistent ring state is invalid.', 'storage_locked': 'Another generation holds the state.',
    'storage_unknown': 'Ring storage outcome is unknown; reopen validated state.',
    'deadline': 'Insufficient time to store and forward this entry.',
    'internal_error': 'The operation could not be completed.'}


def dumps(value):
    raw = json.dumps(value, ensure_ascii=False, allow_nan=False, separators=(',', ':'))
    if len(raw.encode()) > MAX_REPLY: raise rings.RingError('internal_error')
    return raw


def error(code):
    if code not in ERROR_TEXT: code = 'internal_error'
    return dumps({'error': {'code': code, 'text': ERROR_TEXT[code]}})


def number(value):
    if not isinstance(value,str) or not re.fullmatch(r'[0-9]+',value): raise rings.RingError('invalid_request')
    # Requests are bounded; cap conversion before Python's large integer limit.
    if len(value) > 16: raise rings.RingError('invalid_request')
    return int(value)


def parse(body):
    try:
        if not isinstance(body,str) or len(body.encode('utf-8')) > MAX_REQUEST or '\0' in body:
            raise rings.RingError('invalid_request')
    except UnicodeError: raise rings.RingError('invalid_request') from None
    if body.lstrip().startswith(('{','[')):
        try: call = rings.strict_json(body)
        except (ValueError,UnicodeError,RecursionError): raise rings.RingError('invalid_request') from None
    else:
        text = body.lstrip(' ')
        call = None
        match = re.fullmatch(r'write +(\S+) +(\S+) (.*)',text,re.S)
        if match: call = dict(method='write',log=match[1],level=match[2],text=match[3])
        else:
            text = text.rstrip(' ')
            match = re.fullmatch(r'(read|pull|clean) +(\S+)(?: +(\S+))?(?: +(\S+))?',text)
            if match:
                method,key,first,second = match.groups();call = {'method':method,'log':key}
                if first in rings.LEVELS:
                    call['level'] = first
                    if second is not None:
                        if method=='read': raise rings.RingError('invalid_request')
                        call['count' if method=='pull' else 'id'] = number(second)
                elif first is not None:
                    if method=='read' or second is not None: raise rings.RingError('invalid_request')
                    call['count' if method=='pull' else 'id'] = number(first)
            elif text in ('list','help','status'): call = {'method':text}
            elif re.fullmatch(r'sub +list',text): call = {'method':'sub.list'}
            else:
                match = re.fullmatch(r'sub +delete +(\S+)',text)
                if match: call = {'method':'sub.delete','id':match[1]}
                else:
                    match = re.fullmatch(r'sub +add +(\S+)(?: +(.*))?',text,re.S)
                    if match:
                        call = {'method':'sub.add','target':match[1]};rest = match[2] or ''
                        for key in ('log','level'):
                            if not rest or rest.startswith('/'): break
                            part = re.match(r'(\S+)(?: +(.*))?\Z',rest,re.S)
                            if not part: raise rings.RingError('invalid_request')
                            call[key] = part[1];rest = part[2] or ''
                        if rest:
                            if not rest.startswith('/'): raise rings.RingError('invalid_request')
                            call['filter'] = rest
        if call is None: raise rings.RingError('invalid_request')
    if not isinstance(call,dict) or not isinstance(call.get('method'),str) or call['method'] not in FIELDS:
        raise rings.RingError('invalid_request')
    method = call['method']
    if set(call)-FIELDS[method]-{'method'} or REQUIRED[method]-set(call): raise rings.RingError('invalid_request')
    for key,value in call.items():
        if key in ('count',) or key=='id' and method=='clean':
            rings.integer(value,1,500 if key=='count' else rings.SAFE)
        elif not isinstance(value,str) or '\0' in value:
            raise rings.RingError('invalid_request')
        else:
            try:value.encode('utf-8')
            except UnicodeError:raise rings.RingError('invalid_request') from None
    if 'log' in call: rings.name(call['log'])
    if 'level' in call: rings.level(call['level'])
    return call


def selector(value):
    if not isinstance(value,dict) or set(value)-{'log','level'}: raise ValueError()
    if 'log' in value: rings.name(value['log'])
    if 'level' in value: rings.level(value['level'])
    return value


def matches(value,event): return all(event.get(key)==wanted for key,wanted in value.items())


def direct_roles(message, source):
    envelope = getattr(message,'envelope',{})
    if not isinstance(envelope,Mapping): return set()
    original = envelope.get('original_to')
    if original not in (None,'') and (not isinstance(original,str) or original.lower()!=source.lower()): return set()
    roles = envelope.get('roles',())
    if not isinstance(roles,(list,tuple)): return set()
    return {r for r in roles if isinstance(r,str)}


def deadline(message):
    end = time.monotonic()+30
    value = message.envelope.get('deadline')
    if value and value != '0001-01-01T00:00:00Z':
        try:
            instant = datetime.fromisoformat(value.replace('Z','+00:00'))
            end = min(end,time.monotonic()+(instant-datetime.now(timezone.utc)).total_seconds())
        except (ValueError,TypeError,AttributeError): raise rings.RingError('deadline') from None
    return end


@contextmanager
def bus_budget(bus,end):
    """One serialized call's source-KV requests share its remaining deadline."""
    if not hasattr(bus,'_request'):
        yield
        return
    absent=object();prior=vars(bus).get('_request',absent);request=bus._request
    def bounded(*args,**options):
        remaining=end-time.monotonic()
        if remaining<=0:raise BusTimeout()
        requested=options.pop('timeout',None)
        options['timeout']=min(remaining,bus.timeout if requested is None else requested)
        return request(*args,**options)
    bus._request=bounded
    try:yield
    finally:
        if prior is absent:del bus._request
        else:bus._request=prior


class Agent:
    def __init__(self,bus,configuration,directory,*,clock=time.time,warning=None):
        rings.config(configuration)
        self.bus = bus
        self.warning = warning or (lambda code: print('eventlog: '+code,file=sys.stderr))
        self.rings = rings.Rings(directory,configuration,clock=clock)
        self.subs = Subscriptions(bus,bus.name,validate_selector=selector,selector_matches=matches,warning=self.warning,clock=clock)

    def handle(self,message):
        try:end=deadline(message)
        except rings.RingError as failure:return error(failure.code)
        with bus_budget(self.bus,end):return self._handle(message,end)

    def _handle(self,message,end):
        try:
            roles = direct_roles(message,self.bus.name)
            if message.ask=='status':
                self._allow(roles,{'read','write'})
                return 'eventlog: '+str(len(self.rings.list()['logs']))+' active logs'
            answer = dispatch(HERE,message.ask,roles=roles)
            if answer is not None: return answer
            call = parse(message.body);method = call.pop('method')
            if method=='help':return metadata_answer(HERE,roles=roles,as_json=message.body.lstrip().startswith('{'))
            wanted = SUB_ROLES if method.startswith('sub.') else {'write'} if method in ('write','pull','clean') else {'read','write'}
            self._allow(roles,wanted)
            if method=='status':return 'eventlog: '+str(len(self.rings.list()['logs']))+' active logs'
            if method=='write':
                event = {'log':call['log'],'level':call['level']}
                if end<=time.monotonic(): raise rings.RingError('deadline')
                candidates = self.subs.has_candidates(event)
                remaining = end-time.monotonic()
                if remaining<=0 or candidates and remaining<3: raise rings.RingError('deadline')
                entry = self.rings.write(call['log'],call['level'],call['text'])
                body = {'log':call['log'],**entry}
                result = self.subs.deliver(event,entry['text'],dumps(body),topic=call['log'],tag=call['level'],deadline=end)
                return dumps({'entry':entry,'deliveries':result})
            if method in ('read','pull','clean','list'):
                arguments = [] if method=='list' else [call['log']]
                if method!='list': arguments.append(call.get('level'))
                if method=='pull': arguments.append(call.get('count',100))
                if method=='clean': arguments.append(call.get('id'))
                return dumps(getattr(self.rings,method)(*arguments))
            # Subscription calls also sweep TTLs before answering.
            self.rings.list()
            if method=='sub.add':
                value = {key:call[key] for key in ('log','level') if key in call}
                return dumps(self.subs.add(message,call['target'],value,call.get('filter','')))
            if method=='sub.list': return dumps({'subscriptions':self.subs.list(message)})
            if method=='sub.delete': return dumps({'deleted':self.subs.delete(message,call['id'])})
            raise rings.RingError('invalid_request')
        except (rings.RingError,SubscriptionError) as failure:
            if failure.code.startswith('storage_') or failure.code=='store_unknown':self.warning(failure.code)
            return error(failure.code)
        except BusError:
            return error('admission_unavailable')
        except Exception:
            self.warning('internal_error')
            return error('internal_error')

    def _allow(self,roles,wanted):
        if not roles & (set(wanted)|{'owner','maintainer'}): raise rings.RingError('forbidden')

    def close(self):
        try:self.subs.close()
        finally:self.rings.close()


class Shutdown(BaseException): pass


def run():
    # Offline metadata is authored data, not permission to read the instance.
    ask = os.environ.get('AGENT_BUS_ASK')
    if ask:
        if ask=='status':print(error('forbidden'));return 0
        print(dispatch(HERE,ask,roles=set()))
        return 0
    def stop(*_): raise Shutdown()
    signal.signal(signal.SIGTERM,stop);signal.signal(signal.SIGINT,stop)
    agent = None
    try:
        with Bus(timeout=2) as bus:
            cfg = bus.record(bus.name).config
            agent = Agent(bus,cfg,os.environ.get('AGENT_BUS_WORK') or os.getcwd())
            while True:
                try: message = bus.consume(wait=1)
                except BusError:
                    print('eventlog: bus unavailable',file=sys.stderr);time.sleep(.2);continue
                if message is None or message.receipt or message.re: continue
                answer = agent.handle(message)
                try: message.reply(answer)
                except BusError: print('eventlog: answer delivery failed',file=sys.stderr)
    except Shutdown: return 0
    except (rings.RingError,BusError):
        print('eventlog: startup refused',file=sys.stderr);return 78
    finally:
        if agent is not None: agent.close()


if __name__=='__main__': sys.exit(run())
