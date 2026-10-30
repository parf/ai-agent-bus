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


def added_from_diff(diff):
    """The added lines of a unified diff, each with its path and line number in the new file."""
    path, number = None, 0
    for line in diff.splitlines():
        if line.startswith('+++ '):
            path = line[6:] if line.startswith('+++ b/') else None
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
    added = []
    base = subprocess.run(['git', '-C', str(root), 'rev-parse', '--verify', 'main'], capture_output=True).returncode == 0
    # Binary files have no text diff; untracked files have no diff at all, so they are read.
    diffs = [git(root, 'diff', *DIFF, 'main...HEAD') if base else '', git(root, 'diff', *DIFF, 'HEAD')]
    seen = set()
    for diff in diffs:
        for item in added_from_diff(diff):
            if item not in seen:
                seen.add(item)
                added.append(item)
    for name in git(root, 'ls-files', '--others', '--exclude-standard').splitlines():
        path = root / name
        try:
            if path.is_file() and not path.is_symlink() and path.stat().st_size < 2_000_000:
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
