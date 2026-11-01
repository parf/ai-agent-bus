#!/usr/bin/env python3
"""Focused MAC.2 install acceptance. Run only on the authorized Mac as its user.

Every direct sudo invocation is printed. The launchd job and setup files are
removed in finally, including after a failed assertion; the checked hidden role
account is kept for reuse. No existing daemon/tree is replaced; no profiles or
packages change.
"""
import argparse
import contextlib
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shlex
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import tarfile
import threading
import time

BASE = Path.home() / 'agent-bus-dev'
REPO = BASE / 'ai-agent-bus'
HOME = Path('/usr/local/var/agent-bus/daemon')
RUN = Path('/usr/local/var/run/agent-bus')
PREFIX = Path('/usr/local/lib/agent-bus/mac2-check')
PLIST = Path('/Library/LaunchDaemons/agent-busd.plist')
ROLE = '_agentbusd'
JOB = 'system/agent-busd'
OWNER = 'mac2-check@localhost'
AGENT = '#mac2-check@localhost'
COMMAND_LINKS = {}
ENV = {k: v for k, v in os.environ.items() if not k.startswith(('AGENT_BUS_', 'AB_ROLE_'))}
ENV.update(GOTOOLCHAIN='local', GOCACHE=str(BASE / '.cache/go-build'),
           GOMODCACHE=str(BASE / '.cache/gomod'), BUN_INSTALL_CACHE_DIR=str(BASE / '.cache/bun'))


def run(args, *, timeout=20, check=True, private=False, cwd=None):
    args = [str(x) for x in args]
    print(('SUDO: ' if args[0] == 'sudo' else 'RUN: ') + shlex.join(args), flush=True)
    p = subprocess.run(args, cwd=cwd, env=ENV, stdin=subprocess.DEVNULL,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=timeout)
    if check and p.returncode:
        raise RuntimeError('command exited %s: %s\n%s' % (p.returncode, shlex.join(args), p.stderr))
    if not private and p.stdout:
        print(p.stdout, end='' if p.stdout.endswith('\n') else '\n', flush=True)
    if p.stderr:
        print(p.stderr, file=sys.stderr, end='' if p.stderr.endswith('\n') else '\n', flush=True)
    return p


def sudo(*args, **kw):
    return run(['sudo', '-n', *args], **kw)


def root_exists(path):
    # An unreadable parent must never be mistaken for absence.
    result = sudo('/bin/sh', '-c', 'test -e "$1" || test -L "$1"', 'mac2-check', path,
                  check=False, private=True)
    assert result.returncode in (0, 1) and not result.stderr, 'cannot establish path absence: ' + str(path)
    return result.returncode == 0


def alive(pid):
    return bool(run(['/bin/ps', '-p', str(pid), '-o', 'pid='], check=False, private=True).stdout.strip())


def job_pid():
    text = sudo('/bin/launchctl', 'print', JOB, private=True).stdout
    match = re.search(r'^\s*pid = (\d+)\s*$', text, re.M)
    if not match:
        raise AssertionError('launchd job has no process pid')
    return int(match[1])


def role_token(name):
    # Credential JSON is captured, never printed or passed in command arguments.
    response = sudo('-u', ROLE, '/usr/bin/curl', '--fail', '--silent', '--show-error',
                    '--max-time', '3', '--unix-socket', RUN / ('user-' + ROLE + '.sock'),
                    '-H', 'Content-Type: application/json', '--data', json.dumps({'name': name}),
                    'http://localhost/token', private=True)
    value = json.loads(response.stdout)
    assert isinstance(value.get('token'), str) and value['token']
    return value['token']


def disk(label):
    print(label, flush=True)
    run(['/bin/df', '-h', BASE])
    run(['/usr/bin/du', '-sh', BASE])
    for p in (Path.home() / 'go', Path.home() / 'Library/Caches/go-build'):
        assert not p.exists(), 'unexpected cache outside workspace: ' + str(p)
        print('absent: ' + str(p), flush=True)


