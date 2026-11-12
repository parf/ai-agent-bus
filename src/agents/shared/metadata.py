"""Bare-program fallback for package help; authored metadata, never live values."""
import base64
import hashlib
import json
import os
import re
import shlex
import stat
import struct
from pathlib import Path

ASKS = ['help', 'help-config', 'help-secret', 'help-json']


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
    # An agent that declares no version shows none: presence for a bundled agent is the pack check's.
    picture = {}
    if manifest.get('image'):
        picture = {'image': manifest['image'], 'image_sha256': image_load(directory, manifest['image'])[1]}
    return {**{key: manifest[key] for key in ('description','license','author','homepage','docs') if key in manifest},
            **({'version': manifest['version']} if 'version' in manifest else {}), **({'released': manifest['released']} if manifest.get('released') else {}),
            **picture, 'methods': methods, 'asks': ASKS + (['image'] if picture else [])}


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
    footer = (['v' + value['version']] if 'version' in value else []) + (['released ' + value['released']] if value.get('released') else [])
    if value.get('license'): footer.append(value['license'].replace('LicenseRef-PolyForm-Strict-1.0.0', 'PolyForm Strict'))
    if value.get('homepage'): footer.append(value['homepage'].removeprefix('https://').removeprefix('http://'))
    if value.get('author'): footer.append('© ' + value['author'])
    target = shlex.quote(os.environ['AGENT_BUS_NAME']) if os.environ.get('AGENT_BUS_NAME') else '<name>'
    lines += (['', ' · '.join(footer)] if footer else ['']) + (['Docs: ' + value['docs']] if value.get('docs') else []) + ['More: agent-bus help ' + target + ' config | secret | json']
    return '\n'.join(lines)


def dispatch(directory, ask, *, handlers=None, **options):
    """Ask first. None alone means ordinary input; unsupported is a normal reply."""
    if not ask:
        return None
    if handlers and ask in handlers:
        return handlers[ask]()
    if ask == 'image':
        name = json.loads((Path(directory) / 'agent.json').read_text()).get('image')
        if name:
            return image_answer(directory, name)
    if ask in ASKS:
        return answer(directory, ask, **options)
    if ask == 'mcp':
        return '{"tools":[]}'
    return 'unsupported: ' + ask


def normalize_tags(tags):
    # Keep identical to internal/protocol/tags.go NormalizeTags: discovery
    # compares authored labels with the daemon's stored canonical labels.
    normalized = set()
    for raw in tags:
        for word in raw.lower().split(','):
            if word == 'no-tags':
                raise ValueError('invalid tag ' + repr(word))
            if word and not re.fullmatch(r'[a-z0-9_-]+', word):
                raise ValueError('invalid tag ' + repr(word))
            if word:
                normalized.add(word)
    return sorted(normalized)


# An agent's image: the same rules as internal/agentmeta/image.go (Plans/R1.1/agent-images.md#rules).
IMAGE_MAX_BYTES = 256 << 10
IMAGE_MAX_PIXELS = 512


class ImageError(ValueError):
    pass


def image_check_name(name):
    if not isinstance(name, str) or not name.strip() or name != name.strip():
        raise ImageError("agent.json: image must be a file name inside the agent's folder")
    if name.startswith('/') or '\\' in name or os.path.isabs(name):
        raise ImageError(f'agent.json: image "{name}" must not be an absolute path')
    if any(part in ('', '.', '..') for part in name.split('/')):
        raise ImageError(f'agent.json: image "{name}" must not contain empty, . or .. parts')


def image_load(directory, name):
    """Validated bytes of the image name inside directory; never through a symlink."""
    image_check_name(name)
    at = Path(directory)
    parts = name.split('/')
    for i, part in enumerate(parts):
        at = at / part
        try:
            st = os.lstat(at)
        except OSError as e:
            raise ImageError(f'agent.json: image "{name}": {e.strerror.lower() if e.strerror else e}') from None
        if stat.S_ISLNK(st.st_mode):
            raise ImageError(f'agent.json: image "{name}": a symlink is refused')
        if i < len(parts) - 1 and not stat.S_ISDIR(st.st_mode):
            raise ImageError(f'agent.json: image "{name}": {part} is not a directory')
        if i == len(parts) - 1 and not stat.S_ISREG(st.st_mode):
            raise ImageError(f'agent.json: image "{name}" is not a regular file')
    fd = os.open(at, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, 'rb') as f:
        if not stat.S_ISREG(os.fstat(f.fileno()).st_mode):
            raise ImageError(f'agent.json: image "{name}" is not a regular file')
        data = f.read(IMAGE_MAX_BYTES + 1)
    try:
        return image_check(data)
    except ImageError as e:
        raise ImageError(f'agent.json: image "{name}": {e}') from None


