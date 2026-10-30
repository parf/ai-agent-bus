#!/usr/bin/env python3
"""Channel-driven updates and their bounded, per-checkout result lifecycle."""
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import socket
import subprocess
import sys
import threading
import time
from urllib.parse import urlsplit

import git_agent as g

BUDGET = 60
FETCH_ATTEMPTS = 3
EVENT_MAX = 16 * 1024
ERRORS = {
    'dirty': 'working tree or operation state is not clean',
    'diverged': 'local history cannot fast-forward to the origin branch',
    'not_in_branch': 'requested revision is not reachable from the origin branch',
    'fetch': 'repository update could not be completed',
    'timeout': 'repository update deadline exceeded; inspect the current revision',
    'lock': 'checkout is busy; no update attempted',
    'invalid_event': 'invalid update request',
}


def state_directory():
    return g.state_directory()


def after_attempt(repo, checkout, record, publish):
    """Script extension: called under checkout lock after one publication attempt.

    repo is the validated entry; checkout is a verified worktree Path. publish
    returns acceptance/no-op True, delivery failure False, never retries. The
    extension has a separate 60s budget (at most two 30s process-group runs).
    No hook runs when no checkout lock was acquired or root verification failed.
    """


def remote_path(origin):
    """A hosting project path, never a local path, URL, userinfo or hostname."""
    if '://' in origin:
        value = urlsplit(origin)
        if value.query or value.fragment: raise ValueError('invalid origin')
        if value.scheme == 'file':
            path = Path(value.path).name
        elif value.scheme in ('https', 'http', 'ssh', 'git') and value.hostname:
            path = value.path.lstrip('/')
        else:
            raise ValueError('unsupported origin')
    elif re.fullmatch(r'(?:[A-Za-z0-9._-]+@)?[A-Za-z0-9.-]+:.+', origin):
        path = origin.split(':', 1)[1].lstrip('/')
    elif Path(origin).is_absolute():
        path = Path(origin).name  # Local bare-repository fixtures / local upstreams.
    else:
        raise ValueError('unsupported origin')
    path = path.removesuffix('.git')
    if not project(path): raise ValueError('invalid project path')
    return path


def project(value):
    return (isinstance(value, str) and len(value.encode()) <= 256 and
            re.fullmatch(r'[A-Za-z0-9_-][A-Za-z0-9._/-]*', value) is not None and
            all(p not in ('', '.', '..') for p in value.split('/')))


def event(body):
    v = g.load_json(body, EVENT_MAX)
    if not isinstance(v, dict) or set(v) - {'v','type','remote','branch','commit','id'}:
        raise ValueError('invalid event fields')
    if type(v.get('v')) is not int or v['v'] != 1 or v.get('type') not in ('push','deploy') or not project(v.get('remote')):
        raise ValueError('invalid event')
    g.validate_branch(v.get('branch'))
    length = '{40}' if v['type'] == 'push' else '{7,40}'
    if not isinstance(v.get('commit'), str) or not re.fullmatch('[0-9a-f]' + length, v['commit']):
        raise ValueError('invalid event commit')
    if 'id' in v and (not isinstance(v['id'], str) or not re.fullmatch(r'[A-Za-z0-9._-]{1,64}', v['id'])):
        raise ValueError('invalid event id')
    return v


class Failure(Exception):
    def __init__(self, code): self.code = code


class Commands:
    def __init__(self, root, config, deadline):
        self.root, self.env, self.deadline = root, g.git_environment(config, clean=True), deadline

    def run(self, *args, check=True):
        left = self.deadline - time.monotonic()
        if left <= 0: raise Failure('timeout')
        value = g.git(self.root, list(args), self.env, check=False, timeout=left)
        if value['status'] == 124: raise Failure('timeout')
        if value['truncated']: raise Failure('fetch')
        if check and value['status']: raise Failure('fetch')
        return value

    def text(self, *args): return self.run(*args)['stdout'].strip()


