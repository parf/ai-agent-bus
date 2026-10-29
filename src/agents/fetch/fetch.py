#!/usr/bin/env python3
"""One pooled call; all network work stays inside one hard-bounded child."""
import datetime
import json
import os
from pathlib import Path
import selectors
import signal
import subprocess
import sys
import time

HERE=Path(__file__).resolve().parent
sys.path[:0]=[str(HERE),str(HERE/'support/agents/shared' if (HERE/'support').exists() else HERE.parent/'shared')]
from metadata import dispatch
from extract import OUTPUT_MAX,WARNING

PIPE_MAX=2*1024*1024
FAILURE='error: could not fetch that URL'


class CallError(ValueError):
    pass


def pairs(values):
    result={}
    for key,value in values:
        if key in result:raise ValueError('duplicate key')
        result[key]=value
    return result


def loads(raw):
    return json.loads(raw,object_pairs_hook=pairs,parse_constant=lambda _:(_ for _ in ()).throw(ValueError()))


def parse(body):
    if not isinstance(body,str) or len(body.encode())>32768:raise ValueError()
    if body.lstrip().startswith('{'):
        value=loads(body)
        if not isinstance(value,dict):raise CallError('expected a get object')
        if set(value)-{'method','url','format'}:raise CallError('unknown call field')
        if value.get('method')!='get':raise CallError('use get <url> or JSON method get')
    else:value={'method':'get','url':body[4:] if body.startswith('get ') else body}
    if not isinstance(value.get('url'),str) or not 1<=len(value['url'].encode())<=4096 or value.get('format','text') not in ('text','markdown'):raise ValueError()
    return {'url':value['url'],'format':value.get('format','text')}


def remaining(envelope):
    seconds=60.0;stamp=envelope.get('deadline')
    if stamp and not stamp.startswith('0001-'):
        date=datetime.datetime.fromisoformat(stamp.replace('Z','+00:00'))
        if date.tzinfo is None:raise ValueError()
        seconds=min(seconds,date.timestamp()-time.time()-.1)
    if seconds<=0:raise TimeoutError()
    return seconds


def run(request,timeout,*,helper=None):
    payload=json.dumps({**request,'timeout':timeout},ensure_ascii=False).encode()
    env={key:os.environ[key] for key in ('PATH','LANG','LC_ALL','SSL_CERT_FILE','SSL_CERT_DIR') if key in os.environ}
    argv=[sys.executable,'-B',str(HERE/'worker.py')] if helper is None else helper
    process=subprocess.Popen(argv,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=env,start_new_session=True)
    selector=selectors.DefaultSelector();buffers={'out':bytearray(),'err':bytearray()};end=time.monotonic()+timeout
    try:
        for pipe,events,name in ((process.stdin,selectors.EVENT_WRITE,'in'),(process.stdout,selectors.EVENT_READ,'out'),(process.stderr,selectors.EVENT_READ,'err')):
            os.set_blocking(pipe.fileno(),False);selector.register(pipe,events,name)
        offset=0
        while selector.get_map():
            left=end-time.monotonic()
            if left<=0:raise TimeoutError()
            events=selector.select(left)
            if not events:raise TimeoutError()
            for key,_ in events:
                pipe,name=key.fileobj,key.data
                if name=='in':
                    try:offset+=os.write(pipe.fileno(),payload[offset:offset+65536])
                    except BrokenPipeError:selector.unregister(pipe);pipe.close();continue
                    if offset==len(payload):selector.unregister(pipe);pipe.close()
                else:
                    data=os.read(pipe.fileno(),65536)
                    if not data:selector.unregister(pipe);continue
                    buffers[name].extend(data)
                    if len(buffers['out'])+len(buffers['err'])>PIPE_MAX:raise ValueError()
        result=loads(bytes(buffers['out']).decode())
        if not isinstance(result,dict) or set(result)!={'ok','body','class','bytes'} or type(result['ok']) is not bool or not isinstance(result['body'],str) or len(result['body'].encode())>OUTPUT_MAX or result['class'] not in ('ok','url','port','address','dns','connection','protocol','downgrade','timeout','network','content','worker') or type(result['bytes']) is not int or not 0<=result['bytes']<=1<<20:raise ValueError()
        if result['ok'] and not result['body'].startswith(WARNING):raise ValueError()
    finally:
        selector.close()
        # Kill before reaping the leader, including a child holding a pipe open.
        try:os.killpg(process.pid,signal.SIGKILL)
        except ProcessLookupError:pass
        process.wait()
        for pipe in (process.stdin,process.stdout,process.stderr):
            if not pipe.closed:pipe.close()
    if process.returncode:raise ValueError()
    return result


def answer(envelope,config,*,execute=run):
    status='invalid';count=0
    try:
        if not isinstance(config,dict) or config:return 'error: Configuration must be empty'
        request=parse(envelope['body']);timeout=remaining(envelope)
        result=execute(request,timeout);status=result['class'];count=result['bytes']
        return result['body']
    except CallError as error:return 'error: '+str(error)
    except TimeoutError:status='timeout';return FAILURE
    except (OSError,ValueError,TypeError,KeyError,AttributeError,RecursionError,UnicodeError):return FAILURE
    finally:
        print(json.dumps({'status':status,'response_bytes':count}),file=sys.stderr)


def main():
    result=dispatch(HERE,os.environ.get('AGENT_BUS_ASK'))
    if result is not None:print(result);return 0
    try:
        raw=sys.stdin.buffer.read(65537)
        if len(raw)>65536:raise ValueError()
        envelope=loads(raw)
        if not isinstance(envelope,dict):raise ValueError()
        result=dispatch(HERE,envelope.get('ask'))
        if result is None:result=answer(envelope,loads(os.environ.get('AGENT_BUS_CONFIG','{}')))
    except (ValueError,TypeError,UnicodeError,RecursionError):result=FAILURE
    print(result);return 0


if __name__=='__main__':sys.exit(main())
