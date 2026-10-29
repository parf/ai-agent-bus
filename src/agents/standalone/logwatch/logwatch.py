#!/usr/bin/env python3
"""Long-lived standalone log history, served by the shared Python bus client."""
from collections import OrderedDict
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
import uuid

# The shared starter runs the installed script in the checkout, not a copy.
sys.path.insert(0, str(Path(__file__).resolve().parents[3] / 'clients/python'))
from agent_bus import Bus
from agent_bus._version import VERSION
from agent_bus.errors import BusError, Refusal, UnexpectedAnswer
from filters import match
from watcher import Monitor, Ring, configuration
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "shared"))
from metadata import answer as metadata_answer, ASKS, dispatch
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


def display(line):
    return f"[{line['source']}] {line['text']}"


class Agent:
    def __init__(self, bus, config, log=lambda text: print('logwatch: ' + text, file=sys.stderr), clock=time.time):
        self.bus, self.config, self.log, self.clock = bus, config, log, clock
        self.ring = Ring(config['ring_size'], clock, config['line_bytes'], config['ring_bytes'])
        self.monitor = Monitor(config, self.ring, log)
        self.started = clock()
        self.stop = threading.Event()
        self.subs = {}
        self.fold = {}
        self.lock = threading.RLock()
        self.queue = queue.Queue(maxsize=1000)
        self.forward_dropped = 0
        self.check_down = False
        self.ring.on_line = self.enqueue
        self.threads = []

    def enqueue(self, line):
        try: self.queue.put_nowait(line)
        except queue.Full:
            self.forward_dropped += 1
            self.log('forward queue full; line dropped')

    def load(self):
        for entry in self.bus.kv.list().get('json', []):
            if entry['name'].startswith('sub.'):
                value = self.bus.kv.get(entry['name'], kind='json')
                self.subs[value['id']] = value | {'forwarded': 0}

    def authorized(self, message, target):
        if message.original_to in (None, '', self.bus.name) and {'owner', 'maintainer'} & set(message.roles): return
        if self.bus._request('GET', '/manages', query={'who': getattr(message, 'from'), 'name': target}).get('manages') is not True:
            raise ValueError('subscription changes require management of destination ' + target)

    def author_may_send(self, who, target):
        """The daemon's yes or no for who; a refused destination is a ValueError, anything else that is not an answer is BusError."""
        try: yes = self.bus.may_send(who, target)
        except Refusal as error:
            if error.status == 404: raise ValueError('no such destination ' + target) from error
            raise
        except UnexpectedAnswer as error:
            raise ValueError('cannot check that ' + who + ' may send to ' + target + ': the daemon does not answer may-send for another name; nothing stored') from error
        if not yes: raise ValueError(who + ' may not send to ' + target)

    def pause(self, sub, reason):
        """A subscription whose author may no longer send stops, visibly, until it is deleted."""
        sub['paused'] = reason
        self.log('subscription ' + sub['id'] + ' paused: ' + reason)
        stored = {k: v for k, v in sub.items() if k in ('id', 'target', 'filter', 'created_by', 'paused')}
        try: self.bus.kv.set('sub.' + sub['id'], stored, kind='json', mode='replace')
        except (BusError, OSError) as error: self.log('subscription ' + sub['id'] + ': pause not stored (' + type(error).__name__ + ')')

    def subscription(self, call, message):
        with self.lock:
            method = call['method']
            if method == 'sub.list': return {'subs': [dict(s) for s in self.subs.values()]}
            if method == 'sub.add':
                self.authorized(message, call['target'])
                self.author_may_send(getattr(message, 'from'), call['target'])
                query = call.get('filter', '')
                match(query, [])
                sub = {'id': uuid.uuid4().hex[:12], 'target': call['target'], 'filter': query,
                       'created_by': getattr(message, 'from')}
                self.bus.kv.set('sub.' + sub['id'], sub, kind='json', mode='add')
                self.subs[sub['id']] = sub | {'forwarded': 0}
                return {'id': sub['id']}
            sub = self.subs.get(call['id'])
            if not sub: raise ValueError('unknown subscription ' + call['id'])
            self.authorized(message, sub['target'])
            self.bus.kv.delete('sub.' + sub['id'], kind='json')
            del self.subs[sub['id']]; self.fold.pop(sub['id'], None)
            return {'deleted': sub['id']}

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

    def send(self, sub, line, count=1):
        if sub.get('paused'): return
        # The author's own right to send to the target, asked again before every
        # send; a check that cannot be answered sends nothing, and is not asked
        # again for the rest of this batch.
        who = sub.get('created_by')
        if not isinstance(who, str) or not who: self.pause(sub, 'author unknown'); return
        allowed = None
        if not self.check_down:
            try: allowed = self.bus.may_send(who, sub['target'])
            except Refusal as error:
                if error.status == 404: self.pause(sub, 'destination gone'); return
                self.check_down = True
            except (BusError, OSError): self.check_down = True
        if allowed is None:
            if sub.get('last_error') != 'unavailable': self.log('subscription ' + sub['id'] + ': may-send check unavailable; not sending')
            sub['last_error'] = 'unavailable'
            return
        if not allowed: self.pause(sub, 'denied'); return
        if sub.get('last_error') == 'unavailable': sub.pop('last_error')
        body = display(line)
        if count > 1: body += f'\n×{count} in 5 min'
        try:
            self.bus.send(sub['target'], body, topic='logwatch', tag=line['source'])
            sub['forwarded'] += 1
        except BusError as error:
            if sub.get('last_error') != type(error).__name__:
                self.log('subscription ' + sub['id'] + ': send failed (' + type(error).__name__ + ')')
            sub['last_error'] = type(error).__name__

    def forward(self, lines):
        with self.lock:
            self.check_down = False
            for sub in self.subs.values():
                if sub.get('disabled') or sub.get('paused'): continue
                try: selected = match(sub['filter'], [line['text'] for line in lines]) if lines else []
                except ValueError as error:
                    sub['disabled'] = True; sub['last_error'] = str(error); self.log('subscription ' + sub['id'] + ': ' + str(error)); continue
                folded = self.fold.setdefault(sub['id'], OrderedDict())
                for line, yes in zip(lines, selected):
                    if not yes: continue
                    key = (line['source'], line['text'])
                    old = folded.get(key)
                    if old and self.clock() - old['at'] < 300: old['count'] += 1; continue
                    if old and old['count'] > 1: self.send(sub, old['line'], old['count'])
                    self.send(sub, line)
                    folded[key] = {'at': self.clock(), 'count': 1, 'line': line}
                    while len(folded) > self.config['ring_size']:
                        _, old = folded.popitem(last=False)
                        if old['count'] > 1: self.send(sub, old['line'], old['count'])
                for key, old in list(folded.items()):
                    if self.clock() - old['at'] >= 300:
                        if old['count'] > 1: self.send(sub, old['line'], old['count'])
                        del folded[key]

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
