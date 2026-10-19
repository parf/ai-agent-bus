#!/usr/bin/env python3
"""Test-only systemctl: acting requires a real write to root-owned state."""
import json
import os
from pathlib import Path
import sys

args = sys.argv[1:]
assert args[:2] == ['--no-pager', '--no-ask-password']
method = args[2]
units = args[args.index('--') + 1:] if '--' in args else []
state = Path(os.environ['B2_HOST_STATE'])
with Path(os.environ['B2_HOST_CALLS']).open('a') as log:
    log.write(json.dumps({'uid': os.geteuid(), 'method': method, 'units': units}) + '\n')
try:
    with state.open('r' if method in ('list-units', 'list-timers', 'status') else 'r+') as data:
        values = json.load(data)
        if method in ('list-units', 'list-timers'):
            kind = 'timer' if method == 'list-timers' else next((a.split('=',1)[1] for a in args if a.startswith('--type=')), None)
            units = [u for u in values if not kind or u.endswith('.'+kind)]
            if '--failed' in args:
                units = [u for u in units if values[u] == 'failed']
        if any(unit not in values for unit in units):
            raise ValueError('unit not found')
        if method in ('list-units', 'list-timers', 'status'):
            if '--failed' in args:
                for unit in units:
                    print(unit + ' loaded failed failed Fixture failure')
            else:
                print(json.dumps({unit: values[unit] for unit in units}))
        else:
            for unit in units:
                values[unit] = method
            data.seek(0)
            json.dump(values, data)
            data.truncate()
            print('ok')
except (PermissionError, ValueError) as error:
    print(str(error), file=sys.stderr)
    sys.exit(1)
