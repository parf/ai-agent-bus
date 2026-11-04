"""Bounded UID sets, flags and search arguments for ordinary IMAP admission."""
import datetime
import json
import re
import shlex
from cache import uid
from mail import MailError
import wire

METHODS = {'mark', 'flag', 'headers', 'parts', 'part', 'list', 'search', 'copy', 'move', 'trash', 'delete', 'compact_trash'}
CRITERIA = {'from', 'to', 'subject', 'header', 'since', 'before', 'flag', 'no_flag', 'larger', 'smaller'}
SYSTEM = {name: '\\' + name.title() for name in ('flagged', 'answered', 'draft', 'seen', 'deleted')}


def big_name(value):
    if not isinstance(value, str) or not value or len(value.encode('utf-8')) > 256:
        raise MailError('bigfile name must be 1 to 256 UTF-8 bytes')
    return value


def uid_set(value):
    if type(value) is int:
        return [uid(value)]
    if isinstance(value, list):
        if not 1 <= len(value) <= 1000:
            raise MailError('at most 1000 UIDs')
        result = [uid(v) for v in value]
        if len(set(result)) != len(result): raise MailError('duplicate UIDs')
        return result
    if not isinstance(value, str) or not re.fullmatch(r'[0-9]+(?::[0-9]+)?(?:,[0-9]+(?::[0-9]+)?)*', value):
        raise MailError('uids must be 123, 123,130 or 100:200; at most 1000 UIDs')
    result = []
    for item in value.split(','):
        ends = item.split(':')
        first, last = uid(ends[0]), uid(ends[-1])
        if last < first or last - first + 1 + len(result) > 1000:
            raise MailError('each UID range must run low:high; at most 1000 UIDs')
        result.extend(range(first, last + 1))
    if len(set(result)) != len(result):
        raise MailError('duplicate UIDs')
    return result


def flag(value):
    if not isinstance(value, str):
        raise MailError('flag must be text')
    name = value.lstrip('\\').lower()
    if name == 'deleted':
        raise MailError('\\Deleted is refused')
    if name in SYSTEM:
        return SYSTEM[name]
    if value.startswith('\\') or not re.fullmatch(r'[A-Za-z0-9_$+.-]{1,128}', value):
        raise MailError('flag must be flagged, answered, draft, seen or a custom keyword')
    return value


def pairs(rows):
    result = {}
    for key, value in rows:
        if key in result:
            raise MailError('duplicate request key')
        result[key] = value
    return result


def subscription(body):
    """Keep regex bytes verbatim; split --folder only outside a closed regex."""
    target, _, pattern = body[len('sub add '):].partition(' ')
    value = {'method': 'sub.add', 'target': target, 'filter': pattern}
    from subscription_filters import split, FilterError
    for match in re.finditer(r'(?:^| )--folder(?==| |$)', pattern):
        prefix = pattern[:match.start()].rstrip(' ')
        if prefix.startswith('/'):
            try: split(prefix)
            except FilterError: continue  # A --folder inside the pattern.
        words = shlex.split(pattern[match.start():].strip())
        if len(words) == 1 and words[0].startswith('--folder='):
            name = words[0].split('=', 1)[1]
        elif len(words) == 2 and words[0] == '--folder': name = words[1]
        else: raise MailError('use one --folder <name> after the optional filter')
        wire.mailbox(name)
        value.update(filter=prefix, folder=name)
        break
    return value


