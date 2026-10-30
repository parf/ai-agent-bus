#!/usr/bin/env python3
"""Install git with Configuration-defined repositories through the shared starter."""
import argparse
import os
from pathlib import Path
import re
import subprocess
import sys

import timers


def main():
    p = argparse.ArgumentParser(description=__doc__, epilog="Other options: install|remove --name #git@host --user account; shared starter connection/unit options also apply.")
    if any(arg == '--autopull' or arg.startswith('--autopull=') for arg in sys.argv[1:]):
        p.error('--autopull timers were removed; use Configuration mode/follow/autopull-cmd/autopull-log')
    args, rest = p.parse_known_args()
    context = argparse.ArgumentParser(add_help=False)
    context.add_argument('action', choices=['install','remove'])
    context.add_argument('--name', required=True)
    context.add_argument('--unit-dir', default='/etc/systemd/system')
    context.add_argument('--systemctl', default='systemctl')
    context.add_argument('--bus-binary', default='agent-bus')
    options, _ = context.parse_known_args(rest)
    try:
        key = 'agent-bus-' + re.sub(r'[^a-z0-9-]+', '-', options.name[1:].split('@',1)[0])
        unit = Path(options.unit_dir).absolute() / (key + '.service')
        if unit.exists():
            timers.retire(unit, options.name, options.systemctl)
        here = Path(__file__).absolute().parent
        result = subprocess.run([sys.executable, str(here.parent / 'install.py'), *rest,
                                 '--script', str(here / 'git_agent.py'), '--serves-itself', '--script-arg=--serve'])
        if result.returncode: return result.returncode
        if options.action == 'install':
            timers.retire(unit, options.name, options.systemctl)
    except (ValueError, OSError, subprocess.TimeoutExpired, RuntimeError) as e:
        print('git installer: ' + str(e), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