def head(cmd):
    value = cmd.text('rev-parse', '--verify', 'HEAD^{commit}')
    if not re.fullmatch(r'[0-9a-f]{40}', value): raise Failure('fetch')
    return value


def branch_file(root):
    return state_directory() / ('branch-' + hashlib.sha256(str(root).encode()).hexdigest() + '.json')


def effective_branch(cmd, repo):
    if 'branch' in repo: return repo['branch']
    try:
        value = g.load_json(branch_file(cmd.root).read_text())
        return g.validate_branch(value['branch'])
    except FileNotFoundError:
        pass
    value = g.repository_branch(cmd.root, repo, cmd.env)
    return g.validate_branch(value)


def in_operation(cmd):
    for name in ('MERGE_HEAD', 'CHERRY_PICK_HEAD', 'REVERT_HEAD', 'rebase-merge', 'rebase-apply', 'sequencer'):
        path = Path(cmd.text('rev-parse', '--git-path', name))
        if not path.is_absolute(): path = cmd.root / path
        if path.exists(): return True
    return False


def dirty_paths(cmd):
    raw = cmd.run('status', '--porcelain=v1', '-z', '--untracked-files=all')['stdout']
    fields, paths, index = raw.split('\x00'), set(), 0
    while index < len(fields) and fields[index]:
        row = fields[index]; paths.add(row[3:]); index += 1
        if row[:1] in ('R','C') or row[1:2] in ('R','C'):
            if index < len(fields): paths.add(fields[index]); index += 1
    return paths


def ancestry(cmd, old, target):
    return cmd.run('merge-base', '--is-ancestor', old, target, check=False)['status'] == 0


def last_status(name):
    try: state = g.load_json(g.autopull_file(name).read_text())
    except FileNotFoundError: return 'no autopull attempt recorded'
    record = state.get('record', state)
    if 'result' not in record: return 'no channel-driven autopull attempt recorded'
    error = record.get('error')
    line = f"{record['result'].upper()} {record['repo']}@{record['host']} {record['branch']}: "
    line += (error['code'] if error else record['result']) + f" ({record['mode']})"
    if state.get('publish_failed'): line += '; log publish failed or unknown; not retried'
    return line


def publisher(config, bus):
    def publish(record):
        if not config.get('autopull-log'): return True
        if bus is None: return False
        try:
            bus.publish(config['autopull-log'], json.dumps(record, separators=(',', ':')),
                        topic=record['repo'] + '@' + record['host'], tag=record['result'])
            return True
        except Exception:
            # No exception text: transports can echo credentials or remote bodies.
            return False
    return publish


