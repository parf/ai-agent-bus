#!/usr/bin/env python3
"""One kept, sandboxed app identity; one worker for record and cache changes."""
import json
import os
from pathlib import Path
import signal
import sys
import threading

HERE=Path(__file__).resolve().parent
SUPPORT=HERE/'support' if (HERE/'support').is_dir() else HERE.parents[1]
sys.path.insert(0,str(SUPPORT/'clients/python'))
sys.path.insert(0,str(SUPPORT/'agents/shared'))
sys.path.insert(0,str(HERE))
from agent_bus import Bus
from agent_bus.errors import BusError
from metadata import dispatch
from blocks import Blocks,OwnKV,Budget
from core import Core
from rules import RagError,Unknown,Incomplete,config,dumps,loads,parse,BODY_MAX,RECORD_MAX,DATA_MAX,CHUNK_MAX,SCALARS_MAX,QUERY_MAX,SEARCH_MODES,METHODS,CATALOGUE_MAX,NAMESPACE_MAX,TABLES_MAX

VERSION=json.loads((HERE/'agent.json').read_text())['version']


class Agent:
    def __init__(self,bus,cfg):
        self.cfg=None;self.core=None;self.problem='configure RAG block names and collection'
        try:
            cfg=config(cfg);self.cfg=cfg
            backend=Blocks(bus,cfg)
            kv=OwnKV(bus,lambda:self.core.budget)
            self.core=Core(cfg,backend,kv)
        except RagError:pass
    def capabilities(self):
        # Static discovery is deliberately independent of provider and storage readiness.
        return {'version':VERSION,'methods':list(METHODS),'modes':list(SEARCH_MODES),
                'limits':{'body_bytes':BODY_MAX,'record_bytes':RECORD_MAX,'data_bytes':DATA_MAX,
                          'chunks':CHUNK_MAX,'embedding_scalars':SCALARS_MAX,'query_bytes':QUERY_MAX,
                          'page_records':16,'catalogue_bytes':CATALOGUE_MAX,'namespaces':NAMESPACE_MAX,'tables_per_namespace':TABLES_MAX},
                'embedding_model':None if self.cfg is None else self.cfg['embedding_model'],
                'dimensions':None if self.cfg is None else self.cfg['dimensions']}
    def status(self,envelope=None):
        if self.core is not None:
            try:
                self.core.budget=Budget(envelope);self.core.backend.begin(self.core.budget);self.core.verify()
            except (RagError,BusError):self.core.health='failed: collection verification failed'
        return ('failed: '+self.problem if self.core is None else self.core.health)+' (RAG '+VERSION+')'
    def handle(self,message):
        result=dispatch(HERE,message.ask,handlers={'status':lambda:self.status(message.envelope)},roles=message.roles)
        if result is not None:return result
        method='invalid';status='error'
        try:
            request=parse(message.body);method=request['method']
            if method=='capabilities':
                status='ok';return dumps(self.capabilities())
            if self.core is None:raise RagError(self.problem)
            result=dumps(self.core.run(request,message.envelope))
            status='ok';return result
        except Incomplete as error:
            status='incomplete';return 'incomplete: '+str(error)
        except Unknown as error:
            status='unknown';return 'unknown: '+str(error)
        except RagError as error:return 'error: '+str(error)
        except BusError:
            if method=='prompts' and request.get('action') in ('set','delete'):
                status='unknown';return 'unknown: prompt write outcome unknown; read it before repeating'
            return 'error: bus request failed'
        except (KeyError,TypeError,ValueError,OverflowError,AttributeError):
            return 'error: invalid request, configuration or block response'
        finally:
            print(json.dumps({'method':method,'status':status}),file=sys.stderr)


def main():
    # Direct package help must work without an identity, key, or Configuration.
    result=dispatch(HERE,os.environ.get('AGENT_BUS_ASK'),handlers={'status':lambda:'unknown: RAG '+VERSION+' has not verified its collection'})
    if result is not None:print(result);return 0
    stop=threading.Event()
    signal.signal(signal.SIGTERM,lambda *_:stop.set())
    signal.signal(signal.SIGINT,lambda *_:stop.set())
    try:
        with Bus(timeout=10) as bus:
            try:cfg=bus.config
            except BusError:cfg=None
            agent=Agent(bus,cfg)
            bus.serve(agent.handle,workers=1,stop=stop,drain_timeout=3)
        return 0
    except BusError:
        print('rag: bus unavailable',file=sys.stderr);return 1


if __name__=='__main__':sys.exit(main())
