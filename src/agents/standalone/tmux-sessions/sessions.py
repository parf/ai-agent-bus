#!/usr/bin/env python3
"""Start a configured session set as the installing person."""
import json
import os
from pathlib import Path
import pwd
import re
import subprocess
import sys
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'shared'))
from metadata import answer as metadata_answer, ASKS, dispatch, normalize_tags

PACKAGE = Path(__file__).resolve().parent
NAME = re.compile(r'#[a-z0-9][a-z0-9._-]*(?:/[a-z0-9][a-z0-9._-]*)?(?:@[a-z0-9][a-z0-9._-]*)?\Z')
RUNTIMES = ('claude', 'codex', 'opencode')


def tag(raw):
    if not isinstance(raw, str) or ',' in raw:
        raise ValueError(f'invalid tag: {raw!r}')
    values = normalize_tags([raw])
    if len(values) != 1:
        raise ValueError(f'invalid tag: {raw!r}')
    return values[0]


def tmux_name(name):
    return re.sub('[^a-zA-Z0-9_-]', '-', name.removeprefix('#'))


def configuration(raw):
    try:
        config = json.loads(raw) if raw else {'sessions': []}
    except (ValueError, TypeError) as error:
        raise ValueError('Configuration is not JSON') from error
    if not isinstance(config, dict) or set(config) - {'tag', 'sessions'}:
        raise ValueError('Configuration allows only tag and sessions')
    set_tag = tag(config['tag']) if 'tag' in config else None
    rows = config.get('sessions', [])
    if not isinstance(rows, list) or len(rows) > 32:
        raise ValueError('sessions must be a list of at most 32 entries')
    result, names = [], set()
    for index, row in enumerate(rows):
        where = f'sessions[{index}]'
        if not isinstance(row, dict) or set(row) - {'name', 'runtime', 'cwd', 'args', 'tags', 'tmux'}:
            raise ValueError(f'{where}: allowed keys are name, runtime, cwd, args, tags, tmux')
        name, runtime, cwd = (row.get(key) for key in ('name', 'runtime', 'cwd'))
        if not isinstance(name, str) or len(name) > 64 or not NAME.fullmatch(name):
            raise ValueError(f'{where}.name: expected a canonical #session name, at most 64 bytes')
        if runtime not in RUNTIMES:
            raise ValueError(f'{where}.runtime: expected claude, codex or opencode')
        if not isinstance(cwd, str) or not Path(cwd).is_absolute() or not Path(cwd).is_dir():
            raise ValueError(f'{where}.cwd: expected an existing absolute directory')
        args, tags = row.get('args', []), row.get('tags', [])
        if not isinstance(args, list) or len(args) > 64 or any(not isinstance(arg, str) or '\0' in arg or len(arg) > 4096 for arg in args):
            raise ValueError(f'{where}.args: expected at most 64 strings of at most 4096 characters, without NUL')
        if not isinstance(tags, list) or len(tags) > 32:
            raise ValueError(f'{where}.tags: expected at most 32 tags')
        tags = sorted({tag(value) for value in tags} | ({set_tag} if set_tag else set()))
        tmux = row.get('tmux', tmux_name(name))
        if 'tmux' in row and (not isinstance(tmux, str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,32}', tmux)):
            raise ValueError(f'{where}.tmux: expected 1..32 letters, digits, - or _')
        if tmux in names:
            raise ValueError(f'{where}.name: duplicate tmux name {tmux}')
        names.add(tmux)
        result.append({'name': name, 'runtime': runtime, 'cwd': cwd, 'args': args, 'tags': tags, 'tmux': tmux})
    return {'tag': set_tag, 'sessions': result}


def clean_environment():
    env = {key: value for key, value in os.environ.items()
           if not key.startswith(('AGENT_BUS_', 'AB_ROLE_')) and key != 'TMUX'}
    runtime = '/run/user/' + str(os.getuid())
    env.update(XDG_RUNTIME_DIR=runtime, DBUS_SESSION_BUS_ADDRESS='unix:path=' + runtime + '/bus')
    return env


def launcher_argv(row, path=None):
    # env -i also clears values retained by an already-running tmux server.
    person = pwd.getpwuid(os.getuid())
    env = {key: value for key, value in clean_environment().items()
           if key in ('PATH', 'TERM', 'COLORTERM', 'LANG', 'DISPLAY', 'WAYLAND_DISPLAY',
                      'DBUS_SESSION_BUS_ADDRESS', 'SSH_AUTH_SOCK', 'XDG_CONFIG_HOME',
                      'XDG_DATA_HOME', 'XDG_STATE_HOME', 'XDG_RUNTIME_DIR') or key.startswith('LC_')}
    env.update(HOME=person.pw_dir, USER=person.pw_name, LOGNAME=person.pw_name,
               SHELL=person.pw_shell, AGENT_BUS_NAME=row['name'])
    env['PATH'] = path or env.get('PATH') or '/usr/local/bin:/usr/bin:/bin'
    env.setdefault('TERM', 'xterm-256color')
    argv = ['env', '-i', *(f'{key}={value}' for key, value in sorted(env.items())),
            sys.executable, '-c', 'import os,sys; os.umask(0o022); os.execvp(sys.argv[1],sys.argv[1:])',
            'ab-' + row['runtime'], '--restart']
    if row['tags']:
        argv.extend(['--tag', ','.join(row['tags'])])
    return argv + row['args']


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


def same_name(actual, requested):
    return actual == requested or ('@' not in requested and actual.split('@', 1)[0] == requested)


def output_tail(target):
    result = command(['tmux', 'capture-pane', '-p', '-t', target, '-S', '-10'])
    text = re.sub(r'\x1b\[[0-?]*[ -/]*[@-~]', '', result.stdout)
    return ' '.join(''.join(c for c in text if c.isprintable() or c.isspace()).split())[-300:]


def wait_ready(row, target):
    started = time.monotonic()
    until = started + 20
    while True:
        pane = command(['tmux', 'display-message', '-p', '-t', target, '#{pane_dead} #{pane_dead_status}'])
        if pane.returncode or pane.stdout.split()[:1] != ['0']:
            tail = output_tail(target) or 'session disappeared'
            # Only this newly created session, and only with every pane dead.
            # Capture first, then remove it so fixing the launcher permits retry.
            session = '=' + row['tmux']
            dead = command(['tmux', 'list-panes', '-s', '-t', session, '-F', '#{pane_dead}'])
            if not dead.returncode and dead.stdout.split() and set(dead.stdout.split()) == {'1'}:
                command(['tmux', 'kill-session', '-t', session])
            raise ValueError('launcher exited: ' + tail)
        listing = command(['agent-bus', 'ls', '--all'])
        if listing.returncode:
            raise ValueError('could not verify the session bus reader')
        records = json.loads(listing.stdout)
        reader = any(same_name(r.get('name', ''), row['name']) and r.get('readers', 0) > 0
                     for r in records if isinstance(r, dict))
        if reader and time.monotonic() - started >= .2:
            return
        if time.monotonic() >= until:
            raise ValueError('no bus reader within 20s: ' + (output_tail(target) or 'launcher is still initializing'))
        time.sleep(.1)


def start(config):
    rows = config['sessions']
    if not rows:
        return 'no sessions configured'
    # Use the person's local login socket, never this agent's connection/token.
    listing = command(['agent-bus', 'ls', '--all'])
    if listing.returncode:
        raise ValueError('could not check live bus names: ' + listing.stderr.strip()[:200])
    records = json.loads(listing.stdout)
    if not isinstance(records, list):
        raise ValueError('could not check live bus names: expected a record list')
    path = user_path()
    live = {record['name'] for record in records if isinstance(record, dict) and record.get('readers', 0) > 0}
    answers = []
    for row in rows:
        try:
            name = row['tmux']
            exists = command(['tmux', 'has-session', '-t', '=' + name])
            if exists.returncode == 0:
                raise ValueError('tmux session already exists: ' + name)
            if exists.returncode != 1:
                raise ValueError(exists.stderr.strip()[:200] or 'tmux check failed')
            if any(same_name(actual, row['name']) for actual in live):
                raise ValueError('bus name is already live')
            created = command(['systemd-run', '--user', '--scope', '--quiet', '--collect',
                               '--unit=ab-session-' + name, '--', 'tmux', 'new-session',
                               '-d', '-s', name, '-c', row['cwd'], '--', 'sleep', '60'])
            if created.returncode:
                raise ValueError(created.stderr.strip()[:200] or 'tmux refused the session')
            # Keep an exited pane long enough to report its output. Set this
            # before the runtime starts, so an immediate exit cannot race it.
            target = '=' + name + ':0.0'
            kept = command(['tmux', 'set-option', '-p', '-t', target, 'remain-on-exit', 'on'])
            if kept.returncode:
                raise ValueError('could not retain launcher diagnostics')
            launched = command(['tmux', 'respawn-pane', '-k', '-t', target, '-c', row['cwd'], '--', *launcher_argv(row, path)])
            if launched.returncode:
                raise ValueError('could not launch runtime: ' + launched.stderr.strip()[:200])
            wait_ready(row, target)
            answers.append(row['name'] + ': started')
        except (OSError, ValueError, subprocess.TimeoutExpired) as error:
            answers.append(row['name'] + ': failed: ' + str(error)[:240].replace('\n', ' '))
    return '\n'.join(answers)


def main():
    result = dispatch(PACKAGE, os.environ.get('AGENT_BUS_ASK'))
    if result is not None:
        print(result)
        return
    body = ' '.join(sys.argv[1:]).strip()
    if body == 'help':
        print(metadata_answer(PACKAGE))
        return
    try:
        if body.startswith('{'):
            value = json.loads(body)
            if value != {'method': 'start'}:
                raise ValueError('expected {"method":"start"}')
        elif body != 'start':
            raise ValueError('expected start')
        print(start(configuration(os.environ.get('AGENT_BUS_CONFIG'))))
    except (OSError, ValueError, subprocess.TimeoutExpired) as error:
        print('error: ' + str(error)[:300].replace('\n', ' '))


if __name__ == '__main__':
    main()
