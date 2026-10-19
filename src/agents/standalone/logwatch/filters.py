"""Literal filters and isolated, time-bounded regular expressions."""
import json
from pathlib import Path
import re
import subprocess
import sys


def parts(query):
    if not isinstance(query, str) or len(query) > 210:
        raise ValueError('filter must be text or /pattern/flags, at most 200 pattern characters')
    if not query.startswith('/'):
        return None
    close = query.rfind('/')
    if close == 0:
        raise ValueError('filter needs a closing slash')
    pattern, flags = query[1:close], query[close+1:]
    if len(pattern) > 200 or any(flag not in 'imsu' for flag in flags):
        raise ValueError('regex accepts at most 200 characters and flags i m s u')
    return pattern, sum({'i': re.I, 'm': re.M, 's': re.S, 'u': 0}[flag] for flag in set(flags))


def match(query, lines):
    parsed = parts(query)
    if parsed is None:
        return [query.casefold() in line.casefold() for line in lines]
    try:
        answer = subprocess.run([sys.executable, str(Path(__file__).resolve()), '--worker'],
            input=json.dumps({'pattern': parsed[0], 'flags': parsed[1], 'lines': lines}),
            capture_output=True, text=True, timeout=1)
    except subprocess.TimeoutExpired:
        raise ValueError('filter took too long') from None
    if answer.returncode:
        raise ValueError('invalid regular expression')
    return json.loads(answer.stdout)


if __name__ == '__main__':
    request = json.load(sys.stdin)
    regex = re.compile(request['pattern'], request['flags'])
    print(json.dumps([bool(regex.search(line)) for line in request['lines']]))
