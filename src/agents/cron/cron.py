#!/usr/bin/env python3
"""#cron: recurring subscriptions and one-shot `later` calls, on the shared scheduler engine."""
from datetime import datetime, timezone
import json
from pathlib import Path
import re
import sys

HERE = Path(__file__).resolve().parent
SUPPORT = HERE / 'support' if (HERE / 'support').is_dir() else HERE.parents[1]
sys.path.insert(0, str(SUPPORT / 'clients/python'))
sys.path.insert(0, str(SUPPORT / 'agents/shared'))
import cronexpr  # noqa: E402
from scheduler import (BUS_NAME, ID, MAX_BODY, MAX_SUBS, SUBS_EACH, CallError, Scheduler,  # noqa: E402,F401
                       MAX_LATER, LATER_EACH, run)

FIELDS = {'sub.add': {'cron', 'target', 'body'}, 'sub.list': set(), 'sub.delete': {'id'},
          'later': {'in', 'at', 'target', 'body'}, 'later.list': set(), 'later.cancel': {'id'}, 'help': set()}
TEXT = (
    (re.compile(r'sub +add +(\S+) +(\S+) +(\S+) +(\S+) +(\S+) +(\S+) (.*)\Z', re.S),
     lambda m: {'method': 'sub.add', 'cron': ' '.join(m.groups()[:5]), 'target': m[6], 'body': m[7]}),
    (re.compile(r'sub +list *\Z'), lambda m: {'method': 'sub.list'}),
    (re.compile(r'sub +delete +(\S+) *\Z'), lambda m: {'method': 'sub.delete', 'id': m[1]}),
    (re.compile(r'later +list *\Z'), lambda m: {'method': 'later.list'}),
    (re.compile(r'later +cancel +(\S+) *\Z'), lambda m: {'method': 'later.cancel', 'id': m[1]}),
    (re.compile(r'later +(\S+) +(\S+) (.*)\Z', re.S),
     lambda m: {'method': 'later', 'in' if m[1].startswith('+') else 'at': m[1], 'target': m[2], 'body': m[3]}),
    (re.compile(r'help *\Z'), lambda m: {'method': 'help'}),
)


def parse(body):
    """A text call (the message is the rest of the line, verbatim) or its JSON form."""
    if body.lstrip().startswith('{'):
        try:
            call = json.loads(body)
        except ValueError:
            raise CallError('invalid JSON call') from None
    else:
        for pattern, build in TEXT:
            m = pattern.match(body.lstrip(' '))
            if m:
                call = build(m)
                break
        else:
            raise CallError('use sub add|list|delete, later, later list|cancel or help; a message needs all its arguments')
    if not isinstance(call, dict) or call.get('method') not in FIELDS:
        raise CallError('unknown method; ask help')
    method = call['method']
    if set(call) - FIELDS[method] - {'method'}:
        raise CallError('unknown method argument')
    if any(not isinstance(v, str) for k, v in call.items()):
        raise CallError('arguments are text')
    need = {'sub.add': ('cron', 'target', 'body'), 'sub.delete': ('id',), 'later': ('target', 'body'),
            'later.cancel': ('id',)}.get(method, ())
    for key in need:
        if key not in call or key != 'body' and not call[key]:
            raise CallError(key + ' is required')
    if method == 'later' and ('in' in call) == ('at' in call):
        raise CallError('later takes one of in (+90s, +15m, +2h, +3d) or at (RFC 3339 with an offset)')
    if 'body' in call and len(call['body'].encode()) > MAX_BODY:
        raise CallError(f'a message is at most {MAX_BODY} bytes')
    if 'target' in call and not BUS_NAME.fullmatch(call['target']):
        raise CallError('target must be a bus name: a queue or an agent')
    if 'id' in call and not ID.fullmatch(call['id']):
        raise CallError('unknown id')
    return call


class Agent(Scheduler):
    NAME = 'cron'
    PACKAGE = HERE
    LATER_TOPIC = 'later'
    METHODS = {'later': 'later_add'}
    parse = staticmethod(parse)

    def sub_add(self, call, message):
        spec = cronexpr.parse(call['cron'])
        self.target(call)
        who = self.caller(message)
        if len(self.subs) >= MAX_SUBS or sum(s['by'] == who for s in self.subs.values()) >= SUBS_EACH:
            raise CallError(f'at most {MAX_SUBS} subscriptions, {SUBS_EACH} per creator')
        self.author(message, call['target'])
        sub = {'cron': call['cron'], 'target': call['target'], 'body': call['body'], 'by': who, 'created': self.clock()}
        key = self.store('sub', sub)
        self.subs[key] = sub | {'spec': spec}
        return {'id': key, 'next': self.next(spec)}

    def sub_list(self, call, message):
        return {'subs': self.listing(message, self.subs, lambda k, s: {'cron': s['cron'], 'next': self.next(s['spec'])}),
                'tz': str(self.cfg['tz']),
                'outcomes_since': datetime.fromtimestamp(self.started, timezone.utc).isoformat(), **self.check_status()}

    def sub_delete(self, call, message):
        return self.remove(message, self.subs, call['id'], 'sub')


if __name__ == '__main__':
    sys.exit(run(Agent))