def attempt(name, config, *, action='autopull', trigger='operator', branch=None, rev=None,
            requested_by=None, event_id=None, invalid=False, bus=None, agent=None):
    started = time.monotonic(); deadline = started + BUDGET
    repo = config['repos'].get(name)
    if repo is None: raise ValueError('undeclared repository')
    if g.LEVELS[repo['access']] < 1: raise ValueError('repository access cap read refuses update')
    record = {'v':1, 'host':socket.getfqdn(), 'repo':name, 'branch':repo.get('branch'),
              'result':'fail', 'action':action, 'trigger':trigger, 'mode':repo.get('mode','ff'),
              'at':None, 'agent':agent or os.environ.get('AGENT_BUS_NAME'), 'remote':None,
              'requested_by':requested_by, 'id':event_id, 'from':None, 'to':None, 'requested':None,
              'discarded':{'files':0,'commits':0} if action == 'deploy' or repo.get('mode') == 'mirror' else None,
              'script':None, 'error':None, 'ms':0}
    root = Path(repo['path']).resolve()
    publish_raw = publisher(config, bus); failed = False
    def publish(value):
        nonlocal failed
        accepted = publish_raw(value)
        failed |= not accepted
        if not accepted: print('git: log publish failed or unknown', file=sys.stderr)
        g.store_autopull(g.autopull_file(name), {'record':value, 'publish_failed':failed})
        return accepted
    def finish(code=None, hooked=False):
        if code: record['error'] = {'code':code, 'text':ERRORS[code]}
        record['at'] = datetime.now().astimezone().isoformat(timespec='seconds')
        record['ms'] = int((time.monotonic() - started) * 1000)
        publish(record)
        if hooked: after_attempt(repo, root, record, publish)
        return record
    state_directory().mkdir(parents=True, exist_ok=True, mode=0o700)
    locked = False
    try:
        with g.checkout_lock(root, deadline):
            locked = True
            safe = False
            try:
                cmd = Commands(root, config, deadline)
                if cmd.text('rev-parse', '--show-toplevel') != str(root): raise Failure('fetch')
                safe = True
                record['from'] = record['to'] = head(cmd)
                record['remote'] = remote_path(cmd.text('remote', 'get-url', 'origin'))
                current = effective_branch(cmd, repo)
                chosen = g.validate_branch(branch) if branch is not None else current
                record['branch'] = chosen
                if trigger == 'event' and not invalid and chosen != current: return None
                if invalid or 'branch' in repo and chosen != repo['branch']: raise Failure('invalid_event')
                cmd.run('check-ref-format', '--branch', chosen)
                if in_operation(cmd): raise Failure('dirty')
                before = dirty_paths(cmd)
                destructive = action == 'deploy' or record['mode'] == 'mirror'
                if not destructive and before: raise Failure('dirty')
                target = None
                for index in range(FETCH_ATTEMPTS):
                    cmd.run('fetch', '--no-tags', 'origin', '+refs/heads/' + chosen + ':refs/remotes/origin/' + chosen)
                    target = cmd.text('rev-parse', '--verify', '--end-of-options', 'refs/remotes/origin/' + chosen + '^{commit}')
                    if not re.fullmatch(r'[0-9a-f]{40}', target): raise Failure('fetch')
                    if rev is None: break
                    resolved = cmd.run('rev-parse', '--disambiguate=' + rev, check=False)
                    matches = resolved['stdout'].splitlines()
                    requested = matches[0] if len(matches) == 1 else ''
                    is_commit = bool(re.fullmatch(r'[0-9a-f]{40}', requested)) and cmd.text('cat-file', '-t', requested) == 'commit'
                    if resolved['status'] == 0 and is_commit and ancestry(cmd, requested, target):
                        record['requested'] = requested
                        break
                    if index == FETCH_ATTEMPTS - 1: raise Failure('not_in_branch')
                    time.sleep(min(.1, max(0, deadline-time.monotonic())))
                if action == 'deploy' and rev is not None: target = record['requested']
                if not destructive:
                    actual = cmd.run('symbolic-ref', '--quiet', '--short', 'HEAD', check=False)['stdout'].strip()
                    if actual != chosen or not ancestry(cmd, record['from'], target): raise Failure('diverged')
                    cmd.run('merge', '--ff-only', '--no-edit', target, '--')
                else:
                    if in_operation(cmd): raise Failure('dirty')
                    before = dirty_paths(cmd)  # Snapshot after fetch, immediately before destructive work.
                    actual = cmd.run('symbolic-ref', '--quiet', '--short', 'HEAD', check=False)['stdout'].strip()
                    if record['discarded'] is not None:
                        tips = {record['from']}
                        local = cmd.run('rev-parse', '--verify', 'refs/heads/' + chosen + '^{commit}', check=False)
                        if local['status'] == 0: tips.add(local['stdout'].strip())
                        record['discarded']['commits'] = int(cmd.text('rev-list', '--count', *sorted(tips), '--not', target, '--'))
                    cmd.run('checkout', '--force', '-B', chosen, target, '--')
                    cmd.run('reset', '--hard', target, '--')
                    cmd.run('clean', '-fd', '--')
                record['to'] = head(cmd)
                # Persist logical branch independently of HEAD (future detached reads).
                g.store_autopull(branch_file(root), {'branch':chosen})
                if record['discarded'] is not None:
                    record['discarded']['files'] = len(before - dirty_paths(cmd))
                record['result'] = 'ok' if record['to'] != record['from'] or destructive and (bool(before - dirty_paths(cmd)) or actual != chosen) else 'skip'
                code = None
            except (Failure, g.GitError, ValueError, OSError) as error:
                code = error.code if isinstance(error, Failure) else 'fetch'
                # A failed write might have moved HEAD; report the actual safe hash.
                if safe:
                    try: record['to'] = head(cmd)
                    except (Failure, g.GitError, ValueError, OSError): pass
            return finish(code, hooked=safe)
    except TimeoutError:
        if locked: raise
        return finish('lock')


