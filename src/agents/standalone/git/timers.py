"""Install-time systemd timers for fixed named repositories."""
import importlib.util
from pathlib import Path
import re

spec = importlib.util.spec_from_file_location('git_starter', Path(__file__).resolve().parent.parent / 'install.py')
starter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(starter)


def intervals(entries, repos):
    result = {}
    for entry in entries:
        name, sep, value = entry.partition('=')
        if not sep or name not in repos or name in result:
            raise ValueError('--autopull needs a unique declared repo=interval')
        if repos[name].get('access') not in ('deploy', 'commit'):
            raise ValueError(name + ': autopull requires access deploy or commit')
        if not re.fullmatch(r'(?:[0-9]+(?:\.[0-9]+)?(?:ms|s|m|h|d))+', value) or not any(c in '123456789' for c in value):
            raise ValueError(name + ': autopull interval must be positive, like 5m')
        result[name] = value
    return result


def sync(unit_file, desired, repos, name, ctl):
    marker = '# agent-bus standalone: ' + name + '\n'
    if unit_file.is_symlink() or not unit_file.read_text().startswith(marker):
        raise ValueError(unit_file.name + ': belongs to another installation')
    fields = dict(line.split('=', 1) for line in unit_file.read_text().splitlines() if '=' in line and not line.startswith('#'))
    key = unit_file.stem
    prefix = key + '-autopull-'
    wanted = {}
    for repo, interval in desired.items():
        stem = prefix + repo
        argv = [str(Path(__file__).resolve().with_name('autopull.py')), '--repo', repo, '--name', name]
        service = (marker + '[Unit]\nDescription=Automatic fast-forward git pull\nAfter=network.target\n\n[Service]\nType=oneshot\nUser=' + fields['User']
            + '\nEnvironmentFile=' + fields['EnvironmentFile'] + '\nEnvironment=PYTHONDONTWRITEBYTECODE=1\nStateDirectory=' + key + '\nEnvironment=' + fields['Environment']
            + '\nExecStart=' + ' '.join(starter.quote(arg) for arg in argv)
            + '\nUMask=0077\nTimeoutStartSec=4min\nTimeoutStopSec=45\n')
        timer = (marker + '[Unit]\nDescription=Periodic fast-forward git pull\n\n[Timer]\nOnActiveSec=' + interval
            + '\nOnUnitInactiveSec=' + interval + '\nAccuracySec=100ms\nUnit=' + stem + '.service\n\n[Install]\nWantedBy=timers.target\n')
        wanted[stem + '.service'] = service
        wanted[stem + '.timer'] = timer
    old = [p for p in unit_file.parent.glob(prefix + '*') if p.suffix in ('.service', '.timer')]
    for path in [*old, *(unit_file.parent / n for n in wanted)]:
        if path.is_symlink() or (path.exists() and not path.read_text().startswith(marker)):
            raise ValueError(path.name + ': belongs to another installation')
    # Stop timers before services, so removal cannot launch another attempt.
    for path in sorted(old, key=lambda p: p.suffix == '.service'):
        changed = path.name not in wanted or path.read_text() != wanted[path.name]
        if changed:
            starter.command([ctl, 'disable', '--now', path.name])
            starter.command([ctl, 'reset-failed', path.name], optional=True)
            path.unlink()
    for filename, content in wanted.items():
        path = unit_file.parent / filename
        if not path.exists(): starter.atomic(path, content, 0o644)
    starter.command([ctl, 'daemon-reload'])
    for repo in desired:
        starter.command([ctl, 'enable', '--now', prefix + repo + '.timer'])