def role_account():
    """Allow only setup's hidden, non-login role pair, or a fresh absent pair."""
    present = []
    for node in ('Users', 'Groups'):
        found = run(['/usr/bin/dscl', '.', '-list', '/' + node], private=True)
        assert not found.stderr, 'cannot inspect role ' + node
        present.append(ROLE in found.stdout.splitlines())
    assert present[0] == present[1], 'incomplete role account/group pair: ' + ROLE
    if not present[0]:
        return False
    def fields(node):
        result = run(['/usr/bin/dscl', '.', '-read', '/' + node + '/' + ROLE], private=True)
        assert not result.stderr, 'cannot inspect role ' + node
        return {key: value.strip() for key, value in re.findall(r'^([A-Za-z]+): (.*)$', result.stdout, re.M)}
    user, group = fields('Users'), fields('Groups')
    assert user.get('UserShell') == '/usr/bin/false', 'unsafe role UserShell: ' + ROLE
    assert user.get('IsHidden') == '1', 'unsafe role IsHidden: ' + ROLE
    assert user.get('NFSHomeDirectory') == str(HOME), 'unsafe role NFSHomeDirectory: ' + ROLE
    for field in ('UniqueID', 'PrimaryGroupID'):
        assert re.fullmatch(r'[0-9]+', user.get(field, '')), 'invalid role ' + field
        assert 200 <= int(user[field]) < 400, 'unsafe role ' + field
    assert group.get('PrimaryGroupID') == user['PrimaryGroupID'] == user['UniqueID'], 'unsafe role group id: ' + ROLE
    for privileged in ('admin', 'wheel'):
        result = run(['/usr/bin/dscl', '.', '-read', '/Groups/' + privileged, 'GroupMembership'], private=True)
        assert not result.stderr, 'cannot inspect privileged group ' + privileged
        membership = re.search(r'^GroupMembership:(.*)$', result.stdout, re.M)
        assert membership is not None, 'cannot inspect GroupMembership: ' + privileged
        assert ROLE not in membership[1].split(), 'role belongs to privileged group ' + privileged
    print('PASS existing hidden non-login role account is safe to reuse', flush=True)
    return True


def preflight(archive=False):
    role_account()
    assert sudo('/bin/launchctl', 'print', JOB, check=False, private=True).returncode != 0, 'existing launchd job'
    for path in (HOME, RUN, PREFIX, PLIST, *COMMAND_LINKS, *([Path('/usr/local/bin/agent-bus-web')] if archive else [])):
        assert not root_exists(path), 'refusing to replace existing setup path: ' + str(path)


def uninstall(missing_parents):
    print('UNINSTALL', flush=True)
    sudo('/bin/launchctl', 'bootout', JOB, check=False)
    sudo('/usr/bin/pkill', '-KILL', '-u', ROLE, check=False)
    sudo('/bin/rm', '-f', PLIST)
    for link, target in COMMAND_LINKS.items():
        if root_exists(link):
            assert link.is_symlink() and os.readlink(link) == str(target), 'unexpected command link; refusing removal: ' + str(link)
            sudo('/bin/rm', '-f', link)
    sudo('/bin/rm', '-rf', HOME, RUN, PREFIX)
    for parent in sorted(missing_parents, key=lambda p: len(p.parts), reverse=True):
        sudo('/bin/rmdir', parent)
    assert not root_exists(HOME) and not root_exists(RUN) and not root_exists(PREFIX) and not root_exists(PLIST)
    assert all(not root_exists(link) for link in COMMAND_LINKS), 'command link remains'
    assert sudo('/bin/launchctl', 'print', JOB, check=False, private=True).returncode != 0, 'launchd job remains'
    assert role_account(), 'kept role account/group is missing'
    print('role account kept (by design)', flush=True)
    print('PASS uninstall removed the fixture job and setup files', flush=True)



def uninstall_after_checks(missing_parents):
    # No partial account-removal exit: the role account is deliberately kept.
    uninstall(missing_parents)


