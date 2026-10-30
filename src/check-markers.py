#!/usr/bin/env python3
"""Fail a landing whose diff adds a git conflict-marker line (J29).

A marker left behind by a rebase or a merge is text that renders and parses as prose, so nothing else
catches it (9d761fd7 landed `||||||| parent of ...` in a plan). The check reads what the working tree adds
against main: the commits, the staged and unstaged changes and the untracked files, as smoke-select.py does.

  python3 src/check-markers.py            the real diff; exit 1 and the lines when it adds a marker
  python3 src/check-markers.py --stdin    a unified diff on standard input (the tests use this)

A marker is the full git form at the start of a line: seven `<`, `|` or `>` then a space or the end of the
line, and seven `=` alone. A line of `=======` is also a Markdown setext heading underline, so it counts only
inside a `<<<<<<<` ... `>>>>>>>` block that the same file adds.
"""
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OPEN, BASE, CLOSE, MID = '<' * 7, '|' * 7, '>' * 7, '=' * 7
# Fixed prefixes and no external diff: a user's git config must not change the headers read below.
DIFF = ('-U0', '--no-color', '--no-ext-diff', '--src-prefix=a/', '--dst-prefix=b/')


def marker(line):
    """'open', 'base', 'close' or 'mid' for a conflict-marker line, else None."""
    for kind, mark in (('open', OPEN), ('base', BASE), ('close', CLOSE)):
        if line.startswith(mark) and (len(line) == len(mark) or line[len(mark)] == ' '):
            return kind
    return 'mid' if line == MID else None


def scan(added):
    """added: (path, line number, text) in order. Returns the marker lines as (path, number, text)."""
    found, inside = [], {}
    for path, number, text in added:
        kind = marker(text.rstrip('\r'))
        if kind == 'open':
            inside[path] = True
        if kind in ('open', 'base', 'close') or (kind == 'mid' and inside.get(path)):
            found.append((path, number, text))
        if kind == 'close':
            inside[path] = False
    return found


_ESCAPES = {'a': 7, 'b': 8, 't': 9, 'n': 10, 'v': 11, 'f': 12, 'r': 13, '"': 34, '\\': 92}


def unquote(name):
    """A path as git prints it: plain, or in double quotes with C escapes and octal bytes for the unusual."""
    if not (len(name) >= 2 and name.startswith('"') and name.endswith('"')):
        return name
    out, body, i = bytearray(), name[1:-1], 0
    while i < len(body):
        c = body[i]
        if c == '\\' and i + 1 < len(body):
            n = body[i + 1]
            if n in _ESCAPES:
                out.append(_ESCAPES[n])
                i += 2
            elif n.isdigit() and re.fullmatch(r'[0-7]{3}', body[i + 1:i + 4] or ''):
                out.append(int(body[i + 1:i + 4], 8))
                i += 4
            else:
                out += c.encode()
                i += 1
        else:
            out += c.encode()
            i += 1
    return out.decode('utf-8', 'replace')


def added_from_diff(diff, path=None):
    """The added lines of a unified diff, each with its path and line number in the new file.

    The path is the caller's when it knows it (git lists changed names NUL-separated, never quoted);
    otherwise it comes from the `+++ b/...` header, unquoted.
    """
    known, number = path, 0
    for line in diff.splitlines():
        if line.startswith('+++ '):
            if known is None or path is None:
                name = unquote(line[4:])
                path = name[2:] if name.startswith('b/') else None
        elif line.startswith('@@'):
            m = re.search(r'\+(\d+)', line)
            number = int(m.group(1)) if m else 0
        elif path and line.startswith('+') and not line.startswith('+++'):
            yield path, number, line[1:]
            number += 1
        elif path and line.startswith(' '):
            number += 1


def git(root, *args):
    return subprocess.run(['git', '-C', str(root), *args], capture_output=True, text=True, errors='replace').stdout


def added_from_tree(root=ROOT):
    """What the working tree adds against where it branched from main: the final tree, not each commit on the way."""
    added = []
    base = git(root, 'merge-base', 'main', 'HEAD').strip() or 'HEAD'
    # Names come NUL-separated, so no name is ever quoted; each file's own diff is then read by that name.
    for name in git(root, 'diff', '--name-only', '-z', '--no-renames', base).split('\0'):
        if name:
            added += list(added_from_diff(git(root, 'diff', *DIFF, base, '--', ':(literal)' + name), name))
    # Untracked files have no diff at all, so they are read.
    for name in git(root, 'ls-files', '--others', '--exclude-standard', '-z').split('\0'):
        path = root / name
        try:
            if name and path.is_file() and not path.is_symlink() and path.stat().st_size < 2_000_000:
                for number, text in enumerate(path.read_text(errors='replace').splitlines(), 1):
                    added.append((name, number, text))
        except OSError:
            pass
    return added


def main(argv):
    added = list(added_from_diff(sys.stdin.read())) if '--stdin' in argv else added_from_tree()
    found = scan(added)
    for path, number, text in found:
        print('conflict marker: %s:%d: %s' % (path, number, text[:80]))
    if found:
        print('%d conflict-marker line(s) added; resolve the merge or rebase that left them' % len(found))
    return 1 if found else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
