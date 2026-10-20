#!/usr/bin/env python3
"""Dev-node bounds. Root owns the installed copy; no session records are deleted."""
import argparse
import os
from pathlib import Path
import pwd
import re
import shutil
import subprocess
import time

REPO = Path('/usr/local/src/ai-agent-bus')
GiB = 1024 ** 3


def size(path):
    if not path.exists():
        return 0
    return int(subprocess.check_output(['du', '-sb', '--', str(path)], text=True).split()[0])


def remove(path):
    # Never follow a candidate symlink into another project's files.
    if path.is_symlink():
        return
    print('remove', path, flush=True)
    if path.is_dir():
        shutil.rmtree(path)
    else:
        path.unlink()


def retain(paths, count, protected=()):
    paths = sorted(paths, key=lambda p: p.stat().st_mtime, reverse=True)
    keep = set(protected)
    for p in paths:
        if len(keep) >= count:
            break
        keep.add(p)
    for p in paths:
        if p not in keep:
            remove(p)


def ours_crash(path):
    for key in ('executable', 'cmdline', 'reason'):
        try:
            value = (path / key).read_text(errors='replace')
        except OSError:
            continue
        if '/ai-agent-bus/' in value or 'launcher-smoke' in value:
            return True
    return False


def ours_tmp(path, uid, tests):
    if path.is_symlink() or not path.is_dir() or path.stat().st_uid != uid:
        return False
    if path.name.startswith('tmp.'):
        return (path / 'agent-busd').is_file() or (path / 'bin/agent-busd').is_file()
    if path.name.startswith('launcher-smoke'):
        return any('ai-agent-bus' in p.read_text(errors='ignore') for p in path.glob('*.log'))
    if path.name.startswith('go-build'):
        return any('github.com/parf/ai-agent-bus' in p.read_text(errors='ignore')
                   for pattern in ('*/importcfg', '*/importcfg.link') for p in path.glob(pattern))
    return any(path.name.startswith(name) and re.search(r'\d+$', path.name) for name in tests)


def busy_paths():
    # Keep any leftover still referenced by a process, even if it is old.
    paths = []
    for proc in Path('/proc').glob('[0-9]*'):
        for p in [proc / 'cwd', proc / 'exe', *list((proc / 'fd').glob('*'))]:
            try:
                paths.append(os.readlink(p).removesuffix(' (deleted)'))
            except OSError:
                pass
    return paths


def root_cleanup(age):
    uid = pwd.getpwnam('parf').pw_uid
    tests = set()
    for p in (REPO / 'src').rglob('*_test.go'):
        if 'node_modules' not in p.parts:
            tests.update(re.findall(r'func (Test\w+)\(', p.read_text()))
    busy = busy_paths()
    for p in Path('/tmp').iterdir():
        if p.stat(follow_symlinks=False).st_mtime > time.time() - age:
            continue
        if ours_tmp(p, uid, tests) and not any(x == str(p) or x.startswith(str(p) + '/') for x in busy):
            remove(p)
    spool = Path('/var/spool/abrt')
    crashes = [p for p in spool.iterdir() if p.is_dir() and not p.is_symlink() and ours_crash(p)]
    # Only our crash entries count toward this cap. Foreign application dumps stay intact.
    for p in sorted(crashes, key=lambda p: p.stat().st_mtime):
        if sum(size(q) for q in crashes if q.exists()) <= 100 * 1024 ** 2:
            break
        remove(p)
    releases = Path('/usr/local/lib/agent-bus/releases')
    current = Path('/usr/local/lib/agent-bus/current').resolve()
    retain([p for p in releases.iterdir() if p.is_dir() and not p.is_symlink()], 2, [current])
    backup = Path('/var/lib/agent-bus/daemon.backup')
    retain([p for p in backup.iterdir() if p.is_file() and (p.name.endswith('.tar') or p.name.endswith('.tar.gz'))], 3)
    # Sidecar unit copies are kept only alongside a retained archive of the same stem.
    for p in backup.glob('*.service'):
        if not p.with_suffix('.tar').exists() and not p.with_suffix('.tar.gz').exists():
            remove(p)
    # Live files are handled by logrotate; trim only rotated daemon log files here.
    logs = Path('/var/log/agent-bus')
    for name in ('audit', 'error', 'debug'):
        retain(list(logs.glob(name + '.log-*')), 3)


def user_cleanup():
    if os.getuid() != pwd.getpwnam('parf').pw_uid:
        raise SystemExit('user cleanup must run as parf')
    cache = Path.home() / '.cache/go-build'
    if size(cache) > 10 * GiB:
        subprocess.run(['go', 'clean', '-cache'], env={**os.environ, 'GOCACHE': str(cache)}, check=True)
    target = REPO / 'tmp/cargo-target'
    if size(target) > 5 * GiB:
        subprocess.run(['cargo', 'clean', '--manifest-path', str(REPO / 'src/clients/rust/Cargo.toml'),
                        '--target-dir', str(target)], check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', action='store_true')
    parser.add_argument('--tmp-age', type=int, default=86400, help='minimum age in seconds (default one day)')
    args = parser.parse_args()
    # Share the test/build coordination lock. Skip this week rather than interrupt a gate.
    bus = ['runuser', '-u', 'parf', '--', '/usr/local/bin/agent-bus'] if args.root else ['/usr/local/bin/agent-bus']
    lock = [*bus, 'lock', 'smoke-gate@parf.us', 'slow', '--ttl', '15m', '--wait', '1ms']
    if subprocess.run(lock, stdout=subprocess.DEVNULL).returncode:
        return
    try:
        if args.root:
            if os.getuid() != 0:
                raise SystemExit('--root requires root')
            root_cleanup(args.tmp_age)
        else:
            user_cleanup()
    finally:
        subprocess.run([*bus, 'release', 'smoke-gate@parf.us', 'slow'], check=True)


if __name__ == '__main__':
    main()
