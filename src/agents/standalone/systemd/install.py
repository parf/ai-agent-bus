#!/usr/bin/env python3
"""B.2 wrapper: validate fixed instance scope, then use the shared starter."""
import argparse
import importlib.util
import os
from pathlib import Path
import sys

here = Path(__file__).resolve().parent
parser = argparse.ArgumentParser(add_help=False)
parser.add_argument('--units-file')
args, rest = parser.parse_known_args()
fixed = []
if rest and rest[0] == 'install':
    if not args.units_file:
        sys.exit('systemd install: --units-file is required')
    try:
        units = Path(args.units_file).resolve(strict=True)
        spec = importlib.util.spec_from_file_location('systemd_agent', here / 'systemd.py')
        agent = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(agent)
        agent.configuration(units)
    except (ValueError, OSError) as error:
        sys.exit('systemd install: ' + str(error))
    fixed = ['--script-arg=--units-file', '--script-arg=' + str(units)]
os.execv(sys.executable, [sys.executable, str(here.parent / 'install.py'), *rest,
                         '--script', str(here / 'systemd.py'), *fixed])
