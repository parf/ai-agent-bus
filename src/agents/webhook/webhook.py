#!/usr/bin/env python3
"""Kept webhook bridge; Configuration and Secret are its own record's values."""
import json
import os
from pathlib import Path
import signal
import sys
import threading

HERE = Path(__file__).resolve().parent
SUPPORT = HERE / 'support' if (HERE / 'support').is_dir() else HERE.parents[1]
sys.path[:0] = [str(HERE), str(SUPPORT / 'clients/python'), str(SUPPORT / 'agents/shared')]
from agent_bus import Bus
from agent_bus.errors import BusError
from metadata import dispatch, data as metadata_data
from rules import Invalid, State, configuration, credentials, loads, parse_call, references
from incoming import Listener
from outgoing import post, Unknown

ROOT_HELP = 'Configuration is a root list of named inbound {name,path,target,...} or {name,path,publish,event,branches,...}, or outbound {name,url,...} routes; no object wrapper. See docs for optional Basic, signature, limits and examples.'


def address(raw):
    try:
        if not isinstance(raw, str) or len(raw) > 256 or any(ord(c) <= 32 or ord(c) >= 127 for c in raw): raise Invalid('listen address must be numeric host:port')
        host, port = raw.rsplit(':', 1)
        # The first version binds IPv4 only, without DNS or implicit wildcard names.
        import ipaddress
        ipaddress.IPv4Address(host)
        if not port.isdigit() or not 0 <= int(port) <= 65535: raise Invalid('listen port invalid')
        return host, int(port)
    except (ValueError, TypeError):
        raise Invalid('WEBHOOK_LISTEN must be numeric IPv4:port') from None


def publishable(bus, channel):
    # Checked once at start: the record is a pubsub and this agent may publish to it.
    try:
        return bus.record(channel).lookup().get('kind') == 'pubsub' and bus.may_send(bus.name, channel)
    except BusError:
        return False


class Agent:
    def __init__(self, bus, cfg=None, secret=b'', listen='127.0.0.1:8788'):
        self.bus = bus; self.routes = []; self.secrets = {}; self.listener = None
        self.state = State([]); self.problem = 'not configured'; self.stopping = False
        self.outbound = threading.BoundedSemaphore(1)
        try:
            self.routes = configuration(cfg); self.secrets = credentials(self.routes, secret)
            self.state = State(self.routes)
            for route in self.routes:
                if 'publish' in route and not publishable(bus, route['publish']):
                    self.problem = 'route ' + route['name'] + ' may not publish to its pubsub record'
                    return
            if any('path' in r for r in self.routes):
                self.listener = Listener(address(listen), self.routes, self.secrets, bus, self.state)
                self.listener.start()
            self.problem = None
            from urllib.parse import urlsplit
            import ipaddress
            for route in self.routes:
                if 'url' in route and 'basic' in route and urlsplit(route['url']).scheme == 'http':
                    try: loopback = ipaddress.ip_address(urlsplit(route['url']).hostname).is_loopback
                    except ValueError: loopback = False
                    if not loopback:
                        self.state.log('-', 0, 'plaintext-basic-auth-configured'); break
        except OSError:
            self.listener = None; self.problem = 'listener bind unavailable'
        except Invalid:
            # No routes are partially enabled when any validation, Secret or bind fails.
            if self.listener is not None: self.listener.stop()
            self.listener = None; self.problem = 'Configuration, required Secret or listener unavailable'
    def status(self):
        if self.stopping: return 'stopping: webhook draining'
        if self.problem: return 'failed: ' + self.problem
        if self.listener and not self.listener.thread.is_alive(): return 'failed: listener stopped'
        counts = self.state.snapshot()
        listening = ('listening '+str(self.listener.server_address[0])+':'+str(self.listener.server_port)) if self.listener else 'outbound only'
        active, preauth = self.listener.activity() if self.listener else (0, 0)
        word = 'degraded' if counts.get('unknown', 0) else 'ok'
        return f"{word}: {listening}; active {active}, preauth {preauth}; accepted {counts.get('accepted', 0)}, refused {sum(v for k,v in counts.items() if k not in ('accepted','unknown','outbound'))}, unknown {counts.get('unknown', 0)}"
    def metadata(self, ask):
        if ask == 'help-config': return ROOT_HELP
        if ask == 'help-secret':
            return '\n'.join(key + ': required bus Secret env-file entry (never a host env credential)' for key in references(self.routes)) or 'No Secret values read by help; configure route key references first. ' + ROOT_HELP
        if ask == 'help-json':
            value = metadata_data(HERE, ask)
            value['config'] = {'(root)': {'type':'list', 'help':ROOT_HELP}}
            value['secrets'] += [{'name':key, 'help':'bus Secret env-file entry referenced by a route', 'required':True, 'has_default':False} for key in references(self.routes)]
            return json.dumps(value, ensure_ascii=False)
        raise Invalid('unknown help ask')
    def handle(self, message):
        result = dispatch(HERE, message.ask, handlers={
            'status': self.status, 'help-config': lambda:self.metadata('help-config'),
            'help-secret': lambda:self.metadata('help-secret'), 'help-json': lambda:self.metadata('help-json')})
        if result is not None: return result
        # Fire-and-forget inbound forwarding may receive a target's answer.
        # It is not a new outbound command and must not create an answer loop.
        if message.envelope.get('re'): return None
        if self.problem: return 'error: ' + self.problem
        try:
            call = parse_call(message.body)
            if call['method'] == 'routes':
                return json.dumps([{'name':r['name'], 'direction':'in' if 'path' in r else 'out', 'counts':self.state.route_snapshot(r['name'])} for r in self.routes])
            route = next((r for r in self.routes if r['name'] == call['route'] and 'url' in r), None)
            if route is None: raise Invalid('unknown outbound route')
            if len(call['body'].encode()) > route['max_bytes']: raise Invalid('outbound body byte limit')
            if not self.outbound.acquire(blocking=False): return 'error: outbound call busy'
            try: answer = post(route, call['body'], self.secrets, message.envelope)
            finally: self.outbound.release()
            self.state.count('unknown' if answer.get('unknown') else 'outbound', route['name'])
            self.state.log(route['name'], len(call['body'].encode()), 'unknown' if answer.get('unknown') else answer.get('status', 'failed'))
            return json.dumps(answer, ensure_ascii=False)
        except Unknown:
            self.state.count('unknown'); self.state.log('-', 0, 'unknown')
            return 'unknown: outbound delivery outcome unknown; never retried'
        except (Invalid, UnicodeError, ValueError):
            return 'error: invalid outbound call, route or configured limit'
    def begin_stop(self):
        if self.stopping: return
        self.stopping = True
        if self.listener: self.listener.stopping = True
        self.state.log('-', 0, 'stopping')
    def close(self):
        self.begin_stop()
        if self.listener: self.listener.stop()


