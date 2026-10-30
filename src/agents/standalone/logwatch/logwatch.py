#!/usr/bin/env python3
"""Long-lived standalone log history, served by the shared Python bus client."""
import json
import os
from pathlib import Path
import queue
import re
import shlex
import signal
import sys
import threading
import time

# The shared starter runs the installed script in the checkout, not a copy.
sys.path.insert(0, str(Path(__file__).resolve().parents[3] / 'clients/python'))
from agent_bus import Bus
from agent_bus._version import VERSION
from agent_bus.errors import BusError
from filters import match
from watcher import Monitor, Ring, configuration
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "shared"))
from metadata import answer as metadata_answer, ASKS, dispatch
import subscriptions as shared_subscriptions
from subscription_migration import migrate
PACKAGE = Path(__file__).resolve().parent

FIELDS = {'tail': {'n'}, 'grep': {'query', 'n'}, 'since': {'age', 'n'}, 'mark': {'label'},
          'after': {'label'}, 'sources': set(), 'stats': set(), 'sub.add': {'target', 'filter'},
          'sub.list': set(), 'sub.delete': {'id'}, 'help': set()}
BUS_NAME = re.compile(r'(?:#[a-z0-9][a-z0-9._/-]*@[a-z0-9][a-z0-9.-]*|[a-z0-9][a-z0-9._-]*(?:@[a-z0-9][a-z0-9.-]*)?)\Z')


def parse(body):
    if body.lstrip().startswith('{'):
        call = json.loads(body)
    else:
        words = shlex.split(body)
        if not words: return {'method': 'help'}
        method = words.pop(0)
        if method == 'sub':
            if not words: raise ValueError('sub takes add, list or delete')
            method += '.' + words.pop(0)
        if method == 'sub.edit': raise ValueError('delete it and add it again')
        call = {'method': method}
        if method in ('tail', 'grep', 'since') and words and words[-1].isdigit() and (method == 'tail' or len(words) > 1):
            call['n'] = int(words.pop())
        if method in ('grep', 'since', 'mark', 'after') and words:
            call[{'grep':'query', 'since':'age', 'mark':'label', 'after':'label'}[method]] = ' '.join(words); words = []
        if method == 'sub.add' and words:
            call['target'] = words.pop(0)
            if words: call['filter'] = ' '.join(words); words = []
        if method == 'sub.delete' and len(words) == 1: call['id'] = words.pop()
        if words: raise ValueError('unexpected arguments')
    if isinstance(call, dict) and call.get('method') == 'sub.edit': raise ValueError('delete it and add it again')
    if not isinstance(call, dict) or not isinstance(call.get('method'), str) or call['method'] not in FIELDS:
        raise ValueError('unknown method; ask help')
    method = call['method']
    if set(call) - FIELDS[method] - {'method'}: raise ValueError('unknown method argument')
    if 'n' in call and (type(call['n']) is not int or not 1 <= call['n'] <= 100000): raise ValueError('n must be 1..100000')
    for key in ['query', 'age', 'label', 'target', 'filter', 'id']:
        if key in call and not isinstance(call[key], str): raise ValueError(key + ' must be text')
    for key in {'grep': ['query'], 'since': ['age'], 'after': ['label'], 'sub.add': ['target'], 'sub.delete': ['id']}.get(method, []):
        if not call.get(key): raise ValueError(key + ' is required')
    if 'label' in call and not re.fullmatch(r'[A-Za-z0-9_.-]{1,100}', call['label']): raise ValueError('invalid mark label')
    if method == 'since' and not re.fullmatch(r'[1-9][0-9]*[smhd]', call['age']): raise ValueError('age is a positive duration such as 30s or 5m')
    if 'target' in call and not BUS_NAME.fullmatch(call['target']): raise ValueError('target must be a bus name')
    return call


def selector(value):
    if value != {}: raise ValueError('logwatch has no selector argument')
    return {}


def display(line):
    return f"[{line['source']}] {line['text']}"


