"""Bounded sender/subject matching; never renders or fetches email content."""
from email import policy
from email.parser import BytesHeaderParser
import re
import unicodedata

from budget import alarm_budget

MAX_HEADERS = 8192
MAX_TEXT = 1536
MAX_PATTERN = 200
JS_SPACE = '\t\n\v\f\r \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff'


class MatchError(ValueError):
    pass


class RegexTimeout(TimeoutError):
    pass


def plain(value):
    return ' '.join(''.join(' ' if unicodedata.category(c).startswith('C') else c
                            for c in unicodedata.normalize('NFC', value)).split())


def match_text(raw):
    """Exactly one addr-spec, lowercased domain, newline, decoded subject.

    Overlong/malformed headers are refused before matching, including match-all;
    header ambiguity must never select an attacker-chosen address. Subject text
    is capped on a UTF-8 boundary, and display names never enter the match.
    """
    if not isinstance(raw, bytes) or len(raw) > MAX_HEADERS:
        raise MatchError('sender/subject headers exceed the 8192-byte limit')
    message = BytesHeaderParser(policy=policy.default).parsebytes(raw)
    senders, subjects = message.get_all('From', []), message.get_all('Subject', [])
    if message.defects or len(senders) != 1 or len(subjects) > 1:
        raise MatchError('ambiguous or malformed sender/subject headers')
    sender = senders[0]
    if sender.defects or len(sender.addresses) != 1:
        raise MatchError('mail must have one valid sender address')
    address = sender.addresses[0]
    if not address.username or not address.domain:
        raise MatchError('mail sender must have a domain')
    name = unicodedata.normalize('NFC', address.username) + '@' + address.domain.lower()
    if plain(name) != name or len(name.encode()) > 320:
        raise MatchError('invalid or oversized sender address')
    subject = subjects[0] if subjects else ''
    if getattr(subject, 'defects', ()):
        raise MatchError('malformed subject encoding')
    subject = plain(str(subject))
    available = MAX_TEXT - len(name.encode()) - 1
    return name + '\n' + subject.encode()[:available].decode('utf-8', 'ignore')


def compile_filter(value):
    """Common JS/Python subset: literals, ASCII classes, groups, anchors,
    alternation, quantifiers and d/w/s escapes. No lookarounds, inline flags,
    backreferences, named groups, Unicode classes or class escape shorthands.
    Flags i/m/s/u only; u selects JS Unicode case-folding of ASCII ranges.
    """
    if not isinstance(value, str):
        raise MatchError('filter must be text')
    flags = ''
    if value.startswith('/'):
        end = value.rfind('/')
        if end < 1:
            raise MatchError('filter needs its closing slash')
        value, flags = value[1:end], value[end + 1:]
    if len(value) > MAX_PATTERN or any(ord(c) < 32 for c in value):
        raise MatchError('filter is at most 200 characters without controls')
    if len(set(flags)) != len(flags) or any(c not in 'imsu' for c in flags):
        raise MatchError('filter flags are i m s u, without duplicates')
    options = (re.I if 'i' in flags else 0) | (re.M if 'm' in flags else 0) | (re.S if 's' in flags else 0)
    # Python's Unicode ignore-case additionally equates dotted/dotless I to I.
    # JS never does; for non-u JS also excludes Kelvin sign and long S.
    exclusions = '\u0130\u0131' + ('' if 'u' in flags else '\u212a\u017f')
    guard = '(?!(?-i:[' + exclusions + ']))' if 'i' in flags else ''
    word = 'A-Za-z0-9_' + ('\u212a\u017f' if 'i' in flags and 'u' in flags else '')
    escapes = {'d': '[0-9]', 'D': '[^0-9]', 'w': '[' + word + ']', 'W': '[^' + word + ']',
               's': '[' + JS_SPACE + ']', 'S': '[^' + JS_SPACE + ']'}
    out, i = [], 0
    while i < len(value):
        char = value[i]
        if char == '\\':
            i += 1
            if i == len(value):
                raise MatchError('filter ends with a backslash')
            escaped = value[i]
            if escaped in escapes:
                # Case-folding must not expand JS's ASCII-only \w definition.
                out.append('(?-i:' + escapes[escaped] + ')')
            elif escaped in r'\.^$|?*+(){}[]/':
                out.append(re.escape(escaped))
            elif escaped in 'nrt':
                out.append({'n': r'\n', 'r': r'\r', 't': r'\t'}[escaped])
            else:
                raise MatchError('unsupported filter escape')
        elif char == '[':
            end = value.find(']', i + 1)
            body = value[i + 1:end] if end >= 0 else ''
            if not body or not body.isascii() or '[' in body or '\\' in body:
                raise MatchError('filter classes must be nonempty ASCII without escapes')
            # Negate the guarded positive class, so excluded fold characters
            # still match [^a-z] just as they do in JS.
            if body.startswith('^'):
                out.append('(?:(?!' + guard + '[' + body[1:] + '])[\\s\\S])')
            else:
                out.append('(?:' + guard + value[i:end + 1] + ')')
            i = end
        elif char == '(' and value[i:i + 2] == '(?':
            if value[i:i + 3] != '(?:':
                raise MatchError('only ordinary and noncapturing groups are supported')
            out.append('(?:')
            i += 2
        elif char == '{':
            quantifier = re.match(r'\{([0-9]+)(?:,([0-9]*))?\}', value[i:])
            if not quantifier or any(int(n) > 10000 for n in quantifier.groups() if n):
                raise MatchError('invalid or excessive filter quantifier')
            out.append(quantifier.group())
            i += len(quantifier.group()) - 1
        elif char == '}' or (char in '*+?' and i and value[i - 1] in '*+?}'):
            raise MatchError('unsupported filter quantifier')
        elif char.isascii() and char.isalpha():
            out.append('(?:' + guard + char + ')')
        elif char in '\u0130\u0131' and 'i' in flags:
            out.append('(?-i:' + char + ')')
        elif char in '\u212a\u017f' and 'i' in flags and 'u' not in flags:
            out.append('(?-i:' + char + ')')
        else:
            out.append(char)
        i += 1
    try:
        return re.compile(''.join(out), options)
    except re.error:
        raise MatchError('invalid filter expression') from None


def matches(compiled, text):
    if not isinstance(text, str) or len(text.encode()) > MAX_TEXT:
        raise MatchError('matching text exceeds its 1536-byte limit')
    with alarm_budget(0.05, RegexTimeout):
        return compiled.search(text) is not None
