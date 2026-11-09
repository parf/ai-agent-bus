#!/usr/bin/env python3
"""Give each AB-DVP window its own directory under .ai-team/, and copy its memory notes there.

  windows.py [plan]                    dry run, the default: what would move where, per window; writes nothing
  windows.py apply --window NAME ...   one or more windows, by bus name without '#' (ab-w1a); --all for every one

A window's directory is .ai-team/<bus name without '#'>/, e.g. .ai-team/ab-w1a/. The launcher config
(.ai-team/ab-dvp-v2-config.json) starts each window in its `cwd`; `apply` points that one entry at the new directory.
Claude keeps a window's memory in ~/<account dir>/projects/<its cwd with every non-alphanumeric as '-'>/memory, so
a new directory means a new, empty memory: `apply` COPIES the notes the window had (from its old directory's memory,
same account) into the new one. It never moves, deletes or overwrites anything: originals stay, a note already
present in the target is kept as it is. It writes only inside the chosen window's own directory and memory, and the
config entries of the chosen windows. DIR and the CEO keep the directories they have.
"""
import argparse
import json
import os
import re
import shutil
import sys
import time
from pathlib import Path

LOCAL = Path(os.environ.get('WINDOWS_LOCAL', Path(__file__).resolve().parents[1]))
HOME = Path(os.environ.get('WINDOWS_HOME', Path.home()))
CONFIG = LOCAL / 'ab-dvp-v2-config.json'
BACKUP = LOCAL / 't13-backup'
KEEP = {'ab-dir', 'ab-ceo'}   # already in their own directories; their memory is not moved


def slug(path):
    return re.sub(r'[^A-Za-z0-9]', '-', str(path))


def account_dir(command):
    """ab-claude -2 is the second account (~/.claude2); ab-claude alone the first; the others keep no Claude memory."""
    if 'ab-claude' not in command:
        return None
    return HOME / ('.claude2' if re.search(r'\bab-claude\s+-2\b', command) else '.claude')


def memory(account, cwd):
    return account / 'projects' / slug(cwd) / 'memory'


def windows(config):
    """(bus name without '#', window entry) for every window that has a bus name."""
    for entry in config['windows']:
        bus = entry.get('env', {}).get('AGENT_BUS_NAME', '')
        if bus.startswith('#'):
            yield bus[1:], entry


def target_dir(bus):
    return LOCAL / bus


def notes(directory):
    return sorted(p for p in directory.glob('*.md') if p.is_file()) if directory.is_dir() else []


def describe(name, entry):
    """The facts of one window: where it runs, where it will, and which notes go where."""
    old = Path(entry['cwd'])
    new = target_dir(name)
    account = account_dir(entry['command'])
    out = {'bus': name, 'window': entry['name'], 'old': old, 'new': new, 'keep': name in KEEP or old == new, 'notes': []}
    if account is None or out['keep']:
        return out
    source, target = memory(account, old), memory(account, new)
    out.update(source=source, target=target)
    for note in notes(source):
        have = target / note.name
        state = 'copy' if not have.exists() else ('same' if have.read_bytes() == note.read_bytes() else 'differs')
        out['notes'].append((note.name, state))
    return out


def plan(config):
    rows = [describe(name, entry) for name, entry in windows(config)]
    for r in rows:
        print(f"{r['window']:6} #{r['bus']:8} {r['old']}  ->  {r['new']}" + ('   (unchanged)' if r['keep'] else ''))
        if 'source' not in r:
            if not r['keep']:
                print('       no Claude memory (not ab-claude): directory only')
            continue
        counts = {s: sum(1 for _, x in r['notes'] if x == s) for s in ('copy', 'same', 'differs')}
        print(f"       notes from {r['source']}  ({len(r['notes'])})")
        print(f"       into       {r['target']}: copy {counts['copy']}, already there {counts['same']}, kept as they are {counts['differs']}")
        for n, s in r['notes']:
            print(f"         {s:7} {n}")
    return rows


def inside(path, root):
    path, root = Path(os.path.abspath(path)), Path(os.path.abspath(root))
    return root == path or root in path.parents


def copy_new(source, dest):
    """Copy `source` to `dest` only if nothing is at `dest`; True when this call made it.

    The file is created exclusively (O_EXCL, never following a link), so a note another writer put there a moment
    ago, or a dangling link, stops the copy instead of being replaced. The source's mode and times go onto the file
    this call created, through its own descriptor, never onto a path someone else may have since swapped.
    """
    try:
        fd = os.open(dest, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    except FileExistsError:
        return False
    with os.fdopen(fd, 'wb') as out, open(source, 'rb') as src:
        shutil.copyfileobj(src, out)
        out.flush()   # the times last, or the write itself would set them
        info = os.fstat(src.fileno())
        os.fchmod(out.fileno(), info.st_mode & 0o777)
        os.utime(out.fileno(), ns=(info.st_atime_ns, info.st_mtime_ns))
    return True


def apply(config, names):
    chosen = [(n, e) for n, e in windows(config) if n in names]
    missing = set(names) - {n for n, _ in chosen}
    if missing:
        sys.exit('no such window: ' + ', '.join(sorted(missing)))
    done = []
    for name, entry in chosen:
        r = describe(name, entry)
        if r['keep']:
            print(f"#{name}: unchanged (keeps its directory)")
            continue
        new = r['new']
        new.mkdir(exist_ok=True)
        if 'target' in r:
            # The one place this writes memory: this window's own, under its own new directory's name.
            account = account_dir(entry['command'])
            target = r['target']
            if not inside(target, account / 'projects' / slug(new)):
                sys.exit(f'refusing to write {target}: not #{name}\'s own memory')
            target.mkdir(parents=True, exist_ok=True)
            for note in notes(r['source']):
                copy_new(note, target / note.name)
        entry['cwd'] = str(new)
        done.append(name)
    return done


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument('action', nargs='?', default='plan', choices=('plan', 'apply'))
    ap.add_argument('--window', action='append', default=[], metavar='NAME', help="bus name without '#', e.g. ab-w1a")
    ap.add_argument('--all', action='store_true', help='every window (apply only)')
    args = ap.parse_args(argv)
    config = json.loads(CONFIG.read_text())
    if args.action == 'plan':
        plan(config)
        return 0
    names = [n for n, _ in windows(config)] if args.all else args.window
    if not names:
        sys.exit('apply needs --window NAME or --all; run `windows.py plan` first')
    BACKUP.mkdir(exist_ok=True)
    backup = BACKUP / ('ab-dvp-v2-config.json.' + time.strftime('%Y%m%d-%H%M%S'))
    shutil.copy2(CONFIG, backup)
    done = apply(config, names)
    if done:
        text = json.dumps(config, indent=1) + '\n'
        tmp = CONFIG.with_name(CONFIG.name + '.new')
        tmp.write_text(text)
        os.replace(tmp, CONFIG)
    print('applied: ' + (', '.join('#' + n for n in done) or 'nothing') + f'; config backup {backup}')
    return 0


if __name__ == '__main__':
    sys.exit(main())