class Agent:
    def __init__(self, bus, config, log=lambda text: print('logwatch: ' + text, file=sys.stderr), clock=time.time, *, delivery=None):
        self.bus, self.config, self.log, self.clock = bus, config, log, clock
        self.ring = Ring(config['ring_size'], clock, config['line_bytes'], config['ring_bytes'])
        self.monitor = Monitor(config, self.ring, log)
        self.started = clock()
        self.stop = threading.Event()
        self.subscriptions = shared_subscriptions.Subscriptions(
            bus, bus.name, validate_selector=selector, selector_matches=lambda _selector, _event: True,
            warning=lambda code: self.log('subscription: ' + code), clock=lambda: self.clock(), delivery=delivery)
        self.lock = threading.RLock()
        self.queue = queue.Queue(maxsize=1000)
        self.forward_dropped = 0
        self.ring.on_line = self.enqueue
        self.threads = []

    def enqueue(self, line):
        try: self.queue.put_nowait(line)
        except queue.Full:
            self.forward_dropped += 1
            self.log('forward queue full; line dropped')

    def load(self):
        # No handler/forwarder starts until interrupted legacy moves are finished.
        migrate(self.subscriptions, self.clock)

    def subscription(self, call, message):
        if call['method'] == 'sub.list':
            return {'subs': self.subscriptions.list(message)}
        if call['method'] == 'sub.add':
            row = self.subscriptions.add(message, call['target'], {}, call.get('filter', ''))
            return {'id': row['id']}
        self.subscriptions.delete(message, call['id'])
        return {'deleted': call['id']}

    def handle(self, message):
        try:
            result = dispatch(PACKAGE, message.ask)
            if result is not None: return result
            call = parse(message.body); method = call['method']
            if method == 'help': return metadata_answer(PACKAGE, as_json=message.body.lstrip().startswith('{'))
            if method.startswith('sub.'): result = self.subscription(call, message)
            elif method == 'sources': result = {'sources': self.monitor.sources()}
            elif method == 'stats':
                result = {'version': VERSION, 'uptime': self.clock()-self.started, 'ring_size': self.ring.size,
                          'fill': len(self.ring.snapshot()), 'ring_bytes': self.ring.bytes, 'ring_byte_capacity': self.ring.ring_bytes,
                          'lines_seen': self.ring.total,
                          'dropped': max(0, self.ring.total-len(self.ring.snapshot())), 'forward_dropped': self.forward_dropped}
            elif method == 'mark': result = {'mark': self.ring.mark(call.get('label'))}
            else:
                lines = self.ring.after(call['label']) if method == 'after' else self.ring.snapshot()
                if method == 'grep': lines = [line for line, yes in zip(lines, match(call['query'], [line['text'] for line in lines])) if yes]
                if method == 'since':
                    age = int(call['age'][:-1]) * {'s':1,'m':60,'h':3600,'d':86400}[call['age'][-1]]
                    lines = [line for line in lines if line['at'] >= self.clock()-age]
                if method != 'after': lines = lines[-call.get('n', 20):]
                result = {'lines': lines}
            if message.body.lstrip().startswith('{'): return json.dumps(result)
            if 'lines' in result and isinstance(result['lines'], list): return '\n'.join(map(display, result['lines'])) or '(no lines)'
            if 'mark' in result: return result['mark']
            return json.dumps(result, indent=2)
        except (ValueError, BusError, OSError) as error:
            return 'error: ' + str(error)

    def forward(self, lines):
        # Each arrival is a fresh match, including repeated source/text pairs.
        # The engine alone owns admission, filtering, send attempts and counts.
        with self.lock:
            for line in lines:
                if self.stop.is_set(): break
                try:
                    self.subscriptions.deliver({}, line['text'], display(line), topic='logwatch',
                                               tag=line['source'], deadline=time.monotonic()+5)
                except (shared_subscriptions.SubscriptionError, BusError, OSError) as error:
                    self.log('subscription: ' + (error.code if isinstance(error, shared_subscriptions.SubscriptionError) else 'store_unknown'))

    def start(self):
        # Initial history is queried, not forwarded as new subscription traffic.
        self.monitor.tick()
        while not self.queue.empty(): self.queue.get_nowait()
        def monitor():
            while not self.stop.wait(.1):
                try: self.monitor.tick()
                except OSError as error: self.log(type(error).__name__ + ' reading sources')
        def forward():
            while not self.stop.is_set():
                lines = []
                try: lines.append(self.queue.get(timeout=.2))
                except queue.Empty: pass
                while len(lines) < 256:
                    try: lines.append(self.queue.get_nowait())
                    except queue.Empty: break
                self.forward(lines)
        for run in [monitor, forward]:
            thread = threading.Thread(target=run, daemon=True); thread.start(); self.threads.append(thread)

    def close(self):
        self.stop.set()
        self.subscriptions.close()
        for thread in self.threads: thread.join(timeout=5)
        self.monitor.close()


def main():
    with Bus(timeout=5) as bus:
        config = configuration(bus.config)
        agent = Agent(bus, config)
        signal.signal(signal.SIGTERM, lambda *_: agent.stop.set())
        signal.signal(signal.SIGINT, lambda *_: agent.stop.set())
        try:
            agent.load(); agent.start()
            bus.serve(agent.handle, workers=4, stop=agent.stop, drain_timeout=10)
        finally: agent.close()


if __name__ == '__main__': main()