def direct_help(ask):
    # The package's offline asks must never open a listener, fetch Secret or use the bus.
    agent = Agent.__new__(Agent); agent.routes = []; agent.problem = 'not configured'; agent.stopping = False
    agent.listener = None; agent.state = State([])
    try:
        if os.environ.get('AGENT_BUS_CONFIG'): agent.routes = configuration(loads(os.environ['AGENT_BUS_CONFIG']))
    except Invalid: pass
    return dispatch(HERE, ask, handlers={'status':lambda:'unknown: webhook not started',
        'help-config':lambda:agent.metadata('help-config'), 'help-secret':lambda:agent.metadata('help-secret'), 'help-json':lambda:agent.metadata('help-json')})


def main():
    answer = direct_help(os.environ.get('AGENT_BUS_ASK'))
    if answer is not None: print(answer); return 0
    stop = threading.Event()
    agent = None
    def stopping(*_):
        stop.set()
        if agent: agent.begin_stop()
    for sig in (signal.SIGINT, signal.SIGTERM): signal.signal(sig, stopping)
    try:
        with Bus(timeout=3) as bus:
            try:
                # Raw read preserves duplicate-key refusal; the client's JSON decoder
                # would otherwise erase duplicate Configuration fields before validation.
                cfg = loads(bus._request('GET', '/config', query={'name':bus.name}, raw=True).decode())
                routes = configuration(cfg)
                secret = bus.secret.reveal() if references(routes) else b''
            except (BusError, Invalid, UnicodeError): cfg, secret = None, b''
            agent = Agent(bus, cfg, secret, os.environ.get('WEBHOOK_LISTEN', '127.0.0.1:8788'))
            if stop.is_set(): agent.begin_stop()
            try: bus.serve(agent.handle, workers=2, stop=stop, drain_timeout=12)
            finally: agent.close()
        return 0
    except BusError:
        print('webhook: bus unavailable', file=sys.stderr); return 1
    finally:
        if agent and not agent.stopping: agent.close()


if __name__ == '__main__': sys.exit(main())
