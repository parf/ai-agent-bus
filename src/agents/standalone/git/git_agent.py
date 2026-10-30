#!/usr/bin/env python3
"""B.8: fixed git operations over configured repositories."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import stat
import shlex
import subprocess
import sys
import tempfile
import time
from contextlib import contextmanager

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'shared'))
from metadata import answer as metadata_answer, ASKS, dispatch
PACKAGE = Path(__file__).resolve().parent

TIERS = {'list': 0, 'autopull': 0, 'status': 0, 'log': 0, 'rev': 0, 'fetch': 0, 'diff': 0,
         'pull': 1, 'deploy': 1, 'add': 2, 'commit': 2, 'push': 2, 'reset': 1, 'clean': 1}
FIELDS = {'autopull': {'now'}, 'deploy': {'branch', 'rev', 'id'}, 'log': {'lines'}, 'pull': {'rev', 'file'}, 'add': {'paths'}, 'commit': {'message'}}
LIMIT = 256 * 1024
COMMAND_TIMEOUT = 60


def tier():
    if any(os.environ.get("AB_ROLE_" + role) == "1" for role in ("OWNER", "MAINTAINER")):
        return 2
    return 2 if os.environ.get('AB_ROLE_COMMIT') == '1' else 1 if os.environ.get('AB_ROLE_DEPLOY') == '1' else 0


def argument(value, what):
    if not isinstance(value, str) or not value or value.startswith('-') or any(ord(c) < 32 for c in value):
        raise ValueError('invalid ' + what + ': ' + repr(value))
    return value


def parse(body):
    if body.lstrip().startswith('{'):
        value = load_json(body)
    else:
        words = shlex.split(body)
        if not words:
            raise ValueError('expected a method')
        method = words.pop(0)
        value = {'method': method}
        if words:
            value['repo'] = words.pop(0)
        if method == 'autopull' and words == ['now']:
            value['now'] = True
        elif method == 'deploy' and len(words) <= 2:
            value.update(zip(('branch', 'rev'), words))
        elif method == 'pull' and len(words) <= 2:
            value.update(zip(('rev', 'file'), words))
        elif method == 'log' and len(words) == 1 and words[0].isdigit():
            value['lines'] = int(words[0])
        elif method == 'add':
            value['paths'] = words
        elif method == 'commit' and len(words) == 1:
            value['message'] = words[0]
        elif words:
            raise ValueError('unexpected arguments; use help')
    if not isinstance(value, dict) or not isinstance(value.get('method'), str) or value['method'] not in TIERS:
        raise ValueError('unknown method')
    method = value['method']
    allowed = {'method'} if method == 'list' else {'method', 'repo'} | FIELDS.get(method, set())
    if set(value) - allowed:
        raise ValueError('unknown field; roles and identity come from the server, not the call')
    if method != 'list':
        argument(value.get('repo'), 'repository name')
    if method == 'autopull' and 'now' in value and type(value['now']) is not bool:
        raise ValueError('now must be a boolean')
    if method == 'deploy':
        if 'branch' in value: validate_branch(value['branch'])
        if 'rev' in value and not isinstance(value['rev'], str) or 'rev' in value and not re.fullmatch(r'[0-9a-f]{7,40}', value['rev']):
            raise ValueError('deploy revision must be 7..40 lower-case hex')
        if 'id' in value and (not isinstance(value['id'], str) or not re.fullmatch(r'[A-Za-z0-9._-]{1,64}', value['id'])):
            raise ValueError('invalid id')
    if method == 'log':
        n = value.get('lines', 20)
        if type(n) is not int or not 1 <= n <= 1000:
            raise ValueError('lines must be an integer from 1 to 1000')
        value['lines'] = n
    if method == 'pull':
        if 'file' in value and 'rev' not in value:
            raise ValueError('file requires a revision')
        if 'rev' in value:
            rev = argument(value['rev'], 'revision')
            if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._/~-]*(?:\^[0-9]*)?', rev):
                raise ValueError('invalid revision: ' + repr(rev))
    if method == 'commit':
        argument(value.get('message'), 'commit message')
    if method == 'add':
        paths = value.get('paths')
        if not isinstance(paths, list) or not paths:
            raise ValueError('add requires paths')
    return value


def relative_path(root, value):
    value = argument(value, 'path')
    path = Path(value)
    if path.is_absolute() or '..' in path.parts or value.startswith(':') or '.git' in path.parts:
        raise ValueError('path outside repository or not a literal file: ' + repr(value))
    resolved = (root / path).resolve()
    if not resolved.is_relative_to(root):
        raise ValueError('path outside repository: ' + repr(value))
    return value


LEVELS = {'read': 0, 'deploy': 1, 'commit': 2}


def load_json(raw, limit=LIMIT):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result: raise ValueError('duplicate JSON field')
            result[key] = value
        return result
    def constant(_): raise ValueError('non-finite JSON value')
    if not isinstance(raw, str) or len(raw.encode('utf-8')) > limit or '\x00' in raw:
        raise ValueError('JSON exceeds bounds')
    try:
        value = json.loads(raw, object_pairs_hook=pairs, parse_constant=constant)
    except (RecursionError, UnicodeError):
        raise ValueError('invalid JSON') from None
    work = [(value, 0)]
    while work:
        item, depth = work.pop()
        if depth > 8: raise ValueError('JSON nesting exceeds bounds')
        if isinstance(item, dict): work.extend((v, depth + 1) for v in item.values())
        elif isinstance(item, list): work.extend((v, depth + 1) for v in item)
    return value


def validate_branch(branch):
    if (not isinstance(branch, str) or len(branch.encode()) > 255 or
        not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._/-]*', branch) or
        '..' in branch or '//' in branch or branch.endswith(('/', '.')) or
        any(part.startswith('.') or part.endswith('.lock') for part in branch.split('/'))):
        raise ValueError('invalid branch')
    return branch


def channel_name(name, identity):
    return name if '@' in name or '@' not in identity else name + '@' + identity.rsplit('@', 1)[1]


def configuration(raw=None, *, identity=None, check_scripts=True):
    config = load_json(os.environ.get('AGENT_BUS_CONFIG', '{}') if raw is None else raw)
    if not isinstance(config, dict) or set(config) - {'repos', 'ssh_key', 'autopull-cmd', 'autopull-log'}:
        raise ValueError('Configuration accepts repos, ssh_key, autopull-cmd and autopull-log')
    for key in ('autopull-cmd', 'autopull-log'):
        if key in config and (not isinstance(config[key], str) or not re.fullmatch(r'[a-z0-9][a-z0-9._/-]*(?:@[a-z0-9.-]+)?', config[key]) or len(config[key]) > 128):
            raise ValueError('Configuration channel must be a canonical channel name')
    identity = identity or os.environ.get('AGENT_BUS_NAME', '')
    if config.get('autopull-cmd') and config.get('autopull-log') and channel_name(config['autopull-cmd'], identity) == channel_name(config['autopull-log'], identity):
        raise ValueError('Configuration command and log channels must differ')
    repos = config.get('repos')
    if not isinstance(repos, dict) or len(repos) > 64:
        raise ValueError('Configuration repos must be an object of at most 64 repositories')
    for name, entry in repos.items():
        try:
            if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_-]{0,63}', name):
                raise ValueError('invalid name')
            if not isinstance(entry, dict) or set(entry) - {'path','access','branch','mode','follow','autopull-success-script','autopull-fail-script'}:
                raise ValueError('expected path, access and optional branch, mode, follow')
            path = entry.get('path')
            if not isinstance(path, str) or not Path(path).is_absolute() or '\x00' in path:
                raise ValueError('path must be absolute')
            if entry.get('access') not in LEVELS:
                raise ValueError('access must be read, deploy or commit')
            if 'branch' in entry: validate_branch(entry['branch'])
            if entry.get('mode', 'ff') not in ('ff', 'mirror') or type(entry.get('follow', False)) is not bool:
                raise ValueError('mode is ff or mirror; follow is boolean')
            if (entry.get('mode') == 'mirror' or entry.get('follow')) and LEVELS[entry['access']] < 1:
                raise ValueError('mirror/follow requires access deploy or commit')
            if entry.get('follow') and not config.get('autopull-cmd'):
                raise ValueError('follow requires autopull-cmd')
            for key in ('autopull-success-script', 'autopull-fail-script'):
                if key in entry:
                    script_name(entry[key])
                    if check_scripts:
                        fd = open_script(entry[key])
                        os.close(fd)
        except (ValueError, TypeError) as error:
            raise ValueError(f'Configuration repos {name!r}: {error}') from error
    return config


def repository_branch(root, declared, env):
    if 'branch' in declared:
        return declared['branch']
    current = git(root, ['symbolic-ref', '--quiet', '--short', 'HEAD'], env, check=False)
    if current['status'] == 0:
        return current['stdout'].strip()
    default = git(root, ['symbolic-ref', '--quiet', '--short', 'refs/remotes/origin/HEAD'], env, check=False)
    if default['status'] == 0 and default['stdout'].strip().startswith('origin/'):
        return default['stdout'].strip()[len('origin/'):]
    raise ValueError('Configuration repos: branch is required for this detached repository')


def git_environment(config=None, *, clean=False):
    env = {k:v for k,v in os.environ.items() if not clean or not k.startswith(('AGENT_BUS_', 'AB_ROLE_', 'GIT_'))}
    env.update(GIT_TERMINAL_PROMPT='0', GIT_LITERAL_PATHSPECS='1')
    config = configuration(check_scripts=False) if config is None else config
    if 'ssh_key' in config:
        key = argument(config['ssh_key'], 'ssh_key path')
        if not Path(key).is_absolute() or not Path(key).is_file() or not os.access(key, os.R_OK):
            raise ValueError('ssh_key must name a readable absolute file')
        env['GIT_SSH_COMMAND'] = shlex.join(['ssh', '-F', '/dev/null', '-o', 'BatchMode=yes',
            '-o', 'IdentitiesOnly=yes', '-o', 'IdentityAgent=none', '-i', key])
    return env


class GitError(Exception):
    def __init__(self, result):
        self.result = result


def git(root, args, env, check=True, timeout=None):
    # File-backed capture bounds returned output without pipe deadlocks.
    if timeout is None:
        timeout = COMMAND_TIMEOUT
        if 'GIT_AGENT_DEADLINE' in env:
            timeout = min(timeout, float(env['GIT_AGENT_DEADLINE']) - time.monotonic())
    if timeout <= 0:
        raise GitError({'status':124, 'error':'git deadline exceeded', 'stdout':'', 'stderr':'', 'truncated':False})
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        try:
            p = subprocess.Popen(['git', '-C', str(root), '--no-pager', *args], env=env,
                                 stdout=out, stderr=err, start_new_session=True)
            code = p.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            # Git's SSH/hook children share this private group. A timeout must
            # not leave a transport pushing after the caller was told it failed.
            try: os.killpg(p.pid, signal.SIGKILL)
            except ProcessLookupError: pass
            p.wait()
            code = 124
        out.seek(0); err.seek(0)
        a, b = out.read(LIMIT + 1), err.read(LIMIT + 1)
    result = {'stdout': a[:LIMIT].decode(errors='replace'), 'stderr': b[:LIMIT].decode(errors='replace'),
              'status': code, 'truncated': len(a) > LIMIT or len(b) > LIMIT}
    if check and code:
        result['error'] = 'git ' + args[0] + ' failed'
        raise GitError(result)
    return result


def state_directory():
    return Path(os.environ.get('XDG_STATE_HOME', str(Path.home() / '.local/state'))) / 'agent-bus-git'


def _trusted(info, directory=False):
    kind = stat.S_ISDIR if directory else stat.S_ISREG
    if (not kind(info.st_mode) or info.st_uid not in (0, os.geteuid())
            or info.st_mode & 0o022 or info.st_mode & (stat.S_ISUID | stat.S_ISGID)
            or not directory and not info.st_mode & 0o111):
        raise ValueError('post-attempt script or directory is not trusted')


def _trusted_open(path, *, directory=False):
    """Walk all parents with nofollow; never block opening a special file."""
    path = Path(path)
    if not path.is_absolute() or '..' in path.parts:
        raise ValueError('post-attempt script path is invalid')
    current = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        _trusted(os.fstat(current), True)
        for index, part in enumerate(path.parts[1:]):
            folder = index < len(path.parts) - 2 or directory
            flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
            if folder: flags |= os.O_DIRECTORY
            child = os.open(part, flags, dir_fd=current)
            os.close(current); current = child
            _trusted(os.fstat(current), folder)
        return current
    except BaseException:
        os.close(current)
        raise


def _resolve_trusted(path, *, directory=False):
    """Check each alias namespace too, not only its eventual target."""
    path = Path(path)
    if not path.is_absolute(): raise ValueError('script interpreter path is not absolute')
    current, pending, links, steps = Path('/'), list(path.parts[1:]), 0, 0
    _trusted(os.stat('/'), True)
    while pending:
        steps += 1
        if steps > 256: raise ValueError('script path resolution is too deep')
        part = pending.pop(0)
        if part == '.': continue
        if part == '..':
            current = current.parent
            continue
        child = current / part
        info = os.lstat(child)
        if stat.S_ISLNK(info.st_mode):
            links += 1
            if links > 40 or info.st_uid not in (0, os.geteuid()):
                raise ValueError('script alias is not trusted')
            target = Path(os.readlink(child))
            if target.is_absolute(): current = Path('/')
            pending = list(target.parts[1:] if target.is_absolute() else target.parts) + pending
            continue
        _trusted(info, bool(pending) or directory)
        current = child
    return current


def _script_executable(fd, depth=0):
    if depth > 4:
        raise ValueError('post-attempt script interpreter chain is too deep')
    header = os.pread(fd, 512, 0)
    if header.startswith(b'\x7fELF'):
        return
    first = header.split(b'\n', 1)[0]
    if not first.startswith(b'#!') or len(first) > 255:
        raise ValueError('post-attempt script must be executable ELF or have a shebang')
    words = first[2:].strip().split()
    if not words:
        raise ValueError('post-attempt script has no interpreter')
    interpreter = Path(os.fsdecode(words[0]))
    if not interpreter.is_absolute():
        raise ValueError('post-attempt script interpreter must be absolute')
    # System interpreter aliases (e.g. /bin/sh) may resolve to a trusted ELF.
    interpreter = _resolve_trusted(interpreter)
    handle = _trusted_open(interpreter)
    try: _script_executable(handle, depth + 1)
    finally: os.close(handle)


def script_name(name):
    if (not isinstance(name, str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,127}', name)
            or '..' in name):
        raise ValueError('post-attempt script must be a bare file name in autopull.d')
    return name


def open_script(name):
    """Configuration chooses only an admin-installed file within autopull.d."""
    script_name(name)
    fd = rootfd = None
    try:
        root = _resolve_trusted(state_directory() / 'autopull.d', directory=True)
        rootfd = _trusted_open(root, directory=True)
        target = _resolve_trusted(root / name)
        if not target.is_relative_to(root) or target == root:
            raise ValueError('post-attempt script must stay inside autopull.d')
        fd = _trusted_open(target)
        root_stat = os.stat(root, follow_symlinks=False)
        if (root_stat.st_dev, root_stat.st_ino) != (os.fstat(rootfd).st_dev, os.fstat(rootfd).st_ino):
            raise ValueError('post-attempt script directory changed')
        _script_executable(fd)
        return fd
    except (OSError, ValueError, RuntimeError):
        if fd is not None: os.close(fd)
        raise ValueError('post-attempt script is unavailable or untrusted') from None
    finally:
        if rootfd is not None: os.close(rootfd)


def recheck_script(fd, name):
    _trusted(os.fstat(fd))
    current = open_script(name)
    try:
        before, after = os.fstat(fd), os.fstat(current)
        if (before.st_dev, before.st_ino) != (after.st_dev, after.st_ino):
            raise ValueError('post-attempt script changed before execution')
    finally: os.close(current)


def autopull_file(root):
    return state_directory() / ('autopull-' + hashlib.sha256(str(root).encode()).hexdigest() + '.json')


@contextmanager
def checkout_lock(root, deadline):
    state_directory().mkdir(parents=True, exist_ok=True, mode=0o700)
    path = state_directory() / (hashlib.sha256(str(root).encode()).hexdigest() + '.lock')
    with path.open('a') as lock:
        while True:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                break
            except BlockingIOError:
                if time.monotonic() >= deadline: raise TimeoutError('checkout lock deadline') from None
                time.sleep(min(.02, max(0, deadline - time.monotonic())))
        try: yield
        finally: fcntl.flock(lock, fcntl.LOCK_UN)


def store_autopull(path, result):
    fd, tmp = tempfile.mkstemp(dir=path.parent, prefix=path.name + '.')
    try:
        with os.fdopen(fd, 'w') as out: json.dump(result, out)
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp): os.unlink(tmp)


def execute(repos, call):
    method = call['method']
    if tier() < TIERS[method]:
        raise ValueError('requires ' + ('commit' if TIERS[method] == 2 else 'deploy') + ' role')
    if method == 'list':
        return {'repos': sorted(repos), 'stdout': '\n'.join(sorted(repos)), 'status': 0}
    if method == 'deploy':
        raise ValueError('updates use channel-driven autopull')
    name = call['repo']
    if name not in repos:
        raise ValueError('undeclared repository: ' + name)
    declared = repos[name]
    if LEVELS[declared['access']] < TIERS[method]:
        raise ValueError(f"repository {name}: access cap {declared['access']} refuses {method}")
    root = Path(declared['path']).resolve(strict=True)
    if method == 'autopull':
        import autopull
        return {'status': 0, 'stdout': autopull.last_status(name)}
    env = git_environment()
    if git(root, ['rev-parse', '--show-toplevel'], env)['stdout'].strip() != str(root):
        raise ValueError('declared path is not a repository root')
    paths = [relative_path(root, p) for p in call.get('paths', [])]
    file = relative_path(root, call['file']) if 'file' in call else None
    # Each invocation takes a lock outside the work tree; parallel calls from
    # this instance cannot interleave multi-command deploys.
    with checkout_lock(root, time.monotonic() + COMMAND_TIMEOUT):
        branch = repository_branch(root, declared, env) if method in ('fetch','reset') else None
        return operation(root, branch, call, paths, file, env)


def operation(root, branch, call, paths, file, env):
    method = call['method']
    def run(*args): return git(root, list(args), env)
    def text(*args): return run(*args)['stdout'].strip()
    if method == 'status': return run('status', '--short', '--branch', '--untracked-files=all')
    if method == 'log': return run('log', '--oneline', '-n', str(call['lines']), '--')
    if method == 'rev':
        head = text('rev-parse', '--verify', 'HEAD')
        b = git(root, ['symbolic-ref', '--quiet', '--short', 'HEAD'], env, check=False)
        return {'head': head, 'branch': b['stdout'].strip() or None, 'stdout': head + '\n' + (b['stdout'].strip() or 'detached'), 'status': 0}
    if method == 'diff':
        result = run('diff', '--stat', 'HEAD', '--')
        result['stdout'] += run('ls-files', '--others', '--exclude-standard')['stdout']
        return result
    if method == 'add': return run('add', '--', *paths)
    if method == 'commit': return run('commit', '-m', call['message'], '--')
    if method == 'push': return run('push')
    if method == 'clean': return run('clean', '-fd', '--')
    if method == 'reset': return run('reset', '--hard', text('rev-parse', '--verify', '--end-of-options', branch + '@{upstream}^{commit}'), '--')
    old = text('rev-parse', '--verify', 'HEAD')
    run('fetch')
    if method == 'fetch':
        counts = text('rev-list', '--left-right', '--count', 'HEAD...' + branch + '@{upstream}')
        ahead, behind = map(int, counts.split())
        return {'ahead': ahead, 'behind': behind, 'status': 0, 'stdout': f'ahead {ahead}, behind {behind}'}
    if 'rev' in call:
        rev = call['rev']
        resolved = git(root, ['rev-parse', '--verify', '--end-of-options', rev + '^{commit}'], env, check=False)
        if resolved['status']:
            raise ValueError('unknown revision: ' + rev)
        sha = resolved['stdout'].strip()
        if file is not None:
            kind = git(root, ['cat-file', '-t', sha + ':' + file], env, check=False)
            if kind['status'] or kind['stdout'].strip() != 'blob':
                raise ValueError('not a file at revision: ' + file)
            return run('restore', '--source=' + sha, '--worktree', '--', file)
        return run('checkout', '--detach', sha, '--')
    return run('pull', '--ff-only')


def describe(repos, ask='help', as_json=False):
    cap = max((LEVELS[r['access']] for r in repos.values()), default=0)
    allowed = {m for m, required in TIERS.items() if required <= cap}
    scope = 'Repositories (cap): ' + ', '.join(f"{name} ({entry['access']})" for name, entry in repos.items())
    result = metadata_answer(PACKAGE, ask, allowed=allowed, as_json=as_json)
    if ask == 'help-json' or ask == 'help' and as_json:
        data = json.loads(result)
        data['description'] += '\n' + scope
        return json.dumps(data, ensure_ascii=False, separators=(',', ':'))
    return scope + '\n' + result if ask == 'help' else result


def answer(body):
    as_json = body.lstrip().startswith('{')
    try:
        repos = configuration(check_scripts=False)['repos']
        if body in ('help', '--help'):
            return describe(repos)
        if as_json and json.loads(body) == {'method':'help'}:
            return describe(repos, as_json=True)
        call = parse(body)
        if call['method'] == 'deploy' or call['method'] == 'autopull' and call.get('now'):
            import autopull
            if tier() < 1: raise ValueError('requires deploy role')
            return json.dumps(autopull.attempt(call['repo'], configuration(check_scripts=False), action='deploy' if call['method'] == 'deploy' else 'autopull', branch=call.get('branch'), rev=call.get('rev'), event_id=call.get('id'), requested_by=os.environ.get('AGENT_BUS_FROM')))
        result = {'method': call['method'], **execute(repos, call)}
    except GitError as e:
        result = e.result
    except (ValueError, OSError) as e:
        result = {'error': str(e), 'status': 1}
    if as_json: return json.dumps(result)
    if result.get('error'):
        return 'error: ' + result['error'] + '\n' + result.get('stderr', '').rstrip()
    return (result.get('stdout', '').rstrip() or 'ok') + ('\n[output truncated]' if result.get('truncated') else '')


def main():
    try:
        result = dispatch(PACKAGE, os.environ.get('AGENT_BUS_ASK'), handlers={key: lambda key=key: describe(configuration(check_scripts=False)['repos'], key) for key in ASKS})
    except (ValueError, OSError) as error:
        result = 'error: ' + str(error)
    if result is not None:
        print(result)
        return
    if sys.argv[1:] == ['--serve']:
        import autopull
        return autopull.serve()
    p = argparse.ArgumentParser(add_help=False)
    p.add_argument('body', nargs='?', default='')
    print(answer(p.parse_args().body))


if __name__ == '__main__':
    main()
