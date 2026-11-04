#!/usr/bin/env python3
"""Install git through the shared standalone installer."""
from pathlib import Path
import subprocess
import sys


def main():
    here = Path(__file__).resolve().parent
    return subprocess.run([sys.executable, str(here.parent / 'install.py'), *sys.argv[1:],
                           '--script', str(here / 'git_agent.py'), '--serves-itself', '--script-arg=--serve']).returncode


if __name__ == '__main__':
    sys.exit(main())