def canonical_channel(name, identity):
    return g.channel_name(name, identity)


def event_repositories(config, body):
    """Unambiguous strict JSON hint; malformed/duplicate JSON cannot claim a repo."""
    try: hint = g.load_json(body, EVENT_MAX)
    except (ValueError, UnicodeError): return []
    if not isinstance(hint, dict) or not project(hint.get('remote')): return []
    selected = []
    for name, repo in config['repos'].items():
        if not repo.get('follow'): continue
        try:
            cmd = Commands(Path(repo['path']).resolve(), config, time.monotonic() + BUDGET)
            if remote_path(cmd.text('remote', 'get-url', 'origin')) == hint['remote']:
                selected.append(name)
        except (ValueError, OSError, Failure, g.GitError): continue
    return selected


class Runtime:
    """Two bounded pools keep update locks/scripts off the inbox reader."""
    def __init__(self, bus):
        self.bus = bus
        self.updates = ThreadPoolExecutor(max_workers=2, thread_name_prefix='git-update')
        self.reads = ThreadPoolExecutor(max_workers=2, thread_name_prefix='git-call')
        self.update_slots = threading.BoundedSemaphore(8)
        self.read_slots = threading.BoundedSemaphore(8)

    def config(self):
        self.bus.refresh('config')
        return g.configuration(json.dumps(self.bus.config), identity=self.bus.name)

    def child(self, config, message):
        env = {k:v for k,v in os.environ.items() if not k.startswith(('AB_ROLE_', 'AGENT_BUS_'))}
        env.update(AGENT_BUS_CONFIG=json.dumps(config), AGENT_BUS_NAME=self.bus.name,
                   AGENT_BUS_FROM=getattr(message, 'from'), GIT_AGENT_DEADLINE=str(time.monotonic()+BUDGET))
        if message.ask: env['AGENT_BUS_ASK'] = message.ask
        for role in message.roles:
            if role in ('owner','maintainer','deploy','commit'): env['AB_ROLE_' + role.upper()] = '1'
        # Reuse bounded file-backed capture, including kill-before-wait.
        with g.tempfile.TemporaryFile() as out, g.tempfile.TemporaryFile() as err:
            child = subprocess.Popen([sys.executable, str(g.PACKAGE/'git_agent.py'), message.body],
                                     stdout=out, stderr=err, env=env, start_new_session=True)
            try: child.wait(timeout=BUDGET+1)
            except subprocess.TimeoutExpired:
                try: os.killpg(child.pid, signal.SIGKILL)
                except ProcessLookupError: pass
                child.wait()
                return 'error: call deadline exceeded; inspect the current revision'
            out.seek(0); value = out.read(g.LIMIT + 1)
        return value[:g.LIMIT].decode(errors='replace').rstrip() if len(value) <= g.LIMIT else 'error: answer exceeds limit'

    def answer(self, config, message, call):
        if call is None: return self.child(config, message)
        if not set(message.roles) & {'owner','maintainer','deploy','commit'}:
            return 'error: requires deploy role'
        try:
            return json.dumps(attempt(call['repo'], config,
                action='deploy' if call['method'] == 'deploy' else 'autopull',
                branch=call.get('branch'), rev=call.get('rev'), event_id=call.get('id'),
                requested_by=getattr(message, 'from'), bus=self.bus, agent=self.bus.name))
        except (ValueError, OSError): return 'error: repository update refused'

    def events(self, config, message):
        names = event_repositories(config, message.body)
        try: parsed = event(message.body)
        except (ValueError, UnicodeError): parsed = None
        for name in names:
            repo = config['repos'][name]
            try:
                if parsed is not None:
                    cmd = Commands(Path(repo['path']).resolve(), config, time.monotonic()+BUDGET)
                    # Events, including deploy, follow this copy's effective branch.
                    if effective_branch(cmd, repo) != parsed['branch']: continue
                attempt(name, config, trigger='event', action='deploy' if parsed and parsed['type'] == 'deploy' else 'autopull',
                    branch=parsed['branch'] if parsed else None, rev=parsed['commit'] if parsed else None,
                    event_id=parsed.get('id') if parsed else None, invalid=parsed is None,
                    requested_by=getattr(message,'from'), bus=self.bus, agent=self.bus.name)
            except (ValueError, OSError): print('git: event update refused', file=sys.stderr)

    def submit(self, pool, slots, fn, message=None):
        if not slots.acquire(blocking=False):
            if message is not None and not message.original_to:
                try: message.reply('error: busy; update was not admitted')
                except Exception: pass
            print('git: admission busy', file=sys.stderr)
            return False
        def work():
            try:
                value = fn()
                if message is not None and value is not None: message.reply(value)
            except Exception:
                print('git: request failed', file=sys.stderr)
                if message is not None and not message.original_to:
                    try: message.reply('error: request could not be completed; inspect current revision')
                    except Exception: pass
            finally: slots.release()
        pool.submit(work)
        return True

    def startup(self):
        try: config = self.config()
        except Exception:
            print('git: Configuration unavailable or invalid', file=sys.stderr)
            return
        # At most 64; bounded pool: startup itself does not block help/reader.
        def start():
            for name, repo in config['repos'].items():
                if repo.get('follow'):
                    try: attempt(name, config, trigger='start', bus=self.bus, agent=self.bus.name)
                    except (ValueError, OSError): print('git: repository startup check failed', file=sys.stderr)
        self.submit(self.updates, self.update_slots, start)

    def dispatch(self, message):
        if message.receipt or message.re: return
        if message.original_to and getattr(message, 'from') == self.bus.name: return
        try: config = self.config()
        except Exception:
            # Help still describes how to configure an unconfigured installation.
            config = {'repos':{}}
            print('git: Configuration unavailable or invalid; forwarded events dropped', file=sys.stderr)
        channel = config.get('autopull-cmd')
        if message.original_to:
            if channel and message.original_to == canonical_channel(channel, self.bus.name):
                self.submit(self.updates, self.update_slots, lambda: self.events(config, message))
            return  # Never derive direct-call authority from a forwarded role.
        call = None
        if not message.ask:
            try:
                parsed = g.parse(message.body)
                if parsed['method'] == 'deploy' or parsed['method'] == 'autopull' and parsed.get('now'): call = parsed
            except ValueError: pass  # Child produces the ordinary parser/help error.
        update = call is not None
        self.submit(self.updates if update else self.reads, self.update_slots if update else self.read_slots,
                    lambda: self.answer(config, message, call), message)

    def close(self):
        self.updates.shutdown(wait=True, cancel_futures=True)
        self.reads.shutdown(wait=True, cancel_futures=True)


def serve():
    sys.path.insert(0, str(g.PACKAGE.parents[2] / 'clients/python'))
    from agent_bus import Bus
    from agent_bus.errors import BusError
    stop = threading.Event()
    signal.signal(signal.SIGTERM, lambda *_: stop.set())
    signal.signal(signal.SIGINT, lambda *_: stop.set())
    with Bus(timeout=5) as bus:
        runtime = Runtime(bus)
        runtime.startup()
        try:
            while not stop.is_set():
                try: message = bus.consume(wait=.2)
                except BusError:
                    print('git: bus unavailable', file=sys.stderr)
                    stop.wait(1)
                    continue
                if message is not None: runtime.dispatch(message)
        finally: runtime.close()
