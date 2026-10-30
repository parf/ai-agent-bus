"""Retire owned legacy git timers; never create timers."""
import importlib.util
from pathlib import Path
import re

spec = importlib.util.spec_from_file_location('git_starter', Path(__file__).resolve().parent.parent / 'install.py')
starter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(starter)


def retire(unit_file, name, ctl):
    marker = '# agent-bus standalone: ' + name + '\n'
    if unit_file.is_symlink() or not unit_file.read_text().startswith(marker):
        raise ValueError(unit_file.name + ': belongs to another installation')
    prefix = unit_file.stem + '-autopull-'
    old = [p for p in unit_file.parent.glob(prefix + '*') if p.suffix in ('.service', '.timer')]
    for path in old:
        if path.is_symlink() or (path.exists() and not path.read_text().startswith(marker)):
            raise ValueError(path.name + ': belongs to another installation')
    # Stop timers before services, so removal cannot launch another attempt.
    for path in sorted(old, key=lambda p: p.suffix == '.service'):
        starter.command([ctl, 'disable', '--now', path.name])
        starter.command([ctl, 'reset-failed', path.name], optional=True)
        path.unlink()
    if old: starter.command([ctl, 'daemon-reload'])
