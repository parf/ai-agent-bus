"""Explicitly enabled UID-only mailbox changes; never CLOSE or bare EXPUNGE."""
import datetime
import imaplib

from cache import uid
from mail import MailError
import folders
import operations
import wire

METHODS = {'copy', 'move', 'trash', 'delete', 'compact_trash'}


def require_enabled(configuration, method):
    if method in METHODS and not configuration.get('allow_destructive', False):
        raise MailError(method + ' requires Configuration allow_destructive=true (off by default)')


def capabilities(connection, method):
    if method in {'move', 'trash'} and not ({'MOVE', 'UIDPLUS'} & set(connection.capabilities)):
        raise wire.WireError(method + ' requires MOVE or UIDPLUS; no copy was made')
    if method in {'delete', 'compact_trash'} and 'UIDPLUS' not in connection.capabilities:
        raise wire.WireError(method + ' requires UIDPLUS for exact-UID expunge')


def trash_folder(connection):
    names = [row['name'] for row in folders.listing(connection) if row['role'] == 'Trash' and row['selectable']]
    if len(names) != 1:
        raise wire.WireError('no selectable Trash role folder' if not names else 'ambiguous Trash role: more than one selectable folder')
    return names[0]


def deleted_permission(connection):
    if '\\deleted' not in getattr(connection, 'permanent_flags', set()):
        raise wire.WireError('server does not permit persistent \\Deleted; no mail was changed')


def copyuid(connection, number):
    """Read and validate a fresh one-UID COPYUID mapping, never a stale reply."""
    _, values = connection.response('COPYUID')
    if not values or values == [None]:
        return None
    if len(values) != 1 or not isinstance(values[0], bytes) or len(values[0]) > 256:
        raise wire.WireError('invalid COPYUID mapping')
    try:
        epoch, source, destination = values[0].decode('ascii').split()
        epoch = uid(epoch)
        sources, destinations = operations.uid_set(source), operations.uid_set(destination)
        if sources != [number] or len(destinations) != 1:
            raise ValueError()
        return epoch, destinations[0]
    except (ValueError, UnicodeError):
        raise wire.WireError('invalid COPYUID mapping') from None


def copied_to(mapping):
    return ('destination UID ' + str(mapping[1]) + ' (UIDVALIDITY ' + str(mapping[0]) + ')') if mapping else 'destination UID unavailable'


def copy_one(connection, number, destination, verb):
    connection.response('COPYUID')  # Clear any unsolicited or preceding mapping.
    try:
        response = connection.uid(verb, str(number), wire.mailbox(destination))
        wire.ok(response)
    except wire.Refused:
        raise  # A definite refusal is never retried or turned into another verb.
    except (OSError, imaplib.IMAP4.error, wire.WireError):
        raise wire.FlagOutcomeUnknown(verb + ' outcome unknown; not retried') from None
    try:
        mapping = copyuid(connection, number)
        return mapping, None
    except wire.WireError:
        return None, 'invalid COPYUID mapping'


def remove_one(connection, number):
    """Return confirmed removal or a definite refusal; loss after STORE is unknown."""
    connection.seen_store_active = True
    try:
        response = connection.uid('STORE', str(number), '+FLAGS.SILENT', '(\\Deleted)')
        wire.ok(response)
    except wire.Refused:
        return 'source kept; Deleted flag refused', False
    except (OSError, imaplib.IMAP4.error, wire.WireError):
        raise wire.FlagOutcomeUnknown('Deleted flag outcome unknown; source removal not retried') from None
    finally:
        connection.seen_store_active = False
    try:
        # UIDPLUS is checked before even the first flag write.
        wire.ok(connection.uid('EXPUNGE', str(number)))
        return 'source removed', True
    except wire.Refused:
        return 'source kept; Deleted flag set; UID EXPUNGE refused', False
    except (OSError, imaplib.IMAP4.error, wire.WireError):
        raise wire.FlagOutcomeUnknown('source removal outcome unknown after Deleted flag set; not retried') from None


def apply(connection, method, number, destination=None):
    capabilities(connection, method)
    fallback = method in {'move', 'trash'} and 'MOVE' not in connection.capabilities
    if method == 'delete' or fallback:
        deleted_permission(connection)
    if not wire.exists(connection, number):
        raise wire.WireError('UID is no longer present in the mailbox')
    if method == 'delete':
        text, removed = remove_one(connection, number)
        return ('deleted; ' if removed else 'delete incomplete; ') + text, removed
    if method == 'copy' or fallback:
        mapping, problem = copy_one(connection, number, destination, 'COPY')
        detail = copied_to(mapping)
        if method == 'copy':
            return 'copied, source kept; ' + detail + ('; ' + problem if problem else ''), False
        if mapping is None:
            return 'copied, source kept; ' + (problem or 'COPYUID omitted; removal not attempted'), False
        try:
            text, removed = remove_one(connection, number)
        except wire.FlagOutcomeUnknown as error:
            raise wire.FlagOutcomeUnknown('copied; ' + detail + '; ' + str(error)) from None
        return ('moved; ' if removed else 'copied, source kept; ') + detail + '; ' + text, removed
    mapping, problem = copy_one(connection, number, destination, 'MOVE')
    return 'moved; source removed; ' + copied_to(mapping) + ('; ' + problem if problem else ''), True


def compact_numbers(connection, older=None):
    capabilities(connection, 'compact_trash')
    deleted_permission(connection)
    arguments = ['ALL']
    if older is not None:
        date = datetime.datetime.now(datetime.timezone.utc).date() - datetime.timedelta(days=older)
        cutoff = str(date.day).zfill(2) + '-' + 'Jan Feb Mar Apr May Jun Jul Aug Sep Oct Nov Dec'.split()[date.month - 1] + '-' + str(date.year)
        arguments = ['BEFORE', operations.quote(cutoff)]  # INTERNALDATE, never the message's Date header.
    return sorted(wire.criteria_search(connection, arguments))
