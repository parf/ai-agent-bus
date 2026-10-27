#!/usr/bin/env python3
"""Keep one tmux session of configured named command windows."""
import json
import os
from pathlib import Path
import pwd
import re
import signal
import subprocess
import sys
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'shared'))
from metadata import dispatch
from envfile import parse as parse_envfile

PACKAGE = Path(__file__).resolve().parent
READY_WAIT = 2.0
STOP_GRACE = 10.0



def environment(value):
    if isinstance(value, str):
        if len(value.encode()) > 512 * 1024:
            raise ValueError('env exceeds 512 KiB')
        values = {v['Key']: v['Value'] for v in parse_envfile(value)}
    elif isinstance(value, dict):
        values = value.copy()
    else:
        raise ValueError('env must be an env-file string or an object')
    if len(values) > 64:
        raise ValueError('env allows at most 64 keys')
    for key, val in values.items():
        if not isinstance(key, str) or not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*', key):
            raise ValueError('invalid environment key')
        if key == 'AGENT_BUS_TOKEN' or key.startswith('AB_ROLE_'):
            raise ValueError('env may not set ' + key)
        if not isinstance(val, str) or '\0' in val or len(val.encode()) > 4096:
            raise ValueError('invalid value for environment key ' + key)
    return values

def configuration(raw):
    try:
        config = json.loads(raw) if raw else {}
    except (ValueError, TypeError) as error:
        raise ValueError('Configuration is not JSON') from error
    if not isinstance(config, dict) or set(config) - {'session', 'windows'}:
        raise ValueError('Configuration allows only session and windows')
    identity = os.environ.get('AGENT_BUS_NAME', '#tmux-sessions').split('@', 1)[0]
    session = config.get('session', identity.removeprefix('#').rsplit('/', 1)[-1])
    if not isinstance(session, str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,32}', session):
        raise ValueError('session: expected 1..32 letters, digits, - or _')
    rows = config.get('windows', [])
    if not isinstance(rows, list) or len(rows) > 32:
        raise ValueError('windows must be a list of at most 32 entries')
    result, names = [], set()
    for index, row in enumerate(rows):
        where = f'windows[{index}]'
        if not isinstance(row, dict) or set(row) - {'name', 'cwd', 'command', 'env'}:
            raise ValueError(f'{where}: allowed keys are name, cwd, command, env')
        name, cwd, argv = row.get('name'), row.get('cwd'), row.get('command')
        if not isinstance(name, str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,32}', name):
            raise ValueError(f'{where}.name: expected 1..32 letters, digits, - or _')
        if not isinstance(cwd, str) or not Path(cwd).is_absolute() or not Path(cwd).is_dir():
            raise ValueError(f'{where}.cwd: expected an existing absolute directory')
        if not isinstance(argv, str) or not argv.strip() or len(argv.encode()) > 4096 or any(c in argv for c in '\0\r\n'):
            raise ValueError(f'{where}.command: expected a non-empty string, at most 4096 bytes, without NUL or newline')
        try:
            env = environment(row.get('env', {}))
        except ValueError as error:
            raise ValueError(f'{where}.env: {error}') from error
        if name in names:
            raise ValueError(f'{where}.name: duplicate window name {name}')
        names.add(name)
        result.append({'name': name, 'cwd': cwd, 'command': argv, 'env': env})
    return {'session': session, 'windows': result}


def clean_environment():
    env = {key: value for key, value in os.environ.items()
           if not key.startswith(('AGENT_BUS_', 'AB_ROLE_')) and key != 'TMUX'}
    runtime = '/run/user/' + str(os.getuid())
    env.update(XDG_RUNTIME_DIR=runtime, DBUS_SESSION_BUS_ADDRESS='unix:path=' + runtime + '/bus')
    return env


