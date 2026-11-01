"""Folder management and one-shot APPEND; never retry a possibly applied write."""
import datetime
import imaplib
import json
import re
import shlex

from mail import MailError
import folders
import operations
import wire

METHODS = {'folder.create', 'folder.rename', 'folder.delete', 'append'}
GATED = METHODS - {'folder.create'}
REQUEST_CAP = 1024 * 1024  # The bus API's existing JSON request-body ceiling.
MAIL_CAP = 25 * 1024 * 1024


def parse(body):
    if not isinstance(body, str):
        return None
    if body.startswith('{'):
        # Do not expand large unrelated calls past their original parser limit.
        if len(body.encode()) > REQUEST_CAP:
            raise MailError('append exceeds the 1048576-byte bus request cap (including encoded JSON and envelope)')
        value = json.loads(body, object_pairs_hook=operations.pairs)
        if not isinstance(value, dict) or not isinstance(value.get('method'), str):
            return None
        value['method'] = {name.replace('.', ' '): name for name in METHODS}.get(value['method'], value['method'])
        if value['method'] not in METHODS:
            return None
    else:
        header, separator, raw = body.partition('\n')
        if not re.match(r'^(folder(?: |$)|append(?: |$))', header):
            return None
        if len(body.encode()) > REQUEST_CAP:
            raise MailError('append exceeds the 1048576-byte bus request cap (including encoded JSON and envelope)')
        words = shlex.split(header.rstrip('\r'))
        if not words:
            raise MailError('use folder create/rename/delete or append')
        method = words.pop(0)
        if method == 'folder':
            if not words or words[0] not in ('create', 'rename', 'delete'):
                raise MailError('use folder create/rename/delete')
            method += '.' + words.pop(0)
        value, positional = {'method': method}, []
        while words:
            word = words.pop(0)
            if word.startswith('--'):
                key, equal, arg = word[2:].partition('=')
                if key == 'force':
                    if equal: raise MailError('--force takes no value')
                    arg = True
                elif not equal:
                    if not words or words[0].startswith('--'): raise MailError('--' + key + ' requires a value')
                    arg = words.pop(0)
                if key in value: raise MailError('duplicate option ' + key)
                value[key] = arg
            else:
                positional.append(word)
        count = 2 if method == 'folder.rename' else 1
        if len(positional) != count:
            raise MailError('invalid arguments for ' + method)
        value['folder' if method == 'append' else 'name'] = positional[0]
        if method == 'folder.rename': value['new'] = positional[1]
        if method == 'append':
            if separator: value['raw'] = raw
            elif 'big' not in value: raise MailError('append requires raw RFC 5322 mail after the first newline')
        elif separator:
            raise MailError('folder calls must be one line')
    method = value['method']
    if method != 'append' and len(body.encode()) > 32768:
        raise MailError('folder call must be text or JSON of at most 32768 bytes')
    allowed = {'folder.create': {'name'}, 'folder.rename': {'name', 'new'},
               'folder.delete': {'name', 'force'}, 'append': {'folder', 'raw', 'flags', 'date', 'big'}}[method]
    if set(value) - allowed - {'method'}: raise MailError('unknown argument for ' + method)
    wire.mailbox(value.get('folder' if method == 'append' else 'name'))
    if method == 'folder.rename': wire.mailbox(value.get('new'))
    if method == 'folder.delete' and type(value.get('force', False)) is not bool:
        raise MailError('force must be a boolean')
    if method == 'append':
        if 'big' in value:
            if 'raw' in value: raise MailError('append takes either raw mail or --big, never both')
            ref = value['big']
            record, slash, name = ref.rpartition('/') if isinstance(ref, str) else ('', '', '')
            if not slash or not record or not name:
                raise MailError('big reference requires <record>/<name>: the key name after the last /; a name containing / can\'t be used here')
            operations.big_name(name)
            if any(ord(c) <= 32 or ord(c) == 127 for c in record):
                raise MailError('big reference record must not contain whitespace or controls')
            value['big'] = (record, name)
        else:
            raw = value.get('raw')
            if not isinstance(raw, str) or not raw or '\x00' in raw:
                raise MailError('append raw must be nonempty RFC 5322 text without NUL')
            if len(raw.encode()) > MAIL_CAP: raise MailError('append exceeds the 25 MB (26214400-byte) mail cap')
        flags = value.get('flags', [])
        if isinstance(flags, str): flags = flags.split(',')
        if not isinstance(flags, list) or len(flags) > 100: raise MailError('append flags must be at most 100 flag names')
        value['flags'] = list(dict.fromkeys(operations.flag(flag) for flag in flags))
        date = value.get('date')
        if date is not None:
            if not isinstance(date, str) or not re.fullmatch(r'[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,6})?(?:Z|[+-][0-9]{2}:[0-9]{2})', date):
                raise MailError('append date must be RFC3339 with a timezone')
            try:
                date = datetime.datetime.fromisoformat(date.replace('Z', '+00:00'))
            except ValueError:
                raise MailError('append date must be a valid RFC3339 date') from None
            value['date'] = date
    return value


