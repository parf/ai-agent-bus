"""Bare-program fallback for package help; authored metadata, never live values."""
import json
import os
import shlex
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
            raw = match[2]
            quote, escaped, at = '', False, len(raw)
            for i, char in enumerate(raw):
                if escaped: escaped = False
                elif char == '\\' and quote: escaped = True
                elif quote and char == quote: quote = ''
                elif not quote and char in "\"'`": quote = char
                elif not quote and char == '#': at = i; break
            value, comment = raw[:at].strip(), raw[at+1:].strip() if at < len(raw) else ''
            if len(value) >= 2 and value[0] in "\"'`" and value[-1] == value[0]: value = value[1:-1]
            result.append({'name': match[1], 'help': '\n'.join(comments),
                           'required': comment.split()[:1] == ['required'], 'has_default': bool(value)})
        comments = []
    return result


def typed(value):
    return {'type': 'string', **value}


def granted(method, roles):
    required = method.get('role')
    return not required or bool(roles & {'owner', 'maintainer'}) or any(r in roles for r in (required if isinstance(required, list) else [required]))


def data(directory, ask='help', *, allowed=None, roles=None):
    directory = Path(directory)
    manifest = json.loads((directory / 'agent.json').read_text())
    if ask == 'help-json':
        delivered = set(roles) if roles is not None else {key[8:].lower() for key, value in os.environ.items() if key.startswith('AB_ROLE_') and value == '1'}
        hidden = [m for name, m in manifest.get('methods', {}).items()
                  if (allowed is None or name in allowed or name == 'help') and not granted(m, delivered)]
        needed = sorted({r for m in hidden for r in (m['role'] if isinstance(m['role'], list) else [m['role']])})
        return {**data(directory, 'help', allowed=allowed, roles=roles),
                **data(directory, 'help-config'), **data(directory, 'help-secret'),
                'body_formats': manifest.get('body_formats', ['text']),
                'hidden_methods': {'count': len(hidden), 'roles': needed}}
    if ask == 'help-secret':
        source = directory / 'env.dist'
        return {'secrets': secrets(source.read_text() if source.exists() else '')}
    if ask == 'help-config':
        return {'config': {name: typed(field) for name, field in manifest.get('config', {}).items()}}
    if ask != 'help': raise ValueError('unknown metadata ask ' + ask)
    roles = set(roles) if roles is not None else {key[8:].lower() for key, value in os.environ.items() if key.startswith('AB_ROLE_') and value == '1'}
    methods = {name: {**typed(m), 'args': [typed({**(manifest.get('arguments', {}).get(a['name'], {}) if not a.get('help') else {}), **a}) for a in m.get('args', [])]}
               for name, m in manifest.get('methods', {}).items()
               if (allowed is None or name in allowed or name == 'help') and
               granted(m, roles)}
    return {**{key: manifest[key] for key in ('description','license','author','homepage') if key in manifest},
            'version': manifest.get('version', VERSION), 'methods': methods, 'asks': ASKS}


def answer(directory, ask='help', *, allowed=None, roles=None, as_json=False):
    value = data(directory, ask, allowed=allowed, roles=roles)
    dumps = lambda v: json.dumps(v, ensure_ascii=False, separators=(',', ':'))
    kind = lambda v: '|'.join(v) if isinstance(v, list) else v
    if as_json or ask == 'help-json': return dumps(value)
    if 'secrets' in value:
        return '\n'.join(s['name'] + (' (required' if s['required'] else ' (optional') + ('; has default' if s['has_default'] else '') + '): ' + s['help'] for s in value['secrets']) or 'No environment keys declared.'
    if 'config' in value:
        return '\n'.join(name + ' (' + kind(c['type']) + '): ' + c['help'] +
                         ''.join('; ' + k + ': ' + dumps(c[k]) for k in ('example','default') if k in c)
                         for name, c in sorted(value['config'].items())) or 'No Configuration keys declared.'
    manifest = json.loads((Path(directory) / 'agent.json').read_text())
    lines, shared, examples = [value['description']], {}, []
    def argument(a, prefix=''):
        return prefix + a['name'] + ' (' + kind(a['type']) + '): ' + a['help'] + ('; default: ' + dumps(a['default']) if 'default' in a else '')
    for name, method in value['methods'].items():
        if name == 'help': continue
        args = [a for a in method['args'] if not a.get('json_only')]
        usage = method.get('text') or name + ''.join(' ' + ('[' if a.get('optional') else '<') + a['name'] + ('...' if a.get('rest') else '') + (']' if a.get('optional') else '>') for a in args)
        lines.append(('[destructive] ' if method.get('destructive') else '') + usage + ': ' + method['help'] + ''.join('; ' + k + ': ' + (kind(method[k]) if k == 'role' else method[k]) for k in ('returns','role') if method.get(k)))
        authored = {a['name']: a for a in manifest['methods'][name].get('args', [])}
        for a in args:
            if not authored[a['name']].get('help'): shared[a['name']] = a
            else: lines.append(argument(a, '  '))
        if method.get('example'): examples.append(method['example'])
    if shared: lines += ['', 'Arguments'] + [argument(shared[name]) for name in sorted(shared)]
    if examples: lines += ['', 'Examples'] + examples
    footer = ['v' + value['version']]
    if value.get('license'): footer.append(value['license'].replace('PolyForm-Noncommercial-1.0.0', 'PolyForm-NC'))
    if value.get('homepage'): footer.append(value['homepage'].removeprefix('https://').removeprefix('http://'))
    target = shlex.quote(os.environ['AGENT_BUS_NAME']) if os.environ.get('AGENT_BUS_NAME') else '<name>'
    lines += ['', ' · '.join(footer), 'More: agent-bus help ' + target + ' config | secret | json']
    return '\n'.join(lines)