def window_argv(row, path):
    # env -i also clears values retained by an already-running tmux server.
    person = pwd.getpwuid(os.getuid())
    env = {key: value for key, value in clean_environment().items()
           if key in ('PATH', 'TERM', 'COLORTERM', 'LANG', 'DISPLAY', 'WAYLAND_DISPLAY',
                      'DBUS_SESSION_BUS_ADDRESS', 'SSH_AUTH_SOCK', 'XDG_CONFIG_HOME',
                      'XDG_DATA_HOME', 'XDG_STATE_HOME', 'XDG_RUNTIME_DIR') or key.startswith('LC_')}
    env.update(HOME=person.pw_dir, USER=person.pw_name, LOGNAME=person.pw_name,
               SHELL=person.pw_shell)
    env['PATH'] = path or env.get('PATH') or '/usr/local/bin:/usr/bin:/bin'
    env.setdefault('TERM', 'xterm-256color')
    env.update(row['env'])
    return ['env', '-i', *(f'{key}={value}' for key, value in sorted(env.items())),
            sys.executable, '-c', 'import os,sys; os.umask(0o022); os.execvp(sys.argv[1],sys.argv[1:])',
            '/bin/sh', '-c', row['command']]


def command(argv):
    return subprocess.run(argv, env=clean_environment(), text=True, capture_output=True, timeout=5, umask=0o022)

def user_path():
    # The starter's system unit has system PATH; runtimes belong to this
    # person's user manager. Read only PATH, never import its credentials.
    result = command(['systemctl', '--user', 'show-environment'])
    path = next((line[5:] for line in result.stdout.splitlines() if line.startswith('PATH=')), '')
    if result.returncode or not path:
        raise ValueError('could not read PATH from the user manager')
    return path


def output_tail(target):
    result = command(['tmux', 'capture-pane', '-p', '-t', target, '-S', '-10'])
    text = re.sub(r'\x1b\[[0-?]*[ -/]*[@-~]', '', result.stdout)
    return ' '.join(''.join(c for c in text if c.isprintable() or c.isspace()).split())[-180:]


def wait_ready(target):
    until = time.monotonic() + READY_WAIT
    while True:
        pane = command(['tmux', 'display-message', '-p', '-t', target, '#{pane_dead}'])
        if pane.returncode or pane.stdout.strip() != '0':
            tail = output_tail(target) or 'window disappeared'
            dead = command(['tmux', 'list-panes', '-t', target, '-F', '#{pane_dead}'])
            if not dead.returncode and dead.stdout.split() and set(dead.stdout.split()) == {'1'}:
                command(['tmux', 'kill-window', '-t', target])
            raise ValueError('command exited: ' + tail)
        if time.monotonic() >= until:
            return
        time.sleep(.1)


def pane_info(target):
    result = command(['tmux', 'list-panes', '-t', target, '-F', '#{pane_id}\t#{pane_pid}\t#{pane_dead}'])
    if result.returncode:
        return []
    panes = []
    for line in result.stdout.splitlines():
        fields = line.split('\t')
        if len(fields) != 3 or not re.fullmatch(r'%[0-9]+', fields[0]):
            raise ValueError('invalid tmux pane state')
        pid, dead = int(fields[1] or 0), fields[2] != '0'
        ticks, uptime = None, None
        try:
            ticks = int(Path('/proc', str(pid), 'stat').read_text().rsplit(') ', 1)[1].split()[19])
            uptime = max(0, int(float(Path('/proc/uptime').read_text().split()[0]) - ticks / os.sysconf('SC_CLK_TCK')))
        except (OSError, ValueError, IndexError):
            pass
        panes.append({'id': fields[0], 'pid': pid, 'alive': not dead, 'ticks': ticks, 'uptime': uptime})
    return panes


