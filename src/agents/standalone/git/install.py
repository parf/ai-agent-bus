#!/usr/bin/env python3
"""Install git with fixed named repositories through the shared starter."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys


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
    p = argparse.ArgumentParser(add_help=False)
    p.add_argument('--repo', action='append', default=[])
    args, rest = p.parse_known_args()
    try:
        extra = []
        if 'install' in rest:
            extra = ['--script-arg=--repos-json', '--script-arg=' + json.dumps(repositories(args.repo), separators=(',', ':'))]
        here = Path(__file__).resolve().parent
        os.execv(sys.executable, [sys.executable, str(here.parent / 'install.py'), *rest,
                                 '--script', str(here / 'git_agent.py'), *extra])
    except (ValueError, OSError, subprocess.TimeoutExpired) as e:
        print('git installer: ' + str(e), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
