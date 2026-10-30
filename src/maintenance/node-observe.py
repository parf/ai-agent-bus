#!/usr/bin/env python3
"""Report disk use and growth; never clean, install or schedule anything."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import time


def measure(paths):
    """GNU du allocated bytes; errors are lower bounds, never a clean zero."""
    if not paths:
        return {'status': 'ok', 'bytes': 0}
    try:
        result = subprocess.run(['du', '-s', '-c', '-x', '-B1', '--', *map(str, paths)],
                                capture_output=True, text=True, timeout=60)
    except subprocess.TimeoutExpired:
        return {'status': 'timeout', 'bytes': None}
    except OSError:
        return {'status': 'unavailable', 'bytes': None}
    try:
        total = int(result.stdout.splitlines()[-1].split('\t', 1)[0])
    except (ValueError, IndexError):
        total = None
    return {'status': 'ok' if result.returncode == 0 and total is not None else 'partial',
            'bytes': total}


def directory(path):
    try:
        path.stat()
    except FileNotFoundError:
        return {'status': 'missing', 'bytes': None}
    except OSError:
        return {'status': 'unavailable', 'bytes': None}
    return measure([path])


def old_releases(root):
    releases = root / 'releases'
    try:
        current = (root / 'current').resolve()
        paths = [p for p in releases.iterdir()
                 if not p.is_symlink() and p.is_dir() and p.resolve() != current]
    except FileNotFoundError:
        return {'status': 'missing', 'bytes': None}
    except OSError:
        return {'status': 'unavailable', 'bytes': None}
    return measure(paths)


def growth(current, previous):
    """Only complete measurements of the same path can establish growth."""
    for key, row in current.items():
        old = previous.get('sizes', {}).get(key, {})
        if not isinstance(old, dict):
            old = {}
        comparable = (row['status'] == old.get('status') == 'ok'
                      and row['path'] == old.get('path')
                      and type(row['bytes']) is int and type(old.get('bytes')) is int)
        row['growth_bytes'] = row['bytes'] - old['bytes'] if comparable else None


def previous_report(output):
    # Damaged reports are kept as evidence; an older complete one can still
    # serve as a baseline. Never rewrite an existing observation.
    for path in sorted(output.glob('report-*.json'), reverse=True):
        try:
            if path.is_symlink() or path.stat().st_size > 128 * 1024:
                continue
            value = json.loads(path.read_text())
            if isinstance(value, dict) and isinstance(value.get('sizes'), dict):
                return value
        except (OSError, ValueError):
            continue
    return {}


def observe(repo, output):
    if os.geteuid() == 0:
        raise SystemExit('run as your ordinary user, without sudo')
    started = time.monotonic()
    now = datetime.now(timezone.utc)
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    previous = previous_report(output)
    try:
        result = subprocess.run(['go', 'env', 'GOCACHE'], capture_output=True,
                                text=True, timeout=10)
        cache = Path(result.stdout.strip()) if result.returncode == 0 and result.stdout.strip() else None
    except (OSError, subprocess.TimeoutExpired):
        cache = None
    paths = {
        'go_cache': cache,
        'cargo_target': repo / 'tmp/cargo-target',
        'abrt': Path('/var/spool/abrt'),
        'logs': Path('/var/log'),
        'bus_logs': Path('/var/log/agent-bus'),
        'daemon_backups': Path('/var/lib/agent-bus/daemon.backup'),
        'record_backups': Path('/var/lib/agent-bus/record-backups'),
        'system_tmp': Path('/tmp'),
        'checkout_tmp': repo / 'tmp',
    }
    sizes = {}
    for key, path in paths.items():
        row = directory(path) if path is not None else {'status': 'unavailable', 'bytes': None}
        sizes[key] = {'path': str(path) if path is not None else '(go env GOCACHE unavailable)', **row}
    sizes['old_releases'] = {'path': '/usr/local/lib/agent-bus/releases (excluding current)',
                             **old_releases(Path('/usr/local/lib/agent-bus'))}
    growth(sizes, previous)
    try:
        df = subprocess.run(['df', '-B1', '--', '/', str(repo), '/tmp'], capture_output=True,
                            text=True, timeout=10)
        disk = df.stdout.strip()
        disk_status = 'ok' if df.returncode == 0 else 'partial'
    except (OSError, subprocess.TimeoutExpired):
        disk, disk_status = '', 'unavailable'
    report = {'observed_at': now.isoformat(), 'user_id': os.geteuid(),
              'previous_at': previous.get('observed_at'), 'sizes': sizes,
              'df_status': disk_status, 'df': disk,
              'elapsed_seconds': round(time.monotonic() - started, 3)}
    name = 'report-' + now.strftime('%Y%m%dT%H%M%S.%fZ') + '.json'
    path = output / name
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump(report, stream, indent=2)
        stream.write('\n')
    print(path)
    print('Allocated bytes; overlapping categories must not be added. Partial values are lower bounds.')
    print(f"Previous: {report['previous_at'] or 'none (first baseline; growth unavailable)'}")
    for key, row in sizes.items():
        print(f"{key}: status={row['status']} bytes={row['bytes']} growth={row['growth_bytes']}")
    print(f'df ({disk_status}):\n{disk}')
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, default=Path(__file__).resolve().parents[2],
                        help='checkout to observe (use the main checkout when running from a worktree)')
    parser.add_argument('--output', type=Path, help='report directory (default: REPO/tmp/b15)')
    args = parser.parse_args()
    repo = args.repo.resolve()
    observe(repo, args.output or repo / 'tmp/b15')


if __name__ == '__main__':
    main()