def session_windows(config):
    session = '=' + config['session']
    exists = command(['tmux', 'has-session', '-t', session])
    if exists.returncode == 1:
        return None
    if exists.returncode:
        raise ValueError('could not inspect tmux session')
    result = command(['tmux', 'list-windows', '-t', session, '-F', '#{window_id}'])
    if result.returncode:
        raise ValueError('could not list tmux windows')
    windows = []
    for target in result.stdout.splitlines():
        if not re.fullmatch(r'@[0-9]+', target):
            raise ValueError('invalid tmux window identity')
        name = command(['tmux', 'display-message', '-p', '-t', target, '#{window_name}'])
        if name.returncode:
            raise ValueError('could not read tmux window name')
        panes = pane_info(target)
        cwd = command(['tmux', 'display-message', '-p', '-t', target, '#{pane_current_path}'])
        windows.append({'id': target, 'name': name.stdout.rstrip('\n'), 'panes': panes, 'cwd': cwd.stdout.rstrip('\n')})
    return windows


def selected(config, names):
    valid = [row['name'] for row in config['windows']]
    if any(name not in valid for name in names):
        raise ValueError('unknown window; valid windows: ' + ', '.join(valid))
    return [row for row in config['windows'] if not names or row['name'] in names]


def start(config, names=()):
    rows = selected(config, names)
    if not rows:
        return 'no windows configured'
    path = user_path()
    answers = []
    for row in rows:
        try:
            windows = session_windows(config)
            if any(w['name'] == row['name'] for w in windows or []):
                answers.append(row['name'] + ': already running')
                continue
            if windows is None:
                # Only session creation needs a scope; additions use the same
                # existing user-manager tmux server, never the starter cgroup.
                argv = ['systemd-run', '--user', '--scope', '--quiet', '--collect',
                        '--unit=ab-session-' + config['session'], '--', 'tmux', 'new-session',
                        '-d', '-s', config['session'], '-n', row['name']]
            else:
                argv = ['tmux', 'new-window', '-d', '-t', '=' + config['session'] + ':', '-n', row['name']]
            created = command([*argv, '-c', row['cwd'], '-P', '-F', '#{pane_id}', '--', 'sleep', '60'])
            target = created.stdout.strip()
            if created.returncode or not re.fullmatch(r'%[0-9]+', target):
                raise ValueError(created.stderr.strip()[:200] or 'tmux refused the window')
            kept = command(['tmux', 'set-option', '-p', '-t', target, 'remain-on-exit', 'on'])
            if kept.returncode:
                raise ValueError('could not retain command output')
            launched = command(['tmux', 'respawn-pane', '-k', '-t', target, '-c', row['cwd'], '--', *window_argv(row, path)])
            if launched.returncode:
                raise ValueError('could not launch command: ' + launched.stderr.strip()[:200])
            wait_ready(target)
            answers.append(row['name'] + ': started')
        except (OSError, ValueError, subprocess.TimeoutExpired) as error:
            answers.append(row['name'] + ': failed: ' + str(error)[:240].replace('\n', ' '))
    return '\n'.join(answers)


def signal_window(pane):
    if not pane['alive'] or pane['ticks'] is None:
        return
    try:
        fd = os.pidfd_open(pane['pid'])
        try:
            # Recheck the observed process after opening its handle. Tmux
            # gives each pane its own process group; never signal a shared one.
            current = pane_info(pane['id'])
            if current and current[0]['pid'] == pane['pid'] and current[0]['ticks'] == pane['ticks']:
                if os.getpgid(pane['pid']) != pane['pid']:
                    raise ValueError('window command no longer leads its own process group')
                os.killpg(pane['pid'], signal.SIGTERM)
        finally:
            os.close(fd)
    except ProcessLookupError:
        pass


