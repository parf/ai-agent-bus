#!/usr/bin/env python3
"""Install git with fixed named repositories through the shared starter."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

import timers


def repositories(entries):
    repos = {}
    for entry in entries:
        name, sep, raw = entry.partition('=')
        if not sep or not re.fullmatch(r'[a-zA-Z0-9][a-zA-Z0-9_-]*', name) or name in repos:
            raise ValueError('each --repo must be a unique name=absolute-path')
        path = Path(raw)
        if not path.is_absolute():
            raise ValueError('repository path must be absolute')
        path = path.resolve(strict=True)
        def git(*args):
            p = subprocess.run(['git', '-C', str(path), *args], capture_output=True, text=True, timeout=10)
            if p.returncode: raise ValueError(name + ': requires a working repository on a branch')
            return p.stdout.strip()
        if git('rev-parse', '--show-toplevel') != str(path):
            raise ValueError(name + ': path must be the repository root')
        branch = git('symbolic-ref', '--short', 'HEAD')
        repos[name] = {'path': str(path), 'branch': branch}
    if not repos: raise ValueError('install requires at least one --repo name=path')
    return repos


def main():
    p = argparse.ArgumentParser(description=__doc__, epilog="Other options: install|remove --name #git@host --user account; shared starter connection/unit options also apply.")
    p.add_argument('--repo', action='append', default=[], help='fixed name=absolute-path; repeat for each repository')
    p.add_argument('--autopull', action='append', default=[], help='opt-in repo=interval timer, e.g. ab=5m')
    args, rest = p.parse_known_args()
    context = argparse.ArgumentParser(add_help=False)
    context.add_argument('action', choices=['install','remove'])
    context.add_argument('--name', required=True)
    context.add_argument('--unit-dir', default='/etc/systemd/system')
    context.add_argument('--systemctl', default='systemctl')
    options, _ = context.parse_known_args(rest)
    try:
        extra = []
        repos = repositories(args.repo) if options.action == 'install' else {}
        wanted = timers.intervals(args.autopull, repos)
        key = 'agent-bus-' + re.sub(r'[^a-z0-9-]+', '-', options.name[1:].split('@',1)[0])
        unit = Path(options.unit_dir).absolute() / (key + '.service')
        if options.action == 'remove' and unit.exists():
            timers.sync(unit, {}, {}, options.name, options.systemctl)
        if options.action == 'install':
            extra = ['--script-arg=--repos-json', '--script-arg=' + json.dumps(repos, separators=(',', ':'))]
        here = Path(__file__).resolve().parent
        result = subprocess.run([sys.executable, str(here.parent / 'install.py'), *rest,
                                 '--script', str(here / 'git_agent.py'), *extra])
        if result.returncode: return result.returncode
        if options.action == 'install':
            timers.sync(unit, wanted, repos, options.name, options.systemctl)
    except (ValueError, OSError, subprocess.TimeoutExpired, RuntimeError) as e:
        print('git installer: ' + str(e), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
