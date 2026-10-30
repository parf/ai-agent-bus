#!/usr/bin/env python3
"""Fail a landing whose Markdown leaves a link to nothing, including a link in a file it did not touch (J53).

A link check that reads only the files a change edits misses the one that matters most when a change moves
or deletes a file: the links to it sit in files nobody edited (B.27m moved a plan and left DONE.md pointing at
the old path). The check reads what the working tree changes against main (the commits, the staged and unstaged
changes and the untracked files, as check-markers.py does) and reports two things:

  - a link in a changed or new Markdown file whose file, or `#anchor` in that file, does not exist;
  - a link, in ANY Markdown file of the tree, to a path the change moved or deleted.

  python3 src/check-links.py              the real tree; exit 1 and the links when one is broken

Skipped: links with a scheme (https:, mailto:), absolute paths, anything inside a fenced code block or a code
span, and an anchor on a target that is not Markdown. An anchor is a heading slug as GitHub makes it, with
`-1`, `-2` for repeats, or an explicit `id=` / `name=` attribute.
"""
import os
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parent.parent
LINK = re.compile(r'\]\(([^)#\s]*)(?:#([^)\s]*))?\)')
SCHEME = re.compile(r'^(?:[A-Za-z][A-Za-z0-9+.-]*:|//)')
FENCE = re.compile(r'^\s*(```|~~~)')


def git(root, *args):
    return subprocess.run(['git', '-C', str(root), *args], capture_output=True, text=True, errors='replace').stdout


def links(text):
    """(line number, path, anchor) of each inline link outside fenced code and code spans."""
    out, fence = [], None
    for number, line in enumerate(text.splitlines(), 1):
        m = FENCE.match(line)
        if m:
            fence = None if fence == m.group(1) else (fence or m.group(1))
            continue
        if fence:
            continue
        for found in LINK.finditer(re.sub(r'`[^`]*`', '', line)):
            out.append((number, unquote(found.group(1)), found.group(2)))
    return out


def slug(heading):
    heading = re.sub(r'<[^>]+>', '', heading.strip().lower())
    return re.sub(r'[^\w\- ]', '', heading).replace(' ', '-')


def anchors(text):
    seen, out, fence = {}, set(), None
    for line in text.splitlines():
        m = FENCE.match(line)
        if m:
            fence = None if fence == m.group(1) else (fence or m.group(1))
            continue
        if fence:
            continue
        h = re.match(r'#{1,6}\s+(.*?)\s*#*\s*$', line)
        if h:
            base = slug(h.group(1))
            n = seen.get(base, 0)
            seen[base] = n + 1
            out.add(base if n == 0 else '%s-%d' % (base, n))
        out.update(re.findall(r'''\b(?:id|name)=["']([^"']+)["']''', line))
    return out


def target(root, source, path):
    """The repository-relative path a link from `source` names, or None when it leaves the repository."""
    if not path:
        return source
    full = Path(os.path.normpath((root / source).parent / path))
    try:
        return full.relative_to(root).as_posix()
    except ValueError:
        return None


def changes(root):
    """(markdown files changed or new, paths moved away or deleted) against where the tree branched from main."""
    base = git(root, 'merge-base', 'main', 'HEAD').strip() or 'HEAD'
    changed, gone = set(), set()
    names = git(root, 'diff', '--name-status', '-z', '-M', '--no-ext-diff', base).split('\0')
    i = 0
    while i < len(names) and names[i]:
        status = names[i]
        if status.startswith('R') or status.startswith('C'):
            old, new = names[i + 1], names[i + 2]
            i += 3
            if status.startswith('R'):
                gone.add(old)
            changed.add(new)
        else:
            path = names[i + 1]
            i += 2
            (gone if status.startswith('D') else changed).add(path)
    for name in git(root, 'ls-files', '--others', '--exclude-standard', '-z').split('\0'):
        if name:
            changed.add(name)
    return {p for p in changed if p.endswith('.md') and (root / p).is_file()}, gone


def tracked_markdown(root):
    names = git(root, 'ls-files', '-z', '--cached', '--others', '--exclude-standard').split('\0')
    return {n for n in names if n.endswith('.md') and (root / n).is_file()}


def check(root=ROOT, inbound=True):
    """Problems as (file, line, link, why), sorted."""
    root = Path(root)
    changed, gone = changes(root)
    scan = changed | (tracked_markdown(root) if inbound and gone else set())
    problems, cache = set(), {}
    for source in sorted(scan):
        for number, path, anchor in links((root / source).read_text(errors='replace')):
            if SCHEME.match(path) or path.startswith('/'):
                continue
            there = target(root, source, path)
            if there is None:
                continue
            link = path + ('#' + anchor if anchor else '')
            if there in gone and not (root / there).exists():
                problems.add((source, number, link, 'moved or deleted by this change'))
            elif source in changed:
                full = root / there
                if not full.exists():
                    problems.add((source, number, link, 'no such file'))
                elif anchor and there.endswith('.md') and full.is_file():
                    if there not in cache:
                        cache[there] = anchors(full.read_text(errors='replace'))
                    if anchor not in cache[there]:
                        problems.add((source, number, link, 'no such anchor'))
    return sorted(problems)


def main(argv):
    problems = check()
    for source, number, link, why in problems:
        print('%s:%d: %s -> %s' % (source, number, link, why))
    if problems:
        print('%d broken link(s); retarget them, never delete the reference' % len(problems))
    return 1 if problems else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
