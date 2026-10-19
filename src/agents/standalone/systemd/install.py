#!/usr/bin/env python3
"""Install the host systemd agent through the shared standalone starter."""
import os
from pathlib import Path
import sys

here = Path(__file__).resolve().parent
os.execv(sys.executable, [sys.executable, str(here.parent / 'install.py'), *sys.argv[1:],
                         '--script', str(here / 'systemd.py')])
