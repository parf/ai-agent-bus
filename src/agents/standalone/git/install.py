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
    p.add_argument('--autopull', action='append', default=[], help='opt-in repo=interval timer, e.g. ab=5m')
    args, rest = p.parse_known_args()
    context = argparse.ArgumentParser(add_help=False)
    context.add_argument('action', choices=['install','remove'])
    context.add_argument('--name', required=True)
    context.add_argument('--unit-dir', default='/etc/systemd/system')
    context.add_argument('--systemctl', default='systemctl')
    context.add_argument('--bus-binary', default='agent-bus')
    options, _ = context.parse_known_args(rest)
    try:
        repos = {}
        if options.action == 'install' and args.autopull:
            import git_agent
            result = subprocess.run([options.bus_binary, 'agent-template', options.name], capture_output=True, text=True, timeout=30)
            if result.returncode:
                raise ValueError('configure repos on the installed agent before enabling autopull')
            os.environ['AGENT_BUS_CONFIG'] = result.stdout
            repos = git_agent.configuration()['repos']
        wanted = timers.intervals(args.autopull, repos)
        key = 'agent-bus-' + re.sub(r'[^a-z0-9-]+', '-', options.name[1:].split('@',1)[0])
        unit = Path(options.unit_dir).absolute() / (key + '.service')
        if options.action == 'remove' and unit.exists():
            timers.sync(unit, {}, {}, options.name, options.systemctl)
        here = Path(__file__).resolve().parent
        result = subprocess.run([sys.executable, str(here.parent / 'install.py'), *rest,
                                 '--script', str(here / 'git_agent.py')])
        if result.returncode: return result.returncode
        if options.action == 'install':
            timers.sync(unit, wanted, repos, options.name, options.systemctl)
    except (ValueError, OSError, subprocess.TimeoutExpired, RuntimeError) as e:
        print('git installer: ' + str(e), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
