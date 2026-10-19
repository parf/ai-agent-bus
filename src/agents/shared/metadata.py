"""Bare-program fallback for package help; authored metadata, never live values."""
import json
import os
from pathlib import Path

ASKS = ['help', 'help-config', 'help-secret', 'help-json']
VERSION = (Path(__file__).resolve().parents[2] / 'internal/version/VERSION').read_text().strip()


def secrets(source):
    import re
    result, comments = [], []
    for line in source.splitlines():
        if line.strip().startswith('#'):
            comments.append(line.strip()[1:].strip())
            continue
        match = re.fullmatch(r'\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*', line)
        if match:
            result.append({'name': match[1], 'help': '\n'.join(comments), 'required': match[2] in ('', '""', "''")})
        comments = []
    return result


def typed(value):
    return {'type': 'string', **value}


def data(directory, ask='help', *, allowed=None, roles=None):
    directory = Path(directory)
    if ask == 'help-json':
        return {**data(directory, 'help', allowed=allowed, roles=roles),
                **data(directory, 'help-config'), **data(directory, 'help-secret')}
    manifest = json.loads((directory / 'agent.json').read_text())
    if ask == 'help-secret':
        source = directory / 'env.dist'
        return {'secrets': secrets(source.read_text() if source.exists() else '')}
    if ask == 'help-config':
        return {'config': {name: typed(field) for name, field in manifest.get('config', {}).items()}}
    if ask != 'help': raise ValueError('unknown metadata ask ' + ask)
    roles = set(roles) if roles is not None else {key[8:].lower() for key, value in os.environ.items() if key.startswith('AB_ROLE_') and value == '1'}
    methods = {name: {**typed(m), 'args': [typed(a) for a in m.get('args', [])]}
               for name, m in manifest.get('methods', {}).items()
               if (allowed is None or name in allowed or name == 'help') and
               (not m.get('role') or any(r in roles for r in (m['role'] if isinstance(m['role'], list) else [m['role']])) or roles & {'owner','maintainer'})}
    return {**{key: manifest[key] for key in ('description','license','author','homepage') if key in manifest},
            'version': manifest.get('version', VERSION), 'methods': methods, 'asks': ASKS}


def answer(directory, ask='help', *, allowed=None, roles=None, as_json=False):
    value = data(directory, ask, allowed=allowed, roles=roles)
    dumps = lambda v: json.dumps(v, ensure_ascii=False, separators=(',', ':'))
    kind = lambda v: '|'.join(v) if isinstance(v, list) else v
    if as_json or ask == 'help-json': return dumps(value)
    if 'secrets' in value:
        return '\n'.join(s['name'] + (' (required): ' if s['required'] else ' (optional): ') + s['help'] for s in value['secrets']) or 'No environment keys declared.'
    if 'config' in value:
        return '\n'.join(name + ' (' + kind(c['type']) + '): ' + c['help'] +
                         ''.join('; ' + k + ': ' + dumps(c[k]) for k in ('example','default') if k in c)
                         for name, c in sorted(value['config'].items())) or 'No Configuration keys declared.'
    lines = [value['description'], 'Version: ' + value['version']]
    lines += [key.title() + ': ' + value[key] for key in ('license','author','homepage') if value.get(key)]
    lines.append('Asks: --help, --help-config, --help-secret, --help-json')
    for name, method in sorted(value['methods'].items()):
        arguments = ''.join(' ' + ('[' if a.get('optional') else '<') + a['name'] + ('...' if a.get('rest') else '') + (']' if a.get('optional') else '>') for a in method['args'])
        lines.append(name + arguments + ': ' + method['help'] + ''.join('; ' + k + ': ' + (kind(method[k]) if k == 'role' else method[k]) for k in ('returns','role') if method.get(k)) + ('; destructive' if method.get('destructive') else ''))
        lines += ['  ' + a['name'] + ' (' + kind(a['type']) + '): ' + a['help'] for a in method['args']]
    return '\n'.join(lines)