def parse_call(body):
    if body.startswith('{'):
        value = json.loads(body, object_pairs_hook=pairs)
        if not isinstance(value, dict) or not isinstance(value.get('method'), str) or value['method'] not in METHODS:
            return None
    else:
        method = body.partition(' ')[0]
        if method not in METHODS:
            return None
        if any(ord(c) < 32 or ord(c) == 127 for c in body):
            raise MailError('call must not contain controls')
        if method == 'search' and not re.search(r'(?:^| )--', body):
            words = ['search', body.partition(' ')[2]]
        else:
            words = shlex.split(body)
        value, positional = {'method': words.pop(0)}, []
        while words:
            word = words.pop(0)
            if word.startswith('--'):
                key, equal, arg = word[2:].partition('=')
                key = key.replace('-', '_')
                if key == 'unread':
                    if equal:
                        raise MailError('--unread takes no value')
                    arg = True
                else:
                    if not equal:
                        if not words or words[0].startswith('--'):
                            raise MailError('--' + key + ' requires a value')
                        arg = words.pop(0)
                if key in value:
                    raise MailError('duplicate option ' + key)
                value[key] = arg
            else:
                positional.append(word)
        if method in {'mark', 'flag', 'headers', 'parts', 'part', 'copy', 'move', 'trash', 'delete'}:
            count = {'mark': 2, 'headers': 1, 'parts': 1, 'part': 2, 'copy': 2, 'move': 2, 'trash': 1, 'delete': 1}.get(method)
            if (count is not None and len(positional) != count) or (method == 'flag' and len(positional) < 3):
                raise MailError('invalid arguments for ' + method)
            value['uids' if method in {'mark', 'flag', 'headers', 'copy', 'move', 'trash', 'delete'} else 'uid'] = positional[0]
            if method == 'mark': value['state'] = positional[1]
            if method == 'flag': value.update(action=positional[1], flags=positional[2:])
            if method == 'part': value['part_id'] = positional[1]
            if method in {'copy', 'move'}: value['dest'] = positional[1]
        elif method == 'compact_trash':
            if positional: raise MailError('use compact_trash [--older <days>]')
        elif method == 'list':
            if len(positional) > 1 or ('limit' in value and positional):
                raise MailError('use list [limit] [--flag <f>]')
            if positional: value['limit'] = positional[0]
        else:
            value['text'] = ' '.join(positional)
    method = value['method']
    allowed = {'mark': {'uids', 'state'}, 'flag': {'uids', 'action', 'flags'},
               'headers': {'uids'}, 'parts': {'uid'}, 'part': {'uid', 'part_id', 'bigfile'},
               'list': {'limit', 'unread', 'flag'}, 'search': CRITERIA | {'limit', 'text'},
               'copy': {'uids', 'dest'}, 'move': {'uids', 'dest'}, 'trash': {'uids'},
               'delete': {'uids'}, 'compact_trash': {'older'}}[method]
    if set(value) - allowed - {'method', 'folder'}:
        raise MailError('unknown argument for ' + method)
    if 'folder' in value: wire.mailbox(value['folder'])
    if method in {'mark', 'flag', 'headers', 'copy', 'move', 'trash', 'delete'}:
        value['uids'] = uid_set(value.get('uids'))
    if method in {'copy', 'move'}: wire.mailbox(value.get('dest'))
    if method == 'compact_trash' and 'older' in value:
        days = value['older']
        if isinstance(days, str) and re.fullmatch(r'[0-9]{1,6}', days): days = int(days)
        if type(days) is not int or not 0 <= days <= 365000:
            raise MailError('older must be days in 0..365000')
        value['older'] = days
    if method in {'parts', 'part'}: value['uid'] = uid(value.get('uid'))
    if method == 'part' and (not isinstance(value.get('part_id'), str) or not re.fullmatch(r'[1-9][0-9]{0,3}(?:\.[1-9][0-9]{0,3}){0,30}', value['part_id'])):
        raise MailError('part-id must be a MIME part id from parts')
    if method == 'part' and 'bigfile' in value:
        big_name(value['bigfile'])
    if method == 'mark' and value.get('state') not in ('read', 'unread'):
        raise MailError('mark state must be read or unread')
    if method == 'flag':
        if value.get('action') not in ('add', 'remove') or not isinstance(value.get('flags'), list) or not 1 <= len(value['flags']) <= 100:
            raise MailError('use flag <uids> add|remove <flag...>')
        value['flags'] = list(dict.fromkeys(flag(f) for f in value['flags']))
    if method in {'list', 'search'}:
        limit = value.get('limit', 20 if method == 'list' else 100)
        if isinstance(limit, str) and re.fullmatch('[1-9][0-9]{0,2}', limit): limit = int(limit)
        if type(limit) is not int or not 1 <= limit <= 100: raise MailError('limit must be in 1..100')
        value['limit'] = limit
    if method == 'list':
        if type(value.get('unread', False)) is not bool: raise MailError('unread must be a boolean')
        if 'flag' in value: value['flag'] = flag(value['flag'])
    if method == 'search':
        text = value.setdefault('text', '')
        if not isinstance(text, str) or len(text.encode()) > 4096 or any(ord(c) < 32 or ord(c) == 127 for c in text):
            raise MailError('search requires at most 4096 UTF-8 bytes of local text without controls')
        if not text and not (set(value) & CRITERIA): raise MailError('search requires text or server criteria')
        for key in CRITERIA & set(value):
            arg = value[key]
            if key in {'larger', 'smaller'}:
                if isinstance(arg, str) and re.fullmatch(r'[0-9]{1,10}', arg): arg = int(arg)
                if type(arg) is not int or not 0 <= arg <= 4294967295: raise MailError(key + ' must be 0..4294967295')
            else:
                if not isinstance(arg, str) or not arg or len(arg.encode()) > 1024 or any(ord(c) < 32 or ord(c) == 127 for c in arg):
                    raise MailError(key + ' must be nonempty text without controls')
                if key in {'flag', 'no_flag'}: arg = flag(arg)
                if key == 'header' and not re.fullmatch(r'[A-Za-z0-9-]+:.+', arg): raise MailError('header must be Name:value')
                if key in {'since', 'before'}:
                    try:
                        if re.fullmatch(r'[0-9]{4}-[0-9]{2}-[0-9]{2}', arg):
                            date = datetime.date.fromisoformat(arg)
                            arg = str(date.day).zfill(2) + '-' + ('Jan Feb Mar Apr May Jun Jul Aug Sep Oct Nov Dec'.split()[date.month - 1]) + '-' + str(date.year)
                        else:
                            datetime.datetime.strptime(arg, '%d-%b-%Y')
                    except ValueError: raise MailError(key + ' must be YYYY-MM-DD or DD-Mon-YYYY') from None
            value[key] = arg
    return value


def quote(value):
    return '"' + value.replace('\\', '\\\\').replace('"', '\\"') + '"'


def search_args(call):
    result = []
    for key in ('from', 'to', 'subject', 'header', 'since', 'before', 'flag', 'no_flag', 'larger', 'smaller'):
        if key not in call: continue
        value = call[key]
        if key in {'flag', 'no_flag'}:
            system = value.startswith('\\')
            atom = value[1:].upper() if system else quote(value)
            result.extend((['UN' + atom] if key == 'no_flag' else [atom]) if system else ['UNKEYWORD' if key == 'no_flag' else 'KEYWORD', atom])
        elif key == 'header': result.extend(['HEADER', *[quote(v) for v in value.split(':', 1)]])
        else: result.extend([key.upper(), str(value) if isinstance(value, int) else quote(value)])
    return result
