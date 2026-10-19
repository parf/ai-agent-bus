#!/usr/bin/env python3
"""B.8: fixed git operations over install-declared repositories."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import shlex
import subprocess
import sys
import tempfile

TIERS = {'list': 0, 'status': 0, 'log': 0, 'rev': 0, 'fetch': 0, 'diff': 0,
         'pull': 1, 'deploy': 1, 'add': 1, 'commit': 1, 'push': 1, 'reset': 2, 'clean': 2}
FIELDS = {'log': {'lines'}, 'pull': {'rev', 'file'}, 'add': {'paths'}, 'commit': {'message'}}
LIMIT = 256 * 1024
COMMAND_TIMEOUT = 60


def tier():
    return 2 if os.environ.get('AB_ROLE_ADMIN') == '1' else 1 if os.environ.get('AB_ROLE_OPERATE') == '1' else 0


def argument(value, what):
    if not isinstance(value, str) or not value or value.startswith('-') or any(ord(c) < 32 for c in value):
        raise ValueError('invalid ' + what + ': ' + repr(value))
    return value


def parse(body):
    if body.lstrip().startswith('{'):
        value = json.loads(body)
    else:
        words = shlex.split(body)
        if not words:
            raise ValueError('expected a method')
        method = words.pop(0)
        value = {'method': method}
        if words:
            value['repo'] = words.pop(0)
        if method == 'pull' and len(words) <= 2:
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


def git_environment():
    env = dict(os.environ)
    env.update(GIT_TERMINAL_PROMPT='0', GIT_LITERAL_PATHSPECS='1')
    config = json.loads(env.get('AGENT_BUS_CONFIG', '{}'))
    if not isinstance(config, dict) or set(config) - {'ssh_key'}:
        raise ValueError('Configuration accepts only ssh_key')
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


def git(root, args, env, check=True):
    # File-backed capture bounds returned output without pipe deadlocks.
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        try:
            p = subprocess.Popen(['git', '-C', str(root), '--no-pager', *args], env=env,
                                 stdout=out, stderr=err, start_new_session=True)
            code = p.wait(timeout=COMMAND_TIMEOUT)
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


def execute(repos, call):
    method = call['method']
    if tier() < TIERS[method]:
        raise ValueError('requires ' + ('admin' if TIERS[method] == 2 else 'operate') + ' role')
    if method == 'list':
        return {'repos': sorted(repos), 'stdout': '\n'.join(sorted(repos)), 'status': 0}
    name = call['repo']
    if name not in repos:
        raise ValueError('undeclared repository: ' + name)
    declared = repos[name]
    root = Path(declared['path']).resolve(strict=True)
    if str(root) != declared['path']:
        raise ValueError('installed repository path changed')
    env = git_environment()
    if git(root, ['rev-parse', '--show-toplevel'], env)['stdout'].strip() != str(root):
        raise ValueError('declared path is not a repository root')
    paths = [relative_path(root, p) for p in call.get('paths', [])]
    file = relative_path(root, call['file']) if 'file' in call else None
    # Each invocation takes a lock outside the work tree; parallel calls from
    # this instance cannot interleave multi-command deploys.
    state = Path(os.environ.get('XDG_STATE_HOME', str(Path.home() / '.local/state'))) / 'agent-bus-git'
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    with (state / (hashlib.sha256(str(root).encode()).hexdigest() + '.lock')).open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        return operation(root, declared['branch'], call, paths, file, env)


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
    if method == 'deploy':
        new = text('rev-parse', '--verify', '--end-of-options', branch + '@{upstream}^{commit}')
        run('checkout', '--force', branch, '--')
        run('reset', '--hard', new, '--')
        result = run('clean', '-fd', '--')
        return {**result, 'old_head': old, 'new_head': new, 'stdout': old + ' -> ' + new + '\n' + result['stdout']}
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


HELP = {
 'list': 'list: declared repository names; example: list',
 'status': 'status <repo>: branch and changed/untracked paths; example: status ab',
 'log': 'log <repo> [N]: recent commits, N 1..1000 (default 20); example: log ab 10',
 'rev': 'rev <repo>: HEAD and branch, or detached; example: rev ab',
 'fetch': 'fetch <repo>: fetch upstream and return ahead/behind counts; example: fetch ab',
 'diff': 'diff <repo>: local change stat and untracked paths; example: diff ab',
 'pull': 'pull <repo> [rev [file]]: ff-only pull, detached revision, or restore one file; example: pull ab main README.md',
 'deploy': 'deploy <repo>: fetch, installed branch, hard reset to upstream, clean; old/new HEAD; example: deploy ab',
 'add': 'add <repo> <path...>: stage literal paths; example: add ab README.md',
 'commit': 'commit <repo> "message": commit staged changes as server identity; example: commit ab "Update README"',
 'push': 'push <repo>: push using server identity/key; example: push ab',
 'reset': 'reset <repo>: hard reset to installed branch upstream; example: reset ab',
 'clean': 'clean <repo>: remove untracked files/directories; example: clean ab',
}


def describe():
    return ('git: inspect and synchronize install-declared repositories. Only delivered roles grant actions.\n'
            + '\n'.join(HELP[m] for m in TIERS if TIERS[m] <= tier())
            + '\nText as above; JSON: {"method":"status","repo":"ab"}. Optional keys: log lines; pull rev/file; add paths; commit message.\n'
            + 'Answers: stdout/stderr/status in JSON, output or error in text. help equals --help. Keep secrets in Secret and config in Configuration; ssh_key is a path only.\n')


def answer(repos, body):
    if body in ('help', '--help'):
        return describe()
    as_json = body.lstrip().startswith('{')
    try:
        call = parse(body)
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
    p = argparse.ArgumentParser(add_help=False)
    p.add_argument('--repos-json', required=True)
    p.add_argument('body', nargs='?', default='')
    args = p.parse_args()
    if os.environ.get('AGENT_BUS_ASK') == 'help': print(describe())
    elif os.environ.get('AGENT_BUS_ASK') == 'mcp': print('{"tools":[]}')
    else: print(answer(json.loads(args.repos_json), args.body))


if __name__ == '__main__':
    main()
