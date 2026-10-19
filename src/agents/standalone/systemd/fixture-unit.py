#!/usr/bin/env python3
"""Private unit controller: root test process switches to the unit's real UID."""
import os
from pathlib import Path
import pwd
import shlex
import signal
import subprocess
import sys
import time

root = Path(os.environ['B2_UNIT_ROOT'])
verb, unit = sys.argv[1], sys.argv[-1]
pidfile = root / (unit + '.pid')


def alive():
    if not pidfile.exists():
        return False
    try:
        return Path('/proc/' + pidfile.read_text() + '/stat').read_text().rsplit(')', 1)[1].split()[0] != 'Z'
    except FileNotFoundError:
        return False


def stop():
    if alive():
        os.kill(int(pidfile.read_text()), signal.SIGTERM)
        until = time.monotonic() + 40
        while alive() and time.monotonic() < until:
            time.sleep(.02)
        if alive():
            raise RuntimeError('private unit did not stop')
    pidfile.unlink(missing_ok=True)


if verb == 'is-active':
    sys.exit(0 if alive() else 3)
if verb in ('disable', 'restart'):
    stop()
if verb in ('start', 'restart'):
    fields = dict(line.split('=', 1) for line in (root / 'units' / unit).read_text().splitlines() if '=' in line and not line.startswith('#'))
    account = pwd.getpwnam(fields['User'])
    envfile = Path(fields['EnvironmentFile'])
    if not envfile.is_absolute():
        raise ValueError('EnvironmentFile path is not absolute')
    env = {k: v for k, v in os.environ.items() if not k.startswith('AGENT_BUS_')}
    env.update(dict(line.split('=', 1) for line in envfile.read_text().splitlines() if line and not line.startswith('#')))
    state = root / ('state-' + unit + '-' + str(account.pw_uid))
    state.mkdir(exist_ok=True, mode=0o700)
    os.chown(state, account.pw_uid, account.pw_gid)
    env['XDG_STATE_HOME'] = str(state)
    env['XDG_CACHE_HOME'] = str(state / 'cache')
    env['PATH'] = str(root / 'bin') + ':' + os.environ['PATH']
    # Deliberately planted: the dedicated runner must replace this with the
    # delivered roles, not hand it to a no-role caller's script.
    env['AB_ROLE_ADMIN'] = '1'
    argv = [a.replace('%%', '%').replace('$$', '$') for a in shlex.split(fields['ExecStart'])]
    with (root / 'unit.log').open('ab') as log:
        child = subprocess.Popen(argv, env=env, stdout=log, stderr=log, start_new_session=True,
                                 user=account.pw_uid, group=account.pw_gid, extra_groups=[])
    pidfile.write_text(str(child.pid))
