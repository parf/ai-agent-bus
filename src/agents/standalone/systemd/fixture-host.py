#!/usr/bin/env python3
"""Test-only systemctl: acting requires a real write to root-owned state."""
import json
import os
from pathlib import Path
import sys

args = sys.argv[1:]
assert args[:2] == ['--no-pager', '--no-ask-password']
method = args[2]
units = args[args.index('--') + 1:]
state = Path(os.environ['B2_HOST_STATE'])
with Path(os.environ['B2_HOST_CALLS']).open('a') as log:
    log.write(json.dumps({'uid': os.geteuid(), 'method': method, 'units': units}) + '\n')
try:
    with state.open('r' if method in ('list-units', 'status') else 'r+') as data:
        values = json.load(data)
        if any(unit not in values for unit in units):
            raise ValueError('unit not found')
        if method in ('list-units', 'status'):
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