def under(name, parent, delimiter):
    return folders.same(name, parent) or bool(delimiter and name.startswith(parent + delimiter))


def perform(connection, call, cfg, *, watched=(), raw=None):
    method = call['method']
    rows = folders.listing(connection)
    if method == 'append':
        name = folders.choose(connection, call['folder'])
        connection.response('APPENDUID')  # No stale mapping from any earlier response.
        try:
            wire.ok(connection.append(wire.mailbox(name), '(' + ' '.join(call['flags']) + ')',
                                      call.get('date'), raw if raw is not None else call['raw'].encode('utf-8')))
        except wire.Refused:
            raise wire.WireError('APPEND refused; no retry') from None
        except (OSError, imaplib.IMAP4.error, wire.WireError):
            raise wire.WireError('APPEND outcome unknown; inspect the folder before repeating') from None
        _, response = connection.response('APPENDUID')
        if not response or response == [None]: return 'appended; destination UID unavailable (no APPENDUID); do not repeat'
        if len(response) != 1 or not isinstance(response[0], bytes) or not re.fullmatch(rb'[1-9][0-9]{0,9} [1-9][0-9]{0,9}', response[0]):
            return 'appended; invalid APPENDUID; destination UID unknown; do not repeat'
        validity, number = map(int, response[0].split())
        if max(validity, number) > 4294967295: return 'appended; invalid APPENDUID; destination UID unknown; do not repeat'
        return f'appended; destination UID {number}, UIDVALIDITY {validity}'
    name = call['name']
    if method == 'folder.create':
        command, args = connection.create, (wire.mailbox(name),)
    else:
        if name.casefold() == 'inbox': raise wire.WireError('INBOX cannot be renamed or deleted')
        row = next((row for row in rows if folders.same(row['name'], name)), None)
        if row is None: raise wire.WireError('folder not found')
        name = row['name']
        # All current subscriptions watch cfg.mailbox. An ancestor rename can
        # relocate that mailbox too; never silently move its cache or progress.
        if under(cfg['mailbox'], name, row['delimiter']):
            raise wire.WireError('configured mailbox or its ancestor cannot be renamed or deleted; subscriptions stay bound')
        if any(under(folder, name, row['delimiter']) for folder in watched):
            raise wire.WireError('a subscribed folder or its ancestor cannot be renamed or deleted')
        if method == 'folder.rename':
            if call['new'].casefold() == 'inbox': raise wire.WireError('INBOX cannot be a rename destination')
            if any(folders.same(r['name'], call['new']) for r in rows):
                raise wire.WireError('rename destination already exists')
            command, args = connection.rename, (wire.mailbox(name), wire.mailbox(call['new']))
        else:
            if any((r['role'] or r['special_use']) and under(r['name'], name, row['delimiter']) for r in rows):
                raise wire.WireError('role folders or their ancestors cannot be deleted, even with --force')
            total, _ = folders.counts(connection, name)
            if total and not call.get('force', False): raise wire.WireError('non-empty folder requires --force')
            command, args = connection.delete, (wire.mailbox(name),)
    try:
        wire.ok(command(*args))
    except wire.Refused:
        raise wire.WireError(method + ' refused; no retry') from None
    except (OSError, imaplib.IMAP4.error, wire.WireError):
        raise wire.WireError(method + ' outcome unknown; inspect folders before repeating') from None
    return method + ' completed: ' + json.dumps(name, ensure_ascii=False)
