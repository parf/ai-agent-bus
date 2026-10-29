#!/usr/bin/env python3
"""#at: one deferred call, `in 30m` or `at 09:30`, on the shared scheduler engine (RD139, RD140)."""
from datetime import datetime, timedelta, timezone
import json
from pathlib import Path
import re
import sys

HERE = Path(__file__).resolve().parent
SUPPORT = HERE / 'support' if (HERE / 'support').is_dir() else HERE.parents[1]
sys.path.insert(0, str(SUPPORT / 'clients/python'))
sys.path.insert(0, str(SUPPORT / 'agents/shared'))
import cronexpr  # noqa: E402
from scheduler import BUS_NAME, ID, MAX_BODY, CallError, Scheduler, run  # noqa: E402

FIELDS = {'in': {'in', 'target', 'body'}, 'at': {'at', 'target', 'body'}, 'list': set(), 'cancel': {'id'}, 'help': set()}
# The command word is the first token only; the message is the rest of the line, verbatim.
TEXT = (
    (re.compile(r'(in|at) +(\S+) +(\S+) (.+)\Z', re.S), lambda m: {'method': m[1], m[1]: m[2], 'target': m[3], 'body': m[4]}),
    (re.compile(r'list *\Z'), lambda m: {'method': 'list'}),
    (re.compile(r'cancel +(\S+) *\Z'), lambda m: {'method': 'cancel', 'id': m[1]}),
    (re.compile(r'help *\Z'), lambda m: {'method': 'help'}),
)
DURATION = re.compile(r'\+?([1-9][0-9]{0,6})([smhd])\Z')
CLOCK = re.compile(r'([01][0-9]|2[0-3]):([0-5][0-9])\Z')


def parse(body):
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
            raise CallError('use in <duration> <target> <message>, at <time> <target> <message>, list, cancel <id> or help')
    if not isinstance(call, dict) or call.get('method') not in FIELDS:
        raise CallError('unknown method; ask help')
    method = call['method']
    if set(call) - FIELDS[method] - {'method'}:
        raise CallError('unknown method argument')
    if any(not isinstance(v, str) for v in call.values()):
        raise CallError('arguments are text')
    for key in FIELDS[method]:
        if not call.get(key):
            raise CallError(key + ' is required' + (': an empty message is refused' if key == 'body' else ''))
    if 'body' in call and len(call['body'].encode()) > MAX_BODY:
        raise CallError(f'a message is at most {MAX_BODY} bytes')
    if 'target' in call and not BUS_NAME.fullmatch(call['target']):
        raise CallError('target must be a bus name: a queue or an agent')
    if 'id' in call and not ID.fullmatch(call['id']):
        raise CallError('unknown id')
    return call


def next_clock(text, now, zone):
    """HH:MM: the earliest instant strictly after the current minute whose local time is HH:MM.
    Both occurrences of a repeated (fall-back) time count; a time in a spring-forward gap
    that would come first is refused with the nearest valid time, never shifted."""
    m = CLOCK.fullmatch(text)
    current = cronexpr.wall(datetime.fromtimestamp(now, timezone.utc), zone)
    after = (now // 60 + 1) * 60
    candidates, gaps = [], []
    for days in range(3):
        wall = (current + timedelta(days=days)).replace(hour=int(m[1]), minute=int(m[2]))
        if not cronexpr.exists(wall, zone):
            gaps.append(wall)
            continue
        for fold in (0, 1):
            stamp = wall.replace(tzinfo=zone, fold=fold).timestamp()
            if stamp >= after:
                candidates.append((stamp, wall))
    stamp, wall = min(candidates)
    gap = next((g for g in gaps if current < g < wall), None)
    if gap is not None:
        valid = gap
        while not cronexpr.exists(valid, zone):
            valid += timedelta(minutes=1)
        name = valid.replace(tzinfo=zone).tzname()
        raise CallError(f'{text} does not exist on {gap.date()} in {zone}: the clocks skip it; '
                        f'the nearest valid time is {valid:%H:%M} {name}')
    return stamp


class Agent(Scheduler):
    NAME = 'at'
    PACKAGE = HERE
    LATER_TOPIC = 'at'
    METHODS = {'in': 'at_in', 'at': 'at_at', 'list': 'later_list', 'cancel': 'later_cancel'}
    KINDS = ('later',)
    parse = staticmethod(parse)

    def resolved(self, answer):
        """Echo the instant in the agent's zone with its offset, and the zone's name."""
        instant = datetime.fromisoformat(answer['at']).astimezone(self.cfg['tz'])
        return {'id': answer['id'], 'at': instant.isoformat(), 'tz': str(self.cfg['tz'])}

    def at_in(self, call, message):
        m = DURATION.fullmatch(call['in'])
        if not m:
            raise CallError('in is <n>s, <n>m, <n>h or <n>d, such as 90s or 30m; one unit, n at least 1')
        return self.resolved(self.later_add({'in': '+' + m[1] + m[2], 'target': call['target'], 'body': call['body']}, message))

    def at_at(self, call, message):
        text = call['at']
        if CLOCK.fullmatch(text):
            stamp = next_clock(text, self.clock(), self.cfg['tz'])
            text = datetime.fromtimestamp(stamp, timezone.utc).isoformat()
        elif re.fullmatch(r'[0-9]{1,2}:[0-9]{1,2}(:[0-9]{2})?', text):
            raise CallError('a time of day is HH:MM, two digits each, 00:00 to 23:59')
        elif not re.search(r'[0-9]', text):
            raise CallError('at takes HH:MM or an RFC 3339 instant with an offset, '
                            'such as 09:30 or 2026-10-29T09:30:00-04:00')
        return self.resolved(self.later_add({'at': text, 'target': call['target'], 'body': call['body']}, message))


if __name__ == '__main__':
    sys.exit(run(Agent))