def stop(config, names=()):
    rows = selected(config, names)  # Validate every name before the first signal.
    windows = session_windows(config) or []
    answers = []
    for row in rows:
        matches = [w for w in windows if w['name'] == row['name']]
        if not matches:
            answers.append(row['name'] + ': already stopped')
            continue
        try:
            for window in matches:
                for pane in window['panes']:
                    signal_window(pane)
                until = time.monotonic() + STOP_GRACE
                while time.monotonic() < until and any(p['alive'] for p in pane_info(window['id'])):
                    time.sleep(.1)
                # Window IDs came from this session, matched against config;
                # never kill the session or an unmanaged sibling window.
                result = command(['tmux', 'kill-window', '-t', window['id']])
                if result.returncode and pane_info(window['id']):
                    raise ValueError('could not stop window')
            answers.append(row['name'] + ': stopped')
        except (OSError, ValueError, subprocess.TimeoutExpired) as error:
            answers.append(row['name'] + ': failed: ' + str(error)[:240].replace('\n', ' '))
    return '\n'.join(answers) or 'no windows configured'


def restart(config, names=()):
    selected(config, names)
    stopped = stop(config, names)
    return stopped + '\n' + start(config, names)


def list_windows(config):
    windows = session_windows(config) or []
    lines, managed = [], {r['name'] for r in config['windows']}
    def describe(name, window, cwd, unmanaged=False):
        pane = window['panes'][0] if window and window['panes'] else None
        return (json.dumps(name) + (': unmanaged; ' if unmanaged else ': ') +
                ('present' if window else 'missing') +
                f"; pane alive={'yes' if pane and pane['alive'] else 'no'}; pid={pane['pid'] if pane and pane['pid'] else '-'}" +
                f"; uptime={str(pane['uptime'])+'s' if pane and pane['uptime'] is not None else '-'}; cwd=" + json.dumps(cwd))
    for row in config['windows']:
        matches = [w for w in windows if w['name'] == row['name']]
        if not matches:
            lines.append(describe(row['name'], None, row['cwd']))
        for window in matches:
            lines.append(describe(row['name'], window, window['cwd']))
    for window in windows:
        if window['name'] not in managed:
            lines.append(describe(window['name'], window, window['cwd'], True))
    return '\n'.join(lines) or 'no windows configured'


def status(config):
    windows = session_windows(config)
    if windows is None:
        return 'failed: session ' + config['session'] + ' absent'
    missing, unready, up = [], [], 0
    for row in config['windows']:
        matches = [w for w in windows if w['name'] == row['name']]
        if not matches:
            missing.append(row['name'])
        elif any(p['alive'] for w in matches for p in w['panes']):
            up += 1
        else:
            unready.append(row['name'])
    total = len(config['windows'])
    word = 'ok' if up and up == total else 'degraded' if up else 'failed'
    return f'{word}: {up}/{total} windows up' + ('; missing ' + ', '.join(missing) if missing else '') + ('; not running ' + ', '.join(unready) if unready else '')


METHODS = {'start': start, 'stop': stop, 'restart': restart, 'list': list_windows}


def request(body, config):
    if body.startswith('{'):
        value = json.loads(body)
        if not isinstance(value, dict) or set(value) - {'method', 'windows'}:
            raise ValueError('expected method and optional windows')
        method, names = value.get('method'), value.get('windows', [])
        if not isinstance(names, list) or any(not isinstance(n, str) for n in names):
            raise ValueError('windows must be a list of names')
    else:
        words = body.split()
        method, names = (words[0], words[1:]) if words else ('', [])
    if not isinstance(method, str) or method not in METHODS or method == 'list' and names:
        raise ValueError('expected start, stop, restart [<window>...] or list')
    selected(config, names)
    return list_windows(config) if method == 'list' else METHODS[method](config, names)

def main():
    ask = os.environ.get('AGENT_BUS_ASK')
    try:
        result = dispatch(PACKAGE, ask, handlers={'status': lambda: status(configuration(os.environ.get('AGENT_BUS_CONFIG')))})
        if result is not None:
            print(result)
            return
        body = ' '.join(sys.argv[1:]).strip()
        print(request(body, configuration(os.environ.get('AGENT_BUS_CONFIG'))))
    except (OSError, ValueError, subprocess.TimeoutExpired) as error:
        print(('failed: ' if ask == 'status' else 'error: ') + str(error)[:300].replace('\n', ' '))


if __name__ == '__main__':
    main()