def archive_bundle(archive, work):
    """Check the delivered bytes before executing any archive program."""
    archive = archive.resolve(strict=True)
    cpu = {'arm64': 'aarch64', 'x86_64': 'x86_64'}[platform.machine()]
    match = re.fullmatch(r'agent-bus-(\d+\.\d+\.\d+)-darwin-' + cpu + r'\.tar\.gz', archive.name)
    assert match, 'need the full Darwin archive for this Mac CPU'
    checksum = Path(str(archive) + '.sha256').read_text().split()
    assert len(checksum) == 2 and checksum[1] == archive.name, 'unexpected archive checksum entry'
    assert hashlib.sha256(archive.read_bytes()).hexdigest() == checksum[0], 'archive checksum mismatch'
    name = archive.name[:-7]
    destination = work / 'archive'
    destination.mkdir()
    with tarfile.open(archive, 'r:gz') as bundle:
        members = bundle.getmembers()
        aliases = {name + '/' + launcher: 'launchers/' + launcher for launcher in
                   ('ab-claude', 'ab-codex', 'ab-opencode', 'ab-opencode1')}
        for member in members:
            path = Path(member.name)
            assert not path.is_absolute() and '..' not in path.parts and path.parts[0] == name, 'unsafe archive path'
            assert (member.isfile() or member.isdir() or
                    member.issym() and aliases.get(member.name) == member.linkname), 'unsafe archive link or special file'
        # Only the four known, inward-pointing launcher aliases may be links.
        bundle.extractall(destination, members=members)
    root = destination / name
    manifest = root / 'MANIFEST.sha256'
    checked = set()
    for line in manifest.read_text().splitlines():
        digest, relative = line.split('  ', 1)
        path = Path(relative)
        assert not path.is_absolute() and '..' not in path.parts, 'unsafe manifest path'
        assert relative not in checked, 'duplicate manifest entry'
        checked.add(relative)
        assert hashlib.sha256((root / path).read_bytes()).hexdigest() == digest, 'manifest mismatch: ' + relative
    delivered = {str(p.relative_to(root)) for p in root.rglob('*') if p.is_file() and not p.is_symlink()}
    # Same non-runtime example exception as npm's release reader. Setup's
    # artifact allowlist intentionally does not install this example.
    assert delivered - checked <= {'MANIFEST.sha256', 'launchers/review-settings.json.dist'}, 'archive contains unchecked files'
    version = match[1]
    assert (root / 'internal/version/VERSION').read_text().strip() == version
    for program in ('agent-bus', 'agent-busd', 'agent-bus-admin', 'agent-bus-runner', 'agent-bus-setup', 'agent-bus-ts'):
        assert (root / program).stat().st_mode & 0o111, 'missing executable: ' + program
    return root, version


def check_archive_build(bundle, version, revision):
    for program, args in [('agent-bus', ['--version']), ('agent-busd', ['--version']),
                          ('agent-bus-admin', ['--version']), ('agent-bus-runner', ['--version']),
                          ('agent-bus-setup', ['--version']), ('agent-bus-ts', ['mcp', '--version']),
                          ('agent-bus-ts', ['launcher', 'claude', '--version']),
                          ('agent-bus-ts', ['web', '--version'])]:
        answer = run([bundle / program, *args]).stdout
        assert answer.splitlines()[0] == version, 'archive version differs from source: ' + program
        # MCP and launcher queries report only the version. The same compiled
        # TS binary exposes its build stamp through the web part's version query.
        if program != 'agent-bus-ts' or args[0] == 'web':
            stamp = re.search(r'^build_info: .* ([0-9a-f]{40})$', answer, re.M)
            assert stamp and stamp[1] == revision, 'archive revision differs from source: ' + program


def command_links():
    root = PREFIX / 'current'
    links = {Path('/usr/local/bin') / name: root / name for name in
             ('agent-bus', 'agent-busd', 'agent-bus-admin', 'agent-bus-runner', 'agent-bus-setup', 'agent-bus-token')}
    links.update({Path('/usr/local/bin') / name: root / 'launchers' / name for name in
                  ('ab-claude', 'ab-codex', 'ab-opencode', 'ab-opencode1')})
    links[Path('/usr/local/bin/agent-bus-mcp')] = root / 'agent-bus-ts'
    return links