def image_check(data):
    """(bytes, sha256 hex, type) when data keeps the caps; the type is from the first bytes."""
    if len(data) > IMAGE_MAX_BYTES:
        raise ImageError(f'over {IMAGE_MAX_BYTES >> 10} KiB')
    kind, w, h = _image_size(data)
    if w <= 0 or h <= 0:
        raise ImageError('has no size in its header')
    if w > IMAGE_MAX_PIXELS or h > IMAGE_MAX_PIXELS:
        raise ImageError(f'is {w}×{h}, over {IMAGE_MAX_PIXELS}×{IMAGE_MAX_PIXELS}')
    if w != h:
        raise ImageError(f'is {w}×{h}, not square')
    return data, hashlib.sha256(data).hexdigest(), kind


def image_answer(directory, name):
    """The image ask's answer: the file in base64."""
    return base64.b64encode(image_load(directory, name)[0]).decode()


def _image_size(b):
    if b.startswith(b'\x89PNG\r\n\x1a\n'):
        if len(b) < 24 or b[12:16] != b'IHDR':
            raise ImageError('is a PNG without IHDR')
        return ('image/png',) + struct.unpack('>II', b[16:24])
    if b.startswith(b'\xff\xd8'):
        return ('image/jpeg',) + _image_jpeg(b)
    if len(b) >= 12 and b[:4] == b'RIFF' and b[8:12] == b'WEBP':
        return ('image/webp',) + _image_webp(b)
    if len(b) >= 12 and b[4:8] == b'ftyp' and _image_is_avif(b):
        return ('image/avif',) + _image_avif(b)
    raise ImageError('is not AVIF, WebP, PNG or JPEG')


def _image_jpeg(b):
    i = 2
    while i + 4 <= len(b):
        if b[i] != 0xff:
            raise ImageError('is a JPEG with a broken marker')
        marker = b[i + 1]
        if marker == 0xff:
            i += 1
            continue
        if marker in (0xd8, 0x01) or 0xd0 <= marker <= 0xd7:
            i += 2
            continue
        size = struct.unpack('>H', b[i + 2:i + 4])[0]
        if size < 2 or i + 2 + size > len(b):
            break
        if 0xc0 <= marker <= 0xcf and marker not in (0xc4, 0xc8, 0xcc):
            if size < 7:
                break
            h, w = struct.unpack('>HH', b[i + 5:i + 9])
            return w, h
        i += 2 + size
    raise ImageError('is a JPEG without a frame size')


def _image_webp(b):
    if len(b) < 30:
        raise ImageError('is a short WebP')
    chunk = b[12:16]
    if chunk == b'VP8X':
        return int.from_bytes(b[24:27], 'little') + 1, int.from_bytes(b[27:30], 'little') + 1
    if chunk == b'VP8L':
        if b[20] != 0x2f:
            raise ImageError('is a WebP with a bad lossless signature')
        bits = struct.unpack('<I', b[21:25])[0]
        return (bits & 0x3fff) + 1, (bits >> 14 & 0x3fff) + 1
    if chunk == b'VP8 ':
        if b[23:26] != b'\x9d\x01\x2a':
            raise ImageError('is a WebP with a bad lossy start code')
        return struct.unpack('<H', b[26:28])[0] & 0x3fff, struct.unpack('<H', b[28:30])[0] & 0x3fff
    raise ImageError('is a WebP without a known image chunk')


def _image_boxes(b):
    out = []
    while b:
        if len(b) < 8:
            raise ImageError('is an AVIF with a truncated box')
        size, head = struct.unpack('>I', b[:4])[0], 8
        if size == 0:
            size = len(b)
        elif size == 1:
            if len(b) < 16:
                raise ImageError('is an AVIF with a truncated box')
            size, head = struct.unpack('>Q', b[8:16])[0], 16
        if size < head or size > len(b):
            raise ImageError('is an AVIF with a box past its end')
        out.append((b[4:8], b[head:size]))
        b = b[size:]
    return out


def _image_is_avif(b):
    size = struct.unpack('>I', b[:4])[0]
    if size < 16 or size > len(b):
        return False
    return any(b[i:i + 4] in (b'avif', b'avis') for i in range(8, size - 3, 4) if i != 12)


def _image_avif(b):
    find = lambda boxes, kind: next((body for k, body in boxes if k == kind), b'')
    meta = find(_image_boxes(b), b'meta')
    if len(meta) < 4:
        raise ImageError('is an AVIF without meta')
    props = _image_boxes(find(_image_boxes(find(_image_boxes(meta[4:]), b'iprp')), b'ipco'))
    first = None
    for kind, body in props:
        if kind != b'ispe':
            continue
        if len(body) < 12:
            raise ImageError('is an AVIF with a short ispe')
        w, h = struct.unpack('>II', body[4:12])
        if w > IMAGE_MAX_PIXELS or h > IMAGE_MAX_PIXELS:
            return w, h
        first = first or (w, h)
    if not first:
        raise ImageError('is an AVIF without ispe')
    return first
