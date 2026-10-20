#!/usr/bin/env python3
"""Install the session starter through the shared standalone installer."""
import os
from pathlib import Path
import pwd
import sys
if len(sys.argv) > 1 and sys.argv[1] == 'install' and '--help' not in sys.argv:
    value = next((arg.split('=', 1)[1] for arg in sys.argv if arg.startswith('--user=')), None)
    if '--user' in sys.argv:
        index = sys.argv.index('--user')
        value = sys.argv[index + 1] if index + 1 < len(sys.argv) else None
    if not value or pwd.getpwnam(value).pw_uid == 0:
        sys.exit('sessions install requires --user with a non-root person account')
here = Path(__file__).absolute().parent
os.execv(sys.executable, [sys.executable, str(here.parent / 'install.py'), *sys.argv[1:],
                         '--script', str(here / 'sessions.py')])