def main():
    assert platform.system() == 'Darwin', 'this acceptance runs only on the authorized Mac'
    assert os.geteuid() != 0, 'run as the installing user; sudo is limited and listed'
    global PREFIX, COMMAND_LINKS
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('revision')
    parser.add_argument('--archive', type=Path, help='native full archive plus its .sha256 file; no build or --exec')
    options = parser.parse_args()
    revision = options.revision
    if options.archive:
        PREFIX = Path('/usr/local/lib/agent-bus')
        COMMAND_LINKS = command_links()
    assert re.fullmatch('[0-9a-f]{40}', revision)
    assert not run(['git', 'status', '--porcelain'], cwd=REPO, private=True).stdout
    assert run(['git', 'branch', '--show-current'], cwd=REPO, private=True).stdout.strip() == 'main'
    disk('BEFORE')
    work = Path(tempfile.mkdtemp(prefix='mac2-install-', dir=BASE))
    ENV['TMPDIR'] = str(work / 'tmp')
    (work / 'tmp').mkdir()
    armed, missing_parents = False, []
    buses = []
    try:
        bundle, version = archive_bundle(options.archive, work) if options.archive else (None, None)
        sudo('/usr/bin/true')
        preflight(bool(options.archive))
        parents = {p for leaf in (HOME, RUN, PREFIX, PLIST, *COMMAND_LINKS) for p in leaf.parents if str(p) not in ('/', '/usr', '/Library')}
        for path in sorted(parents, key=lambda p: len(p.parts)):
            if not root_exists(path):
                missing_parents.append(path)
        run(['git', 'switch', '--detach', revision], cwd=REPO)
        if not options.archive:
            run(['go', 'version'])
            run(['bun', '--version'])
            stamp = '-X=github.com/parf/ai-agent-bus/internal/version.Build=MAC2-' + revision[:8]
            run(['go', 'build', '-ldflags', stamp, '-o', str(work / 'bin') + '/',
                 './cmd/agent-busd', './cmd/agent-bus-setup', './cmd/agent-bus-admin'], cwd=REPO / 'src', timeout=180)
        else:
            assert (REPO / 'src/internal/version/VERSION').read_text().strip() == version, 'archive and acceptance revision versions differ'
            check_archive_build(bundle, version, revision)
            print('PASS native full archive checksum, manifest, versions and build stamps', flush=True)
        run(['/usr/bin/ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', work / 'fixture-key'])
        with socket.socket() as probe:
            probe.bind(('127.0.0.1', 0))
            port = probe.getsockname()[1]
        armed = True
        if options.archive:
            sudo('/usr/bin/env', 'AGENT_BUS_TLS_DIR=' + str(HOME / 'tls'), bundle / 'agent-bus-setup',
                 '--owner', OWNER, '--addr', '127.0.0.1:' + str(port),
                 '--key', work / 'fixture-key.pub', '--tls', 'self-signed', timeout=45)
            for link, target in COMMAND_LINKS.items():
                assert link.is_symlink() and os.readlink(link) == str(target), 'incorrect installed link: ' + str(link)
            assert (PREFIX / 'current/agent-busd').read_bytes() == (bundle / 'agent-busd').read_bytes()
            assert run([PREFIX / 'current/agent-bus-token', '--version']).stdout.splitlines()[0] == version
            print('PASS full archive installed by reduced setup without --exec, token wrapper version', flush=True)
        else:
            sudo('/usr/bin/install', '-d', '-m', '0755', PREFIX)
            for name in ('agent-busd', 'agent-bus-setup', 'agent-bus-admin'):
                sudo('/usr/bin/install', '-m', '0755', work / 'bin' / name, PREFIX / name)
            print('SETUP CHILD SUDO (fresh state): sudo -u _agentbusd ' + str(PREFIX / 'agent-busd') +
                  ' --owner ' + OWNER + ' --db ' + str(HOME / 'agent-bus.db') + ' --init', flush=True)
            print('SETUP CHILD SUDO: sudo -u _agentbusd env AGENT_BUS_HOME=' + str(HOME) +
                  ' AGENT_BUS_ADDR=' + str(RUN / ('user-' + ROLE + '.sock')) + ' ' +
                  str(PREFIX / 'agent-bus-admin') + ' user add ' + OWNER + ' - --admin', flush=True)
            sudo('/usr/bin/env', 'AGENT_BUS_TLS_DIR=' + str(HOME / 'tls'), PREFIX / 'agent-bus-setup',
                 '--owner', OWNER, '--addr', '127.0.0.1:' + str(port), '--exec', PREFIX / 'agent-busd',
                 '--key', work / 'fixture-key.pub', '--tls', 'self-signed', timeout=45)
        pid = job_pid()
        user = run(['/bin/ps', '-p', str(pid), '-o', 'user='], private=True).stdout.strip()
        assert user == ROLE and user != 'root', 'daemon runs as wrong account: ' + user
        assert role_account(), 'setup did not create or reuse the role account'
        assert not os.access(HOME, os.R_OK | os.X_OK), 'installing user can read private storage'
        permissions = sudo('/usr/bin/stat', '-f', '%Lp', HOME, private=True).stdout.strip()
        assert permissions == '700', permissions
        print('PASS role account, launchd identity and private storage', flush=True)
        cert = sudo('-u', ROLE, '/usr/bin/openssl', 'x509', '-in', HOME / 'tls/cert.pem',
                    '-noout', '-sha256', '-fingerprint', private=True).stdout
        pin = 'sha256:' + cert.split('=', 1)[1].strip().replace(':', '').lower()
        assert re.fullmatch(r'sha256:[0-9a-f]{64}', pin)
        owner_token = role_token(OWNER)
        sys.path.insert(0, str(REPO / 'src/clients/python'))
        from agent_bus import Bus
        address = 'https://127.0.0.1:' + str(port)
        owner = Bus(OWNER, addr=address, token=owner_token, fingerprint=pin, timeout=1)
        buses.append(owner)
        owner.record(AGENT).register(kind='agent', allow=[OWNER])
        agent = Bus(AGENT, addr=address, token=role_token(AGENT), fingerprint=pin, timeout=1)
        buses.append(agent)
        errors, seen = [], []
        def respond():
            try:
                message = agent.consume(wait=3)
                assert message is not None and message.body == 'ping'
                seen.append(message.id)
                message.reply('pong')
            except Exception as error:
                errors.append(type(error).__name__ + ': ' + str(error))
        thread = threading.Thread(target=respond, daemon=True)
        thread.start()
        answer = owner.call(AGENT, 'ping', timeout=4)
        thread.join(5)
        assert not thread.is_alive() and not errors, errors
        assert len(seen) == 1 and getattr(answer, 'message', None).body == 'pong'
        assert answer.message.envelope['from'] == AGENT
        print('PASS pinned TLS request/reply, exactly once, agent identity', flush=True)
        for bus in buses:
            bus.close()
        buses.clear()
        def ready():
            until = time.monotonic() + 15
            while time.monotonic() < until:
                check = Bus(OWNER, addr=address, token=owner_token, fingerprint=pin, timeout=.5)
                try:
                    assert check.record(AGENT).lookup()['name'] == AGENT
                    return
                except Exception:
                    time.sleep(.1)
                finally:
                    check.close()
            raise AssertionError('registry/daemon did not return within 15 seconds')
        def changed_job(previous):
            end = time.monotonic() + 15
            while time.monotonic() < end:
                try:
                    current = job_pid()
                    if current != previous:
                        return current
                except (RuntimeError, AssertionError):
                    pass
                time.sleep(.2)
            raise AssertionError('launchd did not replace its old supervisor')
        previous = job_pid()
        sudo('/bin/launchctl', 'kickstart', '-k', JOB)
        changed_job(previous)
        ready()
        print('PASS registry survives launchd restart', flush=True)
        parent = job_pid()
        children = run(['/usr/bin/pgrep', '-P', str(parent)], private=True).stdout.split()
        assert children, 'supervisor has no bus child'
        sudo('/bin/kill', '-KILL', str(parent))
        changed_job(parent)
        ready()
        end = time.monotonic() + 10
        while any(alive(p) for p in children) and time.monotonic() < end:
            time.sleep(.1)
        assert all(not alive(p) for p in children), 'old bus child survived its supervisor'
        assert job_pid() != parent
        print('PASS supervisor kill leaves no old bus child and registry survives', flush=True)
    finally:
        for bus in buses:
            with contextlib.suppress(Exception):
                bus.close()
        try:
            if armed:
                uninstall_after_checks(missing_parents)
        finally:
            if not options.archive and Path(ENV['GOMODCACHE']).is_dir():
                run(['go', 'clean', '-modcache'], cwd=REPO / 'src', timeout=30)
            if not options.archive:
                shutil.rmtree(BASE / '.cache', ignore_errors=True)
            shutil.rmtree(work)
            run(['git', 'switch', 'main'], cwd=REPO)
            assert not run(['git', 'status', '--porcelain'], cwd=REPO, private=True).stdout
            disk('AFTER')


def cli():
    main()


if __name__ == '__main__':
    for sig in (signal.SIGINT, signal.SIGTERM, signal.SIGHUP):
        signal.signal(sig, lambda number, frame: sys.exit(128 + number))
    cli()
