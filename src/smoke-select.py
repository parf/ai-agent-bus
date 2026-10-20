#!/usr/bin/env python3
"""Resolve existing subsystem prefixes and a static check map; start no tests."""
import argparse
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parent.parent
ALL = 'ABCDEFGHIJKLMNOP'


def prefixes(root):
    rows = []
    for line in (root/'Plans/subsystems.md').read_text().splitlines():
        match = re.match(r'\| ([A-P]) \| ([^|]+)\|', line)
        if match:
            rows.extend((prefix, match[1]) for prefix in re.findall(r'`([^`]+)`', match[2]))
    return rows


def ignored(path):
    return path == 'src/internal/version/VERSION' or re.fullmatch(r'CHANGELOG\..*\.md', path) is not None


def changed(root):
    def git(*args):
        return subprocess.check_output(['git', '-C', str(root), *args], stderr=subprocess.DEVNULL).decode().splitlines()
    try:
        git('rev-parse', '--verify', 'main')
        return sorted(set(git('diff', '--name-only', 'main...HEAD') + git('diff', '--name-only', 'HEAD') + git('ls-files', '--others', '--exclude-standard')))
    except subprocess.CalledProcessError:
        return None


def plan(root, mode, targets='', paths=None):
    config = json.loads((root/'src/smoke-map.json').read_text())
    paths = changed(root) if paths is None else paths
    paths = None if paths is None else [p for p in paths if not ignored(p)]
    rows = prefixes(root)
    direct, why, fallback = {}, {}, ''
    if targets:
        letters = ALL if targets == 'all' else targets.replace(',', '')
        if targets != 'all' and re.fullmatch(r'[A-P](?:,[A-P])*', targets) is None:
            raise ValueError('subsystems must be comma-separated letters A–P or all')
        direct = {c: [] for c in letters}
        why = {c: 'explicit override' for c in letters}
    elif paths is None:
        fallback = 'missing main diff base'
    elif not paths:
        fallback = 'nothing changed: all fast'
        direct = {c: [] for c in ALL}
        why = {c: fallback for c in ALL}
    else:
        for path in paths:
            if path.startswith('src/smoke') or path == 'src/agents/test-pool.py' or path.endswith(('/go.mod', '/go.sum', '/bun.lock', '/package-lock.json', '/Cargo.lock')) or path in ('go.mod', 'go.sum'):
                fallback = 'shared test/build dependency: '+path
                break
            letter = next((letter for prefix, letter in rows if path.startswith(prefix)), None)
            if letter is None:
                fallback = 'unmapped path: '+path
                break
            direct.setdefault(letter, []).append(path)
            why[letter] = 'changed: '+', '.join(direct[letter])
    promoted = set(direct)
    broad = set(direct) if targets else set()
    if fallback and not fallback.startswith('nothing changed'):
        promoted, broad = set(ALL), set(ALL)
        why = {c: fallback for c in ALL}
    # A shared helper outside a platform prefix affects every check of its letter.
    for letter, files in direct.items():
        scoped = [prefix for row in config['checks'].values() if letter in row['tags'] for prefix in row.get('paths', [])]
        if any(not any(path.startswith(prefix) for prefix in scoped) for path in files):
            broad.add(letter)
    todo = list(promoted)
    while todo:
        source = todo.pop()
        for target in config['reverse'].get(source, ''):
            if target not in promoted:
                promoted.add(target)
                broad.add(target)
                why[target] = 'reverse dependency of '+source
                todo.append(target)
    modes = {c: mode if c in promoted and not fallback.startswith('nothing changed') else 'fast' for c in ALL}
    levels = {}
    for check, row in config['checks'].items():
        active = []
        for tag in row['tags']:
            if tag not in promoted:
                continue
            if 'paths' not in row or tag in broad or targets or any(p.startswith(prefix) for p in direct.get(tag, []) for prefix in row['paths']):
                active.append(modes[tag])
        levels[check] = max(active, key=lambda x: ('fast','normal','slow').index(x)) if active else ('skip' if mode == 'fast' else 'fast')
    # Prose-only changes still get a non-empty conservative fast run.
    if all(value == 'skip' for value in levels.values()):
        fallback = 'no executable checks for changed paths: all fast'
        levels = {name: 'fast' for name in levels}
    # A fast set is an explicit essential-operation allowlist, never the
    # entire normal suite with its race detector removed.
    for check in levels:
        if levels[check] == 'fast' and check not in config['fast']:
            levels[check] = 'skip'
    # Stateful core sections share fixture prerequisites; retain those at fast.
    if any(level in ('normal','slow') for check,level in levels.items() if check.startswith('core_')):
        for check in levels:
            if check.startswith('core_') and levels[check] == 'skip': levels[check] = 'fast'
    return {'paths': paths, 'fallback': fallback, 'modes': modes, 'reasons': why, 'checks': levels}


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--mode', choices=['fast','normal','slow'], required=True)
    parser.add_argument('--targets', default='')
    parser.add_argument('--out', required=True)
    args = parser.parse_args()
    try:
        result = plan(ROOT,args.mode,args.targets)
    except ValueError as error:
        parser.error(str(error))
    Path(args.out).write_text(''.join(f'{name}\t{level}\n' for name,level in result['checks'].items()))
    print('SELECTION '+('LANDING RUN' if args.mode!='fast' and not result['fallback'].startswith('nothing changed') else 'NOT A LANDING GATE'))
    if result['fallback']:
        print('  fallback: '+result['fallback'])
    for letter,level in result['modes'].items():
        print(f"  {letter}: {level if args.mode != 'fast' or letter in result['reasons'] else 'not selected'} — {result['reasons'].get(letter,'unchanged')}")
    for name, level in result['checks'].items():
        if level != 'skip': print(f'  check {name}: {level}')
